# Cache Clean 保留最新版本实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 `eget cache clean` 增加仅处理 `pkg-cache/` 的 `--keep-latest` 模式，安全保留每个可识别资产族的最新稳定版和必要预发布版。

**Architecture:** `internal/app/cache/keep_latest.go` 负责纯文件名解析、版本比较和候选选择；cache service 扫描一次并把 Preview 的内部候选快照交给 `ApplyClean`。CLI 只处理 flag、互斥校验、确认和输出，不复制清理逻辑。

**Tech Stack:** Go 标准库、现有 `github.com/gookit/goutil/x/assert`、gookit/cflag；不增加依赖。

## Global Constraints

- 严格只处理 `pkg-cache/` 下非 partial、非 symlink 的普通文件，不修改 `classifyEntry`。
- 无法可靠解析的文件保留；不请求 provider，不增加 sidecar、catalog、锁或文件 hash。
- 版本、平台、family、build metadata 和等价版本规则以设计规格为准。
- Preview 后只删除同一快照中身份、size、mtime 均未变化的候选。
- `--older=` 视为未设置；普通模式仍默认 `3d`。
- 每完成一个 Task 更新 checkbox 并提交；不得提交用户现有的 `internal/cli/install_cmd.go` 修改。
- 修改现有 Go symbol 前必须运行 GitNexus upstream impact；HIGH/CRITICAL 先向用户报告。

---

### Task 1: 实现版本与文件名解析

**Files:**

- Create: `internal/app/cache/keep_latest.go`
- Create: `internal/app/cache/keep_latest_test.go`

**Interfaces:**

- Produces: `parseAssetVersion(string) (assetVersion, bool)`
- Produces: `compareAssetVersion(assetVersion, assetVersion) int`
- Produces: `parseKeepLatestEntry(Entry) (parsedCacheAsset, bool)`

- [ ] **Step 1: 写版本文法与比较失败测试**

使用 `t.Run()` 和 `assert` 建表，精确覆盖：`1.2.3`、`v1.2.3`、`2026.7.17`、`beta.2 < beta.10`、`alpha < beta`、`beta < beta.0`、`Beta.1 < beta.1`、`preview.9 < rc.1`；拒绝 `1`、`1.02.3`、`beta.01`、第二个 `-`、`_`、首 identifier 为大小写任意的 `build`。

```go
func TestParseAssetVersion(t *testing.T) {
	tests := []struct{ in string; ok bool }{
		{"1.2.3", true}, {"v1.2.3", true}, {"2.0.0-beta.1", true},
		{"1", false}, {"1.02.3", false}, {"1.2.3-beta.01", false},
		{"1.2.3-beta-1", false}, {"1.2.3-build.1", false},
	}
	for _, tt := range tests { t.Run(tt.in, func(t *testing.T) {
		_, ok := parseAssetVersion(tt.in); assert.Eq(t, tt.ok, ok)
	}) }
}
```

- [ ] **Step 2: 运行失败测试**

Run: `go test ./internal/app/cache -run 'Test(Parse|Compare)AssetVersion'`

Expected: FAIL，解析函数尚未定义。

- [ ] **Step 3: 实现有限版本模型**

```go
type versionIdentifier struct { text string; number uint64; numeric bool }
type assetVersion struct { core []uint64; prerelease []versionIdentifier }
func (v assetVersion) stable() bool { return len(v.prerelease) == 0 }
func parseAssetVersion(raw string) (assetVersion, bool)
func compareAssetVersion(a, b assetVersion) int
func compareVersionCore(a, b []uint64) int
func comparePrerelease(a, b []versionIdentifier) int
```

仅用 `strconv`、`strings` 落实设计文法；不要引入 semver 包。

- [ ] **Step 4: 写真实缓存名解析失败测试**

表格必须覆盖：

```text
tool-v2.4.1-linux-amd64-2.4.1-<hash>.zip -> generic, unrecognized
gomi_Linux_x86_64-1.6.3-<hash>.tar.gz   -> gomi, 1.6.3
claude-2.1.160-linux-amd64-<hash>.bin   -> claude, 2.1.160
PowerShell-7.6.3-win-x64-7.6.3-<hash>.msi -> powershell, 7.6.3
cscli-windows-amd64-0.5.2-<hash>.exe    -> cscli, 0.5.2
foo-1.2.3-1.2.3-<hash>.zip              -> foo, 1.2.3
foo-1.2.3-2.0.0-<hash>.zip              -> foo, 2.0.0
```

同时拒绝大写 hash、未知平台 tuple、根目录、`misc/`、partial、symlink；断言 `windows-terminal != terminal`、`arm-tool != tool`，且 `go/fd/jq` 不受 denylist 影响。

- [ ] **Step 5: 实现 cache 名解析与 family 归一化**

```go
type parsedCacheAsset struct {
	entry Entry
	family, rawVer string
	version assetVersion
}
func parseKeepLatestEntry(entry Entry) (parsedCacheAsset, bool)
func splitCacheAssetName(name string) (rawName, version string, ok bool)
func normalizeAssetFamily(rawName, appendedVersion string) (string, bool)
func cacheAssetExt(name string) string
```

顺序固定：严格路径/类型检查 → 组合扩展名或 `path.Ext` → `[0-9a-f]{8}` → 有限平台 tuple 优先 → 最右侧合法版本起点 → family 归一化。只用精确 denylist，不拆 target triple。

- [ ] **Step 6: 验证并提交**

Run: `go test ./internal/app/cache -run 'Test(Parse|Compare|Normalize)'`

Expected: PASS。

```bash
git add internal/app/cache/keep_latest.go internal/app/cache/keep_latest_test.go docs/superpowers/plans/2026-07-18-cache-clean-keep-latest.md
git commit -m "feat: parse cache asset versions"
```

---

### Task 2: 选择每个 family 的保留版本

**Files:**

- Modify: `internal/app/cache/keep_latest.go`
- Modify: `internal/app/cache/keep_latest_test.go`

**Interfaces:**

- Consumes: `parseKeepLatestEntry`
- Produces: `selectKeepLatest([]Entry) keepLatestSelection`

- [ ] **Step 1: 写选择规则失败测试**

```go
type keepLatestSelection struct {
	Matched, Kept, Unrecognized []Entry
}
```

逐个 `t.Run()` 覆盖：`1.8/1.9/2.0-beta.1` 保留后两者；`2.0-beta.1/2.0` 只保留正式版；只有 prerelease 时保留最高者；最高版本所有平台/扩展名/hash 全保留；`2.0/2.0.0/v2.0.0` 全保留；unrecognized 不进入 Matched；qualifier family 不互相淘汰。

- [ ] **Step 2: 运行失败测试**

Run: `go test ./internal/app/cache -run TestSelectKeepLatest`

Expected: FAIL，`selectKeepLatest` 未定义。

- [ ] **Step 3: 实现两遍选择器**

第一遍按 family 找最高 stable/prerelease，第二遍归类。相等必须使用 `compareAssetVersion(...) == 0`，不能比较原始字符串；仅当 prerelease core 大于 stable core 时额外保留 prerelease。

- [ ] **Step 4: 验证并提交**

Run: `go test ./internal/app/cache`

Expected: PASS。

```bash
git add internal/app/cache/keep_latest.go internal/app/cache/keep_latest_test.go docs/superpowers/plans/2026-07-18-cache-clean-keep-latest.md
git commit -m "feat: select latest cache assets"
```

---

### Task 3: 接入 Preview 快照与安全删除

**Files:**

- Modify: `internal/app/cache/model.go`
- Modify: `internal/app/cache/service.go`
- Modify: `internal/app/cache/cache_test.go`

**Interfaces:**

- Consumes: `selectKeepLatest`
- Produces: `func (s Service) ApplyClean(preview CleanResult) (CleanResult, error)`
- Preserves: `Clean(cacheDir, opts)` 直接调用语义。

- [ ] **Step 1: 对 `Entry/CleanOptions/CleanResult/Scan/Clean/PreviewClean/clean` 做 GitNexus impact**

记录调用方并向用户报告 HIGH/CRITICAL；不得修改 `classifyEntry`。

- [ ] **Step 2: 写 service 失败测试**

覆盖严格 `pkg-cache/` 范围、根目录/`misc/`/partial/symlink 不计数；Preview 后新增文件不删除；删除重建同路径、size 变化或 mtime 变化时保留并返回 `changed since preview`；普通 `Clean(...Older...)` 仍工作。

```go
preview, err := service.PreviewClean(cacheDir, CleanOptions{KeepLatest: true})
assert.NoErr(t, err)
result, err := service.ApplyClean(preview)
assert.NoErr(t, err)
```

- [ ] **Step 3: 扩展模型**

```go
type cleanCandidate struct {
	path, relPath string
	size int64
	modTime time.Time
	fileInfo os.FileInfo
}
```

`Entry` 增加 `IsSymlink bool` 和未导出的 `fileInfo os.FileInfo`；`CleanOptions` 增加 `KeepLatest bool`；`CleanResult` 增加带 JSON tag 的 `KeptLatestFiles/UnrecognizedFiles`、未导出的 `snapshot []cleanCandidate` 和 `prepared bool`。`Scan` 只填充新字段，不改分类和返回范围。

- [ ] **Step 4: 拆分 Preview 与 Apply**

```go
func (s Service) PreviewClean(cacheDir string, opts CleanOptions) (CleanResult, error) {
	return s.buildCleanPreview(cacheDir, opts)
}
func (s Service) Clean(cacheDir string, opts CleanOptions) (CleanResult, error) {
	p, err := s.buildCleanPreview(cacheDir, opts)
	if err != nil { return CleanResult{}, err }
	return s.ApplyClean(p)
}
func (s Service) ApplyClean(preview CleanResult) (CleanResult, error)
```

keep-latest 强制扫描 `KindPkg` 并只把 Matched 放入 snapshot。Apply 验证 prepared/cache root，再依次 `ensurePathInDir`、`os.Lstat`、`os.SameFile`、size、mtime，最后沿用 `os.Remove/removeEmptyParents`。

- [ ] **Step 5: 验证 JSON 与 service 并提交**

Run: `go test ./internal/app/cache`

Expected: PASS；JSON 真实反序列化包含两个新字段且不暴露 snapshot。

```bash
git add internal/app/cache/model.go internal/app/cache/service.go internal/app/cache/cache_test.go docs/superpowers/plans/2026-07-18-cache-clean-keep-latest.md
git commit -m "feat: apply cache clean previews safely"
```

---

### Task 4: 接入 CLI flag、校验与输出

**Files:**

- Modify: `internal/cli/cache_cmd.go`
- Modify: `internal/cli/cache_handler.go`
- Modify: `internal/cli/app_cache_test.go`
- Modify: `internal/cli/cache_cmd_test.go`

**Interfaces:**

- Consumes: `CleanOptions.KeepLatest`、`PreviewClean`、`ApplyClean`
- Produces: `eget cache clean --keep-latest`

- [ ] **Step 1: 对 `CacheCleanOptions/newCacheCmd/newCacheCleanCmd/cleanOptionsFromCLI/handleCacheClean` 做 GitNexus impact**

报告 HIGH/CRITICAL 后再编辑。

- [ ] **Step 2: 写 flag、reset 与互斥失败测试**

覆盖：keep-latest 合法；与非空 older/all/api/sdk/sdk-index/partial 互斥；与 pkg 合法；`--older=` 合法；普通空 older 得到 72h；同一 app 二次运行不残留 KeepLatest，Older reset 为空。

```go
got, err := cleanOptionsFromCLI(&CacheCleanOptions{KeepLatest: true})
assert.NoErr(t, err)
assert.True(t, got.KeepLatest)
assert.Eq(t, []appcache.Kind{appcache.KindPkg}, got.Kinds)
```

- [ ] **Step 3: 实现 options 与 flag**

`CacheCleanOptions` 增加 `KeepLatest bool`；`cleanOpts` 初始化和 reset 改为空结构；`StrOpt` 默认值改空且帮助写明默认 3d；增加 `BoolOpt(..."keep-latest"...)`。`cleanOptionsFromCLI` 先校验冲突，普通模式空值回填 `3d`，keep-latest 固定 KindPkg 且不解析 duration。

- [ ] **Step 4: 写文本和真实 JSON 失败测试**

临时写同一 family 两个版本。dry-run JSON 反序列化后断言 matched=1、removed=0、kept=1；文本两行只在 keep-latest 出现；非 dry-run 删除旧版本。不得只用 JSON 字符串 contains 替代反序列化。

- [ ] **Step 5: handler 改用同一 Preview**

```go
preview, err := s.cacheService.PreviewClean("", cleanOpts)
// dry-run / confirm
result, err := s.cacheService.ApplyClean(preview)
```

keep-latest 文本输出新增 kept/unrecognized；普通模式不显示。确认阈值和 `--yes` 不变。

- [ ] **Step 6: 验证并提交**

Run: `go test ./internal/cli`

Expected: PASS。

```bash
git add internal/cli/cache_cmd.go internal/cli/cache_handler.go internal/cli/app_cache_test.go internal/cli/cache_cmd_test.go docs/superpowers/plans/2026-07-18-cache-clean-keep-latest.md
git commit -m "feat: add cache clean keep latest mode"
```

---

### Task 5: 全量验证与收尾

**Files:**

- Modify: `docs/superpowers/plans/2026-07-18-cache-clean-keep-latest.md`
- Modify: `AGENTS.md`

- [ ] **Step 1: 格式化并运行聚焦测试**

Run: `gofmt -w internal/app/cache/keep_latest.go internal/app/cache/keep_latest_test.go internal/app/cache/model.go internal/app/cache/service.go internal/app/cache/cache_test.go internal/cli/cache_cmd.go internal/cli/cache_handler.go internal/cli/app_cache_test.go internal/cli/cache_cmd_test.go`

Run: `go test ./internal/app/cache ./internal/cli`

Expected: PASS。

- [ ] **Step 2: 运行 MVP 全量测试**

Run: `go test ./...`

Expected: PASS；若有无关既有失败，保留命令和失败包证据，不宣称全量通过。

- [ ] **Step 3: 检查 diff**

Run: `git diff --check`

Expected: 无输出。

- [ ] **Step 4: 运行 GitNexus change detection**

Run: `npx gitnexus detect-changes --repo eget --scope all`

Expected: 仅影响 cache clean parser/selector/service/CLI 及测试；无无关 flow。

- [ ] **Step 5: 更新 checkbox 和进行中事项**

所有实际完成步骤改为 `[x]`；从 `AGENTS.md` 移除 Cache Clean 工作项，不移除其他事项。

- [ ] **Step 6: 提交收尾并审计状态**

```bash
git add AGENTS.md docs/superpowers/plans/2026-07-18-cache-clean-keep-latest.md
git commit -m "docs: complete cache clean keep latest plan"
git status --short
git log -5 --oneline
```

Expected: 只剩用户原有修改；最近提交按 parser、selector、service、CLI、收尾分阶段存在。
