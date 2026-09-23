package install

import (
	"context"
	"io"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/inherelab/eget/internal/cachemirror"
	"github.com/inherelab/eget/internal/source/forge"
	"github.com/inherelab/eget/internal/source/pkgtemplate"
	"github.com/inherelab/eget/internal/source/sourceforge"
	"github.com/inherelab/eget/internal/source/urltemplate"
)

type Options struct {
	Tag                 string
	TagPolicy           string
	Prerelease          bool
	Operation           string
	CurrentVersion      string
	TargetVersion       string
	Name                string
	Verbose             bool
	Source              bool
	SourcePath          string
	Sys7zPath           string
	Output              string
	OutputExplicit      bool
	GuiTarget           string
	IsGUI               bool
	InstallMode         string
	CacheDir            string
	CacheName           string
	CacheVersion        string
	ProxyURL            string
	ProxyExclude        []string
	NoProxy             bool
	UserAgent           string
	APICacheEnabled     bool
	APICacheDir         string
	APICacheTime        int
	CacheMirror         cachemirror.Options
	GhproxyEnabled      bool
	GhproxyHostURL      string
	GhproxyFallbacks    []string
	System              string
	ExtractFile         string
	All                 bool
	StripComponents     int
	Quiet               bool
	DownloadOnly        bool
	FallbackVersions    int
	Retries             int
	ChunkConcurrency    int
	BatchConcurrency    int
	ChunkConcurrencySet bool
	BatchConcurrencySet bool
	UpgradeOnly         bool
	Asset               []string
	RenameFiles         map[string]string
	Hash                bool
	Verify              string
	URLTemplate         URLTemplateOptions
	DisableSSL          bool
	// Context cancels the run at its checkpoints; the web console cancels
	// tasks with it. A nil context means "no cancellation".
	Context context.Context
	// Progress, when set, replaces the terminal progress bar. It receives the
	// total size and returns the writer that downloaded bytes are reported to;
	// returning a write error aborts the transfer, which is how cancellation
	// reaches an in-flight download.
	Progress func(total int64) io.Writer
}

// contextErr reports cancellation at a checkpoint.
func (o Options) contextErr() error {
	if o.Context == nil {
		return nil
	}
	return o.Context.Err()
}

type URLTemplateOptions struct {
	URLTemplate         string
	LatestURL           string
	LatestFormat        string
	LatestJSONPath      string
	VersionRegex        string
	OSMap               map[string]string
	ArchMap             map[string]string
	ExtMap              map[string]string
	LibcMap             map[string]string
	ChecksumURLTemplate string
	ChecksumFormat      string
	ChecksumJSONPath    string
	ChecksumRegex       string
	InstallAction       string
	InstallArgs         []string
	ResolvedVersion     string
	ResolvedVars        map[string]string
}

const (
	OperationInstall      = "install"
	OperationUpdate       = "update"
	InstallModePortable   = "portable"
	InstallModeInstaller  = "installer"
	InstallModeRunAsset   = "run-asset"
	InstallActionRunAsset = "run-asset"
)

type TargetKind string

const (
	TargetUnknown     TargetKind = "unknown"
	TargetRepo        TargetKind = "repo"
	TargetGitHubURL   TargetKind = "github_url"
	TargetDirectURL   TargetKind = "direct_url"
	TargetLocalFile   TargetKind = "local_file"
	TargetSourceForge TargetKind = "sourceforge"
	TargetForge       TargetKind = "forge"
	TargetTemplate    TargetKind = "template"
	TargetPkgTemplate TargetKind = "pkg_template"
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
	case sourceforge.IsTarget(target):
		return TargetSourceForge
	case forge.IsTarget(target):
		return TargetForge
	case pkgtemplate.IsTarget(target):
		return TargetPkgTemplate
	case urltemplate.IsTarget(target):
		return TargetTemplate
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

func TargetKindDisplayName(kind TargetKind) string {
	switch kind {
	case TargetRepo, TargetGitHubURL:
		return "github"
	case TargetPkgTemplate:
		return "pkg-template"
	default:
		return string(kind)
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
