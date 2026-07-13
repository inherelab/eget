package client

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gookit/goutil/x/assert"
)

func TestCacheFilePathUsesReadableAssetNameVersionAndShortHash(t *testing.T) {
	cacheDir := t.TempDir()
	got := CacheFilePath(cacheDir, "https://github.com/babarot/gomi/releases/download/v1.6.3/gomi_Linux_x86_64.tar.gz")

	assert.Eq(t, filepath.Join(cacheDir, "pkg-cache"), filepath.Dir(got))
	base := filepath.Base(got)
	assert.True(t, strings.HasPrefix(base, "gomi_Linux_x86_64-1.6.3-"))
	assert.True(t, strings.HasSuffix(base, ".tar.gz"))
	shortHash := strings.TrimSuffix(strings.TrimPrefix(base, "gomi_Linux_x86_64-1.6.3-"), ".tar.gz")
	assert.Eq(t, 8, len(shortHash))
}

func TestCacheFilePathStoresPackagesUnderPkgCache(t *testing.T) {
	cacheDir := t.TempDir()
	got := CacheFilePathWithMeta(cacheDir, "https://example.com/download?id=123", CacheMeta{Name: "tool"})

	assert.Eq(t, filepath.Join(cacheDir, "pkg-cache"), filepath.Dir(got))
}

func TestCacheFilePathFallsBackToVersionFromFilename(t *testing.T) {
	got := CacheFilePath(t.TempDir(), "https://example.com/releases/tool-v2.4.1-linux-amd64.zip")
	base := filepath.Base(got)

	assert.True(t, strings.HasPrefix(base, "tool-v2.4.1-linux-amd64-2.4.1-"))
	assert.True(t, strings.HasSuffix(base, ".zip"))
}

func TestCacheFilePathWithMetaUsesNameAndVersionFallbacks(t *testing.T) {
	got := CacheFilePathWithMeta(t.TempDir(), "https://example.com/download?id=123", CacheMeta{
		Name:    "gomi",
		Version: "v1.6.3",
	})
	base := filepath.Base(got)

	assert.True(t, strings.HasPrefix(base, "gomi-1.6.3-"))
	assert.True(t, strings.HasSuffix(base, ".bin"))
}

func TestCacheFilePathWithMetaAddsPlatformWhenNameHasNoPlatform(t *testing.T) {
	got := CacheFilePathWithMeta(t.TempDir(), "https://example.com/download?id=123", CacheMeta{
		Name:    "claude",
		Version: "2.1.160",
		OS:      "linux",
		Arch:    "amd64",
	})
	base := filepath.Base(got)

	assert.True(t, strings.HasPrefix(base, "claude-2.1.160-linux-amd64-"))
	assert.True(t, strings.HasSuffix(base, ".bin"))
}

func TestCacheFilePathWithMetaDoesNotDuplicatePlatformFromName(t *testing.T) {
	got := CacheFilePathWithMeta(t.TempDir(), "https://downloads.example.com/gomi_Linux_x86_64.tar.gz", CacheMeta{
		Name:    "gomi",
		Version: "v1.6.3",
		OS:      "linux",
		Arch:    "amd64",
	})
	base := filepath.Base(got)

	assert.True(t, strings.HasPrefix(base, "gomi_Linux_x86_64-1.6.3-"))
	assert.False(t, strings.Contains(base, "linux-amd64"))
	assert.True(t, strings.HasSuffix(base, ".tar.gz"))
}

func TestCacheFilePathWithMetaKeepsAssetNameAndUsesMetaVersion(t *testing.T) {
	got := CacheFilePathWithMeta(t.TempDir(), "https://downloads.example.com/files/tool-linux-amd64.tar.gz", CacheMeta{
		Name:    "tool",
		Version: "v2.0.0",
	})
	base := filepath.Base(got)

	assert.True(t, strings.HasPrefix(base, "tool-linux-amd64-2.0.0-"))
	assert.True(t, strings.HasSuffix(base, ".tar.gz"))
}

func TestCacheFilePathWithMetaPrefersVersionOverIPAddress(t *testing.T) {
	got := CacheFilePathWithMeta(t.TempDir(), "http://172.20.0.6:5000/tools/cscli/cscli-windows-amd64.exe", CacheMeta{Version: "0.5.2"})

	assert.Eq(t, "cscli-windows-amd64-0.5.2-b913cc7a.exe", filepath.Base(got))
}

func TestCacheFilePathSanitizesVersionWithPathSeparators(t *testing.T) {
	got := CacheFilePath(t.TempDir(), "https://github.com/example/tool/releases/download/release%2Fv2.5.0/tool.tar.gz")
	base := filepath.Base(got)

	assert.True(t, strings.HasPrefix(base, "tool-release-v2.5.0-"))
	assert.True(t, strings.HasSuffix(base, ".tar.gz"))
}

func TestAPICacheFilePathUsesReadableEndpointAndShortHash(t *testing.T) {
	cacheDir := t.TempDir()
	got := APICacheFilePath(cacheDir, "https://api.github.com/repos/babarot/gomi/releases/latest")

	assert.Eq(t, cacheDir, filepath.Dir(got))
	base := filepath.Base(got)
	assert.True(t, strings.HasPrefix(base, "api.github.com-repos-babarot-gomi-releases-latest-"))
	assert.True(t, strings.HasSuffix(base, ".json"))
	shortHash := strings.TrimSuffix(strings.TrimPrefix(base, "api.github.com-repos-babarot-gomi-releases-latest-"), ".json")
	assert.Eq(t, 8, len(shortHash))
}
