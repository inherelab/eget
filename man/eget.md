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
  2. **`~/.eget.toml`**
  3. XDG / LocalAppData fallback paths

  Supported sections:

  * **`[global]`**
  * **`["owner/repo"]`**
  * **`[packages.<name>]`**

  Example:

```toml
[global]
target = "~/.local/bin"
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

# EXAMPLES
```bash
eget install --tag nightly inhere/markview
eget download --file go --to ~/go1.17.5 https://go.dev/dl/go1.17.5.linux-amd64.tar.gz
eget add --name fzf --to ~/.local/bin junegunn/fzf
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
