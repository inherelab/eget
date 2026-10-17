package filekind

import "strings"

// IsExecutableExact reports whether name ends with an executable suffix, using
// the case-sensitive comparison of install.rename, BinaryChooser.Choose and
// isExec.
func IsExecutableExact(name string) bool {
	return hasSuffixExact(name, ExtSetExecutable)
}

// IsExecutableFold reports whether the folded name ends with an executable
// suffix (install.isExecutableForGOOS, autoSelectOnlyWindowsExecutable,
// requiresSystem7z, shouldUseSystem7z).
func IsExecutableFold(name string) bool {
	return hasSuffixFold(name, ExtSetExecutable)
}

// IsNonExecutableExact reports whether name ends with a suffix that is never
// executable (install.isDefinitelyNotExec). Its base-name list stays in the
// caller.
func IsNonExecutableExact(name string) bool {
	return hasSuffixExact(name, ExtSetNonExecutable)
}

// ExecutableSuffixExact returns the executable suffix of name using the
// case-sensitive axis (install.rename, install.executableSuffix callers that
// keep the caller's case), or an empty string.
func ExecutableSuffixExact(name string) string {
	for _, ext := range ExtSetExecutable {
		if strings.HasSuffix(name, ext) {
			return ext
		}
	}
	return ""
}

// ExecutableSuffixFold returns the executable suffix of the folded name, or an
// empty string — the mapping used by install.heuristicExecutableName and
// normalizedAssetNameHint.
func ExecutableSuffixFold(name string) string {
	lower := strings.ToLower(name)
	for _, ext := range ExtSetExecutable {
		if strings.HasSuffix(lower, ext) {
			return ext
		}
	}
	return ""
}

// IsInstallerFold reports whether the folded name ends with an installer suffix
// (install.DetectGUIInstallMode, DetectInstallerKind). Keyword checks such as
// "setup"/"installer" stay in the callers.
func IsInstallerFold(name string) bool {
	return hasSuffixFold(name, ExtSetInstaller)
}

// RequiresSystemArchiverFold reports whether the folded name must be handled by
// the system archiver: the unconditional suffixes of ExtSetSystemArchiver only.
// The conditional ".exe" case is expressed by the callers as
// IsExecutableFold(name) && extractArchive.
func RequiresSystemArchiverFold(name string) bool {
	return hasSuffixFold(name, ExtSetSystemArchiver)
}

func hasSuffixExact(name string, set []string) bool {
	for _, ext := range set {
		if strings.HasSuffix(name, ext) {
			return true
		}
	}
	return false
}

func hasSuffixFold(name string, set []string) bool {
	lower := strings.ToLower(name)
	for _, ext := range set {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}
