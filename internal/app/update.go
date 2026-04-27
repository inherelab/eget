package app

import (
	"fmt"
	"sort"
	"strings"

	cfgpkg "github.com/inherelab/eget/internal/config"
	"github.com/inherelab/eget/internal/install"
	"github.com/inherelab/eget/internal/util"
)

type Installer interface {
	InstallTarget(target string, opts install.Options, extras ...InstallExtras) (RunResult, error)
}

type UpdateService struct {
	Install    Installer
	LoadConfig func() (*cfgpkg.File, error)
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

	if strings.Contains(nameOrRepo, "/") {
		return s.Install.InstallTarget(nameOrRepo, cli)
	}

	return RunResult{}, fmt.Errorf("unknown package %q", nameOrRepo)
}

func (s UpdateService) UpdateAllPackages(cli install.Options) ([]UpdateResult, error) {
	cfg, err := s.loadConfig()
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(cfg.Packages))
	for name := range cfg.Packages {
		names = append(names, name)
	}
	sort.Strings(names)

	results := make([]UpdateResult, 0, len(names))
	for _, name := range names {
		result, err := s.UpdatePackage(name, cli)
		if err != nil {
			return nil, err
		}
		results = append(results, UpdateResult{
			Name:   name,
			Target: util.DerefString(cfg.Packages[name].Repo),
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
