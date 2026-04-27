package cli

import (
	"strings"

	cfgpkg "github.com/inherelab/eget/internal/config"
	"github.com/inherelab/eget/internal/install"
)

func installOptionsFromInstall(opts *InstallOptions) install.Options {
	return install.Options{
		Tag:            opts.Tag,
		Name:           opts.Name,
		Source:         opts.Source,
		Output:         opts.To,
		OutputExplicit: opts.To != "",
		System:         opts.System,
		ExtractFile:    opts.File,
		All:            opts.All,
		IsGUI:          opts.GUI,
		Quiet:          opts.Quiet,
		Asset:          splitAssetFilters(opts.Asset),
	}
}

func applyGlobalNetworkConfig(opts *install.Options, cfg *cfgpkg.File) {
	if opts == nil || cfg == nil {
		return
	}
	if cfg.ApiCache.Enable != nil {
		opts.APICacheEnabled = *cfg.ApiCache.Enable
	}
	if cfg.ApiCache.CacheTime != nil {
		opts.APICacheTime = *cfg.ApiCache.CacheTime
	}
	if cfg.Ghproxy.Enable != nil {
		opts.GhproxyEnabled = *cfg.Ghproxy.Enable
	}
	if cfg.Ghproxy.HostURL != nil {
		opts.GhproxyHostURL = *cfg.Ghproxy.HostURL
	}
	if cfg.Ghproxy.SupportAPI != nil {
		opts.GhproxySupportAPI = *cfg.Ghproxy.SupportAPI
	}
	if len(cfg.Ghproxy.Fallbacks) > 0 {
		opts.GhproxyFallbacks = append([]string(nil), cfg.Ghproxy.Fallbacks...)
	}
}

func installOptionsFromDownload(opts *DownloadOptions) install.Options {
	base := installOptionsFromInstall(&InstallOptions{
		Tag:    opts.Tag,
		System: opts.System,
		To:     opts.To,
		File:   opts.File,
		Asset:  opts.Asset,
		Source: opts.Source,
		All:    opts.All,
		Quiet:  opts.Quiet,
	})
	if hasMultipleFilePatterns(opts.File) {
		base.All = true
	}
	base.DownloadOnly = opts.File == "" && !opts.All
	return base
}

func installOptionsFromAdd(opts *AddOptions) install.Options {
	return install.Options{
		Tag:            opts.Tag,
		Source:         opts.Source,
		Output:         opts.To,
		OutputExplicit: opts.To != "",
		System:         opts.System,
		ExtractFile:    opts.File,
		All:            opts.All,
		IsGUI:          opts.GUI,
		Quiet:          opts.Quiet,
		Asset:          splitAssetFilters(opts.Asset),
	}
}

func installOptionsFromUpdate(opts *UpdateOptions) install.Options {
	return install.Options{
		Tag:    opts.Tag,
		Source: opts.Source,
		Output: opts.To,
		System: opts.System,
		Quiet:  opts.Quiet,
		Asset:  splitAssetFilters(opts.Asset),
	}
}

func splitAssetFilters(value string) []string {
	if value == "" {
		return nil
	}
	return strings.Split(value, ",")
}

func hasMultipleFilePatterns(value string) bool {
	parts := strings.Split(value, ",")
	count := 0
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			count++
			if count > 1 {
				return true
			}
		}
	}
	return false
}
