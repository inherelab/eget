# 外部包管理器纳入管理实施计划

## 目标

- **默认不改变现状**：`eget list` / `eget update` 默认不处理外部管理器的包（`managers_mode = "off"`），不启动任何管理器进程。
- 用 `--managers <all|npm,scoop>`（只看管理器包）/ `--with-managers <all|npm,bun>`（eget 包 + 指定管理器）显式纳入；`[global] managers_mode = "off"|"on"` 改默认（`on` = 默认带上全部可用管理器，等价默认 `--with-managers all`）。
- 纳入后 `eget list` / `list --outdated` / `eget update` 支持外部包；`eget update <manager>:<pkg>` 始终可用。
- `eget managers list|upgrade` 提供管理器维度操作；`[managers.<name>]` 可扩展到任意工具。

设计见 [2026-09-21-external-managers-design.md](../specs/2026-09-21-external-managers-design.md)，其中 §4 是本机实测记录、"选择模型"一节是 flag/config 语义的权威定义。

## 已确认的事实（已实测，不要再假设）

- Windows：`exec.LookPath("npm")` → `npm.cmd`，`exec.CommandContext` **可直接执行 `.cmd`**；不要自己拼 `cmd /c`。
- `npm ls -g --depth=0 --json` → `{name, dependencies:{<pkg>:{version,...}}}`，exit 0。
- `npm outdated -g --json` → 仅过期项，**exit 1**，JSON 在 stdout；npm 的错误也写 stdout。
- `pnpm list -g --depth=0 --json` → **顶层数组**，依赖在 `[0].dependencies`；`pnpm outdated -g --json` 空时为 `{}`。
- `uv tool list` → 文本 `名称 v版本` + 缩进 `- 可执行名`；`uv tool list --outdated` 多出 ` [latest: X]`。
- `bun pm ls -g`（空）→ exit 1、stderr 报缺 `package.json`，必须特判为空；`bun pm ls -g`（有包）→ 首行 `<dir> node_modules (N installed)` + 树行 `└── name@version`；`bun outdated -g` → 横幅行 + markdown 表格（`Package | Current | Update | Latest`，跳过分隔行）。
- `pipx list --json` → `{pipx_spec_version, venvs}`，人类提示走 **stderr**、exit 0；`pipx list --json --outdated` 是**另一种 schema + 另一套字段名**（`data.packages[].{package,version,latest_version}` vs list 的 `main_package.package_version`）。
- `cargo install --list` 空态 = **0 字节、exit 0**；有包时是 `<name> v<version> (<source>):` + 缩进可执行名。cargo 是 rustup shim，**需要 `CARGO_HOME`/`RUSTUP_HOME`**，否则报 `rustup could not choose a version of cargo to run`（exit 1）。
- **PATH 陷阱（3 个管理器中招）**：pipx（用户级 Scripts）、cargo（rustup shim）、pnpm（`pnpm add -g` 直接因 global bin 不在 PATH 而报错）。`LookPath` 失败时要给出可操作提示，并支持 `bin = "<绝对路径>"`。
- 耗时：npm ≈ 0.7~0.8s（瓶颈）、pnpm ≈ 25~40ms、uv ≈ 17ms。
- 6 个管理器的空态与已填充输出均已实测，**唯一缺口是 pnpm 的已填充样本**（需 `pnpm setup`，未执行）。

## 阶段

### 1. 文档与登记

- [x] 创建设计文档与实施计划（`docs/superpowers/{specs,plans}/2026-09-21-external-managers*`）。
- [x] `AGENTS.md` 的 `PROCESSING WORKS` 区块登记本次工作并链接设计文档（完成后移除）。

### 2. 配置模型

- [x] `internal/config/model.go`：`ManagerSection`（`bin` / `list_args` / `outdated_args` / `upgrade_args` / `upgrade_all_args` / `parser` / `list_regex` / `outdated_regex` / `enabled` / `timeout`）+ `File.Managers`；`Section` 增全局键 `managers_mode`（`off` | `on`）。
- [x] `internal/config/loader.go`：`NewFile()` 初始化 `Managers`。
- [x] `internal/config/gookit.go`：`decodeConfigFile` 增 `MapOnExists("managers", ...)`；`encodeConfigFile` 增 `"managers"` 段 + `managerSectionToMap()`；`sectionToMap` 增 `managers_mode`；`isReservedConfigRootKey` 增 `"managers"`。
- [x] `internal/config/gookit.go`：`preserveUnchangedRawValues()` 根层跳过列表增 `"managers"`。
- [x] `internal/config/gookit.go`：`normalizePathValue()` 增 `enabled` bool 与 `list_args` / `outdated_args` / `upgrade_args` / `upgrade_all_args` 的 `splitAndTrim`。
- [x] `internal/cli/config_handler.go`：`config list` 增 `managers` 段。
- [x] 测试：`[managers.*]` 与 `global.managers_mode` round-trip、保留键、保存不丢字段、`config list` 显示（`internal/config/loader_managers_test.go`、`internal/cli/config_handler_test.go`）。

### 3. `internal/extpkg` 包

- [x] `model.go`：`Package`、`Manager`（含 `UpgradeAllArgs`）、`CommandResult{Stdout,Stderr,ExitCode}`、`Failure`、`UpgradeResult`。
- [x] `exec.go`：Runner —— `exec.LookPath` + `exec.CommandContext` + 超时，stdout/stderr 分开接、回填 `ExitCode`；不注入 eget 的 proxy 配置（仅继承环境）。
- [x] `builtin.go`：6 个内置管理器（npm/pnpm/uv/pipx/cargo/bun），参数照抄设计 §4 表格；**不内置 deno / yarn / go**。JSON 解析：npm / pnpm / pipx；文本解析：uv / bun / cargo（pipx 要处理两种 schema）。
- [x] `config.go`：`Managers(cfg)` 内置 + 覆盖合并，`enabled = false` 移除；纯函数。
- [x] `parse.go`：`ParseList` / `ParseOutdated` 收 `CommandResult`；实现 `npm-json` / `pnpm-json` / `pipx-json`（两种 schema 按形状分派）/ `uv-tool-text` / `cargo-text`（空输出=空列表）/ `bun-text`（树行 + markdown 表格、跳横幅与分隔行）/ `lines-regex`；文本解析前剥离 ANSI。
- [x] `service.go`：`List`（并发 + 缺 bin 跳过）/ `Outdated` / `Upgrade` / `Resolve` / `Manager` / `Names` / `Path`；**Bin 先解析成路径再交给 Runner**（否则绝对路径逃生口失效，实测踩到）。
- [x] 测试：黄金样本（含 pipx 两种 schema、bun 空态与表格、cargo 0 字节）、`lines-regex` 命名分组、配置合并、假 Runner 下的缺 bin / 超时 / 空结果 / 非零退出 / 并发；断言"PATH 失败"与"空结果"走不同分支。
- [x] 本机实跑 `extpkg`（临时 live 探针，验证后已删除）：npm 10 包 / 8 个过期、uv 2 包 / 2 个过期、pnpm 空、bun 空（错误特判生效）、pipx 空、cargo 空，**零 failure**；并借此发现并修掉了上面的 Bin 解析缺陷。

### 4. 固化样本

- [x] 已填充样本已实测（pipx `pycowsay==0.0.0.1`、bun `is-number@1.0.0`、cargo 本地 crate `cargo install --path`），测完均已卸载还原；原始输出见设计文档 §4.1/§4.4~§4.6。
- [x] 把实测输出落成 `internal/extpkg/testdata/` 下的 fixture 文件：`npm-ls.json`、`npm-outdated.json`、`npm-error-stdout.txt`、`pnpm-ls.json`、`uv-tool-list.txt`、`uv-tool-outdated.txt`、`pipx-list-filled.json`、`pipx-outdated-filled.json`、`cargo-list-filled.txt`、`bun-ls-empty.err`、`bun-ls-filled.txt`、`bun-outdated-filled.txt`（空态用内联字符串）。
- [ ] 唯一缺口：pnpm 已填充样本需要 `pnpm setup`（会改 shell 配置）——未执行；pnpm 的 `dependencies.<pkg>` 形状按 npm 兼容实现并容错，设计 §4.7 已标注未验证。

### 5. `app` 层（含选择模型）

- [x] `internal/app/list.go`：`ListItem.Manager`、`OutdatedItem.Manager`、`ManagersSelection{Mode, Managers}`（`off`/`with`/`only` + `Enabled()`/`OnlyManagers()`）、`ExternalProvider` 接口、`ListService.External` + `ListService.Managers` + `OnExternalFailure`。
- [x] `Mode == "off"`（零值）时不调用 `External`（零子进程，测试断言调用次数为 0）；`only` 只输出外部项；`with` 追加外部项（`Repo = "npm:x"`、`Version = InstalledTag`、`IgnoreUpdate` 取 `ignore_update_packages`，同时匹配名字与 `manager:name`）。
- [x] `checkOutdatedItems()` 排除 `item.Manager != ""`。
- [x] `ListOutdatedPackages()`：内部 ListService 故意不带 External（外部项永不进 repo 检查），外部过期由外层一次性批量取；`only` 时跳过 repo 检查；应用 `ignore_update_packages`；`checked` 计入外部包；失败转 `OutdatedCheckFailure{Name: 管理器名, Repo: 管理器名}`。
- [x] `internal/app/update.go`：`UpdateService.External` + `Managers`；`UpdatePackageStatus` 三分支（显式引用始终可用 / eget 目标优先且不调用外部服务 / 不是 eget 目标时才按裸名在作用域内解析）；外部单包更新先比对 outdated 再升级。
- [x] `internal/app/update_candidates.go`：`ListUpdateCandidates()` 批量追加外部候选（off 时不追加）；`ListUpdateCandidatesForTargets()` 支持显式 `manager:pkg`。
- [x] `internal/app/update_batch.go`：按 `item.Manager` 分派 `External.Upgrade`；含外部候选时强制 `batch = 1`。
- [x] 测试：假 `ExternalProvider` —— off 零调用、only/with 语义、`ignore_update_packages`、跳过、合并外部过期与失败、分派、外部升级串行、裸名作用域、已是最新、未安装报错（`internal/app/external_test.go`）。

### 6. `cli` 层

- [x] `list_cmd.go` / `update_cmd.go`：增带值 flag `--managers` / `--with-managers`。
- [x] 新增 `managers_selection.go` 的 `resolveManagersSelection`：`flag > [global] managers_mode > off`；`on` → `{with, 全部}`；`--managers` → `only`；`--with-managers` → `with`；`all` 或逗号列表（未知名字报错并列出可用项）；互斥与视图组合校验（矩阵见设计"选择模型"，`update --with-managers --check` 允许）。
- [x] `app.go`：`commandFlagSpecs` 给 `list` / `update` 增 `values: setOf("managers","with-managers")`，并增 `managers` 的 subs；`app.add(newManagersCmd(handler))`。
- [x] `service.go` 增 `extService app.ExternalProvider`；`wiring.go` 用 `extpkg.NewService(cfg)` 构造并注入 list/update 服务。
- [x] `list_handler.go`：`packageSource()` 增 `Manager` 分支；按选择设置 `Managers`；`OnExternalFailure` 打印 `check_failed`；组合校验。
- [x] `update_handler.go`：按选择设置 `Managers`；`--managers` 隐含 `--all`；`--check` 透传 managers flags；`--interactive` 只对外部项用 `Repo` 展示。
- [x] 新增 `managers_cmd.go` / `managers_handler.go`（`managers.list`、`managers.upgrade`）；`handlers.go` 增两个 case。
- [x] `render.go`：`ListItemToDisplay` 增 `Manager`。
- [x] 测试：组合矩阵、解析优先级、未知名字报错、`Source=npm`、默认 off 不出现外部包、`managers` 分派与参数校验（`internal/cli/managers_selection_test.go`）。
- [x] 本机端到端验证（构建 `eget-dev.exe`，验证后已删除）：默认 `list` 41 包（与改动前一致、无外部包）；`--with-managers npm` → 51 包且 `Source=npm`；`--managers all` → 只 12 个管理器包；`--managers npm --all` 与未知名字报错正常；`list --outdated --with-managers npm` 与 `update --check --with-managers npm` 含外部过期项；`update --check npm:agent-browser` 正常；`update typescript` 在默认作用域下报未找到（未被劫持）；`update rg` 走原路径；`managers list` 显示 6 个管理器（cargo/pipx 正确显示 Available=no）；`managers_mode="on"`（临时配置）下默认 `list` 带上 12 个外部包；`config list` 显示 Managers 段。

### 7. 文档与交付

- [x] `README.md`、`README.zh-CN.md`、`docs/config.md`、`docs/config.zh-CN.md` 补 `[managers.<name>]`、`[global] managers_mode`、`eget managers`、`--managers` / `--with-managers`，含各字段说明与"`bin` 可用绝对路径"的提示。
- [x] 运行 `go test ./...`：`internal/app`、`internal/config`、`internal/extpkg`、`internal/install` 等全部通过；`internal/cli` 与 `internal/sdk` 各有一批**与本改动无关**的既有环境相关失败（本机 `EGET_CONFIG_DIR` 指向 `D:/work/inhere/config/win-env/eget` 且 `.env` 里设了自更新镜像），在干净工作区上可复现同样的失败。
- [x] 本机端到端实测（见阶段 6 记录），构建产物已删除，无遗留后台进程。
- [x] 更新本计划 checkbox；分阶段提交（config / extpkg / app / cli / docs 各一次）并 push。

## 已知偏差风险与对策

| # | 风险（会导致实现跑偏） | 对策 |
|---|---|---|
| 1 | 把非零退出当失败 | `npm outdated` 有过期包时 exit 1 且 stdout 合法。先解析 stdout，成功即成功；Runner 回填 `ExitCode`，测试覆盖 exit=1 + 合法 JSON |
| 2 | 只依赖 stderr 报错 | npm 把错误写 stdout。解析失败时把 stdout+stderr 一起带进 error |
| 3 | 把空结果当错误 | `{}`（npm/pnpm 无过期）是合法空列表 |
| 4 | pnpm 解析当对象 | pnpm `list -g --json` 顶层是**数组**，依赖在 `[0].dependencies` |
| 5 | uv 文本解析误吞可执行行 | 跳过缩进的 `- <exe>` 行；` [latest: X]` 可选；uv 工具名是包名（`graphifyy` vs 可执行 `graphify`） |
| 6 | uv/bun 文本被 ANSI 或横幅污染 | 文本解析前剥离 ANSI；跳过 `bun outdated v<ver>` 这类横幅行 |
| 7 | bun 空态被判失败 | 缺 `package.json` 的报错**特判为空列表**，否则 bun 永远 `check_failed` |
| 8 | `upgrade_all` 用 bool 表达不了 | 用 `upgrade_all_args`：uv = `tool upgrade --all`，pipx = `upgrade-all` |
| 9 | 在 Windows 手写 `cmd /c npm ...` | 直接 `exec.Command("npm", ...)`；`LookPath` 已解析到 `.cmd`，自己包 shell 会破坏参数转义 |
| 10 | `config list` 看不到新段 | `config_handler.go` 是显式枚举，必须补 `managers` |
| 11 | 保存配置丢 `managers` 字段 | `preserveUnchangedRawValues` 根层跳过列表要加 `managers` |
| 12 | **默认行为被改（最严重回归）** | 选择模型默认必须 `off`；未显式选择时零子进程。用测试锁住 `Mode == off` 时 `External` 不被调用 |
| 13 | 带值 flag 未登记 | `--managers` / `--with-managers` 是带值 flag，必须进 `commandFlagSpecs`，否则 `validateKnownFlags` 直接报错 |
| 14 | flag 组合未校验 | `--managers` + `--with-managers`、`--managers` + `--all` 等非法组合要报错，不静默忽略 |
| 15 | 外部过期检测静默消失 | `ListOutdatedPackages()` / `ListUpdateCandidates()` 内部会**重建** `ListService`，必须显式透传 `External` / `Managers` |
| 16 | 外部项被送进 GitHub 查询 | `checkOutdatedItems` 必须排除 `Manager != ""` |
| 17 | 外部同名包劫持现有用法 | 裸名只在选定作用域内解析；默认 off 时 `eget update <name>` 行为与现状完全一致 |
| 18 | `ignore_update_packages` 对外部包失效 | 该过滤只在 `checkOutdatedItems` 里，外部结果要单独再过滤一次 |
| 19 | 误把 eget 的 http 代理注入管理器 | 不注入，仅继承环境 |
| 20 | 重名歧义导致更新错包 | 分派只认 `item.Manager`；`--interactive` 展示用 `Repo`；裸名多命中要报错 |
| 21 | pipx 的 list 与 outdated 是两种 JSON schema | 解析器按**形状**分派（有 `venvs` → list；有 `data.packages` → outdated 信封），不假设"outdated 是 list 加字段" |
| 22 | pipx 信封结果只看 exit code | `pipx list --outdated` 还要检查 `status` / `errors[]`（错误可能带 `exit_code: 0`） |
| 23 | pipx 装了却不在 PATH 被当成不可用 | 提示 `pipx ensurepath`，并支持 `[managers.pipx] bin = "<绝对路径>"`（`bin` 允许绝对路径） |
| 24 | `managers_mode` 被配成非法值 | 只接受 `off` / `on`，其他值报错；内部 `Mode` 的 `only` 只能由 `--managers` 产生 |
| 25 | cargo 显示"可用"但每个命令都失败 | cargo 是 rustup shim，缺 `CARGO_HOME`/`RUSTUP_HOME` 时报 `rustup could not choose a version of cargo to run`（exit 1）。错误要原样透出（含 `rustup default stable` 提示），不要吞掉当成空结果 |
| 26 | pipx 两个 schema 共用 struct → 字段名取错 | list 用 `main_package.package_version`，outdated 用 `version` / `latest_version`；两套结构分开写，按形状分派 |
| 27 | bun 文本解析把横幅/根行/树前缀当成包 | 跳过 `bun outdated v<ver>` 横幅、`<dir> node_modules (N installed)` 根行、`└──` / `├──` 前缀；outdated 是 markdown 表格，要跳过分隔行 |
| 28 | PATH 类失败与"没有包"混为一谈 | pipx / cargo / pnpm 都可能不在 PATH：`LookPath` 失败 → 标为不可用并给可操作提示；空结果是 exit 0 或 0 字节输出，两者必须区分 |
| 29 | cargo 的空结果是 0 字节被当成"命令没跑" | `cargo install --list` 无包时 stdout 全空、exit 0 → 合法空列表 |

## 验收标准

1. **默认行为不变**：`eget list`、`eget update --all` 输出与改动前一致，且不启动任何管理器进程（可对比改动前的输出）。
2. `eget list --with-managers npm,bun` → eget 包 + npm/bun 的包，`Source` 列为 `npm` / `bun`。
3. `eget list --managers all` → 只有管理器包，没有 eget 包。
4. `eget list --managers npm --all` 与 `eget list --managers npm --with-managers bun` → 报错并提示合法组合。
5. `eget list --outdated --with-managers npm` → 含 npm 过期项，`Checked N` 与实际条数一致；cargo 不出现。
6. `eget update --managers npm` → 只更新 npm 过期项；`eget update --all --with-managers npm` → eget 全部 + npm。
7. `eget update npm:agent-browser` → 无需 flag 直接更新成功。
8. `eget update agent-browser` → 默认作用域下按现状报未找到（不被外部包劫持）。
9. `eget managers list` 显示 6 个管理器的 bin / 可用性 / 是否支持 outdated / 包数；`eget managers upgrade uv` 触发 `uv tool upgrade --all`。
10. `[global] managers_mode = "on"` 时，裸 `eget list` / `eget update --all` 默认带上**全部可用管理器**（等价 `--with-managers all`）；改回 `off`（或不配）后行为与改动前一致。
11. `eget config list` 显示 `managers` 段；`eget config get managers.npm.list_args` 有值；`managers_mode` 配成非法值时报错。
12. `go test ./...` 无新增失败；`internal/extpkg`、`internal/app`、`internal/config` 全绿。

## 涉及文件

| 动作 | 文件（相对路径） |
|---|---|
| 新增 | `internal/extpkg/{model,builtin,config,parse,exec,service}.go` + `*_test.go` + `testdata/` |
| 新增 | `internal/cli/managers_cmd.go`、`internal/cli/managers_handler.go` |
| 改 | `internal/config/{model,gookit,loader}.go` |
| 改 | `internal/app/{list,update,update_candidates,update_batch}.go` |
| 改 | `internal/cli/{wiring,service,list_cmd,list_handler,update_cmd,update_handler,app,handlers,config_handler}.go` |
| 改 | `internal/cli/render/render.go` |
| 改 | `README.md`、`README.zh-CN.md`、`docs/config.md`、`docs/config.zh-CN.md`、`AGENTS.md` |

## 验证

1. `go build ./...` 与 `go test ./...`。
2. 本机 end-to-end（`go build -o eget-dev.exe ./cmd/eget`）：
   - `eget-dev.exe list`（应无外部包，与改动前一致）
   - `eget-dev.exe list --with-managers npm,bun` / `--managers all` / `--managers npm --all`（报错）
   - `eget-dev.exe list --outdated --with-managers npm`
   - `eget-dev.exe update --check --with-managers npm`
   - `eget-dev.exe update --managers npm` / `update npm:agent-browser` / `update agent-browser`（后者应报未找到）
   - `eget-dev.exe managers list` / `managers upgrade uv`
   - `eget-dev.exe config list` 与 `config get managers.npm.list_args`
3. 完成后删除临时构建的 `eget-dev.exe`，不留后台进程。
