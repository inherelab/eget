package app

import (
	"testing"
	"time"

	"github.com/gookit/goutil/x/assert"
	cfgpkg "github.com/inherelab/eget/internal/config"
	storepkg "github.com/inherelab/eget/internal/installed"
	"github.com/inherelab/eget/internal/util"
)

func TestShowPackageMergesConfiguredAndInstalledDetails(t *testing.T) {
	installedAt := time.Date(2026, 5, 17, 9, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)
	svc := ShowService{
		LoadConfig: func() (*cfgpkg.File, error) {
			cfg := cfgpkg.NewFile()
			cfg.Packages["fzf"] = cfgpkg.Section{
				Repo:   util.StringPtr("junegunn/fzf"),
				Desc:   util.StringPtr("Manual fzf description"),
				Target: util.StringPtr("~/.local/bin"),
			}
			return cfg, nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{Installed: map[string]storepkg.Entry{
				"junegunn/fzf": {
					Repo:           "junegunn/fzf",
					Target:         "junegunn/fzf",
					InstalledAt:    installedAt,
					URL:            "https://github.com/junegunn/fzf/releases/download/v0.50.0/fzf.tar.gz",
					Asset:          "fzf.tar.gz",
					ExtractedFiles: []string{"/usr/local/bin/fzf"},
					Tag:            "v0.50.0",
					Version:        "v0.50.0",
					UpdatedAt:      updatedAt,
					Desc:           "Repository fzf description",
					Homepage:       "https://junegunn.github.io/fzf",
					RepoURL:        "https://github.com/junegunn/fzf",
				},
			}}, nil
		},
	}

	got, err := svc.ShowPackage("fzf")
	if err != nil {
		t.Fatalf("ShowPackage: %v", err)
	}

	assert.Eq(t, "fzf", got.Name)
	assert.Eq(t, "junegunn/fzf", got.Repo)
	assert.Eq(t, "Manual fzf description", got.Desc)
	assert.Eq(t, "https://junegunn.github.io/fzf", got.Homepage)
	assert.Eq(t, "https://github.com/junegunn/fzf", got.RepoURL)
	assert.True(t, got.Configured)
	assert.True(t, got.Installed)
	assert.Eq(t, "v0.50.0", got.Version)
	assert.Eq(t, updatedAt, got.UpdatedAt)
	assert.Eq(t, []string{"/usr/local/bin/fzf"}, got.ExtractedFiles)
}

func TestShowPackageSupportsInstalledOnlyPackageAndInfersHomepage(t *testing.T) {
	svc := ShowService{
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfgpkg.NewFile(), nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{Installed: map[string]storepkg.Entry{
				"sourceforge:winmerge": {
					Repo:    "sourceforge:winmerge",
					Target:  "sourceforge:winmerge/stable",
					Version: "2.16.44",
					Desc:    "Windows visual diff and merge tool",
				},
			}}, nil
		},
	}

	got, err := svc.ShowPackage("sourceforge:winmerge")
	if err != nil {
		t.Fatalf("ShowPackage installed-only: %v", err)
	}

	assert.Eq(t, "winmerge", got.Name)
	assert.Eq(t, "sourceforge:winmerge", got.Repo)
	assert.Eq(t, "Windows visual diff and merge tool", got.Desc)
	assert.Eq(t, "https://sourceforge.net/projects/winmerge/", got.Homepage)
	assert.Eq(t, "https://sourceforge.net/projects/winmerge/", got.RepoURL)
	assert.False(t, got.Configured)
	assert.True(t, got.Installed)
	assert.Eq(t, "2.16.44", got.Version)
}

func TestShowPackageUsesInstalledAliasName(t *testing.T) {
	svc := ShowService{
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfgpkg.NewFile(), nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{Installed: map[string]storepkg.Entry{
				"gbench": {
					Repo:   "gookit/greq",
					Target: "gookit/greq",
					Tool:   "greq",
					Asset:  "gbench-v0.6.0-windows-amd64.zip",
				},
			}}, nil
		},
	}

	got, err := svc.ShowPackage("gbench")
	if err != nil {
		t.Fatalf("ShowPackage installed alias: %v", err)
	}

	assert.Eq(t, "gbench", got.Name)
	assert.Eq(t, "gookit/greq", got.Repo)
	assert.Eq(t, "greq", got.Tool)
	assert.True(t, got.Installed)
	assert.False(t, got.Configured)
}

func TestShowPackageFindsLegacyRepoKeyByRepoName(t *testing.T) {
	svc := ShowService{
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfgpkg.NewFile(), nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{Installed: map[string]storepkg.Entry{
				"tinyhumansai/openhuman": {
					Repo:        "tinyhumansai/openhuman",
					Target:      "tinyhumansai/openhuman",
					InstallMode: "installer",
					IsGUI:       true,
				},
			}}, nil
		},
	}

	got, err := svc.ShowPackage("openhuman")
	if err != nil {
		t.Fatalf("ShowPackage legacy repo key by name: %v", err)
	}

	assert.Eq(t, "openhuman", got.Name)
	assert.Eq(t, "tinyhumansai/openhuman", got.Repo)
	assert.True(t, got.Installed)
	assert.True(t, got.IsGUI)
}

func TestShowPackageDisplaysPkgTemplateRepo(t *testing.T) {
	cfg := cfgpkg.NewFile()
	cfg.Packages["markview"] = cfgpkg.Section{
		Repo: util.StringPtr("pkg-template:mydev:markview"),
		Desc: util.StringPtr("Markdown preview"),
	}
	svc := ShowService{
		LoadConfig: func() (*cfgpkg.File, error) { return cfg, nil },
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{Installed: map[string]storepkg.Entry{}}, nil
		},
	}

	got, err := svc.ShowPackage("markview")

	assert.NoErr(t, err)
	assert.Eq(t, "markview", got.Name)
	assert.Eq(t, "pkg-template:mydev:markview", got.Repo)
	assert.Eq(t, "Markdown preview", got.Desc)
	assert.Eq(t, "", got.RepoURL)
}

func TestShowPackageUsesConfiguredGUIFields(t *testing.T) {
	installMode := "portable"
	isGUI := true
	cfg := cfgpkg.NewFile()
	cfg.Packages["v2rayn"] = cfgpkg.Section{
		Repo:        util.StringPtr("2dust/v2rayN"),
		IsGUI:       &isGUI,
		InstallMode: &installMode,
	}
	svc := ShowService{
		LoadConfig: func() (*cfgpkg.File, error) { return cfg, nil },
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{Installed: map[string]storepkg.Entry{}}, nil
		},
	}

	got, err := svc.ShowPackage("v2rayn")

	assert.NoErr(t, err)
	assert.True(t, got.IsGUI)
	assert.Eq(t, "portable", got.InstallMode)
}

func TestShowPackageReportsTagPolicy(t *testing.T) {
	store := &storepkg.Config{Installed: map[string]storepkg.Entry{
		"trycua/cua": {
			Repo:    "trycua/cua",
			Target:  "trycua/cua",
			Tag:     "PRE:cua-driver-rs-v",
			Options: map[string]any{"tag": "PRE:cua-driver-rs-v", "tag_policy": "pattern"},
		},
	}}
	svc := ShowService{
		LoadConfig:    func() (*cfgpkg.File, error) { return cfgpkg.NewFile(), nil },
		LoadInstalled: func() (*storepkg.Config, error) { return store, nil },
	}

	got, err := svc.ShowPackage("trycua/cua")
	assert.NoErr(t, err)
	assert.Eq(t, "pattern", got.TagPolicy)
	assert.Eq(t, "PRE:cua-driver-rs-v", got.Tag)
}

func TestShowPackageReturnsErrorForMissingTarget(t *testing.T) {
	svc := ShowService{
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfgpkg.NewFile(), nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{Installed: map[string]storepkg.Entry{}}, nil
		},
	}

	_, err := svc.ShowPackage("missing")
	if err == nil {
		t.Fatal("expected missing package error")
	}
}
