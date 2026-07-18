package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gookit/goutil/x/assert"
	"github.com/gookit/goutil/x/ccolor"
	"github.com/inherelab/eget/internal/app"
	cfgpkg "github.com/inherelab/eget/internal/config"
	"github.com/inherelab/eget/internal/install"
	"github.com/inherelab/eget/internal/util"
)

func TestInstallOptionsFromCommandsDoNotSetCacheDir(t *testing.T) {
	installOpts := installOptionsFromInstall(&InstallOptions{
		Tag:             "nightly",
		Prerelease:      true,
		System:          "linux/amd64",
		To:              "~/.local/bin",
		File:            "tool",
		Asset:           "linux",
		Rename:          "tool-linux-amd64=tool",
		Source:          true,
		All:             true,
		Quiet:           true,
		StripComponents: 1,
		Add:             true,
		Name:            "tool",
		GUI:             true,
		InstallMode:     install.InstallModeInstaller,
	})
	if installOpts.CacheDir != "" {
		t.Fatalf("expected install cache dir to stay empty, got %q", installOpts.CacheDir)
	}
	if installOpts.Name != "tool" {
		t.Fatalf("expected install name to propagate, got %q", installOpts.Name)
	}
	assert.Eq(t, map[string]string{"tool-linux-amd64": "tool"}, installOpts.RenameFiles)
	assert.Eq(t, 1, installOpts.StripComponents)
	assert.True(t, installOpts.IsGUI)
	assert.True(t, installOpts.Prerelease)
	assert.Eq(t, install.InstallModeInstaller, installOpts.InstallMode)

	downloadOpts := installOptionsFromDownload(&DownloadOptions{
		Tag:             "nightly",
		Prerelease:      true,
		System:          "linux/amd64",
		To:              "~/.cache/downloads",
		Asset:           "linux",
		Rename:          "tool-linux-amd64=tool",
		Source:          true,
		Quiet:           true,
		StripComponents: 1,
	})
	if downloadOpts.CacheDir != "" {
		t.Fatalf("expected download cache dir to stay empty, got %q", downloadOpts.CacheDir)
	}
	if !downloadOpts.DownloadOnly {
		t.Fatal("expected plain download options to default to raw download mode")
	}
	assert.True(t, downloadOpts.Prerelease)
	assert.Eq(t, 1, downloadOpts.StripComponents)

	addOpts := installOptionsFromAdd(&AddOptions{
		Name:            "tool",
		Tag:             "nightly",
		System:          "linux/amd64",
		To:              "~/.local/bin",
		File:            "tool",
		Asset:           "linux",
		Rename:          "tool-linux-amd64=tool",
		Source:          true,
		All:             true,
		Quiet:           true,
		StripComponents: 1,
	})
	if addOpts.CacheDir != "" {
		t.Fatalf("expected add cache dir to stay empty, got %q", addOpts.CacheDir)
	}
	assert.Eq(t, map[string]string{"tool-linux-amd64": "tool"}, addOpts.RenameFiles)
	assert.Eq(t, 1, addOpts.StripComponents)

	updateOpts := installOptionsFromUpdate(&UpdateOptions{
		Tag:    "nightly",
		System: "linux/amd64",
		To:     "~/.local/bin",
		Asset:  "linux",
		Source: true,
		Quiet:  true,
	})
	if updateOpts.CacheDir != "" {
		t.Fatalf("expected update cache dir to stay empty, got %q", updateOpts.CacheDir)
	}
}

func TestApplyGlobalNetworkConfigDerivesAPICacheDir(t *testing.T) {
	t.Setenv("NO_PROXY", "")
	cacheDir := "~/.cache/eget"
	proxyURL := "http://127.0.0.1:7890"
	apiCacheEnabled := true
	apiCacheTime := 120
	cfg := cfgpkg.NewFile()
	cfg.Global.CacheDir = &cacheDir
	cfg.Global.ProxyURL = &proxyURL
	cfg.ApiCache.Enable = &apiCacheEnabled
	cfg.ApiCache.CacheTime = &apiCacheTime

	opts := install.Options{}
	applyGlobalNetworkConfig(&opts, cfg)

	expectedCache, err := util.Expand(cacheDir)
	assert.NoErr(t, err)
	assert.True(t, opts.APICacheEnabled)
	assert.Eq(t, 120, opts.APICacheTime)
	assert.Eq(t, filepath.Join(expectedCache, "api-cache"), opts.APICacheDir)
	assert.Eq(t, proxyURL, opts.ProxyURL)
}

func TestApplyGlobalNetworkConfigSkipsProxyURLWhenNoProxyEnvSet(t *testing.T) {
	t.Setenv("NO_PROXY", "1")
	proxyURL := "http://127.0.0.1:7890"
	cfg := cfgpkg.NewFile()
	cfg.Global.ProxyURL = &proxyURL

	opts := install.Options{}
	applyGlobalNetworkConfig(&opts, cfg)

	assert.Eq(t, "", opts.ProxyURL)
}

func TestApplyGlobalNetworkConfigUsesHTTPProxy(t *testing.T) {
	t.Setenv("NO_PROXY", "")
	proxyURL := "http://127.0.0.1:10801"
	cfg := cfgpkg.NewFile()
	cfg.HTTPProxy.URL = &proxyURL
	cfg.HTTPProxy.Exclude = []string{"mydev.com"}

	opts := install.Options{}
	applyGlobalNetworkConfig(&opts, cfg)

	assert.Eq(t, proxyURL, opts.ProxyURL)
	assert.Eq(t, []string{"mydev.com"}, opts.ProxyExclude)
}

func TestApplyGlobalNetworkConfigHTTPProxyEnableFalseDisablesConfiguredProxy(t *testing.T) {
	t.Setenv("NO_PROXY", "")
	enabled := false
	proxyURL := "http://127.0.0.1:10801"
	cfg := cfgpkg.NewFile()
	cfg.HTTPProxy.Enable = &enabled
	cfg.HTTPProxy.URL = &proxyURL

	opts := install.Options{}
	applyGlobalNetworkConfig(&opts, cfg)

	assert.Eq(t, "", opts.ProxyURL)
}

func TestApplyGlobalNetworkConfigHTTPProxyFallsBackToLegacyGlobalProxyURL(t *testing.T) {
	t.Setenv("NO_PROXY", "")
	proxyURL := "http://127.0.0.1:7890"
	cfg := cfgpkg.NewFile()
	cfg.Global.ProxyURL = &proxyURL

	opts := install.Options{}
	applyGlobalNetworkConfig(&opts, cfg)

	assert.Eq(t, proxyURL, opts.ProxyURL)
}

func TestApplyGlobalNetworkConfigHTTPProxyWinsOverLegacyGlobalProxyURL(t *testing.T) {
	t.Setenv("NO_PROXY", "")
	legacyURL := "http://127.0.0.1:7890"
	proxyURL := "http://127.0.0.1:10801"
	cfg := cfgpkg.NewFile()
	cfg.Global.ProxyURL = &legacyURL
	cfg.HTTPProxy.URL = &proxyURL

	opts := install.Options{}
	applyGlobalNetworkConfig(&opts, cfg)

	assert.Eq(t, proxyURL, opts.ProxyURL)
}

func TestApplyGlobalNetworkConfigNoProxyHostListAddsProxyExclude(t *testing.T) {
	t.Setenv("NO_PROXY", "api.github.com,*.corp.local")
	proxyURL := "http://127.0.0.1:10801"
	cfg := cfgpkg.NewFile()
	cfg.HTTPProxy.URL = &proxyURL
	cfg.HTTPProxy.Exclude = []string{"mydev.com"}

	opts := install.Options{}
	applyGlobalNetworkConfig(&opts, cfg)

	assert.Eq(t, proxyURL, opts.ProxyURL)
	assert.Eq(t, []string{"mydev.com", "api.github.com", "*.corp.local"}, opts.ProxyExclude)
}

func TestApplyGlobalNetworkConfigNoProxyEnvDisablesHTTPProxy(t *testing.T) {
	t.Setenv("NO_PROXY", "1")
	proxyURL := "http://127.0.0.1:10801"
	cfg := cfgpkg.NewFile()
	cfg.HTTPProxy.URL = &proxyURL

	opts := install.Options{}
	applyGlobalNetworkConfig(&opts, cfg)

	assert.Eq(t, "", opts.ProxyURL)
}

func TestApplyGlobalNetworkConfigCLIProxyURLOverridesHTTPProxy(t *testing.T) {
	t.Setenv("NO_PROXY", "")
	proxyURL := "http://127.0.0.1:10801"
	cliURL := "http://127.0.0.1:10802"
	cfg := cfgpkg.NewFile()
	cfg.HTTPProxy.URL = &proxyURL

	opts := install.Options{ProxyURL: cliURL}
	applyGlobalNetworkConfig(&opts, cfg)

	assert.Eq(t, cliURL, opts.ProxyURL)
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
		t.Fatal("expected download with extract-all to disable DownloadOnly")
	}
}

func TestInstallOptionsFromDownloadEnablesGhproxy(t *testing.T) {
	opts := installOptionsFromDownload(&DownloadOptions{
		Ghproxy: true,
	})

	assert.True(t, opts.GhproxyEnabled)
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
		Targets: []string{"gookit/gitw"},
		Add:     true,
		Name:    "chlog",
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

func TestHandleAddPrintsInferredPackageName(t *testing.T) {
	var saved *cfgpkg.File
	svc := &cliService{
		cfgService: app.ConfigService{
			Load: func() (*cfgpkg.File, error) {
				return cfgpkg.NewFile(), nil
			},
			Save: func(path string, file *cfgpkg.File) error {
				saved = file
				return nil
			},
		},
	}

	var out bytes.Buffer
	ccolor.SetOutput(&out)
	defer ccolor.SetOutput(os.Stdout)

	err := svc.handle("add", &AddOptions{Target: "sharkdp/fd"})
	if err != nil {
		t.Fatalf("handle add: %v", err)
	}

	if saved == nil {
		t.Fatal("expected config to be saved")
	}
	if _, ok := saved.Packages["fd"]; !ok {
		t.Fatalf("expected packages.fd to be saved, got %#v", saved.Packages)
	}
	if !strings.Contains(out.String(), "Added package config: fd -> sharkdp/fd") {
		t.Fatalf("expected inferred package name in output, got %q", out.String())
	}
}

func TestHandleAddPrintsPkgTemplateAliasName(t *testing.T) {
	var saved *cfgpkg.File
	cfg := cfgpkg.NewFile()
	cfg.PkgTemplates["mydev"] = cfgpkg.Section{
		LatestURL:   util.StringPtr("http://mydev.lan/tools/{name}/latest.yaml"),
		URLTemplate: util.StringPtr("http://mydev.lan/tools/{name}/{name}-{os}-{arch}{ext}"),
	}
	svc := &cliService{
		cfgService: app.ConfigService{
			Load: func() (*cfgpkg.File, error) { return cfg, nil },
			Save: func(path string, file *cfgpkg.File) error {
				saved = file
				return nil
			},
		},
	}
	var out bytes.Buffer
	ccolor.SetOutput(&out)
	defer ccolor.SetOutput(os.Stdout)

	err := svc.handle("add", &AddOptions{Target: "mydev:markview"})

	assert.NoErr(t, err)
	assert.Eq(t, "pkg-template:mydev:markview", *saved.Packages["markview"].Repo)
	assert.Contains(t, out.String(), "Added package config: markview -> mydev:markview")
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
		Targets: []string{"picoclaw"},
	})
	if err != nil {
		t.Fatalf("handle install: %v", err)
	}
}

func TestHandleDownloadAcceptsManagedTemplatePackageName(t *testing.T) {
	runner := &fakeRunnerForCLI{
		result: app.RunResult{
			URL:   "https://example.com/1.2.3/win32-x64/claude.exe",
			Tool:  "claude",
			Asset: "claude.exe",
		},
	}
	svc := &cliService{
		appService: app.Service{
			Runner: runner,
			LoadConfig: func() (*cfgpkg.File, error) {
				cfg := cfgpkg.NewFile()
				cfg.Packages["claude"] = cfgpkg.Section{
					Repo:         util.StringPtr("template:claude"),
					LatestURL:    util.StringPtr("https://example.com/latest"),
					LatestFormat: util.StringPtr("text"),
					URLTemplate:  util.StringPtr("https://example.com/{version}/{os}-{arch}/claude{ext}"),
					OSMap:        map[string]string{"windows": "win32"},
					ArchMap:      map[string]string{"amd64": "x64"},
					ExtMap:       map[string]string{"windows": ".exe"},
				}
				return cfg, nil
			},
		},
	}

	err := svc.handle("download", &DownloadOptions{Target: "claude"})
	if err != nil {
		t.Fatalf("handle download: %v", err)
	}

	assert.Eq(t, []string{"template:claude"}, runner.targets)
	opts := runner.optsByTarget["template:claude"]
	assert.True(t, opts.DownloadOnly)
	assert.Eq(t, "https://example.com/latest", opts.URLTemplate.LatestURL)
	assert.Eq(t, "https://example.com/{version}/{os}-{arch}/claude{ext}", opts.URLTemplate.URLTemplate)
}

func TestHandleInstallInstallsMultipleTargets(t *testing.T) {
	runner := &fakeRunnerForCLI{
		result: app.RunResult{
			URL:            "https://github.com/example/repo/releases/download/v1.0.0/tool.tar.gz",
			Tool:           "tool",
			ExtractedFiles: []string{"./tool"},
		},
	}
	svc := &cliService{
		appService: app.Service{
			Runner: runner,
			LoadConfig: func() (*cfgpkg.File, error) {
				cfg := cfgpkg.NewFile()
				cfg.Packages["fzf"] = cfgpkg.Section{Repo: util.StringPtr("junegunn/fzf")}
				cfg.Packages["rg"] = cfgpkg.Section{Repo: util.StringPtr("BurntSushi/ripgrep")}
				return cfg, nil
			},
		},
	}

	err := svc.handle("install", &InstallOptions{Targets: []string{"fzf", "rg"}, Quiet: true})
	assert.NoErr(t, err)
	assert.Eq(t, []string{"junegunn/fzf", "BurntSushi/ripgrep"}, runner.targets)
	if !runner.optsByTarget["junegunn/fzf"].Quiet || !runner.optsByTarget["BurntSushi/ripgrep"].Quiet {
		t.Fatalf("expected cli install options to propagate, got %#v", runner.optsByTarget)
	}
}

func TestHandleInstallAllInstallsManagedPackages(t *testing.T) {
	runner := &fakeRunnerForCLI{
		result: app.RunResult{
			URL:            "https://github.com/example/repo/releases/download/v1.0.0/tool.tar.gz",
			Tool:           "tool",
			ExtractedFiles: []string{"./tool"},
		},
	}
	svc := &cliService{
		appService: app.Service{
			Runner: runner,
			LoadConfig: func() (*cfgpkg.File, error) {
				cfg := cfgpkg.NewFile()
				cfg.Packages["fzf"] = cfgpkg.Section{Repo: util.StringPtr("junegunn/fzf")}
				cfg.Packages["rg"] = cfgpkg.Section{Repo: util.StringPtr("BurntSushi/ripgrep")}
				return cfg, nil
			},
		},
	}

	err := svc.handle("install", &InstallOptions{InstallAll: true, Quiet: true, BatchConcurrency: 1})
	if err != nil {
		t.Fatalf("handle install --all: %v", err)
	}

	assert.Eq(t, []string{"junegunn/fzf", "BurntSushi/ripgrep"}, runner.targets)
	if !runner.optsByTarget["junegunn/fzf"].Quiet || !runner.optsByTarget["BurntSushi/ripgrep"].Quiet {
		t.Fatalf("expected cli install options to propagate, got %#v", runner.optsByTarget)
	}
}

func TestHandleInstallRejectsMissingTargetWithoutAll(t *testing.T) {
	svc := &cliService{}

	err := svc.handle("install", &InstallOptions{})
	if err == nil {
		t.Fatal("expected missing install target to fail")
	}
	if !strings.Contains(err.Error(), "install target is required") {
		t.Fatalf("expected missing target error, got %v", err)
	}
}

func TestHandleRejectsBatchWithoutAll(t *testing.T) {
	svc := &cliService{}

	err := svc.handle("install", &InstallOptions{Targets: []string{"owner/repo"}, BatchConcurrency: 2})
	if err == nil || !strings.Contains(err.Error(), "--batch can only be used with --all") {
		t.Fatalf("expected install --batch without --all to fail, got %v", err)
	}

	err = svc.handle("update", &UpdateOptions{Targets: []string{"fd"}, BatchConcurrency: 2})
	if err == nil || !strings.Contains(err.Error(), "--batch can only be used with --all") {
		t.Fatalf("expected update --batch without --all to fail, got %v", err)
	}
}

func TestInstallOptionsFromCommandsPropagateConcurrency(t *testing.T) {
	installOpts := installOptionsFromInstall(&InstallOptions{
		ChunkConcurrency: 4,
		BatchConcurrency: 2,
	})
	assert.Eq(t, 4, installOpts.ChunkConcurrency)
	assert.Eq(t, 2, installOpts.BatchConcurrency)
	assert.True(t, installOpts.ChunkConcurrencySet)
	assert.True(t, installOpts.BatchConcurrencySet)

	downloadOpts := installOptionsFromDownload(&DownloadOptions{ChunkConcurrency: 3})
	assert.Eq(t, 3, downloadOpts.ChunkConcurrency)
	assert.True(t, downloadOpts.ChunkConcurrencySet)

	addOpts := installOptionsFromAdd(&AddOptions{ChunkConcurrency: 5})
	assert.Eq(t, 5, addOpts.ChunkConcurrency)
	assert.True(t, addOpts.ChunkConcurrencySet)

	updateOpts := installOptionsFromUpdate(&UpdateOptions{
		ChunkConcurrency: 6,
		BatchConcurrency: 4,
	})
	assert.Eq(t, 6, updateOpts.ChunkConcurrency)
	assert.Eq(t, 4, updateOpts.BatchConcurrency)
	assert.True(t, updateOpts.ChunkConcurrencySet)
	assert.True(t, updateOpts.BatchConcurrencySet)
}

func TestInstallOptionsFromCommandsPropagateRetries(t *testing.T) {
	assert.Eq(t, 3, installOptionsFromInstall(&InstallOptions{Retries: 3}).Retries)
	assert.Eq(t, 3, installOptionsFromUpdate(&UpdateOptions{Retries: 3}).Retries)
	assert.Eq(t, 3, installOptionsFromDownload(&DownloadOptions{Retries: 3}).Retries)
	assert.Eq(t, 3, installOptionsFromAdd(&AddOptions{Retries: 3}).Retries)
}

func TestHandleInstallAllRejectsTarget(t *testing.T) {
	svc := &cliService{}

	err := svc.handle("install", &InstallOptions{InstallAll: true, Targets: []string{"junegunn/fzf"}})
	if err == nil {
		t.Fatal("expected install --all with target to fail")
	}
	if !strings.Contains(err.Error(), "install --all cannot be used with target") {
		t.Fatalf("expected all with target error, got %v", err)
	}
}

func TestHandleInstallWarnsWhenSudoUserConfigExistsButCurrentConfigMissing(t *testing.T) {
	var stderr bytes.Buffer
	runner := &fakeRunnerForCLI{}
	svc := &cliService{
		stderr: &stderr,
		appService: app.Service{
			Runner: runner,
			LoadConfig: func() (*cfgpkg.File, error) {
				return cfgpkg.NewFile(), nil
			},
		},
		configPathResolver: func() (string, error) {
			return "", os.ErrNotExist
		},
		lookupEnv: func(key string) (string, bool) {
			switch key {
			case "SUDO_USER":
				return "alice", true
			default:
				return "", false
			}
		},
		lookupUserHome: func(name string) (string, error) {
			if name != "alice" {
				t.Fatalf("unexpected user lookup %q", name)
			}
			return "/home/alice", nil
		},
		fileExists: func(path string) bool {
			return filepath.ToSlash(path) == "/home/alice/.config/eget/eget.toml"
		},
	}

	err := svc.handle("install", &InstallOptions{Targets: []string{"babarot/gomi"}})
	if err != nil {
		t.Fatalf("handle install: %v", err)
	}

	got := stderr.String()
	if !strings.Contains(got, "sudo may be using a different HOME") {
		t.Fatalf("expected sudo config warning, got %q", got)
	}
	if !strings.Contains(got, `sudo EGET_CONFIG="/home/alice/.config/eget/eget.toml" eget install`) {
		t.Fatalf("expected workaround in warning, got %q", got)
	}
}

func TestHandleInstallDoesNotWarnWhenQuiet(t *testing.T) {
	var stderr bytes.Buffer
	svc := &cliService{
		stderr: &stderr,
		appService: app.Service{
			Runner: &fakeRunnerForCLI{},
			LoadConfig: func() (*cfgpkg.File, error) {
				return cfgpkg.NewFile(), nil
			},
		},
		configPathResolver: func() (string, error) {
			return "", os.ErrNotExist
		},
		lookupEnv: func(key string) (string, bool) {
			if key == "SUDO_USER" {
				return "alice", true
			}
			return "", false
		},
		lookupUserHome: func(string) (string, error) {
			return "/home/alice", nil
		},
		fileExists: func(path string) bool {
			return path == "/home/alice/.config/eget/eget.toml"
		},
	}

	err := svc.handle("install", &InstallOptions{Targets: []string{"babarot/gomi"}, Quiet: true})
	if err != nil {
		t.Fatalf("handle install: %v", err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected quiet install to suppress warning, got %q", stderr.String())
	}
}

func TestHandleInstallDoesNotWarnWhenEGETConfigIsExplicit(t *testing.T) {
	var stderr bytes.Buffer
	svc := &cliService{
		stderr: &stderr,
		appService: app.Service{
			Runner: &fakeRunnerForCLI{},
			LoadConfig: func() (*cfgpkg.File, error) {
				return cfgpkg.NewFile(), nil
			},
		},
		configPathResolver: func() (string, error) {
			return "", os.ErrNotExist
		},
		lookupEnv: func(key string) (string, bool) {
			switch key {
			case "EGET_CONFIG":
				return "/home/alice/.config/eget/eget.toml", true
			case "SUDO_USER":
				return "alice", true
			default:
				return "", false
			}
		},
		lookupUserHome: func(string) (string, error) {
			return "/home/alice", nil
		},
		fileExists: func(path string) bool {
			return path == "/home/alice/.config/eget/eget.toml"
		},
	}

	err := svc.handle("install", &InstallOptions{Targets: []string{"babarot/gomi"}})
	if err != nil {
		t.Fatalf("handle install: %v", err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected explicit EGET_CONFIG to suppress warning, got %q", stderr.String())
	}
}
