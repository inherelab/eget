package app

import (
	"fmt"
	"strings"
	"time"

	cfgpkg "github.com/inherelab/eget/internal/config"
	storepkg "github.com/inherelab/eget/internal/installed"
	forge "github.com/inherelab/eget/internal/source/forge"
	sourcesf "github.com/inherelab/eget/internal/source/sourceforge"
	"github.com/inherelab/eget/internal/util"
)

type ShowService struct {
	LoadConfig    func() (*cfgpkg.File, error)
	LoadInstalled func() (*storepkg.Config, error)
}

type ShowResult struct {
	Name           string
	Repo           string
	Desc           string
	Homepage       string
	RepoURL        string
	Configured     bool
	Installed      bool
	ConfigTarget   string
	InstallTarget  string
	Version        string
	Tag            string
	TagPolicy      string
	InstalledAt    time.Time
	UpdatedAt      time.Time
	Asset          string
	AssetURL       string
	Tool           string
	ExtractedFiles []string
	IsGUI          bool
	InstallMode    string
	SourcePath     string
	Options        map[string]any
}

func (s ShowService) ShowPackage(target string) (ShowResult, error) {
	cfg, err := s.loadConfig()
	if err != nil {
		return ShowResult{}, err
	}
	installed, err := s.loadInstalled()
	if err != nil {
		return ShowResult{}, err
	}

	name, pkg, configured := findShowConfigPackage(cfg, target)
	entry, installedName, installedOK := findShowInstalledEntry(installed, target, pkg)
	if !configured && !installedOK {
		return ShowResult{}, fmt.Errorf("package %q is not configured or installed", target)
	}

	result := ShowResult{
		Name:       name,
		Configured: configured,
		Installed:  installedOK,
	}
	if configured {
		result.Repo = util.DerefString(pkg.Repo)
		result.Desc = util.DerefString(pkg.Desc)
		result.ConfigTarget = util.DerefString(pkg.Target)
		result.SourcePath = util.DerefString(pkg.SourcePath)
		if pkg.IsGUI != nil {
			result.IsGUI = *pkg.IsGUI
		}
		result.InstallMode = util.DerefString(pkg.InstallMode)
		result.TagPolicy = util.DerefString(pkg.TagPolicy)
	}
	if installedOK {
		applyInstalledEntryToShowResult(&result, entry)
	}
	if result.Name == "" {
		result.Name = firstNonEmpty(showInstalledAliasName(installedName, entry), showRepoName(result.Repo))
	}
	if result.Homepage == "" {
		result.Homepage = inferRepoWebURL(result.Repo)
	}
	if result.RepoURL == "" {
		result.RepoURL = inferRepoWebURL(result.Repo)
	}
	return result, nil
}

func showInstalledAliasName(key string, entry storepkg.Entry) string {
	if key == "" || key == storepkg.NormalizeRepoName(entry.Repo) || key == storepkg.NormalizeRepoName(entry.Target) {
		return ""
	}
	if strings.Contains(key, "/") || strings.Contains(key, ":") {
		return ""
	}
	return key
}

func findShowConfigPackage(cfg *cfgpkg.File, target string) (string, cfgpkg.Section, bool) {
	if cfg == nil {
		cfg = cfgpkg.NewFile()
	}
	normalized := storepkg.NormalizeRepoName(target)
	for name, pkg := range cfg.Packages {
		repo := util.DerefString(pkg.Repo)
		if target == name || target == repo || normalized == storepkg.NormalizeRepoName(repo) {
			return name, pkg, true
		}
	}
	return "", cfgpkg.Section{}, false
}

func findShowInstalledEntry(installed *storepkg.Config, target string, pkg cfgpkg.Section) (storepkg.Entry, string, bool) {
	if installed == nil || installed.Installed == nil {
		return storepkg.Entry{}, "", false
	}
	candidates := []string{target, storepkg.NormalizeRepoName(target), util.DerefString(pkg.Repo)}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if entry, ok := installed.Installed[candidate]; ok {
			return entry, candidate, true
		}
		normalized := storepkg.NormalizeRepoName(candidate)
		if entry, ok := installed.Installed[normalized]; ok {
			return entry, normalized, true
		}
	}
	normalized := storepkg.NormalizeRepoName(target)
	for key, entry := range installed.Installed {
		if entry.Target == target ||
			storepkg.NormalizeRepoName(entry.Target) == normalized ||
			target == showRepoName(entry.Repo) ||
			target == showRepoName(entry.Target) {
			return entry, key, true
		}
	}
	return storepkg.Entry{}, "", false
}

func applyInstalledEntryToShowResult(result *ShowResult, entry storepkg.Entry) {
	if result.Repo == "" {
		result.Repo = entry.Repo
	}
	if result.Desc == "" {
		result.Desc = entry.Desc
	}
	result.Homepage = firstNonEmpty(result.Homepage, entry.Homepage)
	result.RepoURL = firstNonEmpty(result.RepoURL, entry.RepoURL)
	result.InstallTarget = entry.Target
	result.Version = firstNonEmpty(entry.Version, entry.Tag)
	result.Tag = entry.Tag
	result.TagPolicy = firstNonEmpty(installedTagPolicy(entry), result.TagPolicy)
	result.InstalledAt = entry.InstalledAt
	result.UpdatedAt = entry.UpdatedAt
	result.Asset = entry.Asset
	result.AssetURL = entry.URL
	result.Tool = entry.Tool
	result.ExtractedFiles = append([]string(nil), entry.ExtractedFiles...)
	result.IsGUI = entry.IsGUI
	result.InstallMode = entry.InstallMode
	result.Options = entry.Options
}

func showRepoName(repo string) string {
	if sfTarget, err := sourcesf.ParseTarget(repo); err == nil {
		return sfTarget.Project
	}
	if forgeTarget, err := forge.ParseTarget(repo); err == nil {
		return forgeTarget.Project
	}
	return repoName(repo)
}

func inferRepoWebURL(repo string) string {
	if repo == "" {
		return ""
	}
	if sfTarget, err := sourcesf.ParseTarget(repo); err == nil {
		return "https://sourceforge.net/projects/" + sfTarget.Project + "/"
	}
	if forgeTarget, err := forge.ParseTarget(repo); err == nil {
		return "https://" + forgeTarget.Host + "/" + forgeTarget.Namespace + "/" + forgeTarget.Project
	}
	if strings.Count(repo, "/") == 1 && !strings.Contains(repo, "://") {
		return "https://github.com/" + repo
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func (s ShowService) loadConfig() (*cfgpkg.File, error) {
	if s.LoadConfig != nil {
		return s.LoadConfig()
	}
	return cfgpkg.Load()
}

func (s ShowService) loadInstalled() (*storepkg.Config, error) {
	if s.LoadInstalled != nil {
		return s.LoadInstalled()
	}
	store, err := storepkg.DefaultStore()
	if err != nil {
		return nil, err
	}
	return store.Load()
}
