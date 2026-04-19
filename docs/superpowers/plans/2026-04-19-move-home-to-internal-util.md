# Move Home Package To Internal Util Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 `home` 包迁移到 `internal/util`，更新所有内部引用，并删除旧包且不保留兼容 shim。

**Architecture:** 保持 `Home()` 和 `Expand()` API 不变，只迁移包路径与包名；所有引用统一切换到 `internal/util`，最后删除根级 `home` 目录。

**Tech Stack:** Go 1.24, internal package refactor

---

## Task 1: 迁移 util 实现与引用

**Files:**
- Create: `internal/util/home.go`
- Modify: `internal/config/paths.go`
- Modify: `internal/install/network.go`
- Modify: `internal/app/install.go`
- Modify: `internal/app/install_test.go`
- Modify: `internal/installed/store.go`

- [x] 复制 `home` 实现到 `internal/util`
- [x] 更新所有引用与测试导入

## Task 2: 删除旧包并验证

**Files:**
- Delete: `home/home.go`
- Modify: `docs/superpowers/plans/2026-04-19-move-home-to-internal-util.md`

- [x] 删除旧 `home` 包
- [x] 运行 `go test ./...`
- [x] 回填 checklist 并提交
