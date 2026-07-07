package app

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/gookit/goutil/x/assert"
	cfgpkg "github.com/inherelab/eget/internal/config"
	storepkg "github.com/inherelab/eget/internal/installed"
	"github.com/inherelab/eget/internal/util"
)

func TestListOutdatedPackagesIncludesInstalledOnlyEntries(t *testing.T) {
	now := time.Unix(1710000000, 0).UTC()
	publishedAt := time.Date(2026, 4, 21, 14, 10, 17, 0, time.UTC)
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
		LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
			repo := target.Repo
			switch repo {
			case "BurntSushi/ripgrep":
				return LatestInfo{Tag: "v14.0.0", PublishedAt: publishedAt}, nil
			case "junegunn/fzf":
				return LatestInfo{Tag: "v0.50.0"}, nil
			default:
				return LatestInfo{}, nil
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
	assert.Eq(t, publishedAt, items[0].PublishedAt)
}

func TestListOutdatedPackagesIgnoresConfiguredPackageNames(t *testing.T) {
	svc := ListService{
		LoadConfig: func() (*cfgpkg.File, error) {
			cfg := cfgpkg.NewFile()
			cfg.Global.IgnoreUpdatePackages = []string{"fzf"}
			cfg.Packages["fzf"] = cfgpkg.Section{Repo: util.StringPtr("junegunn/fzf")}
			cfg.Packages["rg"] = cfgpkg.Section{Repo: util.StringPtr("BurntSushi/ripgrep")}
			return cfg, nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{Installed: map[string]storepkg.Entry{
				"junegunn/fzf":       {Repo: "junegunn/fzf", Tag: "v0.50.0"},
				"BurntSushi/ripgrep": {Repo: "BurntSushi/ripgrep", Tag: "v13.0.0"},
			}}, nil
		},
		LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
			repo := target.Repo
			if repo == "junegunn/fzf" {
				t.Fatal("expected ignored package fzf not to be checked")
			}
			return LatestInfo{Tag: "v14.0.0"}, nil
		},
	}

	items, failures, checked, err := svc.ListOutdatedPackages()
	assert.NoErr(t, err)
	assert.Eq(t, 1, checked)
	assert.Eq(t, 0, len(failures))
	assert.Eq(t, 1, len(items))
	assert.Eq(t, "rg", items[0].Name)
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
		LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
			repo := target.Repo
			if repo == "junegunn/fzf" {
				return LatestInfo{}, fmt.Errorf("github api failed")
			}
			return LatestInfo{Tag: "v14.0.0"}, nil
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

func TestListOutdatedPackagesPassesSourcePathToLatestChecker(t *testing.T) {
	svc := ListService{
		LoadConfig: func() (*cfgpkg.File, error) {
			cfg := cfgpkg.NewFile()
			cfg.Packages["winmerge"] = cfgpkg.Section{
				Repo:       util.StringPtr("sourceforge:winmerge"),
				SourcePath: util.StringPtr("stable"),
			}
			return cfg, nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{Installed: map[string]storepkg.Entry{
				"sourceforge:winmerge": {Repo: "sourceforge:winmerge", Tag: "2.16.42"},
			}}, nil
		},
		LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
			repo, sourcePath := target.Repo, target.SourcePath
			if repo != "sourceforge:winmerge" || sourcePath != "stable" {
				t.Fatalf("unexpected latest check repo=%q sourcePath=%q", repo, sourcePath)
			}
			return LatestInfo{Tag: "2.16.44"}, nil
		},
	}

	items, failures, checked, err := svc.ListOutdatedPackages()
	if err != nil {
		t.Fatalf("list outdated packages: %v", err)
	}
	if checked != 1 {
		t.Fatalf("expected 1 checked package, got %d", checked)
	}
	if len(failures) != 0 {
		t.Fatalf("expected no failures, got %#v", failures)
	}
	if len(items) != 1 || items[0].LatestTag != "2.16.44" {
		t.Fatalf("expected sourceforge outdated item, got %#v", items)
	}
}

func TestListOutdatedPackagesPassesInstalledPrereleaseOption(t *testing.T) {
	svc := ListService{
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfgpkg.NewFile(), nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{Installed: map[string]storepkg.Entry{
				"pkgforge/aeris": {
					Repo: "pkgforge/aeris",
					Tag:  "v0.11.0-beta",
					Options: map[string]any{
						"prerelease": true,
					},
				},
			}}, nil
		},
		LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
			assert.Eq(t, "pkgforge/aeris", target.Repo)
			assert.True(t, target.Prerelease)
			return LatestInfo{Tag: "v0.12.0-beta"}, nil
		},
	}

	items, failures, checked, err := svc.ListOutdatedPackages()

	assert.NoErr(t, err)
	assert.Eq(t, 0, len(failures))
	assert.Eq(t, 1, checked)
	assert.Eq(t, 1, len(items))
	assert.Eq(t, "v0.12.0-beta", items[0].LatestTag)
}

func TestListOutdatedPackagesPassesConfiguredPrereleaseOption(t *testing.T) {
	svc := ListService{
		LoadConfig: func() (*cfgpkg.File, error) {
			cfg := cfgpkg.NewFile()
			cfg.Packages["aeris"] = cfgpkg.Section{
				Repo:       util.StringPtr("pkgforge/aeris"),
				Prerelease: util.BoolPtr(true),
			}
			return cfg, nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{Installed: map[string]storepkg.Entry{
				"pkgforge/aeris": {Repo: "pkgforge/aeris", Tag: "v0.11.0-beta"},
			}}, nil
		},
		LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
			assert.Eq(t, "pkgforge/aeris", target.Repo)
			assert.True(t, target.Prerelease)
			return LatestInfo{Tag: "v0.12.0-beta"}, nil
		},
	}

	_, failures, checked, err := svc.ListOutdatedPackages()

	assert.NoErr(t, err)
	assert.Eq(t, 0, len(failures))
	assert.Eq(t, 1, checked)
}

func TestListOutdatedPackagesPreservesExplicitTagChannel(t *testing.T) {
	tests := []struct {
		name          string
		loadConfig    func() (*cfgpkg.File, error)
		loadInstalled func() (*storepkg.Config, error)
	}{
		{
			name: "installed option tag",
			loadConfig: func() (*cfgpkg.File, error) {
				return cfgpkg.NewFile(), nil
			},
			loadInstalled: func() (*storepkg.Config, error) {
				return &storepkg.Config{Installed: map[string]storepkg.Entry{
					"pkgforge/aeris": {
						Repo: "pkgforge/aeris",
						Tag:  "nightly",
						Options: map[string]any{
							"tag": "nightly",
						},
					},
				}}, nil
			},
		},
		{
			name: "configured tag",
			loadConfig: func() (*cfgpkg.File, error) {
				cfg := cfgpkg.NewFile()
				cfg.Packages["aeris"] = cfgpkg.Section{
					Repo: util.StringPtr("pkgforge/aeris"),
					Tag:  util.StringPtr("nightly"),
				}
				return cfg, nil
			},
			loadInstalled: func() (*storepkg.Config, error) {
				return &storepkg.Config{Installed: map[string]storepkg.Entry{
					"pkgforge/aeris": {Repo: "pkgforge/aeris", Tag: "nightly"},
				}}, nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := ListService{
				LoadConfig:    tt.loadConfig,
				LoadInstalled: tt.loadInstalled,
				LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
					assert.Eq(t, "pkgforge/aeris", target.Repo)
					assert.Eq(t, "nightly", target.Tag)
					return LatestInfo{Tag: "nightly"}, nil
				},
			}

			items, failures, checked, err := svc.ListOutdatedPackages()

			assert.NoErr(t, err)
			assert.Eq(t, 0, len(failures))
			assert.Eq(t, 1, checked)
			assert.Eq(t, 0, len(items))
		})
	}
}

func TestListOutdatedPackagesChecksForgeRepo(t *testing.T) {
	svc := ListService{
		LoadConfig: func() (*cfgpkg.File, error) {
			cfg := cfgpkg.NewFile()
			cfg.Packages["forgejo"] = cfgpkg.Section{Repo: util.StringPtr("gitea:codeberg.org/forgejo/forgejo")}
			return cfg, nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{Installed: map[string]storepkg.Entry{
				"gitea:codeberg.org/forgejo/forgejo": {Repo: "gitea:codeberg.org/forgejo/forgejo", Tag: "v8.0.0"},
			}}, nil
		},
		LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
			repo, sourcePath := target.Repo, target.SourcePath
			if repo != "gitea:codeberg.org/forgejo/forgejo" || sourcePath != "" {
				t.Fatalf("unexpected latest check repo=%q sourcePath=%q", repo, sourcePath)
			}
			return LatestInfo{Tag: "v9.0.0"}, nil
		},
	}

	items, failures, checked, err := svc.ListOutdatedPackages()

	assert.NoErr(t, err)
	assert.Eq(t, 1, checked)
	assert.Eq(t, 0, len(failures))
	assert.Eq(t, 1, len(items))
	assert.Eq(t, "v9.0.0", items[0].LatestTag)
}

func TestListOutdatedPackagesChecksPkgTemplateRepo(t *testing.T) {
	cfg := cfgpkg.NewFile()
	cfg.PkgTemplates["mydev"] = cfgpkg.Section{
		LatestURL:   util.StringPtr("http://mydev.lan/tools/{name}/latest.yaml"),
		URLTemplate: util.StringPtr("http://mydev.lan/tools/{name}/{name}-{os}-{arch}{ext}"),
	}
	cfg.Packages["markview"] = cfgpkg.Section{Repo: util.StringPtr("pkg-template:mydev:markview")}
	installed := &storepkg.Config{Installed: map[string]storepkg.Entry{
		"markview": {Repo: "pkg-template:mydev:markview", Target: "pkg-template:mydev:markview", Tag: "1.0.0"},
	}}
	var got LatestCheckTarget
	svc := ListService{
		LoadConfig:    func() (*cfgpkg.File, error) { return cfg, nil },
		LoadInstalled: func() (*storepkg.Config, error) { return installed, nil },
		LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
			got = target
			return LatestInfo{Tag: "1.1.0"}, nil
		},
	}

	items, failures, checked, err := svc.ListOutdatedPackages()

	assert.NoErr(t, err)
	assert.Eq(t, 0, len(failures))
	assert.Eq(t, 1, checked)
	assert.Eq(t, 1, len(items))
	assert.Eq(t, "pkg-template:mydev:markview", got.Repo)
	if got.Package.LatestURL == nil {
		t.Fatal("expected rendered latest_url in latest check package")
	}
	assert.Eq(t, "http://mydev.lan/tools/markview/latest.yaml", *got.Package.LatestURL)
}

func TestListOutdatedPackagesUsesConfiguredBatchConcurrencyAndPreservesOrder(t *testing.T) {
	block := make(chan struct{})
	batch := 2
	var mu sync.Mutex
	active := 0
	maxActive := 0
	svc := ListService{
		LoadConfig: func() (*cfgpkg.File, error) {
			cfg := cfgpkg.NewFile()
			cfg.Global.BatchConcurrency = &batch
			cfg.Packages["fzf"] = cfgpkg.Section{Repo: util.StringPtr("junegunn/fzf")}
			cfg.Packages["rg"] = cfgpkg.Section{Repo: util.StringPtr("BurntSushi/ripgrep")}
			cfg.Packages["fd"] = cfgpkg.Section{Repo: util.StringPtr("sharkdp/fd")}
			return cfg, nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{Installed: map[string]storepkg.Entry{
				"junegunn/fzf":       {Repo: "junegunn/fzf", Tag: "v0.1.0"},
				"BurntSushi/ripgrep": {Repo: "BurntSushi/ripgrep", Tag: "v0.1.0"},
				"sharkdp/fd":         {Repo: "sharkdp/fd", Tag: "v0.1.0"},
			}}, nil
		},
		LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
			mu.Lock()
			active++
			if active > maxActive {
				maxActive = active
			}
			mu.Unlock()

			<-block

			mu.Lock()
			active--
			mu.Unlock()
			return LatestInfo{Tag: "v1.0.0"}, nil
		},
	}

	done := make(chan []OutdatedItem, 1)
	errCh := make(chan error, 1)
	go func() {
		items, _, _, err := svc.ListOutdatedPackages()
		if err != nil {
			errCh <- err
			return
		}
		done <- items
	}()

	waitForMaxActive(t, func() int {
		mu.Lock()
		defer mu.Unlock()
		return maxActive
	}, 2)
	close(block)

	select {
	case err := <-errCh:
		t.Fatalf("list outdated packages: %v", err)
	case items := <-done:
		assert.Eq(t, []string{"fd", "fzf", "rg"}, []string{items[0].Name, items[1].Name, items[2].Name})
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for list outdated packages")
	}
	assert.Eq(t, 2, func() int {
		mu.Lock()
		defer mu.Unlock()
		return maxActive
	}())
}

func TestListOutdatedPackagesUsesAutoBatchConcurrency(t *testing.T) {
	block := make(chan struct{})
	batch := 0
	var mu sync.Mutex
	active := 0
	maxActive := 0
	svc := ListService{
		LoadConfig: func() (*cfgpkg.File, error) {
			cfg := cfgpkg.NewFile()
			cfg.Global.BatchConcurrency = &batch
			cfg.Packages["fzf"] = cfgpkg.Section{Repo: util.StringPtr("junegunn/fzf")}
			cfg.Packages["rg"] = cfgpkg.Section{Repo: util.StringPtr("BurntSushi/ripgrep")}
			cfg.Packages["fd"] = cfgpkg.Section{Repo: util.StringPtr("sharkdp/fd")}
			return cfg, nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{Installed: map[string]storepkg.Entry{
				"junegunn/fzf":       {Repo: "junegunn/fzf", Tag: "v0.1.0"},
				"BurntSushi/ripgrep": {Repo: "BurntSushi/ripgrep", Tag: "v0.1.0"},
				"sharkdp/fd":         {Repo: "sharkdp/fd", Tag: "v0.1.0"},
			}}, nil
		},
		LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
			mu.Lock()
			active++
			if active > maxActive {
				maxActive = active
			}
			mu.Unlock()

			<-block

			mu.Lock()
			active--
			mu.Unlock()
			return LatestInfo{Tag: "v1.0.0"}, nil
		},
	}

	done := make(chan []OutdatedItem, 1)
	errCh := make(chan error, 1)
	go func() {
		items, _, _, err := svc.ListOutdatedPackages()
		if err != nil {
			errCh <- err
			return
		}
		done <- items
	}()

	waitForMaxActive(t, func() int {
		mu.Lock()
		defer mu.Unlock()
		return maxActive
	}, 3)
	close(block)

	select {
	case err := <-errCh:
		t.Fatalf("list outdated packages: %v", err)
	case items := <-done:
		assert.Eq(t, []string{"fd", "fzf", "rg"}, []string{items[0].Name, items[1].Name, items[2].Name})
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for list outdated packages")
	}
}

func TestListOutdatedPackagesReportsCheckProgress(t *testing.T) {
	var mu sync.Mutex
	var progress []int
	svc := ListService{
		LoadConfig: func() (*cfgpkg.File, error) {
			cfg := cfgpkg.NewFile()
			cfg.Packages["fzf"] = cfgpkg.Section{Repo: util.StringPtr("junegunn/fzf")}
			cfg.Packages["rg"] = cfgpkg.Section{Repo: util.StringPtr("BurntSushi/ripgrep")}
			cfg.Packages["fd"] = cfgpkg.Section{Repo: util.StringPtr("sharkdp/fd")}
			return cfg, nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{Installed: map[string]storepkg.Entry{
				"junegunn/fzf":       {Repo: "junegunn/fzf", Tag: "v0.1.0"},
				"BurntSushi/ripgrep": {Repo: "BurntSushi/ripgrep", Tag: "v0.1.0"},
				"sharkdp/fd":         {Repo: "sharkdp/fd", Tag: "v0.1.0"},
			}}, nil
		},
		LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
			return LatestInfo{Tag: "v1.0.0"}, nil
		},
		OnCheckDone: func(checked, total int) {
			assert.Eq(t, 3, total)
			mu.Lock()
			defer mu.Unlock()
			progress = append(progress, checked)
		},
	}

	_, _, checked, err := svc.ListOutdatedPackages()
	assert.NoErr(t, err)
	assert.Eq(t, 3, checked)

	mu.Lock()
	defer mu.Unlock()
	assert.Eq(t, 4, len(progress))
	assert.Eq(t, 0, progress[0])
	assert.Eq(t, 3, progress[len(progress)-1])
}
