# GitHub Repository Search Design

**Date:** 2026-04-24

## Goal

新增独立命令 `eget search`，用于搜索 GitHub 仓库，支持：

- 普通关键词搜索
- 保留 GitHub 原生搜索 qualifier 语法，例如 `user:junegunn`、`language:go`
- 表格输出与 `--json` 输出
- 基础结果分页控制（以 `--limit` 为主）

## Why Separate Command

不放入现有 `query --action ...`，原因如下：

- `query` 当前是“已知单仓库 -> 查询元数据”的职责
- `query` 现有输入要求是 `owner/repo`
- 仓库搜索的输入是关键字，输出是多个候选仓库，职责和数据模型都不同

因此 `search` 作为独立命令更符合现有 CLI 结构，也更便于用户理解。

## CLI Shape

目标命令：

```bash
eget search [--json --limit 10 --sort stars --order desc] keyword [user:junegunn language:go ...]
```

### Parsing Rules

- 第一个非 flag 参数视为主关键词 `keyword`
- 后续所有剩余参数原样视为 GitHub 搜索 qualifier / 额外搜索词
- 最终拼接为 GitHub search query 字符串：

```text
keyword user:junegunn language:go
```

### Flags

首版建议支持：

- `--json`: JSON 输出
- `--limit`: 返回结果数量，默认 10
- `--sort`: 透传 GitHub search sort，首版支持 `stars`、`updated`
- `--order`: 透传 GitHub search order，支持 `desc`、`asc`

不在首版支持分页游标、交互选择或 clone/install 联动。

## Data Model

新增搜索结果模型，字段以 GitHub repository search API 的高价值字段为主：

- `full_name`
- `description`
- `html_url`
- `homepage`
- `language`
- `stargazers_count`
- `forks_count`
- `open_issues_count`
- `updated_at`
- `archived`
- `private`

app 层结果建议：

- `SearchOptions`
- `SearchResult`
- `SearchRepo`

其中 `SearchResult` 包含：

- `query`
- `total_count`
- `items []SearchRepo`

## Architecture

### CLI Layer

新增 `internal/cli/search_cmd.go`：

职责：
- 注册 `search` 子命令
- 解析 flags 和剩余参数
- 将剩余参数拼接为查询串

### App Layer

新增 `internal/app/search.go`：

职责：
- 校验输入
- 规范化 `limit/sort/order`
- 调用 search client
- 返回统一结果模型

### Client Layer

建议新增 `internal/cli/search_client.go`，而不是塞进 `query_client.go`：

原因：
- `query_client` 当前围绕 repo/release/info 查询组织
- search API 是另一套 endpoint 和结果结构
- 独立文件边界更清楚，避免 query client 继续膨胀

调用 endpoint：

```text
GET https://api.github.com/search/repositories?q=...&per_page=...&sort=...&order=...
```

网络层继续复用现有：
- `githubAPIGetWithOptions`
- 全局代理配置
- API cache 行为

## Output

### Text Output

默认输出表格，列建议为：

- `Full Name`
- `Language`
- `Stars`
- `Updated`
- `Description`

说明：
- 描述字段可直接输出，不做复杂截断逻辑；先遵循现有 table 风格
- `updated_at` 用日期或 RFC3339，选与当前 query 输出一致的简洁格式

### JSON Output

`--json` 直接输出 `SearchResult` 的 JSON。

## Validation Rules

- 至少要求一个关键词参数
- `--limit <= 0` 时回退到默认值 10
- `--sort` 非允许值时报错
- `--order` 非允许值时报错
- 其余搜索 qualifier 不做语义解析，原样透传

## Testing Strategy

### CLI Tests

- 命令参数解析正确
- `keyword + extras` 正确拼接为 query 字符串
- `--json` / `--limit` / `--sort` / `--order` 正确透传

### App Tests

- 默认值填充正确
- 非法 `sort/order` 报错
- 空 query 报错

### Client Tests

- 正确构造 GitHub search API URL
- 正确解析 `total_count/items`
- 正确处理 GitHub 非 200 返回

## Non-Goals

首版明确不做：

- 搜索结果交互选择安装
- 搜索结果缓存到本地配置
- 多页翻取
- 高级 qualifier 语义校验
- 本地 fuzzy search

## Success Criteria

以下命令可工作：

```bash
eget search ripgrep
eget search ripgrep language:rust user:BurntSushi
eget search --limit 5 --sort stars --order desc terminal ui
eget search --json picoclaw user:sipeed
```

并且：
- 默认文本输出可读
- JSON 输出结构稳定
- `go test ./...` 通过
