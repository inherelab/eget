package cli

import (
	"fmt"
	"strings"

	"github.com/gookit/goutil/x/ccolor"
	"github.com/inherelab/eget/internal/app"
	cfgpkg "github.com/inherelab/eget/internal/config"
	"github.com/inherelab/eget/internal/install"
)

func (s *cliService) handle(name string, options any) error {
	switch name {
	case "install":
		opts := options.(*InstallOptions)
		cliInstallOpts := s.applyGlobalFlags(installOptionsFromInstall(opts))
		if opts.InstallAll {
			if len(opts.Targets) > 0 {
				return fmt.Errorf("install --all cannot be used with target")
			}
			if opts.Add {
				return fmt.Errorf("install --all cannot be used with --add")
			}
			s.warnIfSudoUserConfigLooksSkipped(opts.Quiet)
			_, err := s.appService.InstallAllPackages(cliInstallOpts)
			return err
		}
		if opts.BatchConcurrency > 0 {
			return fmt.Errorf("--batch can only be used with --all")
		}
		if len(opts.Targets) == 0 {
			return fmt.Errorf("install target is required")
		}
		if len(opts.Targets) > 1 && opts.Name != "" {
			return fmt.Errorf("install --name cannot be used with multiple targets")
		}
		s.warnIfSudoUserConfigLooksSkipped(opts.Quiet)
		for _, target := range opts.Targets {
			_, err := s.appService.InstallTarget(target, cliInstallOpts, app.InstallExtras{
				AddToConfig: opts.Add,
				PackageName: opts.Name,
				PackageOpts: cliInstallOpts,
			})
			if err != nil {
				return err
			}
			if opts.Add {
				pkgName := opts.Name
				if pkgName == "" {
					if repo, repoErr := install.NormalizeRepoTarget(target); repoErr == nil {
						if _, name, found := strings.Cut(repo, "/"); found {
							pkgName = name
						}
					}
				}
				if pkgName != "" {
					fmt.Printf("Added package config: %s -> %s\n", pkgName, target)
				}
			}
		}
		return nil
	case "download":
		opts := options.(*DownloadOptions)
		_, err := s.appService.DownloadTarget(opts.Target, s.applyGlobalFlags(installOptionsFromDownload(opts)))
		return err
	case "add":
		opts := options.(*AddOptions)
		err := s.cfgService.AddPackage(opts.Target, opts.Name, s.applyGlobalFlags(installOptionsFromAdd(opts)))
		if err == nil {
			pkgName := opts.Name
			if pkgName == "" {
				if inferred, inferErr := app.ResolvePackageName(opts.Target, opts.Name); inferErr == nil {
					pkgName = inferred
				} else if cfg, loadErr := s.loadConfigForPackageName(); loadErr == nil {
					if inferred, inferErr := app.ResolvePackageNameWithConfig(cfg, opts.Target, opts.Name); inferErr == nil {
						pkgName = inferred
					}
				}
			}
			ccolor.Infof("✓ Added package config: %s -> %s\n", pkgName, opts.Target)
		}
		return err
	case "uninstall":
		opts := options.(*UninstallOptions)
		return s.handleUninstall(opts)
	case "list":
		opts := options.(*ListOptions)
		return s.handleList(opts)
	case "show":
		opts := options.(*ShowOptions)
		return s.handleShow(opts)
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
	case "sdk.download":
		opts := options.(*SDKDownloadOptions)
		return s.handleSDKDownload(opts)
	case "sdk.install":
		opts := options.(*SDKInstallOptions)
		return s.handleSDKInstall(opts)
	case "sdk.list":
		opts := options.(*SDKListOptions)
		return s.handleSDKList(opts)
	case "sdk.remove":
		opts := options.(*SDKRemoveOptions)
		return s.handleSDKRemove(opts)
	case "sdk.path":
		opts := options.(*SDKPathOptions)
		return s.handleSDKPath(opts)
	case "sdk.search":
		opts := options.(*SDKSearchOptions)
		return s.handleSDKSearch(opts)
	case "sdk.index.list", "sdk.index.show", "sdk.index.refresh", "sdk.index.clear":
		opts := options.(*SDKIndexOptions)
		return s.handleSDKIndex(opts)
	case "sdk.config.add":
		opts := options.(*SDKConfigOptions)
		return s.handleSDKConfig(opts)
	case "cache.clean":
		opts := options.(*CacheCleanOptions)
		return s.handleCacheClean(opts)
	case "cache.list":
		opts := options.(*CacheListOptions)
		return s.handleCacheList(opts)
	case "cache.status":
		opts := options.(*CacheStatusOptions)
		return s.handleCacheStatus(opts)
	case "cache.serve":
		opts := options.(*CacheServeOptions)
		return s.handleCacheServe(opts)
	case "managers.list":
		return s.handleManagersList()
	case "managers.upgrade":
		opts := options.(*ManagersOptions)
		return s.handleManagersUpgrade(opts)
	default:
		return ErrNotImplemented
	}
}

func (s *cliService) loadConfigForPackageName() (*cfgpkg.File, error) {
	if s.cfgService.Load != nil {
		return s.cfgService.Load()
	}
	return cfgpkg.Load()
}

func splitTargets(args []string) []string {
	var targets []string
	for _, arg := range args {
		for _, part := range strings.Split(arg, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				targets = append(targets, part)
			}
		}
	}
	return targets
}
