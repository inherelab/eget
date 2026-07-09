package installed

import "time"

type Entry struct {
	Repo           string         `toml:"repo" mapstructure:"repo"`
	Target         string         `toml:"target" mapstructure:"target"`
	InstalledAt    time.Time      `toml:"installed_at" mapstructure:"installed_at"`
	UpdatedAt      time.Time      `toml:"updated_at,omitempty" mapstructure:"updated_at"`
	URL            string         `toml:"url" mapstructure:"url"`
	Asset          string         `toml:"asset" mapstructure:"asset"`
	AssetID        int64          `toml:"asset_id,omitempty" mapstructure:"asset_id"`
	AssetSize      int64          `toml:"asset_size,omitempty" mapstructure:"asset_size"`
	AssetUpdatedAt time.Time      `toml:"asset_updated_at,omitempty" mapstructure:"asset_updated_at"`
	AssetDigest    string         `toml:"asset_digest,omitempty" mapstructure:"asset_digest"`
	Desc           string         `toml:"desc,omitempty" mapstructure:"desc"`
	Homepage       string         `toml:"homepage,omitempty" mapstructure:"homepage"`
	RepoURL        string         `toml:"repo_url,omitempty" mapstructure:"repo_url"`
	Tool           string         `toml:"tool,omitempty" mapstructure:"tool"`
	ExtractedFiles []string       `toml:"extracted_files" mapstructure:"extracted_files"`
	Options        map[string]any `toml:"options" mapstructure:"options"`
	Version        string         `toml:"version,omitempty" mapstructure:"version"`
	Tag            string         `toml:"tag,omitempty" mapstructure:"tag"`
	TagPolicy      string         `toml:"tag_policy,omitempty" mapstructure:"tag_policy"`
	ReleaseDate    time.Time      `toml:"release_date,omitempty" mapstructure:"release_date"`
	IsGUI          bool           `toml:"is_gui,omitempty" mapstructure:"is_gui"`
	InstallMode    string         `toml:"install_mode,omitempty" mapstructure:"install_mode"`
}

type Config struct {
	Installed map[string]Entry `toml:"installed" mapstructure:"installed"`
}
