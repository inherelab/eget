package app

import (
	"fmt"
	"strings"

	cfgpkg "github.com/inherelab/eget/internal/config"
	"github.com/inherelab/eget/internal/install"
	"github.com/inherelab/eget/internal/source/sourceforge"
	"github.com/inherelab/eget/internal/util"
)

type ConfigService struct {
	ConfigPath string
	Load       func() (*cfgpkg.File, error)
	Save       func(path string, file *cfgpkg.File) error
}

type ConfigInfoResult struct {
	Path   string
	Exists bool
}

func (s ConfigService) AddPackage(repo, name string, opts install.Options) error {
	cfg, err := s.load()
	if err != nil {
		return err
	}

	if sfTarget, sfErr := sourceforge.ParseTarget(repo); sfErr == nil {
		repo = sfTarget.Normalized
		if opts.SourcePath == "" {
			opts.SourcePath = sfTarget.Path
		}
		if name == "" {
			name = sfTarget.Project
		}
	}

	if name == "" {
		parts := strings.Split(repo, "/")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return fmt.Errorf("invalid repo %q", repo)
		}
		name = parts[1]
	}

	if cfg.Packages == nil {
		cfg.Packages = make(map[string]cfgpkg.Section)
	}
	cfg.Packages[name] = sectionFromInstallOptions(repo, name, opts)
	return s.save(cfg)
}

func (s ConfigService) ConfigInfo() (ConfigInfoResult, error) {
	path := s.ConfigPath
	if path == "" {
		resolved, err := cfgpkg.ResolveConfigPath()
		if err != nil {
			if cfgpkg.IsNotExist(err) {
				return ConfigInfoResult{Exists: false}, nil
			}
			return ConfigInfoResult{}, err
		}
		path = resolved
	}
	_, err := cfgpkg.LoadFile(path)
	if err != nil {
		if cfgpkg.IsNotExist(err) {
			return ConfigInfoResult{Path: path, Exists: false}, nil
		}
		return ConfigInfoResult{}, err
	}
	return ConfigInfoResult{Path: path, Exists: true}, nil
}

func (s ConfigService) ConfigInit() (string, error) {
	path := s.ConfigPath
	if path == "" {
		resolved, err := cfgpkg.ResolveWritablePath()
		if err != nil {
			return "", err
		}
		path = resolved
	}

	file := cfgpkg.NewFile()
	target := "~/.local/bin"
	cacheDir := "~/.cache/eget"
	proxyURL := ""
	empty := ""
	apiCacheEnable := false
	apiCacheTime := 300
	ghproxyEnable := false
	ghproxyHostURL := ""
	ghproxySupportAPI := true
	file.Global.Target = &target
	file.Global.CacheDir = &cacheDir
	file.Global.ProxyURL = &proxyURL
	file.Global.System = &empty
	file.ApiCache.Enable = &apiCacheEnable
	file.ApiCache.CacheTime = &apiCacheTime
	file.Ghproxy.Enable = &ghproxyEnable
	file.Ghproxy.HostURL = &ghproxyHostURL
	file.Ghproxy.SupportAPI = &ghproxySupportAPI
	file.Ghproxy.Fallbacks = []string{}
	if err := cfgpkg.Save(path, file); err != nil {
		return "", err
	}
	return path, nil
}

func (s ConfigService) ConfigList() (*cfgpkg.File, error) {
	return s.load()
}

func (s ConfigService) ConfigGet(key string) (any, error) {
	cfg, err := s.load()
	if err != nil {
		return nil, err
	}

	value, ok := cfgpkg.GetByPath(cfg, key)
	if !ok {
		return nil, fmt.Errorf("unsupported config key %q", key)
	}
	return value, nil
}

func (s ConfigService) ConfigSet(key, value string) error {
	cfg, err := s.load()
	if err != nil {
		return err
	}

	// cast: packages.<name>.asset -> packages.<name>.asset_filters
	if strings.HasPrefix(key, "packages.") && strings.HasSuffix(key, ".asset") {
		key = key + "_filters"
	}
	if err := cfgpkg.SetByPath(cfg, key, value); err != nil {
		return err
	}
	return s.save(cfg)
}

func (s ConfigService) load() (*cfgpkg.File, error) {
	if s.Load != nil {
		return s.Load()
	}
	return cfgpkg.Load()
}

func (s ConfigService) save(file *cfgpkg.File) error {
	path := s.ConfigPath
	if path == "" {
		resolved, err := cfgpkg.ResolveWritablePath()
		if err != nil {
			return err
		}
		path = resolved
	}
	if s.Save != nil {
		return s.Save(path, file)
	}
	return cfgpkg.Save(path, file)
}

func sectionFromInstallOptions(repo, name string, opts install.Options) cfgpkg.Section {
	section := cfgpkg.Section{
		AssetFilters: append([]string(nil), opts.Asset...),
	}
	section.Repo = util.StringPtr(repo)
	section.Name = util.StringPtr(name)
	if opts.Output != "" {
		section.Target = util.StringPtr(opts.Output)
	}
	if opts.CacheDir != "" {
		section.CacheDir = util.StringPtr(opts.CacheDir)
	}
	if opts.System != "" {
		section.System = util.StringPtr(opts.System)
	}
	if opts.ExtractFile != "" {
		section.File = util.StringPtr(opts.ExtractFile)
	}
	if opts.Tag != "" {
		section.Tag = util.StringPtr(opts.Tag)
	}
	if opts.Verify != "" {
		section.Verify = util.StringPtr(opts.Verify)
	}
	if opts.Source {
		section.Source = util.BoolPtr(true)
	}
	if opts.SourcePath != "" {
		section.SourcePath = util.StringPtr(opts.SourcePath)
	}
	if opts.DisableSSL {
		section.DisableSSL = util.BoolPtr(true)
	}
	if opts.All {
		section.ExtractAll = util.BoolPtr(true)
	}
	if opts.IsGUI {
		section.IsGUI = util.BoolPtr(true)
	}
	return section
}
