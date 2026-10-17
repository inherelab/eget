package detect

import (
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/inherelab/eget/internal/filekind"
	"github.com/inherelab/eget/internal/platform"
)

type detectorChain struct {
	detectors []Detector
	system    Detector
}

type Detector interface {
	Detect(assets []string) (string, []string, error)
}

type assetDetector struct {
	Asset string
	Anti  bool
	Regex *regexp.Regexp
}

type allDetector struct{}

type systemOS struct {
	name     string
	regex    *regexp.Regexp
	anti     *regexp.Regexp
	priority *regexp.Regexp
}

type systemArch struct {
	name  string
	regex *regexp.Regexp
}

type systemDetector struct {
	Os   systemOS
	Arch systemArch
	Libc string
}

type releaseAssetKind int

const (
	releaseAssetInstallable releaseAssetKind = iota
	releaseAssetChecksum
	releaseAssetSignature
	releaseAssetMetadata
)

func (dc *detectorChain) Detect(assets []string) (string, []string, error) {
	for _, d := range dc.detectors {
		choice, candidates, err := d.Detect(assets)
		if len(candidates) == 0 && err != nil {
			return "", nil, err
		}
		if len(candidates) == 0 {
			return choice, nil, nil
		}
		assets = candidates
	}
	choice, candidates, err := dc.system.Detect(assets)
	if len(candidates) == 0 && err != nil {
		return "", nil, err
	}
	if len(candidates) == 0 {
		return choice, nil, nil
	}
	return "", candidates, fmt.Errorf("%d candidates found for asset chain", len(candidates))
}

func NewChain(detectors []Detector, system Detector) Detector {
	return &detectorChain{detectors: detectors, system: system}
}

func (a *allDetector) Detect(assets []string) (string, []string, error) {
	assets = SelectableReleaseAssets(assets)
	if len(assets) == 1 {
		return assets[0], nil, nil
	}
	return "", assets, fmt.Errorf("%d matches found", len(assets))
}

func NewAllDetector() Detector {
	return &allDetector{}
}

func (s *assetDetector) Detect(assets []string) (string, []string, error) {
	var candidates []string
	for _, a := range assets {
		if isReleaseMetadataAsset(a) {
			continue
		}
		base := path.Base(a)
		if !s.Anti && base == s.Asset {
			return a, nil, nil
		}
		if !s.Anti {
			if s.matches(base) {
				candidates = append(candidates, a)
			}
		}
		if s.Anti && !s.matches(base) {
			candidates = append(candidates, a)
		}
	}
	if len(candidates) == 1 {
		return candidates[0], nil, nil
	}
	if len(candidates) > 1 {
		return "", candidates, fmt.Errorf("%d candidates found for asset `%s`", len(candidates), s.Asset)
	}
	return "", nil, fmt.Errorf("asset `%s` not found", s.Asset)
}

func NewAssetDetector(asset string, anti bool, re *regexp.Regexp) Detector {
	return &assetDetector{Asset: asset, Anti: anti, Regex: re}
}

func (s *assetDetector) matches(base string) bool {
	if s.Regex != nil {
		return s.Regex.MatchString(base)
	}
	return strings.Contains(strings.ToLower(base), strings.ToLower(s.Asset))
}

func CompileAssetRegex(expr string) (*regexp.Regexp, error) {
	return regexp.Compile(expr)
}

func (osv *systemOS) Match(s string) (bool, bool) {
	if osv.anti != nil && osv.anti.MatchString(s) {
		return false, false
	}
	if osv.priority != nil {
		return osv.regex.MatchString(s), osv.priority.MatchString(s)
	}
	return osv.regex.MatchString(s), false
}

func (a *systemArch) Match(s string) bool {
	return a.regex.MatchString(s)
}

func NewSystemDetector(goos, goarch string) (Detector, error) {
	return NewSystemDetectorWithLibc(goos, goarch, "")
}

func NewSystemDetectorWithLibc(goos, goarch, libc string) (Detector, error) {
	osv, ok := installGOOSMap[goos]
	if !ok {
		return nil, fmt.Errorf("unsupported target OS: %s", goos)
	}
	arch, ok := installGOARCHMap[goarch]
	if !ok {
		return nil, fmt.Errorf("unsupported target arch: %s", goarch)
	}
	return &systemDetector{Os: osv, Arch: arch, Libc: libc}, nil
}

func (d *systemDetector) Detect(assets []string) (string, []string, error) {
	var priority []string
	var matches []string
	var candidates []string
	all := make([]string, 0, len(assets))
	assets = SelectableReleaseAssets(assets)
	for _, a := range assets {
		osMatch, extra := d.Os.Match(a)
		archMatch := d.Arch.Match(a)
		if extra && archMatch {
			priority = append(priority, a)
		}
		if osMatch && archMatch {
			matches = append(matches, a)
		}
		if osMatch {
			candidates = append(candidates, a)
		}
		all = append(all, a)
	}
	if len(priority) == 1 {
		return priority[0], nil, nil
	}
	if len(priority) > 1 {
		return "", matches, fmt.Errorf("%d matches found", len(matches))
	}
	if len(matches) == 1 {
		return matches[0], nil, nil
	}
	if len(matches) > 1 {
		if selected := selectLinuxLibcAsset(matches, d.Libc); selected != "" {
			return selected, nil, nil
		}
		return "", matches, fmt.Errorf("%d matches found", len(matches))
	}
	if len(candidates) == 1 {
		return candidates[0], nil, nil
	}
	if len(candidates) > 1 {
		return "", candidates, fmt.Errorf("%d candidates found (unsure architecture)", len(candidates))
	}
	if len(all) == 1 {
		return all[0], nil, nil
	}
	return "", all, fmt.Errorf("no candidates found")
}

func selectLinuxLibcAsset(matches []string, libc string) string {
	if libc == "" {
		return ""
	}
	libc = strings.ToLower(libc)
	want := "gnu"
	if libc == "musl" {
		want = "musl"
	}
	var selected string
	for _, match := range matches {
		name := strings.ToLower(path.Base(match))
		if !assetHasLibcToken(name, want) {
			continue
		}
		if selected != "" {
			return ""
		}
		selected = match
	}
	return selected
}

func assetHasLibcToken(name, libc string) bool {
	for _, token := range strings.FieldsFunc(name, func(r rune) bool {
		return r == '-' || r == '_' || r == '.' || r == '/'
	}) {
		if token == libc {
			return true
		}
	}
	return false
}

func isReleaseMetadataAsset(asset string) bool {
	return classifyReleaseAsset(asset) != releaseAssetInstallable
}

func SelectableReleaseAssets(assets []string) []string {
	selectable := make([]string, 0, len(assets))
	for _, asset := range assets {
		if !isReleaseMetadataAsset(asset) {
			selectable = append(selectable, asset)
		}
	}
	return selectable
}

func classifyReleaseAsset(asset string) releaseAssetKind {
	switch filekind.ClassifyAsset(asset) {
	case filekind.AssetMetadata:
		return releaseAssetMetadata
	case filekind.AssetChecksum:
		return releaseAssetChecksum
	case filekind.AssetSignature:
		return releaseAssetSignature
	default:
		return releaseAssetInstallable
	}
}

var (
	installGOOSMap   = buildGOOSMap()
	installGOARCHMap = buildGOARCHMap()
)

// systemOSFrom / systemArchFrom adapt the shared recognition table
// (internal/platform) to the local types; the anti/priority policy stays in
// systemOS.Match.
func systemOSFrom(spec platform.OSSpec) systemOS {
	return systemOS{name: spec.Name, regex: spec.Regex, anti: spec.Anti, priority: spec.Priority}
}

func systemArchFrom(spec platform.ArchSpec) systemArch {
	return systemArch{name: spec.Name, regex: spec.Regex}
}

func buildGOOSMap() map[string]systemOS {
	m := make(map[string]systemOS, len(platform.GOOSSpecIndex))
	for name, spec := range platform.GOOSSpecIndex {
		m[name] = systemOSFrom(spec)
	}
	return m
}

func buildGOARCHMap() map[string]systemArch {
	m := make(map[string]systemArch, len(platform.GOARCHSpecIndex))
	for name, spec := range platform.GOARCHSpecIndex {
		m[name] = systemArchFrom(spec)
	}
	return m
}

func IsKnownGOOS(value string) bool {
	_, ok := installGOOSMap[value]
	return ok
}
