package sdk

import (
	"testing"

	"github.com/gookit/goutil/x/assert"
)

// Filter-station guards (docs/plans/2026-10-17-rule-consolidation-plan.md,
// S-F04): the sdk search only interprets REG:, everything else — including
// PRE:/SUF: — stays a substring match, and the entry keeps its own "^"/trim
// handling.

func TestEquivS_F04(t *testing.T) {
	t.Run("plain and PRE/SUF stay substring", func(t *testing.T) {
		ok, err := searchKeywordMatches([]string{"tool-linux-x64"}, "tool-linux-x64", "linux")
		assert.NoErr(t, err)
		assert.True(t, ok)

		// PRE: is not a prefix operator here: it is matched literally. The plain
		// path lower-cases the keyword only, so the haystack must already match it.
		ok, err = searchKeywordMatches([]string{"pre:linux"}, "pre:linux", "PRE:linux")
		assert.NoErr(t, err)
		assert.True(t, ok)

		ok, err = searchKeywordMatches([]string{"tool-linux-x64"}, "tool-linux-x64", "PRE:linux")
		assert.NoErr(t, err)
		assert.False(t, ok)
	})

	t.Run("anti inverts the substring match", func(t *testing.T) {
		ok, err := searchKeywordMatches([]string{"tool-linux-x64"}, "tool-linux-x64", "^linux")
		assert.NoErr(t, err)
		assert.False(t, ok)

		ok, err = searchKeywordMatches([]string{"tool-darwin-arm64"}, "tool-darwin-arm64", "^linux")
		assert.NoErr(t, err)
		assert.True(t, ok)
	})

	t.Run("anti plus literal PRE", func(t *testing.T) {
		// The literal (lower-cased) keyword is absent, so the exclusion holds.
		ok, err := searchKeywordMatches([]string{"PRE:x"}, "PRE:x", "^PRE:x")
		assert.NoErr(t, err)
		assert.True(t, ok)

		ok, err = searchKeywordMatches([]string{"pre:x"}, "pre:x", "^PRE:x")
		assert.NoErr(t, err)
		assert.False(t, ok)
	})

	t.Run("REG: uses the user regex", func(t *testing.T) {
		ok, err := searchKeywordMatches([]string{"tool-1.2.3"}, "tool-1.2.3", `REG:^tool-\d`)
		assert.NoErr(t, err)
		assert.True(t, ok)

		ok, err = searchKeywordMatches([]string{"tool-1.2.3"}, "tool-1.2.3", `REG:^other`)
		assert.NoErr(t, err)
		assert.False(t, ok)
	})

	t.Run("keyword is trimmed by the entry", func(t *testing.T) {
		ok, err := searchKeywordMatches([]string{"tool-linux-x64"}, "tool-linux-x64", "  linux  ")
		assert.NoErr(t, err)
		assert.True(t, ok)
	})

	t.Run("invalid regex keeps the raw regexp wording", func(t *testing.T) {
		_, err := searchKeywordMatches([]string{"x"}, "x", "REG:[")
		assert.Err(t, err)
		assert.Eq(t, "error parsing regexp: missing closing ]: `[`", err.Error())
	})
}
