package cache

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/gookit/goutil/x/assert"
	cfgpkg "github.com/inherelab/eget/internal/config"
	"github.com/inherelab/eget/internal/util"
)

func TestServiceListReportsFilesWithPathKeys(t *testing.T) {
	cacheDir := t.TempDir()
	writeCacheTestFile(t, filepath.Join(cacheDir, "pkg-cache", "tool.zip"), "pkg")
	writeCacheTestFile(t, filepath.Join(cacheDir, "sdk-downloads", "go", "1.22.0", "go.zip"), "sdk")
	writeCacheTestFile(t, filepath.Join(cacheDir, "tool.zip.part"), "partial")

	result, err := (Service{}).List(cacheDir, ListOptions{})

	assert.NoErr(t, err)
	assert.Eq(t, cacheDir, result.CacheDir)
	assert.Eq(t, 2, len(result.Files))
	assert.Eq(t, int64(6), result.TotalSize)
	assert.Eq(t, "pkg-cache/tool.zip", result.Files[0].Path)
	assert.Contains(t, result.Files[0].PathKey, "path-md5:")
}

func TestServiceListCanSelectPartialRoot(t *testing.T) {
	cacheDir := t.TempDir()
	writeCacheTestFile(t, filepath.Join(cacheDir, "pkg-cache", "tool.zip"), "pkg")
	writeCacheTestFile(t, filepath.Join(cacheDir, "tool.zip.part"), "partial")

	result, err := (Service{}).List(cacheDir, ListOptions{Root: "partial"})

	assert.NoErr(t, err)
	assert.Eq(t, 1, len(result.Files))
	assert.Eq(t, KindPartial, result.Files[0].Kind)
}

func TestServiceStatusSummarizesKindsAndMirrorConfig(t *testing.T) {
	cacheDir := t.TempDir()
	writeCacheTestFile(t, filepath.Join(cacheDir, "pkg-cache", "tool.zip"), "pkg")
	writeCacheTestFile(t, filepath.Join(cacheDir, "api-cache", "repo.json"), "{}")
	timeout := 5
	cfg := cfgpkg.NewFile()
	cfg.Global.CacheDir = util.StringPtr(cacheDir)
	cfg.CacheMirror.Enable = util.BoolPtr(true)
	cfg.CacheMirror.URL = util.StringPtr("http://127.0.0.1:8686")
	cfg.CacheMirror.Timeout = &timeout
	cfg.CacheMirror.Fallback = util.BoolPtr(true)

	result, err := (Service{Config: cfg, Now: func() time.Time {
		return time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC)
	}}).Status("")

	assert.NoErr(t, err)
	assert.Eq(t, cacheDir, result.CacheDir)
	assert.Eq(t, 2, result.TotalFiles)
	assert.Eq(t, int64(5), result.TotalSize)
	assert.Eq(t, 1, result.Kinds[string(KindPkg)].Files)
	assert.True(t, result.CacheMirror.Enable)
	assert.Eq(t, "http://127.0.0.1:8686", result.CacheMirror.URL)
	assert.Contains(t, result.ServeCommand, "eget web")
}
