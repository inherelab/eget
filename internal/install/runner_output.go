package install

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gobwas/glob"
	"github.com/gookit/goutil/x/ccolor"
	"github.com/inherelab/eget/internal/filekind"
	"github.com/inherelab/eget/internal/platform"
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

// hoistPortableAppBundle moves a macOS *.app bundle that the portable GUI layout
// put one level below guiTarget (the injected <gui_target>/<name> directory) up to
// <gui_target>/<App>.app, where macOS expects applications. Any other layout, an
// existing destination or a failed rename keeps the extracted tree as it is.
func hoistPortableAppBundle(output, guiTarget string, paths []string) []string {
	if guiTarget == "" || output == "" || filepath.Clean(output) == filepath.Clean(guiTarget) {
		return paths
	}
	entries, err := os.ReadDir(output)
	if err != nil || len(entries) != 1 || !entries[0].IsDir() {
		return paths
	}
	bundle := entries[0].Name()
	if !isAppBundleFold(bundle) {
		return paths
	}
	src := filepath.Join(output, bundle)
	dst := filepath.Join(guiTarget, bundle)
	if _, err := os.Stat(dst); err == nil {
		return paths
	}
	if err := os.Rename(src, dst); err != nil {
		return paths
	}
	_ = os.Remove(output)

	hoisted := make([]string, 0, len(paths))
	for _, path := range paths {
		rel, err := filepath.Rel(src, path)
		if err != nil || rel == ".." || filepath.IsAbs(rel) || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			hoisted = append(hoisted, path)
			continue
		}
		hoisted = append(hoisted, filepath.Join(dst, rel))
	}
	return hoisted
}

func isExtractAllArchiveAsset(assetURL string) bool {
	return filekind.IsArchiveFold(path.Base(assetURL), filekind.ExtSetExtractAll)
}

func outputPath(file ExtractedFile, output string, all bool, preferredName string, outputExplicit bool, renameFiles ...map[string]string) (string, error) {
	mode := file.Mode()
	renamed := renamedOutputName(file, firstRenameMap(renameFiles))
	out := resolvedPreferredOutputName(file, renamed, mode, preferredName)
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
	for _, key := range renameFileKeys(file) {
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
	return renamedByGlobPattern(file, renameFiles)
}

func renameFileKeys(file ExtractedFile) []string {
	return []string{file.ArchiveName, file.Name, filepath.Base(file.ArchiveName), filepath.Base(file.Name)}
}

// renamedByGlobPattern applies rename rules whose source carries a glob, using
// the same matching rules as --file: the pattern matches the normalized archive
// path or the file base name. Several matching rules resolve in sorted order so
// the result does not depend on map iteration.
func renamedByGlobPattern(file ExtractedFile, renameFiles map[string]string) string {
	names := make([]string, 0, 2)
	for _, name := range []string{file.ArchiveName, file.Name} {
		if name == "" {
			continue
		}
		names = append(names, archivePathForCompare(name))
	}
	if len(names) == 0 {
		return ""
	}
	sources := sortedRenameSources(renameFiles, true)
	for _, source := range sources {
		renamed := renameFiles[source]
		pattern, err := glob.Compile(archivePathForCompare(source), '/')
		if err != nil {
			// Invalid patterns are rejected up front by validateRenamePatterns.
			continue
		}
		for _, name := range names {
			if pattern.Match(name) || pattern.Match(path.Base(name)) {
				return renamed
			}
		}
	}
	return ""
}

// validateRenamePatterns rejects rename rules with an unusable glob instead of
// silently leaving the renamed files untouched.
func validateRenamePatterns(renameFiles map[string]string) error {
	for _, source := range sortedRenameSources(renameFiles, true) {
		if _, err := glob.Compile(archivePathForCompare(source), '/'); err != nil {
			return fmt.Errorf("invalid rename pattern %q: %w", source, err)
		}
	}
	return nil
}

// warnUnmatchedRenamePatterns reports rename rules that renamed nothing in this
// run, so a typo or an unsupported pattern does not pass silently.
func warnUnmatchedRenamePatterns(w io.Writer, renameFiles map[string]string, outputs []string) {
	if w == nil || len(renameFiles) == 0 || len(outputs) == 0 {
		return
	}
	targets := make(map[string]bool, len(outputs))
	for _, out := range outputs {
		if out == "" || out == "-" {
			continue
		}
		targets[archivePathForCompare(filepath.Base(out))] = true
	}
	for _, source := range sortedRenameSources(renameFiles, false) {
		if target := archivePathForCompare(filepath.Base(renameFiles[source])); targets[target] {
			continue
		}
		ccolor.Fprintf(w, "<yellow>Warning: rename rule %q matched no extracted file</>\n", source)
	}
}

// sortedRenameSources lists rename rule sources in a stable order. With
// onlyGlob only rules that carry a glob are listed.
func sortedRenameSources(renameFiles map[string]string, onlyGlob bool) []string {
	sources := make([]string, 0, len(renameFiles))
	for source, renamed := range renameFiles {
		if renamed == "" || (onlyGlob && !renamePatternHasGlob(source)) {
			continue
		}
		sources = append(sources, source)
	}
	sort.Strings(sources)
	return sources
}

func renamePatternHasGlob(source string) bool {
	return strings.ContainsAny(source, "*?[{")
}

// explicitFileTarget returns the --to path when the user named a file rather
// than a directory: the target is explicit, is neither an existing directory nor
// written with a trailing separator, and carries a file extension. A name
// without an extension stays a directory, the way --extract-all and multi-pattern
// --file targets have always been read.
func explicitFileTarget(opts Options) string {
	if !opts.OutputExplicit {
		return ""
	}
	out := effectiveOutput(opts)
	if out == "" || out == "-" || outputIsDirectoryTarget(out) {
		return ""
	}
	if filepath.Ext(filepath.Base(out)) == "" {
		return ""
	}
	return out
}

// collectSingleFileOutput moves the only extracted file to the file target the
// user asked for. Extract-all always builds a directory tree first, so that tree
// is dropped again once the file sits at the target path.
func collectSingleFileOutput(target string, paths []string) ([]string, error) {
	if len(paths) != 1 {
		// The extract-all run filled a directory the file target never asked for.
		_ = os.RemoveAll(target)
		return nil, fileTargetMultipleFilesError(target, len(paths))
	}
	tmp := target + ".eget-extract"
	if err := os.RemoveAll(tmp); err != nil {
		return nil, err
	}
	if err := moveFile(paths[0], tmp); err != nil {
		return nil, err
	}
	if err := os.RemoveAll(target); err != nil {
		return nil, err
	}
	if err := moveFile(tmp, target); err != nil {
		return nil, err
	}
	return []string{target}, nil
}

func fileTargetMultipleFilesError(target string, count int) error {
	return fmt.Errorf("%s is a file target but %d files were extracted; use a directory target (with a trailing separator) or select one file with --file", target, count)
}

// moveFile renames src to dst and falls back to copy and remove when the two
// paths live on different volumes or the destination already exists.
func moveFile(src, dst string) error {
	if src == dst {
		return nil
	}
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := copyFile(src, dst, info.Mode(), info.ModTime()); err != nil {
		return err
	}
	return os.Remove(src)
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

// resolvedPreferredOutputName resolves the output name for the extracted file.
// Single-file downloads are renamed to the repo/tool name during extraction, which
// drops the platform and arch tokens carried by the asset name. When the preferred
// (--name) value only matches the original asset name, fall back to it so platform
// suffixed assets like `omp-darwin-arm64` can still be renamed to `omp`.
func resolvedPreferredOutputName(file ExtractedFile, renamed string, mode os.FileMode, preferredName string) string {
	primary := firstNonEmpty(renamed, file.Name)
	out := resolvedOutputName(primary, mode, preferredName)
	if preferredName == "" || renamed != "" || out != filepath.Base(primary) {
		return out
	}
	assetBase := filepath.Base(file.ArchiveName)
	if assetBase == "." || assetBase == filepath.Base(primary) {
		return out
	}
	if alt := resolvedOutputName(file.ArchiveName, mode, preferredName); alt != assetBase {
		return alt
	}
	return out
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
	return platform.HasAnyToken(tokens, platform.OSTokens()...) ||
		platform.HasAnyToken(tokens, platform.ArchTokens()...) ||
		platform.HasAnyToken(tokens, platform.VariantTokens()...) ||
		hasVersionToken(tokens)
}

func heuristicExecutableName(name string) string {
	ext := executableSuffix(name)
	base := strings.TrimSuffix(name, ext)
	for _, re := range platform.TrimPlatformSuffixRegexps() {
		if trimmed := re.ReplaceAllString(base, ""); trimmed != "" && trimmed != base {
			return trimmed + ext
		}
	}
	return name
}

func executableSuffix(name string) string {
	return filekind.ExecutableSuffixFold(name)
}
