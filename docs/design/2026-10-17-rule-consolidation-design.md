<!-- template_id: design; template_version: 1.1.1 -->
# eget 匹配与转换规则整合 设计

> 状态：Draft 0.1 / 待人工决策批准

## 修订记录

| 版本 | 日期 | 作者 | 摘要 |
|---|---|---|---|
| 0.1 | 2026-10-17 | inhere | 初稿：确认规则散落事实，给出平台/文件类型/过滤三类知识的集中化方案 |

> 仅语义变化递增版本；纯 identity/provenance/元数据纠正沿用原版本，并在 Git/进度记录中留痕。

## 背景与目标

`eget` 的「匹配、转换规则」当前分散在 `internal/` 多个包里，同一份领域知识存在多份实现，且互相之间没有共享层。已确认的重复分布见「已确认事实」表。

用户可见目标：

- 新增一个目标平台/架构（例如 `loong64`）、一种归档格式、一种过滤语法时，只需要改一处知识表。
- 消除「多处同伴规则漂移」导致选错发布资产（装错平台、把校验文件当安装包）的风险。
- 让这些规则可被单元测试集中覆盖，而不是在 4~5 个包里各测一份。

非可见目标：本设计不改变任何命令行为、匹配结果和配置格式。

## 名词

- **别名（alias）**：同一平台概念的多种写法，例如 `amd64` 的 `x64` / `x86_64`。
- **token**：把文件名按 `-_.` 等分隔符切分后的片段；别名匹配以 token 为单位，避免子串误命中。
- **规则数据（data）**：别名表、token 表、后缀分类表等「事实」。
- **判定策略（policy）**：某调用点在拿到规则数据后如何决策，例如 detector 的 `anti`/`priority` 语义、`trimPlatformSuffix` 的「连续 os-arch 后缀」语义、`nameHasPlatform` 的「含任意 token」语义。
- **release asset 分类**：把发布资产判定为可安装包 / 校验和 / 签名 / 元数据。
- **等价性**：迁移前后对同一输入的输出完全相同。

## 范围与非目标

范围：

- `inhere-tools/eget/internal` 下的规则数据与规则解析层。
- 建立 3 个新的低层包：`internal/platform`、`internal/filekind`、`internal/filter`。
- 把既有调用点改为消费新包，并删除被替代的重复定义。

非目标：

- 不改变 CLI 参数、配置文件字段、命令输出与任何匹配结果。
- 不引入运行时规则文件 / 插件 / 规则引擎（见 D6）。
- 不整合版本解析与比较规则（见 D5、Q4）。
- 不调整 `install/archive`、`chooser`、`system7z` 的包结构（`docs/plans/go-subpackage-utility-plan.md` 阶段 5 已评估暂缓，本设计不重启该话题）。
- 不改动 `web/`、`sdk` 的索引模型与安装流程。

## 已确认事实与规范

事实（均为当前 HEAD 的只读证据）：

| 编号 | 事实 | 证据位置 |
|---|---|---|
| F1 | OS 别名/token 表存在 5 份实现，其中 3 份 token 数组逐字相同 | `internal/install/detect/detectors.go:302-343`、`internal/install/runner_platform.go:192-252`、`internal/client/cache_path.go:153-154`、`internal/app/cache/keep_latest.go:9-10`、`internal/install/runner_output.go:204-208` |
| F2 | 归档/可执行后缀判定分散在 8 处 | `internal/install/extractor.go:56-81`、`internal/install/runner_output.go:30-42`、`internal/install/system7z.go:32-46`、`internal/install/file_modes.go:51-76`、`internal/install/gui.go:44-50`、`internal/client/cache_path.go:75-83`、`internal/app/cache/keep_latest.go:435-443`、`internal/sdk/json_index.go:224` |
| F3 | 多段扩展名列表 `.tar.gz/.tar.xz/...` 有 3 份逐字重复 | `internal/client/cache_path.go:77`、`internal/app/cache/keep_latest.go:437`、`internal/install/extractor.go:57-64` |
| F4 | `PRE:`/`SUF:`/`REG:`/`^` 过滤语法有 4 套独立实现 | `internal/tagfilter/tagfilter.go:25-53`、`internal/install/service.go:319-349`、`internal/install/chooser.go:57-71`、`internal/sdk/search_service.go:105-106` |
| F5 | release asset 分类是 30+ 后缀硬编码 switch | `internal/install/detect/detectors.go:259-300` |
| F6 | 版本解析/比较规则有 5~6 套实现 | `internal/app/cache/keep_latest.go:88-279`、`internal/sdk/html_index.go:245-287`、`internal/install/runner_select.go:188-223`、`internal/install/runner_platform.go:218-227`、`internal/app/tag_semantics.go:12`、`internal/client/cache_path.go:156-163` |
| F7 | `{name}{version}{os}{arch}{ext}` 模板语法有 3 套实现（2 正向 + 1 逆向解析） | `internal/source/urltemplate/template.go:60-75`、`internal/sdk/template.go:16-29`、`internal/sdk/html_index.go:187-218` |
| F8 | 上述 5 组知识分属 `detect`/`install`/`client`/`app/cache`/`sdk` 五个包，当前无任何共享低层包 | `internal/` 下 24 个包已枚举，无 `rules`/`platform`/`match` 类包 |
| F9 | F1 中三份 token 数组已逐字相同，属已发生的复制粘贴；F3、F4 同类 | 同 F1、F3、F4 |
| F10 | `tagfilter` 包的文档注释自述其语法 `mirrors the --asset filter grammar`，但实现是复制而非共享 | `internal/tagfilter/tagfilter.go:1-8` |

规范与约束：

- 本设计遵循 SR1403（不做→复用→stdlib→最小实现）、SR1405.1（抽象需 ≥2 个已验证重复用例或用户明确要求平台化）、SR1107（Design → Planning → Implement）、SR1409（治理暴露度）、SR1101（不重复中央规范正文）。F9 提供 ≥2 个已验证重复用例，满足 SR1405.1 的抽象前提。
- 实施期遵循 GR104（单文件 ≤1000 行）、GR106（测试使用 `goutil/x/assert`）。
- 无对中央规范的偏离。

## 总体方案

核心判断：这组问题**不是文件过大，而是同一领域知识跨包散落**，因此解法是「抽数据、留策略」，而不是继续拆文件或引入配置文件。

三件事：

1. **抽数据到低层包，策略留在原调用点。**
   `internal/platform` 只回答「某个平台概念有哪些别名 / 某段文本是否含平台 token / 某 OS 的识别正则是什么」；`anti`、`priority`、`连续后缀`、`含任意 token` 等判定语义仍由各调用点持有。这样可以消除重复数据而**不改变行为**（等价性由现有测试保护）。

2. **把"文件名 → 类型"变成一张表。**
   `internal/filekind` 集中持有：多段扩展名、归档扩展名集合、可执行后缀、安装器后缀、release asset 分类后缀。`extractor.go` 只保留「类型 → 构造函数」的映射（因为构造函数属于 `install` 包，不能下移）。

3. **把过滤语法收敛成一个解析器。**
   `internal/filter` 提供统一的 `PRE:`/`SUF:`/`REG:`/`^` 解析，输出中间结构；`tagfilter`、asset filter、file chooser、sdk search 四个入口改为复用它。

## 架构

依赖方向（只允许向上依赖，禁止反向，避免 import cycle）：

```text
        app / app-cache ─┐
        install ─────────┼──► platform
        client ──────────┤    filekind
        sdk ─────────────┘    filter
        install/detect ──┘
```

包职责：

| 新包 | 职责 | 明确不做 |
|---|---|---|
| `internal/platform` | OS/Arch 别名与 token 表、`KnownOS`、token 切分、OS 识别正则表（含 anti/priority 数据）、arch 目录选择模式 | 不做判定策略、不做资产分类、不读配置 |
| `internal/filekind` | 多段扩展名提取、归档/可执行/安装器后缀集合、release asset 分类 | 不构造解压器、不碰文件系统 |
| `internal/filter` | `PRE:`/`SUF:`/`REG:`/`^` 语法解析、`IsPattern`、编译为 matcher | 不绑定 tag/asset/文件名任一语义、不做匹配 |

迁移映射（旧 → 新）：

| 旧位置 | 迁往 | 保留在原处的策略 |
|---|---|---|
| `detect/detectors.go` OS/Arch 正则与 map | `platform`（作为数据表） | `systemDetector.Detect` 的 priority/matches/candidates 流程 |
| `install/runner_platform.go` tokens/aliases/archSelectionPatterns | `platform` | `archiveNameMatchesPlatform`、`autoSelectWindowsMSVCAsset` |
| `client/cache_path.go` cacheOSTokens/cacheArchTokens | `platform` | `nameHasPlatform`、`cachePlatform` |
| `app/cache/keep_latest.go` keepLatest*Tokens | `platform` | `trimPlatformSuffix` 的连续后缀语义 |
| `install/runner_output.go` 三条 os/arch 正则 | `platform`（由 token 表生成或复用正则表） | `heuristicExecutableName` 的输出名裁剪 |
| `client/cache_path.go` archiveExt、`keep_latest.go` cacheAssetExt | `filekind` | 缓存文件命名与 family 归一化 |
| `install/extractor.go` 后缀 switch | `filekind` 判类型 + `install` 内构造映射 | 解压器构造 |
| `install/system7z.go` shouldUseSystem7z、`file_modes.go`、`gui.go` 后缀判定 | `filekind` | 走系统 7z / 可执行位 / GUI 安装模式决策 |
| `detect/detectors.go` classifyReleaseAsset | `filekind`（表驱动） | `SelectableReleaseAssets` 过滤 |
| `tagfilter`、`install/service.go` parseAssetFilter、`chooser.go`、`sdk/search_service.go` | `filter` 解析 | 各自 matcher 的语义与错误文案 |

## 关键流程

### 流程 1：发布资产选择（install）

```text
releases assets
  → filekind.Classify 过滤掉 metadata/checksum/signature
  → 若指定 --asset：filter.Parse 生成 matcher，串成 detector chain
  → detect.systemDetector 用 platform 的 OS/Arch 正则做 osMatch/archMatch
      · priority 命中优先（如 .exe / .appimage）
      · linux 多变体时用 libc token 选择
  → 唯一候选则选中，否则进入选择/回退
```

### 流程 2：过滤表达式解析（4 个入口 → 1 个解析器）

```text
--tag "PRE:app-v"        ─┐
--asset "^REG:\\.sha256$" ├─► filter.Parse → Matcher{Kind, Expr, Anti, Regex}
--extract-file "REG:x"   ┤        │
sdk search "REG:^22"     ─┘        └─ 调用方决定匹配对象（tag / 文件名 / 关键词）
```

### 流程 3：缓存与清理中的平台判定

```text
下载命名：cache_path.nameHasPlatform  → platform 的 token 表
清理保留：keep_latest.trimPlatformSuffix → platform 的 token 表 + 本地连续后缀策略
输出命名：runner_output 裁剪 os/arch/版本后缀 → platform + 本地版本 token 策略
```

## 安全、数据、运维与回滚

- 安全：无 Secret、无凭据路径改动；`filter` 解析正则时保持现有 `regexp.Compile` 错误返回（不引入 `MustCompile` 到用户输入路径）。
- 数据：不改缓存 layout、installed store、`sdk.installed.json` 结构与 key 计算。
- 运维：纯内部重构，无迁移、无部署、无出机器动作；`eget.exe` 行为不变。
- 回滚：每个阶段独立提交，回滚为 revert 对应提交；阶段之间可独立发布。
- 等价性护栏：每个迁移阶段都必须先补齐「同一输入 → 同一输出」的表驱动测试，再替换调用点。

## 决策

| 编号 | 决策 | 理由 |
|---|---|---|
| D1 | 建立 `internal/platform`，作为 OS/Arch 别名与识别正则的单一真源；删除 4 份重复 token/别名表 | F1、F9；满足 SR1405.1 |
| D2 | 建立 `internal/filekind`，把后缀集合与 release asset 分类改为表驱动；`install` 只保留「类型 → 构造函数」映射 | F2、F3、F5 |
| D3 | 建立 `internal/filter`，统一 `PRE:`/`SUF:`/`REG:`/`^` 语法；`tagfilter` 并入（见 Q1） | F4、F10 |
| D4 | 只做「数据集中」，不下移判定策略；判定语义保持在原包 | 保证等价性、避免大爆炸式改写 |
| D5 | 版本解析/比较规则（F6）本轮**不整合**，记为 deferred | 各调用点模型不同（asset family / sdk index / tag 语义），统一需先决定单一模型，属独立设计 |
| D6 | **不**引入运行时规则文件/规则引擎，保持编译期表 | 无扩展性需求证据；SR1405.1 要求平台化需用户明确要求 |
| D7 | 模板语法（F7）本轮**不整合**，记为 deferred | 属渲染层，与本设计的三类知识正交；`urltemplate` 与 `sdk` 的生命周期不同 |
| D8 | 新包只能依赖 stdlib 与既有低层 `util`，禁止 import `install`/`app` | 防止 import cycle |

## 待确认事项

| 编号 | 问题 | 建议 |
|---|---|---|
| Q1 | `internal/tagfilter` 是删除并改调用点，还是保留为 `filter` 的薄 facade？ | 建议**删除**：内部包、调用点少、facade 会变成第二真源 |
| Q2 | `internal/client/cache_path.go` 目前无低层依赖，允许它 import 新包吗？ | 建议允许，方向为 `client → platform`，无环 |
| Q3 | 三处「含平台」语义（token 包含 / 连续 os-arch 后缀 / detector anti-priority）确认保留为三个独立函数？ | 建议保留；`platform` 只提供数据与切分原语 |
| Q4 | F6 版本规则与 F7 模板语法是否要合并进本轮范围？ | 建议本轮不做，另立设计 |
| Q5 | 是否需要为规则知识补一份 `docs/` 说明（例如新增平台要改哪里）？ | 建议在实施完成后补一节，不单独建设计 |

## 结论与人工计划 Gate

结论：规则散落事实成立（F1~F10），且不是文件规模问题。方案是抽 3 个低层数据/解析包并保持判定策略原地不动，可保证行为等价、阶段可独立回滚。

Delivery Track 判定：Full（涉及跨 Module 内部 Interface、业务文件 >5、业务逻辑行 >300）；按 SR1409 属**低暴露度**（v0 未投产、可廉价回滚、无 Secret/部署动作），因此评审封顶一轮合并轴评审。

**本设计为 Draft，不授权规划，也不授权实施。** 需要用户明确批准后才进入实施计划（plan）编写；plan 完成后需要单独的人工计划批准 Gate 才可开始改动代码。
