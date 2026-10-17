package platform

import (
	"regexp"
	"strings"
)

// DirArchPatterns returns the patterns that match an extracted directory or file
// path belonging to goarch (install.autoSelectExtractedFile). It returns nil for
// architectures without a dedicated pattern.
func DirArchPatterns(goarch string) []*regexp.Regexp {
	switch strings.ToLower(goarch) {
	case "amd64":
		return compileArchPatterns(`(^|/)(x64|amd64|x86_64)(/|$)`)
	case "386":
		return compileArchPatterns(`(^|/)(x86|i386|386)(/|$)`)
	case "arm64":
		return compileArchPatterns(`(^|/)(arm64|aarch64)(/|$)`)
	case "arm":
		return compileArchPatterns(`(^|/)(arm32|armv6|armv7|arm)(/|$)`)
	default:
		return nil
	}
}

// TrimPlatformSuffixRegexps returns the three patterns that strip an OS/arch (or
// bare OS) suffix from an executable name, as used by
// install.heuristicExecutableName.
//
// The table is deliberately kept verbatim: "armv?6"/"armv?7" express an optional
// "v", and the patterns also encode the os-arch order, the separators and the
// end anchor, none of which can be derived from the token lists.
func TrimPlatformSuffixRegexps() []*regexp.Regexp {
	return compileArchPatterns(
		`(?i)[-_.](windows|darwin|linux|freebsd|openbsd|netbsd|android|illumos|solaris|plan9|macos|osx)[-_.](amd64|x86_64|x64|386|x86|i386|arm64|aarch64|armv?6|armv?7|arm|riscv64)$`,
		`(?i)[-_.](amd64|x86_64|x64|386|x86|i386|arm64|aarch64|armv?6|armv?7|arm|riscv64)[-_.](windows|darwin|linux|freebsd|openbsd|netbsd|android|illumos|solaris|plan9|macos|osx)$`,
		`(?i)[-_.](windows|darwin|linux|freebsd|openbsd|netbsd|android|illumos|solaris|plan9|macos|osx)$`,
	)
}

func compileArchPatterns(exprs ...string) []*regexp.Regexp {
	patterns := make([]*regexp.Regexp, 0, len(exprs))
	for _, expr := range exprs {
		patterns = append(patterns, regexp.MustCompile(expr))
	}
	return patterns
}
