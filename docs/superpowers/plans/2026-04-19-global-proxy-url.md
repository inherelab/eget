# Global Proxy URL Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 `global` 配置新增 `proxy_url`，并让 GitHub 查询与远程下载统一走该代理。

**Architecture:** 在配置模型中增加 `proxy_url` 字段，只允许定义在 `global`；app 层在解析安装选项时把它透传到 `install.Options`；网络层优先使用该值构造 HTTP 代理，未配置时回退到环境变量代理逻辑。

**Tech Stack:** Go 1.24, `net/http`, `net/url`, TOML config

---

## Task 1: 配置模型与 app 层透传

**Files:**
- Modify: `internal/config/model.go`
- Modify: `internal/config/merge.go`
- Modify: `internal/config/merge_test.go`
- Modify: `internal/app/config.go`
- Modify: `internal/app/config_test.go`
- Modify: `internal/app/install.go`
- Modify: `internal/app/install_test.go`

- [x] 增加 `global.proxy_url` 字段与合并测试
- [x] 支持 `config --init/get/set/list` 处理 `global.proxy_url`
- [x] 让 `resolveInstallOptions()` 透传 `proxy_url`

## Task 2: 网络层代理支持

**Files:**
- Modify: `internal/install/options.go`
- Modify: `internal/install/network.go`
- Modify: `internal/install/runner_test.go`

- [x] 为 `install.Options` 增加 `ProxyURL`
- [x] 让 HTTP client 优先使用 `opts.ProxyURL`
- [x] 补充代理解析测试

## Task 3: 文档与验证

**Files:**
- Modify: `README.md`
- Modify: `DOCS.md`
- Modify: `man/eget.md`
- Modify: `docs/superpowers/plans/2026-04-19-global-proxy-url.md`

- [x] 更新配置文档中的 `proxy_url`
- [x] 运行 `go test ./...`
- [x] 回填 checklist 并提交
