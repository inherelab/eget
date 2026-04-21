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
- 缓存与代理支持：支持 `cache_dir` 下载缓存复用，以及 `proxy_url` 统一代理 GitHub 查询与远程下载请求。
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

**安装示例**

```bash
# install
eget install --tag nightly inhere/markview
# 安装并指定可执行文件名
eget install --name chlog gookit/gitw
# 安装到指定目录
eget install --to ~/.local/bin/fzf junegunn/fzf
# 安装 并 记录
eget install --add junegunn/fzf
eget install --add --name rg BurntSushi/ripgrep
```

**其他命令示例**

```bash
# download
eget download --file go --to ~/go1.17.5 https://go.dev/dl/go1.17.5.linux-amd64.tar.gz
# uninstall
eget uninstall fzf
eget list
# update
eget update fzf
eget update --all
```

**配置命令示例**

```bash
# config
eget add --name fzf --to ~/.local/bin junegunn/fzf
eget config init
eget config list|show
eget config get global.target
eget config set global.target ~/.local/bin
```

### 支持的目标

`install` 和 `download` 的目标参数可以是：

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

- 复用安装链路，但只做下载/提取，不记录 installed store。

`add`

- 将一个托管包写入配置文件的 `[packages.<name>]`。

`uninstall`(alias: `uni`, `remove`, `rm`)

- 删除已安装文件并清理 installed store 记录，不移除 `[packages.<name>]` 配置。

`list`(alias: `ls`)

- 列出本地 managed packages 与 installed store 的并集，并尽可能关联最近一次安装状态。

`update`(alias: `up`)

- 更新单个托管包，或通过 `--all` 更新全部托管包。

`config`(alias: `cfg`)

- 支持 `init`、`list` / `ls` / `show`、`get KEY`、`set KEY VALUE`。

## 主要选项

`install`、`download`、`add` 共享这些安装相关选项：

- `--tag`
- `--system`
- `--to`
- `--cache-dir`
- `--file`
- `--asset`
- `--source`
- `--all`
- `--quiet`

`install` 额外支持：

- `--add`
- `--name`

`update` 额外支持：

- `--all`
- `--dry-run`
- `--interactive`

说明：

- `--asset` 当前按单值字符串解析，再映射到内部 `[]string`。
- `--cache-dir` 用于覆盖配置中的 `cache_dir`，控制远程下载缓存目录。
- `install --name` 可用于指定单文件可执行资产的输出文件名，例如将 `chlog-windows-amd64.exe` 安装为 `chlog.exe`。
- `install --add` 仅对 repo 目标生效，并在安装成功后追加托管包配置。
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

默认会写入 `~/.config/eget/eget.toml`。

目录语义：

- `target` 是默认安装目录
- `cache_dir` 是默认下载缓存目录
- `proxy_url` 是全局远程请求代理，GitHub 查询和远程下载都会使用它
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
