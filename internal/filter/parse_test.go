package filter

import (
	"testing"

	"github.com/gookit/goutil/x/assert"
)

func TestParseKinds(t *testing.T) {
	cases := []struct {
		raw      string
		kind     Kind
		expr     string
		hasRegex bool
	}{
		{"v1.2.3", KindPlain, "v1.2.3", false},
		{"PRE:app-v", KindPrefix, "app-v", true},
		{"SUF:-linux", KindSuffix, "-linux", true},
		{`REG:^v1\.`, KindRegex, `^v1\.`, true},
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			spec, err := Parse(tc.raw)
			assert.NoErr(t, err)
			assert.Eq(t, tc.raw, spec.Raw)
			assert.Eq(t, tc.kind, spec.Kind)
			assert.Eq(t, tc.expr, spec.Expr)
			assert.False(t, spec.Anti)
			if tc.hasRegex {
				assert.NotNil(t, spec.Regex)
			} else {
				assert.Nil(t, spec.Regex)
			}
		})
	}

	t.Run("no trim and no anti stripping", func(t *testing.T) {
		spec, err := Parse(" PRE:v1 ")
		assert.NoErr(t, err)
		assert.Eq(t, KindPlain, spec.Kind)
		assert.Eq(t, " PRE:v1 ", spec.Expr)

		spec, err = Parse("^PRE:v1")
		assert.NoErr(t, err)
		assert.Eq(t, KindPlain, spec.Kind)
		assert.Eq(t, "^PRE:v1", spec.Expr)
	})
}

func TestParseRegexStringsMatchLegacy(t *testing.T) {
	prefix, err := Parse("PRE:app-v")
	assert.NoErr(t, err)
	assert.Eq(t, `(?i)^`+"app-v", prefix.Regex.String())
	assert.Eq(t, prefix.Regex.String(), prefix.PrefixRegex().String())

	suffix, err := Parse("SUF:-linux")
	assert.NoErr(t, err)
	assert.Eq(t, `(?i)`+"-linux"+`$`, suffix.Regex.String())
	assert.Eq(t, suffix.Regex.String(), suffix.SuffixRegex().String())

	t.Run("quote meta", func(t *testing.T) {
		spec, err := Parse("PRE:v1.0+build(1)")
		assert.NoErr(t, err)
		assert.Eq(t, `(?i)^`+"v1\\.0\\+build\\(1\\)", spec.Regex.String())
	})

	t.Run("matching behaviour", func(t *testing.T) {
		assert.True(t, prefix.Regex.MatchString("APP-V1.0"))
		assert.False(t, prefix.Regex.MatchString("x-app-v"))
		assert.True(t, suffix.Regex.MatchString("tool-LINUX"))
		assert.False(t, suffix.Regex.MatchString("linux-tool"))
	})
}

func TestParseRegexError(t *testing.T) {
	_, err := Parse("REG:[")
	assert.Err(t, err)
	re, ok := RegexErrorOf(err)
	assert.True(t, ok)
	assert.Eq(t, "[", re.Expr)
	assert.NotNil(t, re.Err)

	_, err = ParseRegex("[")
	_, ok = RegexErrorOf(err)
	assert.True(t, ok)
}

func TestIsPattern(t *testing.T) {
	assert.True(t, IsPattern("PRE:v1"))
	assert.True(t, IsPattern("SUF:v1"))
	assert.True(t, IsPattern("REG:^v1"))
	assert.False(t, IsPattern("v1.2.3"))
	assert.False(t, IsPattern(" PRE:v1"))
	assert.False(t, IsPattern("^PRE:v1"))
}

func TestStripAnti(t *testing.T) {
	anti, rest := StripAnti("^REG:x")
	assert.True(t, anti)
	assert.Eq(t, "REG:x", rest)

	anti, rest = StripAnti("REG:x")
	assert.False(t, anti)
	assert.Eq(t, "REG:x", rest)

	anti, rest = StripAnti("^")
	assert.True(t, anti)
	assert.Eq(t, "", rest)

	t.Run("does not trim", func(t *testing.T) {
		anti, rest = StripAnti(" ^REG:x")
		assert.False(t, anti)
		assert.Eq(t, " ^REG:x", rest)
	})
}
