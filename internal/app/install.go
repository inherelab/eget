package app

import (
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"time"

	cfgpkg "github.com/inherelab/eget/internal/config"
	"github.com/inherelab/eget/internal/install"
	storepkg "github.com/inherelab/eget/internal/installed"
	forge "github.com/inherelab/eget/internal/source/forge"
	sourcesf "github.com/inherelab/eget/internal/source/sourceforge"
	"github.com/inherelab/eget/internal/util"
)

type RunResult = install.RunResult

type Runner interface {
	Run(target string, opts install.Options) (RunResult, error)
}

type InstalledStore interface {
	Record(target string, entry storepkg.Entry) error
}

type PackageAdder interface {
	AddPackage(repo, name string, opts install.Options) error
}

type InstallExtras struct {
	AddToConfig bool
	PackageName string
	PackageOpts install.Options
}

type ReleaseInfoFunc func(repo, url string) (string, time.Time, error)

type Service struct {
	Runner      Runner
	Store       InstalledStore
	Config      PackageAdder
	Now         func() time.Time
	ReleaseInfo ReleaseInfoFunc
	LoadConfig  func() (*cfgpkg.File, error)
}

func (s Service) InstallTarget(target string, opts install.Options, extras ...InstallExtras) (RunResult, error) {
	runTarget, opts, err := s.resolveInstallRequest(target, opts, false)
	if err != nil {
		return RunResult{}, err
	}
	opts = normalizeExtractionOptions(opts)
	result, err := s.Runner.Run(runTarget, opts)
	if err != nil {
		return RunResult{}, err
	}

	installMode := result.InstallMode
	if installMode == "" && opts.IsGUI && len(result.ExtractedFiles) > 0 {
		installMode = install.InstallModePortable
	}
	shouldRecord := len(result.ExtractedFiles) > 0 || installMode == install.InstallModeInstaller
	if s.Store != nil && shouldRecord {
		repo := storepkg.NormalizeRepoName(runTarget)
		tag, releaseDate := tagFromReleaseURL(result.URL), time.Time{}
		isSourceForge := sourcesf.IsTarget(repo)
		isForge := forge.IsTarget(repo)
		if tag == "" && isSourceForge {
			tag = sourcesf.VersionFromText(result.URL)
		}
		if tag == "" && isForge && opts.Tag != "" {
			tag = opts.Tag
		}
		if s.ReleaseInfo != nil {
			if gotTag, gotDate, err := s.ReleaseInfo(repo, result.URL); err == nil {
				if tag == "" {
					tag = gotTag
				}
				releaseDate = gotDate
			}
		}

		entry := storepkg.Entry{
			Repo:           repo,
			Target:         runTarget,
			InstalledAt:    s.now(),
			URL:            result.URL,
			Asset:          chooseAsset(result),
			Tool:           result.Tool,
			ExtractedFiles: append([]string(nil), result.ExtractedFiles...),
			Options:        extractOptionsMap(opts),
			Tag:            tag,
			Version:        sourceVersion(tag, isSourceForge || isForge),
			ReleaseDate:    releaseDate,
			IsGUI:          result.IsGUI || opts.IsGUI,
			InstallMode:    installMode,
		}
		if err := s.Store.Record(runTarget, entry); err != nil {
			return RunResult{}, err
		}
	}

	if len(extras) > 0 && extras[0].AddToConfig {
		if s.Config == nil {
			return RunResult{}, fmt.Errorf("config service is required")
		}
		repo := runTarget
		if normalized, err := install.NormalizeRepoTarget(runTarget); err == nil {
			repo = normalized
		} else if !isManagedConfigTarget(runTarget) {
			return RunResult{}, err
		}
		addOpts := extras[0].PackageOpts
		if result.IsGUI {
			addOpts.IsGUI = true
		}
		if err := s.Config.AddPackage(repo, extras[0].PackageName, addOpts); err != nil {
			return RunResult{}, err
		}
	}

	return result, nil
}

func isManagedConfigTarget(target string) bool {
	switch install.DetectTargetKind(target) {
	case install.TargetRepo, install.TargetGitHubURL, install.TargetSourceForge, install.TargetForge:
		return true
	default:
		return false
	}
}

func sourceVersion(tag string, sourceBacked bool) string {
	if sourceBacked {
		return tag
	}
	return ""
}

func (s Service) resolveInstallRequest(target string, cli install.Options, preferCacheDir bool) (string, install.Options, error) {
	cfg, err := s.loadConfig()
	if err != nil {
		return "", install.Options{}, err
	}

	if pkg, ok := cfg.Packages[target]; ok {
		repo := util.DerefString(pkg.Repo)
		if repo == "" {
			return "", install.Options{}, fmt.Errorf("package %q has no repo", target)
		}
		opts, err := s.resolveInstallOptionsWithConfig(cfg, repo, pkg, cli, preferCacheDir)
		if err != nil {
			return "", install.Options{}, err
		}
		return repo, opts, nil
	}

	opts, err := s.resolveInstallOptionsWithConfig(cfg, target, cfgpkg.Section{}, cli, preferCacheDir)
	if err != nil {
		return "", install.Options{}, err
	}
	return target, opts, nil
}

func (s Service) DownloadTarget(target string, opts install.Options) (RunResult, error) {
	var err error
	opts, err = s.resolveInstallOptions(target, opts, true)
	if err != nil {
		return RunResult{}, err
	}
	opts = normalizeExtractionOptions(opts)
	opts.DownloadOnly = opts.ExtractFile == "" && !opts.All
	return s.Runner.Run(target, opts)
}

func normalizeExtractionOptions(opts install.Options) install.Options {
	if hasMultipleExtractPatterns(opts.ExtractFile) {
		opts.All = true
	}
	if opts.ExtractFile != "" || opts.All {
		opts.DownloadOnly = false
	}
	return opts
}

func hasMultipleExtractPatterns(value string) bool {
	parts := strings.Split(value, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(value, ",") {
			return true
		}
		if strings.ContainsAny(part, "*?[{") {
			return true
		}
	}
	return false
}

func (s Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func chooseAsset(result RunResult) string {
	if result.Asset != "" {
		return result.Asset
	}
	return path.Base(result.URL)
}

func tagFromReleaseURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	for i := 0; i+2 < len(parts); i++ {
		if parts[i] == "releases" && parts[i+1] == "download" {
			tag, err := url.PathUnescape(parts[i+2])
			if err != nil {
				return parts[i+2]
			}
			return tag
		}
		if parts[i] == "releases" && parts[i+2] == "downloads" {
			tag, err := url.PathUnescape(parts[i+1])
			if err != nil {
				return parts[i+1]
			}
			return tag
		}
	}
	return ""
}

func (s Service) loadConfig() (*cfgpkg.File, error) {
	if s.LoadConfig != nil {
		return s.LoadConfig()
	}
	return cfgpkg.Load()
}

func (s Service) resolveInstallOptions(target string, cli install.Options, preferCacheDir bool) (install.Options, error) {
	cfg, err := s.loadConfig()
	if err != nil {
		return install.Options{}, err
	}
	return s.resolveInstallOptionsWithConfig(cfg, target, cfgpkg.Section{}, cli, preferCacheDir)
}

func (s Service) resolveInstallOptionsWithConfig(cfg *cfgpkg.File, target string, pkg cfgpkg.Section, cli install.Options, preferCacheDir bool) (install.Options, error) {
	repoKey := ""
	if repo, err := install.NormalizeRepoTarget(target); err == nil {
		repoKey = repo
	}

	merged := cfgpkg.MergeInstallOptions(cfg.Global, cfg.Repos[repoKey], pkg, cfgpkg.CLIOverrides{
		ExtractAll:   boolOpt(cli.All),
		AssetFilters: stringsOpt(cli.Asset),
		CacheDir:     stringOpt(cli.CacheDir),
		ProxyURL:     stringOpt(cli.ProxyURL),
		DownloadOnly: boolOpt(cli.DownloadOnly),
		File:         stringOpt(cli.ExtractFile),
		IsGUI:        boolOpt(cli.IsGUI),
		Quiet:        boolOpt(cli.Quiet),
		ShowHash:     boolOpt(cli.Hash),
		Source:       boolOpt(cli.Source),
		SourcePath:   stringOpt(cli.SourcePath),
		System:       stringOpt(cli.System),
		Tag:          stringOpt(cli.Tag),
		Target:       stringOpt(cli.Output),
		UpgradeOnly:  boolOpt(cli.UpgradeOnly),
		Verify:       stringOpt(cli.Verify),
		DisableSSL:   boolOpt(cli.DisableSSL),
	})

	targetDir, err := expandPath(merged.Target)
	if err != nil {
		return install.Options{}, err
	}
	cacheDir, err := expandPath(merged.CacheDir)
	if err != nil {
		return install.Options{}, err
	}
	guiTarget, err := expandPath(merged.GuiTarget)
	if err != nil {
		return install.Options{}, err
	}
	apiCacheDir := ""
	if cacheDir != "" {
		apiCacheDir = filepath.Join(cacheDir, "api-cache")
	}

	output := targetDir
	if preferCacheDir && cli.Output == "" && cacheDir != "" {
		output = cacheDir
	}

	apiCacheEnabled := false
	if cfg.ApiCache.Enable != nil {
		apiCacheEnabled = *cfg.ApiCache.Enable
	}
	apiCacheTime := 0
	if cfg.ApiCache.CacheTime != nil {
		apiCacheTime = *cfg.ApiCache.CacheTime
	}
	ghproxyEnabled := false
	if cfg.Ghproxy.Enable != nil {
		ghproxyEnabled = *cfg.Ghproxy.Enable
	}
	ghproxyHostURL := util.DerefString(cfg.Ghproxy.HostURL)
	ghproxySupportAPI := false
	if cfg.Ghproxy.SupportAPI != nil {
		ghproxySupportAPI = *cfg.Ghproxy.SupportAPI
	}

	return install.Options{
		Tag:               merged.Tag,
		Name:              cli.Name,
		Source:            merged.Source,
		SourcePath:        merged.SourcePath,
		Output:            output,
		OutputExplicit:    cli.Output != "",
		GuiTarget:         guiTarget,
		IsGUI:             merged.IsGUI,
		CacheDir:          cacheDir,
		ProxyURL:          merged.ProxyURL,
		APICacheEnabled:   apiCacheEnabled,
		APICacheDir:       apiCacheDir,
		APICacheTime:      apiCacheTime,
		GhproxyEnabled:    ghproxyEnabled,
		GhproxyHostURL:    ghproxyHostURL,
		GhproxySupportAPI: ghproxySupportAPI,
		GhproxyFallbacks:  append([]string(nil), cfg.Ghproxy.Fallbacks...),
		System:            merged.System,
		ExtractFile:       merged.File,
		All:               merged.ExtractAll,
		Quiet:             merged.Quiet,
		DownloadOnly:      merged.DownloadOnly,
		UpgradeOnly:       merged.UpgradeOnly,
		Asset:             append([]string(nil), merged.AssetFilters...),
		Hash:              merged.ShowHash,
		Verify:            merged.Verify,
		DisableSSL:        merged.DisableSSL,
	}, nil
}

func expandPath(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	return util.Expand(value)
}

func extractOptionsMap(opts install.Options) map[string]interface{} {
	recorded := make(map[string]interface{})
	if opts.Tag != "" {
		recorded["tag"] = opts.Tag
	}
	if opts.System != "" {
		recorded["system"] = opts.System
	}
	if opts.Output != "" {
		recorded["output"] = opts.Output
	}
	if opts.GuiTarget != "" {
		recorded["gui_target"] = opts.GuiTarget
	}
	if opts.IsGUI {
		recorded["is_gui"] = true
	}
	if opts.ExtractFile != "" {
		recorded["extract_file"] = opts.ExtractFile
	}
	if opts.All {
		recorded["all"] = true
	}
	if opts.Quiet {
		recorded["quiet"] = true
	}
	if opts.DownloadOnly {
		recorded["download_only"] = true
	}
	if opts.UpgradeOnly {
		recorded["upgrade_only"] = true
	}
	if len(opts.Asset) > 0 {
		recorded["asset"] = append([]string(nil), opts.Asset...)
	}
	if opts.Hash {
		recorded["hash"] = true
	}
	if opts.Verify != "" {
		recorded["verify"] = opts.Verify
	}
	if opts.Source {
		recorded["download_source"] = true
	}
	if opts.DisableSSL {
		recorded["disable_ssl"] = true
	}
	return recorded
}
