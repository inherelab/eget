# GUI Install Support Design

## Goal

Add first-class GUI application support without weakening the existing CLI-tool install flow.

This design adds:

- `global.gui_target` for portable GUI application install directories.
- `is_gui` on package/repo configuration to mark GUI applications.
- `install --gui` and `add --gui` for explicit CLI control.
- `list --gui` to filter GUI applications.
- installed-store metadata that records the confirmed GUI status and install mode used at install time.

Installer-style GUI applications such as `.msi` and `setup.exe` are launched automatically, but `eget` does not try to detect or record their final installation directory.

## Non-Goals

This first version does not support:

- Silent installer arguments.
- Uninstalling GUI applications installed through external installers.
- Detecting final installer output paths.
- Creating desktop shortcuts or Start Menu entries.
- Running Windows installers through Wine on non-Windows systems.
- Adding `download --gui`.

## Concepts

### GUI Application

A target is treated as GUI when one of these is true:

- The user passes `install --gui`.
- The user passes `add --gui`.
- The resolved package or repo configuration has `is_gui = true`.

GUI status participates in the same layered precedence model as install options:

```text
CLI > package > repo > default
```

`global` does not define `is_gui` in the MVP. `global.gui_target` is a global directory setting, not a default GUI marker.

### Install Mode

GUI installs resolve to one of two modes:

- `portable`: the asset is directly runnable or can be extracted into an application directory.
- `installer`: the asset is an external installer that `eget` launches.

Installer mode is only inferred when `is_gui = true`. Non-GUI installs keep the current behavior.

## Configuration Model

### Global Fields

```toml
[global]
target = "~/.local/bin"
gui_target = "~/Applications"
```

`global.gui_target` is the default output directory for portable GUI applications.

Directory resolution rules:

1. If `--to` is provided, use `--to`.
2. If the final install is GUI + portable and `global.gui_target` is set, use `global.gui_target`.
3. Otherwise use `global.target`.

The app layer should pass both `target` and `gui_target` into runtime install options. The runner confirms `install_mode` only after it has selected the final asset or extracted file, so final output-path selection for GUI portable installs belongs at the runner boundary, not only in app option resolution.

Installer mode never uses `global.gui_target` because the external installer owns its final destination.

### Package and Repo Fields

```toml
[packages.picoclaw]
repo = "sipeed/picoclaw"
is_gui = true
file = "*.exe"
```

Repo sections may also use `is_gui = true` for consistency with existing repo/package layering.

`add --gui` and `install --add --gui` write `is_gui = true` into the package config.

## CLI Surface

### Install

```bash
eget install --gui owner/repo
eget install --gui --add --name picoclaw sipeed/picoclaw
```

`install --gui` marks the current install as GUI.

For portable GUI assets, it switches the default output directory from `global.target` to `global.gui_target` when `--to` is not set.

For installer GUI assets, it launches the installer and considers the command successful once the installer process starts successfully.

### Add

```bash
eget add --gui --name picoclaw sipeed/picoclaw
```

`add --gui` only records package configuration. It does not download or launch anything.

### Download

`download --gui` is intentionally not supported. `download` keeps its current raw-fetch semantics. Users can still choose a GUI-related output directory explicitly with `--to`.

Implementation should not register a `--gui` flag on `download`; the parser should reject it as an unknown flag.

### List

```bash
eget list
eget list --gui
eget list --all --gui
```

Rules:

- `eget list` continues to show all installed packages, including GUI packages.
- `eget list --gui` filters the default installed-package view to GUI packages.
- `eget list --all --gui` filters the managed + installed union to GUI packages.
- `eget list --info <name>` should show `is_gui` and `install_mode` when known.

## Installer Detection

Installer detection runs only after a target is confirmed GUI.

Rules:

- `.msi` asset: `installer`
- `.exe` asset whose filename contains `setup`: `installer`
- `.exe` asset whose filename contains `installer`: `installer`
- Everything else: `portable`

Matching should be case-insensitive and based on the final selected asset or extracted file name.

Examples:

- `picoclaw.exe` -> `portable`
- `PicoClaw-Setup.exe` -> `installer`
- `foo-installer-x64.exe` -> `installer`
- `app.msi` -> `installer`
- `tool.zip` containing `tool.exe` -> `portable`
- `tool.zip` containing `setup.exe` -> `installer`

Installer detection must run after archive file selection. A zip asset is not installer-mode by itself; the selected file inside the archive decides the mode.

## Runtime Model

The runtime model should separate GUI intent, directory defaults, confirmed install mode, and installer launch side effects.

`install.Options` should gain fields equivalent to:

- `IsGUI bool`: the final GUI intent after CLI/config resolution.
- `GuiTarget string`: expanded portable GUI default directory.
- `InstallMode string`: optional internal field for the runner-confirmed mode. CLI/config install-mode override is not part of the MVP.

`install.RunResult` should gain fields equivalent to:

- `IsGUI bool`: confirmed GUI status used for this run.
- `InstallMode string`: empty for non-GUI, `portable` or `installer` for GUI installs.
- `InstallerFile string`: path to the installer file that was started, when mode is `installer`.

The app layer resolves config and CLI intent. The runner owns these decisions because it sees the selected asset and extracted file:

- Confirm `install_mode`.
- Choose `global.gui_target` for GUI portable installs when `--to` is empty.
- Materialize installer files.
- Launch installers.

Installer launch should use a narrow interface so tests do not start real installers:

```go
type InstallerLauncher interface {
    LaunchInstaller(path string, kind InstallerKind) error
}
```

The default implementation uses `os/exec`. Tests should inject a fake launcher that records the command and returns success or failure.

## Installer Launch Behavior

Installer mode downloads or extracts the installer file, starts it, and returns success when the installer process starts successfully.

Success means process start succeeds. The implementation should call `cmd.Start()` and should not call `cmd.Wait()` for GUI installers.

Windows behavior:

- `.msi`: start `msiexec /i <file>`.
- `.exe`: start the selected installer executable directly.

Non-Windows behavior:

- `.msi`: return an unsupported installer error.
- Windows-style setup `.exe`: return an unsupported installer error.
- Native installer formats are out of scope for this MVP unless they are already handled as portable artifacts.

If process start fails, `install` returns an error and does not write installed-store metadata.

Installer file materialization rules:

- Direct `.msi` or installer `.exe` asset: reuse the cached/downloaded asset path when available. If the current download path only exists in memory, write it to a deterministic file under `cache_dir` before launch.
- Archive containing an installer file: extract only the selected installer file to a deterministic file under `cache_dir` before launch.
- Local file target: launch the local file path directly when it matches installer rules.

If `cache_dir` is empty and an installer file needs to be materialized, use the same fallback output/cache directory that direct downloads currently use. The implementation plan should pin the exact helper to avoid ad-hoc temp paths.

## Installed Store

Installed entries should record the confirmed values used during install:

```toml
[installed."sipeed/picoclaw"]
repo = "sipeed/picoclaw"
tag = "v0.2.7"
is_gui = true
install_mode = "portable"
```

Fields:

- `is_gui`: bool, records final GUI status.
- `install_mode`: string, records final mode. Valid values for GUI installs are `portable` and `installer`.

Recording rules:

- GUI portable installs record `is_gui = true`, `install_mode = "portable"`, and the usual extracted files.
- GUI installer installs record `is_gui = true`, `install_mode = "installer"`, and the selected/downloaded installer asset information.
- GUI installer installs do not record a final application install directory.
- GUI installer installs write installed-store metadata when installer launch succeeds, even if `ExtractedFiles` is empty.
- Non-GUI installs may omit `install_mode` to avoid changing existing CLI-tool records unnecessarily.

`list --gui` should use both config and installed-store metadata. A package is GUI if either source says it is GUI.

This is intentionally an OR merge for the MVP. A config value of `is_gui = false` does not hide an installed-store entry that was recorded as GUI.

## Data Flow

### Portable GUI Install

1. CLI parses `install --gui` or app resolves `is_gui = true` from config.
2. App resolves install options and passes `IsGUI=true` plus expanded `GuiTarget` to the runner.
3. Runner downloads/selects the asset and selects the archive file when needed.
4. Runner confirms `install_mode = "portable"`.
5. If `--to` is empty, runner uses `GuiTarget` when set, otherwise the resolved normal output target.
6. Runner extracts the selected file using the current install flow.
7. App records installed metadata with `is_gui = true` and `install_mode = "portable"`.

### Installer GUI Install

1. CLI parses `install --gui` or app resolves `is_gui = true` from config.
2. App resolves install options and passes `IsGUI=true` plus expanded `GuiTarget` to the runner.
3. Runner selects/downloads the asset and selects the archive file when needed.
4. Installer detection marks the selected asset or extracted file as `installer`.
5. Runner materializes the installer file to disk when necessary.
6. Runner launches the installer via `InstallerLauncher`.
7. If process start succeeds, app records installed metadata with `is_gui = true` and `install_mode = "installer"`, even when there are no extracted files.
8. App does not record a final install directory.

## Error Handling

- Missing `global.gui_target`: fallback to `global.target` for portable GUI installs.
- Unsupported installer platform: return a clear error and do not write installed state.
- Installer process start failure: return the process start error and do not write installed state.
- Ambiguous asset selection: keep existing asset selection behavior.
- Non-GUI installs: keep existing behavior.

## Testing Strategy

Config tests:

- Load and save `global.gui_target`.
- Load and save `packages.<name>.is_gui`.
- `config set global.gui_target <path>` round-trips as string.
- `config set packages.foo.is_gui true` parses as bool.

CLI tests:

- `install --gui` binds to install options.
- `add --gui` binds to add options.
- `download --gui` is not registered and is rejected by the parser as an unknown flag.
- `list --gui` binds to list options.

App tests:

- Package `is_gui = true` makes install use GUI behavior.
- GUI portable install with no `--to` uses `global.gui_target`.
- GUI portable install with `--to` uses `--to` over `global.gui_target`.
- GUI portable install falls back to `global.target` when `global.gui_target` is empty.
- `add --gui` writes `is_gui = true`.
- `install --add --gui` writes `is_gui = true`.

Install runner tests:

- `.msi` is detected as installer only when GUI is true.
- `setup.exe` and `installer.exe` are detected as installer only when GUI is true.
- Plain `.exe` remains portable.
- Windows MSI launch uses `msiexec /i <file>`.
- Windows setup exe launch starts the exe directly.
- Unsupported platform returns an error before writing installed state.

Installed-store tests:

- `is_gui` and `install_mode` round-trip through TOML.
- Installer-mode records do not require extracted files.

List tests:

- Default list still includes GUI installed packages.
- `list --gui` filters to GUI packages.
- `list --all --gui` includes managed GUI packages that are not installed.
- Installed-only GUI entries are included by `list --gui` using installed-store metadata.
- `list --info` always displays `is_gui: yes|no`; it displays `install_mode` only when non-empty.

## Open Implementation Notes

The current runtime install options can keep using internal names that already exist, but config-facing and CLI-facing names should use `gui_target`, `is_gui`, and `install_mode`.

Installer-mode support likely needs a narrow interface around process launching so tests can fake launch success and failure without starting real installers.
