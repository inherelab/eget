package app

import (
	"sort"
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

	names := make([]string, 0, len(cfg.Packages))
	for name := range cfg.Packages {
		names = append(names, name)
	}
	sort.Strings(names)

	items := make([]ListItem, 0, len(names))
	for _, name := range names {
		pkg := cfg.Packages[name]
		repo := derefString(pkg.Repo)
		item := ListItem{
			Name:   name,
			Repo:   repo,
			Target: derefString(pkg.Target),
			Tag:    derefString(pkg.Tag),
		}
		if installed != nil && installed.Installed != nil {
			if entry, ok := installed.Installed[repo]; ok {
				item.Installed = true
				item.InstalledAt = entry.InstalledAt
				item.Asset = entry.Asset
				item.URL = entry.URL
			}
		}
		items = append(items, item)
	}
	return items, nil
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
