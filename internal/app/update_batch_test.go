package app

import (
	"errors"
	"testing"
	"time"

	"github.com/gookit/goutil/x/assert"
	cfgpkg "github.com/inherelab/eget/internal/config"
	"github.com/inherelab/eget/internal/install"
	storepkg "github.com/inherelab/eget/internal/installed"
	"github.com/inherelab/eget/internal/util"
)

func TestUpdateAllPackagesIteratesOutdatedManagedPackages(t *testing.T) {
	installer := &fakeInstallService{}
	svc := UpdateService{
		Install: installer,
		LoadConfig: func() (*cfgpkg.File, error) {
			cfg := cfgpkg.NewFile()
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
			switch repo {
			case "junegunn/fzf":
				return LatestInfo{Tag: "v0.51.0"}, nil
			case "BurntSushi/ripgrep":
				return LatestInfo{Tag: "v14.0.0"}, nil
			default:
				return LatestInfo{}, nil
			}
		},
	}

	results, err := svc.UpdateAllPackages(install.Options{})
	if err != nil {
		t.Fatalf("update all packages: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 update results, got %d", len(results))
	}
	if len(installer.targets) != 2 {
		t.Fatalf("expected installer to run twice, got %d", len(installer.targets))
	}
}

func TestApplyUpdateCLIOverridesPreservesRetries(t *testing.T) {
	got := applyUpdateCLIOverrides(install.Options{Retries: 1}, install.Options{Retries: 3})

	assert.Eq(t, 3, got.Retries)
}

func TestUpdateAllPackagesInstallsOnlyOutdatedInstalledPackages(t *testing.T) {
	now := time.Unix(1710000000, 0).UTC()
	installer := &fakeInstallService{}
	svc := UpdateService{
		Install: installer,
		LoadConfig: func() (*cfgpkg.File, error) {
			cfg := cfgpkg.NewFile()
			cfg.Packages["fzf"] = cfgpkg.Section{Repo: util.StringPtr("junegunn/fzf")}
			cfg.Packages["rg"] = cfgpkg.Section{Repo: util.StringPtr("BurntSushi/ripgrep")}
			cfg.Packages["fd"] = cfgpkg.Section{Repo: util.StringPtr("sharkdp/fd")}
			return cfg, nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{Installed: map[string]storepkg.Entry{
				"junegunn/fzf":       {Repo: "junegunn/fzf", InstalledAt: now, Tag: "v0.50.0"},
				"BurntSushi/ripgrep": {Repo: "BurntSushi/ripgrep", InstalledAt: now, Tag: "v13.0.0"},
			}}, nil
		},
		LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
			repo := target.Repo
			switch repo {
			case "junegunn/fzf":
				return LatestInfo{Tag: "v0.50.0"}, nil
			case "BurntSushi/ripgrep":
				return LatestInfo{Tag: "v14.0.0"}, nil
			default:
				t.Fatalf("unexpected latest tag check for %s", repo)
				return LatestInfo{}, nil
			}
		},
	}

	results, err := svc.UpdateAllPackages(install.Options{})
	assert.NoErr(t, err)
	assert.Eq(t, []string{"rg"}, installer.targets)
	assert.Eq(t, 1, len(results))
	assert.Eq(t, "rg", results[0].Name)
	assert.Eq(t, "BurntSushi/ripgrep", results[0].Target)
}

func TestUpdateAllPackagesUpdatesInstalledOnlyPackage(t *testing.T) {
	installer := &fakeInstallService{}
	svc := UpdateService{
		Install: installer,
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfgpkg.NewFile(), nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{Installed: map[string]storepkg.Entry{
				"microsoft/apm": {
					Repo:   "microsoft/apm",
					Target: "microsoft/apm",
					Tag:    "v0.23.1",
					Options: map[string]any{
						"asset": []string{"x86_64", "linux", "gz"},
					},
				},
			}}, nil
		},
		LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
			assert.Eq(t, "microsoft/apm", target.Repo)
			return LatestInfo{Tag: "v0.24.0"}, nil
		},
	}

	results, err := svc.UpdateAllPackages(install.Options{})

	assert.NoErr(t, err)
	assert.Eq(t, []string{"microsoft/apm"}, installer.targets)
	assert.Eq(t, 1, len(results))
	assert.Eq(t, "apm", results[0].Name)
	assert.Eq(t, "microsoft/apm", results[0].Target)
	assert.Eq(t, []string{"x86_64", "linux", "gz"}, installer.options[0].Asset)
	assert.Eq(t, install.OperationUpdate, installer.options[0].Operation)
	assert.Eq(t, "v0.23.1", installer.options[0].CurrentVersion)
	assert.Eq(t, "v0.24.0", installer.options[0].TargetVersion)
}

func TestUpdateAllPackagesContinuesAfterPackageFailure(t *testing.T) {
	installer := &fakeInstallService{
		errs: map[string]error{"aeris": errors.New("asset `\\.xz$` not found")},
	}
	svc := UpdateService{
		Install: installer,
		LoadConfig: func() (*cfgpkg.File, error) {
			cfg := cfgpkg.NewFile()
			cfg.Packages["aeris"] = cfgpkg.Section{Repo: util.StringPtr("pkgforge/aeris")}
			cfg.Packages["rg"] = cfgpkg.Section{Repo: util.StringPtr("BurntSushi/ripgrep")}
			cfg.Packages["uv"] = cfgpkg.Section{Repo: util.StringPtr("astral-sh/uv")}
			return cfg, nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{Installed: map[string]storepkg.Entry{
				"pkgforge/aeris":     {Repo: "pkgforge/aeris", Tag: "nightly"},
				"BurntSushi/ripgrep": {Repo: "BurntSushi/ripgrep", Tag: "v13.0.0"},
				"astral-sh/uv":       {Repo: "astral-sh/uv", Tag: "v0.7.0"},
			}}, nil
		},
		LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
			return LatestInfo{Tag: "v99.0.0"}, nil
		},
	}

	results, err := svc.UpdateAllPackages(install.Options{BatchConcurrency: 1, BatchConcurrencySet: true})

	assert.Err(t, err)
	assert.Contains(t, err.Error(), "1 update failed")
	assert.Eq(t, []string{"aeris", "rg", "uv"}, installer.targets)
	assert.Eq(t, 2, len(results))
}

func TestUpdateAllPackagesConcurrentContinuesAfterPackageFailure(t *testing.T) {
	installer := &fakeInstallService{
		errs: map[string]error{"aeris": errors.New("asset `\\.xz$` not found")},
	}
	svc := UpdateService{
		Install: installer,
		LoadConfig: func() (*cfgpkg.File, error) {
			cfg := cfgpkg.NewFile()
			cfg.Packages["aeris"] = cfgpkg.Section{Repo: util.StringPtr("pkgforge/aeris")}
			cfg.Packages["rg"] = cfgpkg.Section{Repo: util.StringPtr("BurntSushi/ripgrep")}
			cfg.Packages["uv"] = cfgpkg.Section{Repo: util.StringPtr("astral-sh/uv")}
			return cfg, nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{Installed: map[string]storepkg.Entry{
				"pkgforge/aeris":     {Repo: "pkgforge/aeris", Tag: "nightly"},
				"BurntSushi/ripgrep": {Repo: "BurntSushi/ripgrep", Tag: "v13.0.0"},
				"astral-sh/uv":       {Repo: "astral-sh/uv", Tag: "v0.7.0"},
			}}, nil
		},
		LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
			return LatestInfo{Tag: "v99.0.0"}, nil
		},
	}

	results, err := svc.UpdateAllPackages(install.Options{BatchConcurrency: 2})

	assert.Err(t, err)
	assert.Contains(t, err.Error(), "1 update failed")
	assert.Eq(t, []string{"aeris", "rg", "uv"}, sortedStrings(installer.targets))
	assert.Eq(t, 2, len(results))
}

func TestUpdateAllPackagesIgnoresConfiguredPackageNames(t *testing.T) {
	installer := &fakeInstallService{}
	svc := UpdateService{
		Install: installer,
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

	results, err := svc.UpdateAllPackages(install.Options{})
	assert.NoErr(t, err)
	assert.Eq(t, []string{"rg"}, installer.targets)
	assert.Eq(t, 1, len(results))
	assert.Eq(t, "rg", results[0].Name)
}

func TestUpdateAllPackagesPassesAPICacheOptionsFromConfigToInstaller(t *testing.T) {
	installer := &fakeInstallService{}
	cacheTime := 120
	apiCacheEnabled := true
	svc := UpdateService{
		Install: installer,
		LoadConfig: func() (*cfgpkg.File, error) {
			cfg := cfgpkg.NewFile()
			cfg.Global.CacheDir = util.StringPtr("~/.cache/eget")
			cfg.ApiCache.Enable = &apiCacheEnabled
			cfg.ApiCache.CacheTime = &cacheTime
			cfg.Packages["rg"] = cfgpkg.Section{Repo: util.StringPtr("BurntSushi/ripgrep")}
			return cfg, nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{Installed: map[string]storepkg.Entry{
				"BurntSushi/ripgrep": {Repo: "BurntSushi/ripgrep", Tag: "v13.0.0"},
			}}, nil
		},
		LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
			return LatestInfo{Tag: "v14.0.0"}, nil
		},
	}

	_, err := svc.UpdateAllPackages(install.Options{})
	assert.NoErr(t, err)
	assert.Eq(t, 1, len(installer.options))
	assert.True(t, installer.options[0].APICacheEnabled)
	assert.Eq(t, 120, installer.options[0].APICacheTime)
	if installer.options[0].APICacheDir == "" {
		t.Fatalf("expected api cache dir to be derived from config, got %#v", installer.options[0])
	}
}

func TestUpdateAllPackagesNoProxySkipsConfiguredProxyURL(t *testing.T) {
	t.Setenv("NO_PROXY", "")
	installer := &fakeInstallService{}
	proxyURL := "http://127.0.0.1:7890"
	svc := UpdateService{
		Install: installer,
		LoadConfig: func() (*cfgpkg.File, error) {
			cfg := cfgpkg.NewFile()
			cfg.Global.ProxyURL = &proxyURL
			cfg.Packages["rg"] = cfgpkg.Section{Repo: util.StringPtr("BurntSushi/ripgrep")}
			return cfg, nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{Installed: map[string]storepkg.Entry{
				"BurntSushi/ripgrep": {Repo: "BurntSushi/ripgrep", Tag: "v13.0.0"},
			}}, nil
		},
		LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
			return LatestInfo{Tag: "v14.0.0"}, nil
		},
	}

	_, err := svc.UpdateAllPackages(install.Options{NoProxy: true})

	assert.NoErr(t, err)
	assert.Eq(t, 1, len(installer.options))
	assert.Eq(t, "", installer.options[0].ProxyURL)
	assert.True(t, installer.options[0].NoProxy)
}

func TestUpdateAllPackagesUsesHTTPProxyConfig(t *testing.T) {
	t.Setenv("NO_PROXY", "")
	installer := &fakeInstallService{}
	proxyURL := "http://127.0.0.1:10801"
	svc := UpdateService{
		Install: installer,
		LoadConfig: func() (*cfgpkg.File, error) {
			cfg := cfgpkg.NewFile()
			cfg.HTTPProxy.URL = &proxyURL
			cfg.HTTPProxy.Exclude = []string{"mydev.com"}
			cfg.Packages["rg"] = cfgpkg.Section{Repo: util.StringPtr("BurntSushi/ripgrep")}
			return cfg, nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{Installed: map[string]storepkg.Entry{
				"BurntSushi/ripgrep": {Repo: "BurntSushi/ripgrep", Tag: "v13.0.0"},
			}}, nil
		},
		LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
			return LatestInfo{Tag: "v14.0.0"}, nil
		},
	}

	_, err := svc.UpdateAllPackages(install.Options{})

	assert.NoErr(t, err)
	assert.Eq(t, 1, len(installer.options))
	assert.Eq(t, proxyURL, installer.options[0].ProxyURL)
	assert.Eq(t, []string{"mydev.com"}, installer.options[0].ProxyExclude)
}

func TestUpdateAllPackagesHTTPProxyEnableFalseDisablesConfiguredProxy(t *testing.T) {
	t.Setenv("NO_PROXY", "")
	installer := &fakeInstallService{}
	enabled := false
	proxyURL := "http://127.0.0.1:10801"
	svc := UpdateService{
		Install: installer,
		LoadConfig: func() (*cfgpkg.File, error) {
			cfg := cfgpkg.NewFile()
			cfg.HTTPProxy.Enable = &enabled
			cfg.HTTPProxy.URL = &proxyURL
			cfg.Packages["rg"] = cfgpkg.Section{Repo: util.StringPtr("BurntSushi/ripgrep")}
			return cfg, nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{Installed: map[string]storepkg.Entry{
				"BurntSushi/ripgrep": {Repo: "BurntSushi/ripgrep", Tag: "v13.0.0"},
			}}, nil
		},
		LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
			return LatestInfo{Tag: "v14.0.0"}, nil
		},
	}

	_, err := svc.UpdateAllPackages(install.Options{})

	assert.NoErr(t, err)
	assert.Eq(t, 1, len(installer.options))
	assert.Eq(t, "", installer.options[0].ProxyURL)
}

func TestUpdateAllPackagesHTTPProxyPrefersNewBlockOverLegacy(t *testing.T) {
	t.Setenv("NO_PROXY", "")
	installer := &fakeInstallService{}
	legacyURL := "http://127.0.0.1:7890"
	proxyURL := "http://127.0.0.1:10801"
	svc := UpdateService{
		Install: installer,
		LoadConfig: func() (*cfgpkg.File, error) {
			cfg := cfgpkg.NewFile()
			cfg.Global.ProxyURL = &legacyURL
			cfg.HTTPProxy.URL = &proxyURL
			cfg.Packages["rg"] = cfgpkg.Section{Repo: util.StringPtr("BurntSushi/ripgrep")}
			return cfg, nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{Installed: map[string]storepkg.Entry{
				"BurntSushi/ripgrep": {Repo: "BurntSushi/ripgrep", Tag: "v13.0.0"},
			}}, nil
		},
		LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
			return LatestInfo{Tag: "v14.0.0"}, nil
		},
	}

	_, err := svc.UpdateAllPackages(install.Options{})

	assert.NoErr(t, err)
	assert.Eq(t, 1, len(installer.options))
	assert.Eq(t, proxyURL, installer.options[0].ProxyURL)
}

func TestUpdateAllPackagesNoProxyEnvAddsProxyExcludeOrDisablesProxy(t *testing.T) {
	proxyURL := "http://127.0.0.1:10801"
	newService := func(installer *fakeInstallService) UpdateService {
		return UpdateService{
			Install: installer,
			LoadConfig: func() (*cfgpkg.File, error) {
				cfg := cfgpkg.NewFile()
				cfg.HTTPProxy.URL = &proxyURL
				cfg.Packages["rg"] = cfgpkg.Section{Repo: util.StringPtr("BurntSushi/ripgrep")}
				return cfg, nil
			},
			LoadInstalled: func() (*storepkg.Config, error) {
				return &storepkg.Config{Installed: map[string]storepkg.Entry{
					"BurntSushi/ripgrep": {Repo: "BurntSushi/ripgrep", Tag: "v13.0.0"},
				}}, nil
			},
			LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
				return LatestInfo{Tag: "v14.0.0"}, nil
			},
		}
	}

	t.Setenv("NO_PROXY", "api.github.com")
	installer := &fakeInstallService{}
	_, err := newService(installer).UpdateAllPackages(install.Options{})
	assert.NoErr(t, err)
	assert.Eq(t, proxyURL, installer.options[0].ProxyURL)
	assert.Eq(t, []string{"api.github.com"}, installer.options[0].ProxyExclude)

	t.Setenv("NO_PROXY", "1")
	installer = &fakeInstallService{}
	_, err = newService(installer).UpdateAllPackages(install.Options{})
	assert.NoErr(t, err)
	assert.Eq(t, "", installer.options[0].ProxyURL)
}

func TestUpdateAllPackagesCLIProxyURLOverridesHTTPProxyConfig(t *testing.T) {
	t.Setenv("NO_PROXY", "")
	installer := &fakeInstallService{}
	proxyURL := "http://127.0.0.1:10801"
	cliURL := "http://127.0.0.1:10802"
	svc := UpdateService{
		Install: installer,
		LoadConfig: func() (*cfgpkg.File, error) {
			cfg := cfgpkg.NewFile()
			cfg.HTTPProxy.URL = &proxyURL
			cfg.Packages["rg"] = cfgpkg.Section{Repo: util.StringPtr("BurntSushi/ripgrep")}
			return cfg, nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{Installed: map[string]storepkg.Entry{
				"BurntSushi/ripgrep": {Repo: "BurntSushi/ripgrep", Tag: "v13.0.0"},
			}}, nil
		},
		LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
			return LatestInfo{Tag: "v14.0.0"}, nil
		},
	}

	_, err := svc.UpdateAllPackages(install.Options{ProxyURL: cliURL})

	assert.NoErr(t, err)
	assert.Eq(t, 1, len(installer.options))
	assert.Eq(t, cliURL, installer.options[0].ProxyURL)
}

func TestUpdateAllPackagesWithAppInstallerPreservesPackageProxyOverride(t *testing.T) {
	t.Setenv("NO_PROXY", "")
	cfg := cfgpkg.NewFile()
	globalURL := "http://127.0.0.1:10801"
	packageURL := "http://127.0.0.1:10802"
	cfg.HTTPProxy.URL = &globalURL
	cfg.Packages["rg"] = cfgpkg.Section{
		Repo:     util.StringPtr("BurntSushi/ripgrep"),
		ProxyURL: &packageURL,
	}
	runner := &fakeRunner{}
	installSvc := Service{
		Runner: runner,
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfg, nil
		},
	}
	updateSvc := UpdateService{
		Install: installSvc,
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfg, nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{Installed: map[string]storepkg.Entry{
				"BurntSushi/ripgrep": {Repo: "BurntSushi/ripgrep", Tag: "v13.0.0"},
			}}, nil
		},
		LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
			return LatestInfo{Tag: "v14.0.0"}, nil
		},
	}

	_, err := updateSvc.UpdateAllPackages(install.Options{})

	assert.NoErr(t, err)
	assert.Eq(t, packageURL, runner.opts.ProxyURL)
}

func TestUpdateAllPackagesWithAppInstallerPreservesRepoProxyOverride(t *testing.T) {
	t.Setenv("NO_PROXY", "")
	cfg := cfgpkg.NewFile()
	globalURL := "http://127.0.0.1:10801"
	repoURL := "http://127.0.0.1:10802"
	cfg.HTTPProxy.URL = &globalURL
	cfg.Repos["BurntSushi/ripgrep"] = cfgpkg.Section{ProxyURL: &repoURL}
	cfg.Packages["rg"] = cfgpkg.Section{Repo: util.StringPtr("BurntSushi/ripgrep")}
	runner := &fakeRunner{}
	installSvc := Service{
		Runner: runner,
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfg, nil
		},
	}
	updateSvc := UpdateService{
		Install: installSvc,
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfg, nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{Installed: map[string]storepkg.Entry{
				"BurntSushi/ripgrep": {Repo: "BurntSushi/ripgrep", Tag: "v13.0.0"},
			}}, nil
		},
		LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
			return LatestInfo{Tag: "v14.0.0"}, nil
		},
	}

	_, err := updateSvc.UpdateAllPackages(install.Options{})

	assert.NoErr(t, err)
	assert.Eq(t, repoURL, runner.opts.ProxyURL)
}

func TestUpdateAllPackagesUsesBatchConcurrencyAndPreservesResultOrder(t *testing.T) {
	block := make(chan struct{})
	installer := &fakeInstallService{block: block}
	svc := UpdateService{
		Install: installer,
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
	}

	done := make(chan []UpdateResult, 1)
	errCh := make(chan error, 1)
	go func() {
		results, err := svc.UpdateAllPackages(install.Options{BatchConcurrency: 2})
		if err != nil {
			errCh <- err
			return
		}
		done <- results
	}()

	waitForMaxActive(t, func() int { return installer.currentMaxActive() }, 2)
	close(block)

	select {
	case err := <-errCh:
		t.Fatalf("update all packages: %v", err)
	case results := <-done:
		assert.Eq(t, []string{"fd", "fzf", "rg"}, []string{results[0].Name, results[1].Name, results[2].Name})
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for update all")
	}
	assert.Eq(t, 2, installer.currentMaxActive())
}

func TestUpdateAllPackagesUsesAutoBatchConcurrency(t *testing.T) {
	block := make(chan struct{})
	installer := &fakeInstallService{block: block}
	svc := UpdateService{
		Install: installer,
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
	}

	done := make(chan []UpdateResult, 1)
	errCh := make(chan error, 1)
	go func() {
		results, err := svc.UpdateAllPackages(install.Options{BatchConcurrency: 0, BatchConcurrencySet: true})
		if err != nil {
			errCh <- err
			return
		}
		done <- results
	}()

	waitForMaxActive(t, func() int { return installer.currentMaxActive() }, 3)
	close(block)

	select {
	case err := <-errCh:
		t.Fatalf("update all packages: %v", err)
	case results := <-done:
		assert.Eq(t, []string{"fd", "fzf", "rg"}, []string{results[0].Name, results[1].Name, results[2].Name})
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for update all")
	}
	assert.Eq(t, 3, installer.currentMaxActive())
}
