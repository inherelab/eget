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
CLI > package > repo > global > default
```

`global` does not need an `is_gui` default in the MVP. It is listed in the precedence model only because the config merge utility already supports global fields as a general pattern.

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

## Installer Launch Behavior

Installer mode downloads or extracts the installer file, starts it, and returns success when the installer process starts successfully.

Windows behavior:

- `.msi`: start `msiexec /i <file>`.
- `.exe`: start the selected installer executable directly.

Non-Windows behavior:

- `.msi`: return an unsupported installer error.
- Windows-style setup `.exe`: return an unsupported installer error.
- Native installer formats are out of scope for this MVP unless they are already handled as portable artifacts.

If process start fails, `install` returns an error and does not write installed-store metadata.

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
- Non-GUI installs may omit `install_mode` to avoid changing existing CLI-tool records unnecessarily.

`list --gui` should use both config and installed-store metadata. A package is GUI if either source says it is GUI.

## Data Flow

### Portable GUI Install

1. CLI parses `install --gui` or app resolves `is_gui = true` from config.
2. App resolves install options.
3. If `--to` is empty and install mode is portable, app uses `global.gui_target` when set.
4. Runner downloads/selects/extracts the asset using the current install flow.
5. App records installed metadata with `is_gui = true` and `install_mode = "portable"`.

### Installer GUI Install

1. CLI parses `install --gui` or app resolves `is_gui = true` from config.
2. Runner selects/downloads the asset.
3. Installer detection marks the selected file as `installer`.
4. Runner launches the installer.
5. If process start succeeds, app records installed metadata with `is_gui = true` and `install_mode = "installer"`.
6. App does not record a final install directory.

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
- `download --gui` is rejected or absent.
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
- `list --info` displays `is_gui` and `install_mode` when available.

## Open Implementation Notes

The current runtime install options can keep using internal names that already exist, but config-facing and CLI-facing names should use `gui_target`, `is_gui`, and `install_mode`.

Installer-mode support likely needs a narrow interface around process launching so tests can fake launch success and failure without starting real installers.
