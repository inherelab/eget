package detect

import (
	"testing"

	"github.com/gookit/goutil/x/assert"
)

// Platform-station guards (docs/plans/2026-10-17-rule-consolidation-plan.md,
// S-P01/S-P02/S-P11): the recognition table now comes from internal/platform.

func TestEquivS_P01DetectSpecs(t *testing.T) {
	t.Run("windows priority on exe", func(t *testing.T) {
		d, err := NewSystemDetector("windows", "amd64")
		assert.NoErr(t, err)
		got, _, err := d.Detect([]string{"tool_windows_x86_64.zip", "tool_windows_x86_64.exe"})
		assert.NoErr(t, err)
		assert.Eq(t, "tool_windows_x86_64.exe", got)
	})

	t.Run("linux anti excludes android", func(t *testing.T) {
		d, err := NewSystemDetector("linux", "amd64")
		assert.NoErr(t, err)
		got, _, err := d.Detect([]string{"tool_android_amd64.tar.gz", "tool_linux_amd64.tar.gz"})
		assert.NoErr(t, err)
		assert.Eq(t, "tool_linux_amd64.tar.gz", got)
	})

	t.Run("linux appimage priority is case sensitive", func(t *testing.T) {
		d, err := NewSystemDetector("linux", "amd64")
		assert.NoErr(t, err)
		got, _, err := d.Detect([]string{"tool_LINUX_AMD64.tar.gz", "tool_LINUX_AMD64.appimage"})
		assert.NoErr(t, err)
		assert.Eq(t, "tool_LINUX_AMD64.appimage", got)

		// Upper-case .AppImage does not hit the priority pattern.
		d2, err := NewSystemDetector("linux", "amd64")
		assert.NoErr(t, err)
		_, _, err = d2.Detect([]string{"tool_LINUX_AMD64.tar.gz", "tool_LINUX_AMD64.AppImage"})
		assert.Err(t, err)
	})

	t.Run("arch aliases", func(t *testing.T) {
		d, err := NewSystemDetector("linux", "arm64")
		assert.NoErr(t, err)
		got, _, err := d.Detect([]string{"tool_linux_aarch64.tar.gz"})
		assert.NoErr(t, err)
		assert.Eq(t, "tool_linux_aarch64.tar.gz", got)
	})
}

func TestEquivS_P02KnownTargets(t *testing.T) {
	for _, goos := range []string{"darwin", "windows", "linux", "netbsd", "freebsd",
		"openbsd", "android", "illumos", "solaris", "plan9"} {
		assert.True(t, IsKnownGOOS(goos), goos)
	}
	assert.False(t, IsKnownGOOS("windows7"))

	_, err := NewSystemDetector("linux", "mips64")
	assert.Err(t, err)
	_, err = NewSystemDetector("haiku", "amd64")
	assert.Err(t, err)
}
