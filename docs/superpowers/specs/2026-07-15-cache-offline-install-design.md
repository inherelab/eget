# Cache Server 离线安装增强设计

## 修订记录

| 日期 | 变更 |
| --- | --- |
| 2026-07-15 | 明确两期路线：一期支持已知工具从 cache server 全离线安装；二期规划正式的服务端可安装工具目录。 |

## 相关文档

- [缓存管理命令设计](2026-05-26-cache-management-design.md)
- [Path-key cache mirror 实施计划](../plans/2026-06-01-cache-mirror-path-key.md)

## 背景与问题

当前 `cache serve` 已经可以分享以下缓存：

```text
pkg-cache
api-cache
sdk-downloads
sdk-index
```

服务端也已经提供 manifest、文件下载和 path-key 下载协议。客户端普通资产下载与 SDK 归档下载会优先尝试 cache mirror。

但 provider 元数据请求仍采用以下流程：

```text
客户端本地 api-cache
  -> miss/expired
  -> GitHub/GitLab/Gitea/SourceForge API
  -> 得到 release 和资产 URL
  -> cache mirror 下载资产
```

因此客户端即使能够访问包含完整 `api-cache` 和 `pkg-cache` 的 cache server，仍必须先访问外网 provider。`cache_mirror.fallback=false` 目前只能阻止资产回源，不能阻止元数据请求外网，无法形成真正的离线安装链路。

## 总体路线

本增强分为两期，避免把简单缓存镜像一次性升级成完整 registry。

### 一期：已知工具全离线安装

客户端已经知道安装目标，例如：

```text
eget install PowerShell/PowerShell
eget install rg
eget install pkg-template:kdev:cscli
```

cache server 已由联网机器预热了该目标所需的 provider API 响应和资产文件。客户端通过本地配置或明确 target 发起安装，不要求从 server 搜索工具。

### 二期：正式可安装工具目录

cache server 建立 package/version/platform 级目录。客户端可以发现、搜索并解析 server 上可安装的工具，不再依赖自己预先知道 target 或第三方 provider。

二期只在一期稳定后实施，本设计先定义边界和协议方向，不把目录逻辑混入一期。

## 方案比较

### 方案 A：复用现有 api-cache path-key（一期采用）

客户端对 provider API URL 使用现有 `APICacheFilePath` 计算相对路径和 path-key，先从 cache server 下载对应 API cache，再按原有 finder 解析；资产继续使用现有 pkg-cache path-key。

优点：

- 直接复用已有 server 路由、缓存目录和 path-key 协议。
- 不需要服务端理解 GitHub release、package 或平台语义。
- 已经预热的老缓存可以直接使用。
- 对现有 finder 和 detector 改动小。

限制：

- 客户端必须知道 target。
- API URL、缓存命名规则和 eget 版本需要兼容。
- server 不能回答“有哪些工具可安装”。

### 方案 B：cache server 反向代理 provider API（不采用）

客户端把 GitHub API 请求改写到 cache server，由 server 命中缓存或代为访问外网。

不采用原因：

- server 变成 provider 代理，需要处理认证、Header、rate limit 和响应缓存策略。
- 严格离线时仍需要额外定义 server 回源开关。
- 与当前只读文件缓存服务定位冲突。

### 方案 C：直接建设 package catalog（作为二期）

服务端保存 package、version、platform 和资产元数据，客户端直接查询 catalog。

这是长期能力，但一期采用会显著扩大范围，引入索引生成、冲突处理、schema 兼容、搜索和可信度问题，因此推迟到二期。

## 一期详细设计

### 目标

当配置为：

```toml
[cache_mirror]
enable = true
url = "http://192.168.1.10:8686"
fallback = false
```

且 server 已经预热所需缓存时，已知工具的安装全过程不得请求外网。

### 元数据请求流程

对现有 provider metadata 请求执行：

```text
1. 计算本地 APICacheFilePath。
2. 本地 api-cache 有效时直接使用。
3. cache mirror 启用时：
   3.1 计算 api-cache 相对路径。
   3.2 计算 path-key。
   3.3 请求 server /download/{path-key}。
   3.4 命中后写入本地 api-cache，并使用现有 JSON 解析链路。
4. mirror miss/error：
   - fallback=true：记录原因并请求原 provider。
   - fallback=false：立即返回明确的 offline cache miss，不发起外网请求。
5. provider 请求成功后继续写入现有本地 api-cache。
```

mirror 命中的 API response 写入本地时以当前时间作为 mtime，因此继续遵循现有 `api_cache.cache_time`；本地响应过期后可重新从 server 获取，不需要 server 修改文件内容。

### 资产请求流程

保持当前实现：

```text
本地 pkg-cache
  -> cache server pkg-cache
  -> fallback=true 时访问 origin
```

`fallback=false` 时 mirror miss 必须终止，不能访问 GitHub release asset URL。

### 启用条件

- provider metadata 的 mirror 查找由 `cache_mirror.enable + url` 激活，不要求用户另外开启 `api_cache.enable`。
- 本地仍使用 `{cache_dir}/api-cache` 暂存 mirror 元数据。
- `cache_dir`、mirror URL、timeout 和 fallback 继续来自现有配置解析，不新增 `offline` 开关。
- `fallback=false` 即严格离线语义，统一约束元数据和资产两层。

### 服务端变化

一期不新增 HTTP 路由，也不修改 manifest schema。现有 server 已扫描 `KindAPI`，并能通过 `/download/{path-key}` 返回 api-cache 文件。

如果现有路由在 token、root scope 或 path-key 校验中发现 API cache 未覆盖，只做必要修正，不另建 `/api-proxy`。

### 支持范围

一期优先覆盖已经进入 `isProviderMetadataRequest` / `api-cache` 的公开元数据请求：

```text
GitHub
GitLab
Gitea
SourceForge
```

直接 URL 且无需元数据解析的工具，只需要现有资产 mirror。

任意 URL template 的 latest/checksum endpoint 如果尚未进入 api-cache，不在一期隐式扩展范围；需要时后续按同一机制显式纳入 metadata cache。

一期不提供：

- 从 server 搜索工具。
- 自动列出 server 可安装 package。
- 根据模糊名称解析 target。
- server 主动访问 provider 或刷新缓存。
- cache 上传接口。

### 预热要求

cache server 是只读服务。联网机器需要先正常安装或下载目标，使以下内容同时存在：

```text
api-cache/<provider-metadata>.json
pkg-cache/<asset>
```

缺少任一层时，严格离线客户端返回缺失的 kind/path-key，方便服务端补齐，而不是笼统报告网络失败。

### 安全与可信度

- API metadata 和资产都来自受信任的内网 cache server，但仍不是新的信任根。
- 现有 checksum/verify 配置在 mirror 命中后继续执行。
- token 保护沿用 cache server 现有认证能力。
- 日志不得打印 bearer token。
- server 继续保持只读，无远程写入和刷新接口。

### 可观测输出

建议区分元数据和资产命中：

```text
Using cache mirror metadata: api-cache/...
Using cache mirror file: pkg-cache/...
```

严格离线 miss：

```text
cache mirror metadata miss: api-cache/... (fallback disabled)
```

### 一期测试

至少覆盖：

1. 本地 API cache 命中时不请求 mirror 和 origin。
2. 本地 miss、mirror metadata 命中时不请求 origin。
3. metadata 与 asset 都命中时，完整安装链路不触发任何外网请求。
4. metadata miss 且 `fallback=true` 时回源并写入本地 API cache。
5. metadata miss 且 `fallback=false` 时返回错误，origin 调用次数为 0。
6. asset miss 且 `fallback=false` 时保持现有禁止回源行为。
7. mirror token、timeout 和 server root scope 行为不回归。
8. GitHub target 的 release/version/asset 解析结果与在线流程一致。

## 二期：正式可安装工具目录规划

### 目标

cache server 能回答：

```text
有哪些工具可用？
某个工具有哪些版本？
当前 OS/arch 可安装哪个资产？
该资产的下载 key、大小和 checksum 是什么？
```

客户端可在不访问外网的情况下执行发现和安装。

### 独立 catalog 协议

不要继续扩张面向物理文件的 `manifest.json`。二期增加独立、带版本的目录协议，例如：

```text
GET /v1/catalog
GET /v1/packages/{name}
GET /v1/packages/{name}/versions/{version}
```

候选条目：

```json
{
  "name": "PowerShell",
  "target": "PowerShell/PowerShell",
  "provider": "github",
  "version": "7.6.3",
  "os": "windows",
  "arch": "amd64",
  "asset": "PowerShell-7.6.3-win-x64.msi",
  "path_key": "path-md5:...",
  "size": 123456,
  "checksum": "sha256:...",
  "cached_at": "2026-07-15T10:00:00Z"
}
```

### 目录数据来源

二期不能仅靠文件名猜测 package/version/platform。应在联网下载成功时写入轻量 metadata sidecar，记录 finder 已知的结构化信息；server 启动时扫描 sidecar 构建只读 catalog。

手动放入 cache 的未知文件可以继续出现在文件 manifest，但不自动进入可安装 catalog。

### 待解决问题

二期实施前需要单独确认：

- package 名称、repo target 和 alias 的唯一性规则。
- 同版本多资产和多平台选择。
- mutable tag / 同 tag 资产更新。
- catalog schema 兼容与客户端版本协商。
- sidecar 缺失、损坏和重建策略。
- 搜索排序、分页和模糊匹配。
- checksum 缺失时的可信度展示。
- 多 server 或多个缓存来源的冲突策略。

## 分期成功标准

### 一期

- 已知 target 在 server 缓存齐全时可全程离线安装。
- `fallback=false` 保证元数据和资产都不访问外网。
- 不新增 registry/catalog 数据模型。
- 复用现有 api-cache、pkg-cache 和 path-key 协议。

### 二期

- server 提供稳定、版本化的可安装工具目录。
- 客户端可以发现、查询并安装 server 上的工具。
- catalog 来源于结构化 sidecar，而不是文件名猜测。
- 文件 manifest 与 package catalog 职责分离。
