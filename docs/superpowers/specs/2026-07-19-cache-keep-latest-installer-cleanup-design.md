# Cache Keep-Latest 与 Installer 缓存优化设计

## 背景

`tmp/eget-pkg-cache.txt` 是另一台机器运行 `cache clean --keep-latest` 后的剩余列表。207 个文件中，当前解析器仍将 100 个文件视为无法识别，其中约 92 个使用 eget 自己生成的 Git describe 版本；另有 DBX、OxideTerm 等文件因为版本位于原始 asset 名中间，被错误拆成多个 family。

GUI installer 还有一条独立问题：直接下载的 `.exe`/`.msi` 已写入 `pkg-cache/`，启动前又会写入 `installers/`。归档内 installer 则确实需要先物化成普通文件才能启动。

## 目标

- `--keep-latest` 能比较 Git describe 版本并清理旧构建。
- 能识别原始 asset 名中与 appended version 重复的版本段。
- 直接下载的 installer 复用实际 `pkg-cache` 文件，不产生副本。
- 归档内 installer 继续写入 `installers/`。
- `cache clean --keep-latest` 将 `installers/` 视为派生文件并全部清理。

## 非目标

- 不引入 sidecar、catalog 或新的 cache kind。
- 不删除同一最高版本的不同 URL hash、平台或扩展名变体。
- 不在异步启动 installer 后立即删除文件。
- 不为未知版本格式增加宽泛的自然排序。

## 设计

### Git describe 版本

在现有语义版本之外接受 `<core>-<commit-count>-g<hex>[-dirty]`。`core` 沿用现有规则；`commit-count` 使用无前导零的非负整数；hash 只接受十六进制。比较时先比较 core，再比较 commit count；相同 core/count 的 hash 和 dirty 状态视为等价构建，全部保留，避免凭 hash 猜测新旧。

### Family 归一化

若 raw asset name 中存在与 appended version 完全相同、由 `-`、`_` 或 `.` 分隔的唯一版本段，则移除该段，保留其余平台和安装形态 token：

```text
DBX_0.5.24_x64-setup + 0.5.24 -> dbx-x64-setup
OxideTerm_1.6.12_windows_x64-setup + 1.6.12 -> oxideterm-windows-x64-setup
```

零个或多个匹配时保持现有保守行为。`portable`、`setup`、`installer`、target triple 等 qualifier 不删除。

### Installer 文件复用

仅当选中的 installer 就是下载 asset 本身时，使用下载阶段相同的 metadata 和现有 `CacheFilePathWithMeta` 计算实际 `pkg-cache` 路径，并直接交给 launcher；若 installer 来自 ZIP 等归档，仍调用 `Extract` 写入 `installers/`。

不能沿用不带 metadata 的 `CacheFilePath`，否则可能与下载阶段得到不同文件名。这里不向 `downloadBodyResult` 增加冗余字段，因为同一组 `opts` 已完整包含所需 metadata。

### Installers 清理

`--keep-latest` 扫描时仍只对 `pkg-cache/` 做版本选择，同时把 `installers/` 下的普通完整文件直接加入删除候选。它们是可由 pkg-cache/归档重新生成的派生文件，不计入 kept 或 unrecognized。

普通按时间清理保持现状。Windows 正在占用的文件由现有 `ApplyClean` 删除失败处理进入 `Skipped`，不增加重试或强制终止进程。

## 测试与提交边界

1. 解析与 family 分组：覆盖 Git describe、DBX、OxideTerm，形成独立提交。
2. installer 复用：覆盖直接 MSI/EXE 复用以及归档内 installer 物化，形成独立提交。
3. installers 清理：覆盖 preview/apply、统计和 pkg-cache 保留行为，形成独立提交。
4. 最终运行 `go test ./...`、`staticcheck ./...` 和 GitNexus change detection。
