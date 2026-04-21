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

func TestListPackagesIncludesInstalledOnlyEntries(t *testing.T) {
	now := time.Unix(1710000000, 0).UTC()
	svc := ListService{
		LoadConfig: func() (*cfgpkg.File, error) {
			cfg := cfgpkg.NewFile()
			cfg.Packages["fzf"] = cfgpkg.Section{
				Repo: stringPtr("junegunn/fzf"),
			}
			return cfg, nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{
				Installed: map[string]storepkg.Entry{
					"BurntSushi/ripgrep": {
						Repo:        "BurntSushi/ripgrep",
						InstalledAt: now,
						Asset:       "rg.zip",
						URL:         "https://github.com/BurntSushi/ripgrep/releases/download/v1.0.0/rg.zip",
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

	if items[0].Name != "fzf" {
		t.Fatalf("expected first item to be fzf, got %#v", items[0])
	}
	if items[1].Name != "ripgrep" {
		t.Fatalf("expected installed-only item name ripgrep, got %#v", items[1])
	}
	if items[1].Repo != "BurntSushi/ripgrep" {
		t.Fatalf("expected installed-only repo BurntSushi/ripgrep, got %#v", items[1])
	}
	if !items[1].Installed {
		t.Fatalf("expected installed-only item to be marked installed, got %#v", items[1])
	}
	if items[1].InstalledAt != now {
		t.Fatalf("expected installed_at %v, got %v", now, items[1].InstalledAt)
	}
}

func TestListPackagesMergesInstalledStateIntoExplicitPackageName(t *testing.T) {
	now := time.Unix(1710000000, 0).UTC()
	svc := ListService{
		LoadConfig: func() (*cfgpkg.File, error) {
			cfg := cfgpkg.NewFile()
			cfg.Packages["chlog"] = cfgpkg.Section{
				Repo: stringPtr("gookit/gitw"),
			}
			return cfg, nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{
				Installed: map[string]storepkg.Entry{
					"gookit/gitw": {
						Repo:        "gookit/gitw",
						InstalledAt: now,
						Asset:       "chlog-windows-amd64.exe",
						URL:         "https://github.com/gookit/gitw/releases/download/v0.3.6/chlog-windows-amd64.exe",
					},
				},
			}, nil
		},
	}

	items, err := svc.ListPackages()
	if err != nil {
		t.Fatalf("list packages: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 merged list item, got %#v", items)
	}
	if items[0].Name != "chlog" {
		t.Fatalf("expected explicit package name chlog, got %#v", items[0])
	}
	if !items[0].Installed {
		t.Fatalf("expected explicit package to be marked installed, got %#v", items[0])
	}
	if items[0].InstalledAt != now {
		t.Fatalf("expected installed_at %v, got %v", now, items[0].InstalledAt)
	}
}
