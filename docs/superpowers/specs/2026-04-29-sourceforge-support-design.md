# SourceForge Support Design

## Goal

Add first-class SourceForge project support while keeping the existing install pipeline focused on source discovery, asset selection, download, verification, extraction, and installed-state recording.

This design adds:

- `sourceforge:<project>` as a supported install target and managed package repo value.
- Optional `source_path` package/repo configuration to constrain SourceForge file discovery.
- A new `internal/source/sourceforge` backend that returns real downloadable asset URLs.
- SourceForge latest-version checking for `list --outdated`, `update --check`, and `update --all`.
- Installed-store metadata that records SourceForge targets in the same lifecycle as GitHub targets.

The main user-facing configuration stays compact:

```toml
[packages.winmerge]
repo = "sourceforge:winmerge"
source_path = "stable"
system = "windows/amd64"
asset_filters = ["x64", "setup"]
```

## Non-Goals

This first version does not support:

- SourceForge project search.
- `query sourceforge:...` parity with GitHub query commands.
- SourceForge account authentication.
- Mirroring every SourceForge project layout automatically.
- Silent installer flags or project-specific install recipes.
- Rewriting arbitrary SourceForge web pages into managed package config unless the URL is already a standard project/files URL.

## Concepts

### SourceForge Target

A SourceForge target is one of:

```text
sourceforge:<project>
sourceforge:<project>/<path>
```

Examples:

```text
sourceforge:winmerge
sourceforge:winmerge/stable
```

The target is installable directly:

```bash
eget install sourceforge:winmerge --asset x64,setup
```

It can also be persisted as a managed package:

```bash
eget add sourceforge:winmerge --name winmerge --system windows/amd64 --asset x64,setup
```

Direct installs do not require prior configuration. Batch update flows still require managed package config because `update --all` only works over configured packages.

### Source Path

`source_path` is an optional directory constraint under the SourceForge project's files area.

```toml
[packages.winmerge]
repo = "sourceforge:winmerge"
source_path = "stable"
```

If omitted, the SourceForge backend attempts project-level latest discovery. If present, discovery is restricted under that path. This field exists because SourceForge projects often have irregular directory layouts.

`source_path` is not an asset filter. It narrows where release files are discovered. Existing `asset_filters`, `system`, and `file` settings still perform asset selection after candidates are discovered.

## Configuration Model

Add `SourcePath` to `config.Section`:

```go
SourcePath *string `toml:"source_path" mapstructure:"source_path"`
```

Add the merged runtime value:

```go
SourcePath string
```

Precedence follows the existing install option model:

```text
CLI > package > repo > global > default
```

The MVP can expose `source_path` in config first. A CLI flag such as `--source-path` can be added later if direct installs need one-off path constraints without writing config.

## Target Detection

Extend target detection with:

```go
TargetSourceForge TargetKind = "sourceforge"
```

Rules:

- `sourceforge:<project>` is a SourceForge target.
- `sourceforge:<project>/<path>` is a SourceForge target with an inline path.
- Standard SourceForge project/file URLs may be recognized later, but direct URL download behavior remains valid for any SourceForge URL.

Parsing should produce:

```text
project: winmerge
path: stable
```

The inline path and configured `source_path` should not silently conflict. Package config should prefer explicit package `source_path`; direct target inline path should be used when no config path exists. If both are present and different, return a clear error.

## Finder Design

Add a new package:

```text
internal/source/sourceforge
```

The backend should expose a finder compatible with the existing install interface:

```go
type Finder struct {
    Project string
    Path    string
    Tag     string
    Getter  HTTPGetter
}

func (f *Finder) Find() ([]string, error)
```

The finder returns real asset URLs, not SourceForge HTML pages, so the existing detector and downloader can run unchanged.

Preferred discovery order:

1. If `source_path` or inline path is provided, list files under that SourceForge path.
2. If `tag` is provided, use it as a version/path hint under the constrained path.
3. If no path is provided, use SourceForge latest discovery for the project.
4. Return candidate file download URLs.

The initial implementation should prefer SourceForge's machine-readable file listing endpoint if available and stable enough. If the endpoint is not sufficient, a small HTML parser can be added locally inside `internal/source/sourceforge`, but it should remain isolated from the install runner.

## Asset Selection

Do not add SourceForge-specific asset selection rules to the runner.

Once SourceForge returns candidate URLs, the existing selection chain handles:

- `system`
- `asset_filters`
- `file`
- `extract_all`
- prompt fallback for multiple candidates
- hard failure for zero matching assets

This keeps the recently fixed behavior intact: if the current SourceForge release has no matching asset, installation stops with a clear error and does not fall back to an old installed asset.

## Installed Store

Installed entries should keep using the existing fields:

```toml
[installed."sourceforge:winmerge"]
repo = "sourceforge:winmerge"
target = "sourceforge:winmerge"
url = "https://downloads.sourceforge.net/project/..."
asset = "WinMerge-2.16.44-x64-Setup.exe"
tag = "2.16.44"
version = "2.16.44"
```

The key should be the normalized SourceForge target:

```text
sourceforge:<project>
```

If `source_path` is needed to reproduce update checks, it should be recorded either in `Options` or derived from managed config during update. The installed store should not become the primary source of managed package configuration.

## Version Discovery

Add a source-aware latest checker used by list/update flows.

Current behavior:

- GitHub packages use GitHub latest release tag.

New behavior:

- GitHub packages continue to use GitHub latest release tag.
- SourceForge packages use SourceForge latest version discovery.

Version discovery rules for SourceForge:

1. Use `source_path` if configured.
2. Identify version-like path segments or file names with semantic-version-friendly parsing.
3. Prefer stable releases over obvious source/debug/checksum-only files.
4. If a version cannot be parsed reliably, return a check failure instead of silently updating.

The `list --outdated`, `update --check`, and `update --all` outputs should report SourceForge check failures in the existing `check_failed` format.

## Update Flow

Single update:

```bash
eget update winmerge
eget update sourceforge:winmerge
```

`update winmerge` resolves managed config and can use `source_path`.

`update sourceforge:winmerge` is allowed as a direct update target, but without managed config it only has CLI options and installed-store metadata. For stable long-term updates, users should configure a managed package.

Batch update:

```bash
eget update --all
```

Only managed packages are considered. For SourceForge managed packages:

1. Resolve package config.
2. Check latest SourceForge version.
3. Compare with installed store tag/version.
4. Update only if a newer version is found.
5. Stop with an explicit error if the latest version has no matching asset.

## Error Handling

Errors should be explicit and source-specific:

- Invalid target: `invalid SourceForge target "sourceforge:"`
- Missing project: `sourceforge project is required`
- Conflicting paths: `source_path "stable" conflicts with target path "beta"`
- No files: `no SourceForge files found for winmerge/stable`
- No version: `could not determine SourceForge latest version for winmerge`
- No asset match: reuse existing detector errors such as `asset "x64" not found`

Do not silently fall back to older installed SourceForge assets when the current release does not match.

## Testing Strategy

Unit tests:

- Target detection for `sourceforge:<project>` and `sourceforge:<project>/<path>`.
- SourceForge target parsing and normalization.
- Config merge for `source_path`.
- Finder parsing for mocked SourceForge file listing responses.
- Latest checker version parsing and failure modes.
- Install runner integration with SourceForge finder candidates.
- `list --outdated` and `update --all` with mixed GitHub and SourceForge packages.

Regression tests:

- Current release with no matching SourceForge asset stops before download.
- Unparseable SourceForge version becomes a check failure, not a false update.
- Direct SourceForge URL continues to work as a normal direct URL.

Full verification:

```bash
go test ./...
```

## Documentation Updates

Update:

- `README.md`
- `README.zh-CN.md`
- `docs/DOCS.md`
- `docs/example.eget.toml`

Docs should show:

```bash
eget install sourceforge:winmerge --asset x64,setup
eget add sourceforge:winmerge --name winmerge --system windows/amd64 --asset x64,setup
```

And:

```toml
[packages.winmerge]
repo = "sourceforge:winmerge"
source_path = "stable"
system = "windows/amd64"
asset_filters = ["x64", "setup"]
```

## Open Decisions

The implementation plan should validate the best SourceForge file listing endpoint before coding the finder. If SourceForge's machine-readable endpoint is insufficient, the fallback parser should be scoped to extracting file names, directories, download URLs, timestamps, and sizes from the relevant project files page only.

The first implementation should not promise perfect support for every SourceForge project. It should provide a reliable path for common projects and clear errors when project layout prevents safe version detection.
