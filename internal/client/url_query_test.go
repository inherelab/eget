package client

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gookit/goutil/x/assert"
)

func TestQueryURL(t *testing.T) {
	t.Run("reads HEAD metadata after redirect", testQueryURLReadsHeadMetadataAfterRedirect)
	t.Run("rejects non HTTP URL", testQueryURLRejectsNonHTTPURL)
	t.Run("falls back when HEAD is unsupported", testQueryURLFallsBackWhenHeadUnsupported)
	t.Run("falls back when HEAD omits size", testQueryURLFallsBackWhenHeadOmitsSize)
}

func testQueryURLReadsHeadMetadataAfterRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "/files/tool.zip", http.StatusFound)
			return
		}
		assert.Eq(t, http.MethodHead, r.Method)
		w.Header().Set("Content-Disposition", `attachment; filename="release.zip"`)
		w.Header().Set("Content-Length", "2048")
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Last-Modified", "Tue, 05 Aug 2026 10:00:00 GMT")
		w.Header().Set("ETag", `"abc123"`)
		w.Header().Set("Accept-Ranges", "bytes")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	requestedURL := server.URL + "/start"
	info, err := QueryURL(requestedURL, Options{})

	assert.NoErr(t, err)
	assert.Eq(t, requestedURL, info.RequestedURL)
	assert.Eq(t, server.URL+"/files/tool.zip", info.FinalURL)
	assert.Eq(t, "200 OK", info.Status)
	assert.Eq(t, http.StatusOK, info.StatusCode)
	assert.Eq(t, "release.zip", info.FileName)
	assert.NotNil(t, info.Size)
	assert.Eq(t, int64(2048), *info.Size)
	assert.Eq(t, "application/zip", info.ContentType)
	assert.Eq(t, "Tue, 05 Aug 2026 10:00:00 GMT", info.LastModified)
	assert.Eq(t, `"abc123"`, info.ETag)
	assert.Eq(t, "bytes", info.AcceptRanges)
}

func testQueryURLRejectsNonHTTPURL(t *testing.T) {
	_, err := QueryURL("ftp://example.com/tool.zip", Options{})

	assert.Err(t, err)
	assert.Contains(t, err.Error(), "HTTP or HTTPS")
}

func testQueryURLFallsBackWhenHeadUnsupported(t *testing.T) {
	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		assert.Eq(t, "bytes=0-0", r.Header.Get("Range"))
		w.Header().Set("Content-Disposition", `attachment; filename="fallback.zip"`)
		w.Header().Set("Content-Range", "bytes 0-0/4096")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte("x"))
	}))
	defer server.Close()

	info, err := QueryURL(server.URL+"/fallback.zip", Options{})

	assert.NoErr(t, err)
	assert.Eq(t, []string{http.MethodHead, http.MethodGet}, methods)
	assert.NotNil(t, info.Size)
	if info.Size != nil {
		assert.Eq(t, int64(4096), *info.Size)
	}
	assert.Eq(t, "fallback.zip", info.FileName)
}

func testQueryURLFallsBackWhenHeadOmitsSize(t *testing.T) {
	getCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		getCalled = true
		assert.Eq(t, "bytes=0-0", r.Header.Get("Range"))
		w.Header().Set("Content-Range", "bytes 0-0/8192")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte("x"))
	}))
	defer server.Close()

	info, err := QueryURL(server.URL+"/tool.bin", Options{})

	assert.NoErr(t, err)
	assert.True(t, getCalled)
	assert.NotNil(t, info.Size)
	if info.Size != nil {
		assert.Eq(t, int64(8192), *info.Size)
	}
}
