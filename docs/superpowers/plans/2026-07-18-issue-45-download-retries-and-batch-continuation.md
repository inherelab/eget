# Issue #45 下载重试与批量安装容错实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 `install --all` 在单包失败后继续处理剩余包，并为 `install/update/download/add` 增加默认一次的文件下载请求重试控制。

**Architecture:** 批量安装沿用 update 批处理的“全部执行、成功结果过滤、失败聚合”模式；下载重试在 `client.GetWithOptions` 和 `client.requestWithOptions` 的单候选 URL 请求边界实现，前者用现有 provider 元数据判定禁止 API 重试。CLI 只负责合法值和 options 传播，配置结构和安装器保持不变。

**Tech Stack:** Go 标准库、gookit/gcli、`github.com/gookit/goutil/x/assert`、GitNexus。

## Global Constraints

- 严格生成两个实现提交；设计和计划文档随第一个提交纳入。
- `--retries N` 是每个下载候选 URL 的总尝试次数，默认 `1`，显式值必须 `N >= 1`。
- 选项覆盖 `install`、`update`、`download`、`add`，不增加配置文件字段。
- 只重试 `httpDo` 返回的传输错误；已有 HTTP 响应不重试，provider API 不重试。
- ghproxy 每个候选 URL 最多尝试 `N` 次，再进入现有 fallback。
- 不增加退避、延迟、策略接口、依赖或通用批处理框架。
- 每个生产 symbol 修改前运行 GitNexus upstream impact；HIGH/CRITICAL 必须先报告。
- 每次提交前运行 `npx gitnexus detect-changes --repo eget --scope all`。
- 新测试使用 `github.com/gookit/goutil/x/assert`；同一方法多用例用 `t.Run()`。

---

### Task 1: 修复 install --all 遇错中断并完成第一个提交

**Files:**

- Modify: `internal/app/install_all.go`
- Test: `internal/app/install_all_test.go`
- Test: `internal/app/install_test.go`
- Modify: `AGENTS.md`
- Add: `docs/superpowers/specs/2026-07-18-issue-45-download-retries-and-batch-continuation-design.md`
- Add: `docs/superpowers/plans/2026-07-18-issue-45-download-retries-and-batch-continuation.md`

**Interfaces:**

- Consumes: `Service.installResolvedTarget(...) (RunResult, error)`。
- Produces: `InstallAllPackages(install.Options) ([]InstallAllResult, error)` 返回全部成功结果和包级聚合错误。

- [x] **Step 1: 对待修改 symbols 做 upstream impact analysis**

Run:

```powershell
npx gitnexus impact InstallAllPackages --direction upstream --repo eget
npx gitnexus impact installAllPackagesConcurrent --direction upstream --repo eget
npx gitnexus impact sendFirstError --direction upstream --repo eget
npx gitnexus impact fakeBatchRunner.Run --direction upstream --repo eget --include-tests
```

Expected: `InstallAllPackages → handle → Main` 为 HIGH/CRITICAL；并发 helper 只有 `InstallAllPackages` 生产调用方。报告风险后把改动限定在 `internal/app/install_all.go`。

- [x] **Step 2: 写串行和并发失败继续执行测试**

在 `internal/app/install_all_test.go` 增加两个用例，复用现有 fake runner：

```go
func TestInstallAllPackagesContinuesAfterPackageFailure(t *testing.T) {
	runner := newFailingInstallAllRunner()
	svc := installAllFailureTestService(runner)

	results, err := svc.InstallAllPackages(install.Options{
		BatchConcurrency: 1, BatchConcurrencySet: true, Quiet: true,
	})

	assert.Err(t, err)
	assert.Contains(t, err.Error(), "1 install failed")
	assert.Eq(t, []string{"aeris", "rg", "uv"}, sortedStrings(runner.targets))
	assert.Eq(t, []string{"rg", "uv"}, installAllResultNames(results))
}

func TestInstallAllPackagesConcurrentContinuesAfterPackageFailure(t *testing.T) {
	runner := newFailingInstallAllRunner()
	svc := installAllFailureTestService(runner)

	results, err := svc.InstallAllPackages(install.Options{
		BatchConcurrency: 2, BatchConcurrencySet: true, Quiet: true,
	})

	assert.Err(t, err)
	assert.Contains(t, err.Error(), "1 install failed")
	assert.Eq(t, []string{"aeris", "rg", "uv"}, sortedStrings(runner.targets))
	assert.Eq(t, []string{"rg", "uv"}, installAllResultNames(results))
}

func newFailingInstallAllRunner() *fakeBatchRunner {
	return &fakeBatchRunner{
		results: map[string]RunResult{
			"BurntSushi/ripgrep": {Tool: "rg"},
			"astral-sh/uv":       {Tool: "uv"},
		},
		errs: map[string]error{"pkgforge/aeris": errors.New("EOF")},
	}
}

func installAllFailureTestService(runner *fakeBatchRunner) Service {
	return Service{Runner: runner, LoadConfig: func() (*cfgpkg.File, error) {
		cfg := cfgpkg.NewFile()
		cfg.Packages["aeris"] = cfgpkg.Section{Repo: util.StringPtr("pkgforge/aeris")}
		cfg.Packages["rg"] = cfgpkg.Section{Repo: util.StringPtr("BurntSushi/ripgrep")}
		cfg.Packages["uv"] = cfgpkg.Section{Repo: util.StringPtr("astral-sh/uv")}
		return cfg, nil
	}}
}

func installAllResultNames(results []InstallAllResult) []string {
	names := make([]string, len(results))
	for index, result := range results {
		names[index] = result.Name
	}
	return names
}
```

给 `fakeBatchRunner` 增加 `errs map[string]error`；在 `internal/app/install_test.go` 的现有 `Run` 方法记录完调用后返回目标错误：

```go
if err := f.errs[target]; err != nil {
	return RunResult{}, err
}
if result, ok := f.results[target]; ok {
	return result, nil
}
```

这些 helper 只构造现有 fake runner 所需数据和提取结果名称，不向生产代码增加测试入口。

- [x] **Step 3: 运行测试确认 RED**

Run:

```powershell
go test ./internal/app -run 'TestInstallAllPackages(Concurrent)?ContinuesAfterPackageFailure' -count=1
```

Expected: FAIL；串行只调用第一个包，并发在首错后未调用全部目标或返回空成功结果。

- [x] **Step 4: 最小修改串行路径**

在 `InstallAllPackages` 的包循环中收集包级错误而非立即返回；启动前验证仍立即返回：

```go
var failures []error
result, err := s.installResolvedTarget(runTarget, recordTarget, opts)
if err != nil {
	failures = append(failures, fmt.Errorf("%s: %w", name, err))
	continue
}
results = append(results, InstallAllResult{Name: name, Target: runTarget, Result: result})

if len(failures) > 0 {
	return results, fmt.Errorf("%d install failed: %w", len(failures), errors.Join(failures...))
}
```

`repo` 为空、单包 options 解析失败和单包并发参数失败也按相同方式记录并继续；配置文件加载和全局 batch 参数错误保持快速失败。

- [x] **Step 5: 最小修改并发路径**

删除 `context.WithCancel`、`errCh`、`sendLoop` 取消分支和 `sendFirstError`；按 index 保存结果、成功标记和失败：

```go
results := make([]InstallAllResult, len(names))
ok := make([]bool, len(names))
var failures []error
var mu sync.Mutex

// worker 中包级失败
mu.Lock()
failures = append(failures, fmt.Errorf("%s: %w", item.name, err))
mu.Unlock()

// worker 中成功
results[item.index] = InstallAllResult{Name: item.name, Target: runTarget, Result: result}
ok[item.index] = true

// wg.Wait 后按输入顺序过滤
out := make([]InstallAllResult, 0, len(names)-len(failures))
for index, result := range results {
	if ok[index] {
		out = append(out, result)
	}
}
if len(failures) > 0 {
	return out, fmt.Errorf("%d install failed: %w", len(failures), errors.Join(failures...))
}
return out, nil
```

删除修改产生的孤儿 `sendFirstError` 和不再使用的 `context` import。

- [x] **Step 6: 验证 Task 1**

Run:

```powershell
gofmt -w internal/app/install_all.go internal/app/install_all_test.go internal/app/install_test.go
go test ./internal/app -run 'TestInstallAllPackages' -count=1
go test ./internal/cli -run 'TestHandleInstallAll' -count=1
git diff --check
npx gitnexus detect-changes --repo eget --scope all
```

Expected: 全部 PASS；GitNexus 影响仅限 install-all 主流程和测试。

- [x] **Step 7: 更新进度并创建第一个提交**

把 Task 1 checkbox 更新为 `[x]`，保留 `AGENTS.md` 进行中链接并补充计划链接：

```markdown
- Issue #45 下载重试与批量安装容错：[设计](docs/superpowers/specs/2026-07-18-issue-45-download-retries-and-batch-continuation-design.md) · [实施计划](docs/superpowers/plans/2026-07-18-issue-45-download-retries-and-batch-continuation.md)
```

Run:

```powershell
git add internal/app/install_all.go internal/app/install_all_test.go internal/app/install_test.go AGENTS.md docs/superpowers/specs/2026-07-18-issue-45-download-retries-and-batch-continuation-design.md docs/superpowers/plans/2026-07-18-issue-45-download-retries-and-batch-continuation.md
git diff --cached --check
git commit -m "fix: continue install all after package failures (#45)"
```

Expected: 第一个提交只包含批量安装修复、测试和两份文档。

---

### Task 2: 贯通四个命令的 retries 选项

**Files:**

- Modify: `internal/cli/install_cmd.go`
- Modify: `internal/cli/update_cmd.go`
- Modify: `internal/cli/download_cmd.go`
- Modify: `internal/cli/add_cmd.go`
- Modify: `internal/cli/app.go`
- Modify: `internal/cli/options.go`
- Test: `internal/cli/app_install_test.go`
- Test: `internal/cli/install_handler_test.go`
- Modify: `internal/install/options.go`
- Modify: `internal/app/install_resolve.go`
- Test: `internal/app/install_config_test.go`
- Modify: `internal/app/update_options.go`
- Test: `internal/app/update_options_test.go`
- Modify: `internal/install/network.go`
- Test: `internal/install/runner_network_test.go`
- Modify: `internal/client/network.go`

**Interfaces:**

- Produces: `Retries int` 依次存在于四个 CLI options、`install.Options` 和 `client.Options`。
- Consumes later: `requestWithOptions(..., opts client.Options)` 使用 `opts.Retries`。

- [ ] **Step 1: 对 options 传播 symbols 做 upstream impact analysis**

对四个 command constructor、四个 options converter 和三个下游 copier 运行：

```powershell
npx gitnexus impact newInstallCmd --direction upstream --repo eget
npx gitnexus impact newUpdateCmd --direction upstream --repo eget
npx gitnexus impact newDownloadCmd --direction upstream --repo eget
npx gitnexus impact newAddCmd --direction upstream --repo eget
npx gitnexus impact installOptionsFromInstall --direction upstream --repo eget
npx gitnexus impact installOptionsFromUpdate --direction upstream --repo eget
npx gitnexus impact installOptionsFromDownload --direction upstream --repo eget
npx gitnexus impact installOptionsFromAdd --direction upstream --repo eget
npx gitnexus impact resolveInstallOptionsWithConfig --direction upstream --repo eget
npx gitnexus impact applyUpdateCLIOverrides --direction upstream --repo eget
npx gitnexus impact ClientOptions --direction upstream --repo eget
```

Expected: CLI constructors 和 install options resolver 为 HIGH/CRITICAL。逐项报告后只增加字段、绑定、校验和原样复制，不改变现有默认合并规则。

- [ ] **Step 2: 写 CLI 解析和传播失败测试**

在 `internal/cli/app_install_test.go` 用表驱动运行四个命令并捕获 handler snapshot：

```go
tests := []struct {
	name string
	args []string
}{
	{"install", []string{"install", "--retries", "3", "owner/repo"}},
	{"update", []string{"update", "--retries", "3", "owner/repo"}},
	{"download", []string{"download", "--retries", "3", "owner/repo"}},
	{"add", []string{"add", "--retries", "3", "owner/repo"}},
}
```

每个子测试断言对应 options 的 `Retries == 3`；另加 `--retries 0` 用例，断言错误包含 `retries must be at least 1` 且 handler 未调用。

在 `internal/cli/install_handler_test.go` 扩展现有 options propagation 测试：

```go
assert.Eq(t, 3, installOptionsFromInstall(&InstallOptions{Retries: 3}).Retries)
assert.Eq(t, 3, installOptionsFromUpdate(&UpdateOptions{Retries: 3}).Retries)
assert.Eq(t, 3, installOptionsFromDownload(&DownloadOptions{Retries: 3}).Retries)
assert.Eq(t, 3, installOptionsFromAdd(&AddOptions{Retries: 3}).Retries)
```

在 `internal/install/runner_network_test.go` 断言 `ClientOptions(Options{Retries: 3}).Retries == 3`。

在 `internal/app/install_config_test.go` 通过 `resolveInstallOptionsWithConfig` 断言 CLI `Retries: 3` 保留；在 `internal/app/update_options_test.go` 直接断言：

```go
got := applyUpdateCLIOverrides(install.Options{Retries: 1}, install.Options{Retries: 3})
assert.Eq(t, 3, got.Retries)
```

- [ ] **Step 3: 运行测试确认 RED**

Run:

```powershell
go test ./internal/cli -run 'TestMain_.*Retries|TestInstallOptionsFromCommandsPropagateRetries' -count=1
go test ./internal/install -run TestClientOptionsPropagatesRetries -count=1
go test ./internal/app -run 'Test.*Retries' -count=1
```

Expected: FAIL，提示 options 没有 `Retries` 字段或 `--retries` 未定义。

- [ ] **Step 4: 增加字段、CLI 绑定与校验**

四个 CLI options struct 增加 `Retries int`，command constructor 使用默认值 `1`：

```go
c.IntOpt(&opts.Retries, "retries", "", 1, "Download request attempts per URL")
```

四个 command reset snapshot 均恢复 `Retries: 1`。在 `internal/cli/options.go` 增加并复用：

```go
func validateRetries(value int) error {
	if value < 1 {
		return fmt.Errorf("retries must be at least 1")
	}
	return nil
}
```

每个 command Func 在调用 handler 前执行 `validateRetries(opts.Retries)`。在 `internal/cli/app.go` 的 install/update/download/add value flag whitelist 中加入 `"retries"`。

- [ ] **Step 5: 贯通 app/install/client options**

给 `install.Options` 和 `client.Options` 增加 `Retries int`。四个 `installOptionsFrom*`、`resolveInstallOptionsWithConfig` 和 `install.ClientOptions` 在 struct literal 中原样复制该字段：

```go
Retries: opts.Retries,
```

`applyUpdateCLIOverrides` 使用 CLI 正值覆盖 resolved base：

```go
if cli.Retries > 0 {
	base.Retries = cli.Retries
}
```

没有显式配置合并：CLI 默认已经是 `1`，项目不新增 `retries` 配置字段。

- [ ] **Step 6: 验证 options 传播**

Run:

```powershell
gofmt -w internal/cli/app.go internal/cli/install_cmd.go internal/cli/update_cmd.go internal/cli/download_cmd.go internal/cli/add_cmd.go internal/cli/options.go internal/cli/app_install_test.go internal/cli/install_handler_test.go internal/install/options.go internal/install/network.go internal/install/runner_network_test.go internal/app/install_resolve.go internal/app/install_config_test.go internal/app/update_options.go internal/app/update_options_test.go internal/client/network.go
go test ./internal/cli -run 'TestMain_.*Retries|TestInstallOptionsFromCommandsPropagateRetries' -count=1
go test ./internal/install -run TestClientOptionsPropagatesRetries -count=1
go test ./internal/app -run 'Test.*Retries' -count=1
```

Expected: PASS。Task 2 不提交，和 Task 3 的请求重试实现一起进入第二个提交。

---

### Task 3: 实现请求重试、完成第二个提交和全量验证

**Files:**

- Modify: `internal/client/http_client.go`
- Test: `internal/client/http_client_test.go`
- Modify: `internal/client/network.go`
- Test: `internal/client/network_test.go`
- Modify: `docs/superpowers/plans/2026-07-18-issue-45-download-retries-and-batch-continuation.md`
- Modify: `AGENTS.md`

**Interfaces:**

- Consumes: `client.Options.Retries int`，`requestAttemptURLs(...) []string`。
- Produces: 文件下载的每个候选 URL 最多调用 `httpDo` `Retries` 次；成功立即返回，耗尽后进入下一候选或返回最后错误；provider 元数据保持一次。

- [ ] **Step 1: 对两个下载请求入口做 upstream impact analysis**

Run:

```powershell
npx gitnexus impact requestWithOptions --direction upstream --repo eget
npx gitnexus impact GetWithOptions --direction upstream --repo eget
```

Expected: `downloadFileSingle`、Range chunks、probe、provider 请求和 SDK 下载为 CRITICAL/HIGH。先报告影响，再限定修改为两个现有 attempt URL 循环内的 transport retry，并用 provider metadata 判定保持 API 单次请求。

- [ ] **Step 2: 写请求重试失败测试**

在 `internal/client/http_client_test.go` 使用现有 `SetHTTPDoForTest`：

```go
func TestRequestWithOptionsRetriesTransportErrors(t *testing.T) {
	calls := 0
	restore := SetHTTPDoForTest(func(*http.Client, *http.Request) (*http.Response, error) {
		calls++
		if calls < 3 {
			return nil, io.ErrUnexpectedEOF
		}
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: http.NoBody}, nil
	})
	defer restore()

	resp, err := requestWithOptions(http.MethodGet, "https://example.test/tool", "", Options{Retries: 3})
	assert.NoErr(t, err)
	assert.Eq(t, http.StatusOK, resp.StatusCode)
	assert.Eq(t, 3, calls)
}

func TestRequestWithOptionsReturnsLastRetryError(t *testing.T) {
	calls := 0
	restore := SetHTTPDoForTest(func(*http.Client, *http.Request) (*http.Response, error) {
		calls++
		return nil, fmt.Errorf("attempt %d", calls)
	})
	defer restore()

	_, err := requestWithOptions(http.MethodGet, "https://example.test/tool", "", Options{Retries: 2})
	assert.Err(t, err)
	assert.Contains(t, err.Error(), "attempt 2")
	assert.Eq(t, 2, calls)
}
```

再用 ghproxy 两个候选地址和 `Retries: 2` 断言调用顺序为“候选 A 两次、候选 B 两次”，证明 retry 在 fallback 内层。

在 `internal/client/network_test.go` 增加普通文件 GET 重试和 provider API 不重试：

```go
func TestGetWithOptionsRetriesDownloadTransportErrors(t *testing.T) {
	calls := 0
	restore := SetHTTPDoForTest(func(*http.Client, *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return nil, io.ErrUnexpectedEOF
		}
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: http.NoBody}, nil
	})
	defer restore()

	resp, err := GetWithOptions("https://example.test/tool.exe", Options{Retries: 2})
	assert.NoErr(t, err)
	assert.Eq(t, 2, calls)
	_ = resp.Body.Close()
}

func TestGetWithOptionsDoesNotRetryProviderMetadata(t *testing.T) {
	calls := 0
	restore := SetHTTPDoForTest(func(*http.Client, *http.Request) (*http.Response, error) {
		calls++
		return nil, io.ErrUnexpectedEOF
	})
	defer restore()

	_, err := GetWithOptions("https://api.github.com/repos/owner/repo/releases/latest", Options{Retries: 3})
	assert.Err(t, err)
	assert.Eq(t, 1, calls)
}
```

保留现有默认/零值调用一次的测试，确保内部调用者不因零值死循环。

- [ ] **Step 3: 运行测试确认 RED**

Run:

```powershell
go test ./internal/client -run 'Test(RequestWithOptions.*Retry|GetWithOptions.*Retry)' -count=1
```

Expected: FAIL；第一次 transport error 直接返回，calls 小于期望值。

- [ ] **Step 4: 实现最小 transport retry**

增加一个文件内 helper，provider 元数据固定返回一次，其他请求把零值规范为一次：

```go
func requestRetries(opts Options, parsed *url.URL) int {
	if isProviderMetadataRequest(parsed) || opts.Retries < 1 {
		return 1
	}
	return opts.Retries
}
```

在 `GetWithOptions` 和 `requestWithOptions` 的候选 URL 循环内增加 retry 循环；每次循环重新创建 request。`requestWithOptions` 的 request 初始化完整保留如下：

```go
retries := requestRetries(opts, originalURL)
for i, attemptURL := range attempts {
	for retry := 1; retry <= retries; retry++ {
		req, err := http.NewRequest(method, attemptURL, nil)
		if err != nil {
			return nil, err
		}
		if rangeHeader != "" {
			req.Header.Set("Range", rangeHeader)
		}
		if err := setAuthHeader(req, opts.DisableSSL); err != nil {
			return nil, err
		}
		setDefaultHeaders(req, opts)
		printDownloadProxyNoticeForRequest(rawURL, req.URL, opts)
		resp, err := httpDo(client, req)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if retry < retries {
			verbosef("request retry %d/%d: %s", retry+1, retries, attemptURL)
			continue
		}
	}
	if i < len(attempts)-1 {
		verbosef("ghproxy fallback: switching to next host")
	}
}
return nil, lastErr
```

不对 `resp.StatusCode` 加判断，不加 sleep/backoff；`GetWithOptions` 的缓存、proxy notice 和响应保存逻辑保持原位。

- [ ] **Step 5: 运行聚焦、包级和全量测试**

Run:

```powershell
gofmt -w internal/client/http_client.go internal/client/http_client_test.go internal/client/network.go internal/client/network_test.go
go test ./internal/client -run 'Test(RequestWithOptions.*Retry|GetWithOptions.*Retry)' -count=1
go test ./internal/client ./internal/install ./internal/app ./internal/cli -count=1
go test -count=1 ./...
git diff --check
npx gitnexus detect-changes --repo eget --scope all
```

Expected: 全部 PASS；GitNexus 影响限定在下载请求、options 传播和 CLI 参数绑定。

- [ ] **Step 6: 更新进度并创建第二个提交**

把 Task 2、Task 3 checkbox 更新为 `[x]`，从 `AGENTS.md` 移除本项进行中事项，然后确认 staged 文件不包含无关改动：

```powershell
git add internal/cli/app.go internal/cli/install_cmd.go internal/cli/update_cmd.go internal/cli/download_cmd.go internal/cli/add_cmd.go internal/cli/options.go internal/cli/app_install_test.go internal/cli/install_handler_test.go internal/app/install_resolve.go internal/app/install_config_test.go internal/app/update_options.go internal/app/update_options_test.go internal/install/options.go internal/install/network.go internal/install/runner_network_test.go internal/client/network.go internal/client/network_test.go internal/client/http_client.go internal/client/http_client_test.go AGENTS.md docs/superpowers/plans/2026-07-18-issue-45-download-retries-and-batch-continuation.md
git diff --cached --check
git diff --cached --name-only
git commit -m "feat: add configurable download retries (fix #45)"
```

Expected: 第二个提交只包含 retries 选项、请求重试、测试和进度收尾。

- [ ] **Step 7: 提交后最终验证**

Run:

```powershell
go test -count=1 ./...
git status --short
git log -2 --oneline
```

Expected: 全量 PASS；工作树干净；最新两个提交依次为批量安装修复和 retries feature。不要 push。
