# 外部包管理器纳入管理设计

## 背景

eget 目前只管理自己安装的包：`[packages.<name>]` 描述配置，`installed.toml` 记录安装结果，`list` 合并两者展示，`update` 通过 `LatestInfoFunc` 比较版本。

实际使用中，大量命令行工具是通过其他工具链装的：

- `npm i -g` / `pnpm add -g` 装的 node 全局包
- `uv tool install` 装的 python 工具
- `pipx install` 装的 python 应用
- `cargo install` 装的 rust 二进制
- `bun add -g` 装的包

这些包当前完全在 eget 视野之外。痛点是：**想看它们的版本、想更新，只能分别记住每个工具的 `outdated` / `upgrade` 命令，结果就是长期忘了更新。**

现有主链路的关键约束（相对路径）：

- `internal/app/list.go`：`ListService.ListPackages()` 由 `cfg.Packages` + `installed.toml` 构建 `ListItem`；`checkOutdatedItem()` 依赖 `InstalledTag != ""`，并通过 `LatestInfoFunc` 按 target 前缀分派（`internal/cli/wiring.go` 的 `latestInfo` 闭包）。
- `internal/app/update.go`：`UpdatePackageStatus()` 通过 `findUpdateTarget()` 定位后调用 `Install.InstallTarget()`；`UpdateCandidates()` 批量执行。
- `internal/sdk/`：一个自带存储（`sdk.installed.json`）、自带 CLI 命令组、数据驱动（`[sdk.<name>]` + `builtin_config.go` 内置模板）的子系统，是本功能最合适的参照。

## 目标

- **默认不改变现有行为**：`eget list` / `eget update` 默认**不**处理外部管理器的包，与当前完全一致。
- 用 `--managers <all|npm,scoop…>`（只看管理器包）或 `--with-managers <all|npm,bun…>`（eget 包 + 指定管理器）显式选择参与的外部包；也可用 `[global]` 配置默认行为。
- 选中后，`eget list` 中外部包与 eget 自己的包一起展示，`Source` 列显示管理器名（`npm`、`uv`…）。
- 选中后，`eget list --outdated` / `eget update` / `eget update --interactive` 纳入支持过期检测的外部包。
- `eget update <manager>:<pkg>` 直接更新单个外部包（显式引用不受默认开关限制）。
- `eget managers list` / `eget managers upgrade <name> [pkg...]` 提供管理器维度操作，覆盖 cargo 这类没有 `outdated` 命令的场景（uv 有 `uv tool list --outdated`，可用）。
- 可通过 `[managers.<name>]` 配置扩展到任意工具（scoop、go、自研脚本…），无需改代码。

## 非目标

- `eget install npm:<pkg>`（用管理器装包）。语法已预留，后续再加。
- 把外部包写入 `installed.toml`，或用 `eget uninstall` 卸载外部包。
- **不内置 deno、yarn**（按需求去除）。`go install` 也没有全局包清单，同样不内置；这三者需要时用 `lines-regex` 自行配置。
- 默认纳管外部包：作为默认行为纳入不在范围内，必须显式选择（CLI flag 或 `[global]` 配置）。
- `eget show <manager>:<pkg>` 详情页。

## 命名决策

对外引用使用：

```text
<manager>:<pkg>
```

例如 `npm:typescript`、`uv:ruff`。选择理由：

- 与现有 `owner/repo`、`sourceforge:`、`template:`、`pkg-template:` 前缀风格一致。
- 天然避免与 eget 包重名冲突（npm 里有 `rg`，eget 里 `[packages.rg]` 是 ripgrep）。
- `ListItem.Repo` 直接存该引用，既用于去重也用于 `update` 定位，不需要额外的 id 概念。

配置块使用：

```toml
[managers.<name>]
```

不使用 `ext_managers`（过长）或 `providers`（与 source provider 语义冲突）。不使用 `[packages."npm:x"]`，因为外部包不写 config、不落盘，混入 `packages` 会污染“eget 管理的包”这一语义。

CLI 使用 `eget managers`（别名 `mgr`）。

## 选择模型（哪些外部包参与 list / update）

默认 **off**：不传 flag、`[global]` 也没配时，`list` / `update` 完全维持现状，不启动任何管理器进程。

**CLI**

| flag | 取值 | 含义 |
|---|---|---|
| `--managers <sel>` | `all` 或 `npm,scoop` | **只**处理这些管理器的包（不含 eget 自己的包） |
| `--with-managers <sel>` | `all` 或 `npm,bun` | eget 自己的包 **+** 这些管理器的包 |

- 两者互斥，同时传报错。
- `<sel>` 中的名字必须是已配置/内置的管理器名，未知名字报错并列出可用名字。
- 优先级：CLI flag > `[global]` 配置 > 默认 off。

**配置默认行为**

```toml
[global]
# off（默认）= 不改变现状，外部包只能通过下面的新选项查看
# on          = 改变默认行为：list / update 默认带上所有可用管理器的包
managers_mode = "off"
```

- `managers_mode = "on"` 等价于默认带上 `--with-managers all`（eget 包 + 全部管理器）。
- **不需要**在 global 里列管理器名单：管理器集合由“内置适配器 + `[managers.<name>]` 配置”自动收集，`all` 是 CLI 的内置常量取值。
- 该键只决定默认值，CLI flag 永远优先。

解析结果是 app 层共享的一个小结构（`internal/app`）：

```go
type ManagersSelection struct {
    Mode     string   // 解析后："off" | "with" | "only"
    Managers []string // 为空且 Mode != off 时表示“全部可用管理器”
}
```

配置只能产生 `off` / `with`（`on` → `with` + 全部）；`only` 只能由 `--managers` 产生。

**`list` 语义**

| 调用 | 显示内容 |
|---|---|
| `eget list`（默认） | 仅 eget 包（与现状一致） |
| `eget list --with-managers npm,bun` | eget 包 + npm/bun 的包 |
| `eget list --managers all` | 仅所有可用管理器的包 |
| `eget list --managers npm,scoop` | 仅 npm/scoop 的包 |

`list --outdated`、`list --info` 遵循同一套选择。

**`update` 语义**

| 调用 | 行为 |
|---|---|
| `eget update --all` | 仅 eget 包（与现状一致） |
| `eget update --all --with-managers npm` | eget 全部 + npm 的过期项 |
| `eget update --managers npm` / `--managers all` | 仅所选管理器的过期项（已明确集合，等同于对该集合执行 `--all`） |
| `eget update npm:agent-browser` | 直接更新该外部包，**不需要** flag（显式引用始终可用） |
| `eget update agent-browser --with-managers npm` | flag 同时充当裸名的作用域，用于消歧 |

裸名解析只在**选定作用域内**进行：`eget update typescript` 在默认作用域下仍按现状处理（找不到就报错），要更新 npm 的它必须写 `npm:typescript` 或加 `--with-managers npm`。这样默认行为不变，也不会因为外部同名包突然劫持已有用法。

**组合约束**（不兼容就报错，不静默忽略）

| 组合 | 结果 |
|---|---|
| `--managers` + `--with-managers` | 报错（互斥） |
| `--managers` + `--all` / `--gui` / `--no-installed` | 报错（这些是 eget 侧视图过滤，与“只看管理器包”冲突） |
| `--managers` + `--info <name>` | 允许，在管理器包里查详情 |
| `--with-managers` + `--all` | 允许：配置包 + 所选管理器包 |
| `--with-managers` + `--no-installed` / `--gui` | 报错（这两个视图没有“已安装的 eget 包”这一集合） |
| `--with-managers` + `--info <name>` | 允许，用于给裸名解析确定作用域 |
| `--managers` / `--with-managers` + `--outdated` | 允许（`list` 与 `update` 都适用） |

**`eget managers` 与选择模型的关系**：`eget managers ...` 是管理器维度的显式命令，always 生效，不受 `--managers` / `--with-managers` 影响。


## 设计

### 1. 实时查询，不落盘

外部包**不写入** `installed.toml`，每次由管理器 CLI 现查。

理由：版本的真相在管理器手里；写快照必然过期并产生假的“有更新”，还会让 `list` 出现无法收敛的状态。`installed.toml` 继续只表示“eget 自己装的包”。

代价：外部包的安装时间未知，`Update Time` 列显示 `-`。

### 2. 配置模型

新增顶层段 `managers`，类型 `map[string]ManagerSection`：

```toml
[managers.npm]
bin              = "npm"
list_args        = ["ls", "-g", "--depth=0", "--json"]
outdated_args    = ["outdated", "-g", "--json"]
upgrade_args     = ["update", "-g"]   # 指定包名时追加在最后
upgrade_all_args = ["update", "-g"]   # 不带包名执行，升级全部
parser           = "npm-json"
enabled          = true
timeout          = 60                 # 秒，可选

# 扩展任意工具
[managers.scoop]
bin              = "scoop"
list_args        = ["list"]
outdated_args    = ["status"]
upgrade_args     = ["update"]
upgrade_all_args = ["update", "*"]
parser           = "lines-regex"
list_regex       = '^(?P<name>\S+)\s+(?P<version>\S+)'
outdated_regex   = '^(?P<name>\S+)\s+(?P<version>\S+)\s+(?P<latest>\S+)'
```

`upgrade_all_args` 用**显式参数**而不是 bool：uv 是 `tool upgrade --all`（在子命令后加 flag），pipx 是 `upgrade-all`（独立子命令），形态不同，bool 表达不了。

`[global]` 另有一个决定默认行为的键（属于 `Section` 的 global 字段，见“选择模型”）：

```toml
[global]
managers_mode = "off"    # off | on
```

它不参与 `MergeInstallOptions`（不是安装选项），只被 list/update 的选择解析读取；管理器的集合不需要配置，由内置 + `[managers.<name>]` 自动收集。

**能力可缺省**：`outdated_args` 为空即该管理器“不支持过期检测”。这类包照常出现在 `eget list`，但不进 `--outdated` / `update --all`，更新走 `eget managers upgrade <name>`。

配置需要按项目现状在 5 处同步（严格类型、无兜底字段，漏一处就会丢字段或报错）：

- `internal/config/model.go`：`ManagerSection` + `File.Managers`；`Section` 增 global 键 `managers_mode`。
- `internal/config/loader.go`：`NewFile()` 初始化 `Managers` map。
- `internal/config/gookit.go` 解码/编码：`decodeConfigFile` 增 `MapOnExists("managers", ...)`；`encodeConfigFile` 增 `"managers"` 段与 `managerSectionToMap()`；`sectionToMap` 增 `managers_mode` 输出；`isReservedConfigRootKey` 增 `"managers"`（否则会被当成 legacy repo 段）。
- `internal/config/gookit.go` 的 `preserveUnchangedRawValues()`：根层跳过列表（现为 `packages`、`sdk`）增 `managers`，与 `packages` / `sdk` 一样整段由类型模型重新生成。
- `internal/config/gookit.go` 的 `normalizePathValue()`：如需 `config set managers.npm.enabled true`，补 `enabled` 的 bool 分支与 `list_args` / `outdated_args` / `upgrade_args` / `upgrade_all_args` 的 `splitAndTrim` 分支。

`config list` **不会**自动显示新段：`internal/cli/config_handler.go` 的 `list` 分支是显式枚举（`global` / `apiCache` / `ghproxy` / `packages` / `sdk`），需补一段 `managers`（`show.MList(cfg.Managers, showListConfig)`）。`config export` / `get` / `set` 走 `encodeConfigFile`，加完上面第 3 条即自动生效。

复用现有 `[global] ignore_update_packages` 排除特定外部包，可写 `typescript` 或 `npm:typescript`；注意它必须同时作用于**外部过期检测结果**，见下节。

### 3. 新增包 `internal/extpkg/`

- `model.go`

  ```go
  type Package struct { Manager, Name, Version, Latest string }

  type Manager struct {
      Name, Bin                           string
      ListArgs, OutdatedArgs, UpgradeArgs []string
      UpgradeAllArgs                      []string
      Parser                              string
      ListRegex, OutdatedRegex            string
      Enabled                             bool
      Timeout                             time.Duration
  }

  type CommandResult struct { // 三个字段都必需，见 §4.3 解析器契约
      Stdout   []byte
      Stderr   []byte
      ExitCode int
  }

  type UpgradeResult struct {
      Manager, Name, From, To, Output string
  }
  ```

- `builtin.go`：`builtinManagers() []Manager`（见下表）。
- `config.go`：`Managers(cfg *cfgpkg.File) []Manager`，内置 + `cfg.Managers` 合并（按 name 覆盖，`enabled = false` 移除）。纯函数，便于单测。
- `parse.go`：解析器注册表，签名统一收 `CommandResult`（不能只收 `[]byte`，否则丢掉退出码）：
  `ParseList(parser, listRegex string, res CommandResult) ([]Package, error)`、
  `ParseOutdated(parser, outdatedRegex string, res CommandResult) ([]Package, error)`。
  内置 `npm-json`、`pnpm-json`、`pipx-json`、`uv-tool-text`、`cargo-text`、`bun-text`，以及通用 `lines-regex`（支持 `name` / `version` / `latest` 命名分组）。
- `exec.go`：默认 Runner，`exec.LookPath` + `exec.CommandContext` + 超时，分别接 stdout/stderr 并回填退出码（Windows 上 `.cmd` 可直接执行，已实测）。
- `service.go`

  ```go
  type Runner interface {
      Run(ctx context.Context, bin string, args []string, timeout time.Duration) (CommandResult, error)
  }

  type Service struct {
      Managers []Manager
      Runner   Runner
      LookPath func(string) (string, error)
  }

  func (s Service) List(ctx context.Context) ([]Package, error)
  func (s Service) Outdated(ctx context.Context) ([]Package, []OutdatedFailure, error)
  func (s Service) Upgrade(ctx context.Context, ref string, names []string) ([]UpgradeResult, error)
  func (s Service) Resolve(ref string) (Manager, string, bool) // "npm:typescript" -> (npm, typescript, true)
  func (s Service) Manager(name string) (Manager, bool)
  ```

**执行外挂抽象**：所有命令经 `Runner` 走，测试注入假实现即可，无需真装 npm；每个命令带超时（默认 60s，可配置），避免管理器卡死。

### 4. 内置适配器（含本机实测）

实测环境：Windows，2026-09-21。6 个内置管理器都已在**空态 + 已填充**两种状态下实测：npm 11.12.1、pnpm 12.4.2、uv 0.12.15、bun 1.4.2、pipx 1.17.5、cargo 1.98.1。唯一仍缺的是 **pnpm 的已填充样本**（原因见 §4.7）。

⚠️ 本机有 3 个管理器的可执行文件**不在 PATH**（pipx、cargo、pnpm 的 global bin 目录），这直接影响"管理器是否可用"的判定，详见 §4.3 第 9 条。

| name | bin（LookPath 实际解析） | list_args | outdated_args | upgrade_args | upgrade_all_args | parser | 实测 |
|---|---|---|---|---|---|---|---|
| npm | `npm` → `npm.cmd`（nodejs 安装目录） | `ls -g --depth=0 --json` | `outdated -g --json` | `update -g` | `update -g` | npm-json | ✅ |
| pnpm | `pnpm` → `pnpm.cmd`（npm 全局 bin） | `list -g --depth=0 --json` | `outdated -g --json` | `update -g` | `update -g` | pnpm-json | ⚠️ 部分 |
| uv | `uv` → `uv.exe` | `tool list` | `tool list --outdated` | `tool upgrade` | `tool upgrade --all` | uv-tool-text | ✅ |
| pipx | pipx（常不在 PATH，见 §4.5） | `list --json` | `list --json --outdated` | `upgrade` | `upgrade-all` | pipx-json | ✅ |
| cargo | cargo（rustup shim，需 `CARGO_HOME`/`RUSTUP_HOME`，见 §4.6） | `install --list` | *(空)* | `install` | *(空)* | cargo-text | ✅ |
| bun | bun | `pm ls -g` | `outdated -g` | `update -g` | `update -g` | bun-text | ✅ |

耗时实测（热态，单次）：`npm ls -g` 710ms、`npm outdated -g` 805ms、`pnpm list -g` 37ms、`pnpm outdated -g` 25ms、`uv tool list` 17ms。即 npm 是绝对瓶颈，**按管理器并发后 `list` 的额外耗时 ≈ 最慢管理器（约 0.7~0.8s）**，可接受。

#### 4.1 实测原始输出

npm `ls -g --depth=0 --json`（exit 0，stderr 空）：

```json
{
  "name": "npm",
  "dependencies": {
    "@colbymchenry/codegraph": { "version": "0.9.4", "overridden": false },
    "command-code": { "version": "1.58.1", "overridden": false }
  }
}
```

> 只解析 `dependencies.<name>.version`；本机样本无 `problems` 等额外键，解析器需容忍未来新增键。

npm `outdated -g --json`（**exit 1**，JSON 在 stdout，stderr 空）：

```json
{
  "agent-browser": {
    "current": "0.27.2",
    "wanted": "0.38.1",
    "latest": "0.38.1",
    "dependent": "global",
    "location": "C:\\Users\\<user>\\AppData\\Roaming\\npm\\node_modules\\agent-browser"
  }
}
```

> 只有过期的包会出现；取 `current` 与 `latest`（`wanted` 是满足 semver 范围的版本，展示时二者都保留但比较用 `latest`）。

pnpm `list -g --depth=0 --json`（exit 0）：**顶层是数组**

```json
[ { "path": "…\\AppData\\Local\\pnpm\\global\\v11", "private": true, "dependencies": {} } ]
```

> 依赖在 `[0].dependencies`，**不是**顶层对象。本机 pnpm 全局目录为空（pnpm 自己由 npm 全局安装），所以 `dependencies` 有值时的字段形状未实测；实现时按 npm 兼容的 `{ "version": "x.y.z" }` 解析并容错。

pnpm `outdated -g --json`（exit 0）：本机无全局包，返回 `{}`。空结果是合法 JSON，需按"无过期"处理。

uv `tool list`（exit 0）：

```text
graphifyy v0.8.18
- graphify
semble v0.2.0
- semble
```

uv `tool list --outdated`（exit 0）：

```text
graphifyy v0.8.18 [latest: 0.9.65]
- graphify
semble v0.2.0 [latest: 0.6.0]
- semble
```

> 格式：`<tool> v<version>`，可选 ` [latest: <latest>]`；随后是缩进的 `- <executable>` 行，解析时**必须跳过**（uv 工具名是包名，可执行文件名可能不同：`graphifyy` → `graphify`）。同一份文本解析器同时服务 list（无 `[latest:]`）与 outdated（有 `[latest:]`）。upgrade 用工具名。

#### 4.2 Windows 执行方式（已用 Go 验证）

- `exec.LookPath("npm")` 返回 `npm.cmd`（Go 会按 `PATHEXT` 解析，不是 `.ps1`），`exec.CommandContext` **可直接执行 `.cmd`**，无需 `cmd.exe /c` 包裹。
- 缺失命令：`exec.LookPath` 返回 `exec: "x": executable file not found in %PATH%` —— 据此判定"管理器不可用"。
- 实测：`LookPath("npm")`→`npm.cmd`、`LookPath("pnpm")`→`pnpm.cmd`、`LookPath("uv")`→`uv.exe`，三者 `--version` 均正常返回。
- `bin` 除了名字也接受**绝对路径**（`exec.LookPath` 对含路径分隔符的值直接按路径解析）。这是"工具装了但不在 PATH"的逃生口，例如 pipx（见 §4.5）。

#### 4.3 解析器契约（由实测反推，必须遵守）

1. `Runner` 必须同时拿到 **stdout、stderr、退出码**，不能只看 error。
2. **非零退出码不等于失败**：`npm outdated -g --json` 在有过期包时 exit 1，stdout 是合法 JSON。判定顺序应为"先尝试解析 stdout，解析成功则成功"。
3. **npm 把错误写到 stdout**（实测 `npm definitely-not-a-command`：exit 1、stdout 有 102B 错误文本、stderr 为空）；而 **pipx 把人类提示写到 stderr**、stdout 保持干净。两个流约定相反，所以 Runner 统一收两流、解析只吃 stdout、报错时带上两流。
4. 空结果是合法的：`{}`（npm / pnpm 无过期）、`{"venvs":{}}`（pipx 无包）都要解析成"空列表"，不能当失败。
5. uv 的 `[latest: x]` 与 `- exe` 行要用同一套行解析器，避免两份重复逻辑。
6. **同一管理器的 list 与 outdated 可能是不同 schema**：pipx 的 `list --json` 是 `venvs` 形状，而 `list --json --outdated` 是 `data.packages` 信封。解析器按**形状分派**，不能假设"outdated 只是 list 多一个字段"。
7. 信封型输出（pipx `--outdated`）除 exit code 外还要检查 `status` / `errors[]`。
8. 命令失败但语义是"空"（bun 缺 `package.json`）要特判为空，不报 `check_failed`。
9. **"已安装但不在 PATH"是常态，不是异常**：本机 6 个管理器里有 3 个中招（pipx 的用户级 Scripts、cargo 的 rustup shim、pnpm 的 global bin 目录）。所以 `LookPath` 失败时不能只说"不可用"，要给出可操作的提示（`<tool> setup` / `ensurepath` / 配 `[managers.<x>] bin`）。
10. **同一管理器的 list 与 outdated 字段名可能不一致**：pipx list 用 `package_version`，outdated 用 `version`。两个 schema 各写各的解析，不共用 struct。
11. **空输出也是合法结果**：`cargo install --list` 无包时是 **0 字节、exit 0**，不能当成"命令没跑起来"。

#### 4.4 bun 实测（已装，但无全局包）

bun 1.4.2（PATH 中为 `bun.exe`）。本机没有 bun 全局包，而 bun 在缺 `package.json` 时**报错而非返回空**：

```text
$ bun pm ls -g          # exit 1
stdout: (空)
stderr: error: No package.json was found for directory "…\.bun\install\global"
        note: Run "bun init" to initialize a project

$ bun outdated -g       # exit 1
stdout: bun outdated v1.4.2 (744846f84)
stderr: error: missing package.json, nothing outdated
        error: failed to initialize bun install: MissingPackageJSON
```

要点：

- `bun pm ls` **没有 `--json`**（只有 `bun pm licenses --json`），bun 必须走文本解析。
- `bun outdated -g` 的 stdout 会先打印 `bun outdated v<ver>` 横幅，解析必须跳过。
- "缺 package.json" 语义上等于"没有全局包"，**必须特判为空列表**，否则 bun 永远出现在 `check_failed` 里。

已填充样本（临时 `bun add -g is-number@1.0.0` 后测得，测完已 `bun remove -g` 还原）：

`bun pm ls -g`（exit 0）：首行是根信息，随后是依赖树，`└──` / `├──` 前缀要剥离：

```text
C:\Users\<user>\.bun\install\global node_modules (1 installed)
└── is-number@1.0.0
```

`bun outdated -g`（exit 0）：横幅 + markdown 风格表格，分隔行（`|---|`）要跳过：

```text
bun outdated v1.4.2 (744846f84)
|---------------------------------------|
| Package   | Current | Update | Latest |
|-----------|---------|--------|--------|
| is-number | 1.0.0   | 1.0.0  | 7.0.0  |
|---------------------------------------|
```

即 `Package` → 名称、`Current` → 当前版本、`Latest` → 最新版本；`Update` 是满足 semver 范围的目标版本（与 npm 的 `wanted` 同义）。

#### 4.5 pipx 实测（已装，但无 venv）

pipx 1.17.5，用 `python -m pip install --user pipx` 安装。**脚本落在用户级 Scripts 目录，默认不在 PATH**（`Get-Command pipx` 找不到）。

`pipx list --json`（exit 0）：stdout 是干净 JSON，人类可读提示写在 **stderr**：

```json
{
    "pipx_spec_version": "0.1",
    "venvs": {}
}
```

```text
stderr: nothing has been installed with pipx 😴
```

`pipx list --json --outdated`（exit 0）：**完全不同的 schema**（"pipx_result" 信封）：

```json
{
  "command": ["list"],
  "data": { "packages": [], "packages_checked": 0, "skipped": [] },
  "errors": [],
  "exit_code": 0,
  "pipx_result_version": "1",
  "status": "success"
}
```

要点：

- **两个子命令的 JSON 结构不同**：list 是 `venvs.<pkg>`，outdated 是 `data.packages[]`。同一个 `pipx-json` 解析器必须按形状分派（有 `venvs` → list 形状；有 `data.packages` → outdated 信封），不能假设"outdated 只是多加一个字段"。
- **字段名也不同**：list 里版本字段是 `main_package.package_version`，outdated 里是 `version` / `latest_version`。
- 与 npm **相反**：pipx 的人类提示走 stderr，stdout 干净。所以"JSON 只吃 stdout"在这里成立，但两个流都要收仍然必要（npm 那条要靠 stdout 报错）。
- 空态：`venvs: {}`、`packages: []` 都是合法"没有包"，exit 0。
- `--outdated` 是 pipx 官方 flag（`pipx list -h` 已确认）。信封里的 `errors[]` / `status` 也要检查，不能只看 exit code。
- **PATH 问题（重要）**：pipx 装完默认不在 PATH，`exec.LookPath("pipx")` 会失败 → eget 视为"管理器不可用"。对策两条：跑 `pipx ensurepath`，或在 `[managers.pipx] bin = "<绝对路径>"` 显式指定（`exec.LookPath` 对含分隔符的路径按路径直接解析，这是 `bin` 字段的通用逃生口）。

已填充样本（临时 `pipx install pycowsay==0.0.0.1` 后测得，测完已 `pipx uninstall` 还原）：

`pipx list --json`（exit 0，节选）：名称与版本在 `main_package` 下：

```json
{
  "pipx_spec_version": "0.1",
  "venvs": {
    "pycowsay": {
      "metadata": {
        "main_package": {
          "package": "pycowsay",
          "package_or_url": "pycowsay==0.0.0.1",
          "package_version": "0.0.0.1",
          "pinned": false,
          "apps": ["pycowsay.exe"]
        }
      }
    }
  }
}
```

`pipx list --json --outdated`（exit 0）：

```json
{
  "command": ["list"],
  "data": {
    "packages": [
      { "package": "pycowsay", "version": "0.0.0.1", "latest_version": "0.0.0.2",
        "environment": "pycowsay", "pinned": false, "injected": false }
    ],
    "packages_checked": 1,
    "skipped": []
  },
  "errors": [], "exit_code": 0, "pipx_result_version": "1", "status": "success"
}
```

#### 4.6 cargo 实测（rustup 安装，需环境变量）

cargo 是通过 scoop 的 `rustup-msvc` 装的：`<scoop>\persist\rustup-msvc\.cargo\bin\cargo.exe`（rustup shim）。cargo 1.98.1。

- **不在 PATH**，且直接调用 shim 会失败（exit 1）：

  ```text
  error: rustup could not choose a version of cargo to run, because one wasn't
  specified explicitly, and no default is configured.
  help: run 'rustup default stable' to download the latest stable release of Rust and set it as your default toolchain.
  ```

  原因是 rustup shim 需要 `RUSTUP_HOME`（和 `CARGO_HOME`）指向真正的安装目录。补上这两个环境变量后 `cargo --version` → `cargo 1.98.1`，`rustup toolchain list` → `stable-x86_64-pc-windows-msvc (active, default)`。

  → eget 的结论：**只继承父进程环境，不要去猜 `CARGO_HOME`**；如果用户环境不完整，就在错误信息里把 rustup 的原始提示透出来（`rustup default stable`），这才是可操作的。必要时用 `[managers.cargo] bin = "<toolchain>/bin/cargo.exe"` 绕过 shim。

- `cargo install --list` 空态：**0 字节输出、exit 0**（stderr 也空）—— 空的 stdout 是合法结果。
- 已填充样本（用本地临时 crate `cargo install --path` 测得，测完已 `cargo uninstall` 还原，回到 0 字节）：

  ```text
  cargotest v0.1.0 (C:\...\cargotest):
      cargotest.exe
  ```

  格式：`<name> v<version> (<source>):` 一行，随后每行是缩进（4 空格）的可执行文件名。registry 安装时 `<source>` 为 `crates.io`。
- cargo **无内置 outdated**（需要第三方 `cargo-update`），`outdated_args` 留空。

#### 4.7 未实测项与补测方式

唯一缺口是 **pnpm 的已填充样本**：

```bash
pnpm add -g is-number@1.0.0
```

本机执行会**直接报错退出**：

```text
× The configured global bin directory "…\AppData\Local\pnpm\bin" is not in PATH
  help: Run "pnpm setup" to update your shell configuration.
```

即 pnpm 要求它的 global bin 目录在 PATH 上，否则连安装都拒绝（`--global-bin-dir` 在 1.x 的 `pnpm add` 上不是合法参数，试过无效）。修法是 `pnpm setup`（会改用户的 shell 配置），**未执行**。因此 pnpm 的 `dependencies.<pkg>` 已填充字段形状按 npm 兼容的 `{ "version": "x.y.z" }` 实现并容错，标注未验证。

若确实需要该样本：先 `pnpm setup`（或手动把该 bin 目录加进 PATH），再重跑上面的命令。

至此 6 个内置管理器的命令、参数、空态与（除 pnpm）已填充输出均已实测。实施时**不要再假设**，直接照 §4 写解析器与 fixture。

### 5. `list` 集成

- `ListItem` 增 `Manager string`；`OutdatedItem` 增 `Manager string`。
- `ListService` 增 `External ExternalProvider`（接口 `List(ctx) ([]extpkg.Package, error)` / `Outdated(ctx)`，即 `extpkg.Service` 的窄化）与 `Managers ManagersSelection`（见“选择模型”）。
- `ListPackages()`：`Managers.Mode == "off"` 时**完全不碰外部包**（不启动任何管理器进程、零额外耗时）；否则在合并 installed 之后追加所选管理器的包：`Name = 包名`、`Repo = "npm:typescript"`、`Manager = "npm"`、`Version = InstalledTag = 版本`、`Installed = true`、`IgnoreUpdate = ignoredUpdates[name]`。
- `Mode == "only"` 时只输出管理器包（丢弃 eget 侧的 item）；`Mode == "with"` 时追加。
- `Managers` 由 CLI 按调用点解析（flag > `[global]` 配置 > 默认 off），**不用** wiring 里的静态默认值。
- `ListOutdatedPackages()` 内部会新建一个 `ListService`（只透传 `LoadConfig` / `LoadInstalled` / `LatestInfo`）——必须把 `External` / `Managers` 一并透传，否则外部检测在两处调用点丢失。
- `checkOutdatedItems()` 增加过滤：`item.Manager != ""` 一律跳过，外部过期检测**不走** `LatestInfoFunc`（否则会落到 GitHub 分支报错）。
- 外部过期检测在 repo 检查之后调用 `External.Outdated(ctx)` 合并：同样应用 `ignore_update_packages`；失败项转成现有 `OutdatedCheckFailure`（`Name` 与 `Repo` 都填管理器名，避免现有 `check_failed %s (%s)` 输出出现空括号）；`checked` 计数要把外部包算进去，否则 “Checked N packages” 与实际不符。
- `internal/cli/list_handler.go` 的 `packageSource()` 增分支：`item.Manager != ""` 返回管理器名（否则 `DetectTargetKind("npm:x")` 落到 `TargetUnknown`，会显示成 `unknown`）。
- 与 list 视图过滤（`--all` / `--gui` / `--no-installed` / `--info`）的组合约束见“选择模型”，不兼容组合要**报错**，不静默忽略。
- 性能：默认 off 时零开销；开启后 `extpkg.Service.List` 按管理器并发，bin 不存在的直接跳过，只会多出约“最慢那个管理器”的耗时（实测 npm 约 0.7~0.8s）。若实测过慢，后续再加短 TTL 缓存，本期不做。

### 6. `update` 集成

- `UpdateService` 增 `External ExternalProvider`（含 `Outdated` 与 `Upgrade`）与 `Managers ManagersSelection`。
- `ListUpdateCandidates()` 追加外部过期候选：`OutdatedItem{Manager: "npm", Name: "typescript", Repo: "npm:typescript", InstalledTag, LatestTag}`。它内部新建的 `ListService` 同样需要透传 `External` / `Managers`；`Mode == "off"` 时不追加。
- `UpdateCandidates()` 按 `item.Manager` 分派，**不经过名字解析**：`item.Manager != ""` → `s.External.Upgrade(ctx, item.Repo, []string{item.Name})`；其余沿用原 `s.UpdatePackage(item.Name, cli)`。这样彻底避开重名歧义。
- `update --interactive` 的候选展示改用 `item.Repo`（形如 `npm:typescript`）而非 `item.Name`，否则不同管理器的同名包无法区分。
- **候选里含外部项时强制串行（batch = 1）**，避免多个管理器进程互相抢锁（npm/pnpm 会争同一目录）。
- 单包路径 `eget update <target>` 分三档：`<manager>:<pkg>` 显式引用**始终可用**（不受选择模型影响）；裸名只在选定作用域内解析（`Mode != off` 时才查外部包）；命中多个、或多个管理器同名，则报错并列出候选要求写显式引用；都不命中才走原有 repo 流程。`findUpdateTarget()` 内部自建的 `ListService` 不带 `External`，所以这套判断要在 `UpdatePackageStatus` 开头显式做。
- 外部包不受 `InstalledTag == ""` 限制（该检查只对 repo 类 item 有意义）。
- `--managers npm` / `--managers all` 隐含对该集合执行 `--all`（只更新所选管理器的过期项，不含 eget 包）；`--with-managers` 需与 `--all` 或显式 target 联用。

### 7. CLI

- `internal/cli/list_cmd.go` / `update_cmd.go`：新增 `--managers <sel>` 与 `--with-managers <sel>`（都是**带值**的字符串 flag）；两者互斥与视图过滤的组合约束在 handler 里校验（见“选择模型”）。
- 新增选择解析函数（放在 `internal/cli`，例如 `resolveManagersSelection(opts, cfg)`）：按 `flag > [global] managers_mode > off` 得出 `app.ManagersSelection{Mode, Managers}` —— `managers_mode = "on"` → `{Mode: "with", Managers: nil}`（全部），`--managers <sel>` → `only`，`--with-managers <sel>` → `with`；`<sel>` = `all` 或逗号列表，名字必须是内置/已配置管理器，未知名字报错并列出可用项。
- `internal/cli/wiring.go`：构造 `extpkg.Service{Managers: extpkg.Managers(cfg), Runner: extpkg.NewExecRunner(), LookPath: exec.LookPath}`，注入 `listService.External` / `updService.External`；“是否启用、启用哪些”每次调用按解析结果设置。
- `internal/cli/service.go`：`cliService` 增 `extService extCLIService`（接口，便于测试替身）。
- 新增 `internal/cli/managers_cmd.go` + `managers_handler.go`（参照 `sdk_cmd.go` / `sdk_handler.go`）：
  - `eget managers list`：管理器名 / bin 路径 / 是否可用 / 是否支持 outdated / 包数。
  - `eget managers upgrade <name> [pkg...]`：无包名时按 `upgrade_all_args`（如 uv `tool upgrade --all`）或全部已装包执行。
  - 分派名 `managers.list` / `managers.upgrade`。
- `internal/cli/app.go`：`app.add(newManagersCmd(handler))`；`commandFlagSpecs` 增 `"managers"` 的 `subs`，**并给 `list` / `update` 的 spec 增 `values: setOf("managers","with-managers")`** —— 这两个 flag 带值，不进这张表 `validateKnownFlags` 会直接报未知 flag。
- `internal/cli/handlers.go`：增 `case "managers.list"` / `case "managers.upgrade"`。
- `internal/cli/config_handler.go`：`config list` 的 `list` 分支增 `managers` 段渲染（见 §2）。
- `internal/cli/render/render.go`：`ListItemToDisplay` 增 `Manager` 字段（`list --info` 展示）。

### 8. 错误处理

- 管理器 bin 不存在：静默跳过（`-v` 时输出提示），不影响其他管理器。
- 管理器不在 PATH（实测 pipx / cargo / pnpm 都会中招）：提示可操作命令（`pipx ensurepath`、`rustup default stable`、`pnpm setup`），并提示可用 `[managers.<x>] bin = "<绝对路径>"` 绕过。**不要**把这类失败当成"没有包"。
- 管理器命令执行失败（如 npm registry 不可达）：转为 `OutdatedCheckFailure{Name: 管理器名, Repo: 管理器名, Error: err}`，走现有 `check_failed %s (%s)` 输出，不中断整体检查。
- 命令失败但语义是“空”（bun 缺 `package.json` 报 `missing package.json`）：**特判为空列表**，不算失败。
- 超时：按 section `timeout` 结束并报错。
- 解析失败：报清晰错误，包含管理器名与所用 parser，并带上 stdout/stderr 文本。
- 选择参数：`--managers` 与 `--with-managers` 同传、未知管理器名、与视图过滤的非法组合 —— 一律报错，错误信息里列出可用管理器名或合法组合。
- 裸名歧义（eget 包与外部包同名，或多个管理器都有该包）：报错并列出候选，要求使用 `manager:pkg`。
- 非法引用（`npm:` 空包名、未知管理器名）：报错。

### 9. 测试策略

单元测试（使用 `github.com/gookit/goutil/x/assert`，同方法多用例用 `t.Run()`）：

- `extpkg`：各管理器真实输出的黄金样本解析（含 bun 的“缺 package.json = 空”）；`lines-regex` 自定义分组；配置合并（内置 / 覆盖 / 禁用）；假 Runner 下的缺 bin、超时、空结果、非零退出、并发。
- `app`：假 `ExternalProvider` → `ListPackages` 外部项字段正确；`Mode == off` 时**不调用** `External`（用假实现断言零调用）；`only` 只输出外部项、`with` 追加；`checkOutdatedItems` 跳过外部项；`ListOutdatedPackages` / `ListUpdateCandidates` 确实透传了 `External` / `Managers`；外部过期结果受 `ignore_update_packages` 过滤；`checked` 计数含外部包；`UpdateCandidates` 分派到 `Upgrade` 且混合候选强制串行；`UpdatePackageStatus` 的显式引用 / 裸名唯一 / 歧义报错三分支。
- `cli`：习惯用法的组合矩阵（`--managers` + `--all` 报错、`--managers` + `--with-managers` 报错、`--with-managers` + `--all` 通过）；选择解析优先级（flag > `[global]` > off）；未知管理器名报错；`list` 外部行 `Source=npm`；`managers list/upgrade` 分派与 `flagSpec` 校验（`managers` / `with-managers` 必须已登记为带值 flag）；`config list` 显示 `managers`。
- `config`：`[managers.*]` round-trip（decode → encode 不丢字段）；`global.managers_mode` round-trip；`isReservedConfigRootKey` 包含 `managers`；`preserveUnchangedRawValues` 在含 `managers` 配置下的保存不丢字段。

本功能主要依赖外部进程，故以假 Runner + 本机实测为主，不引入新的网络集成测试。

完成 MVP 主链路改动后必须运行：

```bash
go test ./...
```

### 10. 实施范围建议

本功能涉及配置模型、配置读写、`extpkg` 新包、`app` 的 list/update、`cli` 的 wiring/命令/渲染与文档，**修改的逻辑文件远超 3 个**，按项目规范实施前需要再次确认范围。

建议分期：

1. 文档落位：在设计/计划目录创建本设计文档与实施计划，并登记 `AGENTS.md`。
2. 配置模型 `[managers.*]` + `[global] managers_mode`（model / gookit / loader / config list 渲染）与 round-trip 测试。
3. `internal/extpkg` 包：model / builtin / config / parse / exec / service + 黄金样本测试。
4. 固化**已实测**的 npm / pnpm / uv / bun（空态）样本到 `internal/extpkg/testdata/`；补测 pipx / cargo（bun 补装一个全局包）后定稿其解析器与参数（补测不可得时，该管理器先 `enabled = false`，并在设计文档 §4 标注未验证）。
5. `app` 层：list / update 集成 + 选择模型（`ManagersSelection`）与测试。
6. `cli` 层：选择解析（flag > `[global]` > off，含互斥/组合校验）/ wiring / service / flags（含 `commandFlagSpecs` 登记）/ handlers / `managers` 子命令 / 渲染 + 测试。
7. 文档：`README.md`、`README.zh-CN.md`、`docs/config.md`、`docs/config.zh-CN.md` 补 `[managers.*]`、`[global]` 两个新键与 `eget managers`、`--managers` / `--with-managers`。
8. 全量 `go test ./...` + 本机实测，然后提交并 push。

## 自查

- 外部包不落盘、不进入 `installed.toml`，`installed.toml` 语义保持“eget 自己装的包”。
- **默认 off**：不传 flag、`[global]` 未配时，`list` / `update` 行为与现状一致，且不启动任何管理器进程、零额外耗时。
- 选择模型只用 flag + `[global]` 两个键表达，没有新增按包配置；`--managers` 与 `--with-managers` 语义互斥且能覆盖"只看 / 附带"两种诉求。
- 外部过期检测独立于 `LatestInfoFunc`，没有污染 GitHub / template / sourceforge 分派逻辑。
- 配置字段全部指针/切片，符合现有 `Section` / `SDKSection` 的“未设置可区分”风格，无兜底 map。
- 命令与解析全部数据驱动，新增工具不需要改 Go 代码（`lines-regex` 兜底）。
- 执行全部经 `Runner` 抽象，测试不依赖真实安装 npm/cargo。
- 复用现有 `ignore_update_packages`、`OutdatedCheckFailure`、`UpdateCandidates` 等既有机制，不新增重复概念。
- 文档内路径均为相对路径。
- 6 个内置管理器的命令、参数与输出形状（空态 + 已填充）已在本机实测并写入 §4；唯一未验证的是 pnpm 的已填充字段形状（§4.7 说明了原因与补测方式）。
- 解析器契约（§4.3）由实测反推得出：非零退出不能直接判失败、npm 把错误写在 stdout、空结果是合法 JSON。
- Windows 上 `.cmd` shim 可被 Go 直接执行、`LookPath` 能正确解析，已用 Go 程序验证。

## 实施确认

实施计划见 [2026-09-21-external-managers.md](../plans/2026-09-21-external-managers.md)。实现保持“实时查询 + 聚合视图 + 统一更新入口”，不扩展为包管理器本身：不负责用外部工具装包，也不接管外部包的卸载。
