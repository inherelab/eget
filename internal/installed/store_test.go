package installed

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreLoadInitializesEmptyConfig(t *testing.T) {
	tmp := t.TempDir()
	store := NewStore(Options{
		HomeDir:   filepath.Join(tmp, "home"),
		GOOS:      "linux",
		LookupEnv: func(string) (string, bool) { return "", false },
	})

	cfg, err := store.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Installed == nil {
		t.Fatal("expected installed map to be initialized")
	}
	if len(cfg.Installed) != 0 {
		t.Fatalf("expected empty installed map, got %d entries", len(cfg.Installed))
	}
}

func TestStoreSaveAndRemoveEntry(t *testing.T) {
	tmp := t.TempDir()
	store := NewStore(Options{
		HomeDir:   filepath.Join(tmp, "home"),
		GOOS:      "linux",
		LookupEnv: func(string) (string, bool) { return "", false },
	})

	entry := Entry{
		Repo:        "junegunn/fzf",
		Target:      "junegunn/fzf",
		InstalledAt: time.Unix(1710000000, 0).UTC(),
		URL:         "https://github.com/junegunn/fzf/releases/download/v1.0.0/fzf.tar.gz",
		Asset:       "fzf.tar.gz",
	}

	cfg, err := store.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.Installed[entry.Repo] = entry

	if err := store.Save(cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	reloaded, err := store.Load()
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}

	saved, ok := reloaded.Installed[entry.Repo]
	if !ok {
		t.Fatalf("expected entry %q to be saved", entry.Repo)
	}
	if saved.Asset != entry.Asset || saved.URL != entry.URL {
		t.Fatalf("expected saved entry to match, got %#v", saved)
	}

	delete(reloaded.Installed, entry.Repo)
	if err := store.Save(reloaded); err != nil {
		t.Fatalf("save config after delete: %v", err)
	}

	afterDelete, err := store.Load()
	if err != nil {
		t.Fatalf("load after delete: %v", err)
	}
	if _, ok := afterDelete.Installed[entry.Repo]; ok {
		t.Fatalf("expected entry %q to be removed", entry.Repo)
	}
}

func TestStoreSaveRoundTripWithExtendedFields(t *testing.T) {
	tmp := t.TempDir()
	store := NewStore(Options{
		HomeDir:   filepath.Join(tmp, "home"),
		GOOS:      "linux",
		LookupEnv: func(string) (string, bool) { return "", false },
	})

	releasedAt := time.Unix(1710003600, 0).UTC()
	cfg := &Config{
		Installed: map[string]Entry{
			"gookit/gitw": {
				Repo:           "gookit/gitw",
				Target:         "gookit/gitw",
				InstalledAt:    time.Unix(1710000000, 0).UTC(),
				URL:            "https://github.com/gookit/gitw/releases/download/v0.3.6/chlog-windows-amd64.exe",
				Asset:          "chlog-windows-amd64.exe",
				Tool:           "chlog",
				ExtractedFiles: []string{"chlog.exe"},
				Options: map[string]interface{}{
					"tag":    "v0.3.6",
					"system": "windows/amd64",
				},
				Version:     "v0.3.6",
				Tag:         "v0.3.6",
				ReleaseDate: releasedAt,
			},
		},
	}

	if err := store.Save(cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	reloaded, err := store.Load()
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}

	entry, ok := reloaded.Installed["gookit/gitw"]
	if !ok {
		t.Fatal("expected saved entry to reload")
	}
	if entry.Tool != "chlog" || entry.Tag != "v0.3.6" || entry.Version != "v0.3.6" {
		t.Fatalf("expected extended fields to round-trip, got %#v", entry)
	}
	if !entry.ReleaseDate.Equal(releasedAt) {
		t.Fatalf("expected release date %v, got %v", releasedAt, entry.ReleaseDate)
	}
	if entry.Options["system"] != "windows/amd64" {
		t.Fatalf("expected options to round-trip, got %#v", entry.Options)
	}
}

func TestStoreRecordAndRemove(t *testing.T) {
	tmp := t.TempDir()
	store := NewStore(Options{
		HomeDir:   filepath.Join(tmp, "home"),
		GOOS:      "linux",
		LookupEnv: func(string) (string, bool) { return "", false },
	})

	recordedAt := time.Unix(1710000000, 0).UTC()
	err := store.Record("https://github.com/junegunn/fzf", Entry{
		Repo:           "ignored",
		Target:         "https://github.com/junegunn/fzf",
		InstalledAt:    recordedAt,
		URL:            "https://github.com/junegunn/fzf/releases/download/v1.0.0/fzf.tar.gz",
		Asset:          "fzf.tar.gz",
		ExtractedFiles: []string{"fzf"},
	})
	if err != nil {
		t.Fatalf("record entry: %v", err)
	}

	cfg, err := store.Load()
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}

	entry, ok := cfg.Installed["junegunn/fzf"]
	if !ok {
		t.Fatal("expected normalized repo entry to be recorded")
	}
	if entry.InstalledAt != recordedAt {
		t.Fatalf("expected recorded timestamp to be preserved, got %v", entry.InstalledAt)
	}

	if err := store.Remove("https://github.com/junegunn/fzf/"); err != nil {
		t.Fatalf("remove entry: %v", err)
	}

	cfg, err = store.Load()
	if err != nil {
		t.Fatalf("reload after remove: %v", err)
	}
	if _, ok := cfg.Installed["junegunn/fzf"]; ok {
		t.Fatal("expected normalized repo entry to be removed")
	}
}

func TestResolvePathPrefersLegacyDotfileWhenPresent(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	legacyPath := filepath.Join(homeDir, ".eget.installed.toml")
	writeInstalledFile(t, legacyPath, "[installed]\n")

	store := NewStore(Options{
		HomeDir:   homeDir,
		GOOS:      "linux",
		LookupEnv: func(string) (string, bool) { return "", false },
	})

	path := store.Path()
	if path != legacyPath {
		t.Fatalf("expected legacy path %q, got %q", legacyPath, path)
	}
}

func TestResolvePathFallsBackToXDGLocation(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	xdgHome := filepath.Join(tmp, "xdg")
	wantPath := filepath.Join(xdgHome, "eget", "installed.toml")
	writeInstalledFile(t, wantPath, "[installed]\n")

	store := NewStore(Options{
		HomeDir: homeDir,
		GOOS:    "linux",
		LookupEnv: func(key string) (string, bool) {
			if key == "XDG_CONFIG_HOME" {
				return xdgHome, true
			}
			return "", false
		},
	})

	path := store.Path()
	if path != wantPath {
		t.Fatalf("expected fallback path %q, got %q", wantPath, path)
	}
}

func TestResolvePathDefaultsToHomeConfigDirOnWindows(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	wantPath := filepath.Join(homeDir, ".config", "eget", "installed.toml")

	store := NewStore(Options{
		HomeDir:   homeDir,
		GOOS:      "windows",
		LookupEnv: func(string) (string, bool) { return "", false },
	})

	path := store.Path()
	if path != wantPath {
		t.Fatalf("expected fallback path %q, got %q", wantPath, path)
	}
}

func writeInstalledFile(t *testing.T, path, content string) {
	t.Helper()

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
