# Cache Clean 保留最新版本设计

## 背景

当前 `eget cache clean` 以文件修改时间为主要筛选条件：默认删除超过 3 天的缓存，`--all` 删除所选类型的全部缓存。这适合释放磁盘空间，但不适合承担局域网分享职责的 cache server。一个较旧但仍是工具最新版本的文件，可能因为修改时间早而被删除，其他机器随后无法继续从 server 离线安装。

本功能为 `pkg-cache` 增加按版本清理模式：删除每个可识别工具或资产族的旧版本，同时保留仍适合分享的最新稳定版和必要的最新预发布版。

## 目标

- 增加 `eget cache clean --keep-latest` 独立清理模式。
- 只处理 `pkg-cache`，不依赖文件修改时间。
- 保留每个可识别资产族的最新稳定版。
- 当预发布版的版本核心高于最新稳定版时，同时保留最高预发布版。
- 保留被选中版本的全部平台、架构、格式和 URL hash 变体。
- 对无法可靠识别的文件采取保守策略：保留，不猜测删除。
- 复用现有 dry-run、JSON、确认、安全路径验证、删除和统计流程。

## 非目标

- 不处理 `api-cache`、SDK download、SDK index 或 partial cache。
- 不增加 metadata sidecar、catalog、registry 或 provider 回源查询。
- 不修改现有 pkg-cache 命名格式。
- 不允许配置保留最近 N 个版本；第一版固定保留最新版本集合。
- 不尝试把所有第三方非常规文件名强行归并为同一个工具。

## CLI 语义

新增命令形式：

```text
eget cache clean --keep-latest
eget cache clean --keep-latest --dry-run
eget cache clean --keep-latest --dry-run --json
eget cache clean --keep-latest --yes
```

规则：

- `--keep-latest` 自动限定为 `pkg-cache`，无需同时指定 `--pkg`。
- 允许同时指定 `--pkg`，语义不变。
- 与 `--older`、`--all` 互斥。
- 与 `--api`、`--sdk`、`--sdk-index`、`--partial` 互斥。
- 继续支持 `--dry-run`、`--json` 和 `--yes`。
- 大量删除仍经过现有确认阈值；非 TTY 环境仍要求 `--yes`。

互斥参数在 CLI options 映射阶段返回明确错误，避免进入 service 后出现“先按时间还是先按版本”的隐式优先级。

## 架构与数据流

```text
cache clean --keep-latest
  -> 解析并校验 CLI 参数
  -> CleanOptions.KeepLatest = true
  -> 只扫描 KindPkg
  -> 保守解析 pkg-cache 文件名
  -> 按 asset family 分组
  -> 计算每组保留的稳定版/预发布版
  -> 其他可识别版本成为 matched entries
  -> 复用现有 preview/confirm/remove/result 流程
```

现有 `Service.clean` 仍负责 cache root 验证、扫描、删除和统计。版本模式只替换“哪些 entry 匹配删除”的选择条件，不复制删除流程。

文件名解析和版本选择放在 `internal/app/cache` 的独立文件中，保持为无文件写入、无网络访问的纯逻辑，便于表驱动测试，也避免继续膨胀 `service.go`。

## pkg-cache 文件解析

当前 eget 生成的 pkg-cache 文件大致遵循：

```text
<asset-name>-<version[-platform]>-<8位URL哈希><扩展名>
```

解析器只接受满足当前格式且能可靠提取身份的文件。

### 解析步骤

1. 仅接受 `KindPkg` 且非 partial 的普通文件。
2. 去掉已知归档或可执行文件扩展名，包括组合扩展名，如 `.tar.gz`。
3. 要求主体以 `-<8位十六进制 URL hash>` 结尾；不符合则无法识别。
4. 去掉 URL hash。
5. 从右侧识别 eget 追加的平台后缀，如 `windows-amd64`、`linux-arm64`、`darwin-amd64`。
6. 从右侧识别 eget 追加的版本表达式。
7. 版本之前的内容作为原始 asset family，并进行保守归一化。

### 版本格式

支持：

```text
1.2.3
v1.2.3
2026.7.17
2.0.0-beta.1
7.6.3-preview.4
```

以下情况标记为无法识别：

- 版本为 `unknown`。
- 没有至少两个数字版本段。
- 缺少当前格式的 URL hash。
- family 归一化后为空。
- 版本边界存在歧义。

### Asset family 归一化

- 转为小写。
- 去掉重复出现的同一版本。
- 去掉已知 OS、arch 和 target-triple token。
- 合并连续的 `-`、`_`、`.` 分隔符。
- 保留 `portable`、`installer`、`minimal`、`musl`、`gnu`、`msvc` 等可能影响可用性的 token。

保留这些 token 会让部分工具形成多个 asset family，从而多保留一些旧文件；这是有意的安全倾向，避免跨运行时或打包形式误删。

示例：

```text
PowerShell-7.6.3-win-x64-7.6.3-7ed27cfc.msi
  -> family=powershell
  -> version=7.6.3

cscli-windows-amd64-0.5.2-b913cc7a.exe
  -> family=cscli
  -> version=0.5.2

ripgrep-14.1.1-x86_64-pc-windows-msvc-14.1.1-a1b2c3d4.zip
  -> family=ripgrep-msvc
  -> version=14.1.1
```

## 版本比较与保留集合

版本解析结果包含完整版本字符串、数字版本核心和预发布 token。数字版本核心逐段比较，缺失段按 0 处理，因此 `1.10.0 > 1.9.9`、`2.0 == 2.0.0`。预发布 token 在预发布通道内按数字/文本自然顺序比较。

每个 asset family 的选择规则：

1. 找到最高稳定版本 `S`。
2. 找到最高预发布版本 `P`。
3. 始终保留 `S` 的全部文件。
4. 没有稳定版时，保留 `P` 的全部文件。
5. 同时存在时，仅当 `P.core > S.core` 才额外保留 `P`。
6. 当 `P.core <= S.core` 时，正式版已覆盖该预发布系列，不再保留预发布版。
7. 除保留集合外的可识别版本全部成为删除候选。
8. 同一保留版本下，不按平台、扩展名、mtime 或 URL hash 进一步淘汰。

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

- `MatchedFiles`：当前模式将删除的旧版本文件数。
- `KeptLatestFiles`：因属于保留版本而保护的文件数。
- `UnrecognizedFiles`：无法可靠解析而保护的文件数。

无法识别不是文件操作失败，不加入现有 `Skipped` 列表。`Skipped` 继续只记录路径越界或删除失败等异常。

普通输出和 dry-run 都增加：

```text
 - kept latest files: 8
 - unrecognized files: 2
```

JSON 输出自然包含新增字段。确认阈值继续只依据 `MatchedFiles` 和 `MatchedSize`，因为它们代表真实删除候选。

## 安全性与并发边界

- 选择阶段不执行文件写入。
- 删除阶段继续使用 `ensurePathInDir` 防止越界。
- 清理不跟随符号链接，沿用现有扫描安全规则。
- Preview 和 Clean 都调用同一纯选择函数。
- 如果 preview 后缓存内容发生变化，Clean 会重新扫描并按最新快照计算。
- partial 文件不参与版本分组。

## 测试策略

### 解析器与选择器

- 标准稳定版、前导 `v`、日期版本和预发布版本。
- PowerShell、cscli、ripgrep 等真实命名形态。
- 多扩展名归档、缺 hash、unknown、手工文件和歧义文件。
- family 不跨 `musl`、`gnu`、`msvc`、`portable` 分组。
- 最高稳定版、额外最高预发布版、正式版覆盖预发布版。
- 同版本的多平台、多扩展名、多 hash 全部保留。

### Service 与 CLI

- Preview 不删除文件，统计 matched/kept/unrecognized 正确。
- Clean 只删除可识别旧版本，且强制只扫描 pkg-cache。
- 现有按时间、`--all`、SDK index 默认保护行为不回归。
- `--keep-latest` flag 绑定和互斥错误。
- dry-run 文本、JSON、大量删除确认和 `--yes` 行为。

完成 MVP 主链路后运行：

```text
go test ./internal/app/cache ./internal/cli
go test ./...
```
