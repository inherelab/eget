package app

import (
	"testing"

	cfgpkg "github.com/inherelab/eget/internal/config"
	"github.com/inherelab/eget/internal/install"
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

func TestUpdatePackageUsesManagedPackageConfig(t *testing.T) {
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

	if _, err := svc.UpdatePackage("fzf", install.Options{}); err != nil {
		t.Fatalf("update package: %v", err)
	}

	if len(installer.targets) != 1 || installer.targets[0] != "junegunn/fzf" {
		t.Fatalf("expected installer to use managed repo, got %#v", installer.targets)
	}
	if installer.options[0].Output != "~/.local/bin" {
		t.Fatalf("expected package target to be merged, got %#v", installer.options[0].Output)
	}
	if installer.options[0].Tag != "nightly" {
		t.Fatalf("expected package tag to be merged, got %#v", installer.options[0].Tag)
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

func TestUpdateAllPackagesIteratesManagedPackages(t *testing.T) {
	installer := &fakeInstallService{}
	svc := UpdateService{
		Install: installer,
		LoadConfig: func() (*cfgpkg.File, error) {
			cfg := cfgpkg.NewFile()
			cfg.Packages["fzf"] = cfgpkg.Section{Repo: util.StringPtr("junegunn/fzf")}
			cfg.Packages["rg"] = cfgpkg.Section{Repo: util.StringPtr("BurntSushi/ripgrep")}
			return cfg, nil
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
