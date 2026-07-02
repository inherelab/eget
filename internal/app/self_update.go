package app

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/inherelab/eget/internal/install"
	"github.com/inherelab/eget/internal/source/urltemplate"
)

const SelfUpdateRepo = "inherelab/eget"

type SelfUpdateInstaller interface {
	DownloadTarget(target string, opts install.Options) (RunResult, error)
}

type SelfUpdateOptions struct {
	CheckOnly bool
	Tag       string
	Source    string
	Asset     []string
	Install   install.Options
}

type SelfUpdateResult struct {
	CurrentVersion string
	LatestVersion  string
	Updated        bool
	Outdated       bool
	Replacement    string
	Executable     string
	Deferred       bool
}

type SelfUpdateService struct {
	CurrentVersion   string
	LatestInfo       LatestInfoFunc
	SourceLatestInfo func(source string, opts install.Options) (LatestInfo, error)
	Installer        SelfUpdateInstaller
	Replacer         ExecutableReplacer
	RuntimeGOOS      string
	RuntimeGOARCH    string
	TempDir          func() (string, error)
	ExecutablePath   func() (string, error)
}

func (s SelfUpdateService) Update(opts SelfUpdateOptions) (SelfUpdateResult, error) {
	result, err := s.versionResult(opts)
	if err != nil {
		return SelfUpdateResult{}, err
	}
	if opts.CheckOnly || !result.Outdated {
		return result, nil
	}
	if s.Installer == nil {
		return SelfUpdateResult{}, fmt.Errorf("self update installer is required")
	}

	goos, goarch := s.runtimePlatform()
	installOpts := opts.Install
	output, err := s.tempDir()
	if err != nil {
		return SelfUpdateResult{}, err
	}
	downloadTarget := SelfUpdateRepo
	installOpts.Tag = firstNonEmpty(opts.Tag, installOpts.Tag)
	installOpts.Name = selfUpdateDownloadName(goos)
	installOpts.System = selfUpdateSystem(goos, goarch)
	installOpts.Output = output
	installOpts.OutputExplicit = false
	installOpts.DownloadOnly = false
	installOpts.ExtractFile = selfUpdateArchiveFileName(goos, goarch)
	installOpts.Asset = nil
	installOpts.CacheName = "eget"
	installOpts.CacheVersion = result.LatestVersion
	if len(opts.Asset) > 0 {
		installOpts.Asset = append([]string(nil), opts.Asset...)
	}
	if opts.Source != "" {
		downloadTarget = selfUpdateSourceAssetURL(opts.Source, goos, goarch)
		installOpts.System = "all"
		installOpts.Asset = nil
		installOpts.Tag = ""
		installOpts.DownloadOnly = true
		installOpts.ExtractFile = ""
	}

	downloaded, err := s.Installer.DownloadTarget(downloadTarget, installOpts)
	if err != nil {
		return SelfUpdateResult{}, err
	}
	replacement, err := singleSelfUpdateReplacement(downloaded, selfUpdateExecutableName(goos))
	if err != nil {
		return SelfUpdateResult{}, err
	}
	executable, err := s.executablePath()
	if err != nil {
		return SelfUpdateResult{}, err
	}
	replaceResult, err := s.replacer().Replace(executable, replacement)
	if err != nil {
		return SelfUpdateResult{}, err
	}

	result.Replacement = replacement
	result.Executable = executable
	result.Deferred = replaceResult.Deferred
	result.Updated = true
	return result, nil
}

func (s SelfUpdateService) versionResult(opts SelfUpdateOptions) (SelfUpdateResult, error) {
	result := SelfUpdateResult{CurrentVersion: s.CurrentVersion}
	current := normalizeSelfVersion(s.CurrentVersion)
	if opts.Tag != "" {
		result.LatestVersion = opts.Tag
		result.Outdated = true
		return result, nil
	}
	if opts.Source != "" {
		latest, err := s.sourceLatestInfo(opts.Source, opts.Install)
		if err != nil {
			return SelfUpdateResult{}, err
		}
		result.LatestVersion = latest.Tag
		result.Outdated = selfVersionOutdated(current, latest.Tag)
		return result, nil
	}
	if s.LatestInfo == nil {
		return SelfUpdateResult{}, fmt.Errorf("latest info checker is required")
	}
	latest, err := s.LatestInfo(LatestCheckTarget{Repo: SelfUpdateRepo})
	if err != nil {
		return SelfUpdateResult{}, err
	}
	result.LatestVersion = latest.Tag
	result.Outdated = selfVersionOutdated(current, latest.Tag)
	return result, nil
}

func selfVersionOutdated(current, latest string) bool {
	current = normalizeSelfVersion(current)
	latest = normalizeSelfVersion(latest)
	if current == latest {
		return false
	}
	currentVersion, currentDescribe, currentOK := parseSelfVersion(current)
	latestVersion, latestDescribe, latestOK := parseSelfVersion(latest)
	if currentOK && latestOK {
		if currentVersion.semver != latestVersion.semver {
			return currentVersion.semver.lessThan(latestVersion.semver)
		}
		if latestDescribe && !currentDescribe {
			return true
		}
		if currentDescribe && latestDescribe {
			return currentVersion.commits < latestVersion.commits
		}
		return false
	}
	return current != latest
}

func normalizeSelfVersion(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "v")
	return value
}

type selfVersion struct {
	semver  selfSemver
	commits int
}

func parseSelfVersion(value string) (selfVersion, bool, bool) {
	if info, ok := parseSelfGitDescribe(value); ok {
		return info, true, true
	}
	semver, ok := parseSelfSemver(value)
	return selfVersion{semver: semver}, false, ok
}

func parseSelfGitDescribe(value string) (selfVersion, bool) {
	parts := strings.Split(value, "-")
	if len(parts) < 3 {
		return selfVersion{}, false
	}
	semver, ok := parseSelfSemver(parts[0])
	if !ok {
		return selfVersion{}, false
	}
	commits, err := strconv.Atoi(parts[1])
	if err != nil {
		return selfVersion{}, false
	}
	if !strings.HasPrefix(parts[2], "g") || len(parts[2]) <= 1 {
		return selfVersion{}, false
	}
	return selfVersion{semver: semver, commits: commits}, true
}

type selfSemver struct {
	major int
	minor int
	patch int
}

func parseSelfSemver(value string) (selfSemver, bool) {
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return selfSemver{}, false
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return selfSemver{}, false
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return selfSemver{}, false
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return selfSemver{}, false
	}
	return selfSemver{major: major, minor: minor, patch: patch}, true
}

func (v selfSemver) lessThan(other selfSemver) bool {
	if v.major != other.major {
		return v.major < other.major
	}
	if v.minor != other.minor {
		return v.minor < other.minor
	}
	return v.patch < other.patch
}

func (v selfSemver) String() string {
	return fmt.Sprintf("%d.%d.%d", v.major, v.minor, v.patch)
}

func (s SelfUpdateService) runtimePlatform() (string, string) {
	goos := s.RuntimeGOOS
	if goos == "" {
		goos = runtime.GOOS
	}
	goarch := s.RuntimeGOARCH
	if goarch == "" {
		goarch = runtime.GOARCH
	}
	return goos, goarch
}

func selfUpdateSystem(goos, goarch string) string {
	return goos + "/" + goarch
}

func selfUpdateAssetFilters() []string {
	return nil
}

func selfUpdateDownloadName(goos string) string {
	if goos == "windows" {
		return "eget.exe"
	}
	return "eget"
}

func selfUpdateArchiveFileName(goos, goarch string) string {
	return "eget-" + goos + "-" + goarch + selfUpdateSourceAssetExt(goos)
}

func selfUpdateExecutableName(goos string) string {
	if goos == "windows" {
		return "eget.exe"
	}
	return "eget"
}

func selfUpdateSourceAssetURL(source, goos, goarch string) string {
	base := selfUpdateSourceBaseURL(source)
	return base + path.Base("eget-"+goos+"-"+goarch+selfUpdateSourceAssetExt(goos))
}

func selfUpdateSourceAssetExt(goos string) string {
	if goos == "windows" {
		return ".exe"
	}
	return ""
}

func selfUpdateSourceLatestURL(source string) string {
	source = strings.TrimSpace(source)
	if strings.HasSuffix(source, "/latest.yaml") {
		return source
	}
	return strings.TrimRight(source, "/") + "/latest.yaml"
}

func selfUpdateSourceBaseURL(source string) string {
	latestURL := selfUpdateSourceLatestURL(source)
	parsed, err := url.Parse(latestURL)
	if err != nil {
		return strings.TrimRight(source, "/") + "/"
	}
	parsed.Path = path.Dir(parsed.Path) + "/"
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func (s SelfUpdateService) sourceLatestInfo(source string, opts install.Options) (LatestInfo, error) {
	if s.SourceLatestInfo != nil {
		return s.SourceLatestInfo(source, opts)
	}
	return fetchSelfUpdateSourceLatestInfo(source, opts)
}

func fetchSelfUpdateSourceLatestInfo(source string, opts install.Options) (LatestInfo, error) {
	getter := install.NewHTTPGetter(opts)
	resp, err := getter.Get(selfUpdateSourceLatestURL(source))
	if err != nil {
		return LatestInfo{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return LatestInfo{}, fmt.Errorf("fetch self update metadata: %s", resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return LatestInfo{}, err
	}
	cfg := urltemplate.Config{LatestFormat: "yaml"}
	version, err := urltemplate.ParseLatest(data, cfg)
	if err != nil {
		return LatestInfo{}, err
	}
	publishedAt, err := urltemplate.ParseLatestPublishedAt(data, cfg)
	if err != nil {
		return LatestInfo{}, err
	}
	return LatestInfo{Tag: version, PublishedAt: publishedAt}, nil
}

func singleSelfUpdateReplacement(result RunResult, expectedName string) (string, error) {
	if len(result.ExtractedFiles) != 1 {
		return "", fmt.Errorf("self update expected exactly one extracted file, got %d", len(result.ExtractedFiles))
	}
	replacement := result.ExtractedFiles[0]
	info, err := os.Stat(replacement)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("self update replacement is a directory: %s", replacement)
	}
	if filepath.Base(replacement) != expectedName {
		return "", fmt.Errorf("self update replacement must be %s, got %s", expectedName, filepath.Base(replacement))
	}
	return replacement, nil
}

func (s SelfUpdateService) executablePath() (string, error) {
	if s.ExecutablePath != nil {
		return s.ExecutablePath()
	}
	return os.Executable()
}

func (s SelfUpdateService) tempDir() (string, error) {
	if s.TempDir != nil {
		return s.TempDir()
	}
	return os.MkdirTemp("", "eget-self-update-*")
}

func (s SelfUpdateService) replacer() ExecutableReplacer {
	if s.Replacer != nil {
		return s.Replacer
	}
	return DefaultExecutableReplacer{}
}
