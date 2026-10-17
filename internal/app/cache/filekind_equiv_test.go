package cache

import (
	"testing"

	"github.com/gookit/goutil/x/assert"
)

// TestEquivS_P09 pins trimPlatformSuffix, which now reads the shared platform
// token tables (docs/plans/2026-10-17-rule-consolidation-plan.md).
func TestEquivS_P09(t *testing.T) {
	t.Run("single os-arch suffix", func(t *testing.T) {
		name, ok := trimPlatformSuffix("tool-linux-amd64", "-")
		assert.True(t, ok)
		assert.Eq(t, "tool", name)

		name, ok = trimPlatformSuffix("TOOL-LINUX-AMD64", "-")
		assert.True(t, ok)
		assert.Eq(t, "TOOL", name)
	})

	t.Run("aliases are recognised", func(t *testing.T) {
		name, ok := trimPlatformSuffix("tool-windows-x64", "-")
		assert.True(t, ok)
		assert.Eq(t, "tool", name)

		name, ok = trimPlatformSuffix("tool-darwin-arm64", "-")
		assert.True(t, ok)
		assert.Eq(t, "tool", name)
	})

	t.Run("only a continuous suffix, with the same separator", func(t *testing.T) {
		_, ok := trimPlatformSuffix("tool_linux_amd64", "-")
		assert.False(t, ok)
		_, ok = trimPlatformSuffix("tool-amd64", "-")
		assert.False(t, ok)
		_, ok = trimPlatformSuffix("tool-linux-amd64-extra", "-")
		assert.False(t, ok)
	})
}

// TestEquivS_A05 pins cacheAssetExt before it starts consuming
// internal/filekind (docs/plans/2026-10-17-rule-consolidation-plan.md): folded
// comparison, the matched slice keeps the original case, otherwise path.Ext.
func TestEquivS_A05(t *testing.T) {
	assert.Eq(t, ".tar.gz", cacheAssetExt("tool_1.0_linux_x64.tar.gz"))
	assert.Eq(t, ".tar.xz", cacheAssetExt("tool_1.0_linux_x64.tar.xz"))
	assert.Eq(t, ".tar.bz2", cacheAssetExt("tool_1.0_linux_x64.tar.bz2"))
	assert.Eq(t, ".tar.zst", cacheAssetExt("tool_1.0_linux_x64.tar.zst"))
	assert.Eq(t, ".tar.br", cacheAssetExt("tool_1.0_linux_x64.tar.br"))
	assert.Eq(t, ".tar.lz4", cacheAssetExt("tool_1.0_linux_x64.tar.lz4"))

	// Folded axis keeps the original case of the input.
	assert.Eq(t, ".TAR.LZ4", cacheAssetExt("TOOL-x64.TAR.LZ4"))

	assert.Eq(t, ".zip", cacheAssetExt("tool.zip"))
	assert.Eq(t, "", cacheAssetExt("tool"))
}
