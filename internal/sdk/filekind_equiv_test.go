package sdk

import (
	"testing"

	"github.com/gookit/goutil/x/assert"
)

// The tests below pin the sdk index suffix stations before they start consuming
// internal/filekind (docs/plans/2026-10-17-rule-consolidation-plan.md, S-A06/
// S-A07/S-A08). They must keep passing unchanged after the migration.

// TestNodeFileDescriptorMatchesUpstream replaces the earlier S-A06
// characterization: that test pinned the old parser (only "-tar.xz", "-tar.gz"
// and "-zip"), which did not match nodejs.org/dist/index.json at all — see
// my-tools-ycv1. The descriptors below are verbatim samples from the real index.
func TestNodeFileDescriptorMatchesUpstream(t *testing.T) {
	cases := []struct {
		desc string
		want []nodeDescriptor
	}{
		{"linux-x64", []nodeDescriptor{{os: "linux", arch: "x64", ext: "tar.gz"}, {os: "linux", arch: "x64", ext: "tar.xz"}}},
		{"linux-x64-musl", []nodeDescriptor{{os: "linux", arch: "x64-musl", ext: "tar.gz"}, {os: "linux", arch: "x64-musl", ext: "tar.xz"}}},
		{"linux-armv7l", []nodeDescriptor{{os: "linux", arch: "armv7l", ext: "tar.gz"}, {os: "linux", arch: "armv7l", ext: "tar.xz"}}},
		{"aix-ppc64", []nodeDescriptor{{os: "aix", arch: "ppc64", ext: "tar.gz"}}},
		{"osx-x64-tar", []nodeDescriptor{{os: "darwin", arch: "x64", ext: "tar.gz"}, {os: "darwin", arch: "x64", ext: "tar.xz"}}},
		{"osx-arm64-tar", []nodeDescriptor{{os: "darwin", arch: "arm64", ext: "tar.gz"}, {os: "darwin", arch: "arm64", ext: "tar.xz"}}},
		{"win-x64-zip", []nodeDescriptor{{os: "win", arch: "x64", ext: "zip"}}},
		{"win-arm64-7z", []nodeDescriptor{{os: "win", arch: "arm64", ext: "7z"}}},
		{"LINUX-X64", []nodeDescriptor{{os: "linux", arch: "x64", ext: "tar.gz"}, {os: "linux", arch: "x64", ext: "tar.xz"}}},
		// Not installable platform archives.
		{"headers", nil},
		{"src", nil},
		{"win-x64-exe", nil},
		{"win-x64-msi", nil},
		{"osx-x64-pkg", nil},
		{"", nil},
		{"linux", nil},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			assert.Eq(t, tc.want, parseNodeFileDescriptor(tc.desc))
		})
	}
}

// TestEquivS_A07 pins parseZuluFilename: extraction via archiveExt, then the
// narrow upstream allow-list (tar.gz/zip only).
func TestEquivS_A07(t *testing.T) {
	osName, arch, ext, ok := parseZuluFilename("zulu8.72.0.17-ca-jdk8.0.382-linux_x64.tar.gz")
	assert.True(t, ok)
	assert.Eq(t, "linux", osName)
	assert.Eq(t, "x64", arch)
	assert.Eq(t, "tar.gz", ext)

	osName, arch, ext, ok = parseZuluFilename("zulu8.0.382-win_x64.zip")
	assert.True(t, ok)
	assert.Eq(t, "win", osName)
	assert.Eq(t, "x64", arch)
	assert.Eq(t, "zip", ext)

	// archiveExt knows .tar.xz, the Zulu allow-list does not.
	_, _, _, ok = parseZuluFilename("zulu8.0.382-linux_x64.tar.xz")
	assert.False(t, ok)

	// The extension comparison folds case now, so an upper-case name resolves to
	// the same allow-list entry as its lower-case sibling.
	osName, arch, ext, ok = parseZuluFilename("zulu8.0.382-linux_x64.TAR.GZ")
	assert.True(t, ok)
	assert.Eq(t, "linux", osName)
	assert.Eq(t, "tar.gz", ext)
	// Quirk kept: the extension comparison folds case, but the suffix trim that
	// splits off the platform does not, so the arch keeps the suffix tail.
	assert.Eq(t, "x64.TAR.GZ", arch)
}

// TestEquivS_A08 pins archiveExt after the drift fix (my-tools-3gdb): the
// multi-segment set matches the other archive stations and the comparison folds
// case, so the extension label is stable for any source casing.
func TestEquivS_A08(t *testing.T) {
	assert.Eq(t, "tar.gz", archiveExt("tool.tar.gz"))
	assert.Eq(t, "tar.xz", archiveExt("tool.tar.xz"))
	assert.Eq(t, "tar.bz2", archiveExt("tool.tar.bz2"))
	assert.Eq(t, "tar.zst", archiveExt("tool.tar.zst"))
	assert.Eq(t, "tar.br", archiveExt("tool.tar.br"))
	assert.Eq(t, "tar.lz4", archiveExt("tool.tar.lz4"))
	assert.Eq(t, "zip", archiveExt("tool.zip"))
	assert.Eq(t, "7z", archiveExt("tool.7z"))

	// Folded: upper case reports the same label as the lower-case sibling.
	assert.Eq(t, "tar.gz", archiveExt("TOOL.TAR.GZ"))
	assert.Eq(t, "zip", archiveExt("TOOL.ZIP"))

	assert.Eq(t, "", archiveExt("tool"))
}
