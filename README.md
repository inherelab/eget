# Eget

![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/inherelab/eget?style=flat-square)
[![GitHub tag (latest SemVer)](https://img.shields.io/github/tag/inherelab/eget)](https://github.com/inherelab/eget)
[![Unit-Tests](https://github.com/inherelab/eget/actions/workflows/go.yml/badge.svg)](https://github.com/inherelab/eget)

---

[English](./README.md) | [简体中文](./README.zh-CN.md)

`eget` helps locate, download, and extract prebuilt binaries from GitHub.

> Forked from https://github.com/zyedidia/eget Refactored and enhanced the tool's functionality.

## Features

- Explicit subcommand CLI: uses the consistent `eget <command> --options... arguments...` form, with clear command boundaries and better automation ergonomics.
- Multiple target types: `install` and `download` accept `owner/repo`, GitHub repository URLs, direct download URLs, and local files.
- Unified download, verify, and extract flow: built-in asset discovery, system/asset selection, SHA-256 verification, and archive extraction reduce manual steps.
- Cache and proxy support: supports `cache_dir` for download reuse and `proxy_url` for both GitHub lookups and remote download requests.
- Managed package lifecycle: supports `add`, `list`, `update`, and `uninstall` for package definitions, installed state, and cleanup workflows.
- Traceable installed state: keeps a dedicated installed store with the latest asset, install time, and extracted files for each package.
- Layered configuration merging: supports `global`, repo sections, and `packages.<name>` with predictable option precedence.
- Unified default config directory: configuration and installed-state files default to `~/.config/eget/`, while legacy paths remain readable.

## Install

- Download from Releases [https://github.com/inherelab/eget/releases](https://github.com/inherelab/eget/releases)
- Install by `go install` command(require Go sdk)

```bash
go install github.com/inherelab/eget/cmd/eget@latest
```

## Command Style

```bash
eget <command> --options... arguments...
```

example:

```bash
eget install --tag nightly owner/repo
```

## Examples

**Install Examples**

```bash
eget install --tag nightly inhere/markview
# Install and override the executable name
eget install --name chlog gookit/gitw
# Install to a custom directory
eget install --to ~/.local/bin/fzf junegunn/fzf
# Install and record the package definition
eget install --add junegunn/fzf
eget install --add --name rg BurntSushi/ripgrep
```

**Others Examples**

```bash
# download
eget download --file go --to ~/go1.17.5 https://go.dev/dl/go1.17.5.linux-amd64.tar.gz
# uninstall
eget uninstall fzf
eget list
# update
eget update fzf
eget update --all
```

**Config Examples**

```bash
# config
eget add --name fzf --to ~/.local/bin junegunn/fzf
eget config init
eget config list|show
eget config get global.target
eget config set global.target ~/.local/bin
```

### Supported Targets

The target argument accepted by `install` and `download` can be:

- `owner/repo`
- GitHub repository URL
- Direct download URL
- Local file

## Available Commands

`install` (aliases: `i`, `ins`)

- Resolve, download, verify, and extract a target, then record installation state.
- `--name` can be used to override the installed executable name; without `--to`, it also acts as the rename hint for single-file assets.
- With `--add`, a successful install also writes the repo target to `[packages.<name>]`; use `--name` to override the package name.

`download` (alias: `dl`)

- Reuses the install pipeline, but only downloads/extracts and does not record installed state.

`add`

- Writes a managed package definition to `[packages.<name>]` in the config file.

`uninstall` (aliases: `uni`, `remove`, `rm`)

- Removes installed files and clears the installed store entry without deleting `[packages.<name>]`.

`list` (alias: `ls`)

- Lists the union of local managed packages and installed-store entries, and attaches recent installed-state details when available.

`update` (alias: `up`)

- Updates a single managed package, or all managed packages with `--all`.

`config` (alias: `cfg`)

- Supports `init`, `list` / `ls` / `show`, `get KEY`, and `set KEY VALUE`.

## Main Options

`install`, `download`, and `add` share these installation-related options:

- `--tag`: Select a release tag; defaults to `latest` when omitted.
- `--system`: Override the target OS/arch, for example `windows/amd64` or `linux/arm64`.
- `--to`: Set the install or download output path; accepts either a directory or a full file path.
- `--cache-dir`: Set the remote download cache directory and reuse cached files on subsequent runs.
- `--file`: Select a file to extract from an archive when multiple candidates exist.
- `--asset`: Filter release assets by keyword; multiple filters can be separated by commas.
- `--source`: Download the source archive instead of a prebuilt binary release.
- `--all`: Extract all files from the archive instead of selecting a single target file.
- `--quiet`: Reduce normal command output for scripting or batch use.

`install` additionally supports:

- `--add`: After a successful install, append the repo target to `[packages.<name>]` managed config.
- `--name`: Override the managed package name; for single executable assets, it also acts as the default output-name hint.

`update` additionally supports:

- `--all`: Update all managed packages instead of a single target.
- `--dry-run`: Preview the update plan without performing installation changes.
- `--interactive`: Interactively select which managed packages to update.

Global options:

- `-v`, `--verbose`: Show more execution details such as API requests, response summaries, asset selection, cache hits, and key workflow steps.

Notes:

- `install --name` can rename a single executable asset, for example installing `chlog-windows-amd64.exe` as `chlog.exe`.
- `install --add` only applies to repo targets and appends the managed package definition after a successful install.
- Argument order follows the `cflag/capp` parser constraint and must be `CMD --OPTIONS... ARGUMENTS...`.

## Configuration

The config file is resolved in this order:

1. `EGET_CONFIG`
2. `~/.config/eget/eget.toml`
3. XDG / LocalAppData fallback path
4. Legacy `~/.eget.toml`

Supported config sections:

- `[global]`
- `["owner/repo"]`
- `[packages.<name>]`

Example:

```toml
[global]
target = "~/.local/bin"
cache_dir = "~/.cache/eget"
proxy_url = "http://127.0.0.1:7890"
system = "windows/amd64"

["inhere/markview"]
tag = "nightly"

[packages.markview]
repo = "inhere/markview"
target = "~/.local/bin"
tag = "nightly"
asset_filters = ["windows"]
```

Common fields:

- `target`
- `cache_dir`
- `proxy_url`
- `system`
- `tag`
- `file`
- `asset_filters`
- `download_source`
- `all`
- `quiet`
- `upgrade_only`

Default initialization:

```bash
eget config init
```

This writes:

- `global.target = "~/.local/bin"`
- `global.cache_dir = "~/.cache/eget"`
- `global.proxy_url = ""`

By default, the file is created at `~/.config/eget/eget.toml`.

Directory semantics:

- `target` is the default install directory
- `cache_dir` is the default download cache directory
- `proxy_url` is the global proxy for remote requests; both GitHub lookups and remote downloads use it
- `download` uses `cache_dir` by default when `--to` is not provided
- `install` and `download` will reuse cached remote download contents from `cache_dir` when available

The installed-state store also defaults to `~/.config/eget/installed.toml`.

## Build And Test

```bash
make build
make test
```

## Project Structure

The current version has been restructured into an explicit subcommand CLI, with the entry point in `cmd/eget/main.go` and business logic concentrated under `internal/`.

- `cmd/eget`: command entry point
- `internal/cli`: `capp` command registration and argument binding
- `internal/app`: install/add/list/update/config use-case orchestration
- `internal/install`: find, download, verify, and extract execution pipeline
- `internal/config`: config loading, merging, and persistence
- `internal/installed`: installed-state storage
- `internal/source/github`: GitHub asset discovery

> For more details, see [docs/DOCS.md](docs/DOCS.md).

## References

- [https://github.com/zyedidia/eget](https://github.com/zyedidia/eget)
- [https://github.com/gmatheu/eget](https://github.com/gmatheu/eget)
