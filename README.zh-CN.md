# Eget

![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/inherelab/eget?style=flat-square)
[![GitHub tag (latest SemVer)](https://img.shields.io/github/tag/inherelab/eget)](https://github.com/inherelab/eget)
[![Unit-Tests](https://github.com/inherelab/eget/actions/workflows/go.yml/badge.svg)](https://github.com/inherelab/eget)

---

[English](./README.md) | [简体中文](./README.zh-CN.md)

`eget` 用于从 GitHub、GitLab、Gitea/Forgejo 和 SourceForge 查找、下载并提取预构建二进制。

> Forked from https://github.com/zyedidia/eget 重构并增强了工具的功能。

## 功能特性

- 多来源安装：支持从 GitHub、GitLab、Gitea/Forgejo、SourceForge、直接下载 URL 和本地文件安装或下载二进制资源。
- 自动选择与提取：按系统架构、资源关键词或正则筛选 release asset，支持 SHA-256 校验和常见归档格式提取。
- 托管包管理：通过 `add`、`list`、`update`、`uninstall` 管理常用工具，记录安装状态并支持批量检查更新。
- SDK 下载：支持 Go、Node 等多版本 SDK 归档下载，带索引缓存、断点续传和独立 SDK 安装记录。
- 并发下载：大文件会自动使用 HTTP Range 分片并发下载，安装或更新全部包时会自动批量并发下载。
- 查询与搜索：支持查询 GitHub release 信息、SourceForge 最新版本和 assets，并使用 GitHub 搜索语法查找仓库。
- 缓存与代理：支持下载缓存、API 响应缓存、`[http_proxy]` 和 `ghproxy`，适合网络受限或重复安装场景。
- 配置化使用：支持全局配置、仓库配置和 `packages.<name>` 托管包配置，配置文件和 installed store 默认位于 `~/.config/eget/`。

## 安装

- 使用 release 安装脚本

```bash
curl -fsSL https://raw.githubusercontent.com/inherelab/eget/master/.github/install.sh | sh
```

```powershell
iwr https://raw.githubusercontent.com/inherelab/eget/master/.github/install.ps1 -UseB | iex
```

- 从 Releases 下载安装 [https://github.com/inherelab/eget/releases](https://github.com/inherelab/eget/releases)
- 使用命令 `go install` 安装 (需要本地有 Go sdk)

```bash
go install github.com/inherelab/eget/cmd/eget@latest
```

## 命令风格

```bash
eget <command> --options... arguments...
```

## 使用示例

### 安装命令示例

**从 GitHub 安装**:

```bash
# quickly
eget i ORG/REPO
# install
eget install --tag nightly inhere/markview
# 安装并指定可执行文件名
eget install --name chlog gookit/gitw
# 安装 zip 资产
eget install --asset zip windirstat/windirstat
# 使用正则筛选资源
eget install --asset "REG:\\.deb$" owner/repo
# 使用前缀/后缀筛选资源
eget install --asset "PRE:codex,SUF:.zip" openai/codex
# 解压全部文件，并重命名其中一个解压文件
eget install --extract-all --rename "codex-x86_64-pc-windows-msvc.exe=codex.exe" openai/codex
# 解压全部文件，并剥离归档顶层目录
eget download --extract-all --strip-components 1 --asset "windows,zip" ventoy/Ventoy
# 安装到指定目录
eget install --to ~/.local/bin/fzf junegunn/fzf
```

**安装 SourceForge 项目**:

```bash
# 直接安装 SourceForge 项目
eget install --asset x64,PerUser,setup sourceforge:winmerge
```

**安装 GitLab/Gitea/Forgejo 项目**:

```bash
# 从 GitLab releases 安装
eget install gitlab:fdroid/fdroidserver
eget install gitlab:gitlab.gnome.org/GNOME/gtk
# 从 Gitea/Forgejo-compatible releases 安装
eget install --asset linux,amd64 gitea:codeberg.org/forgejo/forgejo
```

**安装并记录**:

```bash
# 安装 并 记录
eget install --add junegunn/fzf
eget install --add --name rg BurntSushi/ripgrep
# 添加 SourceForge 项目为托管包
eget add --name winmerge --system windows/amd64 --asset x64,PerUser,setup sourceforge:winmerge
# 安装 [packages] 下配置的全部托管包
eget install --all
```

**安装 GUI 应用**:

```bash
# 安装 GUI 应用；免安装 GUI 程序默认写入 global.gui_target
eget install --gui sipeed/picoclaw
# 强制将普通命名的 .exe GUI asset 当作安装器启动
eget ins --gui --install-mode installer owner/repo
eget add --gui --name picoclaw sipeed/picoclaw
```

### 下载命令示例

```bash
# download
eget download ip7z/7zip
eget download --file go --to ~/go1.17.5 https://go.dev/dl/go1.17.5.linux-amd64.tar.gz
eget download --file README.md,LICENSE --to ./dist owner/repo
eget download --file "*.txt" owner/repo
eget download --file "bin/*" owner/repo
eget download --file "*.exe,^*x86*,^*.sig" owner/repo
eget download --extract-all --to ./dist windirstat/windirstat
```

### SDK 命令示例

```bash
eget sdk install go@1.22.0
eget sdk install go:1.22 node:20.11.1
eget sdk install --force go@1.22.0
eget sdk list
eget sdk list --json
eget sdk remove go@1.22.0
eget sdk path go
eget sdk path go:1.20
eget sdk path java:17
eget sdk config add --all
eget sdk config add --all --mirror mirror
eget sdk config add jdk --mirror zulu
eget sdk index refresh go
eget sdk index show go
```

> `eget sdk` 只负责下载和解压安装 SDK，不会修改 `PATH`、不会写 shell hook、不会管理当前激活版本，也不会写 `.xenv.toml`。如需环境切换，可在安装后配合 `xenv tools index` 和 `xenv use`。

### 缓存管理

`eget cache` 用于管理本机缓存目录。顶层命令也支持短别名：

```bash
eget ca status
eget ca list --root sdk
```

子命令：

- `cache clean`: 清理选定的缓存文件。
- `cache list`: 按缓存根目录列出文件，支持 `all`、`pkg`、`api`、`sdk`、`sdk-index`、`partial`。
- `cache status`: 查看缓存目录占用摘要。
- `cache serve`: 以只读 HTTP 服务暴露缓存文件。

`eget cache clean` 用于清理 `global.cache_dir` 下的本机缓存。默认清理 3 天前的 package 下载缓存、API cache、SDK 下载缓存和未完成下载状态。SDK index 默认保留；如果确认要清理 SDK index，请显式使用 `--sdk-index`。

```bash
eget cache clean
eget cache clean --dry-run --older 7d
eget cache clean --dry-run --json
eget cache clean --api --all
eget ca clean --sdk --partial --older 12h
```

查看缓存内容和状态：

```bash
eget cache status
eget cache list --root sdk
eget cache list --root sdk-index
eget cache list --json
```

`eget cache serve` 会启动只读 HTTP 服务，方便同一局域网内其它机器浏览或下载本机已有的 package/SDK 缓存文件。浏览器打开服务根路径可以查看内置文件列表界面，也可以请求 `/manifest.json` 获取机器可读的 manifest。

```bash
eget cache serve
eget cache serve --host 127.0.0.1 --port 0 --root sdk --no-index
```

服务端默认输出 text 请求日志。需要时可以开启简单 bearer 保护，并把请求日志切换为 JSON lines：

```bash
eget cache serve --host 0.0.0.0 --port 8686 --token "$EGET_CACHE_TOKEN" --json-log
```

客户端开启回源前优先尝试 cache server：

```toml
[cache_mirror]
enable = true
url = "http://192.168.1.10:8686"
```

### 查询命令示例

**查询仓库信息**:

```bash
# query repo info
eget query owner/repo
eget query --action releases --limit 5 owner/repo
eget query --action assets --tag v1.2.3 owner/repo

# 查询 SourceForge 最新版本或 assets
eget query sourceforge:winmerge
eget query https://sourceforge.net/projects/victoria-ssd-hdd
eget query --action assets sourceforge:winmerge/stable
eget query --action assets --tag 2.16.44 sourceforge:winmerge/stable
```

**搜索 GitHub 仓库**:

```bash
eget search ripgrep
eget search skillc language:go user:inhere
eget search --limit 5 --sort stars --order desc terminal ui
eget search --json picoclaw user:sipeed
```

### 其他命令示例

```bash
# uninstall
eget uninstall fzf
# 列出已安装包
eget list|ls
# 列出全部托管包和已安装包
eget list --all
# 列出 config 中配置但未安装的包
eget list --no-installed
# 显示 package 详情
eget show fzf
# 只列出 GUI 包
eget list --gui
# update fzf
eget update fzf
eget update --all
# 更新 eget 自身
eget update --self
# 仅检查 eget 自身是否有新版本
eget update --self --check
```

### 配置命令示例

```bash
# config
eget add --name fzf --to ~/.local/bin junegunn/fzf
eget config init
eget config list|ls
eget config doctor
eget cfg path --check cache_dir
eget config get global.target
eget config set global.target ~/.local/bin
```

### 支持的目标

`install` 和 `download` 的目标参数可以是：

-  `name` config 里配置的包名称
- GitHub 仓库，例如 `owner/repo`
- GitHub 仓库 URL，例如 `https://github.com/owner/repo`
- GitLab 目标，例如 `gitlab:fdroid/fdroidserver` 或 `gitlab:gitlab.gnome.org/GNOME/gtk`
- Gitea/Forgejo 目标，例如 `gitea:codeberg.org/forgejo/forgejo`
- SourceForge 目标，例如 `sourceforge:winmerge`、`sourceforge:winmerge/stable` 或 `https://sourceforge.net/projects/winmerge`
- 直接下载 URL，例如 `https://example.com/file.tar.gz`
- 本地文件路径，例如 `file:///path/to/file`

> 注意：GitLab 和 Gitea/Forgejo 目前支持通过 release assets 进行 `install`、`download` 和 `update`。SourceForge 还支持 `query latest` 和 `query assets`。暂不支持 search 对齐和私有仓库认证。

## 当前命令

`install`(alias: `i`, `ins`)

- 查找、下载、校验、提取目标，并记录安装状态。
- 可通过 `--name` 指定安装后的可执行文件名；未指定 `--to` 时，也会作为单文件资产的重命名提示。
- `--gui` 会将目标标记为 GUI 应用。GUI `.exe` 和 `.msi` 默认按安装器模式启动；如果最终选中的 asset 或归档内文件名包含 `portable`，则自动按免安装 GUI 处理，默认使用 `global.gui_target`。未传 `--gui` 但选中疑似安装器资源时，会先提示是否启动；确认后若同时传入 `--add`，会持久化 `is_gui = true`。可传 `--install-mode portable|installer`，或在 package 配置里设置 `install_mode` 覆盖自动判断。
- 传入 `--add` 时，安装成功后会自动将 repo 目标写入 `[packages.<name>]`；可配合 `--name` 指定包名。
- 不带目标传入 `--all` 时，会安装 `[packages]` 下配置的全部托管包；每个包仍按单包安装一样合并包级选项。

`download`(alias: `dl`)

- 复用安装链路，但不记录 installed store。
- 默认仅下载原始 asset；只有设置 `--file` 或 `--extract-all` 时才会自动解压归档内容。

`add`

- 将一个托管包写入配置文件的 `[packages.<name>]`。未手动设置 `desc` 时，eget 会尝试写入 repository 描述。

`uninstall`(alias: `uni`, `rm`)

- 删除已安装文件并清理 installed store 记录，不移除 `[packages.<name>]` 配置。
- 使用 `--purge` 可同时删除匹配的 `[packages.<name>]` 配置；不会清理 package 下载缓存。

`list`(alias: `ls`)

- 默认列出已安装包。
- 使用 `--all` / `-a` 列出本地 managed packages 与 installed store 的并集。
- 使用 `--no-installed` / `--ni` 列出 `[packages]` 中已配置但未安装的包。
- 使用 `--gui` 只显示当前列表视图中的 GUI 应用。

`show`

- 合并 `[packages.<name>]` 与 installed store 信息，显示 package 详情，包括描述、版本、状态、主页、repository URL、选中的 asset、asset URL、安装目标和已提取文件。
- `packages.<name>.desc` 可手动设置；为空时，`add` / `install --add` 会尝试获取 repository 描述，installed 记录也会在可用时保存描述、主页和 repository URL。

`query`(alias: `q`)

- 查询 GitHub repo 的 release 与元数据，以及 SourceForge 的最新版本和 assets，不涉及安装或本地状态写入。
- 默认 action 为 `latest`，可通过 `--action` 切换为 `info`、`releases`、`assets`。

`search`

- 搜索 GitHub 仓库，不涉及安装或本地状态写入。
- 第一个参数作为搜索关键词，后续参数会原样作为 GitHub 搜索限定条件传递，例如 `language:go`、`user:inhere` 或 `topic:cli`。

`update`(alias: `up`)

- 先检查目标是否有新版本，再更新已配置或已安装的目标；也可通过 `--all` 更新全部托管包。
- `update --self` 会检查 `inherelab/eget` release，选择当前 OS/arch 对应的原始可执行文件 asset，并替换当前正在运行的 eget 可执行文件。Windows 下替换会延迟到当前进程退出后执行。
- `update --self --check` 会在请求最新版本前输出自更新检查来源 host，方便确认使用的是 GitHub 还是私有源。
- `update --self --self-source <url>` 会从内部源更新 eget。内部源需要提供 `latest.yaml`，以及同目录下的原始平台文件，例如 `eget-linux-amd64` 和 `eget-windows-amd64.exe`。`<url>` 可以是目录地址，也可以直接是 `latest.yaml` 地址；也可通过 `EGET_SELF_UPDATE_SOURCE` 设置默认内部源。

`sdk`

- 下载并安装多版本 SDK 到 `global.sdk_target`，支持 `.part` 断点续传，并用独立的 `sdk.installed.json` 记录安装状态。
- 支持的安装目标格式为 `name`、`name@latest`、`name:latest`、`name@1.22`、`name:1.22`、`name@1.22.0`、`name:1.22.0`。刻意不支持 `go 1.22.0` 这种空格版本格式。
- `sdk install` 支持一次传入多个 SDK target，首版按顺序串行安装。
- `sdk list` 读取 SDK 安装记录，可加 `--json` 输出 JSON。
- `sdk remove <name@version>` 只删除 SDK 安装记录中存在、且通过 SDK 根目录安全校验的目录。
- `sdk path <target>` 输出 SDK 路径；`go`、`java` 等不带版本目标输出配置的 SDK 基础目录，`go:1.20`、`go@1.21.5`、`java:17` 等带版本目标输出已安装版本目录。
- `sdk index list/show/refresh/clear` 管理规范化后的 SDK 索引缓存 JSON。
- 首版内置示例覆盖 Go 和 Node。其它 SDK 只要归档命名能用 `url_template`、`filename_pattern`、`os_map`、`arch_map`、`ext_map` 和可选 HTML/JSON index 描述，也可以通过配置使用。

`config`(alias: `cfg`)

- 支持 `init`、`list` / `ls`、`doctor`、`path [--check] [target]`、`get KEY`、`set KEY VALUE`。
- `config path` 用于输出单个本地路径，方便脚本使用。支持的 target 有 `config_dir`、`config_file`（默认）、`env_file`、`bin_dir`、`cache_dir`、`sdk_dir`、`pkg_store_file`、`sdk_store_file`。设置 `--check` 后输出格式为 `path, exists: bool`。
- `config doctor` 会输出本机 config/cache/store/install 相关路径、存在性、可写性，以及敏感配置是否已设置；不会打印 secret 原文。
- 读取 `eget.toml` 前会加载可选的 `~/.config/eget/.env`，因此配置值可以继续写成 `github_token = "${GITHUB_TOKEN}"`、`[http_proxy].url = "${PROXY_URL}"`，敏感信息无需直接写进配置文件。

## 主要选项

`install`、`download`、`add` 共享这些安装相关选项：

- `--tag`: 指定发布版本标签；未提供时默认使用 `latest`。
- `--system`: 指定目标系统与架构，例如 `windows/amd64`、`linux/arm64`。
- `--to`: 指定安装或下载输出路径；可传目录，也可传完整文件路径。
- `--file`: 指定归档内要提取的文件；支持逗号分隔多个文件或 glob 模式，例如 `README.md,LICENSE`。排除可用 `^`，例如 `*.exe,^*x86*,^*.sig`；只有排除项时（如 `^*.sig`）表示匹配除被排除条目外的全部文件。对 7z 可读取的 `.exe` 安装包使用时，需要系统 7z。
- `--asset`: 指定资源过滤关键词；可用逗号分隔多个过滤条件。支持 `REG:` 前缀正则，例如 `REG:\\.deb$`。支持 `PRE:` 和 `SUF:` 前缀/后缀匹配，例如 `PRE:codex` 或 `SUF:.zip`。排除可用 `^`，例如 `^REG:...` 或 `^SUF:.sha256`。过滤条件可用 Go OS 前缀限定目标系统，例如 `windows:zip`、`linux:tar.gz`、`darwin:SUF:.zip`；仅当当前 `--system` 的 OS 匹配时生效。
- `--rename`: 使用逗号分隔的 `from=to` 映射重命名解压文件，例如 `--rename "tool-windows-amd64.exe=tool.exe"`。可配合 `--file` 和 `--extract-all` 使用，并会被 `install --add` 持久化。
- `--source`: 下载源码归档而不是预构建二进制。
- `--extract-all`, `--ea`: 提取归档中的全部文件，而不是只选择一个目标文件。
- `--strip-components N`: 解压全部文件时剥离归档内路径前 `N` 层，适用于归档内容被版本号顶层目录包裹的场景。
- `--chunk N`: 控制单个下载文件的 HTTP Range 分片并发。`0` 表示自动，`1` 表示单连接下载，大于 `1` 表示最多使用该数量的分片。
- `--quiet`: 精简常规输出；存在多个资源候选时自动选择第一项，适用于脚本或批处理场景。

`install` 和 `download` 还支持 GitHub 目标的 `--prerelease` / `-p`。使用默认 latest 时，会把预发布版本也纳入最新版本选择。

`install` 和 `download` 还支持 SourceForge 目标的 `--fallback-versions N`。当最新版本目录没有匹配资产时，eget 会最多扫描 `N` 个更旧版本目录，并使用第一个能被当前 `--asset` / `--system` 过滤条件唯一匹配的文件。

`download` 还支持对完整 GitHub release asset URL 使用 `--ghproxy`。它只会用 `[ghproxy].host_url` / `fallbacks` 重写 `github.com` 下载 URL，不会查询或重写 GitHub API 请求。

> 缓存目录请通过 `config set global.cache_dir ...` 或配置文件中的 `cache_dir` 设置。

`install` 额外支持：

- `--add`: 安装成功后，将 repo 目标追加到 `[packages.<name>]` 托管配置中。
- `--all`: 安装 `[packages]` 下配置的全部托管包；不能同时传入目标或 `--add`。
- `--batch N`: 控制 `install --all` 的包任务并发。`0` 表示自动，`1` 表示串行，大于 `1` 表示最多同时处理该数量的包。
- `--gui`: 按 GUI 应用安装；配合 `--add` 时会持久化 `is_gui = true`。未传 `--gui` 但确认启动疑似安装器时，配合 `--add` 也会持久化 `is_gui = true`。
- `--install-mode portable|installer`: 覆盖本次 GUI 安装模式。默认情况下，`--gui` 会把最终选中的 `.exe` / `.msi` 当作安装器；文件名包含 `portable` 时自动切换为免安装模式。
- `--name`: 指定托管包名；对于单文件可执行资产，也会作为默认输出文件名提示。

`update` 支持选项：

- `--all`: 检查托管包，只更新已安装且有新版本的包。
- `--self`: 更新当前 `eget` 可执行文件，而不是普通托管包。
- 单目标 `update <target>` 要求目标已存在于 config 或 installed store；新目标请使用 `install`。
- `--batch N`: 控制 `update --all` 的包任务并发。`0` 表示自动，`1` 表示串行，大于 `1` 表示最多同时处理该数量的包。
- `--chunk N`: 控制 update 触发下载时的 HTTP Range 分片并发。
- `--check`: 检查并列出有新版本的已安装包，等同于 `list --outdated`。如果提供 target，例如 `eget up --check markview jq`，则只检查这些包。
- `--dry-run`: 仅预览更新计划，不执行实际安装。
- `--interactive`: 交互式选择要更新的托管包。

`query` 支持选项：

- `--action`, `-a`: 查询动作，支持 `latest`、`releases`、`assets`、`info`。
- `--tag`, `-t`: 为 `assets` 动作指定 release tag；不传时默认查询 latest。
- `--limit`, `-l`: 限制 `releases` 动作返回数量，默认 `10`。
- `--json`, `-j`: 使用 JSON 输出结果，方便脚本处理。
- `--prerelease`, `-p`: 在 `latest` / `releases` 中包含预发布版本。

SourceForge 查询目标使用 `sourceforge:<project>`、`sourceforge:<project>/<path>`、`sf:<project>` 或 `https://sourceforge.net/projects/winmerge` 这类 SourceForge 项目 URL。目前支持 `latest`、`releases` 和 `assets`；`info` 仍仅适用于 GitHub。

`search` 支持选项：

- `--limit`, `-l`: 限制返回的仓库数量，默认 `10`。
- `--sort`: 指定搜索结果排序字段，支持 `stars`、`updated`。
- `--order`: 指定排序方向，支持 `desc`、`asc`。
- `--json`, `-j`: 使用 JSON 输出结果，方便脚本处理。

`sdk` 支持选项：

- `sdk install --force`: 安全删除已有 SDK 目标目录后重新安装。
- `sdk list --json`: 用 JSON 输出 SDK 安装记录。
- `sdk index list --json`: 用 JSON 输出 SDK 索引缓存摘要。
- `sdk index refresh --all`: 刷新所有配置了 `index_url` 的 SDK 索引。
- `sdk index clear --all`: 删除全部 SDK 索引缓存 JSON。

全局选项：

- `-v`, `--verbose`: 输出更详细的调试信息，例如请求的 API、响应摘要、asset 选择、缓存命中和关键流程节点。

说明：

- `install --name` 可用于指定单文件可执行资产的输出文件名，例如将 `chlog-windows-amd64.exe` 安装为 `chlog.exe`。
- `install --rename` 可在解压多个文件时只重命名指定文件；配置字段为 `rename_files`，例如 `rename_files = { "codex-x86_64-pc-windows-msvc.exe" = "codex.exe" }`。
- `install --add` 仅对 repo 目标生效，并在安装成功后追加托管包配置。
- `global.gui_target` 只用于免安装 GUI 应用。`.msi`、`setup.exe` 等 GUI 安装器会被启动，但不会记录最终安装目录。对于文件名是产品/版本/平台格式而不是 setup 风格的 `.exe` 安装器，使用 `install_mode = "installer"`。
- `download` 默认保存原始下载文件；只有设置了 `--file` 或 `--extract-all` 才会自动提取归档内容。
- `sdk` 使用 `global.sdk_target` 作为安装根目录，使用 `{cache_dir}/sdk-downloads` 保存 SDK 归档下载缓存；断点续传状态由 `.part` 和 `.meta.json` 维护。
- 归档提取当前支持 `zip`、`tar.*` 以及 `7z`。当 `global.sys7z_path` 或 `PATH` 提供 `7z`、`7zz`、`7za` 时，`.7z`、`.rar`、`.msi`、`.cab`、`.iso` 以及 `--extract-all` 的 `.exe` 会优先使用系统 7z；`tar.*` 归档继续使用内置 Go 解压流程。
- 命令选项可以写在位置参数前后，例如 `eget install --tag nightly owner/repo` 和 `eget install owner/repo --tag nightly` 等价。

## 配置

配置文件位置按以下顺序解析：

1. `EGET_CONFIG`
2. `~/.config/eget/eget.toml`
3. XDG / LocalAppData fallback 路径
4. 旧路径 `~/.eget.toml`

配置同时支持：

- `[global]`
- `[http_proxy]`
- `["owner/repo"]`
- `[packages.<name>]`
- `[pkg_templates.<name>]`
- `[sdk.<name>]`

最小示例：

```toml
[global]
target = "~/.local/bin"
cache_dir = "~/.cache/eget"
user_agent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36"
sdk_target = "~/.local/sdks"

[http_proxy]
url = "http://127.0.0.1:7890"
exclude = []

[packages.markview]
repo = "inhere/markview"
tag = "nightly"
asset_filters = ["windows"]
rename_files = { "markview-windows-amd64.exe" = "markview.exe" }

[packages.markview_mirror]
repo = "template:markview"
latest_url = "https://example.com/tools/markview/latest.yaml"
latest_format = "yaml"
url_template = "https://example.com/tools/markview/markview-{version}-{os}-{arch}{ext}"
os_map = { windows = "windows", linux = "linux", darwin = "darwin" }
arch_map = { amd64 = "amd64", arm64 = "arm64" }
ext_map = { windows = ".zip", linux = ".tar.gz", darwin = ".tar.gz" }
extract_file = "markview"

[packages.claude]
repo = "template:claude"
latest_url = "https://downloads.claude.ai/claude-code-releases/latest"
latest_format = "text"
url_template = "https://downloads.claude.ai/claude-code-releases/{version}/{os}-{arch}{libc}/claude{ext}"
os_map = { windows = "win32", linux = "linux", darwin = "darwin" }
arch_map = { amd64 = "x64", arm64 = "arm64" }
ext_map = { windows = ".exe", linux = "", darwin = "" }
libc_map = { glibc = "", musl = "-musl" }
checksum_url_template = "https://downloads.claude.ai/claude-code-releases/{version}/manifest.json"
checksum_format = "json"
checksum_json_path = "platforms.{os}-{arch}{libc}.checksum"
install_action = "run-asset"
install_args = ["install", "latest"]

[pkg_templates.mydev]
latest_url = "http://mydev.lan/tools/{name}/latest.yaml"
url_template = "http://mydev.lan/tools/{name}/{name}-{os}-{arch}{ext}"

[packages.markview_internal]
repo = "pkg-template:mydev:markview"

[sdk.go]
aliases = ["golang"]
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
index_url = "https://mirrors.aliyun.com/nodejs-release/"
index_format = "html"
index_path_prefix = "/nodejs-release/"
filename_pattern = "node-v{version}-{os}-{arch}.{ext}"
strip_components = 1
os_map = { windows = "win", linux = "linux", darwin = "darwin" }
arch_map = { amd64 = "x64", arm64 = "arm64", 386 = "x86" }
ext_map = { windows = "zip", linux = "tar.xz", darwin = "tar.gz" }
```

也可以通过内置模板快速写入 SDK 配置：

```bash
eget sdk config add --all
eget sdk config add --all --mirror mirror
eget sdk config add jdk --mirror zulu
```

创建默认配置：

```bash
eget config init
```

默认会写入 `~/.config/eget/eget.toml`。

`template:<id>` 可用于独立下载站 package，例如 Claude Code 这种通过 latest metadata 和 URL 模板发布的工具。`run-asset` 只执行已下载并通过 checksum 校验的 asset，不是通用 `post_install`。如果多个内部工具使用同一发布规则，可以用 `pkg_templates` 复用配置，并直接运行 `eget add mydev:markview` 或 `eget install mydev:markview`。完整配置说明见 [docs/config.zh-CN.md](docs/config.zh-CN.md)，包括全局字段、package 配置、Template Package Source、pkg_templates、SDK 配置、缓存目录、安装记录文件、ghproxy、API cache 和 SDK index 设置。SDK 专题使用说明见 [docs/sdk-usage.md](docs/sdk-usage.md)。

## 构建与测试

```bash
make build
make test
```

## 开发结构

当前版本已经重构为显式子命令 CLI，入口在 `cmd/eget/main.go`，业务逻辑集中在 `internal/`。

- `cmd/eget`: 命令入口
- `internal/cli`: `gcli` 命令注册与参数绑定
- `internal/app`: install/add/list/update/config 用例编排
- `internal/install`: 查找、下载、校验、提取执行链路
- `internal/config`: 配置加载、合并、写回
- `internal/installed`: 安装记录存储
- `internal/sdk`: SDK target 解析、索引缓存、断点续传、解压安装和 SDK 安装记录
- `internal/source/github`: GitHub 资源查找
- `internal/source/forge`: GitLab/Gitea/Forgejo 资源查找

> 更详细说明见 [docs/architecture.md](docs/architecture.md)。

## 参考项目

- [https://github.com/zyedidia/eget](https://github.com/zyedidia/eget)
- [https://github.com/gmatheu/eget](https://github.com/gmatheu/eget)
