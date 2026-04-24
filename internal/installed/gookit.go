package installed

import (
	"os"
	"path/filepath"

	gconfig "github.com/gookit/config/v2"
	gtoml "github.com/gookit/config/v2/toml"
)

const storeFormatTOML = "toml"

func newStoreConfigManager() *gconfig.Config {
	cfg := gconfig.NewEmpty("eget-installed-store")
	cfg.AddDriver(gtoml.Driver)
	return cfg
}

func loadStoreConfigManager(path string) (*gconfig.Config, error) {
	cfg := newStoreConfigManager()
	if err := cfg.LoadFilesByFormat(storeFormatTOML, path); err != nil {
		return nil, err
	}
	return cfg, nil
}

func decodeStoreConfig(cfg *gconfig.Config) (*Config, error) {
	conf := &Config{Installed: map[string]Entry{}}
	if cfg == nil || !cfg.Exists("installed", true) {
		return conf, nil
	}
	if err := cfg.BindStruct("installed", &conf.Installed); err != nil {
		return nil, err
	}
	if conf.Installed == nil {
		conf.Installed = map[string]Entry{}
	}
	return conf, nil
}

func encodeStoreConfig(conf *Config) *gconfig.Config {
	cfg := newStoreConfigManager()
	if conf == nil {
		conf = &Config{}
	}
	installed := map[string]any{}
	for repo, entry := range conf.Installed {
		installed[repo] = entryToMap(entry)
	}
	cfg.SetData(map[string]any{
		"installed": installed,
	})
	return cfg
}

func saveStoreConfig(path string, conf *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return encodeStoreConfig(conf).DumpToFile(path, storeFormatTOML)
}

func entryToMap(entry Entry) map[string]any {
	data := map[string]any{
		"repo":            entry.Repo,
		"target":          entry.Target,
		"installed_at":    entry.InstalledAt,
		"url":             entry.URL,
		"asset":           entry.Asset,
		"extracted_files": append([]string(nil), entry.ExtractedFiles...),
		"options":         entry.Options,
		"release_date":    entry.ReleaseDate,
	}
	if entry.Tool != "" {
		data["tool"] = entry.Tool
	}
	if entry.Version != "" {
		data["version"] = entry.Version
	}
	if entry.Tag != "" {
		data["tag"] = entry.Tag
	}
	return data
}
