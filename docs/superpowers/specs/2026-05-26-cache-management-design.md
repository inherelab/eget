# eget 缓存管理命令 cache 设计

## 修订记录

| 日期 | 变更 |
| --- | --- |
| 2026-09-23 | `cache serve` 命令已删除：其缓存镜像协议（`/manifest.json`、`/download/path-md5:<key>`、`/files/<path>`）与只读浏览能力全部并入 `eget web`，见 [2026-09-23-web-console-design.md](./2026-09-23-web-console-design.md)。本文中 `cache serve` 默认监听 `0.0.0.0:8686`、以及"不新增前端依赖，不引入静态资源目录"的约定均已被该设计取代（`eget web` 默认 `127.0.0.1:8787`，前端采用 Vite/React 构建产物嵌入）。协议本身保持不变。 |
| 2026-06-01 | 后续 cache mirror 协议调整为 path-key 优先：客户端和服务端基于缓存相对路径 `md5(relpath)` 复用现有老缓存；普通 package cache 暂不要求新增 `.meta.json`，source metadata 和 registry 化能力留给后续阶段单独设计。 |
| 2026-05-26 | 初始设计：定义 `cache clean`、`cache serve`、manifest schema、客户端自动 mirror 的分期方向。 |

## 相关文档

- Phase 1+2 实现计划：[2026-05-26-cache-management-phase1+2.md](../plans/2026-05-26-cache-management-phase1+2.md)
- Path-key cache mirror 实施计划：[2026-06-01-cache-mirror-path-key.md](../plans/2026-06-01-cache-mirror-path-key.md)

## 背景

`eget` 已经支持普通 package 下载/安装、SDK 多版本安装、API cache、断点续传和并发下载。缓存能力目前分散在几个主链路中：

- 普通 package/download 缓存写入 `global.cache_dir` 根目录。
- provider 元数据 API cache 写入 `{cache_dir}/api-cache/`。
- SDK 下载归档写入 `{cache_dir}/sdk-downloads/`。
- SDK index JSON 写入 `{cache_dir}/sdk-index/`。
- 断点续传状态使用目标文件旁边的 `.part` 和 `.meta.json`。

`docs/TODO.md` 中提出新增 `eget` 缓存管理命令 `cache`：

- `cache clean` 清理缓存，默认清理 3 天前的缓存。
- `cache serve` 启动内网 server，分享 package/sdk cache 文件到内网环境，方便多台机器共享下载资源。

本设计把 `cache` 定位为 `eget` 本机运行环境管理命令。它不替代 `install/update/sdk/config`，而是管理这些命令共同依赖的缓存、缓存服务和后续可能扩展的镜像复用机制。

## 目标

完整设计目标：

1. 提供统一的缓存清理入口，覆盖普通下载缓存、API cache、SDK 下载缓存、SDK index 和未完成下载状态。
2. 提供局域网只读缓存服务，让多台机器可以复用同一台机器已经下载过的文件。
3. 为后续客户端自动使用局域网缓存镜像预留稳定协议，不让第一期 `cache serve` 变成只能人工浏览的临时工具。
4. 保持默认行为保守：不会误删安装目录，不会默认暴露写接口，不会默认破坏 SDK latest/search 体验。
5. 支持分期实现：先完成最需要的清理和只读服务，再逐步接入客户端 cache mirror、manifest 索引和可观测能力。

## 非目标

首版和完整设计都不把 `cache` 做成通用包管理器。

不纳入 `cache`：

- package 安装、升级、卸载；这些继续由 `install/update/uninstall` 负责。
- SDK 安装、删除、index 刷新；这些继续由 `sdk` 负责。
- 配置编辑；继续由 `config` 负责。
- 公网 registry、账号体系、上传服务、远程执行。
- 后台 daemon 生命周期管理。`cache serve` 是前台进程，退出进程即停止服务。

后续可以增加 `cache list`、`cache status` 等命令，但不作为当前设计的实现重点。诊断类能力更适合后续独立设计为 `eget doctor`，不放入 `cache` 命令族。

## 总体方案

新增顶层命令：

```bash
eget cache clean
eget cache serve
```

完整命令族预留为：

```text
cache
├── clean      清理本机缓存
├── serve      启动只读缓存服务
├── list       查看缓存文件，后续预留
├── mirror     管理远端缓存镜像配置，后续预留
└── status     查看缓存状态和占用，后续预留
```

当前设计只要求实现 `clean` 和 `serve`。预留命令不注册到 CLI，避免暴露不可用入口。

## 缓存模型

### 缓存根目录

`cache` 统一从配置解析缓存根目录：

```text
global.cache_dir
```

如果配置不存在或 `cache_dir` 为空，使用默认值：

```text
~/.cache/eget
```

路径展开复用现有 `util.Expand()`，保证 `~`、环境变量等行为与 install/sdk 一致。

### 缓存类型

`cache` 内部使用明确的缓存类型，而不是只按目录删除：

| 类型 | 默认路径 | 说明 |
| --- | --- | --- |
| `pkg` | `{cache_dir}` 根目录中排除已知子目录后的文件 | 普通 package/download 下载缓存 |
| `api` | `{cache_dir}/api-cache/` | GitHub/GitLab/Gitea/SourceForge 等 provider 元数据响应 |
| `sdk` | `{cache_dir}/sdk-downloads/` | SDK 归档下载缓存 |
| `sdk-index` | `{cache_dir}/sdk-index/` | SDK index JSON 缓存 |
| `partial` | 各缓存目录中的 `.part` 和下载状态文件 | 未完成下载状态 |

普通 package 下载缓存目前直接写在 `cache_dir` 根下，因此清理 `pkg` 时必须排除：

```text
api-cache/
sdk-downloads/
sdk-index/
```

避免 `--pkg` 间接删除 SDK/API 缓存。

### 文件元信息

`cache clean --dry-run`、`cache serve /manifest.json` 和后续 `cache list` 都需要同一套扫描结果。建议定义统一内部结构：

```go
type CacheEntry struct {
    Kind      string
    Path      string
    RelPath   string
    Size      int64
    ModTime   time.Time
    IsPartial bool
}
```

`RelPath` 始终是相对 `cache_dir` 的 slash path，用于 manifest、HTTP URL 和展示。服务端禁止使用绝对路径作为外部协议字段。

## cache clean 设计

### 命令语义

基础用法：

```bash
eget cache clean
eget cache clean --dry-run
eget cache clean --older 7d
eget cache clean --all
```

按类型选择：

```bash
eget cache clean --pkg
eget cache clean --api
eget cache clean --sdk
eget cache clean --sdk-index
eget cache clean --partial
```

组合使用：

```bash
eget cache clean --pkg --sdk --older 30d
eget cache clean --api --all
eget cache clean --partial --all
```

建议参数：

| 参数 | 默认 | 说明 |
| --- | --- | --- |
| `--older <duration>` | `3d` | 删除早于指定时间的缓存文件 |
| `--all` | false | 忽略时间条件，删除选中类型的全部缓存 |
| `--dry-run` | false | 只打印将删除的内容，不实际删除 |
| `--yes, -y` | false | 跳过大批量删除确认 |
| `--pkg` | false | 选择普通 package/download 下载缓存 |
| `--api` | false | 选择 API cache |
| `--sdk` | false | 选择 SDK 下载缓存 |
| `--sdk-index` | false | 选择 SDK index |
| `--partial` | false | 选择未完成下载状态 |

如果用户没有指定任何类型，默认选择：

```text
pkg + api + sdk + partial
```

默认不选择 `sdk-index`。原因是 SDK index 通常体积小，但影响 `sdk install go@latest`、`sdk install go:1.22` 和 `sdk search` 的体验。清理 SDK index 应显式执行：

```bash
eget cache clean --sdk-index
```

### duration 格式

`--older` 支持易读格式：

```text
3d
12h
30m
1w
```

映射规则：

- `m` = minute
- `h` = hour
- `d` = 24 hours
- `w` = 7 days

也可以接受 Go `time.ParseDuration` 支持的格式，例如 `72h`。不支持月份和年份，避免不明确的天数换算。

### 删除规则

`cache clean` 只删除文件，不直接删除整个缓存根目录。删除文件后可以递归清理空目录，但不能删除 `cache_dir` 本身。

候选文件必须同时满足：

1. 位于解析后的 `cache_dir` 内。
2. 属于选中的缓存类型。
3. 如果没有 `--all`，文件 `mtime` 早于 `now - older`。
4. 不是目录、符号链接指向目录或其它特殊文件。

符号链接处理：

- 默认不跟随 symlink。
- 如果 symlink 本身位于 cache dir 内，可按普通文件候选处理并删除 symlink 本身。
- 不根据 symlink 目标递归删除任何内容。

安全边界：

- 如果 `cache_dir` 解析为空、根目录、用户 home 根目录或磁盘根目录，拒绝执行删除。
- 所有待删除路径需要经过 `filepath.Abs` 和 `filepath.Rel` 校验，确保仍在 `cache_dir` 内。
- `--dry-run` 使用同一套扫描和安全校验，只是不执行删除。

### 输出

普通执行输出：

```text
Cleaned eget cache
 - cache dir: ~/.cache/eget
 - removed files: 24
 - freed size: 318.4 MB
 - skipped files: 0
```

`--dry-run` 输出：

```text
Dry run: eget cache clean
 - cache dir: ~/.cache/eget
 - matched files: 24
 - matched size: 318.4 MB
```

如果有跳过项，打印原因摘要：

```text
Skipped:
 - 2 files are outside cache dir after path resolution
 - 1 file cannot be removed: permission denied
```

后续可以增加 `--json`，但首期不必实现。

## cache serve 设计

### 目标

`cache serve` 启动一个只读 HTTP 服务，分享本机 `cache_dir` 中的缓存文件。

它需要同时满足两种场景：

1. 人工使用：在浏览器中查看缓存文件，复制下载链接，或在内网机器上手动下载。
2. 自动使用：后续其它 `eget` 客户端可以通过 manifest 找到缓存文件，命中后从局域网下载，失败时回源下载。

### 命令语义

基础用法：

```bash
eget cache serve
```

等价于：

```bash
eget cache serve --host 0.0.0.0 --port 8686 --root all
```

建议参数：

| 参数 | 默认 | 说明 |
| --- | --- | --- |
| `--host <host>` | `0.0.0.0` | 监听地址 |
| `--port <port>` | `8686` | 监听端口，`0` 表示随机空闲端口 |
| `--root <scope>` | `all` | 分享范围：`all/pkg/api/sdk/sdk-index` |
| `--no-index` | false | 禁止目录列表 |
| `--token <token>` | 空 | 可选 bearer token，后续阶段实现 |
| `--manifest-ttl <duration>` | `30s` | manifest 扫描缓存时间，后续阶段实现 |

首期可以只实现：

```text
--host
--port
--root
--no-index
```

`--token` 和 `--manifest-ttl` 先进入文档设计，暂不暴露 CLI，等客户端自动 mirror 需要时再实现。

### HTTP 路由

完整协议路由：

```text
GET /healthz
GET /manifest.json
GET /files/{relpath}
GET /download/{cache-key}
```

首期必须实现：

```text
GET /
GET /healthz
GET /manifest.json
GET /files/{relpath}
```

`/download/{cache-key}` 为后续客户端自动 mirror 预留。当前优先使用 path-key 协议，让客户端和服务端基于现有缓存相对路径计算 key，从而直接复用已经存在的老缓存文件。

### / Web UI

Phase 2.1 增加一个内置只读 Web UI，服务端在 `GET /` 返回简洁的 HTML 页面，方便人工查看和下载缓存文件。

设计约束：

- 页面只展示文件，不提供删除、清理、上传或配置修改能力。
- 页面复用 manifest 扫描结果和 `/files/{relpath}` 下载 URL，不新增第二套文件发现逻辑。
- 页面展示服务名、版本、当前分享范围、文件总数、总大小和生成时间。
- 文件列表展示 kind、相对路径、大小、修改时间和下载链接。
- 前端只使用内嵌 HTML/CSS 和少量原生 JavaScript，支持按 kind 和路径关键字过滤。
- 不引入 React/Vue/Tailwind 等前端构建链路，保持单二进制发布。
- `--no-index` 继续只表示禁止目录列表，不禁用 `/` 页面；如后续需要完全关闭 UI，再单独设计 `--no-ui`。

这个 UI 面向内网临时共享和排查缓存状态，视觉风格应当克制、清晰、信息密度适中，避免做成管理后台。

### /healthz

返回：

```json
{
  "ok": true,
  "name": "eget-cache",
  "version": "..."
}
```

用于快速检测服务是否可用。

### /manifest.json

返回当前可分享文件清单：

```json
{
  "schema": 1,
  "server": {
    "name": "eget-cache",
    "version": "0.0.0",
    "base_url": "http://192.168.1.10:8686"
  },
  "cache": {
    "root": "",
    "generated_at": "2026-05-26T10:00:00Z"
  },
  "files": [
    {
      "kind": "sdk",
      "path": "sdk-downloads/go/1.22.0/go1.22.0.windows-amd64.zip",
      "url": "/files/sdk-downloads/go/1.22.0/go1.22.0.windows-amd64.zip",
      "size": 123456,
      "mod_time": "2026-05-26T09:00:00Z"
    }
  ]
}
```

`cache.root` 不输出本机绝对路径，避免泄露本机目录结构。需要调试时可在本机命令行启动日志中打印 cache dir。

manifest 首期可以不包含 hash。原因是现有普通下载缓存并不统一保存 checksum，SDK 下载缓存只保存 URL/size/etag/last-modified 元数据。后续自动 mirror 阶段可先扩展 path-key 字段：

```json
{
  "path_key": "path-md5:...",
  "key_path": "pkg-cache/tool-1.2.3-a1b2c3d4.zip"
}
```

更后续如果把 mirror server 做成可搜索、可解析 package/sdk/version/platform 的 registry，再增加 source metadata：

```json
{
  "sha256": "...",
  "etag": "...",
  "source_url": "..."
}
```

### /files/{relpath}

根据 manifest 中的 `path` 提供只读下载。

安全规则：

- `relpath` 必须是相对路径。
- 清理 `..`、重复分隔符和 URL 编码后，最终路径必须仍在 `cache_dir` 内。
- 不跟随目录遍历。
- 默认允许目录列表；`--no-index` 时目录请求返回 403。
- 对文件请求支持 `HEAD` 和 HTTP Range，方便大文件断点下载。

### /download/{cache-key}

后续客户端自动 mirror 使用。

现有 `client.CacheFilePathWithMeta(cacheDir, rawURL, meta)` 会基于 URL 和元数据生成缓存文件名。为了让老缓存直接可用，后续第一版自动 mirror 不强制普通 package cache 增加 `.meta.json`，也不要求服务端知道原始 URL。

后续先增加 path-key 下载：

```text
GET /download/path-md5:<relpath-hash>
```

`relpath-hash` 使用规范化后的缓存相对路径计算，例如：

```text
pkg-cache/tool-1.2.3-a1b2c3d4.zip
sdk-downloads/go/1.22.0/go1.22.0.windows-amd64.zip
```

客户端本来就需要计算本地 `cachePath` 或 SDK `finalPath`，因此可以把它转换为相对 `cache_dir` 的 slash path，再计算 `md5(relpath)` 并请求 mirror。服务端启动或请求时扫描 `cache_dir`，建立 `path-md5:<hash> -> cache file path` 映射。这里 MD5 只用于生成短 key，不作为安全校验。

这个设计优先解决“已有老缓存可以立即被局域网复用”的目标。它的限制是客户端和服务端需要使用一致的缓存路径规则；如果因为 eget 版本不同、package 名/version 元数据不同或用户手动移动文件导致 key 不一致，则 mirror miss 后按 fallback 策略回源。

后续如果 mirror server 需要自成 registry，让客户端不请求第三方 provider 就能搜索、解析和下载 package/sdk，再增加 URL/source metadata 协议：

```json
{
  "source_url_hash": "sha256:..."
}
```

首期不实现 `/download/{cache-key}`，但 manifest schema 从一开始保留 `schema` 字段，后续可兼容扩展。

## 客户端自动 mirror 设计

这是完整目标的一部分，但不建议首期实现。

### 配置

新增独立配置块：

```toml
[cache_mirror]
enable = true
url = "http://192.168.1.10:8686"
timeout = 5
fallback = true
```

字段说明：

| 字段 | 默认 | 说明 |
| --- | --- | --- |
| `enable` | false | 是否启用客户端自动 cache mirror |
| `url` | 空 | 局域网 `cache serve` 地址 |
| `timeout` | `5` | mirror 连接、TLS 握手和响应头超时秒数；不限制完整文件 body 下载耗时 |
| `fallback` | `true` | mirror 失败后是否回源下载 |

不把 mirror 配置放在 `[global]` 下，原因是 mirror 有独立的启用状态、地址、超时和 fallback 策略，后续还可能增加 token、manifest TTL、scope 等字段。独立 `[cache_mirror]` 更利于扩展，也能避免 `global` 继续膨胀。

### 下载流程

普通 package/download：

```text
1. 计算本地 cachePath。
2. 如果本地完整 cache 命中，直接使用。
3. 如果启用 `[cache_mirror]` 且配置了 `url`：
   3.1 将 cachePath 转为相对 cache_dir 的 slash path。
   3.2 计算 pathKey = md5(relpath)。
   3.3 请求 mirror /download/path-md5:{pathKey}。
   3.4 命中后下载到本地 cachePath。
   3.5 对已有 checksum 的 package 执行现有 checksum 校验。
4. mirror 未命中或失败，且 `cache_mirror.fallback=true`，则提示原因并回源下载。
5. 回源下载成功后写入本地 cache。
```

SDK download：

```text
1. 计算 SDK finalPath 和 metaPath。
2. 如果本地完整 SDK cache 命中，直接使用。
3. 将 finalPath 转为相对 cache_dir 的 slash path。
4. 计算 pathKey = md5(relpath)，请求 mirror /download/path-md5:{pathKey}。
5. 下载成功后写入 finalPath，后续仍按现有 SDK cache 规则校验/记录。
6. mirror 失败则回源下载。
```

API cache：

API cache 不建议首期自动从 mirror 复用。API response TTL、认证 token、rate limit 和 provider 状态更敏感。后续如需支持，应只复用无 token 的公开 provider metadata，并严格遵守 `api_cache.cache_time`。

### 校验策略

mirror 是性能优化，不是信任根。

校验优先级：

1. 如果 package 配置有 `verify_sha256` 或 checksum manifest，继续使用现有校验。
2. SDK 下载如果后续增加 checksum，也必须校验。
3. 没有 checksum 的缓存文件，只能按 size/meta 做弱校验；这与当前直接从源站下载的安全等级一致。

因此文档和日志中应避免暗示 mirror 文件天然可信。

## 后续 registry 化设计思考

path-key mirror 的目标是让客户端在已经解析出下载 URL 后，优先复用局域网里同路径规则生成的缓存文件。它仍然依赖客户端先走现有 provider 流程，例如 GitHub release、SourceForge、template latest 或 SDK index。这个阶段不要求普通 package cache 增加 `.meta.json`，也不要求 mirror server 理解 package 名、版本、平台和来源。

如果后续希望 mirror server 自成 registry，目标会变成：

- 客户端可以直接向 mirror server 搜索或解析 package/sdk，不必先访问第三方 provider。
- mirror server 能回答“某个 package/sdk 在某个 version/os/arch 下有没有可下载文件”。
- mirror server 能返回下载文件、文件大小、checksum/source metadata、缓存时间和来源说明。
- mirror server 能区分普通 package、SDK、template source、手动导入文件等不同来源。

这时才需要更完整的 metadata/index，而不是只靠 path-key。候选数据包括：

- `kind`：`pkg`、`sdk`、`template`、`manual` 等。
- `name`、`version`、`os`、`arch`、`filename`、`ext`。
- `source_url`、`source_url_hash`、`provider`、`repo` 或 template id。
- `checksum`、`checksum_type`、`etag`、`last_modified`、`size`。
- `cache_path`、`cached_at`、`updated_at`。

registry 化不应该混入第一版自动 mirror。原因是它会改变客户端解析职责，并引入搜索、索引刷新、冲突处理、数据可信度和权限控制等新问题。当前更稳的路线是：

1. 先做 path-key mirror，让老缓存直接可用。
2. 再补 token、manifest TTL、观测和运维能力。
3. 如果确实需要离线/内网 registry，再单独设计 registry index、搜索 API 和导入/刷新策略。

## 安全设计

### cache clean

`cache clean` 的主要风险是误删。设计上用以下规则降低风险：

- 只操作 `cache_dir` 内部文件。
- 拒绝危险 cache dir，例如磁盘根目录、home 根目录、空路径。
- 默认按时间清理，不默认 `--all`。
- 默认不清理 `sdk-index`。
- `--dry-run` 走同一套扫描逻辑。
- 大量删除时可以要求确认，`--yes` 才跳过。

大量删除阈值建议：

```text
files >= 100 或 size >= 1GB
```

### cache serve

`cache serve` 的主要风险是暴露本机文件或被公网访问。

规则：

- 只读服务，不提供上传、删除、写配置接口。
- 只暴露 `cache_dir` 内选定范围。
- 所有路径都必须经过相对路径校验。
- 启动时打印安全提示：不要暴露到公网。
- 如后续实现 `--token`，仅用于简单内网访问控制，不把它设计成公网安全认证。

默认监听 `0.0.0.0` 是为了满足 TODO 的内网共享目标。如果用户只想本机调试，可以显式：

```bash
eget cache serve --host 127.0.0.1
```

## 内部架构

建议新增 app/cache 子包承载缓存功能主体逻辑：

```text
internal/app/cache/model.go
internal/app/cache/service.go
internal/app/cache/server.go
```

职责：

- 定义缓存条目、清理选项、serve 选项和 manifest DTO。
- 解析 cache dir。
- 扫描缓存。
- 根据清理条件生成候选。
- 执行删除。
- 生成 manifest。
- 提供只读 HTTP handler。

建议新增 CLI 文件：

```text
internal/cli/cache_cmd.go
```

职责：

- 注册 `cache` 命令和子命令。
- 解析 CLI 参数。
- 调用 `cliService.handleCacheClean/handleCacheServe`。

`handlers.go` 只保留参数校验、调用 service、格式化输出，不承载核心清理和扫描逻辑。

不建议把 `cache.go`、`cache_server.go` 继续放在 `internal/app` 根目录。当前 `internal/app` 已经承载较多命令服务，自更新等历史功能也有继续拆分的空间；`cache` 从第一期就包含扫描、清理、manifest、HTTP 服务和路径安全逻辑，放入 `internal/app/cache` 子包更利于控制文件数量和后续扩展。

### 数据结构

核心类型：

```go
type Service struct {
    Config *config.File
    Now    func() time.Time
}

type CleanOptions struct {
    Older        time.Duration
    All          bool
    DryRun       bool
    Yes          bool
    Kinds        []Kind
}

type CleanResult struct {
    CacheDir     string
    MatchedFiles int
    RemovedFiles int
    MatchedSize  int64
    RemovedSize  int64
    Skipped      []CleanSkip
}

type ServeOptions struct {
    Host    string
    Port    int
    Root    string
    NoIndex bool
}
```

服务层不依赖 gcli、color、stdout/stderr，方便单测。

## 与现有命令的关系

### 与 sdk index clear

`eget sdk index clear` 是 SDK 专用操作，语义是清理 SDK index。

`eget cache clean --sdk-index` 是全局缓存清理的一部分，面向“释放空间”和“清理本机缓存”。

两者可以共存：

- `sdk index clear` 仍保留，适合 SDK 用户明确刷新 index。
- `cache clean --sdk-index` 适合统一清理缓存时使用。

### 与 download/install

`download/install` 不需要知道 `cache clean`。

后续接入 `[cache_mirror]` 时，需要在下载逻辑中新增 mirror 尝试，但要保持：

- 本地 cache 优先。
- mirror 是回源前的优化。
- mirror 失败不影响正常下载，除非用户显式关闭 fallback。

### 与 config

`cache clean/serve` 使用现有 `global.cache_dir`。

后续客户端自动 mirror 需要新增 `[cache_mirror]` 配置块，并同步更新：

- `docs/config.md`
- `docs/config.zh-CN.md`
- `docs/example.eget.toml`
- README 简要说明

## 分期实现计划

### Phase 1: 缓存清理 MVP

目标：先解决本机缓存占用问题。

实现：

- 新增 `eget cache clean` 命令。
- 支持 `--older`、`--all`、`--dry-run`、`--yes`。
- 支持 `--pkg`、`--api`、`--sdk`、`--sdk-index`、`--partial`。
- 默认清理 `pkg + api + sdk + partial` 中 3 天前文件。
- 输出清理摘要。

验证：

- 单测覆盖默认类型、`--all`、`--dry-run`、`--sdk-index` 显式清理、安全路径校验。
- 运行 `go test ./...`。

### Phase 2: 只读缓存服务 MVP

目标：先让内网其它机器可以浏览和下载缓存文件。

实现：

- 新增 `eget cache serve`。
- 支持 `--host`、`--port`、`--root`、`--no-index`。
- 实现 `/healthz`、`/manifest.json`、`/files/{relpath}`。
- 文件请求支持 `HEAD`，尽量复用 Go 标准库的 Range 支持。
- 启动日志打印监听地址、cache dir 和安全提示。

验证：

- 单测覆盖 manifest、路径逃逸防护、root scope 过滤、文件下载。
- 本地手动启动后用浏览器或 `curl` 验证 `/healthz` 和文件下载。
- 运行 `go test ./...`。

### Phase 2.1: 内置只读 Web UI

目标：让 `cache serve` 打开根路径即可查看缓存文件清单，降低人工浏览和内网分享时的使用成本。

实现：

- 新增 `GET /`，返回内置 HTML 页面。
- 页面复用现有扫描、root scope 和路径安全逻辑。
- 页面展示服务状态、文件统计和文件列表。
- 文件列表提供 kind 过滤、路径搜索和下载链接。
- UI 渲染逻辑放在 `internal/app/cache` 子文件中，避免继续膨胀 `server.go`。
- 不新增前端依赖，不引入静态资源目录。

验证：

- 单测覆盖 `/` 返回 HTML、文件统计、下载链接、root scope 过滤和 HTML 转义。
- 手动启动 `cache serve` 后用浏览器或 HTTP 请求验证首页。
- 运行 `go test ./...`。

### Phase 3: manifest 增强与 cache mirror 协议

目标：为自动 mirror 提供能复用老缓存的 path-key 协议。

实现：

- manifest 可选增加 `path_key` 字段，值为 `path-md5:<md5(relpath)>`。
- `cache serve` 扫描 cache 时建立 `path-md5:<hash> -> cache file path` 映射。
- 新增 `/download/path-md5:<hash>`，命中后返回对应缓存文件。
- path-key 使用相对 `cache_dir` 的 slash path 计算，不暴露本机绝对路径。
- 不要求普通 package cache 增加 `.meta.json`，确保老缓存能直接参与 mirror。
- URL/source metadata 和 registry 化能力留给后续阶段单独设计。

验证：

- 单测覆盖 path-key 计算、manifest schema 向后兼容、cache-key 命中、root scope 过滤和路径逃逸防护。

### Phase 4: 客户端自动使用 cache mirror

目标：其它机器执行 `install/download/sdk install` 时自动优先复用局域网缓存。

实现：

- 新增 `[cache_mirror]` 配置块，包含 `enable`、`url`、`timeout`、`fallback`。
- install/download 在回源前尝试 mirror。
- SDK download 在回源前尝试 mirror。
- mirror 命中后写入本地 cache，后续流程仍走现有校验/解压/安装。
- 失败时默认回源，并在 verbose 模式输出失败原因。

验证：

- 单测覆盖 mirror 命中、mirror 404 回源、mirror 超时回源、禁用 fallback 报错。
- 端到端手动验证两台机器或两个临时 cache dir 之间复用下载。
- 运行 `go test ./...`。

### Phase 5: 可观测和运维增强

目标：提升长期使用体验。

可选实现：

- `eget cache list`：列出缓存文件，支持按 kind 过滤。
- `eget cache status`：展示 cache dir、缓存大小、SDK index 数量、缓存服务建议地址。
- `eget doctor`：检查 config path、cache dir 权限、配置加载、7z 可用性、target 是否在 PATH。
- `cache serve --token`：简单 bearer token，适合内网共享时避免无关机器访问。
- `cache clean --json` 和 `cache serve --json-log`：方便脚本集成。

这些增强不影响 Phase 1-4 的核心设计。

实施计划：

- [2026-06-02 Cache Ops Integration Enhancements](../plans/2026-06-02-cache-ops-integration.md)

## 测试策略

### 单元测试

重点测试 `internal/app/cache` 子包：

- cache dir 解析。
- cache entry 扫描分类。
- `older` 时间过滤。
- `--all` 行为。
- `--dry-run` 不删除。
- `sdk-index` 默认不选。
- 路径逃逸和危险目录拒绝。
- manifest 生成。
- HTTP 文件服务路径防护。

Go 单测断言继续使用项目规范：

```go
github.com/gookit/goutil/x/assert
```

同一方法多个用例使用 `t.Run()`。

### 集成测试

CLI 层测试：

- `eget cache clean --dry-run`
- `eget cache clean --older 7d --pkg`
- `eget cache serve --host 127.0.0.1 --port 0`

HTTP server 可以使用 `httptest.Server` 测试 handler，不需要在单测中真实占用固定端口。

### 回归测试

实现涉及 MVP 主链路或下载逻辑阶段时，必须运行：

```bash
go test ./...
```

Phase 1/2 虽不直接改下载主链路，也建议运行全量测试，因为会新增顶层 CLI 命令和配置加载路径。

## 文档更新

实现时需要同步更新：

- `README.md`：增加 `cache clean`、`cache serve` 简要说明。
- `README.zh-CN.md`：增加中文说明。
- `docs/config.md` / `docs/config.zh-CN.md`：Phase 4 新增 mirror 配置时更新。
- `docs/TODO.md`：对应阶段完成后勾选或拆分子项。

## 推荐首期实施范围

推荐先实现 Phase 1 和 Phase 2：

1. `cache clean` 立刻解决缓存清理需求。
2. `cache serve` 先提供人工可用的内网只读服务。
3. manifest 从第一期就按 schema 输出，保证后续 Phase 3/4 可以向后扩展。

不建议首期直接实现 Phase 4。原因是自动 mirror 会进入普通 package、direct URL、template package、SDK download 等多个下载路径，触碰主链路较多。先把服务端协议和清理能力打稳，再接客户端复用，风险更可控。

## 默认决策

为避免后续实现时反复分叉，本设计先给出默认决策。后续如有明确需求，可以在实现计划前调整。

1. `cache serve` 默认监听 `0.0.0.0:8686`。理由是该命令的核心目标就是内网共享；安全提示和 `--host 127.0.0.1` 覆盖本机调试场景。
2. `cache clean` 大量删除在 TTY 下需要确认，`--yes` 跳过确认；非 TTY 下如果触发大量删除阈值且没有 `--yes`，直接返回错误。这样兼顾交互安全和脚本可预测性。
3. Phase 4 默认不做 API cache mirror。自动 mirror 先覆盖普通下载缓存和 SDK 下载缓存；API cache 涉及 token、TTL 和 provider 状态，除非后续确有强需求，否则不纳入自动复用。

## 自查

- 没有把 `cache` 设计成第二套 package/sdk 管理入口。
- `cache clean` 默认行为保守，不默认清理 SDK index。
- `cache serve` 从第一期就有 manifest schema，后续自动 mirror 不需要推翻服务端协议。
- 客户端自动 mirror 被设计为后续阶段，不阻塞首期最需要能力。
- 所有删除和文件服务都以 `cache_dir` 为安全边界。
