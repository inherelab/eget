package install

import (
	"net/url"
	"os"
	"regexp"
	"strings"
)

type Options struct {
	Tag          string
	Prerelease   bool
	Source       bool
	Output       string
	CacheDir     string
	ProxyURL     string
	System       string
	ExtractFile  string
	All          bool
	Quiet        bool
	DownloadOnly bool
	UpgradeOnly  bool
	Asset        []string
	Hash         bool
	Verify       string
	DisableSSL   bool
}

type TargetKind string

const (
	TargetUnknown   TargetKind = "unknown"
	TargetRepo      TargetKind = "repo"
	TargetGitHubURL TargetKind = "github_url"
	TargetDirectURL TargetKind = "direct_url"
	TargetLocalFile TargetKind = "local_file"
)

var githubURLPattern = regexp.MustCompile(`^(http(s)?://)?github\.com/[\w\-_.,]+/[\w\-_.,]+(.git)?(/)?$`)

func IsURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && u.Scheme != "" && u.Host != ""
}

func IsGitHubURL(s string) bool {
	return githubURLPattern.MatchString(s)
}

func IsLocalFile(s string) bool {
	_, err := os.Stat(s)
	return err == nil
}

func DetectTargetKind(target string) TargetKind {
	switch {
	case IsLocalFile(target):
		return TargetLocalFile
	case IsGitHubURL(target):
		return TargetGitHubURL
	case IsURL(target):
		return TargetDirectURL
	case isRepoTarget(target):
		return TargetRepo
	default:
		return TargetUnknown
	}
}

func isRepoTarget(target string) bool {
	parts := strings.Split(target, "/")
	return len(parts) == 2 && parts[0] != "" && parts[1] != ""
}
