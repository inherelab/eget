package install

import (
	"bytes"
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

func TestDownloadPrintsProxyNoticeForRemoteRequest(t *testing.T) {
	var notice bytes.Buffer
	origNoticeWriter := proxyNoticeWriter
	origHTTPDo := httpDo
	defer func() {
		proxyNoticeWriter = origNoticeWriter
		httpDo = origHTTPDo
	}()
	proxyNoticeWriter = &notice
	var requested string
	httpDo = func(client *http.Client, req *http.Request) (*http.Response, error) {
		requested = req.URL.String()
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("network-data")),
		}, nil
	}

	err := Download("https://example.com/tool.tar.gz", io.Discard, func(size int64) io.Writer {
		return io.Discard
	}, Options{ProxyURL: "http://127.0.0.1:7890", ProxyExclude: []string{"github.com"}})
	if err != nil {
		t.Fatalf("Download(): %v", err)
	}
	assert.Eq(t, "https://example.com/tool.tar.gz", requested)

	if got := notice.String(); !strings.Contains(got, "http_proxy for download request") {
		t.Fatalf("expected download proxy notice, got %q", got)
	}
}

func TestDownloadSkipsProxyNoticeForLocalFile(t *testing.T) {
	tmpDir := t.TempDir()
	localFile := filepath.Join(tmpDir, "tool.tar.gz")
	if err := os.WriteFile(localFile, []byte("ok"), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	var notice bytes.Buffer
	origNoticeWriter := proxyNoticeWriter
	defer func() { proxyNoticeWriter = origNoticeWriter }()
	proxyNoticeWriter = &notice

	err := Download(localFile, io.Discard, func(size int64) io.Writer {
		return io.Discard
	}, Options{ProxyURL: "http://127.0.0.1:7890"})
	if err != nil {
		t.Fatalf("Download(local): %v", err)
	}

	if got := notice.String(); got != "" {
		t.Fatalf("expected no proxy notice for local file, got %q", got)
	}
}

func TestNewHTTPGetterUsesProxyURL(t *testing.T) {
	proxyFunc, err := proxyFuncFor("http://127.0.0.1:7890", nil)
	if err != nil {
		t.Fatalf("proxyFuncFor: %v", err)
	}
	req, err := http.NewRequest(http.MethodGet, "https://example.com/tool.tar.gz", nil)
	if err == nil {
		proxyURL, err := proxyFunc(req)
		if err != nil {
			t.Fatalf("proxy func: %v", err)
		}
		if proxyURL == nil {
			t.Fatal("expected proxy url to be returned")
		}
		if proxyURL.String() != "http://127.0.0.1:7890" {
			t.Fatalf("expected proxy url http://127.0.0.1:7890, got %q", proxyURL.String())
		}
		return
	}
	t.Fatalf("new request: %v", err)
}

func TestProxyFuncForRejectsInvalidProxyURL(t *testing.T) {
	_, err := proxyFuncFor("://bad-proxy", nil)
	if err == nil {
		t.Fatal("expected invalid proxy url error")
	}
	if !strings.Contains(err.Error(), "invalid proxy_url") {
		t.Fatalf("expected invalid proxy_url error, got %v", err)
	}
}

func TestProxyFuncForFallsBackToEnvironment(t *testing.T) {
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("http_proxy", "")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:7891")
	t.Setenv("https_proxy", "http://127.0.0.1:7891")
	t.Setenv("NO_PROXY", "")
	t.Setenv("no_proxy", "")
	t.Setenv("REQUEST_METHOD", "")
	proxyFunc, err := proxyFuncFor("", nil)
	if err != nil {
		t.Fatalf("proxyFuncFor env fallback: %v", err)
	}
	req := &http.Request{URL: &url.URL{Scheme: "https", Host: "example.com"}}
	proxyURL, err := proxyFunc(req)
	if err != nil {
		t.Fatalf("proxy func env fallback: %v", err)
	}
	if proxyURL == nil {
		t.Fatal("expected environment proxy url to be returned")
	}
	if proxyURL.String() != "http://127.0.0.1:7891" {
		t.Fatalf("expected env proxy url http://127.0.0.1:7891, got %q", proxyURL.String())
	}
}

func TestProxyFuncForSkipsExcludedHost(t *testing.T) {
	proxyFunc, err := proxyFuncFor("http://127.0.0.1:7890", []string{"github.com"})
	assert.NoErr(t, err)

	cases := []struct {
		name      string
		rawURL    string
		wantProxy bool
	}{
		{name: "exact host", rawURL: "https://github.com/owner/repo"},
		{name: "subdomain", rawURL: "https://api.github.com/repos/owner/repo"},
		{name: "non excluded", rawURL: "https://example.com/file.zip", wantProxy: true},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got, err := proxyFunc(httptest.NewRequest(http.MethodGet, tt.rawURL, nil))
			assert.NoErr(t, err)
			if !tt.wantProxy {
				assert.Eq(t, (*url.URL)(nil), got)
				return
			}
			assert.Eq(t, "http://127.0.0.1:7890", got.String())
		})
	}
}

func TestClientOptionsCopiesProxyExclude(t *testing.T) {
	opts := ClientOptions(Options{ProxyExclude: []string{"github.com"}})

	assert.Eq(t, []string{"github.com"}, opts.ProxyExclude)
	opts.ProxyExclude[0] = "example.com"
	assert.Eq(t, []string{"github.com"}, ClientOptions(Options{ProxyExclude: []string{"github.com"}}).ProxyExclude)
}

func TestClientOptionsPropagatesRetries(t *testing.T) {
	opts := ClientOptions(Options{Retries: 3})

	assert.Eq(t, 3, opts.Retries)
}

func TestClientOptionsCopiesCacheMirror(t *testing.T) {
	want := cachemirror.Options{
		Enable: true, URL: "http://mirror.local:8686", Timeout: 4 * time.Second, Fallback: false,
	}
	got := ClientOptions(Options{CacheMirror: want})
	assert.Eq(t, want, got.CacheMirror)
}

func TestGetWithOptionsPrintsProxyNoticeForGitHubAPI(t *testing.T) {
	var notice bytes.Buffer
	origNoticeWriter := proxyNoticeWriter
	origHTTPDo := httpDo
	defer func() {
		proxyNoticeWriter = origNoticeWriter
		httpDo = origHTTPDo
	}()
	proxyNoticeWriter = &notice
	httpDo = func(client *http.Client, req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{}`)),
		}, nil
	}

	resp, err := GetWithOptions("https://api.github.com/repos/gookit/gitw/releases/latest", Options{ProxyURL: "http://127.0.0.1:7890"})
	if err != nil {
		t.Fatalf("GetWithOptions(): %v", err)
	}
	_ = resp.Body.Close()

	if got := notice.String(); !strings.Contains(got, "http_proxy for GitHub API request") {
		t.Fatalf("expected GitHub API proxy notice, got %q", got)
	}
}

func TestGetWithOptionsSkipsProxyNoticeWithoutProxyURL(t *testing.T) {
	var notice bytes.Buffer
	origNoticeWriter := proxyNoticeWriter
	origHTTPDo := httpDo
	defer func() {
		proxyNoticeWriter = origNoticeWriter
		httpDo = origHTTPDo
	}()
	proxyNoticeWriter = &notice
	httpDo = func(client *http.Client, req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{}`)),
		}, nil
	}

	resp, err := GetWithOptions("https://api.github.com/repos/gookit/gitw/releases/latest", Options{})
	if err != nil {
		t.Fatalf("GetWithOptions(): %v", err)
	}
	_ = resp.Body.Close()

	if got := notice.String(); got != "" {
		t.Fatalf("expected no proxy notice without proxy_url, got %q", got)
	}
}

func TestGetWithOptionsPrintsVerboseRequestAndResponse(t *testing.T) {
	var verbose bytes.Buffer
	origVerboseEnabled := verboseEnabled
	origVerboseWriter := verboseWriter
	origHTTPDo := httpDo
	defer func() {
		verboseEnabled = origVerboseEnabled
		verboseWriter = origVerboseWriter
		httpDo = origHTTPDo
	}()
	SetVerbose(true, &verbose)
	httpDo = func(client *http.Client, req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader(`{}`)),
		}, nil
	}

	resp, err := GetWithOptions("https://api.github.com/repos/gookit/gitw/releases/latest", Options{})
	if err != nil {
		t.Fatalf("GetWithOptions(): %v", err)
	}
	_ = resp.Body.Close()

	got := verbose.String()
	if !strings.Contains(got, "request: GET https://api.github.com/repos/gookit/gitw/releases/latest") {
		t.Fatalf("expected verbose request log, got %q", got)
	}
	if !strings.Contains(got, "response: https://api.github.com/repos/gookit/gitw/releases/latest 200 OK") {
		t.Fatalf("expected verbose response log, got %q", got)
	}
}

func TestGetWithOptionsUsesAPICacheWhenAvailable(t *testing.T) {
	cacheDir := t.TempDir()
	apiURL := "https://api.github.com/repos/gookit/gitw/releases/latest"
	cachePath := APICacheFilePath(cacheDir, apiURL)
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		t.Fatalf("mkdir cache dir: %v", err)
	}
	if err := os.WriteFile(cachePath, []byte(`{"tag_name":"v0.3.6"}`), 0o644); err != nil {
		t.Fatalf("write cache file: %v", err)
	}

	calls := 0
	origHTTPDo := httpDo
	origNoticeWriter := apiCacheNoticeWriter
	defer func() { httpDo = origHTTPDo }()
	defer func() { apiCacheNoticeWriter = origNoticeWriter }()
	httpDo = func(client *http.Client, req *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader(`{"tag_name":"network"}`)),
		}, nil
	}
	var notice bytes.Buffer
	apiCacheNoticeWriter = &notice

	resp, err := GetWithOptions(apiURL, Options{
		APICacheEnabled: true,
		APICacheDir:     cacheDir,
		APICacheTime:    300,
	})
	if err != nil {
		t.Fatalf("GetWithOptions(): %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(body) != `{"tag_name":"v0.3.6"}` {
		t.Fatalf("expected cached response body, got %q", string(body))
	}
	if calls != 0 {
		t.Fatalf("expected no network calls, got %d", calls)
	}
	if got := notice.String(); !strings.Contains(got, "api_cache file") {
		t.Fatalf("expected api cache notice, got %q", got)
	}
}

func TestGetWithOptionsWritesAPICacheAfterNetworkRequest(t *testing.T) {
	cacheDir := t.TempDir()
	apiURL := "https://api.github.com/repos/gookit/gitw/releases/latest"

	origHTTPDo := httpDo
	defer func() { httpDo = origHTTPDo }()
	httpDo = func(client *http.Client, req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader(`{"tag_name":"v0.3.6"}`)),
		}, nil
	}

	resp, err := GetWithOptions(apiURL, Options{
		APICacheEnabled: true,
		APICacheDir:     cacheDir,
		APICacheTime:    300,
	})
	if err != nil {
		t.Fatalf("GetWithOptions(): %v", err)
	}
	defer resp.Body.Close()

	_, err = io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	cachePath := APICacheFilePath(cacheDir, apiURL)
	saved, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("read cache file: %v", err)
	}
	if string(saved) != `{"tag_name":"v0.3.6"}` {
		t.Fatalf("expected cached response body, got %q", string(saved))
	}
}

func TestGetWithOptionsUsesGhproxyForDownloads(t *testing.T) {
	origHTTPDo := httpDo
	defer func() { httpDo = origHTTPDo }()

	var requested string
	httpDo = func(client *http.Client, req *http.Request) (*http.Response, error) {
		requested = req.URL.String()
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader("ok")),
		}, nil
	}

	resp, err := GetWithOptions("https://github.com/gookit/gitw/releases/download/v0.3.6/chlog-windows-amd64.exe", Options{
		GhproxyEnabled: true,
		GhproxyHostURL: "https://gh.felicity.ac.cn",
	})
	if err != nil {
		t.Fatalf("GetWithOptions(): %v", err)
	}
	_ = resp.Body.Close()

	want := "https://gh.felicity.ac.cn/https://github.com/gookit/gitw/releases/download/v0.3.6/chlog-windows-amd64.exe"
	if requested != want {
		t.Fatalf("expected ghproxy rewritten url %q, got %q", want, requested)
	}
}

func TestGetWithOptionsDoesNotUseGhproxyForGitHubAPI(t *testing.T) {
	origHTTPDo := httpDo
	defer func() { httpDo = origHTTPDo }()

	var requested string
	httpDo = func(client *http.Client, req *http.Request) (*http.Response, error) {
		requested = req.URL.String()
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader(`{}`)),
		}, nil
	}

	resp, err := GetWithOptions("https://api.github.com/repos/gookit/gitw/releases/latest", Options{
		GhproxyEnabled: true,
		GhproxyHostURL: "https://gh.felicity.ac.cn",
	})
	if err != nil {
		t.Fatalf("GetWithOptions(): %v", err)
	}
	_ = resp.Body.Close()

	want := "https://api.github.com/repos/gookit/gitw/releases/latest"
	if requested != want {
		t.Fatalf("expected original api url %q, got %q", want, requested)
	}
}

func TestGetWithOptionsFallsBackToNextGhproxyHost(t *testing.T) {
	origHTTPDo := httpDo
	defer func() { httpDo = origHTTPDo }()

	var requested []string
	httpDo = func(client *http.Client, req *http.Request) (*http.Response, error) {
		requested = append(requested, req.URL.String())
		if strings.Contains(req.URL.Host, "gh.felicity.ac.cn") {
			return nil, io.EOF
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader("ok")),
		}, nil
	}

	resp, err := GetWithOptions("https://github.com/gookit/gitw/releases/download/v0.3.6/chlog-windows-amd64.exe", Options{
		GhproxyEnabled:   true,
		GhproxyHostURL:   "https://gh.felicity.ac.cn",
		GhproxyFallbacks: []string{"https://gh.llkk.cc"},
	})
	if err != nil {
		t.Fatalf("GetWithOptions(): %v", err)
	}
	_ = resp.Body.Close()

	if len(requested) != 2 {
		t.Fatalf("expected 2 ghproxy attempts, got %#v", requested)
	}
	if !strings.Contains(requested[1], "gh.llkk.cc") {
		t.Fatalf("expected fallback ghproxy host, got %#v", requested)
	}
}
