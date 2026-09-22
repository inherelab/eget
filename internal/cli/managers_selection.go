package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gookit/goutil/x/ccolor"
	"github.com/inherelab/eget/internal/app"
	cfgpkg "github.com/inherelab/eget/internal/config"
	"github.com/inherelab/eget/internal/extpkg"
)

// managersSelectAll is the built-in selector value that means "every
// configured manager". Manager names themselves come from the built-in
// adapters plus [managers.<name>] sections, so nothing has to be collected
// here.
const managersSelectAll = "all"

// managersModeOn is the only enabling value of [global] managers_mode.
const managersModeOn = "on"

// resolveManagersSelection turns the --managers / --with-managers flags and the
// [global] managers_mode default into the selection used by list and update.
//
// Precedence: flags > managers_mode > off. A zero selection is off and starts
// no manager process at all.
func (s *cliService) resolveManagersSelection(managersFlag, withManagersFlag string) (app.ManagersSelection, error) {
	managersFlag = strings.TrimSpace(managersFlag)
	withManagersFlag = strings.TrimSpace(withManagersFlag)

	if managersFlag != "" && withManagersFlag != "" {
		return app.ManagersSelection{}, fmt.Errorf("--managers and --with-managers cannot be used together")
	}
	switch {
	case managersFlag != "":
		names, err := s.parseManagersSelector(managersFlag)
		if err != nil {
			return app.ManagersSelection{}, err
		}
		return app.ManagersSelection{Mode: app.ManagersModeOnly, Managers: names}, nil
	case withManagersFlag != "":
		names, err := s.parseManagersSelector(withManagersFlag)
		if err != nil {
			return app.ManagersSelection{}, err
		}
		return app.ManagersSelection{Mode: app.ManagersModeWith, Managers: names}, nil
	}

	mode, err := s.configuredManagersMode()
	if err != nil {
		return app.ManagersSelection{}, err
	}
	if mode == managersModeOn {
		return app.ManagersSelection{Mode: app.ManagersModeWith}, nil
	}
	return app.ManagersSelection{Mode: app.ManagersModeOff}, nil
}

// parseManagersSelector accepts "all" or a comma separated list of manager
// names, validating each against the configured managers.
func (s *cliService) parseManagersSelector(selector string) ([]string, error) {
	if strings.EqualFold(selector, managersSelectAll) {
		return nil, nil
	}

	names := make([]string, 0, 2)
	for _, part := range strings.Split(selector, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if !s.hasManager(part) {
			return nil, fmt.Errorf("unknown manager %q, available: %s", part, strings.Join(s.managerNames(), ", "))
		}
		names = append(names, part)
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("--managers/--with-managers requires %q or a manager name list", managersSelectAll)
	}
	sort.Strings(names)
	return names, nil
}

func (s *cliService) hasManager(name string) bool {
	if s.extService == nil {
		return false
	}
	_, ok := s.extService.Manager(name)
	return ok
}

func (s *cliService) managerNames() []string {
	if s.extService == nil {
		return nil
	}
	return s.extService.Names()
}

// configuredManagersMode reads [global] managers_mode, defaulting to off.
func (s *cliService) configuredManagersMode() (string, error) {
	cfg, err := s.loadConfigFile()
	if err != nil {
		return "", err
	}
	if cfg == nil || cfg.Global.ManagersMode == nil {
		return app.ManagersModeOff, nil
	}
	mode := strings.ToLower(strings.TrimSpace(*cfg.Global.ManagersMode))
	switch mode {
	case "", app.ManagersModeOff:
		return app.ManagersModeOff, nil
	case managersModeOn:
		return managersModeOn, nil
	default:
		return "", fmt.Errorf("invalid global.managers_mode %q, want %q or %q", mode, app.ManagersModeOff, managersModeOn)
	}
}

func (s *cliService) loadConfigFile() (*cfgpkg.File, error) {
	if s != nil && s.cfgService.Load != nil {
		return s.cfgService.Load()
	}
	return cfgpkg.Load()
}

// applyManagersSelection stores the selection on the list and update services.
// Both are value copies on the cliService, so this only affects the current
// invocation; the returned function restores the previous values.
func (s *cliService) applyManagersSelection(selection app.ManagersSelection) func() {
	prevList := s.listService.Managers
	prevUpdate := s.updService.Managers
	s.listService.Managers = selection
	s.updService.Managers = selection
	return func() {
		s.listService.Managers = prevList
		s.updService.Managers = prevUpdate
	}
}

// applyListManagersSelection also wires the failure printer: the plain list
// path has no failure channel of its own.
func (s *cliService) applyListManagersSelection(selection app.ManagersSelection) func() {
	restore := s.applyManagersSelection(selection)
	prevFailure := s.listService.OnExternalFailure
	s.listService.OnExternalFailure = func(failure extpkg.Failure) {
		name := failure.Manager
		if name == "" {
			name = "managers"
		}
		ccolor.Fprintf(s.stderrWriter(), "<yellow>check_failed</> %s: %v\n", name, failure.Err)
	}
	return func() {
		s.listService.OnExternalFailure = prevFailure
		restore()
	}
}

// listManagersSelection resolves the list flags and rejects combinations that
// would silently drop half of what the user asked for.
func (s *cliService) listManagersSelection(opts *ListOptions) (app.ManagersSelection, error) {
	if opts == nil {
		return app.ManagersSelection{}, nil
	}
	selection, err := s.resolveManagersSelection(opts.Managers, opts.WithManagers)
	if err != nil {
		return app.ManagersSelection{}, err
	}
	switch selection.Mode {
	case app.ManagersModeOnly:
		for _, other := range []struct {
			set  bool
			name string
		}{
			{opts.All, "--all"},
			{opts.GUI, "--gui"},
			{opts.NoInstalled, "--no-installed"},
		} {
			if other.set {
				return app.ManagersSelection{}, fmt.Errorf("--managers cannot be used with %s", other.name)
			}
		}
	case app.ManagersModeWith:
		if opts.GUI {
			return app.ManagersSelection{}, fmt.Errorf("--with-managers cannot be used with --gui")
		}
		if opts.NoInstalled {
			return app.ManagersSelection{}, fmt.Errorf("--with-managers cannot be used with --no-installed")
		}
	}
	return selection, nil
}

// updateManagersSelection resolves the update flags. --managers already names
// the whole selection, so it implies --all for that selection.
func (s *cliService) updateManagersSelection(opts *UpdateOptions) (app.ManagersSelection, error) {
	if opts == nil {
		return app.ManagersSelection{}, nil
	}
	if opts.Self {
		if opts.Managers != "" || opts.WithManagers != "" {
			return app.ManagersSelection{}, fmt.Errorf("update --self cannot be used with --managers/--with-managers")
		}
		return app.ManagersSelection{}, nil
	}
	selection, err := s.resolveManagersSelection(opts.Managers, opts.WithManagers)
	if err != nil {
		return app.ManagersSelection{}, err
	}
	if selection.Mode == app.ManagersModeWith && !opts.Check && !opts.All && !opts.Interactive && len(opts.Targets) == 0 {
		return app.ManagersSelection{}, fmt.Errorf("--with-managers requires --check, --all, --interactive or a target")
	}
	return selection, nil
}
