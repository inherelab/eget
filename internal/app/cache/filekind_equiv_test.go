package cache

import (
	"testing"

	"github.com/gookit/goutil/x/assert"
)

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
