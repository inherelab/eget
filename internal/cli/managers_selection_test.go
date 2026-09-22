package cli

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/gookit/goutil/x/assert"
	"github.com/gookit/goutil/x/ccolor"
	"github.com/inherelab/eget/internal/app"
	cfgpkg "github.com/inherelab/eget/internal/config"
	"github.com/inherelab/eget/internal/extpkg"
	storepkg "github.com/inherelab/eget/internal/installed"
	"github.com/inherelab/eget/internal/util"
)

// fakeExtService is a minimal app.ExternalProvider for CLI tests.
type fakeExtService struct {
	names    []string
	packages []extpkg.Package
	outdated []extpkg.Package
	upgraded []string
}

func newFakeExtService() *fakeExtService {
	return &fakeExtService{names: []string{"bun", "npm", "uv"}}
}

func (f *fakeExtService) List(_ context.Context, only ...string) ([]extpkg.Package, []extpkg.Failure, error) {
	return f.packages, nil, nil
}

func (f *fakeExtService) Outdated(_ context.Context, only ...string) ([]extpkg.Package, []extpkg.Failure, error) {
	return f.outdated, nil, nil
}

func (f *fakeExtService) Upgrade(_ context.Context, managerName string, names []string) (extpkg.UpgradeResult, error) {
	f.upgraded = append(f.upgraded, managerName)
	return extpkg.UpgradeResult{Manager: managerName, Names: names}, nil
}

func (f *fakeExtService) Manager(name string) (extpkg.Manager, bool) {
	for _, candidate := range f.names {
		if candidate == name {
			return extpkg.Manager{Name: name, Bin: name, OutdatedArgs: []string{"outdated"}, UpgradeArgs: []string{"upgrade"}}, true
		}
	}
	return extpkg.Manager{}, false
}

func (f *fakeExtService) Names() []string { return f.names }

func selectionService(mode string) *cliService {
	svc := &cliService{
		extService: newFakeExtService(),
		cfgService: app.ConfigService{
			Load: func() (*cfgpkg.File, error) {
				cfg := cfgpkg.NewFile()
				if mode != "" {
					cfg.Global.ManagersMode = util.StringPtr(mode)
				}
				return cfg, nil
			},
		},
	}
	return svc
}

func TestResolveManagersSelectionDefaultOff(t *testing.T) {
	for _, mode := range []string{"", "off", "OFF"} {
		t.Run("mode "+mode, func(t *testing.T) {
			selection, err := selectionService(mode).resolveManagersSelection("", "")
			assert.NoErr(t, err)
			assert.False(t, selection.Enabled())
			assert.Eq(t, app.ManagersModeOff, selection.Mode)
		})
	}
}

func TestResolveManagersSelectionFromGlobalMode(t *testing.T) {
	selection, err := selectionService("on").resolveManagersSelection("", "")
	assert.NoErr(t, err)
	assert.True(t, selection.Enabled())
	assert.Eq(t, app.ManagersModeWith, selection.Mode)
	assert.Eq(t, 0, len(selection.Managers), "on means every configured manager")
}

func TestResolveManagersSelectionRejectsUnknownMode(t *testing.T) {
	_, err := selectionService("with").resolveManagersSelection("", "")
	assert.Err(t, err)
	assert.Contains(t, err.Error(), "managers_mode")
}

func TestResolveManagersSelectionFlags(t *testing.T) {
	svc := selectionService("on")

	t.Run("managers all is only-mode", func(t *testing.T) {
		selection, err := svc.resolveManagersSelection("all", "")
		assert.NoErr(t, err)
		assert.Eq(t, app.ManagersModeOnly, selection.Mode)
		assert.Eq(t, 0, len(selection.Managers))
	})

	t.Run("named managers are validated and sorted", func(t *testing.T) {
		selection, err := svc.resolveManagersSelection("uv, npm", "")
		assert.NoErr(t, err)
		assert.Eq(t, app.ManagersModeOnly, selection.Mode)
		assert.Eq(t, []string{"npm", "uv"}, selection.Managers)
	})

	t.Run("with-managers", func(t *testing.T) {
		selection, err := svc.resolveManagersSelection("", "bun")
		assert.NoErr(t, err)
		assert.Eq(t, app.ManagersModeWith, selection.Mode)
		assert.Eq(t, []string{"bun"}, selection.Managers)
	})

	t.Run("flags override the configured mode", func(t *testing.T) {
		selection, err := selectionService("off").resolveManagersSelection("", "npm")
		assert.NoErr(t, err)
		assert.True(t, selection.Enabled())
	})

	t.Run("both flags are rejected", func(t *testing.T) {
		_, err := svc.resolveManagersSelection("npm", "bun")
		assert.Err(t, err)
		assert.Contains(t, err.Error(), "cannot be used together")
	})

	t.Run("unknown manager lists the available ones", func(t *testing.T) {
		_, err := svc.resolveManagersSelection("deno", "")
		assert.Err(t, err)
		assert.Contains(t, err.Error(), "unknown manager")
		assert.Contains(t, err.Error(), "npm")
	})
}

func TestListManagersSelectionCombos(t *testing.T) {
	svc := selectionService("off")

	t.Run("managers only rejects eget view filters", func(t *testing.T) {
		for _, opts := range []*ListOptions{
			{Managers: "all", All: true},
			{Managers: "all", GUI: true},
			{Managers: "all", NoInstalled: true},
		} {
			if _, err := svc.listManagersSelection(opts); err == nil {
				t.Fatalf("expected combination error for %+v", opts)
			}
		}
	})

	t.Run("with-managers rejects installed-only views", func(t *testing.T) {
		if _, err := svc.listManagersSelection(&ListOptions{WithManagers: "all", GUI: true}); err == nil {
			t.Fatal("expected --with-managers with --gui to be rejected")
		}
		if _, err := svc.listManagersSelection(&ListOptions{WithManagers: "all", NoInstalled: true}); err == nil {
			t.Fatal("expected --with-managers with --no-installed to be rejected")
		}
	})

	t.Run("with-managers works with all and outdated", func(t *testing.T) {
		selection, err := svc.listManagersSelection(&ListOptions{WithManagers: "all", All: true, Outdated: true})
		assert.NoErr(t, err)
		assert.Eq(t, app.ManagersModeWith, selection.Mode)
	})
}

func TestUpdateManagersSelectionCombos(t *testing.T) {
	svc := selectionService("off")

	t.Run("with-managers needs check all interactive or a target", func(t *testing.T) {
		_, err := svc.updateManagersSelection(&UpdateOptions{WithManagers: "npm"})
		assert.Err(t, err)
		assert.Contains(t, err.Error(), "requires --check")

		if _, err := svc.updateManagersSelection(&UpdateOptions{WithManagers: "npm", All: true}); err != nil {
			t.Fatalf("expected --with-managers with --all to pass, got %v", err)
		}
		// --check is read-only, so it needs no further selection.
		if _, err := svc.updateManagersSelection(&UpdateOptions{WithManagers: "npm", Check: true}); err != nil {
			t.Fatalf("expected --with-managers with --check to pass, got %v", err)
		}
		if _, err := svc.updateManagersSelection(&UpdateOptions{WithManagers: "npm", Targets: []string{"typescript"}}); err != nil {
			t.Fatalf("expected --with-managers with a target to pass, got %v", err)
		}
	})

	t.Run("managers only needs nothing else", func(t *testing.T) {
		selection, err := svc.updateManagersSelection(&UpdateOptions{Managers: "npm"})
		assert.NoErr(t, err)
		assert.Eq(t, app.ManagersModeOnly, selection.Mode)
	})

	t.Run("self update rejects manager flags", func(t *testing.T) {
		_, err := svc.updateManagersSelection(&UpdateOptions{Self: true, Managers: "npm"})
		assert.Err(t, err)
		assert.Contains(t, err.Error(), "cannot be used with")
	})
}

func TestUpdateManagersSelectionFoldsManagerNameTargets(t *testing.T) {
	svc := selectionService("")

	t.Run("bare manager names join the selector", func(t *testing.T) {
		opts := &UpdateOptions{Managers: "npm", Targets: []string{"uv", "npm"}}
		selection, err := svc.updateManagersSelection(opts)
		assert.NoErr(t, err)
		assert.Eq(t, app.ManagersModeOnly, selection.Mode)
		assert.Eq(t, []string{"npm", "uv"}, selection.Managers)
		assert.Eq(t, "npm,uv", opts.Managers)
		assert.Eq(t, 0, len(opts.Targets))
	})

	t.Run("all drops bare manager names", func(t *testing.T) {
		opts := &UpdateOptions{Managers: "all", Targets: []string{"uv"}}
		selection, err := svc.updateManagersSelection(opts)
		assert.NoErr(t, err)
		assert.Eq(t, 0, len(selection.Managers))
		assert.Eq(t, "all", opts.Managers)
		assert.Eq(t, 0, len(opts.Targets))
	})

	t.Run("explicit external references stay targets", func(t *testing.T) {
		opts := &UpdateOptions{Managers: "npm", Targets: []string{"npm:typescript"}}
		selection, err := svc.updateManagersSelection(opts)
		assert.NoErr(t, err)
		assert.Eq(t, []string{"npm"}, selection.Managers)
		assert.Eq(t, []string{"npm:typescript"}, opts.Targets)
	})

	t.Run("eget targets are rejected", func(t *testing.T) {
		_, err := svc.updateManagersSelection(&UpdateOptions{Managers: "npm", Targets: []string{"fd"}})
		assert.Err(t, err)
		assert.Contains(t, err.Error(), "--managers selects manager packages only")
	})
}

func TestHandleListWithManagersShowsManagerSource(t *testing.T) {
	ext := newFakeExtService()
	ext.packages = []extpkg.Package{{Manager: "npm", Name: "typescript", Version: "5.8.0"}}

	svc := &cliService{
		extService: ext,
		listService: app.ListService{
			External: ext,
			LoadConfig: func() (*cfgpkg.File, error) {
				cfg := cfgpkg.NewFile()
				cfg.Packages["fzf"] = cfgpkg.Section{Repo: util.StringPtr("junegunn/fzf")}
				return cfg, nil
			},
			LoadInstalled: func() (*storepkg.Config, error) {
				return &storepkg.Config{Installed: map[string]storepkg.Entry{
					"junegunn/fzf": {Repo: "junegunn/fzf", Tag: "v0.50.0"},
				}}, nil
			},
		},
		cfgService: app.ConfigService{Load: func() (*cfgpkg.File, error) { return cfgpkg.NewFile(), nil }},
	}

	var out bytes.Buffer
	ccolor.SetOutput(&out)
	defer ccolor.SetOutput(os.Stdout)

	assert.NoErr(t, svc.handleList(&ListOptions{WithManagers: "npm"}))
	got := out.String()
	assert.Contains(t, got, "typescript")
	assert.Contains(t, got, "npm:typescript")
	// The Source column shows the manager name.
	assert.Contains(t, got, "npm")
	assert.Contains(t, got, "fzf")
}

func TestHandleListOffKeepsExternalOut(t *testing.T) {
	ext := newFakeExtService()
	ext.packages = []extpkg.Package{{Manager: "npm", Name: "typescript", Version: "5.8.0"}}

	svc := &cliService{
		extService: ext,
		listService: app.ListService{
			External: ext,
			LoadConfig: func() (*cfgpkg.File, error) {
				return cfgpkg.NewFile(), nil
			},
			LoadInstalled: func() (*storepkg.Config, error) {
				return &storepkg.Config{Installed: map[string]storepkg.Entry{
					"junegunn/fzf": {Repo: "junegunn/fzf", Tag: "v0.50.0"},
				}}, nil
			},
		},
		cfgService: app.ConfigService{Load: func() (*cfgpkg.File, error) { return cfgpkg.NewFile(), nil }},
	}

	var out bytes.Buffer
	ccolor.SetOutput(&out)
	defer ccolor.SetOutput(os.Stdout)

	assert.NoErr(t, svc.handleList(&ListOptions{}))
	got := out.String()
	if strings.Contains(got, "typescript") {
		t.Fatalf("external packages must not appear by default, got %q", got)
	}
	assert.Contains(t, got, "fzf")
}

func TestHandleManagersUpgradeRejectsUnknownManager(t *testing.T) {
	svc := &cliService{extService: newFakeExtService()}

	err := svc.handleManagersUpgrade(&ManagersOptions{Targets: []string{"deno"}})
	assert.Err(t, err)
	assert.Contains(t, err.Error(), "unknown manager")
}

func TestHandleManagersUpgradeRequiresManagerName(t *testing.T) {
	svc := &cliService{extService: newFakeExtService()}

	err := svc.handleManagersUpgrade(&ManagersOptions{})
	assert.Err(t, err)
	assert.Contains(t, err.Error(), "requires a manager name")
}

func TestHandleManagersUpgradeCallsService(t *testing.T) {
	ext := newFakeExtService()
	svc := &cliService{extService: ext, stderr: os.Stderr}

	var out bytes.Buffer
	ccolor.SetOutput(&out)
	defer ccolor.SetOutput(os.Stdout)

	assert.NoErr(t, svc.handleManagersUpgrade(&ManagersOptions{Targets: []string{"uv"}}))
	assert.Eq(t, []string{"uv"}, ext.upgraded)
	assert.Contains(t, out.String(), "uv")
}
