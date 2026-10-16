package install

import (
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/inherelab/eget/internal/util"
)

func selectionPlatform(opts Options) (string, string) {
	goos, goarch := runtime.GOOS, runtime.GOARCH
	if opts.System == "" {
		return goos, goarch
	}
	parts := strings.SplitN(opts.System, "/", 2)
	if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
		return parts[0], parts[1]
	}
	return goos, goarch
}

func autoSelectAssetCandidate(candidates []string, opts Options) (string, bool) {
	goos, goarch := selectionPlatform(opts)
	if !strings.EqualFold(goos, "windows") {
		return "", false
	}

	return autoSelectWindowsMSVCAsset(candidates, goos, goarch)
}

func autoSelectWindowsMSVCAsset(candidates []string, goos, goarch string) (string, bool) {
	var selected string
	sawGNU := false
	for _, candidate := range candidates {
		tokens := platformTokens(path.Base(candidate))
		if !hasAnyToken(tokens, osAliases(goos)...) || !hasAnyToken(tokens, archAliases(goarch)...) {
			return "", false
		}
		if hasAnyToken(tokens, "gnu") {
			sawGNU = true
			continue
		}
		if hasAnyToken(tokens, "msvc") {
			if selected != "" {
				return "", false
			}
			selected = candidate
			continue
		}
		return "", false
	}
	return selected, selected != "" && sawGNU
}

func autoSelectExtractedFile(candidates []ExtractedFile, goos, goarch string) (ExtractedFile, bool) {
	if len(candidates) == 0 {
		return ExtractedFile{}, false
	}
	if strings.EqualFold(goos, "windows") {
		if selected, ok := autoSelectOnlyWindowsExecutable(candidates); ok {
			return selected, true
		}
	}
	patterns := archSelectionPatterns(goarch)
	if len(patterns) == 0 {
		return ExtractedFile{}, false
	}

	matches := make([]ExtractedFile, 0, len(candidates))
	for _, candidate := range candidates {
		name := util.NormalizeSlashesLower(candidate.ArchiveName)
		for _, pattern := range patterns {
			if pattern.MatchString(name) {
				matches = append(matches, candidate)
				break
			}
		}
	}
	if len(matches) == 1 {
		return matches[0], true
	}
	return ExtractedFile{}, false
}

func autoExtractCurrentPlatformExecutables(candidates []ExtractedFile, opts Options) ([]ExtractedFile, bool) {
	goos, goarch := selectionPlatform(opts)
	selected := make([]ExtractedFile, 0, len(candidates))
	for _, candidate := range candidates {
		if !isExecutableForGOOS(candidate, goos) {
			continue
		}
		if !archiveNameMatchesPlatform(candidate.ArchiveName, goos, goarch) {
			continue
		}
		selected = append(selected, candidate)
	}
	return stripSharedPackageDir(selected), len(selected) > 0
}

// stripSharedPackageDir removes the shared top-level directory (typically a
// `<name>-<version>-<platform>` package dir) from auto-extracted executables so
// they land at the output root, while keeping deeper sub-directories
// (e.g. `conpty/x64`) that carry real directory structure.
func stripSharedPackageDir(files []ExtractedFile) []ExtractedFile {
	if len(files) == 0 {
		return files
	}
	var top string
	for _, file := range files {
		parts := archivePathParts(firstNonEmpty(file.Name, file.ArchiveName))
		if len(parts) < 2 {
			// Root-level file: there is no package dir to strip.
			return files
		}
		if top == "" {
			top = parts[0]
		} else if top != parts[0] {
			// Files span multiple top-level dirs: the structure is real, keep it.
			return files
		}
	}
	stripped := make([]ExtractedFile, len(files))
	for i, file := range files {
		name := archivePathForCompare(firstNonEmpty(file.Name, file.ArchiveName))
		file.Name = name[len(top)+1:]
		stripped[i] = file
	}
	return stripped
}

func isExecutableForGOOS(file ExtractedFile, goos string) bool {
	if file.Dir {
		return false
	}
	name := firstNonEmpty(file.ArchiveName, file.Name)
	ext := strings.ToLower(filepath.Ext(name))
	if strings.EqualFold(goos, "windows") {
		return ext == ".exe"
	}
	return isExec(name, file.mode) && ext != ".exe"
}

func archiveNameMatchesPlatform(name, goos, goarch string) bool {
	tokens := platformTokens(name)
	if hasAnyToken(tokens, osTokens...) && !hasAnyToken(tokens, osAliases(goos)...) {
		return false
	}
	if hasAnyToken(tokens, archTokens...) && !hasAnyToken(tokens, archAliases(goarch)...) {
		return false
	}
	return true
}

func platformTokens(name string) []string {
	base := strings.ToLower(util.NormalizeSlashesLower(name))
	base = strings.TrimSuffix(base, executableSuffix(base))
	tokens := strings.FieldsFunc(base, func(r rune) bool {
		return r == '-' || r == '.' || r == '/' || r == '\\'
	})
	tokens = append(tokens, strings.FieldsFunc(base, func(r rune) bool {
		return r == '-' || r == '_' || r == '.' || r == '/' || r == '\\'
	})...)
	return tokens
}

func hasAnyToken(tokens []string, aliases ...string) bool {
	for _, token := range tokens {
		for _, alias := range aliases {
			if token == alias {
				return true
			}
		}
	}
	return false
}

func autoSelectOnlyWindowsExecutable(candidates []ExtractedFile) (ExtractedFile, bool) {
	var selected ExtractedFile
	count := 0
	for _, candidate := range candidates {
		if strings.EqualFold(filepath.Ext(candidate.ArchiveName), ".exe") {
			selected = candidate
			count++
		}
	}
	return selected, count == 1
}

func archSelectionPatterns(goarch string) []*regexp.Regexp {
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

func compileArchPatterns(exprs ...string) []*regexp.Regexp {
	patterns := make([]*regexp.Regexp, 0, len(exprs))
	for _, expr := range exprs {
		patterns = append(patterns, regexp.MustCompile(expr))
	}
	return patterns
}

var osTokens = []string{"windows", "win", "win32", "win64", "darwin", "macos", "osx", "linux", "freebsd", "openbsd", "netbsd", "android", "illumos", "solaris", "plan9"}
var archTokens = []string{"amd64", "x86_64", "x64", "386", "x86", "i386", "arm64", "aarch64", "arm32", "armv6", "armv7", "arm", "riscv64"}
var executableVariantTokens = []string{"portable"}
var versionTokenPattern = regexp.MustCompile(`^v?\d+$`)

func hasVersionToken(tokens []string) bool {
	for _, token := range tokens {
		if versionTokenPattern.MatchString(token) {
			return true
		}
	}
	return false
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
