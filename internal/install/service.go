package install

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/inherelab/eget/internal/install/detect"
	forge "github.com/inherelab/eget/internal/source/forge"
	sourcegithub "github.com/inherelab/eget/internal/source/github"
	"github.com/inherelab/eget/internal/source/pkgtemplate"
	sourcesf "github.com/inherelab/eget/internal/source/sourceforge"
	"github.com/inherelab/eget/internal/source/urltemplate"
	"github.com/inherelab/eget/internal/util"
)

type Finder interface {
	Find() ([]string, error)
}

type VersionFallbackFinder interface {
	FallbackVersionAssets(limit int) ([][]string, error)
}

type Detector = detect.Detector

type Verifier interface {
	Verify(b []byte) error
}

type DirectAssetFinder struct {
	URL string
}

func (f *DirectAssetFinder) Find() ([]string, error) {
	return []string{f.URL}, nil
}

type HTTPGetterFunc func(url string) (*http.Response, error)

func (f HTTPGetterFunc) Get(url string) (*http.Response, error) {
	return f(url)
}

type Service struct {
	BinaryModTime            func(tool, output string) time.Time
	GitHubGetter             sourcegithub.HTTPGetter
	GitHubGetterFactory      func(opts Options) sourcegithub.HTTPGetter
	ForgeGetterFactory       func(opts Options) forge.HTTPGetter
	SourceForgeGetterFactory func(opts Options) sourcesf.HTTPGetter
	TemplateGetterFactory    func(opts Options) urltemplate.HTTPGetter

	AllDetectorFactory    func() Detector
	SystemDetectorFactory func(goos, goarch string) (Detector, error)
	AssetDetectorFactory  func(asset string, anti bool, re *regexp.Regexp) Detector
	DetectorChainFactory  func(detectors []Detector, system Detector) Detector

	Sha256VerifierFactory      func(expected string) (Verifier, error)
	Sha256AssetVerifierFactory func(assetURL string, opts Options) Verifier
	Sha256PrinterFactory       func() Verifier
	NoVerifierFactory          func() Verifier

	DownloadOnlyExtractorFactory func(name string) any
	GlobChooserFactory           func(pattern string) (any, error)
	BinaryChooserFactory         func(tool string) any
	ExtractorFactory             func(filename, tool string, chooser any) any
	System7zPathResolver         func(configured string) string
	System7zExtractorFactory     func(filename, tool string, chooser Chooser, exe string) Extractor
}

func NewService() *Service {
	return &Service{}
}

func Cast[T any](value any) (T, error) {
	typed, ok := value.(T)
	if !ok {
		var zero T
		return zero, fmt.Errorf("unexpected type %T", value)
	}
	return typed, nil
}

func SelectExtractorAs[T any](s *Service, url, tool string, opts *Options) (T, error) {
	value, err := s.SelectExtractor(url, tool, opts)
	if err != nil {
		var zero T
		return zero, err
	}
	return Cast[T](value)
}

func (s *Service) SelectFinder(target string, opts *Options) (Finder, string, error) {
	switch DetectTargetKind(target) {
	case TargetLocalFile, TargetDirectURL:
		if opts.System == "" {
			opts.System = "all"
		}
		return &DirectAssetFinder{URL: target}, "", nil
	case TargetGitHubURL, TargetRepo:
		repo, err := NormalizeRepoTarget(target)
		if err != nil {
			return nil, "", err
		}

		parts := strings.Split(repo, "/")
		tool := parts[1]

		if opts.Source {
			tag := "master"
			if opts.Tag != "" {
				tag = opts.Tag
			}
			return sourcegithub.NewSourceFinder(repo, tag, tool), tool, nil
		}

		tag := "latest"
		if opts.Tag != "" {
			tag = fmt.Sprintf("tags/%s", opts.Tag)
		}

		var minTime time.Time
		if opts.UpgradeOnly && s.BinaryModTime != nil {
			minTime = s.BinaryModTime(tool, opts.Output)
		}

		finder := sourcegithub.NewAssetFinder(repo, tag, opts.Prerelease, minTime)
		if s.GitHubGetterFactory != nil {
			finder.Getter = s.GitHubGetterFactory(*opts)
		} else {
			finder.Getter = s.GitHubGetter
		}
		return finder, tool, nil
	case TargetSourceForge:
		sfTarget, err := sourcesf.ParseTarget(target)
		if err != nil {
			return nil, "", err
		}

		sourcePath := strings.Trim(opts.SourcePath, "/")
		targetPath := strings.Trim(sfTarget.Path, "/")
		if sourcePath != "" && targetPath != "" && sourcePath != targetPath {
			return nil, "", fmt.Errorf("source_path %q conflicts with target path %q", sourcePath, targetPath)
		}
		if sourcePath == "" {
			sourcePath = targetPath
		}
		if s.SourceForgeGetterFactory == nil {
			return nil, "", fmt.Errorf("sourceforge getter factory is required")
		}

		return sourcesf.Finder{
			Project: sfTarget.Project,
			Path:    sourcePath,
			Tag:     opts.Tag,
			Getter:  s.SourceForgeGetterFactory(*opts),
		}, sfTarget.Project, nil
	case TargetForge:
		forgeTarget, err := forge.ParseTarget(target)
		if err != nil {
			return nil, "", err
		}
		if s.ForgeGetterFactory == nil {
			return nil, "", fmt.Errorf("forge getter factory is required")
		}

		return forge.Finder{
			Target: forgeTarget,
			Tag:    opts.Tag,
			Getter: s.ForgeGetterFactory(*opts),
		}, forgeTarget.Project, nil
	case TargetTemplate:
		templateTarget, err := urltemplate.ParseTarget(target)
		if err != nil {
			return nil, "", err
		}
		getter := urltemplate.HTTPGetter(NewHTTPGetter(*opts))
		if s.TemplateGetterFactory != nil {
			getter = s.TemplateGetterFactory(*opts)
		}
		goos, goarch, libc := urltemplate.EffectiveSystem(opts.System, runtime.GOOS, runtime.GOARCH, urltemplate.DetectLibc, urltemplate.FixDarwinRosetta)
		return &urltemplate.Finder{
			Name:   templateTarget.ID,
			Target: templateTarget,
			Config: urlTemplateConfigFromOptions(opts.URLTemplate),
			Tag:    opts.Tag,
			GOOS:   goos,
			GOARCH: goarch,
			Libc:   libc,
			Getter: getter,
		}, templateTarget.ID, nil
	case TargetPkgTemplate:
		templateTarget, err := pkgtemplate.ParseTarget(target)
		if err != nil {
			return nil, "", err
		}
		getter := urltemplate.HTTPGetter(NewHTTPGetter(*opts))
		if s.TemplateGetterFactory != nil {
			getter = s.TemplateGetterFactory(*opts)
		}
		goos, goarch, libc := urltemplate.EffectiveSystem(opts.System, runtime.GOOS, runtime.GOARCH, urltemplate.DetectLibc, urltemplate.FixDarwinRosetta)
		return &urltemplate.Finder{
			Name:   templateTarget.Package,
			Config: urlTemplateConfigFromOptions(opts.URLTemplate),
			Tag:    opts.Tag,
			GOOS:   goos,
			GOARCH: goarch,
			Libc:   libc,
			Getter: getter,
		}, templateTarget.Package, nil
	default:
		return nil, "", fmt.Errorf("invalid argument (must be of the form `user/repo`)")
	}
}

func urlTemplateConfigFromOptions(opts URLTemplateOptions) urltemplate.Config {
	return urltemplate.Config{
		URLTemplate:         opts.URLTemplate,
		LatestURL:           opts.LatestURL,
		LatestFormat:        opts.LatestFormat,
		LatestJSONPath:      opts.LatestJSONPath,
		VersionRegex:        opts.VersionRegex,
		OSMap:               util.CloneStringMap(opts.OSMap),
		ArchMap:             util.CloneStringMap(opts.ArchMap),
		ExtMap:              util.CloneStringMap(opts.ExtMap),
		LibcMap:             util.CloneStringMap(opts.LibcMap),
		ChecksumURLTemplate: opts.ChecksumURLTemplate,
		ChecksumFormat:      opts.ChecksumFormat,
		ChecksumJSONPath:    opts.ChecksumJSONPath,
		ChecksumRegex:       opts.ChecksumRegex,
		InstallAction:       opts.InstallAction,
		InstallArgs:         append([]string(nil), opts.InstallArgs...),
	}
}

func (s *Service) SelectDetector(opts *Options) (Detector, error) {
	var system Detector
	targetGOOS := runtime.GOOS
	switch {
	case opts.System == "all":
		if s.AllDetectorFactory == nil {
			return nil, fmt.Errorf("all detector factory is required")
		}
		system = s.AllDetectorFactory()
	case opts.System != "":
		if s.SystemDetectorFactory == nil {
			return nil, fmt.Errorf("system detector factory is required")
		}
		split := strings.Split(opts.System, "/")
		if len(split) < 2 {
			return nil, fmt.Errorf("system descriptor must be os/arch")
		}
		targetGOOS = split[0]
		detector, err := s.SystemDetectorFactory(split[0], split[1])
		if err != nil {
			return nil, err
		}
		system = detector
	default:
		if s.SystemDetectorFactory == nil {
			return nil, fmt.Errorf("system detector factory is required")
		}
		detector, err := s.SystemDetectorFactory(runtime.GOOS, runtime.GOARCH)
		if err != nil {
			return nil, err
		}
		system = detector
	}

	if len(opts.Asset) == 0 {
		return system, nil
	}
	if s.AssetDetectorFactory == nil || s.DetectorChainFactory == nil {
		return nil, fmt.Errorf("asset detector factories are required")
	}

	detectors := make([]Detector, 0, len(opts.Asset))
	for _, asset := range opts.Asset {
		expr, ok := assetFilterForGOOS(asset, targetGOOS)
		if !ok {
			continue
		}
		filter, err := parseAssetFilter(expr)
		if err != nil {
			return nil, err
		}
		detectors = append(detectors, s.AssetDetectorFactory(filter.Expr, filter.Anti, filter.Regex))
	}
	if len(detectors) == 0 {
		return system, nil
	}
	return s.DetectorChainFactory(detectors, system), nil
}

func assetFilterForGOOS(raw, goos string) (string, bool) {
	prefix, expr, found := strings.Cut(raw, ":")
	if !found || !isKnownGOOS(prefix) {
		return raw, true
	}
	return expr, prefix == goos
}

func isKnownGOOS(value string) bool {
	return detect.IsKnownGOOS(value)
}

type assetFilter struct {
	Expr  string
	Anti  bool
	Regex *regexp.Regexp
}

func parseAssetFilter(raw string) (assetFilter, error) {
	filter := assetFilter{}
	if strings.HasPrefix(raw, "^") {
		filter.Anti = true
		raw = raw[1:]
	}
	if strings.HasPrefix(raw, "REG:") {
		expr := strings.TrimPrefix(raw, "REG:")
		re, err := regexp.Compile(expr)
		if err != nil {
			return assetFilter{}, fmt.Errorf("invalid asset regex %q: %w", expr, err)
		}
		filter.Expr = expr
		filter.Regex = re
		return filter, nil
	}
	if strings.HasPrefix(raw, "PRE:") {
		expr := strings.TrimPrefix(raw, "PRE:")
		filter.Expr = expr
		filter.Regex = regexp.MustCompile(`(?i)^` + regexp.QuoteMeta(expr))
		return filter, nil
	}
	if strings.HasPrefix(raw, "SUF:") {
		expr := strings.TrimPrefix(raw, "SUF:")
		filter.Expr = expr
		filter.Regex = regexp.MustCompile(`(?i)` + regexp.QuoteMeta(expr) + `$`)
		return filter, nil
	}
	filter.Expr = raw
	return filter, nil
}

func (s *Service) SelectVerifier(sumAsset string, opts *Options) (Verifier, error) {
	switch {
	case opts.Verify != "":
		if s.Sha256VerifierFactory == nil {
			return nil, fmt.Errorf("sha256 verifier factory is required")
		}
		return s.Sha256VerifierFactory(opts.Verify)
	case sumAsset != "":
		if s.Sha256AssetVerifierFactory == nil {
			return nil, fmt.Errorf("sha256 asset verifier factory is required")
		}
		return s.Sha256AssetVerifierFactory(sumAsset, *opts), nil
	case opts.URLTemplate.ChecksumURLTemplate != "":
		if s.Sha256VerifierFactory == nil {
			return nil, fmt.Errorf("sha256 verifier factory is required")
		}
		checksum, err := s.resolveTemplateChecksum(opts)
		if err != nil {
			return nil, err
		}
		return s.Sha256VerifierFactory(checksum)
	case opts.Hash:
		if s.Sha256PrinterFactory == nil {
			return nil, fmt.Errorf("sha256 printer factory is required")
		}
		return s.Sha256PrinterFactory(), nil
	default:
		if s.NoVerifierFactory == nil {
			return nil, fmt.Errorf("no verifier factory is required")
		}
		return s.NoVerifierFactory(), nil
	}
}

func (s *Service) resolveTemplateChecksum(opts *Options) (string, error) {
	vars := opts.URLTemplate.ResolvedVars
	if len(vars) == 0 {
		return "", fmt.Errorf("template checksum requires resolved variables")
	}
	manifestURL, err := urltemplate.Render(opts.URLTemplate.ChecksumURLTemplate, vars)
	if err != nil {
		return "", err
	}
	checksumPath := opts.URLTemplate.ChecksumJSONPath
	if checksumPath != "" {
		checksumPath, err = urltemplate.Render(checksumPath, vars)
		if err != nil {
			return "", err
		}
	}

	// Template latest/checksum URLs are arbitrary site metadata. They use the
	// shared HTTP client for proxy/SSL behavior, but do not force API-cache
	// classification because arbitrary metadata URLs are not provider APIs.
	resp, err := GetWithOptions(manifestURL, *opts)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("fetch checksum metadata: %s", resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return urltemplate.ParseChecksum(data, urltemplate.Config{
		ChecksumFormat:   opts.URLTemplate.ChecksumFormat,
		ChecksumJSONPath: checksumPath,
		ChecksumRegex:    opts.URLTemplate.ChecksumRegex,
	})
}

func (s *Service) SelectExtractor(url, tool string, opts *Options) (any, error) {
	filename := assetFilename(url)
	if opts.InstallMode == InstallModeInstaller || (opts.DownloadOnly && opts.ExtractFile == "" && !opts.All) {
		if s.DownloadOnlyExtractorFactory == nil {
			return nil, fmt.Errorf("download-only extractor factory is required")
		}
		return s.DownloadOnlyExtractorFactory(filename), nil
	}

	if opts.ExtractFile != "" {
		if s.GlobChooserFactory == nil || s.ExtractorFactory == nil {
			return nil, fmt.Errorf("extractor factories are required")
		}
		chooser, err := s.GlobChooserFactory(opts.ExtractFile)
		if err != nil {
			return nil, err
		}
		return s.newExtractor(filename, tool, chooser, opts)
	}

	if opts.All {
		if s.GlobChooserFactory == nil || s.ExtractorFactory == nil {
			return nil, fmt.Errorf("extractor factories are required")
		}
		chooser, err := s.GlobChooserFactory("*")
		if err != nil {
			return nil, err
		}
		return s.newExtractor(filename, tool, chooser, opts)
	}

	if s.BinaryChooserFactory == nil || s.ExtractorFactory == nil {
		return nil, fmt.Errorf("extractor factories are required")
	}
	return s.newExtractor(filename, tool, s.BinaryChooserFactory(tool), opts)
}

func assetFilename(raw string) string {
	u, err := url.Parse(raw)
	if err == nil && u.Path != "" {
		return path.Base(u.Path)
	}
	return path.Base(raw)
}

func (s *Service) newExtractor(filename, tool string, chooser any, opts *Options) (any, error) {
	extractArchive := opts != nil && (opts.All || opts.ExtractFile != "")
	if opts != nil && shouldUseSystem7z(filename, extractArchive) {
		resolver := s.System7zPathResolver
		if resolver == nil {
			resolver = resolveSystem7zPath
		}
		if exe := resolver(opts.Sys7zPath); exe != "" {
			typedChooser, ok := chooser.(Chooser)
			if !ok {
				return s.ExtractorFactory(filename, tool, chooser), nil
			}
			factory := s.System7zExtractorFactory
			if factory == nil {
				factory = func(filename, tool string, chooser Chooser, exe string) Extractor {
					return NewSystem7zExtractor(filename, tool, chooser, exe)
				}
			}
			return factory(filename, tool, typedChooser, exe), nil
		}
		if requiresSystem7z(filename, extractArchive) {
			return nil, fmt.Errorf("system 7z is required to extract files from %s", filename)
		}
	}
	if s.ExtractorFactory == nil {
		return nil, fmt.Errorf("extractor factories are required")
	}
	return s.ExtractorFactory(filename, tool, chooser), nil
}

func requiresSystem7z(filename string, extractArchive bool) bool {
	name := strings.ToLower(path.Base(filename))
	return strings.HasSuffix(name, ".exe") && extractArchive
}

func NormalizeRepoTarget(target string) (string, error) {
	switch DetectTargetKind(target) {
	case TargetRepo:
		return validateRepo(target)
	case TargetGitHubURL:
		before, after, found := strings.Cut(target, "github.com/")
		_ = before
		if !found {
			return "", fmt.Errorf("invalid GitHub repo URL %s", target)
		}
		return validateRepo(strings.Trim(after, "/"))
	default:
		return "", fmt.Errorf("invalid argument (must be of the form `user/repo`)")
	}
}

func validateRepo(repo string) (string, error) {
	parts := strings.Split(repo, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", fmt.Errorf("invalid argument (must be of the form `user/repo`)")
	}
	return repo, nil
}
