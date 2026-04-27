package app

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	cfgpkg "github.com/inherelab/eget/internal/config"
	"github.com/inherelab/eget/internal/install"
	storepkg "github.com/inherelab/eget/internal/installed"
	"github.com/inherelab/eget/internal/util"
)

type fakeRunner struct {
	target string
	opts   install.Options
	result RunResult
	err    error
	calls  int
}

func (f *fakeRunner) Run(target string, opts install.Options) (RunResult, error) {
	f.calls++
	f.target = target
	f.opts = opts
	return f.result, f.err
}

type fakeInstalledStore struct {
	target string
	entry  storepkg.Entry
	err    error
	calls  int
}

func (f *fakeInstalledStore) Record(target string, entry storepkg.Entry) error {
	f.calls++
	f.target = target
	f.entry = entry
	return f.err
}

type fakeConfigRecorder struct {
	repo  string
	name  string
	opts  install.Options
	err   error
	calls int
}

func (f *fakeConfigRecorder) AddPackage(repo, name string, opts install.Options) error {
	f.calls++
	f.repo = repo
	f.name = name
	f.opts = opts
	return f.err
}

func TestInstallTargetRunsInstallFlowAndRecordsInstalledState(t *testing.T) {
	now := time.Unix(1710000000, 0).UTC()
	runner := &fakeRunner{
		result: RunResult{
			URL:            "https://github.com/junegunn/fzf/releases/download/v1.0.0/fzf.tar.gz",
			Tool:           "fzf",
			ExtractedFiles: []string{"./fzf"},
		},
	}
	store := &fakeInstalledStore{}
	svc := Service{
		Runner: runner,
		Store:  store,
		Now: func() time.Time {
			return now
		},
		ReleaseInfo: func(repo, url string) (string, time.Time, error) {
			if repo != "junegunn/fzf" {
				t.Fatalf("expected repo junegunn/fzf, got %q", repo)
			}
			return "v1.0.0", now.Add(-time.Hour), nil
		},
	}

	opts := install.Options{
		System:      "linux/amd64",
		Output:      "~/.local/bin",
		ExtractFile: "fzf",
		Asset:       []string{"linux"},
		Tag:         "v1.0.0",
		Verify:      "abc123",
		Source:      true,
		DisableSSL:  true,
		All:         true,
	}

	result, err := svc.InstallTarget("junegunn/fzf", opts)
	if err != nil {
		t.Fatalf("install target: %v", err)
	}

	if runner.calls != 1 {
		t.Fatalf("expected runner to be called once, got %d", runner.calls)
	}
	if runner.target != "junegunn/fzf" {
		t.Fatalf("expected target junegunn/fzf, got %q", runner.target)
	}
	if store.calls != 1 {
		t.Fatalf("expected store to be called once, got %d", store.calls)
	}
	if store.target != "junegunn/fzf" {
		t.Fatalf("expected store target junegunn/fzf, got %q", store.target)
	}
	if store.entry.Tool != "fzf" {
		t.Fatalf("expected tool fzf, got %q", store.entry.Tool)
	}
	if store.entry.Tag != "v1.0.0" {
		t.Fatalf("expected tag v1.0.0, got %q", store.entry.Tag)
	}
	if store.entry.InstalledAt != now {
		t.Fatalf("expected installed at %v, got %v", now, store.entry.InstalledAt)
	}
	if got := store.entry.Options["system"]; got != "linux/amd64" {
		t.Fatalf("expected system option to be recorded, got %#v", got)
	}
	if got := store.entry.Options["download_source"]; got != true {
		t.Fatalf("expected source option to be recorded, got %#v", got)
	}
	if len(result.ExtractedFiles) != 1 || result.ExtractedFiles[0] != "./fzf" {
		t.Fatalf("expected extracted files to round-trip, got %#v", result.ExtractedFiles)
	}
}

func TestDownloadTargetRunsWithoutRecordingInstalledState(t *testing.T) {
	runner := &fakeRunner{
		result: RunResult{
			URL:            "https://go.dev/dl/go1.22.0.linux-amd64.tar.gz",
			ExtractedFiles: []string{"./go1.22.0.tar.gz"},
		},
	}
	store := &fakeInstalledStore{}
	svc := Service{Runner: runner, Store: store}

	opts := install.Options{System: "linux/amd64"}
	result, err := svc.DownloadTarget("https://go.dev/dl/go1.22.0.linux-amd64.tar.gz", opts)
	if err != nil {
		t.Fatalf("download target: %v", err)
	}

	if runner.calls != 1 {
		t.Fatalf("expected runner to be called once, got %d", runner.calls)
	}
	if !runner.opts.DownloadOnly {
		t.Fatalf("expected download target to force DownloadOnly=true")
	}
	if store.calls != 0 {
		t.Fatalf("expected store to not be called, got %d", store.calls)
	}
	if result.URL == "" {
		t.Fatal("expected result URL to be preserved")
	}
}

func TestDownloadTargetWithExtractFileRunsExtractionFlow(t *testing.T) {
	runner := &fakeRunner{
		result: RunResult{
			URL:            "https://example.com/tool.tar.gz",
			ExtractedFiles: []string{"./tool"},
		},
	}
	store := &fakeInstalledStore{}
	svc := Service{Runner: runner, Store: store}

	_, err := svc.DownloadTarget("https://example.com/tool.tar.gz", install.Options{ExtractFile: "tool"})
	if err != nil {
		t.Fatalf("download target: %v", err)
	}

	if runner.calls != 1 {
		t.Fatalf("expected runner to be called once, got %d", runner.calls)
	}
	if runner.opts.DownloadOnly {
		t.Fatal("expected download target with --file to disable DownloadOnly")
	}
	if store.calls != 0 {
		t.Fatalf("expected store to not be called, got %d", store.calls)
	}
}

func TestDownloadTargetWithGlobExtractFileEnablesExtractAll(t *testing.T) {
	runner := &fakeRunner{
		result: RunResult{
			URL:            "https://example.com/tool.zip",
			ExtractedFiles: []string{"./picoclaw.exe", "./picoclaw-launcher.exe"},
		},
	}
	svc := Service{Runner: runner}

	_, err := svc.DownloadTarget("https://example.com/tool.zip", install.Options{ExtractFile: "*.exe"})
	if err != nil {
		t.Fatalf("download target: %v", err)
	}

	if !runner.opts.All {
		t.Fatal("expected glob extract file to enable extract-all mode")
	}
	if runner.opts.DownloadOnly {
		t.Fatal("expected glob extract file to disable DownloadOnly")
	}
}

func TestDownloadTargetWithAllRunsExtractionFlow(t *testing.T) {
	runner := &fakeRunner{
		result: RunResult{
			URL:            "https://example.com/tool.zip",
			ExtractedFiles: []string{"./bin/tool.exe"},
		},
	}
	store := &fakeInstalledStore{}
	svc := Service{Runner: runner, Store: store}

	_, err := svc.DownloadTarget("https://example.com/tool.zip", install.Options{All: true})
	if err != nil {
		t.Fatalf("download target: %v", err)
	}

	if runner.calls != 1 {
		t.Fatalf("expected runner to be called once, got %d", runner.calls)
	}
	if runner.opts.DownloadOnly {
		t.Fatal("expected download target with extract-all to disable DownloadOnly")
	}
	if store.calls != 0 {
		t.Fatalf("expected store to not be called, got %d", store.calls)
	}
}

func TestInstallTargetUsesGuiTargetForPortableGUI(t *testing.T) {
	runner := &fakeRunner{
		result: RunResult{
			URL:            "https://github.com/sipeed/picoclaw/releases/download/v0.2.7/picoclaw.exe",
			Asset:          "picoclaw.exe",
			ExtractedFiles: []string{"picoclaw.exe"},
		},
	}
	svc := Service{
		Runner: runner,
		LoadConfig: func() (*cfgpkg.File, error) {
			cfg := cfgpkg.NewFile()
			target := "~/bin"
			guiTarget := "~/Applications"
			repo := "sipeed/picoclaw"
			isGUI := true
			cfg.Global.Target = &target
			cfg.Global.GuiTarget = &guiTarget
			cfg.Packages["picoclaw"] = cfgpkg.Section{Repo: &repo, IsGUI: &isGUI}
			return cfg, nil
		},
	}

	_, err := svc.InstallTarget("picoclaw", install.Options{})
	if err != nil {
		t.Fatalf("install gui package: %v", err)
	}
	if !runner.opts.IsGUI {
		t.Fatalf("expected IsGUI option, got %#v", runner.opts)
	}
	if runner.opts.GuiTarget == "" || !strings.Contains(runner.opts.GuiTarget, "Applications") {
		t.Fatalf("expected expanded GuiTarget, got %#v", runner.opts.GuiTarget)
	}
	if runner.opts.OutputExplicit {
		t.Fatalf("expected OutputExplicit false without --to")
	}
}

func TestInstallTargetKeepsExplicitOutputForGUI(t *testing.T) {
	runner := &fakeRunner{
		result: RunResult{
			URL:            "https://github.com/sipeed/picoclaw/releases/download/v0.2.7/picoclaw.exe",
			Asset:          "picoclaw.exe",
			ExtractedFiles: []string{"D:/Apps/PicoClaw/picoclaw.exe"},
		},
	}
	svc := Service{
		Runner: runner,
		LoadConfig: func() (*cfgpkg.File, error) {
			cfg := cfgpkg.NewFile()
			target := "~/bin"
			guiTarget := "~/Applications"
			cfg.Global.Target = &target
			cfg.Global.GuiTarget = &guiTarget
			return cfg, nil
		},
	}

	_, err := svc.InstallTarget("sipeed/picoclaw", install.Options{IsGUI: true, Output: "D:/Apps/PicoClaw"})
	if err != nil {
		t.Fatalf("install gui package with output: %v", err)
	}
	if !runner.opts.OutputExplicit {
		t.Fatalf("expected OutputExplicit true when --to is provided")
	}
	if runner.opts.Output != "D:/Apps/PicoClaw" {
		t.Fatalf("expected explicit output to win, got %#v", runner.opts.Output)
	}
}

func TestInstallTargetRecordsGUIInstallerWithoutExtractedFiles(t *testing.T) {
	runner := &fakeRunner{
		result: RunResult{
			URL:           "https://github.com/sipeed/picoclaw/releases/download/v0.2.7/picoclaw-setup.exe",
			Asset:         "picoclaw-setup.exe",
			IsGUI:         true,
			InstallMode:   install.InstallModeInstaller,
			InstallerFile: "C:/Temp/picoclaw-setup.exe",
		},
	}
	store := &fakeInstalledStore{}
	svc := Service{
		Runner: runner,
		Store:  store,
		Now:    func() time.Time { return time.Unix(1710000000, 0).UTC() },
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfgpkg.NewFile(), nil
		},
	}

	_, err := svc.InstallTarget("sipeed/picoclaw", install.Options{IsGUI: true})
	if err != nil {
		t.Fatalf("install gui installer: %v", err)
	}
	if store.calls != 1 {
		t.Fatalf("expected installer install to be recorded, got %d calls", store.calls)
	}
	if !store.entry.IsGUI || store.entry.InstallMode != install.InstallModeInstaller {
		t.Fatalf("expected gui installer metadata, got %#v", store.entry)
	}
	if len(store.entry.ExtractedFiles) != 0 {
		t.Fatalf("expected no extracted files for installer, got %#v", store.entry.ExtractedFiles)
	}
}

func TestInstallTargetUsesConfiguredDefaults(t *testing.T) {
	cfg := mustLoadFromString(t, `
[global]
target = "~/.local/bin"
cache_dir = "~/.cache/eget"
proxy_url = "http://127.0.0.1:7890"
`)
	runner := &fakeRunner{
		result: RunResult{
			URL:            "https://example.com/tool.tar.gz",
			ExtractedFiles: []string{"./tool"},
		},
	}
	svc := Service{
		Runner: runner,
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfg, nil
		},
	}

	_, err := svc.InstallTarget("junegunn/fzf", install.Options{})
	if err != nil {
		t.Fatalf("install target: %v", err)
	}

	expectedTarget, err := util.Expand("~/.local/bin")
	if err != nil {
		t.Fatalf("expand target: %v", err)
	}
	expectedCache, err := util.Expand("~/.cache/eget")
	if err != nil {
		t.Fatalf("expand cache: %v", err)
	}

	if runner.opts.Output != expectedTarget {
		t.Fatalf("expected configured install target, got %q", runner.opts.Output)
	}
	if runner.opts.CacheDir != expectedCache {
		t.Fatalf("expected configured cache dir, got %q", runner.opts.CacheDir)
	}
	if runner.opts.ProxyURL != "http://127.0.0.1:7890" {
		t.Fatalf("expected configured proxy url, got %q", runner.opts.ProxyURL)
	}
	if expected := filepath.Join(expectedCache, "api-cache"); runner.opts.APICacheDir != expected {
		t.Fatalf("expected derived api cache dir, got %q", runner.opts.APICacheDir)
	}
}

func TestInstallTargetResolvesManagedPackageName(t *testing.T) {
	cfg := mustLoadFromString(t, `
[global]
target = "~/.local/bin"

["sipeed/picoclaw"]
system = "windows/amd64"

[packages.picoclaw]
repo = "sipeed/picoclaw"
target = "D:/Program/AITools/PicoClaw"
tag = "v1.2.3"
file = "*.exe"
asset_filters = ["windows"]
`)
	runner := &fakeRunner{
		result: RunResult{
			URL:            "https://github.com/sipeed/picoclaw/releases/download/v1.2.3/picoclaw.zip",
			ExtractedFiles: []string{"./picoclaw.exe"},
		},
	}
	store := &fakeInstalledStore{}
	svc := Service{
		Runner: runner,
		Store:  store,
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfg, nil
		},
	}

	_, err := svc.InstallTarget("picoclaw", install.Options{})
	if err != nil {
		t.Fatalf("install target: %v", err)
	}

	if runner.target != "sipeed/picoclaw" {
		t.Fatalf("expected managed package to resolve repo, got %q", runner.target)
	}
	if runner.opts.Output != "D:/Program/AITools/PicoClaw" {
		t.Fatalf("expected package target to be used, got %q", runner.opts.Output)
	}
	if runner.opts.System != "windows/amd64" {
		t.Fatalf("expected repo system to be merged, got %q", runner.opts.System)
	}
	if runner.opts.Tag != "v1.2.3" {
		t.Fatalf("expected package tag to be merged, got %q", runner.opts.Tag)
	}
	if runner.opts.ExtractFile != "*.exe" {
		t.Fatalf("expected package file glob to be merged, got %q", runner.opts.ExtractFile)
	}
	if !runner.opts.All {
		t.Fatal("expected file glob to enable extract-all mode")
	}
	if len(runner.opts.Asset) != 1 || runner.opts.Asset[0] != "windows" {
		t.Fatalf("expected package asset filter to be merged, got %#v", runner.opts.Asset)
	}
	if store.target != "sipeed/picoclaw" {
		t.Fatalf("expected installed store to record real repo target, got %q", store.target)
	}
	if store.entry.Repo != "sipeed/picoclaw" {
		t.Fatalf("expected installed repo sipeed/picoclaw, got %q", store.entry.Repo)
	}
	if store.entry.Target != "sipeed/picoclaw" {
		t.Fatalf("expected installed target to be real repo, got %q", store.entry.Target)
	}
}

func TestInstallTargetRejectsManagedPackageWithoutRepo(t *testing.T) {
	cfg := mustLoadFromString(t, `
[packages.picoclaw]
target = "D:/Program/AITools/PicoClaw"
`)
	svc := Service{
		Runner: &fakeRunner{},
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfg, nil
		},
	}

	_, err := svc.InstallTarget("picoclaw", install.Options{})
	if err == nil {
		t.Fatal("expected install target to fail when package repo is missing")
	}
	if err.Error() != `package "picoclaw" has no repo` {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDownloadTargetUsesConfiguredCacheDirByDefault(t *testing.T) {
	cfg := mustLoadFromString(t, `
[global]
target = "~/.local/bin"
cache_dir = "~/.cache/eget"
`)
	runner := &fakeRunner{
		result: RunResult{
			URL:            "https://example.com/tool.tar.gz",
			ExtractedFiles: []string{"./tool.tar.gz"},
		},
	}
	svc := Service{
		Runner: runner,
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfg, nil
		},
	}

	_, err := svc.DownloadTarget("https://example.com/tool.tar.gz", install.Options{})
	if err != nil {
		t.Fatalf("download target: %v", err)
	}

	expectedCache, err := util.Expand("~/.cache/eget")
	if err != nil {
		t.Fatalf("expand cache: %v", err)
	}

	if runner.opts.Output != expectedCache {
		t.Fatalf("expected configured cache dir as download output, got %q", runner.opts.Output)
	}
	if runner.opts.CacheDir != expectedCache {
		t.Fatalf("expected configured cache dir, got %q", runner.opts.CacheDir)
	}
}

func TestInstallTargetReturnsRunnerErrorWithoutRecording(t *testing.T) {
	runner := &fakeRunner{err: errors.New("boom")}
	store := &fakeInstalledStore{}
	svc := Service{Runner: runner, Store: store}

	_, err := svc.InstallTarget("junegunn/fzf", install.Options{})
	if err == nil {
		t.Fatal("expected install target to return runner error")
	}
	if store.calls != 0 {
		t.Fatalf("expected store to not be called on runner error, got %d", store.calls)
	}
}

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
	if config.name != "" {
		t.Fatalf("expected empty explicit name, got %q", config.name)
	}
	if config.opts.ExtractFile != "fzf" {
		t.Fatalf("expected extract file to be forwarded, got %q", config.opts.ExtractFile)
	}
	if config.opts.Tag != "v1.0.0" {
		t.Fatalf("expected tag to be forwarded, got %q", config.opts.Tag)
	}
	if len(config.opts.Asset) != 1 || config.opts.Asset[0] != "linux" {
		t.Fatalf("expected asset filter to be forwarded, got %#v", config.opts.Asset)
	}
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
