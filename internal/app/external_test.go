package app

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/gookit/goutil/x/assert"
	cfgpkg "github.com/inherelab/eget/internal/config"
	"github.com/inherelab/eget/internal/extpkg"
	"github.com/inherelab/eget/internal/install"
	storepkg "github.com/inherelab/eget/internal/installed"
	"github.com/inherelab/eget/internal/util"
)

// fakeExternal stands in for extpkg.Service and records what was asked of it.
type fakeExternal struct {
	mu            sync.Mutex
	listCalls     int
	outdatedCalls int
	upgradeCalls  int
	lastOnly      []string
	packages      []extpkg.Package
	outdated      []extpkg.Package
	failures      []extpkg.Failure
	upgraded      []string
	active        int
	maxActive     int
	block         chan struct{}

	managers []extpkg.Manager
}

func newFakeExternal(managers ...extpkg.Manager) *fakeExternal {
	if len(managers) == 0 {
		managers = []extpkg.Manager{
			{Name: "npm", Bin: "npm", OutdatedArgs: []string{"outdated"}, UpgradeArgs: []string{"update"}, Enabled: true},
		}
	}
	return &fakeExternal{managers: managers}
}

func (f *fakeExternal) List(_ context.Context, only ...string) ([]extpkg.Package, []extpkg.Failure, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.listCalls++
	f.lastOnly = only
	return f.packages, f.failures, nil
}

func (f *fakeExternal) Outdated(_ context.Context, only ...string) ([]extpkg.Package, []extpkg.Failure, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.outdatedCalls++
	f.lastOnly = only
	return f.outdated, f.failures, nil
}

func (f *fakeExternal) Upgrade(_ context.Context, managerName string, names []string) (extpkg.UpgradeResult, error) {
	f.mu.Lock()
	f.upgradeCalls++
	f.upgraded = append(f.upgraded, managerName+":"+names[0])
	f.active++
	if f.active > f.maxActive {
		f.maxActive = f.active
	}
	block := f.block
	f.mu.Unlock()

	if block != nil {
		<-block
	}

	f.mu.Lock()
	f.active--
	f.mu.Unlock()
	return extpkg.UpgradeResult{Manager: managerName, Names: names}, nil
}

func (f *fakeExternal) Manager(name string) (extpkg.Manager, bool) {
	for _, manager := range f.managers {
		if manager.Name == name {
			return manager, true
		}
	}
	return extpkg.Manager{}, false
}

func (f *fakeExternal) Names() []string {
	names := make([]string, 0, len(f.managers))
	for _, manager := range f.managers {
		names = append(names, manager.Name)
	}
	return names
}

func repoListService(installed map[string]storepkg.Entry) ListService {
	return ListService{
		LoadConfig: func() (*cfgpkg.File, error) {
			cfg := cfgpkg.NewFile()
			cfg.Packages["rg"] = cfgpkg.Section{Repo: util.StringPtr("BurntSushi/ripgrep")}
			return cfg, nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{Installed: installed}, nil
		},
		LatestInfo: func(LatestCheckTarget) (LatestInfo, error) { return LatestInfo{}, nil },
	}
}

func TestListPackagesOffDoesNotTouchExternal(t *testing.T) {
	external := newFakeExternal()
	svc := repoListService(map[string]storepkg.Entry{
		"BurntSushi/ripgrep": {Repo: "BurntSushi/ripgrep", Tag: "v13.0.0"},
	})
	svc.External = external
	// Managers stays the zero value: off.

	items, err := svc.ListPackages()
	assert.NoErr(t, err)
	assert.Eq(t, 1, len(items))
	assert.Eq(t, 0, external.listCalls)
	assert.Eq(t, 0, external.outdatedCalls)
}

func TestListPackagesWithManagersAppendsExternal(t *testing.T) {
	external := newFakeExternal()
	external.packages = []extpkg.Package{
		{Manager: "npm", Name: "typescript", Version: "5.8.0"},
		{Manager: "uv", Name: "ruff", Version: "0.8.1"},
	}

	svc := repoListService(map[string]storepkg.Entry{
		"BurntSushi/ripgrep": {Repo: "BurntSushi/ripgrep", Tag: "v13.0.0"},
	})
	svc.External = external
	svc.Managers = ManagersSelection{Mode: ManagersModeWith, Managers: []string{"npm", "uv"}}

	items, err := svc.ListPackages()
	assert.NoErr(t, err)
	assert.Eq(t, 3, len(items))
	assert.Eq(t, 1, external.listCalls)
	assert.Eq(t, []string{"npm", "uv"}, external.lastOnly)

	byName := map[string]ListItem{}
	for _, item := range items {
		byName[item.Name] = item
	}

	ts := byName["typescript"]
	assert.Eq(t, "npm:typescript", ts.Repo)
	assert.Eq(t, "npm", ts.Manager)
	assert.Eq(t, "5.8.0", ts.Version)
	assert.Eq(t, "5.8.0", ts.InstalledTag)
	assert.True(t, ts.Installed)
	assert.True(t, ts.InstalledAt.IsZero())

	ruff := byName["ruff"]
	assert.Eq(t, "uv:ruff", ruff.Repo)
	assert.Eq(t, "uv", ruff.Manager)
}

func TestListPackagesOnlyManagersReturnsOnlyExternal(t *testing.T) {
	external := newFakeExternal()
	external.packages = []extpkg.Package{{Manager: "npm", Name: "typescript", Version: "5.8.0"}}

	svc := repoListService(map[string]storepkg.Entry{
		"BurntSushi/ripgrep": {Repo: "BurntSushi/ripgrep", Tag: "v13.0.0"},
	})
	svc.External = external
	svc.Managers = ManagersSelection{Mode: ManagersModeOnly}

	items, err := svc.ListPackages()
	assert.NoErr(t, err)
	assert.Eq(t, 1, len(items))
	assert.Eq(t, "typescript", items[0].Name)
	assert.Eq(t, "npm", items[0].Manager)
}

func TestListPackagesExternalHonoursIgnoreUpdatePackages(t *testing.T) {
	external := newFakeExternal()
	external.packages = []extpkg.Package{{Manager: "npm", Name: "typescript", Version: "5.8.0"}}

	svc := ListService{
		LoadConfig: func() (*cfgpkg.File, error) {
			cfg := cfgpkg.NewFile()
			cfg.Global.IgnoreUpdatePackages = []string{"typescript"}
			return cfg, nil
		},
		LoadInstalled: func() (*storepkg.Config, error) { return &storepkg.Config{}, nil },
		External:      external,
		Managers:      ManagersSelection{Mode: ManagersModeWith},
	}

	items, err := svc.ListPackages()
	assert.NoErr(t, err)
	assert.Eq(t, 1, len(items))
	assert.True(t, items[0].IgnoreUpdate)
}

func TestListOutdatedPackagesMergesExternal(t *testing.T) {
	publishedAt := time.Date(2026, 4, 21, 14, 10, 17, 0, time.UTC)
	external := newFakeExternal()
	external.outdated = []extpkg.Package{
		{Manager: "npm", Name: "typescript", Version: "5.8.0", Latest: "5.9.2"},
	}
	external.failures = []extpkg.Failure{
		{Manager: "cargo", Err: fmt.Errorf("cargo: boom")},
	}

	svc := ListService{
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
		LatestInfo: func(LatestCheckTarget) (LatestInfo, error) {
			return LatestInfo{Tag: "v0.60.0", PublishedAt: publishedAt}, nil
		},
		External: external,
		Managers: ManagersSelection{Mode: ManagersModeWith},
	}

	outdated, failures, checked, err := svc.ListOutdatedPackages()
	assert.NoErr(t, err)
	assert.Eq(t, 2, checked)
	assert.Eq(t, 2, len(outdated))

	byName := map[string]OutdatedItem{}
	for _, item := range outdated {
		byName[item.Name] = item
	}
	assert.Eq(t, "v0.50.0", byName["fzf"].InstalledTag)
	assert.Eq(t, "v0.60.0", byName["fzf"].LatestTag)
	assert.Eq(t, "", byName["fzf"].Manager)
	assert.Eq(t, "5.8.0", byName["typescript"].InstalledTag)
	assert.Eq(t, "5.9.2", byName["typescript"].LatestTag)
	assert.Eq(t, "npm:typescript", byName["typescript"].Repo)
	assert.Eq(t, "npm", byName["typescript"].Manager)

	assert.Eq(t, 1, len(failures))
	assert.Eq(t, "cargo", failures[0].Name)
	assert.Eq(t, "cargo", failures[0].Repo)
	assert.Contains(t, failures[0].Error.Error(), "boom")
}

func TestListOutdatedPackagesOnlySkipsRepoChecks(t *testing.T) {
	external := newFakeExternal()
	external.outdated = []extpkg.Package{
		{Manager: "npm", Name: "typescript", Version: "5.8.0", Latest: "5.9.2"},
	}

	latestCalled := 0
	svc := repoListService(map[string]storepkg.Entry{
		"BurntSushi/ripgrep": {Repo: "BurntSushi/ripgrep", Tag: "v13.0.0"},
	})
	svc.LatestInfo = func(LatestCheckTarget) (LatestInfo, error) {
		latestCalled++
		return LatestInfo{Tag: "v14.0.0"}, nil
	}
	svc.External = external
	svc.Managers = ManagersSelection{Mode: ManagersModeOnly}

	outdated, failures, checked, err := svc.ListOutdatedPackages()
	assert.NoErr(t, err)
	assert.Eq(t, 0, len(failures))
	assert.Eq(t, 1, len(outdated))
	assert.Eq(t, 1, checked)
	assert.Eq(t, 0, latestCalled)
	assert.Eq(t, "typescript", outdated[0].Name)
}

func TestUpdateCandidatesCallsOnUpdateDone(t *testing.T) {
	external := newFakeExternal()
	var done []string
	svc := UpdateService{
		LoadConfig: func() (*cfgpkg.File, error) { return cfgpkg.NewFile(), nil },
		External:   external,
		Managers:   ManagersSelection{Mode: ManagersModeWith},
		OnUpdateDone: func(item OutdatedItem, _ RunResult, err error) {
			assert.NoErr(t, err)
			done = append(done, item.Repo)
		},
	}

	_, err := svc.UpdateCandidates([]OutdatedItem{
		{Name: "typescript", Repo: "npm:typescript", Manager: "npm"},
	}, install.Options{})
	assert.NoErr(t, err)
	assert.Eq(t, []string{"npm:typescript"}, done)
	assert.Eq(t, []string{"npm:typescript"}, external.upgraded)
}

func TestListOutdatedPackagesOffDoesNotTouchExternal(t *testing.T) {
	external := newFakeExternal()
	svc := repoListService(map[string]storepkg.Entry{})
	svc.External = external

	_, _, _, err := svc.ListOutdatedPackages()
	assert.NoErr(t, err)
	assert.Eq(t, 0, external.outdatedCalls)
}

func TestUpdateCandidatesDispatchExternal(t *testing.T) {
	external := newFakeExternal()
	svc := UpdateService{
		Install:  &fakeInstallService{},
		External: external,
		Managers: ManagersSelection{Mode: ManagersModeWith},
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfgpkg.NewFile(), nil
		},
	}
	candidates := []OutdatedItem{
		{Name: "typescript", Repo: "npm:typescript", InstalledTag: "5.8.0", LatestTag: "5.9.2", Manager: "npm"},
	}

	results, err := svc.UpdateCandidates(candidates, install.Options{})
	assert.NoErr(t, err)
	assert.Eq(t, 1, len(results))
	assert.Eq(t, 1, external.upgradeCalls)
	assert.Eq(t, []string{"npm:typescript"}, external.upgraded)
	assert.Eq(t, "npm:typescript", results[0].Target)
}

func TestUpdateCandidatesExternalUpgradesStaySerial(t *testing.T) {
	external := newFakeExternal(
		extpkg.Manager{Name: "npm", Bin: "npm", OutdatedArgs: []string{"outdated"}, UpgradeArgs: []string{"update"}, Enabled: true},
		extpkg.Manager{Name: "uv", Bin: "uv", OutdatedArgs: []string{"outdated"}, UpgradeArgs: []string{"tool upgrade"}, Enabled: true},
	)
	external.block = make(chan struct{})
	svc := UpdateService{
		Install:  &fakeInstallService{},
		External: external,
		Managers: ManagersSelection{Mode: ManagersModeWith},
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfgpkg.NewFile(), nil
		},
	}
	// A high --batch must still be downgraded to serial: two managers would
	// otherwise write to their global stores at the same time.
	cli := install.Options{BatchConcurrency: 8, BatchConcurrencySet: true}
	candidates := []OutdatedItem{
		{Name: "typescript", Repo: "npm:typescript", Manager: "npm"},
		{Name: "ruff", Repo: "uv:ruff", Manager: "uv"},
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = svc.UpdateCandidates(candidates, cli)
	}()
	waitForMaxActive(t, func() int { return external.active }, 1)
	close(external.block)
	<-done

	assert.Eq(t, 2, external.upgradeCalls)
	assert.Eq(t, 1, external.maxActive)
}

func TestUpdatePackageStatusExternalExplicitRef(t *testing.T) {
	external := newFakeExternal()
	external.outdated = []extpkg.Package{
		{Manager: "npm", Name: "typescript", Version: "5.8.0", Latest: "5.9.2"},
	}
	svc := UpdateService{
		Install:       &fakeInstallService{},
		External:      external,
		Managers:      ManagersSelection{Mode: ManagersModeOff},
		LoadConfig:    func() (*cfgpkg.File, error) { return cfgpkg.NewFile(), nil },
		LoadInstalled: func() (*storepkg.Config, error) { return &storepkg.Config{}, nil },
	}

	// An explicit reference works even with the selection off.
	result, err := svc.UpdatePackageStatus("npm:typescript", install.Options{})
	assert.NoErr(t, err)
	assert.True(t, result.Updated)
	assert.Eq(t, "5.8.0", result.InstalledTag)
	assert.Eq(t, "5.9.2", result.LatestTag)
	assert.Eq(t, 1, external.upgradeCalls)
}

func TestUpdatePackageStatusExternalAlreadyUpToDate(t *testing.T) {
	external := newFakeExternal()
	external.packages = []extpkg.Package{{Manager: "npm", Name: "typescript", Version: "5.9.2"}}

	svc := UpdateService{
		Install:       &fakeInstallService{},
		External:      external,
		LoadConfig:    func() (*cfgpkg.File, error) { return cfgpkg.NewFile(), nil },
		LoadInstalled: func() (*storepkg.Config, error) { return &storepkg.Config{}, nil },
	}

	result, err := svc.UpdatePackageStatus("npm:typescript", install.Options{})
	assert.NoErr(t, err)
	assert.False(t, result.Updated)
	assert.Eq(t, "5.9.2", result.InstalledTag)
	assert.Eq(t, 0, external.upgradeCalls)
}

func TestUpdatePackageStatusExternalNotInstalled(t *testing.T) {
	external := newFakeExternal()
	svc := UpdateService{
		Install:       &fakeInstallService{},
		External:      external,
		LoadConfig:    func() (*cfgpkg.File, error) { return cfgpkg.NewFile(), nil },
		LoadInstalled: func() (*storepkg.Config, error) { return &storepkg.Config{}, nil },
	}

	_, err := svc.UpdatePackageStatus("npm:definitely-not-installed", install.Options{})
	assert.Err(t, err)
	assert.Contains(t, err.Error(), "not installed")
}

func TestUpdatePackageStatusBareNameNeedsSelection(t *testing.T) {
	external := newFakeExternal()
	external.packages = []extpkg.Package{{Manager: "npm", Name: "typescript", Version: "5.8.0"}}
	external.outdated = []extpkg.Package{{Manager: "npm", Name: "typescript", Version: "5.8.0", Latest: "5.9.2"}}

	newService := func(selection ManagersSelection) UpdateService {
		return UpdateService{
			Install:       &fakeInstallService{},
			External:      external,
			Managers:      selection,
			LoadConfig:    func() (*cfgpkg.File, error) { return cfgpkg.NewFile(), nil },
			LoadInstalled: func() (*storepkg.Config, error) { return &storepkg.Config{}, nil },
		}
	}

	// Off: a bare name is not an eget target, so it must not reach a manager.
	_, err := newService(ManagersSelection{Mode: ManagersModeOff}).UpdatePackageStatus("typescript", install.Options{})
	assert.Err(t, err)
	assert.Eq(t, 0, external.listCalls)

	// Selected: the bare name resolves to the external package.
	external.listCalls = 0
	result, err := newService(ManagersSelection{Mode: ManagersModeWith}).UpdatePackageStatus("typescript", install.Options{})
	assert.NoErr(t, err)
	assert.Eq(t, 1, external.listCalls)
	assert.Eq(t, "npm:typescript", result.Target)
}
