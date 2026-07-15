package config

type Section struct {
	ExtractAll           *bool             `toml:"extract_all" mapstructure:"extract_all"`
	AssetFilters         []string          `toml:"asset_filters" mapstructure:"asset_filters"`
	CacheDir             *string           `toml:"cache_dir" mapstructure:"cache_dir"`
	ProxyURL             *string           `toml:"proxy_url" mapstructure:"proxy_url"`
	Prerelease           *bool             `toml:"prerelease" mapstructure:"prerelease"`
	UserAgent            *string           `toml:"user_agent" mapstructure:"user_agent"`
	DownloadOnly         *bool             `toml:"download_only" mapstructure:"download_only"`
	Desc                 *string           `toml:"desc" mapstructure:"desc"`
	File                 *string           `toml:"file" mapstructure:"file"`
	GithubToken          *string           `toml:"github_token" mapstructure:"github_token"`
	GuiTarget            *string           `toml:"gui_target" mapstructure:"gui_target"`
	IgnoreUpdatePackages []string          `toml:"ignore_update_packages,omitempty" mapstructure:"ignore_update_packages"`
	IsGUI                *bool             `toml:"is_gui" mapstructure:"is_gui"`
	Name                 *string           `toml:"name" mapstructure:"name"`
	Quiet                *bool             `toml:"quiet" mapstructure:"quiet"`
	RenameFiles          map[string]string `toml:"rename_files,omitempty" mapstructure:"rename_files"`
	Repo                 *string           `toml:"repo" mapstructure:"repo"`
	ShowHash             *bool             `toml:"show_hash" mapstructure:"show_hash"`
	StripComponents      *int              `toml:"strip_components" mapstructure:"strip_components"`
	Source               *bool             `toml:"download_source" mapstructure:"download_source"`
	SourcePath           *string           `toml:"source_path" mapstructure:"source_path"`
	Sys7zPath            *string           `toml:"sys7z_path" mapstructure:"sys7z_path"`
	SDKTarget            *string           `toml:"sdk_target,omitempty" mapstructure:"sdk_target"`
	SDKExtMap            map[string]string `toml:"sdk_ext_map,omitempty" mapstructure:"sdk_ext_map"`
	System               *string           `toml:"system" mapstructure:"system"`
	Tag                  *string           `toml:"tag" mapstructure:"tag"`
	TagPolicy            *string           `toml:"tag_policy" mapstructure:"tag_policy"`
	Target               *string           `toml:"target" mapstructure:"target"`
	UpgradeOnly          *bool             `toml:"upgrade_only" mapstructure:"upgrade_only"`
	Verify               *string           `toml:"verify_sha256" mapstructure:"verify_sha256"`
	URLTemplate          *string           `toml:"url_template" mapstructure:"url_template"`
	LatestURL            *string           `toml:"latest_url" mapstructure:"latest_url"`
	LatestFormat         *string           `toml:"latest_format" mapstructure:"latest_format"`
	LatestJSONPath       *string           `toml:"latest_json_path" mapstructure:"latest_json_path"`
	VersionRegex         *string           `toml:"version_regex" mapstructure:"version_regex"`
	OSMap                map[string]string `toml:"os_map,omitempty" mapstructure:"os_map"`
	ArchMap              map[string]string `toml:"arch_map,omitempty" mapstructure:"arch_map"`
	ExtMap               map[string]string `toml:"ext_map,omitempty" mapstructure:"ext_map"`
	LibcMap              map[string]string `toml:"libc_map,omitempty" mapstructure:"libc_map"`
	ChecksumURLTemplate  *string           `toml:"checksum_url_template" mapstructure:"checksum_url_template"`
	ChecksumFormat       *string           `toml:"checksum_format" mapstructure:"checksum_format"`
	ChecksumJSONPath     *string           `toml:"checksum_json_path" mapstructure:"checksum_json_path"`
	ChecksumRegex        *string           `toml:"checksum_regex" mapstructure:"checksum_regex"`
	InstallAction        *string           `toml:"install_action" mapstructure:"install_action"`
	InstallArgs          []string          `toml:"install_args" mapstructure:"install_args"`
	InstallMode          *string           `toml:"install_mode" mapstructure:"install_mode"`
	DisableSSL           *bool             `toml:"disable_ssl" mapstructure:"disable_ssl"`
	ChunkConcurrency     *int              `toml:"chunk_concurrency" mapstructure:"chunk_concurrency"`
	BatchConcurrency     *int              `toml:"batch_concurrency" mapstructure:"batch_concurrency"`
}

type SDKSection struct {
	Aliases         []string          `toml:"aliases" mapstructure:"aliases"`
	Target          *string           `toml:"target" mapstructure:"target"`
	URLTemplate     *string           `toml:"url_template" mapstructure:"url_template"`
	IndexURL        *string           `toml:"index_url" mapstructure:"index_url"`
	IndexFormat     *string           `toml:"index_format" mapstructure:"index_format"`
	IndexParser     *string           `toml:"index_parser" mapstructure:"index_parser"`
	IndexPathPrefix *string           `toml:"index_path_prefix" mapstructure:"index_path_prefix"`
	FilenamePattern *string           `toml:"filename_pattern" mapstructure:"filename_pattern"`
	StripComponents *int              `toml:"strip_components" mapstructure:"strip_components"`
	OSMap           map[string]string `toml:"os_map" mapstructure:"os_map"`
	ArchMap         map[string]string `toml:"arch_map" mapstructure:"arch_map"`
	ExtMap          map[string]string `toml:"ext_map" mapstructure:"ext_map"`
}

type APICacheSection struct {
	Enable    *bool `toml:"enable" mapstructure:"enable"`
	CacheTime *int  `toml:"cache_time" mapstructure:"cache_time"`
}

type GhproxySection struct {
	HostURL   *string  `toml:"host_url" mapstructure:"host_url"`
	Fallbacks []string `toml:"fallbacks" mapstructure:"fallbacks"`
}

type CacheMirrorSection struct {
	Enable   *bool   `toml:"enable" mapstructure:"enable"`
	URL      *string `toml:"url" mapstructure:"url"`
	Timeout  *int    `toml:"timeout" mapstructure:"timeout"`
	Fallback *bool   `toml:"fallback" mapstructure:"fallback"`
}

type HTTPProxySection struct {
	Enable  *bool    `toml:"enable" mapstructure:"enable"`
	URL     *string  `toml:"url" mapstructure:"url"`
	Exclude []string `toml:"exclude" mapstructure:"exclude"`
}

type File struct {
	Meta struct {
		Keys      []string
		HasGlobal bool
	}
	Global       Section            `toml:"global" mapstructure:"global"`
	HTTPProxy    HTTPProxySection   `toml:"http_proxy" mapstructure:"http_proxy"`
	ApiCache     APICacheSection    `toml:"api_cache" mapstructure:"api_cache"`
	Ghproxy      GhproxySection     `toml:"ghproxy" mapstructure:"ghproxy"`
	CacheMirror  CacheMirrorSection `toml:"cache_mirror" mapstructure:"cache_mirror"`
	Repos        map[string]Section
	Packages     map[string]Section    `toml:"packages" mapstructure:"packages"`
	PkgTemplates map[string]Section    `toml:"pkg_templates" mapstructure:"pkg_templates"`
	SDK          map[string]SDKSection `toml:"sdk" mapstructure:"sdk"`
}

type Merged struct {
	ExtractAll          bool
	AssetFilters        []string
	CacheDir            string
	ProxyURL            string
	Prerelease          bool
	UserAgent           string
	DownloadOnly        bool
	File                string
	GithubToken         string
	GuiTarget           string
	IsGUI               bool
	Name                string
	Quiet               bool
	RenameFiles         map[string]string
	ShowHash            bool
	StripComponents     int
	Source              bool
	SourcePath          string
	Sys7zPath           string
	System              string
	Tag                 string
	TagPolicy           string
	Target              string
	UpgradeOnly         bool
	Verify              string
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
	InstallMode         string
	DisableSSL          bool
	ChunkConcurrency    int
}

type CLIOverrides struct {
	ExtractAll       *bool
	AssetFilters     *[]string
	CacheDir         *string
	ProxyURL         *string
	Prerelease       *bool
	DownloadOnly     *bool
	File             *string
	GithubToken      *string
	IsGUI            *bool
	Name             *string
	Quiet            *bool
	RenameFiles      *map[string]string
	ShowHash         *bool
	StripComponents  *int
	Source           *bool
	SourcePath       *string
	System           *string
	Tag              *string
	TagPolicy        *string
	Target           *string
	InstallMode      *string
	UpgradeOnly      *bool
	Verify           *string
	DisableSSL       *bool
	ChunkConcurrency *int
}
