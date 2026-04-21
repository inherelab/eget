package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/inherelab/eget/internal/app"
	cfgpkg "github.com/inherelab/eget/internal/config"
	"github.com/inherelab/eget/internal/install"
	storepkg "github.com/inherelab/eget/internal/installed"
	sourcegithub "github.com/inherelab/eget/internal/source/github"
)

func TestInstallOptionsFromCommandsIncludeCacheDir(t *testing.T) {
	installOpts := installOptionsFromInstall(&InstallOptions{
		Tag:      "nightly",
		System:   "linux/amd64",
		To:       "~/.local/bin",
		CacheDir: "~/.cache/eget",
		File:     "tool",
		Asset:    "linux",
		Source:   true,
		All:      true,
		Quiet:    true,
		Add:      true,
		Name:     "tool",
	})
	if installOpts.CacheDir != "~/.cache/eget" {
		t.Fatalf("expected install cache dir to propagate, got %q", installOpts.CacheDir)
	}
	if installOpts.Name != "tool" {
		t.Fatalf("expected install name to propagate, got %q", installOpts.Name)
	}

	downloadOpts := installOptionsFromDownload(&DownloadOptions{
		Tag:      "nightly",
		System:   "linux/amd64",
		To:       "~/.cache/downloads",
		CacheDir: "~/.cache/eget",
		File:     "tool",
		Asset:    "linux",
		Source:   true,
		All:      true,
		Quiet:    true,
	})
	if downloadOpts.CacheDir != "~/.cache/eget" {
		t.Fatalf("expected download cache dir to propagate, got %q", downloadOpts.CacheDir)
	}
	if !downloadOpts.DownloadOnly {
		t.Fatal("expected download options to force DownloadOnly")
	}

	addOpts := installOptionsFromAdd(&AddOptions{
		Name:     "tool",
		Tag:      "nightly",
		System:   "linux/amd64",
		To:       "~/.local/bin",
		CacheDir: "~/.cache/eget",
		File:     "tool",
		Asset:    "linux",
		Source:   true,
		All:      true,
		Quiet:    true,
	})
	if addOpts.CacheDir != "~/.cache/eget" {
		t.Fatalf("expected add cache dir to propagate, got %q", addOpts.CacheDir)
	}

	updateOpts := installOptionsFromUpdate(&UpdateOptions{
		Tag:      "nightly",
		System:   "linux/amd64",
		To:       "~/.local/bin",
		CacheDir: "~/.cache/eget",
		Asset:    "linux",
		Source:   true,
		Quiet:    true,
	})
	if updateOpts.CacheDir != "~/.cache/eget" {
		t.Fatalf("expected update cache dir to propagate, got %q", updateOpts.CacheDir)
	}
}

func TestPrintConfigListIncludesHeaderComment(t *testing.T) {
	cfg := cfgpkg.NewFile()
	target := "~/.local/bin"
	cfg.Global.Target = &target
	enable := false
	cacheTime := 300
	hostURL := ""
	supportAPI := true
	cfg.ApiCache.Enable = &enable
	cfg.ApiCache.CacheTime = &cacheTime
	cfg.Ghproxy.Enable = &enable
	cfg.Ghproxy.HostURL = &hostURL
	cfg.Ghproxy.SupportAPI = &supportAPI
	cfg.Ghproxy.Fallbacks = []string{}

	var out bytes.Buffer
	printConfigList(&out, "testdata/eget.toml", true, cfg)

	got := out.String()
	if !strings.Contains(got, "# testdata/eget.toml, exists: true") {
		t.Fatalf("expected header comment, got %q", got)
	}
	if !strings.Contains(got, "[global]") {
		t.Fatalf("expected global section, got %q", got)
	}
	if !strings.Contains(got, "[api_cache]") {
		t.Fatalf("expected api_cache section, got %q", got)
	}
	if !strings.Contains(got, "cache_time = 300") {
		t.Fatalf("expected api_cache cache_time, got %q", got)
	}
	if !strings.Contains(got, "[ghproxy]") {
		t.Fatalf("expected ghproxy section, got %q", got)
	}
	if !strings.Contains(got, "host_url = ") {
		t.Fatalf("expected ghproxy host_url, got %q", got)
	}
}

func TestHandleInstallPrintsAddedPackageMessage(t *testing.T) {
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()
	os.Stdout = w
	defer func() {
		os.Stdout = origStdout
	}()

	svc := &cliService{
		appService: app.Service{
			Runner: &fakeRunnerForCLI{
				result: app.RunResult{
					URL:            "https://github.com/gookit/gitw/releases/download/v0.3.6/chlog-windows-amd64.exe",
					Tool:           "gitw",
					ExtractedFiles: []string{"C:/Users/inhere/.local/bin/chlog.exe"},
				},
			},
			Store:  &fakeInstalledStoreForCLI{},
			Config: &fakeConfigRecorderForCLI{},
			Now: func() time.Time {
				return time.Unix(1710000000, 0)
			},
			LoadConfig: func() (*cfgpkg.File, error) {
				return cfgpkg.NewFile(), nil
			},
		},
	}

	err = svc.handle("install", &InstallOptions{
		Target: "gookit/gitw",
		Add:    true,
		Name:   "chlog",
	})
	if err != nil {
		t.Fatalf("handle install: %v", err)
	}

	_ = w.Close()
	var out bytes.Buffer
	if _, err := io.Copy(&out, r); err != nil {
		t.Fatalf("copy stdout: %v", err)
	}
	if !strings.Contains(out.String(), "Added package config: chlog -> gookit/gitw") {
		t.Fatalf("expected add-package message, got %q", out.String())
	}
}

func TestConfigureVerboseUpdatesVerboseLoggers(t *testing.T) {
	var out bytes.Buffer
	configureVerbose(true, &out)
	if !install.VerboseEnabledForTest() {
		t.Fatalf("expected install verbose to be enabled")
	}
	if !sourcegithub.VerboseEnabledForTest() {
		t.Fatalf("expected source verbose to be enabled")
	}
	configureVerbose(false, &out)
}

func TestHandleConfigInitRejectsOverwriteWithoutConfirmation(t *testing.T) {
	svc := &cliService{
		cfgService: app.ConfigService{
			ConfigPath: "testdata/eget.toml",
			Load: func() (*cfgpkg.File, error) {
				cfg := cfgpkg.NewFile()
				target := "~/bin"
				cfg.Global.Target = &target
				return cfg, nil
			},
		},
	}

	origStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()
	os.Stdin = r
	defer func() { os.Stdin = origStdin }()

	if _, err := w.WriteString("n\n"); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	_ = w.Close()

	err = svc.handleConfig(&ConfigOptions{Action: "init"})
	if err == nil {
		t.Fatal("expected overwrite rejection error")
	}
	if !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("expected cancellation error, got %v", err)
	}
}

func TestHandleConfigInitAllowsOverwriteWithConfirmation(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "eget.toml")

	svc := &cliService{
		cfgService: app.ConfigService{
			ConfigPath: configPath,
		},
	}

	if err := os.WriteFile(configPath, []byte("[global]\ntarget = \"~/bin\"\n"), 0o644); err != nil {
		t.Fatalf("write existing config: %v", err)
	}

	origStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()
	os.Stdin = r
	defer func() { os.Stdin = origStdin }()

	if _, err := w.WriteString("y\n"); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	_ = w.Close()

	if err := svc.handleConfig(&ConfigOptions{Action: "init"}); err != nil {
		t.Fatalf("expected overwrite confirmation to allow init, got %v", err)
	}

	cfg, err := cfgpkg.LoadFile(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Global.Target == nil || *cfg.Global.Target != "~/.local/bin" {
		t.Fatalf("expected config to be overwritten with defaults, got %#v", cfg.Global.Target)
	}
}

type fakeRunnerForCLI struct {
	result app.RunResult
}

func (f *fakeRunnerForCLI) Run(target string, opts install.Options) (app.RunResult, error) {
	return f.result, nil
}

type fakeInstalledStoreForCLI struct{}

func (f *fakeInstalledStoreForCLI) Record(target string, entry storepkg.Entry) error {
	return nil
}

type fakeConfigRecorderForCLI struct{}

func (f *fakeConfigRecorderForCLI) AddPackage(repo, name string, opts install.Options) error {
	return nil
}
