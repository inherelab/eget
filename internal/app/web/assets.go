package web

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gookit/rux/v2"
)

// distFS holds the built Vite assets. The directory always exists (it keeps a
// .gitkeep placeholder) so a checkout without a frontend build still compiles
// and runs; in that case the SPA fallback explains how to build it.
//
//go:embed all:dist
var distFS embed.FS

func (s *Server) registerAssets(r *rux.Router) {
	assets, err := fs.Sub(distFS, "dist/assets")
	if err != nil {
		return
	}
	r.StaticFS("/assets", http.FS(assets))
}

// iconsFS holds the browser furniture of the console: tab icon, PWA icons and
// the web manifest. They are embedded in the binary instead of the frontend
// bundle so a checkout without a frontend build (and the token page it serves)
// still shows the console icon. assets/logo/exports stays the brand source of
// truth these files are copied from.
//
//go:embed icons
var iconsFS embed.FS

// browserIcons maps the served path to its embedded file and content type. The
// types are declared because Go's built-in mime table knows neither .ico nor
// .webmanifest, and the wrong type makes browsers drop the icon or the manifest.
var browserIcons = map[string]struct {
	file string
	typ  string
}{
	"/favicon.ico":                {"icons/favicon.ico", "image/x-icon"},
	"/favicon.svg":                {"icons/favicon.svg", "image/svg+xml"},
	"/apple-touch-icon.png":       {"icons/apple-touch-icon.png", "image/png"},
	"/android-chrome-192x192.png": {"icons/android-chrome-192x192.png", "image/png"},
	"/android-chrome-512x512.png": {"icons/android-chrome-512x512.png", "image/png"},
	"/site.webmanifest":           {"icons/site.webmanifest", "application/manifest+json"},
}

// registerBrowserIcons serves the embedded browser furniture. It fails closed on
// a missing file: the set is fixed at compile time, so a gap is a broken build
// rather than a request that deserves a 404.
func (s *Server) registerBrowserIcons(r *rux.Router) error {
	for path, icon := range browserIcons {
		data, err := iconsFS.ReadFile(icon.file)
		if err != nil {
			return fmt.Errorf("embedded browser icon %s: %w", path, err)
		}
		typ, data := icon.typ, data
		r.GET(path, func(c *rux.Context) {
			header := c.Resp.Header()
			header.Set("Content-Type", typ)
			// Not content-hashed, so cache briefly instead of forever.
			header.Set("Cache-Control", "public, max-age=3600")
			_, _ = c.Resp.Write(data)
		})
	}
	return nil
}

// handleCatchAll classifies requests that matched no route. It exists because
// rux serves those through NotFound/NotAllowed handlers that bypass the global
// middleware chain; routing them here keeps auth, security headers and request
// logging in force.
func (s *Server) handleCatchAll(c *rux.Context) {
	path := c.Req.URL.Path
	switch {
	case strings.HasPrefix(path, "/api/"), path == "/api":
		s.writeError(c, http.StatusNotFound, "not_found", fmt.Errorf("no API route for %s", path))
		return
	case strings.HasPrefix(path, "/assets/"):
		s.writeError(c, http.StatusNotFound, "not_found", fmt.Errorf("no asset at %s", path))
		return
	case strings.HasPrefix(path, "/files/"), path == "/files",
		strings.HasPrefix(path, "/download/"), path == "/download":
		// Machine-facing endpoints answer JSON rather than the SPA shell.
		s.writeError(c, http.StatusNotFound, "not_found", fmt.Errorf("no cache file at %s", path))
		return
	}
	if c.Req.Method != http.MethodGet && c.Req.Method != http.MethodHead {
		s.writeError(c, http.StatusMethodNotAllowed, "method_not_allowed",
			fmt.Errorf("%s is not allowed for %s", c.Req.Method, path))
		return
	}
	s.handleSPAFallback(c)
}

// handleSPAFallback serves the single-page app shell for unknown paths so
// client-side routes work on a hard refresh.
func (s *Server) handleSPAFallback(c *rux.Context) {
	if c.Req.Method != http.MethodGet && c.Req.Method != http.MethodHead {
		c.AbortWithStatus(http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	index, err := distFS.ReadFile("dist/index.html")
	if err != nil {
		c.HTMLString(http.StatusOK, unbuiltPage)
		return
	}
	c.HTMLString(http.StatusOK, string(index))
}

const unbuiltPage = `<!doctype html>
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
<span class="state">not built</span>
</div>
<h1>Frontend bundle not built</h1>
<p>The API is running, but the web UI assets are missing from this binary.</p>
<p>Build them once, then restart <code>eget web</code>:</p>
<pre>make web-build      # builds internal/app/web/dist, needs node + pnpm
go build -o eget ./cmd/eget</pre>
<p class="muted">Meanwhile: <a href="/healthz">/healthz</a>, <a href="/api/overview">/api/overview</a>, and the cache mirror at <a href="/manifest.json">/manifest.json</a>.</p>
</main>
</body>
</html>
`
