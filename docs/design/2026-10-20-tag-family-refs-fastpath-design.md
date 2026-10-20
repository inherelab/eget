<!-- template_id: design; template_version: 1.1.1 -->
# eget tag 家族 refs 快路径 设计

> 状态：Draft 0.2 / 待人工计划批准

## 修订记录

| 版本 | 日期 | 作者 | 摘要 |
|---|---|---|---|
| 0.1 | 2026-10-20 | inhere | 初稿：为 tag 模式过期检查引入 matching-refs 快路径；固定窄版本比较器与 fail-open 级联；明确与 release 扫描 stable-preference 的关系及一处接受偏差 |
| 0.2 | 2026-10-20 | inhere | 按 design contract 补「架构」章节：固定 tagfamily.go 与各 owner 的分工、TagFamilyMax 形状及回退条件、依赖方向 |

> 仅语义变化递增版本；纯 identity/provenance/元数据纠正沿用原版本，并在 Git/进度记录中留痕。

## 背景与目标

用户结果：`eget update --check` / `list --outdated` 对 monorepo 产品（tag 跟踪策略 `pattern`，如 `trycua/cua` 的 `PRE:cua-driver-rs-v`）完成一次"是否需要更新"的判断，应在秒级完成，而不是等一次全仓库 release 历史扫描；且判断结论必须与现行行为（扫描语义）一致——既不漏报更新，也不把用户引向错误的版本。

已落地的两个优化（commit `1bce7bc`：stable 命中即停的分页 + per_page=100 + 发布时间随扫描返回）处理了"stable 存在于扫描首页"的常态，但 `trycua/cua` 把仓库全部 release 都标为 prerelease（保护仓库级 Latest 的 monorepo 做法），扫描永远走满页分支。本设计处理该残余瓶颈。

实测数据（trycua/cua，经 127.0.0.1:10801 代理，2026-10-20）：

| 请求 | 体积 | 耗时 |
|---|---|---|
| `releases?per_page=100&page=1`（27→3 页之一） | ~5.3 MB | ~32 s |
| `releases?per_page=100` 全量 3 页（现况冷检） | ~16 MB | ~55 s |
| `git/matching-refs/tags/cua-driver-rs-v`（120 个 ref） | ~44 KB | ~0.94 s |

另：GitHub Search API 不覆盖 releases（只有 repos/code/issues/users），"关键词搜索"路线不可行，本设计不再展开。

## 名词

- **release 扫描（slow path）**：`source/github/finder.go` 的 `AssetFinder.FindMatch()`——按创建时间倒序逐页拉 `/releases`，用 `filter.Spec` 匹配 tag，stable 优先（无 stable 时取最新 prerelease，`Prerelease=true` 时直接取最新匹配）。
- **tag 家族**：`matching-refs` 端点 `GET /repos/{repo}/git/matching-refs/tags/<literal-prefix>` 返回的、以字面前缀开头的全部 tag 引用；不改匹配语义，只回答"这个家族里版本最新的 tag 是谁"。
- **窄版本比较器（family comparator）**：仅为家族内 tag 排序新增的私有比较逻辑，只识别 `数字核心 + 残留后缀` 形态；任何无法判定序的情形立即放弃（fail-open），不引入通用版本解析模型（consolidation 计划 F6 保持 deferred）。
- **快路径（fast path）**：检查侧的短路——用 refs + 比较器判定"家族版本最 max == 已安装 tag ⇒ 未过期"，跳过 release 扫描；其余一切情形回落到扫描。
- **fail-open**：快路径信息不足以给出确定结论时，回退到扫描而不是给出可能错误的"已最新"。

## 范围与非目标

范围（实施阶段约 5 个 owner 文件，均在 `inhere-tools/eget/internal`）：

- `internal/source/github`：新增家族 refs 获取与窄比较器（新小文件如 `tagfamily.go` + 单测），带分页与 api_cache 复用。
- `internal/app/list.go`：`LatestCheckTarget` 增加 `InstalledTag` 字段并在 `checkOutdatedItem` 填充。
- `internal/cli/wiring.go`：`githubTagLatestInfo` 增加快路径分支与 fail-open 回落。
- 测试：比较器 characterization、家族端点 fake、wiring 快路径/回落用例。

非目标：

- 不改安装路径（`AssetFinder.Find/FindMatch`）的任何匹配语义与返回资产行为；安装仍走 release 扫描（安装需要资产与 prerelease 标志）。
- 不为 `SUF:` / `REG:` 提供快路径（无字面前缀，家族端点无法服务端过滤）。
- 不建 `internal/version` 或任何共享版本比较抽象（SR1405.1：仅单一用例，不满足平台化条件）；比较器私有于 `source/github`。
- 不改 `ResolveRelease` 的签名与对外语义。
- 不引入新配置项 / 新 CLI 参数。

## 已确认事实与规范

- 快路径与扫描的关系必须是"快路径只短路最常见的'未过期'结论；其余一切回落扫描"。扫描语义（创建时间倒序 + stable 优先 + prerelease 兜底）保持唯一权威，保证与安装行为同一套判断（refs #44 / commit `1bce7bc` 的原则）。
- `filter.Spec`：`KindPlain` 的 `Expr==Raw`（调用点定义为锚定前缀，`tagMatcher` default 分支 → `PrefixRegex`）；`KindPrefix`（`PRE:`）的 `Expr` 即字面前缀。快路径因此对 `PRE:` 与裸值锚定前缀两类生效。
- 过期判定的现行核心是**字符串相等**：`latest.Tag == item.InstalledTag`（`list.go` `checkOutdatedItem`），这使快路径的相等判断无需比较器参与"等于"判定——比较器只负责选出家族版本 max 的 tag 字符串。
- 实测：`matching-refs/tags/cua-driver-rs-v` 单响应返回全部 120 个 ref，结果按 refname 字典序**升序**（`v0.10.0` 排在 `v0.2.0` 前），节点只含 ref 名与 SHA，无 prerelease 标志与发布时间——排序与筛选必须全部在客户端完成。
- 治理：模式解析快路径不改变外部 CLI/配置/缓存 layout；单 commit 可独立 revert（SR1409 低暴露度：本地工具、无生产数据、无出机器动作）。

## 总体方案

检查侧新增前置短路：`githubTagLatestInfo` 在 tag 命中"字面前缀形态"（`KindPlain` 裸值 / `PRE:`）且 `target.InstalledTag != ""` 时，先调 `source/github` 的家族端点取 ref 集，用窄比较器求出家族版本 max 的 tag：

- `家族 max == 已安装 tag` ⇒ 直接返回 `LatestInfo{Tag: installed}`（未过期，检查结束，1 次轻量请求）；
- `家族 max != 已安装 tag`、refs 请求失败/为空、集合中出现不可比较对或最大者并列 ⇒ 一律 fail-open 回落现有 `ResolveRelease`（release 扫描）。

扫描仍负责给出"最新"的最终答案（stable 优先 + 发布时间 + 资产可见性），快路径永远不会让检查得出比扫描更"新"的错误结论。

### 窄版本比较器规格（fail-open 设计）

- 输入：字面前缀 `P` 与家族 tag `t`（已剥 `refs/tags/` 且须有前缀 `P`）；后缀 `S(t)=t[len(P):]`。
- 解析：`S` 按 `.` 切分，从零起收集**全为数字**的 token 直至遇到第一个非数字 token（含空段）；收齐的数字序列为核心 `C(t)`，其后剩余字符串为残留 `X(t)`（可比对 `""`，`-rc1` / `+build` / 日期形态均落在此处）。
- 比较 `cmp(a,b)`：
  1. 按 `C` 逐位数值比较（缺失位=0 不做相等合并：`v0.37` vs `v0.37.0` 走到"核心等长截断后逐位相等但位数不同"的情形时视为**不可比较**，fail-open）；首位可分的最长者胜。
  2. 核心逐位相等时：`X` 均空 → 不可比较（如 `v0.37` vs `v0.37.0`，语义为实现细节，fail-open）；一空一非空 → 空侧**更大**（稳定版高于带残留的版本，与 semver `-rc1` 的惯例一致）；两侧均非空且字典序不等 → 字典序较大者为大（`-rc2 > -rc1`），相守恒仅用于家族内排序。
  3. 任一步比较落到"不可比较" ⇒ 整个快路径放弃，回落扫描。
- 该规格只为家族内排序而存在；不用它解释版本、不用它比较非同前缀的 tag。characterization 必含：`v0.10.0 > v0.2.0`、`v0.37.0 > v0.37.0-rc1`、`v1.0.0 > v0.99.0`、缺失位/混合残留/非数字残留（如日期 tag）触发 fail-open。

### refs 获取

- 端点：`GET https://api.github.com/repos/{repo}/git/matching-refs/tags/{url.PathEscape(P)}`；沿用现有 getter（代理、`api_cache`、鉴权回退），分页跟随 `rel="next"`（`per_page=100` 起步，与扫描分页循环同构、复用早停思路），实测单响应已含全家族。
- 快路径请求同样进入 api_cache（30 分钟 TTL），重复检查近零成本。

## 架构

总层分工（快路径是检查侧末端的一小步，不新建包）：

```text
internal/source/github (owner)
  ├─ tagfamily.go  // 新增：matching-refs 抓取(带分页) + 窄版本比较器
  │      TagFamilyMax(repo, prefix, getter) → (maxTag string, ok bool)
  │      // ok=false 一律 fail-open；请求走现有 getter，进 api_cache
  └─ finder.go     // 不改：Find/FindMatch 仍是唯一权威解析路径
internal/app
  └─ list.go       // LatestCheckTarget 增 InstalledTag，checkOutdatedItem 填充
internal/cli
  └─ wiring.go     // githubTagLatestInfo 增快路径分支：位面前缀 + InstalledTag
                   // 非空才尝试；ok=false 或 max≠installed → ResolveRelease 扫描
```

依赖方向：`cli → app/filter/source-github → filter`，与 consolidation 计划的低层包消费方向一致；不新增包、不引入新依赖。

## 关键流程

```text
update --check <pattern-package>
  └─ checkOutdatedItem(item)  // item.InstalledTag 可用
       └─ latestInfo(target{Tag:"PRE:...", InstalledTag:"cua-driver-rs-v0.35.4"})
            └─ githubTagLatestInfo
                 ├─ filter.IsPattern && 字面前缀可得 && InstalledTag != ""
                 │    └─ TagFamilyMax(repo, prefix, getter)          // refs + 窄比较器
                 │         ├─ max == InstalledTag → LatestInfo{Tag: InstalledTag}   // 快路径命中，~1 轻量请求
                 │         └─ max != InstalledTag 徝失败/不可判定 → (b)
                 └─ (b) ResolveRelease → FlexMatch release 扫描 → LatestInfo{Tag, PublishedAt}
```

输出层（`checkOutdatedItem`）不变：`latest.Tag == item.InstalledTag` ⇒ 未过期；否则报告过期并显示扫描给出的 latest 与发布时间。

## 安全、数据、运维与回滚

- 无外部动作、不涉数据迁移；改动纯内部，单 commit revert 即回滚（含其测试）。
- 失败模式：refs 端点失败/返回空 → fail-open 扫描，检查结论不改；api_cache 故障沿用现有 getter 行为无差别。
- 序列化影响：快路径仅替换"一次冷检"的头部请求，命中时少 3~27 个重型请求；对代理与 GitHub rate limit 是纯减负。

## 决策

- **D1（采纳）**：快路径只做"`家族版本 max == 已安装 tag ⇒ 未过期`"一种结论，其余路径一律回落 release 扫描；扫描语义保持唯一权威。理由：检查侧只需是/否，安装侧需要资产与 prerelease 信息，两者不混用。
- **D2（采纳）**：引入窄版本比较器，私有于 `internal/source/github`；小、fail-open、不外溢成通用版本解析（F6 deferred 不受此影响）。理由：refs 天然无序（字典序），没有排序就没有"家族最新"；fail-open 保证任何语义不明确家族直接走扫描，错误代价为零。
- **D3（采纳）**：快路径仅覆盖 `PRE:` 与裸值锚定前缀；`SUF:` / `REG:` 不受益但也不变慢（仍走扫描）。理由：家族端点只能 literal 前缀过滤。
- **D4（接受偏差，需用户确认）**：家族 max 判断与扫描结论在一类罕见案例上不一致——"按创建日期重建/后发的低版本 stable"（如家族含 date-newer 的 `v0.35.9` stable + 已安装 `v0.37.0`）：现行扫描会把它报成 latest（本质是"降级提示"），快路径则终判未过期。取快路径结论并接受归一：家族版本 max 定义"新鲜度"。理由：现行降级提示本身不符合用户预期，且该案例需要上游重打低版本 tag 才会发生。
- **D5（否决并记录）**：关键词搜索（GitHub Search API 不含 releases）；以发布时间剪枝替换版本排序（creation date 可从 refs 的 SHA 免费近似但需逐 ref 查 commit，请求数反升）。两者不采纳。

## 待确认事项

1. D4 的语义归一是否接受（解除实施前提的关键决策点）。
2. 比较器中"空残留高于非空残留"（稳定版优先于带残留版本）与"核心位数不同或残留并列即 fail-open"这两条舍入规则是否接受。

## 结论与人工计划 Gate

- 本设计确立快路径的取舍边界，不授权规划或实施。
- 批准后进入 plan：按"已确认事实与规范"中的文件清单拆任务（比较器 characterization → 家族端点封装 → list/wiring 接线与回落 → e2e 实测 trycua/cua 为验收），并对实施保留 SR1204 scope confirmation。
- 实施预期验收：trycua/cua 冷检 `update --check cua-driver` 从 ~55s 降至 ~2s 内（1 次轻量请求）；已过期路径回落扫描后结果与现行一致；`go test ./internal/...` 全绿。
