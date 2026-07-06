package app

import (
	"fmt"

	cfgpkg "github.com/inherelab/eget/internal/config"
	"github.com/inherelab/eget/internal/install"
)

func (s UpdateService) ListUpdateCandidates() ([]OutdatedItem, []OutdatedCheckFailure, int, error) {
	if s.LatestInfo == nil {
		return nil, nil, 0, fmt.Errorf("latest info checker is required")
	}

	cfg, err := s.loadConfig()
	if err != nil {
		return nil, nil, 0, err
	}

	listService := ListService{
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfg, nil
		},
		LoadInstalled: s.loadInstalled,
		LatestInfo:    s.LatestInfo,
	}
	items, err := listService.ListPackages()
	if err != nil {
		return nil, nil, 0, err
	}

	outdated, failures, checked := checkOutdatedItems(items, s.LatestInfo, nil, batchConcurrencyFromConfig(cfg, install.Options{}), s.OnCheckDone)
	return outdated, failures, checked, nil
}

func (s UpdateService) ListUpdateCandidatesForTargets(targets []string) ([]OutdatedItem, []OutdatedCheckFailure, int, error) {
	if s.LatestInfo == nil {
		return nil, nil, 0, fmt.Errorf("latest info checker is required")
	}
	if len(targets) == 0 {
		return s.ListUpdateCandidates()
	}

	cfg, err := s.loadConfig()
	if err != nil {
		return nil, nil, 0, err
	}
	installed, err := s.loadInstalled()
	if err != nil {
		return nil, nil, 0, err
	}

	items := make([]ListItem, 0, len(targets))
	seen := make(map[string]bool, len(targets))
	for _, target := range targets {
		item, entry, _, ok := findUpdateTarget(cfg, installed, target)
		if !ok {
			return nil, nil, 0, fmt.Errorf("update target %q is not configured or installed; use install first", target)
		}
		if !item.Installed {
			return nil, nil, 0, fmt.Errorf("update target %q is not installed; use install first", target)
		}
		enrichListItemFromInstalledEntry(&item, entry)
		key := item.Name
		if key == "" {
			key = item.Repo
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		items = append(items, item)
	}

	outdated, failures, checked := checkOutdatedItems(items, s.LatestInfo, nil, batchConcurrencyFromConfig(cfg, install.Options{}), s.OnCheckDone)
	return outdated, failures, checked, nil
}
