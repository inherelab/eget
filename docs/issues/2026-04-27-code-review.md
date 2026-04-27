# 2026-04-27 Code Review Issues

## Scope

本报告基于当前项目代码进行结构和行为审查，关注逻辑清晰、分层边界、可维护性、安全性和用户可预期行为。验证命令：

```bash
go test ./...
go vet ./...
```

审查时两者均通过。

## Findings

### P1: 归档提取存在路径逃逸风险

- 位置：`internal/install/defaults.go`
- 影响：`extract-all` 或目录提取时，如果归档条目包含 `../`、绝对路径、Windows 卷路径或 UNC 路径，可能写出目标目录。
- 风险：覆盖用户文件，属于安全边界问题。
- 建议：所有归档条目写入前必须做路径清理和目标目录边界校验。

### P1: `update --dry-run` 被暴露但仍会执行真实安装

- 位置：`internal/cli/update_cmd.go`、`internal/cli/service.go`
- 影响：用户认为 dry-run 只预览，但当前会继续调用 update 安装流程。
- 风险：误触真实安装或覆盖。
- 建议：在完整 dry-run 实现前，`--dry-run` 和尚未接通的 `--interactive` 应 fail fast，不进入安装流程。

### P1: installed store 记录的 tag 可能不是实际安装版本

- 位置：`internal/cli/service.go`、`internal/app/install.go`
- 影响：`ReleaseInfo` 当前忽略安装 URL，使用 latest release 写入安装状态。
- 风险：安装指定 `--tag` 后记录为 latest，导致 `list --outdated`、`update`、资产回退判断失真。
- 建议：优先从实际下载 URL 解析 release tag，无法解析时再回退到 release info 查询。

### P2: `update` 路径重复做配置合并

- 位置：`internal/app/update.go`、`internal/app/install.go`
- 影响：`UpdatePackage` 先合并配置，再调用 `InstallTarget` 二次合并。
- 风险：新增字段需要维护两套合并逻辑，且 false override 这类语义容易丢失。
- 建议：把安装配置解析收敛到 `InstallTarget`，`update` 只负责解析目标。

### P2: `--quiet` 没有完全静默

- 位置：`internal/install/runner.go`
- 影响：runner 内仍直接调用全局 `ccolor.Infof` 输出安装日志，绕过 `opts.Quiet`。
- 风险：脚本调用或静默模式输出不稳定。
- 建议：所有 runner 输出统一走注入的 writer。

### P2: `config init` 覆盖确认仍使用不稳定输入读取

- 位置：`internal/cli/service.go`
- 影响：直接回车时可能出现与之前 installer prompt 类似的 `unexpected newline`。
- 风险：交互行为不一致。
- 建议：复用统一的行读取/确认 helper。

### P3: `internal/cli/service.go` 职责过重

- 位置：`internal/cli/service.go`
- 影响：依赖组装、命令分发、业务调用、输出渲染、交互输入集中在一个文件。
- 风险：新增命令选项容易注册但漏接业务逻辑，测试也更难聚焦。
- 建议：后续拆分为 wiring、handlers、render、prompt 等文件。

## Fix Plan

- [x] 修复归档路径逃逸，新增恶意路径回归测试。
- [x] 修复 `update --dry-run` / `--interactive` 误导性真实执行。
- [x] 修复 installed store tag 记录，优先从实际 release URL 解析。
- [x] 修复 `update` 重复配置合并，收敛到 `InstallTarget`。
- [x] 修复 `config init` prompt 空回车处理。
- [x] 修复 `--quiet` 绕过全局输出的问题。
- [x] 拆分 `internal/cli/service.go`，按 wiring、handlers、options、prompts、render 分层。
- [x] 验证 `go test ./...` 和 `go vet ./...`。
