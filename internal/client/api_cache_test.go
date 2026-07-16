package client

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gookit/goutil/x/assert"
	"github.com/inherelab/eget/internal/cachemirror"
)

func TestResolvedAPICachePathSupportsMetadataMirror(t *testing.T) {
	parsed, err := url.Parse("https://api.github.com/repos/owner/tool/releases/latest")
	assert.NoErr(t, err)
	cacheDir := filepath.Join(t.TempDir(), "api-cache")

	for _, tc := range []struct {
		name string
		opts Options
		want bool
	}{
		{name: "api cache", opts: Options{APICacheEnabled: true, APICacheDir: cacheDir}, want: true},
		{name: "mirror only", opts: Options{APICacheDir: cacheDir, CacheMirror: cachemirror.Options{Enable: true, URL: "http://mirror"}}, want: true},
		{name: "disabled", opts: Options{APICacheDir: cacheDir}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, got := resolvedAPICachePath(tc.opts, parsed.String(), parsed)
			assert.Eq(t, tc.want, got)
		})
	}

	download, err := url.Parse("https://github.com/owner/tool/releases/download/v1.2.3/tool.zip")
	assert.NoErr(t, err)
	_, got := resolvedAPICachePath(Options{
		APICacheDir: cacheDir,
		CacheMirror: cachemirror.Options{Enable: true, URL: "http://mirror"},
	}, download.String(), download)
	assert.False(t, got)
}

func TestGetWithOptionsUsesMetadataMirrorBeforeOrigin(t *testing.T) {
	apiURL := "https://api.github.com/repos/owner/tool/releases/latest"
	body := `{"tag_name":"v1.2.3"}`
	apiCacheDir := filepath.Join(t.TempDir(), "api-cache")
	cachePath := APICacheFilePath(apiCacheDir, apiURL)
	rel, err := cachemirror.RelPath(filepath.Dir(apiCacheDir), cachePath)
	assert.NoErr(t, err)
	wantPath := "/download/" + cachemirror.KeyForRelPath(rel)

	mirrorCalls := 0
	mirror := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mirrorCalls++
		assert.Eq(t, wantPath, r.URL.Path)
		_, _ = io.WriteString(w, body)
	}))
	defer mirror.Close()

	originCalls := 0
	restoreHTTPDo := SetHTTPDoForTest(func(client *http.Client, req *http.Request) (*http.Response, error) {
		originCalls++
		return jsonResponse(http.StatusOK, "200 OK", `{"tag_name":"origin"}`), nil
	})
	defer restoreHTTPDo()

	resp, err := GetWithOptions(apiURL, Options{
		APICacheDir: apiCacheDir,
		CacheMirror: cachemirror.Options{Enable: true, URL: mirror.URL, Fallback: false},
	})
	assert.NoErr(t, err)
	defer resp.Body.Close()
	got, err := io.ReadAll(resp.Body)
	assert.NoErr(t, err)
	assert.Eq(t, body, string(got))
	assert.Eq(t, 1, mirrorCalls)
	assert.Eq(t, 0, originCalls)

	cached, err := os.ReadFile(cachePath)
	assert.NoErr(t, err)
	assert.Eq(t, body, string(cached))
	info, err := os.Stat(cachePath)
	assert.NoErr(t, err)
	assert.True(t, time.Since(info.ModTime()) < time.Minute)

	resp, err = GetWithOptions(apiURL, Options{
		APICacheDir: apiCacheDir,
		CacheMirror: cachemirror.Options{Enable: true, URL: mirror.URL, Fallback: false},
	})
	assert.NoErr(t, err)
	_ = resp.Body.Close()
	assert.Eq(t, 1, mirrorCalls)
	assert.Eq(t, 0, originCalls)
}

func TestGetWithOptionsMetadataMirrorFallback(t *testing.T) {
	for _, tc := range []struct {
		name       string
		status     int
		fallback   bool
		wantErr    string
		wantOrigin int
	}{
		{name: "miss falls back", status: http.StatusNotFound, fallback: true, wantOrigin: 1},
		{name: "miss is strict", status: http.StatusNotFound, wantErr: "cache mirror metadata miss", wantOrigin: 0},
		{name: "error falls back", status: http.StatusInternalServerError, fallback: true, wantOrigin: 1},
		{name: "error is strict", status: http.StatusInternalServerError, wantErr: "cache mirror metadata", wantOrigin: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mirror := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
			}))
			defer mirror.Close()

			originCalls := 0
			restoreHTTPDo := SetHTTPDoForTest(func(client *http.Client, req *http.Request) (*http.Response, error) {
				originCalls++
				return jsonResponse(http.StatusOK, "200 OK", `{"tag_name":"origin"}`), nil
			})
			defer restoreHTTPDo()

			apiCacheDir := filepath.Join(t.TempDir(), "api-cache")
			resp, err := GetWithOptions("https://api.github.com/repos/owner/tool/releases/latest", Options{
				APICacheDir: apiCacheDir,
				CacheMirror: cachemirror.Options{Enable: true, URL: mirror.URL, Fallback: tc.fallback},
			})
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
				}
				if tc.status == http.StatusNotFound {
					assert.Contains(t, err.Error(), "api-cache/")
					assert.Contains(t, err.Error(), "fallback disabled")
				}
			} else {
				assert.NoErr(t, err)
				if resp != nil {
					_ = resp.Body.Close()
				}
				cached, readErr := os.ReadFile(APICacheFilePath(apiCacheDir, "https://api.github.com/repos/owner/tool/releases/latest"))
				assert.NoErr(t, readErr)
				assert.Eq(t, `{"tag_name":"origin"}`, string(cached))
			}
			assert.Eq(t, tc.wantOrigin, originCalls)
		})
	}
}

func TestGetWithOptionsUsesAPICacheForKnownProviderMetadataRequests(t *testing.T) {
	for _, tt := range []struct {
		name   string
		apiURL string
		body   string
	}{
		{
			name:   "gitlab release api",
			apiURL: "https://gitlab.com/api/v4/projects/fdroid%2Ffdroidserver/releases/permalink/latest",
			body:   `{"tag_name":"v2.3.4"}`,
		},
		{
			name:   "gitea release api",
			apiURL: "https://codeberg.org/api/v1/repos/forgejo/forgejo/releases/latest",
			body:   `{"tag_name":"v9.0.0"}`,
		},
		{
			name:   "sourceforge files listing",
			apiURL: "https://sourceforge.net/projects/winmerge/files/stable/",
			body:   `<html>cached sourceforge listing</html>`,
		},
		{
			name:   "sourceforge root files listing",
			apiURL: "https://sourceforge.net/projects/winmerge/files/",
			body:   `<html>cached sourceforge root listing</html>`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cacheDir := t.TempDir()
			cachePath := APICacheFilePath(cacheDir, tt.apiURL)
			if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
				t.Fatalf("mkdir cache dir: %v", err)
			}
			if err := os.WriteFile(cachePath, []byte(tt.body), 0o644); err != nil {
				t.Fatalf("write cache file: %v", err)
			}

			calls := 0
			restoreHTTPDo := SetHTTPDoForTest(func(client *http.Client, req *http.Request) (*http.Response, error) {
				calls++
				return jsonResponse(http.StatusOK, "200 OK", `network`), nil
			})
			defer restoreHTTPDo()

			resp, err := GetWithOptions(tt.apiURL, Options{
				APICacheEnabled: true,
				APICacheDir:     cacheDir,
				APICacheTime:    300,
			})
			if err != nil {
				t.Fatalf("GetWithOptions(): %v", err)
			}
			defer resp.Body.Close()

			got, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			assert.Eq(t, tt.body, string(got))
			assert.Eq(t, 0, calls)
		})
	}
}

func TestGetWithOptionsDoesNotUseAPICacheForDownloads(t *testing.T) {
	cacheDir := t.TempDir()
	downloadURL := "https://downloads.sourceforge.net/project/winmerge/stable/WinMerge.zip"
	mirrorCalls := 0
	mirror := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mirrorCalls++
		http.NotFound(w, r)
	}))
	defer mirror.Close()
	cachePath := APICacheFilePath(cacheDir, downloadURL)
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		t.Fatalf("mkdir cache dir: %v", err)
	}
	if err := os.WriteFile(cachePath, []byte("cached download"), 0o644); err != nil {
		t.Fatalf("write cache file: %v", err)
	}

	calls := 0
	restoreHTTPDo := SetHTTPDoForTest(func(client *http.Client, req *http.Request) (*http.Response, error) {
		calls++
		return jsonResponse(http.StatusOK, "200 OK", `network download`), nil
	})
	defer restoreHTTPDo()

	resp, err := GetWithOptions(downloadURL, Options{
		APICacheEnabled: true,
		APICacheDir:     cacheDir,
		APICacheTime:    300,
		CacheMirror:     cachemirror.Options{Enable: true, URL: mirror.URL, Fallback: false},
	})
	if err != nil {
		t.Fatalf("GetWithOptions(): %v", err)
	}
	defer resp.Body.Close()

	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	assert.Eq(t, "network download", string(got))
	assert.Eq(t, 1, calls)
	assert.Eq(t, 0, mirrorCalls)
}
