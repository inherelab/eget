package platform

import (
	"testing"

	"github.com/gookit/goutil/x/assert"
)

func TestTokens(t *testing.T) {
	t.Run("os tokens", func(t *testing.T) {
		assert.Eq(t, []string{"windows", "win", "win32", "win64", "darwin", "macos", "osx", "linux",
			"freebsd", "openbsd", "netbsd", "android", "illumos", "solaris", "plan9"}, OSTokens())
	})

	t.Run("arch tokens", func(t *testing.T) {
		assert.Eq(t, []string{"amd64", "x86_64", "x64", "386", "x86", "i386", "arm64", "aarch64",
			"arm32", "armv6", "armv7", "arm", "riscv64"}, ArchTokens())
	})

	t.Run("variant tokens", func(t *testing.T) {
		assert.Eq(t, []string{"portable"}, VariantTokens())
	})
}

func TestSplitTokens(t *testing.T) {
	t.Run("normalises case and separators with two passes", func(t *testing.T) {
		// The first pass keeps "_" inside tokens, the second splits on it; both
		// passes are concatenated, exactly as the original platformTokens did.
		want := []string{"tool_1", "0", "linux", "x64", "tool", "1", "0", "linux", "x64"}
		assert.Eq(t, want, SplitTokens("TOOL_1.0-Linux/x64"))
		assert.Eq(t, []string{"dir", "tool_1", "0", "linux", "x64", "dir", "tool", "1", "0", "linux", "x64"},
			SplitTokens(`dir\TOOL_1.0-Linux/x64`))
	})
}

func TestHasAnyToken(t *testing.T) {
	tokens := SplitTokens("tool_1.0_linux_x64")
	assert.True(t, HasAnyToken(tokens, "linux"))
	assert.True(t, HasAnyToken(tokens, "darwin", "x64"))
	assert.False(t, HasAnyToken(tokens, "windows"))
	assert.False(t, HasAnyToken(nil, "linux"))
}

func TestAliases(t *testing.T) {
	cases := []struct {
		goos, goarch string
		wantOS       []string
		wantArch     []string
	}{
		{"windows", "amd64", []string{"windows", "win", "win32", "win64"}, []string{"amd64", "x86_64", "x64"}},
		{"Windows", "AMD64", []string{"windows", "win", "win32", "win64"}, []string{"amd64", "x86_64", "x64"}},
		{"darwin", "386", []string{"darwin", "macos", "osx"}, []string{"386", "x86", "i386"}},
		{"linux", "arm64", []string{"linux"}, []string{"arm64", "aarch64"}},
		{"freebsd", "arm", []string{"freebsd"}, []string{"arm32", "armv6", "armv7", "arm"}},
		{"plan9", "riscv64", []string{"plan9"}, []string{"riscv64"}},
	}
	for _, tc := range cases {
		t.Run(tc.goos+"/"+tc.goarch, func(t *testing.T) {
			osAliases, archAliases := Aliases(tc.goos, tc.goarch)
			assert.Eq(t, tc.wantOS, osAliases)
			assert.Eq(t, tc.wantArch, archAliases)
		})
	}
}
