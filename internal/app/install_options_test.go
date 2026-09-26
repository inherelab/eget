package app

import (
	"path/filepath"
	"testing"

	"github.com/gookit/goutil/x/assert"
	cfgpkg "github.com/inherelab/eget/internal/config"
	"github.com/inherelab/eget/internal/install"
)

func TestExtractOptionsMapOmitsGuiTargetForCLIInstall(t *testing.T) {
	got := extractOptionsMap(install.Options{
		Output:    "C:/Users/inhere/.local/bin",
		GuiTarget: "D:/Program/Tools",
	}, false)

	if _, ok := got["gui_target"]; ok {
		t.Fatalf("expected non-GUI install options to omit gui_target, got %#v", got)
	}
	assert.Eq(t, "C:/Users/inhere/.local/bin", got["output"])
}

func TestExtractOptionsMapKeepsGuiTargetForGUIInstall(t *testing.T) {
	got := extractOptionsMap(install.Options{
		Output:      "C:/Users/inhere/.local/bin",
		GuiTarget:   "D:/Program/Tools",
		InstallMode: install.InstallModeInstaller,
	}, true)

	assert.Eq(t, "D:/Program/Tools", got["gui_target"])
	assert.Eq(t, true, got["is_gui"])
	assert.Eq(t, install.InstallModeInstaller, got["install_mode"])
}

func TestExtractOptionsMapKeepsStripComponents(t *testing.T) {
	got := extractOptionsMap(install.Options{StripComponents: 1}, false)

	assert.Eq(t, 1, got["strip_components"])
}

func TestResolveInstallOptionsPassesGlobalSys7zPath(t *testing.T) {
	cfg := cfgpkg.NewFile()
	path := "~/bin/7z"
	cfg.Global.Sys7zPath = &path

	svc := Service{
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfg, nil
		},
	}

	opts, err := svc.resolveInstallOptions("owner/repo", install.Options{}, false)
	if err != nil {
		t.Fatalf("resolve install options: %v", err)
	}

	assert.Contains(t, filepath.ToSlash(opts.Sys7zPath), "bin/7z")
}

func TestResolveInstallOptionsKeepsCallerOnlyInstallerFlags(t *testing.T) {
	cfg := cfgpkg.NewFile()
	svc := Service{
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfg, nil
		},
	}

	// Silent and DeferInstaller are the caller's decision, not the config's: they
	// must survive the merge or the console (and --silent) silently lose them.
	opts, err := svc.resolveInstallOptions("owner/repo", install.Options{Silent: true, DeferInstaller: true}, false)
	if err != nil {
		t.Fatalf("resolve install options: %v", err)
	}

	assert.Eq(t, true, opts.Silent)
	assert.Eq(t, true, opts.DeferInstaller)
}

func TestResolveInstallOptionsUsesGlobalUserAgent(t *testing.T) {
	cfg := cfgpkg.NewFile()
	userAgent := "custom-agent/1.0"
	cfg.Global.UserAgent = &userAgent

	svc := Service{
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfg, nil
		},
	}

	opts, err := svc.resolveInstallOptions("owner/repo", install.Options{}, false)
	if err != nil {
		t.Fatalf("resolve install options: %v", err)
	}

	assert.Eq(t, "custom-agent/1.0", opts.UserAgent)
}
