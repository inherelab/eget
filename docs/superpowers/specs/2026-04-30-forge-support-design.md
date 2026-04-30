# GitLab and Gitea Forge Support Design

## Goal

Add first-class public release support for GitLab, Gitea, and Forgejo-style repositories while preserving the existing install pipeline:

```text
source target -> candidate asset URLs -> system/asset/file selection -> download -> verify -> extract -> installed store
```

The first implementation should support public release assets on both public and self-hosted instances. Authentication for private repositories is intentionally deferred, but the target and config model should not block adding tokens later.

## Supported Targets

Use provider-prefixed, host-aware targets:

```text
gitlab:<namespace>/<project>
gitlab:<host>/<namespace>/<project>
gitea:<host>/<owner>/<repo>
forgejo:<host>/<owner>/<repo>
```

Examples:

```bash
eget install gitlab:fdroid/fdroidserver
eget install gitlab:gitlab.gnome.org/GNOME/gtk
eget install gitea:codeberg.org/forgejo/forgejo
eget install forgejo:codeberg.org/forgejo/forgejo
```

Rules:

- `gitlab:<namespace>/<project>` defaults to `gitlab.com`.
- `gitlab:<host>/<namespace>/<project>` targets a self-hosted GitLab-compatible instance.
- `gitea:` and `forgejo:` require an explicit host.
- `forgejo:` is treated as a Gitea-compatible API target with a distinct normalized prefix.
- Existing `owner/repo` targets continue to mean GitHub only.
- Direct `https://...` URLs continue to mean direct downloads unless they are explicitly supported later as repository page URLs.

## Managed Config

Managed packages keep using `repo` as the source identity:

```toml
[packages.forgejo]
repo = "gitea:codeberg.org/forgejo/forgejo"
system = "linux/amd64"
asset_filters = ["linux", "amd64"]

[packages.gtk]
repo = "gitlab:gitlab.gnome.org/GNOME/gtk"
system = "linux/amd64"
asset_filters = ["linux", "x86_64"]
```

No new required config fields are needed for the first version.

Token fields can be added later without changing target syntax:

```toml
[global]
gitlab_token = "..."
gitea_token = "..."

["gitlab:gitlab.example.com/group/project"]
token = "..."
```

These token fields are not part of the first implementation.

## Normalization

Installed store and config normalization should use full host-qualified keys:

```text
gitlab:gitlab.com/fdroid/fdroidserver
gitlab:gitlab.gnome.org/GNOME/gtk
gitea:codeberg.org/forgejo/forgejo
forgejo:codeberg.org/forgejo/forgejo
```

This means:

- `gitlab:fdroid/fdroidserver` is accepted as user input.
- It is normalized to `gitlab:gitlab.com/fdroid/fdroidserver`.
- `gitea:codeberg.org/forgejo/forgejo` stays unchanged.
- Case in namespace/project should be preserved in URLs, but comparisons should use the normalized string already stored by the package config.

The installed store key should be the normalized target, not the raw user input.

## Architecture

Add a small shared forge package for target parsing and provider dispatch:

```text
internal/source/forge
```

Recommended files:

```text
internal/source/forge/target.go
internal/source/forge/finder.go
internal/source/forge/gitlab.go
internal/source/forge/gitea.go
internal/source/forge/log.go
```

Core model:

```go
type Provider string

const (
	ProviderGitLab Provider = "gitlab"
	ProviderGitea  Provider = "gitea"
	ProviderForgejo Provider = "forgejo"
)

type Target struct {
	Provider   Provider
	Host       string
	Namespace  string
	Project    string
	Normalized string
}
```

For GitLab, `Namespace` may include nested groups:

```text
gitlab:gitlab.example.com/group/subgroup/project
```

Parsing rule:

- For `gitlab:` with 2 path parts, use `gitlab.com` and treat the first part as namespace and second as project.
- For `gitlab:` with 3 or more path parts, treat the first part as host, the last part as project, and everything in between as namespace.
- For `gitea:` / `forgejo:`, require at least 3 path parts: host, owner/namespace, repo. The last part is project; middle parts are namespace. This keeps room for nested namespaces on Gitea-compatible servers that support them.

## Finder Interface

The forge finder should implement the existing install `Finder` interface:

```go
type Finder struct {
	Target Target
	Tag    string
	Getter HTTPGetter
}

func (f Finder) Find() ([]string, error)
```

Behavior:

- If `Tag` is empty, discover the latest release.
- If `Tag` is set, fetch that release tag.
- Return real asset download URLs.
- Do not apply `system`, `asset_filters`, `file`, or archive extraction logic in the forge package.
- Do not fall back to older installed assets if the selected release has no matching current asset.

## GitLab API

GitLab project paths must be URL-escaped as a single path parameter for API calls.

For `gitlab:gitlab.com/fdroid/fdroidserver`:

```text
project path: fdroid/fdroidserver
encoded: fdroid%2Ffdroidserver
```

Latest release:

```text
GET https://<host>/api/v4/projects/<encoded-project-path>/releases/permalink/latest
```

Specific release:

```text
GET https://<host>/api/v4/projects/<encoded-project-path>/releases/<url-escaped-tag>
```

Relevant response fields:

```json
{
  "tag_name": "v1.2.3",
  "assets": {
    "links": [
      {
        "name": "tool-linux-amd64.tar.gz",
        "direct_asset_url": "https://gitlab.com/.../downloads/tool-linux-amd64.tar.gz",
        "url": "https://gitlab.com/.../-/releases/v1.2.3/downloads/tool-linux-amd64.tar.gz"
      }
    ]
  }
}
```

URL choice:

- Prefer `direct_asset_url` when present.
- Fall back to `url` when `direct_asset_url` is empty.
- Ignore links with no usable URL.

Error handling:

- Non-200 responses should return an error with status and request URL.
- `404` for a specific tag should not search old releases in the first version.
- Empty release assets should return a source-specific error before detector selection when possible.

## Gitea and Forgejo API

Gitea-compatible release API:

Latest release:

```text
GET https://<host>/api/v1/repos/<namespace>/<project>/releases/latest
```

Specific release:

```text
GET https://<host>/api/v1/repos/<namespace>/<project>/releases/tags/<url-escaped-tag>
```

Relevant response fields:

```json
{
  "tag_name": "v1.2.3",
  "assets": [
    {
      "name": "tool-linux-amd64.tar.gz",
      "browser_download_url": "https://codeberg.org/.../releases/download/v1.2.3/tool-linux-amd64.tar.gz"
    }
  ]
}
```

URL choice:

- Use `browser_download_url`.
- Ignore assets with no download URL.

Forgejo should use the same API shape unless a smoke test proves an instance requires a small compatibility branch. That branch should stay inside `internal/source/forge`, not in the install runner.

## Latest Version Checks

Current app services use:

```go
LatestTag func(repo, sourcePath string) (string, error)
```

This can continue for the first version, but the implementation should route by target parser:

```text
sourceforge.ParseTarget(repo) -> SourceForge latest
forge.ParseTarget(repo)       -> GitLab/Gitea latest
else                          -> GitHub latest
```

The forge package should expose:

```go
type LatestInfo struct {
	Tag string
}

func LatestVersion(target Target, getter HTTPGetter) (LatestInfo, error)
```

Rules:

- GitLab uses `releases/permalink/latest`.
- Gitea/Forgejo uses `releases/latest`.
- Returned tag is compared directly with installed store tag/version, matching existing update behavior.
- If no latest release can be determined, report a `check_failed` row through existing list/update failure handling.

A later cleanup can replace `LatestTag(repo, sourcePath)` with a more general source-aware interface. That is not required for the first version.

## Install Service Integration

Add a new target kind:

```go
TargetForge TargetKind = "forge"
```

or explicit kinds:

```go
TargetGitLab TargetKind = "gitlab"
TargetGitea  TargetKind = "gitea"
TargetForgejo TargetKind = "forgejo"
```

Recommendation: use `TargetForge` internally and let `forge.Target.Provider` distinguish provider behavior.

`DetectTargetKind` should check forge targets before generic URL and GitHub repo checks:

```text
local file
sourceforge target
forge target
GitHub URL
direct URL
owner/repo
unknown
```

`install.Service` should avoid adding one factory per provider. Add one forge factory:

```go
ForgeGetterFactory func(opts Options) forge.HTTPGetter
```

`SelectFinder` should parse forge target, create `forge.Finder`, and return project name as the tool hint.

Tool hint:

- Use the target project/repo basename.
- For `gitlab:gitlab.com/group/tool`, tool is `tool`.

## App Layer Integration

Update paths that currently special-case SourceForge:

- `internal/app/config.go`: normalize `repo` and default package name for forge targets.
- `internal/installed/store.go`: normalize forge targets for installed store keys.
- `internal/app/update.go`: allow direct `update gitlab:...`, `update gitea:...`, and `update forgejo:...`.
- `internal/app/install.go`: record forge tag/version from release URL only if API release info is unavailable.

Preferred installed store metadata:

```toml
[installed."gitlab:gitlab.com/fdroid/fdroidserver"]
repo = "gitlab:gitlab.com/fdroid/fdroidserver"
target = "gitlab:gitlab.com/fdroid/fdroidserver"
url = "https://gitlab.com/.../tool-linux-amd64.tar.gz"
asset = "tool-linux-amd64.tar.gz"
tag = "v1.2.3"
version = "v1.2.3"
```

For Gitea/Forgejo:

```toml
[installed."gitea:codeberg.org/forgejo/forgejo"]
repo = "gitea:codeberg.org/forgejo/forgejo"
tag = "v1.2.3"
version = "v1.2.3"
```

## CLI Wiring

`newCLIService` should configure the forge getter through the same network stack:

```go
installService.ForgeGetterFactory = func(opts install.Options) forge.HTTPGetter {
	return client.NewGitHubClient(install.ClientOptions(opts))
}
```

The existing client name is GitHub-specific, but its `Get(url)` behavior is already generic enough for public HTTP APIs. A future cleanup can rename this to a neutral HTTP client.

Verbose wiring:

```go
forge.SetVerbose(verbose, stderr)
```

Verbose logs should include:

```text
[verbose] forge gitlab request: ...
[verbose] forge gitlab response: ...
[verbose] forge gitlab assets: 3
```

## Query and Search

First version does not add:

- `query gitlab:...`
- `query gitea:...`
- `search gitlab ...`
- `search gitea ...`

Existing `query` and `search` remain GitHub-only. Documentation should say that GitLab/Gitea support is for install/download/update release assets only.

## Authentication

First version does not implement authentication.

Design constraints for future token support:

- GitLab public API can work without token but has rate limits.
- GitLab private repos will need `PRIVATE-TOKEN` or `Authorization: Bearer`.
- Gitea/Forgejo private repos typically use `Authorization: token <token>` or bearer token depending on server version.
- Token config should be host-aware, not only provider-wide, because self-hosted instances differ.

Potential future config:

```toml
[forge_hosts."gitlab.example.com"]
provider = "gitlab"
token = "..."

[forge_hosts."codeberg.org"]
provider = "gitea"
token = "..."
```

This is intentionally not part of the first implementation.

## Error Handling

Errors should be source-specific and explicit:

- `invalid forge target "gitlab:"`
- `gitlab project is required`
- `gitea host is required`
- `unsupported forge provider "bitbucket"`
- `gitlab release request failed: 404 Not Found (URL: ...)`
- `gitea release assets not found for codeberg.org/forgejo/forgejo`
- Existing detector errors such as `asset "linux" not found` should be preserved when assets exist but no filter matches.

Do not silently fall back to an old installed asset when the current release has no matching asset.

## Testing Strategy

Unit tests:

- Target parsing:
  - `gitlab:fdroid/fdroidserver`
  - `gitlab:gitlab.gnome.org/GNOME/gtk`
  - `gitlab:gitlab.example.com/group/subgroup/project`
  - `gitea:codeberg.org/forgejo/forgejo`
  - `forgejo:codeberg.org/forgejo/forgejo`
  - invalid empty host/project cases
- Target kind detection.
- Normalized installed store keys.
- Add package normalization and default package names.
- GitLab release JSON parsing and URL selection.
- Gitea release JSON parsing and URL selection.
- Latest version checks for GitLab and Gitea.
- `list --outdated` and `update --all` with mixed GitHub, SourceForge, GitLab, and Gitea packages.
- Direct `update gitlab:...` and `update gitea:...` target routing.

Integration-style tests with fake HTTP getters:

- GitLab latest release returns candidate assets.
- GitLab specific tag returns candidate assets.
- Gitea latest release returns candidate assets.
- Empty release assets return source-specific errors.
- Non-200 API responses include status and URL.

Manual smoke tests:

```bash
go run ./cmd/eget download --asset linux,amd64 --to .tmp-forge-smoke gitea:codeberg.org/forgejo/forgejo
go run ./cmd/eget download --asset linux,amd64 --to .tmp-forge-smoke gitlab:gitlab.com/<known-public-project>
```

The implementation plan should identify stable public test projects before relying on smoke tests.

## Documentation Updates

Update:

- `README.md`
- `README.zh-CN.md`
- `docs/DOCS.md`
- `docs/example.eget.toml`

Docs should show:

```bash
eget install gitlab:fdroid/fdroidserver
eget install gitlab:gitlab.gnome.org/GNOME/gtk
eget install gitea:codeberg.org/forgejo/forgejo
```

And:

```toml
[packages.forgejo]
repo = "gitea:codeberg.org/forgejo/forgejo"
system = "linux/amd64"
asset_filters = ["linux", "amd64"]
```

## Non-Goals

This first version does not support:

- Private repositories.
- Provider search.
- Query command parity.
- Automatic provider detection from arbitrary repository web URLs.
- Bitbucket, Gogs, or custom non-Gitea-compatible APIs.
- Package registry artifacts.
- GitLab job artifacts.
- Release notes display.
- Project-specific install recipes.

## Open Implementation Decisions

Before implementation, validate public API behavior for:

- GitLab latest release endpoint availability on `gitlab.com`.
- GitLab self-hosted project path encoding with nested namespaces.
- Codeberg/Forgejo release API response shape and download URLs.
- A stable public GitLab project with release assets suitable for smoke tests.

If a public instance behaves differently but is still Gitea-compatible, keep the compatibility branch isolated in `internal/source/forge`.
