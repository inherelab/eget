# Default Config And Store Paths To XDG Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将配置文件和 installed store 的默认存储路径统一到 `~/.config/eget/`，同时保持旧路径读取兼容。

**Architecture:** 保持读取顺序兼容 `EGET_CONFIG`、旧 dotfile 和现有 fallback；只调整“默认写入路径”到 XDG/LocalAppData 目录，并同步测试与文档说明。

**Tech Stack:** Go 1.24, filesystem path resolution

---

## Task 1: 配置默认写入路径

**Files:**
- Modify: `internal/config/paths.go`
- Modify: `internal/config/loader_test.go`

- [x] 将配置默认写入路径改为 OS/XDG 配置目录
- [x] 更新路径解析测试

## Task 2: store 默认路径与文档

**Files:**
- Modify: `internal/installed/store.go`
- Modify: `internal/installed/store_test.go`
- Modify: `README.md`
- Modify: `README.zh-CN.md`
- Modify: `docs/DOCS.md`
- Modify: `man/eget.md`

- [x] 确认 installed store 默认路径行为与测试
- [x] 更新中英文 README / docs / man 文档

## Task 3: 验证

**Files:**
- Modify: `docs/superpowers/plans/2026-04-20-default-config-xdg-path.md`

- [x] 运行 `go test ./...`
- [x] 回填 checklist 并提交
