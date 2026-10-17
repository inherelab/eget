package install

import (
	"testing"

	"github.com/gookit/goutil/x/assert"
)

// Filter-station guards (docs/plans/2026-10-17-rule-consolidation-plan.md,
// S-F02/S-F03): the parsing now lives in internal/filter, while the matching
// semantics (substring default, glob default, "^" exclusion) stay here.

// TestEquivS_F02 pins parseAssetFilter.
func TestEquivS_F02(t *testing.T) {
	t.Run("plain value keeps the substring default", func(t *testing.T) {
		f, err := parseAssetFilter("tool")
		assert.NoErr(t, err)
		assert.Eq(t, "tool", f.Expr)
		assert.False(t, f.Anti)
		assert.Nil(t, f.Regex)
	})

	t.Run("pre/suf build the anchored regexes", func(t *testing.T) {
		f, err := parseAssetFilter("PRE:app-v")
		assert.NoErr(t, err)
		assert.Eq(t, "app-v", f.Expr)
		assert.Eq(t, `(?i)^`+"app-v", f.Regex.String())

		f, err = parseAssetFilter("SUF:-linux")
		assert.NoErr(t, err)
		assert.Eq(t, `(?i)`+"-linux"+`$`, f.Regex.String())
	})

	t.Run("anti prefix", func(t *testing.T) {
		f, err := parseAssetFilter("^PRE:app-v")
		assert.NoErr(t, err)
		assert.True(t, f.Anti)
		assert.Eq(t, "app-v", f.Expr)
		assert.Eq(t, `(?i)^`+"app-v", f.Regex.String())

		f, err = parseAssetFilter("^tool")
		assert.NoErr(t, err)
		assert.True(t, f.Anti)
		assert.Eq(t, "tool", f.Expr)
		assert.Nil(t, f.Regex)
	})

	t.Run("regex keeps the user expression", func(t *testing.T) {
		f, err := parseAssetFilter(`REG:^v1\.`)
		assert.NoErr(t, err)
		assert.Eq(t, `^v1\.`, f.Expr)
		assert.Eq(t, `^v1\.`, f.Regex.String())
	})

	t.Run("invalid regex keeps the wording", func(t *testing.T) {
		_, err := parseAssetFilter("REG:[")
		assert.Err(t, err)
		assert.Eq(t, `invalid asset regex "[": error parsing regexp: missing closing ]: `+"`[`", err.Error())
	})

	t.Run("no trim", func(t *testing.T) {
		f, err := parseAssetFilter(" PRE:app-v ")
		assert.NoErr(t, err)
		assert.Eq(t, " PRE:app-v ", f.Expr)
		assert.Nil(t, f.Regex)
	})
}

// TestEquivS_F03 pins the chooser side: REG: is parsed through filter, while the
// glob default and "^" exclusion stay local.
func TestEquivS_F03(t *testing.T) {
	t.Run("regex expression is trimmed after the prefix", func(t *testing.T) {
		ch, err := NewFileChooser("REG: x ")
		assert.NoErr(t, err)
		rc, ok := ch.(*RegexChooser)
		assert.True(t, ok)
		assert.Eq(t, "x", rc.expr)
	})

	t.Run("glob default is untouched", func(t *testing.T) {
		ch, err := NewFileChooser("tool*")
		assert.NoErr(t, err)
		_, ok := ch.(*GlobChooser)
		assert.True(t, ok)
	})

	t.Run("exclusion still works", func(t *testing.T) {
		ch, err := NewFileChooser("tool*,^tool-dev*")
		assert.NoErr(t, err)
		fc, ok := ch.(*FilterChooser)
		assert.True(t, ok)
		assert.Eq(t, 1, len(fc.include))
		assert.Eq(t, 1, len(fc.exclude))
	})

	t.Run("invalid regex keeps the wording", func(t *testing.T) {
		_, err := NewFileChooser("REG:[")
		assert.Err(t, err)
		assert.Eq(t, `invalid file regex "[": error parsing regexp: missing closing ]: `+"`[`", err.Error())
	})

	t.Run("empty regex expression", func(t *testing.T) {
		_, err := NewFileChooser("REG:   ")
		assert.Err(t, err)
		assert.Eq(t, "empty file regex expression", err.Error())
	})
}
