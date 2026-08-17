package install

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/inherelab/eget/internal/util"
)

func effectiveOutput(opts Options) string {
	if opts.IsGUI && opts.InstallMode == InstallModePortable && !opts.OutputExplicit && opts.GuiTarget != "" {
		if opts.Name != "" {
			return filepath.Join(opts.GuiTarget, opts.Name)
		}
		return opts.GuiTarget
	}
	return opts.Output
}

func portableGUIArchiveExtractAll(assetURL string, opts Options) bool {
	return opts.IsGUI &&
		opts.InstallMode == InstallModePortable &&
		opts.ExtractFile == "" &&
		isExtractAllArchiveAsset(assetURL)
}

func isExtractAllArchiveAsset(assetURL string) bool {
	name := strings.ToLower(path.Base(assetURL))
	return strings.HasSuffix(name, ".zip") ||
		strings.HasSuffix(name, ".7z") ||
		strings.HasSuffix(name, ".tar") ||
		strings.HasSuffix(name, ".tar.gz") ||
		strings.HasSuffix(name, ".tgz") ||
		strings.HasSuffix(name, ".tar.bz2") ||
		strings.HasSuffix(name, ".tbz") ||
		strings.HasSuffix(name, ".tar.xz") ||
		strings.HasSuffix(name, ".txz") ||
		strings.HasSuffix(name, ".tar.zst")
}

func outputPath(file ExtractedFile, output string, all bool, preferredName string, outputExplicit bool, renameFiles ...map[string]string) (string, error) {
	mode := file.Mode()
	renamed := renamedOutputName(file, firstRenameMap(renameFiles))
	out := resolvedOutputName(firstNonEmpty(renamed, file.Name), mode, preferredName)
	if all && output != "-" && file.Name != "" {
		if renamed != "" {
			return safeArchiveOutputPath(output, renamed)
		}
		var err error
		out, err = safeArchiveOutputPath(output, file.Name)
		if err != nil {
			return "", err
		}
	}
	if output == "-" {
		return "-", nil
	}
	if all && output != "" && file.Name != "" {
		return out, nil
	}
	if output != "" && all {
		os.MkdirAll(output, 0o755)
		return filepath.Join(output, out), nil
	}
	if output != "" && outputIsDirectoryTarget(output) {
		return filepath.Join(output, out), nil
	}
	if output != "" {
		if outputExplicit {
			out = output
		} else {
			if err := os.MkdirAll(output, 0o755); err != nil {
				return "", err
			}
			return filepath.Join(output, out), nil
		}
	}
	if os.Getenv("EGET_BIN") != "" && !strings.ContainsRune(out, os.PathSeparator) && mode&0o111 != 0 && !file.Dir {
		return filepath.Join(os.Getenv("EGET_BIN"), out), nil
	}
	return out, nil
}

func outputIsDirectoryTarget(output string) bool {
	return util.IsDirectory(output) || strings.HasSuffix(output, "/") || strings.HasSuffix(output, "\\")
}

func firstRenameMap(items []map[string]string) map[string]string {
	if len(items) == 0 {
		return nil
	}
	return items[0]
}

func renamedOutputName(file ExtractedFile, renameFiles map[string]string) string {
	if len(renameFiles) == 0 {
		return ""
	}
	for _, key := range []string{file.ArchiveName, file.Name, filepath.Base(file.ArchiveName), filepath.Base(file.Name)} {
		if key == "" {
			continue
		}
		if renamed := renameFiles[key]; renamed != "" {
			return renamed
		}
		normalizedKey := archivePathForCompare(key)
		for source, renamed := range renameFiles {
			if renamed != "" && archivePathForCompare(source) == normalizedKey {
				return renamed
			}
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func resolvedOutputName(name string, mode os.FileMode, preferredName string) string {
	base := filepath.Base(name)
	if !isExec(base, mode) {
		return base
	}
	if preferredName != "" {
		return applyPreferredName(base, preferredName)
	}
	return heuristicExecutableName(base)
}

func applyPreferredName(originalName, preferredName string) string {
	ext := executableSuffix(originalName)
	if preferredName == "" {
		return originalName
	}
	if filepath.Ext(preferredName) != "" {
		return preferredName
	}
	if shouldUsePreferredExecutableName(originalName, preferredName) {
		return preferredName + ext
	}
	return originalName
}

func shouldUsePreferredExecutableName(originalName, preferredName string) bool {
	ext := executableSuffix(originalName)
	base := strings.TrimSuffix(filepath.Base(originalName), ext)
	preferred := strings.TrimSuffix(filepath.Base(preferredName), executableSuffix(preferredName))
	baseLower := strings.ToLower(base)
	preferredLower := strings.ToLower(preferred)
	if baseLower == preferredLower {
		return true
	}
	if preferredLower == "" || len(baseLower) <= len(preferredLower) || !strings.HasPrefix(baseLower, preferredLower) {
		return false
	}
	switch baseLower[len(preferredLower)] {
	case '-', '_', '.':
	default:
		return false
	}
	remainder := baseLower[len(preferredLower)+1:]
	tokens := strings.FieldsFunc(remainder, func(r rune) bool {
		return r == '-' || r == '_' || r == '.'
	})
	return hasAnyToken(tokens, osTokens...) ||
		hasAnyToken(tokens, archTokens...) ||
		hasAnyToken(tokens, executableVariantTokens...) ||
		hasVersionToken(tokens)
}

func heuristicExecutableName(name string) string {
	ext := executableSuffix(name)
	base := strings.TrimSuffix(name, ext)
	patterns := []string{
		`(?i)[-_.](windows|darwin|linux|freebsd|openbsd|netbsd|android|illumos|solaris|plan9|macos|osx)[-_.](amd64|x86_64|x64|386|x86|i386|arm64|aarch64|armv?6|armv?7|arm|riscv64)$`,
		`(?i)[-_.](amd64|x86_64|x64|386|x86|i386|arm64|aarch64|armv?6|armv?7|arm|riscv64)[-_.](windows|darwin|linux|freebsd|openbsd|netbsd|android|illumos|solaris|plan9|macos|osx)$`,
		`(?i)[-_.](windows|darwin|linux|freebsd|openbsd|netbsd|android|illumos|solaris|plan9|macos|osx)$`,
	}
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if trimmed := re.ReplaceAllString(base, ""); trimmed != "" && trimmed != base {
			return trimmed + ext
		}
	}
	return name
}

func executableSuffix(name string) string {
	switch {
	case strings.HasSuffix(strings.ToLower(name), ".exe"):
		return ".exe"
	case strings.HasSuffix(strings.ToLower(name), ".appimage"):
		return ".appimage"
	default:
		return ""
	}
}
