package extpkg

import (
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"sync"

	cfgpkg "github.com/inherelab/eget/internal/config"
)

// Service lists and upgrades packages owned by external managers.
type Service struct {
	Managers []Manager
	Runner   Runner
	LookPath func(string) (string, error)
}

// NewService builds the service from config, using the real process runner.
func NewService(cfg *cfgpkg.File) Service {
	return Service{
		Managers: Managers(cfg),
		Runner:   NewExecRunner(),
		LookPath: exec.LookPath,
	}
}

// Manager returns the manager with the given name.
func (s Service) Manager(name string) (Manager, bool) {
	for _, manager := range s.Managers {
		if manager.Name == name {
			return manager, true
		}
	}
	return Manager{}, false
}

// Names returns every configured manager name, sorted.
func (s Service) Names() []string {
	names := make([]string, 0, len(s.Managers))
	for _, manager := range s.Managers {
		names = append(names, manager.Name)
	}
	sort.Strings(names)
	return names
}

// Resolve splits a "<manager>:<pkg>" reference. An empty manager name or an
// unknown manager is not a valid reference.
func (s Service) Resolve(ref string) (Manager, string, bool) {
	managerName, pkgName, found := strings.Cut(ref, ":")
	if !found || managerName == "" || pkgName == "" {
		return Manager{}, "", false
	}
	manager, ok := s.Manager(managerName)
	if !ok {
		return Manager{}, "", false
	}
	return manager, pkgName, true
}

// Path reports the resolved binary path of a manager. A configured bin that
// contains a path separator is used as-is, which is the escape hatch for tools
// installed outside PATH (pipx, cargo, ...).
func (s Service) Path(manager Manager) (string, bool) {
	if s.LookPath == nil {
		return "", false
	}
	path, err := s.LookPath(manager.Bin)
	if err != nil {
		return "", false
	}
	return path, true
}

// List returns the packages installed by every available manager. Managers that
// are missing or fail are reported as failures instead of aborting the rest.
func (s Service) List(ctx context.Context) ([]Package, []Failure, error) {
	return s.collect(ctx, func(manager Manager) ([]string, error) {
		if !manager.SupportsList() {
			return nil, errNoListArgs
		}
		return manager.ListArgs, nil
	}, ParseList)
}

// Outdated returns the packages that are behind, using only the managers that
// can detect it.
func (s Service) Outdated(ctx context.Context) ([]Package, []Failure, error) {
	return s.collect(ctx, func(manager Manager) ([]string, error) {
		if !manager.SupportsOutdated() {
			return nil, errNoOutdatedSupport
		}
		return manager.OutdatedArgs, nil
	}, ParseOutdated)
}

var (
	errNoListArgs         = fmt.Errorf("manager has no list command")
	errNoOutdatedSupport  = fmt.Errorf("manager cannot detect outdated packages")
	errManagerUnavailable = fmt.Errorf("manager is not available")
)

type commandFunc func(manager Manager) ([]string, error)
type parseFunc func(manager Manager, res CommandResult) ([]Package, error)

// collect runs one command per available manager concurrently and merges the
// results in name order.
func (s Service) collect(ctx context.Context, argsFor commandFunc, parse parseFunc) ([]Package, []Failure, error) {
	if s.Runner == nil {
		return nil, nil, fmt.Errorf("manager runner is required")
	}

	type outcome struct {
		packages []Package
		failure  *Failure
	}

	managers := make([]Manager, 0, len(s.Managers))
	paths := make(map[string]string, len(s.Managers))
	for _, manager := range s.Managers {
		path, ok := s.Path(manager)
		if !ok {
			// Not installed (or not on PATH) is not a failure: skip quietly.
			continue
		}
		if _, err := argsFor(manager); err != nil {
			// No list command / no outdated support: also skipped quietly.
			continue
		}
		managers = append(managers, manager)
		paths[manager.Name] = path
	}

	results := make([]outcome, len(managers))
	var wg sync.WaitGroup
	for i, manager := range managers {
		args, err := argsFor(manager)
		if err != nil {
			continue
		}
		// Run the resolved path: Bin may be a bare name that is not on PATH, and
		// the configured bin may be an absolute path.
		bin := paths[manager.Name]
		wg.Add(1)
		go func(index int, manager Manager, bin string, args []string) {
			defer wg.Done()
			res, err := s.Runner.Run(ctx, bin, args, manager.Timeout)
			if err != nil {
				results[index] = outcome{failure: &Failure{Manager: manager.Name, Err: err}}
				return
			}
			packages, err := parse(manager, res)
			if err != nil {
				results[index] = outcome{failure: &Failure{Manager: manager.Name, Err: err}}
				return
			}
			results[index] = outcome{packages: packages}
		}(i, manager, bin, args)
	}
	wg.Wait()

	packages := make([]Package, 0)
	failures := make([]Failure, 0)
	for _, result := range results {
		packages = append(packages, result.packages...)
		if result.failure != nil {
			failures = append(failures, *result.failure)
		}
	}
	sort.SliceStable(packages, func(i, j int) bool {
		if packages[i].Manager != packages[j].Manager {
			return packages[i].Manager < packages[j].Manager
		}
		return packages[i].Name < packages[j].Name
	})
	return packages, failures, nil
}

// Upgrade runs the upgrade command of one manager. names may be empty, which
// means "the whole manager": that uses the manager's upgrade-all command when
// it has one, otherwise every installed package is upgraded by name.
//
// The returned From/To fields are left empty: the caller already knows the
// versions under check.
func (s Service) Upgrade(ctx context.Context, managerName string, names []string) (UpgradeResult, error) {
	manager, ok := s.Manager(managerName)
	if !ok {
		return UpgradeResult{}, fmt.Errorf("unknown manager %q", managerName)
	}
	if s.Runner == nil {
		return UpgradeResult{}, fmt.Errorf("manager runner is required")
	}
	bin, ok := s.Path(manager)
	if !ok {
		return UpgradeResult{}, fmt.Errorf("%s: %w", manager.Name, errManagerUnavailable)
	}
	if len(manager.UpgradeArgs) == 0 {
		return UpgradeResult{}, fmt.Errorf("%s: manager has no upgrade command", manager.Name)
	}

	names = append([]string(nil), names...)
	args := append([]string(nil), manager.UpgradeArgs...)
	if len(names) > 0 {
		args = append(args, names...)
	} else if len(manager.UpgradeAllArgs) > 0 {
		args = append([]string(nil), manager.UpgradeAllArgs...)
	} else {
		installed, _, err := s.List(ctx)
		if err != nil {
			return UpgradeResult{}, err
		}
		for _, pkg := range installed {
			if pkg.Manager == manager.Name {
				names = append(names, pkg.Name)
			}
		}
		if len(names) == 0 {
			return UpgradeResult{Manager: manager.Name}, nil
		}
		args = append(args, names...)
	}

	res, err := s.Runner.Run(ctx, bin, args, manager.Timeout)
	if err != nil {
		return UpgradeResult{}, err
	}
	if res.ExitCode != 0 {
		return UpgradeResult{}, outputError(manager, res, "upgrade failed")
	}
	return UpgradeResult{
		Manager: manager.Name,
		Names:   names,
		Output:  strings.TrimSpace(string(res.Stdout)),
	}, nil
}
