package app

import (
	"context"
	"fmt"
	"strings"

	cfgpkg "github.com/inherelab/eget/internal/config"
	"github.com/inherelab/eget/internal/install"
	storepkg "github.com/inherelab/eget/internal/installed"
)

type Installer interface {
	InstallTarget(target string, opts install.Options, extras ...InstallExtras) (RunResult, error)
}

type UpdateService struct {
	Install       Installer
	LoadConfig    func() (*cfgpkg.File, error)
	LoadInstalled func() (*storepkg.Config, error)
	LatestInfo    LatestInfoFunc
	OnCheckDone   func(checked, total int)
	OnUpdateStart func(index, total int, name string)
	// OnUpdateDone reports every finished candidate update. External
	// candidates are silent otherwise: their manager output is captured, not
	// printed like the installer's own progress. Concurrent batch updates
	// call it from several goroutines.
	OnUpdateDone func(item OutdatedItem, result RunResult, err error)
	// External and Managers let updates reach packages owned by external
	// managers. Both are optional: with a zero ManagersSelection only explicit
	// "manager:pkg" targets work.
	External ExternalProvider
	Managers ManagersSelection
}

type UpdateResult struct {
	Name   string
	Target string
	Result RunResult
}

type UpdatePackageResult struct {
	Name   string
	Target string
	// Manager is set for packages owned by an external package manager.
	Manager      string
	InstalledTag string
	LatestTag    string
	Updated      bool
	Result       RunResult
}

func (s UpdateService) UpdatePackage(nameOrRepo string, cli install.Options) (RunResult, error) {
	result, err := s.UpdatePackageStatus(nameOrRepo, cli)
	return result.Result, err
}

func (s UpdateService) UpdatePackageStatus(nameOrRepo string, cli install.Options) (UpdatePackageResult, error) {
	cfg, err := s.loadConfig()
	if err != nil {
		return UpdatePackageResult{}, err
	}
	installed, err := s.loadInstalled()
	if err != nil {
		return UpdatePackageResult{}, err
	}

	item, entry, managed, ok := findUpdateTarget(cfg, installed, nameOrRepo)
	if !ok {
		// Not an eget target: an explicit "manager:pkg" reference still works,
		// and a bare name resolves to an external package only when external
		// packages are selected at all.
		if managerName, pkgName, found := s.resolveExternalTarget(nameOrRepo); found {
			return s.updateExternalPackage(managerName, pkgName)
		}
		return UpdatePackageResult{}, fmt.Errorf("update target %q is not configured or installed; use install first", nameOrRepo)
	}
	if !item.Installed {
		return UpdatePackageResult{}, fmt.Errorf("update target %q is not installed; use install first", nameOrRepo)
	}
	if s.LatestInfo == nil {
		return UpdatePackageResult{}, fmt.Errorf("latest info checker is required")
	}
	enrichListItemFromInstalledEntry(&item, entry)
	check := checkOutdatedItem(item, s.LatestInfo)
	if check.failure != nil {
		return UpdatePackageResult{}, check.failure.Error
	}
	latestTag := item.InstalledTag
	if check.outdated != nil {
		latestTag = check.outdated.LatestTag
	}
	status := UpdatePackageResult{Name: item.Name, Target: item.Repo, InstalledTag: item.InstalledTag, LatestTag: latestTag}
	if check.outdated == nil {
		return status, nil
	}

	target := item.Name
	opts := cli
	if !managed {
		target = installedUpdateTarget(item, entry)
		opts = applyUpdateCLIOverrides(optionsFromInstalledEntry(entry), cli)
		if opts.Name == "" {
			opts.Name = item.Name
		}
	}
	opts.Operation = install.OperationUpdate
	opts.CurrentVersion = item.InstalledTag
	opts.TargetVersion = check.outdated.LatestTag
	result, err := s.Install.InstallTarget(target, opts)
	if err != nil {
		return UpdatePackageResult{}, err
	}
	status.Result = result
	status.Updated = true
	return status, nil
}

func (s UpdateService) UpdateAllPackages(cli install.Options) ([]UpdateResult, error) {
	candidates, _, _, err := s.ListUpdateCandidates()
	if err != nil {
		return nil, err
	}

	return s.UpdateCandidates(candidates, cli)
}

func (s UpdateService) loadConfig() (*cfgpkg.File, error) {
	if s.LoadConfig != nil {
		return s.LoadConfig()
	}
	return cfgpkg.Load()
}

// resolveExternalTarget resolves an external update target. A "manager:pkg"
// reference is always valid; a bare name only resolves when external packages
// are selected, and only to a package exactly one manager owns.
func (s UpdateService) resolveExternalTarget(target string) (string, string, bool) {
	if s.External == nil {
		return "", "", false
	}
	if managerName, pkgName, ok := strings.Cut(target, ":"); ok {
		if managerName == "" || pkgName == "" {
			return "", "", false
		}
		if _, found := s.External.Manager(managerName); !found {
			return "", "", false
		}
		return managerName, pkgName, true
	}
	if !s.Managers.Enabled() {
		return "", "", false
	}

	packages, _, err := s.External.List(context.Background(), s.Managers.Managers...)
	if err != nil {
		return "", "", false
	}
	managerName := ""
	for _, pkg := range packages {
		if pkg.Name != target {
			continue
		}
		if managerName != "" && managerName != pkg.Manager {
			// Owned by more than one manager: require an explicit reference.
			return "", "", false
		}
		managerName = pkg.Manager
	}
	if managerName == "" {
		return "", "", false
	}
	return managerName, target, true
}

// updateExternalPackage updates one package through its manager. When the
// manager can detect outdated packages the update only runs if the package is
// actually behind, so an up-to-date package reports its version instead of
// being upgraded blindly.
func (s UpdateService) updateExternalPackage(managerName, pkgName string) (UpdatePackageResult, error) {
	manager, ok := s.External.Manager(managerName)
	if !ok {
		return UpdatePackageResult{}, fmt.Errorf("unknown manager %q", managerName)
	}
	status := UpdatePackageResult{Name: pkgName, Target: managerName + ":" + pkgName, Manager: managerName}

	latest := ""
	if manager.SupportsOutdated() {
		outdated, _, err := s.External.Outdated(context.Background(), managerName)
		if err != nil {
			return status, err
		}
		for _, pkg := range outdated {
			if pkg.Name == pkgName {
				status.InstalledTag, latest = pkg.Version, pkg.Latest
				break
			}
		}
	}

	if status.InstalledTag == "" {
		packages, _, err := s.External.List(context.Background(), managerName)
		if err != nil {
			return status, err
		}
		found := false
		for _, pkg := range packages {
			if pkg.Name == pkgName {
				status.InstalledTag, found = pkg.Version, true
				break
			}
		}
		if !found {
			return status, fmt.Errorf("package %q is not installed by %s", pkgName, managerName)
		}
	}

	if latest == "" {
		if manager.SupportsOutdated() {
			// The manager knows how to compare versions and did not report it.
			status.LatestTag = status.InstalledTag
			return status, nil
		}
		// No outdated support (cargo): upgrade on request.
	} else {
		status.LatestTag = latest
	}

	if _, err := s.External.Upgrade(context.Background(), managerName, []string{pkgName}); err != nil {
		return status, err
	}
	status.Updated = true
	return status, nil
}

func (s UpdateService) loadInstalled() (*storepkg.Config, error) {
	if s.LoadInstalled != nil {
		return s.LoadInstalled()
	}
	store, err := storepkg.DefaultStore()
	if err != nil {
		return nil, err
	}
	return store.Load()
}
