package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gookit/goutil/x/assert"
)

const managersConfigTOML = `
[global]
managers_mode = "on"

[managers.npm]
bin = "npm"
list_args = ["ls", "-g", "--depth=0", "--json"]
outdated_args = ["outdated", "-g", "--json"]
upgrade_args = ["update", "-g"]
upgrade_all_args = ["update", "-g"]
parser = "npm-json"
enabled = true
timeout = 60

[managers.scoop]
bin = "scoop"
list_args = ["list"]
parser = "lines-regex"
list_regex = '^(?P<name>\S+)\s+(?P<version>\S+)'
enabled = false
`

func TestLoadFileReadsManagerSections(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "eget.toml")
	writeTestFile(t, configPath, managersConfigTOML)

	cfg, err := LoadFile(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	assert.Eq(t, "on", *cfg.Global.ManagersMode)
	assert.Eq(t, 2, len(cfg.Managers))

	npm := cfg.Managers["npm"]
	assert.Eq(t, "npm", *npm.Bin)
	assert.Eq(t, []string{"ls", "-g", "--depth=0", "--json"}, npm.ListArgs)
	assert.Eq(t, []string{"outdated", "-g", "--json"}, npm.OutdatedArgs)
	assert.Eq(t, []string{"update", "-g"}, npm.UpgradeArgs)
	assert.Eq(t, []string{"update", "-g"}, npm.UpgradeAllArgs)
	assert.Eq(t, "npm-json", *npm.Parser)
	assert.Eq(t, true, *npm.Enabled)
	assert.Eq(t, 60, *npm.Timeout)

	scoop := cfg.Managers["scoop"]
	assert.Eq(t, "lines-regex", *scoop.Parser)
	assert.Eq(t, `^(?P<name>\S+)\s+(?P<version>\S+)`, *scoop.ListRegex)
	assert.Eq(t, false, *scoop.Enabled)

	if _, ok := cfg.Repos["managers"]; ok {
		t.Fatal("managers must be a reserved root key, not a legacy repo section")
	}
}

func TestSaveRoundTripsManagerSections(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "eget.toml")
	writeTestFile(t, configPath, managersConfigTOML)

	cfg, err := LoadFile(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	savedPath := filepath.Join(tmp, "saved.toml")
	if err := Save(savedPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	reloaded, err := LoadFile(savedPath)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}

	assert.Eq(t, "on", *reloaded.Global.ManagersMode)
	assert.Eq(t, 2, len(reloaded.Managers))
	assert.Eq(t, []string{"ls", "-g", "--depth=0", "--json"}, reloaded.Managers["npm"].ListArgs)
	assert.Eq(t, "npm-json", *reloaded.Managers["npm"].Parser)
	assert.Eq(t, 60, *reloaded.Managers["npm"].Timeout)
	assert.Eq(t, false, *reloaded.Managers["scoop"].Enabled)
	assert.Eq(t, `^(?P<name>\S+)\s+(?P<version>\S+)`, *reloaded.Managers["scoop"].ListRegex)
}

func TestDumpConfigStringIncludesManagers(t *testing.T) {
	cfg := NewFile()
	mode := "off"
	cfg.Global.ManagersMode = &mode
	bin := "npm"
	parser := "npm-json"
	cfg.Managers["npm"] = ManagerSection{
		Bin:          &bin,
		Parser:       &parser,
		ListArgs:     []string{"ls", "-g", "--json"},
		UpgradeArgs:  []string{"update", "-g"},
		OutdatedArgs: []string{"outdated", "-g", "--json"},
	}

	text, err := dumpConfigString(cfg)
	if err != nil {
		t.Fatalf("dump config string: %v", err)
	}

	assert.Contains(t, text, `managers_mode = "off"`)
	assert.Contains(t, text, "[managers.npm]")
	assert.Contains(t, text, `bin = "npm"`)
	assert.Contains(t, text, `parser = "npm-json"`)
	assert.Contains(t, text, `list_args = ["ls", "-g", "--json"]`)
}

func TestSetByPathSupportsManagerFields(t *testing.T) {
	cfg := NewFile()
	bin := "npm"
	cfg.Managers["npm"] = ManagerSection{Bin: &bin}

	if err := SetByPath(cfg, "managers.npm.enabled", "true"); err != nil {
		t.Fatalf("set managers.npm.enabled: %v", err)
	}
	assert.Eq(t, true, *cfg.Managers["npm"].Enabled)

	if err := SetByPath(cfg, "managers.npm.list_args", "ls, -g, --json"); err != nil {
		t.Fatalf("set managers.npm.list_args: %v", err)
	}
	assert.Eq(t, []string{"ls", "-g", "--json"}, cfg.Managers["npm"].ListArgs)
}

func TestSavePreservesManagerSectionsAndRawValues(t *testing.T) {
	t.Setenv("MY_UA", "ua-resolved")
	t.Setenv("MANAGER_BIN", "npm-resolved")
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "eget.toml")
	writeTestFile(t, configPath, `
[global]
user_agent = "${MY_UA}"

[managers.npm]
bin = "${MANAGER_BIN}"
parser = "npm-json"
`)

	cfg, err := LoadFile(configPath)
	assert.NoErr(t, err)

	if err := Save(configPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	saved, err := os.ReadFile(configPath)
	assert.NoErr(t, err)
	text := string(saved)

	// global 的 env 占位符按原样保留
	assert.Contains(t, text, `user_agent = "${MY_UA}"`)
	// managers 子树与 packages/sdk 一样，整段由类型模型重新生成（写入解析后的值）
	assert.Contains(t, text, `bin = "npm-resolved"`)
	assert.Contains(t, text, `parser = "npm-json"`)

	reloaded, err := LoadFile(configPath)
	assert.NoErr(t, err)
	assert.Eq(t, "npm-json", *reloaded.Managers["npm"].Parser)
}
