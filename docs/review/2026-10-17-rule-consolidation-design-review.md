<!-- template_id: review; template_version: 1.1.0 -->
# eget 匹配与转换规则整合设计 评审

## 评审目标与范围

- 目标: `inhere-tools/eget/docs/design/2026-10-17-rule-consolidation-design.md`（「eget 匹配与转换规则整合 设计」，Draft 0.1）
- 修订版本: 0.1
- 评审者: 独立评审者（独立子代理会话，非设计作者）；主会话仅复核证据并汇编本报告，未增删发现
- 日期: 2026-10-17
- 排除项:
  - 不评审实现（当前不存在实现）
  - 不评审 `docs/plans/go-subpackage-utility-plan.md` 全文，仅核实其阶段 5 的"暂缓"表述
  - 不审计 `cmd/`、`web/`、`internal/app/web/` 与全部测试文件
  - 不运行 `go build` / `go test`，不主张任何端到端行为证明
  - 不修复任何发现（REPORT_ONLY，见 review-contract 授权边界）

### 候选身份（exact candidate）

| 项 | 值 |
|---|---|
| Git root | `inhere-tools/eget` |
| candidate commit | `80a190fcdba2be710b7f817bd6f5267709e1570c` |
| subject path | `docs/design/2026-10-17-rule-consolidation-design.md` |
| document revision | `0.1` |

候选已提交；`git status --short` 无该文档未提交改动，candidate 未变。

## 输入与方法

评审轴：**合并轴（Standards/Governance + Spec/Executability）**，单轮。该选择依据被评文档 `:190` 的 SR1409 低暴露度判定；**但暴露度本身被 R-01 判为存疑**，因此本合并轴评审在其暴露度未被确认前不可替代两轴独立评审。

方法：

1. 完整读取被评设计文档，逐条打开其引用的 `file:line` 复核 F1–F10 与 M1 迁移映射。
2. 独立枚举 `internal/**` 包与实测行数，定向 `rg` 搜索五类知识（OS/Arch 别名与 token、归档/可执行后缀、`PRE:`/`SUF:`/`REG:` 语法、版本解析比较、模板渲染）以验证枚举完备性。
3. 核实影响治理判定的仓库事实：`git tag` 数量与 `git describe`、`git tag --points-at`、`.github/workflows/release.yml`、`README.md` 安装方式。
4. 对三个新增包逐个先答 scope necessity，再评 internal validity。

实际读取范围（节选，均为相对路径）：

- `internal/install/detect/detectors.go:1-344`、`internal/install/runner_platform.go:180-253`、`internal/install/runner_output.go:1-227`、`internal/install/runner_download.go:155-184`、`internal/install/runner_select.go:185-234`、`internal/install/extractor.go:1-130`、`internal/install/system7z.go:1-423`、`internal/install/file_modes.go:1-77`、`internal/install/gui.go:1-70`、`internal/install/chooser.go:1-159`、`internal/install/service.go:265-359`、`internal/install/default_service.go:60-84`
- `internal/client/cache_path.go:1-219`、`internal/app/cache/keep_latest.go:1-455`、`internal/app/tag_semantics.go:1-85`
- `internal/tagfilter/tagfilter.go:1-65`、`internal/sdk/{search_service,html_index,template,json_index,builtin_config}.go`、`internal/source/{urltemplate/template.go,sourceforge/version.go,pkgtemplate/target.go}`
- `internal/config/proxy.go:1-158`、`.github/workflows/release.yml`、`README.md`

规范加载：`idev-std load --delivery semantic --task review` → `binding_status=BOUND`，payload `sha256:bdd6438f9993249f4dedcc4eef67edf0212f229081c4d4f728ff3221895354dc`，本地重算一致。

## 验收标准

- 治理：SR1107（Design → Planning → Implement，Full 需 design、plan、两轮独立 review 与人工计划 Gates）、SR1409（暴露度分档与仪式上限，unknown 按高暴露）、SR1408（RIGOROUS 边界与五类 `CORE_BLOCKING`）、SR1405.1（抽象需 ≥2 个已验证重复用例）、SR1403（最小充分路径）、SR1101/SR1102（不重复规范事实、保持设计简洁）、SR1106/SR1408（不猜测未决核心行为）。
- 安全：SR1301 安全核对清单逐项核对。
- Go：GR104（单文件 ≤1000 行）、GR106（单测用 `github.com/gookit/goutil/x/assert`）。
- 被评文档自述的核心验收：`docs/design/2026-10-17-rule-consolidation-design.md:14-24`「消除重复规则知识且不改变任何匹配结果」。

## 发现

### R-01 [HIGH] 暴露度判定无证据且被仓库证据反驳，Full 评审仪式被单轮缩减

- disposition: **CORE_BLOCKING**，类别 **AUTHORITY**
- 证据:
  - 被评文档 `:190`：「Delivery Track 判定：Full（…）；按 SR1409 属**低暴露度**（v0 未投产、可廉价回滚、无 Secret/部署动作），因此评审封顶一轮合并轴评审。」
  - 复核为真的反证：`git tag` 计数 = **52**；`git describe --tags` = `v1.14.1-1-g80a190f`，`git tag --points-at HEAD~1` = `v1.14.1`（candidate 的直接父提交即发布 tag）；`.github/workflows/release.yml` 在 `push.tags: v*` 上执行 `make release` 并经 `softprops/action-gh-release@v3` 上传资产；`README.md:31-40` 面向用户提供 `raw.githubusercontent.com/inherelab/eget/master/.github/install.sh`（及 `.ps1`）安装方式，两脚本均在库内。
  - 反方向证据（主会话复核补充）：`inhere-tools/eget/AGENTS.md` 写明「当前正在进行 v0 版本开发」；而 `v1.x` tag 序列承自上游 `zyedidia/eget` fork 历史，不能单独证明本项目已有生产用户。
  - 结论：暴露度的必要条件之一「**未投产且无生产用户/生产数据**」既未被文档举证，也无法由仓库证据确证为真 → 实际为 **unknown**。
- 影响: 按 SR1409「暴露度 unknown 时按高暴露处理」与 planning-core §Full「Full 需要 design、plan、**两轮独立 review** 和人工计划批准」，本设计应走两轴独立评审（必要时含 review ledger），而非单轮合并轴。文档以未举证的断言把评审仪式降档，等于缩减一项治理 Gate；而该设计自述的首要风险（`:21`「多处同伴规则漂移导致选错发布资产」）恰是两轴独立评审要防的面向。
- 建议: 二选一并在文档内留证：(a) 将 `:190` 的暴露度判定改为 **高暴露/unknown**，按 Full 执行两轴独立评审（本合并轴报告只计一轮）；（b) 若坚持低暴露度，需在文档内给出可核验证据，并说明「生产用户/生产数据」如何判定为无。**该判定属人类治理决策，评审者不得代为定档。**
- necessity proof:
  - core_acceptance: 设计自身 `:186-192`「结论与人工计划 Gate」——进入 plan 前必须满足其声明的 Delivery Track 与暴露度所对应的评审合同。
  - necessary_causal_chain: 不纠正本项 → 文档继续以「低暴露度」为唯一依据 → 只做一轮合并轴评审、不建 ledger、不做第二轴独立评审 → SR1409/planning-core 的 Full 评审要求被跳过 → 设计在治理 Gate 不完整的状态下被当作可进入 plan 的候选。
  - minimal_owner: 被评文档 `:190` 一节（暴露度判定与结论），无需触及任何代码或新增包。
  - deletion_alternative: **PASS**（撤回未举证的「v0 未投产」断言并按 SR1409 默认高暴露处理即可闭合）
  - scope_effect: 不新增 Module/Interface/Schema/state/lifecycle/external action，仅恢复 SR1409 要求的评审仪式。

### R-02 [MEDIUM] F2 枚举不完整：至少 5 处同类后缀判定未列入，filekind 迁移范围不足以达成可见目标

- disposition: CORE_CORRECTIVE
- 证据（文档 `:58` 只列 8 处；以下均已复核存在且未在所引行区间内）:
  - `internal/install/runner_download.go:163-169`：`case ".zip", ".gz", ".tgz", ".xz", ".bz2", ".zst", ".7z", ".rar":`（坏 HTML 缓存的归档后缀集合，且缺 `.tar.gz` 等多段扩展，已是漂移实例）
  - `internal/install/runner_select.go:230-232`：`for _, suffix := range []string{".exe", ".appimage"}`
  - `internal/install/runner_output.go:218-226`：`executableSuffix`（`.exe` / `.appimage`）
  - `internal/install/chooser.go:167`：`base == b.Tool+".exe" || base == b.Tool+".appimage"`
  - `internal/install/file_modes.go:69`：`isDefinitelyNotExec` 的 `.deb` / `.1` / `.txt`
- 影响: D2 的可见目标（`:20`「新增一种归档格式只需改一处知识表」）以 filekind 收口全部后缀知识为前提；只迁移所列 8 处时，上述站点仍各自持有同类常量，重复知识未消除，验收只部分达成。
- 建议: 扩充 F2 与迁移映射，逐站点标注「迁往 filekind」或「刻意保留并写明理由（如该站点有意使用更窄子集）」；对保留者加字段注释，避免二次漂移。

### R-03 [MEDIUM] `:117`「由 token 表生成或复用正则表」两个分支不等价，实施者可能选到改变行为的一支

- disposition: CORE_CORRECTIVE
- 证据:
  - `internal/install/runner_output.go:205-206` 的正则含 `armv?6|armv?7`（注意 `v?`），且**不含** `arm32`。
  - 三处 token 表为字面量且逐字相同（复核确认）：`internal/install/runner_platform.go:216`、`internal/client/cache_path.go:154`、`internal/app/cache/keep_latest.go:10` 均为 `...arm64, aarch64, arm32, armv6, armv7, arm, riscv64`，**不含**可匹配 `arm6` 的 `armv?6`。
  - `archAliases("arm")`（`internal/install/runner_platform.go:249`）返回 `{arm32, armv6, armv7, arm}`，同样无 `armv?6`。
- 影响: 若按「由 token 表生成」实现，`heuristicExecutableName` 对 `foo-linux-arm6` 一类名字的裁剪结果改变；若把 `armv?6` 塞进共享 token 表，则 `nameHasPlatform`（`internal/client/cache_path.go:130`）与 `trimPlatformSuffix`（`internal/app/cache/keep_latest.go:405`）的行为同时改变——两处都违反 `:24`「不改变任何匹配结果」。
- 建议: 迁移映射中删除「由 token 表生成」分支，明确 `runner_output.go:204-208` 三条正则**整表迁入 platform 并原样复用**（含大小写与锚点）。

### R-04 [MEDIUM] F4「4 套独立实现」夸大，且按流程图把 4 个入口统一接入 `filter.Parse` 会改变行为

- disposition: CORE_CORRECTIVE
- 证据:
  - 真正实现 `PRE:`/`SUF:` 的只有 2 处：`internal/tagfilter/tagfilter.go:34-53`、`internal/install/service.go:319-349`（`^` anti 另见 `internal/install/service.go:321-324`、`internal/install/chooser.go:128-135`）。此 2 处已满足 SR1405.1 的抽象前提。
  - `internal/install/chooser.go:57-60`：非 `REG:` 一律走 `NewGlobChooser`（gobwas/glob 语义）。
  - `internal/sdk/search_service.go:104-117`：非 `REG:` 走 `strings.Contains`（子串语义）。
- 影响: 若按 `:87`/`:138-145` 流程图字面接入，`--extract-file "foo*"`（现为 glob）与 sdk search 关键词（现为子串）会被改成 filter 的前缀/默认语义，直接违反非目标 `:24`。
- 建议: F4 收敛为「`PRE:`/`SUF:` 2 处重复 + `REG:` 前缀解析 4 处」；在架构/迁移映射显式写明 `chooser.go` 的 glob 默认值与 `search_service.go` 的子串默认值**不进入** filter，filter 只处理显式前缀与 `^`。

### R-05 [MEDIUM] filekind 未定义「大小写轴」，统一为一张表必然改变其中一侧行为

- disposition: CORE_CORRECTIVE
- 证据（同一名词下存在两种语义）:
  - 大小写敏感：`internal/install/extractor.go:57-78`（`strings.HasSuffix(filename, ".tar.gz")` 等，作用对象为 `internal/install/default_service.go:76-78` 传来的原始 asset 名）、`internal/install/file_modes.go:54,56,76`（`rename`/`isExec` 的 `.appimage`/`.exe`）
  - 先转小写：`internal/install/runner_output.go:31`、`internal/install/system7z.go:33`、`internal/client/cache_path.go:76`、`internal/app/cache/keep_latest.go:436`、`internal/sdk/json_index.go:223-230`
- 影响: 文档 `:106` 只声明 filekind 持有后缀集合，未声明每类判定的大小写语义。实现者若统一小写，`NewExtractor` 对 `TOOL.TAR.GZ` 会从「单文件解压」变为「tar 解包」；若统一敏感，`isExtractAllArchiveAsset` 等会变化。二者都违反等价性（`:33`）。
- 建议: 在 filekind 职责中显式加入大小写轴（表字段，或提供 `Xxx` / `XxxFold` 两组 API），并在迁移映射中逐站点标注沿用敏感或折叠。

### R-06 [MEDIUM] F6 漏列 `sourceforge` 的第 7 套版本解析/比较实现

- disposition: CORE_CORRECTIVE
- 证据: `internal/source/sourceforge/version.go:11`（`versionPattern = \d+(?:\.\d+)+`）、`:65-89`（`compareVersion`）、`:91-102`（`splitVersion`）；被 `internal/source/sourceforge/finder.go:413` 调用。文档 `:62` F6 只列 6 处，未含 `internal/source/**`。
- 影响: F6 是支撑 D5「本轮不整合」的事实表；漏项使 deferred 覆盖面不完整，且 `versionPattern` 与 `cacheVersionPattern` / `assetVersionPattern` / `versionTagPattern` 属同类版本正则漂移。
- 建议: 补入该证据并同步修订 D5 的 deferred 说明。

### R-07 [LOW] F3「3 份逐字重复」不准确：实为 2 份逐字 + 1 份等价 switch + 1 份漂移变体

- disposition: CORE_CORRECTIVE
- 证据: `internal/client/cache_path.go:77` 与 `internal/app/cache/keep_latest.go:437` 逐字相同；`internal/install/extractor.go:57-64` 是 `strings.HasSuffix` 的 switch（同扩展集，非列表）；`internal/sdk/json_index.go:224` 为第 4 个**不同**变体 `{".tar.gz",".tar.xz",".tar.bz2",".zip",".7z"}`（且未 ToLower，见 R-05）。
- 影响: 「逐字重复」措辞夸大其一；但 SR1405.1 前提仍由前两份逐字重复满足，结论不变。`json_index.go:224` 的漂移反而强化 filekind 必要性。
- 建议: F3 改为「2 份逐字重复 + 1 份等价 switch + 1 份漂移变体」并给出对应行号。

### R-08 [LOW] F8 包数「24 个」与实测 23 个不符

- disposition: CORE_CORRECTIVE
- 证据: `internal/**` 下含 `.go` 的目录实测 **23** 个（app、app/cache、app/web、cachemirror、cli、cli/prompts、cli/render、client、config、extpkg、install、install/detect、installed、sdk、source/forge、source/github、source/pkgtemplate、source/sourceforge、source/urltemplate、tagfilter、util、util/atomicfile、util/configutil）；`internal/source` 根目录无 `.go`，不计为包。
- 影响: 仅计数偏差；「无 `rules`/`platform`/`match` 类共享低层包」的实质结论已独立复核为真。
- 建议: 更正为 23 或说明计数口径。

### R-09 [LOW] sdk 内置配置中的 OS/Arch 命名映射未纳入枚举，边界需说明

- disposition: DEFERRED_ENHANCEMENT
- 证据: `internal/sdk/builtin_config.go:109,157-158` 的 `ArchMap`(`amd64→x64`,`arm64→aarch64`)、`OSMap`(`windows→win`,`darwin→macos`/`macosx`)。已复核存在。
- 影响: 属「OS/Arch 命名映射」知识，但职责是 URL 模板变量取值（用户可配置的 SDK 侧命名约定），与 F1 的「识别 token」不同类，不必然并入 platform。
- 建议: 在名词或 F8 补一句边界说明（识别 token ≠ URL 命名映射），避免下游误计或漏计。

### R-10 [LOW] 决策已定（D3/D1）而关键待确认项（Q1/Q3）仍能改变其形态

- disposition: CORE_CORRECTIVE
- 证据: 文档 `:169` D3 写「`tagfilter` 并入（见 Q1）」而 `:180` Q1 未决；`:167` D1 的 platform 边界与 `:182` Q3 相关；`:37-41` 已把「建立 3 个新低层包」写入范围。
- 影响: 文档自称 Draft 不授权规划（`:192`），风险有限；但下游若把范围/决策当作冻结 scope，会与 Q1 的答案冲突。
- 建议: 在范围/决策前标注「3 包边界与 D3 形态待 Q1–Q3 收敛后冻结」，不改正文结论（符合 SR1106/SR1408）。

## 覆盖缺口与限制

- 未运行 `go build` / `go test`：等价性判断均基于源码静态阅读，不构成行为证明。
- 未穷举 `internal/**` 全部文件（定向 `rg` + 重点通读），五类知识可能仍有未发现站点；`cmd/`、`web/`、`internal/app/web/**` 未纳入。
- 「无生产用户/生产数据」无法从仓库证实（无遥测可查），只能以 tag / 发布流水线 / 安装脚本作反证 —— 这正是 R-01 需要人类治理定档的原因。
- 本报告为**合并轴**单轮评审；若 R-01 判定为高暴露，则本报告不构成两轴独立评审，需补第二轴（连同 ledger，如适用）。
- SR1301 安全核对清单逐项：鉴权（认证与授权）**不适用**（纯后缀/别名知识的内部重构，无鉴权面）；注入防护 **已核对**（文档 `:157` 保留 `regexp.Compile` 错误返回、禁止对用户输入用 `MustCompile`，与现状一致：`internal/tagfilter/tagfilter.go:39`、`internal/install/service.go:327` 用 `Compile`；`tagfilter.go:46,49,51` 的 `MustCompile` 仅作用于 `regexp.QuoteMeta` 转义后的字面量）；秘密不进日志/输出/仓库 **已核对**（无 Secret 与凭据路径改动）；入参校验 **已核对**（`REG:` 编译错误仍返回）；资源归属校验（防越权）**不适用**。

## 结论

**BLOCKED** —— 存在 1 项 `CORE_BLOCKING`：**R-01 [HIGH][AUTHORITY]**。

被评文档 `:190` 以「v0 未投产」为由把 Full 降为单轮合并轴评审，该前提未举证，且仓库证据两侧并存（52 个 tag、`v1.14.1` 为 candidate 直接父提交、在用发布流水线与面向用户的安装脚本；另一侧 `AGENTS.md` 的「v0 版本开发」与 v1.x tag 承自上游 fork）。按 SR1409「unknown 按高暴露处理」与 planning-core §Full，暴露度未定档前不得使用单轮封顶。

已复核为真的其余部分：F1 行号准确且三处 token 数组逐字相同；F5、F7、F10 成立；Q1（`tagfilter` 全仓仅 `internal/source/github/finder.go:109,204` 与 `internal/app/tag_semantics.go:42,51` 共 4 处调用）、Q2（`internal/client/cache_path.go` 仅依赖 stdlib，`client → platform` 无环）、Q3（三种「含平台」语义确实不同）、D8 无环、GR104/GR106 现状达标。

R-02~R-06 为 MEDIUM 且均属「不先修正事实表与迁移映射，实施期等价性验收可能失败」，不单独构成阻断，但在 plan 阶段应写成显式验收项。

## 剩余风险与后续

1. **R-01 未关闭前不得进入 plan Gate。** 关闭方式二选一：删除未举证的暴露度断言并按高暴露走两轴独立评审与 ledger；或补齐「低暴露度」的可核验证据。该定档属人类治理决策。
2. R-02/R-03/R-04/R-05 应在 plan 中落为显式验收项：迁移站点全量清单、正则整表迁移、四个过滤入口的默认语义边界、filekind 大小写轴。
3. R-06/R-07/R-08/R-09/R-10 属事实表与范围表述准确性，随下次修订一并更正即可，不新开评审轮次（SR1406）。
4. Q1–Q3 建议在进入 plan 前收敛并留痕，避免 Draft 范围被下游当作已批准 scope。
5. 本报告为 `REPORT_ONLY`：未修改被评文档。若要求修复，需另行授权并保留本报告与处置记录。
