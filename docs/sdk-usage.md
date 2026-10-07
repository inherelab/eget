# SDK 配置和使用说明

`eget sdk` 用于下载、解压和记录多版本 SDK，例如 Go、Node 等运行时归档包。它的边界是“下载管理”：不负责切换当前 SDK 版本，不修改 `PATH`，不写 shell hook，也不生成 `.xenv.toml`。如果需要 `use`、`env`、项目目录自动切换等能力，可以在安装后交给 `kite xenv` 等环境管理工具处理。

## 命令概览

```bash
eget sdk install go@1.22.0
eget sdk install go:1.22 node:20.11.1
eget sdk install --force go@1.22.0
eget sdk download go:1.22
eget sdk dl --os linux --arch arm64 go:1.22
eget sdk dl -o ./downloads go:1.22
eget sdk list
eget sdk list --json
eget sdk remove go@1.22.0
eget sdk search go 1.22 amd64 ^windows
eget sdk search --json node 20 linux
eget sdk index refresh go
eget sdk index refresh --all
eget sdk index show go
eget sdk index list
eget sdk index clear go
eget sdk index clear --all
```

常用别名：

- `sdk install`: `sdk i`, `sdk ins`
- `sdk download`: `sdk dl`
- `sdk list`: `sdk ls`
- `sdk remove`: `sdk rm`
- `sdk index`: `sdk idx`
- `sdk index list`: `sdk idx ls`
- `sdk index refresh`: `sdk idx build`

## Target 格式

`sdk install` 支持以下 target 格式：

- `go`: 安装索引里的最新稳定版本
- `go@latest`: 安装索引里的最新稳定版本
- `go@1.22.0`: 安装精确版本
- `go@1.22`: 安装 `1.22.x` 前缀中匹配到的最新稳定版本
- `go:1.22`: 等价于 `go@1.22`
- `node:20.11.1`: 安装 Node 精确版本

不支持 `go 1.22.0` 这种空格格式。这样可以保留参数位置，用于一次安装多个 SDK：

```bash
eget sdk install go:1.22 node:20.11.1
```

## 安装和删除

安装流程：

1. 解析 target，得到 SDK 名称和版本需求。
2. 读取 `[sdk.<name>]` 配置，合并 `os_map`、`arch_map`、`ext_map`。
3. 对 `latest` 或前缀版本读取本地 index cache，选择匹配版本和当前平台文件。
4. 下载归档到 `{cache_dir}/sdk-downloads/{sdk}/{version}/`。
5. 通过 `.part` 和 `.meta.json` 支持断点续传。
6. 解压到 `{sdk_target}/.eget-tmp/...` 临时目录。
7. 按 `strip_components` 剥离归档内路径前缀。
8. rename 到最终目录，并写入 `sdk.installed.json`。

`--force` 会在安全校验通过后删除已有目标目录并重新安装：

```bash
eget sdk install --force go@1.22.0
```

删除只允许删除安装记录中存在、并且路径通过 SDK 根目录安全校验的版本：

```bash
eget sdk remove go@1.22.0
```

`sdk remove` 需要明确版本，不支持 `go` 或 `go@latest`。

## 只下载归档

`sdk download` 只下载 SDK 归档，不解压、不写安装记录：

```bash
eget sdk download go:1.22
eget sdk dl go:1.22
```

默认下载当前平台归档，文件保存在 `{cache_dir}/sdk-downloads/{sdk}/{version}/`。如果归档已经完整存在且 meta 匹配，会直接复用缓存。

可以下载非当前平台归档：

```bash
eget sdk dl --os linux --arch arm64 go:1.22
eget sdk dl --os windows --arch amd64 node:20
```

`--os` 和 `--arch` 使用 Go 平台名，例如 `linux`、`windows`、`darwin`、`amd64`、`arm64`。发布文件名里的平台命名仍由 SDK 配置中的 `os_map`、`arch_map` 和 `ext_map` 决定。

可以把最终归档复制到指定目录：

```bash
eget sdk dl -o ./downloads go:1.22
```

`--output/-o` 只表示目录，不支持改名为单个文件。

## 搜索 Index Cache

`sdk search` 只搜索本地 SDK index cache，不会联网刷新。第一个参数固定是 SDK 名称，后续参数是搜索关键词：

```bash
eget sdk search go 1.22 amd64
eget sdk search go "1.22 amd64"
eget sdk search go 1.22 amd64 ^windows ^rc
eget sdk search --sort desc node REG:^22
eget sdk search -n 0 go 1.22 amd64
eget sdk search --json node 20 linux
eget sdk search --prerelease go 1.22
```

默认只展示 stable 版本；需要连同 prerelease（非正式版本）一起列出时加 `--prerelease`，此时才与旧行为一致。

多个关键词使用 AND 匹配，所有普通关键词都必须命中。以 `^` 开头的关键词表示排除，语义和 asset filter 的 exclude 类似：

- `amd64`：结果中必须包含 `amd64`。
- `^windows`：结果中不能包含 `windows`。
- `^rc`：结果中不能包含 `rc`。
- `REG:^22`：用正则匹配字段，例如匹配版本号以 `22` 开头的 Node 版本。

搜索字段包括版本号、stable/prerelease 状态、文件的 `os`、`arch`、`ext`、`filename` 和 `url`。输出按匹配到的文件展示，每行对应一个 index asset 文件。

默认最多显示 20 条结果。可以通过 `-n, --number` 调整数量，设置为 `0` 或负数表示不限制：

```bash
eget sdk search --number 50 go 1.22
eget sdk search -n 0 go amd64
```

可以通过 `--sort asc|desc` 按版本号升序或倒序排序。排序会先于 `-n, --number` 限制执行：

```bash
eget sdk search --sort desc -n 5 node REG:^22
```

## Index 管理

`latest` 和 `go:1.22` 这类版本解析依赖 SDK index cache。首次使用前通常需要刷新：

```bash
eget sdk index refresh go
eget sdk index refresh node
```

也可以刷新所有配置了 `index_url` 的 SDK：

```bash
eget sdk index refresh --all
```

查看缓存列表：

```bash
eget sdk index list
eget sdk index list --json
```

查看单个 SDK index 摘要：

```bash
eget sdk index show go
```

`show` 输出统计信息，而不是完整 JSON，包括：

- SDK 名称和来源 URL
- 拉取时间
- 版本总数
- 稳定版本数
- 文件总数
- latest 和 latest stable
- 平台文件统计
- 最近若干版本

清理缓存：

```bash
eget sdk index clear go
eget sdk index clear --all
```

## 全局配置

SDK 相关全局配置放在 `[global]`：

```toml
[global]
cache_dir = "~/.cache/eget"
sdk_target = "~/.local/sdks"
sdk_ext_map = { windows = "zip", linux = "tar.gz", darwin = "tar.gz" }
```

字段说明：

- `cache_dir`: 缓存根目录。SDK index 默认写入 `{cache_dir}/sdk-index/`，下载归档默认写入 `{cache_dir}/sdk-downloads/`。
- `sdk_target`: SDK 安装根目录。SDK 配置里的相对 `target` 会基于该目录解析。
- `sdk_ext_map`: 默认归档扩展名映射，key 使用 Go 的 OS 名称，例如 `windows`、`linux`、`darwin`。

SDK 安装记录默认写入：

```text
~/.config/eget/sdk.installed.json
```

这个 store 和 package 安装记录 `installed.toml` 分开，避免多版本 SDK 语义污染普通单版本工具记录。

## 内置 SDK 配置模板

可以通过 `eget sdk config add` 写入内置 SDK 配置模板，避免手写 `[sdk.<name>]`。

默认使用官方源：

```bash
eget sdk config add go
eget sdk config add node
eget sdk config add jdk
eget sdk config add --all
```

使用内置镜像源：

```bash
eget sdk config add go --mirror aliyun
eget sdk config add node --mirror aliyun
eget sdk config add jdk --mirror huawei
eget sdk config add jdk --mirror zulu
eget sdk config add --all --mirror mirror
```

已存在的 SDK 配置默认不会覆盖。需要覆盖时使用：

```bash
eget sdk config add jdk --force
```

## SDK 配置字段

每个 SDK 用 `[sdk.<name>]` 配置：

```toml
[sdk.go]
aliases = ["golang"]
target = "gosdk/go{version}"
url_template = "https://mirrors.aliyun.com/golang/go{version}.{os}-{arch}.{ext}"
index_url = "https://mirrors.aliyun.com/golang/"
index_format = "html"
filename_pattern = "go{version}.{os}-{arch}.{ext}"
strip_components = 1
ext_map = { windows = "zip", linux = "tar.gz", darwin = "tar.gz" }
```

字段说明：

- `aliases`: SDK 别名。例如 `[sdk.go]` 配置 `aliases = ["golang"]` 后，可以使用 `eget sdk install golang@1.22.0`。
- `target`: 安装目录模板。相对路径会基于 `global.sdk_target` 解析。
- `url_template`: 归档下载 URL 模板。
- `index_url`: 远端 HTML 或 JSON index 地址。
- `index_format`: index 格式，当前常用 `html` 或 `json`。
- `index_parser`: JSON index 解析器，当前支持 `go-json`、`node-json` 和 `zulu-json`。
- `index_path_prefix`: HTML index 链接路径前缀过滤。
- `filename_pattern`: HTML index 归档文件名模板。
- `strip_components`: 解压时剥离归档内路径前缀层数。
- `os_map`: 当前系统 OS 到 SDK 发布命名的映射。
- `arch_map`: 当前 CPU arch 到 SDK 发布命名的映射。
- `ext_map`: 当前系统 OS 到归档扩展名的映射。SDK 级别会覆盖 `global.sdk_ext_map`。

`target`、`url_template`、`filename_pattern` 支持变量：

- `{name}`: SDK 名称
- `{version}`: 版本号
- `{os}`: 经过 `os_map` 处理后的 OS 名称
- `{arch}`: 经过 `arch_map` 处理后的架构名称
- `{ext}`: 经过 `ext_map` 处理后的归档扩展名

## HTML Index 解析

HTML index 会读取页面里的 `<a href>` 链接。

第一种情况是页面直接包含归档文件链接，例如 Go 镜像：

```text
go1.22.0.linux-amd64.tar.gz
go1.22.0.windows-amd64.zip
```

此时 `filename_pattern` 用来提取版本、OS、arch、ext：

```toml
filename_pattern = "go{version}.{os}-{arch}.{ext}"
```

第二种情况是页面只包含版本目录链接，例如 Node 镜像：

```text
v20.11.1/
v20.12.0/
```

这种情况下，eget 会从目录名提取版本号，再用 `url_template` 生成当前平台的归档 URL。这样不需要递归请求每个版本目录。

## JSON Index 解析

JSON index 需要配置内置 parser：

```toml
index_format = "json"
index_parser = "go-json"
```

当前支持：

- `go-json`: Go release JSON
- `node-json`: Node release JSON
- `zulu-json`: Azul Zulu package JSON

如果某个 SDK 的 JSON 结构不兼容内置 parser，需要后续扩展 parser 后才能直接使用 JSON index。

## Go 示例

```toml
[sdk.go]
aliases = ["golang"]
target = "gosdk/go{version}"
url_template = "https://mirrors.aliyun.com/golang/go{version}.{os}-{arch}.{ext}"
index_url = "https://mirrors.aliyun.com/golang/"
index_format = "html"
filename_pattern = "go{version}.{os}-{arch}.{ext}"
strip_components = 1
ext_map = { windows = "zip", linux = "tar.gz", darwin = "tar.gz" }
```

使用：

```bash
eget sdk index refresh go
eget sdk install go@1.22.0
eget sdk install go:1.22
eget sdk list go
```

## Node 示例

```toml
[sdk.node]
aliases = ["nodejs"]
target = "nodejs/node{version}"
url_template = "https://mirrors.aliyun.com/nodejs-release/v{version}/node-v{version}-{os}-{arch}.{ext}"
index_url = "https://mirrors.aliyun.com/nodejs-release/"
index_format = "html"
index_path_prefix = "/nodejs-release/"
filename_pattern = "node-v{version}-{os}-{arch}.{ext}"
strip_components = 1
os_map = { windows = "win", linux = "linux", darwin = "darwin" }
arch_map = { amd64 = "x64", arm64 = "arm64", 386 = "x86" }
ext_map = { windows = "zip", linux = "tar.xz", darwin = "tar.gz" }
```

使用：

```bash
eget sdk idx build node
eget sdk install node:20.11.1
eget sdk install node@latest
eget sdk idx show node
```

阿里云 Node index 根目录主要是 `v20.11.1/` 这类版本目录，eget 会用 `url_template` 为当前平台生成下载 URL。

## 其它 SDK 是否可用

其它 SDK 满足以下条件时，也可以用同一套配置接入：

- 有稳定的归档下载 URL 规则，可以用 `url_template` 描述。
- 归档文件名或版本目录能解析出版本号。
- 当前平台可以通过 `os_map`、`arch_map`、`ext_map` 映射到发布命名。
- 解压后的目录结构能用 `strip_components` 处理。
- 如果需要 `latest` 或前缀版本，必须有可解析的 HTML/JSON index。

不满足这些条件的 SDK 仍可以后续通过新增 parser 或 provider 扩展。

## 与环境管理工具配合

`eget sdk` 安装完成后，外部工具可以通过两种方式发现 SDK：

- 扫描 `global.sdk_target` 下的安装目录。
- 读取 `~/.config/eget/sdk.installed.json`。

推荐职责划分：

- `eget`: 下载、断点续传、解压、安装记录、index cache。
- `kite xenv`: SDK 激活、PATH/env 切换、项目配置、shell 集成。

这样可以避免 `eget` 变成完整环境管理器，保持它作为下载和安装工具的主旨。
