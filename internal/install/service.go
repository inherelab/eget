package install

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	sourcegithub "github.com/inherelab/eget/internal/source/github"
)

type Finder interface {
	Find() ([]string, error)
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
	BinaryModTime func(tool, output string) time.Time
	GitHubGetter  sourcegithub.HTTPGetter
}

func NewService() *Service {
	return &Service{}
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
		finder.Getter = s.GitHubGetter
		return finder, tool, nil
	default:
		return nil, "", fmt.Errorf("invalid argument (must be of the form `user/repo`)")
	}
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
