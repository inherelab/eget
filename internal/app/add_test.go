package app

import (
	"path/filepath"
	"testing"

	cfgpkg "github.com/inherelab/eget/internal/config"
	"github.com/inherelab/eget/internal/install"
)

func TestAddPackage(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "eget.toml")

	svc := ConfigService{
		ConfigPath: configPath,
		Load: func() (*cfgpkg.File, error) {
			return cfgpkg.NewFile(), nil
		},
		Save: cfgpkg.Save,
	}

	opts := install.Options{
		Output:      "~/.local/bin",
		CacheDir:    "~/.cache/eget",
		System:      "linux/amd64",
		ExtractFile: "fzf",
		Asset:       []string{"linux_amd64"},
		Tag:         "nightly",
		Verify:      "sha256:123",
		Source:      true,
		SourcePath:  "stable",
		DisableSSL:  true,
		All:         true,
		IsGUI:       true,
	}

	if err := svc.AddPackage("junegunn/fzf", "", opts); err != nil {
		t.Fatalf("add package: %v", err)
	}

	cfg, err := cfgpkg.LoadFile(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	pkg, ok := cfg.Packages["fzf"]
	if !ok {
		t.Fatal("expected packages.fzf to be created")
	}
	if pkg.Repo == nil || *pkg.Repo != "junegunn/fzf" {
		t.Fatalf("expected repo to be persisted, got %#v", pkg.Repo)
	}
	if pkg.Target == nil || *pkg.Target != "~/.local/bin" {
		t.Fatalf("expected target to be persisted, got %#v", pkg.Target)
	}
	if pkg.CacheDir == nil || *pkg.CacheDir != "~/.cache/eget" {
		t.Fatalf("expected cache_dir to be persisted, got %#v", pkg.CacheDir)
	}
	if pkg.Source == nil || !*pkg.Source {
		t.Fatalf("expected download_source to be persisted, got %#v", pkg.Source)
	}
	if pkg.SourcePath == nil || *pkg.SourcePath != "stable" {
		t.Fatalf("expected source_path to be persisted, got %#v", pkg.SourcePath)
	}
	if pkg.DisableSSL == nil || !*pkg.DisableSSL {
		t.Fatalf("expected disable_ssl to be persisted, got %#v", pkg.DisableSSL)
	}
	if len(pkg.AssetFilters) != 1 || pkg.AssetFilters[0] != "linux_amd64" {
		t.Fatalf("expected asset filters to be persisted, got %#v", pkg.AssetFilters)
	}
	if pkg.IsGUI == nil || !*pkg.IsGUI {
		t.Fatalf("expected is_gui to be persisted, got %#v", pkg.IsGUI)
	}
}

func TestAddPackageWithCustomName(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "eget.toml")

	svc := ConfigService{
		ConfigPath: configPath,
		Load: func() (*cfgpkg.File, error) {
			return cfgpkg.NewFile(), nil
		},
		Save: cfgpkg.Save,
	}

	if err := svc.AddPackage("junegunn/fzf", "myfzf", install.Options{}); err != nil {
		t.Fatalf("add package: %v", err)
	}

	cfg, err := cfgpkg.LoadFile(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if _, ok := cfg.Packages["myfzf"]; !ok {
		t.Fatal("expected packages.myfzf to be created")
	}
}
