# eget web 控制台

`eget web` 启动一个本地 HTTP 服务，在浏览器里查看并操作 eget：包列表、过期检查、更新、卸载、缓存管理、SDK 与外部管理器，同时对外提供缓存镜像协议。

> 本页对应当前已交付的范围（服务骨架 + 只读视图 + 缓存镜像接管）。任务引擎、更新/卸载/安装等写入接口见 `docs/superpowers/specs/2026-09-23-web-console-design.md` 的分阶段计划。

## 快速开始

```bash
eget web                    # http://127.0.0.1:8787，自动生成 token 并打印
eget web --open             # 启动后用浏览器打开（URL 带 token，首访后转为 cookie）
eget web -p 0               # 随机端口，实际地址在启动信息里
```

## 选项

| 选项 | 默认 | 说明 |
|---|---|---|
| `--host` | `127.0.0.1` | 监听地址；非 loopback 时必须显式提供 `--token` |
| `--port, -p` | `8787` | 监听端口，`0` 表示随机空闲端口 |
| `--token` | 自动生成 | 控制台与缓存镜像共用的 Bearer token |
| `--no-auth` | 关 | 关闭 token 校验（仅 loopback 允许） |
| `--read-only` | 关 | 只提供读取接口 |
| `--allow-mutations` | 关 | 非 loopback 监听时启用写入接口的必要开关 |
| `--open` | 关 | 启动后用默认浏览器打开控制台 |
| `--no-token-print` | 关 | 自动生成 token 时不打印 |
| `--cache-root` | `all` | 缓存镜像范围：`all`、`pkg`、`api`、`sdk`、`sdk-index` |
| `--no-cache-index` | 关 | 禁止缓存目录列表 |
| `--allow-host` | 空 | 追加允许的 Host 头名称，逗号分隔 |
| `--json-log` | 关 | 每请求输出一行 JSON 日志 |

写入接口的开关规则：**loopback 监听默认允许**（可用 `--read-only` 关闭）；**非 loopback 监听默认只读**，需要 `--allow-mutations` 显式开启。

## 认证

- 默认只绑 `127.0.0.1`；非 loopback 监听必须有显式 `--token`，否则拒绝启动（fail closed）。
- 自动生成的 token 为 6 字节随机值的十六进制（12 位字符，便于从终端重新输入），打印到 stderr，**不会写入配置文件或日志**。
- 三种携带方式：
  - `Authorization: Bearer <token>` —— 缓存镜像客户端与脚本；
  - `X-EGET-Token: <token>` —— 控制台前端；
  - `?token=<token>` —— 仅首次引导，服务端校验通过后会种 `HttpOnly` + `SameSite=Strict` cookie，token 随后离开地址栏。
- **首次打开控制台**：复制启动日志里的 `open:` 行（形如 `http://127.0.0.1:8787/?token=<token>`）——`--port 0` 时也能直接点开。若直接打开根地址，页面会给一个 token 输入框，粘贴后同样种下 cookie；API、静态资源与缓存镜像路径不会返回这个表单，仍是普通的 401。
- `/healthz`、`/readyz` 免鉴权。
- 浏览器图标（`/favicon.ico`、`/favicon.svg`、`/apple-touch-icon.png`、`/android-chrome-192x192.png`、`/android-chrome-512x512.png`、`/site.webmanifest`）同样免鉴权：图标要在未认证的 token 页就显示，而浏览器抓取 web manifest 时不携带凭据。

## API

当前可用端点（同源 AJAX，返回 JSON）：

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/healthz`、`/readyz` | 存活/就绪探针（免鉴权） |
| GET | `/api/overview` | 版本、配置路径、包统计、外部管理器可用性、缓存概览 |
| GET | `/api/packages` | 包列表，支持 `scope=all\|eget\|ext`、`manager=`、`q=`、`installed=true` |
| GET | `/api/packages/{name}` | 单包详情 |
| GET | `/api/outdated?scope=all\|eget\|ext&manager=<name>` | 过期检查（会访问网络），返回 `checked`、`items`、`failures`；`scope=ext` 时可用 `manager` 收窄到单个管理器 |
| GET | `/api/ext` | 外部管理器列表与各自的包数量 |
| GET | `/api/ext/{manager}/packages` | 单个管理器的包 |
| GET | `/api/cache`、`/api/cache/status` | 缓存文件列表与统计 |
| GET | `/api/config` | 配置路径与导出内容 |
| GET | `/api/query`、`/api/search` | 上游查询与仓库搜索 |
| GET | `/api/install/candidates?target=` | 候选资产列表（不下载），用于多候选时显式选择 |

写入端点（需要可写模式：loopback 监听默认可写，非 loopback 监听必须加 `--allow-mutations`）：

| 方法 | 路径 | 请求体 |
|---|---|---|
| POST | `/api/install` | `{target, version, asset, output, file, extractAll, downloadOnly, addToConfig, silent}` |
| POST | `/api/update` | `{targets: ["fd", "npm:typescript"], all: false}` |
| POST | `/api/uninstall` | `{target, purge}` |
| POST | `/api/ext/upgrade` | `{manager, names: []}` |
| POST | `/api/cache/clean` | `{mode: "older"\|"all"\|"keep-latest", days, kinds: [], dryRun}` |
| POST | `/api/sdk/install`、`/api/sdk/download` | `{targets: ["node@20"]}` |
| POST | `/api/config/validate` | `{set: {"global.proxy_url": "..."}}`（只校验，不落盘） |
| PUT | `/api/config` | `{set: {...}}`（应用修改） |

写入端点返回 `202` 与任务 id：

```json
{ "taskId": "t_1790168229_c22359", "kind": "install", "status": "queued" }
```

任务与实时日志：

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/tasks?limit=50` | 任务列表（含持久化历史） |
| GET | `/api/tasks/{id}` | 任务详情：状态、进度、日志、结果 |
| GET | `/api/tasks/{id}/events` | SSE 事件流（`status` / `progress` / `log` / `done`） |
| POST | `/api/tasks/{id}/cancel` | 取消排队中或运行中的任务 |

任务串行执行（同一时刻只跑一个），历史写入 `<config dir>/tasks.json`；进程重启后仍未完成的任务标记为 `interrupted`。

## 控制台的安全边界

- **不提供通用命令执行**：所有操作都是结构化参数，目标与包名经过白名单校验（拒绝 `-` 前缀、路径分隔符、`..`、控制字符），管理器名必须存在于外部管理器注册表。
- **默认不启动 GUI 安装器**：`eget web` 只下载与解压。带 `silent` 的安装请求会以无人值守方式运行 MSI（`msiexec /qn /norestart`）；EXE 安装器的静默参数各家不同，控制台不猜，请在包配置里用 `install_args` 指定。CLI 对应 `eget install --silent`。
- **不执行下载物**：`run-asset` 这类"下载后直接执行"的路径在控制台被禁用。
- **配置编辑受限**：只允许 CLI 已暴露的配置节；含 `token` 的键、`meta.*`、`web.*` 一律拒绝。
- 认证失败、Host 校验、CSRF、安全响应头与请求日志脱敏始终生效——包括未匹配路径（由通配路由处理，不走框架的 NotFound）。

示例：

```bash
curl -H "Authorization: Bearer $TOKEN" http://127.0.0.1:8787/api/overview
curl -H "Authorization: Bearer $TOKEN" "http://127.0.0.1:8787/api/packages?scope=ext&manager=npm"
```

## 缓存镜像协议（不可变）

以下端点是 **eget↔eget 的跨机器契约**：客户端的 URL 由 `internal/cachemirror` 拼出，因此路径与语义不会随版本变化。

| 端点 | 说明 |
|---|---|
| `GET /manifest.json` | 缓存清单：`schema`、`server`（name/version/base_url）、`files`（kind/path/path_key/url/size/mod_time） |
| `GET /download/path-md5:<key>` | 按内容路径键下载缓存文件 |
| `GET /files/<rel-path>` | 直接按相对路径下载；目录列表受 `--cache-root` 与 `--no-cache-index` 约束 |

客户端配置示例：

```toml
[cache_mirror]
enable = true
url = "http://192.168.1.10:8787"
timeout = 5
fallback = true
```

## 从 `eget cache serve` 迁移

`eget cache serve` 已删除，全部能力并入 `eget web`。

| 旧用法 | 新用法 |
|---|---|
| `eget cache serve` | `eget web` |
| `eget cache serve --host 0.0.0.0` | `eget web --host 0.0.0.0 --token <token>` |
| `eget cache serve --port 8686` | `eget web --port 8686` |
| `eget cache serve --root sdk` | `eget web --cache-root sdk` |
| `eget cache serve --no-index` | `eget web --no-cache-index` |
| `eget cache serve --token <token>` | `eget web --token <token>` |
| `eget cache serve --json-log` | `eget web --json-log` |
| 只读浏览页面 `GET /` | 控制台的缓存视图（前端页面） |

⚠️ 两个默认值不同，迁移时注意：

1. `cache serve` 默认监听 `0.0.0.0`，`eget web` 默认监听 `127.0.0.1`。原先对外提供镜像的机器需要显式 `--host 0.0.0.0 --token <token>`。
2. `cache serve` 默认端口 `8686`，`eget web` 默认 `8787`；沿用旧端口请加 `--port 8686`，或更新客户端 `[cache_mirror].url`。

`eget cache list | status | clean` 保持不变。

## 安全须知

- 非 loopback 监听必须提供 token，启动时会打印明文传输警告。
- **不提供内置 TLS**：对外部署请置于反向代理之后，由代理终止 TLS。
- 中间件已包含：Host 白名单（防 DNS rebinding）、变更请求的同源/自定义头校验（防 CSRF）、CSP 等安全响应头、认证失败限速、请求日志脱敏（不记录查询串，避免 token 入日志）。
- 缓存文件服务通过 `os.Root` 打开文件并从同一个 fd 提供内容（`/download` 用 `ServeContent`，`/files` 用 `FileServerFS`）：符号链接无法逃出缓存目录，也不存在"检查后替换"的 TOCTOU 窗口。
- **永久不在 web 上提供**：`self-update`、任意命令执行、执行下载得到的可执行文件、交互式启动 GUI 安装器。

## 默认值配置（`[web]`）

命令行参数优先；下列键只提供默认值：

```toml
[web]
host = "127.0.0.1"
read_only = false
allow_mutations = false
auto_open = false
cache_root = "all"
no_cache_index = false
```

- 端口只能通过 `--port` 指定（`0` 表示随机空闲端口），不放进配置。
- **token 永不写入配置文件**：只能通过 `--token` 传入，或由 `eget web` 每次随机生成。
- 非 loopback 监听时，`host = "0.0.0.0"` 这类配置仍需配合显式的 `--token`，否则拒绝启动。

## 前端资源

控制台前端是 Vite/React 应用，构建产物嵌入 Go 二进制：

```bash
make web-build      # 构建前端到 internal/app/web/dist
go build -o eget ./cmd/eget
```

未构建前端时二进制仍可编译运行，`GET /` 会返回一张说明页；API 与缓存镜像不受影响。

浏览器图标（favicon、PWA 图标、`site.webmanifest`）不随前端构建：它们直接嵌入二进制（`internal/app/web/icons`，由 `assets/logo/exports` 拷贝而来），因此未构建前端时 token 页与说明页也带图标。
