package github

import (
	"strings"
	"testing"

	"github.com/gookit/goutil/x/assert"
	"github.com/inherelab/eget/internal/filter"
)

// Filter-station guards (docs/plans/2026-10-17-rule-consolidation-plan.md,
// S-F05): the tag filter now parses through internal/filter with the same
// trimming and matching semantics as the former tagfilter package.

func TestEquivS_F05TagMatcher(t *testing.T) {
	cases := []struct {
		raw     string
		tag     string
		matches bool
	}{
		// A plain value is an anchored, case-insensitive prefix.
		{"app-v", "APP-V1.0", true},
		{"app-v", "x-app-v1.0", false},
		// "^" is literal content for tags, not an exclusion marker.
		{"^PRE:x", "^PRE:x1", true},
		{"^PRE:x", "PRE:x1", false},
		{"PRE:app-v", "APP-V2", true},
		{"SUF:-linux", "tool-1.0-LINUX", true},
		{"SUF:-linux", "linux-tool", false},
		{`REG:^v1\.`, "v1.2.3", true},
		{`REG:^v1\.`, "v2.0.0", false},
	}
	for _, tc := range cases {
		t.Run(tc.raw+"/"+tc.tag, func(t *testing.T) {
			spec, err := filter.Parse(tc.raw)
			assert.NoErr(t, err)
			assert.Eq(t, tc.matches, tagMatcher(spec)(tc.tag))
		})
	}
}

func TestEquivS_F05TrimAndError(t *testing.T) {
	t.Run("caller trims before IsPattern", func(t *testing.T) {
		assert.True(t, filter.IsPattern(strings.TrimSpace(" PRE:v1")))
		assert.True(t, filter.IsPattern(strings.TrimSpace("PRE:v1 ")))
		assert.False(t, filter.IsPattern("^PRE:v1"))
	})

	t.Run("error wording", func(t *testing.T) {
		spec, err := filter.Parse("[")
		assert.NoErr(t, err)
		assert.Eq(t, filter.KindPlain, spec.Kind)

		_, err = filter.Parse(`REG:[`)
		assert.Err(t, err)
		re, ok := filter.RegexErrorOf(err)
		assert.True(t, ok)
		assert.Eq(t, "[", re.Expr)
	})
}
