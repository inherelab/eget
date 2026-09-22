package extpkg

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gookit/goutil/x/assert"
	cfgpkg "github.com/inherelab/eget/internal/config"
)

func managerNames(managers []Manager) []string {
	names := make([]string, 0, len(managers))
	for _, manager := range managers {
		names = append(names, manager.Name)
	}
	return names
}

func findManager(t *testing.T, managers []Manager, name string) Manager {
	t.Helper()
	for _, manager := range managers {
		if manager.Name == name {
			return manager
		}
	}
	t.Fatalf("manager %q not found in %v", name, managerNames(managers))
	return Manager{}
}

func TestManagersReturnsBuiltins(t *testing.T) {
	managers := Managers(cfgpkg.NewFile())

	assert.Eq(t, []string{"bun", "cargo", "npm", "pipx", "pnpm", "scoop", "uv"}, managerNames(managers))

	npm := findManager(t, managers, "npm")
	assert.Eq(t, []string{"ls", "-g", "--depth=0", "--json"}, npm.ListArgs)
	assert.Eq(t, []string{"outdated", "-g", "--json"}, npm.OutdatedArgs)
	assert.Eq(t, DefaultTimeout, npm.Timeout)
	assert.True(t, npm.SupportsOutdated())

	// cargo has no built-in outdated command.
	cargo := findManager(t, managers, "cargo")
	assert.False(t, cargo.SupportsOutdated())
	assert.Eq(t, []string{"install", "--list"}, cargo.ListArgs)

	// deno / yarn / go are deliberately not built in.
	for _, name := range []string{"deno", "yarn", "go"} {
		for _, manager := range managers {
			if manager.Name == name {
				t.Fatalf("%s must not be a builtin manager", name)
			}
		}
	}
}

func TestManagersOverridesBuiltinSection(t *testing.T) {
	bin := "npm-custom"
	parser := "lines-regex"
	regex := `^(?P<name>\S+)\s+(?P<version>\S+)$`
	timeout := 5
	cfg := cfgpkg.NewFile()
	cfg.Ext["npm"] = cfgpkg.ManagerSection{
		Bin:       &bin,
		Parser:    &parser,
		ListRegex: &regex,
		Timeout:   &timeout,
		ListArgs:  []string{"ls", "--json"},
	}

	npm := findManager(t, Managers(cfg), "npm")
	assert.Eq(t, "npm-custom", npm.Bin)
	assert.Eq(t, "lines-regex", npm.Parser)
	assert.Eq(t, regex, npm.ListRegex)
	assert.Eq(t, time.Duration(5)*time.Second, npm.Timeout)
	assert.Eq(t, []string{"ls", "--json"}, npm.ListArgs)
	// Unset fields keep the builtin value.
	assert.Eq(t, []string{"outdated", "-g", "--json"}, npm.OutdatedArgs)
}

func TestManagersDisabledSectionRemovesManager(t *testing.T) {
	disabled := false
	cfg := cfgpkg.NewFile()
	cfg.Ext["pnpm"] = cfgpkg.ManagerSection{Enabled: &disabled}

	names := managerNames(Managers(cfg))
	for _, name := range names {
		if name == "pnpm" {
			t.Fatalf("pnpm must be removed, got %v", names)
		}
	}
	assert.Eq(t, 6, len(names))
}

func TestManagersAddsConfiguredManager(t *testing.T) {
	bin := "winget"
	parser := "lines-regex"
	regex := `^(?P<name>\S+)\s+(?P<version>\S+)$`
	cfg := cfgpkg.NewFile()
	cfg.Ext["winget"] = cfgpkg.ManagerSection{
		Bin:       &bin,
		Parser:    &parser,
		ListArgs:  []string{"list"},
		ListRegex: &regex,
	}

	managers := Managers(cfg)
	assert.Eq(t, 8, len(managers))
	winget := findManager(t, managers, "winget")
	assert.Eq(t, "winget", winget.Bin)
	assert.Eq(t, []string{"list"}, winget.ListArgs)
	assert.Eq(t, DefaultTimeout, winget.Timeout)
	assert.False(t, winget.SupportsOutdated())
}

// An explicit empty list must clear the builtin command, which is how a user
// turns off outdated detection for a manager.
func TestManagersEmptyArgsDisableCommand(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "eget.toml")
	body := "[ext.npm]\noutdated_args = []\n"
	if err := os.WriteFile(configPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := cfgpkg.LoadFile(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	npm := findManager(t, Managers(cfg), "npm")
	if npm.SupportsOutdated() {
		t.Fatalf("expected empty outdated_args to disable detection, got %v", npm.OutdatedArgs)
	}
}
