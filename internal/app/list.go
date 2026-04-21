package app

import (
	"sort"
	"strings"
	"time"

	cfgpkg "github.com/inherelab/eget/internal/config"
	storepkg "github.com/inherelab/eget/internal/installed"
)

type InstalledLoader interface {
	Load() (*storepkg.Config, error)
}

type ListItem struct {
	Name        string
	Repo        string
	Target      string
	Tag         string
	Installed   bool
	InstalledAt time.Time
	Asset       string
	URL         string
}

type ListService struct {
	LoadConfig    func() (*cfgpkg.File, error)
	LoadInstalled func() (*storepkg.Config, error)
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
		repo := derefString(pkg.Repo)
		byName[name] = ListItem{
			Name:   name,
			Repo:   repo,
			Target: derefString(pkg.Target),
			Tag:    derefString(pkg.Tag),
		}
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
			item.InstalledAt = entry.InstalledAt
			item.Asset = entry.Asset
			item.URL = entry.URL
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
