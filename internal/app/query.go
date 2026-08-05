package app

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/inherelab/eget/internal/client"
	"github.com/inherelab/eget/internal/install"
	sourcesf "github.com/inherelab/eget/internal/source/sourceforge"
)

type QueryRepoInfo = client.RepoInfo
type QueryRelease = client.Release
type QueryAsset = client.Asset
type QueryURLInfo = client.URLInfo

type QueryOptions struct {
	Repo       string
	Action     string
	Tag        string
	Limit      int
	JSON       bool
	Prerelease bool
}

type QueryResult struct {
	Action   string         `json:"action"`
	Repo     string         `json:"repo"`
	Tag      string         `json:"tag,omitempty"`
	Info     *QueryRepoInfo `json:"info,omitempty"`
	Latest   *QueryRelease  `json:"latest,omitempty"`
	Releases []QueryRelease `json:"releases,omitempty"`
	Assets   []QueryAsset   `json:"assets,omitempty"`
	URLInfo  *QueryURLInfo  `json:"url_info,omitempty"`
}

type QueryClient interface {
	RepoInfo(repo string) (QueryRepoInfo, error)
	LatestRelease(repo string, includePrerelease bool) (QueryRelease, error)
	ListReleases(repo string, limit int, includePrerelease bool) ([]QueryRelease, error)
	ReleaseAssets(repo, tag string) ([]QueryAsset, error)
}

type QueryService struct {
	Client              QueryClient
	SourceForgeLatest   func(project, sourcePath string) (sourcesf.LatestInfo, error)
	SourceForgeReleases func(project, sourcePath string, limit int, includePrerelease bool) ([]sourcesf.LatestInfo, error)
	SourceForgeAssets   func(project, sourcePath, tag string) ([]string, error)
	URLInfo             func(rawURL string) (QueryURLInfo, error)
}

func (s QueryService) Query(opts QueryOptions) (QueryResult, error) {
	action := opts.Action
	if action == "" {
		action = "latest"
	}
	if install.DetectTargetKind(opts.Repo) == install.TargetDirectURL {
		if action != "latest" && action != "info" {
			return QueryResult{}, fmt.Errorf("direct URL query does not support action %q", action)
		}
		if s.URLInfo == nil {
			return QueryResult{}, fmt.Errorf("URL info query is required")
		}
		info, err := s.URLInfo(opts.Repo)
		if err != nil {
			return QueryResult{}, err
		}
		return QueryResult{Action: "info", URLInfo: &info}, nil
	}
	if sourcesf.IsTarget(opts.Repo) {
		return s.querySourceForge(opts, action)
	}

	if s.Client == nil {
		return QueryResult{}, fmt.Errorf("query client is required")
	}
	repo, err := install.NormalizeRepoTarget(opts.Repo)
	if err != nil {
		return QueryResult{}, err
	}

	switch action {
	case "latest":
		if opts.Tag != "" {
			return QueryResult{}, fmt.Errorf("query latest does not support --tag")
		}
		if opts.Limit > 0 && opts.Limit != 10 {
			return QueryResult{}, fmt.Errorf("query latest does not support --limit")
		}
		latest, err := s.Client.LatestRelease(repo, opts.Prerelease)
		if err != nil {
			return QueryResult{}, err
		}
		return QueryResult{Action: action, Repo: repo, Latest: &latest}, nil
	case "releases":
		if opts.Tag != "" {
			return QueryResult{}, fmt.Errorf("query releases does not support --tag")
		}
		limit := opts.Limit
		if limit <= 0 {
			limit = 10
		}
		releases, err := s.Client.ListReleases(repo, limit, opts.Prerelease)
		if err != nil {
			return QueryResult{}, err
		}
		return QueryResult{Action: action, Repo: repo, Releases: releases}, nil
	case "assets":
		tag := opts.Tag
		if tag == "" {
			latest, err := s.Client.LatestRelease(repo, opts.Prerelease)
			if err != nil {
				return QueryResult{}, err
			}
			tag = latest.Tag
		}
		assets, err := s.Client.ReleaseAssets(repo, tag)
		if err != nil {
			return QueryResult{}, err
		}
		return QueryResult{Action: action, Repo: repo, Tag: tag, Assets: assets}, nil
	case "info":
		if opts.Tag != "" {
			return QueryResult{}, fmt.Errorf("query info does not support --tag")
		}
		if opts.Limit > 0 && opts.Limit != 10 {
			return QueryResult{}, fmt.Errorf("query info does not support --limit")
		}
		info, err := s.Client.RepoInfo(repo)
		if err != nil {
			return QueryResult{}, err
		}
		return QueryResult{Action: action, Repo: repo, Info: &info}, nil
	default:
		return QueryResult{}, fmt.Errorf("invalid query action %q", action)
	}
}

func (s QueryService) querySourceForge(opts QueryOptions, action string) (QueryResult, error) {
	target, err := sourcesf.ParseTarget(opts.Repo)
	if err != nil {
		return QueryResult{}, err
	}
	repo := sourceForgeRepoName(target)

	switch action {
	case "latest":
		if opts.Tag != "" {
			return QueryResult{}, fmt.Errorf("query latest does not support --tag")
		}
		if opts.Limit > 0 && opts.Limit != 10 {
			return QueryResult{}, fmt.Errorf("query latest does not support --limit")
		}
		if s.SourceForgeLatest == nil {
			return QueryResult{}, fmt.Errorf("sourceforge query latest is required")
		}
		latest, err := s.SourceForgeLatest(target.Project, target.Path)
		if err != nil {
			return QueryResult{}, err
		}
		release := QueryRelease{
			Tag:         sourceForgeReleaseTag(latest),
			Name:        latest.Version,
			PublishedAt: latest.PublishedAt,
			AssetsCount: latest.AssetsCount,
		}
		return QueryResult{Action: action, Repo: repo, Latest: &release}, nil
	case "releases":
		if opts.Tag != "" {
			return QueryResult{}, fmt.Errorf("query releases does not support --tag")
		}
		if s.SourceForgeReleases == nil {
			return QueryResult{}, fmt.Errorf("sourceforge query releases is required")
		}
		limit := opts.Limit
		if limit <= 0 {
			limit = 10
		}
		infos, err := s.SourceForgeReleases(target.Project, target.Path, limit, opts.Prerelease)
		if err != nil {
			return QueryResult{}, err
		}
		releases := make([]QueryRelease, 0, len(infos))
		for _, info := range infos {
			releases = append(releases, QueryRelease{
				Tag:         sourceForgeReleaseTag(info),
				Name:        info.Version,
				PublishedAt: info.PublishedAt,
				Prerelease:  info.Prerelease,
			})
		}
		return QueryResult{Action: action, Repo: repo, Releases: releases}, nil
	case "assets":
		if s.SourceForgeAssets == nil {
			return QueryResult{}, fmt.Errorf("sourceforge query assets is required")
		}
		urls, err := s.SourceForgeAssets(target.Project, target.Path, opts.Tag)
		if err != nil {
			return QueryResult{}, err
		}
		return QueryResult{Action: action, Repo: repo, Tag: opts.Tag, Assets: sourceForgeQueryAssets(urls)}, nil
	case "info":
		return QueryResult{}, fmt.Errorf("sourceforge query does not support action %q", action)
	default:
		return QueryResult{}, fmt.Errorf("invalid query action %q", action)
	}
}

func sourceForgeReleaseTag(info sourcesf.LatestInfo) string {
	if info.Tag != "" {
		return info.Tag
	}
	return info.Version
}

func sourceForgeRepoName(target sourcesf.Target) string {
	repo := sourcesf.Prefix + target.Project
	if target.Path != "" {
		repo += "/" + target.Path
	}
	return repo
}

func sourceForgeQueryAssets(urls []string) []QueryAsset {
	assets := make([]QueryAsset, 0, len(urls))
	for _, rawURL := range urls {
		assets = append(assets, QueryAsset{
			Name: sourceForgeAssetName(rawURL),
			URL:  rawURL,
		})
	}
	return assets
}

func sourceForgeAssetName(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	name := ""
	if err == nil {
		name = path.Base(strings.TrimRight(parsed.Path, "/"))
	}
	if name == "" || name == "." || name == "/" {
		name = path.Base(strings.TrimRight(rawURL, "/"))
	}
	if unescaped, err := url.PathUnescape(name); err == nil {
		name = unescaped
	}
	return name
}

func (r QueryResult) JSONString() (string, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
