package config

import (
	"bytes"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/inherelab/eget/internal/util"
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
	conf.Meta.HasGlobal = cfg.Exists("global", false)
	if err := cfg.MapOnExists("global", &conf.Global); err != nil {
		return nil, err
	}
	if err := cfg.MapOnExists("http_proxy", &conf.HTTPProxy); err != nil {
		return nil, err
	}
	if err := cfg.MapOnExists("api_cache", &conf.ApiCache); err != nil {
		return nil, err
	}
	if err := cfg.MapOnExists("ghproxy", &conf.Ghproxy); err != nil {
		return nil, err
	}
	if err := cfg.MapOnExists("cache_mirror", &conf.CacheMirror); err != nil {
		return nil, err
	}
	if err := cfg.MapOnExists("packages", &conf.Packages); err != nil {
		return nil, err
	}
	if err := cfg.MapOnExists("pkg_templates", &conf.PkgTemplates); err != nil {
		return nil, err
	}
	if err := cfg.MapOnExists("sdk", &conf.SDK); err != nil {
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
		"global":        sectionToMap(file.Global),
		"http_proxy":    httpProxyToMap(file.HTTPProxy),
		"api_cache":     apiCacheToMap(file.ApiCache),
		"ghproxy":       ghproxyToMap(file.Ghproxy),
		"cache_mirror":  cacheMirrorToMap(file.CacheMirror),
		"packages":      map[string]any{},
		"pkg_templates": map[string]any{},
		"sdk":           map[string]any{},
	}
	for name, section := range file.Packages {
		data["packages"].(map[string]any)[name] = sectionToMap(section)
	}
	for name, section := range file.PkgTemplates {
		data["pkg_templates"].(map[string]any)[name] = sectionToMap(section)
	}
	for name, section := range file.SDK {
		data["sdk"].(map[string]any)[name] = sdkSectionToMap(section)
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

// DumpExport writes a portable TOML configuration to out.
func DumpExport(out io.Writer, file *File, withGlobal bool) error {
	cfg := encodeConfigFile(file)
	if !withGlobal {
		data := cfg.Data()
		delete(data, "global")
		cfg.SetData(data)
	}
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
	case "global", "http_proxy", "api_cache", "ghproxy", "cache_mirror", "packages", "pkg_templates", "sdk":
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
	case "proxy_url", "url":
		if strings.HasPrefix(key, "http_proxy.") && text != "" && !strings.HasPrefix(text, "http") {
			text = "http://" + text
		}
		if pathFieldName(key) == "url" {
			return text, true
		}
		if text != "" && !strings.HasPrefix(text, "http") {
			text = "http://" + text
		}
		return text, true
	case "extract_all", "is_gui", "download_only", "quiet", "show_hash", "download_source", "upgrade_only", "disable_ssl", "enable", "fallback":
		parsed, err := strconv.ParseBool(text)
		if err != nil {
			return nil, false
		}
		return parsed, true
	case "cache_time", "chunk_concurrency", "batch_concurrency", "timeout":
		parsed, err := strconv.Atoi(text)
		if err != nil {
			return nil, false
		}
		return parsed, true
	case "asset_filters", "fallbacks", "ignore_update_packages", "install_args":
		return splitAndTrim(text), true
	case "exclude":
		if strings.HasPrefix(key, "http_proxy.") {
			return splitAndTrim(text), true
		}
		return text, true
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
	if section.Prerelease != nil {
		data["prerelease"] = *section.Prerelease
	}
	if section.UserAgent != nil {
		data["user_agent"] = *section.UserAgent
	}
	if section.DownloadOnly != nil {
		data["download_only"] = *section.DownloadOnly
	}
	if section.Desc != nil && *section.Desc != "" {
		data["desc"] = *section.Desc
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
	if len(section.IgnoreUpdatePackages) > 0 {
		data["ignore_update_packages"] = append([]string(nil), section.IgnoreUpdatePackages...)
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
	if len(section.RenameFiles) > 0 {
		data["rename_files"] = util.CloneStringMap(section.RenameFiles)
	}
	if section.Repo != nil {
		data["repo"] = *section.Repo
	}
	if section.ShowHash != nil {
		data["show_hash"] = *section.ShowHash
	}
	if section.StripComponents != nil {
		data["strip_components"] = *section.StripComponents
	}
	if section.Source != nil {
		data["download_source"] = *section.Source
	}
	if section.SourcePath != nil && *section.SourcePath != "" {
		data["source_path"] = *section.SourcePath
	}
	if section.Sys7zPath != nil {
		data["sys7z_path"] = *section.Sys7zPath
	}
	if section.SDKTarget != nil {
		data["sdk_target"] = *section.SDKTarget
	}
	if len(section.SDKExtMap) > 0 {
		data["sdk_ext_map"] = util.CloneStringMap(section.SDKExtMap)
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
	if section.URLTemplate != nil {
		data["url_template"] = *section.URLTemplate
	}
	if section.LatestURL != nil {
		data["latest_url"] = *section.LatestURL
	}
	if section.LatestFormat != nil {
		data["latest_format"] = *section.LatestFormat
	}
	if section.LatestJSONPath != nil {
		data["latest_json_path"] = *section.LatestJSONPath
	}
	if section.VersionRegex != nil {
		data["version_regex"] = *section.VersionRegex
	}
	if len(section.OSMap) > 0 {
		data["os_map"] = util.CloneStringMap(section.OSMap)
	}
	if len(section.ArchMap) > 0 {
		data["arch_map"] = util.CloneStringMap(section.ArchMap)
	}
	if len(section.ExtMap) > 0 {
		data["ext_map"] = util.CloneStringMap(section.ExtMap)
	}
	if len(section.LibcMap) > 0 {
		data["libc_map"] = util.CloneStringMap(section.LibcMap)
	}
	if section.ChecksumURLTemplate != nil {
		data["checksum_url_template"] = *section.ChecksumURLTemplate
	}
	if section.ChecksumFormat != nil {
		data["checksum_format"] = *section.ChecksumFormat
	}
	if section.ChecksumJSONPath != nil {
		data["checksum_json_path"] = *section.ChecksumJSONPath
	}
	if section.ChecksumRegex != nil {
		data["checksum_regex"] = *section.ChecksumRegex
	}
	if section.InstallAction != nil {
		data["install_action"] = *section.InstallAction
	}
	if len(section.InstallArgs) > 0 {
		data["install_args"] = append([]string(nil), section.InstallArgs...)
	}
	if section.InstallMode != nil {
		data["install_mode"] = *section.InstallMode
	}
	if section.DisableSSL != nil {
		data["disable_ssl"] = *section.DisableSSL
	}
	if section.ChunkConcurrency != nil {
		data["chunk_concurrency"] = *section.ChunkConcurrency
	}
	if section.BatchConcurrency != nil {
		data["batch_concurrency"] = *section.BatchConcurrency
	}
	return data
}

func httpProxyToMap(section HTTPProxySection) map[string]any {
	data := map[string]any{}
	if section.Enable != nil {
		data["enable"] = *section.Enable
	}
	if section.URL != nil {
		data["url"] = *section.URL
	}
	if len(section.Exclude) > 0 {
		data["exclude"] = append([]string(nil), section.Exclude...)
	}
	return data
}

func sdkSectionToMap(section SDKSection) map[string]any {
	data := map[string]any{}
	if len(section.Aliases) > 0 {
		data["aliases"] = append([]string(nil), section.Aliases...)
	}
	if section.Target != nil {
		data["target"] = *section.Target
	}
	if section.URLTemplate != nil {
		data["url_template"] = *section.URLTemplate
	}
	if section.IndexURL != nil {
		data["index_url"] = *section.IndexURL
	}
	if section.IndexFormat != nil {
		data["index_format"] = *section.IndexFormat
	}
	if section.IndexParser != nil {
		data["index_parser"] = *section.IndexParser
	}
	if section.IndexPathPrefix != nil {
		data["index_path_prefix"] = *section.IndexPathPrefix
	}
	if section.FilenamePattern != nil {
		data["filename_pattern"] = *section.FilenamePattern
	}
	if section.StripComponents != nil {
		data["strip_components"] = *section.StripComponents
	}
	if len(section.OSMap) > 0 {
		data["os_map"] = util.CloneStringMap(section.OSMap)
	}
	if len(section.ArchMap) > 0 {
		data["arch_map"] = util.CloneStringMap(section.ArchMap)
	}
	if len(section.ExtMap) > 0 {
		data["ext_map"] = util.CloneStringMap(section.ExtMap)
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
	if section.HostURL != nil {
		data["host_url"] = *section.HostURL
	}
	if len(section.Fallbacks) > 0 {
		data["fallbacks"] = append([]string(nil), section.Fallbacks...)
	}
	return data
}

func cacheMirrorToMap(section CacheMirrorSection) map[string]any {
	data := map[string]any{}
	if section.Enable != nil {
		data["enable"] = *section.Enable
	}
	if section.URL != nil {
		data["url"] = *section.URL
	}
	if section.Timeout != nil {
		data["timeout"] = *section.Timeout
	}
	if section.Fallback != nil {
		data["fallback"] = *section.Fallback
	}
	return data
}
