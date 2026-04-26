package app

import (
	"fmt"
	"testing"
	"time"

	cfgpkg "github.com/inherelab/eget/internal/config"
	storepkg "github.com/inherelab/eget/internal/installed"
	"github.com/inherelab/eget/internal/util"
)

func TestListPackagesMergesManagedPackagesWithInstalledState(t *testing.T) {
	now := time.Unix(1710000000, 0).UTC()
	svc := ListService{
		LoadConfig: func() (*cfgpkg.File, error) {
			cfg := cfgpkg.NewFile()
			cfg.Packages["rg"] = cfgpkg.Section{
				Repo:   util.StringPtr("BurntSushi/ripgrep"),
				Target: util.StringPtr("~/.local/bin"),
			}
			cfg.Packages["fzf"] = cfgpkg.Section{
				Repo:   util.StringPtr("junegunn/fzf"),
				Tag:    util.StringPtr("nightly"),
				Target: util.StringPtr("~/.local/bin"),
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
	if items[0].Version != "" {
		t.Fatalf("expected empty version without stored tag/version, got %#v", items[0])
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
				Repo: util.StringPtr("junegunn/fzf"),
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

func TestListInstalledPackagesFiltersManagedOnlyEntries(t *testing.T) {
	now := time.Unix(1710000000, 0).UTC()
	svc := ListService{
		LoadConfig: func() (*cfgpkg.File, error) {
			cfg := cfgpkg.NewFile()
			cfg.Packages["rg"] = cfgpkg.Section{Repo: util.StringPtr("BurntSushi/ripgrep")}
			cfg.Packages["fzf"] = cfgpkg.Section{Repo: util.StringPtr("junegunn/fzf")}
			return cfg, nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{
				Installed: map[string]storepkg.Entry{
					"junegunn/fzf": {
						Repo:        "junegunn/fzf",
						InstalledAt: now,
						Tag:         "v0.50.0",
					},
				},
			}, nil
		},
	}

	items, err := svc.ListInstalledPackages()
	if err != nil {
		t.Fatalf("list installed packages: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected only installed items, got %#v", items)
	}
	if items[0].Name != "fzf" || !items[0].Installed {
		t.Fatalf("expected installed fzf item, got %#v", items[0])
	}
}

func TestListPackagesMergesInstalledStateIntoExplicitPackageName(t *testing.T) {
	now := time.Unix(1710000000, 0).UTC()
	svc := ListService{
		LoadConfig: func() (*cfgpkg.File, error) {
			cfg := cfgpkg.NewFile()
			cfg.Packages["chlog"] = cfgpkg.Section{
				Repo: util.StringPtr("gookit/gitw"),
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

func TestListOutdatedPackagesIncludesInstalledOnlyEntries(t *testing.T) {
	now := time.Unix(1710000000, 0).UTC()
	svc := ListService{
		LoadConfig: func() (*cfgpkg.File, error) {
			cfg := cfgpkg.NewFile()
			cfg.Packages["fzf"] = cfgpkg.Section{
				Repo: util.StringPtr("junegunn/fzf"),
			}
			return cfg, nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{
				Installed: map[string]storepkg.Entry{
					"BurntSushi/ripgrep": {
						Repo:        "BurntSushi/ripgrep",
						InstalledAt: now,
						Tag:         "v13.0.0",
					},
					"junegunn/fzf": {
						Repo:        "junegunn/fzf",
						InstalledAt: now,
						Tag:         "v0.50.0",
					},
				},
			}, nil
		},
		LatestTag: func(repo string) (string, error) {
			switch repo {
			case "BurntSushi/ripgrep":
				return "v14.0.0", nil
			case "junegunn/fzf":
				return "v0.50.0", nil
			default:
				return "", nil
			}
		},
	}

	items, failures, checked, err := svc.ListOutdatedPackages()
	if err != nil {
		t.Fatalf("list outdated packages: %v", err)
	}
	if checked != 2 {
		t.Fatalf("expected 2 checked packages, got %d", checked)
	}
	if len(failures) != 0 {
		t.Fatalf("expected no failures, got %#v", failures)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 outdated item, got %#v", items)
	}
	if items[0].Name != "ripgrep" {
		t.Fatalf("expected installed-only outdated item name ripgrep, got %#v", items[0])
	}
	if items[0].InstalledTag != "v13.0.0" || items[0].LatestTag != "v14.0.0" {
		t.Fatalf("expected outdated tag comparison, got %#v", items[0])
	}
}

func TestListOutdatedPackagesSkipsFailedChecks(t *testing.T) {
	now := time.Unix(1710000000, 0).UTC()
	svc := ListService{
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfgpkg.NewFile(), nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{
				Installed: map[string]storepkg.Entry{
					"junegunn/fzf": {
						Repo:        "junegunn/fzf",
						InstalledAt: now,
						Tag:         "v0.50.0",
					},
					"BurntSushi/ripgrep": {
						Repo:        "BurntSushi/ripgrep",
						InstalledAt: now,
						Tag:         "v13.0.0",
					},
				},
			}, nil
		},
		LatestTag: func(repo string) (string, error) {
			if repo == "junegunn/fzf" {
				return "", fmt.Errorf("github api failed")
			}
			return "v14.0.0", nil
		},
	}

	items, failures, checked, err := svc.ListOutdatedPackages()
	if err != nil {
		t.Fatalf("list outdated packages: %v", err)
	}
	if checked != 2 {
		t.Fatalf("expected 2 checked packages, got %d", checked)
	}
	if len(items) != 1 || items[0].Repo != "BurntSushi/ripgrep" {
		t.Fatalf("expected only successful outdated item, got %#v", items)
	}
	if len(failures) != 1 || failures[0].Repo != "junegunn/fzf" {
		t.Fatalf("expected one failed check, got %#v", failures)
	}
}

func TestFindPackageReturnsMergedItem(t *testing.T) {
	now := time.Unix(1710000000, 0).UTC()
	svc := ListService{
		LoadConfig: func() (*cfgpkg.File, error) {
			cfg := cfgpkg.NewFile()
			cfg.Packages["chlog"] = cfgpkg.Section{
				Repo:   util.StringPtr("gookit/gitw"),
				Target: util.StringPtr("~/.local/bin"),
			}
			return cfg, nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{
				Installed: map[string]storepkg.Entry{
					"gookit/gitw": {
						Repo:        "gookit/gitw",
						InstalledAt: now,
						Tag:         "v0.3.6",
						Asset:       "chlog-windows-amd64.exe",
						URL:         "https://github.com/gookit/gitw/releases/download/v0.3.6/chlog-windows-amd64.exe",
					},
				},
			}, nil
		},
	}

	item, err := svc.FindPackage("chlog")
	if err != nil {
		t.Fatalf("find package: %v", err)
	}
	if item.Name != "chlog" || item.Repo != "gookit/gitw" {
		t.Fatalf("unexpected item: %#v", item)
	}
	if item.Version != "v0.3.6" {
		t.Fatalf("expected version v0.3.6, got %#v", item)
	}
}
