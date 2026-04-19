# Eget

`eget` 用于查找、下载并提取预构建二进制。当前版本已经重构为显式子命令 CLI，入口在 `cmd/eget/main.go`，业务逻辑集中在 `internal/`。

## 命令风格

```text
eget <command> --options... arguments...
```

根命令不再兼容旧写法，以下形式都不再支持：

```text
eget REPO
eget --tag nightly REPO
eget install REPO --tag nightly
```

必须改为：

```text
eget install --tag nightly owner/repo
```

## 示例

```bash
eget install --tag nightly inhere/markview
eget install --to ~/.local/bin/fzf junegunn/fzf
eget download --file go --to ~/go1.17.5 https://go.dev/dl/go1.17.5.linux-amd64.tar.gz
eget add --name fzf --to ~/.local/bin junegunn/fzf
eget update fzf
eget update --all
eget config --info
eget config --init
eget config --list
eget config get global.target
eget config set global.target ~/.local/bin
```

## 当前命令

`install`

- 查找、下载、校验、提取目标，并记录安装状态。

`download`

- 复用安装链路，但只做下载/提取，不记录 installed store。

`add`

- 将一个托管包写入配置文件的 `[packages.<name>]`。

`update`

- 更新单个托管包，或通过 `--all` 更新全部托管包。

`config`

- 支持 `--info`、`--init`、`--list`、`get KEY`、`set KEY VALUE`。

## 支持的目标

`install` 和 `download` 的目标参数可以是：

- `owner/repo`
- GitHub 仓库 URL
- 直接下载 URL
- 本地文件

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

`update` 额外支持：

- `--all`
- `--dry-run`
- `--interactive`

说明：

- `--asset` 当前按单值字符串解析，再映射到内部 `[]string`。
- `--cache-dir` 用于覆盖配置中的 `cache_dir`，控制远程下载缓存目录。
- 参数顺序遵循 `cflag/capp` 约束，必须是 `CMD --OPTIONS... ARGUMENTS...`。

## 配置

配置文件位置按以下顺序解析：

1. `EGET_CONFIG`
2. `~/.eget.toml`
3. XDG / LocalAppData fallback 路径

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

- `eget config --init` 会写入 `global.target = "~/.local/bin"`
- `eget config --init` 会写入 `global.cache_dir = "~/.cache/eget"`
- `eget config --init` 会写入 `global.proxy_url = ""`

目录语义：

- `target` 是默认安装目录
- `cache_dir` 是默认下载缓存目录
- `proxy_url` 是全局远程请求代理，GitHub 查询和远程下载都会使用它
- `download` 在未指定 `--to` 时默认使用 `cache_dir`
- `install`/`download` 对远程 URL 的原始下载内容会优先复用 `cache_dir` 中的缓存文件

## 构建与测试

```bash
make build
make test
```

构建入口已经切到：

```text
./cmd/eget
```

版本信息通过 `internal/version` 注入。

## 开发结构

- `cmd/eget`: 命令入口
- `internal/cli`: `capp` 命令注册与参数绑定
- `internal/app`: install/add/update/config 用例编排
- `internal/install`: 查找、下载、校验、提取执行链路
- `internal/config`: 配置加载、合并、写回
- `internal/installed`: 安装记录存储
- `internal/source/github`: GitHub 资源查找

更详细说明见 `DOCS.md`。
