package platform

import (
	"testing"

	"github.com/gookit/goutil/x/assert"
)

func TestDirArchPatterns(t *testing.T) {
	cases := []struct {
		goarch string
		path   string
		want   bool
	}{
		{"amd64", "tool/x64/bin", true},
		{"amd64", "tool/AMD64/bin", false}, // patterns are not case-insensitive
		{"amd64", "tool/x86_64/bin", true},
		{"amd64", "tool/arm64/bin", false},
		{"386", "tool/i386/bin", true},
		{"arm64", "tool/aarch64/bin", true},
		{"arm", "tool/armv7/bin", true},
		{"arm", "tool/arm32/bin", true},
	}
	for _, tc := range cases {
		t.Run(tc.goarch+"/"+tc.path, func(t *testing.T) {
			patterns := DirArchPatterns(tc.goarch)
			assert.NotEmpty(t, patterns)
			matched := false
			for _, re := range patterns {
				if re.MatchString(tc.path) {
					matched = true
				}
			}
			assert.Eq(t, tc.want, matched)
		})
	}

	t.Run("unknown arch", func(t *testing.T) {
		assert.Nil(t, DirArchPatterns("mips64"))
	})
}

// TestTrimPlatformSuffixRegexps pins the three patterns verbatim: they must not
// be rebuilt from the token tables (armv?6/armv?7 cannot be expressed by them).
func TestTrimPlatformSuffixRegexps(t *testing.T) {
	want := []string{
		`(?i)[-_.](windows|darwin|linux|freebsd|openbsd|netbsd|android|illumos|solaris|plan9|macos|osx)[-_.](amd64|x86_64|x64|386|x86|i386|arm64|aarch64|armv?6|armv?7|arm|riscv64)$`,
		`(?i)[-_.](amd64|x86_64|x64|386|x86|i386|arm64|aarch64|armv?6|armv?7|arm|riscv64)[-_.](windows|darwin|linux|freebsd|openbsd|netbsd|android|illumos|solaris|plan9|macos|osx)$`,
		`(?i)[-_.](windows|darwin|linux|freebsd|openbsd|netbsd|android|illumos|solaris|plan9|macos|osx)$`,
	}

	patterns := TrimPlatformSuffixRegexps()
	assert.Eq(t, len(want), len(patterns))
	for i, re := range patterns {
		assert.Eq(t, want[i], re.String())
	}

	t.Run("os-arch suffix", func(t *testing.T) {
		assert.Eq(t, "tool_1.0", patterns[0].ReplaceAllString("tool_1.0_linux_amd64", ""))
		assert.Eq(t, "TOOL_1.0", patterns[0].ReplaceAllString("TOOL_1.0_LINUX_AMD64", ""))
		assert.Eq(t, "tool_1.0", patterns[0].ReplaceAllString("tool_1.0_linux_armv7", ""))
	})

	t.Run("arch-os suffix", func(t *testing.T) {
		assert.Eq(t, "tool_1.0", patterns[1].ReplaceAllString("tool_1.0_amd64_linux", ""))
	})

	t.Run("bare os suffix", func(t *testing.T) {
		assert.Eq(t, "tool", patterns[2].ReplaceAllString("tool-linux", ""))
	})
}

func TestDetectSpecIndexes(t *testing.T) {
	t.Run("os index keys", func(t *testing.T) {
		assert.Eq(t, 10, len(GOOSSpecIndex))
		for _, name := range []string{"darwin", "windows", "linux", "netbsd", "freebsd",
			"openbsd", "android", "illumos", "solaris", "plan9"} {
			_, ok := GOOSSpecIndex[name]
			assert.True(t, ok, name)
		}
	})

	t.Run("arch index keys", func(t *testing.T) {
		assert.Eq(t, 5, len(GOARCHSpecIndex))
		for _, name := range []string{"amd64", "386", "arm", "arm64", "riscv64"} {
			_, ok := GOARCHSpecIndex[name]
			assert.True(t, ok, name)
		}
	})

	t.Run("spec fields are kept verbatim", func(t *testing.T) {
		win := GOOSSpecIndex["windows"]
		assert.Eq(t, `(?i)([^r]win|windows|\.exe$|\.msi$)`, win.Regex.String())
		assert.Eq(t, `(?i)\.exe$`, win.Priority.String())
		assert.Nil(t, win.Anti)

		linux := GOOSSpecIndex["linux"]
		assert.Eq(t, `(?i)(android)`, linux.Anti.String())
		assert.Eq(t, `\.appimage$`, linux.Priority.String())

		assert.Eq(t, `(?i)(x64|amd64|x86(-|_)?64)`, GOARCHSpecIndex["amd64"].Regex.String())
	})
}
