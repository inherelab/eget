<!-- template_id: plan; template_version: 1.2.0 -->
# eget tag 家族 refs 快路径 实施计划

> 状态：Draft 0.1 / 待人工计划批准

## 修订记录

| 版本 | 日期 | 作者 | 摘要 |
|---|---|---|---|
| 0.1 | 2026-10-20 | inhere | 初稿：按 design 0.2（candidate `d4b1061`）拆出比较器、家族端点、检查接线与实测验证四个任务；固定 fail-open 级联与 SR1204 范围确认点 |

> 仅语义变化递增版本；纯 identity/provenance/元数据纠正沿用原版本，并在 Git/进度记录中留痕。

## 目标与完成定义

用户结果：对 tag 跟踪策略为 `pattern` 的安装项（如 `PRE:cua-driver-rs-v` @ `trycua/cua`），`eget update --check` / `list --outdated` 的冷启动首检用一次轻量 `matching-refs` 请求完成"未过期"结论（trycua/cua 实测 ~55s → ~2s）；出现更新的结论及其显示（latest tag、发布时间）与现行 release 扫描语义一致。

完成定义（全部可观察）：

1. `internal/source/github/tagfamily.go` 存在：字面前缀拉取 `matching-refs`（带分页与 api_cache 复用）+ 窄版本比较器；`ok=false` 语义 = fail-open。
2. wiring 快路径只在「`PRE:` / 裸值锚定前缀 + `InstalledTag` 非空」时尝试，且其唯一结论是"家族 max == 已安装 tag ⇒ 未过期"；其余一切回落现有 `ResolveRelease`。
3. 全部 characterization 通过：比较器序与 fail-open 触发、wiring 快路径命中/落空/回落/短路径缺席。
4. `go build ./...`、`go vet ./internal/...`、`go test ./internal/...` 全绿；e2e 真环境实测达标（T03）。
5. 每任务一个本地 commit，可独立 revert；不 push。

## 范围、排除项与授权

范围：`inhere-tools/eget/internal` 下 — `source/github/tagfamily.go`（新）、`source/github/finder_test.go` 或新 `tagfamily_test.go`、`app/list.go`（`LatestCheckTarget` 增 `InstalledTag` 字段 + `checkOutdatedItem` 填充）、`cli/wiring.go`（`githubTagLatestInfo` 前置短路与 `client` 包装）、`cli/wiring_latestinfo_test.go`（/新用例）。

排除项（明确不做，理由随行）：

| 排除项 | 理由 / 留痕 |
|---|---|
| 安装路径（`AssetFinder.Find/FindMatch`）任何改动 | design D1：扫描语义是唯一权威，安装需要资产与 prerelease 标志 |
| `SUF:` / `REG:` 快路径 | 家族端点只能 literal 前缀过滤（design D3） |
| 新建 `internal/version` 或导出通用版本比较 | SR1405.1：单一用例不满足平台化；比较器私有于 `source/github`（design D2） |
| 新配置项 / CLI 参数 / 缓存 layout 变化 | 行为差异只发生在检查内部，无对外接口变化 |
| 线上二进制 `D:\env\tools\eget.exe` 替换 | 属构建/替换动作，不在实施范围；完成后列为剩余工作待用户授权 |
| storepkg/installed 记录格式改动 | 授权边界内无需触碰 |

授权边界：plan 批准仅授权计划内一次执行请求中要求的本地代码/测试改动与本地提交；不含 push、release、部署或任何出机器动作。

- `host_or_non_offline_action=REQUIRED`：T03 的真环境验证需要网络（GitHub API 经本机代理 127.0.0.1:10801，只读 GET，目标 `api.github.com` 的 `matching-refs` 与 `releases` 端点）；无任何写动作。

## 输入与批准证据

| 项 | 值 |
|---|---|
| 设计文档 | `docs/design/2026-10-20-tag-family-refs-fastpath-design.md`（Draft 0.2） |
| 设计 candidate | commit `d4b10613148765d93adcf79dac3b5cc87157f5e7`，blob `7d901f1715bd7a42689d6d4614eccd02dd0c0cd9` |
| 设计验证 | `validate_document.py --kind design` → `ok: true`（/plugin skill scripts，2026-10-20） |
| 设计批准证据 | 用户于本会话明确指示「你再检查一遍设计；没问题的话就编写实施计划」：视为接受 design 0.2 及其开放决策（D4 家族 max 定义新鲜度；比较器"空残留>非空残留"与"核心位数不同/并列⇒fail-open"两条舍入规则） |
| 基线数据 | trycua/cua：27 页×30 / 3 页×100 全 prerelease 扫描 ~55s 冷检；`matching-refs/tags/cua-driver-rs-v` 单请求 120 ref / ~44KB / ~0.94s |
| 交付跟踪 | 设计门已过；review 一轮合并轴（低暴露度），批准后实施 |

## Capability Discovery

### Capability decisions

| capability_id | required_capability | searched_candidates | direct_reuse | thin_adapter_or_owner_extension | decision | proven_gap | duplication_and_lifecycle_risk |
|---|---|---|---|---|---|---|---|
| CAP-01 | tag 家族端点抓取与家族内排序的单一 owner | `internal/source/github`（finder 虽同域但语义为 release 扫描）、`internal/client`（仅 GitHub API 封装）、stdlib `sort`/`strconv` | 无现成家族解析 | owner 仍是 `internal/source/github`，新增 `tagfamily.go`（不新建包） | OWNER_EXTENSION | 现代码无 matching-refs 消费点，也无任何版本序比较（consolidation F6 确认 deferred），缺位已验证 | 同包相邻文件，生命周期随 eget 内部演进；删除该文件即回到全扫描行为（fail-open 语义下的等价回归） |

### New module candidates

| candidate_id | capability_id | proposed_module | searched_candidates | direct_reuse_gap | thin_adapter_or_owner_extension_gap | proven_gap | unique_owner_and_lifecycle | deletion_or_merge_handling |
|---|---|---|---|---|---|---|---|---|
| NONE | none | none | none | none | none | none | none | none |

### Rejected new tools

| rejected_candidate | capability_id | deletion_test_and_reason |
|---|---|---|
| 通用版本解析/比较库 or `internal/version` 包 | CAP-01 | 删除测试：仅一个调用点、仅家族内排序、consolidation 计划 F6 已显式 deferred「需先定单一模型」；删掉即回到 fail-open 全扫描，无功能缺口 |

## 前置检查与 fail-closed 条件

### workspace baseline（mutation 保护快照）

| 项 | 值 |
|---|---|
| Git root | `D:\work\inhere\my-tools-dev\inhere-tools\eget` |
| branch / HEAD | `master` / `d4b10613148765d93adcf79dac3b5cc87157f5e7` |
| `git status --short` | 空（设计文档与既有修复均已提交；实施前逐项核对，出现他人 dirty 触及 expected paths 即停） |
| 模块 | `module github.com/inherelab/eget`，go 1.25 工具链 |
| 预跑基线 | `go build ./...` exit 0；`go test ./internal/source/github ./internal/cli -count=1` exit 0（2026-10-20 已验证） |
| 预期触及路径 | `internal/source/github/tagfamily.go`（新）、`internal/source/github/tagfamily_test.go`（新）、`internal/app/list.go`、`internal/cli/wiring.go`、`internal/cli/wiring_latestinfo_test.go`、必要时 `internal/client/github.go`（加包装方法） |
| 依赖与授权 | 不新增 go.mod 依赖（stdlib only）；每任务一个本地 commit；push/release 未授权 |

### fail-closed 条件

- 任一现有测试（基线 characterization）变红，或必须修改既有断言才能通过 → `Ownership`/`Semantic` 复核：若为快路径引入的行为分叉（fail-open 设计外），停止并回滚该任务 commit。
- 实施中发现需要触碰 `internal/install/*` 或安装语义 → `Semantic Amendment`，未问询不得继续，返回 plan review。
- 匹配-refs 行为与实测不符（如某仓库返回 DNA+分页异常）→ 比较/分页以 fail-open 兜底，不得在计划外自造启发式；需扩围则 `DEFER_OR_REQUEST`。
- 团队/SR1204 scope confirmation：业务文件约 5~6 个（>3），实施开始前由本计划批准完成范围确认（本提交即范围确认载体）。

## 波次与依赖

```text
T01 比较器 + 家族端点(tagfamily) ─► T02 wiring 接线 ─► T03 真环境验证 ─► T04 完成 Gate
```

## 任务

> 所有命令运行目录为 Git root（`D:\work\inhere\my-tools-dev\inhere-tools\eget`）；下文相对路径以该目录为准。

### T01 窄版本比较器与家族端点抓取（tagfamily.go）

- 文件：`internal/source/github/tagfamily.go`（新）、`internal/source/github/tagfamily_test.go`（新）
- 动作：
  - 未导出比较器 `compareFamilyTags(a, b string) (int, bool)`：按 design 0.2 规格 — 剥字面前缀后的 `tagFamilySuffix` 解析为「数字核心 token 序列 + 残留」；核心逐位数值比（首位可分者优先，位数不同或残留并列 → `ok=false`）；核心相等且残留一空一非空 → 空侧大；残留均非空且不等 → 字典序；相等 → `ok=false`（并列放弃）。
  - 导出 API `TagFamilyLatest(repo, prefix string, getter HTTPGetter) (tag string, ok bool)`：
    - `GET https://api.github.com/repos/{repo}/git/matching-refs/tags/{prefix}`（prefix 原样进路径，仅做空格剔除；分页跟随：累计 refs，单批 ≥ per_page（100）时 `&page=N` 翻页；复用 `finder` 的 HTTP 读取与日志风格，`verbosef` 记录请求）。
    - 收集带字面前缀的 `refs/tags/...`；用比较器线性求 max；任何比较返回 `!ok` ⇒ 整体 `ok=false`；getter 失败/非 200/refs 空 ⇒ `ok=false`；不吞错误（错误即 fail-open，扫面兜底，不打扰调用方语义）。
    - prefix 为空 ⇒ 直接 `ok=false`。
  - 设计形态禁止项：不导出比较器；不 import `internal/filter`（过滤由调用方用 filter.Spec.Expr 得到前缀后传入）。
- 验证：`go test ./internal/source/github -run 'TagFamily|compareFamilyTags' -count=1`；
  characterization 必含：`v0.10.0 > v0.2.0`、`v0.37.0 > v0.37.0-rc1`、`v1.0.0 > v0.99.0`、`v0.37` vs `v0.37.0` fail-open、日期残留 fail-open、refs 分页批满翻页、API 失败/空 refs fail-open、单请求全量（fake 120 条）成 max。
- 完成标准：上述 focused 测试全绿；tagfamily.go `go vet` 干净。
- 依赖：无；commit `feat(check): add tag family refs resolver with fail-open comparator`。

### T02 检查路径接线（list.go + wiring.go + 测试）

- 文件：`internal/app/list.go`、`internal/cli/wiring.go`、`internal/cli/wiring_latestinfo_test.go`
- 动作：
  - `app.LatestCheckTarget` 增字段 `InstalledTag string`；`checkOutdatedItem` 构造 target 时填 `InstalledTag: item.InstalledTag`（唯一填充点；其它 latestInfo 调用方不传即天然回落扫描）。
  - `client.GitHubClient` 增薄包装 `TagFamilyLatest(repo, prefix string) (string, bool)`，内部复用 `c.get / c.opts` 组 getter（与 `ResolveRelease` 同构）。
  - `githubTagLatestInfo` 前置短路（依次判断）：`target.Tag` 可解析出**字面前缀**（`filter.Parse` 后 `Kind==KindPrefix||Kind==KindPlain`，取 `spec.Expr`）且 `target.InstalledTag != ""` 且前缀非空 → 先 `TagFamilyLatest`：
    - `ok && tag == target.InstalledTag` ⇒ 返回 `app.LatestInfo{Tag: target.InstalledTag}`（不请求 release 扫描）；
    - `!ok` 或 `tag != target.InstalledTag` ⇒ 现有 `ResolveRelease` 流程原样执行（该分支逐字不动）。
  - 注释标明 fail-open 级联与 design D1/D4（家族 max 定义新鲜度的归一）。
- 验证：`go test ./internal/cli -run 'GithubTagLatestInfo' -count=1`；
  新用例：(a) 快路径命中 → 仅 1 个 matching-refs 请求、零 `/releases?` 请求、返回 `InstalledTag`；(b) max≠installed → 回落扫描，结果与现行用例一致；(c) refs 请求失败/空 → 扫描兜底；(d) 家族不可比（`v0.37` vs `v0.37.0`）→ 兜底；(e) `InstalledTag==""`（如 self-update 路径）→ 零 matching-refs 请求。既有 3 个用例保持原断言不变（同一断言对改前基线 `d4b1061` 已绿，作为现有行为回放）。
- 完成标准：focused 测试全绿；`go test ./internal/app -count=1` 绿。
- 依赖：T01；commit `feat(check): short-circuit up-to-date check via tag family refs`。

### T03 真环境验证（e2e）

- 文件：无业务文件改动（验证任务）
- 动作：`go build -o tmp/eget-fastpath.exe ./cmd/eget` 后执行：
  1. 冷检：删除 `D:/work/inhere/data/cache/eget/api-cache/api.github.com-repos-trycua-cua-*` 后 `measure` `tmp/eget-fastpath.exe update --check cua-driver`（预期 ≤5s、仅 1 个 matching-refs 请求）；
  2. `update --check --all`（预期与 T02 前基线产物一致：无 `check_failed`、其余包过期结果一致）；
  3. 用 `-v` 确认只打 `matching-refs`、无 `releases?per_page=100` 页请求（cua 当前实为最新态）。
   另选一个构造可复验的过期案例：临时把本机 `installed.toml` 的 cua-driver tag 改为 `cua-driver-rs-v0.35.4`（真实旧版，可逆、完成后还原），验证 max≠installed → 扫描兜底仍报告 outdated 到 `cua-driver-rs-v0.37.0` 且显示发布时间。
- 可观察验收：三项结果数值与设计目标一致（~2s 级冷检；outdated 表数值与基线一致；还原后重检回到"未过期/或与还原前一致"）。
- 完成标准：述命令执行得到的日志/摘要存 `tmp/`，并在 completion 总结引用关键数值。
- 依赖：T02；`host_or_non_offline_action=REQUIRED`（目标：api.github.com 只读）。

### T04 完成 Gate（全量回归 + completion）

- 文件：无新增改动
- 动作：`go build ./...`；`go vet ./internal/...`；`go test ./internal/... -count=1` 全绿；按 plan-closure 输出 ≤8 行 progress 总结（主旨、状态、自主决策、commit 区间、耗时、剩余项）；不自动 push。
- 完成标准：三项命令零失败；`tmp/eget-fastpath.exe` 留存待用户决定是否替换线上二进制。
- 依赖：T01、T02、T03。

## 回滚与恢复

- 每任务一 commit：任一任务失败或发现越界行为变化即 `git revert <task-commit>`，工作树回基线 `d4b1061`；T03 临时改动的 installed.toml 在任务完成前还原（含备份件 filenames/copy 校验）。
- 实施中断恢复：以 HEAD / `git status` 为准重建进度；`tmp/eget-*.log` 只作运行证据不作恢复依据。

## 人工 Gate

1. **计划批准**：批准本 plan 0.1（candidate 以提交后 `git_candidate_revision.py --kind plan` 输出为准）→ 解除实施 Gate。
2. **当前执行请求**：批准后的下一次会话指令开始实施（计划批准不自动授权）。
3. **外部动作 Gate（仅限 T03 真环境验证目标的扩展）**：额外网络目标（若需要）逐项具名，其余均为本机只读网络 GET（已在 scope 声明 REQUIRED + 目标）。
4. 完成后替换线上 `D:\env\tools\eget.exe` 仍需单独授权（列于完成 Gate 剩余工作）。

## 可追溯性

| design/需求 | 任务/验证 |
|---|---|
| D1（快路径唯一"未过期"结论 + 一律 fail-open 回落） | T02 接线结构 + 新用例 (a)(b)(c)；T04 全量回归 |
| D2（窄比较器 fail-open、私有于 source/github） | T01 `compareFamilyTags` + characterization（含 fail-open 组） |
| D3（仅 `PRE:`/裸值，`SUF:/`/`REG:` 不受益不变慢） | T02 接线入口判断（Kind 前缀类）+ 用例 (e) 反证 |
| D4（家族 max 定义新鲜度；接受降级案例归一） | T02 注释落位；T03 旧版 installed 复验 outdated 行为 |
| 用户结果（~55s → ~2s，结论一致） | T03 三项可观察验收 + T02 用例 (a)(b) |
| 基线回放（现有断言改前已绿、改后不放宽） | T02 既有用例原断言 + T04 全量 |

## 完成 Gate 与剩余工作

- 完成 Gate：T04 三命令零失败 + T03 验收数值达标 + 各任务 commit 独立可 revert。
- 剩余工作（不阻塞 COMPLETE）：
  1. 线上二进制替换（需用户授权后另行动作）；
  2. `matching-refs` 对其它 monorepo（计划外仓库）的行为观察与必要时 fail-open 记录；
  3. F6 版本解析收敛与本比较器的再归并评估（维持 consolidation 计划 deferred 状态）。
