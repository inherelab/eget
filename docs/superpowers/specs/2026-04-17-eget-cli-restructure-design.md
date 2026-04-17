# Eget CLI 重构设计

## 背景

当前 `eget` 仍采用单入口、根目录平铺 Go 文件的结构。CLI 基于 `go-flags` 直接解析，配置、安装、下载、升级、交互选择等职责散落在多个文件中，已经不适合继续扩展子命令和配置管理能力。

这次重构的目标不是改名，也不是改变 `eget` 的产品定位，而是完成一次结构性收敛：

- 代码收敛到 `internal/`
- 入口迁移到 `cmd/eget/main.go`
- 使用 `gookit/cflag/capp` 重建 CLI 结构
- 在不破坏现有默认安装体验的前提下，支持 `add`、`install`、`update`、`config`、`download` 子命令

## 目标

- 建立清晰、可维护的 CLI 架构，减少根目录平铺逻辑
- 保持现有默认用法兼容：`eget owner/repo [flags...]`
- 新增受管理包模型，支持 `add` 和 `update --all`
- 继续复用现有配置文件 `~/.eget.toml`
- 将“声明式配置”和“安装状态记录”分离
- 为后续继续扩展子命令或支持更多 source 留出边界

## 非目标

- 本轮不改变项目名，继续使用 `eget`
- 本轮不改变项目的核心定位，仍以 GitHub release 下载/安装为主
- 本轮不做强制配置迁移脚本
- 本轮不把产品完全重构成完整包管理器
- 本轮不主动扩展到 GitHub 之外的新 source

## 方案选择

评估了三种方案：

### 方案一：兼容包裹式迁移

仅新增 `cmd/eget/main.go` 和 `capp` 命令壳子，内部继续大量复用旧逻辑。

优点：

- 变更快，风险低

缺点：

- 只是“搬家”，不能真正解决职责混杂问题
- 后续继续扩展时仍会再次重构

### 方案二：分层重构

按 CLI、应用用例、配置、安装、source、安装状态等职责拆分为 `internal/` 下多个包，并以 `capp` 注册子命令。

优点：

- 本次即可把结构收敛到位
- 兼顾现有兼容性和后续扩展性

缺点：

- 改动面较大，需要分阶段实施和回归测试

### 方案三：包管理器化重建

以“受管理包”为中心重写命令模型，兼容旧安装模式。

优点：

- 产品模型最统一

缺点：

- 范围超出本次目标，容易失控

### 结论

采用方案二：分层重构。

## 目录结构设计

重构后的目录结构如下：

```txt
cmd/
  eget/
    main.go

internal/
  cli/
    app.go
    root.go
    install_cmd.go
    add_cmd.go
    update_cmd.go
    download_cmd.go
    config_cmd.go
  app/
    install.go
    download.go
    add.go
    update.go
    config.go
  config/
    model.go
    loader.go
    writer.go
    paths.go
    compat.go
  install/
    finder.go
    detector.go
    downloader.go
    verifier.go
    extractor.go
    installer.go
  source/
    github/
      finder.go
      release.go
  installed/
    model.go
    store.go
    upgrade.go
  ui/
    select.go
    upgrade_view.go
```

说明：

- `cmd/eget/main.go` 只负责启动 CLI，不承载业务逻辑
- `internal/cli` 负责 `capp` 应用构造、命令注册、参数绑定、默认命令兼容
- `internal/app` 是用例编排层，负责把配置、下载、安装、记录串联起来
- `internal/config` 负责配置文件定位、加载、兼容、写回
- `internal/install` 负责资源查找、下载、校验、解压、写盘
- `internal/source/github` 隔离 GitHub source 相关逻辑
- `internal/installed` 负责已安装记录和升级分析
- `internal/ui` 保留交互选择和升级结果展示

最终要求是业务代码收敛到 `internal/`，根目录不再保留平铺的业务实现文件。

## CLI 命令模型

### 根命令兼容策略

重构后保留旧用法：

```bash
eget owner/repo [flags...]
```

该行为等价于：

```bash
eget install owner/repo [flags...]
```

路由规则：

1. 如果第一个非 flag 参数是已知子命令名，则进入对应子命令
2. 否则按默认 `install` 处理

这样可以同时兼容：

- `eget inhere/markview --tag nightly`
- `eget install inhere/markview --tag nightly`
- `eget --system linux/amd64 sharkdp/fd`

且不会与下面的子命令冲突：

- `eget add ...`
- `eget update ...`
- `eget config ...`
- `eget download ...`

### 子命令列表

#### `install`

显式安装命令。负责查找 release、下载、校验、解压、写入目标位置，并记录安装结果。

支持 repo、GitHub URL、直链 URL、本地文件。

#### `download`

只下载原始文件，不解压、不记录 installed 状态。

支持：

- repo
- GitHub URL
- 直链 URL
- 本地文件

它比 `install` 更底层，负责“获取原始包”，不负责安装。

#### `add`

将受管理包条目写入配置文件，不执行下载。

行为：

- 输入 repo
- 支持 `--name`
- 未提供 `--name` 时，默认使用 repo 名作为包名
- 将可复用安装参数写入 `[packages.<name>]`

#### `update`

更新受管理条目。

支持两种模式：

```bash
eget update <name|repo>
eget update --all
```

行为：

- `update <name|repo>`：优先按受管理包名称解析，也允许直接传 repo
- `update --all`：更新全部已登记条目

#### `config`

配置相关操作，首版支持：

- `eget config --info`
- `eget config --init`
- `eget config --list`
- `eget config get <key>`
- `eget config set <key> <value>`

不在本轮实现更复杂的 config 子命令树。

### 参数继承策略

参数分三层：

1. 根级公共参数
   例如 `--config` 等运行时参数
2. 安装类共享参数
   例如 `--tag`、`--system`、`--file`、`--asset`、`--verify-sha256`、`--to`、`--source`
3. 子命令专属参数
   - `add`: `--name`
   - `update`: `--all`、`--interactive`、`--dry-run`
   - `config`: `--info`、`--init`、`--list`

约束：

- `install` 继续承接大部分旧参数
- `download` 复用查找/下载相关参数，但不做安装后动作
- `add` 只接收声明式可复用安装参数
- `update` 只接收更新流程需要的参数

## 配置模型设计

### 配置文件位置

继续复用现有 `~/.eget.toml` 及现有备用查找逻辑。

### 配置结构

采用三层结构：

```toml
[global]
target = "~/.local/bin"
system = "linux/amd64"
quiet = false
github_token = ""

["junegunn/fzf"]
target = "~/.local/bin"
asset_filters = ["linux_amd64"]
file = "fzf"

[packages.fzf]
repo = "junegunn/fzf"
target = "~/.local/bin"
system = "linux/amd64"
file = "fzf"
asset_filters = ["linux_amd64"]
tag = ""
verify_sha256 = ""
download_source = false
disable_ssl = false
```

含义如下：

- `global`
  全局默认安装配置
- `["owner/repo"]`
  旧有 repo 级配置，继续兼容保留
- `[packages.<name>]`
  新的受管理包条目，由 `add` 和 `update` 使用

### 为什么采用 `[packages.<name>]`

不采用 `[[packages]]`，原因如下：

- `update fzf` 可以直接按键名解析
- `config get packages.fzf.target` 更直观
- 条目覆盖和删除更直接
- 实现复杂度更低

### 受管理包条目字段

首版由 `add` 写入的字段为声明式安装参数，推荐包含：

- `repo`
- `target`
- `system`
- `file`
- `asset_filters`
- `tag`
- `verify_sha256`
- `download_source`
- `disable_ssl`
- `all`

首版不将以下运行态或弱相关字段作为包定义核心字段：

- `quiet`
- `download_only`

### 配置优先级

统一采用如下优先级：

1. CLI 显式参数
2. `packages.<name>`
3. `["owner/repo"]`
4. `global`
5. 程序默认值

用途说明：

- `update fzf` 时，先读 `packages.fzf`
- 再根据 `repo` 补充 repo 级配置
- 最后以 `global` 和默认值补齐

### 配置迁移策略

不做强制迁移，采用兼容读取、按需写入：

- 没有 `[packages]` 的旧配置继续可用
- 只有执行 `add` 时才会新增 `[packages.<name>]`
- `config --init` 生成新格式模板，但不破坏已有配置
- `config --list` 同时展示旧 repo 配置和新 packages

## 安装状态模型

安装状态记录与配置分离，继续保留独立状态文件思路。

原则：

- 配置文件表示“希望如何安装”
- installed 状态表示“过去安装了什么”

现有 installed 记录逻辑保留，但代码迁入 `internal/installed/`。

## 分层职责

### `internal/cli`

负责：

- 构造 `capp.App`
- 注册命令
- 绑定命令参数
- 识别默认 `install` 路由
- 统一帮助与错误出口

不负责：

- GitHub API 调用
- 文件下载或解压
- 配置写盘细节

### `internal/app`

负责应用用例：

- `InstallTarget`
- `DownloadTarget`
- `AddPackage`
- `UpdatePackage`
- `UpdateAllPackages`
- `ConfigInfo`
- `ConfigInit`
- `ConfigList`
- `ConfigGet`
- `ConfigSet`

这是 CLI 和底层能力之间的稳定边界。

### `internal/config`

负责：

- 配置文件路径解析
- 加载与写入
- 旧配置兼容
- 新 package 配置模型定义
- 配置优先级合并

### `internal/install`

负责：

- finder 选择
- detector 选择
- downloader
- verifier
- extractor
- 最终安装落盘

### `internal/source/github`

负责：

- GitHub release/source 查询
- 相关 API 解析

### `internal/installed`

负责：

- 安装状态读写
- 升级候选构造
- 升级检查与结果整理

### `internal/ui`

负责：

- 候选资源交互选择
- 升级结果展示
- 终端交互能力隔离

## 迁移实施策略

采用分阶段、每一步可运行的迁移方式：

### 第一步：建立新入口和 CLI 骨架

- 新增 `cmd/eget/main.go`
- 新增 `internal/cli`
- 用 `capp` 搭建 `install`、`download`、`add`、`update`、`config` 命令壳子
- 暂时允许命令层调用旧逻辑适配层

目标是先跑通新入口和命令分发。

### 第二步：提取底层能力

将现有能力迁出根目录：

- flags/config 迁入 `internal/config`
- finder/detector/extractor/verifier/download 迁入 `internal/install`
- GitHub 查找能力迁入 `internal/source/github`

目标是让核心逻辑摆脱根目录平铺结构。

### 第三步：建立 app 用例层

新增 `internal/app`，统一承载命令级行为编排：

- 安装
- 下载
- 添加受管理包
- 更新单个包
- 更新全部包
- config 相关操作

### 第四步：引入 packages 模型

- 增加 `[packages.<name>]` 读写
- `add` 写 package 条目
- `update --all` 遍历 package 条目
- `config` 读写 package 配置

### 第五步：移除旧入口耦合

- 删除或清空根目录历史业务入口
- 保证业务代码仅存在于 `internal/`
- 保证最终只保留新的 CLI 架构

## 兼容性要求

本轮必须保证：

- `eget owner/repo [flags...]` 继续可用
- 旧配置 `~/.eget.toml` 继续可读
- 旧 repo section 继续生效
- 旧 installed 状态记录继续可读写
- GitHub 下载/检测/解压行为首版尽量不改语义

本轮允许变化：

- 帮助输出会因 `capp` 接管而变化
- 历史 flag 型入口如 `--list-installed`、`--upgrade-all` 可逐步迁移为命令能力，不再作为主设计中心

## 测试策略

测试分三层：

### 单元测试

覆盖纯逻辑模块：

- 配置路径解析
- 配置优先级合并
- repo/name 解析
- 默认命令路由判断
- update 目标解析

### 组件测试

覆盖 app 用例层：

- `add` 写入 `[packages.<name>]`
- `config --info`
- `config --init`
- `config --list`
- `config get`
- `config set`
- `update <name>`
- `update --all`

### 集成测试

保留并迁移现有真实下载测试，重点覆盖：

- 默认 install 兼容入口
- 显式 `install`
- `download`
- `add + update` 最小链路
- GitHub release 下载安装核心路径

策略上保留少量真实网络测试，同时增加更多本地可控测试，避免把全部回归建立在网络稳定性之上。

## 验收标准

本次重构完成时，应满足：

- 新入口位于 `cmd/eget/main.go`
- 业务代码收敛到 `internal/`
- 根命令兼容旧 `install` 用法
- `install`、`add`、`update`、`config`、`download` 可用
- `config` 支持 `--info`、`--init`、`get`、`set`、`--list`
- `update <name|repo>` 与 `update --all` 可用
- 旧配置仍可读取
- 现有核心下载测试可迁移并通过

## 风险与控制

### 风险一：兼容入口与子命令路由冲突

控制措施：

- 固定“首个非 flag 参数命中子命令，否则默认 install”的规则
- 为命令路由单独补测试

### 风险二：配置优先级行为变化

控制措施：

- 将优先级规则固化到独立逻辑和测试
- 避免在 CLI 层直接拼接配置

### 风险三：迁移期间结构稳定但行为回归

控制措施：

- 采用分阶段迁移
- 每一步都要求编译和回归测试通过

### 风险四：`add`/`update` 与旧安装记录逻辑冲突

控制措施：

- 保持 package 配置与 installed 状态分离
- 先让 `update` 依赖 package 配置，installed 只作为运行结果与辅助信息

## 结论

本设计以“分层重构 + 渐进兼容”为核心策略，目标是在不破坏现有默认安装体验的前提下，把 `eget` 从单入口平铺结构迁移为基于 `gookit/cflag/capp` 的多命令 CLI，并为后续继续扩展管理类命令和 source 抽象提供明确边界。
