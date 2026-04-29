package cli

import (
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/inherelab/eget/internal/app"
	"github.com/inherelab/eget/internal/client"
	cfgpkg "github.com/inherelab/eget/internal/config"
	"github.com/inherelab/eget/internal/install"
	storepkg "github.com/inherelab/eget/internal/installed"
	sourcegithub "github.com/inherelab/eget/internal/source/github"
	sourcesf "github.com/inherelab/eget/internal/source/sourceforge"
	"github.com/inherelab/eget/internal/util"
)

func newCLIService() (*cliService, error) {
	cfg, err := cfgpkg.Load()
	if err != nil {
		return nil, err
	}
	defaultOpts := install.Options{}
	applyGlobalNetworkConfig(&defaultOpts, cfg)
	githubClient := client.NewGitHubClient(install.ClientOptions(defaultOpts))
	installService := install.NewDefaultService(githubClient, binaryModTime)
	installService.GitHubGetterFactory = func(opts install.Options) sourcegithub.HTTPGetter {
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
	latestTag := func(repo, sourcePath string) (string, error) {
		if sfTarget, err := sourcesf.ParseTarget(repo); err == nil {
			info, err := sourcesf.LatestVersion(sfTarget.Project, sourcePath, install.NewHTTPGetter(defaultOpts))
			if err != nil {
				return "", err
			}
			return info.Version, nil
		}
		tag, _, err := githubClient.LatestReleaseInfo(repo)
		return tag, err
	}
	listService := app.ListService{
		LatestTag: latestTag,
	}
	queryService := app.QueryService{
		Client: githubClient,
	}
	searchService := app.SearchService{
		Client: githubClient,
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
			return githubClient.LatestReleaseInfo(repo)
		},
	}
	updService := app.UpdateService{
		Install:   &appService,
		LatestTag: latestTag,
	}
	return &cliService{
		appService:       appService,
		cfgService:       cfgService,
		listService:      listService,
		queryService:     queryService,
		searchService:    searchService,
		uninstallService: uninstallService,
		updService:       updService,
	}, nil
}

func configureVerbose(verbose bool, stderr io.Writer) {
	install.SetVerbose(verbose, stderr)
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
