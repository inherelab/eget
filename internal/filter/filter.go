// Package filter holds the shared parser for the explicit PRE:/SUF:/REG: filter
// grammar and the "^" exclusion prefix.
//
// It only parses: it does not decide what is being matched (release tag, asset
// name, file name or search keyword), does not build matchers and does not take
// over the glob or substring defaults of its callers. Callers keep their own
// trimming, error wording and matching semantics — see
// docs/plans/2026-10-17-rule-consolidation-plan.md.
package filter

import (
	"errors"
	"regexp"
	"strings"
)

// Kind is the filter form a raw expression uses.
type Kind int

// Filter forms.
const (
	KindPlain Kind = iota
	KindPrefix
	KindSuffix
	KindRegex
)

// Spec is the parsed form of a filter expression.
//
// Raw is the input as passed in (not trimmed, "^" not stripped); Expr is the
// expression without its syntax prefix (for KindPlain it equals Raw); Anti is set
// by the caller through StripAnti, never by Parse. Regex is filled for
// KindPrefix/KindSuffix/KindRegex and is nil for KindPlain, whose meaning
// (substring or anchored prefix) belongs to the caller.
type Spec struct {
	Raw   string
	Anti  bool
	Kind  Kind
	Expr  string
	Regex *regexp.Regexp
}

// RegexError reports an invalid REG: expression. Callers take Expr to keep their
// own error wording (e.g. `invalid tag regex %q`).
type RegexError struct {
	Expr string
	Err  error
}

func (e *RegexError) Error() string { return e.Err.Error() }

func (e *RegexError) Unwrap() error { return e.Err }

// IsPattern reports whether raw uses an explicit PRE:/SUF:/REG: filter. It does
// not trim and does not treat "^PRE:" as a pattern.
func IsPattern(raw string) bool {
	return strings.HasPrefix(raw, "PRE:") ||
		strings.HasPrefix(raw, "SUF:") ||
		strings.HasPrefix(raw, "REG:")
}

// StripAnti removes a single leading "^" exclusion marker and reports whether it
// was present. It does not trim and does not look at the syntax prefixes.
func StripAnti(raw string) (bool, string) {
	if strings.HasPrefix(raw, "^") {
		return true, raw[1:]
	}
	return false, raw
}

// ParseRegex compiles a REG: expression and wraps failures in *RegexError.
func ParseRegex(expr string) (*regexp.Regexp, error) {
	re, err := regexp.Compile(expr)
	if err != nil {
		return nil, &RegexError{Expr: expr, Err: err}
	}
	return re, nil
}

// PrefixRegex returns the case-insensitive, anchored-prefix matcher for the
// expression — the regex the tag and asset filters built before.
func (s Spec) PrefixRegex() *regexp.Regexp {
	return regexp.MustCompile(`(?i)^` + regexp.QuoteMeta(s.Expr))
}

// SuffixRegex returns the case-insensitive suffix matcher for the expression.
func (s Spec) SuffixRegex() *regexp.Regexp {
	return regexp.MustCompile(`(?i)` + regexp.QuoteMeta(s.Expr) + `$`)
}

// RegexErrorOf extracts the *RegexError from err, if any.
func RegexErrorOf(err error) (*RegexError, bool) {
	var re *RegexError
	if errors.As(err, &re) {
		return re, true
	}
	return nil, false
}
