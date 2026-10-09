# 按 package 配置忽略更新检查实施计划

> 来源：<https://github.com/inherelab/eget/issues/61>（用户 hhromic 提出）

## 目标

- 新增 package 级配置 `ignore_update = true`，让单个 package 自己声明"不参与更新检查"，不必再到 `[global] ignore_update_packages` 里登记名字。
- 主要收益场景：`repo` 是直接下载地址的静态包（无 tag 信息），现在必然报 `check_failed <name>: installed tag is empty`。
- 显式 `eget update <name>` 命中被忽略的包时给出跳过提示，不静默无操作。

命名选择 `ignore_update`：与既有 `ignore_update_packages` 同源，语义一致；不使用 `tag_policy = "ignore"`（会侵入 tag 解析链路，且语义不符）。

## 已确认的事实（已实测，不要再假设）

- 静态 URL 包的 installed 记录没有 tag → `checkOutdatedItem()` 直接返回 `installed tag is empty` 失败，**不发起任何网络请求**（`internal/app/list.go`）。
- 忽略判定原有唯一入口：`ignoreUpdatePackageSet()` 读 `[global] ignore_update_packages` → `ListPackages()` 打 `ListItem.IgnoreUpdate` → `checkOutdatedItems()` 跳过。
- `ListOutdatedPackages()` / `ListUpdateCandidates()` 都经 `ListPackages()`，改一处即可覆盖 `list --outdated` / `update --check --all` / `update --all`。
- `UpdatePackageStatus()`（`eget update <name>` 不带 `--check`/`--all`）**完全不走** `IgnoreUpdate`，直接调 `checkOutdatedItem()`，需要单独处理。
- 配置回写链路：`saveConfigFile()` → `preserveUnchangedRawValues()` 对 `packages` 根键整体重写，因此新字段必须进 `sectionToMap()`，否则 web 控制台 / `eget add` 保存后丢失。
- `eget config set` / web 控制台的 key/value 走 `SetByPath()` → `normalizePathValue()`，bool 类键要在其中的白名单里，否则字符串 `"true"` 不会被解析成 bool。

## 阶段

### 1. 配置模型

- [x] `internal/config/model.go`：`Section` 增 `IgnoreUpdate *bool`（toml/mapstructure: `ignore_update`）。
- [x] `internal/config/gookit.go`：`sectionToMap()` 写回 `ignore_update`；`normalizePathValue()` 的 bool 键白名单增 `ignore_update`。
- [x] 测试：TOML 读取、`dumpConfigString` 回写、`SetByPath`/`GetByPath`（`loader_sections_test.go`、`gookit_test.go`）。

### 2. 检查链路

- [x] `internal/app/list.go`：`ListPackages()` 里 `IgnoreUpdate` 合并 package 级声明与全局名单。
- [x] `internal/app/update.go`：`UpdatePackageResult` 增 `Skipped`；`UpdatePackageStatus()` 命中忽略包时直接返回跳过结果，不做 latest 查询、不安装。
- [x] `internal/app/update_candidates.go`：`ListUpdateCandidatesForTargets()` 对显式目标里的忽略包放行并回调 `OnIgnoredTarget`；`UpdateService` 增该回调字段。
- [x] 测试：静态 URL 包不再进检查、显式目标回调通知、`UpdatePackageStatus` 返回 `Skipped`（`list_outdated_test.go`、`update_candidates_test.go`、`update_package_test.go`）。

### 3. CLI 输出

- [x] `internal/cli/update_handler.go`：`update <name>` 打印 `<name> update skipped: ignore_update is set`；`update --check <name>` / `update --interactive <name>` 在 `✅ Checked N packages` 之后打印同名提示；`ignoredTargetCollector` 收集并还原回调。
- [x] 文档：`docs/config.md` 与 `docs/config.zh-CN.md` 的 package 字段表增 `ignore_update`，`ignore_update_packages` 条目补一句可按包单独声明。

### 4. 验证

- [x] `go test ./...` 全绿。
- [x] 隔离环境（`EGET_CONFIG_DIR` 指向临时目录）端到端实测：
  - `ignore_update = true`：`update --check --all` 仅检查 `rg`，vault/vector 无 `check_failed`。
  - `update vault` / `update --check vault`：输出跳过提示。
  - 去掉 `ignore_update` 的对照组：复现 issue 原始症状 `check_failed vault (...): installed tag is empty` 与 `update_failed vault: installed tag is empty`。
