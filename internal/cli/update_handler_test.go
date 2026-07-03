package cli

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/gookit/goutil/x/assert"
	"github.com/gookit/goutil/x/ccolor"
	"github.com/inherelab/eget/internal/app"
	cfgpkg "github.com/inherelab/eget/internal/config"
	storepkg "github.com/inherelab/eget/internal/installed"
	"github.com/inherelab/eget/internal/util"
)

func TestHandleUpdateUpdatesMultipleTargets(t *testing.T) {
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
			LatestInfo: func(target app.LatestCheckTarget) (app.LatestInfo, error) {
				repo := target.Repo
				switch repo {
				case "junegunn/fzf":
					return app.LatestInfo{Tag: "v0.51.0"}, nil
				case "BurntSushi/ripgrep":
					return app.LatestInfo{Tag: "v14.0.0"}, nil
				default:
					t.Fatalf("unexpected latest check for %s", repo)
					return app.LatestInfo{}, nil
				}
			},
		},
	}

	err := svc.handle("update", &UpdateOptions{Targets: []string{"fzf", "rg"}, Quiet: true})
	assert.NoErr(t, err)
	assert.Eq(t, []string{"fzf", "rg"}, installer.targets)
	if len(installer.options) != 2 || !installer.options[0].Quiet || !installer.options[1].Quiet {
		t.Fatalf("expected cli update options to propagate, got %#v", installer.options)
	}
}

func TestHandleUpdateSeparatesMultipleTargetOutput(t *testing.T) {
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
			LatestInfo: func(target app.LatestCheckTarget) (app.LatestInfo, error) {
				return app.LatestInfo{Tag: "v99.0.0"}, nil
			},
		},
	}

	var out bytes.Buffer
	ccolor.SetOutput(&out)
	defer ccolor.SetOutput(os.Stdout)

	err := svc.handleUpdate(&UpdateOptions{Targets: []string{"fzf", "rg"}})

	assert.NoErr(t, err)
	got := ccolor.ClearCode(out.String())
	assert.Eq(t, 1, countSeparatorLines(got))
}

func TestHandleUpdateWarnsAndContinuesAfterTargetFailure(t *testing.T) {
	installer := &fakeUpdateInstallerForCLI{
		errByTarget: map[string]error{
			"codex": errors.New("file is being used by another process"),
		},
	}
	svc := &cliService{
		stderr: &bytes.Buffer{},
		updService: app.UpdateService{
			Install: installer,
			LoadConfig: func() (*cfgpkg.File, error) {
				cfg := cfgpkg.NewFile()
				cfg.Packages["codex"] = cfgpkg.Section{Repo: util.StringPtr("openai/codex")}
				cfg.Packages["dbx"] = cfgpkg.Section{Repo: util.StringPtr("owner/dbx")}
				cfg.Packages["uv"] = cfgpkg.Section{Repo: util.StringPtr("astral-sh/uv")}
				return cfg, nil
			},
			LoadInstalled: func() (*storepkg.Config, error) {
				return &storepkg.Config{Installed: map[string]storepkg.Entry{
					"openai/codex": {Repo: "openai/codex", Tag: "v1.0.0"},
					"owner/dbx":    {Repo: "owner/dbx", Tag: "v1.0.0"},
					"astral-sh/uv": {Repo: "astral-sh/uv", Tag: "v1.0.0"},
				}}, nil
			},
			LatestInfo: func(target app.LatestCheckTarget) (app.LatestInfo, error) {
				return app.LatestInfo{Tag: "v2.0.0"}, nil
			},
		},
	}

	err := svc.handleUpdate(&UpdateOptions{Targets: []string{"codex", "dbx", "uv"}})

	assert.Err(t, err)
	assert.Contains(t, err.Error(), "1 update failed")
	assert.Eq(t, []string{"codex", "dbx", "uv"}, installer.targets)
	gotErr := ccolor.ClearCode(svc.stderr.(*bytes.Buffer).String())
	assert.Contains(t, gotErr, "update_failed codex")
	assert.Contains(t, gotErr, "file is being used by another process")
}

func TestHandleUpdatePrintsAlreadyUpToDateTarget(t *testing.T) {
	installer := &fakeUpdateInstallerForCLI{}
	svc := &cliService{
		updService: app.UpdateService{
			Install: installer,
			LoadConfig: func() (*cfgpkg.File, error) {
				cfg := cfgpkg.NewFile()
				cfg.Packages["xenv"] = cfgpkg.Section{Repo: util.StringPtr("inhere/xenv")}
				return cfg, nil
			},
			LoadInstalled: func() (*storepkg.Config, error) {
				return &storepkg.Config{Installed: map[string]storepkg.Entry{
					"inhere/xenv": {Repo: "inhere/xenv", Tag: "v1.2.3"},
				}}, nil
			},
			LatestInfo: func(target app.LatestCheckTarget) (app.LatestInfo, error) {
				assert.Eq(t, "inhere/xenv", target.Repo)
				return app.LatestInfo{Tag: "v1.2.3"}, nil
			},
		},
	}

	var out bytes.Buffer
	ccolor.SetOutput(&out)
	defer ccolor.SetOutput(os.Stdout)

	err := svc.handleUpdate(&UpdateOptions{Targets: []string{"xenv"}})

	assert.NoErr(t, err)
	assert.Eq(t, 0, len(installer.targets))
	got := ccolor.ClearCode(out.String())
	assert.Contains(t, got, "xenv is already up to date")
	assert.Contains(t, got, "v1.2.3")
}

func TestHandleUpdateRejectsUnimplementedDryRun(t *testing.T) {
	svc := &cliService{}

	err := svc.handleUpdate(&UpdateOptions{DryRun: true, Targets: []string{"junegunn/fzf"}})
	if err == nil || !strings.Contains(err.Error(), "update --dry-run is not implemented") {
		t.Fatalf("expected dry-run unsupported error, got %v", err)
	}
}

func TestHandleUpdateSelfRejectsTargets(t *testing.T) {
	svc := &cliService{}

	err := svc.handleUpdate(&UpdateOptions{Self: true, Targets: []string{"junegunn/fzf"}})

	assert.Err(t, err)
	assert.Contains(t, err.Error(), "update --self cannot be used with target")
}

func TestHandleUpdateSelfRejectsAll(t *testing.T) {
	svc := &cliService{}

	err := svc.handleUpdate(&UpdateOptions{Self: true, All: true})

	assert.Err(t, err)
	assert.Contains(t, err.Error(), "update --self cannot be used with --all")
}

func TestHandleUpdateSelfRejectsInteractive(t *testing.T) {
	svc := &cliService{}

	err := svc.handleUpdate(&UpdateOptions{Self: true, Interactive: true})

	assert.Err(t, err)
	assert.Contains(t, err.Error(), "update --self cannot be used with --interactive")
}

func TestHandleUpdateSelfRunsSelfUpdateService(t *testing.T) {
	fake := &fakeSelfUpdateCLIService{
		result: app.SelfUpdateResult{CurrentVersion: "1.7.1", LatestVersion: "v1.7.2", Updated: true},
	}
	svc := &cliService{
		selfUpdateService: fake,
		stderr:            io.Discard,
		lookupEnv:         func(string) (string, bool) { return "", false },
	}

	err := svc.handleUpdate(&UpdateOptions{Self: true, Tag: "v1.7.2", Asset: "custom,linux", Quiet: true})

	assert.NoErr(t, err)
	assert.Eq(t, "v1.7.2", fake.opts.Tag)
	assert.Eq(t, []string{"custom", "linux"}, fake.opts.Asset)
	assert.True(t, fake.opts.Install.Quiet)
}

func TestHandleUpdateSelfPassesSourceFromFlag(t *testing.T) {
	fake := &fakeSelfUpdateCLIService{
		result: app.SelfUpdateResult{CurrentVersion: "1.7.1", LatestVersion: "1.7.1-18-gabcd1234", Updated: true},
	}
	svc := &cliService{
		selfUpdateService: fake,
		stderr:            io.Discard,
		lookupEnv:         func(string) (string, bool) { return "", false },
	}

	err := svc.handleUpdate(&UpdateOptions{Self: true, SelfSource: "https://example.com/tools/eget/"})

	assert.NoErr(t, err)
	assert.Eq(t, "https://example.com/tools/eget/", fake.opts.Source)
}

func TestHandleUpdateSelfPassesSourceFromEnv(t *testing.T) {
	fake := &fakeSelfUpdateCLIService{
		result: app.SelfUpdateResult{CurrentVersion: "1.7.1", LatestVersion: "1.7.1-18-gabcd1234", Updated: true},
	}
	svc := &cliService{
		selfUpdateService: fake,
		stderr:            io.Discard,
		lookupEnv: func(key string) (string, bool) {
			if key == "EGET_SELF_UPDATE_SOURCE" {
				return "https://example.com/tools/eget/", true
			}
			return "", false
		},
	}

	err := svc.handleUpdate(&UpdateOptions{Self: true})

	assert.NoErr(t, err)
	assert.Eq(t, "https://example.com/tools/eget/", fake.opts.Source)
}

func TestHandleUpdateSelfCheckPrintsCheckSource(t *testing.T) {
	fake := &fakeSelfUpdateCLIService{
		result: app.SelfUpdateResult{CurrentVersion: "1.7.1", LatestVersion: "1.7.1", Updated: false},
	}
	svc := &cliService{selfUpdateService: fake, stderr: io.Discard}

	var out bytes.Buffer
	ccolor.SetOutput(&out)
	defer ccolor.SetOutput(os.Stdout)

	err := svc.handleUpdate(&UpdateOptions{Self: true, Check: true, SelfSource: "https://example.com/tools/eget/latest.yaml"})

	assert.NoErr(t, err)
	got := out.String()
	assert.Contains(t, got, "Checking self update from: example.com")
	assert.Contains(t, got, "https://example.com/tools/eget/latest.yaml")
}

func TestHandleUpdateSelfCheckPrintsDefaultGitHubSource(t *testing.T) {
	fake := &fakeSelfUpdateCLIService{
		result: app.SelfUpdateResult{CurrentVersion: "1.7.1", LatestVersion: "1.7.1", Updated: false},
	}
	svc := &cliService{selfUpdateService: fake, stderr: io.Discard}

	var out bytes.Buffer
	ccolor.SetOutput(&out)
	defer ccolor.SetOutput(os.Stdout)

	err := svc.handleUpdate(&UpdateOptions{Self: true, Check: true})

	assert.NoErr(t, err)
	got := out.String()
	assert.Contains(t, got, "Checking self update from: github.com")
	assert.Contains(t, got, app.SelfUpdateRepo)
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
			LatestInfo: func(target app.LatestCheckTarget) (app.LatestInfo, error) {
				repo := target.Repo
				switch repo {
				case "junegunn/fzf":
					return app.LatestInfo{Tag: "v0.50.0"}, nil
				case "BurntSushi/ripgrep":
					return app.LatestInfo{Tag: "v14.0.0"}, nil
				default:
					t.Fatalf("unexpected latest tag check for %s", repo)
					return app.LatestInfo{}, nil
				}
			},
		},
	}

	var out bytes.Buffer
	ccolor.SetOutput(&out)
	defer ccolor.SetOutput(os.Stdout)

	err := svc.handleUpdate(&UpdateOptions{All: true, BatchConcurrency: -1})
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

func TestHandleUpdateAllSeparatesMultiplePackageOutput(t *testing.T) {
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
			LatestInfo: func(target app.LatestCheckTarget) (app.LatestInfo, error) {
				return app.LatestInfo{Tag: "v99.0.0"}, nil
			},
		},
	}

	var out bytes.Buffer
	ccolor.SetOutput(&out)
	defer ccolor.SetOutput(os.Stdout)

	err := svc.handleUpdate(&UpdateOptions{All: true, BatchConcurrency: -1})

	assert.NoErr(t, err)
	got := ccolor.ClearCode(out.String())
	assert.Eq(t, 1, countSeparatorLines(got))
	assert.Eq(t, []string{"fzf", "rg"}, installer.targets)
}

func TestHandleUpdateInteractiveSelectsOutdatedPackages(t *testing.T) {
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
			LatestInfo: func(target app.LatestCheckTarget) (app.LatestInfo, error) {
				switch target.Repo {
				case "junegunn/fzf":
					return app.LatestInfo{Tag: "v0.51.0"}, nil
				case "BurntSushi/ripgrep":
					return app.LatestInfo{Tag: "v14.0.0"}, nil
				default:
					t.Fatalf("unexpected latest tag check for %s", target.Repo)
					return app.LatestInfo{}, nil
				}
			},
		},
	}

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
	if _, err := writer.WriteString("2\n"); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close stdin writer: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	ccolor.SetOutput(&out)
	defer ccolor.SetOutput(os.Stdout)
	origStderr := os.Stderr
	errReader, errWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stderr pipe: %v", err)
	}
	os.Stderr = errWriter
	defer func() {
		os.Stderr = origStderr
		_ = errReader.Close()
		_ = errWriter.Close()
	}()

	err = svc.handleUpdate(&UpdateOptions{Interactive: true, BatchConcurrency: -1})
	if err != nil {
		t.Fatalf("handle interactive update: %v", err)
	}
	if err := errWriter.Close(); err != nil {
		t.Fatalf("close stderr writer: %v", err)
	}
	rendered, err := io.ReadAll(errReader)
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	errOut.Write(rendered)

	assert.Eq(t, []string{"rg"}, installer.targets)
	gotPrompt := ccolor.ClearCode(errOut.String())
	assert.Contains(t, gotPrompt, "fzf  v0.50.0 -> v0.51.0")
	assert.Contains(t, gotPrompt, "rg  v13.0.0 -> v14.0.0")
	assert.NotContains(t, gotPrompt, "junegunn/fzf")
	assert.NotContains(t, gotPrompt, "BurntSushi/ripgrep")
}

func countSeparatorLines(text string) int {
	count := 0
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "---" {
			count++
		}
	}
	return count
}

func TestHandleUpdateCheckPrintsSameOutdatedListWithoutUpdating(t *testing.T) {
	installer := &fakeUpdateInstallerForCLI{}
	svc := &cliService{
		listService: app.ListService{
			LatestInfo: func(target app.LatestCheckTarget) (app.LatestInfo, error) {
				repo := target.Repo
				switch repo {
				case "BurntSushi/ripgrep":
					return app.LatestInfo{Tag: "v14.0.0"}, nil
				case "junegunn/fzf":
					return app.LatestInfo{Tag: "v0.50.0"}, nil
				default:
					return app.LatestInfo{}, nil
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

func TestHandleUpdateCheckWithTargetsOnlyChecksRequestedPackages(t *testing.T) {
	installer := &fakeUpdateInstallerForCLI{}
	checkedRepos := make([]string, 0, 2)
	svc := &cliService{
		listService: app.ListService{
			LatestInfo: func(target app.LatestCheckTarget) (app.LatestInfo, error) {
				t.Fatalf("expected targeted update --check to use update service")
				return app.LatestInfo{}, nil
			},
		},
		updService: app.UpdateService{
			Install: installer,
			LoadConfig: func() (*cfgpkg.File, error) {
				cfg := cfgpkg.NewFile()
				cfg.Packages["jq"] = cfgpkg.Section{Repo: util.StringPtr("jqlang/jq")}
				cfg.Packages["markview"] = cfgpkg.Section{Repo: util.StringPtr("OXY2DEV/markview.nvim")}
				cfg.Packages["rg"] = cfgpkg.Section{Repo: util.StringPtr("BurntSushi/ripgrep")}
				return cfg, nil
			},
			LoadInstalled: func() (*storepkg.Config, error) {
				return &storepkg.Config{Installed: map[string]storepkg.Entry{
					"jqlang/jq":             {Repo: "jqlang/jq", Tag: "jq-1.7"},
					"OXY2DEV/markview.nvim": {Repo: "OXY2DEV/markview.nvim", Tag: "v1.0.0"},
					"BurntSushi/ripgrep":    {Repo: "BurntSushi/ripgrep", Tag: "v13.0.0"},
				}}, nil
			},
			LatestInfo: func(target app.LatestCheckTarget) (app.LatestInfo, error) {
				checkedRepos = append(checkedRepos, target.Repo)
				switch target.Repo {
				case "jqlang/jq":
					return app.LatestInfo{Tag: "jq-1.8"}, nil
				case "OXY2DEV/markview.nvim":
					return app.LatestInfo{Tag: "v1.0.0"}, nil
				default:
					t.Fatalf("unexpected latest check for %s", target.Repo)
					return app.LatestInfo{}, nil
				}
			},
		},
	}

	var out bytes.Buffer
	ccolor.SetOutput(&out)
	defer ccolor.SetOutput(os.Stdout)

	err := svc.handleUpdate(&UpdateOptions{Check: true, Targets: []string{"markview", "jq"}})
	assert.NoErr(t, err)

	got := out.String()
	assert.Eq(t, []string{"OXY2DEV/markview.nvim", "jqlang/jq"}, checkedRepos)
	assert.Contains(t, got, "jqlang/jq")
	assert.Contains(t, got, "jq-1.8")
	assert.NotContains(t, got, "BurntSushi/ripgrep")
	assert.Eq(t, 0, len(installer.targets))
}
