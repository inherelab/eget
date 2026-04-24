# Eget

![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/inherelab/eget?style=flat-square)
[![GitHub tag (latest SemVer)](https://img.shields.io/github/tag/inherelab/eget)](https://github.com/inherelab/eget)
[![Unit-Tests](https://github.com/inherelab/eget/actions/workflows/go.yml/badge.svg)](https://github.com/inherelab/eget)

---

[English](./README.md) | [简体中文](./README.zh-CN.md)

`eget` 用于从 GitHub 查找、下载并提取预构建二进制。

> Forked from https://github.com/zyedidia/eget 重构并增强了工具的功能。

## 功能特性

- 显式子命令 CLI：统一使用 `eget <command> --options... arguments...` 形式，命令职责清晰，便于扩展和自动化调用。
- 多种目标输入：`install` 和 `download` 支持 `owner/repo`、GitHub 仓库 URL、直接下载 URL 以及本地文件。
- 下载、校验、提取一体化：内置资源发现、系统/资产筛选、SHA-256 自动校验与归档提取流程，减少手工步骤。
- 缓存与代理支持：支持 `cache_dir` 下载缓存复用、`api_cache` GitHub API 响应缓存，以及 `proxy_url`/`ghproxy` 组合代理远程请求。
- 托管包生命周期管理：通过 `add`、`list`、`update`、`uninstall` 管理本地 package 定义、安装状态和卸载流程。
- 安装状态可追踪：独立记录 installed store，保存最近一次安装的资源、时间、输出文件等信息，便于查询与回收。
- 配置分层合并：支持 `global`、repo section、`packages.<name>` 多层配置，并按约定优先级合并安装参数。
- 默认配置目录统一：配置文件和 installed store 默认写入 `~/.config/eget/`，同时兼容旧路径读取。

## 安装

- 从 Releases 下载安装 [https://github.com/inherelab/eget/releases](https://github.com/inherelab/eget/releases)
- 使用命令 `go install` 安装。(require Go sdk)

```bash
go install github.com/inherelab/eget/cmd/eget@latest
```

## 命令风格

```bash
eget <command> --options... arguments...
```

示例：

```bash
eget install --tag nightly owner/repo
```

## 示例

**安装命令示例**

```bash
# install
eget install --tag nightly inhere/markview
# 安装并指定可执行文件名
eget install --name chlog gookit/gitw
# 安装 zip 资产
eget install --asset zip windirstat/windirstat
# 使用正则筛选资源
eget install --asset "REG:\\.deb$" owner/repo
# 安装到指定目录
eget install --to ~/.local/bin/fzf junegunn/fzf
# 安装 并 记录
eget install --add junegunn/fzf
eget install --add --name rg BurntSushi/ripgrep
```

**下载命令示例**

```bash
# download
eget download ip7z/7zip
eget download --file go --to ~/go1.17.5 https://go.dev/dl/go1.17.5.linux-amd64.tar.gz
eget download --file README.md,LICENSE --to ./dist owner/repo
eget download --file "*.txt" owner/repo
eget download --file "bin/*" owner/repo
eget download --all --to ./dist windirstat/windirstat
```

**其他命令示例**

```bash
# uninstall
eget uninstall fzf
# list config and installed store
eget list|ls
# query repo info
eget query owner/repo
eget query --action releases --limit 5 owner/repo
eget query --action assets --tag v1.2.3 owner/repo
# update fzf
eget update fzf
eget update --all
```

**配置命令示例**

```bash
# config
eget add --name fzf --to ~/.local/bin junegunn/fzf
eget config init
eget config list|ls
eget config get global.target
eget config set global.target ~/.local/bin
```

### 支持的目标

`install` 和 `download` 的目标参数可以是：

- `name` in the config packages
- `owner/repo`
- GitHub 仓库 URL
- 直接下载 URL
- 本地文件

## 当前命令

`install`(alias: `i`, `ins`)

- 查找、下载、校验、提取目标，并记录安装状态。
- 可通过 `--name` 指定安装后的可执行文件名；未指定 `--to` 时，也会作为单文件资产的重命名提示。
- 传入 `--add` 时，安装成功后会自动将 repo 目标写入 `[packages.<name>]`；可配合 `--name` 指定包名。

`download`(alias: `dl`)

- 复用安装链路，但不记录 installed store。
- 默认仅下载原始 asset；只有设置 `--file` 或 `--all` 时才会自动解压归档内容。

`add`

- 将一个托管包写入配置文件的 `[packages.<name>]`。

`uninstall`(alias: `uni`, `remove`, `rm`)

- 删除已安装文件并清理 installed store 记录，不移除 `[packages.<name>]` 配置。

`list`(alias: `ls`)

- 列出本地 managed packages 与 installed store 的并集，并尽可能关联最近一次安装状态。

`query`(alias: `q`)

- 查询 GitHub repo 的 release 与元数据，不涉及安装或本地状态写入。
- 默认 action 为 `latest`，可通过 `--action` 切换为 `info`、`releases`、`assets`。

`update`(alias: `up`)

- 更新单个托管包，或通过 `--all` 更新全部托管包。

`config`(alias: `cfg`)

- 支持 `init`、`list` / `ls`、`get KEY`、`set KEY VALUE`。

## 主要选项

`install`、`download`、`add` 共享这些安装相关选项：

- `--tag`: 指定发布版本标签；未提供时默认使用 `latest`。
- `--system`: 指定目标系统与架构，例如 `windows/amd64`、`linux/arm64`。
- `--to`: 指定安装或下载输出路径；可传目录，也可传完整文件路径。
- `--file`: 指定归档内要提取的文件；支持逗号分隔多个文件或 glob 模式，例如 `README.md,LICENSE`。
- `--asset`: 指定资源过滤关键词；可用逗号分隔多个过滤条件，也支持 `REG:` 前缀正则，例如 `REG:\\.deb$`，排除可用 `^REG:...`。
- `--source`: 下载源码归档而不是预构建二进制。
- `--all`: 提取归档中的全部文件，而不是只选择一个目标文件。
- `--quiet`: 精简常规输出，适用于脚本或批处理场景。

> 缓存目录请通过 `config set global.cache_dir ...` 或配置文件中的 `cache_dir` 设置。

`install` 额外支持：

- `--add`: 安装成功后，将 repo 目标追加到 `[packages.<name>]` 托管配置中。
- `--name`: 指定托管包名；对于单文件可执行资产，也会作为默认输出文件名提示。

`update` 支持选项：

- `--all`: 更新全部托管包，而不是只更新单个目标。
- `--dry-run`: 仅预览更新计划，不执行实际安装。
- `--interactive`: 交互式选择要更新的托管包。

`query` 支持选项：

- `--action`, `-a`: 查询动作，支持 `latest`、`releases`、`assets`、`info`。
- `--tag`, `-t`: 为 `assets` 动作指定 release tag；不传时默认查询 latest。
- `--limit`, `-l`: 限制 `releases` 动作返回数量，默认 `10`。
- `--json`, `-j`: 使用 JSON 输出结果，方便脚本处理。
- `--prerelease`, `-p`: 在 `latest` / `releases` 中包含预发布版本。

全局选项：

- `-v`, `--verbose`: 输出更详细的调试信息，例如请求的 API、响应摘要、asset 选择、缓存命中和关键流程节点。

说明：

- `install --name` 可用于指定单文件可执行资产的输出文件名，例如将 `chlog-windows-amd64.exe` 安装为 `chlog.exe`。
- `install --add` 仅对 repo 目标生效，并在安装成功后追加托管包配置。
- `download` 默认保存原始下载文件；只有设置了 `--file` 或 `--all` 才会自动提取归档内容。
- 归档提取当前支持 `zip`、`tar.*` 以及 `7z`。
- 参数顺序遵循 `cflag/capp` 约束，必须是 `CMD --OPTIONS... ARGUMENTS...`。

## 配置

配置文件位置按以下顺序解析：

1. `EGET_CONFIG`
2. `~/.config/eget/eget.toml`
3. XDG / LocalAppData fallback 路径
4. 旧路径 `~/.eget.toml`

配置同时支持：

- `[global]`
- `["owner/repo"]`
- `[packages.<name>]`

示例：

```toml
[global]
target = "~/.local/bin"
cache_dir = "~/.cache/eget"
proxy_url = "http://127.0.0.1:7890"
system = "windows/amd64"

[api_cache]
enable = false
cache_time = 300

[ghproxy]
enable = false
host_url = ""
support_api = true
fallbacks = []

["inhere/markview"]
tag = "nightly"

[packages.markview]
repo = "inhere/markview"
target = "~/.local/bin"
tag = "nightly"
asset_filters = ["windows"]
```

常见字段：

- `target`
- `cache_dir`
- `proxy_url`
- `api_cache.enable`
- `api_cache.cache_time`
- `ghproxy.enable`
- `ghproxy.host_url`
- `ghproxy.support_api`
- `ghproxy.fallbacks`
- `system`
- `tag`
- `file`
- `asset_filters`
- `download_source`
- `all`
- `quiet`
- `upgrade_only`

默认初始化配置：

```bash
eget config init
```

会写入:
- `global.target = "~/.local/bin"`
- `global.cache_dir = "~/.cache/eget"`
- `global.proxy_url = ""`
- `api_cache.enable = false`
- `api_cache.cache_time = 300`
- `ghproxy.enable = false`
- `ghproxy.host_url = ""`
- `ghproxy.support_api = true`

默认会写入 `~/.config/eget/eget.toml`。

目录语义：

- `target` 是默认安装目录
- `cache_dir` 是默认下载缓存目录
- `proxy_url` 是全局远程请求代理，GitHub 查询和远程下载都会使用它
- `api_cache` 仅缓存 GitHub API 的 `GET` 响应，缓存文件目录派生为 `{cache_dir}/api-cache/`
- `cache_time` 单位为秒；缓存过期后会重新请求并刷新缓存
- `ghproxy` 会重写 GitHub 资源下载 URL；当 `support_api = true` 时，也会重写 `api.github.com` 请求
- `ghproxy.fallbacks` 会在主代理失败时按顺序回退重试
- `proxy_url` 是 HTTP 层代理，`ghproxy` 是请求 URL 重写，两者可以同时启用
- `download` 在未指定 `--to` 时默认使用 `cache_dir`
- `install`/`download` 对远程 URL 的原始下载内容会优先复用 `cache_dir` 中的缓存文件

安装记录 store 默认也会写入 `~/.config/eget/installed.toml`。

## 构建与测试

```bash
make build
make test
```

## 开发结构

当前版本已经重构为显式子命令 CLI，入口在 `cmd/eget/main.go`，业务逻辑集中在 `internal/`。

- `cmd/eget`: 命令入口
- `internal/cli`: `capp` 命令注册与参数绑定
- `internal/app`: install/add/list/update/config 用例编排
- `internal/install`: 查找、下载、校验、提取执行链路
- `internal/config`: 配置加载、合并、写回
- `internal/installed`: 安装记录存储
- `internal/source/github`: GitHub 资源查找

> 更详细说明见 [docs/DOCS.md](docs/DOCS.md)。

## 参考项目

- [https://github.com/zyedidia/eget](https://github.com/zyedidia/eget)
- [https://github.com/gmatheu/eget](https://github.com/gmatheu/eget)
