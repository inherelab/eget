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
body{margin:0;font:15px/1.6 ui-sans-serif,system-ui,"Segoe UI",sans-serif;background:#f6f7f9;color:#20242a}
main{max-width:640px;margin:12vh auto;padding:28px;background:#fff;border:1px solid #d9dde5;border-radius:10px}
h1{margin:0 0 12px;font-size:22px}
code{background:#eef4f3;padding:1px 6px;border-radius:5px;font-family:ui-monospace,Consolas,monospace}
ul{padding-left:20px}
</style>
</head>
<body>
<main>
<h1>Frontend bundle not built</h1>
<p>The API is running, but the web UI assets are missing from this binary.</p>
<p>Build them once, then restart <code>eget web</code>:</p>
<ul>
<li><code>make web-build</code> (requires node + pnpm)</li>
<li><code>go build -o eget ./cmd/eget</code></li>
</ul>
<p>API endpoints are available meanwhile, for example
<code>GET /healthz</code>, <code>GET /api/overview</code>, and the cache mirror at
<code>/manifest.json</code>.</p>
</main>
</body>
</html>
`
