# Eget

![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/inherelab/eget?style=flat-square)
[![GitHub tag (latest SemVer)](https://img.shields.io/github/tag/inherelab/eget)](https://github.com/inherelab/eget)
[![Unit-Tests](https://github.com/inherelab/eget/actions/workflows/go.yml/badge.svg)](https://github.com/inherelab/eget)

---

[English](./README.md) | [简体中文](./README.zh-CN.md)

`eget` helps locate, download, and extract prebuilt binaries. The current version has been restructured into an explicit subcommand CLI, with the entry point in `cmd/eget/main.go` and business logic concentrated under `internal/`.

> Forked from https://github.com/zyedidia/eget and inspired by https://github.com/gmatheu/eget

## Supported Targets

The target argument accepted by `install` and `download` can be:

- `owner/repo`
- GitHub repository URL
- Direct download URL
- Local file

## Command Style

```bash
eget <command> --options... arguments...
```

The root command no longer supports the legacy forms below:

```bash
eget REPO
eget --tag nightly REPO
eget install REPO --tag nightly
```

Use this form instead:

```bash
eget install --tag nightly owner/repo
```

## Examples

```bash
eget install --tag nightly inhere/markview
eget install --to ~/.local/bin/fzf junegunn/fzf
# download
eget download --file go --to ~/go1.17.5 https://go.dev/dl/go1.17.5.linux-amd64.tar.gz
# uninstall
eget uninstall fzf
eget list
# update
eget update fzf
eget update --all
# config
eget add --name fzf --to ~/.local/bin junegunn/fzf
eget config --info
eget config --init
eget config --list
eget config get global.target
eget config set global.target ~/.local/bin
```

## Available Commands

`install` (aliases: `i`, `ins`)

- Resolve, download, verify, and extract a target, then record installation state.

`download` (alias: `dl`)

- Reuses the install pipeline, but only downloads/extracts and does not record installed state.

`add`

- Writes a managed package definition to `[packages.<name>]` in the config file.

`uninstall` (aliases: `uni`, `remove`, `rm`)

- Removes installed files and clears the installed store entry without deleting `[packages.<name>]`.

`list` (alias: `ls`)

- Lists local managed packages and attaches recent installed-state details when available.

`update` (alias: `up`)

- Updates a single managed package, or all managed packages with `--all`.

`config` (alias: `cfg`)

- Supports `--info`, `--init`, `--list`, `get KEY`, and `set KEY VALUE`.

## Main Options

`install`, `download`, and `add` share these installation-related options:

- `--tag`
- `--system`
- `--to`
- `--cache-dir`
- `--file`
- `--asset`
- `--source`
- `--all`
- `--quiet`

`update` additionally supports:

- `--all`
- `--dry-run`
- `--interactive`

Notes:

- `--asset` is currently parsed as a single string and then mapped to internal `[]string`.
- `--cache-dir` overrides `cache_dir` from config and controls the remote download cache directory.
- Argument order follows the `cflag/capp` parser constraint and must be `CMD --OPTIONS... ARGUMENTS...`.

## Configuration

The config file is resolved in this order:

1. `EGET_CONFIG`
2. `~/.eget.toml`
3. XDG / LocalAppData fallback path

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
eget config --init
```

This writes:

- `global.target = "~/.local/bin"`
- `global.cache_dir = "~/.cache/eget"`
- `global.proxy_url = ""`

Directory semantics:

- `target` is the default install directory
- `cache_dir` is the default download cache directory
- `proxy_url` is the global proxy for remote requests; both GitHub lookups and remote downloads use it
- `download` uses `cache_dir` by default when `--to` is not provided
- `install` and `download` will reuse cached remote download contents from `cache_dir` when available

## Build And Test

```bash
make build
make test
```

## Project Structure

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
