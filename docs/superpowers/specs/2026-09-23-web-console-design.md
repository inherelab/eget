# eget Web 控制台设计

> 状态：设计稿（2026-09-23），待评审。
>
> 已确认决策：**范围=完整含安装与卸载**、**前端=构建型（Vite/React）**、**访问控制=token + 可选 `--host` 对外**、**HTTP 层=`github.com/gookit/rux/v2`**、**删除 `eget cache serve` 命令并由 web 接管其机器接口**、**任务历史落 JSON 文件**、**允许编辑配置**、**不提供原生 TLS**。

## 背景

eget 目前有 13 个顶层命令（`install` / `download` / `uninstall` / `list` / `show` / `update` / `config` / `query` / `search` / `cache` / `sdk` / `ext` / `add`），全部通过终端使用。存在两个现实痛点：

- **看不见全局状态**：已装了什么、哪些过期、外部管理器（npm/pnpm/uv/pipx/cargo/bun/scoop）里有多少包，得逐条敲命令才能拼出全貌。
- **操作门槛**：批量更新、清理缓存、查看包详情都需要记住命令与参数组合。

仓库已有一次"把 eget 能力搬到 HTTP 上"的实践 —— `eget cache serve`，它包含两类性质完全不同的东西：

- **人类界面**：`GET /` 返回内联 `html/template` 页面（`internal/app/cache/ui.go`）。
- **机器接口**：`/manifest.json`、`/download/{path-md5:<key>}`、`/files/*`，其中前两者是 **eget↔eget 的缓存分发协议** —— 客户端在 `internal/cachemirror/key.go` 拼接 `<mirrorBaseURL>/download/path-md5:<key>`，由 `internal/sdk/download.go` 与 `internal/install` 的缓存镜像下载路径消费。

这个区分决定了接管方式（见下节）：**人类界面与命令本身删除，机器接口原样迁移**。

现有可复用资产（相对路径）：

- `internal/app/cache/server.go`：路由 switch、`cleanCacheRelPath` / `pathStaysInDirAfterSymlinks` 防穿越与防软链逃逸。
- `internal/app/cache/auth_log.go`：`withBearerToken` / `withJSONLog` / `withTextLog` 中间件（标准 `func(http.Handler) http.Handler`，可经 `rux.WrapHTTPHandler` 转为 rux 中间件）。
- `internal/cli/cache_handler.go`：`net.Listen` + `http.Server.Serve` 的监听与地址回显。

## 目标

- `eget web` 启动一个本地 HTTP 服务，浏览器中**查看并操作** eget 的全部核心能力（安装、卸载、更新、缓存清理、SDK、外部管理器、配置查看与编辑）。
- **业务逻辑零复制**：web 层只做协议转换与编排，全部能力调用 `internal/app/*` 服务层，与 CLI handler 同层。
- **变更操作可观测**：所有写操作进入任务引擎，串行执行，实时日志与进度通过 SSE 推送到页面。
- **默认安全**：默认只绑 loopback，默认要求 token；对外监听是显式 opt-in。
- **成为唯一的 HTTP 入口**：缓存分发协议与缓存管理界面都归入 `eget web`。

## 非目标

- **不做通用命令执行**：不提供 shell、不提供任意 argv 透传，所有操作都是结构化参数（包名、管理器名、任务类型）。
- **不做多用户 / 账号体系 / 权限模型**：单用户本地工具，token 即唯一凭据。
- **不做 `self-update`**：它会替换正在运行的可执行文件，在常驻服务进程中风险不可控，永久留在 CLI。
- **不改动 `/download/path-md5:`、`/manifest.json`、`/files/*` 的路径与语义**：它们是跨机器协议，任何变化都会破坏既有镜像客户端。
- **不提供原生 TLS**：对外监听只可能置于反代之后；`--host` 仍可用，但明文传输风险自负。
- 不引入 gin/echo 等框架；不引入 WebSocket（SSE 单向推送足够）。

## 决策记录

| # | 决策 | 选择 | 备选与否决理由 |
|---|---|---|---|
| 1 | 功能范围 | 完整含安装与卸载 | 备选"只读优先"范围更小、风险更低，但用户要求 web 成为完整替代入口 |
| 2 | 前端形态 | Vite/React 构建型 | 备选"内联模板零依赖"（对齐 cache serve）被否：用户要求构建型前端 |
| 3 | 访问控制 | token + 可选 `--host` 对外 | 备选"仅 loopback 无 token"被否：需要局域网访问能力 |
| 4 | HTTP 层 | **`github.com/gookit/rux/v2`** | 与既有 gookit 生态一致；自带 `pkg/sse`、`StaticFS`、`pkg/binding`、生产级 `server` 包、`WrapHTTPHandler` 适配器。备选"标准库裸 switch"被否：需自研 SSE/路由/优雅关闭 |
| 5 | 前端产物嵌入 | `go:embed` + `rux.StaticFS` | 单文件分发；Vite `outDir` 直指 embed 包目录，免去拷贝步骤 |
| 6 | 实时推送 | SSE（`rux/v2/pkg/sse`） | 单向推送足够；库内已含 keepalive 与按 key 推送的 `Hub`（正好按 taskId 订阅） |
| 7 | 变更执行模型 | 进程内单任务串行队列 | 规避 `installed.toml` 无锁读改写与 npm/pnpm 争用同一全局目录 |
| 8 | web 包位置 | `internal/app/web` | AGENTS.md 子包约定；与 `internal/app/cache` 对称 |
| 9 | 二进制与前端的关系 | 未构建前端时仍可编译（占位 `dist/.gitkeep`） | 否则每个跑 `go test ./...` 的人都必须装 node |
| 10 | `cache serve` 的归宿 | **删除命令与其 UI** | 用户明确同意；机器接口由 web 接管，命令本身无对外承诺 |
| 11 | 机器接口（`/manifest.json`、`/download/*`、`/files/*`） | **路径与语义不变**，迁到 web 路由 | 客户端 URL 由 `internal/cachemirror` 拼出，不可改 |
| 12 | 任务历史 | 落 JSON 文件（`tasks.json`）+ 原子写 | 进程重启后仍可回溯；全量日志事件不落盘 |
| 13 | 配置编辑 | **允许**（web 端提供编辑） | 需原子写 + 键白名单 + 语义校验 |
| 14 | 原生 TLS | **不提供** | 对外监听建议置于反代之后 |
| 15 | 依赖拉取 | 无阻塞（用户已配置 Go proxy） | 新增 rux/v2 可直接 `go mod tidy` |

## 接管 cache serve

`eget cache serve` 命令**删除**，能力全部并入 `eget web`；机器接口的路径与语义保持不变。

### 拆分与归宿

| 现有部分 | 归宿 | 说明 |
|---|---|---|
| `GET /` 内联 HTML 页面（`internal/app/cache/ui.go`、`ui_test.go`） | **删除** | 由 React 的 Cache 页面提供 |
| `GET /manifest.json` | **迁移到 web 根路径** | 路径与语义不变 |
| `GET /download/{path-md5:*}` | **迁移到 web 根路径** | 跨机器协议，客户端 URL 由 `internal/cachemirror/key.go` 拼出，不可改 |
| `GET /files/*` | **迁移到 web 根路径** | 保留 root scope 与 no-index 语义（web 侧 `--cache-root` / `--no-cache-index`） |
| `cleanCacheRelPath` / `pathStaysInDirAfterSymlinks` | **保留并复用** | web 的缓存文件路由继续调用 |
| `internal/app/cache/auth_log.go` 的中间件 | **迁到 `internal/app/web/middleware.go`** | 认证/日志是 HTTP 层职责；同时修正常量时间比较等缺口 |
| `internal/app/cache/server.go` 的 `NewHandler` + `ServeHTTP` switch | **删除**，改为三个可挂载 handler | 见下 |
| `internal/cli/cache_cmd.go` 的 `newCacheServeCmd` / `CacheServeOptions`、`internal/cli/cache_handler.go` | **删除** | 连同 `handlers.go` 的 `case "cache.serve"`、`app.go` 的 flag 规格与别名、`service.go` 的相关字段 |
| `internal/cli/cache_cmd.go` 的其余子命令（`list` / `status` / `clean`） | **保留** | 只删除 `serve` |

### 挂载方式

`internal/app/cache/machine.go`（新）导出三个纯 `http.HandlerFunc`：

```go
func ManifestHandler(service Service, cacheDir string, opts MachineOptions) http.HandlerFunc
func DownloadHandler(service Service, cacheDir string, opts MachineOptions) http.HandlerFunc
func FileHandler(service Service, cacheDir string, opts MachineOptions) http.HandlerFunc
```

web 侧按原路径挂载：

```go
r.GET("/manifest.json", rux.WrapHTTPHandlerFunc(appcache.ManifestHandler(...)))
r.GET("/download/*key", rux.WrapHTTPHandlerFunc(appcache.DownloadHandler(...)))
r.GET("/files/*path", rux.WrapHTTPHandlerFunc(appcache.FileHandler(...)))
```

`appcache.ServeOptions` 更名为 `MachineOptions`（去掉只对旧监听循环有意义的字段），保留 token / root scope / no-index 语义。

### 鉴权

机器接口沿用 `Authorization: Bearer <token>`，由 **web 的统一认证中间件**处理 → 对既有 `cachemirror` 客户端**完全透明**（它本就发这个头）。文档中 `eget cache serve --token` 的用法改写为 `eget web --token`。

### 迁移与兼容处理

- **这是破坏性变更**：`eget cache serve` 将不可用，发布说明与 `docs/web.md` 必须给出对照表（旧命令/选项 → 新命令/选项）。
- `eget cache list|status|clean` 保留；在 `cache` 命令的帮助文本中加一行提示"启动 HTTP 服务请用 `eget web`"。
- `docs/TODO.md`、`docs/architecture.md`、`README.md` / `README.zh-CN.md` 中所有 `cache serve` 段落同步更新。
- `docs/superpowers/specs/2026-05-26-cache-management-design.md` 与 `docs/superpowers/plans/2026-05-27-cache-serve-web-ui.md` 加修订说明（其"不新增前端依赖"与"serve 命令"设计已被本设计取代）。

## 架构

```
cmd/eget/main.go
  └─ internal/cli/app.go            newApp：注册 web 命令 + flag 规格表
       └─ internal/cli/web_cmd.go   newWebCmd：WebOptions + 快照传给 handler
            └─ internal/cli/web_handler.go  handleWeb：组装依赖、启动 rux server
                 │
                 └─ internal/app/web/         ← 新包：HTTP 层，无 CLI 依赖
                      ├─ server.go      NewServer(Deps, Options) *rux.Router + server.Server
                      ├─ routes.go      rux 路由分组 + SPA fallback
                      ├─ middleware.go  auth / host / origin / csrf / log / recover（含接管自 cache 的中间件）
                      ├─ api_read.go    只读端点（含 config 视图）
                      ├─ api_config.go  配置编辑（校验 + 原子写）
                      ├─ api_task.go    变更端点 + 任务查询 + SSE
                      ├─ tasks.go       任务引擎（串行队列 + sse.Hub + tasks.json 持久化）
                      ├─ install_adapter.go  非交互 runner 装配与输出行捕获
                      ├─ assets.go      //go:embed all:dist + rux.StaticFS
                      └─ dist/          Vite 构建产物（构建时生成，git 忽略）
                              │ 只调用
                              ▼
                 internal/app/*    ListService / UpdateService / UninstallService /
                                   ConfigService / ShowService / QueryService / SearchService
                 internal/app/cache.Service + 机器接口 handler、
                 internal/sdk.Service、internal/extpkg.Service
```

依赖方向单向：`cli → app/web → app/*`。`internal/app/web` **不得** import `internal/cli`，也不直接使用 `internal/install` 的 UI 相关默认实现（只通过注入字段配置）。

## 命令层设计

`internal/cli/web_cmd.go`：

```go
type WebOptions struct {
    Host           string // 默认 127.0.0.1
    Port           int    // 默认 8787
    Token          string // 空则自动生成
    NoAuth         bool   // 仅 loopback 允许
    ReadOnly       bool   // 关闭全部变更端点
    AllowMutations bool   // 非 loopback 时必须显式开启
    Open           bool   // 启动后打开浏览器
    CacheRoot      string // 机器接口的 root scope
    NoCacheIndex   bool   // 机器接口禁目录列表
    JSONLog        bool
    Verbose        bool
}
```

`newWebCmd(handler)` 遵循既有三件套（`cmd.Config` 绑定 flag → `cmd.Func` 快照 → 返回 reset 闭包），`cmd.Func` 内做 `validateNoFlagArgs`。

**必须在 6 处登记**（漏掉任一处即静默失效或直接报错）：

1. `internal/cli/app.go` 的 `newApp`：`app.add(newWebCmd(handler))`
2. `internal/cli/app.go` 的 `commandFlagSpecs`：`"web": {bools: setOf("no-auth","read-only","allow-mutations","open","no-cache-index","json-log"), values: setOf("host","port","p","token","cache-root")}`（否则 `validateKnownFlags` 拦不住拼错的 flag）
3. `internal/cli/app.go` 的 `commandAliases`（可选）
4. `internal/cli/app.go` 的 `commonCommandHelp`（示例文本）
5. `internal/cli/handlers.go` 的 `handle` switch：`case "web":`（同时删除 `case "cache.serve"`）
6. `internal/cli/service.go` / `wiring.go`：把已有服务实例组装成 `web.Deps`

**启动与生命周期**：`internal/app/web/server.go` 内自建监听循环（rux 的路由、中间件、SSE 全部保留）：

```go
router := rux.New()                    // 首次 ServeHTTP 后路由表冻结
router.Use(recover, log, securityHeaders, host, auth)
// ... 注册路由 ...
httpServer := &http.Server{
    Handler:           router,
    ReadHeaderTimeout: 5 * time.Second,
    ReadTimeout:       60 * time.Second,
    WriteTimeout:      0,              // SSE 与长下载必需
    IdleTimeout:       120 * time.Second,
    MaxHeaderBytes:    1 << 20,
}
listener, err := net.Listen("tcp", addr)
onReady(listener.Addr().String())      // 回传实际地址，支撑 port=0 与 --open
go func() { <-ctx.Done(); httpServer.Shutdown(stopCtx) }()
serveErr := httpServer.Serve(listener)
```

**实现偏差（M1a 实测确认）**：原计划用 `rux/v2/server` 包（`server.New` + `Run()`）换取开箱即用的超时与优雅关闭，但它的 `Run()` 不回传实际监听地址 —— 于是 `--port 0` 的随机端口无法回显、`--open` 也拼不出 URL。改为自建循环后逐项补齐等价能力（上述超时、`signal.NotifyContext(SIGINT/SIGTERM)` → `Shutdown`、显式 `WriteTimeout = 0`、自注册 `/healthz` 与 `/readyz`），代价约 30 行样板代码，收益是地址回显、`--open` 与测试可控性。

> **更新（2026-09-23 晚）**：rux v2.1.0 已提供 `SetListener`/`ServeListener`/`ListenAddr` 与带中间件链的 404/405 处理，实现已迁回 `server` 包（`Serve` 里仍保留一个 ctx 取消钩子供测试与外部取消），见「实施记录」。

- `port=0` 表示随机端口；默认端口 `8787` 由 CLI flag 提供（`web.DefaultPort`）。
- 启动信息写 **stderr**：实际地址、配置路径、缓存目录与 mirror scope、token（`--no-token-print` 可抑制）、只读/可写状态、对外监听警告。
- **非 loopback 监听必须显式提供 `--token`**：空 token 不会自动生成后对外监听（fail closed，已实测拒绝启动）。
- `--open`：按 `runtime.GOOS` 分派 `rundll32 url.dll,FileProtocolHandler <url>`（Windows，仓库已有 rundll32 先例）/ `open`（darwin）/ `xdg-open`（linux）；URL 带上 token 便于首次引导，随机端口与通配地址用实际绑定地址替换（`0.0.0.0`/`::` → `127.0.0.1`），失败只告警不退出。

## HTTP 服务层（rux/v2）

### 路由

```go
r := rux.New()                                   // 首次 ServeHTTP 后路由表自动冻结
r.Use(logMW, recoverMW, hostMW, authMW)          // 全局中间件

api := r.Group("/api", func() {
    r.GET("/overview", h.Overview)
    r.GET("/packages", h.ListPackages)
    r.GET("/packages/{name}", h.ShowPackage)
    r.GET("/outdated", h.Outdated)
    r.GET("/ext/{manager}/packages", h.ExtPackages)
    r.GET("/config", h.ConfigView)
    r.PUT("/config", h.ConfigUpdate, csrfMW)
    r.POST("/update", h.SubmitUpdate, csrfMW)
    r.GET("/tasks/{id}/events", h.TaskEvents)    // SSE
})

// 机器接口：原路径挂载，保证跨机器协议不变
r.GET("/manifest.json", rux.WrapHTTPHandlerFunc(appcache.ManifestHandler(svc, dir, opts)))
r.GET("/download/*key", rux.WrapHTTPHandlerFunc(appcache.DownloadHandler(svc, dir, opts)))
r.GET("/files/*path", rux.WrapHTTPHandlerFunc(appcache.FileHandler(svc, dir, opts)))

r.StaticFS("/assets", http.FS(distFS))           // 前端产物（内容哈希文件名，长缓存）
r.NotFound(h.SPAFallback)                        // 未匹配 → index.html（仅 GET/HEAD）
```

要点：

- **路由必须在启动时全部注册完毕**：rux v2 在首次 `ServeHTTP` 后冻结路由表，运行期不能再加路由。
- **路径参数语法**：`{name}` 命名参数、`*path` 通配；v2 **不支持**正则约束（如 `{id:\d+}`），校验放在 handler 或小中间件里。
- `PUT`/`POST` 类路由挂 `csrfMW`；`/api/tasks/{id}/events` 不挂任何会设置 `WriteTimeout` 的包装。

### 中间件

rux 中间件与标准 `http.Handler` 中间件可经 `rux.WrapHTTPHandler` 互换，因此**接管并扩展** `internal/app/cache/auth_log.go` 的既有实现（迁入 `internal/app/web/middleware.go`）：

| 中间件 | 来源 | 改动 |
|---|---|---|
| `authMW` | 迁自 `appcache.withBearerToken` | ① 常量时间比较（`crypto/subtle`）；② 支持 `Authorization: Bearer`（机器接口/CLI）与 `X-EGET-Token`（前端），`?token=` 仅用于首次引导后换 cookie；③ 放行 `/healthz`、`/readyz`；④ 失败限速 |
| `logMW` | 迁自 `appcache.withJSONLog` / `withTextLog` | ① 路径脱敏（丢弃 `?token=`）；② 输出到 stderr |
| `recoverMW` | 迁移时新增 | panic → 500，避免坏请求打死进程 |
| `hostMW` | 迁移时新增 | `Host` 白名单，挡 DNS rebinding |
| `csrfMW` | 迁移时新增 | 变更方法要求 `X-EGET-Token` 或 `Origin` 与 `Host` 同源 |
| `securityHeadersMW` | 迁移时新增 | `nosniff` / `Referrer-Policy` / `X-Frame-Options` / CSP |

### 请求与响应

- 请求体绑定用 `rux/v2/pkg/binding`（JSON → struct），**不使用**其校验扩展（白名单校验在 handler 内显式做）。
- 响应统一 JSON，错误结构 `{"error":{"code":"...","message":"..."}}`，状态码语义化（400/401/403/404/409/422/500）；可用 `rux/v2/pkg/render` 简化。

### 静态资源与降级

```go
// internal/app/web/assets.go
//go:embed all:dist
var distFS embed.FS
```

Vite 的 `build.outDir` 直接指向 `../internal/app/web/dist`（相对前端工程目录），产物无需拷贝即可被 embed。由于 `.gitignore` 的 `dist/` 规则会忽略该目录，仓库内**保留占位文件** `internal/app/web/dist/.gitkeep` 并加白名单，保证：

- 未构建前端的机器上 `go build ./...` / `go test ./...` **仍然通过**。
- 启动时若 `dist/index.html` 不存在，`SPAFallback` 返回说明页（"前端资源未构建，请执行 `make web-build`"）。

## API 规格

### 只读端点

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/overview` | 版本、配置路径、store 计数、可更新数、ext 管理器状态、活跃任务 |
| GET | `/api/packages?scope=all\|eget\|ext&ext=npm,pnpm&outdated=true&q=&sort=` | 包列表 |
| GET | `/api/packages/{name}` | 包详情（`ShowService.ShowPackage`） |
| GET | `/api/outdated?scope=` | 可更新项（`ListUpdateCandidates*`，网络操作，显式触发） |
| GET | `/api/ext` | 管理器表（available / outdated / 计数） |
| GET | `/api/ext/{manager}/packages` | 单个管理器的包 |
| GET | `/api/cache`、`/api/cache/status`、`/api/cache/clean/preview` | 缓存视图与清理预览 |
| GET | `/api/sdk`、`/api/sdk/index` | 已装 SDK 与索引 |
| GET | `/api/config` | 配置视图（值、来源路径、生效优先级、每项来源） |
| GET | `/api/query?target=`、`/api/search?q=` | 查询与搜索 |

示例 `GET /api/overview`：

```json
{
  "version": "0.9.4",
  "configPath": "C:/Users/me/.eget.toml",
  "store": { "packages": 39, "gui": 12 },
  "outdated": { "checked": 41, "total": 5 },
  "ext": [{ "manager": "npm", "available": true, "outdated": true, "packages": 9 }],
  "tasks": { "running": 1, "queued": 0 }
}
```

### 变更端点（提交任务）

| 方法 | 路径 | body（结构化） |
|---|---|---|
| POST | `/api/update` | `{targets:["fd","npm:typescript"], all:false, scope:"all\|eget\|ext"}` |
| POST | `/api/install` | `{target:"owner/repo", version:"", asset:"", output:"", addToConfig:false, silent:true}` |
| POST | `/api/uninstall` | `{target:"fd", purge:false}` |
| POST | `/api/ext/upgrade` | `{manager:"npm", names:["typescript"]}` |
| POST | `/api/sdk/install` | `{target:"node@20"}` |
| POST | `/api/cache/clean` | `{mode:"older", days:30, kinds:["pkg"]}` |
| PUT | `/api/config` | 同步执行（不走任务队列），见"配置编辑"章节 |
| GET | `/api/install/candidates?target=owner/repo` | 候选资产列表（两步式选择） |

统一返回 `202 Accepted`：

```json
{ "taskId": "t_20260923_8f3a1c", "kind": "update", "status": "queued" }
```

**安装的两步式资产选择**（替代终端里的交互式 `Prompt`）：

1. `GET /api/install/candidates?target=owner/repo` → `{candidates:["tool-x86_64.msi","tool-x86_64.zip"], needsChoice:true}`
   —— 复用 `internal/install/detect` 的纯函数链路（`detector.Detect` 返回候选列表、`autoSelectAssetCandidate` 做平台自动裁决），不打印、不交互。
2. `POST /api/install` 带 `asset` 精确指定；若候选多于一个且未指定，任务以 `422` 失败并回传候选列表（**不猜测**）。

### 任务端点

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/tasks?limit=50` | 任务列表（含历史，来自内存 + `tasks.json`） |
| GET | `/api/tasks/{id}` | 任务详情与结果 |
| GET | `/api/tasks/{id}/events` | **SSE** 实时事件流 |
| POST | `/api/tasks/{id}/cancel` | 取消（需要 ctx 透传，见 M3） |

## 任务引擎与进度

### 任务模型（`internal/app/web/tasks.go`）

```go
type Status string // queued | running | succeeded | failed | canceled | interrupted

type Task struct {
    ID         string
    Kind       string   // update | install | uninstall | ext.upgrade | sdk.install | cache.clean
    Params     map[string]any
    Status     Status
    CreatedAt  time.Time
    StartedAt  time.Time
    FinishedAt time.Time
    Progress   Progress // percent / phase / current / total
    Logs       []LogLine // 仅保留最近 N 条（落盘同样只存摘要）
    Result     any
    Err        string
}

type Engine struct {
    mu    sync.Mutex
    tasks map[string]*Task
    order []string
    queue chan *job
    hub   *sse.Hub     // 按 taskID 推送事件
    store *taskStore   // tasks.json 持久化 + 原子写
}
```

- **单 worker 串行执行**：同一时刻只有一个变更任务在跑。理由：`installed.toml` 是整表读改写且无锁；`npm` 与 `pnpm` 争用同一全局目录（`update_candidates.go` 已有"含外部包时强制串行"的先例）。
- 队列有界（默认 32），满则 `409 Conflict`，不静默丢弃。

### 任务历史持久化

- 落盘位置：`<config dir>/tasks.json`（与 `installed.toml` / `sdk.installed.json` 同目录），复用本期引入的**原子写**（同目录临时文件 + rename）与 mutex。
- 写入时机：任务入队、状态变更、进度里程碑（节流：≥1s 或 ≥5% 变化）、结束 —— **不逐条持久化日志事件**（避免写放大）。
- 持久化内容：任务元数据 + 结果 + 最近 N 条日志（默认 50）；滚动保留最近 200 条任务。
- 启动恢复：`running` / `queued` 一律标记为 `interrupted`（进程终止后无法续跑），UI 明确标注。
- 全量日志事件只存在于内存与 SSE 会话；页面刷新后展示持久化的摘要。

### SSE 实现

用 `rux/v2/pkg/sse`，不手写帧格式：

```go
r.GET("/api/tasks/{id}/events", func(c *rux.Context) {
    id := c.Param("id")
    _ = sse.StreamWith(c, &sse.Options{
        SendConnected:     true,
        KeepaliveInterval: 30 * time.Second,
    }, sse.HubProducer(engine.Hub(), id))
})
```

- 生产者是引擎侧：任务每次状态/进度/日志变化都 `hub.Send(taskID, sse.Event{Name: "log"|"progress"|"status"|"done", Data: ...})`。
- `Hub` 的 keyed push + 多连接扇出对应"一个任务被多个标签页订阅"。
- 连接建立时的**鉴权在 `OnConnect` 回调里做**（它在写 SSE 头之前运行，可返回 401）。
- ⚠️ **`server.Server.WriteTimeout` 必须设为 0** —— 它是整个响应生命周期的上限，心跳无法挽救。

### 进度来源（三级，按落地顺序）

1. **现成回调（M2 即可用）**：`app.UpdateService` 的 `OnCheckDone` / `OnUpdateStart` / `OnUpdateDone`，`app.ListService` 的 `OnCheckDone` / `OnExternalFailure`。
   ⚠️ **每请求拷贝一份 service 值**再设置回调（这些是值类型 struct，天然隔离），**绝不修改共享实例的字段** —— CLI 现有的"临时替换 + defer 还原"模式在多请求并发下不安全。
2. **输出行捕获（M2）**：为安装链路构造专用 `install.InstallRunner`，把 `Stdout` 指向一个**行解析 writer**（剥 ANSI 序列后按行转 `log` 事件）；`Stderr` 同理。
3. **结构化进度（M3）**：`install.Options` 新增 `Progress func(Event)` 与 `Context context.Context`；`runner_download.go` 的 `downloadProgress` 在回调非 nil 时返回计数 writer 而非终端进度条。

## 配置编辑

允许通过网页修改 `eget.toml`：

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/config` | 全量视图：值、来源路径、生效优先级、每个键是否来自配置文件 |
| POST | `/api/config/validate` | 只校验不落盘，返回将要发生的变化（diff） |
| PUT | `/api/config` | 批量设置：`{"set":{"ext.npm.bin":"npm-custom"},"unset":["global.proxy_url"]}` |

安全与正确性约束：

- **必须用 `cfgpkg.SaveAtomic`**（`internal/config/atomic.go` 已有同目录临时文件 + rename），不用默认的非原子 `Save`。
- **键白名单**：复用 `internal/config/gookit.go` 的 `SetByPath` / `GetByPath`，但 web 层先按允许集合过滤；**禁止**写 `token`（web 段）、`meta` 等敏感或内部键。
- 写入前做**类型与语义校验**（端口范围、路径存在性、枚举值），失败返回 422 并附原因。
- 与任务引擎共用一把配置锁：更新任务会读配置，编辑配置与任务执行不得并发写同一文件。
- 前端提供 diff 预览与二次确认（保存前展示将要改变的键）。

## 非交互化改造

安装链路当前与终端强耦合，web 侧通过**注入字段**覆盖（`internal/install` 的这些字段已存在）：

| 注入点 | 位置 | 默认实现（CLI） | web 策略 |
|---|---|---|---|
| `Runner.Prompt` | `runner.go:47` | `prompts.Select`（TTY 选择） | 返回错误 + 候选列表 → 任务失败，前端引导两步选择 |
| `Runner.ConfirmLaunchInstaller` | `runner.go:47` | `defaultConfirmLaunchInstaller`（读 `os.Stdin`） | `silent` 为真时直接返回 `true`（不询问），否则 `false` 并提示"仅下载" |
| `Runner.InstallerLauncher` | `gui.go:19` | `DefaultInstallerLauncher`（Windows 走 `shellExecute runas`，会弹 UAC） | 默认不启动；`silent=true` 时按安装器类型追加静默参数 |
| `Runner.AssetRunner` | `runner.go:53` | `nil` → `exec.Command` 直接跑下载物 | **禁用**（web 永不执行下载的可执行文件） |
| `Runner.Stdout` / `Stderr` | `runner.go` | `os.Stdout` / `os.Stderr` | 行捕获 writer → SSE |
| `Options.Quiet` | `options.go:51` | 抑制输出 + 多候选取第一个 | web **不使用**（要输出、不猜测）；语义拆分由 `Prompt` 注入承担 |

⚠️ 唯一的硬编码漏点：`internal/install/runner_installer.go:21` 的 `defaultConfirmLaunchInstaller` 直接读写 `os.Stdin`/`os.Stderr`，绕过 `r.Stdout`/`r.Stderr`。web 侧注入替代实现即可绕过，但该函数本身应顺带改成使用注入的 writer（M2 小改）。

**静默安装参数**（M3/M4）：`internal/install/gui.go` 的 `windowsInstallerCommand` 当前 MSI 返回 `msiexec.exe /i "<path>"`、EXE 返回裸路径，没有无人值守开关。需扩展：MSI `msiexec /i "<path>" /qn /norestart`、NSIS `/S`、Inno Setup `/VERYSILENT /SUPPRESSMSGBOXES`，并新增 `Options.Silent bool`。

**取消能力（M3）**：`install.Options` 加 `Context`，`InstallRunner.Run` 加检查点，并把 ctx 透传到 `internal/client`（`DownloadFile` / `DownloadWithResult` / `GetWithOptions` 改用 `http.NewRequestWithContext`；`runner_download.go:133` 的 `cachemirror.DownloadToFile` 已支持 ctx，只是当前写死 `context.Background()`）。`internal/sdk` 是已经 ctx 化的现成模板。

## 卸载与 SDK

- **卸载**：`app.UninstallService.UninstallWithOptions` 已经完全服务化 —— 非交互、无网络、无外部进程。web 直接调用，M2 交付。
- **SDK**：`internal/sdk.Service` 已带 `ctx` 与 `Progress` / `OnStart` / `OnArchiveReady` / `OnExtractStart` 回调，M3 接入。
- **外部管理器**：`extpkg.Service` 已带 `ctx`，`Runner` / `LookPath` 可注入，M2 接入 `upgrade`。
- **缓存清理**：`app/cache.Service` 的 `PreviewClean` → `ApplyClean` 天然对应"预览 + 确认"两步 UI，M2 接入。

## 前端工程

### 目录与构建

```
web/                          ← 前端工程根（新建）
├─ package.json               (pnpm)
├─ pnpm-lock.yaml
├─ vite.config.ts             outDir: ../internal/app/web/dist
├─ tsconfig.json
├─ index.html
└─ src/
   ├─ main.tsx / App.tsx
   ├─ api/client.ts           统一 fetch 封装（注入 X-EGET-Token）
   ├─ api/sse.ts              EventSource 封装与重连
   ├─ pages/{Overview,Packages,PackageDetail,Outdated,Ext,SDK,Cache,Config,Tasks}.tsx
   ├─ components/{TaskPanel,Table,FilterBar,ConfirmDialog,ConfigEditor}.tsx
   └─ hooks/{useTasks,usePolling}.tsx
```

- 构建：`make web-build`（`cd web && pnpm install --frozen-lockfile && pnpm build`），`make build` 与 `make build-all` 依赖它。
- **交叉编译注意**：前端产物平台无关，只需构建一次即可被 5 个平台的 `go build` 复用。
- 开发模式：`pnpm dev` 起 Vite dev server，`vite.config.ts` 配 `server.proxy` 把 `/api` 转发到 `eget web`（默认 `127.0.0.1:8787`）。

### 信息架构

| 页面 | 内容与操作 |
|---|---|
| Overview | 版本、配置路径、store 统计、可更新数、ext 管理器状态、活跃任务 |
| Packages | 合并列表（eget + ext），来源列、过期标记、搜索/筛选/排序，行内"更新/详情/卸载" |
| Package Detail | `show` 全量字段 + 更新/卸载/上游链接 |
| Outdated | 可更新列表，单个/批量更新，检查进度（复用 `OnCheckDone`） |
| Ext | 管理器表 + 单包升级 |
| SDK | 已装 SDK、索引浏览、安装/下载 |
| Cache | 列表/状态、清理预览 → 确认执行；**取代 `cache serve` 的只读页面** |
| Config | 查看 + **编辑**（diff 预览、校验、原子保存） |
| Tasks | 任务列表 + 实时日志面板（SSE），支持取消；`interrupted` 状态可见 |

### 状态与交互约定

- 变更操作统一"**发起 → 跳任务 → SSE 流式日志 → 完成后刷新对应数据**"。
- 危险操作（卸载、清理、批量更新、配置保存）二次确认对话框，显示将受影响的具体条目/键。
- 页面 token 来源：首次带 `?token=` 加载后由服务端种 `HttpOnly` + `SameSite=Strict` cookie，随后前端调用改带 `X-EGET-Token` 头。

## 安全模型

### 绑定与凭据

- 默认 `Host=127.0.0.1`。非 loopback 绑定必须显式 `--host`，启动时打印**醒目警告**（明文 HTTP、token 会在网络中传输）。
- token：`--token` 指定，否则 `crypto/rand` 生成 32 字节十六进制；启动时打印到 stderr（`--no-token-print` 可抑制）；**绝不写入日志、绝不持久化到配置文件**。
- 比较用 `crypto/subtle.ConstantTimeCompare`（现有 `withBearerToken` 用 `!=` 明文比较，存在时序侧信道，迁移时修正）。
- `--no-auth` 仅在 loopback 下允许；与 `--host` 非 loopback 组合时**拒绝启动**（fail closed）。
- 不提供内置 TLS：对外监听应置于反代之后（反代负责终止 TLS）。

### 危险操作开关

- `--read-only`：关闭全部变更端点（含配置编辑，返回 403），只保留只读视图。
- 非 loopback 绑定时，变更端点默认**关闭**，需显式 `--allow-mutations`。
- **永久禁止**：`self-update`、任意命令执行、下载物直接执行（`AssetRunner` 禁用）、GUI 安装器交互式启动（`shellExecute runas`）。

### 请求校验

- `Host` 白名单（防 DNS rebinding）：loopback 别名 + 显式 `--host` 值 + `--allow-host` 追加项。
- 变更方法要求 `Origin` 与 `Host` 同源，或存在自定义头 `X-EGET-Token`。
- 所有输入白名单校验：管理器名必须存在于 `extpkg.Service.Names()`；包名/目标拒绝 `..`、路径分隔符、前导 `-`（防注入到 npm/cargo/scoop 等外部命令的参数位）；配置键必须在允许集合内；输出目录必须是绝对路径且不在系统敏感目录。
- 路径参数（`{name}` / `*key`）在 rux handler 内先行净化。
- CSP：`default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; connect-src 'self'; frame-ancestors 'none'`。
- 认证失败限速（简单令牌桶，按远端地址）。
- 文件服务复用 `cleanCacheRelPath` / `pathStaysInDirAfterSymlinks`；已知 TOCTOU 窗口（`EvalSymlinks` 与 `ServeFile` 之间）在 M4 用 `O_NOFOLLOW` 打开方式修掉。

## 并发与持久化

web 引入并发请求，以下**必须先修**（M2 前置）：

1. **`installed.Store` 加进程内互斥 + 原子写**：`Load→改→Save` 是整表读改写且无锁（`internal/installed/store.go`），`Save` 直写目标文件（`internal/util/configutil/toml.go` 的 `DumpToFile`），崩溃会留半截文件。方案：`sync.Mutex` 串行化 + 抽一个 `atomicWrite(path, bytes)`（复用 `internal/config/atomic.go` 的"同目录临时文件 + rename"思路）。
2. **`sdk.Store` 同样处理**（`internal/sdk/store.go`）。
3. **`tasks.json` 使用同一套 `atomicWrite`**（见任务引擎）。
4. **`eget.toml` 的写入统一走 `SaveAtomic`**（配置编辑与 `config set`）。
5. **任务串行化**：见任务引擎（单 worker），天然避免并发写 store。
6. **全局可变状态在启动期固化**：`install.SetVerbose` / `client.SetProxyNoticeWriter` / `client.SetAPICacheNoticeWriter` / `source/{github,forge,sourceforge}.SetVerbose` 等包级 writer 一律在服务启动时设置一次，**请求处理过程中绝不修改**。
7. **不共享可变 service 字段**：`UpdateService.OnUpdateStart` / `OnUpdateDone` / `OnCheckDone` 等回调字段按请求拷值注入。

## 配置

新增 `[web]` 节（CLI flag 优先于配置）：

```toml
[web]
host = "127.0.0.1"
port = 8787
read_only = false
allow_mutations = false
auto_open = false
cache_root = "all"
no_cache_index = false
```

改动点（共约 7 处，需同步，否则 `[web]` 会被误判为 repo 段）：

1. `internal/config/model.go`：新增 `WebSection`（字段用 `*string` / `*int` / `*bool` 以区分未设置，带 `toml` + `mapstructure` 双 tag）。
2. `internal/config/model.go`：`File` 增 `Web WebSection`。
3. `internal/config/gookit.go` 的 `decodeConfigFile`：`MapOnExists("web", &conf.Web)`。
4. `internal/config/gookit.go` 的 `encodeConfigFile`：`data["web"] = webSectionToMap(file.Web)`。
5. `internal/config/gookit.go` 的 `isReservedConfigRootKey`：加 `"web"`。
6. `internal/config/gookit.go`：新增 `webSectionToMap`，并在 `normalizePathValue` 补 `port`（整数）与布尔分支。
7. `docs/config.md` + `docs/config.zh-CN.md` 的 Sections 清单补 `[web]`。

**token 不进配置文件**（一旦落盘就是长期凭据），只允许 CLI 传入或每次随机生成；配置编辑 API 亦显式拒绝写入该键。

## 改造清单

### 新增

| 文件 | 内容 | 难度 |
|---|---|---|
| `internal/cli/web_cmd.go` | `WebOptions` + `newWebCmd` | 易 |
| `internal/cli/web_handler.go` | `handleWeb`：依赖组装、启动 rux server、开浏览器 | 易 |
| `internal/app/web/server.go` / `routes.go` | `NewServer` + rux 路由分组 + SPA fallback | 易 |
| `internal/app/web/middleware.go` | rux 中间件（含接管自 cache 的认证/日志） | 中 |
| `internal/app/web/api_read.go` | 只读端点 | 易 |
| `internal/app/web/api_config.go` | 配置视图、校验、原子写入 | 中 |
| `internal/app/web/api_task.go` | 变更端点 + 任务查询 + SSE | 中 |
| `internal/app/web/tasks.go` | 任务引擎 + `sse.Hub` + `tasks.json` 持久化 | 中 |
| `internal/app/web/install_adapter.go` | 非交互 runner 装配与输出行捕获 | 中 |
| `internal/app/web/assets.go` + `dist/.gitkeep` | `go:embed` 与未构建降级页 | 易 |
| `internal/app/cache/machine.go` | 从 `server.go` 拆出的 `ManifestHandler` / `DownloadHandler` / `FileHandler` | 易 |
| `web/**` | Vite/React 前端工程 | 中大 |
| `docs/web.md` | 使用文档（启动参数、安全须知、旧 `cache serve` → `eget web` 对照表） | 易 |
| `docs/superpowers/specs/2026-09-23-web-console-design.md` | 本文 | — |

### 修改 / 删除

| 文件 | 改动 | 难度 |
|---|---|---|
| `internal/cli/app.go` | 注册 web 命令 + flag 规格；**删除 cache.serve 的规格与别名** | 易 |
| `internal/cli/handlers.go` | 加 `case "web"`；**删除 `case "cache.serve"`** | 易 |
| `internal/cli/service.go` / `wiring.go` | 组装 `web.Deps`；**删除 `cacheServeOptions` 相关字段** | 中 |
| `internal/cli/cache_cmd.go` | **删除 `serve` 子命令与 `CacheServeOptions`**；`cache` 帮助加迁移提示 | 易 |
| `internal/cli/cache_handler.go` | **删除文件**（监听逻辑由 rux server 接管） | 易 |
| `internal/app/cache/server.go` | 拆出 `machine.go`；**删除 `NewHandler` 与 `ServeHTTP` switch**；保留路径防护 | 易 |
| `internal/app/cache/ui.go`、`ui_test.go` | **删除**（UI 由 React 取代） | 易 |
| `internal/app/cache/auth_log.go`、`auth_log_test.go` | 中间件**迁往** `internal/app/web/middleware.go` 并扩展 | 中 |
| `internal/installed/store.go` | 互斥 + 原子写 | 中 |
| `internal/sdk/store.go` | 互斥 + 原子写 | 易 |
| `internal/install/runner_installer.go` | `defaultConfirmLaunchInstaller` 改用注入 writer | 易 |
| `internal/install/options.go` | `Context` / `Progress` / `Silent` 字段 | 易 |
| `internal/install/runner_download.go` | 进度回调分支（非 TTY 走事件） | 中 |
| `internal/client/{network,download_file}.go` | ctx 透传（`NewRequestWithContext`） | 中 |
| `internal/install/gui.go` | 静默安装参数（msi/nsis/inno） | 中 |
| `internal/config/{model,gookit}.go` | `[web]` 节 | 易 |
| `go.mod` / `go.sum` | 新增 `github.com/gookit/rux/v2` | 易 |
| `Makefile` | `web-build` 目标并前置到 `build` / `build-all` | 易 |
| `.github/workflows/go.yml` | 加 node/pnpm 构建步骤；`paths` 放宽到 `web/**` | 易 |
| `.github/workflows/release.yml` | 发布前构建前端 | 易 |
| `.gitignore` | `node_modules/`；`internal/app/web/dist/` 白名单保留 `.gitkeep` | 易 |
| `README.md` / `README.zh-CN.md` / `docs/architecture.md` / `docs/TODO.md` | 命令清单更新（移除 `cache serve`、新增 `web`）；顺带修既有 `make test` 文档缺陷 | 易 |
| `docs/superpowers/specs/2026-05-26-cache-management-design.md`、`docs/superpowers/plans/2026-05-27-cache-serve-web-ui.md` | 加修订说明 | 易 |

## 分阶段实施

每阶段结束都必须 `go test ./...` 全绿 + 真机冒烟，并按功能点提交。

### M1 服务骨架 + 只读面板 + cache 接管

- 引入 rux/v2；命令注册；`NewServer`（rux 路由 + `server` 包）；中间件（含从 cache 迁移并加固的认证/日志）；静态资源 embed 与降级页；只读 API；**`internal/app/cache` 机器接口拆分与挂载、删除 `cache serve` 命令与 `ui.go`**；Vite 工程与 Overview/Packages/Outdated/Ext/Cache/Config 页面；`--open`；`docs/web.md`（含迁移对照表）。
- **验收**：`eget web --open` 打开页面显示真实数据；`curl -H "Authorization: Bearer $T" 127.0.0.1:8787/api/overview` 通过；无 token 401；**用 `cachemirror` 客户端按旧协议从 `eget web` 拉取缓存文件成功**；`eget cache serve` 不再存在且 `eget cache list` 正常；`go test ./...` 全绿；未装 node 的机器 `go build ./...` 通过。

### M2 任务引擎 + 更新/卸载 + 任务持久化

- store 互斥 + 原子写（前置）、任务引擎 + `sse.Hub` + `tasks.json` 持久化与启动恢复、非交互 runner 注入、update（单个/批量/ext）、uninstall、cache clean、Tasks 页面。
- **验收**：网页触发单个更新，SSE 实时显示日志并以 `succeeded` 结束；并发提交两个任务时第二个排队；杀进程重启后任务以 `interrupted` 出现在列表；`-race` 下 `go test ./...` 通过。

### M3 安装链路 + 配置编辑

- 资产两步选择、`Options.Context` + `Progress` + ctx 透传、取消、SDK 安装/下载、静默安装策略；配置编辑 API 与 Config 页面（diff + 校验 + 原子写）。
- **验收**：从网页安装一个真实 GitHub 包（含多候选与指定资产两种场景）；任务可取消且无残留 `.part`；通过网页修改一个配置键，`eget.toml` 与 CLI 读取结果一致，非法键/非法值被拒绝。

### M4 加固与收尾

- 对外监听的完整化（Host 白名单、限速）、TOCTOU 修复、静默安装器参数补全、`[web]` 配置节、文档与 README 收尾。

## 测试策略

- **HTTP 层**：`httptest.NewRequest` + `httptest.NewRecorder()` + `router.ServeHTTP(rec, req)`，表驱动覆盖：鉴权矩阵（无 token / 错 token / 正确 token / `X-EGET-Token` / `/healthz` 与 `/readyz` 白名单）、Host 白名单、Origin 校验、只读模式 403、非 loopback 无 `--allow-mutations` 拒绝启动、路径穿越、SPA fallback 命中与 405。
- **cache 协议兼容**：把原 `internal/app/cache/server_test.go` 的用例改写为直接测三个新 handler，并新增"经 web 路由访问同一路径"的等价用例；`ui_test.go` 随 `ui.go` 删除。
- **任务引擎**：单测覆盖排队/串行/失败/取消/订阅者收发/持久化与恢复（用 `t.TempDir()` 作为 store 目录），用注入的假执行函数避免真实网络。
- **配置编辑**：单测覆盖白名单外键被拒、非法值被拒、原子写落盘后可被 `cfgpkg.Load` 正确读回、并发写不损坏文件。
- **SSE**：用 `httptest.NewServer` + 逐行读取响应体，断言事件名与 JSON 负载。
- **服务层复用**：全部依赖以接口注入（`Deps` 用小接口按需定义），测试注入假实现，不触碰真实 GitHub。
- **前端**：不做组件快照测试；断言"`dist/index.html` 存在且包含入口 script"、`/assets/*` 可达、SPA fallback 行为。
- **真机冒烟**：每个里程碑执行一次真实 `eget web` 会话（只读浏览 + 一次真实更新 + 一次真实卸载 + 一次 mirror 拉取）。

## 风险与取舍

| 风险 | 说明与缓解 |
|---|---|
| **删除 `cache serve` 是破坏性变更** | 依赖该命令的脚本/CI 会失效。缓解：迁移对照表写进 `docs/web.md` 与发布说明；`eget cache -h` 里提示改用 `eget web`；机器接口路径不变，客户端配置只需换端口/命令。 |
| **rux v2 较新** | v2 是 2026 年的 clean-room 重写（当前 v2.0.2），路由表在首次 `ServeHTTP` 后**冻结**，且不支持正则路由约束。缓解：路由全部启动期注册；路径校验放 handler；锁定版本。 |
| **SSE 与服务器超时冲突** | `server.Server.WriteTimeout` 默认 30s 会切断事件流，必须显式设 0；同时减弱对慢速客户端的保护，需靠 `KeepaliveInterval` 与连接数限制兜底。 |
| **与既有成文约定冲突** | `docs/superpowers/specs/2026-05-26-cache-management-design.md` 写着"不新增前端依赖，不引入静态资源目录"。本设计**推翻**该决定（用户已明确选择构建型前端），需在该 spec 加修订说明。 |
| **AGENTS.md 确认门槛** | 约定"改动超过 3 个逻辑文件或 100 行需先确认"。本设计远超阈值，实施前需逐阶段确认。 |
| **cache 协议被误改** | `/download/path-md5:` 与 `/manifest.json` 是跨机器契约。缓解：把"路径与语义不可变"写进非目标；保留原测试并按新 handler 改写；M1 验收含真实 mirror 拉取。 |
| **配置编辑写坏配置** | 缓解：`SaveAtomic` + 键白名单 + 类型/语义校验 + 前端 diff 确认；`--read-only` 可整体关闭。 |
| **构建链复杂度上升** | CI/发布/本地开发都新增 node + pnpm 依赖；`go:embed` 要求"先前端后 Go"的顺序，顺序错了会以难读的编译错误失败。缓解：占位 `.gitkeep`；`make` 目标串好顺序；文档写明。 |
| **CI 触发缺口** | 现有 `go.yml` 的 `paths` 只含 `go.mod` / `**.go` / `**.yml`，纯前端改动不会触发 CI。必须放宽到 `web/**` 等。 |
| **安装链路的非交互化** | 候选资产、GUI 安装器确认、执行下载物都需要"不猜测、不执行"的策略；两处硬编码（`defaultConfirmLaunchInstaller`、`windowsInstallerCommand` 无静默参数）需要小改。 |
| **取消能力缺口** | `internal/client` 下载链路全线无 ctx，需逐层加参数（M3），是本设计中改动面最大的一处。 |
| **并发安全** | store 无锁非原子是硬伤，M2 前置必须修；包级全局 writer 需启动期固化。 |
| **对外监听风险** | 明文 HTTP + token 在局域网传输；默认关闭变更端点并要求显式 opt-in；部署到非可信网络时应置于反代（终止 TLS）之后。 |
| **进程模型** | 前台阻塞进程，无 daemon / pidfile / 开机自启（与 `cache serve` 一致）。跨平台常驻需另行设计。 |

## 实施记录（2026-09-23）

| 阶段 | 状态 | 提交 |
|---|---|---|
| M1a 服务骨架 + 只读 API + 缓存镜像接管 + 删除 `eget cache serve` | 已完成 | `03eb196`、`288e151` |
| M1b Vite/React 前端与 embed 产物 | 已完成 | `0e2d40a` |
| M2 任务引擎 + SSE + `tasks.json` + 写入端点 + store 加锁/原子写 | 已完成 | `ad99046` |
| M3a 配置编辑（原子写 + 键白名单 + diff 预览） | 已完成 | `beb7f80` |
| M3b 安装链路（取消、进度、非交互、SDK 安装/下载） | 已完成 | `d040f83` |
| M4 加固收尾 | 已完成 | 见下 |

相对本设计的实现偏差：

- **已回到框架原生写法（2026-09-23 晚，升级到 rux v2.1.0 后）**：上游修复了"404/405 不经过中间件链"与"`server` 包不回传实际监听地址"两个问题，eget 因此撤掉了两处规避——现在用 `r.NotFound` / `r.NotAllowed`（未认证的未知路径实测返回 401 且带安全头）+ `server.New` / `SetListener` / `ServeListener`（`MountHealthChecks` 提供 `/healthz`、`/readyz`，`PreShutdown` 里撤销任务）。
  v2.0.2 时期的问题记录与规避过程见 `docs/superpowers/notes/2026-09-23-rux-v2-feedback.md`。
- **配置编辑只支持 set**：清空某个键用空字符串表达，暂不提供键删除。
- **M4 已完成**：
  - 认证失败限速（每地址 20 次/分钟 → 429）；
  - `[web]` 配置节（host/read_only/allow_mutations/auto_open/cache_root/no_cache_index；token 与端口不入配置）；
  - cache 文件服务改用 `os.Root`，并从同一个 fd 提供内容（`/download` 用 `ServeContent`，`/files` 用 `FileServerFS`）：消除 `EvalSymlinks` 与 `ServeFile` 之间的 symlink TOCTOU，同时保留 `/files` 的目录列表能力；
  - GUI 安装器的无人值守参数：MSI `msiexec /i <path> /qn /norestart`，CLI 侧 `eget install --silent`，控制台侧安装请求的 `silent` 字段（EXE 安装器的参数各家不同，交给 `install_args`）。

## 开放问题

1. `tasks.json` 的保留策略（默认最近 200 条任务、每任务 50 条日志摘要）是否合适，是否需要可配置？
2. 配置编辑是否需要"保存前自动备份 `eget.toml`"（如 `eget.toml.bak`）？
3. 前端是否纳入 CodeQL / Dependabot（`.github/dependabot.yml` 目前只有 gomod 与 github-actions）？
