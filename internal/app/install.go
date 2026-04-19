package app

import (
	"path"
	"time"

	"github.com/inherelab/eget/home"
	cfgpkg "github.com/inherelab/eget/internal/config"
	"github.com/inherelab/eget/internal/install"
	storepkg "github.com/inherelab/eget/internal/installed"
)

type RunResult = install.RunResult

type Runner interface {
	Run(target string, opts install.Options) (RunResult, error)
}

type InstalledStore interface {
	Record(target string, entry storepkg.Entry) error
}

type ReleaseInfoFunc func(repo, url string) (string, time.Time, error)

type Service struct {
	Runner      Runner
	Store       InstalledStore
	Now         func() time.Time
	ReleaseInfo ReleaseInfoFunc
	LoadConfig  func() (*cfgpkg.File, error)
}

func (s Service) InstallTarget(target string, opts install.Options) (RunResult, error) {
	opts, err := s.resolveInstallOptions(target, opts, false)
	if err != nil {
		return RunResult{}, err
	}
	result, err := s.Runner.Run(target, opts)
	if err != nil {
		return RunResult{}, err
	}

	if s.Store != nil && len(result.ExtractedFiles) > 0 {
		repo := storepkg.NormalizeRepoName(target)
		tag, releaseDate := "", time.Time{}
		if s.ReleaseInfo != nil {
			if gotTag, gotDate, err := s.ReleaseInfo(repo, result.URL); err == nil {
				tag = gotTag
				releaseDate = gotDate
			}
		}

		entry := storepkg.Entry{
			Repo:           repo,
			Target:         target,
			InstalledAt:    s.now(),
			URL:            result.URL,
			Asset:          chooseAsset(result),
			Tool:           result.Tool,
			ExtractedFiles: append([]string(nil), result.ExtractedFiles...),
			Options:        extractOptionsMap(opts),
			Tag:            tag,
			ReleaseDate:    releaseDate,
		}
		if err := s.Store.Record(target, entry); err != nil {
			return RunResult{}, err
		}
	}

	return result, nil
}

func (s Service) DownloadTarget(target string, opts install.Options) (RunResult, error) {
	opts.DownloadOnly = true
	var err error
	opts, err = s.resolveInstallOptions(target, opts, true)
	if err != nil {
		return RunResult{}, err
	}
	return s.Runner.Run(target, opts)
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

	repoKey := ""
	if repo, err := install.NormalizeRepoTarget(target); err == nil {
		repoKey = repo
	}

	merged := cfgpkg.MergeInstallOptions(cfg.Global, cfg.Repos[repoKey], cfgpkg.Section{}, cfgpkg.CLIOverrides{
		All:          boolOpt(cli.All),
		AssetFilters: stringsOpt(cli.Asset),
		CacheDir:     stringOpt(cli.CacheDir),
		ProxyURL:     stringOpt(cli.ProxyURL),
		DownloadOnly: boolOpt(cli.DownloadOnly),
		File:         stringOpt(cli.ExtractFile),
		Quiet:        boolOpt(cli.Quiet),
		ShowHash:     boolOpt(cli.Hash),
		Source:       boolOpt(cli.Source),
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

	output := targetDir
	if preferCacheDir && cli.Output == "" && cacheDir != "" {
		output = cacheDir
	}

	return install.Options{
		Tag:          merged.Tag,
		Source:       merged.Source,
		Output:       output,
		CacheDir:     cacheDir,
		ProxyURL:     merged.ProxyURL,
		System:       merged.System,
		ExtractFile:  merged.File,
		All:          merged.All,
		Quiet:        merged.Quiet,
		DownloadOnly: merged.DownloadOnly,
		UpgradeOnly:  merged.UpgradeOnly,
		Asset:        append([]string(nil), merged.AssetFilters...),
		Hash:         merged.ShowHash,
		Verify:       merged.Verify,
		DisableSSL:   merged.DisableSSL,
	}, nil
}

func expandPath(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	return home.Expand(value)
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
