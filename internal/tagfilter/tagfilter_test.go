package tagfilter

import (
	"testing"

	"github.com/gookit/goutil/x/assert"
)

func TestIsPattern(t *testing.T) {
	assert.True(t, IsPattern("PRE:cua-driver-rs-v"))
	assert.True(t, IsPattern("SUF:-nightly"))
	assert.True(t, IsPattern("REG:^v\\d+"))
	assert.False(t, IsPattern("v1.2.3"))
	assert.False(t, IsPattern("nightly"))
	assert.False(t, IsPattern(""))
}

func TestPlainValueMatchesAsAnchoredPrefix(t *testing.T) {
	m, err := Parse("cua-driver-rs-v")
	assert.NoErr(t, err)

	assert.True(t, m.Match("cua-driver-rs-v0.34.0"))
	// anchored: a tag that merely contains the value must not match
	assert.False(t, m.Match("x-cua-driver-rs-v0.34.0"))
	assert.False(t, m.Match("nightly-cua-driver-rs-v0.34.0"))
	// case-insensitive
	assert.True(t, m.Match("CUA-DRIVER-RS-V0.34.0"))
}

func TestPreSufFilters(t *testing.T) {
	pre, err := Parse("PRE:cua-driver-rs-v")
	assert.NoErr(t, err)
	assert.True(t, pre.Match("cua-driver-rs-v0.34.0"))
	assert.False(t, pre.Match("nightly-lume-v0.6.2"))

	suf, err := Parse("SUF:-binary.zip")
	assert.NoErr(t, err)
	assert.True(t, suf.Match("cua-driver-rs-0.34.0-windows-x86_64-binary.zip"))
	assert.False(t, suf.Match("cua-driver-rs-0.34.0-windows-x86_64.zip"))
}

func TestRegexFilterIsCaseSensitive(t *testing.T) {
	m, err := Parse(`REG:^cua-driver-rs-v\d+\.`)
	assert.NoErr(t, err)
	assert.True(t, m.Match("cua-driver-rs-v0.34.0"))
	assert.False(t, m.Match("Cua-Driver-RS-v0.34.0"))
	assert.False(t, m.Match("cua-sdk-v0.4.1"))
}

func TestParseInvalidRegex(t *testing.T) {
	_, err := Parse("REG:[")
	assert.True(t, err != nil)
}
