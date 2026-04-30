# GitLab / Gitea Forge 支持设计

## 目标

为 GitLab、Gitea、Forgejo 风格的仓库增加一等公开 release 资源支持，同时保持现有安装主链路不变：

```text
source target -> 候选 asset URL -> system/asset/file 筛选 -> 下载 -> 校验 -> 提取 -> installed store
```

第一版支持公有站点和自托管实例上的公开 release assets。私有仓库认证暂不实现，但 target 和配置模型需要为后续 token 支持保留空间，避免未来推翻语法。

## 支持的目标格式

采用 provider 前缀和 host-aware target：

```text
gitlab:<namespace>/<project>
gitlab:<host>/<namespace>/<project>
gitea:<host>/<owner>/<repo>
forgejo:<host>/<owner>/<repo>
```

示例：

```bash
eget install gitlab:fdroid/fdroidserver
eget install gitlab:gitlab.gnome.org/GNOME/gtk
eget install gitea:codeberg.org/forgejo/forgejo
eget install forgejo:codeberg.org/forgejo/forgejo
```

规则：

- `gitlab:<namespace>/<project>` 默认 host 为 `gitlab.com`。
- `gitlab:<host>/<namespace>/<project>` 指向自托管 GitLab 兼容实例。
- `gitea:` 和 `forgejo:` 第一版必须显式提供 host。
- `forgejo:` 按 Gitea-compatible API 处理，但保留独立 normalized prefix。
- 现有 `owner/repo` 继续只表示 GitHub，不改变已有行为。
- 直接 `https://...` URL 继续表示直接下载 URL。第一版不自动把任意仓库网页 URL 识别为 GitLab/Gitea 仓库。

## 托管配置

托管包继续使用 `repo` 表示来源身份：

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

第一版不需要新增必填配置字段。

后续可以在不改变 target 语法的前提下补 token 配置：

```toml
[global]
gitlab_token = "..."
gitea_token = "..."

["gitlab:gitlab.example.com/group/project"]
token = "..."
```

这些 token 字段不属于第一版实现范围。

## 规范化

配置和 installed store 都应使用带 host 的完整 normalized key：

```text
gitlab:gitlab.com/fdroid/fdroidserver
gitlab:gitlab.gnome.org/GNOME/gtk
gitea:codeberg.org/forgejo/forgejo
forgejo:codeberg.org/forgejo/forgejo
```

含义：

- 用户可以输入 `gitlab:fdroid/fdroidserver`。
- 内部记录时规范化为 `gitlab:gitlab.com/fdroid/fdroidserver`。
- `gitea:codeberg.org/forgejo/forgejo` 保持原样。
- namespace/project 的大小写在 URL 中应保留；比较时依赖已存储的 normalized string。

installed store 的 key 使用 normalized target，而不是用户原始输入。

## 架构

新增一个小型 forge 包，用于 target 解析和 provider 分发：

```text
internal/source/forge
```

建议文件结构：

```text
internal/source/forge/target.go
internal/source/forge/finder.go
internal/source/forge/gitlab.go
internal/source/forge/gitea.go
internal/source/forge/log.go
```

核心模型：

```go
type Provider string

const (
	ProviderGitLab  Provider = "gitlab"
	ProviderGitea   Provider = "gitea"
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

GitLab 的 `Namespace` 可以包含多级 group：

```text
gitlab:gitlab.example.com/group/subgroup/project
```

解析规则：

- `gitlab:` 后只有 2 段 path 时，使用 `gitlab.com`，第一段为 namespace，第二段为 project。
- `gitlab:` 后有 3 段或更多 path 时，第一段为 host，最后一段为 project，中间所有段拼成 namespace。
- `gitea:` / `forgejo:` 至少需要 3 段 path：host、owner/namespace、repo。第一段为 host，最后一段为 project，中间所有段拼成 namespace。

## Finder 接口

forge finder 实现现有 install `Finder` 接口：

```go
type Finder struct {
	Target Target
	Tag    string
	Getter HTTPGetter
}

func (f Finder) Find() ([]string, error)
```

行为：

- `Tag` 为空时发现 latest release。
- `Tag` 非空时获取指定 tag 的 release。
- 返回真实 asset download URL。
- 不在 forge 包里处理 `system`、`asset_filters`、`file` 或归档提取。
- 当前 release 没有匹配 asset 时，不回退旧 installed asset。

## GitLab API

GitLab project path 需要作为一个 API path 参数整体 URL escape。

对于 `gitlab:gitlab.com/fdroid/fdroidserver`：

```text
project path: fdroid/fdroidserver
encoded: fdroid%2Ffdroidserver
```

latest release：

```text
GET https://<host>/api/v4/projects/<encoded-project-path>/releases/permalink/latest
```

指定 release：

```text
GET https://<host>/api/v4/projects/<encoded-project-path>/releases/<url-escaped-tag>
```

相关响应字段：

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

URL 选择：

- 优先使用 `direct_asset_url`。
- `direct_asset_url` 为空时回退到 `url`。
- 忽略没有可用 URL 的 link。

错误处理：

- 非 200 响应返回包含状态码和请求 URL 的错误。
- 指定 tag 返回 404 时，第一版不扫描旧 release 列表做模糊匹配。
- release assets 为空时，在进入 detector 前尽量返回 source-specific error。

## Gitea / Forgejo API

Gitea-compatible release API：

latest release：

```text
GET https://<host>/api/v1/repos/<namespace>/<project>/releases/latest
```

指定 release：

```text
GET https://<host>/api/v1/repos/<namespace>/<project>/releases/tags/<url-escaped-tag>
```

相关响应字段：

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

URL 选择：

- 使用 `browser_download_url`。
- 忽略没有下载 URL 的 asset。

Forgejo 默认复用同一套 API。若 smoke test 证明某个实例存在小差异，兼容逻辑应限制在 `internal/source/forge` 内，不进入 install runner。

## 最新版本检查

当前 app services 使用：

```go
LatestTag func(repo, sourcePath string) (string, error)
```

第一版可以继续沿用这个签名，但实现要按 target parser 路由：

```text
sourceforge.ParseTarget(repo) -> SourceForge latest
forge.ParseTarget(repo)       -> GitLab/Gitea latest
else                          -> GitHub latest
```

forge 包暴露：

```go
type LatestInfo struct {
	Tag string
}

func LatestVersion(target Target, getter HTTPGetter) (LatestInfo, error)
```

规则：

- GitLab 使用 `releases/permalink/latest`。
- Gitea/Forgejo 使用 `releases/latest`。
- 返回 tag 直接与 installed store 中的 tag/version 比较，保持现有 update 行为。
- 无法判断 latest release 时，通过现有 list/update failure 流程输出 `check_failed`。

后续可以把 `LatestTag(repo, sourcePath)` 重构成更通用的 source-aware interface，但这不是第一版必须项。

## Install Service 接入

新增 target kind：

```go
TargetForge TargetKind = "forge"
```

也可以拆成显式 kind：

```go
TargetGitLab  TargetKind = "gitlab"
TargetGitea   TargetKind = "gitea"
TargetForgejo TargetKind = "forgejo"
```

推荐内部使用 `TargetForge`，具体 provider 由 `forge.Target.Provider` 区分。

`DetectTargetKind` 的检测顺序应为：

```text
local file
sourceforge target
forge target
GitHub URL
direct URL
owner/repo
unknown
```

`install.Service` 不应该为每个 provider 继续增加单独 factory。新增一个 forge factory 即可：

```go
ForgeGetterFactory func(opts Options) forge.HTTPGetter
```

`SelectFinder` 解析 forge target 后创建 `forge.Finder`，并返回项目名作为 tool hint。

tool hint 规则：

- 使用 target 的 project/repo basename。
- `gitlab:gitlab.com/group/tool` 的 tool 是 `tool`。

## App 层接入

需要更新当前 special-case SourceForge 的路径：

- `internal/app/config.go`：规范化 forge `repo`，并为未指定 name 的包使用 project 作为默认包名。
- `internal/installed/store.go`：规范化 forge target 作为 installed store key。
- `internal/app/update.go`：允许直接 `update gitlab:...`、`update gitea:...`、`update forgejo:...`。
- `internal/app/install.go`：优先通过 API release info 记录 tag/version；API 信息不可用时，再尝试从 release URL 提取 tag。

推荐 installed store 结构：

```toml
[installed."gitlab:gitlab.com/fdroid/fdroidserver"]
repo = "gitlab:gitlab.com/fdroid/fdroidserver"
target = "gitlab:gitlab.com/fdroid/fdroidserver"
url = "https://gitlab.com/.../tool-linux-amd64.tar.gz"
asset = "tool-linux-amd64.tar.gz"
tag = "v1.2.3"
version = "v1.2.3"
```

Gitea/Forgejo：

```toml
[installed."gitea:codeberg.org/forgejo/forgejo"]
repo = "gitea:codeberg.org/forgejo/forgejo"
tag = "v1.2.3"
version = "v1.2.3"
```

## CLI Wiring

`newCLIService` 通过同一套网络配置创建 forge getter：

```go
installService.ForgeGetterFactory = func(opts install.Options) forge.HTTPGetter {
	return client.NewGitHubClient(install.ClientOptions(opts))
}
```

当前 client 名称带 GitHub，但它的 `Get(url)` 行为已经足够通用，可先复用。后续可单独把它重命名为中性的 HTTP client。

verbose wiring：

```go
forge.SetVerbose(verbose, stderr)
```

verbose 日志示例：

```text
[verbose] forge gitlab request: ...
[verbose] forge gitlab response: ...
[verbose] forge gitlab assets: 3
```

## Query 和 Search

第一版不增加：

- `query gitlab:...`
- `query gitea:...`
- `search gitlab ...`
- `search gitea ...`

现有 `query` 和 `search` 仍然是 GitHub-only。文档需要说明 GitLab/Gitea 支持范围是 install/download/update release assets。

## 认证

第一版不实现认证。

后续 token 支持需要考虑：

- GitLab 公共 API 无 token 可用，但有 rate limit。
- GitLab 私有仓库需要 `PRIVATE-TOKEN` 或 `Authorization: Bearer`。
- Gitea/Forgejo 私有仓库通常使用 `Authorization: token <token>` 或 bearer token，取决于服务端版本。
- token 配置应按 host 维度管理，而不是只按 provider 管理，因为自托管实例各不相同。

后续可能的配置：

```toml
[forge_hosts."gitlab.example.com"]
provider = "gitlab"
token = "..."

[forge_hosts."codeberg.org"]
provider = "gitea"
token = "..."
```

这不属于第一版实现。

## 错误处理

错误应明确带 source 信息：

- `invalid forge target "gitlab:"`
- `gitlab project is required`
- `gitea host is required`
- `unsupported forge provider "bitbucket"`
- `gitlab release request failed: 404 Not Found (URL: ...)`
- `gitea release assets not found for codeberg.org/forgejo/forgejo`
- release 存在但 asset filter 不匹配时，继续复用现有 detector 错误，例如 `asset "linux" not found`

当前 release 没有匹配 asset 时，不允许静默回退到旧 installed asset。

## 测试策略

单元测试：

- target 解析：
  - `gitlab:fdroid/fdroidserver`
  - `gitlab:gitlab.gnome.org/GNOME/gtk`
  - `gitlab:gitlab.example.com/group/subgroup/project`
  - `gitea:codeberg.org/forgejo/forgejo`
  - `forgejo:codeberg.org/forgejo/forgejo`
  - 空 host / 空 project 等非法输入
- target kind detection。
- installed store normalized key。
- add package normalization 和默认 package name。
- GitLab release JSON 解析和 URL 选择。
- Gitea release JSON 解析和 URL 选择。
- GitLab/Gitea latest version 检查。
- 混合 GitHub、SourceForge、GitLab、Gitea 包的 `list --outdated` 和 `update --all`。
- 直接 `update gitlab:...` / `update gitea:...` 的路由。

使用 fake HTTP getter 的 integration-style 测试：

- GitLab latest release 返回候选 assets。
- GitLab 指定 tag 返回候选 assets。
- Gitea latest release 返回候选 assets。
- 空 release assets 返回 source-specific error。
- 非 200 API 响应包含状态码和 URL。

手动 smoke 测试：

```bash
go run ./cmd/eget download --asset linux,amd64 --to .tmp-forge-smoke gitea:codeberg.org/forgejo/forgejo
go run ./cmd/eget download --asset linux,amd64 --to .tmp-forge-smoke gitlab:gitlab.com/<known-public-project>
```

实现计划需要先确定稳定的公开测试项目，再依赖 smoke 测试。

## 文档更新

需要更新：

- `README.md`
- `README.zh-CN.md`
- `docs/DOCS.md`
- `docs/example.eget.toml`

文档示例：

```bash
eget install gitlab:fdroid/fdroidserver
eget install gitlab:gitlab.gnome.org/GNOME/gtk
eget install gitea:codeberg.org/forgejo/forgejo
```

配置示例：

```toml
[packages.forgejo]
repo = "gitea:codeberg.org/forgejo/forgejo"
system = "linux/amd64"
asset_filters = ["linux", "amd64"]
```

## 非目标

第一版不支持：

- 私有仓库。
- provider search。
- query 命令对 GitLab/Gitea 的等价支持。
- 从任意 repository web URL 自动检测 provider。
- Bitbucket、Gogs 或非 Gitea-compatible 的自定义 API。
- package registry artifacts。
- GitLab job artifacts。
- release notes 展示。
- 项目专用安装配方。

## 实现前需要验证的事项

实现前需要确认这些公开 API 行为：

- `gitlab.com` 的 latest release endpoint 是否稳定可用。
- GitLab 自托管多级 namespace 的 project path encoding。
- Codeberg/Forgejo release API 响应结构和 download URL 字段。
- 可用于 smoke test 的稳定公开 GitLab release assets 项目。

如果某个公开实例存在兼容差异，但仍属于 Gitea-compatible 范畴，兼容分支应限制在 `internal/source/forge` 内。
