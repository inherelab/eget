package app

import (
	"fmt"
	"sort"
	"strings"

	cfgpkg "github.com/inherelab/eget/internal/config"
	"github.com/inherelab/eget/internal/install"
)

type Installer interface {
	InstallTarget(target string, opts install.Options) (RunResult, error)
}

type UpdateService struct {
	Install    Installer
	LoadConfig func() (*cfgpkg.File, error)
}

type UpdateResult struct {
	Name   string
	Target string
	Result RunResult
}

func (s UpdateService) UpdatePackage(nameOrRepo string, cli install.Options) (RunResult, error) {
	cfg, err := s.loadConfig()
	if err != nil {
		return RunResult{}, err
	}

	if pkg, ok := cfg.Packages[nameOrRepo]; ok {
		repo := derefString(pkg.Repo)
		if repo == "" {
			return RunResult{}, fmt.Errorf("package %q has no repo", nameOrRepo)
		}
		opts := mergeInstallOptions(cfg.Global, cfg.Repos[repo], pkg, cli)
		return s.Install.InstallTarget(repo, opts)
	}

	if strings.Contains(nameOrRepo, "/") {
		opts := mergeInstallOptions(cfg.Global, cfg.Repos[nameOrRepo], cfgpkg.Section{}, cli)
		return s.Install.InstallTarget(nameOrRepo, opts)
	}

	return RunResult{}, fmt.Errorf("unknown package %q", nameOrRepo)
}

func (s UpdateService) UpdateAllPackages(cli install.Options) ([]UpdateResult, error) {
	cfg, err := s.loadConfig()
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(cfg.Packages))
	for name := range cfg.Packages {
		names = append(names, name)
	}
	sort.Strings(names)

	results := make([]UpdateResult, 0, len(names))
	for _, name := range names {
		result, err := s.UpdatePackage(name, cli)
		if err != nil {
			return nil, err
		}
		results = append(results, UpdateResult{
			Name:   name,
			Target: derefString(cfg.Packages[name].Repo),
			Result: result,
		})
	}
	return results, nil
}

func (s UpdateService) loadConfig() (*cfgpkg.File, error) {
	if s.LoadConfig != nil {
		return s.LoadConfig()
	}
	return cfgpkg.Load()
}

func mergeInstallOptions(global, repo, pkg cfgpkg.Section, cli install.Options) install.Options {
	merged := cfgpkg.MergeInstallOptions(global, repo, pkg, cfgpkg.CLIOverrides{
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

	return install.Options{
		Tag:          merged.Tag,
		Source:       merged.Source,
		Output:       merged.Target,
		CacheDir:     merged.CacheDir,
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
	}
}

func boolOpt(value bool) *bool {
	if !value {
		return nil
	}
	return &value
}

func stringOpt(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func stringsOpt(value []string) *[]string {
	if len(value) == 0 {
		return nil
	}
	copied := append([]string(nil), value...)
	return &copied
}
