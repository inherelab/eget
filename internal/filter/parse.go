package filter

import "strings"

// Parse parses a PRE:/SUF:/REG: expression (a plain value stays KindPlain).
//
// It deliberately does not TrimSpace and does not strip "^": the tag callers trim
// before calling and treat "^" as literal content, while the asset/search callers
// strip "^" through StripAnti first.
func Parse(raw string) (Spec, error) {
	spec := Spec{Raw: raw, Kind: KindPlain, Expr: raw}
	switch {
	case strings.HasPrefix(raw, "REG:"):
		expr := strings.TrimPrefix(raw, "REG:")
		re, err := ParseRegex(expr)
		if err != nil {
			return Spec{}, err
		}
		spec.Kind = KindRegex
		spec.Expr = expr
		spec.Regex = re
	case strings.HasPrefix(raw, "PRE:"):
		spec.Kind = KindPrefix
		spec.Expr = strings.TrimPrefix(raw, "PRE:")
		spec.Regex = spec.PrefixRegex()
	case strings.HasPrefix(raw, "SUF:"):
		spec.Kind = KindSuffix
		spec.Expr = strings.TrimPrefix(raw, "SUF:")
		spec.Regex = spec.SuffixRegex()
	}
	return spec, nil
}
