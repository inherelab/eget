package app

import (
	"fmt"
	"sort"
	"strings"
	"time"

	cfgpkg "github.com/inherelab/eget/internal/config"
	storepkg "github.com/inherelab/eget/internal/installed"
	"github.com/inherelab/eget/internal/util"
)

type InstalledLoader interface {
	Load() (*storepkg.Config, error)
}

type ListItem struct {
	Name         string
	Repo         string
	Target       string
	Tag          string
	Version      string
	InstalledTag string
	Installed    bool
	InstalledAt  time.Time
	Asset        string
	URL          string
	IsGUI        bool
	InstallMode  string
}

type OutdatedItem struct {
	Name         string
	Repo         string
	Target       string
	InstalledTag string
	LatestTag    string
	InstalledAt  time.Time
}

type OutdatedCheckFailure struct {
	Name  string
	Repo  string
	Error error
}

type ListService struct {
	LoadConfig    func() (*cfgpkg.File, error)
	LoadInstalled func() (*storepkg.Config, error)
	LatestTag     func(repo string) (string, error)
}

func (s ListService) ListPackages() ([]ListItem, error) {
	cfg, err := s.loadConfig()
	if err != nil {
		return nil, err
	}
	installed, err := s.loadInstalled()
	if err != nil {
		return nil, err
	}

	byName := make(map[string]ListItem, len(cfg.Packages))
	for name, pkg := range cfg.Packages {
		repo := util.DerefString(pkg.Repo)
		item := ListItem{
			Name:   name,
			Repo:   repo,
			Target: util.DerefString(pkg.Target),
			Tag:    util.DerefString(pkg.Tag),
		}
		if pkg.IsGUI != nil && *pkg.IsGUI {
			item.IsGUI = true
		}
		byName[name] = item
	}

	if installed != nil && installed.Installed != nil {
		repoToName := make(map[string]string, len(byName))
		for name, item := range byName {
			if item.Repo != "" {
				repoToName[item.Repo] = name
			}
		}
		for repo, entry := range installed.Installed {
			name := repoToName[repo]
			if name == "" {
				name = repoName(repo)
			}
			item, ok := byName[name]
			if !ok {
				item = ListItem{
					Name: name,
					Repo: repo,
				}
			}
			if item.Repo == "" {
				item.Repo = repo
			}
			item.Installed = true
			item.Version = entry.Tag
			if item.Version == "" {
				item.Version = entry.Version
			}
			item.InstalledTag = entry.Tag
			item.InstalledAt = entry.InstalledAt
			item.Asset = entry.Asset
			item.URL = entry.URL
			if entry.IsGUI {
				item.IsGUI = true
			}
			if entry.InstallMode != "" {
				item.InstallMode = entry.InstallMode
			}
			byName[name] = item
		}
	}

	names := make([]string, 0, len(byName))
	for name := range byName {
		names = append(names, name)
	}
	sort.Strings(names)

	items := make([]ListItem, 0, len(names))
	for _, name := range names {
		items = append(items, byName[name])
	}
	return items, nil
}

func (s ListService) ListInstalledPackages() ([]ListItem, error) {
	items, err := s.ListPackages()
	if err != nil {
		return nil, err
	}
	installed := make([]ListItem, 0, len(items))
	for _, item := range items {
		if item.Installed {
			installed = append(installed, item)
		}
	}
	return installed, nil
}

func (s ListService) ListGUIPackages(all bool) ([]ListItem, error) {
	var items []ListItem
	var err error
	if all {
		items, err = s.ListPackages()
	} else {
		items, err = s.ListInstalledPackages()
	}
	if err != nil {
		return nil, err
	}
	gui := make([]ListItem, 0, len(items))
	for _, item := range items {
		if item.IsGUI {
			gui = append(gui, item)
		}
	}
	return gui, nil
}

func (s ListService) FindPackage(name string) (*ListItem, error) {
	items, err := s.ListPackages()
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.Name == name {
			found := item
			return &found, nil
		}
	}
	return nil, fmt.Errorf("package %q not found", name)
}

func (s ListService) ListOutdatedPackages() ([]OutdatedItem, []OutdatedCheckFailure, int, error) {
	if s.LatestTag == nil {
		return nil, nil, 0, fmt.Errorf("latest tag checker is required")
	}

	items, err := s.ListPackages()
	if err != nil {
		return nil, nil, 0, err
	}

	outdated := make([]OutdatedItem, 0, len(items))
	failures := make([]OutdatedCheckFailure, 0)
	checked := 0
	for _, item := range items {
		if !item.Installed || item.Repo == "" {
			continue
		}
		checked++
		if item.InstalledTag == "" {
			failures = append(failures, OutdatedCheckFailure{
				Name:  item.Name,
				Repo:  item.Repo,
				Error: fmt.Errorf("installed tag is empty"),
			})
			continue
		}

		latestTag, err := s.LatestTag(item.Repo)
		if err != nil {
			failures = append(failures, OutdatedCheckFailure{
				Name:  item.Name,
				Repo:  item.Repo,
				Error: err,
			})
			continue
		}
		if latestTag == "" || latestTag == item.InstalledTag {
			continue
		}

		outdated = append(outdated, OutdatedItem{
			Name:         item.Name,
			Repo:         item.Repo,
			Target:       item.Target,
			InstalledTag: item.InstalledTag,
			LatestTag:    latestTag,
			InstalledAt:  item.InstalledAt,
		})
	}
	return outdated, failures, checked, nil
}

func repoName(repo string) string {
	parts := strings.Split(repo, "/")
	if len(parts) == 2 && parts[1] != "" {
		return parts[1]
	}
	return repo
}

func (s ListService) loadConfig() (*cfgpkg.File, error) {
	if s.LoadConfig != nil {
		return s.LoadConfig()
	}
	return cfgpkg.Load()
}

func (s ListService) loadInstalled() (*storepkg.Config, error) {
	if s.LoadInstalled != nil {
		return s.LoadInstalled()
	}
	store, err := storepkg.DefaultStore()
	if err != nil {
		return nil, err
	}
	return store.Load()
}
