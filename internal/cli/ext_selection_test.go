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
					cfg.Global.ExtPackageMode = util.StringPtr(mode)
				}
				return cfg, nil
			},
		},
	}
	return svc
}

func TestResolveExtSelectionDefaultOff(t *testing.T) {
	for _, mode := range []string{"", "off", "OFF"} {
		t.Run("mode "+mode, func(t *testing.T) {
			selection, err := selectionService(mode).resolveExtSelection("", "")
			assert.NoErr(t, err)
			assert.False(t, selection.Enabled())
			assert.Eq(t, app.ManagersModeOff, selection.Mode)
		})
	}
}

func TestResolveExtSelectionFromGlobalMode(t *testing.T) {
	selection, err := selectionService("on").resolveExtSelection("", "")
	assert.NoErr(t, err)
	assert.True(t, selection.Enabled())
	assert.Eq(t, app.ManagersModeWith, selection.Mode)
	assert.Eq(t, 0, len(selection.Managers), "on means every configured manager")
}

func TestResolveExtSelectionRejectsUnknownMode(t *testing.T) {
	_, err := selectionService("with").resolveExtSelection("", "")
	assert.Err(t, err)
	assert.Contains(t, err.Error(), "ext_package_mode")
}

func TestResolveExtSelectionFlags(t *testing.T) {
	svc := selectionService("on")

	t.Run("ext all is only-mode", func(t *testing.T) {
		selection, err := svc.resolveExtSelection("all", "")
		assert.NoErr(t, err)
		assert.Eq(t, app.ManagersModeOnly, selection.Mode)
		assert.Eq(t, 0, len(selection.Managers))
	})

	t.Run("named managers are validated and sorted", func(t *testing.T) {
		selection, err := svc.resolveExtSelection("uv, npm", "")
		assert.NoErr(t, err)
		assert.Eq(t, app.ManagersModeOnly, selection.Mode)
		assert.Eq(t, []string{"npm", "uv"}, selection.Managers)
	})

	t.Run("with-ext", func(t *testing.T) {
		selection, err := svc.resolveExtSelection("", "bun")
		assert.NoErr(t, err)
		assert.Eq(t, app.ManagersModeWith, selection.Mode)
		assert.Eq(t, []string{"bun"}, selection.Managers)
	})

	t.Run("flags override the configured mode", func(t *testing.T) {
		selection, err := selectionService("off").resolveExtSelection("", "npm")
		assert.NoErr(t, err)
		assert.True(t, selection.Enabled())
	})

	t.Run("both flags are rejected", func(t *testing.T) {
		_, err := svc.resolveExtSelection("npm", "bun")
		assert.Err(t, err)
		assert.Contains(t, err.Error(), "cannot be used together")
	})

	t.Run("unknown manager lists the available ones", func(t *testing.T) {
		_, err := svc.resolveExtSelection("deno", "")
		assert.Err(t, err)
		assert.Contains(t, err.Error(), "unknown manager")
		assert.Contains(t, err.Error(), "npm")
	})
}

func TestListExtSelectionCombos(t *testing.T) {
	svc := selectionService("off")

	t.Run("ext only rejects eget view filters", func(t *testing.T) {
		for _, opts := range []*ListOptions{
			{Ext: "all", All: true},
			{Ext: "all", GUI: true},
			{Ext: "all", NoInstalled: true},
		} {
			if _, err := svc.listExtSelection(opts); err == nil {
				t.Fatalf("expected combination error for %+v", opts)
			}
		}
	})

	t.Run("with-ext rejects installed-only views", func(t *testing.T) {
		if _, err := svc.listExtSelection(&ListOptions{WithExt: "all", GUI: true}); err == nil {
			t.Fatal("expected --with-ext with --gui to be rejected")
		}
		if _, err := svc.listExtSelection(&ListOptions{WithExt: "all", NoInstalled: true}); err == nil {
			t.Fatal("expected --with-ext with --no-installed to be rejected")
		}
	})

	t.Run("with-ext works with all and outdated", func(t *testing.T) {
		selection, err := svc.listExtSelection(&ListOptions{WithExt: "all", All: true, Outdated: true})
		assert.NoErr(t, err)
		assert.Eq(t, app.ManagersModeWith, selection.Mode)
	})
}

func TestUpdateExtSelectionCombos(t *testing.T) {
	svc := selectionService("off")

	t.Run("with-ext needs check all interactive or a target", func(t *testing.T) {
		_, err := svc.updateExtSelection(&UpdateOptions{WithExt: "npm"})
		assert.Err(t, err)
		assert.Contains(t, err.Error(), "requires --check")

		if _, err := svc.updateExtSelection(&UpdateOptions{WithExt: "npm", All: true}); err != nil {
			t.Fatalf("expected --with-ext with --all to pass, got %v", err)
		}
		// --check is read-only, so it needs no further selection.
		if _, err := svc.updateExtSelection(&UpdateOptions{WithExt: "npm", Check: true}); err != nil {
			t.Fatalf("expected --with-ext with --check to pass, got %v", err)
		}
		if _, err := svc.updateExtSelection(&UpdateOptions{WithExt: "npm", Targets: []string{"typescript"}}); err != nil {
			t.Fatalf("expected --with-ext with a target to pass, got %v", err)
		}
	})

	t.Run("ext only needs nothing else", func(t *testing.T) {
		selection, err := svc.updateExtSelection(&UpdateOptions{Ext: "npm"})
		assert.NoErr(t, err)
		assert.Eq(t, app.ManagersModeOnly, selection.Mode)
	})

	t.Run("self update rejects manager flags", func(t *testing.T) {
		_, err := svc.updateExtSelection(&UpdateOptions{Self: true, Ext: "npm"})
		assert.Err(t, err)
		assert.Contains(t, err.Error(), "cannot be used with")
	})
}

func TestUpdateExtSelectionKeepsTargetsUntouched(t *testing.T) {
	svc := selectionService("")

	t.Run("targets stay targets beside --ext", func(t *testing.T) {
		// `--ext npm pnpm` updates the npm package named pnpm, like
		// "npm update -g pnpm". The name must not be swallowed as a manager.
		opts := &UpdateOptions{Ext: "npm", Targets: []string{"pnpm"}}
		selection, err := svc.updateExtSelection(opts)
		assert.NoErr(t, err)
		assert.Eq(t, app.ManagersModeOnly, selection.Mode)
		assert.Eq(t, []string{"npm"}, selection.Managers)
		assert.Eq(t, "npm", opts.Ext)
		assert.Eq(t, []string{"pnpm"}, opts.Targets)
	})

	t.Run("a target named like a manager stays a target", func(t *testing.T) {
		opts := &UpdateOptions{Ext: "npm", Targets: []string{"uv"}}
		_, err := svc.updateExtSelection(opts)
		assert.NoErr(t, err)
		assert.Eq(t, "npm", opts.Ext)
		assert.Eq(t, []string{"uv"}, opts.Targets)
	})
}

func TestHandleListWithExtShowsManagerSource(t *testing.T) {
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

	assert.NoErr(t, svc.handleList(&ListOptions{WithExt: "npm"}))
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

func TestHandleExtUpgradeRejectsUnknownManager(t *testing.T) {
	svc := &cliService{extService: newFakeExtService()}

	err := svc.handleExtUpgrade(&ExtOptions{Targets: []string{"deno"}})
	assert.Err(t, err)
	assert.Contains(t, err.Error(), "unknown manager")
}

func TestHandleExtUpgradeRequiresManagerName(t *testing.T) {
	svc := &cliService{extService: newFakeExtService()}

	err := svc.handleExtUpgrade(&ExtOptions{})
	assert.Err(t, err)
	assert.Contains(t, err.Error(), "requires a manager name")
}

func TestHandleExtUpgradeCallsService(t *testing.T) {
	ext := newFakeExtService()
	svc := &cliService{extService: ext, stderr: os.Stderr}

	var out bytes.Buffer
	ccolor.SetOutput(&out)
	defer ccolor.SetOutput(os.Stdout)

	assert.NoErr(t, svc.handleExtUpgrade(&ExtOptions{Targets: []string{"uv"}}))
	assert.Eq(t, []string{"uv"}, ext.upgraded)
	assert.Contains(t, out.String(), "uv")
}
