package cli

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/gookit/goutil/cliutil"
	"github.com/gookit/goutil/x/ccolor"
)

// handleManagersList shows every configured manager, whether it is available
// and how many packages it owns.
func (s *cliService) handleManagersList() error {
	if s.extService == nil {
		return fmt.Errorf("external manager service is required")
	}

	packages, failures, err := s.extService.List(context.Background())
	if err != nil {
		return err
	}
	counts := make(map[string]int, len(packages))
	for _, pkg := range packages {
		counts[pkg.Manager]++
	}
	failed := make(map[string]error, len(failures))
	for _, failure := range failures {
		failed[failure.Manager] = failure.Err
	}

	names := s.extService.Names()
	rows := make([][]any, 0, len(names))
	for _, name := range names {
		manager, ok := s.extService.Manager(name)
		if !ok {
			continue
		}
		available := "no"
		bin := manager.Bin
		if path, lookErr := exec.LookPath(manager.Bin); lookErr == nil {
			available = "yes"
			bin = path
		}
		outdated := "no"
		if manager.SupportsOutdated() {
			outdated = "yes"
		}
		count := fmt.Sprintf("%d", counts[name])
		if err := failed[name]; err != nil {
			count = "error"
		}
		rows = append(rows, []any{name, bin, available, outdated, count})
	}

	if len(rows) == 0 {
		ccolor.Infoln("no external managers configured")
		return nil
	}
	cols := []string{"Manager", "Bin", "Available", "Outdated", "Packages"}
	ccolor.Print(cliutil.FormatTable(cols, rows, cliutil.MinimalStyle))
	for _, failure := range failures {
		ccolor.Fprintf(s.stderrWriter(), "<yellow>check_failed</> %s: %v\n", failure.Manager, failure.Err)
	}
	return nil
}

// handleManagersUpgrade upgrades one manager as a whole, or the named packages
// of that manager.
func (s *cliService) handleManagersUpgrade(opts *ManagersOptions) error {
	if s.extService == nil {
		return fmt.Errorf("external manager service is required")
	}
	if opts == nil || len(opts.Targets) == 0 {
		return fmt.Errorf("managers upgrade requires a manager name")
	}
	managerName := opts.Targets[0]
	names := opts.Targets[1:]
	if _, ok := s.extService.Manager(managerName); !ok {
		return fmt.Errorf("unknown manager %q, available: %v", managerName, s.extService.Names())
	}

	result, err := s.extService.Upgrade(context.Background(), managerName, names)
	if err != nil {
		return err
	}
	if len(result.Names) == 0 {
		ccolor.Cyanf("%s: no packages to upgrade\n", managerName)
		return nil
	}
	ccolor.Successf("✅ %s: upgraded %d packages\n", result.Manager, len(result.Names))
	if result.Output != "" && appVerbose() {
		ccolor.Grayln(result.Output)
	}
	return nil
}
