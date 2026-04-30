package app

import (
	"fmt"
	"sort"

	cfgpkg "github.com/inherelab/eget/internal/config"
	"github.com/inherelab/eget/internal/install"
	storepkg "github.com/inherelab/eget/internal/installed"
	"github.com/inherelab/eget/internal/util"
)

type Installer interface {
	InstallTarget(target string, opts install.Options, extras ...InstallExtras) (RunResult, error)
}

type UpdateService struct {
	Install       Installer
	LoadConfig    func() (*cfgpkg.File, error)
	LoadInstalled func() (*storepkg.Config, error)
	LatestTag     func(repo, sourcePath string) (string, error)
}

type UpdateResult struct {
	Name   string
	Target string
	Result RunResult
}

func (s UpdateService) UpdatePackage(nameOrRepo string, cli install.Options) (RunResult, error) {
	cfg, err := s.loadConfig()
	if err != nil {
		return RunResult{}, err
	}

	if pkg, ok := cfg.Packages[nameOrRepo]; ok {
		if util.DerefString(pkg.Repo) == "" {
			return RunResult{}, fmt.Errorf("package %q has no repo", nameOrRepo)
		}
		return s.Install.InstallTarget(nameOrRepo, cli)
	}

	if isDirectUpdateTarget(nameOrRepo) {
		return s.Install.InstallTarget(nameOrRepo, cli)
	}

	return RunResult{}, fmt.Errorf("unknown package %q", nameOrRepo)
}

func isDirectUpdateTarget(target string) bool {
	switch install.DetectTargetKind(target) {
	case install.TargetRepo,
		install.TargetGitHubURL,
		install.TargetDirectURL,
		install.TargetLocalFile,
		install.TargetSourceForge,
		install.TargetForge:
		return true
	default:
		return false
	}
}

func (s UpdateService) UpdateAllPackages(cli install.Options) ([]UpdateResult, error) {
	candidates, _, _, err := s.ListUpdateCandidates()
	if err != nil {
		return nil, err
	}

	return s.UpdateCandidates(candidates, cli)
}

func (s UpdateService) ListUpdateCandidates() ([]OutdatedItem, []OutdatedCheckFailure, int, error) {
	if s.LatestTag == nil {
		return nil, nil, 0, fmt.Errorf("latest tag checker is required")
	}

	cfg, err := s.loadConfig()
	if err != nil {
		return nil, nil, 0, err
	}

	managedNames := make(map[string]bool, len(cfg.Packages))
	managedRepos := make(map[string]bool, len(cfg.Packages))
	for name, pkg := range cfg.Packages {
		managedNames[name] = true
		if repo := util.DerefString(pkg.Repo); repo != "" {
			managedRepos[repo] = true
		}
	}

	listService := ListService{
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfg, nil
		},
		LoadInstalled: s.loadInstalled,
		LatestTag:     s.LatestTag,
	}
	items, err := listService.ListPackages()
	if err != nil {
		return nil, nil, 0, err
	}

	outdated := make([]OutdatedItem, 0, len(items))
	failures := make([]OutdatedCheckFailure, 0)
	checked := 0
	for _, item := range items {
		if !managedNames[item.Name] && !managedRepos[item.Repo] {
			continue
		}
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

		latestTag, err := s.LatestTag(item.Repo, item.SourcePath)
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

func (s UpdateService) UpdateCandidates(candidates []OutdatedItem, cli install.Options) ([]UpdateResult, error) {
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Name < candidates[j].Name
	})

	results := make([]UpdateResult, 0, len(candidates))
	for _, item := range candidates {
		result, err := s.UpdatePackage(item.Name, cli)
		if err != nil {
			return nil, err
		}
		results = append(results, UpdateResult{
			Name:   item.Name,
			Target: item.Repo,
			Result: result,
		})
	}
	return results, nil
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

func boolOpt(value bool) *bool {
	if !value {
		return nil
	}
	return &value
}

func stringOpt(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func stringsOpt(value []string) *[]string {
	if len(value) == 0 {
		return nil
	}
	copied := append([]string(nil), value...)
	return &copied
}
