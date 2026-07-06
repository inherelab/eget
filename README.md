# Eget

![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/inherelab/eget?style=flat-square)
[![GitHub tag (latest SemVer)](https://img.shields.io/github/tag/inherelab/eget)](https://github.com/inherelab/eget)
[![Unit-Tests](https://github.com/inherelab/eget/actions/workflows/go.yml/badge.svg)](https://github.com/inherelab/eget)

---

[English](./README.md) | [简体中文](./README.zh-CN.md)

`eget` helps locate, download, and extract prebuilt binaries from GitHub, GitLab, Gitea/Forgejo, and SourceForge.

> Forked from https://github.com/zyedidia/eget Refactored and enhanced the tool's functionality.

## Features

- Multi-source installs: install or download binaries from GitHub, GitLab, Gitea/Forgejo, SourceForge, direct download URLs, and local files.
- Automatic selection and extraction: filter release assets by OS/arch, keyword, or regex, with SHA-256 verification and common archive extraction.
- Managed package workflow: use `add`, `list`, `update`, and `uninstall` to manage frequently used tools, record install state, and check batch updates.
- SDK downloads: install versioned SDK archives such as Go and Node with index cache, resumable downloads, and a separate SDK installed store.
- Concurrent downloads: automatically split large files into HTTP Range chunks for parallel download, and run package downloads concurrently when installing or updating all packages.
- Query and search: query GitHub release info, query SourceForge latest/assets, and search repositories with native GitHub search qualifiers.
- Cache and proxy support: use download cache, API response cache, `[http_proxy]`, and `ghproxy` for restricted networks or repeated installs.
- Config-driven usage: configure global defaults, repo-level options, and `packages.<name>` managed packages; config and installed store default to `~/.config/eget/`.

## Install

- Install with the release script

```bash
curl -fsSL https://raw.githubusercontent.com/inherelab/eget/master/.github/install.sh | sh
```

```powershell
iwr https://raw.githubusercontent.com/inherelab/eget/master/.github/install.ps1 -UseB | iex
```

- Download from Releases [https://github.com/inherelab/eget/releases](https://github.com/inherelab/eget/releases)
- Install by `go install` command (requires a local Go SDK)

```bash
go install github.com/inherelab/eget/cmd/eget@latest
```

## Command Style

```bash
eget <command> --options... arguments...
```

## Usage Examples

### Install Examples

**Install from GitHub**:

```bash
# quickly
eget i ORG/REPO
# install with tag
eget install --tag nightly inhere/markview
# Install and override the executable name
eget install --name chlog gookit/gitw
# Install and override the asset name
eget install --asset zip windirstat/windirstat
# Filter assets with regex
eget install --asset "REG:\\.deb$" owner/repo
# Filter assets by prefix/suffix
eget install --asset "PRE:codex,SUF:.zip" openai/codex
# Extract all files and rename one extracted file
eget install --extract-all --rename "codex-x86_64-pc-windows-msvc.exe=codex.exe" openai/codex
# Extract all files and strip the archive top-level directory
eget download --extract-all --strip-components 1 --asset "windows,zip" ventoy/Ventoy
# Install to a custom directory
eget install --to ~/.local/bin/fzf junegunn/fzf
```

**Install a SourceForge project**:

```bash
# Install a SourceForge project directly
eget install --asset x64,PerUser,setup sourceforge:winmerge
```

**Install a GitLab/Gitea/Forgejo project**:

```bash
# Install from GitLab releases
eget install gitlab:fdroid/fdroidserver
eget install gitlab:gitlab.gnome.org/GNOME/gtk
# Install from Gitea/Forgejo-compatible releases
eget install --asset linux,amd64 gitea:codeberg.org/forgejo/forgejo
```

**Install and record**:

```bash
# Install and record the package definition
eget install --add junegunn/fzf
eget install --add --name rg BurntSushi/ripgrep
# Add a SourceForge project as a managed package
eget add --name winmerge --system windows/amd64 --asset x64,PerUser,setup sourceforge:winmerge
# Install every package configured under [packages]
eget install --all
```

**Install GUI apps**:

```bash
# Install a GUI app; portable GUI apps use global.gui_target by default
eget install --gui sipeed/picoclaw
# Force a plain .exe GUI asset to launch as an installer
eget ins --gui --install-mode installer owner/repo
eget add --gui --name picoclaw sipeed/picoclaw
```

### Download Examples

```bash
# download
eget download ip7z/7zip
eget download --file go --to ~/go1.17.5 https://go.dev/dl/go1.17.5.linux-amd64.tar.gz
eget download --file README.md,LICENSE --to ./dist owner/repo
eget download --file "*.txt" owner/repo
eget download --file "bin/*" owner/repo
eget download --file "*.exe,^*x86*,^*.sig" owner/repo
eget download --extract-all --to ./dist windirstat/windirstat
```

### SDK Examples

```bash
eget sdk install go@1.22.0
eget sdk install go:1.22 node:20.11.1
eget sdk install --force go@1.22.0
eget sdk list
eget sdk list --json
eget sdk remove go@1.22.0
eget sdk path go
eget sdk path go:1.20
eget sdk path java:17
eget sdk config add --all
eget sdk config add --all --mirror mirror
eget sdk config add jdk --mirror zulu
eget sdk index refresh go
eget sdk index show go
```

`eget sdk` only downloads and extracts SDK archives. It does not modify `PATH`, write shell hooks, or manage active SDK versions. For environment switching, use a dedicated tool such as `kite xenv` after installation.

### Cache Management

`eget cache` manages the local cache directory. The top-level command also has a short alias:

```bash
eget ca status
eget ca list --root sdk
```

Subcommands:

- `cache clean`: remove selected cache files.
- `cache list`: list cache files by root (`all`, `pkg`, `api`, `sdk`, `sdk-index`, `partial`).
- `cache status`: show cache directory usage summary.
- `cache serve`: serve cache files over a read-only HTTP server.

`eget cache clean` removes local cache files from `global.cache_dir`. By default it removes package, API, SDK download, and partial download cache files older than 3 days. SDK index cache is kept by default; use `--sdk-index` when you explicitly want to clear it.

```bash
eget cache clean
eget cache clean --dry-run --older 7d
eget cache clean --dry-run --json
eget cache clean --api --all
eget ca clean --sdk --partial --older 12h
```

Inspect cache contents:

```bash
eget cache status
eget cache list --root sdk
eget cache list --root sdk-index
eget cache list --json
```

`eget cache serve` starts a read-only HTTP server for local cache files so machines on the same LAN can browse or download cached packages and SDK archives. Open the server root URL in a browser to view the built-in file list UI, or request `/manifest.json` for the machine-readable manifest.

```bash
eget cache serve
eget cache serve --host 127.0.0.1 --port 0 --root sdk --no-index
```

The server prints text request logs by default. Add simple bearer protection and switch request logs to JSON lines when needed:

```bash
eget cache serve --host 0.0.0.0 --port 8686 --token "$EGET_CACHE_TOKEN" --json-log
```

Enable a client to try the cache server before origin downloads:

```toml
[cache_mirror]
enable = true
url = "http://192.168.1.10:8686"
```

### Query Examples

**Query repository info**:

```bash
# query repo info
eget query owner/repo
eget query --action releases --limit 5 owner/repo
eget query --action assets --tag v1.2.3 owner/repo

# query SourceForge latest version or assets
eget query sourceforge:winmerge
eget query https://sourceforge.net/projects/victoria-ssd-hdd
eget query --action assets sourceforge:winmerge/stable
eget query --action assets --tag 2.16.44 sourceforge:winmerge/stable
```

**Search GitHub repositories**:

```bash
eget search ripgrep
eget search skillc language:go user:inhere
eget search --limit 5 --sort stars --order desc terminal ui
eget search --json picoclaw user:sipeed
```

### Other Examples

```bash
# uninstall
eget uninstall fzf
# list installed packages
eget list|ls
# list all managed and installed packages
eget list --all
# list packages configured but not installed
eget list --no-installed
# show package details
eget show fzf
# list GUI packages
eget list --gui
# update fzf
eget update fzf
eget update --all
# update eget itself
eget update --self
# check whether eget itself has a newer release
eget update --self --check
```

### Config Examples

```bash
# config
eget add --name fzf --to ~/.local/bin junegunn/fzf
eget config init
eget config list|ls
eget config doctor
eget cfg path --check cache_dir
eget config get global.target
eget config set global.target ~/.local/bin
```

### Supported Targets

The target argument accepted by `install` and `download` can be:

- `name` configured in packages
- GitHub repository, for example `owner/repo`
- GitHub repository URL, for example `https://github.com/owner/repo`
- GitLab target, for example `gitlab:fdroid/fdroidserver` or `gitlab:gitlab.gnome.org/GNOME/gtk`
- Gitea/Forgejo target, for example `gitea:codeberg.org/forgejo/forgejo`
- SourceForge target, for example `sourceforge:winmerge`, `sourceforge:winmerge/stable`, or `https://sourceforge.net/projects/winmerge`
- Direct download URL, for example `https://example.com/file.tar.gz`
- Local file path, for example `file:///path/to/file`

> Note: GitLab and Gitea/Forgejo support currently covers `install`, `download`, and `update` for release assets. SourceForge also supports `query latest` and `query assets`. Search parity and private repository authentication are not provided.

## Available Commands

`install` (aliases: `i`, `ins`)

- Resolve, download, verify, and extract a target, then record installation state.
- `--name` can be used to override the installed executable name; without `--to`, it also acts as the rename hint for single-file assets.
- `--gui` marks the target as a GUI application. GUI `.exe` and `.msi` assets default to installer mode and are launched, unless the selected asset or extracted file name contains `portable`; portable GUI apps use `global.gui_target` by default. Without `--gui`, installer-like assets prompt before launch; with `--add`, a confirmed installer is persisted with `is_gui = true`. Pass `--install-mode portable|installer` or set `install_mode` in package config to override detection.
- With `--add`, a successful install also writes the repo target to `[packages.<name>]`; use `--name` to override the package name.
- Use `--all` without a target to install every package configured under `[packages]`. Package-level options are merged the same way as single package installs.

`download` (alias: `dl`)

- Reuses the install pipeline without recording installed state.
- Downloads the raw asset by default; archive extraction only happens when `--file` or `--extract-all` is set.

`add`

- Writes a managed package definition to `[packages.<name>]` in the config file. When `desc` is not set manually, eget tries to store the repository description.

`uninstall` (aliases: `uni`, `rm`)

- Removes installed files and clears the installed store entry without deleting `[packages.<name>]`.
- Use `--purge` to also remove the matching `[packages.<name>]` definition from config. It does not remove package download cache files.

`list` (alias: `ls`)

- Lists installed packages by default.
- Use `--all` / `-a` to list the union of local managed packages and installed-store entries.
- Use `--no-installed` / `--ni` to list packages configured in `[packages]` but not installed.
- Use `--gui` to filter the current list view to GUI applications.

`show`

- Shows package details merged from `[packages.<name>]` and the installed store, including description, version, status, homepage, repository URL, selected asset, asset URL, install target, and extracted files.
- `packages.<name>.desc` can be set manually. If it is empty, `add` / `install --add` try to fetch the repository description, and installed records also store description/homepage/repository URL metadata when available.

`query` (alias: `q`)

- Queries GitHub repository release metadata and SourceForge latest/assets without installing anything or touching local state.
- Defaults to the `latest` action, and can switch to `info`, `releases`, or `assets` with `--action`.

`search`

- Searches GitHub repositories without installing anything or touching local state.
- Uses the first argument as the keyword and passes remaining arguments through as GitHub search qualifiers, for example `language:go`, `user:inhere`, or `topic:cli`.

`update` (alias: `up`)

- Updates a configured or installed target after checking that a newer version exists, or all managed packages with `--all`.
- `update --self` checks `inherelab/eget` releases, selects the raw executable asset for the current OS/arch, and replaces the running executable. On Windows, replacement is deferred until the current process exits.
- `update --self --check` prints the self-update check host before requesting the latest version, so it is clear whether the check uses GitHub or a private source.
- `update --self --self-source <url>` updates from an internal source that exposes `latest.yaml` plus raw platform files such as `eget-linux-amd64` and `eget-windows-amd64.exe`. `<url>` may be either the base directory or the `latest.yaml` URL. You can also set `EGET_SELF_UPDATE_SOURCE`.

`sdk`

- Downloads and installs versioned SDK archives into `global.sdk_target`, with resumable `.part` downloads and independent records in `sdk.installed.json`.
- Supported install target formats are `name`, `name@latest`, `name:latest`, `name@1.22`, `name:1.22`, `name@1.22.0`, and `name:1.22.0`. Space-separated version forms such as `go 1.22.0` are intentionally not supported.
- `sdk install` accepts one or more SDK targets and installs them serially.
- `sdk list` reads installed SDK records. Use `--json` for machine-readable output.
- `sdk remove <name@version>` removes only paths recorded in the SDK installed store and verified under the configured SDK root.
- `sdk path <target>` prints a configured SDK base path for unversioned targets such as `go` or `java`, and prints an installed version path for versioned targets such as `go:1.20`, `go@1.21.5`, or `java:17`.
- `sdk index list/show/refresh/clear` manages normalized SDK index cache files.
- The first built-in examples target Go and Node. Other SDKs can also work when their archive names can be described by `url_template`, `filename_pattern`, `os_map`, `arch_map`, `ext_map`, and optional HTML/JSON index settings.

`config` (alias: `cfg`)

- Supports `init`, `list` / `ls`, `doctor`, `path [--check] [target]`, `get KEY`, and `set KEY VALUE`.
- `config path` prints one local path for scripting. Supported targets are `config_dir`, `config_file` (default), `env_file`, `bin_dir`, `cache_dir`, `sdk_dir`, `pkg_store_file`, and `sdk_store_file`. With `--check`, output is `path, exists: bool`.
- `config doctor` prints local config/cache/store/install paths, existence checks, writability checks, and set/unset status for sensitive config values without printing secret values.
- Loads optional dotenv variables from the config directory before reading `eget.toml`, so config values such as `github_token = "${GITHUB_TOKEN}"` and `[http_proxy].url = "${PROXY_URL}"` can reference secrets without storing them directly in the config file. Set `EGET_CONFIG_DIR` to move `.env`, `eget.toml`, and install store files together.

## Main Options

`install`, `download`, and `add` share these installation-related options:

- `--tag`: Select a release tag; defaults to `latest` when omitted.
- `--system`: Override the target OS/arch, for example `windows/amd64` or `linux/arm64`.
- `--to`: Set the install or download output path; accepts either a directory or a full file path.
- `--file`: Select file(s) to extract from an archive; supports comma-separated file names or glob patterns such as `README.md,LICENSE`. Exclusions can use `^`, such as `*.exe,^*x86*,^*.sig`; exclude-only expressions such as `^*.sig` match all files except excluded entries. For 7z-readable `.exe` installers, system 7z is required.
- `--asset`: Filter release assets by keyword; multiple filters can be separated by commas. Regex is also supported with the `REG:` prefix, for example `REG:\\.deb$`. Prefix and suffix matching are supported with `PRE:` and `SUF:`, for example `PRE:codex` or `SUF:.zip`. Exclusions can use `^`, such as `^REG:...` or `^SUF:.sha256`. Filters can be scoped to the target OS with a Go OS prefix such as `windows:zip`, `linux:tar.gz`, or `darwin:SUF:.zip`; scoped filters only apply when the current `--system` OS matches.
- `--rename`: Rename extracted files with comma-separated `from=to` pairs, for example `--rename "tool-windows-amd64.exe=tool.exe"`. This works with `--file` and `--extract-all`, and is persisted by `install --add`.
- `--source`: Download the source archive instead of a prebuilt binary release.
- `--extract-all`, `--ea`: Extract all files from the archive instead of selecting a single target file.
- `--strip-components N`: Remove `N` leading archive path components when extracting all files, useful for archives that wrap contents in a versioned top-level directory.
- `--chunk N`: Control HTTP Range chunk concurrency for one downloaded file. `0` means auto, `1` means single-connection download, and values greater than `1` request up to that many chunks.
- `--quiet`: Reduce normal command output for scripting or batch use.

`install` and `download` also support `--prerelease` / `-p` for GitHub targets. With the default latest tag, it selects the newest release including prereleases.

`install` and `download` also support `--fallback-versions N` for SourceForge targets. When the latest version folder does not contain a matching asset, eget scans up to `N` older version folders and uses the first folder where the current `--asset` / `--system` filters produce a single match.

`download` also supports `--ghproxy` for full GitHub release asset URLs. It rewrites only `github.com` download URLs with the configured `[ghproxy].host_url` / `fallbacks`; it does not query or rewrite GitHub API requests.

> Cache behavior is configured via `config set global.cache_dir ...` or the `cache_dir` field in the config file.
> Package download cache files are stored under `{cache_dir}/pkg-cache`; provider metadata API cache, SDK downloads, and SDK indexes use `{cache_dir}/api-cache`, `{cache_dir}/sdk-downloads`, and `{cache_dir}/sdk-index`.

`install` additionally supports:

- `--add`: After a successful install, append the repo target to `[packages.<name>]` managed config.
- `--all`: Install every package configured under `[packages]`; cannot be combined with a target or `--add`.
- `--batch N`: Control package task concurrency for `install --all`. `0` means auto, `1` means serial, and values greater than `1` process up to that many packages at once.
- `--gui`: Install as a GUI application; with `--add`, persist `is_gui = true`. Installer-like assets selected without `--gui` prompt before launch and also persist `is_gui = true` when confirmed with `--add`.
- `--install-mode portable|installer`: Override GUI install mode for this run. By default, `--gui` treats selected `.exe` / `.msi` files as installers unless the selected asset or extracted file name contains `portable`.
- `--name`: Override the managed package name; for single executable assets, it also acts as the default output-name hint.

`update` options:

- `--all`: Check managed packages and update only outdated installed packages.
- `--self`: Update the current `eget` executable instead of a managed package.
- Single-target `update <target>` requires the target to already exist in config or the installed store. Use `install` for new targets.
- `--batch N`: Control package task concurrency for `update --all`. `0` means auto, `1` means serial, and values greater than `1` process up to that many packages at once.
- `--chunk N`: Control HTTP Range chunk concurrency for downloads triggered by update.
- `--check`: Check and list outdated installed packages, same as `list --outdated`. When targets are provided, for example `eget up --check markview jq`, only those packages are checked.
- `--dry-run`: Preview the update plan without performing installation changes.
- `--interactive`: Interactively select which managed packages to update.

`query` options:

- `--action`, `-a`: Query action. Supported values: `latest`, `releases`, `assets`, `info`.
- `--tag`, `-t`: Select the release tag for the `assets` action; defaults to latest when omitted.
- `--limit`, `-l`: Limit the number of rows returned by the `releases` action. Default: `10`.
- `--json`, `-j`: Output JSON for scripting or automation.
- `--prerelease`, `-p`: Include prerelease entries for `latest` and `releases`.

SourceForge query targets use `sourceforge:<project>`, `sourceforge:<project>/<path>`, `sf:<project>`, or SourceForge project URLs such as `https://sourceforge.net/projects/winmerge`. They currently support `latest`, `releases`, and `assets`; `info` remains GitHub-only.

`search` options:

- `--limit`, `-l`: Limit the number of repositories returned. Default: `10`.
- `--sort`: Sort search results. Supported values: `stars`, `updated`.
- `--order`: Sort order. Supported values: `desc`, `asc`.
- `--json`, `-j`: Output JSON for scripting or automation.

`sdk` options:

- `sdk install --force`: Remove an existing SDK target directory after safety checks, then reinstall.
- `sdk list --json`: Output installed SDK records as JSON.
- `sdk index list --json`: Output cached SDK index summaries as JSON.
- `sdk index refresh --all`: Refresh all configured SDK indexes with `index_url`.
- `sdk index clear --all`: Delete all cached SDK index JSON files.

Global options:

- `-v`, `--verbose`: Show more execution details such as API requests, response summaries, asset selection, cache hits, and key workflow steps.

Notes:

- `install --name` can rename a single executable asset, for example installing `chlog-windows-amd64.exe` as `chlog.exe`.
- `install --rename` can rename selected files while extracting multiple files; the config field is `rename_files`, for example `rename_files = { "codex-x86_64-pc-windows-msvc.exe" = "codex.exe" }`.
- `install --add` only applies to repo targets and appends the managed package definition after a successful install.
- `global.gui_target` is used only for portable GUI applications. GUI installers such as `.msi` or `setup.exe` are launched and do not record a final install directory. Use `install_mode = "installer"` for installer `.exe` assets with product/version/platform names rather than setup-style names.
- `download` stores the raw downloaded asset by default; extraction only happens when `--file` or `--extract-all` is provided.
- `sdk` uses `global.sdk_target` for install directories and `{cache_dir}/sdk-downloads` for archive downloads. Resumable state is stored as `.part` plus `.meta.json`.
- Archive extraction currently supports `zip`, `tar.*`, and `7z`. System 7z is preferred for `.7z`, `.rar`, `.msi`, `.cab`, `.iso`, and `--extract-all` `.exe` archives when `global.sys7z_path` or `PATH` provides `7z`, `7zz`, or `7za`; `tar.*` archives continue to use the built-in Go extractor.
- Command options can appear before or after positional arguments, for example `eget install --tag nightly owner/repo` and `eget install owner/repo --tag nightly` are equivalent.

## Configuration

The config file is resolved in this order:

1. `EGET_CONFIG`
2. `{EGET_CONFIG_DIR}/eget.toml`
3. Legacy `~/.eget.toml`
4. Local `eget.toml`
5. XDG / home fallback path, such as `~/.config/eget/eget.toml`

`EGET_CONFIG` only changes the `eget.toml` path. `EGET_CONFIG_DIR` changes the default config directory used by `.env`, `eget.toml`, package install records, and SDK install records.

Supported config sections:

- `[global]`
- `[http_proxy]`
- `["owner/repo"]`
- `[packages.<name>]`
- `[pkg_templates.<name>]`
- `[sdk.<name>]`

Minimal example:

```toml
[global]
target = "~/.local/bin"
cache_dir = "~/.cache/eget"
user_agent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36"
sdk_target = "~/.local/sdks"

[http_proxy]
url = "http://127.0.0.1:7890"
exclude = []

[packages.markview]
repo = "inhere/markview"
tag = "nightly"
asset_filters = ["windows"]
rename_files = { "markview-windows-amd64.exe" = "markview.exe" }

[packages.markview_mirror]
repo = "template:markview"
latest_url = "https://example.com/tools/markview/latest.yaml"
latest_format = "yaml"
url_template = "https://example.com/tools/markview/markview-{version}-{os}-{arch}{ext}"
os_map = { windows = "windows", linux = "linux", darwin = "darwin" }
arch_map = { amd64 = "amd64", arm64 = "arm64" }
ext_map = { windows = ".zip", linux = ".tar.gz", darwin = ".tar.gz" }
extract_file = "markview"

[packages.claude]
repo = "template:claude"
latest_url = "https://downloads.claude.ai/claude-code-releases/latest"
latest_format = "text"
url_template = "https://downloads.claude.ai/claude-code-releases/{version}/{os}-{arch}{libc}/claude{ext}"
os_map = { windows = "win32", linux = "linux", darwin = "darwin" }
arch_map = { amd64 = "x64", arm64 = "arm64" }
ext_map = { windows = ".exe", linux = "", darwin = "" }
libc_map = { glibc = "", musl = "-musl" }
checksum_url_template = "https://downloads.claude.ai/claude-code-releases/{version}/manifest.json"
checksum_format = "json"
checksum_json_path = "platforms.{os}-{arch}{libc}.checksum"
install_action = "run-asset"
install_args = ["install", "latest"]

[pkg_templates.mydev]
latest_url = "http://mydev.lan/tools/{name}/latest.yaml"
url_template = "http://mydev.lan/tools/{name}/{name}-{os}-{arch}{ext}"

[packages.markview_internal]
repo = "pkg-template:mydev:markview"

[sdk.go]
aliases = ["golang"]
target = "gosdk/go{version}"
url_template = "https://mirrors.aliyun.com/golang/go{version}.{os}-{arch}.{ext}"
index_url = "https://mirrors.aliyun.com/golang/"
index_format = "html"
filename_pattern = "go{version}.{os}-{arch}.{ext}"
strip_components = 1
ext_map = { windows = "zip", linux = "tar.gz", darwin = "tar.gz" }

[sdk.node]
aliases = ["nodejs"]
target = "nodejs/node{version}"
url_template = "https://mirrors.aliyun.com/nodejs-release/v{version}/node-v{version}-{os}-{arch}.{ext}"
index_url = "https://mirrors.aliyun.com/nodejs-release/"
index_format = "html"
index_path_prefix = "/nodejs-release/"
filename_pattern = "node-v{version}-{os}-{arch}.{ext}"
strip_components = 1
os_map = { windows = "win", linux = "linux", darwin = "darwin" }
arch_map = { amd64 = "x64", arm64 = "arm64", 386 = "x86" }
ext_map = { windows = "zip", linux = "tar.xz", darwin = "tar.gz" }
```

You can also write built-in SDK templates with:

```bash
eget sdk config add --all
eget sdk config add --all --mirror mirror
eget sdk config add jdk --mirror zulu
```

Create a default config:

```bash
eget config init
```

By default, this writes `~/.config/eget/eget.toml`.

`template:<id>` can describe packages from independent download sites, such as Claude Code tools published through latest metadata and URL templates. `run-asset` only executes the downloaded asset after checksum verification; it is not a general `post_install`. When multiple internal tools share one release layout, `pkg_templates` can reuse that template, and you can run `eget add mydev:markview` or `eget install mydev:markview` directly. See [docs/config.md](docs/config.md) for the full configuration reference, including global fields, package sections, Template Package Source, pkg_templates, SDK sections, cache directories, installed-state files, ghproxy, API cache, and SDK index settings. For SDK-specific usage, see [docs/sdk-usage.md](docs/sdk-usage.md).

## Build and Test

```bash
make build
make test
```

## Project Structure

The current version has been restructured into an explicit subcommand CLI, with the entry point in `cmd/eget/main.go` and business logic concentrated under `internal/`.

- `cmd/eget`: command entry point
- `internal/cli`: `gcli` command registration and argument binding
- `internal/app`: install/add/list/update/config use-case orchestration
- `internal/install`: find, download, verify, and extract execution pipeline
- `internal/config`: config loading, merging, and persistence
- `internal/installed`: installed-state storage
- `internal/sdk`: SDK target parsing, index cache, resumable downloads, extraction, and SDK installed-state storage
- `internal/source/github`: GitHub asset discovery
- `internal/source/forge`: GitLab/Gitea/Forgejo asset discovery

> For more details, see [docs/architecture.md](docs/architecture.md).

## References

- [https://github.com/zyedidia/eget](https://github.com/zyedidia/eget)
- [https://github.com/gmatheu/eget](https://github.com/gmatheu/eget)
