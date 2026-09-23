package web

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gookit/goutil/x/assert"
	"github.com/gookit/rux/v2"
	app "github.com/inherelab/eget/internal/app"
	appcache "github.com/inherelab/eget/internal/app/cache"
	"github.com/inherelab/eget/internal/extpkg"
)

type fakeList struct {
	items       []app.ListItem
	outdated    []app.OutdatedItem
	failures    []app.OutdatedCheckFailure
	checked     int
	lenErr      error
	outdatedErr error
}

func (f fakeList) ListPackages() ([]app.ListItem, error) { return f.items, f.lenErr }

func (f fakeList) ListInstalledPackages() ([]app.ListItem, error) {
	installed := make([]app.ListItem, 0, len(f.items))
	for _, item := range f.items {
		if item.Installed {
			installed = append(installed, item)
		}
	}
	return installed, f.lenErr
}

func (f fakeList) ListOutdatedPackages() ([]app.OutdatedItem, []app.OutdatedCheckFailure, int, error) {
	return f.outdated, f.failures, f.checked, f.outdatedErr
}

func (f fakeList) FindPackage(name string) (*app.ListItem, error) {
	for _, item := range f.items {
		if item.Name == name {
			found := item
			return &found, nil
		}
	}
	return nil, io.EOF
}

type fakeShow struct {
	result app.ShowResult
	err    error
}

func (f fakeShow) ShowPackage(string) (app.ShowResult, error) { return f.result, f.err }

type fakeConfig struct {
	info    app.ConfigInfoResult
	content string
}

func (f fakeConfig) ConfigInfo() (app.ConfigInfoResult, error) { return f.info, nil }

func (f fakeConfig) ConfigExport(out io.Writer, _ bool) error {
	_, err := io.WriteString(out, f.content)
	return err
}

type fakeCache struct{}

func (fakeCache) ResolveCacheDir() (string, error) { return "/tmp/eget-cache", nil }

func (fakeCache) List(string, appcache.ListOptions) (appcache.ListResult, error) {
	return appcache.ListResult{Root: "all", TotalFiles: 0, Files: []appcache.ListFile{}}, nil
}

func (fakeCache) Status(string) (appcache.StatusResult, error) {
	return appcache.StatusResult{CacheDir: "/tmp/eget-cache", TotalFiles: 0, Kinds: map[string]appcache.KindSummary{}}, nil
}

type fakeExt struct{}

func (fakeExt) Names() []string { return []string{"npm", "cargo"} }

func (fakeExt) Manager(name string) (extpkg.Manager, bool) {
	if name == "npm" || name == "cargo" {
		return extpkg.Manager{Name: name, Bin: name}, true
	}
	return extpkg.Manager{}, false
}

func (fakeExt) Path(manager extpkg.Manager) (string, bool) {
	if manager.Name == "npm" {
		return "/usr/bin/npm", true
	}
	return "", false
}

func (fakeExt) List(context.Context, ...string) ([]extpkg.Package, []extpkg.Failure, error) {
	return []extpkg.Package{{Manager: "npm", Name: "typescript", Version: "5.9.2"}}, nil, nil
}

func (fakeExt) Outdated(context.Context, ...string) ([]extpkg.Package, []extpkg.Failure, error) {
	return []extpkg.Package{{Manager: "npm", Name: "typescript", Version: "5.8.0", Latest: "5.9.2"}}, nil, nil
}

func testServer(t *testing.T, opts Options) *Server {
	t.Helper()
	if opts.LogWriter == nil {
		opts.LogWriter = io.Discard
	}
	deps := Deps{
		List: fakeList{
			items: []app.ListItem{
				{Name: "fd", Repo: "sharkdp/fd", Installed: true, InstalledTag: "v10.4.2", Version: "v10.4.2"},
				{Name: "typescript", Repo: "npm:typescript", Manager: "npm", Installed: true, Version: "5.9.2"},
			},
			outdated: []app.OutdatedItem{{Name: "fd", Repo: "sharkdp/fd", InstalledTag: "v10.3.0", LatestTag: "v10.4.2"}},
			checked:  2,
		},
		Show:   fakeShow{result: app.ShowResult{Name: "fd", Repo: "sharkdp/fd", Installed: true, Configured: true, Tag: "v10.4.2"}},
		Config: fakeConfig{info: app.ConfigInfoResult{Path: "/home/me/.eget.toml", Exists: true}, content: "[global]\n"},
		Cache:  fakeCache{},
		Ext:    fakeExt{},
		Manifest: func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"schema":1,"files":[{"path":"pkg.zip"}]}`)
		},
		Download: func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, "payload")
		},
		File: func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, "file")
		},
	}
	server, err := NewServer(deps, opts)
	assert.NoErr(t, err)
	return server
}

func get(t *testing.T, server *Server, path string, mutate func(*http.Request)) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Host = "127.0.0.1:8787"
	if mutate != nil {
		mutate(req)
	}
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	return rec
}

func TestWebRequiresToken(t *testing.T) {
	server := testServer(t, Options{Token: "secret"})

	rec := get(t, server, "/api/overview", nil)
	assert.Eq(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Header().Get("WWW-Authenticate"), "Bearer")

	rec = get(t, server, "/api/overview", func(r *http.Request) {
		r.Header.Set("Authorization", "Bearer secret")
	})
	assert.Eq(t, http.StatusOK, rec.Code)

	rec = get(t, server, "/api/overview", func(r *http.Request) {
		r.Header.Set(authTokenHead, "secret")
	})
	assert.Eq(t, http.StatusOK, rec.Code)

	rec = get(t, server, "/api/overview", func(r *http.Request) {
		r.Header.Set(authTokenHead, "wrong")
	})
	assert.Eq(t, http.StatusUnauthorized, rec.Code)
}

func TestWebHealthzBypassesToken(t *testing.T) {
	server := testServer(t, Options{Token: "secret"})

	rec := get(t, server, "/healthz", nil)

	assert.Eq(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"name":"eget-web"`)
}

func TestWebQueryTokenPlantsCookie(t *testing.T) {
	server := testServer(t, Options{Token: "secret"})

	rec := get(t, server, "/api/overview?token=secret", nil)

	assert.Eq(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Set-Cookie"), authCookieName)
}

func TestWebRejectsUnknownHost(t *testing.T) {
	server := testServer(t, Options{Token: "secret"})

	rec := get(t, server, "/api/overview", func(r *http.Request) {
		r.Host = "evil.example.com"
		r.Header.Set("Authorization", "Bearer secret")
	})

	assert.Eq(t, http.StatusForbidden, rec.Code)
}

func TestWebRejectsNonLoopbackWithoutToken(t *testing.T) {
	_, err := NewServer(Deps{}, Options{Host: "0.0.0.0"})

	assert.Err(t, err)
	assert.Contains(t, err.Error(), "without a token")
}

func TestWebOverview(t *testing.T) {
	server := testServer(t, Options{Token: "secret", Version: "1.2.3"})

	rec := get(t, server, "/api/overview", func(r *http.Request) {
		r.Header.Set("Authorization", "Bearer secret")
	})

	assert.Eq(t, http.StatusOK, rec.Code)
	var body overviewResponse
	assert.NoErr(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Eq(t, "1.2.3", body.Version)
	assert.Eq(t, "/home/me/.eget.toml", body.ConfigPath)
	assert.Eq(t, 2, body.Packages)
	assert.Eq(t, 2, body.Installed)
	assert.Eq(t, 2, len(body.Ext))
	assert.True(t, body.Ext[0].Available)
	assert.Eq(t, "npm", body.Ext[0].Manager)
	assert.False(t, body.Ext[1].Available)
}

func TestWebPackagesFilters(t *testing.T) {
	server := testServer(t, Options{Token: "secret"})
	auth := func(r *http.Request) { r.Header.Set("Authorization", "Bearer secret") }

	rec := get(t, server, "/api/packages", auth)
	var all packagesResponse
	assert.NoErr(t, json.Unmarshal(rec.Body.Bytes(), &all))
	assert.Eq(t, 2, all.Total)

	rec = get(t, server, "/api/packages?scope=eget", auth)
	var egetOnly packagesResponse
	assert.NoErr(t, json.Unmarshal(rec.Body.Bytes(), &egetOnly))
	assert.Eq(t, 1, egetOnly.Total)
	assert.Eq(t, "eget", egetOnly.Items[0].Source)

	rec = get(t, server, "/api/packages?scope=ext", auth)
	var extOnly packagesResponse
	assert.NoErr(t, json.Unmarshal(rec.Body.Bytes(), &extOnly))
	assert.Eq(t, 1, extOnly.Total)
	assert.Eq(t, "npm", extOnly.Items[0].Source)
	assert.Eq(t, "npm", extOnly.Items[0].Manager)

	rec = get(t, server, "/api/packages?q=type", auth)
	var searched packagesResponse
	assert.NoErr(t, json.Unmarshal(rec.Body.Bytes(), &searched))
	assert.Eq(t, 1, searched.Total)
	assert.Eq(t, "typescript", searched.Items[0].Name)

	rec = get(t, server, "/api/packages?scope=bogus", auth)
	assert.Eq(t, http.StatusBadRequest, rec.Code)
}

func TestWebPackageDetail(t *testing.T) {
	server := testServer(t, Options{Token: "secret"})

	rec := get(t, server, "/api/packages/fd", func(r *http.Request) {
		r.Header.Set("Authorization", "Bearer secret")
	})

	assert.Eq(t, http.StatusOK, rec.Code)
	var detail packageDetail
	assert.NoErr(t, json.Unmarshal(rec.Body.Bytes(), &detail))
	assert.Eq(t, "fd", detail.Name)
	assert.Eq(t, "v10.4.2", detail.InstalledTag)
	assert.True(t, detail.Configured)
}

func TestWebOutdated(t *testing.T) {
	server := testServer(t, Options{Token: "secret"})

	rec := get(t, server, "/api/outdated", func(r *http.Request) {
		r.Header.Set("Authorization", "Bearer secret")
	})

	assert.Eq(t, http.StatusOK, rec.Code)
	var body outdatedResponse
	assert.NoErr(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Eq(t, 2, body.Checked)
	assert.Eq(t, 1, len(body.Items))
	assert.Eq(t, "v10.4.2", body.Items[0].LatestTag)
}

func TestWebConfigView(t *testing.T) {
	server := testServer(t, Options{Token: "secret"})

	rec := get(t, server, "/api/config", func(r *http.Request) {
		r.Header.Set("Authorization", "Bearer secret")
	})

	var view configView
	assert.NoErr(t, json.Unmarshal(rec.Body.Bytes(), &view))
	assert.Eq(t, "/home/me/.eget.toml", view.Path)
	assert.True(t, view.Exists)
	assert.Contains(t, view.Content, "[global]")
}

func TestWebServesCacheMirrorPaths(t *testing.T) {
	server := testServer(t, Options{Token: "secret"})
	auth := func(r *http.Request) { r.Header.Set("Authorization", "Bearer secret") }

	manifest := get(t, server, "/manifest.json", auth)
	assert.Eq(t, http.StatusOK, manifest.Code)
	assert.Contains(t, manifest.Body.String(), `"schema":1`)

	download := get(t, server, "/download/path-md5:abc", auth)
	assert.Eq(t, http.StatusOK, download.Code)
	assert.Eq(t, "payload", download.Body.String())

	file := get(t, server, "/files/pkg-cache/tool.zip", auth)
	assert.Eq(t, http.StatusOK, file.Code)

	// The mirror endpoints are protected by the same token middleware.
	assert.Eq(t, http.StatusUnauthorized, get(t, server, "/manifest.json", nil).Code)
}

func TestWebSPAFallback(t *testing.T) {
	server := testServer(t, Options{Token: "secret"})

	rec := get(t, server, "/packages/fd", func(r *http.Request) {
		r.Header.Set("Authorization", "Bearer secret")
	})

	assert.Eq(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/html")
	// Without a frontend build the placeholder explains how to create one.
	assert.Contains(t, rec.Body.String(), "web-build")
}

func TestWebSecurityHeaders(t *testing.T) {
	server := testServer(t, Options{})

	rec := get(t, server, "/healthz", nil)

	assert.Eq(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	assert.Eq(t, "DENY", rec.Header().Get("X-Frame-Options"))
	assert.Contains(t, rec.Header().Get("Content-Security-Policy"), "default-src 'self'")
}

func TestWebLogRedactsQueryString(t *testing.T) {
	var log strings.Builder
	server := testServer(t, Options{Token: "secret", LogWriter: &log, JSONLog: true})

	rec := get(t, server, "/api/overview?token=secret", nil)
	assert.Eq(t, http.StatusOK, rec.Code)

	line := strings.TrimSpace(log.String())
	assert.Contains(t, line, `"/api/overview"`)
	assert.False(t, strings.Contains(line, "secret"))
}

func TestWebServeResolvesRandomPort(t *testing.T) {
	server := testServer(t, Options{Port: 0})

	ctx, cancel := context.WithCancel(context.Background())
	ready := make(chan string, 1)
	done := make(chan error, 1)
	go func() {
		done <- server.Serve(ctx, func(addr string) { ready <- addr })
	}()

	var addr string
	select {
	case addr = <-ready:
	case <-time.After(5 * time.Second):
		t.Fatal("server did not become ready")
	}
	assert.False(t, strings.HasSuffix(addr, ":0"))

	cancel()
	select {
	case err := <-done:
		assert.NoErr(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("server did not shut down")
	}
}

// rux skips the global middleware chain when no route matches, so a CSRF test
// needs a real write route. M2 replaces this stand-in with the real endpoints.
func echoRoute(t *testing.T, server *Server) {
	t.Helper()
	server.Router().POST("/api/echo", func(c *rux.Context) {
		c.JSON(http.StatusOK, map[string]any{"ok": true})
	})
}

func TestWebRejectsCrossSiteWrite(t *testing.T) {
	server := testServer(t, Options{Token: "secret"})
	echoRoute(t, server)

	req := httptest.NewRequest(http.MethodPost, "/api/echo", strings.NewReader("{}"))
	req.Host = "127.0.0.1:8787"
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("Origin", "http://evil.example.com")
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	assert.Eq(t, http.StatusForbidden, rec.Code)
}

func TestWebAllowsWriteWithCustomHeader(t *testing.T) {
	server := testServer(t, Options{Token: "secret"})
	echoRoute(t, server)

	req := httptest.NewRequest(http.MethodPost, "/api/echo", strings.NewReader("{}"))
	req.Host = "127.0.0.1:8787"
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set(authTokenHead, "secret")
	req.Header.Set("Origin", "http://evil.example.com")
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	assert.Eq(t, http.StatusOK, rec.Code)
}

func TestWebAllowsSameOriginWrite(t *testing.T) {
	server := testServer(t, Options{Token: "secret"})
	echoRoute(t, server)

	req := httptest.NewRequest(http.MethodPost, "/api/echo", strings.NewReader("{}"))
	req.Host = "127.0.0.1:8787"
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("Origin", "http://127.0.0.1:8787")
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	assert.Eq(t, http.StatusOK, rec.Code)
}
