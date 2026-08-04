package app

import (
	"fmt"

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
}

type UpdateResult struct {
	Name   string
	Target string
	Result RunResult
}

type UpdatePackageResult struct {
	Name         string
	Target       string
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
