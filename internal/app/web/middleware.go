package web

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gookit/rux/v2"
)

const (
	authCookieName = "eget_web_token"
	authTokenHead  = "X-EGET-Token"
)

// recoverMiddleware converts a handler panic into a 500 response.
func (s *Server) recoverMiddleware(c *rux.Context) {
	defer func() {
		if rec := recover(); rec != nil {
			s.logf("panic while serving %s: %v", c.Req.URL.Path, rec)
			if !c.IsAborted() {
				c.AbortWithStatus(http.StatusInternalServerError, "internal error")
			}
		}
	}()
	c.Next()
}

// securityHeadersMiddleware applies the console's baseline response headers.
func (s *Server) securityHeadersMiddleware(c *rux.Context) {
	header := c.Resp.Header()
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Referrer-Policy", "no-referrer")
	header.Set("X-Frame-Options", "DENY")
	header.Set("Content-Security-Policy",
		"default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; "+
			"connect-src 'self'; img-src 'self' data:; frame-ancestors 'none'")
	c.Next()
}

// hostMiddleware rejects requests whose Host header is not a known local name.
// This blocks DNS-rebinding attacks against the loopback listener.
func (s *Server) hostMiddleware(c *rux.Context) {
	host := requestHost(c.Req)
	if host == "" || s.opts.allowedHosts()[host] {
		c.Next()
		return
	}
	for _, allowed := range s.opts.AllowHosts {
		if strings.EqualFold(host, strings.TrimSpace(strings.ToLower(allowed))) {
			c.Next()
			return
		}
	}
	c.AbortWithStatus(http.StatusForbidden, "host not allowed")
}

func requestHost(r *http.Request) string {
	host := strings.TrimSpace(r.Host)
	if host == "" {
		return ""
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		return strings.ToLower(strings.Trim(h, "[]"))
	}
	return strings.ToLower(strings.Trim(host, "[]"))
}

// authMiddleware enforces the bearer token. The cache mirror endpoints keep
// using Authorization: Bearer; the web UI uses X-EGET-Token, an auth cookie,
// or a one-time ?token= bootstrap.
func (s *Server) authMiddleware(c *rux.Context) {
	if s.opts.NoAuth() || publicPath(c.Req.URL.Path) {
		c.Next()
		return
	}
	if s.authenticated(c.Req) {
		c.Next()
		return
	}
	if s.bootstrapFromQuery(c) {
		c.Next()
		return
	}
	c.Resp.Header().Set("WWW-Authenticate", "Bearer")
	c.AbortWithStatus(http.StatusUnauthorized, "unauthorized")
}

func publicPath(path string) bool {
	return path == "/healthz" || path == "/readyz"
}

func (s *Server) authenticated(r *http.Request) bool {
	if token := bearerToken(r); token != "" && s.tokenEqual(token) {
		return true
	}
	if token := strings.TrimSpace(r.Header.Get(authTokenHead)); token != "" && s.tokenEqual(token) {
		return true
	}
	if cookie, err := r.Cookie(authCookieName); err == nil && s.tokenEqual(cookie.Value) {
		return true
	}
	return false
}

// bootstrapFromQuery accepts ?token=<token> once, planting an HttpOnly cookie so
// the token leaves the URL bar. Cache mirror clients never use it.
func (s *Server) bootstrapFromQuery(c *rux.Context) bool {
	token := c.Query("token")
	if token == "" || !s.tokenEqual(token) {
		return false
	}
	http.SetCookie(c.Resp, &http.Cookie{
		Name:     authCookieName,
		Value:    s.opts.Token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
	return true
}

func bearerToken(r *http.Request) string {
	value := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(value, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(value, prefix))
}

func (s *Server) tokenEqual(candidate string) bool {
	expected := strings.TrimSpace(s.opts.Token)
	candidate = strings.TrimSpace(candidate)
	if expected == "" || candidate == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(candidate)) == 1
}

// csrfMiddleware guards state-changing requests: a browser cannot forge a
// custom header or a same-origin Origin on a cross-site form post.
func (s *Server) csrfMiddleware(c *rux.Context) {
	switch c.Req.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		c.Next()
		return
	}
	if strings.TrimSpace(c.Req.Header.Get(authTokenHead)) != "" {
		c.Next()
		return
	}
	origin := strings.TrimSpace(c.Req.Header.Get("Origin"))
	if origin == "" || sameOrigin(origin, c.Req) {
		c.Next()
		return
	}
	c.AbortWithStatus(http.StatusForbidden, "cross-site request rejected")
}

func sameOrigin(origin string, r *http.Request) bool {
	origin = strings.TrimSuffix(strings.ToLower(origin), "/")
	if !strings.HasPrefix(origin, "http://") && !strings.HasPrefix(origin, "https://") {
		return false
	}
	host := strings.ToLower(r.Host)
	return strings.HasSuffix(origin, "//"+host)
}

type requestLogEvent struct {
	TS         time.Time `json:"ts"`
	Method     string    `json:"method"`
	Path       string    `json:"path"`
	Status     int       `json:"status"`
	Bytes      int       `json:"bytes"`
	DurationMS int64     `json:"duration_ms"`
	RemoteAddr string    `json:"remote_addr,omitempty"`
}

// logMiddleware writes one line per request. The query string is never logged:
// ?token= would otherwise leak the credential.
func (s *Server) logMiddleware(c *rux.Context) {
	start := time.Now()
	c.Next()

	event := requestLogEvent{
		TS:         start,
		Method:     c.Req.Method,
		Path:       c.Req.URL.Path,
		Status:     statusOrOK(c.StatusCode()),
		Bytes:      c.Length(),
		DurationMS: time.Since(start).Milliseconds(),
		RemoteAddr: c.Req.RemoteAddr,
	}
	writer := s.opts.LogWriter
	if s.opts.JSONLog {
		_ = json.NewEncoder(writer).Encode(event)
		return
	}
	_, _ = fmt.Fprintf(writer, "%s %s %s %d bytes=%d duration_ms=%d remote_addr=%s\n",
		event.TS.Format("01-02 15:04:05"),
		event.Method,
		event.Path,
		event.Status,
		event.Bytes,
		event.DurationMS,
		event.RemoteAddr,
	)
}

func statusOrOK(status int) int {
	if status == 0 {
		return http.StatusOK
	}
	return status
}

func (s *Server) logf(format string, args ...any) {
	_, _ = fmt.Fprintf(s.opts.LogWriter, "[eget web] "+format+"\n", args...)
}
