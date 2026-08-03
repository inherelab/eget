package app

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/inherelab/eget/internal/client"
	cfgpkg "github.com/inherelab/eget/internal/config"
	"github.com/inherelab/eget/internal/install"
	forge "github.com/inherelab/eget/internal/source/forge"
	"github.com/inherelab/eget/internal/source/pkgtemplate"
	"github.com/inherelab/eget/internal/source/sourceforge"
	"github.com/inherelab/eget/internal/util"
)

type ConfigService struct {
	ConfigPath   string
	Load         func() (*cfgpkg.File, error)
	Save         func(path string, file *cfgpkg.File) error
	RepoMetadata func(repo string) (RepoMetadata, error)
}

type RepoMetadata struct {
	Desc     string
	Homepage string
	RepoURL  string
}

type ConfigInfoResult struct {
	Path   string
	Exists bool
}

type ConfigPathResult struct {
	Target string
	Path   string
	Exists bool
}

func (s ConfigService) AddPackage(repo, name string, opts install.Options) error {
	cfg, err := s.load()
	if err != nil {
		return err
	}

	if normalized, ok := pkgtemplate.ResolveAlias(repo, configuredTemplateNames(cfg)); ok {
		repo = normalized
	}
	repo, name, opts, err = ResolvePackageConfig(repo, name, opts)
	if err != nil {
		return err
	}

	if cfg.Packages == nil {
		cfg.Packages = make(map[string]cfgpkg.Section)
	}
	section := sectionFromInstallOptions(repo, name, opts)
	if section.Desc == nil || *section.Desc == "" {
		if meta, ok := s.repoMetadata(repo); ok && meta.Desc != "" {
			section.Desc = util.StringPtr(meta.Desc)
		}
	}
	cfg.Packages[name] = section
	return s.save(cfg)
}

func ResolvePackageName(repo, name string) (string, error) {
	_, resolvedName, _, err := ResolvePackageConfig(repo, name, install.Options{})
	return resolvedName, err
}

func ResolvePackageNameWithConfig(cfg *cfgpkg.File, repo, name string) (string, error) {
	if cfg == nil {
		cfg = cfgpkg.NewFile()
	}
	if normalized, ok := pkgtemplate.ResolveAlias(repo, configuredTemplateNames(cfg)); ok {
		repo = normalized
	}
	return ResolvePackageName(repo, name)
}

func ResolvePackageConfig(repo, name string, opts install.Options) (string, string, install.Options, error) {
	if pkgTarget, pkgErr := pkgtemplate.ParseTarget(repo); pkgErr == nil {
		repo = pkgTarget.Normalized
		if name == "" {
			name = pkgTarget.Package
		}
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

	if forgeTarget, forgeErr := forge.ParseTarget(repo); forgeErr == nil {
		repo = forgeTarget.Normalized
		if name == "" {
			name = forgeTarget.Project
		}
	}

	if name == "" {
		parts := strings.Split(repo, "/")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return "", "", install.Options{}, fmt.Errorf("invalid repo %q", repo)
		}
		name = parts[1]
	}
	return repo, name, opts, nil
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
	sdkTarget := "~/.local/sdks"
	cacheDir := "~/.cache/eget"
	proxyURL := ""
	userAgent := client.DefaultUserAgent
	empty := ""
	sys7zPath := ""
	apiCacheTime := 1800
	ghproxyHostURL := ""
	chunkConcurrency := 0
	batchConcurrency := 0
	file.Global.Target = &target
	file.Global.SDKTarget = &sdkTarget
	file.Global.CacheDir = &cacheDir
	file.Global.ProxyURL = &proxyURL
	file.Global.UserAgent = &userAgent
	file.Global.System = &empty
	file.Global.Sys7zPath = &sys7zPath
	file.Global.ChunkConcurrency = &chunkConcurrency
	file.Global.BatchConcurrency = &batchConcurrency
	file.ApiCache.CacheTime = &apiCacheTime
	file.Ghproxy.HostURL = &ghproxyHostURL
	file.Ghproxy.Fallbacks = []string{}
	if err := cfgpkg.Save(path, file); err != nil {
		return "", err
	}
	return path, nil
}

func (s ConfigService) ConfigList() (*cfgpkg.File, error) {
	return s.load()
}

// ConfigExport writes the current configuration as portable TOML.
func (s ConfigService) ConfigExport(out io.Writer, withGlobal bool) error {
	cfg, err := s.load()
	if err != nil {
		return err
	}
	return cfgpkg.DumpExport(out, cfg, withGlobal)
}

// ConfigImport validates and replaces the writable configuration file.
func (s ConfigService) ConfigImport(sourcePath string) (string, error) {
	targetPath, incoming, err := s.prepareConfigImport(sourcePath)
	if err != nil {
		return "", err
	}
	if err := cfgpkg.SaveAtomic(targetPath, incoming); err != nil {
		return "", err
	}
	return targetPath, nil
}

// ConfigImportCheck validates an import without changing the target file.
func (s ConfigService) ConfigImportCheck(sourcePath string) error {
	_, _, err := s.prepareConfigImport(sourcePath)
	return err
}

func (s ConfigService) prepareConfigImport(sourcePath string) (string, *cfgpkg.File, error) {
	targetPath, err := s.configFilePath()
	if err != nil {
		return "", nil, err
	}
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		return "", nil, err
	}
	if targetInfo, statErr := os.Stat(targetPath); statErr == nil {
		if os.SameFile(sourceInfo, targetInfo) {
			return "", nil, fmt.Errorf("config source and target are the same file: %s", sourcePath)
		}
	} else if !os.IsNotExist(statErr) {
		return "", nil, statErr
	}
	incoming, err := cfgpkg.LoadFile(sourcePath)
	if err != nil {
		return "", nil, err
	}
	if !incoming.Meta.HasGlobal {
		current, loadErr := cfgpkg.LoadFile(targetPath)
		if loadErr == nil {
			incoming.Global = current.Global
		} else if !os.IsNotExist(loadErr) {
			return "", nil, loadErr
		}
	}
	return targetPath, incoming, nil
}

func (s ConfigService) ConfigGet(key string) (any, error) {
	cfg, err := s.load()
	if err != nil {
		return nil, err
	}

	// Support pkg and pkgs as aliases for packages.
	if after, ok := strings.CutPrefix(key, "pkg."); ok {
		key = "packages." + after
	} else if after, ok := strings.CutPrefix(key, "pkgs."); ok {
		key = "packages." + after
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

func (s ConfigService) ConfigPathInfo(target string) (ConfigPathResult, error) {
	if strings.TrimSpace(target) == "" {
		target = "config_file"
	}

	configPath, err := s.configFilePath()
	if err != nil {
		return ConfigPathResult{}, err
	}
	configDir := filepath.Dir(configPath)

	path := ""
	isDir := false
	switch target {
	case "config_file":
		path = configPath
	case "config_dir":
		path = configDir
		isDir = true
	case "env_file":
		path = filepath.Join(configDir, ".env")
	case "bin_dir":
		cfg, err := s.load()
		if err != nil {
			return ConfigPathResult{}, err
		}
		path = util.ExpandPathOrRaw(util.FirstNonEmptyString(util.DerefString(cfg.Global.Target), "~/.local/bin"))
		isDir = true
	case "cache_dir":
		cfg, err := s.load()
		if err != nil {
			return ConfigPathResult{}, err
		}
		path = util.ExpandPathOrRaw(util.FirstNonEmptyString(util.DerefString(cfg.Global.CacheDir), "~/.cache/eget"))
		isDir = true
	case "sdk_dir":
		cfg, err := s.load()
		if err != nil {
			return ConfigPathResult{}, err
		}
		path = util.ExpandPathOrRaw(util.FirstNonEmptyString(util.DerefString(cfg.Global.SDKTarget), "~/.local/sdks"))
		isDir = true
	case "pkg_store_file":
		path = filepath.Join(configDir, "installed.toml")
	case "sdk_store_file":
		path = filepath.Join(configDir, "sdk.installed.json")
	default:
		return ConfigPathResult{}, fmt.Errorf("unsupported config path target %q", target)
	}

	return ConfigPathResult{Target: target, Path: path, Exists: pathExists(path, isDir)}, nil
}

func (s ConfigService) configFilePath() (string, error) {
	if s.ConfigPath != "" {
		return s.ConfigPath, nil
	}
	if resolved, err := cfgpkg.ResolveConfigPath(); err == nil {
		return resolved, nil
	} else if !cfgpkg.IsNotExist(err) {
		return "", err
	}
	return cfgpkg.ResolveWritablePath()
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

func pathExists(path string, isDir bool) bool {
	info, err := os.Stat(filepath.Clean(path))
	if err != nil {
		return false
	}
	if isDir {
		return info.IsDir()
	}
	return !info.IsDir()
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
	if len(opts.RenameFiles) > 0 {
		section.RenameFiles = util.CloneStringMap(opts.RenameFiles)
	}
	if opts.Tag != "" {
		section.Tag = util.StringPtr(opts.Tag)
	}
	if policy := tagPolicyForInstall(opts.Tag, opts.TagPolicy); policy != "" {
		section.TagPolicy = util.StringPtr(policy)
	}
	if opts.Verify != "" {
		section.Verify = util.StringPtr(opts.Verify)
	}
	if opts.Source {
		section.Source = util.BoolPtr(true)
	}
	if opts.Prerelease {
		section.Prerelease = util.BoolPtr(true)
	}
	if opts.SourcePath != "" {
		section.SourcePath = util.StringPtr(opts.SourcePath)
	}
	if opts.DisableSSL {
		section.DisableSSL = util.BoolPtr(true)
	}
	if opts.ChunkConcurrencySet || opts.ChunkConcurrency > 0 {
		section.ChunkConcurrency = &opts.ChunkConcurrency
	}
	if opts.All {
		section.ExtractAll = util.BoolPtr(true)
	}
	if opts.StripComponents > 0 {
		section.StripComponents = &opts.StripComponents
	}
	if opts.IsGUI {
		section.IsGUI = util.BoolPtr(true)
	}
	return section
}

func (s ConfigService) repoMetadata(repo string) (RepoMetadata, bool) {
	if s.RepoMetadata == nil {
		return RepoMetadata{}, false
	}
	meta, err := s.RepoMetadata(repo)
	if err != nil {
		return RepoMetadata{}, false
	}
	return meta, true
}
