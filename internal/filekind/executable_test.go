package filekind

import (
	"testing"

	"github.com/gookit/goutil/x/assert"
)

func TestIsExecutableExact(t *testing.T) {
	t.Run("case sensitive", func(t *testing.T) {
		assert.True(t, IsExecutableExact("tool.exe"))
		assert.True(t, IsExecutableExact("tool.appimage"))
		assert.False(t, IsExecutableExact("TOOL.EXE"))
		assert.False(t, IsExecutableExact("TOOL.APPIMAGE"))
		assert.False(t, IsExecutableExact("tool.msi"))
		assert.False(t, IsExecutableExact("tool.zip"))
	})
}

func TestIsExecutableFold(t *testing.T) {
	t.Run("folded", func(t *testing.T) {
		assert.True(t, IsExecutableFold("tool.exe"))
		assert.True(t, IsExecutableFold("TOOL.EXE"))
		assert.True(t, IsExecutableFold("tool.AppImage"))
		assert.False(t, IsExecutableFold("tool.msi"))
		assert.False(t, IsExecutableFold("tool.zip"))
	})
}

func TestIsNonExecutableExact(t *testing.T) {
	t.Run("case sensitive", func(t *testing.T) {
		assert.True(t, IsNonExecutableExact("pkg.deb"))
		assert.True(t, IsNonExecutableExact("man.1"))
		assert.True(t, IsNonExecutableExact("notes.txt"))
		assert.False(t, IsNonExecutableExact("PKG.DEB"))
		assert.False(t, IsNonExecutableExact("tool.exe"))
	})
}

func TestExecutableSuffixFold(t *testing.T) {
	t.Run("returns the matched suffix", func(t *testing.T) {
		assert.Eq(t, ".exe", ExecutableSuffixFold("tool_1.0_linux_x64.exe"))
		assert.Eq(t, ".exe", ExecutableSuffixFold("TOOL.EXE"))
		assert.Eq(t, ".appimage", ExecutableSuffixFold("TOOL.APPIMAGE"))
		assert.Eq(t, "", ExecutableSuffixFold("tool.zip"))
		assert.Eq(t, "", ExecutableSuffixFold("tool"))
	})
}

func TestIsInstallerFold(t *testing.T) {
	t.Run("folded", func(t *testing.T) {
		assert.True(t, IsInstallerFold("setup.exe"))
		assert.True(t, IsInstallerFold("SETUP.MSI"))
		assert.False(t, IsInstallerFold("setup.appimage"))
	})
}

func TestRequiresSystemArchiverFold(t *testing.T) {
	t.Run("unconditional suffixes only", func(t *testing.T) {
		for _, name := range []string{"a.7z", "a.rar", "a.msi", "a.cab", "a.iso"} {
			assert.True(t, RequiresSystemArchiverFold(name), name)
		}
		assert.True(t, RequiresSystemArchiverFold("A.CAB"))
		// .exe is conditional at the call site (extractArchive), never here.
		assert.False(t, RequiresSystemArchiverFold("a.exe"))
		assert.False(t, RequiresSystemArchiverFold("a.tar.gz"))
	})
}
