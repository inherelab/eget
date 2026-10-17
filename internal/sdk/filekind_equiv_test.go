package sdk

import (
	"testing"

	"github.com/gookit/goutil/x/assert"
)

// The tests below pin the sdk index suffix stations before they start consuming
// internal/filekind (docs/plans/2026-10-17-rule-consolidation-plan.md, S-A06/
// S-A07/S-A08). They must keep passing unchanged after the migration.

// TestEquivS_A06 pins parseNodeFileDescriptor: the node index uses "-"
// separated suffixes and the sensitive axis.
func TestEquivS_A06(t *testing.T) {
	osName, arch, ext, ok := parseNodeFileDescriptor("linux-x64-tar.xz")
	assert.True(t, ok)
	assert.Eq(t, "linux", osName)
	assert.Eq(t, "x64", arch)
	assert.Eq(t, "tar.xz", ext)

	_, _, _, ok = parseNodeFileDescriptor("linux-x64-tar.gz")
	assert.True(t, ok)
	_, _, _, ok = parseNodeFileDescriptor("linux-x64-zip")
	assert.True(t, ok)

	// Sensitive axis: upper case is not recognised.
	_, _, _, ok = parseNodeFileDescriptor("linux-x64-TAR.XZ")
	assert.False(t, ok)
	// Not part of the node index naming convention.
	_, _, _, ok = parseNodeFileDescriptor("linux-x64-tar.zst")
	assert.False(t, ok)
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
	_, _, _, ok = parseZuluFilename("zulu8.0.382-linux_x64.TAR.GZ")
	assert.False(t, ok)
}

// TestEquivS_A08 pins archiveExt: the deliberately narrow legacy set on the
// sensitive axis.
func TestEquivS_A08(t *testing.T) {
	assert.Eq(t, "tar.gz", archiveExt("tool.tar.gz"))
	assert.Eq(t, "tar.xz", archiveExt("tool.tar.xz"))
	assert.Eq(t, "tar.bz2", archiveExt("tool.tar.bz2"))
	assert.Eq(t, "zip", archiveExt("tool.zip"))
	assert.Eq(t, "7z", archiveExt("tool.7z"))

	// Drift kept on purpose: the legacy set lacks these, so only the tail of
	// path.Ext is reported.
	assert.Eq(t, "zst", archiveExt("tool.tar.zst"))
	assert.Eq(t, "br", archiveExt("tool.tar.br"))
	assert.Eq(t, "lz4", archiveExt("tool.tar.lz4"))

	// Sensitive axis.
	assert.Eq(t, "GZ", archiveExt("TOOL.TAR.GZ"))
	assert.Eq(t, "", archiveExt("tool"))
}
