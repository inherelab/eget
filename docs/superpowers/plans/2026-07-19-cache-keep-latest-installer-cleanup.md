# Cache Keep-Latest 与 Installer 缓存优化 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复真实 pkg-cache 列表中的旧版本漏删，并消除直接 installer 的重复缓存，同时让 keep-latest 清理派生 installer 文件。

**Architecture:** 保持现有纯文件名解析和 preview/apply 清理架构。启动阶段使用与下载阶段相同的 metadata 计算 cache path，直接 asset 复用该文件；归档内容仍按原路径物化。

**Tech Stack:** Go、标准库、`github.com/gookit/goutil/x/assert`、GitNexus CLI。

---

### Task 1: Git describe 与嵌入版本 family

**Files:** `internal/app/cache/keep_latest.go`、`internal/app/cache/keep_latest_test.go`

- [x] 写失败测试：验证 Git describe 可解析和比较；DBX/OxideTerm 多版本归入同一 family。
- [x] 运行 `go test ./internal/app/cache -run "Test(ParseAssetVersion|CompareAssetVersion|ParseKeepLatestEntry|SelectKeepLatest)"`，确认因缺少新行为而失败。
- [x] 在 `assetVersion` 保存可选 commit count；精确接受 `core-count-ghex[-dirty]`；core 相等时比较 count。
- [x] 只删除 raw name 中唯一、由分隔符包围且等于 appended version 的版本段。
- [x] 运行 `go test ./internal/app/cache`，预期通过。
- [x] 运行 `npx gitnexus detect-changes --repo eget --scope all` 后提交 `fix(cache): recognize cached build versions`。

### Task 2: 直接 installer 复用 pkg-cache

**Files:** `internal/install/runner.go`、`internal/install/runner_installer.go`、`internal/install/runner_installer_test.go`

- [x] 写失败测试：直接 `.exe`/`.msi` 使用 metadata cache path 且不创建 `installers/`；归档内 installer 仍调用 `Extract`。
- [x] 运行 `go test ./internal/install -run "TestRun.*Installer.*(Cache|Archive)"`，确认直接 installer 仍从 `installers/` 启动。
- [x] 使用下载阶段同一 metadata 计算 cache path；asset 本身为 installer 且该路径存在时直接返回，否则保持归档物化路径。
- [x] 运行 `go test ./internal/install`，预期通过。
- [x] 运行 `npx gitnexus detect-changes --repo eget --scope all` 后提交 `fix(install): reuse cached installer assets`。

### Task 3: keep-latest 清理 installers

**Files:** `internal/app/cache/keep_latest.go`、`internal/app/cache/service.go`、`internal/app/cache/cache_test.go`

- [x] 写失败测试：preview 同时匹配旧 pkg 与 `installers/setup.exe`，保留最新 pkg；apply 删除两个候选。
- [x] 运行 `go test ./internal/app/cache -run TestServiceKeepLatest`，确认 installer 未进入 matched。
- [x] 保持版本选择仅解析 `pkg-cache/`，在 keep-latest preview 中把严格位于 `installers/` 的完整普通文件直接加入候选。
- [x] 运行 `go test ./internal/app/cache`，预期通过。
- [x] 运行 `npx gitnexus detect-changes --repo eget --scope all` 后提交 `fix(cache): clean derived installer files`。

### Task 4: 全量验证与进度收尾

- [x] 运行 `go test ./...`，预期通过。
- [x] 运行 `staticcheck ./...`，预期无输出且退出码 0。
- [x] 运行 `npx gitnexus detect-changes --repo eget --scope all`，确认只有预期链路。
- [x] 将本计划 checkbox 全部更新为完成，并从 `AGENTS.md` 正在进行的工作移除本任务。
