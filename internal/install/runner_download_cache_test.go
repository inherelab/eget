package install

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gookit/goutil/x/assert"
	"github.com/inherelab/eget/internal/cachemirror"
)

func TestCacheFilePath(t *testing.T) {
	cacheDir := t.TempDir()
	got := CacheFilePath(cacheDir, "https://github.com/babarot/gomi/releases/download/v1.6.3/gomi_Linux_x86_64.tar.gz")
	wantDir := filepath.Join(cacheDir, "pkg-cache")
	if filepath.Dir(got) != wantDir {
		t.Fatalf("expected cache file under %q, got %q", wantDir, got)
	}
	name := filepath.Base(got)
	if !strings.HasPrefix(name, "gomi_Linux_x86_64-1.6.3-") {
		t.Fatalf("expected readable cache name, got %q", name)
	}
	if !strings.HasSuffix(name, ".tar.gz") {
		t.Fatalf("expected extension .tar.gz, got %q", name)
	}
	if len(strings.TrimSuffix(strings.TrimPrefix(name, "gomi_Linux_x86_64-1.6.3-"), ".tar.gz")) != 8 {
		t.Fatalf("expected 8-char short hash in %q", name)
	}
}

func TestCacheFilePathUsesMetadataForOpaqueURL(t *testing.T) {
	cacheDir := t.TempDir()
	got := CacheFilePathWithMeta(cacheDir, "https://example.com/download?id=123", CacheMeta{
		Name:    "gomi",
		Version: "v1.6.3",
	})

	name := filepath.Base(got)
	if !strings.HasPrefix(name, "gomi-1.6.3-") {
		t.Fatalf("expected metadata cache name, got %q", name)
	}
	if !strings.HasSuffix(name, ".bin") {
		t.Fatalf("expected .bin fallback extension, got %q", name)
	}
}

func TestDownloadBodyAddsPlatformToOpaqueCacheName(t *testing.T) {
	cacheDir := t.TempDir()
	url := "https://example.com/download?id=123"
	origGetWithOptions := downloadGetWithOptions
	defer func() { downloadGetWithOptions = origGetWithOptions }()
	downloadGetWithOptions = func(url string, opts Options) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("binary")),
		}, nil
	}

	runner := &InstallRunner{}
	_, err := runner.downloadBody(url, Options{
		CacheDir:     cacheDir,
		CacheName:    "claude",
		CacheVersion: "2.1.160",
		System:       "linux/amd64",
	})

	assert.NoErr(t, err)
	matches, err := filepath.Glob(filepath.Join(cacheDir, "pkg-cache", "claude-2.1.160-linux-amd64-*.bin"))
	assert.NoErr(t, err)
	assert.Eq(t, 1, len(matches))
}

func TestDownloadBodyAddsTemplateResolvedPlatformToCacheName(t *testing.T) {
	cacheDir := t.TempDir()
	url := "https://example.com/claude/latest"
	origGetWithOptions := downloadGetWithOptions
	defer func() { downloadGetWithOptions = origGetWithOptions }()
	downloadGetWithOptions = func(url string, opts Options) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("binary")),
		}, nil
	}

	runner := &InstallRunner{}
	_, err := runner.downloadBody(url, Options{
		CacheDir:     cacheDir,
		CacheName:    "claude",
		CacheVersion: "2.1.160",
		URLTemplate: URLTemplateOptions{
			ResolvedVars: map[string]string{"os": "darwin", "arch": "arm64"},
		},
	})

	assert.NoErr(t, err)
	matches, err := filepath.Glob(filepath.Join(cacheDir, "pkg-cache", "claude-2.1.160-darwin-arm64-*.bin"))
	assert.NoErr(t, err)
	assert.Eq(t, 1, len(matches))
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

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	runner := &InstallRunner{Stdout: &stdout, Stderr: &stderr}
	downloaded, err := runner.downloadBody(url, Options{CacheDir: cacheDir})
	if err != nil {
		t.Fatalf("download body: %v", err)
	}

	if string(downloaded.Body) != "cached-data" {
		t.Fatalf("expected cached data, got %q", string(downloaded.Body))
	}
	if calls != 0 {
		t.Fatalf("expected no network calls, got %d", calls)
	}
	if got := stdout.String(); !strings.Contains(got, "Using cached file") {
		t.Fatalf("expected cached-file notice, got %q", got)
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("expected cached-file notice stderr to be empty, got %q", got)
	}
}

func TestDownloadBodyUsesCachedFileWithoutRemoteProbe(t *testing.T) {
	cacheDir := t.TempDir()
	url := "https://github.com/pbatard/rufus/releases/download/v4.14/rufus-4.14p.exe"
	cachePath := CacheFilePath(cacheDir, url)
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		t.Fatalf("mkdir cache dir: %v", err)
	}
	if err := os.WriteFile(cachePath, []byte("cached-data"), 0o644); err != nil {
		t.Fatalf("write cache file: %v", err)
	}
	localTime := time.Date(2026, 5, 24, 13, 19, 24, 0, time.UTC)
	assert.Nil(t, applyModTime(cachePath, localTime))

	origHTTPDo := httpDo
	defer func() { httpDo = origHTTPDo }()
	httpDo = func(client *http.Client, req *http.Request) (*http.Response, error) {
		t.Fatalf("cache hit should not probe remote metadata with %s %s", req.Method, req.URL.String())
		return nil, nil
	}

	origDownloadGetWithOptions := downloadGetWithOptions
	defer func() { downloadGetWithOptions = origDownloadGetWithOptions }()
	downloadGetWithOptions = func(url string, opts Options) (*http.Response, error) {
		t.Fatal("cache hit should not re-download body")
		return nil, nil
	}

	var stdout bytes.Buffer
	runner := &InstallRunner{Stdout: &stdout, Stderr: io.Discard}
	downloaded, err := runner.downloadBody(url, Options{CacheDir: cacheDir})
	if err != nil {
		t.Fatalf("download body: %v", err)
	}

	assert.Eq(t, "cached-data", string(downloaded.Body))
	assert.Eq(t, localTime, downloaded.ModTime.UTC())
	assert.Eq(t, localTime, fileModTime(cachePath).UTC())
	if got := stdout.String(); !strings.Contains(got, "Using cached file") {
		t.Fatalf("expected cached-file notice, got %q", got)
	}
}

func TestDownloadBodyRedownloadsHTMLCachedArchive(t *testing.T) {
	cacheDir := t.TempDir()
	url := "https://downloads.sourceforge.net/project/victoria-ssd-hdd/Victoria537.zip"
	cachePath := CacheFilePath(cacheDir, url)
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		t.Fatalf("mkdir cache dir: %v", err)
	}
	if err := os.WriteFile(cachePath, []byte("<!doctype html><html></html>"), 0o644); err != nil {
		t.Fatalf("write cache file: %v", err)
	}

	calls := 0
	origGetWithOptions := downloadGetWithOptions
	defer func() { downloadGetWithOptions = origGetWithOptions }()
	downloadGetWithOptions = func(url string, opts Options) (*http.Response, error) {
		calls++
		return &http.Response{
			StatusCode:    http.StatusOK,
			Body:          io.NopCloser(strings.NewReader("zip-data")),
			ContentLength: 8,
		}, nil
	}

	var stdout bytes.Buffer
	runner := &InstallRunner{Stdout: &stdout, Stderr: io.Discard}
	downloaded, err := runner.downloadBody(url, Options{CacheDir: cacheDir})
	if err != nil {
		t.Fatalf("download body: %v", err)
	}

	assert.Eq(t, "zip-data", string(downloaded.Body))
	assert.Eq(t, 1, calls)
	saved, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("read cache file: %v", err)
	}
	assert.Eq(t, "zip-data", string(saved))
	assert.False(t, strings.Contains(stdout.String(), "Using cached file"))
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
	downloaded, err := runner.downloadBody(url, Options{CacheDir: cacheDir})
	if err != nil {
		t.Fatalf("download body: %v", err)
	}
	if string(downloaded.Body) != "network-data" {
		t.Fatalf("expected network data, got %q", string(downloaded.Body))
	}

	saved, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("read cache file: %v", err)
	}
	if string(saved) != "network-data" {
		t.Fatalf("expected cached network data, got %q", string(saved))
	}
}

func TestDownloadBodyUsesContentDispositionFilename(t *testing.T) {
	origGetWithOptions := downloadGetWithOptions
	defer func() { downloadGetWithOptions = origGetWithOptions }()
	downloadGetWithOptions = func(url string, opts Options) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Disposition": []string{`attachment; filename="the-skeleton-key-linux.zip"`}},
			Body:       io.NopCloser(strings.NewReader("zip-data")),
		}, nil
	}

	downloaded, err := (&InstallRunner{}).downloadBody("https://example.com/raw/tool?id=123", Options{})

	assert.NoErr(t, err)
	assert.Eq(t, "the-skeleton-key-linux.zip", downloaded.Filename)
}

func TestDownloadBodyUsesCacheMirrorBeforeOrigin(t *testing.T) {
	cacheDir := t.TempDir()
	var originHit bool
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		originHit = true
		_, _ = w.Write([]byte("origin"))
	}))
	defer origin.Close()

	downloadURL := origin.URL + "/tool.zip"
	cachePath := CacheFilePath(cacheDir, downloadURL)
	rel, err := cachemirror.RelPath(cacheDir, cachePath)
	assert.NoErr(t, err)
	expectedPath := "/download/" + cachemirror.KeyForRelPath(rel)
	mirror := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Eq(t, expectedPath, r.URL.Path)
		_, _ = w.Write([]byte("mirror"))
	}))
	defer mirror.Close()

	var stdout bytes.Buffer
	runner := &InstallRunner{Stdout: &stdout}
	got, err := runner.downloadBody(downloadURL, Options{
		CacheDir: cacheDir,
		CacheMirror: cachemirror.Options{
			Enable:   true,
			URL:      mirror.URL,
			Fallback: true,
		},
	})

	assert.NoErr(t, err)
	assert.Eq(t, "mirror", string(got.Body))
	assert.False(t, originHit)
	output := stdout.String()
	noticeAt := strings.Index(output, "Use cache mirror")
	progressAt := strings.Index(output, "Downloading")
	if noticeAt < 0 || progressAt < 0 || noticeAt > progressAt {
		t.Fatalf("cache mirror notice should precede download progress, got %q", output)
	}
	assert.Contains(t, output, mirror.URL)
	assert.False(t, strings.Contains(output, "Using cache mirror file"))
	saved, err := os.ReadFile(cachePath)
	assert.NoErr(t, err)
	assert.Eq(t, "mirror", string(saved))
}

func TestDownloadBodyFallsBackWhenCacheMirrorMisses(t *testing.T) {
	mirror := httptest.NewServer(http.NotFoundHandler())
	defer mirror.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("origin"))
	}))
	defer origin.Close()

	got, err := (&InstallRunner{}).downloadBody(origin.URL+"/tool.zip", Options{
		CacheDir: t.TempDir(),
		CacheMirror: cachemirror.Options{
			Enable:   true,
			URL:      mirror.URL,
			Fallback: true,
		},
	})

	assert.NoErr(t, err)
	assert.Eq(t, "origin", string(got.Body))
}

func TestDownloadBodyPrintsCacheMirrorFallbackError(t *testing.T) {
	mirror := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "mirror down", http.StatusInternalServerError)
	}))
	defer mirror.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("origin"))
	}))
	defer origin.Close()

	var stdout bytes.Buffer
	got, err := (&InstallRunner{Stdout: &stdout}).downloadBody(origin.URL+"/tool.zip", Options{
		CacheDir: t.TempDir(),
		CacheMirror: cachemirror.Options{
			Enable:   true,
			URL:      mirror.URL,
			Fallback: true,
		},
	})

	assert.NoErr(t, err)
	assert.Eq(t, "origin", string(got.Body))
	assert.Contains(t, stdout.String(), "Cache mirror failed")
	assert.Contains(t, stdout.String(), "fallback to origin")
	assert.Contains(t, stdout.String(), "500")
}

func TestDownloadBodyErrorsWhenCacheMirrorFallbackDisabled(t *testing.T) {
	mirror := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer mirror.Close()

	_, err := (&InstallRunner{}).downloadBody("https://example.com/tool.zip", Options{
		CacheDir: t.TempDir(),
		CacheMirror: cachemirror.Options{
			Enable:   true,
			URL:      mirror.URL,
			Fallback: false,
		},
	})

	assert.Err(t, err)
}

func TestDownloadBodyUsesCacheMetadata(t *testing.T) {
	cacheDir := t.TempDir()
	url := "https://example.com/download?id=123"
	cachePath := CacheFilePathWithMeta(cacheDir, url, CacheMeta{Name: "gomi", Version: "v1.6.3"})
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

	runner := &InstallRunner{Stderr: io.Discard}
	downloaded, err := runner.downloadBody(url, Options{CacheDir: cacheDir, CacheName: "gomi", CacheVersion: "v1.6.3"})
	if err != nil {
		t.Fatalf("download body: %v", err)
	}
	if string(downloaded.Body) != "cached-data" {
		t.Fatalf("expected cached data, got %q", string(downloaded.Body))
	}
	if calls != 0 {
		t.Fatalf("expected no network calls, got %d", calls)
	}
}

func TestDownloadBodyResumesLargeCachedDownload(t *testing.T) {
	body := bytes.Repeat([]byte("r"), 12*1024*1024)
	chunkSize := 4 * 1024 * 1024
	chunkStart := 2 * chunkSize
	chunkEnd := len(body) - 1

	var gotRange atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("ETag", `"install-resume-v1"`)
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Length", strconv.Itoa(len(body)))
			return
		}
		rangeHeader := r.Header.Get("Range")
		gotRange.Store(rangeHeader)
		if rangeHeader != "" {
			if rangeHeader != fmt.Sprintf("bytes=%d-%d", chunkStart, chunkEnd) {
				t.Fatalf("unexpected range %q", rangeHeader)
			}
			w.Header().Set("Content-Length", strconv.Itoa(len(body)-chunkStart))
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", chunkStart, chunkEnd, len(body)))
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(body[chunkStart:])
			return
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		_, _ = w.Write(body)
	}))
	defer server.Close()
	downloadURL := server.URL + "/tool.zip"

	cacheDir := t.TempDir()
	cachePath := CacheFilePathWithMeta(cacheDir, downloadURL, CacheMeta{})
	assert.Nil(t, os.MkdirAll(filepath.Dir(cachePath), 0o755))
	part := make([]byte, len(body))
	copy(part[:chunkStart], body[:chunkStart])
	assert.Nil(t, os.WriteFile(cachePath+".part", part, 0o644))
	meta := fmt.Sprintf(`{
  "schema": 2,
  "url": %q,
  "size": %d,
  "etag": %q,
  "chunk_size": %d,
  "chunks": [
    {"start": 0, "end": %d, "done": true},
    {"start": %d, "end": %d, "done": true},
    {"start": %d, "end": %d, "done": false}
  ]
}
`, downloadURL, len(body), `"install-resume-v1"`, chunkSize, chunkSize-1, chunkSize, chunkStart-1, chunkStart, chunkEnd)
	assert.Nil(t, os.WriteFile(cachePath+".meta.json", []byte(meta), 0o644))

	runner := &InstallRunner{Stderr: io.Discard}
	got, err := runner.downloadBody(downloadURL, Options{CacheDir: cacheDir})

	assert.Nil(t, err)
	assert.Eq(t, fmt.Sprintf("bytes=%d-%d", chunkStart, chunkEnd), gotRange.Load())
	assert.Eq(t, body, got.Body)
	saved, readErr := os.ReadFile(cachePath)
	assert.Nil(t, readErr)
	assert.Eq(t, body, saved)
	_, statErr := os.Stat(cachePath + ".part")
	assert.True(t, os.IsNotExist(statErr))
}
