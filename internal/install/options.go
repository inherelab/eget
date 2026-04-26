package install

import (
	"net/url"
	"os"
	"regexp"
	"strings"
)

type Options struct {
	Tag               string
	Prerelease        bool
	Name              string
	Verbose           bool
	Source            bool
	Output            string
	OutputExplicit    bool
	GuiTarget         string
	IsGUI             bool
	InstallMode       string
	CacheDir          string
	ProxyURL          string
	APICacheEnabled   bool
	APICacheDir       string
	APICacheTime      int
	GhproxyEnabled    bool
	GhproxyHostURL    string
	GhproxySupportAPI bool
	GhproxyFallbacks  []string
	System            string
	ExtractFile       string
	All               bool
	Quiet             bool
	DownloadOnly      bool
	UpgradeOnly       bool
	Asset             []string
	Hash              bool
	Verify            string
	DisableSSL        bool
}

const (
	InstallModePortable  = "portable"
	InstallModeInstaller = "installer"
)

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

func extractAllFromFileSpec(file string) bool {
	for _, part := range strings.Split(file, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(file, ",") {
			return true
		}
		if strings.ContainsAny(part, "*?[{") {
			return true
		}
	}
	return false
}
