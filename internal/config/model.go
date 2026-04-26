package config

type Section struct {
	ExtractAll   *bool    `toml:"extract_all" mapstructure:"extract_all"`
	AssetFilters []string `toml:"asset_filters" mapstructure:"asset_filters"`
	CacheDir     *string  `toml:"cache_dir" mapstructure:"cache_dir"`
	ProxyURL     *string  `toml:"proxy_url" mapstructure:"proxy_url"`
	DownloadOnly *bool    `toml:"download_only" mapstructure:"download_only"`
	File         *string  `toml:"file" mapstructure:"file"`
	GithubToken  *string  `toml:"github_token" mapstructure:"github_token"`
	GuiTarget    *string  `toml:"gui_target" mapstructure:"gui_target"`
	IsGUI        *bool    `toml:"is_gui" mapstructure:"is_gui"`
	Name         *string  `toml:"name" mapstructure:"name"`
	Quiet        *bool    `toml:"quiet" mapstructure:"quiet"`
	Repo         *string  `toml:"repo" mapstructure:"repo"`
	ShowHash     *bool    `toml:"show_hash" mapstructure:"show_hash"`
	Source       *bool    `toml:"download_source" mapstructure:"download_source"`
	System       *string  `toml:"system" mapstructure:"system"`
	Tag          *string  `toml:"tag" mapstructure:"tag"`
	Target       *string  `toml:"target" mapstructure:"target"`
	UpgradeOnly  *bool    `toml:"upgrade_only" mapstructure:"upgrade_only"`
	Verify       *string  `toml:"verify_sha256" mapstructure:"verify_sha256"`
	DisableSSL   *bool    `toml:"disable_ssl" mapstructure:"disable_ssl"`
}

type APICacheSection struct {
	Enable    *bool `toml:"enable" mapstructure:"enable"`
	CacheTime *int  `toml:"cache_time" mapstructure:"cache_time"`
}

type GhproxySection struct {
	Enable     *bool    `toml:"enable" mapstructure:"enable"`
	HostURL    *string  `toml:"host_url" mapstructure:"host_url"`
	SupportAPI *bool    `toml:"support_api" mapstructure:"support_api"`
	Fallbacks  []string `toml:"fallbacks" mapstructure:"fallbacks"`
}

type File struct {
	Meta struct {
		Keys []string
	}
	Global   Section         `toml:"global" mapstructure:"global"`
	ApiCache APICacheSection `toml:"api_cache" mapstructure:"api_cache"`
	Ghproxy  GhproxySection  `toml:"ghproxy" mapstructure:"ghproxy"`
	Repos    map[string]Section
	Packages map[string]Section `toml:"packages" mapstructure:"packages"`
}

type Merged struct {
	ExtractAll   bool
	AssetFilters []string
	CacheDir     string
	ProxyURL     string
	DownloadOnly bool
	File         string
	GithubToken  string
	GuiTarget    string
	IsGUI        bool
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
	ExtractAll   *bool
	AssetFilters *[]string
	CacheDir     *string
	ProxyURL     *string
	DownloadOnly *bool
	File         *string
	GithubToken  *string
	IsGUI        *bool
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
