package app

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	cfgpkg "github.com/inherelab/eget/internal/config"
	"github.com/inherelab/eget/internal/install"
	"github.com/inherelab/eget/internal/source/pkgtemplate"
	"github.com/inherelab/eget/internal/source/urltemplate"
	"github.com/inherelab/eget/internal/util"
)

func (s Service) resolveInstallRequest(target string, cli install.Options, preferCacheDir bool) (string, string, install.Options, error) {
	cfg, err := s.loadConfig()
	if err != nil {
		return "", "", install.Options{}, err
	}
	return s.resolveInstallRequestWithConfig(cfg, target, cli, preferCacheDir)
}

func (s Service) resolveInstallRequestWithConfig(cfg *cfgpkg.File, target string, cli install.Options, preferCacheDir bool) (string, string, install.Options, error) {
	if normalized, ok := pkgtemplate.ResolveAlias(target, configuredTemplateNames(cfg)); ok {
		target = normalized
	}
	if pkgName, ok := configuredPkgTemplatePackageName(cfg, target); ok {
		target = pkgName
	}

	if pkg, ok := cfg.Packages[target]; ok {
		repo := util.DerefString(pkg.Repo)
		if repo == "" {
			return "", "", install.Options{}, fmt.Errorf("package %q has no repo", target)
		}
		source, err := resolveInstallSourceSection(cfg, repo)
		if err != nil {
			return "", "", install.Options{}, err
		}
		opts, err := s.resolveInstallOptionsWithConfig(cfg, repo, source, pkg, cli, preferCacheDir)
		if err != nil {
			return "", "", install.Options{}, err
		}
		if opts.Name == "" && opts.IsGUI && opts.InstallMode == install.InstallModePortable {
			opts.Name = target
		}
		return repo, target, opts, nil
	}

	pkg := packageSectionForRepoTarget(cfg, target, cli.Name)
	target, pkg = resolveRawPkgTemplateTarget(cfg, target, pkg)
	source, err := resolveInstallSourceSection(cfg, target)
	if err != nil {
		return "", "", install.Options{}, err
	}
	opts, err := s.resolveInstallOptionsWithConfig(cfg, target, source, pkg, cli, preferCacheDir)
	if err != nil {
		return "", "", install.Options{}, err
	}
	return target, installRecordTarget(target, opts), opts, nil
}

func installRecordTarget(target string, opts install.Options) string {
	if opts.Name != "" {
		return opts.Name
	}
	if normalized, err := install.NormalizeRepoTarget(target); err == nil {
		return repoName(normalized)
	}
	return target
}

func packageSectionForRepoTarget(cfg *cfgpkg.File, target, name string) cfgpkg.Section {
	targetRepo, err := install.NormalizeRepoTarget(target)
	if err != nil {
		return cfgpkg.Section{}
	}
	for pkgName, pkg := range cfg.Packages {
		if name != "" && pkgName != name {
			continue
		}
		repo := util.DerefString(pkg.Repo)
		if repo == "" {
			continue
		}
		normalized, err := install.NormalizeRepoTarget(repo)
		if err != nil {
			continue
		}
		if normalized == targetRepo {
			return pkg
		}
	}
	return cfgpkg.Section{}
}

func configuredTemplateNames(cfg *cfgpkg.File) map[string]struct{} {
	names := make(map[string]struct{}, len(cfg.PkgTemplates))
	for name := range cfg.PkgTemplates {
		names[name] = struct{}{}
	}
	return names
}

func configuredPkgTemplatePackageName(cfg *cfgpkg.File, target string) (string, bool) {
	parsed, err := pkgtemplate.ParseTarget(target)
	if err != nil {
		return "", false
	}
	if pkg, ok := cfg.Packages[parsed.Package]; ok && util.DerefString(pkg.Repo) == parsed.Normalized {
		return parsed.Package, true
	}

	names := make([]string, 0, len(cfg.Packages))
	for name := range cfg.Packages {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if util.DerefString(cfg.Packages[name].Repo) == parsed.Normalized {
			return name, true
		}
	}
	return "", false
}

func resolveInstallSourceSection(cfg *cfgpkg.File, repo string) (cfgpkg.Section, error) {
	target, err := pkgtemplate.ParseTarget(repo)
	if err != nil {
		if repoKey, normErr := install.NormalizeRepoTarget(repo); normErr == nil {
			return cfg.Repos[repoKey], nil
		}
		return cfgpkg.Section{}, nil
	}
	template, ok := cfg.PkgTemplates[target.Template]
	if !ok {
		return cfgpkg.Section{}, fmt.Errorf("pkg template %q is not configured", target.Template)
	}
	if util.DerefString(template.LatestURL) == "" {
		return cfgpkg.Section{}, fmt.Errorf("pkg template %q for package %q has no latest_url", target.Template, target.Package)
	}
	if util.DerefString(template.URLTemplate) == "" {
		return cfgpkg.Section{}, fmt.Errorf("pkg template %q for package %q has no url_template", target.Template, target.Package)
	}
	return renderPkgTemplateSection(template, target.Package)
}

func resolveRawPkgTemplateTarget(cfg *cfgpkg.File, target string, pkg cfgpkg.Section) (string, cfgpkg.Section) {
	if normalized, ok := pkgtemplate.ResolveAlias(target, configuredTemplateNames(cfg)); ok {
		target = normalized
	}
	if parsed, err := pkgtemplate.ParseTarget(target); err == nil && pkg.Name == nil {
		pkg.Name = util.StringPtr(parsed.Package)
	}
	return target, pkg
}

func renderPkgTemplateSection(section cfgpkg.Section, name string) (cfgpkg.Section, error) {
	vars := map[string]string{"name": name}
	render := func(ptr *string) (*string, error) {
		if ptr == nil {
			return nil, nil
		}
		value, err := urltemplate.Render(*ptr, vars)
		if err != nil {
			return nil, err
		}
		return &value, nil
	}
	var err error
	if section.LatestURL, err = render(section.LatestURL); err != nil {
		return cfgpkg.Section{}, err
	}
	if section.LatestJSONPath, err = render(section.LatestJSONPath); err != nil {
		return cfgpkg.Section{}, err
	}
	return section, nil
}

func (s Service) resolveInstallOptions(target string, cli install.Options, preferCacheDir bool) (install.Options, error) {
	cfg, err := s.loadConfig()
	if err != nil {
		return install.Options{}, err
	}
	source, err := resolveInstallSourceSection(cfg, target)
	if err != nil {
		return install.Options{}, err
	}
	return s.resolveInstallOptionsWithConfig(cfg, target, source, cfgpkg.Section{}, cli, preferCacheDir)
}

func (s Service) resolveInstallOptionsWithConfig(cfg *cfgpkg.File, target string, source cfgpkg.Section, pkg cfgpkg.Section, cli install.Options, preferCacheDir bool) (install.Options, error) {
	proxy := cfgpkg.ResolveHTTPProxy(cfg, cfgpkg.ProxyResolveOptions{
		NoProxy:     cli.NoProxy,
		EnvNoProxy:  os.Getenv("NO_PROXY"),
		OverrideURL: cli.ProxyURL,
		PackageURL:  util.DerefString(pkg.ProxyURL),
		RepoURL:     util.DerefString(source.ProxyURL),
	})
	global := cfg.Global
	global.ProxyURL = nil
	source.ProxyURL = nil
	pkg.ProxyURL = nil
	merged := cfgpkg.MergeInstallOptions(global, source, pkg, cfgpkg.CLIOverrides{
		ExtractAll:       boolOpt(cli.All),
		AssetFilters:     stringsOpt(cli.Asset),
		CacheDir:         stringOpt(cli.CacheDir),
		DownloadOnly:     boolOpt(cli.DownloadOnly),
		File:             stringOpt(cli.ExtractFile),
		IsGUI:            boolOpt(cli.IsGUI),
		InstallMode:      stringOpt(cli.InstallMode),
		Name:             stringOpt(cli.Name),
		Prerelease:       boolOpt(cli.Prerelease),
		Quiet:            boolOpt(cli.Quiet),
		RenameFiles:      mapOpt(cli.RenameFiles),
		ShowHash:         boolOpt(cli.Hash),
		StripComponents:  intOpt(cli.StripComponents, cli.StripComponents > 0),
		Source:           boolOpt(cli.Source),
		SourcePath:       stringOpt(cli.SourcePath),
		System:           stringOpt(cli.System),
		Tag:              stringOpt(cli.Tag),
		TagPolicy:        stringOpt(cli.TagPolicy),
		Target:           stringOpt(cli.Output),
		UpgradeOnly:      boolOpt(cli.UpgradeOnly),
		Verify:           stringOpt(cli.Verify),
		DisableSSL:       boolOpt(cli.DisableSSL),
		ChunkConcurrency: intOpt(cli.ChunkConcurrency, cli.ChunkConcurrencySet),
	})
	if shouldIgnoreUpdateVersionTag(cli, merged.Tag, merged.TagPolicy) {
		merged.Tag = ""
	}

	targetDir, err := expandPath(merged.Target)
	if err != nil {
		return install.Options{}, err
	}
	cacheDir, err := expandPath(merged.CacheDir)
	if err != nil {
		return install.Options{}, err
	}
	guiTarget, err := expandPath(merged.GuiTarget)
	if err != nil {
		return install.Options{}, err
	}
	sys7zPath, err := expandPath(merged.Sys7zPath)
	if err != nil {
		return install.Options{}, err
	}
	apiCacheDir := ""
	if cacheDir != "" {
		apiCacheDir = filepath.Join(cacheDir, "api-cache")
	}

	output := targetDir
	if preferCacheDir && cli.Output == "" && cacheDir != "" {
		output = cacheDir
	}

	apiCacheEnabled := false
	if cfg.ApiCache.Enable != nil {
		apiCacheEnabled = *cfg.ApiCache.Enable
	}
	apiCacheTime := 0
	if cfg.ApiCache.CacheTime != nil {
		apiCacheTime = *cfg.ApiCache.CacheTime
	}
	ghproxyHostURL := util.DerefString(cfg.Ghproxy.HostURL)
	batchConcurrency := 0
	if cli.BatchConcurrencySet || cli.BatchConcurrency > 0 {
		batchConcurrency = cli.BatchConcurrency
	} else if cfg.Global.BatchConcurrency != nil {
		batchConcurrency = *cfg.Global.BatchConcurrency
	}

	cacheName := merged.Name
	if cli.CacheName != "" {
		cacheName = cli.CacheName
	}
	cacheVersion := merged.Tag
	if cli.CacheVersion != "" {
		cacheVersion = cli.CacheVersion
	}

	return install.Options{
		Tag:                 merged.Tag,
		TagPolicy:           merged.TagPolicy,
		Prerelease:          merged.Prerelease,
		Operation:           cli.Operation,
		CurrentVersion:      cli.CurrentVersion,
		TargetVersion:       cli.TargetVersion,
		Name:                merged.Name,
		Source:              merged.Source,
		SourcePath:          merged.SourcePath,
		Sys7zPath:           sys7zPath,
		Output:              output,
		OutputExplicit:      cli.Output != "",
		GuiTarget:           guiTarget,
		IsGUI:               merged.IsGUI,
		InstallMode:         merged.InstallMode,
		CacheDir:            cacheDir,
		CacheName:           cacheName,
		CacheVersion:        cacheVersion,
		ProxyURL:            proxy.URL,
		ProxyExclude:        append([]string(nil), proxy.Exclude...),
		NoProxy:             cli.NoProxy,
		UserAgent:           merged.UserAgent,
		APICacheEnabled:     apiCacheEnabled,
		APICacheDir:         apiCacheDir,
		APICacheTime:        apiCacheTime,
		CacheMirror:         CacheMirrorOptionsFromConfig(cfg),
		GhproxyEnabled:      cli.GhproxyEnabled,
		GhproxyHostURL:      ghproxyHostURL,
		GhproxyFallbacks:    append([]string(nil), cfg.Ghproxy.Fallbacks...),
		System:              merged.System,
		ExtractFile:         merged.File,
		All:                 merged.ExtractAll,
		StripComponents:     merged.StripComponents,
		Quiet:               merged.Quiet,
		DownloadOnly:        merged.DownloadOnly,
		FallbackVersions:    cli.FallbackVersions,
		Retries:             cli.Retries,
		ChunkConcurrency:    merged.ChunkConcurrency,
		BatchConcurrency:    batchConcurrency,
		ChunkConcurrencySet: true,
		BatchConcurrencySet: true,
		UpgradeOnly:         merged.UpgradeOnly,
		Asset:               append([]string(nil), merged.AssetFilters...),
		RenameFiles:         util.CloneStringMap(merged.RenameFiles),
		Hash:                merged.ShowHash,
		Verify:              merged.Verify,
		URLTemplate: install.URLTemplateOptions{
			URLTemplate:         merged.URLTemplate,
			LatestURL:           merged.LatestURL,
			LatestFormat:        merged.LatestFormat,
			LatestJSONPath:      merged.LatestJSONPath,
			VersionRegex:        merged.VersionRegex,
			OSMap:               util.CloneStringMap(merged.OSMap),
			ArchMap:             util.CloneStringMap(merged.ArchMap),
			ExtMap:              util.CloneStringMap(merged.ExtMap),
			LibcMap:             util.CloneStringMap(merged.LibcMap),
			ChecksumURLTemplate: merged.ChecksumURLTemplate,
			ChecksumFormat:      merged.ChecksumFormat,
			ChecksumJSONPath:    merged.ChecksumJSONPath,
			ChecksumRegex:       merged.ChecksumRegex,
			InstallAction:       merged.InstallAction,
			InstallArgs:         append([]string(nil), merged.InstallArgs...),
		},
		DisableSSL: merged.DisableSSL,
	}, nil
}

func shouldIgnoreUpdateVersionTag(cli install.Options, tag, policy string) bool {
	return cli.Operation == install.OperationUpdate && cli.Tag == "" && policy == "" && isVersionTag(tag)
}
