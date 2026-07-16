# Cache Server 一期全离线安装实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让已知工具在 cache server 已预热 `api-cache` 与 `pkg-cache` 时，通过 `cache_mirror.fallback=false` 完成 metadata、资产下载和安装全链路离线运行，且客户端不发起任何 provider 外网请求。

**Architecture:** 在 `client.GetWithOptions` 的本地 API cache miss 与 provider 请求之间插入 metadata mirror 查询；mirror 命中后将响应落到本地 `api-cache` 并复用原有 response/finder 链路，miss 或错误则严格按 `fallback` 决定回源或失败。资产层继续使用现有 `cachemirror.DownloadToFile` 与 `/download/{path-key}`，服务端不新增路由、manifest schema、catalog 或上传接口。

**Tech Stack:** Go、`net/http/httptest`、现有 `internal/client`/`internal/cachemirror`/`internal/install`、`github.com/gookit/goutil/x/assert`、GitNexus。

---

## 范围边界

本计划只实现设计文档中的一期：客户端已知道 repo、package alias 或 pkg-template target，server 已由联网机器预热对应 provider metadata 与资产。

- 不实现工具搜索、可安装列表、package/version/platform catalog。
- 不新增 cache server 路由、反向代理、上传接口或认证配置。
- 不改变 provider metadata 判断范围，继续复用 `isProviderMetadataRequest`。
- 不改变现有资产 mirror 和 SDK archive mirror 的下载实现。
- 不让 metadata mirror 依赖 `api_cache.enable`；该配置只控制正常本地 API cache 策略。
- 不为 `GetWithOptions` 引入新的 context 参数；一期在小 helper 中使用 `context.Background()`。

## 文件结构

- 修改 `internal/client/network.go`：在公共 metadata 请求入口增加 cache mirror 选项和短路流程。
- 修改 `internal/client/api_cache.go`：允许 metadata mirror 独立解析 API cache path，并封装 path-key 下载。
- 修改 `internal/client/api_cache_test.go`：覆盖本地命中、mirror 命中、回源、严格离线和非 metadata URL。
- 修改 `internal/install/network.go`：将 `install.Options.CacheMirror` 传给 `client.Options`。
- 修改 `internal/install/network_test.go`：验证 install 到 client 的选项映射。
- 修改 `internal/app/sdk.go`：将相同 cache mirror 配置传给 SDK metadata client。
- 修改 `internal/app/sdk_test.go`：验证 SDK metadata 选项与独立 APICacheDir。
- 新建 `internal/install/runner_offline_cache_test.go`：组合 GitHub metadata mirror、pkg mirror 与实际解压安装，断言 origin 请求数为零。
- 修改 `internal/app/cache/server_test.go`：证明 server 的现有 path-key handler 可读取 `api-cache/...`。
- 修改 `docs/config.md`、`docs/config.zh-CN.md`：记录预热、严格离线配置、边界和错误语义。
- 修改 `AGENTS.md`：实施过程中维护正在进行工作，全部完成后移除该条目。

## 实施前强制检查

`client.GetWithOptions` 是 GitHub、GitLab、Gitea、SourceForge 与 SDK metadata 的公共网络入口，图谱显示其传播风险为 HIGH/CRITICAL。开始编辑前必须执行并向用户报告 blast radius；如果结果仍为 HIGH 或 CRITICAL，先明确警告再继续：

```powershell
npx gitnexus impact GetWithOptions --direction upstream --depth 3 --repo eget
npx gitnexus impact resolvedAPICachePath --direction upstream --depth 3 --repo eget
npx gitnexus impact ClientOptions --direction upstream --depth 3 --repo eget
npx gitnexus impact sdkClientOptionsFromConfig --direction upstream --depth 3 --repo eget
```

每个阶段提交前都运行：

```powershell
npx gitnexus detect-changes --repo eget --scope all
```

只提交下述预期文件。如果 GitNexus 报索引过期，更新索引后先检查工作树；工具生成的 `AGENTS.md`、`CLAUDE.md` 等无关变化不得进入提交。

### Task 1: Metadata mirror 核心与严格回源语义

**Files:**
- Modify: `internal/client/network.go`
- Modify: `internal/client/api_cache.go`
- Test: `internal/client/api_cache_test.go`

- [x] **Step 1: 为 API cache path 激活条件写 RED 测试**

在 `internal/client/api_cache_test.go` 增加表驱动测试，固定 GitHub release URL，分别验证普通 API cache、仅 mirror、两者关闭和非 metadata URL：

```go
func TestResolvedAPICachePathSupportsMetadataMirror(t *testing.T) {
	parsed, err := url.Parse("https://api.github.com/repos/owner/tool/releases/latest")
	assert.NoErr(t, err)
	cacheDir := filepath.Join(t.TempDir(), "api-cache")

	for _, tc := range []struct {
		name string
		opts Options
		want bool
	}{
		{name: "api cache", opts: Options{APICacheEnabled: true, APICacheDir: cacheDir}, want: true},
		{name: "mirror only", opts: Options{APICacheDir: cacheDir, CacheMirror: cachemirror.Options{Enable: true, URL: "http://mirror"}}, want: true},
		{name: "disabled", opts: Options{APICacheDir: cacheDir}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, got := resolvedAPICachePath(tc.opts, parsed.String(), parsed)
			assert.Eq(t, tc.want, got)
		})
	}
}
```

另加一个 download URL 用例，断言即使 mirror 开启也返回 `false`。

- [x] **Step 2: 运行测试确认 RED**

Run: `go test ./internal/client -run TestResolvedAPICachePathSupportsMetadataMirror`

Expected: FAIL，`mirror only` 用例得到 `false`，且 `client.Options` 尚无 `CacheMirror` 字段。

- [x] **Step 3: 最小扩展 client 选项与 API cache path 条件**

在 `internal/client/network.go` 引入已有包并给 `Options` 添加一个字段：

```go
import "github.com/inherelab/eget/internal/cachemirror"

type Options struct {
	CacheMirror cachemirror.Options
}
```

这里只展示新增字段；其余现有字段原样保留，不重排、不重命名。

在 `internal/client/api_cache.go` 只放宽激活条件，不改变 provider 范围和目录规则：

```go
func resolvedAPICachePath(opts Options, rawURL string, parsed *url.URL) (string, bool) {
	if (!opts.APICacheEnabled && !opts.CacheMirror.Active()) || !isProviderMetadataRequest(parsed) {
		return "", false
	}
	cacheDir := opts.APICacheDir
	if cacheDir == "" {
		return "", false
	}
	expanded, err := util.Expand(cacheDir)
	if err != nil {
		verbosef("api cache expand error: %v", err)
		return "", false
	}
	return APICacheFilePath(expanded, rawURL), true
}
```

- [x] **Step 4: 写 metadata mirror 命中、持久化和本地优先 RED 测试**

新增 `TestGetWithOptionsUsesMetadataMirrorBeforeOrigin`：

1. 用 `APICacheFilePath(apiCacheDir, rawURL)` 得到客户端目标路径。
2. 用 `cachemirror.RelPath(filepath.Dir(apiCacheDir), cachePath)` 得到带 `api-cache/` 前缀的相对路径。
3. 用 `cachemirror.KeyForRelPath(rel)` 得到 server path-key。
4. `httptest.Server` 只在 `/download/<key>` 返回 release JSON。
5. `SetHTTPDoForTest` 的 origin stub 增加计数并返回错误。
6. `APICacheEnabled=false`、mirror enable、fallback=false 调用 `GetWithOptions`。
7. 断言 response body、mirror 请求路径、本地缓存内容和 origin 计数 0。
8. 关闭 mirror server 后再次请求，开启 `APICacheEnabled=true`，断言复用刚写入的本地 cache。

release JSON 使用有效的最小 provider body：

```go
body := `{"tag_name":"v1.2.3","prerelease":false,"assets":[{"name":"tool-windows-amd64.zip","browser_download_url":"https://origin.invalid/tool-windows-amd64.zip"}]}`
```

另增 `TestGetWithOptionsPrefersLocalAPICacheToMetadataMirror`，预先写入本地 cache，mirror handler 和 origin stub 被调用即失败，断言本地内容直接返回。

- [x] **Step 5: 运行命中测试确认 RED**

Run: `go test ./internal/client -run 'TestGetWithOptionsUsesMetadataMirrorBeforeOrigin|TestGetWithOptionsPrefersLocalAPICacheToMetadataMirror'`

Expected: FAIL；当前本地 miss 后直接进入 origin，mirror handler 未收到请求。

- [x] **Step 6: 实现最小 metadata mirror helper**

在 `internal/client/api_cache.go` 增加 helper。cache root 必须是 `filepath.Dir(apiCacheDir)`，从而 key 对应 server 扫描到的 `api-cache/<filename>`，不能只 hash basename：

```go
func tryAPICacheMirror(cachePath string, opts cachemirror.Options) (bool, error) {
	if !opts.Active() {
		return false, nil
	}
	apiCacheDir := filepath.Dir(cachePath)
	cacheRoot := filepath.Dir(apiCacheDir)
	rel, err := cachemirror.RelPath(cacheRoot, cachePath)
	if err != nil {
		return false, err
	}
	key := cachemirror.KeyForRelPath(rel)
	result, err := cachemirror.DownloadToFile(context.Background(), opts, key, cachePath)
	if err != nil {
		return false, fmt.Errorf("cache mirror metadata %s: %w", rel, err)
	}
	if !result.Hit && !opts.Fallback {
		return false, fmt.Errorf("cache mirror metadata miss: %s (fallback disabled)", rel)
	}
	return result.Hit, nil
}
```

在 `GetWithOptions` 中保留现有本地 cache 优先级；本地 miss 后、`requestAttemptURLs` 前调用 helper。命中时使用 `loadAPICacheResponse(cachePath, 0)`，确保刚下载文件不受用户的普通 cache TTL 排除，并直接返回 response：

```go
if useAPICache && opts.CacheMirror.Active() {
	hit, mirrorErr := tryAPICacheMirror(cachePath, opts.CacheMirror)
	if mirrorErr != nil {
		if !opts.CacheMirror.Fallback {
			return nil, mirrorErr
		}
		verbosef("cache mirror metadata fallback: %v", mirrorErr)
	} else if hit {
		resp, ok, err := loadAPICacheResponse(cachePath, 0)
		if err != nil {
			return nil, fmt.Errorf("load mirrored api cache: %w", err)
		}
		if !ok {
			return nil, fmt.Errorf("mirrored api cache is unavailable: %s", cachePath)
		}
		return resp, nil
	}
}
```

`cachePath` 已由 `resolvedAPICachePath` 展开后生成。helper 固定从该路径的父目录得到 `api-cache`，再从上一级得到 cache root；不得改为使用可能仍含 `~` 的 `opts.APICacheDir`，也不得只对 basename 计算 key。

- [x] **Step 7: 写 fallback 与请求范围 RED 测试**

增加以下用例，均通过 origin 计数证明真实行为：

- `TestGetWithOptionsMetadataMirrorMissFallsBackToOrigin`：mirror 404、`fallback=true`，origin 返回 200，响应被写入本地 cache，origin 次数 1。
- `TestGetWithOptionsMetadataMirrorMissFailsWithoutOrigin`：mirror 404、`fallback=false`，错误包含 `cache mirror metadata miss`、`api-cache/`、`fallback disabled`，origin 次数 0。
- `TestGetWithOptionsMetadataMirrorErrorFailsWithoutOrigin`：mirror 500、`fallback=false`，错误保留 server 状态语义，origin 次数 0。
- `TestGetWithOptionsDoesNotUseMetadataMirrorForDownload`：普通 asset URL 不请求 metadata mirror，继续走原有请求流程。

- [x] **Step 8: 运行 client 聚焦测试并修正最小实现**

Run: `go test ./internal/client -run 'TestResolvedAPICachePathSupportsMetadataMirror|TestGetWithOptions.*MetadataMirror|TestGetWithOptionsPrefersLocalAPICacheToMetadataMirror|TestGetWithOptionsDoesNotUseAPICacheForDownloads'`

Expected: PASS；严格模式所有用例 origin 次数为 0，fallback 模式 origin 次数恰为 1。

- [x] **Step 9: 回归整个 client 包并提交阶段 1**

Run: `go test ./internal/client`

Expected: PASS。

Run: `npx gitnexus detect-changes --repo eget --scope all`

Expected: 只影响 client metadata/API cache 流程及新增测试，无非 provider 或 server 路由变化。

```powershell
git add internal/client/network.go internal/client/api_cache.go internal/client/api_cache_test.go
git commit -m "feat: fetch provider metadata from cache mirror"
```

### Task 2: Install 与 SDK metadata 选项贯通

**Files:**
- Modify: `internal/install/network.go`
- Test: `internal/install/network_test.go`
- Modify: `internal/app/sdk.go`
- Test: `internal/app/sdk_test.go`

- [ ] **Step 1: 写 install client 映射 RED 测试**

在 `internal/install/network_test.go` 增加：

```go
func TestClientOptionsCopiesCacheMirror(t *testing.T) {
	want := cachemirror.Options{
		Enable: true, URL: "http://mirror.local:8686", Timeout: 4 * time.Second, Fallback: false,
	}
	got := ClientOptions(Options{CacheMirror: want})
	assert.Eq(t, want, got.CacheMirror)
}
```

- [ ] **Step 2: 运行 install 映射测试确认 RED**

Run: `go test ./internal/install -run TestClientOptionsCopiesCacheMirror`

Expected: FAIL；`ClientOptions` 返回的 `client.Options.CacheMirror` 是零值。

- [ ] **Step 3: 最小补齐 install 映射**

在 `ClientOptions` 的现有 struct literal 中加入：

```go
CacheMirror: opts.CacheMirror,
```

不要修改 `resolveInstallOptionsWithConfig`；该入口已经把 `CacheMirrorOptionsFromConfig(cfg)` 写入 `install.Options`。

- [ ] **Step 4: 扩展 SDK 配置 RED 测试**

在现有 `TestNewDefaultSDKServiceUsesConfigPathsAndNetworkOptions` 中补充 client metadata mirror 断言：

```go
assert.True(t, service.ClientOpts.CacheMirror.Enable)
assert.Eq(t, "http://mirror.local:8686", service.ClientOpts.CacheMirror.URL)
assert.Eq(t, 4*time.Second, service.ClientOpts.CacheMirror.Timeout)
assert.False(t, service.ClientOpts.CacheMirror.Fallback)
```

再新增 `TestSDKClientOptionsEnablesAPICachePathForMirrorOnly`，设置 `api_cache.enable=false`、cache mirror enable，断言 `ClientOpts.APICacheEnabled` 仍为 false，但 `ClientOpts.APICacheDir` 为 `<cacheDir>/api-cache` 且 mirror active。

- [ ] **Step 5: 运行 SDK 测试确认 RED**

Run: `go test ./internal/app -run 'TestNewDefaultSDKServiceUsesConfigPathsAndNetworkOptions|TestSDKClientOptionsEnablesAPICachePathForMirrorOnly'`

Expected: FAIL；SDK `ClientOpts.CacheMirror` 仍为零值。

- [ ] **Step 6: 最小补齐 SDK metadata 配置映射**

在 `sdkClientOptionsFromConfig` 构造的 client options 中加入：

```go
opts.CacheMirror = CacheMirrorOptionsFromConfig(cfg)
```

保留 `NewDefaultSDKService` 当前用于 SDK archive 的 `CacheMirror` 字段，metadata 与 archive 两条链路共享同一份规范化配置，不新增 SDK 专用 mirror 实现。

- [ ] **Step 7: 验证选项贯通并提交阶段 2**

Run: `go test ./internal/install -run 'TestClientOptionsCopiesProxyExclude|TestClientOptionsCopiesCacheMirror'`

Run: `go test ./internal/app -run 'TestNewDefaultSDKServiceUsesConfigPathsAndNetworkOptions|TestSDKClientOptionsEnablesAPICachePathForMirrorOnly|TestInstallOptionsIncludeCacheMirrorConfig'`

Expected: 全部 PASS。

Run: `npx gitnexus detect-changes --repo eget --scope all`

Expected: 只影响 install 和 SDK 的 client option wiring，不改变配置 schema。

```powershell
git add internal/install/network.go internal/install/network_test.go internal/app/sdk.go internal/app/sdk_test.go
git commit -m "feat: pass cache mirror to metadata clients"
```

### Task 3: 已知 GitHub 工具完整离线安装回归

**Files:**
- Create: `internal/install/runner_offline_cache_test.go`

- [ ] **Step 1: 写完整离线安装 RED 测试**

新增 `TestRunnerInstallsKnownGitHubToolFullyOfflineFromCacheMirror`。测试只模拟 server 已公开的 `/download/{path-key}` 协议，不 import `internal/app/cache`，避免形成跨层依赖。

测试准备顺序必须与生产计算一致：

```go
cacheDir := t.TempDir()
apiCacheDir := filepath.Join(cacheDir, "api-cache")
metadataURL := "https://api.github.com/repos/owner/tool/releases/latest"
assetURL := "https://origin.invalid/tool-v1.2.3-windows-amd64.zip"

metadataPath := APICacheFilePath(apiCacheDir, metadataURL)
metadataRel, err := cachemirror.RelPath(cacheDir, metadataPath)
assert.NoErr(t, err)
metadataKey := cachemirror.KeyForRelPath(metadataRel)

assetPath := CacheFilePath(cacheDir, assetURL)
assetRel, err := cachemirror.RelPath(cacheDir, assetPath)
assert.NoErr(t, err)
assetKey := cachemirror.KeyForRelPath(assetRel)
```

用现有 `zipBytes` 生成包含 `tool.exe` 的归档。mirror handler 仅对两个精确 path 返回 metadata JSON 和 zip，其他请求 404：

```go
switch r.URL.Path {
case "/download/" + metadataKey:
	_, _ = io.WriteString(w, releaseJSON)
case "/download/" + assetKey:
	_, _ = w.Write(archive)
default:
	http.NotFound(w, r)
}
```

使用 `client.SetHTTPDoForTest` 替换 provider origin 请求，计数后返回 `errors.New("origin access forbidden")`。运行真实 runner：

```go
runner := NewRunner(NewDefaultService(nil, nil))
runner.Stdout, runner.Stderr = io.Discard, io.Discard
result, err := runner.Run("owner/tool", Options{
	CacheDir: cacheDir,
	Output: filepath.Join(t.TempDir(), "bin"),
	System: "windows/amd64",
	CacheMirror: cachemirror.Options{Enable: true, URL: mirror.URL, Fallback: false},
})
```

断言：

- `err == nil`。
- origin 计数为 0。
- metadataKey 与 assetKey 均恰好请求一次。
- 本地 `metadataPath` 和 `assetPath` 已写入。
- `result.ExtractedFiles` 包含安装目录下的 `tool.exe`，内容与归档一致。

- [ ] **Step 2: 运行完整链路测试确认 RED**

Run: `go test ./internal/install -run TestRunnerInstallsKnownGitHubToolFullyOfflineFromCacheMirror -v`

Expected: FAIL；实施 Task 1 前会尝试 origin，实施 Task 1/2 后应直接 GREEN。如果此时直接 PASS，仍检查 origin 计数和两个 key 的精确请求断言，不能仅以文件存在作为成功标准。

- [ ] **Step 3: 只修正测试暴露的组合缺口**

如果失败来自 production wiring，限制修复范围为 Task 1/2 已涉及的 metadata mirror 和 option mapping；不得在本任务新增 finder、server 路由或测试专用生产 API。若 fixture 的字段不足，依据 `GitHubClient.LatestRelease`/asset detector 当前读取字段补全 JSON，而不是绕过真实 finder。

- [ ] **Step 4: 验证完整链路与既有资产 mirror 回归**

Run: `go test ./internal/install -run 'TestRunnerInstallsKnownGitHubToolFullyOfflineFromCacheMirror|TestDownloadBodyUsesCacheMirrorBeforeOrigin|TestDownloadBodyFallsBackWhenCacheMirrorMisses|TestDownloadBodyErrorsWhenCacheMirrorFallbackDisabled'`

Expected: 全部 PASS；新测试证明 metadata 和 asset 均命中 mirror，旧测试证明资产 fallback 语义未回归。

- [ ] **Step 5: 检查影响并提交阶段 3**

Run: `npx gitnexus detect-changes --repo eget --scope all`

Expected: 新增测试覆盖 install execution flow；若有生产文件变化，只能是前两阶段遗漏的 option/mirror wiring，并应在提交说明中指出原因。

```powershell
git add internal/install/runner_offline_cache_test.go
git commit -m "test: cover fully offline cached installation"
```

### Task 4: Server API cache path-key 回归与用户文档

**Files:**
- Modify: `internal/app/cache/server_test.go`
- Modify: `docs/config.md`
- Modify: `docs/config.zh-CN.md`

- [ ] **Step 1: 写 server API cache path-key 回归测试**

在 `internal/app/cache/server_test.go` 增加 `TestCacheServerDownloadPathKeyServesAPICache`，显式创建 `api-cache/github-repos-owner-tool-releases-latest.json`，使用完整相对路径计算 key：

```go
func TestCacheServerDownloadPathKeyServesAPICache(t *testing.T) {
	cacheDir := t.TempDir()
	rel := filepath.ToSlash(filepath.Join("api-cache", "github-repos-owner-tool-releases-latest.json"))
	file := filepath.Join(cacheDir, filepath.FromSlash(rel))
	assert.NoErr(t, os.MkdirAll(filepath.Dir(file), 0o755))
	assert.NoErr(t, os.WriteFile(file, []byte(`{"tag_name":"v1.2.3"}`), 0o644))

	req := httptest.NewRequest(http.MethodGet, "/download/"+cachemirror.KeyForRelPath(rel), nil)
	rec := httptest.NewRecorder()
	NewHandler(Service{}, cacheDir, ServeOptions{}).ServeHTTP(rec, req)
	assert.Eq(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"tag_name":"v1.2.3"`)
}
```

这里使用当前 `NewHandler(Service{}, cacheDir, ServeOptions{})` 签名；测试语义必须保留完整 `api-cache/...` key，不能只复用已有 pkg-cache fixture。

- [ ] **Step 2: 运行 server 回归**

Run: `go test ./internal/app/cache -run 'TestCacheServerDownloadPathKey|TestCacheServerDownloadPathKeyServesAPICache'`

Expected: PASS。现有 server 已扫描 `KindAPI`，正常情况下无需修改 production server；若失败，先确认测试构造是否符合现有 root scope，再决定是否需要最小生产修复。

- [ ] **Step 3: 更新中英文配置文档**

在 `cache_mirror` 配置章节加入完整操作说明，内容必须覆盖：

```toml
[cache_mirror]
enable = true
url = "http://192.168.1.10:8686"
fallback = false
```

中文文档明确：

1. 联网机器先正常查询/安装已知 target，预热 `api-cache` 与 `pkg-cache`，再启动 `eget cache serve`。
2. 离线客户端配置相同 cache server 且设 `fallback=false`，再运行已知 target 的 install/update。
3. `fallback=false` 同时禁止 metadata 和 asset 回源；metadata miss 与 asset miss 会给出不同错误。
4. `api_cache.enable=false` 不会禁用 metadata mirror；mirror 命中的 metadata 仍暂存到本地 `api-cache` 供解析和后续复用。
5. 一期不支持从 server 搜索或列举工具；二期 catalog 不在当前版本范围。

英文文档表达相同语义，不增加新的配置字段或未经实现的命令。

- [ ] **Step 4: 验证 server 与文档并提交阶段 4**

Run: `go test ./internal/app/cache`

Expected: PASS。

Run: `rg -n 'fallback = false|api-cache|known target|已知.*工具|catalog|目录' docs/config.md docs/config.zh-CN.md`

Expected: 两份文档均包含严格离线、预热、已知 target 和一期边界。

Run: `npx gitnexus detect-changes --repo eget --scope all`

Expected: server 仅增加测试覆盖，production handler 没有无依据的变化。

```powershell
git add internal/app/cache/server_test.go docs/config.md docs/config.zh-CN.md
git commit -m "docs: explain fully offline cache installation"
```

### Task 5: 一期总回归、跟踪状态与交付

**Files:**
- Modify: `AGENTS.md`
- Verify only: all files changed by Tasks 1-4

- [ ] **Step 1: 运行聚焦包回归**

Run: `go test ./internal/client ./internal/install ./internal/app ./internal/app/cache`

Expected: PASS，无 data race、fixture 外网依赖或平台特定失败。

- [ ] **Step 2: 运行 MVP 主链路全量回归**

Run: `go test ./...`

Expected: PASS。若失败，区分本次改动导致的回归与仓库已有失败；本次回归必须修复，已有失败要保留完整命令和首个错误证据，不能宣称全量通过。

- [ ] **Step 3: 执行最终 GitNexus 影响核查**

Run: `npx gitnexus detect-changes --repo eget --scope all`

Expected: 受影响流程仅包括 provider metadata cache、install/SDK option wiring、离线安装回归与文档；不得出现 catalog、search、upload、server proxy 等二期能力。

- [ ] **Step 4: 人工验收严格离线语义**

在可访问预热 cache server、但 provider 域名不可访问的测试环境运行：

```powershell
eget install --system windows/amd64 owner/tool
```

Expected: metadata 与 asset 均显示从 cache mirror 命中，工具安装成功；服务端缺 metadata 时错误包含 metadata miss，缺资产时错误为 asset cache mirror miss，且两种情况都无 provider 请求。

- [ ] **Step 5: 更新正在进行工作并提交交付状态**

全部验证通过后，从 `AGENTS.md` 的 `PROCESSING WORKS` 移除一期设计/计划条目；二期设计仍未进入实施，不新增“正在编码”的二期条目。

```powershell
git add AGENTS.md
git commit -m "docs: complete offline cache phase one"
```

- [ ] **Step 6: 输出交付摘要**

交付时列出每阶段 commit、聚焦测试、`go test ./...` 结果、严格离线人工验收结果，以及仍保留到二期的 package catalog 范围。不得以单元测试代替未执行的真实断网验收；若未具备 server 环境，明确标记该项尚未实测。
