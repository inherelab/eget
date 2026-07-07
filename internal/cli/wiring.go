package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/inherelab/eget/internal/app"
	appcache "github.com/inherelab/eget/internal/app/cache"
	"github.com/inherelab/eget/internal/cli/prompts"
	"github.com/inherelab/eget/internal/client"
	cfgpkg "github.com/inherelab/eget/internal/config"
	"github.com/inherelab/eget/internal/install"
	storepkg "github.com/inherelab/eget/internal/installed"
	forge "github.com/inherelab/eget/internal/source/forge"
	sourcegithub "github.com/inherelab/eget/internal/source/github"
	"github.com/inherelab/eget/internal/source/pkgtemplate"
	sourcesf "github.com/inherelab/eget/internal/source/sourceforge"
	"github.com/inherelab/eget/internal/source/urltemplate"
	"github.com/inherelab/eget/internal/util"
)

func newCLIService(noProxyOpt ...bool) (*cliService, error) {
	noProxy := len(noProxyOpt) > 0 && noProxyOpt[0]
	if err := cfgpkg.LoadDotenv(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load dotenv: %v\n", err)
	}
	cfg, err := cfgpkg.Load()
	if err != nil {
		return nil, err
	}
	defaultOpts := install.Options{}
	defaultOpts.NoProxy = noProxy
	applyGlobalNetworkConfig(&defaultOpts, cfg)
	defaultClientOpts := install.ClientOptions(defaultOpts)
	githubClient := client.NewGitHubClient(defaultClientOpts)
	installService := install.NewDefaultService(githubClient, binaryModTime)
	installService.GitHubGetterFactory = func(opts install.Options) sourcegithub.HTTPGetter {
		return client.NewGitHubClient(install.ClientOptions(opts))
	}
	installService.ForgeGetterFactory = func(opts install.Options) forge.HTTPGetter {
		return client.NewGitHubClient(install.ClientOptions(opts))
	}
	installService.SourceForgeGetterFactory = func(opts install.Options) sourcesf.HTTPGetter {
		return client.NewGitHubClient(install.ClientOptions(opts))
	}
	runner := install.NewRunner(installService)
	runner.InstalledLoad = func() (map[string]string, map[string]string, error) {
		store, err := storepkg.DefaultStore()
		if err != nil {
			return nil, nil, err
		}
		cfg, err := store.Load()
		if err != nil {
			return nil, nil, err
		}
		assets := make(map[string]string, len(cfg.Installed)*3)
		urls := make(map[string]string, len(cfg.Installed)*3)
		for repo, entry := range cfg.Installed {
			key := storepkg.NormalizeRepoName(repo)
			assets[key] = entry.Asset
			urls[key] = entry.URL
		}
		for _, entry := range cfg.Installed {
			for _, key := range []string{entry.Repo, entry.Target} {
				key = storepkg.NormalizeRepoName(key)
				if key == "" {
					continue
				}
				if _, ok := assets[key]; !ok {
					assets[key] = entry.Asset
				}
				if _, ok := urls[key]; !ok {
					urls[key] = entry.URL
				}
			}
		}
		return assets, urls, nil
	}
	runner.Prompt = prompts.Select

	store, err := storepkg.DefaultStore()
	if err != nil {
		return nil, err
	}

	cfgPath, err := cfgpkg.ResolveWritablePath()
	if err != nil {
		return nil, err
	}
	repoMetadata := func(repo string) (app.RepoMetadata, error) {
		info, err := githubClient.RepoInfo(repo)
		if err != nil {
			return app.RepoMetadata{}, err
		}
		return app.RepoMetadata{
			Desc:     info.Description,
			Homepage: info.Homepage,
			RepoURL:  "https://github.com/" + repo,
		}, nil
	}
	cfgService := app.ConfigService{ConfigPath: cfgPath, RepoMetadata: repoMetadata}
	latestInfo := func(target app.LatestCheckTarget) (app.LatestInfo, error) {
		repo, sourcePath := target.Repo, target.SourcePath
		if sfTarget, err := sourcesf.ParseTarget(repo); err == nil {
			info, err := sourcesf.LatestVersion(sfTarget.Project, sourcePath, client.NewHTTPGetter(defaultClientOpts))
			if err != nil {
				return app.LatestInfo{}, err
			}
			return app.LatestInfo{Tag: info.Version, PublishedAt: info.PublishedAt}, nil
		}
		if forgeTarget, err := forge.ParseTarget(repo); err == nil {
			info, err := forge.LatestVersion(forgeTarget, client.NewHTTPGetter(defaultClientOpts))
			if err != nil {
				return app.LatestInfo{}, err
			}
			return app.LatestInfo{Tag: info.Tag, PublishedAt: info.PublishedAt}, nil
		}
		if pkgTarget, err := pkgtemplate.ParseTarget(repo); err == nil {
			finder := urltemplate.Finder{
				Name:   pkgTarget.Package,
				Config: urlTemplateConfigFromSection(target.Package),
				Getter: client.NewHTTPGetter(defaultClientOpts),
			}
			info, err := finder.Latest()
			if err != nil {
				return app.LatestInfo{}, err
			}
			return app.LatestInfo{Tag: info.Version, PublishedAt: info.PublishedAt}, nil
		}
		if templateTarget, err := urltemplate.ParseTarget(repo); err == nil {
			finder := urltemplate.Finder{
				Name:   templateTarget.ID,
				Target: templateTarget,
				Config: urlTemplateConfigFromSection(target.Package),
				Getter: client.NewHTTPGetter(defaultClientOpts),
			}
			info, err := finder.Latest()
			if err != nil {
				return app.LatestInfo{}, err
			}
			return app.LatestInfo{Tag: info.Version, PublishedAt: info.PublishedAt}, nil
		}
		var tag string
		var publishedAt time.Time
		var err error
		if target.Tag != "" {
			tag, publishedAt, err = githubClient.ReleaseInfo(repo, target.Tag)
		} else {
			tag, publishedAt, err = githubClient.LatestReleaseInfo(repo, target.Prerelease)
		}
		return app.LatestInfo{Tag: tag, PublishedAt: publishedAt}, err
	}
	listService := app.ListService{
		LatestInfo: latestInfo,
	}
	queryService := app.QueryService{
		Client: githubClient,
		SourceForgeLatest: func(project, sourcePath string) (sourcesf.LatestInfo, error) {
			return sourcesf.LatestVersion(project, sourcePath, install.NewHTTPGetter(defaultOpts))
		},
		SourceForgeReleases: func(project, sourcePath string, limit int, includePrerelease bool) ([]sourcesf.LatestInfo, error) {
			return sourcesf.ListReleases(project, sourcePath, limit, includePrerelease, install.NewHTTPGetter(defaultOpts))
		},
		SourceForgeAssets: func(project, sourcePath, tag string) ([]string, error) {
			return sourcesf.Finder{
				Project: project,
				Path:    sourcePath,
				Tag:     tag,
				Getter:  install.NewHTTPGetter(defaultOpts),
			}.Find()
		},
	}
	searchService := app.SearchService{
		Client: githubClient,
	}
	uninstallService := app.UninstallService{
		Store: store,
		SaveConfig: func(file *cfgpkg.File) error {
			return cfgpkg.Save(cfgPath, file)
		},
	}
	appService := app.Service{
		Runner: runner,
		Store:  store,
		Config: &cfgService,
		Now:    time.Now,
		ReleaseInfo: func(repo, url string) (string, time.Time, error) {
			if forgeTarget, err := forge.ParseTarget(repo); err == nil {
				info, err := forge.LatestVersion(forgeTarget, client.NewHTTPGetter(defaultClientOpts))
				return info.Tag, info.PublishedAt, err
			}
			return githubClient.LatestReleaseInfo(repo)
		},
		RepoMetadata: repoMetadata,
	}
	showService := app.ShowService{}
	updService := app.UpdateService{
		Install:    &appService,
		LatestInfo: latestInfo,
	}
	selfUpdateService := app.SelfUpdateService{
		CurrentVersion: BuildInfo().Version,
		LatestInfo:     latestInfo,
		Installer:      &appService,
	}
	cacheService := appcache.Service{Config: cfg, Now: time.Now}
	sdkService, err := app.NewDefaultSDKService(cfg, noProxy)
	if err != nil {
		return nil, err
	}
	return &cliService{
		appService:        appService,
		cfgService:        cfgService,
		listService:       listService,
		showService:       showService,
		queryService:      queryService,
		searchService:     searchService,
		uninstallService:  uninstallService,
		updService:        updService,
		selfUpdateService: selfUpdateService,
		sdkService:        sdkService,
		cacheService:      cacheService,
		stderr:            os.Stderr,
		proxyURL:          defaultOpts.ProxyURL,
		proxyExclude:      append([]string(nil), defaultOpts.ProxyExclude...),
		noProxy:           noProxy,
	}, nil
}

func urlTemplateConfigFromSection(section cfgpkg.Section) urltemplate.Config {
	return urltemplate.Config{
		LatestURL:      util.DerefString(section.LatestURL),
		LatestFormat:   util.DerefString(section.LatestFormat),
		LatestJSONPath: util.DerefString(section.LatestJSONPath),
		VersionRegex:   util.DerefString(section.VersionRegex),
	}
}

func configureVerbose(verbose bool, stderr io.Writer) {
	install.SetVerbose(verbose, stderr)
	forge.SetVerbose(verbose, stderr)
	sourcegithub.SetVerbose(verbose, stderr)
	sourcesf.SetVerbose(verbose, stderr)
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
