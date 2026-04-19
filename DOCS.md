# Eget Documentation

## Overview

当前 CLI 采用显式子命令结构：

```text
eget <command> --options... arguments...
```

命令集合：

- `install`
- `download`
- `add`
- `update`
- `config`

根命令不再承担默认安装行为。

## Runtime Layout

- `cmd/eget/main.go`: 进程入口
- `internal/cli`: `gookit/cflag/capp` 命令注册、参数绑定、输出
- `internal/app`: 用例编排层
- `internal/install`: 查找、检测、下载、校验、提取执行链路
- `internal/config`: 配置文件路径、加载、合并、写回
- `internal/installed`: 安装记录读写
- `internal/source/github`: GitHub release/source 查找

## Install Flow

`install` 的主流程在 `internal/app/install.go` 与 `internal/install/runner.go`：

1. 解析目标类型
2. 选择 finder
3. 枚举候选资产
4. 按 `system` / `asset_filters` 选择资产
5. 下载内容
6. 执行 SHA-256 自动校验（如果有匹配校验文件）
7. 选择 extractor 并提取
8. 写入 installed store

目标类型支持：

- repo 标识符
- GitHub URL
- 直链 URL
- 本地文件

## Download Flow

`download` 与 `install` 复用同一条执行链路，只是 app 层会强制 `DownloadOnly=true`，并且不写 installed store。

当目标是远程 URL 时，执行链路会优先检查 `cache_dir` 对应的缓存文件：

- 命中缓存时直接复用，不再发起网络下载
- 未命中时正常下载，并在成功后回写缓存

当前缓存策略是最小实现：

- 缓存键使用 URL hash
- 文件名保留原始 URL 的扩展名，缺省时使用 `.bin`
- 目前不做过期策略、ETag 或 Last-Modified 校验

## Add Flow

`add` 不执行下载，只把一个可复用的安装描述写入 `[packages.<name>]`。

默认规则：

- `--name` 未提供时，默认使用 repo basename
- 保存 repo、tag、system、target、file、asset_filters、download_source、all、quiet 等可复用字段

## Update Flow

`update` 由 `internal/app/update.go` 驱动：

- `update <name>` 先查 `[packages.<name>]`
- `update owner/repo` 可以直接按 repo 更新
- `update --all` 遍历全部 managed packages

CLI 当前还保留：

- `--dry-run`
- `--interactive`

其中 `--all` 已接通；其余行为以当前实现为准。

## Config Flow

`config` 当前不是嵌套子命令树，而是这些形式：

- `config --info`
- `config --init`
- `config --list`
- `config get KEY`
- `config set KEY VALUE`

点路径示例：

- `global.target`
- `packages.fzf.repo`

## Config Model

配置模型定义在 `internal/config/model.go`。

支持的主结构：

```toml
[global]

["owner/repo"]

[packages.name]
```

兼容旧 repo section，同时新增 managed packages。

`config --init` 当前生成的默认全局配置为：

```toml
[global]
target = "~/.local/bin"
cache_dir = "~/.cache/eget"
system = ""
```

路径查找优先级：

1. `EGET_CONFIG`
2. `~/.eget.toml`
3. 平台 fallback 路径

安装选项合并优先级：

```text
CLI > package > repo > global > default
```

目录相关语义：

- `target`: 默认安装目录
- `cache_dir`: 默认下载缓存目录
- `download` 未传 `--to` 时，app 层会把输出目录回退到 `cache_dir`

## Installed Store

安装记录抽离到 `internal/installed`，用于：

- 记录安装结果
- 为资产回退选择提供历史信息
- 支撑 update 相关流程

## Option Surface

当前 CLI 已暴露的核心安装选项：

- `--tag`
- `--system`
- `--to`
- `--file`
- `--asset`
- `--source`
- `--all`
- `--quiet`

`update` 额外支持：

- `--all`
- `--dry-run`
- `--interactive`

## Constraints

由于 `cflag/capp` 的解析模型，参数顺序必须遵循：

```text
CMD --OPTIONS... ARGUMENTS...
```

支持：

```text
eget install --tag nightly inhere/markview
```

不支持：

```text
eget install inhere/markview --tag nightly
```

## Verification

常用验证命令：

```bash
go test ./internal/app -v
go test ./internal/cli -v
go test ./...
make build
make test
```
