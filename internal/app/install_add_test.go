package app

import (
	"testing"

	"github.com/gookit/goutil/x/assert"
	cfgpkg "github.com/inherelab/eget/internal/config"
	"github.com/inherelab/eget/internal/install"
	"github.com/inherelab/eget/internal/util"
)

func TestInstallTargetWithAddRecordsManagedPackage(t *testing.T) {
	runner := &fakeRunner{
		result: RunResult{
			URL:            "https://github.com/junegunn/fzf/releases/download/v1.0.0/fzf.tar.gz",
			Tool:           "fzf",
			ExtractedFiles: []string{"./fzf"},
		},
	}
	store := &fakeInstalledStore{}
	config := &fakeConfigRecorder{}
	svc := Service{
		Runner: runner,
		Store:  store,
		Config: config,
	}

	opts := install.Options{
		Output:      "~/.local/bin",
		ExtractFile: "fzf",
		Asset:       []string{"linux"},
		Tag:         "v1.0.0",
		Prerelease:  true,
	}

	_, err := svc.InstallTarget("junegunn/fzf", opts, InstallExtras{AddToConfig: true, PackageOpts: opts})
	if err != nil {
		t.Fatalf("install target with add: %v", err)
	}

	if config.calls != 1 {
		t.Fatalf("expected config add to be called once, got %d", config.calls)
	}
	if config.repo != "junegunn/fzf" {
		t.Fatalf("expected config repo junegunn/fzf, got %q", config.repo)
	}
	if config.name != "fzf" {
		t.Fatalf("expected inferred package name fzf, got %q", config.name)
	}
	if runner.opts.Name != "fzf" {
		t.Fatalf("expected inferred install name fzf, got %q", runner.opts.Name)
	}
	if config.opts.ExtractFile != "fzf" {
		t.Fatalf("expected extract file to be forwarded, got %q", config.opts.ExtractFile)
	}
	if config.opts.Tag != "v1.0.0" {
		t.Fatalf("expected tag to be forwarded, got %q", config.opts.Tag)
	}
	if !config.opts.Prerelease {
		t.Fatalf("expected prerelease to be forwarded, got %#v", config.opts)
	}
	if len(config.opts.Asset) != 1 || config.opts.Asset[0] != "linux" {
		t.Fatalf("expected asset filter to be forwarded, got %#v", config.opts.Asset)
	}
}

func TestInstallTargetWithAddRecordsTagPolicy(t *testing.T) {
	runner := &fakeRunner{
		result: RunResult{
			URL:            "https://github.com/AandStep/ResultV/releases/download/v3.2.5/ResultV.AppImage",
			Tool:           "ResultV",
			ExtractedFiles: []string{"./ResultV.AppImage"},
		},
	}
	config := &fakeConfigRecorder{}
	svc := Service{Runner: runner, Config: config}
	opts := install.Options{Tag: "v3.2.5", TagPolicy: "tag"}

	_, err := svc.InstallTarget("AandStep/ResultV", opts, InstallExtras{AddToConfig: true, PackageOpts: opts})

	assert.NoErr(t, err)
	assert.Eq(t, "tag", config.opts.TagPolicy)
}

func TestInstallTargetWithAddRecordsForgePackage(t *testing.T) {
	runner := &fakeRunner{
		result: RunResult{
			URL:            "https://gitlab.com/fdroid/fdroidserver/-/releases/v2.3.3/downloads/fdroidserver-linux-amd64.tar.gz",
			Tool:           "fdroidserver",
			ExtractedFiles: []string{"./fdroidserver"},
		},
	}
	config := &fakeConfigRecorder{}
	svc := Service{
		Runner: runner,
		Config: config,
	}

	opts := install.Options{Tag: "v2.3.3", Asset: []string{"linux", "amd64"}}
	_, err := svc.InstallTarget("gitlab:fdroid/fdroidserver", opts, InstallExtras{
		AddToConfig: true,
		PackageOpts: opts,
	})
	if err != nil {
		t.Fatalf("install forge target with add: %v", err)
	}

	if config.calls != 1 {
		t.Fatalf("expected config add to be called once, got %d", config.calls)
	}
	if config.repo != "gitlab:fdroid/fdroidserver" {
		t.Fatalf("expected raw forge repo to be forwarded for config normalization, got %q", config.repo)
	}
	if config.opts.Tag != "v2.3.3" {
		t.Fatalf("expected tag to be forwarded, got %q", config.opts.Tag)
	}
	if len(config.opts.Asset) != 2 || config.opts.Asset[0] != "linux" || config.opts.Asset[1] != "amd64" {
		t.Fatalf("expected asset filters to be forwarded, got %#v", config.opts.Asset)
	}
}

func TestInstallTargetWithAddRecordsPkgTemplatePackage(t *testing.T) {
	runner := &fakeRunner{
		result: RunResult{
			URL:            "http://mydev.lan/tools/markview/markview-windows-amd64.exe",
			Tool:           "markview",
			ExtractedFiles: []string{"./markview.exe"},
		},
	}
	config := &fakeConfigRecorder{}
	cfg := cfgpkg.NewFile()
	cfg.PkgTemplates["mydev"] = cfgpkg.Section{
		LatestURL:   util.StringPtr("http://mydev.lan/tools/{name}/latest.yaml"),
		URLTemplate: util.StringPtr("http://mydev.lan/tools/{name}/{name}-{os}-{arch}{ext}"),
	}
	svc := Service{
		Runner: runner,
		Config: config,
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfg, nil
		},
	}

	_, err := svc.InstallTarget("mydev:markview", install.Options{}, InstallExtras{AddToConfig: true, PackageOpts: install.Options{}})

	assert.NoErr(t, err)
	assert.Eq(t, "pkg-template:mydev:markview", runner.target)
	assert.Eq(t, "pkg-template:mydev:markview", config.repo)
	assert.Eq(t, "markview", config.name)
	assert.Eq(t, "markview", config.opts.Name)
}

func TestInstallTargetWithAddPersistsConfirmedGUIInstaller(t *testing.T) {
	runner := &fakeRunner{
		result: RunResult{
			URL:         "https://github.com/sipeed/picoclaw/releases/download/v0.2.7/PicoClaw-Setup.exe",
			Asset:       "PicoClaw-Setup.exe",
			IsGUI:       true,
			InstallMode: install.InstallModeInstaller,
		},
	}
	config := &fakeConfigRecorder{}
	svc := Service{
		Runner: runner,
		Config: config,
	}

	opts := install.Options{}
	_, err := svc.InstallTarget("sipeed/picoclaw", opts, InstallExtras{AddToConfig: true, PackageOpts: opts})
	if err != nil {
		t.Fatalf("install target with add: %v", err)
	}

	if config.calls != 1 {
		t.Fatalf("expected config add to be called once, got %d", config.calls)
	}
	if !config.opts.IsGUI {
		t.Fatalf("expected confirmed installer to persist IsGUI=true, got %#v", config.opts)
	}
}

func TestInstallTargetWithAddRejectsNonRepoTarget(t *testing.T) {
	runner := &fakeRunner{
		result: RunResult{
			URL:            "https://example.com/tool.tar.gz",
			ExtractedFiles: []string{"./tool"},
		},
	}
	config := &fakeConfigRecorder{}
	svc := Service{
		Runner: runner,
		Config: config,
	}

	_, err := svc.InstallTarget("https://example.com/tool.tar.gz", install.Options{}, InstallExtras{AddToConfig: true})
	if err == nil {
		t.Fatal("expected install --add with non-repo target to fail")
	}
	if config.calls != 0 {
		t.Fatalf("expected config add to not be called, got %d", config.calls)
	}
}

func TestInstallTargetWithAddUsesExplicitPackageName(t *testing.T) {
	runner := &fakeRunner{
		result: RunResult{
			URL:            "https://github.com/gookit/gitw/releases/download/v0.3.6/chlog-windows-amd64.exe",
			ExtractedFiles: []string{"./chlog.exe"},
		},
	}
	config := &fakeConfigRecorder{}
	svc := Service{
		Runner: runner,
		Config: config,
	}

	opts := install.Options{Name: "chlog"}
	_, err := svc.InstallTarget("gookit/gitw", opts, InstallExtras{
		AddToConfig: true,
		PackageName: "chlog",
		PackageOpts: opts,
	})
	if err != nil {
		t.Fatalf("install target with explicit package name: %v", err)
	}

	if config.name != "chlog" {
		t.Fatalf("expected explicit package name chlog, got %q", config.name)
	}
}

func TestInstallTargetWithAddInfersPackageNameBeforeInstall(t *testing.T) {
	runner := &fakeRunner{
		result: RunResult{
			URL:            "https://github.com/alacritty/alacritty/releases/download/v0.17.0/Alacritty-v0.17.0-portable.exe",
			ExtractedFiles: []string{"./alacritty.exe"},
		},
	}
	config := &fakeConfigRecorder{}
	svc := Service{
		Runner: runner,
		Config: config,
	}

	_, err := svc.InstallTarget("alacritty/alacritty", install.Options{}, InstallExtras{
		AddToConfig: true,
		PackageOpts: install.Options{},
	})
	if err != nil {
		t.Fatalf("install target with inferred package name: %v", err)
	}

	assert.Eq(t, "alacritty", runner.opts.Name)
	assert.Eq(t, "alacritty", config.name)
	assert.Eq(t, "alacritty", config.opts.Name)
}
