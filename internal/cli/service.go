package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gookit/goutil/cliutil"
	"github.com/gookit/goutil/x/ccolor"
	"github.com/inherelab/eget/internal/app"
	cfgpkg "github.com/inherelab/eget/internal/config"
	"github.com/inherelab/eget/internal/install"
	storepkg "github.com/inherelab/eget/internal/installed"
	sourcegithub "github.com/inherelab/eget/internal/source/github"
	"github.com/inherelab/eget/internal/util"
)

type cliService struct {
	appService       app.Service
	cfgService       app.ConfigService
	listService      app.ListService
	queryService     app.QueryService
	uninstallService app.UninstallService
	updService       app.UpdateService
}

var githubAPIGetWithOptions = install.GetWithOptions

func newCLIService() (*cliService, error) {
	cfg, err := cfgpkg.Load()
	if err != nil {
		return nil, err
	}
	defaultOpts := install.Options{}
	applyGlobalNetworkConfig(&defaultOpts, cfg)
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
	listService := app.ListService{
		LatestTag: func(repo string) (string, error) {
			return latestGitHubTag(repo, defaultOpts)
		},
	}
	queryService := app.QueryService{
		Client: newGitHubQueryClient(defaultOpts),
	}
	uninstallService := app.UninstallService{
		Store: store,
	}
	appService := app.Service{
		Runner: runner,
		Store:  store,
		Config: &cfgService,
		Now:    time.Now,
		ReleaseInfo: func(repo, url string) (string, time.Time, error) {
			return latestGitHubReleaseInfo(repo, defaultOpts)
		},
	}
	updService := app.UpdateService{
		Install: &appService,
	}
	return &cliService{
		appService:       appService,
		cfgService:       cfgService,
		listService:      listService,
		queryService:     queryService,
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
		if err == nil && opts.Add {
			pkgName := opts.Name
			if pkgName == "" {
				if repo, repoErr := install.NormalizeRepoTarget(opts.Target); repoErr == nil {
					if _, name, found := strings.Cut(repo, "/"); found {
						pkgName = name
					}
				}
			}
			if pkgName != "" {
				fmt.Printf("Added package config: %s -> %s\n", pkgName, opts.Target)
			}
		}
		return err
	case "download":
		opts := options.(*DownloadOptions)
		_, err := s.appService.DownloadTarget(opts.Target, installOptionsFromDownload(opts))
		return err
	case "add":
		opts := options.(*AddOptions)
		err := s.cfgService.AddPackage(opts.Target, opts.Name, installOptionsFromAdd(opts))
		if err == nil {
			ccolor.Infof("✓ Added package config: %s -> %s\n", opts.Name, opts.Target)
		}
		return err
	case "uninstall":
		opts := options.(*UninstallOptions)
		return s.handleUninstall(opts)
	case "list":
		opts := options.(*ListOptions)
		return s.handleList(opts)
	case "config":
		opts := options.(*ConfigOptions)
		return s.handleConfig(opts)
	case "query":
		opts := options.(*QueryOptions)
		return s.handleQuery(opts)
	case "update":
		opts := options.(*UpdateOptions)
		return s.handleUpdate(opts)
	default:
		return ErrNotImplemented
	}
}

func configureVerbose(verbose bool, stderr io.Writer) {
	install.SetVerbose(verbose, stderr)
	sourcegithub.SetVerbose(verbose, stderr)
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
	if opts != nil && opts.Outdated && opts.Info != "" {
		return fmt.Errorf("list --outdated and --info cannot be used together")
	}
	if opts != nil && opts.Info != "" {
		item, err := s.listService.FindPackage(opts.Info)
		if err != nil {
			return err
		}
		printListItemDetails(item)
		return nil
	}
	if opts != nil && opts.Outdated {
		items, failures, err := s.listService.ListOutdatedPackages()
		if err != nil {
			return err
		}
		for _, failure := range failures {
			ccolor.Fprintf(os.Stderr, "<yellow>check_failed</> %s (%s): %v\n", failure.Name, failure.Repo, failure.Error)
		}
		if len(items) == 0 {
			ccolor.Infoln("no outdated packages found")
			return nil
		}

		cols := []string{"Name", "Repo", "Installed", "Version", "Latest version"}
		rows := make([][]any, 0, len(items))
		for _, item := range items {
			rows = append(rows, []any{item.Name, item.Repo, "yes", item.InstalledTag, item.LatestTag})
		}
		ccolor.Print(cliutil.FormatTable(cols, rows, cliutil.MinimalStyle))
		return nil
	}

	items, err := s.listService.ListPackages()
	if err != nil {
		return err
	}
	if len(items) == 0 {
		ccolor.Infoln("no managed packages found")
		return nil
	}

	cols := []string{"Name", "Repo", "Installed", "Version"}
	rows := make([][]any, 0, len(items))
	for _, item := range items {
		installed := "no"
		if item.Installed {
			installed = "yes"
		}
		rows = append(rows, []any{item.Name, item.Repo, installed, item.Version})
	}
	ccolor.Print(cliutil.FormatTable(cols, rows, cliutil.MinimalStyle))
	return nil
}

func (s *cliService) handleConfig(opts *ConfigOptions) error {
	switch opts.Action {
	case "init":
		info, err := s.cfgService.ConfigInfo()
		if err != nil {
			return err
		}
		if info.Exists {
			confirmed, err := promptConfirmOverwrite(info.Path)
			if err != nil {
				return err
			}
			if !confirmed {
				return fmt.Errorf("config init cancelled")
			}
		}
		path, err := s.cfgService.ConfigInit()
		if err != nil {
			return err
		}
		ccolor.Successf("✓ Initialized config: %s\n", path)
		return nil
	case "list", "ls", "show":
		info, err := s.cfgService.ConfigInfo()
		if err != nil {
			return err
		}
		cfg, err := s.cfgService.ConfigList()
		if err != nil {
			return err
		}
		printConfigList(os.Stdout, info.Path, info.Exists, cfg)
		return nil
	case "get":
		value, err := s.cfgService.ConfigGet(opts.Key)
		if err != nil {
			return err
		}
		fmt.Println(value)
		return nil
	case "set":
		err := s.cfgService.ConfigSet(opts.Key, opts.Value)
		if err == nil {
			ccolor.Successf("✓ Set config: %s = %s\n", opts.Key, opts.Value)
		}
		return err
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

func (s *cliService) handleQuery(opts *QueryOptions) error {
	result, err := s.queryService.Query(app.QueryOptions{
		Repo:       opts.Target,
		Action:     opts.Action,
		Tag:        opts.Tag,
		Limit:      opts.Limit,
		JSON:       opts.JSON,
		Prerelease: opts.Prerelease,
	})
	if err != nil {
		return err
	}
	if opts.JSON {
		text, err := result.JSONString()
		if err != nil {
			return err
		}
		fmt.Println(text)
		return nil
	}
	printQueryResult(result)
	return nil
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
	if hasMultipleFilePatterns(opts.File) {
		base.All = true
	}
	base.DownloadOnly = opts.File == "" && !opts.All
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

func promptConfirmOverwrite(path string) (bool, error) {
	fmt.Fprintf(os.Stderr, "Config file already exists: %s\n", path)
	fmt.Fprint(os.Stderr, "Overwrite it? [y/N]: ")

	var answer string
	if _, err := fmt.Fscanln(os.Stdin, &answer); err != nil {
		if err == io.EOF {
			return false, nil
		}
		return false, err
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes", nil
}

func printConfigList(out io.Writer, path string, exists bool, cfg *cfgpkg.File) {
	fmt.Fprintf(out, "# %s, exists: %t\n", path, exists)
	if cfg.Global.Target != nil || cfg.Global.System != nil || cfg.Global.CacheDir != nil || cfg.Global.ProxyURL != nil {
		fmt.Fprintln(out, "[global]")
		if cfg.Global.Target != nil {
			fmt.Fprintf(out, "target = %s\n", *cfg.Global.Target)
		}
		if cfg.Global.System != nil {
			fmt.Fprintf(out, "system = %s\n", *cfg.Global.System)
		}
		if cfg.Global.CacheDir != nil {
			fmt.Fprintf(out, "cache_dir = %s\n", *cfg.Global.CacheDir)
		}
		if cfg.Global.ProxyURL != nil {
			fmt.Fprintf(out, "proxy_url = %s\n", *cfg.Global.ProxyURL)
		}
	}
	if cfg.ApiCache.Enable != nil || cfg.ApiCache.CacheTime != nil {
		fmt.Fprintln(out, "\n[api_cache]")
		if cfg.ApiCache.Enable != nil {
			fmt.Fprintf(out, "enable = %t\n", *cfg.ApiCache.Enable)
		}
		if cfg.ApiCache.CacheTime != nil {
			fmt.Fprintf(out, "cache_time = %d\n", *cfg.ApiCache.CacheTime)
		}
	}
	if cfg.Ghproxy.Enable != nil || cfg.Ghproxy.HostURL != nil || cfg.Ghproxy.SupportAPI != nil || len(cfg.Ghproxy.Fallbacks) > 0 {
		fmt.Fprintln(out, "\n[ghproxy]")
		if cfg.Ghproxy.Enable != nil {
			fmt.Fprintf(out, "enable = %t\n", *cfg.Ghproxy.Enable)
		}
		if cfg.Ghproxy.HostURL != nil {
			fmt.Fprintf(out, "host_url = %s\n", *cfg.Ghproxy.HostURL)
		}
		if cfg.Ghproxy.SupportAPI != nil {
			fmt.Fprintf(out, "support_api = %t\n", *cfg.Ghproxy.SupportAPI)
		}
		if len(cfg.Ghproxy.Fallbacks) > 0 {
			fmt.Fprintf(out, "fallbacks = %s\n", strings.Join(cfg.Ghproxy.Fallbacks, ", "))
		}
	}

	if len(cfg.Repos) > 0 {
		names := make([]string, 0, len(cfg.Repos))
		for name := range cfg.Repos {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			fmt.Fprintf(out, "[repo.%s]\n", name)
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
			fmt.Fprintf(out, "[packages.%s]\n", name)
			if section.Repo != nil {
				fmt.Fprintf(out, "repo = %s\n", *section.Repo)
			}
			if section.Target != nil {
				fmt.Fprintf(out, "target = %s\n", *section.Target)
			}
			if section.CacheDir != nil {
				fmt.Fprintf(out, "cache_dir = %s\n", *section.CacheDir)
			}
		}
	}
}

func binaryModTime(bin, output string) time.Time {
	file := ""
	dir := "."
	if output != "" && util.IsDirectory(output) {
		dir = output
	} else if ebin := os.Getenv("EGET_BIN"); ebin != "" {
		dir = ebin
	}

	if output != "" && !util.ContainsPathSeparator(output) {
		bin = output
	} else if output != "" && !util.IsDirectory(output) {
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

func latestGitHubTag(repo string, opts install.Options) (string, error) {
	tag, _, err := latestGitHubReleaseInfo(repo, opts)
	if err != nil {
		return "", err
	}
	return tag, nil
}

func latestGitHubReleaseInfo(repo string, opts install.Options) (string, time.Time, error) {
	resp, err := githubAPIGetWithOptions(fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo), opts)
	if err != nil {
		return "", time.Time{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", time.Time{}, fmt.Errorf("latest tag check failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var payload struct {
		Tag       string    `json:"tag_name"`
		CreatedAt time.Time `json:"created_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", time.Time{}, err
	}
	if payload.Tag == "" {
		return "", time.Time{}, fmt.Errorf("latest tag is empty")
	}
	return payload.Tag, payload.CreatedAt, nil
}

func printListItemDetails(item *app.ListItem) {
	fmt.Printf("name: %s\n", item.Name)
	fmt.Printf("repo: %s\n", item.Repo)
	if item.Target != "" {
		fmt.Printf("target: %s\n", item.Target)
	}
	if item.Tag != "" {
		fmt.Printf("tag: %s\n", item.Tag)
	}
	fmt.Printf("installed: %s\n", map[bool]string{true: "yes", false: "no"}[item.Installed])
	if item.Version != "" {
		fmt.Printf("version: %s\n", item.Version)
	}
	if item.InstalledTag != "" {
		fmt.Printf("installed_tag: %s\n", item.InstalledTag)
	}
	if !item.InstalledAt.IsZero() {
		fmt.Printf("installed_at: %s\n", item.InstalledAt.Format(time.RFC3339))
	}
	if item.Asset != "" {
		fmt.Printf("asset: %s\n", item.Asset)
	}
	if item.URL != "" {
		fmt.Printf("url: %s\n", item.URL)
	}
}

func printQueryResult(result app.QueryResult) {
	fmt.Printf("action: %s\n", result.Action)
	fmt.Printf("repo: %s\n", result.Repo)
	if result.Tag != "" {
		fmt.Printf("tag: %s\n", result.Tag)
	}
	if result.Info != nil {
		if result.Info.Description != "" {
			fmt.Printf("description: %s\n", result.Info.Description)
		}
		if result.Info.Homepage != "" {
			fmt.Printf("homepage: %s\n", result.Info.Homepage)
		}
		if result.Info.DefaultBranch != "" {
			fmt.Printf("default_branch: %s\n", result.Info.DefaultBranch)
		}
		if result.Info.Stars > 0 {
			fmt.Printf("stars: %d\n", result.Info.Stars)
		}
		if result.Info.Forks > 0 {
			fmt.Printf("forks: %d\n", result.Info.Forks)
		}
		if result.Info.OpenIssues > 0 {
			fmt.Printf("open_issues: %d\n", result.Info.OpenIssues)
		}
		if !result.Info.UpdatedAt.IsZero() {
			fmt.Printf("updated_at: %s\n", result.Info.UpdatedAt.Format(time.RFC3339))
		}
		return
	}
	if result.Latest != nil {
		fmt.Printf("tag: %s\n", result.Latest.Tag)
		if result.Latest.Name != "" {
			fmt.Printf("name: %s\n", result.Latest.Name)
		}
		if !result.Latest.PublishedAt.IsZero() {
			fmt.Printf("published_at: %s\n", result.Latest.PublishedAt.Format(time.RFC3339))
		}
		fmt.Printf("prerelease: %t\n", result.Latest.Prerelease)
		fmt.Printf("assets_count: %d\n", result.Latest.AssetsCount)
		return
	}
	if len(result.Releases) > 0 {
		cols := []string{"Tag", "Name", "Published at", "Prerelease", "Assets"}
		rows := make([][]any, 0, len(result.Releases))
		for _, item := range result.Releases {
			rows = append(rows, []any{
				item.Tag,
				item.Name,
				item.PublishedAt.Format(time.RFC3339),
				item.Prerelease,
				item.AssetsCount,
			})
		}
		ccolor.Print(cliutil.FormatTable(cols, rows, cliutil.MinimalStyle))
		return
	}
	if len(result.Assets) > 0 {
		cols := []string{"Name", "Size", "Downloads", "Updated at", "URL"}
		rows := make([][]any, 0, len(result.Assets))
		for _, item := range result.Assets {
			rows = append(rows, []any{
				item.Name,
				item.Size,
				item.DownloadCount,
				item.UpdatedAt.Format(time.RFC3339),
				item.URL,
			})
		}
		ccolor.Print(cliutil.FormatTable(cols, rows, cliutil.MinimalStyle))
	}
}
