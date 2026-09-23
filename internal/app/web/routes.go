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
		r.POST("/config/validate", s.handleConfigValidate)
		r.PUT("/config", s.handleConfigUpdate)
		r.GET("/query", s.handleQuery)
		r.GET("/search", s.handleSearch)

		// Write endpoints queue a task; the global CSRF middleware already
		// guards every non-GET request.
		r.GET("/install/candidates", s.handleInstallCandidates)
		r.POST("/install", s.handleSubmitInstall)
		r.POST("/update", s.handleSubmitUpdate)
		r.POST("/uninstall", s.handleSubmitUninstall)
		r.POST("/ext/upgrade", s.handleSubmitExtUpgrade)
		r.POST("/cache/clean", s.handleSubmitCacheClean)
		r.POST("/sdk/install", s.handleSubmitSDKInstall)
		r.POST("/sdk/download", s.handleSubmitSDKDownload)

		r.GET("/tasks", s.handleTasks)
		r.GET("/tasks/{id}", s.handleTask)
		r.GET("/tasks/{id}/events", s.handleTaskEvents)
		r.POST("/tasks/{id}/cancel", s.handleTaskCancel)
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
	// Deliberately not r.NotFound: rux's NotFound/NotAllowed handlers bypass the
	// global middleware chain, which would skip auth, security headers and
	// logging on exactly the requests an attacker controls. A catch-all route
	// runs through the normal chain instead.
	r.Any("/*path", s.handleCatchAll)
}
