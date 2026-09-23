# TODO

<!--
简单的直接使用一行 checklist 说明即可。
需要附带较长说明的，使用标题+说明方式新建。使用emoji 表情状态图标(wait: ⏳|ing: 🔄|done: ✅)
-->

- [x] 增强 list --outdated 用于显示有更新的工具
- [x] 增强功能：参考自 https://github.com/marwanhawari/stew
  - [x] 新增命令 query 用于浏览 GitHub repository 的 releases
  - [x] 新增命令 search 用于搜索 GitHub 上的 repository
- [x] 新增配置 global.gui_target 用于指定 GUI 应用的安装目录
  - 同时 package 新增 isgui 字段用于指定是否为 GUI 应用, 如果是 msi, setup exe, 如何启动应用安装？
  - list 支持 --gui 选项用于显示 GUI 应用
- [ ] 新增命令 run 用于运行已安装的工具，即使它没有在 PATH 中
  - 如果是 GUI 应用，需要启动应用安装目录下的可执行文件
  - 如果是命令行工具，直接运行可执行文件
- [x] 增强 install/download/update 支持并发下载
  - `--chunk N` / `global.chunk_concurrency` 控制单文件 HTTP Range 分片并发
  - `--batch N` / `global.batch_concurrency` 控制 `install --all` / `update --all` 批处理并发
- [x] 优化 `list --outdated / update --check` 查询处理。
  - [x] 支持并发查询多个包信息 API
  - [x] 复用 `api_cache.cache_time`，`update --check` 后在缓存时间内执行 `update` 不会重复检查 GitHub API
- [x] 优化 新增 global.sys7z_path 用于指定 7z 可执行文件路径
  - 解压文件处理时优先使用系统环境中的 7z 可执行文件进行处理(优先从sys7z_path获取, 再从环境变量PATH中获取)
  - 如果系统环境中没有, 则使用 go mod 包进行解压处理
  - `--extract-all` 使用系统 7z 时直接一次性解压，不再先 list 文件列表
  - `--extract-all` 使用 Go 内置解压时单次遍历并流式写出，避免先缓存所有文件内容
- [ ] 新增 global.group_packages 用于配置 package 分组（详情见下面）
- [x] 全局配置 新增 global.ignore_update_packages 用于配置忽略检查/更新的 packages
- [x] 新增支持 sdk 下载安装，需要支持多版本。例如 go, node 等 sdk（详情见下面）
- [x] 增强 install/update 的 target 参数支持多个目标。eg: `install name1 name2 ...`
  - 只输入一个参数时，也支持使用逗号分隔，例如: `install name1,name2,name3`
- [x] eget list 新增 --no-installed 用于显示 config 里未安装的 package
- [x] package config 新增 desc 字段用于指定 package 的描述，可以手动设置，为空时默认从 repository 中获取
  - 没有添加config 的 package, 但是 installed 里的 package 也会记录描述信息，方便查看
- [x] 新增命令 show 用于显示 package 详情: 配置信息+installed 信息, 包括版本, 安装路径, 状态, 项目主页URL 等
- [x] 增强 asset filters 支持平台前缀规则，只会在当前平台生效。例如: `windows:zip`, `linux:tar.gz`, `darwin:tar.gz`
- [x] sdk 内置 go, node, jdk 的配置模板，方便快速配置后使用。默认不初始化到配置，通过 sdk config add 初始化到配置的 sdk 下。
- [x] pkg_templates 功能。
  - [x] 通过 `[pkg_templates.<name>]` 复用 `latest_url`、`url_template` 等 template package 字段。
  - [x] 支持 `eget add mydev:markview`、`eget install mydev:markview` 和 `eget install --add mydev:markview`。
  - [x] 落盘 package 保留轻量引用：`repo = "pkg-template:mydev:markview"`。
- [x] 新增 eget 缓存管理命令 `cache` 第一期。
  - [x] `cache clean` 清理下载文件、API cache、SDK 下载缓存和未完成下载状态，默认清理 3 天前的缓存。
  - [x] `cache serve` 启动只读内网 server，分享 package/sdk cache 文件。（2026-09-23：命令已删除，能力并入 `eget web`）
  - [x] `cache serve` 根路径提供内置只读 Web UI，方便浏览和下载缓存文件。（同上，内置页面由 web 控制台取代）
- [ ] 增强 cache mirror 自动复用能力。
  - [x] `cache serve` 增加 path-key 下载协议，基于缓存相对路径 md5 复用现有老缓存。（协议保留在 `eget web` 的 `/download/path-md5:<key>`）
  - [x] 客户端 install/download/sdk install 在回源前尝试使用局域网 cache mirror。
  - [ ] 后续 registry 化阶段再设计 source metadata、搜索和不依赖第三方 provider 的解析能力。
  - [x] `cache serve --token`。（由 `eget web --token` 承接）
  - [ ] manifest TTL。
- [ ] cache 运维和脚本集成增强。
  - [x] `cache list` 和 `cache status`。
  - [x] `cache clean --json` 和 `cache serve --json-log`。（请求日志由 `eget web --json-log` 承接）
- [ ] eget web 控制台（设计见 `docs/superpowers/specs/2026-09-23-web-console-design.md`，使用说明见 `docs/web.md`）。
  - [x] 服务骨架：rux/v2 路由、token 认证、Host/CSRF 防护、只读 API（overview/packages/outdated/ext/cache/config）。
  - [x] 接管缓存镜像协议（`/manifest.json`、`/download/*`、`/files/*`），并删除 `eget cache serve`。
  - [ ] Vite/React 前端工程与嵌入产物。
  - [ ] 任务引擎（串行队列 + SSE），以及更新/卸载/安装/配置编辑等写入接口。
- [x] 基于 `latest.yaml 简版 + url template` 配置支持自定义站点的工具下载
- [ ] ux 体验优化
  - 核心命令帮助信息都加上各种 example 示例说明
  - [x] eget update --self --check 命令, 先显示一下从哪个 host 检测的更新
  - [x] eget config doctor 输出本机配置、缓存、store、安装目录等诊断信息

## [x] registry 功能

之前 eget 通过 template 支持了内部工具分发问题，现在使用中发现 内部工具不少时，但是配置格式都差不多，每次都是名称调整一下，
但是都得手动复制一份修改下名称才行，内部工具较多时不方便添加使用。

思考是否可以新增 `[registry]` 板块配置有很多可用工具的站点(内部或公开的) 

```toml
[registry.mydev]
latest_template = "http://mydev.lan/tools/{name}/latest.yaml"
pkg_template = "http://mydev.lan/tools/{name}/{name}-{os}-{arch}{ext}" 
```

使用
- eget add mydev:markview
- eget ins mydev:markview  # 直接安装
- eget ins --add mydev:markview

## search 结果展示 ✅

```txt
<info>owner/repo</> ⭐{stars} language: {language} update: {update_time}
{description}
---
```

## 新增 global.group_packages 用于配置 package 分组 ⏳

配置新增 `global.group_packages` 用于配置 package 分组, 可以配置多个分组,在需要恢复时指定分组快速安装。
例如:

- `required` 分组用于指定必须安装的 package names
- `optional` 分组用于指定可选安装的 package names
- `dev` 分组用于指定开发环境的 package names

通过 `eget install --group <group-names>` 选项可以安装指定分组的 packages. 可以多个分组名称, 用逗号分隔.
例如：`eget install --group required,dev` 需要安装 `required` 和 `dev` 分组的 packages.

config example:

```toml
[group_packages]
required = ["required1", "required2"]
optional = ["optional1", "optional2"]
dev = ["dev1", "dev2"]
```

## 新增支持 sdk 下载安装，需要支持多版本 ✅

已新增 `eget sdk` 命令，首版支持 SDK 多版本下载、HTML/JSON index 缓存、断点续传、解压安装和独立 `sdk.installed.json`。

已支持：

- `sdk install <target...>`：串行安装一个或多个 SDK target。
- `sdk list [name]` / `sdk list --json`：查看 SDK 安装记录。
- `sdk remove <name@version>`：按安装记录和路径安全校验删除指定版本。
- `sdk index list/show/refresh/clear`：管理解析后的索引缓存 JSON。
- 目标格式：`go`、`go@latest`、`go:latest`、`go@1.22`、`go:1.22`、`go@1.22.0`、`go:1.22.0`。
- 不支持 `go 1.22.0` 这种空格版本格式，为后续支持多个 SDK 同时下载保留参数位置。
- 首版默认示例覆盖 Go 和 Node。其它 SDK 只要能用配置描述归档文件名和索引，也可以使用同一套能力。
- `eget sdk` 只负责下载和安装，不负责 `use/env/PATH/shell hook`，环境切换交给 `kite xenv` 等专用工具。

配置示例：

```toml
[global]
sdk_target = "~/.local/sdks"
sdk_ext_map = { windows = "zip", linux = "tar.gz", darwin = "tar.gz" }

[sdk.go]
aliases = ["golang"]
# 如果是相对路径，则是基于 global.sdk_target 目录
target = "gosdk/go{version}"
url_template = "https://mirrors.aliyun.com/golang/go{version}.{os}-{arch}.{ext}"
index_url = "https://mirrors.aliyun.com/golang/"
index_format = "html"
filename_pattern = "go{version}.{os}-{arch}.{ext}"
strip_components = 1
ext_map = { windows = "zip", linux = "tar.gz", darwin = "tar.gz" }

[sdk.node]
aliases = ["nodejs"]
target = "nodejs/node{version}"
url_template = "https://mirrors.aliyun.com/nodejs-release/v{version}/node-v{version}-{os}-{arch}.{ext}"
# https://nodejs.org/dist/
# mirror2: https://registry.npmmirror.com/binary.html?path=node/
index_url = "https://mirrors.aliyun.com/nodejs-release/"
index_format = "html"
index_path_prefix = "/nodejs-release/"
filename_pattern = "node-v{version}-{os}-{arch}.{ext}"
strip_components = 1
os_map = { windows = "win", linux = "linux", darwin = "darwin" }
arch_map = { amd64 = "x64", arm64 = "arm64", 386 = "x86" }
ext_map = { windows = "zip", linux = "tar.xz", darwin = "tar.gz" }
```

url template 中的占位符有
- `{name}`
- `{version}`
- `{os}`
- `{arch}`
- `{ext}`

target 目标目录结构示例：

```txt
{sdk_target}/
|- gosdk/go{version}
|- nodejs/node{version}
```

后续可增强：

- [ ] 按稳定后的 `internal/sdk` 模型新增 `pkg/sdk` 导出包，便于 `kite xenv` 等外部库复用下载、索引和安装能力。
- [ ] 增加 checksum 校验。
- [ ] 扩展更多内置 JSON index parser 或 SDK provider 示例。
- [ ] 视实际使用情况为 `sdk.installed.json` 增加文件锁。
