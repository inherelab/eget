# List Managed Packages Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 新增 `eget list` 命令，列出本地 managed packages，并关联显示 installed store 中的最近安装状态。

**Architecture:** 以配置文件中的 `[packages.<name>]` 为主表，按 repo 键关联 installed store；app 层返回排序后的聚合结果，cli 层负责命令注册和纯文本输出。

**Tech Stack:** Go 1.24, TOML config, existing `internal/installed` store

---

## Task 1: app 聚合查询

**Files:**
- Create: `internal/app/list.go`
- Create: `internal/app/list_test.go`

- [x] 编写 managed package + installed 聚合失败测试
- [x] 实现排序后的列表结果

## Task 2: CLI 命令与输出

**Files:**
- Create: `internal/cli/list_cmd.go`
- Modify: `internal/cli/app.go`
- Modify: `internal/cli/app_test.go`
- Modify: `internal/cli/service.go`

- [x] 编写 `list` 路由失败测试
- [x] 注册 `eget list`
- [x] 输出列表或空状态提示

## Task 3: 文档与验证

**Files:**
- Modify: `README.md`
- Modify: `DOCS.md`
- Modify: `man/eget.md`
- Modify: `docs/superpowers/plans/2026-04-19-list-managed-packages.md`

- [x] 更新命令文档
- [x] 运行 `go test ./...`
- [x] 回填 checklist 并提交
