package cache

import (
	"path/filepath"
	"testing"

	"github.com/gookit/goutil/x/assert"
)

func TestParseAssetVersion(t *testing.T) {
	tests := []struct {
		input string
		ok    bool
	}{
		{"1.2.3", true},
		{"v1.2.3", true},
		{"2026.7.17", true},
		{"2.0.0-beta.1", true},
		{"7.6.3-preview.4", true},
		{"1", false},
		{"1.02.3", false},
		{"1.2.3-beta.01", false},
		{"1.2.3-beta-1", false},
		{"1.2.3-beta_1", false},
		{"1.2.3-build.1", false},
		{"1.2.3-BUILD.1", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, ok := parseAssetVersion(tt.input)
			assert.Eq(t, tt.ok, ok)
		})
	}
}

func TestCompareAssetVersion(t *testing.T) {
	assertVersionOrder(t, "1.9.9", "1.10.0")
	assertVersionEqual(t, "2.0", "2.0.0")
	assertVersionEqual(t, "2.0.0", "v2.0.0")
	assertVersionOrder(t, "2.0.0-beta.2", "2.0.0-beta.10")
	assertVersionOrder(t, "2.0.0-alpha", "2.0.0-beta")
	assertVersionOrder(t, "2.0.0-beta", "2.0.0-beta.0")
	assertVersionOrder(t, "2.0.0-Beta.1", "2.0.0-beta.1")
	assertVersionOrder(t, "2.0.0-preview.9", "2.0.0-rc.1")
}

func TestParseKeepLatestEntry(t *testing.T) {
	tests := []struct {
		name, rel, family, version string
		ok                         bool
	}{
		{"tool generic", "pkg-cache/tool-v2.4.1-linux-amd64-2.4.1-a1b2c3d4.zip", "", "", false},
		{"gomi", "pkg-cache/gomi_Linux_x86_64-1.6.3-a1b2c3d4.tar.gz", "gomi", "1.6.3", true},
		{"claude appended platform", "pkg-cache/claude-2.1.160-linux-amd64-a1b2c3d4.bin", "claude", "2.1.160", true},
		{"powershell", "pkg-cache/PowerShell-7.6.3-win-x64-7.6.3-a1b2c3d4.msi", "powershell", "7.6.3", true},
		{"cscli", "pkg-cache/cscli-windows-amd64-0.5.2-a1b2c3d4.exe", "cscli", "0.5.2", true},
		{"rightmost duplicate", "pkg-cache/foo-1.2.3-1.2.3-a1b2c3d4.zip", "foo", "1.2.3", true},
		{"rightmost different", "pkg-cache/foo-1.2.3-2.0.0-a1b2c3d4.zip", "foo", "2.0.0", true},
		{"unknown tuple", "pkg-cache/foo-1.2.3-haiku-mips-a1b2c3d4.zip", "", "", false},
		{"uppercase hash", "pkg-cache/foo-1.2.3-A1B2C3D4.zip", "", "", false},
		{"root file", "foo-1.2.3-a1b2c3d4.zip", "", "", false},
		{"misc file", "misc/foo-1.2.3-a1b2c3d4.zip", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseKeepLatestEntry(Entry{Kind: KindPkg, RelPath: tt.rel})
			assert.Eq(t, tt.ok, ok)
			if ok {
				assert.Eq(t, tt.family, got.family)
				assert.Eq(t, tt.version, got.rawVer)
			}
		})
	}
}

func TestNormalizeAssetFamilyKeepsProductTokens(t *testing.T) {
	windowsTerminal, ok := normalizeAssetFamily("windows-terminal", "1.0.0")
	assert.True(t, ok)
	terminal, ok := normalizeAssetFamily("terminal", "1.0.0")
	assert.True(t, ok)
	assert.Neq(t, windowsTerminal, terminal)

	for _, name := range []string{"go", "fd", "jq"} {
		family, ok := normalizeAssetFamily(name, "1.0.0")
		assert.True(t, ok)
		assert.Eq(t, name, family)
	}
	_, ok = normalizeAssetFamily("tool", "1.0.0")
	assert.False(t, ok)
}

func assertVersionOrder(t *testing.T, lower, higher string) {
	t.Helper()
	a, ok := parseAssetVersion(lower)
	assert.True(t, ok)
	b, ok := parseAssetVersion(higher)
	assert.True(t, ok)
	assert.Eq(t, -1, compareAssetVersion(a, b))
}

func assertVersionEqual(t *testing.T, left, right string) {
	t.Helper()
	a, ok := parseAssetVersion(left)
	assert.True(t, ok)
	b, ok := parseAssetVersion(right)
	assert.True(t, ok)
	assert.Eq(t, 0, compareAssetVersion(a, b))
}

func testKeepLatestEntry(rel string) Entry {
	return Entry{Kind: KindPkg, RelPath: rel, Path: filepath.FromSlash(rel), Size: 1}
}
