---
title: eget
section: 1
header: Eget Manual
---

# NAME
  eget - explicit subcommand cli for downloading and extracting prebuilt binaries

# SYNOPSIS
  eget `COMMAND [--Options...] [...Arguments]`

# DESCRIPTION
  Eget is a CLI for locating, downloading, verifying, and extracting prebuilt
  binaries. The current command model is explicit subcommands only:

      eget <command> --options... arguments...

  Legacy root-command install forms such as **`eget user/repo`** are no longer
  supported.

  The target argument accepted by `install` and `download` may be:

  * a repository in **`owner/repo`** form
  * a GitHub repository URL
  * a direct download URL
  * a local file

  If an asset has a matching checksum sidecar such as `xxx.sha256` or
  `xxx.sha256sum`, Eget will automatically verify the SHA-256 checksum before
  extraction.

# COMMANDS
  `install`

:    Resolve, download, verify, extract, and record installed state.

  `download`

:    Resolve, download, verify, and extract without recording installed state.

  `add`

:    Save a managed package entry into `[packages.<name>]`.

  `uninstall`

:    Remove installed files and clear the installed-state record for a package or repo.

  `list`

:    List managed packages and attach installed-state details when available.

  `update`

:    Update a managed package, or all managed packages with **`--all`**.

  `config`

:    Inspect and modify configuration. Supported forms:
     **`eget config --info`**, **`eget config --init`**, **`eget config --list`**,
     **`eget config get KEY`**, **`eget config set KEY VALUE`**.

# COMMON OPTIONS
  `--tag=`

:    Use the specified release tag.

  `--system=`

:    Use the specified target system.

  `--to=`

:    Set the output path or destination.

  `--file=`

:    Select a file to extract.

  `--asset=`

:    Filter assets by substring.

  `--source`

:    Download source archive instead of release asset.

  `--all`

:    Extract all matching files. On `update`, this flag means update all managed packages.

  Cache directory is configured through `global.cache_dir` in config; command-level `--cache-dir` overrides are not supported.

  `--quiet`

:    Reduce output noise.

  `--dry-run`

:    Preview update actions without applying them.

  `--interactive`

:    Interactively choose update targets when supported by the current workflow.

  `-v, --version`

:    Show version information.

  `-h, --help`

:    Show help.

# CONFIGURATION
  Eget reads configuration from:

  1. **`EGET_CONFIG`**
  2. **`~/.config/eget/eget.toml`**
  3. legacy **`~/.eget.toml`**
  4. XDG / LocalAppData fallback paths

  Supported sections:

  * **`[global]`**
  * **`["owner/repo"]`**
  * **`[packages.<name>]`**

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

  Config precedence for install resolution is:

      CLI > package > repo > global > default

  `eget config --init` currently writes these defaults:

      global.target = "~/.local/bin"
      global.cache_dir = "~/.cache/eget"
      global.proxy_url = ""

  By default, the config file is written to **`~/.config/eget/eget.toml`**.

  Directory semantics:

  * `target` is the default install directory
  * `cache_dir` is the default download cache directory
  * `proxy_url` is the global proxy for GitHub lookups and remote downloads
  * `download` falls back to `cache_dir` when `--to` is not provided
  * remote downloads are reused from `cache_dir` when a cached file exists

  The installed-state store defaults to **`~/.config/eget/installed.toml`**.

# EXAMPLES
```bash
eget install --tag nightly inhere/markview
eget download --file go --to ~/go1.17.5 https://go.dev/dl/go1.17.5.linux-amd64.tar.gz
eget add --name fzf --to ~/.local/bin junegunn/fzf
eget uninstall fzf
eget list
eget update --all
eget config --info
eget config get global.target
eget config set global.target ~/.local/bin
```

# NOTES
  The parser follows the standard `CMD --OPTIONS... ARGUMENTS...` form.
  This works:

      eget install --tag nightly inhere/markview

  This does not:

      eget install inhere/markview --tag nightly

# AUTHOR
  Zachary Yedidia <zyedidia@gmail.com>
