package app

import (
	"sort"
	"sync"
	"testing"

	"github.com/gookit/goutil/x/assert"
	cfgpkg "github.com/inherelab/eget/internal/config"
	storepkg "github.com/inherelab/eget/internal/installed"
	"github.com/inherelab/eget/internal/util"
)

func TestListUpdateCandidatesIgnoresConfiguredPackageNames(t *testing.T) {
	svc := UpdateService{
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

	items, failures, checked, err := svc.ListUpdateCandidates()
	assert.NoErr(t, err)
	assert.Eq(t, 1, checked)
	assert.Eq(t, 0, len(failures))
	assert.Eq(t, 1, len(items))
	assert.Eq(t, "rg", items[0].Name)
}

func TestListUpdateCandidatesIncludesInstalledOnlyEntries(t *testing.T) {
	svc := UpdateService{
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfgpkg.NewFile(), nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{Installed: map[string]storepkg.Entry{
				"microsoft/apm": {Repo: "microsoft/apm", Tag: "v0.23.1"},
			}}, nil
		},
		LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
			assert.Eq(t, "microsoft/apm", target.Repo)
			return LatestInfo{Tag: "v0.24.0"}, nil
		},
	}

	items, failures, checked, err := svc.ListUpdateCandidates()

	assert.NoErr(t, err)
	assert.Eq(t, 1, checked)
	assert.Eq(t, 0, len(failures))
	assert.Eq(t, 1, len(items))
	assert.Eq(t, "apm", items[0].Name)
	assert.Eq(t, "v0.24.0", items[0].LatestTag)
}

func TestListUpdateCandidatesForTargetsChecksOnlyRequestedPackages(t *testing.T) {
	checkedRepos := make([]string, 0, 2)
	var checkedMu sync.Mutex
	svc := UpdateService{
		LoadConfig: func() (*cfgpkg.File, error) {
			cfg := cfgpkg.NewFile()
			cfg.Packages["jq"] = cfgpkg.Section{Repo: util.StringPtr("jqlang/jq")}
			cfg.Packages["markview"] = cfgpkg.Section{Repo: util.StringPtr("OXY2DEV/markview.nvim")}
			cfg.Packages["rg"] = cfgpkg.Section{Repo: util.StringPtr("BurntSushi/ripgrep")}
			return cfg, nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{Installed: map[string]storepkg.Entry{
				"jqlang/jq":             {Repo: "jqlang/jq", Tag: "jq-1.7"},
				"OXY2DEV/markview.nvim": {Repo: "OXY2DEV/markview.nvim", Tag: "v1.0.0"},
				"BurntSushi/ripgrep":    {Repo: "BurntSushi/ripgrep", Tag: "v13.0.0"},
			}}, nil
		},
		LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
			checkedMu.Lock()
			checkedRepos = append(checkedRepos, target.Repo)
			checkedMu.Unlock()
			switch target.Repo {
			case "jqlang/jq":
				return LatestInfo{Tag: "jq-1.8"}, nil
			case "OXY2DEV/markview.nvim":
				return LatestInfo{Tag: "v1.0.0"}, nil
			default:
				t.Fatalf("unexpected latest check for %s", target.Repo)
				return LatestInfo{}, nil
			}
		},
	}

	items, failures, checked, err := svc.ListUpdateCandidatesForTargets([]string{"markview", "jq"})
	assert.NoErr(t, err)
	assert.Eq(t, 2, checked)
	sort.Strings(checkedRepos)
	assert.Eq(t, []string{"OXY2DEV/markview.nvim", "jqlang/jq"}, checkedRepos)
	assert.Eq(t, 0, len(failures))
	assert.Eq(t, 1, len(items))
	assert.Eq(t, "jq", items[0].Name)
}

func TestListUpdateCandidatesChecksPkgTemplateTarget(t *testing.T) {
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
	svc := UpdateService{
		LoadConfig:    func() (*cfgpkg.File, error) { return cfg, nil },
		LoadInstalled: func() (*storepkg.Config, error) { return installed, nil },
		LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
			got = target
			return LatestInfo{Tag: "1.1.0"}, nil
		},
	}

	items, failures, checked, err := svc.ListUpdateCandidatesForTargets([]string{"markview"})

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

func TestListUpdateCandidatesPassesSourcePathToLatestChecker(t *testing.T) {
	svc := UpdateService{
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

	items, failures, checked, err := svc.ListUpdateCandidates()
	assert.NoErr(t, err)
	assert.Eq(t, 1, checked)
	assert.Eq(t, 0, len(failures))
	assert.Eq(t, 1, len(items))
	assert.Eq(t, "2.16.44", items[0].LatestTag)
}

func TestListUpdateCandidatesForTargetsPreservesInstalledTagChannel(t *testing.T) {
	svc := UpdateService{
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfgpkg.NewFile(), nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
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
		LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
			assert.Eq(t, "pkgforge/aeris", target.Repo)
			assert.Eq(t, "nightly", target.Tag)
			return LatestInfo{Tag: "nightly"}, nil
		},
	}

	items, failures, checked, err := svc.ListUpdateCandidatesForTargets([]string{"aeris"})

	assert.NoErr(t, err)
	assert.Eq(t, 0, len(failures))
	assert.Eq(t, 1, checked)
	assert.Eq(t, 0, len(items))
}

func TestListUpdateCandidatesChecksForgeRepo(t *testing.T) {
	svc := UpdateService{
		LoadConfig: func() (*cfgpkg.File, error) {
			cfg := cfgpkg.NewFile()
			cfg.Packages["fdroidserver"] = cfgpkg.Section{Repo: util.StringPtr("gitlab:gitlab.com/fdroid/fdroidserver")}
			return cfg, nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{Installed: map[string]storepkg.Entry{
				"gitlab:gitlab.com/fdroid/fdroidserver": {Repo: "gitlab:gitlab.com/fdroid/fdroidserver", Tag: "v2.3.3"},
			}}, nil
		},
		LatestInfo: func(target LatestCheckTarget) (LatestInfo, error) {
			repo, sourcePath := target.Repo, target.SourcePath
			if repo != "gitlab:gitlab.com/fdroid/fdroidserver" || sourcePath != "" {
				t.Fatalf("unexpected latest check repo=%q sourcePath=%q", repo, sourcePath)
			}
			return LatestInfo{Tag: "v2.3.4"}, nil
		},
	}

	items, failures, checked, err := svc.ListUpdateCandidates()

	assert.NoErr(t, err)
	assert.Eq(t, 1, checked)
	assert.Eq(t, 0, len(failures))
	assert.Eq(t, 1, len(items))
	assert.Eq(t, "v2.3.4", items[0].LatestTag)
}
