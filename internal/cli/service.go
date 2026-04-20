package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/inherelab/eget/internal/app"
	cfgpkg "github.com/inherelab/eget/internal/config"
	"github.com/inherelab/eget/internal/install"
	storepkg "github.com/inherelab/eget/internal/installed"
)

type cliService struct {
	appService       app.Service
	cfgService       app.ConfigService
	listService      app.ListService
	uninstallService app.UninstallService
	updService       app.UpdateService
}

func newCLIService() (*cliService, error) {
	defaultOpts := install.Options{}
	runner := install.NewRunner(install.NewDefaultService(install.NewHTTPGetter(defaultOpts), binaryModTime))
	runner.InstalledLoad = func() (map[string]string, map[string]string, error) {
		store, err := storepkg.DefaultStore()
		if err != nil {
			return nil, nil, err
		}
		cfg, err := store.Load()
		if err != nil {
			return nil, nil, err
		}
		assets := make(map[string]string, len(cfg.Installed))
		urls := make(map[string]string, len(cfg.Installed))
		for repo, entry := range cfg.Installed {
			assets[repo] = entry.Asset
			urls[repo] = entry.URL
		}
		return assets, urls, nil
	}
	runner.Prompt = promptIndex

	store, err := storepkg.DefaultStore()
	if err != nil {
		return nil, err
	}

	cfgPath, err := cfgpkg.ResolveWritablePath()
	if err != nil {
		return nil, err
	}
	cfgService := app.ConfigService{ConfigPath: cfgPath}
	listService := app.ListService{}
	uninstallService := app.UninstallService{
		Store: store,
	}
	appService := app.Service{
		Runner: runner,
		Store:  store,
		Config: &cfgService,
		Now:    time.Now,
	}
	updService := app.UpdateService{
		Install: &appService,
	}
	return &cliService{
		appService:       appService,
		cfgService:       cfgService,
		listService:      listService,
		uninstallService: uninstallService,
		updService:       updService,
	}, nil
}

func (s *cliService) handle(name string, options any) error {
	switch name {
	case "install":
		opts := options.(*InstallOptions)
		cliInstallOpts := installOptionsFromInstall(opts)
		_, err := s.appService.InstallTarget(opts.Target, cliInstallOpts, app.InstallExtras{
			AddToConfig: opts.Add,
			PackageName: opts.Name,
			PackageOpts: cliInstallOpts,
		})
		return err
	case "download":
		opts := options.(*DownloadOptions)
		_, err := s.appService.DownloadTarget(opts.Target, installOptionsFromDownload(opts))
		return err
	case "add":
		opts := options.(*AddOptions)
		return s.cfgService.AddPackage(opts.Target, opts.Name, installOptionsFromAdd(opts))
	case "uninstall":
		opts := options.(*UninstallOptions)
		return s.handleUninstall(opts)
	case "list":
		opts := options.(*ListOptions)
		return s.handleList(opts)
	case "config":
		opts := options.(*ConfigOptions)
		return s.handleConfig(opts)
	case "update":
		opts := options.(*UpdateOptions)
		return s.handleUpdate(opts)
	default:
		return ErrNotImplemented
	}
}

func (s *cliService) handleUninstall(opts *UninstallOptions) error {
	result, err := s.uninstallService.Uninstall(opts.Target)
	if err != nil {
		return err
	}
	fmt.Printf("repo: %s\n", result.Repo)
	if len(result.RemovedFiles) == 0 {
		fmt.Println("removed_files: 0")
		return nil
	}
	fmt.Printf("removed_files: %d\n", len(result.RemovedFiles))
	for _, file := range result.RemovedFiles {
		fmt.Printf("removed: %s\n", file)
	}
	return nil
}

func (s *cliService) handleList(opts *ListOptions) error {
	_ = opts
	items, err := s.listService.ListPackages()
	if err != nil {
		return err
	}
	if len(items) == 0 {
		fmt.Println("no managed packages found")
		return nil
	}
	for i, item := range items {
		if i > 0 {
			fmt.Println()
		}
		fmt.Printf("name: %s\n", item.Name)
		fmt.Printf("repo: %s\n", item.Repo)
		if item.Target != "" {
			fmt.Printf("target: %s\n", item.Target)
		}
		if item.Tag != "" {
			fmt.Printf("tag: %s\n", item.Tag)
		}
		if item.Installed {
			fmt.Println("installed: yes")
			fmt.Printf("installed_at: %s\n", item.InstalledAt.Format(time.RFC3339))
			if item.Asset != "" {
				fmt.Printf("asset: %s\n", item.Asset)
			}
			if item.URL != "" {
				fmt.Printf("url: %s\n", item.URL)
			}
		} else {
			fmt.Println("installed: no")
		}
	}
	return nil
}

func (s *cliService) handleConfig(opts *ConfigOptions) error {
	switch {
	case opts.Info:
		info, err := s.cfgService.ConfigInfo()
		if err != nil {
			return err
		}
		fmt.Printf("path: %s\nexists: %t\n", info.Path, info.Exists)
		return nil
	case opts.Init:
		path, err := s.cfgService.ConfigInit()
		if err != nil {
			return err
		}
		fmt.Printf("initialized: %s\n", path)
		return nil
	case opts.List:
		cfg, err := s.cfgService.ConfigList()
		if err != nil {
			return err
		}
		printConfigList(cfg)
		return nil
	case opts.Action == "get":
		value, err := s.cfgService.ConfigGet(opts.Key)
		if err != nil {
			return err
		}
		fmt.Println(value)
		return nil
	case opts.Action == "set":
		return s.cfgService.ConfigSet(opts.Key, opts.Value)
	default:
		return fmt.Errorf("config action is required")
	}
}

func (s *cliService) handleUpdate(opts *UpdateOptions) error {
	installOpts := installOptionsFromUpdate(opts)
	if opts.All {
		_, err := s.updService.UpdateAllPackages(installOpts)
		return err
	}
	if opts.Target == "" {
		return fmt.Errorf("update target is required")
	}
	_, err := s.updService.UpdatePackage(opts.Target, installOpts)
	return err
}

func installOptionsFromInstall(opts *InstallOptions) install.Options {
	return install.Options{
		Tag:         opts.Tag,
		Name:        opts.Name,
		Source:      opts.Source,
		Output:      opts.To,
		CacheDir:    opts.CacheDir,
		System:      opts.System,
		ExtractFile: opts.File,
		All:         opts.All,
		Quiet:       opts.Quiet,
		Asset:       splitAssetFilters(opts.Asset),
	}
}

func installOptionsFromDownload(opts *DownloadOptions) install.Options {
	base := installOptionsFromInstall(&InstallOptions{
		Tag:      opts.Tag,
		System:   opts.System,
		To:       opts.To,
		CacheDir: opts.CacheDir,
		File:     opts.File,
		Asset:    opts.Asset,
		Source:   opts.Source,
		All:      opts.All,
		Quiet:    opts.Quiet,
	})
	base.DownloadOnly = true
	return base
}

func installOptionsFromAdd(opts *AddOptions) install.Options {
	return install.Options{
		Tag:         opts.Tag,
		Source:      opts.Source,
		Output:      opts.To,
		CacheDir:    opts.CacheDir,
		System:      opts.System,
		ExtractFile: opts.File,
		All:         opts.All,
		Quiet:       opts.Quiet,
		Asset:       splitAssetFilters(opts.Asset),
	}
}

func installOptionsFromUpdate(opts *UpdateOptions) install.Options {
	return install.Options{
		Tag:      opts.Tag,
		Source:   opts.Source,
		Output:   opts.To,
		CacheDir: opts.CacheDir,
		System:   opts.System,
		Quiet:    opts.Quiet,
		Asset:    splitAssetFilters(opts.Asset),
	}
}

func splitAssetFilters(value string) []string {
	if value == "" {
		return nil
	}
	return []string{value}
}

func promptIndex(choices []string) (int, error) {
	for i, choice := range choices {
		fmt.Fprintf(os.Stderr, "(%d) %s\n", i+1, choice)
	}
	var picked int
	fmt.Fprint(os.Stderr, "Enter selection number: ")
	if _, err := fmt.Scanf("%d", &picked); err != nil {
		return 0, err
	}
	return picked - 1, nil
}

func printConfigList(cfg *cfgpkg.File) {
	if cfg.Global.Target != nil || cfg.Global.System != nil || cfg.Global.CacheDir != nil || cfg.Global.ProxyURL != nil {
		fmt.Println("[global]")
		if cfg.Global.Target != nil {
			fmt.Printf("target = %s\n", *cfg.Global.Target)
		}
		if cfg.Global.System != nil {
			fmt.Printf("system = %s\n", *cfg.Global.System)
		}
		if cfg.Global.CacheDir != nil {
			fmt.Printf("cache_dir = %s\n", *cfg.Global.CacheDir)
		}
		if cfg.Global.ProxyURL != nil {
			fmt.Printf("proxy_url = %s\n", *cfg.Global.ProxyURL)
		}
	}

	if len(cfg.Repos) > 0 {
		names := make([]string, 0, len(cfg.Repos))
		for name := range cfg.Repos {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			fmt.Printf("[repo.%s]\n", name)
		}
	}

	if len(cfg.Packages) > 0 {
		names := make([]string, 0, len(cfg.Packages))
		for name := range cfg.Packages {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			section := cfg.Packages[name]
			fmt.Printf("[packages.%s]\n", name)
			if section.Repo != nil {
				fmt.Printf("repo = %s\n", *section.Repo)
			}
			if section.Target != nil {
				fmt.Printf("target = %s\n", *section.Target)
			}
			if section.CacheDir != nil {
				fmt.Printf("cache_dir = %s\n", *section.CacheDir)
			}
		}
	}
}

func binaryModTime(bin, output string) time.Time {
	file := ""
	dir := "."
	if output != "" && isDirectory(output) {
		dir = output
	} else if ebin := os.Getenv("EGET_BIN"); ebin != "" {
		dir = ebin
	}

	if output != "" && !containsPathSeparator(output) {
		bin = output
	} else if output != "" && !isDirectory(output) {
		file = output
	}

	if file == "" {
		file = filepath.Join(dir, bin)
	}
	info, err := os.Stat(file)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

func isDirectory(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func containsPathSeparator(value string) bool {
	for _, ch := range value {
		if ch == os.PathSeparator || ch == '/' || ch == '\\' {
			return true
		}
	}
	return false
}
