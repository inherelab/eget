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

// extSelectAll is the built-in selector value that means "every
// configured manager". Manager names themselves come from the built-in
// adapters plus [ext.<name>] sections, so nothing has to be collected
// here.
const extSelectAll = "all"

// extPackageModeOn is the only enabling value of [global] ext_package_mode.
const extPackageModeOn = "on"

// resolveExtSelection turns the --ext / --with-ext flags and the
// [global] ext_package_mode default into the selection used by list and update.
//
// Precedence: flags > ext_package_mode > off. A zero selection is off and starts
// no manager process at all.
func (s *cliService) resolveExtSelection(extFlag, withExtFlag string) (app.ManagersSelection, error) {
	extFlag = strings.TrimSpace(extFlag)
	withExtFlag = strings.TrimSpace(withExtFlag)

	if extFlag != "" && withExtFlag != "" {
		return app.ManagersSelection{}, fmt.Errorf("--ext and --with-ext cannot be used together")
	}
	switch {
	case extFlag != "":
		names, err := s.parseExtSelector(extFlag)
		if err != nil {
			return app.ManagersSelection{}, err
		}
		return app.ManagersSelection{Mode: app.ManagersModeOnly, Managers: names}, nil
	case withExtFlag != "":
		names, err := s.parseExtSelector(withExtFlag)
		if err != nil {
			return app.ManagersSelection{}, err
		}
		return app.ManagersSelection{Mode: app.ManagersModeWith, Managers: names}, nil
	}

	mode, err := s.configuredExtPackageMode()
	if err != nil {
		return app.ManagersSelection{}, err
	}
	if mode == extPackageModeOn {
		return app.ManagersSelection{Mode: app.ManagersModeWith}, nil
	}
	return app.ManagersSelection{Mode: app.ManagersModeOff}, nil
}

// parseExtSelector accepts "all" or a comma separated list of manager
// names, validating each against the configured managers.
func (s *cliService) parseExtSelector(selector string) ([]string, error) {
	if strings.EqualFold(selector, extSelectAll) {
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
		return nil, fmt.Errorf("--ext/--with-ext requires %q or a manager name list", extSelectAll)
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

// configuredExtPackageMode reads [global] ext_package_mode, defaulting to off.
func (s *cliService) configuredExtPackageMode() (string, error) {
	cfg, err := s.loadConfigFile()
	if err != nil {
		return "", err
	}
	if cfg == nil || cfg.Global.ExtPackageMode == nil {
		return app.ManagersModeOff, nil
	}
	mode := strings.ToLower(strings.TrimSpace(*cfg.Global.ExtPackageMode))
	switch mode {
	case "", app.ManagersModeOff:
		return app.ManagersModeOff, nil
	case extPackageModeOn:
		return extPackageModeOn, nil
	default:
		return "", fmt.Errorf("invalid global.ext_package_mode %q, want %q or %q", mode, app.ManagersModeOff, extPackageModeOn)
	}
}

func (s *cliService) loadConfigFile() (*cfgpkg.File, error) {
	if s != nil && s.cfgService.Load != nil {
		return s.cfgService.Load()
	}
	return cfgpkg.Load()
}

// applyExtSelection stores the selection on the list and update services.
// Both are value copies on the cliService, so this only affects the current
// invocation; the returned function restores the previous values.
func (s *cliService) applyExtSelection(selection app.ManagersSelection) func() {
	prevList := s.listService.Managers
	prevUpdate := s.updService.Managers
	s.listService.Managers = selection
	s.updService.Managers = selection
	return func() {
		s.listService.Managers = prevList
		s.updService.Managers = prevUpdate
	}
}

// applyListExtSelection also wires the failure printer: the plain list
// path has no failure channel of its own.
func (s *cliService) applyListExtSelection(selection app.ManagersSelection) func() {
	restore := s.applyExtSelection(selection)
	prevFailure := s.listService.OnExternalFailure
	s.listService.OnExternalFailure = func(failure extpkg.Failure) {
		name := failure.Manager
		if name == "" {
			name = "ext"
		}
		ccolor.Fprintf(s.stderrWriter(), "<yellow>check_failed</> %s: %v\n", name, failure.Err)
	}
	return func() {
		s.listService.OnExternalFailure = prevFailure
		restore()
	}
}

// listExtSelection resolves the list flags and rejects combinations that
// would silently drop half of what the user asked for.
func (s *cliService) listExtSelection(opts *ListOptions) (app.ManagersSelection, error) {
	if opts == nil {
		return app.ManagersSelection{}, nil
	}
	selection, err := s.resolveExtSelection(opts.Ext, opts.WithExt)
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
				return app.ManagersSelection{}, fmt.Errorf("--ext cannot be used with %s", other.name)
			}
		}
	case app.ManagersModeWith:
		if opts.GUI {
			return app.ManagersSelection{}, fmt.Errorf("--with-ext cannot be used with --gui")
		}
		if opts.NoInstalled {
			return app.ManagersSelection{}, fmt.Errorf("--with-ext cannot be used with --no-installed")
		}
	}
	return selection, nil
}

// updateExtSelection resolves the update flags. --ext already names
// the whole selection, so it implies --all for that selection.
func (s *cliService) updateExtSelection(opts *UpdateOptions) (app.ManagersSelection, error) {
	if opts == nil {
		return app.ManagersSelection{}, nil
	}
	if opts.Self {
		if opts.Ext != "" || opts.WithExt != "" {
			return app.ManagersSelection{}, fmt.Errorf("update --self cannot be used with --ext/--with-ext")
		}
		return app.ManagersSelection{}, nil
	}
	selection, err := s.resolveExtSelection(opts.Ext, opts.WithExt)
	if err != nil {
		return app.ManagersSelection{}, err
	}
	if selection.Mode == app.ManagersModeWith && !opts.Check && !opts.All && !opts.Interactive && len(opts.Targets) == 0 {
		return app.ManagersSelection{}, fmt.Errorf("--with-ext requires --check, --all, --interactive or a target")
	}
	return selection, nil
}
