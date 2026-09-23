# Cache Serve Web UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 `eget cache serve` 增加内置只读 Web UI，方便在浏览器中查看和下载缓存文件。

> **修订（2026-09-23）**：`eget cache serve` 命令与其内置只读页面已删除，缓存镜像协议与浏览能力并入 `eget web`（见 [2026-09-23-web-console-design.md](../specs/2026-09-23-web-console-design.md)）。`internal/app/cache/ui.go` 与 `ui_test.go` 已移除，机器接口改为 `internal/app/cache/machine.go` 中可挂载的 handler。本文保留为当时的实现记录。

**Architecture:** `server.go` 只增加根路径路由，HTML 数据整理和模板渲染放入 `internal/app/cache/ui.go`。UI 复用现有扫描、root scope 和 `/files/{relpath}` 下载 URL，不新增前端构建链路。

**Tech Stack:** Go, `html/template`, `net/http/httptest`, `github.com/gookit/goutil/x/assert`。

---

## Task 1: Web UI 路由和渲染

**Files:**
- Modify: `internal/app/cache/server.go`
- Create: `internal/app/cache/ui.go`
- Create: `internal/app/cache/ui_test.go`

- [x] **Step 1: 写失败测试：`GET /` 返回 HTML 文件列表**

Run:

```bash
go test ./internal/app/cache -run TestCacheServeUI -count=1
```

Expected: FAIL，因为根路径当前返回 404。

- [x] **Step 2: 实现根路径路由和 UI 渲染**

要求：

- `GET /` 返回 `text/html; charset=utf-8`。
- 页面包含服务名、版本、root scope、文件总数、总大小。
- 文件列表包含 kind、相对路径、大小、修改时间和下载链接。
- HTML 自动转义路径内容。

- [x] **Step 3: 运行 app cache 测试**

Run:

```bash
go test ./internal/app/cache -count=1
```

Expected: PASS。

- [x] **Step 4: Commit**

```bash
git add internal/app/cache/server.go internal/app/cache/ui.go internal/app/cache/ui_test.go docs/superpowers/plans/2026-05-27-cache-serve-web-ui.md
git commit -m "feat(cache): add cache serve web ui"
```

## Task 2: 文档和冒烟验证

**Files:**
- Modify: `README.md`
- Modify: `README.zh-CN.md`
- Modify: `docs/TODO.md`
- Modify: `docs/superpowers/plans/2026-05-27-cache-serve-web-ui.md`
- Modify: `AGENTS.md`

- [x] **Step 1: 更新文档**

说明 `cache serve` 根路径提供只读 Web UI。

- [x] **Step 2: 运行全量验证**

Run:

```bash
go test ./...
go build ./cmd/eget
```

Expected: PASS。

- [x] **Step 3: 手动验证首页**

Run:

```bash
eget cache serve --host 127.0.0.1 --port 8686 --no-index
```

Request:

```bash
curl http://127.0.0.1:8686/
```

Expected: HTML 中包含 `eget-cache`、文件统计和下载链接。

- [x] **Step 4: 完成收尾**

移除 `AGENTS.md` 正在进行工作项，提交文档和验证记录。
