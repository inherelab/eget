package extpkg

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gookit/goutil/x/assert"
)

type fakeRunner struct {
	calls   []string
	outputs map[string]CommandResult
	errs    map[string]error
}

func newFakeRunner() *fakeRunner {
	return &fakeRunner{outputs: map[string]CommandResult{}, errs: map[string]error{}}
}

// key uses the base name so tests can set expectations by manager name while
// the service runs the resolved path.
func (f *fakeRunner) key(bin string, args []string) string {
	return filepath.Base(bin) + " " + strings.Join(args, " ")
}

func (f *fakeRunner) set(bin string, args []string, res CommandResult) {
	f.outputs[f.key(bin, args)] = res
}

func (f *fakeRunner) setErr(bin string, args []string, err error) {
	f.errs[f.key(bin, args)] = err
}

func (f *fakeRunner) Run(_ context.Context, bin string, args []string, _ time.Duration) (CommandResult, error) {
	key := f.key(bin, args)
	f.calls = append(f.calls, key)
	if err, ok := f.errs[key]; ok {
		return CommandResult{}, err
	}
	res, ok := f.outputs[key]
	if !ok {
		return CommandResult{}, fmt.Errorf("unexpected command: %s", key)
	}
	return res, nil
}

// allAvailable resolves every bin except the ones listed in missing.
func allAvailable(missing ...string) func(string) (string, error) {
	missingSet := make(map[string]bool, len(missing))
	for _, name := range missing {
		missingSet[name] = true
	}
	return func(bin string) (string, error) {
		if missingSet[bin] {
			return "", fmt.Errorf("exec: %q: executable file not found in %%PATH%%", bin)
		}
		return "/usr/bin/" + bin, nil
	}
}

func testService(managers []Manager, runner Runner) Service {
	return Service{Managers: managers, Runner: runner, LookPath: allAvailable()}
}

func TestServiceListSkipsUnavailableManagers(t *testing.T) {
	runner := newFakeRunner()
	runner.set("npm", []string{"ls"}, CommandResult{Stdout: []byte(`{"dependencies":{"a":{"version":"1.0"}}}`)})
	runner.set("uv", []string{"list"}, CommandResult{Stdout: []byte("b v2.0.0\n")})

	svc := Service{
		Managers: []Manager{
			{Name: "npm", Bin: "npm", ListArgs: []string{"ls"}, Parser: ParserNPMJSON},
			{Name: "uv", Bin: "uv", ListArgs: []string{"list"}, Parser: ParserUVToolText},
			{Name: "pipx", Bin: "pipx", ListArgs: []string{"list"}, Parser: ParserPipxJSON},
		},
		Runner:   runner,
		LookPath: allAvailable("pipx"),
	}

	packages, failures, err := svc.List(context.Background())
	assert.NoErr(t, err)
	assert.Eq(t, 0, len(failures))
	assert.Eq(t, 2, len(packages))
	assert.Eq(t, "a", packages[0].Name)
	assert.Eq(t, "npm", packages[0].Manager)
	assert.Eq(t, "b", packages[1].Name)
	assert.Eq(t, "uv", packages[1].Manager)
}

func TestServiceListReportsParseFailurePerManager(t *testing.T) {
	runner := newFakeRunner()
	// npm writes a non-JSON error to stdout and exits 1.
	runner.set("npm", []string{"ls"}, CommandResult{Stdout: []byte("Unknown command\n"), ExitCode: 1})
	runner.set("uv", []string{"list"}, CommandResult{Stdout: []byte("b v2.0.0\n")})

	svc := testService([]Manager{
		{Name: "npm", Bin: "npm", ListArgs: []string{"ls"}, Parser: ParserNPMJSON},
		{Name: "uv", Bin: "uv", ListArgs: []string{"list"}, Parser: ParserUVToolText},
	}, runner)

	packages, failures, err := svc.List(context.Background())
	assert.NoErr(t, err)
	assert.Eq(t, 1, len(packages))
	assert.Eq(t, "b", packages[0].Name)
	assert.Eq(t, 1, len(failures))
	assert.Eq(t, "npm", failures[0].Manager)
	assert.Contains(t, failures[0].Error(), "Unknown command")
}

func TestServiceListReportsRunError(t *testing.T) {
	runner := newFakeRunner()
	runner.setErr("npm", []string{"ls"}, errors.New("npm: command timed out after 1s"))

	svc := testService([]Manager{
		{Name: "npm", Bin: "npm", ListArgs: []string{"ls"}, Parser: ParserNPMJSON},
	}, runner)

	packages, failures, err := svc.List(context.Background())
	assert.NoErr(t, err)
	assert.Eq(t, 0, len(packages))
	assert.Eq(t, 1, len(failures))
	assert.Contains(t, failures[0].Error(), "timed out")
}

func TestServiceOutdatedSkipsManagersWithoutSupport(t *testing.T) {
	runner := newFakeRunner()
	runner.set("npm", []string{"outdated"}, CommandResult{Stdout: []byte(`{"a":{"current":"1.0","latest":"2.0"}}`), ExitCode: 1})

	svc := testService([]Manager{
		{Name: "npm", Bin: "npm", OutdatedArgs: []string{"outdated"}, Parser: ParserNPMJSON},
		// cargo has a list command but no outdated command at all.
		{Name: "cargo", Bin: "cargo", ListArgs: []string{"install", "--list"}, Parser: ParserCargoText},
	}, runner)

	packages, failures, err := svc.Outdated(context.Background())
	assert.NoErr(t, err)
	assert.Eq(t, 0, len(failures))
	assert.Eq(t, 1, len(packages))
	assert.Eq(t, "a", packages[0].Name)
	assert.Eq(t, "2.0", packages[0].Latest)
	assert.Eq(t, 1, len(runner.calls))
}

func TestServiceUpgradeWithNames(t *testing.T) {
	runner := newFakeRunner()
	runner.set("npm", []string{"update", "-g", "typescript"}, CommandResult{Stdout: []byte("changed 1 package\n")})

	svc := testService([]Manager{
		{Name: "npm", Bin: "npm", UpgradeArgs: []string{"update", "-g"}, Parser: ParserNPMJSON},
	}, runner)

	result, err := svc.Upgrade(context.Background(), "npm", []string{"typescript"})
	assert.NoErr(t, err)
	assert.Eq(t, "npm", result.Manager)
	assert.Eq(t, []string{"typescript"}, result.Names)
	assert.Contains(t, result.Output, "changed 1 package")
}

func TestServiceUpgradeWithoutNamesUsesUpgradeAllArgs(t *testing.T) {
	runner := newFakeRunner()
	runner.set("uv", []string{"tool", "upgrade", "--all"}, CommandResult{Stdout: []byte("ok\n")})

	svc := testService([]Manager{
		{Name: "uv", Bin: "uv", UpgradeArgs: []string{"tool", "upgrade"}, UpgradeAllArgs: []string{"tool", "upgrade", "--all"}, Parser: ParserUVToolText},
	}, runner)

	result, err := svc.Upgrade(context.Background(), "uv", nil)
	assert.NoErr(t, err)
	assert.Eq(t, "uv", result.Manager)
	assert.Eq(t, []string{"uv tool upgrade --all"}, runner.calls)

	_, err = svc.Upgrade(context.Background(), "deno", nil)
	assert.Err(t, err)
	assert.Contains(t, err.Error(), "unknown manager")
}

func TestServiceUpgradeFallsBackToInstalledNames(t *testing.T) {
	runner := newFakeRunner()
	runner.set("cargo", []string{"install", "--list"}, CommandResult{Stdout: []byte("cargotest v0.1.0 (C:\\src):\n    cargotest.exe\n")})
	runner.set("cargo", []string{"install", "cargotest"}, CommandResult{Stdout: []byte("Installed\n")})

	svc := testService([]Manager{
		{Name: "cargo", Bin: "cargo", ListArgs: []string{"install", "--list"}, UpgradeArgs: []string{"install"}, Parser: ParserCargoText},
	}, runner)

	result, err := svc.Upgrade(context.Background(), "cargo", nil)
	assert.NoErr(t, err)
	assert.Eq(t, []string{"cargotest"}, result.Names)
	assert.Eq(t, []string{"cargo install --list", "cargo install cargotest"}, runner.calls)
}

func TestServiceUpgradeReportsFailure(t *testing.T) {
	runner := newFakeRunner()
	runner.set("npm", []string{"update", "-g", "gone"}, CommandResult{Stdout: []byte("E404 not found\n"), ExitCode: 1})

	svc := testService([]Manager{
		{Name: "npm", Bin: "npm", UpgradeArgs: []string{"update", "-g"}, Parser: ParserNPMJSON},
	}, runner)

	_, err := svc.Upgrade(context.Background(), "npm", []string{"gone"})
	assert.Err(t, err)
	assert.Contains(t, err.Error(), "E404")
}

func TestServiceUpgradeUnavailableManager(t *testing.T) {
	svc := Service{
		Managers: []Manager{{Name: "pipx", Bin: "pipx", UpgradeArgs: []string{"upgrade"}}},
		Runner:   newFakeRunner(),
		LookPath: allAvailable("pipx"),
	}

	_, err := svc.Upgrade(context.Background(), "pipx", []string{"pycowsay"})
	assert.Err(t, err)
	assert.Contains(t, err.Error(), "not available")
}

func TestServiceResolve(t *testing.T) {
	svc := testService([]Manager{{Name: "npm", Bin: "npm"}}, newFakeRunner())

	t.Run("valid reference", func(t *testing.T) {
		manager, pkg, ok := svc.Resolve("npm:typescript")
		assert.True(t, ok)
		assert.Eq(t, "npm", manager.Name)
		assert.Eq(t, "typescript", pkg)
	})

	t.Run("unknown manager", func(t *testing.T) {
		if _, _, ok := svc.Resolve("deno:foo"); ok {
			t.Fatal("expected unknown manager to fail")
		}
	})

	t.Run("missing package", func(t *testing.T) {
		if _, _, ok := svc.Resolve("npm:"); ok {
			t.Fatal("expected empty package to fail")
		}
	})

	t.Run("no separator", func(t *testing.T) {
		if _, _, ok := svc.Resolve("typescript"); ok {
			t.Fatal("expected bare name to fail")
		}
	})
}

func TestServiceNamesAndManagerLookup(t *testing.T) {
	svc := testService([]Manager{
		{Name: "npm", Bin: "npm"},
		{Name: "bun", Bin: "bun"},
	}, newFakeRunner())

	assert.Eq(t, []string{"bun", "npm"}, svc.Names())
	if _, ok := svc.Manager("npm"); !ok {
		t.Fatal("expected npm manager")
	}
	if _, ok := svc.Manager("deno"); ok {
		t.Fatal("expected deno to be unknown")
	}
}
