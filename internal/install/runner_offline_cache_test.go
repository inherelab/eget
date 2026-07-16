package install

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/gookit/goutil/x/assert"
	"github.com/inherelab/eget/internal/cachemirror"
	clientpkg "github.com/inherelab/eget/internal/client"
)

func TestRunnerInstallsKnownGitHubToolFullyOfflineFromCacheMirror(t *testing.T) {
	cacheRoot := t.TempDir()
	apiCacheDir := filepath.Join(cacheRoot, "api-cache")
	metadataURL := "https://api.github.com/repos/owner/tool/releases/latest"
	assetURL := "https://origin.invalid/tool-v1.2.3-windows-amd64.zip"
	archive := zipBytes(t, map[string]string{"tool.exe": "offline tool"})

	metadataPath := APICacheFilePath(apiCacheDir, metadataURL)
	metadataRel, err := cachemirror.RelPath(cacheRoot, metadataPath)
	assert.NoErr(t, err)
	metadataKey := cachemirror.KeyForRelPath(metadataRel)

	assetPath := CacheFilePathWithMeta(cacheRoot, assetURL, cacheMetaFromOptions(Options{
		CacheName: "tool",
		System:    "windows/amd64",
	}))
	assetRel, err := cachemirror.RelPath(cacheRoot, assetPath)
	assert.NoErr(t, err)
	assetKey := cachemirror.KeyForRelPath(assetRel)

	releaseJSON := fmt.Sprintf(`{"tag_name":"v1.2.3","assets":[{"browser_download_url":%q}]}`, assetURL)
	payloads := map[string][]byte{
		"/download/" + metadataKey: []byte(releaseJSON),
		"/download/" + assetKey:    archive,
	}
	requests := make(map[string]int)
	var requestsMu sync.Mutex
	mirror := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestsMu.Lock()
		requests[r.URL.Path]++
		body, ok := payloads[r.URL.Path]
		requestsMu.Unlock()
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(body)
	}))
	defer mirror.Close()

	originCalls := 0
	restoreHTTPDo := clientpkg.SetHTTPDoForTest(func(client *http.Client, req *http.Request) (*http.Response, error) {
		originCalls++
		return nil, errors.New("origin access forbidden")
	})
	defer restoreHTTPDo()

	outputDir := t.TempDir()
	runner := NewRunner(NewDefaultService(nil, nil))
	runner.Stdout = io.Discard
	runner.Stderr = io.Discard
	result, err := runner.Run("owner/tool", Options{
		APICacheDir: apiCacheDir,
		CacheDir:    cacheRoot,
		Output:      outputDir,
		System:      "windows/amd64",
		CacheMirror: cachemirror.Options{Enable: true, URL: mirror.URL, Fallback: false},
	})
	if err != nil {
		t.Fatalf("run offline install: %v", err)
	}
	assert.Eq(t, 0, originCalls)

	requestsMu.Lock()
	assert.Eq(t, 1, requests["/download/"+metadataKey])
	assert.Eq(t, 1, requests["/download/"+assetKey])
	requestsMu.Unlock()

	_, err = os.Stat(metadataPath)
	assert.NoErr(t, err)
	_, err = os.Stat(assetPath)
	assert.NoErr(t, err)

	installed := filepath.Join(outputDir, "tool.exe")
	body, err := os.ReadFile(installed)
	assert.NoErr(t, err)
	assert.Eq(t, "offline tool", string(body))
	assert.Contains(t, result.ExtractedFiles, installed)
}
