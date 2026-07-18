package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gookit/goutil/x/assert"
	cfgpkg "github.com/inherelab/eget/internal/config"
)

func TestParseOlderDuration(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  time.Duration
	}{
		{"minutes", "30m", 30 * time.Minute},
		{"hours", "12h", 12 * time.Hour},
		{"days", "3d", 72 * time.Hour},
		{"weeks", "1w", 7 * 24 * time.Hour},
		{"go duration", "72h", 72 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseOlderDuration(tt.input)
			assert.NoErr(t, err)
			assert.Eq(t, tt.want, got)
		})
	}
}

func TestParseOlderDurationRejectsInvalidInput(t *testing.T) {
	tests := []string{"", "0", "0d", "-1d", "1mo", "abc"}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			_, err := ParseOlderDuration(input)
			assert.Err(t, err)
		})
	}
}

func TestServiceResolveCacheDir(t *testing.T) {
	tmp := t.TempDir()
	cfg := cfgpkg.NewFile()
	cfg.Global.CacheDir = &tmp
	service := Service{Config: cfg}

	got, err := service.ResolveCacheDir()

	assert.NoErr(t, err)
	assert.Eq(t, tmp, got)
}

func TestServiceResolveCacheDirUsesDefault(t *testing.T) {
	service := Service{Config: cfgpkg.NewFile()}

	got, err := service.ResolveCacheDir()

	assert.NoErr(t, err)
	assert.Contains(t, got, ".cache")
	assert.Contains(t, got, "eget")
}

func TestServiceRejectsDangerousCacheDir(t *testing.T) {
	nonCacheDir := t.TempDir()
	writeCacheTestFile(t, filepath.Join(nonCacheDir, "unrelated.zip"), "data")
	tests := []struct {
		name string
		dir  string
	}{
		{"empty", ""},
		{"root", filepath.VolumeName(filepath.Clean(os.TempDir())) + string(filepath.Separator)},
		{"non cache dir", nonCacheDir},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCacheDirForMutation(tt.dir)
			assert.Err(t, err)
		})
	}
}

func TestServiceAcceptsPkgCacheLayout(t *testing.T) {
	cacheDir := t.TempDir()
	assert.NoErr(t, os.MkdirAll(filepath.Join(cacheDir, "pkg-cache"), 0o755))

	assert.NoErr(t, validateCacheDirForMutation(cacheDir))
}

func TestServiceScanClassifiesEntries(t *testing.T) {
	cacheDir := t.TempDir()
	writeCacheTestFile(t, filepath.Join(cacheDir, "pkg.zip"), "pkg")
	writeCacheTestFile(t, filepath.Join(cacheDir, "api-cache", "repo.json"), "{}")
	writeCacheTestFile(t, filepath.Join(cacheDir, "sdk-downloads", "go", "1.22.0", "go.zip"), "sdk")
	writeCacheTestFile(t, filepath.Join(cacheDir, "sdk-index", "go.json"), "{}")
	writeCacheTestFile(t, filepath.Join(cacheDir, "pkg.zip.part"), "partial")
	writeCacheTestFile(t, filepath.Join(cacheDir, "pkg.zip.meta.json"), "{}")

	service := Service{}
	entries, err := service.Scan(cacheDir, CacheScanOptions{Kinds: []Kind{
		KindPkg,
		KindAPI,
		KindSDK,
		KindSDKIndex,
		KindPartial,
	}})

	assert.NoErr(t, err)
	got := map[string]Kind{}
	for _, entry := range entries {
		got[entry.RelPath] = entry.Kind
	}
	assert.Eq(t, KindPkg, got["pkg.zip"])
	assert.Eq(t, KindAPI, got["api-cache/repo.json"])
	assert.Eq(t, KindSDK, got["sdk-downloads/go/1.22.0/go.zip"])
	assert.Eq(t, KindSDKIndex, got["sdk-index/go.json"])
	assert.Eq(t, KindPartial, got["pkg.zip.part"])
	assert.Eq(t, KindPartial, got["pkg.zip.meta.json"])
}

func TestServiceDefaultCleanKindsExcludeSDKIndex(t *testing.T) {
	kinds := normalizeKinds(nil)

	assert.Eq(t, []Kind{KindPkg, KindAPI, KindSDK, KindPartial}, kinds)
}

func writeCacheTestFile(t *testing.T, path, body string) {
	t.Helper()
	assert.NoErr(t, os.MkdirAll(filepath.Dir(path), 0o755))
	assert.NoErr(t, os.WriteFile(path, []byte(body), 0o644))
}

func newCacheDirForCleanTest(t *testing.T) string {
	t.Helper()
	cacheDir := filepath.Join(t.TempDir(), "eget")
	assert.NoErr(t, os.MkdirAll(cacheDir, 0o755))
	return cacheDir
}

func TestServicePreviewCleanDoesNotRemoveFiles(t *testing.T) {
	cacheDir := newCacheDirForCleanTest(t)
	oldFile := filepath.Join(cacheDir, "old.zip")
	writeCacheTestFile(t, oldFile, "old")
	oldTime := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
	assert.NoErr(t, os.Chtimes(oldFile, oldTime, oldTime))

	service := Service{Now: func() time.Time {
		return time.Date(2026, 5, 26, 10, 0, 0, 0, time.UTC)
	}}
	result, err := service.PreviewClean(cacheDir, CleanOptions{Older: 3 * 24 * time.Hour})

	assert.NoErr(t, err)
	assert.Eq(t, 1, result.MatchedFiles)
	assert.Eq(t, 0, result.RemovedFiles)
	assert.True(t, fileExistsForTest(oldFile))
}

func TestServiceCleanRemovesOnlyOlderMatchedFiles(t *testing.T) {
	cacheDir := newCacheDirForCleanTest(t)
	oldFile := filepath.Join(cacheDir, "old.zip")
	newFile := filepath.Join(cacheDir, "new.zip")
	writeCacheTestFile(t, oldFile, "old")
	writeCacheTestFile(t, newFile, "new")
	oldTime := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
	newTime := time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC)
	assert.NoErr(t, os.Chtimes(oldFile, oldTime, oldTime))
	assert.NoErr(t, os.Chtimes(newFile, newTime, newTime))

	service := Service{Now: func() time.Time {
		return time.Date(2026, 5, 26, 10, 0, 0, 0, time.UTC)
	}}
	result, err := service.Clean(cacheDir, CleanOptions{Older: 3 * 24 * time.Hour})

	assert.NoErr(t, err)
	assert.Eq(t, 1, result.MatchedFiles)
	assert.Eq(t, 1, result.RemovedFiles)
	assert.False(t, fileExistsForTest(oldFile))
	assert.True(t, fileExistsForTest(newFile))
}

func TestServiceCleanAllIgnoresOlder(t *testing.T) {
	cacheDir := newCacheDirForCleanTest(t)
	file := filepath.Join(cacheDir, "new.zip")
	writeCacheTestFile(t, file, "new")

	service := Service{Now: func() time.Time {
		return time.Date(2026, 5, 26, 10, 0, 0, 0, time.UTC)
	}}
	result, err := service.Clean(cacheDir, CleanOptions{Older: 3 * 24 * time.Hour, All: true})

	assert.NoErr(t, err)
	assert.Eq(t, 1, result.RemovedFiles)
	assert.False(t, fileExistsForTest(file))
}

func TestServiceCleanDoesNotRemoveSDKIndexByDefault(t *testing.T) {
	cacheDir := newCacheDirForCleanTest(t)
	indexFile := filepath.Join(cacheDir, "sdk-index", "go.json")
	writeCacheTestFile(t, indexFile, "{}")
	oldTime := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
	assert.NoErr(t, os.Chtimes(indexFile, oldTime, oldTime))

	service := Service{Now: func() time.Time {
		return time.Date(2026, 5, 26, 10, 0, 0, 0, time.UTC)
	}}
	result, err := service.Clean(cacheDir, CleanOptions{Older: 3 * 24 * time.Hour})

	assert.NoErr(t, err)
	assert.Eq(t, 0, result.RemovedFiles)
	assert.True(t, fileExistsForTest(indexFile))
}

func TestServiceCleanRemovesSDKIndexWhenExplicit(t *testing.T) {
	cacheDir := newCacheDirForCleanTest(t)
	indexFile := filepath.Join(cacheDir, "sdk-index", "go.json")
	writeCacheTestFile(t, indexFile, "{}")

	service := Service{}
	result, err := service.Clean(cacheDir, CleanOptions{All: true, Kinds: []Kind{KindSDKIndex}})

	assert.NoErr(t, err)
	assert.Eq(t, 1, result.RemovedFiles)
	assert.False(t, fileExistsForTest(indexFile))
}

func TestServicePreviewCleanReportsLargeDeletionNeed(t *testing.T) {
	cacheDir := newCacheDirForCleanTest(t)
	for i := 0; i < 100; i++ {
		writeCacheTestFile(t, filepath.Join(cacheDir, fmt.Sprintf("pkg-%03d.zip", i)), "pkg")
	}

	service := Service{}
	result, err := service.PreviewClean(cacheDir, CleanOptions{All: true})

	assert.NoErr(t, err)
	assert.Eq(t, 100, result.MatchedFiles)
	assert.True(t, result.NeedsConfirmation())
	assert.True(t, fileExistsForTest(filepath.Join(cacheDir, "pkg-000.zip")))
}

func TestServicePreviewKeepLatestOnlyCountsPkgCacheFiles(t *testing.T) {
	cacheDir := newCacheDirForCleanTest(t)
	writeCacheTestFile(t, filepath.Join(cacheDir, "pkg-cache", "foo-1.0.0-a1b2c3d4.zip"), "old")
	writeCacheTestFile(t, filepath.Join(cacheDir, "pkg-cache", "foo-2.0.0-b1b2c3d4.zip"), "new")
	writeCacheTestFile(t, filepath.Join(cacheDir, "foo-0.9.0-c1b2c3d4.zip"), "root")
	writeCacheTestFile(t, filepath.Join(cacheDir, "misc", "foo-0.8.0-d1b2c3d4.zip"), "misc")
	writeCacheTestFile(t, filepath.Join(cacheDir, "pkg-cache", "manual.zip"), "manual")

	result, err := (Service{}).PreviewClean(cacheDir, CleanOptions{KeepLatest: true})
	assert.NoErr(t, err)
	assert.Eq(t, 1, result.MatchedFiles)
	assert.Eq(t, 1, result.KeptLatestFiles)
	assert.Eq(t, 1, result.UnrecognizedFiles)
}

func TestServiceApplyCleanUsesPreviewSnapshot(t *testing.T) {
	t.Run("does not add files created after preview", func(t *testing.T) {
		cacheDir := newCacheDirForCleanTest(t)
		oldFile := filepath.Join(cacheDir, "pkg-cache", "foo-1.0.0-a1b2c3d4.zip")
		newFile := filepath.Join(cacheDir, "pkg-cache", "foo-2.0.0-b1b2c3d4.zip")
		lateFile := filepath.Join(cacheDir, "pkg-cache", "foo-0.5.0-c1b2c3d4.zip")
		writeCacheTestFile(t, oldFile, "old")
		writeCacheTestFile(t, newFile, "new")

		service := Service{}
		preview, err := service.PreviewClean(cacheDir, CleanOptions{KeepLatest: true})
		assert.NoErr(t, err)
		writeCacheTestFile(t, lateFile, "late")
		result, err := service.ApplyClean(preview)
		assert.NoErr(t, err)
		assert.Eq(t, 1, result.RemovedFiles)
		assert.True(t, fileExistsForTest(lateFile))
	})

	t.Run("skips files changed after preview", func(t *testing.T) {
		cacheDir := newCacheDirForCleanTest(t)
		oldFile := filepath.Join(cacheDir, "pkg-cache", "foo-1.0.0-a1b2c3d4.zip")
		writeCacheTestFile(t, oldFile, "old")
		writeCacheTestFile(t, filepath.Join(cacheDir, "pkg-cache", "foo-2.0.0-b1b2c3d4.zip"), "new")

		service := Service{}
		preview, err := service.PreviewClean(cacheDir, CleanOptions{KeepLatest: true})
		assert.NoErr(t, err)
		writeCacheTestFile(t, oldFile, "changed-size")
		result, err := service.ApplyClean(preview)
		assert.NoErr(t, err)
		assert.Eq(t, 0, result.RemovedFiles)
		assert.Eq(t, 1, len(result.Skipped))
		assert.Eq(t, "changed since preview", result.Skipped[0].Reason)
		assert.True(t, fileExistsForTest(oldFile))
	})
}

func TestServicePreviewKeepLatestExcludesSymlinks(t *testing.T) {
	cacheDir := newCacheDirForCleanTest(t)
	target := filepath.Join(cacheDir, "target.zip")
	link := filepath.Join(cacheDir, "pkg-cache", "foo-1.0.0-a1b2c3d4.zip")
	writeCacheTestFile(t, target, "target")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	writeCacheTestFile(t, filepath.Join(cacheDir, "pkg-cache", "foo-2.0.0-b1b2c3d4.zip"), "new")

	result, err := (Service{}).PreviewClean(cacheDir, CleanOptions{KeepLatest: true})
	assert.NoErr(t, err)
	assert.Eq(t, 0, result.MatchedFiles)
	assert.Eq(t, 1, result.KeptLatestFiles)
	assert.Eq(t, 0, result.UnrecognizedFiles)
}

func TestCleanResultJSONUsesSnakeCaseFields(t *testing.T) {
	data, err := json.Marshal(CleanResult{
		CacheDir: "cache", MatchedFiles: 1, MatchedSize: 3,
		KeptLatestFiles: 2, UnrecognizedFiles: 1,
		Skipped: []CleanSkip{{Path: "bad", Reason: "locked"}},
	})

	assert.NoErr(t, err)
	var got map[string]any
	assert.NoErr(t, json.Unmarshal(data, &got))
	assert.Eq(t, "cache", got["cache_dir"])
	assert.Eq(t, float64(1), got["matched_files"])
	assert.Eq(t, float64(0), got["removed_files"])
	assert.Eq(t, float64(2), got["kept_latest_files"])
	assert.Eq(t, float64(1), got["unrecognized_files"])
	assert.NotContains(t, string(data), "snapshot")
}

func fileExistsForTest(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
