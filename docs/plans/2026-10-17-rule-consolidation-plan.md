<!-- template_id: plan; template_version: 1.2.0 -->
# eget 匹配与转换规则整合 实施计划

> 状态：Draft 0.1 / 待人工计划批准

## 修订记录

| 版本 | 日期 | 作者 | 摘要 |
|---|---|---|---|
| 0.1 | 2026-10-17 | inhere | 初稿：按已批准 design 0.2 拆出 filekind/platform/filter 三包的等价性迁移波次；闭合 X-01/X-02/X-03 站点清单与大小写轴；把 F3 漂移修复与「不改匹配结果」的冲突显式推迟 |

> 仅语义变化递增版本；纯 identity/provenance/元数据纠正沿用原版本，并在 Git/进度记录中留痕。

## 思考模式与 scope freeze

- `thinking_mode=RIGOROUS`（实施计划的编写、审核、问题修复只允许本模式）。
- `core_objective`：把已批准的 design 0.2 方案拆成**行为等价、逐波次可独立回滚**的任务，使「新增平台/归档格式/过滤语法只改一处知识表」成立。
- `allowed_scope`：`inhere-tools/eget/internal` 下新增 `platform`、`filekind`、`filter` 三个低层包，并把设计点名的既有调用点改为消费新包；删除被替代的重复定义。
- `non_goals`：不改变任何文件名匹配结果、CLI 参数、配置字段、命令输出、缓存 layout；不引入运行时规则文件/规则引擎；不整合 F6（版本解析）与 F7（模板语法）。
- `expansion_policy=DEFER_OR_REQUEST`：新站点、新包、新外部协议只在直接追溯到核心验收时才进入本计划，否则记入「完成 Gate 与剩余工作」。
- `review_budget`：Delivery Track=Full、暴露度=高；两轴独立评审（Standards/Governance、Spec/Executability）各 1 轮 discovery/confirmation + 仅在有 `CORE_BLOCKING` 时最多 4 轮 blocking closure，上限 8 轮。
- 停止条件：任一站点等价性用例变红；迁移必须改变集合或大小写轴才能进行；发现 design 内部矛盾；出现 ownership conflict。

## 目标与完成定义

目标：`eget/internal` 的「文件名 → 类型」「平台别名与 token」「过滤语法」三类知识各只有一处真源，既有调用点全部改为消费新包，且迁移前后对同一输入输出完全相同。

完成定义（全部可观察）：

1. `internal/platform`、`internal/filekind`、`internal/filter` 三个包存在，且只 import stdlib 与 `internal/util`（`go list -deps` 可证）。
2. 本计划「站点清单」表中决策为「迁移」的每一行，其原位置不再出现该后缀字面量或本地集合定义（grep 断言逐行可证）。
3. `go build ./...`、`go vet ./internal/...`、`go test ./...` 全绿。
4. 每站点至少一组「同输入 → 同输出」的 characterization 用例（含大写变体），且迁移过程中断言未被放宽。
5. `internal/tagfilter` 包被删除，其 4 个调用点改用 `internal/filter`。

## 范围、排除项与授权

范围：design 0.2「迁移映射」表点名的全部站点，加上设计评审 X-01/X-02/X-03 要求补齐的站点（本计划「站点清单」为权威清单）。

排除项（明确不做，理由随行）：

| 排除项 | 理由 / 留痕 |
|---|---|
| F6 版本解析与比较（7 套） | design D5 deferred，需先定单一模型 |
| F7 `{name}{version}{os}{arch}{ext}` 模板语法（3 套） | design D7 deferred，属渲染层 |
| glob 默认语义（`chooser.go:57-60`）与子串默认语义（`search_service.go:117`） | design 非目标；仅 `REG:`/`PRE:`/`SUF:`/`^` 显式前缀走 `filter` |
| 修复 F3 的「漂移集合」（`sdk/json_index.go:223-231` 缺 `.tar.zst/.tar.br/.tar.lz4`、`json_index.go:121-144` 未折叠大小写） | **本计划保留现状**（见「等价性判定规则」）。补齐即改变匹配结果，与 design 非目标冲突，需一次单独的语义变更批准 |
| `sdk/builtin_config.go:109,157-158` 的 `OSMap`/`ArchMap` | 属 URL 模板取值约定，非识别知识（design 名词表） |
| `install/archive_formats.go:107-123` 的内容嗅探、`install/archive_paths.go` 路径安全 | 不是文件名知识 |
| `app/cache/service.go:343-358` 缓存条目分类、`source/urltemplate/finder.go:117-132` 元数据格式 | 属缓存/元数据域，非本计划三类知识 |
| `web/`、`cmd/`、`internal/app/web/**`、sdk 索引模型与安装流程 | design 非目标 |
| 为规则知识补 `docs/` 说明（Q5） | 未在 Q1–Q3 收敛范围内；列为完成后的候选剩余工作 |

授权边界：本计划只授权**计划批准后的一次执行请求**内的本地代码改动与本地提交。不含 push、release、部署、迁移或任何出机器动作；不含修改 `go.mod` 依赖。

- `host_or_non_offline_action=NOT_APPLICABLE`

## 输入与批准证据

| 项 | 值 |
|---|---|
| 设计文档 | `docs/design/2026-10-17-rule-consolidation-design.md`（Draft 0.2） |
| 设计评审（第 1 轮，0.1，BLOCKED） | `docs/review/2026-10-17-rule-consolidation-design-review.md` |
| 设计评审（第 2 轮，0.2，两轴 PASS） | `docs/review/rule-consolidation-design/r02-0.2-standards-review.md`、`r02-0.2-executability-review.md`、`review-ledger.json` |
| 设计候选 | commit `5146d5c32cfe070e27ea6c39b4f812dc4e8d5156`，revision `0.2` |
| 设计批准证据 | 用户于本会话明确批准：「Q1–Q3 按建议收敛，批准设计 0.2」 |
| Q1–Q3 收敛留痕 | Q1=删除 `internal/tagfilter`（不留 facade）→ T11/T12；Q2=允许 `client` import 新包（方向 `client → platform/filekind`，无环）→ T07；Q3=保留三个独立「含平台」判定函数（`nameHasPlatform`、`trimPlatformSuffix`、`systemDetector.Detect` 的 anti/priority）→ T10。收敛结果与 design 内联「建议」一致，故 design 仍为 0.2，scope 自此视为冻结 |

## 等价性判定规则

本计划的全部等价性论证依赖以下三条规则，任一条被打破即按 implementation-discovery 分类处理：

1. **站点 = (后缀集合, 大小写轴) 二元组**。迁移保持两者都不变：集合不变、轴不变。`filekind` 因此不导出「唯一的归档后缀集合」，而导出**具名集合** + 两种轴 API：
   - `filekind.ExtSetExtractor`（`extractor.go` 的 14 项：`.tar`/`.tar.gz`/`.tgz`/`.tar.bz2`/`.tbz`/`.tar.xz`/`.txz`/`.tar.zst`/`.zip`/`.7z`/`.gz`/`.bz2`/`.xz`/`.zst`，含单文件压缩器）
   - `filekind.ExtSetExtractAll`（`runner_output.go:30-42` 的 10 项，不含单文件压缩器）
   - `filekind.ExtSetCacheMulti`（`.tar.gz`/`.tar.xz`/`.tar.bz2`/`.tar.zst`/`.tar.br`/`.tar.lz4`，供 `cache_path`/`keep_latest`）
   - `filekind.ExtSetDownload`（`.zip`/`.gz`/`.tgz`/`.xz`/`.bz2`/`.zst`/`.7z`/`.rar`，供 `runner_download`）
   - `filekind.ExtSetArchiveLegacy`（`.tar.gz`/`.tar.xz`/`.tar.bz2`/`.zip`/`.7z`，**仅** `sdk/json_index.go` 使用，注释标注「等价性保留，漂移待另一次语义变更」）
   - `filekind.ExtSetSystemArchiver`（`.7z`/`.rar`/`.msi`/`.cab`/`.iso`/`.exe`，供 `system7z` 与 `service.requiresSystem7z`）
2. **大小写轴显式**：`Xxx(name)`=大小写敏感（原样 `HasSuffix`），`XxxFold(name)`=先 `ToLower` 再判定。站点原有轴不得互换。
3. **每站点用例**：至少 1 个正例、1 个反例、1 个全大写变体。迁移前先在原包写成红/绿可重复的 characterization 用例，迁移后**同一断言**必须保持绿；不得为了迁就实现而修改断言。

## Capability Discovery

### Capability decisions

| capability_id | required_capability | searched_candidates | direct_reuse | thin_adapter_or_owner_extension | decision | proven_gap | duplication_and_lifecycle_risk |
|---|---|---|---|---|---|---|---|
| CAP-01 | 文件名 → 归档/可执行/安装器类型与 release asset 分类的单一数据源 | `internal/util`、`internal/install`（extractor/system7z/gui/file_modes）、`internal/client`、`internal/app/cache`、`internal/sdk`、`internal/install/detect`、`goutil`、`gookit/ext` | 无：23 个内部包无任何 `filekind`/`rules` 类共享层（design F8），`util` 仅含字符串/路径工具 | 无：现有站点是同一知识的多份复制，没有可扩展的既有 owner | `MINIMAL_NEW_MODULE` | design F2/F3/F5 + F9 的两个已验证重复用例（`.tar.gz` 2 份逐字重复 + 2 份等价/漂移变体；`.exe` 12 站点） | 若只在一处扩展会让其余站点继续漂移；新包只依赖 stdlib/util，生命周期随 eget 内部；删除测试见 Rejected new tools |
| CAP-02 | OS/Arch 别名、token 表与识别正则的单一数据源 | `internal/install/detect`、`internal/install/runner_platform.go`、`internal/client/cache_path.go`、`internal/app/cache/keep_latest.go`、`internal/install/runner_output.go`、`goutil` | 无：`runner_platform.go:215-218`、`cache_path.go:153-154`、`keep_latest.go:9-10` 三处 `osTokens/archTokens` 逐字相同 | 无：三处都是包内私有变量，无可扩展 owner | `MINIMAL_NEW_MODULE` | design F1/F9（3 份逐字相同的 token 数组） | 同上；Q3 明确不下移判定策略，避免策略与数据耦合 |
| CAP-03 | `PRE:`/`SUF:`/`REG:`/`^` 语法解析的单一实现 | `internal/tagfilter`、`internal/install/service.go`、`internal/install/chooser.go`、`internal/sdk/search_service.go`、stdlib `regexp` | `internal/tagfilter` 是本仓库唯一独立解析器，但只面向 tag，`service.go` 另有一套 | 可行但被否决：把 `tagfilter` 扩成通用解析器并让 asset/chooser/search 依赖它，会形成「tag 语义 + 通用语法」双职责包 | `MINIMAL_NEW_MODULE` | design F4/F10（`tagfilter` 文档注释自述与 asset filter 语法相同却复制实现） | tagfilter 删除后调用点 4 处改指向 filter；`filter` 不绑定任何匹配对象语义 |
| CAP-04 | 表驱动测试断言 | `goutil/x/assert`（已在 go.mod，仓库已在用） | 是（GR106 指定） | 不适用 | `DIRECT_REUSE` | 无 | 无新增依赖 |
| CAP-05 | 解包成员的 glob 默认匹配（`--extract-file` 默认语义） | `github.com/gobwas/glob`（已在 go.mod，`chooser.go` 在用） | 是 | 不适用 | `DIRECT_REUSE` | 无 | 默认语义不迁移，避免行为变化 |
| CAP-06 | 正则编译与字面量转义 | stdlib `regexp`（`Compile` / `MustCompile` / `QuoteMeta`） | 是 | 不适用 | `DIRECT_REUSE` | 无 | 用户输入路径继续 `Compile` 返回错误，`MustCompile` 仅用于 `QuoteMeta` 后的字面量 |
| CAP-07 | 归档 → 解压器构造函数映射 | `internal/install/archive_formats.go`（`NewTarArchive`/`NewZipArchive`/`NewSevenZipArchive`）、`extractor.go:50-54` gunzip 闭包 | 是（保留在 `install`） | 不适用 | `NOT_NEEDED` | 刻意不迁：构造函数与 `install` 内部类型耦合，下移会形成反向依赖 | 无 |

### New module candidates

| candidate_id | capability_id | proposed_module | searched_candidates | direct_reuse_gap | thin_adapter_or_owner_extension_gap | proven_gap | unique_owner_and_lifecycle | deletion_or_merge_handling |
|---|---|---|---|---|---|---|---|---|
| C-01 | CAP-01 | `internal/filekind` | `internal/util`、`internal/install/*`、`internal/client`、`internal/app/cache`、`internal/sdk`、`internal/install/detect`、`goutil`、`gookit/ext` | 无包同时持有后缀集合与 release asset 分类；`util` 无后缀知识 | 无可扩展 owner：所有站点均为包内私有 switch/集合，扩展任一处不覆盖其余 11 处 | design F2/F3/F5 + F9；本计划「站点清单」中 S-A01~S-A08、S-E01~S-E12、S-R01 共 21 个站点、13 份重复 | owner：`internal/filekind`（无 CLI/无状态/无 IO）；lifecycle：随 eget 内部包演进，不对外暴露 | 若评审否证，删除即回到各站点本地集合（revert 对应波次 commit）；不与 `install` 合并（会形成 `client → install` 反向边） |
| C-02 | CAP-02 | `internal/platform` | `internal/install/detect`、`internal/install/runner_platform.go`、`internal/client/cache_path.go`、`internal/app/cache/keep_latest.go`、`internal/util` | 无共享平台数据；三处 token 数组逐字重复 | 无可扩展 owner：`detect` 只服务 detector，`runner_platform` 只服务 install | design F1/F9；3 份逐字重复的 `osTokens/archTokens` | owner：`internal/platform`（纯数据 + 切分原语，无判定策略，Q3）；lifecycle：随 eget 内部包演进 | 若否证，删除后各站点恢复本地表（revert W5 commit）；不与 `detect` 合并（会形成 `client/app-cache → install/detect` 反向边） |
| C-03 | CAP-03 | `internal/filter` | `internal/tagfilter`、`internal/install/service.go:319-349`、`internal/install/chooser.go`、`internal/sdk/search_service.go`、stdlib `regexp` | `tagfilter` 只解析 tag 语法；其余三处各自内联解析 | 可行但会造成双职责包（见 CAP-03 否决理由） | design F4/F10；`PRE:`/`SUF:` 两套同构实现 | owner：`internal/filter`（只解析、不匹配，Q3 同理）；lifecycle：随 eget 内部包演进 | 若否证，保留 `tagfilter` 并只迁移 `service.go` 一处（revert W6 commit）；不保留 facade（Q1） |

### Rejected new tools

| rejected_candidate | capability_id | deletion_test_and_reason |
|---|---|---|
| 单一 `internal/rules` 包（合并 platform+filekind+filter） | CAP-01 | 删除测试：三者的消费者集合互不重叠（`filekind` 被 install/client/app-cache/sdk 消费，`platform` 被 install/client/app-cache 消费，`filter` 被 app/source-github/install/sdk 消费），合并会让「只改一处知识表」的粒度变粗且任一消费者被迫引入无关依赖；3 个薄包更符合 SR1403 |
| 运行时规则文件 / 规则引擎（YAML/JSON 驱动后缀表） | CAP-01 | 删除测试：无扩展性需求证据（design D6）；引入后需 schema、加载失败路径、发布期校验三套机制，且会让「编译期常量」变成运行期可能缺失的事实源；SR1405.1 要求平台化需用户明确要求 |
| 把 `internal/tagfilter` 保留为 `filter` 的薄 facade | CAP-03 | 删除测试：facade 会成为第二真源（4 个调用点 + 包内注释自述语法镜像），且无独立行为可测；design Q1 已判定删除 |
| 新增 `go.mod` 依赖（如 `samber/lo`、通用规则库） | CAP-02 | 删除测试：所需能力均可由 stdlib `strings`/`path`/`regexp` 与既有 `goutil`/`gobwas/glob` 覆盖 |

## 前置检查与 fail-closed 条件

### workspace baseline（mutation 保护快照）

| 项 | 值 |
|---|---|
| Git root | `D:\work\inhere\my-tools-dev\inhere-tools\eget` |
| branch / HEAD | `master` / `6a578d9da2d4e73c82f48f10e8f14f3089b5c4a2` |
| `git status --short` | 空（0 行 dirty/untracked） |
| 模块 | `module github.com/inherelab/eget`，`go 1.25.0`；工具链 `go1.25.10 windows/amd64` |
| 预跑基线 | `go build ./...` exit 0；`go test ./internal/...` exit 0（全 `ok`，2026-10-17 预跑） |
| 预期触及路径 | `internal/filekind/`、`internal/platform/`、`internal/filter/`（新增）；`internal/install/`、`internal/client/`、`internal/app/cache/`、`internal/sdk/`、`internal/install/detect/`、`internal/source/github/`、`internal/app/`（改造）；`internal/tagfilter/`（删除） |
| 依赖与授权 | 不新增 go.mod 依赖；本地提交授权（按波次提交）；push/release 未授权 |

### 前置检查（执行前逐项确认，任一不满足即停）

1. HEAD 仍为 `6a578d9` 且工作树干净；若已有他人 dirty 改动触及上述路径 → `Ownership Conflict`，停止。
2. `go build ./...` 与 `go test ./internal/...` 在本分支全绿。
3. 站点清单中每行的 `path:line` 仍能定位到所述函数；行号漂移但符号一致属 `Operational Discovery`，直接更新清单继续。
4. `internal/tagfilter` 的 4 个调用点仍为 `internal/app/tag_semantics.go:42,51` 与 `internal/source/github/finder.go:109,204`。

### fail-closed 条件

- characterization 用例变红，或必须修改断言才能通过 → 停止该站点，`git revert` 该站点的改动，重新判定（若需改行为 = `Semantic Amendment`，返回 plan 评审与人工 Gate）。
- 迁移某站点需要改变后缀集合或大小写轴 → 同上。
- 发现设计矛盾（例如要求统一集合与非目标冲突）→ 记入剩余工作，不在计划内自行解决。
- 新发现的站点在 `internal/` 之外或属新包职责之外 → `Operational Discovery` 记录并继续，不扩围。

## 波次与依赖

```text
W1 filekind 包（T01,T02） ──► W2 install 迁移（T03→T04,T05→T06）
                            ├─► W3 client/app-cache/sdk 迁移（T07）
                            └─► W4 detect release 分类迁移（T08）

W5 platform 包（T09 → T10）  独立于 W1/W2，可与 W1 并行

W6 filter 包（T11 → T12）  独立于 W1/W5；依赖 T12 完成后才可运行全量测试

W7 收尾（T13）  依赖 W2、W3、W4、T10、T12 全部完成
```

每个波次一个本地 commit（含 characterization 用例），可单独 revert。

## 站点清单（权威清单，闭合 X-01/X-02/X-03）

归档/多段扩展名站点：

| id | path:line | 函数 | 集合 | 轴 | 决策 |
|---|---|---|---|---|---|
| S-A01 | `internal/install/extractor.go:46-82` | `NewExtractor` | `ExtSetExtractor`（14 项） | 敏感 | 迁 `filekind.IsArchiveExact` / `ExtExact`；构造函数映射留 `install`（CAP-07） |
| S-A02 | `internal/install/runner_output.go:30-42` | `isExtractAllArchiveAsset` | `ExtSetExtractAll`（10 项） | 折叠 | 迁 `filekind.IsArchiveFold` |
| S-A03 | `internal/install/runner_download.go:163-169` | `isInvalidCachedDownload` | `ExtSetDownload`（8 项） | 折叠 | 迁 `filekind.IsDownloadFold` |
| S-A04 | `internal/client/cache_path.go:75-83` | `archiveExt` | `ExtSetCacheMulti` | 折叠 | 迁 `filekind.MultiExtFold` |
| S-A05 | `internal/app/cache/keep_latest.go:435-443` | `cacheAssetExt` | `ExtSetCacheMulti` | 折叠 | 迁 `filekind.MultiExtFold`（与 S-A04 同一 API） |
| S-A06 | `internal/sdk/json_index.go:121-144` | `parseNodeFileDescriptor`（X-03） | `-tar.xz/-tar.gz/-zip`（`-` 分隔） | 敏感 | **刻意保留**：单一上游索引命名约定，非通用归档知识；补 characterization 用例防漂移 |
| S-A07 | `internal/sdk/json_index.go:209-220` | `parseZuluFilename` | 经 `archiveExt` 的 `tar.gz/zip` | 敏感 | 随 S-A08 |
| S-A08 | `internal/sdk/json_index.go:223-231` | `archiveExt`（F3 漂移变体） | `ExtSetArchiveLegacy`（缺 `.tar.zst/.tar.br/.tar.lz4`） | 敏感 | 迁 `filekind.MultiExtExact(name, ExtSetArchiveLegacy)`；**集合不补齐**（等价性） |
| S-A09 | `internal/install/archive_formats.go:107-123` | `NewZipArchive` | 内容嗅探 | 不适用 | 刻意保留（非文件名知识） |

可执行/安装器站点（X-01 已全部纳入）：

| id | path:line | 函数 | 集合 | 轴 | 决策 |
|---|---|---|---|---|---|
| S-E01 | `internal/install/file_modes.go:49-61` | `rename` | `.appimage,.exe` | 敏感 | 迁 `filekind.IsExecutableExact` |
| S-E02 | `internal/install/file_modes.go:63-77` | `isDefinitelyNotExec`,`isExec` | `.deb,.1,.txt,.exe,.appimage` | 基础名折叠 / 后缀敏感 | 迁 `filekind.KindExact` + 本地折叠基础名；**保持混合轴** |
| S-E03 | `internal/install/chooser.go:162-169` | `BinaryChooser.Choose` | `.exe,.appimage` | 敏感 | 迁 `filekind.IsExecutableExact` |
| S-E04 | `internal/install/runner_platform.go:134-144` | `isExecutableForGOOS`（X-01） | `.exe` | 折叠 | 迁 `filekind.IsExecutableFold` |
| S-E05 | `internal/install/runner_platform.go:180-190` | `autoSelectOnlyWindowsExecutable`（X-01） | `.exe` | 折叠 | 迁 `filekind.IsExecutableFold` |
| S-E06 | `internal/install/runner_output.go:160-198` | `applyPreferredName`,`shouldUsePreferredExecutableName` | 经 `executableSuffix` | 折叠 | 随 S-E07 |
| S-E07 | `internal/install/runner_output.go:201-226` | `heuristicExecutableName`,`executableSuffix` | `.exe,.appimage` | 折叠 | 迁 `filekind.ExecutableSuffixFold` |
| S-E08 | `internal/install/runner_select.go:225-233` | `normalizedAssetNameHint` | `.exe,.appimage` | 折叠 | 迁 `filekind.ExecutableSuffixFold` |
| S-E09 | `internal/install/gui.go:28-41` | `DetectGUIInstallMode`（X-01；设计原引用区间不含此函数） | `.exe,.msi` | 折叠 | 迁 `filekind.IsInstallerFold` |
| S-E10 | `internal/install/gui.go:43-53` | `DetectInstallerKind` | `.msi,.exe` + `setup`/`installer` 关键词 | 折叠 | 后缀部分迁 `filekind.IsInstallerFold`；关键词判定留原地 |
| S-E11 | `internal/install/service.go:502-505` | `requiresSystem7z`（X-01） | `.exe` | 折叠 | 迁 `filekind.IsExecutableFold`，并与 S-E12 的 `.exe` 分支合并为一处判定 |
| S-E12 | `internal/install/system7z.go:32-45` | `shouldUseSystem7z` | `.7z,.rar,.msi,.cab,.iso,.exe` | 折叠 | 迁 `filekind.RequiresSystemArchiver`（新表，含 `.exe` 分支供 S-E11 复用） |

release asset 分类站点：

| id | path:line | 函数 | 集合 | 轴 | 决策 |
|---|---|---|---|---|---|
| S-R01 | `internal/install/detect/detectors.go:245-300` | `isReleaseMetadataAsset`,`classifyReleaseAsset` | metadata/checksum/signature 三组 30+ 后缀 | 折叠 | 迁 `filekind.ClassifyAsset`；`SelectableReleaseAssets` 的过滤流程留 `detect` |

平台站点（W5）：

| id | path:line | 符号 | 决策 |
|---|---|---|---|
| S-P01 | `internal/install/detect/detectors.go:302-318` | `installOS*`/`installArch*` 正则表（含 anti/priority） | 数据迁 `platform.DetectOSSpecs`；`systemDetector.Detect` 流程留原地（Q3） |
| S-P02 | `internal/install/detect/detectors.go:320-339` | `installGOOSMap`/`installGOARCHMap` | 迁 `platform.GOOSSpecIndex`/`GOARCHSpecIndex`；`sdk.OSMap` 不并入 |
| S-P03 | `internal/install/runner_platform.go:215-218` | `osTokens`/`archTokens` | 删除本地定义，用 `platform.OSTokens()`/`ArchTokens()` |
| S-P04 | `internal/install/runner_platform.go:218` | `executableVariantTokens` | 迁 `platform.VariantTokens()` |
| S-P05 | `internal/install/runner_platform.go:229-253` | `osAliases`/`archAliases` | 迁 `platform.Aliases(goos, goarch)` |
| S-P06 | `internal/install/runner_platform.go:157-177` | `platformTokens`/`hasAnyToken` | 迁 `platform.SplitTokens`/`platform.HasAnyToken` |
| S-P07 | `internal/install/runner_platform.go:192-212` | `archSelectionPatterns` | 迁 `platform.DirArchPatterns` |
| S-P08 | `internal/client/cache_path.go:153-154` | `cacheOSTokens`/`cacheArchTokens` | 删除本地定义，用 `platform.*`；`nameHasPlatform` 留原地（Q3） |
| S-P09 | `internal/app/cache/keep_latest.go:9-10` | `keepLatestOSTokens`/`keepLatestArchTokens` | 删除本地定义；`trimPlatformSuffix` 留原地（Q3） |
| S-P10 | `internal/install/runner_output.go:204-208` | 三条 os/arch 正则 | **整表原样迁入** `platform.TrimPlatformSuffixRegexps()`；不得由 token 表生成（`armv?6`/`armv?7` 无法由字面量 token 表达） |
| S-P11 | `internal/install/detect/detectors.go:302-305` | 正则内 `.exe/.msi/.appimage` 优先级信号 | 随 S-P01 整表迁入，保持 `(?i)` |

过滤站点（W6）：

| id | path:line | 函数 | 语法 | 决策 |
|---|---|---|---|---|
| S-F01 | `internal/tagfilter/tagfilter.go:34-52` | `Parse`,`IsPattern` | `PRE:`/`SUF:`/`REG:` + 普通值属锚定前缀 | 删除包，语法迁 `filter.Parse`；匹配语义（锚定前缀、`(?i)`）留调用点 |
| S-F02 | `internal/install/service.go:319-349` | `parseAssetFilter` | `^` + `PRE:`/`SUF:`/`REG:`，普通值=子串 | 解析迁 `filter.Parse`；子串默认与 `assetDetector.matches` 留原地 |
| S-F03 | `internal/install/chooser.go:57-81,121-159` | `newFilePatternChooser`,`NewFileChooser`,`newFilterChooser` | `REG:` + `^` 排除，普通值=glob | 仅 `REG:`/`^` 前缀解析迁 `filter`；glob 默认留原地 |
| S-F04 | `internal/sdk/search_service.go:85-117` | `searchKeywordMatches`,`searchKeywordContains` | `^` + `REG:`，普通值=子串 | 前缀解析迁 `filter`；子串默认留原地 |
| S-F05 | `internal/app/tag_semantics.go:42,51`；`internal/source/github/finder.go:109,204` | 4 个 `tagfilter.*` 调用点 | 同上 | 改指向 `filter.IsPattern`/`filter.Parse` |

## 任务

### T01 建立 internal/filekind 包与具名集合

- 文件: `internal/filekind/filekind.go`、`internal/filekind/archive.go`、`internal/filekind/executable.go`、`internal/filekind/releaseasset.go`
- 动作:
  - 定义具名集合常量 `ExtSetExtractor`、`ExtSetExtractAll`、`ExtSetCacheMulti`、`ExtSetDownload`、`ExtSetArchiveLegacy`、`ExtSetSystemArchiver`，每项含注释说明消费者，`ExtSetArchiveLegacy` 标注「等价性保留，漂移待另一次语义变更」。
  - 导出双轴 API：`MultiExtExact/MultiExtFold`、`IsArchiveExact/IsArchiveFold`、`IsDownloadFold`、`IsExecutableExact/IsExecutableFold`、`ExecutableSuffixFold`、`IsInstallerFold`、`RequiresSystemArchiver`、`ClassifyAsset`（返回 `AssetInstallable`/`AssetChecksum`/`AssetSignature`/`AssetMetadata`）。
  - 包内只 import stdlib 与 `internal/util`；不构造解压器、不碰文件系统。
  - 数据来源：从站点清单各行原样复制后缀字面量（先读原实现再抄），不得顺手重排或去重；同组集合的项序不影响语义，但**不得**跨集合合并（`ExtSetExtractor` 与 `ExtSetExtractAll` 差在单文件压缩器，`ExtSetArchiveLegacy` 是窄集）。
- 验证: `go build ./...`；`go list -deps ./internal/filekind` 仅含 stdlib + `internal/util`
- 完成标准: 包可编译；六组集合与两轴 API 齐备；包注释写明「只持数据，不持判定策略」。
- 依赖: 无

### T02 filekind 自测（每集合 × 两轴 × 大小写变体）

- 文件: `internal/filekind/archive_test.go`、`internal/filekind/executable_test.go`、`internal/filekind/releaseasset_test.go`
- 动作: 对 S-A01~S-A05、S-A08、S-E01~S-E12、S-R01 涉及的每个后缀写「正例 / 反例 / 全大写变体」表驱动用例；用 `goutil/x/assert`（GR106）；`t.Run()` 分组。
- 验证: `go test ./internal/filekind/... -count=1`
- 完成标准: 每个具名集合至少一条正例与一条反例；`ExtSetArchiveLegacy` 的「缺 `.tar.zst`」以负例固定下来；大写变体在 `Xxx` 下为负例、在 `XxxFold` 下为正例。
- 依赖: T01

### T03 install 包 characterization 用例先行

- 文件: `internal/install/filekind_equiv_test.go`（新增），不改动既有 `*_test.go` 的断言
- 动作: 对 S-A01~S-A03、S-E01~S-E12 的每一行，直接调用**当前**实现函数（同包内可访问），断言其后缀判定结果；输入表含常规名、全大写名、无后缀名、多段扩展名。
- 验证: `go test ./internal/install/... -run 'Equiv' -count=1`（迁移前必须绿）
- 完成标准: 18 个 install 站点各至少 3 条断言；测试名以站点 id 前缀（如 `TestEquivS_E04`）便于逐站点跟踪。
- 依赖: 无（可与 T01/T02 并行）

### T04 迁移 install 可执行/安装器站点并合并 `.exe` 分支

- 文件: `internal/install/file_modes.go`、`internal/install/chooser.go`、`internal/install/gui.go`、`internal/install/runner_platform.go`、`internal/install/runner_output.go`、`internal/install/runner_select.go`、`internal/install/service.go`、`internal/install/system7z.go`
- 动作: 按站点清单逐行替换为 filekind API，保持各站点原轴（S-E01/S-E02/S-E03 敏感，其余折叠）；S-E11 与 S-E12 的 `.exe` 判定合并为 `filekind.RequiresSystemArchiver` 的一处调用；删除被替代的本地集合。
- 验证: `go test ./internal/install/... -count=1`；`rg -n --glob '!*_test.go' '\.appimage|\.msi' internal/install` 只应剩 filekind 调用与刻意保留的关键词判定
- 完成标准: T03 断言全绿且未修改；上述 grep 的剩余行可逐行解释。
- 依赖: T01, T02, T03

### T05 迁移 install 归档站点

- 文件: `internal/install/extractor.go`、`internal/install/runner_output.go`、`internal/install/runner_download.go`
- 动作: S-A01 用 `IsArchiveExact`/`ExtExact` 选类型，`install` 内保留「类型 → 构造函数」映射与 gunzip/bzip2/xz/zstd 闭包；S-A02、S-A03 用折叠 API 与各自具名集合。
- 验证: `go test ./internal/install/... -count=1`；`rg -n --glob '!*_test.go' '\.tar\.gz|\.tar\.xz|\.tar\.zst' internal/install` 只应剩 filekind 引用
- 完成标准: T03 断言全绿；`extractor.go` 不再直接比较后缀字符串。
- 依赖: T01, T02, T03

### T06 install 波次收尾

- 文件: 同 T04、T05
- 动作: 删除 install 内已无引用的私有集合/常量；确认无未使用变量；跑全量测试。
- 验证: `go build ./...`；`go vet ./internal/...`；`go test ./internal/install/... -count=1`
- 完成标准: 三者全绿；`git diff --stat` 仅触及 install 与新包。
- 依赖: T04, T05

### T07 迁移 client / app-cache / sdk 站点

- 文件: `internal/client/cache_path.go`、`internal/app/cache/keep_latest.go`、`internal/sdk/json_index.go`
- 动作: S-A04、S-A05 用 `filekind.MultiExtFold(ExtSetCacheMulti)`；S-A07、S-A08 用 `MultiExtExact(ExtSetArchiveLegacy)`；S-A06 保持现状并补 characterization 用例（`internal/sdk/json_index_test.go` 追加 `-tar.xz`/`-TAR.XZ` 用例，断言大写为不识别）；`client` 新增对 `internal/filekind` 的 import（Q2 已允许，方向 `client → filekind`）。
- 验证: `go test ./internal/client/... ./internal/app/cache/... ./internal/sdk/... -count=1`；`go list -deps ./internal/client | rg 'internal/(install|app)'` 应为空（无反向边）
- 完成标准: 三包测试绿；`.tar.br`/`.tar.lz4` 在 cache 路径有正例；sdk 窄集行为未变（大写仍不识别）。
- 依赖: T01, T02

### T08 迁移 detect 的 release asset 分类

- 文件: `internal/install/detect/detectors.go`
- 动作: S-R01 的 `isReleaseMetadataAsset`/`classifyReleaseAsset` 改为调用 `filekind.ClassifyAsset`；`SelectableReleaseAssets`、`assetDetector.Detect`、`systemDetector.Detect` 流程留原地。
- 验证: `go test ./internal/install/detect/... -count=1`（现有 `TestReleaseAssetMetadataClassification` 等必须绿且未改断言）
- 完成标准: detect 包内不再出现 checksum/signature/metadata 后缀字面量；现有分类测试全绿。
- 依赖: T01, T02

### T09 建立 internal/platform 包

- 文件: `internal/platform/platform.go`、`internal/platform/tokens.go`、`internal/platform/detect.go`、`internal/platform/patterns.go`
- 动作:
  - `OSTokens()/ArchTokens()/VariantTokens()`：从 `runner_platform.go:215-218` 原样复制（三处逐字相同，取其一即可）。
  - `Aliases(goos, goarch string) (osAliases, archAliases []string)`：迁 `runner_platform.go:229-253`。
  - `SplitTokens(name string) []string`、`HasAnyToken(name string, tokens []string) bool`：迁 `runner_platform.go:157-177`。
  - `DetectOSSpecs`/`DetectArchSpecs`（含 `Regex/anti/priority`）与 `GOOSSpecIndex`/`GOARCHSpecIndex`：整表迁 `detect/detectors.go:302-339`，正则内容逐字不变。
  - `DirArchPatterns`：迁 `runner_platform.go:192-212`。
  - `TrimPlatformSuffixRegexps()`：把 `runner_output.go:204-208` 三条正则**整表原样**返回，不重建。
  - **不**提供任何判定函数（Q3）。
- 验证: `go build ./...`；`go list -deps ./internal/platform` 仅 stdlib；`go test ./internal/platform/... -count=1`（新增 tokens/aliases 表测试）
- 完成标准: 三条正则字符串与 HEAD 逐字一致（可用 `git show HEAD:internal/install/runner_output.go` 比对）；包内无 `install`/`client`/`app` import。
- 依赖: 无（可与 W1 并行）

### T10 迁移平台站点

- 文件: `internal/install/runner_platform.go`、`internal/install/runner_output.go`、`internal/install/detect/detectors.go`、`internal/client/cache_path.go`、`internal/app/cache/keep_latest.go`
- 动作: 按 S-P01~S-P11 逐行替换数据来源；删除 `osTokens/archTokens/cacheOSTokens/cacheArchTokens/keepLatest*Tokens` 等本地定义；保留 `nameHasPlatform`、`trimPlatformSuffix`、`archiveNameMatchesPlatform`、`autoSelectWindowsMSVCAsset`、`systemDetector.Detect` 的判定流程（Q3）。
- 验证: `go test ./internal/install/... ./internal/client/... ./internal/app/cache/... -count=1`；`rg -n --glob '!*_test.go' 'armv\?6' internal/` 只应命中 `internal/platform`
- 完成标准: 上述测试绿且既有断言未改；`runner_output` 的裁剪行为对大写平台名与 `armv6/armv7` 有 characterization 用例。
- 依赖: T09

### T11 建立 internal/filter 包

- 文件: `internal/filter/filter.go`、`internal/filter/parse.go`
- 动作: 定义 `Spec{Raw, Anti, Kind, Expr, Regex}`、`KindPlain/KindPrefix/KindSuffix/KindRegex`、`Parse(raw string) (Spec, error)`、`IsPattern(raw string) bool`；`REG:` 用 `regexp.Compile` 返回错误，`PRE:`/`SUF:` 用 `regexp.QuoteMeta` + `MustCompile`；**不**返回 matcher、不决定匹配对象、不内联错误文案（由调用方包装以保留「invalid tag regex %q」/「invalid asset regex %q」）。`^` 前缀剥离为 `Anti=true`。
- 验证: `go build ./...`；`go test ./internal/filter/... -count=1`（新增解析用例：四种前缀 × 正/反例 × 错误输入）
- 完成标准: 包内无 tag/asset/文件名语义；错误类型可被调用方区分正则编译失败。
- 依赖: 无

### T12 迁移过滤站点并删除 internal/tagfilter

- 文件: `internal/install/service.go`、`internal/install/chooser.go`、`internal/sdk/search_service.go`、`internal/app/tag_semantics.go`、`internal/source/github/finder.go`，删除 `internal/tagfilter/`
- 动作: S-F02~S-F05 按清单替换；`tagfilter` 的 4 个调用点改指向 `internal/filter`；glob 默认（`chooser.go`）与子串默认（`service.go`、`search_service.go`）留在原地不动；把 `internal/tagfilter/tagfilter_test.go` 的语法用例迁到 `internal/filter`。
- 验证: `go test ./internal/install/... ./internal/sdk/... ./internal/app/... ./internal/source/github/... ./internal/filter/... -count=1`；`rg -n 'internal/tagfilter' --glob '!*_test.go' .` 为空；`go build ./...`
- 完成标准: 包已删除且无残留 import；`--tag`、`--asset`、`--extract-file`、sdk search 的既有测试全绿且断言未改。
- 依赖: T11

### T13 收尾与全量验证

- 文件: 全波次触及路径（只读检查为主）
- 动作: 逐站点执行 grep 断言；确认三包的 `go list -deps` 白名单；检查 `gofmt -l internal`；对 `internal/` 全量跑测试；核对设计 F1~F10 与站点清单的覆盖关系。
- 验证: `go build ./...`；`go vet ./internal/...`；`go test ./... -count=1`；`gofmt -l internal`（空输出）；`rg -n --glob '!*_test.go' '\.tar\.gz' internal --stats` 的人工核对
- 完成标准: 完成定义的 5 条全部满足；剩余工作清单已更新。
- 依赖: T06, T07, T08, T10, T12

## 回滚与恢复

- 每波次一个本地 commit（W1=T01/T02，W2=T03~T06，W3=T07，W4=T08，W5=T09/T10，W6=T11/T12，W7=T13 合并入最后一波）。
- 回滚策略：`git revert <wave commit>`。三包互相独立，W5/W6 的 revert 不影响 W1~W4。
- 恢复点：任一站点等价性用例红 → 只回退该站点所在任务的改动，保留已完成的其余站点（同波次内可 `git checkout -- <file>` 单文件回退）。
- 无常驻状态、无迁移、无数据写入；缓存 layout 与 installed store 不受影响。

## 人工 Gate

1. **人工计划批准（当前 Gate，待用户明确批准）**：本计划为 Draft 0.1，批准后才可进入执行。批准对象 = 文档 revision + 本文件的 Git candidate commit + subject path。
2. **执行请求**：计划批准不等于执行授权，需单独一句当前执行请求。
3. **外部动作**：本计划 `NOT_APPLICABLE`——不含 push、release、deploy、迁移、设备或外部消息动作。若后续需要 push，另列具名目标取得授权。
4. **Semantic Amendment**：若实施中发现需要改变后缀集合、大小写轴或匹配结果（例如修复 F3 漂移），停止并返回 plan 评审与人工 Gate。

## 可追溯性

| 需求 / 决策 / 评审项 | 任务 | 验证 |
|---|---|---|
| F1、F9、D1（OS/Arch 数据单一真源） | T09, T10 | `rg 'armv\?6' internal/` 只命中 platform；runner/cache/keep-latest 测试绿 |
| F3（多段扩展名重复 + 漂移变体） | T01（`ExtSetArchiveLegacy`）, T02, T07 | filekind 用例含 legacy 窄集负例；sdk 测试断言大写不识别 |
| F2、X-01（后缀站点 ≥16 / 补 4 处 `.exe`） | T03, T04, T08 | 站点清单 S-E01~S-E12 全覆盖；grep 断言 |
| X-02（逐站点大小写轴） | T02, T03, T04 | 敏感站点（S-E01/E02/E03、S-A01/A06/A08）有「全大写为负例」断言 |
| X-03（`sdk/json_index.go:121-144`） | T07（S-A06 保留 + 用例） | `-tar.xz` 正例、`-TAR.XZ` 负例 |
| X-04（D8 表述） | 无代码任务 | 已在设计评审记录；本计划 T07 验证 `client` 无反向边 |
| F5、D2（release asset 分类表驱动） | T01, T08 | detect 分类测试绿；detect 内无后缀字面量 |
| F4、F10、D3、Q1（过滤语法单一解析器、删除 tagfilter） | T11, T12 | `rg 'internal/tagfilter'` 为空；四入口测试绿 |
| 非目标：glob/子串默认语义不变 | T12（S-F03/S-F04 只迁前缀解析） | chooser glob 测试、sdk 子串测试断言未改 |
| 非目标：不改匹配结果 | T03/T05/T07/T10 的 characterization 用例 | 每站点同输入同输出 |
| SR1403/GR104 | T01/T09/T11（单文件职责） | `gofmt -l`、`go vet`、文件行数人工核对 |

## 完成 Gate 与剩余工作

完成 Gate（全部满足才算完成）：

1. 完成定义 5 条逐条可验证。
2. T13 的 `go build ./...`、`go vet ./internal/...`、`go test ./...`、`gofmt -l internal` 全绿。
3. 站点清单中每行决策已落地（迁移或刻意保留 + 理由 + 用例）。
4. Full 轨 retrospective：terminal `COMPLETE` 时按 plan-closure 一次性判定并由 owner 唯一写入，失败或已有 prior COMPLETE 则不写不提示。

剩余工作（不在本计划范围）：

- F3 漂移修复（`sdk/json_index.go:223-231` 补齐 `.tar.zst/.tar.br/.tar.lz4` 并折叠大小写、`json_index.go:121-144` 折叠）：需一次单独的语义变更批准（会改变匹配结果）。
- F6 版本解析/比较（7 套）与 F7 模板语法（3 套）：design D5/D7 deferred。
- Q5：为规则知识补一份「新增平台/归档格式/过滤语法要改哪里」的 `docs/` 说明，实施完成后补一节。
- R-06：「7 套版本实现」计数独立复核（属 F6 前置）。
- 若后续要修复 D8 表述与 R-12 措辞，随下一次 design 修订（不单独开评审轮）。
