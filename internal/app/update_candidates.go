package app

import (
	"context"
	"fmt"
	"sort"

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

	// External packages are batched per manager, one command instead of one
	// check per package.
	externalOutdated, externalFailures, externalChecked, err := s.externalCandidates(cfg)
	if err != nil {
		return nil, nil, 0, err
	}
	outdated = append(outdated, externalOutdated...)
	failures = append(failures, externalFailures...)
	checked += externalChecked
	sortOutdatedItems(outdated)
	return outdated, failures, checked, nil
}

// externalCandidates collects the outdated packages of the selected external
// managers. With a zero ManagersSelection nothing runs.
func (s UpdateService) externalCandidates(cfg *cfgpkg.File) ([]OutdatedItem, []OutdatedCheckFailure, int, error) {
	if !s.Managers.Enabled() || s.External == nil {
		return nil, nil, 0, nil
	}
	listService := ListService{External: s.External, Managers: s.Managers}
	outdated, failures, checked := listService.externalOutdatedItems(ignoreUpdatePackageSet(cfg))
	return outdated, failures, checked, nil
}

func sortOutdatedItems(items []OutdatedItem) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Name != items[j].Name {
			return items[i].Name < items[j].Name
		}
		return items[i].Manager < items[j].Manager
	})
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
	externalTargets := make([][2]string, 0)
	seen := make(map[string]bool, len(targets))
	for _, target := range targets {
		item, entry, _, ok := findUpdateTarget(cfg, installed, target)
		if !ok {
			// Explicit external references are checked too.
			if managerName, pkgName, found := s.resolveExternalTarget(target); found {
				externalTargets = append(externalTargets, [2]string{managerName, pkgName})
				continue
			}
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
	for _, pair := range externalTargets {
		externalOutdated, externalFailures, externalChecked := s.checkExternalTarget(pair[0], pair[1])
		outdated = append(outdated, externalOutdated...)
		failures = append(failures, externalFailures...)
		checked += externalChecked
	}
	sortOutdatedItems(outdated)
	return outdated, failures, checked, nil
}

// checkExternalTarget checks one explicit "manager:pkg" reference.
func (s UpdateService) checkExternalTarget(managerName, pkgName string) ([]OutdatedItem, []OutdatedCheckFailure, int) {
	outdated, _, err := s.External.Outdated(context.Background(), managerName)
	if err != nil {
		return nil, []OutdatedCheckFailure{{Name: managerName, Repo: managerName, Error: err}}, 0
	}
	for _, pkg := range outdated {
		if pkg.Name != pkgName {
			continue
		}
		return []OutdatedItem{{
			Name:         pkg.Name,
			Repo:         managerName + ":" + pkg.Name,
			InstalledTag: pkg.Version,
			LatestTag:    pkg.Latest,
			Manager:      managerName,
		}}, nil, 1
	}
	return nil, nil, 1
}
