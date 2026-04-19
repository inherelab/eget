package app

import (
	"testing"
	"time"

	cfgpkg "github.com/inherelab/eget/internal/config"
	storepkg "github.com/inherelab/eget/internal/installed"
)

func TestListPackagesMergesManagedPackagesWithInstalledState(t *testing.T) {
	now := time.Unix(1710000000, 0).UTC()
	svc := ListService{
		LoadConfig: func() (*cfgpkg.File, error) {
			cfg := cfgpkg.NewFile()
			cfg.Packages["rg"] = cfgpkg.Section{
				Repo:   stringPtr("BurntSushi/ripgrep"),
				Target: stringPtr("~/.local/bin"),
			}
			cfg.Packages["fzf"] = cfgpkg.Section{
				Repo:   stringPtr("junegunn/fzf"),
				Tag:    stringPtr("nightly"),
				Target: stringPtr("~/.local/bin"),
			}
			return cfg, nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{
				Installed: map[string]storepkg.Entry{
					"junegunn/fzf": {
						Repo:        "junegunn/fzf",
						InstalledAt: now,
						Asset:       "fzf.tar.gz",
						URL:         "https://github.com/junegunn/fzf/releases/download/v1.0.0/fzf.tar.gz",
					},
				},
			}, nil
		},
	}

	items, err := svc.ListPackages()
	if err != nil {
		t.Fatalf("list packages: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 list items, got %d", len(items))
	}

	if items[0].Name != "fzf" || items[1].Name != "rg" {
		t.Fatalf("expected items to be sorted by name, got %#v", items)
	}
	if items[0].Repo != "junegunn/fzf" {
		t.Fatalf("expected fzf repo, got %#v", items[0])
	}
	if !items[0].Installed {
		t.Fatalf("expected fzf to be marked installed, got %#v", items[0])
	}
	if items[0].InstalledAt != now {
		t.Fatalf("expected installed_at %v, got %v", now, items[0].InstalledAt)
	}
	if items[0].Asset != "fzf.tar.gz" {
		t.Fatalf("expected asset fzf.tar.gz, got %#v", items[0])
	}
	if items[1].Installed {
		t.Fatalf("expected rg to be marked not installed, got %#v", items[1])
	}
}

func TestListPackagesReturnsEmptySliceWhenNoManagedPackages(t *testing.T) {
	svc := ListService{
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfgpkg.NewFile(), nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{Installed: map[string]storepkg.Entry{}}, nil
		},
	}

	items, err := svc.ListPackages()
	if err != nil {
		t.Fatalf("list packages: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected empty list, got %#v", items)
	}
}
