package app

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	cfgpkg "github.com/inherelab/eget/internal/config"
	"github.com/inherelab/eget/internal/install"
	storepkg "github.com/inherelab/eget/internal/installed"
	"github.com/inherelab/eget/internal/util"
)

func optionsFromInstalledEntry(entry storepkg.Entry) install.Options {
	opts := install.Options{}
	if entry.Options == nil {
		return opts
	}
	if tag, ok := stringOption(entry.Options, "tag"); ok {
		opts.Tag = trackingTag(tag)
	}
	opts.System, _ = stringOption(entry.Options, "system")
	opts.SourcePath, _ = stringOption(entry.Options, "source_path")
	opts.Output, _ = stringOption(entry.Options, "output", "target")
	opts.OutputExplicit = opts.Output != ""
	opts.GuiTarget, _ = stringOption(entry.Options, "gui_target")
	opts.InstallMode, _ = stringOption(entry.Options, "install_mode")
	opts.ExtractFile, _ = stringOption(entry.Options, "extract_file", "file")
	opts.Verify, _ = stringOption(entry.Options, "verify")
	opts.All, _ = boolOption(entry.Options, "all", "extract_all")
	opts.StripComponents, _ = intOption(entry.Options, "strip_components")
	opts.IsGUI, _ = boolOption(entry.Options, "is_gui")
	opts.Quiet, _ = boolOption(entry.Options, "quiet")
	opts.DownloadOnly, _ = boolOption(entry.Options, "download_only")
	opts.UpgradeOnly, _ = boolOption(entry.Options, "upgrade_only")
	opts.Source, _ = boolOption(entry.Options, "download_source", "source")
	opts.Prerelease, _ = boolOption(entry.Options, "prerelease")
	opts.DisableSSL, _ = boolOption(entry.Options, "disable_ssl")
	opts.Asset = stringSliceOption(entry.Options, "asset", "asset_filters")
	opts.RenameFiles = stringMapOption(entry.Options, "rename_files")
	opts.URLTemplate = install.URLTemplateOptions{
		URLTemplate:         stringOptionValue(entry.Options, "url_template"),
		LatestURL:           stringOptionValue(entry.Options, "latest_url"),
		LatestFormat:        stringOptionValue(entry.Options, "latest_format"),
		LatestJSONPath:      stringOptionValue(entry.Options, "latest_json_path"),
		VersionRegex:        stringOptionValue(entry.Options, "version_regex"),
		OSMap:               stringMapOption(entry.Options, "os_map"),
		ArchMap:             stringMapOption(entry.Options, "arch_map"),
		ExtMap:              stringMapOption(entry.Options, "ext_map"),
		LibcMap:             stringMapOption(entry.Options, "libc_map"),
		ChecksumURLTemplate: stringOptionValue(entry.Options, "checksum_url_template"),
		ChecksumFormat:      stringOptionValue(entry.Options, "checksum_format"),
		ChecksumJSONPath:    stringOptionValue(entry.Options, "checksum_json_path"),
		ChecksumRegex:       stringOptionValue(entry.Options, "checksum_regex"),
		InstallAction:       stringOptionValue(entry.Options, "install_action"),
		InstallArgs:         stringSliceOption(entry.Options, "install_args"),
	}
	return opts
}

func applyUpdateCLIOverrides(base, cli install.Options) install.Options {
	if cli.Tag != "" {
		base.Tag = cli.Tag
	}
	if cli.Source {
		base.Source = true
	}
	if cli.SourcePath != "" {
		base.SourcePath = cli.SourcePath
	}
	if cli.Output != "" {
		base.Output = cli.Output
		base.OutputExplicit = cli.OutputExplicit
	}
	if cli.System != "" {
		base.System = cli.System
	}
	if cli.ExtractFile != "" {
		base.ExtractFile = cli.ExtractFile
	}
	if cli.All {
		base.All = true
	}
	if cli.StripComponents > 0 {
		base.StripComponents = cli.StripComponents
	}
	if cli.Quiet {
		base.Quiet = true
	}
	if cli.ChunkConcurrencySet {
		base.ChunkConcurrency = cli.ChunkConcurrency
		base.ChunkConcurrencySet = true
	}
	if cli.BatchConcurrencySet {
		base.BatchConcurrency = cli.BatchConcurrency
		base.BatchConcurrencySet = true
	}
	if len(cli.Asset) > 0 {
		base.Asset = append([]string(nil), cli.Asset...)
	}
	if len(cli.RenameFiles) > 0 {
		base.RenameFiles = util.CloneStringMap(cli.RenameFiles)
	}
	return base
}

func stringOption(values map[string]any, keys ...string) (string, bool) {
	for _, key := range keys {
		if value, ok := values[key]; ok {
			if text, ok := value.(string); ok && text != "" {
				return text, true
			}
		}
	}
	return "", false
}

func stringOptionValue(values map[string]any, keys ...string) string {
	value, _ := stringOption(values, keys...)
	return value
}

func boolOption(values map[string]any, keys ...string) (bool, bool) {
	for _, key := range keys {
		if value, ok := values[key]; ok {
			if enabled, ok := value.(bool); ok {
				return enabled, true
			}
		}
	}
	return false, false
}

func intOption(values map[string]any, keys ...string) (int, bool) {
	for _, key := range keys {
		value, ok := values[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case int:
			return typed, true
		case int64:
			return int(typed), true
		case float64:
			return int(typed), true
		case string:
			parsed, err := strconv.Atoi(typed)
			if err == nil {
				return parsed, true
			}
		}
	}
	return 0, false
}

func stringSliceOption(values map[string]any, keys ...string) []string {
	for _, key := range keys {
		value, ok := values[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case []string:
			return append([]string(nil), typed...)
		case []any:
			items := make([]string, 0, len(typed))
			for _, item := range typed {
				if text, ok := item.(string); ok {
					items = append(items, text)
				}
			}
			return items
		case string:
			if typed != "" {
				return strings.Split(typed, ",")
			}
		}
	}
	return nil
}

func stringMapOption(values map[string]any, keys ...string) map[string]string {
	for _, key := range keys {
		value, ok := values[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case map[string]string:
			return util.CloneStringMap(typed)
		case map[string]any:
			items := make(map[string]string, len(typed))
			for from, to := range typed {
				if text, ok := to.(string); ok {
					items[from] = text
				}
			}
			if len(items) > 0 {
				return items
			}
		}
	}
	return nil
}

func applyConfigNetworkOptions(cfg *cfgpkg.File, opts install.Options) install.Options {
	if cfg == nil {
		return opts
	}
	if opts.CacheDir == "" {
		opts.CacheDir, _ = expandPath(util.DerefString(cfg.Global.CacheDir))
	}
	proxy := cfgpkg.ResolveHTTPProxy(cfg, cfgpkg.ProxyResolveOptions{
		NoProxy:     opts.NoProxy,
		EnvNoProxy:  os.Getenv("NO_PROXY"),
		OverrideURL: opts.ProxyURL,
	})
	opts.ProxyURL = proxy.URL
	opts.ProxyExclude = append([]string(nil), proxy.Exclude...)
	if cfg.ApiCache.Enable != nil {
		opts.APICacheEnabled = *cfg.ApiCache.Enable
	}
	if cfg.ApiCache.CacheTime != nil {
		opts.APICacheTime = *cfg.ApiCache.CacheTime
	}
	if opts.APICacheDir == "" && opts.CacheDir != "" {
		opts.APICacheDir = filepath.Join(opts.CacheDir, "api-cache")
	}
	if opts.GhproxyHostURL == "" {
		opts.GhproxyHostURL = util.DerefString(cfg.Ghproxy.HostURL)
	}
	if len(opts.GhproxyFallbacks) == 0 && len(cfg.Ghproxy.Fallbacks) > 0 {
		opts.GhproxyFallbacks = append([]string(nil), cfg.Ghproxy.Fallbacks...)
	}
	return opts
}

func boolOpt(value bool) *bool {
	if !value {
		return nil
	}
	return &value
}

func stringOpt(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func stringsOpt(value []string) *[]string {
	if len(value) == 0 {
		return nil
	}
	copied := append([]string(nil), value...)
	return &copied
}

func intOpt(value int, explicit bool) *int {
	if value < 0 {
		return nil
	}
	if !explicit && value == 0 {
		return nil
	}
	return &value
}
