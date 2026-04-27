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
- Cache and proxy support: supports `cache_dir` download reuse, `api_cache` for GitHub API response caching, and combined `proxy_url`/`ghproxy` remote request proxying.
- Managed package lifecycle: supports `add`, `list`, `update`, and `uninstall` for package definitions, installed state, and cleanup workflows.
- GitHub repository search: supports `search` with native GitHub search qualifiers, text output, and JSON output.
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
# Install and override the asset name
eget install --asset zip windirstat/windirstat
# Filter assets with regex
eget install --asset "REG:\\.deb$" owner/repo
# Install to a custom directory
eget install --to ~/.local/bin/fzf junegunn/fzf
# Install and record the package definition
eget install --add junegunn/fzf
eget install --add --name rg BurntSushi/ripgrep
# Install a GUI app; portable GUI apps use global.gui_target by default
eget install --gui sipeed/picoclaw
eget add --gui --name picoclaw sipeed/picoclaw
```

**Download Examples**

```bash
# download
eget download ip7z/7zip
eget download --file go --to ~/go1.17.5 https://go.dev/dl/go1.17.5.linux-amd64.tar.gz
eget download --file README.md,LICENSE --to ./dist owner/repo
eget download --file "*.txt" owner/repo
eget download --file "bin/*" owner/repo
eget download --extract-all --to ./dist windirstat/windirstat
```

**Others Examples**

```bash
# uninstall
eget uninstall fzf
# list installed packages
eget list|ls
# list all managed and installed packages
eget list --all
# list GUI packages
eget list --gui
# query repo info
eget query owner/repo
eget query --action releases --limit 5 owner/repo
eget query --action assets --tag v1.2.3 owner/repo
# search GitHub repositories
eget search ripgrep
eget search skillc language:rust user:inhere
eget search --limit 5 --sort stars --order desc terminal ui
eget search --json picoclaw user:sipeed
# update fzf
eget update fzf
eget update --all
```

**Config Examples**

```bash
# config
eget add --name fzf --to ~/.local/bin junegunn/fzf
eget config init
eget config list|ls
eget config get global.target
eget config set global.target ~/.local/bin
```

### Supported Targets

The target argument accepted by `install` and `download` can be:

- `name` in the config packages
- `owner/repo`
- GitHub repository URL
- Direct download URL
- Local file

## Available Commands

`install` (aliases: `i`, `ins`)

- Resolve, download, verify, and extract a target, then record installation state.
- `--name` can be used to override the installed executable name; without `--to`, it also acts as the rename hint for single-file assets.
- `--gui` marks the target as a GUI application. Portable GUI apps use `global.gui_target` by default, while GUI installers such as `.msi` or `setup.exe` are launched and do not record a final install directory. Without `--gui`, installer-like assets prompt before launch; with `--add`, a confirmed installer is persisted with `is_gui = true`.
- With `--add`, a successful install also writes the repo target to `[packages.<name>]`; use `--name` to override the package name.

`download` (alias: `dl`)

- Reuses the install pipeline without recording installed state.
- Downloads the raw asset by default; archive extraction only happens when `--file` or `--extract-all` is set.

`add`

- Writes a managed package definition to `[packages.<name>]` in the config file.

`uninstall` (aliases: `uni`, `remove`, `rm`)

- Removes installed files and clears the installed store entry without deleting `[packages.<name>]`.

`list` (alias: `ls`)

- Lists installed packages by default.
- Use `--all` / `-a` to list the union of local managed packages and installed-store entries.
- Use `--gui` to filter the current list view to GUI applications.

`query` (alias: `q`)

- Queries GitHub repository release metadata without installing anything or touching local state.
- Defaults to the `latest` action, and can switch to `info`, `releases`, or `assets` with `--action`.

`search`

- Searches GitHub repositories without installing anything or touching local state.
- Uses the first argument as the keyword and passes remaining arguments through as GitHub search qualifiers, for example `language:go`, `user:inhere`, or `topic:cli`.

`update` (alias: `up`)

- Updates a single managed package, or all managed packages with `--all`.

`config` (alias: `cfg`)

- Supports `init`, `list` / `ls`, `get KEY`, and `set KEY VALUE`.

## Main Options

`install`, `download`, and `add` share these installation-related options:

- `--tag`: Select a release tag; defaults to `latest` when omitted.
- `--system`: Override the target OS/arch, for example `windows/amd64` or `linux/arm64`.
- `--to`: Set the install or download output path; accepts either a directory or a full file path.
- `--file`: Select file(s) to extract from an archive; supports comma-separated file names or glob patterns such as `README.md,LICENSE`.
- `--asset`: Filter release assets by keyword; multiple filters can be separated by commas. Regex is also supported with the `REG:` prefix, for example `REG:\\.deb$`, and exclusions can use `^REG:...`.
- `--source`: Download the source archive instead of a prebuilt binary release.
- `--extract-all`, `--ea`: Extract all files from the archive instead of selecting a single target file.
- `--quiet`: Reduce normal command output for scripting or batch use.

> Cache behavior is configured via `config set global.cache_dir ...` or the `cache_dir` field in the config file.

`install` additionally supports:

- `--add`: After a successful install, append the repo target to `[packages.<name>]` managed config.
- `--gui`: Install as a GUI application; with `--add`, persist `is_gui = true`. Installer-like assets selected without `--gui` prompt before launch and also persist `is_gui = true` when confirmed with `--add`.
- `--name`: Override the managed package name; for single executable assets, it also acts as the default output-name hint.

`update` options supports:

- `--all`: Update all managed packages instead of a single target.
- `--dry-run`: Preview the update plan without performing installation changes.
- `--interactive`: Interactively select which managed packages to update.

`query` options supports:

- `--action`, `-a`: Query action. Supported values: `latest`, `releases`, `assets`, `info`.
- `--tag`, `-t`: Select the release tag for the `assets` action; defaults to latest when omitted.
- `--limit`, `-l`: Limit the number of rows returned by the `releases` action. Default: `10`.
- `--json`, `-j`: Output JSON for scripting or automation.
- `--prerelease`, `-p`: Include prerelease entries for `latest` and `releases`.

`search` options supports:

- `--limit`, `-l`: Limit the number of repositories returned. Default: `10`.
- `--sort`: Sort search results. Supported values: `stars`, `updated`.
- `--order`: Sort order. Supported values: `desc`, `asc`.
- `--json`, `-j`: Output JSON for scripting or automation.

Global options:

- `-v`, `--verbose`: Show more execution details such as API requests, response summaries, asset selection, cache hits, and key workflow steps.

Notes:

- `install --name` can rename a single executable asset, for example installing `chlog-windows-amd64.exe` as `chlog.exe`.
- `install --add` only applies to repo targets and appends the managed package definition after a successful install.
- `global.gui_target` is used only for portable GUI applications. GUI installers such as `.msi` or `setup.exe` are launched and do not record a final install directory.
- `download` stores the raw downloaded asset by default; extraction only happens when `--file` or `--extract-all` is provided.
- Archive extraction currently supports `zip`, `tar.*`, and `7z`.
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

[api_cache]
enable = false
cache_time = 300

[ghproxy]
enable = false
host_url = ""
support_api = true
fallbacks = []

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
- `gui_target`
- `cache_dir`
- `proxy_url`
- `api_cache.enable`
- `api_cache.cache_time`
- `ghproxy.enable`
- `ghproxy.host_url`
- `ghproxy.support_api`
- `ghproxy.fallbacks`
- `system`
- `tag`
- `file`
- `asset_filters`
- `download_source`
- `extract_all`
- `is_gui`
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
- `api_cache.enable = false`
- `api_cache.cache_time = 300`
- `ghproxy.enable = false`
- `ghproxy.host_url = ""`
- `ghproxy.support_api = true`

By default, the file is created at `~/.config/eget/eget.toml`.

Directory semantics:

- `target` is the default install directory
- `cache_dir` is the default download cache directory
- `proxy_url` is the global proxy for remote requests; both GitHub lookups and remote downloads use it
- `api_cache` only caches GitHub API `GET` responses, and the cache file directory is derived as `{cache_dir}/api-cache/`
- `cache_time` is measured in seconds; expired cache entries are refreshed from the network
- `ghproxy` rewrites GitHub asset download URLs; when `support_api = true`, it also rewrites `api.github.com` requests
- `ghproxy.fallbacks` are tried in order when the primary ghproxy host fails
- `proxy_url` is the HTTP-layer proxy, while `ghproxy` rewrites request URLs; both can be enabled together
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
