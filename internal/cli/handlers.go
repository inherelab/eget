package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gookit/cliui/progress"
	"github.com/gookit/cliui/show"
	"github.com/gookit/goutil/cliutil"
	"github.com/gookit/goutil/x/ccolor"
	"github.com/inherelab/eget/internal/app"
	"github.com/inherelab/eget/internal/install"
)

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
	case "search":
		opts := options.(*SearchOptions)
		return s.handleSearch(opts)
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
	if opts != nil && opts.Outdated && opts.Info != "" {
		return fmt.Errorf("list --outdated and --info cannot be used together")
	}
	if opts != nil && opts.Info != "" {
		item, err := s.listService.FindPackage(opts.Info)
		if err != nil {
			return err
		}
		show.AList("Package Info", item)
		return nil
	}

	if opts != nil && opts.Outdated {
		ccolor.Infoln("🚀 Checking outdated packages")
		sp := progress.RoundTripSpinner(progress.RandomCharTheme(), 100*time.Millisecond)
		sp.Start("%s checking")
		items, failures, checked, err := s.listService.ListOutdatedPackages()
		if err != nil {
			return err
		}
		sp.Stop()
		ccolor.Successf("✅ Checked %d packages\n", checked)

		for _, failure := range failures {
			ccolor.Fprintf(os.Stderr, "<yellow>check_failed</> %s (%s): %v\n", failure.Name, failure.Repo, failure.Error)
		}
		if len(items) == 0 {
			ccolor.Cyanln("🎉 No outdated packages found")
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

	var items []app.ListItem
	var err error
	if opts != nil && opts.GUI {
		items, err = s.listService.ListGUIPackages(opts.All)
	} else if opts != nil && opts.All {
		items, err = s.listService.ListPackages()
	} else {
		items, err = s.listService.ListInstalledPackages()
	}
	if err != nil {
		return err
	}
	if len(items) == 0 {
		if opts != nil && opts.GUI {
			ccolor.Infoln("no GUI packages found")
		} else if opts != nil && opts.All {
			ccolor.Infoln("no managed packages found")
		} else {
			ccolor.Infoln("no installed packages found")
		}
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
	case "list", "ls":
		info, err := s.cfgService.ConfigInfo()
		if err != nil {
			return err
		}
		cfg, err := s.cfgService.ConfigList()
		if err != nil {
			return err
		}
		ccolor.Printf("# %s, exists: %v\n", info.Path, info.Exists)
		show.MList(map[string]any{
			"global":   cfg.Global,
			"apiCache": cfg.ApiCache,
			"ghproxy":  cfg.Ghproxy,
		})
		ccolor.Yellowln("📦 Configed Packages:")
		show.MList(cfg.Packages)
		return nil
	case "get":
		value, err := s.cfgService.ConfigGet(opts.Key)
		if err != nil {
			return err
		}
		if value == nil {
			ccolor.Infoln("nil")
		} else if str, ok := value.(string); ok {
			ccolor.Infoln(str)
		} else {
			show.JSON(value)
		}
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
	if opts.DryRun {
		return fmt.Errorf("update --dry-run is not implemented")
	}
	if opts.Interactive {
		return fmt.Errorf("update --interactive is not implemented")
	}
	installOpts := installOptionsFromUpdate(opts)
	if opts.All {
		ccolor.Infoln("🚀 Checking outdated packages")
		sp := progress.RoundTripSpinner(progress.RandomCharTheme(), 100*time.Millisecond)
		sp.Start("%s checking")
		items, failures, checked, err := s.updService.ListUpdateCandidates()
		if err != nil {
			return err
		}
		sp.Stop()
		ccolor.Successf("✅ Checked %d packages\n", checked)

		for _, failure := range failures {
			ccolor.Fprintf(os.Stderr, "<yellow>check_failed</> %s (%s): %v\n", failure.Name, failure.Repo, failure.Error)
		}
		if len(items) == 0 {
			ccolor.Cyanln("🎉 No outdated packages found")
			return nil
		}

		cols := []string{"Name", "Repo", "Installed", "Latest version"}
		rows := make([][]any, 0, len(items))
		for _, item := range items {
			rows = append(rows, []any{item.Name, item.Repo, item.InstalledTag, item.LatestTag})
		}
		ccolor.Print(cliutil.FormatTable(cols, rows, cliutil.MinimalStyle))
		_, err = s.updService.UpdateCandidates(items, installOpts)
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

func (s *cliService) handleSearch(opts *SearchOptions) error {
	result, err := s.searchService.Search(app.SearchOptions{
		Keyword: opts.Keyword,
		Extras:  opts.Extras,
		Limit:   opts.Limit,
		Sort:    opts.Sort,
		Order:   opts.Order,
	})
	if err != nil {
		return err
	}

	if opts.JSON {
		text, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(text))
		return nil
	}

	printSearchResult(result)
	return nil
}
