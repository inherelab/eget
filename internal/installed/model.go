package installed

import "time"

type Entry struct {
	Repo           string                 `toml:"repo"`
	Target         string                 `toml:"target"`
	InstalledAt    time.Time              `toml:"installed_at"`
	URL            string                 `toml:"url"`
	Asset          string                 `toml:"asset"`
	Tool           string                 `toml:"tool,omitempty"`
	ExtractedFiles []string               `toml:"extracted_files"`
	Options        map[string]interface{} `toml:"options"`
	Version        string                 `toml:"version,omitempty"`
	Tag            string                 `toml:"tag,omitempty"`
	ReleaseDate    time.Time              `toml:"release_date,omitempty"`
}

type Config struct {
	Installed map[string]Entry `toml:"installed"`
}
