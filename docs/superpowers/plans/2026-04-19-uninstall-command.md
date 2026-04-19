# Uninstall Command Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 新增 `eget uninstall` 命令及别名 `uni`、`remove`、`rm`，删除已安装文件并清理 installed store 记录。

**Architecture:** app 层按 package name 或 repo 解析目标，读取 installed store 中的 `ExtractedFiles` 执行文件删除，再移除 installed 记录；CLI 层负责命令注册、别名和删除结果输出，不删除 `[packages.<name>]` 配置。

**Tech Stack:** Go 1.24, existing `internal/installed` store, filesystem operations via `os`

---

## Task 1: app 卸载用例

**Files:**
- Create: `internal/app/uninstall.go`
- Create: `internal/app/uninstall_test.go`

- [x] 编写 package name / repo 卸载失败测试
- [x] 实现删除文件与清理 installed 记录

## Task 2: CLI 命令与别名

**Files:**
- Create: `internal/cli/uninstall_cmd.go`
- Modify: `internal/cli/app.go`
- Modify: `internal/cli/app_test.go`
- Modify: `internal/cli/service.go`

- [x] 编写 `uninstall` 路由与别名失败测试
- [x] 注册 `uninstall` / `uni` / `remove` / `rm`
- [x] 输出卸载结果

## Task 3: 文档与验证

**Files:**
- Modify: `README.md`
- Modify: `DOCS.md`
- Modify: `man/eget.md`
- Modify: `docs/superpowers/plans/2026-04-19-uninstall-command.md`

- [x] 更新命令文档
- [x] 运行 `go test ./...`
- [x] 回填 checklist 并提交
