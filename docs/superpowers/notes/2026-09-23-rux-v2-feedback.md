# rux/v2 使用反馈

> 记录：2026-09-23，`github.com/gookit/rux/v2 v2.0.2`。来源：`eget web` 的实现与实测。按严重程度排序。

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
