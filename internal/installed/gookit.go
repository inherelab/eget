package installed

import (
	"github.com/inherelab/eget/internal/util/configutil"
)

func newStoreConfigManager() *configutil.Manager {
	return configutil.NewManager("eget-installed-store")
}

func loadStoreConfigManager(path string) (*configutil.Manager, error) {
	return configutil.LoadManager("eget-installed-store", path)
}

func decodeStoreConfig(cfg *configutil.Manager) (*Config, error) {
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

func encodeStoreConfig(conf *Config) *configutil.Manager {
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
	return encodeStoreConfig(conf).SaveTo(path)
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
	if entry.IsGUI {
		data["is_gui"] = true
	}
	if entry.InstallMode != "" {
		data["install_mode"] = entry.InstallMode
	}
	return data
}
