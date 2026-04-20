package install

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	pb "github.com/schollz/progressbar/v3"
)

func TestCacheFilePath(t *testing.T) {
	cacheDir := t.TempDir()
	got := CacheFilePath(cacheDir, "https://example.com/tool.tar.gz")
	if filepath.Dir(got) != cacheDir {
		t.Fatalf("expected cache file under %q, got %q", cacheDir, got)
	}
	if filepath.Ext(got) != ".gz" {
		t.Fatalf("expected extension .gz, got %q", filepath.Ext(got))
	}
}

func TestDownloadBodyUsesCacheWhenAvailable(t *testing.T) {
	cacheDir := t.TempDir()
	url := "https://example.com/tool.tar.gz"
	cachePath := CacheFilePath(cacheDir, url)
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		t.Fatalf("mkdir cache dir: %v", err)
	}
	if err := os.WriteFile(cachePath, []byte("cached-data"), 0o644); err != nil {
		t.Fatalf("write cache file: %v", err)
	}

	calls := 0
	origGetWithOptions := downloadGetWithOptions
	defer func() { downloadGetWithOptions = origGetWithOptions }()
	downloadGetWithOptions = func(url string, opts Options) (*http.Response, error) {
		calls++
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("network-data")),
		}, nil
	}

	var stderr bytes.Buffer
	runner := &InstallRunner{Stderr: &stderr}
	body, err := runner.downloadBody(url, Options{CacheDir: cacheDir})
	if err != nil {
		t.Fatalf("download body: %v", err)
	}

	if string(body) != "cached-data" {
		t.Fatalf("expected cached data, got %q", string(body))
	}
	if calls != 0 {
		t.Fatalf("expected no network calls, got %d", calls)
	}
	if got := stderr.String(); !strings.Contains(got, "Using cached file") {
		t.Fatalf("expected cached-file notice, got %q", got)
	}
}

func TestDownloadBodyWritesCacheAfterDownload(t *testing.T) {
	cacheDir := t.TempDir()
	url := "https://example.com/tool.tar.gz"
	cachePath := CacheFilePath(cacheDir, url)

	origGetWithOptions := downloadGetWithOptions
	defer func() { downloadGetWithOptions = origGetWithOptions }()
	downloadGetWithOptions = func(url string, opts Options) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("network-data")),
		}, nil
	}

	runner := &InstallRunner{Stderr: io.Discard}
	body, err := runner.downloadBody(url, Options{CacheDir: cacheDir})
	if err != nil {
		t.Fatalf("download body: %v", err)
	}
	if string(body) != "network-data" {
		t.Fatalf("expected network data, got %q", string(body))
	}

	saved, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("read cache file: %v", err)
	}
	if string(saved) != "network-data" {
		t.Fatalf("expected cached network data, got %q", string(saved))
	}
}

func TestDownloadPrintsProxyNoticeForRemoteRequest(t *testing.T) {
	var notice bytes.Buffer
	origNoticeWriter := proxyNoticeWriter
	origGetWithOptions := downloadGetWithOptions
	defer func() {
		proxyNoticeWriter = origNoticeWriter
		downloadGetWithOptions = origGetWithOptions
	}()
	proxyNoticeWriter = &notice
	downloadGetWithOptions = func(url string, opts Options) (*http.Response, error) {
		if opts.ProxyURL != "http://127.0.0.1:7890" {
			t.Fatalf("expected proxy url to propagate, got %q", opts.ProxyURL)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("network-data")),
		}, nil
	}

	err := Download("https://example.com/tool.tar.gz", io.Discard, func(size int64) *pb.ProgressBar {
		return pb.NewOptions64(size, pb.OptionSetWriter(io.Discard))
	}, Options{ProxyURL: "http://127.0.0.1:7890"})
	if err != nil {
		t.Fatalf("Download(): %v", err)
	}

	if got := notice.String(); !strings.Contains(got, "Using proxy_url for download request: http://127.0.0.1:7890") {
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

	err := Download(localFile, io.Discard, func(size int64) *pb.ProgressBar {
		return pb.NewOptions64(size, pb.OptionSetWriter(io.Discard))
	}, Options{ProxyURL: "http://127.0.0.1:7890"})
	if err != nil {
		t.Fatalf("Download(local): %v", err)
	}

	if got := notice.String(); got != "" {
		t.Fatalf("expected no proxy notice for local file, got %q", got)
	}
}

func TestNewHTTPGetterUsesProxyURL(t *testing.T) {
	proxyFunc, err := proxyFuncFor("http://127.0.0.1:7890")
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
	_, err := proxyFuncFor("://bad-proxy")
	if err == nil {
		t.Fatal("expected invalid proxy url error")
	}
	if !strings.Contains(err.Error(), "invalid proxy_url") {
		t.Fatalf("expected invalid proxy_url error, got %v", err)
	}
}

func TestProxyFuncForFallsBackToEnvironment(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:7891")
	proxyFunc, err := proxyFuncFor("")
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

	if got := notice.String(); !strings.Contains(got, "Using proxy_url for GitHub API request: http://127.0.0.1:7890") {
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

func TestOutputPathUsesHeuristicExecutableRename(t *testing.T) {
	file := ExtractedFile{Name: "chlog-windows-amd64.exe", mode: 0o666}
	got := outputPath(file, "", false, "")
	if got != "chlog.exe" {
		t.Fatalf("expected heuristic output name chlog.exe, got %q", got)
	}
}

func TestOutputPathUsesPreferredNameForExecutable(t *testing.T) {
	file := ExtractedFile{Name: "chlog-windows-amd64.exe", mode: 0o666}
	got := outputPath(file, "", false, "chlog")
	if got != "chlog.exe" {
		t.Fatalf("expected preferred output name chlog.exe, got %q", got)
	}
}

func TestOutputPathUsesPreferredNameWithExplicitExtension(t *testing.T) {
	file := ExtractedFile{Name: "chlog-windows-amd64.exe", mode: 0o666}
	got := outputPath(file, "", false, "custom-name.exe")
	if got != "custom-name.exe" {
		t.Fatalf("expected preferred explicit output name custom-name.exe, got %q", got)
	}
}
