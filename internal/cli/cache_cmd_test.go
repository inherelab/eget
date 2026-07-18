package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gookit/goutil/x/assert"
	"github.com/gookit/goutil/x/ccolor"
	appcache "github.com/inherelab/eget/internal/app/cache"
	cfgpkg "github.com/inherelab/eget/internal/config"
)

func TestCliServiceHandleCacheCleanDryRun(t *testing.T) {
	tmp := newCLICacheDir(t)
	writeCLITestFile(t, filepath.Join(tmp, "old.zip"), "old")
	old := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
	assert.NoErr(t, os.Chtimes(filepath.Join(tmp, "old.zip"), old, old))

	cfg := cfgpkg.NewFile()
	cfg.Global.CacheDir = &tmp
	var stderr bytes.Buffer
	service := &cliService{
		cacheService: appcache.Service{
			Config: cfg,
			Now: func() time.Time {
				return time.Date(2026, 5, 26, 10, 0, 0, 0, time.UTC)
			},
		},
		stderr: &stderr,
	}

	err := service.handleCacheClean(&CacheCleanOptions{Older: "3d", DryRun: true})

	assert.NoErr(t, err)
	out := ccolor.ClearCode(stderr.String())
	assert.Contains(t, out, "Dry run: eget cache clean")
	assert.Contains(t, out, "matched files: 1")
	assert.True(t, fileExistsCLI(filepath.Join(tmp, "old.zip")))
}

func TestCleanOptionsFromCLIKeepLatest(t *testing.T) {
	got, err := cleanOptionsFromCLI(&CacheCleanOptions{KeepLatest: true})
	assert.NoErr(t, err)
	assert.True(t, got.KeepLatest)
	assert.Eq(t, []appcache.Kind{appcache.KindPkg}, got.Kinds)

	got, err = cleanOptionsFromCLI(&CacheCleanOptions{})
	assert.NoErr(t, err)
	assert.Eq(t, 72*time.Hour, got.Older)

	got, err = cleanOptionsFromCLI(&CacheCleanOptions{KeepLatest: true, Pkg: true})
	assert.NoErr(t, err)
	assert.True(t, got.KeepLatest)
}

func TestCleanOptionsFromCLIKeepLatestRejectsConflicts(t *testing.T) {
	tests := []*CacheCleanOptions{
		{KeepLatest: true, Older: "3d"},
		{KeepLatest: true, All: true},
		{KeepLatest: true, API: true},
		{KeepLatest: true, SDK: true},
		{KeepLatest: true, SDKIndex: true},
		{KeepLatest: true, Partial: true},
	}
	for _, opts := range tests {
		_, err := cleanOptionsFromCLI(opts)
		assert.Err(t, err)
	}
}

func TestCliServiceHandleCacheCleanKeepLatest(t *testing.T) {
	cacheDir := newCLICacheDir(t)
	oldFile := filepath.Join(cacheDir, "pkg-cache", "foo-1.0.0-a1b2c3d4.zip")
	newFile := filepath.Join(cacheDir, "pkg-cache", "foo-2.0.0-b1b2c3d4.zip")
	writeCLITestFile(t, oldFile, "old")
	writeCLITestFile(t, newFile, "new")
	writeCLITestFile(t, filepath.Join(cacheDir, "pkg-cache", "manual.zip"), "manual")

	cfg := cfgpkg.NewFile()
	cfg.Global.CacheDir = &cacheDir
	var stderr bytes.Buffer
	service := &cliService{cacheService: appcache.Service{Config: cfg}, stderr: &stderr}

	err := service.handleCacheClean(&CacheCleanOptions{KeepLatest: true, DryRun: true})
	assert.NoErr(t, err)
	out := ccolor.ClearCode(stderr.String())
	assert.Contains(t, out, "kept latest files: 1")
	assert.Contains(t, out, "unrecognized files: 1")
	assert.True(t, fileExistsCLI(oldFile))

	stderr.Reset()
	err = service.handleCacheClean(&CacheCleanOptions{KeepLatest: true, Yes: true})
	assert.NoErr(t, err)
	assert.False(t, fileExistsCLI(oldFile))
	assert.True(t, fileExistsCLI(newFile))
	assert.Contains(t, ccolor.ClearCode(stderr.String()), "kept latest files: 1")
}

func TestCliServiceHandleCacheCleanKeepLatestDryRunJSON(t *testing.T) {
	cacheDir := newCLICacheDir(t)
	writeCLITestFile(t, filepath.Join(cacheDir, "pkg-cache", "foo-1.0.0-a1b2c3d4.zip"), "old")
	writeCLITestFile(t, filepath.Join(cacheDir, "pkg-cache", "foo-2.0.0-b1b2c3d4.zip"), "new")
	cfg := cfgpkg.NewFile()
	cfg.Global.CacheDir = &cacheDir
	service := &cliService{cacheService: appcache.Service{Config: cfg}}

	reader, writer, err := os.Pipe()
	assert.NoErr(t, err)
	original := os.Stdout
	os.Stdout = writer
	err = service.handleCacheClean(&CacheCleanOptions{KeepLatest: true, DryRun: true, JSON: true})
	assert.NoErr(t, err)
	assert.NoErr(t, writer.Close())
	os.Stdout = original
	data, err := io.ReadAll(reader)
	assert.NoErr(t, err)

	var got appcache.CleanResult
	assert.NoErr(t, json.Unmarshal(data, &got))
	assert.Eq(t, 1, got.MatchedFiles)
	assert.Eq(t, 0, got.RemovedFiles)
	assert.Eq(t, 1, got.KeptLatestFiles)
}

func TestCliServiceHandleCacheCleanLargeDeletionRequiresYesInNonTTY(t *testing.T) {
	tmp := newCLICacheDir(t)
	for i := 0; i < 100; i++ {
		writeCLITestFile(t, filepath.Join(tmp, fmt.Sprintf("pkg-%03d.zip", i)), "pkg")
	}
	reader, writer, err := os.Pipe()
	assert.NoErr(t, err)
	assert.NoErr(t, writer.Close())
	origStdin := os.Stdin
	os.Stdin = reader
	defer func() {
		os.Stdin = origStdin
		assert.NoErr(t, reader.Close())
	}()

	cfg := cfgpkg.NewFile()
	cfg.Global.CacheDir = &tmp
	service := &cliService{
		cacheService: appcache.Service{Config: cfg},
		stderr:       io.Discard,
	}

	err = service.handleCacheClean(&CacheCleanOptions{Older: "3d", All: true})

	assert.Err(t, err)
	assert.Contains(t, err.Error(), "--yes")
	assert.True(t, fileExistsCLI(filepath.Join(tmp, "pkg-000.zip")))
}

func TestCliServiceHandleCacheCleanLargeDeletionYesSkipsConfirmation(t *testing.T) {
	tmp := newCLICacheDir(t)
	for i := 0; i < 100; i++ {
		writeCLITestFile(t, filepath.Join(tmp, fmt.Sprintf("pkg-%03d.zip", i)), "pkg")
	}

	cfg := cfgpkg.NewFile()
	cfg.Global.CacheDir = &tmp
	var stderr bytes.Buffer
	service := &cliService{
		cacheService: appcache.Service{Config: cfg},
		stderr:       &stderr,
	}

	err := service.handleCacheClean(&CacheCleanOptions{Older: "3d", All: true, Yes: true})

	assert.NoErr(t, err)
	assert.Contains(t, ccolor.ClearCode(stderr.String()), "removed files: 100")
	assert.False(t, fileExistsCLI(filepath.Join(tmp, "pkg-000.zip")))
}

func TestCliServiceHandleCacheListJSON(t *testing.T) {
	tmp := newCLICacheDir(t)
	writeCLITestFile(t, filepath.Join(tmp, "pkg-cache", "tool.zip"), "pkg")
	cfg := cfgpkg.NewFile()
	cfg.Global.CacheDir = &tmp
	service := &cliService{cacheService: appcache.Service{Config: cfg}}

	err := service.handleCacheList(&CacheListOptions{JSON: true})

	assert.NoErr(t, err)
}

func TestCliServiceHandleCacheStatusText(t *testing.T) {
	tmp := newCLICacheDir(t)
	writeCLITestFile(t, filepath.Join(tmp, "pkg-cache", "tool.zip"), "pkg")
	cfg := cfgpkg.NewFile()
	cfg.Global.CacheDir = &tmp
	var stderr bytes.Buffer
	service := &cliService{cacheService: appcache.Service{Config: cfg}, stderr: &stderr}

	err := service.handleCacheStatus(&CacheStatusOptions{})

	assert.NoErr(t, err)
	assert.Contains(t, stderr.String(), "Cache status")
	assert.Contains(t, stderr.String(), "cache dir:")
}

func TestCliServiceHandleCacheCleanDryRunJSON(t *testing.T) {
	tmp := newCLICacheDir(t)
	writeCLITestFile(t, filepath.Join(tmp, "old.zip"), "old")
	cfg := cfgpkg.NewFile()
	cfg.Global.CacheDir = &tmp
	service := &cliService{cacheService: appcache.Service{Config: cfg}}

	err := service.handleCacheClean(&CacheCleanOptions{Older: "3d", DryRun: true, JSON: true})

	assert.NoErr(t, err)
	assert.True(t, fileExistsCLI(filepath.Join(tmp, "old.zip")))
}

func writeCLITestFile(t *testing.T, path, body string) {
	t.Helper()
	assert.NoErr(t, os.MkdirAll(filepath.Dir(path), 0o755))
	assert.NoErr(t, os.WriteFile(path, []byte(body), 0o644))
}

func newCLICacheDir(t *testing.T) string {
	t.Helper()
	cacheDir := filepath.Join(t.TempDir(), "eget")
	assert.NoErr(t, os.MkdirAll(cacheDir, 0o755))
	return cacheDir
}

func fileExistsCLI(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
