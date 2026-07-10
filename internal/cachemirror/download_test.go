package cachemirror

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gookit/goutil/x/assert"
)

func TestDownloadToFileWritesMirrorHit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Eq(t, "/download/path-md5:abc", r.URL.Path)
		_, _ = w.Write([]byte("archive"))
	}))
	defer server.Close()

	target := filepath.Join(t.TempDir(), "pkg-cache", "tool.zip")
	result, err := DownloadToFile(context.Background(), Options{Enable: true, URL: server.URL}, "path-md5:abc", target)

	assert.NoErr(t, err)
	assert.True(t, result.Hit)
	assert.Eq(t, int64(len("archive")), result.Size)
	data, err := os.ReadFile(target)
	assert.NoErr(t, err)
	assert.Eq(t, "archive", string(data))
}

func TestDownloadToFileWritesProgress(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "7")
		_, _ = w.Write([]byte("archive"))
	}))
	defer server.Close()

	var progress bytes.Buffer
	result, err := DownloadToFile(context.Background(), Options{Enable: true, URL: server.URL}, "path-md5:abc", filepath.Join(t.TempDir(), "tool.zip"), func(size int64) io.Writer {
		assert.Eq(t, int64(7), size)
		return &progress
	})

	assert.NoErr(t, err)
	assert.True(t, result.Hit)
	assert.Eq(t, "archive", progress.String())
}

func TestDownloadToFileReturnsMissOn404(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()

	result, err := DownloadToFile(context.Background(), Options{Enable: true, URL: server.URL}, "path-md5:missing", filepath.Join(t.TempDir(), "tool.zip"))

	assert.NoErr(t, err)
	assert.False(t, result.Hit)
}

func TestDownloadToFileReturnsTimeoutError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		_, _ = w.Write([]byte("late"))
	}))
	defer server.Close()

	_, err := DownloadToFile(context.Background(), Options{Enable: true, URL: server.URL, Timeout: time.Millisecond}, "path-md5:abc", filepath.Join(t.TempDir(), "tool.zip"))

	assert.Err(t, err)
}

func TestDownloadToFileTimeoutOnlyLimitsResponseHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "8")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected response writer to support flush")
		}
		flusher.Flush()
		for _, chunk := range []string{"slow", "body"} {
			time.Sleep(20 * time.Millisecond)
			_, _ = io.WriteString(w, chunk)
			flusher.Flush()
		}
	}))
	defer server.Close()

	target := filepath.Join(t.TempDir(), "pkg-cache", "tool.zip")
	result, err := DownloadToFile(context.Background(), Options{Enable: true, URL: server.URL, Timeout: 10 * time.Millisecond}, "path-md5:abc", target)

	assert.NoErr(t, err)
	assert.True(t, result.Hit)
	assert.Eq(t, int64(8), result.Size)
	data, err := os.ReadFile(target)
	assert.NoErr(t, err)
	assert.Eq(t, "slowbody", string(data))
}

func TestDownloadToFileTimeoutStillAppliesBeforeResponseHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		_, _ = fmt.Fprint(w, "late")
	}))
	defer server.Close()

	_, err := DownloadToFile(context.Background(), Options{Enable: true, URL: server.URL, Timeout: 10 * time.Millisecond}, "path-md5:abc", filepath.Join(t.TempDir(), "tool.zip"))

	assert.Err(t, err)
}
