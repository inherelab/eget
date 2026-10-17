package client

import (
	"testing"

	"github.com/gookit/goutil/x/assert"
)

// TestEquivS_P08 pins the cache platform detection that now reads the shared
// platform token tables (docs/plans/2026-10-17-rule-consolidation-plan.md).
func TestEquivS_P08(t *testing.T) {
	t.Run("nameHasPlatform", func(t *testing.T) {
		assert.True(t, nameHasPlatform("tool_linux_amd64"))
		assert.True(t, nameHasPlatform("tool_amd64"))
		assert.True(t, nameHasPlatform("tool-Windows-x64"))
		assert.False(t, nameHasPlatform("tool"))
	})

	t.Run("cachePlatform only appends when the name carries none", func(t *testing.T) {
		assert.Eq(t, "", cachePlatform("tool_linux_amd64", "linux", "amd64"))
		assert.Eq(t, "", cachePlatform("tool_amd64", "linux", "amd64"))
		assert.Eq(t, "linux-amd64", cachePlatform("tool", "linux", "amd64"))
		assert.Eq(t, "", cachePlatform("tool", "", "amd64"))
	})
}

// TestEquivS_A04 pins archiveExt before it starts consuming internal/filekind
// (docs/plans/2026-10-17-rule-consolidation-plan.md): folded comparison, the
// matched slice keeps the original case, otherwise path.Ext.
func TestEquivS_A04(t *testing.T) {
	assert.Eq(t, ".tar.gz", archiveExt("tool_1.0_linux_x64.tar.gz"))
	assert.Eq(t, ".tar.xz", archiveExt("tool_1.0_linux_x64.tar.xz"))
	assert.Eq(t, ".tar.bz2", archiveExt("tool_1.0_linux_x64.tar.bz2"))
	assert.Eq(t, ".tar.zst", archiveExt("tool_1.0_linux_x64.tar.zst"))
	assert.Eq(t, ".tar.br", archiveExt("tool_1.0_linux_x64.tar.br"))
	assert.Eq(t, ".tar.lz4", archiveExt("tool_1.0_linux_x64.tar.lz4"))

	// Folded axis keeps the original case of the input.
	assert.Eq(t, ".TAR.BR", archiveExt("TOOL-x64.TAR.BR"))

	assert.Eq(t, ".zip", archiveExt("tool.zip"))
	assert.Eq(t, "", archiveExt("tool"))
}
