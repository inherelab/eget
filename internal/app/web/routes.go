package web

import (
	"github.com/gookit/rux/v2"
)

func (s *Server) registerRoutes(r *rux.Router) {
	r.GET("/healthz", s.handleHealthz)
	r.GET("/readyz", s.handleReadyz)

	r.Group("/api", func() {
		r.GET("/overview", s.handleOverview)
		r.GET("/packages", s.handlePackages)
		r.GET("/packages/{name}", s.handlePackageDetail)
		r.GET("/outdated", s.handleOutdated)
		r.GET("/ext", s.handleExtManagers)
		r.GET("/ext/{manager}/packages", s.handleExtPackages)
		r.GET("/cache", s.handleCacheList)
		r.GET("/cache/status", s.handleCacheStatus)
		r.GET("/config", s.handleConfig)
		r.GET("/query", s.handleQuery)
		r.GET("/search", s.handleSearch)
	})

	// Cache mirror protocol: these paths and their semantics are a cross-machine
	// contract (see docs/web.md) and must stay exactly as they were.
	if s.deps.Manifest != nil {
		r.GET("/manifest.json", rux.WrapHTTPHandlerFunc(s.deps.Manifest))
	}
	if s.deps.Download != nil {
		r.GET("/download/*key", rux.WrapHTTPHandlerFunc(s.deps.Download))
	}
	if s.deps.File != nil {
		r.GET("/files/*path", rux.WrapHTTPHandlerFunc(s.deps.File))
	}

	s.registerAssets(r)
	r.NotFound(s.handleSPAFallback)
}
