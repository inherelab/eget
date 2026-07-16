# Configuration Reference

This document describes the `eget` configuration file. The README keeps only a short overview; use this file when you need the complete field list and directory semantics.

## Config Lookup

`eget` resolves the config file in this order:

1. `EGET_CONFIG`
2. `{EGET_CONFIG_DIR}/eget.toml`
3. Legacy `~/.eget.toml`
4. Local `eget.toml`
5. XDG / home fallback path, such as `~/.config/eget/eget.toml`

`EGET_CONFIG` only changes the `eget.toml` file path. `EGET_CONFIG_DIR` changes the default config directory used by `.env`, `eget.toml`, `installed.toml`, and `sdk.installed.json`.

Create the default config:

```bash
eget config init
```

By default, this writes:

```text
~/.config/eget/eget.toml
```

`eget` also loads dotenv variables from:

```text
~/.config/eget/.env
```

If `XDG_CONFIG_HOME` is set, the dotenv path follows the same config directory:

```text
$XDG_CONFIG_HOME/eget/.env
```

If `EGET_CONFIG_DIR` is set, the dotenv path is:

```text
{EGET_CONFIG_DIR}/.env
```

The dotenv file is optional. It is loaded before `eget.toml`, so config values can reference secrets or internal settings through gookit/config env expansion:

```dotenv
GITHUB_TOKEN=...
PROXY_URL=http://127.0.0.1:7890
EGET_SELF_UPDATE_SOURCE=https://example.com/tools/eget/
```

```toml
[global]
github_token = "${GITHUB_TOKEN}"

[http_proxy]
url = "${PROXY_URL}"
```

Keep `.env` out of version control.

## Import and Export

Export a portable configuration from the old machine:

```bash
eget config export portable.toml
```

The entire `[global]` section is excluded by default because it commonly contains machine-specific target paths, cache paths, proxy settings, and tokens. Include it only for a complete backup:

```bash
eget config export --with-global complete.toml
```

When FILE is omitted, the command writes pure TOML to stdout for redirection or piping. The operation is rejected when the output and active config refer to the same file, preventing the source config from being truncated. Import it on the new machine with:

```bash
eget config import portable.toml
eget config import --force portable.toml
```

An existing target config requires interactive confirmation unless `--force` is used. The operation is rejected when the source and target refer to the same file. If the source omits `[global]`, the target machine's current `[global]` is retained; if the source contains `[global]`, it is replaced. Every other top-level section is replaced as a whole from the source instead of merging individual packages. The source TOML is fully validated before the target is safely replaced. Re-serialization does not preserve the source file's comments or formatting.

## Sections

Supported sections:

- `[global]`: global defaults and network/cache settings.
- `[http_proxy]`: preferred global HTTP-layer proxy settings.
- `[api_cache]`: metadata API response cache.
- `[cache_mirror]`: LAN cache mirror client settings.
- `[ghproxy]`: GitHub URL rewrite proxy.
- `["owner/repo"]`: legacy direct package section.
- `[packages.<name>]`: named package section.
- `[pkg_templates.<name>]`: reusable package URL template section.
- `[sdk.<name>]`: SDK download and index section.

## Global Section

Example:

```toml
[global]
target = "~/.local/bin"
gui_target = "~/Applications"
cache_dir = "~/.cache/eget"
user_agent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36"
system = ""
sys7z_path = ""
chunk_concurrency = 0
batch_concurrency = 0
ignore_update_packages = []
sdk_target = "~/.local/sdks"
sdk_ext_map = { windows = "zip", linux = "tar.gz", darwin = "tar.gz" }
```

Fields:

- `target`: default install directory for CLI tools.
- `gui_target`: default install directory for portable GUI applications.
- `cache_dir`: default cache root. Raw downloads, API cache files, SDK downloads, and SDK indexes are derived from this directory.
- `proxy_url`: legacy HTTP-layer proxy fallback. Prefer `[http_proxy].url`; `global.proxy_url` is only read when `[http_proxy]` is not configured.
- `user_agent`: default HTTP `User-Agent`. When empty, eget uses the built-in browser UA; configured values override the default.
- `system`: default target platform in `GOOS/GOARCH` form, for example `windows/amd64`.
- `sys7z_path`: optional 7z executable path. When empty, eget searches `PATH` for `7z`, `7zz`, then `7za`.
- `chunk_concurrency`: default remote download chunk concurrency. `0` means the built-in default behavior.
- `batch_concurrency`: default concurrency for batch package operations and outdated checks. `0` auto-selects up to 6 workers, `1` forces serial execution, and values greater than `1` use that many workers up to the package count.
- `ignore_update_packages`: package names skipped by `list --outdated`, `update --check`, and `update --all`.
- `sdk_target`: SDK installation root. Relative SDK `target` values are resolved under this root.
- `sdk_ext_map`: default SDK archive extension map by Go OS name. SDK-level `ext_map` overrides it.

Directory semantics:

- `download` uses `cache_dir` by default when `--to` is not provided.
- `install` and `download` reuse package download cache files from `{cache_dir}/pkg-cache/` when available.
- API cache files are stored under `{cache_dir}/api-cache/`.
- SDK archive downloads are stored under `{cache_dir}/sdk-downloads/`.
- SDK index JSON files are stored under `{cache_dir}/sdk-index/`.

## HTTP Proxy

Use `[http_proxy]` for global HTTP-layer proxy settings:

```toml
[http_proxy]
enable = true
url = "http://127.0.0.1:10801"
exclude = ["mydev.com", "*.corp.local", "10.0.0.0/8"]
```

Fields:

- `enable`: whether to enable the configured HTTP proxy. `false` disables this config proxy.
- `url`: proxy URL used by GitHub lookups, remote downloads, and SDK requests. An empty URL disables this config proxy.
- `exclude`: host rules that skip the configured proxy for matching request hosts.

The app-level `--no-proxy` option disables configured HTTP proxy settings for one run. `NO_PROXY=1`, `NO_PROXY=true`, `NO_PROXY=yes`, or `NO_PROXY=on` also disables configured proxy settings. Other comma-separated `NO_PROXY` values, such as `NO_PROXY=mydev.com,*.corp.local`, are merged into `exclude`.

`global.proxy_url` is still read as a legacy fallback when `[http_proxy]` is not configured. `[http_proxy]` is preferred and wins over `global.proxy_url` when present.

## API Cache

Example:

```toml
[api_cache]
enable = false
cache_time = 1800
```

Fields:

- `enable`: whether to cache known provider metadata responses.
- `cache_time`: cache TTL in seconds.

The API cache stores known provider metadata `GET` responses, including GitHub API, GitLab/Gitea release API, and SourceForge files listings. Cache files are stored under `{cache_dir}/api-cache/`.

## Cache Mirror

`[cache_mirror]` lets `install`, `download`, and `sdk install` try a LAN `eget cache serve` instance before downloading from the original source.

```toml
[cache_mirror]
enable = true
url = "http://192.168.1.10:8686"
timeout = 5
fallback = true
```

Fields:

- `enable`: enable cache mirror lookup before origin downloads.
- `url`: cache server base URL, usually an `eget cache serve --host 0.0.0.0 --port 8686` instance.
- `timeout`: mirror connect, TLS handshake, and response-header timeout in seconds. Values less than or equal to `0` use the default 5 seconds. The timeout does not cap the full file body download duration, so large LAN mirror downloads can exceed this value once the server starts responding.
- `fallback`: when `true`, mirror miss or error falls back to the original source. When `false`, mirror miss or error stops the download.

The first mirror protocol uses a path key based on the normalized cache relative path. It can reuse old cache files already present on the mirror server. The mirror is an optimization, not a trust root; checksum verification still uses existing package verification when configured.

### Fully offline installation for known targets

On an online machine, query or install the target normally so both `api-cache` and `pkg-cache` under the same cache root are warm, then start the cache server:

```bash
eget install owner/tool
eget cache serve --host 0.0.0.0 --port 8686
```

When the offline client already knows the repo, package alias, or pkg-template target, enable strict mode:

```toml
[cache_mirror]
enable = true
url = "http://192.168.1.10:8686"
fallback = false
```

`fallback = false` blocks origin access for both provider metadata and asset downloads. Missing metadata reports `cache mirror metadata miss`; available metadata with a missing asset reports `cache mirror miss`. Setting `api_cache.enable = false` only disables the normal API cache policy; it does not disable the metadata mirror. Mirrored metadata is still staged in the local `api-cache` for parsing and later reuse.

Phase one supports only targets already known to the client. It does not search or list installable tools from the cache server. A package/version/platform catalog belongs to a later, separate phase.

`[cache_mirror]` is client-side lookup configuration. Server access protection is a runtime `cache serve` option:

```bash
eget cache serve --token "$EGET_CACHE_TOKEN"
```

`cache serve` prints text request logs by default. Add `--json-log` when structured JSON lines are preferred.

Do not put bearer tokens in `[cache_mirror]`; current mirror client downloads do not send a token. If authenticated mirror client downloads are needed later, they should be designed as a separate client/server contract.

## GitHub Proxy

Example:

```toml
[ghproxy]
host_url = ""
fallbacks = []
```

Fields:

- `host_url`: primary proxy host, for example `https://ghfast.top/`.
- `fallbacks`: fallback proxy hosts tried in order when the primary proxy fails.

`http_proxy` and `ghproxy` solve different problems. `[http_proxy]` is an HTTP-layer proxy, while `ghproxy` rewrites GitHub download URLs. `ghproxy` is only used by `download --ghproxy` and does not rewrite `api.github.com` requests. Legacy `global.proxy_url` is only a fallback for `[http_proxy].url`.

```bash
eget dl --ghproxy https://github.com/owner/repo/releases/download/v1.2.3/tool.zip
```

## Package Sections

Use `[packages.<name>]` for named package management.

Example:

```toml
[packages.markview]
repo = "inhere/markview"
target = "~/.local/bin"
tag = "nightly"
asset_filters = ["windows"]
extract_all = true
strip_components = 1
```

Common fields:

- `repo`: package source. Supports GitHub-style `owner/repo`, direct URLs, SourceForge, supported forge prefixes, and `template:<id>`.
- `target`: install directory for this package.
- `system`: package-specific target platform in `GOOS/GOARCH` form.
- `tag`: version tag or release tag preference.
- `source_path`: SourceForge files path filter, for example `stable`.
- `file`: file filter or output filename depending on command context.
- `asset_filters`: substrings used to match release assets.
- `download_source`: download source archives instead of release assets.
- `extract_all`: extract all files from the selected archive.
- `strip_components`: number of leading archive path segments to remove when extracting all files.
- `is_gui`: install as GUI package, using `gui_target` semantics.
- `install_mode`: optional GUI install mode. With `is_gui = true`, selected `.exe` and `.msi` files default to `installer` unless the selected asset or extracted file name contains `portable`. Set `portable` or `installer` here to override detection for a package; for one-off installs, use `install --gui --install-mode portable|installer ...`.
- `quiet`: reduce output for this package.
- `upgrade_only`: only update when an installed package already exists.

Legacy direct sections are also supported:

```toml
["inhere/markview"]
tag = "nightly"
```

Prefer `[packages.<name>]` for new config because it provides an explicit local package name.

### Template Package Source

`repo = "template:<id>"` is for independent download sites outside the built-in release providers. It reads latest-version metadata, renders a download URL, optionally reads a checksum manifest, then continues through the normal install, update, and installed-store flow.

Claude Code example:

```toml
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
```

YAML latest metadata example:

```yaml
version: v1.2.3
released_at: 2026-05-25T10:20:30+08:00
```

```toml
[packages.markview]
repo = "template:markview"
latest_url = "https://example.com/tools/markview/latest.yaml"
url_template = "https://example.com/tools/markview/markview-{version}-{os}-{arch}{ext}"
os_map = { windows = "windows", linux = "linux", darwin = "darwin" }
arch_map = { amd64 = "amd64", arm64 = "arm64" }
extract_file = "markview"
```

Fields:

- `latest_url`: latest-version metadata URL.
- `latest_format`: `text`, `json`, or `yaml`. When empty, eget infers it from `latest_url` suffixes `.yaml`, `.yml`, `.json`, and `.txt`; unknown suffixes fall back to `text`. YAML reads `version` and optional `released_at`.
- `latest_json_path`: dot path used when `latest_format = "json"`.
- `version_regex`: optional version extraction regex. If it has a capture group, the first group is used; otherwise the full match is used.
- `url_template`: download URL template.
- `os_map` / `arch_map` / `ext_map` / `libc_map`: map local platform variables to the download site's naming.
  - For template packages, `{ext}` defaults to `.exe` on Windows and an empty string on Linux/macOS; set `ext_map` only when the download site uses a different suffix such as `.zip` or `.tar.gz`.
- `checksum_url_template`: checksum metadata URL template.
- `checksum_format`: `text` or `json`.
- `checksum_json_path`: dot path used when `checksum_format = "json"`; template variables are allowed.
- `checksum_regex`: optional checksum extraction regex.
- `install_action = "run-asset"`: after download and checksum verification, execute the downloaded asset itself.
- `install_args`: argument array passed to `run-asset`.

`url_template`, `checksum_url_template`, and JSON path templates support:

- `{name}`: template id.
- `{version}`: latest or command-selected version.
- `{os}`: OS after `os_map`.
- `{arch}`: arch after `arch_map`.
- `{ext}`: extension after `ext_map`; defaults to `.exe` on Windows and empty on Linux/macOS when `ext_map` is not set.
- `{libc}`: Linux libc value after `libc_map`; empty outside Linux or when libc is unknown.

`run-asset` is not a general `post_install`. It only executes the downloaded asset after checksum verification, arguments must be an array, and no shell is used. Template `latest_url` and `checksum_url_template` values are arbitrary site metadata. Requests reuse HTTP options such as `[http_proxy]` and `disable_ssl`, but they are not forced into provider API cache classification.

### pkg_templates

`[pkg_templates.<name>]` reuses package template fields for internal tools that share the same release layout and differ only by tool name.

```toml
[pkg_templates.mydev]
latest_url = "http://mydev.lan/tools/{name}/latest.yaml"
latest_format = "yaml"
url_template = "http://mydev.lan/tools/{name}/{name}-{os}-{arch}{ext}"
ext_map = { windows = ".exe", linux = "", darwin = "" }

[packages.markview]
repo = "pkg-template:mydev:markview"
```

Short aliases are also supported:

```bash
eget add mydev:markview
eget install mydev:markview
eget install --add mydev:markview
```

The short alias is active only when `mydev` matches a configured `[pkg_templates.mydev]` section. Persisted package config keeps the lightweight reference `repo = "pkg-template:mydev:markview"` and does not expand template URLs into the package section.

## SDK Sections

Use `[sdk.<name>]` to configure SDK archive downloads.

You can also write built-in SDK templates with:

```bash
eget sdk config add --all
eget sdk config add --all --mirror mirror
eget sdk config add jdk --mirror zulu
```

Example:

```toml
[sdk.go]
aliases = ["golang"]
target = "gosdk/go{version}"
url_template = "https://mirrors.aliyun.com/golang/go{version}.{os}-{arch}.{ext}"
index_url = "https://mirrors.aliyun.com/golang/"
index_format = "html"
filename_pattern = "go{version}.{os}-{arch}.{ext}"
strip_components = 1
ext_map = { windows = "zip", linux = "tar.gz", darwin = "tar.gz" }
```

Fields:

- `aliases`: SDK aliases. For example, `[sdk.go]` with `aliases = ["golang"]` allows `eget sdk install golang@1.22.0`.
- `target`: installation directory template. Relative paths are resolved under `global.sdk_target`.
- `url_template`: archive download URL template.
- `index_url`: remote HTML or JSON index URL.
- `index_format`: index format, usually `html` or `json`.
- `index_parser`: JSON index parser. Currently supported values are `go-json`, `node-json`, and `zulu-json`.
- `index_path_prefix`: path prefix filter for HTML index links.
- `filename_pattern`: archive filename pattern used by HTML index parsing.
- `strip_components`: number of leading archive path segments to remove during extraction.
- `os_map`: map Go OS names to SDK release OS names.
- `arch_map`: map Go arch names to SDK release arch names.
- `ext_map`: map Go OS names to SDK archive extensions. Overrides `global.sdk_ext_map`.

Template variables supported by `target`, `url_template`, and `filename_pattern`:

- `{name}`: SDK name.
- `{version}`: selected version.
- `{os}`: OS value after `os_map`.
- `{arch}`: arch value after `arch_map`.
- `{ext}`: archive extension after `ext_map`.

HTML index parsing supports two common layouts:

- Direct archive links, such as `go1.22.0.linux-amd64.tar.gz`.
- Version directory links, such as `v20.11.1/`. When `url_template` is configured, eget builds the current-platform archive URL from the directory version.

For SDK usage details, see [sdk-usage.md](sdk-usage.md).

## Store Files

Package install records default to:

```text
~/.config/eget/installed.toml
```

SDK install records default to:

```text
~/.config/eget/sdk.installed.json
```

When `EGET_CONFIG_DIR` is set, these records move to `{EGET_CONFIG_DIR}/installed.toml` and `{EGET_CONFIG_DIR}/sdk.installed.json`.

SDK records are separate because SDKs commonly have multiple installed versions, while normal packages are usually managed as one active installed artifact.

## Full Example

See [example.eget.toml](example.eget.toml) for a larger example covering packages, Go, Node, Python, and JDK-style SDK experiments.
