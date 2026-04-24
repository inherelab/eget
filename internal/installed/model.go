package installed

import "time"

type Entry struct {
	Repo           string                 `toml:"repo" mapstructure:"repo"`
	Target         string                 `toml:"target" mapstructure:"target"`
	InstalledAt    time.Time              `toml:"installed_at" mapstructure:"installed_at"`
	URL            string                 `toml:"url" mapstructure:"url"`
	Asset          string                 `toml:"asset" mapstructure:"asset"`
	Tool           string                 `toml:"tool,omitempty" mapstructure:"tool"`
	ExtractedFiles []string               `toml:"extracted_files" mapstructure:"extracted_files"`
	Options        map[string]interface{} `toml:"options" mapstructure:"options"`
	Version        string                 `toml:"version,omitempty" mapstructure:"version"`
	Tag            string                 `toml:"tag,omitempty" mapstructure:"tag"`
	ReleaseDate    time.Time              `toml:"release_date,omitempty" mapstructure:"release_date"`
}

type Config struct {
	Installed map[string]Entry `toml:"installed" mapstructure:"installed"`
}
