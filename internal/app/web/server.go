package web

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gookit/rux/v2"
	"github.com/gookit/rux/v2/server"
	app "github.com/inherelab/eget/internal/app"
	appcache "github.com/inherelab/eget/internal/app/cache"
	"github.com/inherelab/eget/internal/extpkg"
)

// DefaultPort is the eget web default listen port.
const DefaultPort = 8787

// Options configures the eget web console.
type Options struct {
	Host           string
	Port           int
	Token          string
	ReadOnly       bool
	AllowMutations bool
	Version        string
	CacheRoot      string
	NoCacheIndex   bool
	// AllowHosts adds names to the Host header allow list.
	AllowHosts []string
	JSONLog    bool
	LogWriter  io.Writer
	// TaskStore is the tasks.json path (empty disables persistence).
	TaskStore string
	// TaskRunners maps a task kind to its implementation. Write endpoints are
	// unavailable when it is empty.
	TaskRunners map[string]TaskRunner
}

// ListProvider is the subset of app.ListService the console needs.
type ListProvider interface {
	ListPackages() ([]app.ListItem, error)
	ListInstalledPackages() ([]app.ListItem, error)
	ListOutdatedPackages() ([]app.OutdatedItem, []app.OutdatedCheckFailure, int, error)
	FindPackage(name string) (*app.ListItem, error)
}

// ShowProvider is the subset of app.ShowService the console needs.
type ShowProvider interface {
	ShowPackage(target string) (app.ShowResult, error)
}

// QueryProvider is the subset of app.QueryService the console needs.
type QueryProvider interface {
	Query(opts app.QueryOptions) (app.QueryResult, error)
}

// SearchProvider is the subset of app.SearchService the console needs.
type SearchProvider interface {
	Search(opts app.SearchOptions) (app.SearchResult, error)
}

// ConfigProvider is the subset of app.ConfigService the console needs.
type ConfigProvider interface {
	ConfigInfo() (app.ConfigInfoResult, error)
	ConfigExport(out io.Writer, withGlobal bool) error
	ConfigGet(key string) (any, error)
	ConfigSet(key, value string) error
}

// CacheProvider is the subset of appcache.Service the console needs.
type CacheProvider interface {
	ResolveCacheDir() (string, error)
	List(cacheDir string, opts appcache.ListOptions) (appcache.ListResult, error)
	Status(cacheDir string) (appcache.StatusResult, error)
}

// ExtProvider is the subset of extpkg.Service the console needs.
type ExtProvider interface {
	Names() []string
	Manager(name string) (extpkg.Manager, bool)
	Path(manager extpkg.Manager) (string, bool)
	List(ctx context.Context, only ...string) ([]extpkg.Package, []extpkg.Failure, error)
	Outdated(ctx context.Context, only ...string) ([]extpkg.Package, []extpkg.Failure, error)
}

// Deps are the application services the console calls.
type Deps struct {
	List   ListProvider
	Show   ShowProvider
	Query  QueryProvider
	Search SearchProvider
	Config ConfigProvider
	Cache  CacheProvider
	Ext    ExtProvider

	// Machine handlers serve the cache mirror protocol on its original paths
	// (/manifest.json, /download/*, /files/*).
	Manifest http.HandlerFunc
	Download http.HandlerFunc
	File     http.HandlerFunc

	// AssetCandidates lists the assets matching a target without downloading,
	// so the install form can offer an explicit choice.
	AssetCandidates func(ctx context.Context, target string) ([]string, error)
}

// Server hosts the eget web console over HTTP.
type Server struct {
	opts    Options
	deps    Deps
	router  *rux.Router
	inner   *server.Server
	tasks   *Engine
	limiter *authLimiter
}

// NewServer builds the router, middleware chain and cache mirror routes.
func NewServer(deps Deps, opts Options) (*Server, error) {
	opts = opts.withDefaults()
	if err := opts.validate(); err != nil {
		return nil, err
	}

	// rux 2.1 runs the global middleware chain for 404/405 responses, so the
	// console uses the framework's own NotFound/NotAllowed handlers instead of a
	// catch-all route, and the server package owns timeouts, signal handling,
	// graceful shutdown and the liveness endpoints.
	inner := server.New(false)
	inner.Addr = net.JoinHostPort(opts.Host, strconv.Itoa(opts.Port))
	// SSE streams and large mirror downloads must not be cut mid-response.
	inner.WriteTimeout = 0
	rux.WithMethodNotAllowed(true)(inner.Router)

	router := inner.Router
	s := &Server{
		opts:    opts,
		deps:    deps,
		router:  router,
		inner:   inner,
		limiter: newAuthLimiter(authFailureLimit, authFailureWindow),
	}
	if len(opts.TaskRunners) > 0 {
		s.tasks = NewEngine(opts.TaskStore, opts.TaskRunners)
	}
	// Order matters: rux requires Use before any route registration, and
	// MountHealthChecks registers /healthz + /readyz.
	router.Use(
		s.recoverMiddleware,
		s.logMiddleware,
		s.securityHeadersMiddleware,
		s.hostMiddleware,
		s.authMiddleware,
		s.csrfMiddleware,
	)
	inner.MountHealthChecks()
	s.registerRoutes(router)
	return s, nil
}

// Handler exposes the router for tests and httptest servers.
func (s *Server) Handler() http.Handler {
	return s.router
}

// Router exposes the underlying router, for tests that register extra routes
// before the first request freezes the routing table.
func (s *Server) Router() *rux.Router {
	return s.router
}

// Serve listens on the configured address and blocks until a stop signal or a
// canceled ctx, then drains in-flight requests. onReady receives the address
// actually bound, which resolves a random port when Port is 0.
func (s *Server) Serve(ctx context.Context, onReady func(addr string)) error {
	listener, err := net.Listen("tcp", s.inner.Addr)
	if err != nil {
		return err
	}
	s.inner.SetListener(listener)

	// Cancel queued and running tasks before the drain, so a shutdown cannot
	// leave a task half applied.
	s.inner.PreShutdown = append(s.inner.PreShutdown, func(context.Context) error {
		if s.tasks != nil {
			s.tasks.Close()
		}
		return nil
	})

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = s.inner.Shutdown(shutdownCtx)
	}()

	if onReady != nil {
		// The bound listener knows the real address immediately; s.inner.Addr
		// still holds the requested one (":0") until the server publishes it.
		onReady(listener.Addr().String())
	}
	// A graceful shutdown surfaces as http.ErrServerClosed; the CLI should treat
	// that as a normal exit.
	serveErr := s.inner.ServeListener(listener)
	if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
		return serveErr
	}
	return nil
}

// MutationsEnabled reports whether write endpoints may run.
func (s *Server) MutationsEnabled() bool {
	return !s.opts.ReadOnly && s.opts.AllowMutations
}

func (o Options) withDefaults() Options {
	if strings.TrimSpace(o.Host) == "" {
		o.Host = "127.0.0.1"
	}
	// Port keeps 0 as "let the OS pick a free port"; the CLI flag supplies the
	// DefaultPort value.
	if strings.TrimSpace(o.CacheRoot) == "" {
		o.CacheRoot = "all"
	}
	if o.LogWriter == nil {
		o.LogWriter = os.Stderr
	}
	return o
}

func (o Options) validate() error {
	if o.Port < 0 || o.Port > 65535 {
		return fmt.Errorf("invalid port %d", o.Port)
	}
	// A token is mandatory as soon as the listener is reachable from outside
	// this machine: fail closed instead of serving an open console.
	if !IsLoopbackHost(o.Host) && o.NoAuth() {
		return fmt.Errorf("refusing to listen on non-loopback host %q without a token", o.Host)
	}
	return nil
}

// NoAuth reports whether the console runs without token checks.
func (o Options) NoAuth() bool {
	return strings.TrimSpace(o.Token) == ""
}

// IsLoopbackHost reports whether host only accepts local connections.
func IsLoopbackHost(host string) bool {
	host = strings.TrimSpace(strings.ToLower(host))
	if host == "" || host == "localhost" {
		return true
	}
	if ip := net.ParseIP(strings.Trim(host, "[]")); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

// allowedHosts lists Host header values accepted by the host middleware.
func (o Options) allowedHosts() map[string]bool {
	hosts := map[string]bool{
		"localhost": true,
		"127.0.0.1": true,
		"::1":       true,
		"[::1]":     true,
		"0.0.0.0":   true,
	}
	hosts[strings.ToLower(o.Host)] = true
	return hosts
}
