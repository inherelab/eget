// Package platform holds the OS/arch knowledge that used to be duplicated across
// install, install/detect, client and app/cache: alias/token tables and the
// recognition regex tables.
//
// It holds data and splitting primitives only. The "does this name belong to
// this platform" decisions stay with their call sites
// (nameHasPlatform, trimPlatformSuffix, systemDetector.Detect), so the tables can
// be shared without changing any behaviour — see
// docs/plans/2026-10-17-rule-consolidation-plan.md.
package platform

import (
	"strings"

	"github.com/inherelab/eget/internal/util"
)

// OSTokens returns the OS tokens recognised in file names. The returned slice is
// owned by this package: callers must not modify it.
func OSTokens() []string {
	return []string{"windows", "win", "win32", "win64", "darwin", "macos", "osx", "linux", "freebsd", "openbsd", "netbsd", "android", "illumos", "solaris", "plan9"}
}

// ArchTokens returns the architecture tokens recognised in file names. The
// returned slice is owned by this package: callers must not modify it.
func ArchTokens() []string {
	return []string{"amd64", "x86_64", "x64", "386", "x86", "i386", "arm64", "aarch64", "arm32", "armv6", "armv7", "arm", "riscv64"}
}

// VariantTokens returns the executable variant tokens (e.g. "portable").
func VariantTokens() []string {
	return []string{"portable"}
}

// SplitTokens splits a name into tokens using both the "-./\\" and the
// "-_. /\\" separator sets, after lower-casing and normalising slashes.
func SplitTokens(name string) []string {
	base := util.NormalizeSlashesLower(name)
	tokens := strings.FieldsFunc(base, func(r rune) bool {
		return r == '-' || r == '.' || r == '/' || r == '\\'
	})
	tokens = append(tokens, strings.FieldsFunc(base, func(r rune) bool {
		return r == '-' || r == '_' || r == '.' || r == '/' || r == '\\'
	})...)
	return tokens
}

// HasAnyToken reports whether tokens contains one of aliases.
func HasAnyToken(tokens []string, aliases ...string) bool {
	for _, token := range tokens {
		for _, alias := range aliases {
			if token == alias {
				return true
			}
		}
	}
	return false
}

// Aliases expands a goos/goarch pair into the file-name aliases that identify it.
func Aliases(goos, goarch string) ([]string, []string) {
	return osAliases(goos), archAliases(goarch)
}

func osAliases(goos string) []string {
	switch strings.ToLower(goos) {
	case "windows":
		return []string{"windows", "win", "win32", "win64"}
	case "darwin":
		return []string{"darwin", "macos", "osx"}
	default:
		return []string{strings.ToLower(goos)}
	}
}

func archAliases(goarch string) []string {
	switch strings.ToLower(goarch) {
	case "amd64":
		return []string{"amd64", "x86_64", "x64"}
	case "386":
		return []string{"386", "x86", "i386"}
	case "arm64":
		return []string{"arm64", "aarch64"}
	case "arm":
		return []string{"arm32", "armv6", "armv7", "arm"}
	default:
		return []string{strings.ToLower(goarch)}
	}
}
