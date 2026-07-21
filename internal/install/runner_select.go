package install

import (
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"

	"github.com/gookit/goutil/x/ccolor"
	storepkg "github.com/inherelab/eget/internal/installed"
)

func (r *InstallRunner) resolveVersionFallback(finder Finder, detector Detector, opts Options, originalErr error) (string, []string, error) {
	if opts.FallbackVersions <= 0 || !isAssetSelectionMiss(originalErr) {
		return "", nil, nil
	}
	fallback, ok := finder.(VersionFallbackFinder)
	if !ok {
		return "", nil, nil
	}
	groups, err := fallback.FallbackVersionAssets(opts.FallbackVersions)
	if err != nil {
		return "", nil, err
	}
	for _, assets := range groups {
		url, candidates, err := detector.Detect(assets)
		if len(candidates) == 0 && err == nil {
			return url, assets, nil
		}
	}
	return "", nil, nil
}

func isAssetSelectionMiss(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.HasPrefix(msg, "asset `") || msg == "no candidates found"
}

func selectedFileName(url string, file ExtractedFile) string {
	if file.ArchiveName != "" {
		return file.ArchiveName
	}
	if file.Name != "" {
		return file.Name
	}
	return path.Base(url)
}

func (r *InstallRunner) resolveCandidate(target string, candidates []string, opts Options, releaseVersion string) (string, error) {
	if selected := uniqueCandidateForName(candidates, opts.Name); selected != "" {
		return selected, nil
	}
	if selected, ok := autoSelectAssetCandidate(candidates, opts); ok {
		return selected, nil
	}

	previousAssets, _, _ := r.loadInstalled()
	if previous := previousAssets[storepkg.NormalizeRepoName(target)]; previous != "" {
		if selected := uniqueCandidateForPreviousAsset(candidates, previous); selected != "" {
			return selected, nil
		}
		for _, candidate := range candidates {
			if path.Base(candidate) == previous {
				if r.Stderr != nil {
					ccolor.Fprintf(r.Stderr, "<yellow>Warning: using previous selection '%s' as fallback</>\n", previous)
				}
				return candidate, nil
			}
		}
	}

	if opts.Quiet {
		return candidates[0], nil
	}
	if r.Prompt == nil {
		return "", fmt.Errorf("%d candidates found for asset chain", len(candidates))
	}

	choices := make([]string, len(candidates))
	for i, candidate := range candidates {
		choices[i] = path.Base(candidate)
		if decoded, err := url.PathUnescape(choices[i]); err == nil {
			choices[i] = decoded
		}
	}
	choice, err := r.Prompt(candidatePromptTitle(releaseVersion), "Filter assets", choices)
	if err != nil {
		return "", err
	}
	if choice < 0 || choice >= len(candidates) {
		return "", fmt.Errorf("selection %d is out of bounds", choice)
	}
	return candidates[choice], nil
}

func promptReleaseVersion(finder Finder) string {
	versioned, ok := finder.(versionedFinder)
	if !ok {
		return ""
	}
	return versioned.ReleaseVersion()
}

func candidatePromptTitle(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return "Select package resource"
	}
	return "Select package resource " + version
}

func uniqueCandidateForName(candidates []string, name string) string {
	hint := normalizedAssetNameHint(name)
	if hint == "" {
		return ""
	}

	match := ""
	for _, candidate := range candidates {
		if !assetBaseMatchesName(path.Base(candidate), hint) {
			continue
		}
		if match != "" {
			return ""
		}
		match = candidate
	}
	return match
}

func uniqueCandidateForPreviousAsset(candidates []string, previous string) string {
	previousSignature := assetSelectionSignature(previous)
	if previousSignature == "" {
		return ""
	}
	match := ""
	for _, candidate := range candidates {
		if assetSelectionSignature(path.Base(candidate)) != previousSignature {
			continue
		}
		if match != "" {
			return ""
		}
		match = candidate
	}
	return match
}

func assetSelectionSignature(name string) string {
	tokens := platformTokens(name)
	kept := make([]string, 0, len(tokens))
	seen := make(map[string]bool, len(tokens))
	edges := assetSelectionVersionEdges(name)
	for _, token := range tokens {
		if token == "" || isAssetSelectionVersionToken(token, edges) {
			continue
		}
		if seen[token] {
			continue
		}
		seen[token] = true
		kept = append(kept, token)
	}
	return strings.Join(kept, "\x00")
}

type assetVersionEdges struct {
	start map[string]bool
	end   map[string]bool
}

var assetVersionPattern = regexp.MustCompile(`v?\d+(?:[._]\d+)+`)

func assetSelectionVersionEdges(name string) assetVersionEdges {
	edges := assetVersionEdges{
		start: map[string]bool{},
		end:   map[string]bool{},
	}
	for _, match := range assetVersionPattern.FindAllString(strings.ToLower(name), -1) {
		parts := strings.FieldsFunc(match, func(r rune) bool {
			return r == '.' || r == '_'
		})
		if len(parts) == 0 {
			continue
		}
		edges.start[parts[0]] = true
		edges.end[parts[len(parts)-1]] = true
	}
	return edges
}

func isAssetSelectionVersionToken(token string, edges assetVersionEdges) bool {
	if versionTokenPattern.MatchString(token) {
		return true
	}
	for first := range edges.start {
		if strings.HasSuffix(token, "_"+first) {
			return true
		}
	}
	for last := range edges.end {
		if strings.HasPrefix(token, last+"_") {
			return true
		}
	}
	return false
}

func normalizedAssetNameHint(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		return ""
	}
	for _, suffix := range []string{".exe", ".appimage"} {
		name = strings.TrimSuffix(name, suffix)
	}
	return name
}

func assetBaseMatchesName(base, hint string) bool {
	base = strings.ToLower(base)
	if base == hint {
		return true
	}
	if len(base) <= len(hint) || !strings.HasPrefix(base, hint) {
		return false
	}
	switch base[len(hint)] {
	case '-', '_', '.':
		return true
	default:
		return false
	}
}

func (r *InstallRunner) resolveExtractedFile(candidates []ExtractedFile, opts Options) (ExtractedFile, bool, error) {
	goos, goarch := selectionPlatform(opts)
	if selected, ok := autoSelectExtractedFile(candidates, goos, goarch); ok {
		if r.Stderr != nil {
			ccolor.Fprintf(r.Stderr, "🪄 <yellow>Auto-selected extracted file '%s' for %s/%s</>\n", selected.ArchiveName, goos, goarch)
		}
		return selected, false, nil
	}

	if r.Prompt == nil {
		return ExtractedFile{}, false, fmt.Errorf("%d candidates for target found", len(candidates))
	}
	choices := make([]string, len(candidates)+1)
	for i := range candidates {
		choices[i] = candidates[i].String()
	}
	choices[len(candidates)] = "all"
	choice, err := r.Prompt("Select extracted file", "Filter files", choices)
	if err != nil {
		return ExtractedFile{}, false, err
	}
	if choice == len(candidates) {
		return ExtractedFile{}, true, nil
	}
	if choice < 0 || choice >= len(candidates) {
		return ExtractedFile{}, false, fmt.Errorf("selection %d is out of bounds", choice)
	}
	return candidates[choice], false, nil
}
