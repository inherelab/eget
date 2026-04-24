# 2026-04-24 gookit-config migration plan

## Goal

将 `internal/config` 的配置加载与管理切换到 `github.com/gookit/config/v2`，统一支持：

- 路径方式 `get/set`，例如 `global.target`、`packages.fzf.repo`
- `Decode/BindStruct` 到 Go struct
- TOML dump / dump file
- 保持现有配置模型与 CLI 行为不变

## Scope

- `internal/config` 读写实现
- `internal/app/config.go` 的 `config get/set/list/init`
- 必要的测试回归

## Constraints

- 保留现有 TOML 结构：`[global]`、`["owner/repo"]`、`[packages.<name>]`
- 不修改安装选项合并语义
- 不改 installed store
- 尽量不改 CLI 输出格式

## Checklist

- [x] 封装 `gookit/config` 适配层
- [x] 用适配层重写 `LoadFile/Save`
- [x] 新增 path get/set、decode、dump 辅助方法
- [x] 改造 `ConfigService` 使用适配层
- [x] 补充回归测试并运行 `go test ./...`
