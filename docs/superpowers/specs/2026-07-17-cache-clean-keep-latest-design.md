# Cache Clean 保留最新版本设计

## 背景

当前 `eget cache clean` 主要按文件修改时间清理缓存：默认删除超过 3 天的文件，`--all` 删除所选类型的全部文件。这适合释放磁盘空间，但不适合承担局域网分享职责的 cache server。仍是工具最新版本的资产可能因为缓存时间较早而被删除，其他机器随后无法继续从 server 安装。

本功能为 `pkg-cache/` 增加独立的按版本清理模式：删除可可靠识别的旧版本，保留仍适合分享的最新稳定版和必要的最新预发布版。

## 目标

- 增加 `eget cache clean --keep-latest` 独立清理模式。
- 严格只处理 `pkg-cache/` 下的普通完整文件，不依赖 mtime 选择版本。
- 保留每个可识别资产族的最新稳定版。
- 当最高预发布版的版本核心高于最新稳定版时，同时保留该预发布版。
- 保留选中版本下的全部文件变体，包括平台、架构、格式和 URL hash 变体。
- 对无法可靠识别的文件采取保守策略：保留，不猜测删除。
- 复用现有 dry-run、JSON、确认、安全路径验证和删除统计流程。

## 非目标

- 不处理 `api-cache/`、SDK download、SDK index、partial cache、缓存根目录文件或未知子目录。
- 不增加 metadata sidecar、catalog、registry、provider 请求或外部依赖。
- 不修改现有 pkg-cache 命名格式。
- 不支持配置保留最近 N 个版本；第一版固定保留最新版本集合。
- 不承诺从文件名恢复真实 provider、repository 或 package 身份。
- 不强行归并第三方非常规文件名。

## CLI 语义

新增命令形式：

```text
eget cache clean --keep-latest
eget cache clean --keep-latest --dry-run
eget cache clean --keep-latest --dry-run --json
eget cache clean --keep-latest --yes
```

规则：

- `--keep-latest` 自动限定为 `pkg-cache/`，无需同时指定 `--pkg`。
- 允许同时指定 `--pkg`，语义不变。
- 与非空的 `--older`、`--all` 互斥。
- 与 `--api`、`--sdk`、`--sdk-index`、`--partial` 互斥。
- 继续支持 `--dry-run`、`--json` 和 `--yes`。
- 大量删除仍经过现有确认阈值；非 TTY 环境仍要求 `--yes`。

CLI 中 `Older` 的 flag 默认值和命令 reset 值改为空字符串：

- 普通模式把空值解释为现有默认值 `3d`，保持 `eget cache clean` 行为不变。
- keep-latest 模式只接受空值；显式传入 `--older 3d` 返回互斥错误。
- `--older=` 视为未设置：keep-latest 模式允许，普通模式仍使用 `3d`。第一版不为这个无实际 duration 的边界增加独立 `OlderSet` 状态。

互斥参数在 CLI options 映射阶段返回明确错误，不为两种清理条件定义隐式优先级。

## 架构与数据流

```text
cache clean --keep-latest
  -> 解析并校验 CLI 参数
  -> CleanOptions.KeepLatest = true
  -> 扫描缓存条目
  -> 严格筛选 pkg-cache/ 下的 KindPkg 完整普通文件
  -> 保守解析文件名并按推断 asset family 分组
  -> 计算每组保留的稳定版/预发布版
  -> 其余可识别版本成为 matched entries
  -> 复用 preview/confirm/remove/result 流程
```

`KindPkg` 当前是排除其他已知目录后的兜底分类，不等同于严格位于 `pkg-cache/`。因此 keep-latest 候选必须同时满足：

```text
entry.Kind == KindPkg
entry.IsPartial == false
entry.RelPath 严格位于 "pkg-cache/" 下
```

不修改全局 `classifyEntry`，避免改变 list、status、server 和普通 clean 的现有分类行为。缓存根目录或 `misc/` 等未知目录中的 hash-like 文件保持不动，也不计入 keep-latest 的 `kept` 或 `unrecognized` 统计。

现有 `Service.clean` 仍负责 cache root 验证和安全删除。版本模式只替换候选选择逻辑；文件名解析和版本选择放在 `internal/app/cache` 的独立文件中，保持为无写入、无网络访问的纯逻辑。

## pkg-cache 文件解析

当前 `CacheFilePathWithMeta` 生成格式为：

```text
<raw-asset-name>-<appended-version>[-<appended-OS>-<appended-arch>]-<8位URL哈希><扩展名>
```

只有原始 asset name 不含平台信息时才追加平台。解析器只接受能按此格式得到唯一结果的文件。

### 解析步骤

1. 仅接受严格位于 `pkg-cache/` 下、`KindPkg`、非 partial、非 symlink 的普通文件。`Entry` 增加 `IsSymlink`，由现有 `Scan` 的 `Lstat` 结果填充；不改变 Scan 对 list/server 返回 symlink 的现有行为。
2. 按生成器 `archiveExt` 的语义去扩展名：识别其组合归档扩展名，否则使用普通 `path.Ext`；无原始扩展名的生成文件按 `.bin` 处理。不维护另一份模糊扩展名白名单。
3. 要求主体以 `-<8位小写十六进制 URL hash>` 结尾并去掉 hash，即只接受当前生成器产生的 `[0-9a-f]{8}`。
4. 优先匹配 `<raw-name>-<version>-<OS>-<arch>`。这是 v0 的文法消歧规则；平台只接受下列已知 OS alias 与 arch alias 的笛卡尔积，不接受任意两个尾部 token：

   ```text
   OS: windows, win, win32, win64, darwin, macos, osx, linux,
       freebsd, openbsd, netbsd, android, illumos, solaris, plan9
   arch: amd64, x86_64, x64, 386, x86, i386, arm64, aarch64,
         arm32, armv6, armv7, arm, riscv64
   ```

   该有限集合与当前 `cacheOSTokens`、`cacheArchTokens` 对齐；未来生成器新增 token 时再同步扩展。未知 tuple 不按平台猜测。
5. 不匹配时再匹配 `<raw-name>-<version>`。
6. 版本严格按下文有限语法解析。完成可选平台 tuple 剥离后，从右向左枚举 `-` 边界，选择第一个使完整右侧后缀符合版本文法的边界，即最右侧合法版本起点；找到后停止，不再把 raw name 中更早的数字段并入版本。平台形式优先同样是确定的文法规则，不再应用“两个解释都合法则 unrecognized”。
7. 剩余整体是 raw asset name，再单独执行 family 归一化。

解析样例：

| 文件主体（省略 hash/扩展名） | appended version | appended platform | raw asset name | normalized family |
| --- | --- | --- | --- | --- |
| `tool-v2.4.1-linux-amd64-2.4.1` | `2.4.1` | 无 | `tool-v2.4.1-linux-amd64` | `tool`，因命中精确 denylist 最终判 unrecognized |
| `gomi_Linux_x86_64-1.6.3` | `1.6.3` | 无 | `gomi_Linux_x86_64` | `gomi` |
| `claude-2.1.160-linux-amd64` | `2.1.160` | `linux-amd64` | `claude` | `claude` |
| `PowerShell-7.6.3-win-x64-7.6.3` | `7.6.3` | 无 | `PowerShell-7.6.3-win-x64` | `powershell` |
| `cscli-windows-amd64-0.5.2` | `0.5.2` | 无 | `cscli-windows-amd64` | `cscli` |

### 版本格式

版本采用可直接实现的有限文法：

```text
version     = ["v"] core ["-" prerelease]
core        = numeric "." numeric {"." numeric}
prerelease  = identifier {"." identifier}
identifier  = numeric | text
numeric     = "0" | non-zero-digit {digit}
text        = ASCII 字母开头，后续仅允许 ASCII 字母或数字
```

预发布 identifier 内不允许 `-` 或 `_`，整个版本最多只有 core 与 prerelease 之间的一个 `-`。支持示例：

```text
1.2.3
v1.2.3
2026.7.17
2.0.0-beta.1
7.6.3-preview.4
```

以下情况标记为 unrecognized：

- 版本为 `unknown`。
- 少于两个数字核心段。
- 数字核心段为空、包含非数字，或除单个 `0` 外带前导零。
- 预发布标识为空、出现连续分隔符，或数字 identifier 除单个 `0` 外带前导零。
- 预发布第一个 identifier 按 ASCII 大小写不敏感比较后等于 `build`。真实 `-build.1` 与被 `safeCachePart` 改写的 `+build.1` 无法区分，因此两者一律 unrecognized；`build1` 仍是普通文本 identifier，`build-info` 因含第二个 `-` 非法。
- 缺少当前格式的 URL hash。
- raw name 或 family 归一化后为空。
- 不符合上述有限文法的版本或无法按结构确定的 family 边界。

`claude-2.1.160-linux-amd64` 按明确的“平台形式优先”规则解析为稳定版 `2.1.160` 加 appended platform。若上游真实版本本身恰好以已知 `<OS>-<arch>` 结尾，文件名无法在没有 sidecar 的前提下区分来源；v0 仍按生成器 canonical 布局解释。该限制不通过更多启发式猜测解决。

最右侧版本起点示例：

```text
foo-1.2.3-1.2.3        -> raw=foo-1.2.3, version=1.2.3
foo-1.2.3-2.0.0        -> raw=foo-1.2.3, version=2.0.0
foo-2.0.0-beta.1       -> raw=foo, version=2.0.0-beta.1
```

### Asset family 归一化

Family 是从文件名推断出的保守分组键，不是 provider/repository ID。

- 转为小写并合并连续的 `-`、`_`、`.` 分隔符。
- 只从 raw name 的结构化右侧边缘去除可证明重复的 appended version 或完整 OS/arch tuple。
- 不从 family 中间或任意位置全局删除单个 `windows`、`linux`、`arm`、`x64` 等 token。
- 无法证明某 token 是结构化平台信息时保留该 token；不能安全归一化时整项标记为 unrecognized。
- `portable`、`installer`、`minimal`、`musl`、`gnu`、`msvc` 等可能影响可用性的 qualifier 保留在 family 中。
- 最终 family 仅使用精确 denylist：`cli`、`tool`、`download`、`release`、`asset`、`package`、`app`、`binary`。命中时标记为 unrecognized，避免常见通用名互相淘汰；不使用长度阈值，`go`、`fd`、`jq` 等短工具名正常参与分组。

必须保持以下反例不合并：

```text
windows-terminal != terminal
arm-tool != tool
```

该策略可能多保留一些旧文件；这是第一版有意的安全倾向。若未来需要真实来源身份，应由 catalog/sidecar 解决，不在本功能中继续增强文件名猜测。

v0 不额外实现 `x86_64-pc-windows-msvc` 等 target triple 的启发式拆分；无法用上述右侧 OS/arch tuple 规则证明时保留原 token，必要时形成更细的 family。这样会多保留，不会跨 target 误删。

## 版本比较与保留集合

版本核心逐段按整数比较，缺失段按 `0` 处理，因此 `1.10.0 > 1.9.9`、`2.0 == 2.0.0`。前导 `v` 不参与比较。

预发布 identifier 以 `.` 分段，按以下确定规则比较：

1. 两个数字 identifier 按整数比较。
2. 数字 identifier 小于文本 identifier。
3. 两个文本 identifier 按 ASCII 大小写敏感字典序比较，不做通道权重猜测。
4. 公共前缀相同时，较短 identifier 列表更小，例如 `beta < beta.0`。
5. 非法前导零、空 identifier 或连续分隔符直接使文件 unrecognized。

因此测试应明确覆盖：`beta.2 < beta.10`、`beta.01` 非法、`alpha < beta`、`beta < beta.0`、`Beta.1 < beta.1`、`preview.9 < rc.1`。

每个 asset family 的选择规则：

1. 找到最高稳定版本 `S`。
2. 找到最高预发布版本 `P`。
3. 始终保留 `S` 的全部文件变体。
4. 没有稳定版时，保留 `P` 的全部文件变体。
5. 同时存在时，仅当 `P.core > S.core` 才额外保留 `P`。
6. 当 `P.core <= S.core` 时，正式版已达到或覆盖该预发布系列，不再保留预发布版。
7. 除保留集合外的可识别版本全部成为删除候选。
8. 同一保留版本下，不按平台、扩展名、mtime 或 URL hash 进一步淘汰。
9. 比较结果相等即属于同一版本集合；`2.0`、`2.0.0`、`v2.0.0` 等文本变体若共同为最高版本，全部保留。

示例：

```text
1.8.0, 1.9.0, 2.0.0-beta.1
  -> 保留 1.9.0 和 2.0.0-beta.1

1.9.0, 2.0.0-beta.1, 2.0.0
  -> 只保留 2.0.0

2.0.0-beta.1, 2.0.0-beta.3
  -> 只保留 2.0.0-beta.3
```

## 清理结果与输出

`CleanResult` 增加：

```text
KeptLatestFiles    JSON: kept_latest_files
UnrecognizedFiles  JSON: unrecognized_files
```

- `MatchedFiles`：当前模式选中的旧版本文件数。
- `KeptLatestFiles`：因属于保留版本而保护的文件数。
- `UnrecognizedFiles`：位于严格候选范围内、但无法可靠解析而保护的文件数。

无法识别不是文件操作失败，不加入现有 `Skipped`。`Skipped` 继续只记录路径越界、快照变化或删除失败等异常。

文本输出仅在 `--keep-latest` 模式增加：

```text
 - kept latest files: 8
 - unrecognized files: 2
```

JSON 在所有模式固定包含新增字段；普通模式值为 `0`。dry-run JSON 的 `removed_files` 必须为 `0`，`matched_files`、`kept_latest_files`、`unrecognized_files` 来自同一次 preview。测试通过真实 JSON 反序列化验证字段和值，不用字符串包含判断替代。

确认阈值继续只依据 `MatchedFiles` 和 `MatchedSize`，因为它们代表真实删除候选。

## 安全性与并发边界

- 选择阶段不写文件、不请求网络。
- 删除阶段继续使用 `ensurePathInDir`，且不跟随符号链接。
- `CleanResult` 增加未导出的 `snapshot []cleanCandidate`；每项保存已解析 cache root、绝对/相对路径、大小、mtime 和 Preview 时的 `os.FileInfo`。该字段不进入 JSON，也不由 CLI 读取或修改。
- `PreviewClean(cacheDir, opts)` 签名不变，返回带 snapshot 的 `CleanResult`。
- 新增 `func (s Service) ApplyClean(preview CleanResult) (CleanResult, error)`：只消费 preview 内部 snapshot，不重新扫描或重新选择候选。
- 现有 `Clean(cacheDir, opts)` 保持签名和直接调用语义，内部改为 `PreviewClean` 后立即 `ApplyClean`，现有 service 调用方无需迁移。
- CLI 的普通时间模式和 keep-latest 模式统一改为 Preview、可选确认、`ApplyClean(preview)`；dry-run 只输出 Preview，绝不调用 Apply。
- Apply 的结果复制 preview 的 `CacheDir`、matched/kept/unrecognized 统计；removed 和 skipped 根据实际执行重新累计。
- 删除前对候选重新 `Lstat`，用 `os.SameFile`、size 和 mtime 比较快照。文件不存在、身份变化、size 变化或 mtime 变化时保留，并在 `Skipped` 中使用现有绝对路径格式和固定 reason `changed since preview`。不承诺识别同一文件身份下、同时恢复相同 size/mtime 的原地覆盖，也不为此计算全文件 hash。
- 不得把 Preview 后重新扫描中新出现的文件加入本次删除集合。
- 未触发交互确认和 `--yes` 路径同样调用 `ApplyClean(preview)`，保证一次命令的统计与实际删除集合一致。
- 新出现或发生变化的文件留到下一次 clean；不引入锁、catalog 或 sidecar。
- partial 文件不参与版本分组。

该方案只让现有 preview 结果携带不序列化的内部候选快照，并把现有删除循环移到 `ApplyClean` 复用；不新增第二套扫描或删除实现。

## 测试策略

### 解析器与选择器

- 表格中的五种真实命名形态及 appended 字段解析结果。
- 最右侧合法版本起点：`foo-1.2.3-1.2.3`、`foo-1.2.3-2.0.0`、`foo-2.0.0-beta.1-2.0.0-beta.1`。
- 标准稳定版、前导 `v`、日期版本和预发布版本。
- 组合扩展名、普通扩展名、生成的 `.bin`、缺 hash、unknown、build metadata 和歧义文件。
- appended platform 只接受已列出的 OS/arch tuple；覆盖 `win-x64`、`linux-amd64`、`darwin-arm64` 和未知 tuple。
- family 不跨 `musl`、`gnu`、`msvc`、`portable` 分组。
- `windows-terminal != terminal`、`arm-tool != tool`，通用 family 不参与删除。
- 精确的 prerelease 比较边界和非法 identifier。
- 最高稳定版、额外最高预发布版、正式版覆盖预发布版。
- 同版本的多平台、多扩展名、多 hash 全部保留。
- `2.0`、`2.0.0`、`v2.0.0` 比较相等时全部作为最高版本变体保留。

### Service

- 只有 `pkg-cache/` 下的完整非 symlink 普通文件参与选择和统计。
- 缓存根目录、未知子目录、partial 中的 hash-like 文件不删除且不计数。
- `pkg-cache/` 内 hash-like symlink 不参与 matched、kept、unrecognized 统计，也不删除。
- Preview 不删除文件，matched/kept/unrecognized 统计正确。
- ApplyClean 只消费 preview 候选快照；preview 后新增文件不进入删除集合，身份、size 或 mtime 变化的候选不删除并返回固定 skip reason。
- 直接调用 `Clean` 仍完成一次 preview + apply；普通 `--older` 模式也覆盖 preview 后文件变化的回归测试。
- 现有按时间、`--all`、SDK index 默认保护行为不回归。

### CLI 与输出

- `cache clean --keep-latest` 合法。
- `cache clean --keep-latest --older 3d` 返回互斥错误。
- `cache clean --keep-latest --older=` 视为未设置并合法。
- `cache clean` 仍默认使用 `3d`。
- keep-latest 与 kind flags、`--all` 的互斥错误。
- 命令 reset 后 `KeepLatest` 和空 `Older` 不残留上一轮状态。
- dry-run 文本、真实解析后的 JSON、大量删除确认和 `--yes` 行为。

完成 MVP 主链路后运行：

```text
go test ./internal/app/cache ./internal/cli
go test ./...
```
