package install

import (
	"testing"

	"github.com/gookit/goutil/x/assert"
)

// Platform-station guards (docs/plans/2026-10-17-rule-consolidation-plan.md,
// S-P10/S-P01/S-P02/S-P11): the OS/arch data now comes from internal/platform,
// these assertions pin the behaviour the consumers rely on.

// TestEquivS_P10 pins the executable-name trimming that consumes
// platform.TrimPlatformSuffixRegexps.
func TestEquivS_P10(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"tool_1.0_linux_amd64.exe", "tool_1.0.exe"},
		{"tool_1.0_linux_armv6.exe", "tool_1.0.exe"},
		{"tool_1.0_linux_armv7.exe", "tool_1.0.exe"},
		{"tool_1.0_linux_arm.exe", "tool_1.0.exe"},
		{"tool_1.0_amd64_linux.exe", "tool_1.0.exe"},
		{"tool_1.0_linux.exe", "tool_1.0.exe"},
		{"tool-linux.appimage", "tool.appimage"},
		{"tool_1.0_darwin_arm64.appimage", "tool_1.0.appimage"},
		// Mixed axis quirk: the suffix probe folds case, the trim does not.
		{"TOOL_1.0_LINUX_AMD64.EXE", "TOOL_1.0_LINUX_AMD64.EXE"},
		// Nothing platform-like to strip.
		{"tool.exe", "tool.exe"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Eq(t, tc.want, heuristicExecutableName(tc.name))
		})
	}
}

// TestEquivS_P01 lives in internal/install/detect (the detector's own package).

// TestEquivS_P03 pins archiveNameMatchesPlatform (platform.OSTokens/ArchTokens +
// platform.Aliases).
func TestEquivS_P03(t *testing.T) {
	assert.True(t, archiveNameMatchesPlatform("tool_linux_amd64.tar.gz", "linux", "amd64"))
	assert.True(t, archiveNameMatchesPlatform("tool_linux_x64.tar.gz", "linux", "amd64"))
	assert.False(t, archiveNameMatchesPlatform("tool_windows_amd64.zip", "linux", "amd64"))
	assert.False(t, archiveNameMatchesPlatform("tool_linux_arm64.tar.gz", "linux", "amd64"))
	assert.True(t, archiveNameMatchesPlatform("tool_linux.tar.gz", "linux", "amd64"))
	assert.True(t, archiveNameMatchesPlatform("tool.tar.gz", "linux", "amd64"))
}

// TestEquivS_P04 pins autoSelectWindowsMSVCAsset (platform token splitting plus
// platform.Aliases).
func TestEquivS_P04(t *testing.T) {
	// Selection requires both a gnu candidate and exactly one msvc candidate.
	got, ok := autoSelectWindowsMSVCAsset([]string{"tool_windows_amd64_gnu.zip", "tool_windows_amd64_msvc.zip"}, "windows", "amd64")
	assert.True(t, ok)
	assert.Eq(t, "tool_windows_amd64_msvc.zip", got)

	// An msvc-only list is not enough.
	_, ok = autoSelectWindowsMSVCAsset([]string{"tool_windows_amd64_msvc.zip"}, "windows", "amd64")
	assert.False(t, ok)

	// Two msvc candidates are ambiguous.
	_, ok = autoSelectWindowsMSVCAsset([]string{"tool_windows_amd64_gnu.zip", "tool_windows_amd64_msvc.zip", "other_windows_amd64_msvc.zip"}, "windows", "amd64")
	assert.False(t, ok)

	// A candidate for another platform disqualifies the whole list.
	_, ok = autoSelectWindowsMSVCAsset([]string{"tool_linux_amd64_msvc.zip"}, "windows", "amd64")
	assert.False(t, ok)
}
