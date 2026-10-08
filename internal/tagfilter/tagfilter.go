// Package tagfilter matches release tags against a user-supplied filter.
//
// It mirrors the --asset filter grammar. A plain value matches as an anchored,
// case-insensitive prefix; PRE: enters a prefix match, SUF: a suffix match and
// REG: a Go regular expression. The anchor matters: an unanchored substring
// match can select an unrelated product's release inside a monorepo, which is
// why a plain value is treated as a prefix rather than "contains".
package tagfilter

import (
	"fmt"
	"regexp"
	"strings"
)

// Matcher matches a release tag.
type Matcher struct {
	raw   string
	regex *regexp.Regexp
}

// IsPattern reports whether raw uses an explicit PRE:/SUF:/REG: filter. Callers
// use it to tell "follow this tag family" apart from "this exact tag" (a plain
// value is also a valid matcher, but only as a prefix fallback).
func IsPattern(raw string) bool {
	raw = strings.TrimSpace(raw)
	return strings.HasPrefix(raw, "PRE:") ||
		strings.HasPrefix(raw, "SUF:") ||
		strings.HasPrefix(raw, "REG:")
}

// Parse builds a Matcher. A plain value or PRE:/SUF: is matched
// case-insensitively; REG: keeps the caller's case sensitivity.
func Parse(raw string) (Matcher, error) {
	value := strings.TrimSpace(raw)
	switch {
	case strings.HasPrefix(value, "REG:"):
		expr := strings.TrimPrefix(value, "REG:")
		re, err := regexp.Compile(expr)
		if err != nil {
			return Matcher{}, fmt.Errorf("invalid tag regex %q: %w", expr, err)
		}
		return Matcher{raw: value, regex: re}, nil
	case strings.HasPrefix(value, "SUF:"):
		expr := strings.TrimPrefix(value, "SUF:")
		return Matcher{raw: value, regex: regexp.MustCompile(`(?i)` + regexp.QuoteMeta(expr) + `$`)}, nil
	case strings.HasPrefix(value, "PRE:"):
		expr := strings.TrimPrefix(value, "PRE:")
		return Matcher{raw: value, regex: regexp.MustCompile(`(?i)^` + regexp.QuoteMeta(expr))}, nil
	default:
		return Matcher{raw: value, regex: regexp.MustCompile(`(?i)^` + regexp.QuoteMeta(value))}, nil
	}
}

// Match reports whether tag satisfies the filter.
func (m Matcher) Match(tag string) bool {
	if m.regex == nil {
		return false
	}
	return m.regex.MatchString(tag)
}

func (m Matcher) String() string {
	return m.raw
}
