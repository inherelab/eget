package app

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/inherelab/eget/internal/install"
)

type QueryRepoInfo struct {
	Repo          string    `json:"repo"`
	Description   string    `json:"description,omitempty"`
	Homepage      string    `json:"homepage,omitempty"`
	DefaultBranch string    `json:"default_branch,omitempty"`
	Stars         int       `json:"stars,omitempty"`
	Forks         int       `json:"forks,omitempty"`
	OpenIssues    int       `json:"open_issues,omitempty"`
	UpdatedAt     time.Time `json:"updated_at,omitempty"`
}

type QueryRelease struct {
	Tag         string    `json:"tag"`
	Name        string    `json:"name,omitempty"`
	PublishedAt time.Time `json:"published_at,omitempty"`
	Prerelease  bool      `json:"prerelease,omitempty"`
	AssetsCount int       `json:"assets_count,omitempty"`
}

type QueryAsset struct {
	Name          string    `json:"name"`
	Size          int64     `json:"size,omitempty"`
	DownloadCount int       `json:"download_count,omitempty"`
	UpdatedAt     time.Time `json:"updated_at,omitempty"`
	URL           string    `json:"url,omitempty"`
}

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
}

type QueryClient interface {
	RepoInfo(repo string) (QueryRepoInfo, error)
	LatestRelease(repo string, includePrerelease bool) (QueryRelease, error)
	ListReleases(repo string, limit int, includePrerelease bool) ([]QueryRelease, error)
	ReleaseAssets(repo, tag string) ([]QueryAsset, error)
}

type QueryService struct {
	Client QueryClient
}

func (s QueryService) Query(opts QueryOptions) (QueryResult, error) {
	if s.Client == nil {
		return QueryResult{}, fmt.Errorf("query client is required")
	}
	repo, err := install.NormalizeRepoTarget(opts.Repo)
	if err != nil {
		return QueryResult{}, err
	}
	action := opts.Action
	if action == "" {
		action = "latest"
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

func (r QueryResult) JSONString() (string, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
