package installed

import (
	gconfig "github.com/gookit/config/v2"
	"github.com/inherelab/eget/internal/util/configutil"
)

func newStoreConfigManager() *gconfig.Config {
	return configutil.NewTOMLManager("eget-installed-store")
}

func loadStoreConfigManager(path string) (*gconfig.Config, error) {
	return configutil.LoadTOMLFile("eget-installed-store", path)
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
	return configutil.SaveTOMLFile(path, encodeStoreConfig(conf))
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
