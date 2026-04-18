package app

import (
	"fmt"
	"strings"

	cfgpkg "github.com/inherelab/eget/internal/config"
	"github.com/inherelab/eget/internal/install"
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
	empty := ""
	file.Global.Target = &target
	file.Global.CacheDir = &cacheDir
	file.Global.System = &empty
	if err := cfgpkg.Save(path, file); err != nil {
		return "", err
	}
	return path, nil
}

func (s ConfigService) ConfigList() (*cfgpkg.File, error) {
	return s.load()
}

func (s ConfigService) ConfigGet(key string) (string, error) {
	cfg, err := s.load()
	if err != nil {
		return "", err
	}
	section, field, pkgName, err := resolveSection(cfg, key)
	if err != nil {
		return "", err
	}
	_ = pkgName
	return getSectionField(section, field)
}

func (s ConfigService) ConfigSet(key, value string) error {
	cfg, err := s.load()
	if err != nil {
		return err
	}
	section, field, pkgName, err := resolveSection(cfg, key)
	if err != nil {
		return err
	}
	if err := setSectionField(section, field, value); err != nil {
		return err
	}
	if pkgName != "" {
		cfg.Packages[pkgName] = *section
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
	section.Repo = stringPtr(repo)
	section.Name = stringPtr(name)
	if opts.Output != "" {
		section.Target = stringPtr(opts.Output)
	}
	if opts.CacheDir != "" {
		section.CacheDir = stringPtr(opts.CacheDir)
	}
	if opts.System != "" {
		section.System = stringPtr(opts.System)
	}
	if opts.ExtractFile != "" {
		section.File = stringPtr(opts.ExtractFile)
	}
	if opts.Tag != "" {
		section.Tag = stringPtr(opts.Tag)
	}
	if opts.Verify != "" {
		section.Verify = stringPtr(opts.Verify)
	}
	if opts.Source {
		section.Source = boolPtr(true)
	}
	if opts.DisableSSL {
		section.DisableSSL = boolPtr(true)
	}
	if opts.All {
		section.All = boolPtr(true)
	}
	return section
}

func resolveSection(cfg *cfgpkg.File, key string) (*cfgpkg.Section, string, string, error) {
	parts := strings.Split(key, ".")
	switch {
	case len(parts) == 2 && parts[0] == "global":
		return &cfg.Global, parts[1], "", nil
	case len(parts) == 3 && parts[0] == "packages":
		section, ok := cfg.Packages[parts[1]]
		if !ok {
			return nil, "", "", fmt.Errorf("unknown package %q", parts[1])
		}
		return &section, parts[2], parts[1], nil
	default:
		return nil, "", "", fmt.Errorf("unsupported config key %q", key)
	}
}

func getSectionField(section *cfgpkg.Section, field string) (string, error) {
	switch field {
	case "target":
		return derefString(section.Target), nil
	case "system":
		return derefString(section.System), nil
	case "cache_dir":
		return derefString(section.CacheDir), nil
	case "repo":
		return derefString(section.Repo), nil
	case "file":
		return derefString(section.File), nil
	case "tag":
		return derefString(section.Tag), nil
	case "verify_sha256":
		return derefString(section.Verify), nil
	default:
		return "", fmt.Errorf("unsupported config field %q", field)
	}
}

func setSectionField(section *cfgpkg.Section, field, value string) error {
	switch field {
	case "target":
		section.Target = stringPtr(value)
	case "system":
		section.System = stringPtr(value)
	case "cache_dir":
		section.CacheDir = stringPtr(value)
	case "repo":
		section.Repo = stringPtr(value)
	case "file":
		section.File = stringPtr(value)
	case "tag":
		section.Tag = stringPtr(value)
	case "verify_sha256":
		section.Verify = stringPtr(value)
	default:
		return fmt.Errorf("unsupported config field %q", field)
	}
	return nil
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func boolPtr(value bool) *bool {
	return &value
}

func stringPtr(value string) *string {
	return &value
}
