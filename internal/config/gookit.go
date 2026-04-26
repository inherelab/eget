package config

import (
	"bytes"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/inherelab/eget/internal/util/configutil"
)

func newConfigManager() *configutil.Manager {
	return configutil.NewManager("eget-config")
}

func loadConfigManager(path string) (*configutil.Manager, error) {
	return configutil.LoadManager("eget-config", path)
}

func decodeConfigFile(cfg *configutil.Manager) (*File, error) {
	if cfg == nil {
		return NewFile(), nil
	}

	conf := NewFile()
	if err := cfg.MapOnExists("global", &conf.Global); err != nil {
		return nil, err
	}
	if err := cfg.MapOnExists("api_cache", &conf.ApiCache); err != nil {
		return nil, err
	}
	if err := cfg.MapOnExists("ghproxy", &conf.Ghproxy); err != nil {
		return nil, err
	}
	if err := cfg.MapOnExists("packages", &conf.Packages); err != nil {
		return nil, err
	}

	rootData := cfg.Data()
	for _, key := range sortedAnyKeys(rootData) {
		if isReservedConfigRootKey(key) {
			continue
		}

		var section Section
		if err := cfg.BindStruct(key, &section); err != nil {
			return nil, err
		}
		conf.Repos[key] = section
		conf.Meta.Keys = append(conf.Meta.Keys, key)
	}

	return conf, nil
}

func encodeConfigFile(file *File) *configutil.Manager {
	cfg := newConfigManager()
	if file == nil {
		file = NewFile()
	}

	data := map[string]any{
		"global":    sectionToMap(file.Global),
		"api_cache": apiCacheToMap(file.ApiCache),
		"ghproxy":   ghproxyToMap(file.Ghproxy),
		"packages":  map[string]any{},
	}
	for name, section := range file.Packages {
		data["packages"].(map[string]any)[name] = sectionToMap(section)
	}
	cfg.SetData(data)
	for name, section := range file.Repos {
		_ = cfg.Set(name, sectionToMap(section), false)
	}
	return cfg
}

func dumpConfig(file *File, out io.Writer) error {
	cfg := encodeConfigFile(file)
	_, err := cfg.DumpTo(out)
	return err
}

func dumpConfigString(file *File) (string, error) {
	var buf bytes.Buffer
	if err := dumpConfig(file, &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func Save(path string, file *File) error {
	return saveConfigFile(path, file)
}

func saveConfigFile(path string, file *File) error {
	return encodeConfigFile(file).SaveTo(path)
}

func GetByPath(file *File, key string) (any, bool) {
	cfg := encodeConfigFile(file)
	return cfg.GetValue(key, true)
}

func SetByPath(file *File, key string, value any) error {
	cfg := encodeConfigFile(file)
	if normalized, ok := normalizePathValue(key, value); ok {
		value = normalized
	}
	if err := cfg.Set(key, value, true); err != nil {
		return err
	}
	decoded, err := decodeConfigFile(cfg)
	if err != nil {
		return err
	}
	*file = *decoded
	return nil
}

func DecodeTo(file *File, dst any) error {
	cfg := encodeConfigFile(file)
	return cfg.Decode(dst)
}

func BindStruct(file *File, key string, dst any) error {
	cfg := encodeConfigFile(file)
	return cfg.BindStruct(key, dst)
}

func sortedAnyKeys(items map[string]any) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func isReservedConfigRootKey(key string) bool {
	switch key {
	case "global", "api_cache", "ghproxy", "packages":
		return true
	default:
		return false
	}
}

func normalizePathValue(key string, value any) (any, bool) {
	text, ok := value.(string)
	if !ok {
		return nil, false
	}

	switch pathFieldName(key) {
	case "proxy_url":
		if text != "" && !strings.HasPrefix(text, "http") {
			text = "http://" + text
		}
		return text, true
	case "extract_all", "is_gui", "download_only", "quiet", "show_hash", "download_source", "upgrade_only", "disable_ssl", "enable", "support_api":
		parsed, err := strconv.ParseBool(text)
		if err != nil {
			return nil, false
		}
		return parsed, true
	case "cache_time":
		parsed, err := strconv.Atoi(text)
		if err != nil {
			return nil, false
		}
		return parsed, true
	case "asset_filters", "fallbacks":
		return splitAndTrim(text), true
	default:
		return text, true
	}
}

func pathFieldName(key string) string {
	if idx := strings.LastIndexByte(key, '.'); idx >= 0 && idx < len(key)-1 {
		return key[idx+1:]
	}
	return key
}

func splitAndTrim(value string) []string {
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			items = append(items, part)
		}
	}
	return items
}

func sectionToMap(section Section) map[string]any {
	data := map[string]any{}
	if section.ExtractAll != nil {
		data["extract_all"] = *section.ExtractAll
	}
	if len(section.AssetFilters) > 0 {
		data["asset_filters"] = append([]string(nil), section.AssetFilters...)
	}
	if section.CacheDir != nil {
		data["cache_dir"] = *section.CacheDir
	}
	if section.ProxyURL != nil {
		data["proxy_url"] = *section.ProxyURL
	}
	if section.DownloadOnly != nil {
		data["download_only"] = *section.DownloadOnly
	}
	if section.File != nil {
		data["file"] = *section.File
	}
	if section.GithubToken != nil {
		data["github_token"] = *section.GithubToken
	}
	if section.GuiTarget != nil {
		data["gui_target"] = *section.GuiTarget
	}
	if section.IsGUI != nil {
		data["is_gui"] = *section.IsGUI
	}
	if section.Name != nil {
		data["name"] = *section.Name
	}
	if section.Quiet != nil {
		data["quiet"] = *section.Quiet
	}
	if section.Repo != nil {
		data["repo"] = *section.Repo
	}
	if section.ShowHash != nil {
		data["show_hash"] = *section.ShowHash
	}
	if section.Source != nil {
		data["download_source"] = *section.Source
	}
	if section.System != nil {
		data["system"] = *section.System
	}
	if section.Tag != nil {
		data["tag"] = *section.Tag
	}
	if section.Target != nil {
		data["target"] = *section.Target
	}
	if section.UpgradeOnly != nil {
		data["upgrade_only"] = *section.UpgradeOnly
	}
	if section.Verify != nil {
		data["verify_sha256"] = *section.Verify
	}
	if section.DisableSSL != nil {
		data["disable_ssl"] = *section.DisableSSL
	}
	return data
}

func apiCacheToMap(section APICacheSection) map[string]any {
	data := map[string]any{}
	if section.Enable != nil {
		data["enable"] = *section.Enable
	}
	if section.CacheTime != nil {
		data["cache_time"] = *section.CacheTime
	}
	return data
}

func ghproxyToMap(section GhproxySection) map[string]any {
	data := map[string]any{}
	if section.Enable != nil {
		data["enable"] = *section.Enable
	}
	if section.HostURL != nil {
		data["host_url"] = *section.HostURL
	}
	if section.SupportAPI != nil {
		data["support_api"] = *section.SupportAPI
	}
	if len(section.Fallbacks) > 0 {
		data["fallbacks"] = append([]string(nil), section.Fallbacks...)
	}
	return data
}
