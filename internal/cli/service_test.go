package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gookit/goutil/x/ccolor"
	"github.com/inherelab/eget/internal/app"
	cfgpkg "github.com/inherelab/eget/internal/config"
	"github.com/inherelab/eget/internal/install"
	storepkg "github.com/inherelab/eget/internal/installed"
	sourcegithub "github.com/inherelab/eget/internal/source/github"
	"github.com/inherelab/eget/internal/util"
)

func TestInstallOptionsFromCommandsDoNotSetCacheDir(t *testing.T) {
	installOpts := installOptionsFromInstall(&InstallOptions{
		Tag:      "nightly",
		System:   "linux/amd64",
		To:       "~/.local/bin",
		File:     "tool",
		Asset:    "linux",
		Source:   true,
		All:      true,
		Quiet:    true,
		Add:      true,
		Name:     "tool",
	})
	if installOpts.CacheDir != "" {
		t.Fatalf("expected install cache dir to stay empty, got %q", installOpts.CacheDir)
	}
	if installOpts.Name != "tool" {
		t.Fatalf("expected install name to propagate, got %q", installOpts.Name)
	}

	downloadOpts := installOptionsFromDownload(&DownloadOptions{
		Tag:      "nightly",
		System:   "linux/amd64",
		To:       "~/.cache/downloads",
		Asset:    "linux",
		Source:   true,
		Quiet:    true,
	})
	if downloadOpts.CacheDir != "" {
		t.Fatalf("expected download cache dir to stay empty, got %q", downloadOpts.CacheDir)
	}
	if !downloadOpts.DownloadOnly {
		t.Fatal("expected plain download options to default to raw download mode")
	}

	addOpts := installOptionsFromAdd(&AddOptions{
		Name:     "tool",
		Tag:      "nightly",
		System:   "linux/amd64",
		To:       "~/.local/bin",
		File:     "tool",
		Asset:    "linux",
		Source:   true,
		All:      true,
		Quiet:    true,
	})
	if addOpts.CacheDir != "" {
		t.Fatalf("expected add cache dir to stay empty, got %q", addOpts.CacheDir)
	}

	updateOpts := installOptionsFromUpdate(&UpdateOptions{
		Tag:      "nightly",
		System:   "linux/amd64",
		To:       "~/.local/bin",
		Asset:    "linux",
		Source:   true,
		Quiet:    true,
	})
	if updateOpts.CacheDir != "" {
		t.Fatalf("expected update cache dir to stay empty, got %q", updateOpts.CacheDir)
	}
}

func TestInstallOptionsFromDownloadEnablesArchiveExtractionWhenRequested(t *testing.T) {
	opts := installOptionsFromDownload(&DownloadOptions{
		File: "tool,LICENSE",
	})
	if opts.DownloadOnly {
		t.Fatal("expected download with --file to disable DownloadOnly")
	}
	if opts.ExtractFile != "tool,LICENSE" {
		t.Fatalf("expected extract file filters to propagate, got %q", opts.ExtractFile)
	}

	opts = installOptionsFromDownload(&DownloadOptions{
		All: true,
	})
	if opts.DownloadOnly {
		t.Fatal("expected download with --all to disable DownloadOnly")
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

func TestHandleInstallAcceptsManagedPackageName(t *testing.T) {
	svc := &cliService{
		appService: app.Service{
			Runner: &fakeRunnerForCLI{
				result: app.RunResult{
					URL:            "https://github.com/sipeed/picoclaw/releases/download/v1.2.3/picoclaw.zip",
					Tool:           "picoclaw",
					ExtractedFiles: []string{"D:/Program/AITools/PicoClaw/picoclaw.exe"},
				},
			},
			Store: &fakeInstalledStoreForCLI{},
			LoadConfig: func() (*cfgpkg.File, error) {
				cfg := cfgpkg.NewFile()
				cfg.Packages["picoclaw"] = cfgpkg.Section{
					Repo:   util.StringPtr("sipeed/picoclaw"),
					Target: util.StringPtr("D:/Program/AITools/PicoClaw"),
				}
				return cfg, nil
			},
		},
	}

	err := svc.handle("install", &InstallOptions{
		Target: "picoclaw",
	})
	if err != nil {
		t.Fatalf("handle install: %v", err)
	}
}

func TestNewCLIServiceWiresReleaseInfo(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, ".config"))

	svc, err := newCLIService()
	if err != nil {
		t.Fatalf("newCLIService: %v", err)
	}
	if svc.appService.ReleaseInfo == nil {
		t.Fatal("expected ReleaseInfo to be configured")
	}
}

func TestLatestGitHubReleaseInfo(t *testing.T) {
	origGetWithOptions := githubAPIGetWithOptions
	defer func() { githubAPIGetWithOptions = origGetWithOptions }()

	var requestedURL string
	githubAPIGetWithOptions = func(rawURL string, opts install.Options) (*http.Response, error) {
		requestedURL = rawURL
		payload, err := json.Marshal(map[string]any{
			"tag_name":   "v0.3.6",
			"created_at": "2026-04-20T14:10:17Z",
		})
		if err != nil {
			t.Fatalf("marshal payload: %v", err)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(bytes.NewReader(payload)),
			Header:     make(http.Header),
		}, nil
	}

	tag, createdAt, err := latestGitHubReleaseInfo("gookit/gitw", install.Options{})
	if err != nil {
		t.Fatalf("latestGitHubReleaseInfo: %v", err)
	}
	if requestedURL != "https://api.github.com/repos/gookit/gitw/releases/latest" {
		t.Fatalf("unexpected request url: %s", requestedURL)
	}
	if tag != "v0.3.6" {
		t.Fatalf("expected tag v0.3.6, got %q", tag)
	}
	wantTime := time.Date(2026, 4, 20, 14, 10, 17, 0, time.UTC)
	if !createdAt.Equal(wantTime) {
		t.Fatalf("expected created_at %s, got %s", wantTime, createdAt)
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

func TestHandleListOutdatedPrintsOnlyOutdatedInstalledPackages(t *testing.T) {
	svc := &cliService{
		listService: app.ListService{
			LatestTag: func(repo string) (string, error) {
				switch repo {
				case "BurntSushi/ripgrep":
					return "v14.0.0", nil
				case "junegunn/fzf":
					return "v0.50.0", nil
				default:
					return "", nil
				}
			},
			LoadConfig: func() (*cfgpkg.File, error) {
				cfg := cfgpkg.NewFile()
				cfg.Packages["fzf"] = cfgpkg.Section{Repo: util.StringPtr("junegunn/fzf")}
				return cfg, nil
			},
			LoadInstalled: func() (*storepkg.Config, error) {
				return &storepkg.Config{
					Installed: map[string]storepkg.Entry{
						"BurntSushi/ripgrep": {Repo: "BurntSushi/ripgrep", Tag: "v13.0.0"},
						"junegunn/fzf":       {Repo: "junegunn/fzf", Tag: "v0.50.0"},
					},
				}, nil
			},
		},
	}

	var out bytes.Buffer
	ccolor.SetOutput(&out)
	defer ccolor.SetOutput(os.Stdout)

	err := svc.handleList(&ListOptions{Outdated: true})
	if err != nil {
		t.Fatalf("handle list outdated: %v", err)
	}

	got := out.String()
	if !strings.Contains(strings.ToLower(got), "latest version") {
		t.Fatalf("expected last_version column in output, got %q", got)
	}
	if !strings.Contains(got, "BurntSushi/ripgrep") {
		t.Fatalf("expected outdated repo in output, got %q", got)
	}
	if !strings.Contains(got, "v14.0.0") {
		t.Fatalf("expected latest_tag in output, got %q", got)
	}
	if strings.Contains(got, "junegunn/fzf") {
		t.Fatalf("expected up-to-date repo to be omitted, got %q", got)
	}
}

func TestHandleListPrintsTable(t *testing.T) {
	svc := &cliService{
		listService: app.ListService{
			LoadConfig: func() (*cfgpkg.File, error) {
				cfg := cfgpkg.NewFile()
				cfg.Packages["chlog"] = cfgpkg.Section{Repo: util.StringPtr("gookit/gitw")}
				return cfg, nil
			},
			LoadInstalled: func() (*storepkg.Config, error) {
				return &storepkg.Config{
					Installed: map[string]storepkg.Entry{
						"gookit/gitw": {Repo: "gookit/gitw", Tag: "v0.3.6"},
					},
				}, nil
			},
		},
	}

	var out bytes.Buffer
	ccolor.SetOutput(&out)
	defer ccolor.SetOutput(os.Stdout)

	err := svc.handleList(&ListOptions{})
	if err != nil {
		t.Fatalf("handle list: %v", err)
	}

	got := out.String()
	if !strings.Contains(strings.ToLower(got), "name") || !strings.Contains(strings.ToLower(got), "version") {
		t.Fatalf("expected table headers in output, got %q", got)
	}
	if !strings.Contains(got, "chlog") || !strings.Contains(got, "v0.3.6") {
		t.Fatalf("expected table row in output, got %q", got)
	}
}

func TestHandleListInfoPrintsDetails(t *testing.T) {
	now := time.Unix(1710000000, 0).UTC()
	svc := &cliService{
		listService: app.ListService{
			LoadConfig: func() (*cfgpkg.File, error) {
				cfg := cfgpkg.NewFile()
				cfg.Packages["chlog"] = cfgpkg.Section{
					Repo:   util.StringPtr("gookit/gitw"),
					Target: util.StringPtr("~/.local/bin"),
				}
				return cfg, nil
			},
			LoadInstalled: func() (*storepkg.Config, error) {
				return &storepkg.Config{
					Installed: map[string]storepkg.Entry{
						"gookit/gitw": {
							Repo:        "gookit/gitw",
							InstalledAt: now,
							Tag:         "v0.3.6",
							Asset:       "chlog-windows-amd64.exe",
							URL:         "https://github.com/gookit/gitw/releases/download/v0.3.6/chlog-windows-amd64.exe",
						},
					},
				}, nil
			},
		},
	}

	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()
	os.Stdout = w
	defer func() { os.Stdout = origStdout }()

	err = svc.handleList(&ListOptions{Info: "chlog"})
	if err != nil {
		t.Fatalf("handle list info: %v", err)
	}

	_ = w.Close()
	var out bytes.Buffer
	if _, err := io.Copy(&out, r); err != nil {
		t.Fatalf("copy stdout: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "name: chlog") || !strings.Contains(got, "version: v0.3.6") {
		t.Fatalf("expected detail output, got %q", got)
	}
	if !strings.Contains(got, "url: https://github.com/gookit/gitw/releases/download/v0.3.6/chlog-windows-amd64.exe") {
		t.Fatalf("expected detailed url output, got %q", got)
	}
}

func TestHandleListRejectsOutdatedWithInfo(t *testing.T) {
	svc := &cliService{}
	err := svc.handleList(&ListOptions{Outdated: true, Info: "chlog"})
	if err == nil {
		t.Fatal("expected conflicting list options to fail")
	}
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

func TestHandleQueryPrintsLatestRelease(t *testing.T) {
	svc := &cliService{
		queryService: app.QueryService{
			Client: &fakeQueryClientForCLI{
				releases: []app.QueryRelease{{
					Tag:         "v1.2.3",
					Name:        "v1.2.3",
					PublishedAt: time.Date(2026, 4, 22, 9, 0, 0, 0, time.UTC),
					AssetsCount: 2,
				}},
			},
		},
	}

	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()
	os.Stdout = w
	defer func() { os.Stdout = origStdout }()

	err = svc.handleQuery(&QueryOptions{Target: "owner/repo"})
	if err != nil {
		t.Fatalf("handle query: %v", err)
	}

	_ = w.Close()
	var out bytes.Buffer
	if _, err := io.Copy(&out, r); err != nil {
		t.Fatalf("copy stdout: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "action: latest") || !strings.Contains(got, "repo: owner/repo") {
		t.Fatalf("expected latest query output, got %q", got)
	}
}

func TestHandleQueryJSONOutput(t *testing.T) {
	svc := &cliService{
		queryService: app.QueryService{
			Client: &fakeQueryClientForCLI{
				releases: []app.QueryRelease{{Tag: "v1.2.3"}},
			},
		},
	}

	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()
	os.Stdout = w
	defer func() { os.Stdout = origStdout }()

	err = svc.handleQuery(&QueryOptions{Target: "owner/repo", JSON: true})
	if err != nil {
		t.Fatalf("handle query: %v", err)
	}

	_ = w.Close()
	var out bytes.Buffer
	if _, err := io.Copy(&out, r); err != nil {
		t.Fatalf("copy stdout: %v", err)
	}
	if !strings.Contains(out.String(), `"action": "latest"`) {
		t.Fatalf("expected json query output, got %q", out.String())
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

type fakeQueryClientForCLI struct {
	repoInfo QueryRepoInfoAlias
	releases []app.QueryRelease
	assets   []app.QueryAsset
}

type QueryRepoInfoAlias = app.QueryRepoInfo

func (f *fakeQueryClientForCLI) RepoInfo(repo string) (app.QueryRepoInfo, error) {
	info := app.QueryRepoInfo(f.repoInfo)
	if info.Repo == "" {
		info.Repo = repo
	}
	return info, nil
}

func (f *fakeQueryClientForCLI) LatestRelease(repo string, includePrerelease bool) (app.QueryRelease, error) {
	if len(f.releases) == 0 {
		return app.QueryRelease{}, nil
	}
	return f.releases[0], nil
}

func (f *fakeQueryClientForCLI) ListReleases(repo string, limit int, includePrerelease bool) ([]app.QueryRelease, error) {
	return f.releases, nil
}

func (f *fakeQueryClientForCLI) ReleaseAssets(repo, tag string) ([]app.QueryAsset, error) {
	return f.assets, nil
}
