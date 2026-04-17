package main

import (
	"net/http"
	"time"

	sourcegithub "github.com/inherelab/eget/internal/source/github"
)

// A Finder returns a list of URLs making up a project's assets.
type Finder interface {
	Find() ([]string, error)
}

type GithubAssetFinder = sourcegithub.AssetFinder
type GithubSourceFinder = sourcegithub.SourceFinder
type GithubRelease = sourcegithub.Release
type GithubError = sourcegithub.Error

var ErrNoUpgrade = sourcegithub.ErrNoUpgrade

func NewGithubAssetFinder(repo, tag string, prerelease bool, minTime time.Time) *GithubAssetFinder {
	finder := sourcegithub.NewAssetFinder(repo, tag, prerelease, minTime)
	finder.Getter = githubGetter{}
	return finder
}

func NewGithubSourceFinder(repo, tag, tool string) *GithubSourceFinder {
	return sourcegithub.NewSourceFinder(repo, tag, tool)
}

// A DirectAssetFinder returns the embedded URL directly as the only asset.
type DirectAssetFinder struct {
	URL string
}

func (f *DirectAssetFinder) Find() ([]string, error) {
	return []string{f.URL}, nil
}

type githubGetter struct{}

func (githubGetter) Get(url string) (*http.Response, error) {
	return Get(url)
}
