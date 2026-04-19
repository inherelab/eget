package config

import "github.com/BurntSushi/toml"

type Section struct {
	All          *bool    `toml:"all"`
	AssetFilters []string `toml:"asset_filters"`
	CacheDir     *string  `toml:"cache_dir"`
	ProxyURL     *string  `toml:"proxy_url"`
	DownloadOnly *bool    `toml:"download_only"`
	File         *string  `toml:"file"`
	GithubToken  *string  `toml:"github_token"`
	Name         *string  `toml:"name"`
	Quiet        *bool    `toml:"quiet"`
	Repo         *string  `toml:"repo"`
	ShowHash     *bool    `toml:"show_hash"`
	Source       *bool    `toml:"download_source"`
	System       *string  `toml:"system"`
	Tag          *string  `toml:"tag"`
	Target       *string  `toml:"target"`
	UpgradeOnly  *bool    `toml:"upgrade_only"`
	Verify       *string  `toml:"verify_sha256"`
	DisableSSL   *bool    `toml:"disable_ssl"`
}

type File struct {
	Meta struct {
		Keys     []string
		MetaData *toml.MetaData
	}
	Global   Section `toml:"global"`
	Repos    map[string]Section
	Packages map[string]Section `toml:"packages"`
}

type mergedFile struct {
	Global   Section            `toml:"global"`
	Packages map[string]Section `toml:"packages"`
}

type Merged struct {
	All          bool
	AssetFilters []string
	CacheDir     string
	ProxyURL     string
	DownloadOnly bool
	File         string
	GithubToken  string
	Name         string
	Quiet        bool
	ShowHash     bool
	Source       bool
	System       string
	Tag          string
	Target       string
	UpgradeOnly  bool
	Verify       string
	DisableSSL   bool
}

type CLIOverrides struct {
	All          *bool
	AssetFilters *[]string
	CacheDir     *string
	ProxyURL     *string
	DownloadOnly *bool
	File         *string
	GithubToken  *string
	Name         *string
	Quiet        *bool
	ShowHash     *bool
	Source       *bool
	System       *string
	Tag          *string
	Target       *string
	UpgradeOnly  *bool
	Verify       *string
	DisableSSL   *bool
}
