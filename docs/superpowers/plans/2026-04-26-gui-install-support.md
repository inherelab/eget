# GUI Install Support Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement GUI application support with `global.gui_target`, package `is_gui`, `install --gui`, `add --gui`, `list --gui`, installer launch support, and installed-store GUI metadata.

**Architecture:** App resolves config and CLI GUI intent; runner confirms `install_mode` after selecting the asset or archive file. Portable GUI installs use `global.gui_target` only when `--to` is absent; installer GUI installs start an external installer through an injectable launcher and record installed metadata without a final app directory.

**Tech Stack:** Go 1.24, `gookit/config/v2`, `gookit/goutil/cflag/capp`, existing `internal/app`, `internal/install`, `internal/installed`, `internal/cli`, and `internal/config` layers.

---

## File Structure

- Modify `internal/config/model.go`, `internal/config/merge.go`, `internal/config/gookit.go`: add `gui_target`, `is_gui`, and merge support.
- Modify `internal/cli/install_cmd.go`, `internal/cli/add_cmd.go`, `internal/cli/list_cmd.go`: add `--gui` where supported.
- Modify `internal/cli/service.go`: map GUI flags and filter `list --gui`.
- Modify `internal/install/options.go`: add runtime GUI fields and install-mode constants.
- Create `internal/install/gui.go`: add installer detection, installer kind, and launcher interface.
- Modify `internal/install/runner.go`: confirm GUI install mode, choose GUI output, materialize installers, and launch installers.
- Modify `internal/installed/model.go`, `internal/installed/gookit.go`: persist GUI metadata.
- Modify `internal/app/config.go`, `internal/app/install.go`, `internal/app/update.go`, `internal/app/list.go`: resolve GUI config, record metadata, and list GUI packages.
- Update tests under `internal/config`, `internal/cli`, `internal/app`, `internal/install`, and `internal/installed`.
- Update `README.md`, `README.zh-CN.md`, `docs/DOCS.md`, and `docs/example.eget.toml`.

## Task 1: Config Model

**Files:**
- Modify: `internal/config/model.go`
- Modify: `internal/config/merge.go`
- Modify: `internal/config/gookit.go`
- Test: `internal/config/gookit_test.go`
- Test: `internal/config/loader_test.go`
- Test: `internal/config/merge_test.go`

- [x] **Step 1: Write failing config tests**

In `internal/config/gookit_test.go`, extend `TestPathGetAndSet` to set and read `global.gui_target` plus `packages.fzf.is_gui`:

```go
if err := SetByPath(cfg, "global.gui_target", "~/Applications"); err != nil {
	t.Fatalf("set global.gui_target: %v", err)
}
if err := SetByPath(cfg, "packages.fzf.is_gui", "true"); err != nil {
	t.Fatalf("set packages.fzf.is_gui: %v", err)
}
guiTarget, ok := GetByPath(cfg, "global.gui_target")
if !ok || guiTarget != "~/Applications" {
	t.Fatalf("expected global.gui_target to be set, got %#v ok=%t", guiTarget, ok)
}
pkg := cfg.Packages["fzf"]
if pkg.IsGUI == nil || !*pkg.IsGUI {
	t.Fatalf("expected package is_gui to be parsed, got %#v", pkg.IsGUI)
}
```

In `TestDumpConfigStringKeepsLegacyRepoSections`, set `IsGUI: boolPtr(true)` on one section and assert:

```go
if !strings.Contains(text, "is_gui = true") {
	t.Fatalf("expected is_gui field, got %q", text)
}
```

In `internal/config/loader_test.go`, extend `TestLoadFileSupportsLegacyRepoSections` TOML:

```toml
[global]
target = "~/bin"
gui_target = "~/Applications"
quiet = true
github_token = "token"

["owner/repo"]
asset_filters = ["linux", "!arm"]
download_only = true
extract_all = true
is_gui = true
```

Add assertions:

```go
if cfg.Global.GuiTarget == nil || *cfg.Global.GuiTarget != "~/Applications" {
	t.Fatalf("expected global gui_target to be loaded, got %#v", cfg.Global.GuiTarget)
}
if repo.IsGUI == nil || !*repo.IsGUI {
	t.Fatalf("expected repo is_gui=true, got %#v", repo.IsGUI)
}
```

In `internal/config/merge_test.go`, add:

```go
func TestMergeInstallOptionsUsesGUIFromCLIThenPackageThenRepo(t *testing.T) {
	merged := MergeInstallOptions(Section{}, Section{IsGUI: boolPtr(true)}, Section{}, CLIOverrides{})
	if !merged.IsGUI {
		t.Fatalf("expected repo is_gui to apply, got %#v", merged)
	}
	merged = MergeInstallOptions(Section{}, Section{IsGUI: boolPtr(false)}, Section{IsGUI: boolPtr(true)}, CLIOverrides{})
	if !merged.IsGUI {
		t.Fatalf("expected package is_gui to override repo, got %#v", merged)
	}
	merged = MergeInstallOptions(Section{}, Section{IsGUI: boolPtr(true)}, Section{IsGUI: boolPtr(true)}, CLIOverrides{IsGUI: boolPtr(false)})
	if merged.IsGUI {
		t.Fatalf("expected cli is_gui=false to override config, got %#v", merged)
	}
}
```

- [x] **Step 2: Run tests to verify failure**

Run: `go test ./internal/config -run 'TestPathGetAndSet|TestDumpConfigStringKeepsLegacyRepoSections|TestLoadFileSupportsLegacyRepoSections|TestMergeInstallOptions' -v`

Expected: FAIL because `GuiTarget`, `IsGUI`, and `CLIOverrides.IsGUI` do not exist.

- [x] **Step 3: Implement config fields**

Update `internal/config/model.go`:

```go
GuiTarget *string `toml:"gui_target" mapstructure:"gui_target"`
IsGUI     *bool   `toml:"is_gui" mapstructure:"is_gui"`
```

Add `GuiTarget string` and `IsGUI bool` to `Merged`, and `IsGUI *bool` to `CLIOverrides`.

Update `internal/config/merge.go`:

```go
merged.IsGUI = firstBool(cli.IsGUI, pkg.IsGUI, repo.IsGUI)
merged.GuiTarget = firstString(global.GuiTarget)
```

Update `internal/config/gookit.go` bool normalization:

```go
case "extract_all", "is_gui", "download_only", "quiet", "show_hash", "download_source", "upgrade_only", "disable_ssl", "enable", "support_api":
```

Update `sectionToMap`:

```go
if section.GuiTarget != nil {
	data["gui_target"] = *section.GuiTarget
}
if section.IsGUI != nil {
	data["is_gui"] = *section.IsGUI
}
```

- [x] **Step 4: Run config tests**

Run: `go test ./internal/config -v`

Expected: PASS.

- [x] **Step 5: Commit**

```bash
git add internal/config/model.go internal/config/merge.go internal/config/gookit.go internal/config/gookit_test.go internal/config/loader_test.go internal/config/merge_test.go
git commit -m "feat(config): add gui target and package flag"
```

## Task 2: CLI GUI Flags

**Files:**
- Modify: `internal/cli/install_cmd.go`
- Modify: `internal/cli/add_cmd.go`
- Modify: `internal/cli/list_cmd.go`
- Test: `internal/cli/app_test.go`

- [x] **Step 1: Write failing CLI binding tests**

Add to `internal/cli/app_test.go`:

```go
func TestMain_GUIFlagBindsInstallAndAdd(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"install gui", []string{"install", "--gui", "inhere/markview"}, "install"},
		{"add gui", []string{"add", "--gui", "inhere/markview"}, "add"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := make([]commandCall, 0, 1)
			handler := func(name string, options any) error {
				calls = append(calls, commandCall{name: name, options: options})
				return nil
			}
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			err := newApp(handler, &stdout, &stderr).RunWithArgs(tt.args)
			if err != nil {
				t.Fatalf("expected %s command to parse, got %v", tt.name, err)
			}
			if len(calls) != 1 || calls[0].name != tt.want {
				t.Fatalf("unexpected routed call: %#v", calls)
			}
			switch opts := calls[0].options.(type) {
			case *InstallOptions:
				if !opts.GUI {
					t.Fatalf("expected install gui flag to be true")
				}
			case *AddOptions:
				if !opts.GUI {
					t.Fatalf("expected add gui flag to be true")
				}
			default:
				t.Fatalf("unexpected options type %T", calls[0].options)
			}
		})
	}
}

func TestMain_DownloadRejectsGUIFlag(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := newApp(func(string, any) error { return nil }, &stdout, &stderr).RunWithArgs([]string{"download", "--gui", "inhere/markview"})
	if err == nil {
		t.Fatal("expected download --gui to be rejected")
	}
	if !strings.Contains(err.Error(), "gui") {
		t.Fatalf("expected error to mention gui, got %v", err)
	}
}

func TestMain_ListGUIBindsOption(t *testing.T) {
	calls := make([]commandCall, 0, 1)
	handler := func(name string, options any) error {
		calls = append(calls, commandCall{name: name, options: options})
		return nil
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := newApp(handler, &stdout, &stderr).RunWithArgs([]string{"list", "--gui"})
	if err != nil {
		t.Fatalf("expected list --gui command to parse, got %v", err)
	}
	opts := calls[0].options.(*ListOptions)
	if !opts.GUI {
		t.Fatalf("expected gui flag to be true")
	}
}
```

- [x] **Step 2: Run tests to verify failure**

Run: `go test ./internal/cli -run 'TestMain_GUIFlag|TestMain_DownloadRejectsGUIFlag|TestMain_ListGUIBindsOption' -v`

Expected: FAIL because `GUI` fields and flags are missing.

- [x] **Step 3: Add CLI flags**

Add `GUI bool` to `InstallOptions`, `AddOptions`, and `ListOptions`.

Register flags:

```go
cmd.BoolVar(&opts.GUI, "gui", false, "Install as GUI application")
cmd.BoolVar(&opts.GUI, "gui", false, "Add as GUI application")
cmd.BoolVar(&opts.GUI, "gui", false, "List GUI applications")
```

Do not add `GUI` to `DownloadOptions`.

- [x] **Step 4: Run tests**

Run: `go test ./internal/cli -run 'TestMain_GUIFlag|TestMain_DownloadRejectsGUIFlag|TestMain_ListGUIBindsOption' -v`

Expected: PASS.

- [x] **Step 5: Commit**

```bash
git add internal/cli/install_cmd.go internal/cli/add_cmd.go internal/cli/list_cmd.go internal/cli/app_test.go
git commit -m "feat(cli): add gui flags"
```

## Task 3: Runtime and Installed Metadata

**Files:**
- Modify: `internal/install/options.go`
- Modify: `internal/installed/model.go`
- Modify: `internal/installed/gookit.go`
- Test: `internal/installed/store_test.go`

- [x] **Step 1: Write failing installed-store test**

Add to `internal/installed/store_test.go`:

```go
func TestStoreRoundTripGUIFields(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "installed.toml")
	store := Store{Path: path}
	err := store.Record("sipeed/picoclaw", Entry{
		Repo:        "sipeed/picoclaw",
		Target:      "sipeed/picoclaw",
		InstalledAt: time.Unix(1710000000, 0).UTC(),
		URL:         "https://github.com/sipeed/picoclaw/releases/download/v0.2.7/picoclaw-setup.exe",
		Asset:       "picoclaw-setup.exe",
		IsGUI:       true,
		InstallMode: "installer",
	})
	if err != nil {
		t.Fatalf("record gui install: %v", err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load gui install: %v", err)
	}
	entry := loaded.Installed["sipeed/picoclaw"]
	if !entry.IsGUI {
		t.Fatalf("expected is_gui to round-trip, got %#v", entry)
	}
	if entry.InstallMode != "installer" {
		t.Fatalf("expected install_mode installer, got %#v", entry.InstallMode)
	}
}
```

- [x] **Step 2: Run test to verify failure**

Run: `go test ./internal/installed -run TestStoreRoundTripGUIFields -v`

Expected: FAIL because `Entry.IsGUI` and `Entry.InstallMode` do not exist.

- [x] **Step 3: Add runtime fields**

Update `internal/install/options.go`:

```go
const (
	InstallModePortable  = "portable"
	InstallModeInstaller = "installer"
)
```

Add to `Options`:

```go
OutputExplicit bool
GuiTarget      string
IsGUI          bool
InstallMode    string
```

- [x] **Step 4: Add installed metadata fields**

Update `internal/installed/model.go`:

```go
IsGUI       bool   `toml:"is_gui,omitempty" mapstructure:"is_gui"`
InstallMode string `toml:"install_mode,omitempty" mapstructure:"install_mode"`
```

Update `entryToMap` in `internal/installed/gookit.go`:

```go
if entry.IsGUI {
	data["is_gui"] = true
}
if entry.InstallMode != "" {
	data["install_mode"] = entry.InstallMode
}
```

- [x] **Step 5: Run installed tests**

Run: `go test ./internal/installed -v`

Expected: PASS.

- [x] **Step 6: Commit**

```bash
git add internal/install/options.go internal/installed/model.go internal/installed/gookit.go internal/installed/store_test.go
git commit -m "feat(installed): record gui install metadata"
```

## Task 4: App GUI Resolution

**Files:**
- Modify: `internal/app/config.go`
- Modify: `internal/app/install.go`
- Modify: `internal/app/update.go`
- Modify: `internal/cli/service.go`
- Test: `internal/app/add_test.go`
- Test: `internal/app/install_test.go`

- [x] **Step 1: Write failing app tests**

In `internal/app/add_test.go`, include `IsGUI: true` in `TestAddPackageWritesManagedPackage` options and assert:

```go
if pkg.IsGUI == nil || !*pkg.IsGUI {
	t.Fatalf("expected is_gui to be persisted, got %#v", pkg.IsGUI)
}
```

In `internal/app/install_test.go`, update `fakeRunner` to support custom `result RunResult`, then add:

```go
func TestInstallTargetUsesGuiTargetForPortableGUI(t *testing.T) {
	runner := &fakeRunner{}
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

func TestInstallTargetRecordsGUIInstallerWithoutExtractedFiles(t *testing.T) {
	runner := &fakeRunner{result: RunResult{
		URL:           "https://github.com/sipeed/picoclaw/releases/download/v0.2.7/picoclaw-setup.exe",
		Asset:         "picoclaw-setup.exe",
		IsGUI:         true,
		InstallMode:   install.InstallModeInstaller,
		InstallerFile: "C:/Temp/picoclaw-setup.exe",
	}}
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
}
```

- [x] **Step 2: Run tests to verify failure**

Run: `go test ./internal/app -run 'TestAddPackageWritesManagedPackage|TestInstallTargetUsesGuiTargetForPortableGUI|TestInstallTargetRecordsGUIInstallerWithoutExtractedFiles' -v`

Expected: FAIL because GUI app plumbing is missing.

- [x] **Step 3: Implement app resolution**

In `internal/app/config.go`, write package GUI metadata:

```go
if opts.IsGUI {
	section.IsGUI = util.BoolPtr(true)
}
```

In `internal/app/install.go`, pass `IsGUI` into config merge:

```go
IsGUI: boolOpt(cli.IsGUI),
```

Expand `merged.GuiTarget` and return:

```go
OutputExplicit: cli.Output != "",
GuiTarget:      guiTarget,
IsGUI:          merged.IsGUI,
```

Change recording condition:

```go
shouldRecord := len(result.ExtractedFiles) > 0 || result.InstallMode == install.InstallModeInstaller
```

Set entry metadata:

```go
IsGUI:       result.IsGUI || opts.IsGUI,
InstallMode: result.InstallMode,
```

If `opts.IsGUI` and extracted files exist but `result.InstallMode` is empty, record `install.InstallModePortable`.

In `internal/app/update.go`, pass through `IsGUI`, `GuiTarget`, and `OutputExplicit`.

In `internal/cli/service.go`, map install/add `GUI` into `install.Options{IsGUI: opts.GUI}`.

- [x] **Step 4: Run app and CLI tests**

Run: `go test ./internal/app ./internal/cli -v`

Expected: PASS.

- [x] **Step 5: Commit**

```bash
git add internal/app/config.go internal/app/install.go internal/app/update.go internal/cli/service.go internal/app/add_test.go internal/app/install_test.go
git commit -m "feat(app): resolve gui install options"
```

## Task 5: Installer Detection and Launcher

**Files:**
- Create: `internal/install/gui.go`
- Test: `internal/install/gui_test.go`

- [x] **Step 1: Write failing install GUI tests**

Create `internal/install/gui_test.go`:

```go
package install

import (
	"runtime"
	"testing"
)

func TestDetectGUIInstallMode(t *testing.T) {
	tests := []struct {
		name  string
		isGUI bool
		file  string
		want  string
	}{
		{"non gui msi stays empty", false, "app.msi", ""},
		{"gui msi installer", true, "app.msi", InstallModeInstaller},
		{"gui setup exe installer", true, "PicoClaw-Setup.exe", InstallModeInstaller},
		{"gui installer exe installer", true, "foo-installer-x64.exe", InstallModeInstaller},
		{"gui plain exe portable", true, "picoclaw.exe", InstallModePortable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectGUIInstallMode(tt.isGUI, tt.file)
			if got != tt.want {
				t.Fatalf("DetectGUIInstallMode(%t, %q) = %q, want %q", tt.isGUI, tt.file, got, tt.want)
			}
		})
	}
}

func TestDefaultInstallerLauncherCommand(t *testing.T) {
	launcher := DefaultInstallerLauncher{GOOS: "windows"}
	cmd, args, err := launcher.command("C:/Temp/app.msi", InstallerKindMSI)
	if err != nil {
		t.Fatalf("msi command: %v", err)
	}
	if cmd != "msiexec" || len(args) != 2 || args[0] != "/i" || args[1] != "C:/Temp/app.msi" {
		t.Fatalf("unexpected msi command: %s %#v", cmd, args)
	}
	cmd, args, err = launcher.command("C:/Temp/setup.exe", InstallerKindEXE)
	if err != nil {
		t.Fatalf("exe command: %v", err)
	}
	if cmd != "C:/Temp/setup.exe" || len(args) != 0 {
		t.Fatalf("unexpected exe command: %s %#v", cmd, args)
	}
}

func TestDefaultInstallerLauncherRejectsUnsupportedPlatform(t *testing.T) {
	goos := runtime.GOOS
	if goos == "windows" {
		goos = "linux"
	}
	launcher := DefaultInstallerLauncher{GOOS: goos}
	if _, _, err := launcher.command("/tmp/app.msi", InstallerKindMSI); err == nil {
		t.Fatal("expected non-windows msi launcher to fail")
	}
}
```

- [x] **Step 2: Run tests to verify failure**

Run: `go test ./internal/install -run 'TestDetectGUIInstallMode|TestDefaultInstallerLauncher' -v`

Expected: FAIL because GUI detection and launcher do not exist.

- [x] **Step 3: Implement detection and launcher**

Create `internal/install/gui.go` with:

```go
package install

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type InstallerKind string

const (
	InstallerKindUnknown InstallerKind = ""
	InstallerKindMSI     InstallerKind = "msi"
	InstallerKindEXE     InstallerKind = "exe"
)

type InstallerLauncher interface {
	LaunchInstaller(path string, kind InstallerKind) error
}

type DefaultInstallerLauncher struct {
	GOOS string
}

func DetectGUIInstallMode(isGUI bool, fileName string) string {
	if !isGUI {
		return ""
	}
	if DetectInstallerKind(fileName) != InstallerKindUnknown {
		return InstallModeInstaller
	}
	return InstallModePortable
}

func DetectInstallerKind(fileName string) InstallerKind {
	lower := strings.ToLower(filepath.Base(fileName))
	switch {
	case strings.HasSuffix(lower, ".msi"):
		return InstallerKindMSI
	case strings.HasSuffix(lower, ".exe") && (strings.Contains(lower, "setup") || strings.Contains(lower, "installer")):
		return InstallerKindEXE
	default:
		return InstallerKindUnknown
	}
}

func (l DefaultInstallerLauncher) LaunchInstaller(path string, kind InstallerKind) error {
	cmdName, args, err := l.command(path, kind)
	if err != nil {
		return err
	}
	return exec.Command(cmdName, args...).Start()
}

func (l DefaultInstallerLauncher) command(path string, kind InstallerKind) (string, []string, error) {
	goos := l.GOOS
	if goos == "" {
		goos = runtime.GOOS
	}
	if goos != "windows" {
		return "", nil, fmt.Errorf("launching GUI installer %s is unsupported on %s", filepath.Base(path), goos)
	}
	switch kind {
	case InstallerKindMSI:
		return "msiexec", []string{"/i", path}, nil
	case InstallerKindEXE:
		return path, nil, nil
	default:
		return "", nil, fmt.Errorf("unsupported installer kind for %s", filepath.Base(path))
	}
}
```

- [x] **Step 4: Run tests**

Run: `go test ./internal/install -run 'TestDetectGUIInstallMode|TestDefaultInstallerLauncher' -v`

Expected: PASS.

- [x] **Step 5: Commit**

```bash
git add internal/install/gui.go internal/install/gui_test.go
git commit -m "feat(install): detect gui installer mode"
```

## Task 6: Runner GUI Flow

**Files:**
- Modify: `internal/install/runner.go`
- Test: `internal/install/runner_test.go`

- [x] **Step 1: Write failing runner helper tests**

Add to `internal/install/runner_test.go`:

```go
func TestEffectiveOutputUsesGuiTargetForPortableGUI(t *testing.T) {
	opts := Options{Output: "C:/Tools", GuiTarget: "C:/Program/AITools", IsGUI: true, InstallMode: InstallModePortable}
	got := effectiveOutput(opts)
	if got != "C:/Program/AITools" {
		t.Fatalf("expected gui target, got %q", got)
	}
}

func TestEffectiveOutputKeepsExplicitOutputForPortableGUI(t *testing.T) {
	opts := Options{Output: "D:/Custom/PicoClaw", GuiTarget: "C:/Program/AITools", IsGUI: true, InstallMode: InstallModePortable, OutputExplicit: true}
	got := effectiveOutput(opts)
	if got != "D:/Custom/PicoClaw" {
		t.Fatalf("expected explicit output, got %q", got)
	}
}
```

Add fake launcher and helper test:

```go
type fakeInstallerLauncher struct {
	path string
	kind InstallerKind
	err  error
}

func (f *fakeInstallerLauncher) LaunchInstaller(path string, kind InstallerKind) error {
	f.path = path
	f.kind = kind
	return f.err
}

func TestLaunchGUIInstallerReturnsInstallerResult(t *testing.T) {
	launcher := &fakeInstallerLauncher{}
	runner := &InstallRunner{InstallerLauncher: launcher}
	file := ExtractedFile{Name: "PicoClaw-Setup.exe", ArchiveName: "PicoClaw-Setup.exe"}
	path := filepath.Join(t.TempDir(), "PicoClaw-Setup.exe")
	if err := os.WriteFile(path, []byte("installer"), 0o755); err != nil {
		t.Fatalf("write installer: %v", err)
	}
	result, err := runner.launchGUIInstaller(path, file, Options{IsGUI: true})
	if err != nil {
		t.Fatalf("launch gui installer: %v", err)
	}
	if result.InstallMode != InstallModeInstaller || !result.IsGUI || result.InstallerFile != path {
		t.Fatalf("expected installer gui result, got %#v", result)
	}
	if launcher.path != path || launcher.kind != InstallerKindEXE {
		t.Fatalf("unexpected launcher call path=%q kind=%q", launcher.path, launcher.kind)
	}
}
```

- [x] **Step 2: Run tests to verify failure**

Run: `go test ./internal/install -run 'TestEffectiveOutput|TestLaunchGUIInstallerReturnsInstallerResult' -v`

Expected: FAIL because runner GUI helpers are missing.

- [x] **Step 3: Implement runner fields and helpers**

Update `RunResult` in `internal/install/runner.go`:

```go
IsGUI         bool
InstallMode   string
InstallerFile string
```

Add `InstallerLauncher InstallerLauncher` to `InstallRunner` and set `DefaultInstallerLauncher{}` in `NewRunner`.

Add:

```go
func effectiveOutput(opts Options) string {
	if opts.IsGUI && opts.InstallMode == InstallModePortable && !opts.OutputExplicit && opts.GuiTarget != "" {
		return opts.GuiTarget
	}
	return opts.Output
}
```

Use `effectiveOutput(opts)` in the existing `outputPath` call.

Add:

```go
func (r *InstallRunner) launchGUIInstaller(path string, file ExtractedFile, opts Options) (RunResult, error) {
	kind := DetectInstallerKind(file.ArchiveName)
	if kind == InstallerKindUnknown {
		kind = DetectInstallerKind(file.Name)
	}
	launcher := r.InstallerLauncher
	if launcher == nil {
		launcher = DefaultInstallerLauncher{}
	}
	if err := launcher.LaunchInstaller(path, kind); err != nil {
		return RunResult{}, err
	}
	return RunResult{Asset: filepath.Base(path), IsGUI: true, InstallMode: InstallModeInstaller, InstallerFile: path}, nil
}
```

- [x] **Step 4: Integrate installer branch**

After archive file selection, compute:

```go
selectedName := bin.ArchiveName
if selectedName == "" {
	selectedName = path.Base(url)
}
opts.InstallMode = DetectGUIInstallMode(opts.IsGUI, selectedName)
if opts.IsGUI && opts.InstallMode == "" {
	opts.InstallMode = InstallModePortable
}
```

Before normal extraction:

```go
if opts.InstallMode == InstallModeInstaller {
	installerPath, err := r.materializeInstallerFile(body, url, bin, opts)
	if err != nil {
		return RunResult{}, err
	}
	result, err := r.launchGUIInstaller(installerPath, bin, opts)
	if err != nil {
		return RunResult{}, err
	}
	result.URL = url
	result.Tool = tool
	if result.Asset == "" {
		result.Asset = path.Base(url)
	}
	return result, nil
}
```

Implement `materializeInstallerFile` so local files are returned directly, direct downloaded assets use `CacheFilePath(opts.CacheDir, url)` or a deterministic file under `cache_dir/installers`, and archive-contained installers extract only the selected file to that deterministic path.

- [x] **Step 5: Run install tests**

Run: `go test ./internal/install -v`

Expected: PASS.

- [x] **Step 6: Commit**

```bash
git add internal/install/runner.go internal/install/runner_test.go
git commit -m "feat(install): launch gui installers"
```

## Task 7: List GUI Filtering

**Files:**
- Modify: `internal/app/list.go`
- Modify: `internal/cli/service.go`
- Test: `internal/app/list_test.go`
- Test: `internal/cli/service_test.go`

- [x] **Step 1: Write failing list tests**

Add to `internal/app/list_test.go`:

```go
func TestListGUIPackagesFiltersConfigAndInstalledMetadata(t *testing.T) {
	now := time.Unix(1710000000, 0).UTC()
	isGUI := true
	svc := ListService{
		LoadConfig: func() (*cfgpkg.File, error) {
			cfg := cfgpkg.NewFile()
			cfg.Packages["picoclaw"] = cfgpkg.Section{Repo: util.StringPtr("sipeed/picoclaw"), IsGUI: &isGUI}
			cfg.Packages["chlog"] = cfgpkg.Section{Repo: util.StringPtr("gookit/gitw")}
			return cfg, nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return &storepkg.Config{Installed: map[string]storepkg.Entry{
				"sipeed/picoclaw": {Repo: "sipeed/picoclaw", InstalledAt: now, Tag: "v0.2.7", IsGUI: true, InstallMode: "portable"},
				"gookit/gitw":     {Repo: "gookit/gitw", InstalledAt: now, Tag: "v0.3.6"},
			}}, nil
		},
	}
	items, err := svc.ListGUIPackages(false)
	if err != nil {
		t.Fatalf("list gui packages: %v", err)
	}
	if len(items) != 1 || items[0].Name != "picoclaw" || !items[0].IsGUI || items[0].InstallMode != "portable" {
		t.Fatalf("expected only picoclaw with gui metadata, got %#v", items)
	}
}
```

Add to `internal/cli/service_test.go`:

```go
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
```

- [x] **Step 2: Run tests to verify failure**

Run: `go test ./internal/app ./internal/cli -run 'TestListGUIPackages|TestHandleListGUI' -v`

Expected: FAIL because GUI list metadata and filtering are missing.

- [x] **Step 3: Implement list metadata and filtering**

Add to `ListItem`:

```go
IsGUI       bool
InstallMode string
```

When building from config, set `IsGUI` if `section.IsGUI` is true. When merging installed entries, OR `entry.IsGUI` into `item.IsGUI` and copy non-empty `entry.InstallMode`.

Add:

```go
func (s ListService) ListGUIPackages(all bool) ([]ListItem, error) {
	var items []ListItem
	var err error
	if all {
		items, err = s.ListPackages()
	} else {
		items, err = s.ListInstalledPackages()
	}
	if err != nil {
		return nil, err
	}
	gui := make([]ListItem, 0, len(items))
	for _, item := range items {
		if item.IsGUI {
			gui = append(gui, item)
		}
	}
	return gui, nil
}
```

Update `handleList`:

```go
if opts != nil && opts.GUI {
	items, err = s.listService.ListGUIPackages(opts.All)
} else if opts != nil && opts.All {
	items, err = s.listService.ListPackages()
} else {
	items, err = s.listService.ListInstalledPackages()
}
```

Update `printListInfo`:

```go
fmt.Printf("is_gui: %s\n", map[bool]string{true: "yes", false: "no"}[item.IsGUI])
if item.InstallMode != "" {
	fmt.Printf("install_mode: %s\n", item.InstallMode)
}
```

- [x] **Step 4: Run list tests**

Run: `go test ./internal/app ./internal/cli -run 'TestListGUI|TestHandleListGUI|TestHandleListInfo' -v`

Expected: PASS.

- [x] **Step 5: Commit**

```bash
git add internal/app/list.go internal/app/list_test.go internal/cli/service.go internal/cli/service_test.go
git commit -m "feat(list): filter gui packages"
```

## Task 8: Documentation and Verification

**Files:**
- Modify: `README.md`
- Modify: `README.zh-CN.md`
- Modify: `docs/DOCS.md`
- Modify: `docs/example.eget.toml`

- [x] **Step 1: Update docs**

Add English examples:

```markdown
eget install --gui sipeed/picoclaw
eget add --gui --name picoclaw sipeed/picoclaw
eget list --gui
```

Add Chinese examples:

```markdown
eget install --gui sipeed/picoclaw
eget add --gui --name picoclaw sipeed/picoclaw
eget list --gui
```

Add config field docs:

```markdown
- `gui_target`
- `is_gui`
```

Add note:

```markdown
`global.gui_target` is used only for portable GUI applications. GUI installers such as `.msi` or `setup.exe` are launched and do not record a final install directory.
```

Update `docs/example.eget.toml`:

```toml
[global]
target = "~/.local/bin"
gui_target = "~/Applications"

[packages."picoclaw"]
repo = "sipeed/picoclaw"
is_gui = true
file = "*.exe"
```

- [x] **Step 2: Run full test suite**

Run: `go test ./...`

Expected: PASS.

- [x] **Step 3: Check status and avoid unrelated files**

Run: `git status --short`

Expected: only GUI implementation files and docs are modified. Do not stage `docs/TODO.md` unless explicitly requested.

- [x] **Step 4: Commit docs**

```bash
git add README.md README.zh-CN.md docs/DOCS.md docs/example.eget.toml
git commit -m "docs: document gui install support"
```

## Final Verification Checklist

- [x] `go test ./...` passes.
- [x] `download --gui` fails parsing as an unknown flag.
- [x] `install --gui` passes `IsGUI=true` through CLI -> app -> runner.
- [x] `add --gui` writes `is_gui = true`.
- [x] GUI portable installs use `global.gui_target` only when `--to` is not set.
- [x] GUI installer installs launch via `InstallerLauncher` and write installed store even with no extracted files.
- [x] `list` still includes GUI installed packages by default.
- [x] `list --gui` filters to GUI packages.
- [x] `list --all --gui` includes managed GUI packages that are not installed.
- [x] `list --info` prints `is_gui` and non-empty `install_mode`.
- [x] `docs/TODO.md` is not staged unless explicitly requested.
