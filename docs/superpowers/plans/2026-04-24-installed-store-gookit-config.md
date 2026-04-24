# 2026-04-24 installed store gookit-config migration plan

## Goal

将 `internal/installed` 的 `installed.toml` 读写切换到 `github.com/gookit/config/v2`，与主配置层统一，保持当前文件结构与行为兼容。

## Scope

- `internal/installed` 读写实现
- `installed.toml` round-trip
- 现有 `Record/Remove/Path` 行为保持不变

## Constraints

- 保留当前根结构：`[installed]` 与 `installed.<repo>` 子表
- 不改变 `Entry` 字段和序列化 key
- 不改上层调用接口

## Checklist

- [x] 封装 installed store 的 gookit 适配层
- [x] 用适配层重写 `Load/Save`
- [x] 补充 round-trip 和兼容性测试
- [x] 运行 `go test ./...`
