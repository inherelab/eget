package app

import (
	"errors"
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
