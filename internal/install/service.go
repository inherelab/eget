package install

import (
	"fmt"
	"net/http"
	"path"
	"regexp"
	"runtime"
	"strings"
	"time"

	sourcegithub "github.com/inherelab/eget/internal/source/github"
)

type Finder interface {
	Find() ([]string, error)
}

type Detector interface {
	Detect(assets []string) (string, []string, error)
}

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
	BinaryModTime       func(tool, output string) time.Time
	GitHubGetter        sourcegithub.HTTPGetter
	GitHubGetterFactory func(opts Options) sourcegithub.HTTPGetter

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
		opts.System = "all"
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
	default:
		return nil, "", fmt.Errorf("invalid argument (must be of the form `user/repo`)")
	}
}

func (s *Service) SelectDetector(opts *Options) (Detector, error) {
	var system Detector
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

	detectors := make([]Detector, len(opts.Asset))
	for i, asset := range opts.Asset {
		filter, err := parseAssetFilter(asset)
		if err != nil {
			return nil, err
		}
		detectors[i] = s.AssetDetectorFactory(filter.Expr, filter.Anti, filter.Regex)
	}
	return s.DetectorChainFactory(detectors, system), nil
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

func (s *Service) SelectExtractor(url, tool string, opts *Options) (any, error) {
	filename := path.Base(url)
	if opts.DownloadOnly && opts.ExtractFile == "" && !opts.All {
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
		return s.ExtractorFactory(filename, tool, chooser), nil
	}

	if opts.All {
		if s.GlobChooserFactory == nil || s.ExtractorFactory == nil {
			return nil, fmt.Errorf("extractor factories are required")
		}
		chooser, err := s.GlobChooserFactory("*")
		if err != nil {
			return nil, err
		}
		return s.ExtractorFactory(filename, tool, chooser), nil
	}

	if s.BinaryChooserFactory == nil || s.ExtractorFactory == nil {
		return nil, fmt.Errorf("extractor factories are required")
	}
	return s.ExtractorFactory(filename, tool, s.BinaryChooserFactory(tool)), nil
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
