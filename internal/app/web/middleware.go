package web

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gookit/rux/v2"
)

const (
	authCookieName = "eget_web_token"
	authTokenHead  = "X-EGET-Token"

	// Failed token attempts per remote address before the console starts
	// answering 429, and the window those failures are counted in.
	authFailureLimit  = 20
	authFailureWindow = time.Minute
	authFailureMaxIPs = 1024
)

// authLimiter throttles failed authentication attempts per remote address so a
// local process cannot brute-force the token.
type authLimiter struct {
	mu       sync.Mutex
	attempts map[string]*attemptWindow
	limit    int
	window   time.Duration
}

type attemptWindow struct {
	count int
	until time.Time
}

func newAuthLimiter(limit int, window time.Duration) *authLimiter {
	return &authLimiter{attempts: map[string]*attemptWindow{}, limit: limit, window: window}
}

// allow records an attempt and reports whether it may proceed.
func (l *authLimiter) allow(addr string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if entry, ok := l.attempts[addr]; ok && now.Before(entry.until) {
		entry.count++
		return entry.count <= l.limit
	}
	if len(l.attempts) >= authFailureMaxIPs {
		l.pruneLocked(now)
	}
	l.attempts[addr] = &attemptWindow{count: 1, until: now.Add(l.window)}
	return true
}

// reset forgets the failures of an address after a successful authentication.
func (l *authLimiter) reset(addr string) {
	l.mu.Lock()
	delete(l.attempts, addr)
	l.mu.Unlock()
}

func (l *authLimiter) pruneLocked(now time.Time) {
	for addr, entry := range l.attempts {
		if now.After(entry.until) {
			delete(l.attempts, addr)
		}
	}
}

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
	remote := c.Req.RemoteAddr
	if s.authenticated(c.Req) {
		if s.limiter != nil {
			s.limiter.reset(remote)
		}
		c.Next()
		return
	}
	if s.limiter != nil && !s.limiter.allow(remote, time.Now()) {
		c.AbortWithStatus(http.StatusTooManyRequests, "too many failed attempts")
		return
	}
	if s.bootstrapFromQuery(c) {
		c.Next()
		return
	}
	// A browser navigating to the console should get a form instead of a bare
	// 401; API and mirror clients keep the plain unauthorized answer.
	if htmlRequest(c.Req) && servesConsole(c.Req.URL.Path) {
		// The token page is this request's whole answer. Without Abort the router
		// carries on to the SPA route, which appends a second HTML document to
		// the same 401 response.
		c.HTMLString(http.StatusUnauthorized, tokenPage)
		c.Abort()
		return
	}
	c.Resp.Header().Set("WWW-Authenticate", "Bearer")
	c.AbortWithStatus(http.StatusUnauthorized, "unauthorized")
}

// htmlRequest reports whether the client looks like a browser loading a page.
func htmlRequest(r *http.Request) bool {
	return r.Method == http.MethodGet && strings.Contains(r.Header.Get("Accept"), "text/html")
}

// servesConsole reports whether a path belongs to the console UI rather than to
// the machine-facing API (which answers JSON/plain text).
func servesConsole(path string) bool {
	switch {
	case strings.HasPrefix(path, "/api/"), path == "/api",
		strings.HasPrefix(path, "/assets/"),
		strings.HasPrefix(path, "/files/"), path == "/files",
		strings.HasPrefix(path, "/download/"), path == "/download",
		path == "/manifest.json", path == "/healthz", path == "/readyz":
		return false
	default:
		return true
	}
}

// tokenPage is shown when an unauthenticated browser opens the console. The
// form submits back to "/" with ?token=…, which the auth middleware turns into
// the auth cookie.
const tokenPage = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<link rel="icon" href="/favicon.ico" sizes="32x32">
<link rel="icon" href="/favicon.svg" type="image/svg+xml">
<title>eget web</title>
<style>
:root{--paper:#e8eaed;--ink:#0f1720;--ink-2:#3f4a57;--ink-3:#5a6572;--rule:#d3d8de;--rail:#101a23;--rail-ink:#e6edf3;--signal:#0f766e;
--mono:ui-monospace,"JetBrains Mono","Cascadia Mono","SF Mono",Menlo,Consolas,monospace;
--sans:"Segoe UI Variable Text","Segoe UI",system-ui,-apple-system,sans-serif}
*{box-sizing:border-box}
body{margin:0;padding:52px 24px;background:var(--paper);color:var(--ink);font:14px/1.55 var(--sans)}
main{max-width:560px;margin:0 auto}
.head{display:flex;align-items:center;justify-content:space-between;gap:16px;padding-bottom:10px;border-bottom:1px solid var(--rule)}
.brand{display:flex;align-items:center;gap:8px;font:600 14px/1 var(--mono)}
.brand i{color:var(--signal);font-style:normal;font-weight:500}
.state{color:var(--ink-3);font:400 11px/1 var(--mono)}
h1{margin:22px 0 10px;font:600 17px/1.3 var(--mono);letter-spacing:-.01em}
p{margin:0 0 14px;color:var(--ink-2)}
dl{display:grid;grid-template-columns:60px 1fr;gap:7px 16px;margin:18px 0;padding:14px 0;border-top:1px solid var(--rule);border-bottom:1px solid var(--rule)}
dt{color:var(--ink-3);font:400 11px/1.7 var(--mono)}
dd{margin:0;font:400 12.5px/1.7 var(--mono);word-break:break-all}
form{display:flex;gap:8px;margin:0 0 12px}
input{flex:1;height:34px;padding:0 10px;border:1px solid var(--rule);border-radius:5px;background:#fff;color:inherit;font:13px/1 var(--mono)}
button{height:34px;padding:0 14px;border:1px solid var(--signal);border-radius:5px;background:var(--signal);color:#fff;font:500 12.5px/1 var(--mono);cursor:pointer;transition:background 120ms linear}
button:hover{background:#0d6a62}
pre{margin:0 0 16px;padding:13px 15px;border-radius:5px;background:var(--rail);color:var(--rail-ink);font:12.5px/1.6 var(--mono);overflow:auto}
.muted{color:var(--ink-3);font:400 12px/1.5 var(--mono)}
a{color:var(--signal)}
:focus-visible{outline:2px solid var(--signal);outline-offset:2px}
@media (prefers-reduced-motion:reduce){button{transition:none}}
</style>
</head>
<body>
<main>
<div class="head">
<div class="brand"><svg width="20" height="20" viewBox="0 0 512 512" role="img" aria-label="eget"><path fill="currentColor" fill-rule="evenodd" d="M136 76H280A60 60 0 0 1 340 136V216A124 124 0 0 0 216 340H136A60 60 0 0 1 76 280V136A60 60 0 0 1 136 76Z"/><rect x="300" y="300" width="136" height="136" rx="34" fill="currentColor"/></svg>eget<i>web</i></div>
<span class="state">token required</span>
</div>
<h1>This console is protected by a token</h1>
<p>eget prints it in the terminal when the server starts, together with the URL that carries it.</p>
<dl>
<dt>token</dt><dd>&lt;the token&gt;</dd>
<dt>open</dt><dd>http://127.0.0.1:&lt;port&gt;/?token=&lt;the token&gt;</dd>
</dl>
<form method="get" action="/">
<input type="password" name="token" placeholder="paste the token" autofocus aria-label="Token">
<button type="submit">Open console</button>
</form>
<p class="muted">Submitting this form (or visiting the open URL) stores the token as a cookie, so this is a one-time step.</p>
</main>
</body>
</html>
`

// publicPath lists the token-free endpoints: the liveness probes and the
// browser furniture. Icons and the web manifest load before a token exists (the
// token page is itself a browser page), and browsers fetch a manifest with
// credentials omitted, so gating it behind the cookie would break both.
func publicPath(path string) bool {
	if path == "/healthz" || path == "/readyz" {
		return true
	}
	_, isIcon := browserIcons[path]
	return isIcon
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
