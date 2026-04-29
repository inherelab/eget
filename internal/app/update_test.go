package app

import (
	"testing"
	"time"

	"github.com/gookit/goutil/testutil/assert"
	cfgpkg "github.com/inherelab/eget/internal/config"
	"github.com/inherelab/eget/internal/install"
	storepkg "github.com/inherelab/eget/internal/installed"
	"github.com/inherelab/eget/internal/util"
)

type fakeInstallService struct {
	targets []string
	options []install.Options
	result  RunResult
	err     error
}

func (f *fakeInstallService) InstallTarget(target string, opts install.Options, extras ...InstallExtras) (RunResult, error) {
	f.targets = append(f.targets, target)
	f.options = append(f.options, opts)
	return f.result, f.err
}

func TestUpdatePackageDelegatesManagedPackageNameWithRawCLIOptions(t *testing.T) {
	installer := &fakeInstallService{}
	svc := UpdateService{
		Install: installer,
		LoadConfig: func() (*cfgpkg.File, error) {
			cfg := cfgpkg.NewFile()
			cfg.Global.Target = util.StringPtr("~/bin")
			cfg.Packages["fzf"] = cfgpkg.Section{
				Repo:   util.StringPtr("junegunn/fzf"),
				Target: util.StringPtr("~/.local/bin"),
				System: util.StringPtr("linux/amd64"),
				Tag:    util.StringPtr("nightly"),
			}
			return cfg, nil
		},
	}

	cli := install.Options{Tag: "v1.0.0", Quiet: true}
	if _, err := svc.UpdatePackage("fzf", cli); err != nil {
		t.Fatalf("update package: %v", err)
	}

	if len(installer.targets) != 1 || installer.targets[0] != "fzf" {
		t.Fatalf("expected installer to resolve managed package name, got %#v", installer.targets)
	}
	if installer.options[0].Output != "" {
		t.Fatalf("expected update service to leave config merging to installer, got output %q", installer.options[0].Output)
	}
	if installer.options[0].Tag != "v1.0.0" || !installer.options[0].Quiet {
		t.Fatalf("expected raw cli options to pass through, got %#v", installer.options[0])
	}
}

func TestUpdatePackageAllowsDirectRepo(t *testing.T) {
	installer := &fakeInstallService{}
	svc := UpdateService{
		Install: installer,
		LoadConfig: func() (*cfgpkg.File, error) {
			cfg := cfgpkg.NewFile()
			cfg.Global.Target = util.StringPtr("~/bin")
			cfg.Repos["junegunn/fzf"] = cfgpkg.Section{System: util.StringPtr("linux/amd64")}
			return cfg, nil
		},
	}

	if _, err := svc.UpdatePackage("junegunn/fzf", install.Options{Tag: "v1.0.0"}); err != nil {
		t.Fatalf("update direct repo: %v", err)
	}

	if len(installer.targets) != 1 || installer.targets[0] != "junegunn/fzf" {
		t.Fatalf("expected installer to use direct repo, got %#v", installer.targets)
	}
	if installer.options[0].Tag != "v1.0.0" {
		t.Fatalf("expected cli tag to win, got %#v", installer.options[0].Tag)
	}
}

func TestUpdatePackageWithAppInstallerKeepsManagedConfigMerge(t *testing.T) {
	cfg := mustLoadFromString(t, `
[global]
target = "~/.local/bin"

["junegunn/fzf"]
system = "linux/amd64"

[packages.fzf]
repo = "junegunn/fzf"
target = "D:/Tools/fzf"
tag = "nightly"
asset_filters = ["linux"]
`)
	runner := &fakeRunner{
		result: RunResult{
			URL:            "https://github.com/junegunn/fzf/releases/download/nightly/fzf.tar.gz",
			Tool:           "fzf",
			ExtractedFiles: []string{"./fzf"},
		},
	}
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
	}

	if _, err := updateSvc.UpdatePackage("fzf", install.Options{}); err != nil {
		t.Fatalf("update package: %v", err)
	}

	if runner.target != "junegunn/fzf" {
		t.Fatalf("expected installer to resolve repo target, got %q", runner.target)
	}
	if runner.opts.Output != "D:/Tools/fzf" {
		t.Fatalf("expected package target to be merged by installer, got %q", runner.opts.Output)
	}
	if runner.opts.System != "linux/amd64" {
		t.Fatalf("expected repo system to be merged by installer, got %q", runner.opts.System)
	}
	if runner.opts.Tag != "nightly" {
		t.Fatalf("expected package tag to be merged by installer, got %q", runner.opts.Tag)
	}
	if len(runner.opts.Asset) != 1 || runner.opts.Asset[0] != "linux" {
		t.Fatalf("expected package asset filters to be merged by installer, got %#v", runner.opts.Asset)
	}
}

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
		LatestTag: func(repo string) (string, error) {
			switch repo {
			case "junegunn/fzf":
				return "v0.51.0", nil
			case "BurntSushi/ripgrep":
				return "v14.0.0", nil
			default:
				return "", nil
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
		LatestTag: func(repo string) (string, error) {
			switch repo {
			case "junegunn/fzf":
				return "v0.50.0", nil
			case "BurntSushi/ripgrep":
				return "v14.0.0", nil
			default:
				t.Fatalf("unexpected latest tag check for %s", repo)
				return "", nil
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
