# rux/v2 使用反馈

> 记录：2026-09-23，`github.com/gookit/rux/v2 v2.0.2`。来源：`eget web` 的实现与实测。按严重程度排序。
>
> **状态更新（2026-09-23 晚）：7 条已全部在 rux 侧处理，逐条结果见文末「处理结果」。**
> 其中第 2 条的描述在当天已过时（原问题当天已修），第 4、7 条在 v2.0.2 时其实已有文档。
> 当晚另有补充：核心包 `Listen` 也能回传真实地址、`Group` 值写法、容器里跑 race 的命令，
> 见文末「后续补充」。

## 1. NotFound / NotAllowed handler 绕过全局中间件链（安全影响）

**现象**：未匹配路由的请求由 `r.NotFound(h)`（以及方法不匹配时的 NotAllowed 处理）响应时，**不经过 `r.Use(...)` 注册的全局中间件**。

实测（eget web，全局链上有认证 → 安全头 → 日志）：

```
GET /some/unknown/path   无 token  → 200（应为 401）
响应头                    → 只有 Content-Type，没有 CSP / X-Frame-Options / X-Content-Type-Options
GET /api/nope            无 token  → 200（未认证即可触发）
```

**影响**：对"用中间件统一认证 / 安全头 / 审计"的服务，攻击者可控的路径恰好是唯一绕过全部保护的地方；审计日志也会缺失这些请求。

**建议**：让 NotFound / NotAllowed 也走全局链，或提供显式选项（如 `FallbackThroughMiddleware`）；至少在文档里显著提示该行为。

**eget 的规避**：不注册 `NotFound`，改为 `r.Any("/*path", handler)` 通配路由，在 handler 内区分 `/api/*`（JSON 404）、`/assets/*`（404）、其余（SPA fallback / 405），从而始终经过中间件链。

## 2. `server.Server.Run()` 不回传实际监听地址

**现象**：`s := server.New(); s.Addr = "127.0.0.1:0"; s.Run()` 内部完成 `ListenAndServe`，外部拿不到内核分配的端口；也没有 `OnListen` 钩子或 `Listener()` 访问器（`httpServer` 字段私有）。

> 注：本条描述已过时（当天已修）。`Listener()` / `WaitListening` 等现已可用，见文末「处理结果」。

**影响**：`--port 0`（随机端口）无法回显、无法拼出 URL 自动打开浏览器、测试中难以取得实际地址。

**建议**：`Run()` 返回 `(addr string, err error)`，或提供 `OnListen(func(net.Addr))`、`Listen() (net.Listener, error)` + `ServeListener(net.Listener) error`。

**eget 的规避**：自建 `net.Listen` + `http.Server`（继续使用 rux 的路由与中间件），自行补齐超时、`signal.NotifyContext` → `Shutdown`、`/healthz` + `/readyz`。

## 3. `server` 包默认 `WriteTimeout = 30s` 与 SSE 冲突

`server.New()` 的默认 `WriteTimeout` 会切断 SSE 长连接与大文件下载（README 有提示，但极易遗漏，且该值约束的是整个响应生命周期，心跳无法挽救）。

**建议**：为 SSE 场景提供预设或按路由豁免（例如 `sse.FriendlyServer()`），或让默认值注释直接指向 SSE。

## 4. `pkg/sse` 的默认行为需要读源码确认

`Stream` 默认发送 `: connected` 注释帧；`KeepaliveInterval` 默认不开启。功能完备，但接入方需要翻源码确认默认帧与心跳。建议 README 的 SSE 段落补两句。

## 5. 选项函数只有"开启"，没有"关闭"

`StrictLastSlash` / `HandleMethodNotAllowed` / `HandleFallbackRoute` / `InterceptAll` 都是 `func(*Router)` 形式的启用选项，对应字段私有、无 `WithoutXxx`。若将来默认值变化，使用方无法显式关闭。

**建议**：改为 `HandleMethodNotAllowed(bool)` 这类显式取值选项，或为每个开关补一个反向函数。

## 6. `Group` 闭包内需用外层 router 注册（易误用）

```go
r.Group("/api", func() {
    r.GET("/overview", h) // 必须用外层 r，而非组对象
})
```

gin 风格是 `g := r.Group(...)` + `g.GET(...)`。当前写法在 IDE 里容易误补全成 `g.GET` 而失败。

**建议**：提供返回 `*Group` 的形式，或在文档中给出对照说明。

## 7. v2 不再支持正则路由约束

`{id:\d+}` 已移除，校验必须写在 handler 或中间件里。对输入校验严格的服务（eget 对所有输入做白名单校验）需要在 handler 内自行实现。建议文档补一个"校验中间件"示例。

## 顺带记录：做得好、直接省事的地方

- `WrapHTTPHandler` / `WrapHTTPHandlerFunc`：标准 `http.Handler` 中间件与 rux 中间件互通，eget 的 Bearer 校验与请求日志中间件直接复用。
- `StaticFS` 挂 `embed.FS` 开箱即用，前端产物无需额外路由代码。
- `pkg/sse` 的 `Hub`（keyed push、多连接扇出、drop 计数、`OnDrop`）正好匹配"一个任务被多个标签页订阅"的模型。
- 路由表在首次 `ServeHTTP` 后冻结（`Frozen()`），配合"启动期注册全部路由"的约定可以避免运行期竞态。
- `Context` 的 `JSON` / `Param` / `Query` / `BindJSON` / `AbortWithStatus` 语义清晰，写 handler 很直接。

---

## 处理结果（rux 侧，2026-09-23）

### 1. 404/405 绕过全局中间件 → 已修（`80fa55f`）

`Freeze()` 现在把全局中间件链前置到 404/405 处理器链上，服务期间不再写 Router 字段
（顺带修掉了同一段代码在热路径上惰性写 `noRoute`/`noAllowed` 的数据竞争）。
`NotFound`/`NotAllowed` 与 `Use` 一样必须在首个请求前注册，否则 panic。

对 eget 的影响：`/*path` 通配兜底不再是绕开该问题的唯一办法，可以改回 `NotFound`
（全局链会跑）；未认证的未知路径现在会得到中间件给出的 401，而不是 404。

### 2. `Run()` 不回传监听地址 → 当天已修（`6dc915b`）

原文描述（内部 `ListenAndServe`、`httpServer` 私有、无 `OnListen`）已过时。现状：

- `Start()` 显式 `net.Listen`，解析后的地址回填 `Addr`/`Host`/`Port`
- 新增 `ListenAddr()` / `ListenPort()` / `LocalURL()` / `IsListening()` / `WaitListening(ctx)`
- `LocalURL()` 把 `:0`、`0.0.0.0:0`、`[::]:0` 映射成 `127.0.0.1`，端口未定时返回 `""`

```go
s := server.New(false)
s.SetAddr("127.0.0.1", 0) // 或 s.Addr = ":0"
go func() { _ = s.Run() }()
if err := s.WaitListening(ctx); err == nil {
    _ = sysutil.OpenURL(s.LocalURL()) // http://127.0.0.1:50447
}
```

另外新增（`0d88309`）`SetListener(ln)` / `ServeListener(ln)` / `Listener()`：
eget 里自建的 `net.Listen` + `http.Server` + 自补超时/信号/健康检查那套可以删掉，
把 listener 交给 `server.Server` 即可继续用它的 drain / readyz / hooks。

### 3. `WriteTimeout` 与 SSE 冲突 → 已修（`0d88309`）

`responseWriter` 增加 `Unwrap()`，`sse.Stream`/`StreamWith` 通过 `http.ResponseController`
清掉本次响应的写超时，默认 30s 不用改，心跳只需对付代理/NAT 空闲超时。

顺带修掉一个隐藏问题：rux 的 writer 自己实现了 `Flusher`，所以原来的
`c.Resp.(http.Flusher)` 检查永远通过，真正不支持 flush 的 writer 会在 `Flush()` 里 panic；
现在会干净地返回 `ErrFlushNotSupported`。

回归测试：`server/server_sse_test.go` 把 `WriteTimeout` 设为 300ms，流仍能活过 900ms
（旧代码在 300ms 处断流，实测 5 个事件后 EOF）。

### 4. `pkg/sse` 默认值 → v2.0.2 已有文档，本次只补了 godoc 交叉引用

README（`: connected` 默认发送、keepalive 默认关闭、WriteTimeout 说明）与 `sse.Options`
的 godoc 都已写明。第 3 条修好后这条只剩"发现性"问题，已在包 godoc 里指向 README 的 SSE 段。

### 5. 选项只有开启没有关闭 → 已加值语义（`1569547`）

```go
r := rux.New(
    rux.WithMethodNotAllowed(true),
    rux.WithEncodedPath(false), // 可以显式关闭
)
```

`WithStrictLastSlash` / `WithEncodedPath` / `WithMethodNotAllowed` / `WithFallbackRoute`
接受 bool；`StrictLastSlash`、`HandleMethodNotAllowed` 等旧名字保留为别名，现有代码不用改。

### 6. `Group` 闭包风格 → 已提供 `*Group` 值写法（`63bc573`）

闭包式 `Group` 保留且行为不变，另加了 gin 风格的值写法：

```go
api := r.NewGroup("/api", auth())
api.GET("/users", listUsers)              // GET /api/users，auth 先执行

admin := api.NewGroup("/admin", isAdmin()) // 前缀拼接、中间件叠加
admin.DELETE("/users/{id}", deleteUser)    // DELETE /api/admin/users/{id}

api.Use(rateLimit()) // 对之后注册的路由生效
```

- 提供 `Add` / `AddNamed` / `Any` / 10 个 verb / `Use` / `Prefix` / `Router` / 静态目录助手
- 在闭包组内 `NewGroup` 会继承闭包组的前缀与中间件，两种写法可以混用
- 冻结后注册照旧 panic；执行顺序 `全局 -> 组(外到内) -> 路由 -> handler`
- 顺带修掉一个真 bug：`StaticDir`/`StaticFS` 之前用未加前缀的 URL 做 `StripPrefix`，
  闭包组里挂静态目录会 404（实测旧代码 404 / 新代码 200）

### 7. 正则路由约束 → 已有文档，本次补上可复用的实现（`1569547`）

迁移文档 §2 早就写了移除与两种替代方案，但 option B 只给了中间件名字、没有实现。现在：

```go
r.GET("/users/{id}", showUser, handlers.ParamRegex("id", `\d+`)) // 不匹配 → 400
```

- 整值匹配（编译为 `^(?:pattern)$`），`\d+` 不会放过 `12abc`
- 正则只编译一次，写错在注册期 panic（与路由注册一致）
- 400 的响应体不回显参数值；需要 JSON 响应等自定义行为时仍建议自己写中间件

## 仍未处理（记录在案）

- `MaxParams = 16`、路由在首个请求后只读（冻结）等 v2 约束未变
- 核心包的 `Listen()` 已能回传真实地址（见下），但 `rux.Router` 仍没有优雅关闭/信号处理，
  需要这些时还是用 `server` 包

## 后续补充（2026-09-23 深夜）

### 核心包 `Listen` 也能回传真实地址（`474ce52`）

`Listen` / `ListenTLS` / `ListenUnix` 现在都是先绑定再服务，打印与报告的都是解析后的地址；
另加了 `Bind` / `ServeListener` / `Listener` / `ListenAddr` / `ListenPort`：

```go
ln, err := r.Bind("127.0.0.1:0") // 只绑定，不服务
if err != nil {
    log.Fatal(err)
}
fmt.Println(r.ListenAddr()) // 127.0.0.1:50447
r.ServeListener(ln)         // 阻塞；也可以传入自己创建的 listener
```

所以不用 `server` 包、只用核心路由的场景也能做 `--port 0` + 自动打开浏览器了。
顺手把 `Err()` 与监听状态放进了互斥锁（原来 `Err()` 与 `Listen` 的 goroutine 是竞争）。

### race 测试可以在容器里跑（本机无 CGO/gcc）

`golang:1.25` 容器默认连不上 proxy.golang.org，挂宿主模块缓存 + `GOPROXY=off` 即可：

```bash
docker run --rm -v <repo>:/src -v <GOMODCACHE>:/go/pkg/mod -w /src \
  -e GOPROXY=off -e GOSUMDB=off -e GOFLAGS=-mod=mod -e GOCACHE=/tmp/gocache \
  golang:1.25 go test -race -count=1 ./...
```

2026-09-23 全量跑通三次（改动前 / core Listen 改动后 / Group 改动后），无 race 报告。

### 本地跑 CodeQL（复现 code scanning 结论，不用等 CI）

用官方 Go-only bundle（108.8 MiB，sha256 校验）+ 已有 `golang:1.25` 造了镜像 `codeql-go:2.27.1`，
Dockerfile 与用法在 `inhere-tools/codeql/`。建库 + 单条查询约 1 分钟：

```bash
docker run --rm --entrypoint sh -v <repo>:/src -v <GOMODCACHE>:/go/pkg/mod \
  -v <out>:/out -w /src -e GOPROXY=off -e GOFLAGS=-mod=mod codeql-go:2.27.1 -c '
    codeql database create /tmp/db --language=go --source-root=. --overwrite &&
    codeql database analyze /tmp/db codeql/go-queries:Security/CWE-079/ReflectedXss.ql \
      --format=sarif-latest --output=/out/xss.sarif'
```

实测：HEAD 0 命中；旧提交（fixture 未改时）1 命中，正好是 CI 报的
`internal/core/response_writer.go:41`。注意：本地跑 CodeQL 会用 `-mod=mod` 改写
`_benchmarks/*/go.mod`、`go.sum`，跑完记得 `git checkout -- _benchmarks _examples`。

## eget 侧迁移记录（2026-09-23 晚，rux v2.1.0）

eget web 已升级到 v2.1.0 并撤掉了文中的两处规避：

- 通配路由 `r.Any("/*path", …)` → `r.NotFound` / `r.NotAllowed`。实测：未认证的未知路径返回 **401** 且带安全头，方法不匹配同样走中间件链；`/api/*` 仍返回 JSON 404，其余落到 SPA。
- 自建监听循环 → `server.New` + `SetListener` + `ServeListener`（`MountHealthChecks` 提供 `/healthz`（纯文本 `ok`）与 `/readyz`；关闭时 `ServeListener` 返回 `http.ErrServerClosed`，调用方需自行折叠为正常退出；任务取消放进 `PreShutdown`）。
- 选项改用取值式 `rux.WithMethodNotAllowed(true)`。

**新发现（已于同日处理）**：`Use` 曾要求必须在**任何**路由注册之前调用，否则 panic：

```
rux: Use must be called before any route registration (Q6)
```

而 `server.New()` 的 `MountHealthChecks()` 正是注册路由，所以顺序被硬性绑成 `Use(...)` → `MountHealthChecks()` → 业务路由，对调用方不直观。

上游已放宽（`01fb1e5`）：全局中间件链本来就是在 router 冻结（首个请求）时才合并进每条路由的，所以 `Use` 现在在**首个请求之前任意时刻**调用都生效，并覆盖此前注册的路由（回到 v1 的追溯语义）。只剩两条注意：首个请求之后 `Use` 仍 panic（frozen）；`Use` 永远是全局的，写在 `Group` 闭包里不会变成组级中间件。迁移时记录的那条顺序约束因此不再需要。

