package sdk

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gookit/goutil/x/assert"
)

func TestStoreLoadInitializesMissingFile(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "sdk.installed.json")}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load store: %v", err)
	}

	assert.Eq(t, 1, loaded.Schema)
	assert.Eq(t, 0, len(loaded.Installed))
}

func TestDefaultStorePathUsesHomeConfigDir(t *testing.T) {
	home := t.TempDir()
	customConfig := filepath.Join(t.TempDir(), "custom", "eget.toml")
	xdgConfig := filepath.Join(t.TempDir(), "xdg")
	t.Setenv("HOME", home)
	t.Setenv("EGET_CONFIG", customConfig)
	t.Setenv("EGET_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", xdgConfig)

	path, err := DefaultStorePath()
	assert.NoErr(t, err)
	assert.Eq(t, filepath.Join(xdgConfig, "eget", "sdk.installed.json"), path)
}

func TestDefaultStorePathUsesEgetConfigDir(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("EGET_CONFIG", filepath.Join(t.TempDir(), "explicit.toml"))
	t.Setenv("EGET_CONFIG_DIR", configDir)

	path, err := DefaultStorePath()
	assert.NoErr(t, err)
	assert.Eq(t, filepath.Join(configDir, "sdk.installed.json"), path)
}

func TestStoreRecordListAndRemove(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "sdk.installed.json")}
	now := time.Date(2026, 5, 17, 13, 0, 0, 0, time.UTC)

	first := InstalledEntry{Name: "go", Version: "1.21.1", Path: "/sdks/go1.21.1", URL: "https://example.com/go1.21.1.tar.gz", Filename: "go1.21.1.tar.gz", OS: "linux", Arch: "amd64", Ext: "tar.gz", InstalledAt: now, StripComponents: 1}
	second := InstalledEntry{Name: "go", Version: "1.22.0", Path: "/sdks/go1.22.0", URL: "https://example.com/go1.22.0.tar.gz", Filename: "go1.22.0.tar.gz", OS: "linux", Arch: "amd64", Ext: "tar.gz", InstalledAt: now.Add(time.Hour), StripComponents: 1}
	node := InstalledEntry{Name: "node", Version: "20.11.1", Path: "/sdks/node20.11.1", InstalledAt: now}

	if err := store.Record(second); err != nil {
		t.Fatalf("record second: %v", err)
	}
	if err := store.Record(first); err != nil {
		t.Fatalf("record first: %v", err)
	}
	if err := store.Record(node); err != nil {
		t.Fatalf("record node: %v", err)
	}

	all, err := store.List("")
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	assert.Eq(t, []string{"go", "go", "node"}, []string{all[0].Name, all[1].Name, all[2].Name})
	assert.Eq(t, []string{"1.21.1", "1.22.0", "20.11.1"}, []string{all[0].Version, all[1].Version, all[2].Version})

	goItems, err := store.List("go")
	if err != nil {
		t.Fatalf("list go: %v", err)
	}
	assert.Eq(t, 2, len(goItems))

	removed, err := store.Remove("go", "1.21.1")
	if err != nil {
		t.Fatalf("remove go: %v", err)
	}
	assert.Eq(t, first.Path, removed.Path)

	goItems, err = store.List("go")
	if err != nil {
		t.Fatalf("list go after remove: %v", err)
	}
	assert.Eq(t, 1, len(goItems))
	assert.Eq(t, "1.22.0", goItems[0].Version)
}

func TestStoreRemoveMissingVersionReturnsError(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "sdk.installed.json")}

	_, err := store.Remove("go", "1.21.1")
	assert.Err(t, err)
}

func TestStoreLoadRejectsInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sdk.installed.json")
	if err := os.WriteFile(path, []byte("{bad"), 0o644); err != nil {
		t.Fatalf("write bad json: %v", err)
	}

	_, err := (Store{Path: path}).Load()
	assert.Err(t, err)
}
