package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gookit/cliui"
	"github.com/gookit/goutil/x/ccolor"
	"github.com/inherelab/eget/internal/app"
	cfgpkg "github.com/inherelab/eget/internal/config"
	"github.com/inherelab/eget/internal/install"
	storepkg "github.com/inherelab/eget/internal/installed"
	sourcegithub "github.com/inherelab/eget/internal/source/github"
	sourcesf "github.com/inherelab/eget/internal/source/sourceforge"
	"github.com/inherelab/eget/internal/util"
)

func TestInstallOptionsFromCommandsDoNotSetCacheDir(t *testing.T) {
	installOpts := installOptionsFromInstall(&InstallOptions{
		Tag:    "nightly",
		System: "linux/amd64",
		To:     "~/.local/bin",
		File:   "tool",
		Asset:  "linux",
		Source: true,
		All:    true,
		Quiet:  true,
		Add:    true,
		Name:   "tool",
	})
	if installOpts.CacheDir != "" {
		t.Fatalf("expected install cache dir to stay empty, got %q", installOpts.CacheDir)
	}
	if installOpts.Name != "tool" {
		t.Fatalf("expected install name to propagate, got %q", installOpts.Name)
	}

	downloadOpts := installOptionsFromDownload(&DownloadOptions{
		Tag:    "nightly",
		System: "linux/amd64",
		To:     "~/.cache/downloads",
		Asset:  "linux",
		Source: true,
		Quiet:  true,
	})
	if downloadOpts.CacheDir != "" {
		t.Fatalf("expected download cache dir to stay empty, got %q", downloadOpts.CacheDir)
	}
	if !downloadOpts.DownloadOnly {
		t.Fatal("expected plain download options to default to raw download mode")
	}

	addOpts := installOptionsFromAdd(&AddOptions{
		Name:   "tool",
		Tag:    "nightly",
		System: "linux/amd64",
		To:     "~/.local/bin",
		File:   "tool",
		Asset:  "linux",
		Source: true,
		All:    true,
		Quiet:  true,
	})
	if addOpts.CacheDir != "" {
		t.Fatalf("expected add cache dir to stay empty, got %q", addOpts.CacheDir)
	}

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

func TestPromptIndexConsumesTrailingNewline(t *testing.T) {
	origStdin := os.Stdin
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdin pipe: %v", err)
	}
	os.Stdin = reader
	defer func() {
		os.Stdin = origStdin
		_ = reader.Close()
	}()
	if _, err := writer.WriteString("14\ny\n"); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close stdin writer: %v", err)
	}

	choices := make([]string, 14)
	for i := range choices {
		choices[i] = "choice"
	}
	picked, err := promptIndex(choices)
	if err != nil {
		t.Fatalf("prompt index: %v", err)
	}
	if picked != 13 {
		t.Fatalf("expected zero-based selection 13, got %d", picked)
	}

	rest, err := io.ReadAll(os.Stdin)
	if err != nil {
		t.Fatalf("read remaining stdin: %v", err)
	}
	if string(rest) != "y\n" {
		t.Fatalf("expected prompt index to consume selection newline, remaining stdin %q", rest)
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

func TestConfigureVerboseUpdatesVerboseLoggers(t *testing.T) {
	var out bytes.Buffer
	configureVerbose(true, &out)
	if !install.VerboseEnabledForTest() {
		t.Fatalf("expected install verbose to be enabled")
	}
	if !sourcegithub.VerboseEnabledForTest() {
		t.Fatalf("expected source verbose to be enabled")
	}
	if !sourcesf.VerboseEnabledForTest() {
		t.Fatalf("expected sourceforge verbose to be enabled")
	}
	configureVerbose(false, &out)
}

func TestHandleListOutdatedPrintsOnlyOutdatedInstalledPackages(t *testing.T) {
	svc := &cliService{
		listService: app.ListService{
			LatestTag: func(repo, _ string) (string, error) {
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

func TestHandleListOutdatedPrintsCheckedInstalledCountWhenNothingOutdated(t *testing.T) {
	svc := &cliService{
		listService: app.ListService{
			LatestTag: func(repo, _ string) (string, error) {
				switch repo {
				case "gookit/gitw":
					return "v0.3.6", nil
				case "sipeed/picoclaw":
					return "v0.2.7", nil
				case "windirstat/windirstat":
					return "release/v2.5.0", nil
				default:
					return "", nil
				}
			},
			LoadConfig: func() (*cfgpkg.File, error) {
				return cfgpkg.NewFile(), nil
			},
			LoadInstalled: func() (*storepkg.Config, error) {
				return &storepkg.Config{
					Installed: map[string]storepkg.Entry{
						"gookit/gitw":           {Repo: "gookit/gitw", Tag: "v0.3.6"},
						"sipeed/picoclaw":       {Repo: "sipeed/picoclaw", Tag: "v0.2.7"},
						"windirstat/windirstat": {Repo: "windirstat/windirstat", Tag: "release/v2.5.0"},
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
	if !strings.Contains(got, "Checked 3 packages") {
		t.Fatalf("expected checked count for all installed packages, got %q", got)
	}
	if !strings.Contains(got, "No outdated packages found") {
		t.Fatalf("expected no outdated message, got %q", got)
	}
}

func TestHandleListPrintsOnlyInstalledPackagesByDefault(t *testing.T) {
	svc := &cliService{
		listService: app.ListService{
			LoadConfig: func() (*cfgpkg.File, error) {
				cfg := cfgpkg.NewFile()
				cfg.Packages["chlog"] = cfgpkg.Section{Repo: util.StringPtr("gookit/gitw")}
				cfg.Packages["ripgrep"] = cfgpkg.Section{Repo: util.StringPtr("BurntSushi/ripgrep")}
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
	if strings.Contains(got, "ripgrep") {
		t.Fatalf("expected default list to omit managed-only package, got %q", got)
	}
}

func TestHandleListAllPrintsManagedAndInstalledPackages(t *testing.T) {
	svc := &cliService{
		listService: app.ListService{
			LoadConfig: func() (*cfgpkg.File, error) {
				cfg := cfgpkg.NewFile()
				cfg.Packages["chlog"] = cfgpkg.Section{Repo: util.StringPtr("gookit/gitw")}
				cfg.Packages["ripgrep"] = cfgpkg.Section{Repo: util.StringPtr("BurntSushi/ripgrep")}
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

	err := svc.handleList(&ListOptions{All: true})
	if err != nil {
		t.Fatalf("handle list all: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "chlog") || !strings.Contains(got, "ripgrep") {
		t.Fatalf("expected all list to include installed and managed-only packages, got %q", got)
	}
}

func TestHandleListGUIPrintsOnlyGUIPackages(t *testing.T) {
	isGUI := true
	svc := &cliService{
		listService: app.ListService{
			LoadConfig: func() (*cfgpkg.File, error) {
				cfg := cfgpkg.NewFile()
				cfg.Packages["picoclaw"] = cfgpkg.Section{Repo: util.StringPtr("sipeed/picoclaw"), IsGUI: &isGUI}
				cfg.Packages["chlog"] = cfgpkg.Section{Repo: util.StringPtr("gookit/gitw")}
				return cfg, nil
			},
			LoadInstalled: func() (*storepkg.Config, error) {
				return &storepkg.Config{Installed: map[string]storepkg.Entry{
					"sipeed/picoclaw": {Repo: "sipeed/picoclaw", Tag: "v0.2.7", IsGUI: true, InstallMode: "portable"},
					"gookit/gitw":     {Repo: "gookit/gitw", Tag: "v0.3.6"},
				}}, nil
			},
		},
	}
	var out bytes.Buffer
	ccolor.SetOutput(&out)
	defer ccolor.SetOutput(os.Stdout)
	err := svc.handleList(&ListOptions{GUI: true})
	if err != nil {
		t.Fatalf("handle list gui: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "picoclaw") || strings.Contains(got, "chlog") {
		t.Fatalf("expected only gui package output, got %q", got)
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

	var out bytes.Buffer
	cliui.SetOutput(&out)
	defer cliui.ResetOutput()

	err := svc.handleList(&ListOptions{Info: "chlog"})
	if err != nil {
		t.Fatalf("handle list info: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "Package Info") || !strings.Contains(got, "Name") || !strings.Contains(got, "chlog") {
		t.Fatalf("expected detail output, got %q", got)
	}
	if !strings.Contains(got, "Version") || !strings.Contains(got, "v0.3.6") {
		t.Fatalf("expected version detail output, got %q", got)
	}
	if !strings.Contains(got, "URL") || !strings.Contains(got, "https://github.com/gookit/gitw/releases/download/v0.3.6/chlog-windows-amd64.exe") {
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

func TestHandleUpdateRejectsUnimplementedDryRunAndInteractive(t *testing.T) {
	svc := &cliService{}

	err := svc.handleUpdate(&UpdateOptions{DryRun: true, Target: "junegunn/fzf"})
	if err == nil || !strings.Contains(err.Error(), "update --dry-run is not implemented") {
		t.Fatalf("expected dry-run unsupported error, got %v", err)
	}

	err = svc.handleUpdate(&UpdateOptions{Interactive: true, Target: "junegunn/fzf"})
	if err == nil || !strings.Contains(err.Error(), "update --interactive is not implemented") {
		t.Fatalf("expected interactive unsupported error, got %v", err)
	}
}

func TestHandleUpdateAllPrintsCandidatesAndUpdatesOnlyOutdated(t *testing.T) {
	installer := &fakeUpdateInstallerForCLI{}
	svc := &cliService{
		updService: app.UpdateService{
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
			LatestTag: func(repo, _ string) (string, error) {
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
		},
	}

	var out bytes.Buffer
	ccolor.SetOutput(&out)
	defer ccolor.SetOutput(os.Stdout)

	err := svc.handleUpdate(&UpdateOptions{All: true})
	if err != nil {
		t.Fatalf("handle update all: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "BurntSushi/ripgrep") || !strings.Contains(got, "v13.0.0") || !strings.Contains(got, "v14.0.0") {
		t.Fatalf("expected outdated candidate output, got %q", got)
	}
	if strings.Contains(got, "junegunn/fzf") {
		t.Fatalf("expected up-to-date repo to be omitted, got %q", got)
	}
	if len(installer.targets) != 1 || installer.targets[0] != "rg" {
		t.Fatalf("expected only rg to be updated, got %#v", installer.targets)
	}
}

func TestHandleUpdateCheckPrintsSameOutdatedListWithoutUpdating(t *testing.T) {
	installer := &fakeUpdateInstallerForCLI{}
	svc := &cliService{
		listService: app.ListService{
			LatestTag: func(repo, _ string) (string, error) {
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
				return &storepkg.Config{Installed: map[string]storepkg.Entry{
					"BurntSushi/ripgrep": {Repo: "BurntSushi/ripgrep", Tag: "v13.0.0"},
					"junegunn/fzf":       {Repo: "junegunn/fzf", Tag: "v0.50.0"},
				}}, nil
			},
		},
		updService: app.UpdateService{
			Install: installer,
		},
	}

	var out bytes.Buffer
	ccolor.SetOutput(&out)
	defer ccolor.SetOutput(os.Stdout)

	err := svc.handleUpdate(&UpdateOptions{Check: true})
	if err != nil {
		t.Fatalf("handle update check: %v", err)
	}

	got := out.String()
	if !strings.Contains(strings.ToLower(got), "latest version") {
		t.Fatalf("expected outdated table output, got %q", got)
	}
	if !strings.Contains(got, "BurntSushi/ripgrep") || !strings.Contains(got, "v14.0.0") {
		t.Fatalf("expected outdated repo in output, got %q", got)
	}
	if strings.Contains(got, "junegunn/fzf") {
		t.Fatalf("expected up-to-date repo to be omitted, got %q", got)
	}
	if len(installer.targets) != 0 {
		t.Fatalf("expected update --check not to update packages, got %#v", installer.targets)
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

func TestHandleConfigInitTreatsBlankOverwriteConfirmationAsCancel(t *testing.T) {
	svc := &cliService{
		cfgService: app.ConfigService{
			ConfigPath: "testdata/eget.toml",
			Load: func() (*cfgpkg.File, error) {
				return cfgpkg.NewFile(), nil
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

	if _, err := w.WriteString("\n"); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	_ = w.Close()

	err = svc.handleConfig(&ConfigOptions{Action: "init"})
	if err == nil {
		t.Fatal("expected blank confirmation to cancel")
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

func TestPrintQueryResultAssets(t *testing.T) {
	result := app.QueryResult{
		Action: "assets",
		Repo:   "owner/repo",
		Tag:    "v1.2.3",
		Assets: []app.QueryAsset{{
			Name: "tool-linux-amd64.tar.gz",
			URL:  "https://example.com/tool",
		}},
	}

	var out bytes.Buffer
	ccolor.SetOutput(&out)
	defer ccolor.SetOutput(os.Stdout)

	printQueryResult(result)
	if !strings.Contains(out.String(), "tool-linux-amd64.tar.gz") {
		t.Fatalf("expected asset table output, got %q", out.String())
	}
}

func TestHandleSearchPrintsList(t *testing.T) {
	svc := &cliService{
		searchService: app.SearchService{
			Client: &fakeSearchClientForCLI{
				result: app.SearchResult{
					TotalCount: 1,
					Items: []app.SearchRepo{{
						FullName:        "BurntSushi/ripgrep",
						Description:     "ripgrep recursively searches directories",
						StargazersCount: 123,
						Language:        "Rust",
						UpdatedAt:       time.Date(2026, 4, 24, 8, 30, 0, 0, time.UTC),
					}},
				},
			},
		},
	}

	var out bytes.Buffer
	ccolor.SetOutput(&out)
	defer ccolor.SetOutput(os.Stdout)

	err := svc.handle("search", &SearchOptions{Keyword: "ripgrep", Extras: []string{"language:rust"}})
	if err != nil {
		t.Fatalf("handle search: %v", err)
	}

	got := out.String()
	if strings.Contains(strings.ToLower(got), "repo |") || strings.Contains(strings.ToLower(got), "language |") {
		t.Fatalf("expected search to not render a table, got %q", got)
	}
	if !strings.Contains(got, "BurntSushi/ripgrep") || !strings.Contains(got, "⭐123 language: Rust update: 2026-04-24T08:30:00Z") {
		t.Fatalf("expected formatted search headline, got %q", got)
	}
	if !strings.Contains(got, "ripgrep recursively searches directories") {
		t.Fatalf("expected description line, got %q", got)
	}
}

func TestHandleSearchJSONOutput(t *testing.T) {
	svc := &cliService{
		searchService: app.SearchService{
			Client: &fakeSearchClientForCLI{
				result: app.SearchResult{
					TotalCount: 1,
					Items: []app.SearchRepo{{
						FullName:        "BurntSushi/ripgrep",
						StargazersCount: 321,
					}},
				},
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

	err = svc.handle("search", &SearchOptions{Keyword: "ripgrep", JSON: true})
	if err != nil {
		t.Fatalf("handle search json: %v", err)
	}

	_ = w.Close()
	var out bytes.Buffer
	if _, err := io.Copy(&out, r); err != nil {
		t.Fatalf("copy stdout: %v", err)
	}
	if !strings.Contains(out.String(), `"total_count": 1`) || !strings.Contains(out.String(), `"full_name": "BurntSushi/ripgrep"`) {
		t.Fatalf("expected search json output, got %q", out.String())
	}
}

func TestHandleSearchPassesOptionsToSearchService(t *testing.T) {
	fakeClient := &fakeSearchClientForCLI{
		result: app.SearchResult{},
	}
	svc := &cliService{
		searchService: app.SearchService{
			Client: fakeClient,
		},
	}

	err := svc.handle("search", &SearchOptions{
		Keyword: "ripgrep",
		Limit:   20,
		Sort:    "stars",
		Order:   "desc",
		Extras:  []string{"language:rust", "topic:cli"},
	})
	if err != nil {
		t.Fatalf("handle search: %v", err)
	}

	if fakeClient.query != "ripgrep language:rust topic:cli" {
		t.Fatalf("expected merged query to propagate, got %q", fakeClient.query)
	}
	if fakeClient.limit != 20 {
		t.Fatalf("expected limit to propagate, got %d", fakeClient.limit)
	}
	if fakeClient.sort != "stars" {
		t.Fatalf("expected sort to propagate, got %q", fakeClient.sort)
	}
	if fakeClient.order != "desc" {
		t.Fatalf("expected order to propagate, got %q", fakeClient.order)
	}
}

type fakeRunnerForCLI struct {
	result app.RunResult
}

func (f *fakeRunnerForCLI) Run(target string, opts install.Options) (app.RunResult, error) {
	return f.result, nil
}

type fakeUpdateInstallerForCLI struct {
	targets []string
}

func (f *fakeUpdateInstallerForCLI) InstallTarget(target string, opts install.Options, extras ...app.InstallExtras) (app.RunResult, error) {
	f.targets = append(f.targets, target)
	return app.RunResult{}, nil
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

type fakeSearchClientForCLI struct {
	result app.SearchResult
	err    error
	query  string
	limit  int
	sort   string
	order  string
}

func (f *fakeSearchClientForCLI) SearchRepositories(query string, limit int, sort, order string) (app.SearchResult, error) {
	f.query = query
	f.limit = limit
	f.sort = sort
	f.order = order
	if f.err != nil {
		return app.SearchResult{}, f.err
	}
	return f.result, nil
}
