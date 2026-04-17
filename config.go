package main

import (
	"os"
	"runtime"

	"github.com/inherelab/eget/home"
	cfgpkg "github.com/inherelab/eget/internal/config"
	"github.com/jessevdk/go-flags"
)

type ConfigGlobal = cfgpkg.Section

type ConfigRepository = cfgpkg.Section

type Config struct {
	Meta struct {
		Keys     []string
		MetaData interface{}
	}
	Global       ConfigGlobal
	Repositories map[string]ConfigRepository
	Packages     map[string]ConfigRepository
}

func LoadConfigurationFile(path string) (Config, error) {
	file, err := cfgpkg.LoadFile(path)
	if err != nil {
		return Config{}, err
	}
	return adaptConfig(file), nil
}

func GetOSConfigPath(homePath string) string {
	return cfgpkg.OSConfigPath(homePath, runtime.GOOS, os.LookupEnv)
}

func InitializeConfig() (*Config, error) {
	file, err := cfgpkg.Load()
	if err != nil {
		return nil, err
	}

	config := adaptConfig(file)
	return &config, nil
}

func update[T any](config T, cli *T) T {
	if cli == nil {
		return config
	}
	return *cli
}

func SetGlobalOptionsFromConfig(config *Config, parser *flags.Parser, opts *Flags, cli CliFlags) error {
	_ = parser

	merged := cfgpkg.MergeInstallOptions(config.Global, cfgpkg.Section{}, cfgpkg.Section{}, toCLIOverrides(cli, false))
	if merged.GithubToken != "" && os.Getenv("EGET_GITHUB_TOKEN") == "" {
		os.Setenv("EGET_GITHUB_TOKEN", merged.GithubToken)
	}

	opts.Tag = merged.Tag
	opts.Prerelease = update(false, cli.Prerelease)
	opts.Source = merged.Source
	expanded, err := home.Expand(merged.Target)
	if err != nil {
		return err
	}
	opts.Output = expanded
	opts.System = merged.System
	opts.ExtractFile = merged.File
	opts.All = merged.All
	opts.Quiet = merged.Quiet
	opts.DLOnly = merged.DownloadOnly
	opts.UpgradeOnly = merged.UpgradeOnly
	opts.Asset = merged.AssetFilters
	opts.Hash = merged.ShowHash
	opts.Verify = merged.Verify
	opts.Remove = update(false, cli.Remove)
	opts.DisableSSL = merged.DisableSSL
	return nil
}

func SetProjectOptionsFromConfig(config *Config, parser *flags.Parser, opts *Flags, cli CliFlags, projectName string) error {
	_ = parser

	repo := config.Repositories[projectName]
	pkg := config.Packages[projectName]
	merged := cfgpkg.MergeInstallOptions(config.Global, repo, pkg, toCLIOverrides(cli, true))

	opts.All = merged.All
	opts.Asset = merged.AssetFilters
	opts.DLOnly = merged.DownloadOnly
	opts.ExtractFile = merged.File
	opts.Hash = merged.ShowHash
	targ, err := home.Expand(merged.Target)
	if err != nil {
		return err
	}
	opts.Output = targ
	opts.Quiet = merged.Quiet
	opts.Source = merged.Source
	opts.System = merged.System
	opts.Tag = merged.Tag
	opts.UpgradeOnly = merged.UpgradeOnly
	opts.Verify = merged.Verify
	opts.DisableSSL = merged.DisableSSL
	return nil
}

func adaptConfig(file *cfgpkg.File) Config {
	config := Config{
		Global:       file.Global,
		Repositories: make(map[string]ConfigRepository, len(file.Repos)),
		Packages:     make(map[string]ConfigRepository, len(file.Packages)),
	}
	config.Meta.Keys = append([]string(nil), file.Meta.Keys...)
	config.Meta.MetaData = file.Meta.MetaData

	for name, repo := range file.Repos {
		config.Repositories[name] = repo
	}
	for name, pkg := range file.Packages {
		config.Packages[name] = pkg
	}
	return config
}

func toCLIOverrides(cli CliFlags, includeProject bool) cfgpkg.CLIOverrides {
	overrides := cfgpkg.CLIOverrides{
		All:          cli.All,
		DownloadOnly: cli.DLOnly,
		Quiet:        cli.Quiet,
		ShowHash:     cli.Hash,
		Source:       cli.Source,
		System:       cli.System,
		Tag:          cli.Tag,
		Target:       cli.Output,
		UpgradeOnly:  cli.UpgradeOnly,
		Verify:       cli.Verify,
		DisableSSL:   cli.DisableSSL,
	}
	if includeProject {
		overrides.AssetFilters = cli.Asset
		overrides.File = cli.ExtractFile
	}
	return overrides
}
