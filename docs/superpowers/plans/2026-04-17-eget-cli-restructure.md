# Eget CLI Restructure Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 `eget` 从根目录平铺的单入口 CLI 重构为基于 `gookit/cflag/capp` 的显式子命令结构，代码收敛到 `internal/`，并落地 `install`、`download`、`add`、`update`、`config` 命令。

**Architecture:** 入口迁移到 `cmd/eget/main.go`，`internal/cli` 负责 `capp` 命令注册与参数绑定，`internal/app` 负责用例编排，`internal/config`/`internal/install`/`internal/source/github`/`internal/installed` 负责底层能力与数据模型。根命令不再兼容默认安装，所有命令统一采用 `eget <command> --options... arguments...`。

**Tech Stack:** Go 1.24, `github.com/gookit/goutil/cflag/capp`, `BurntSushi/toml`, `bubbletea`, `progressbar`

---

## File Map

### New files

- `cmd/eget/main.go`
- `internal/cli/app.go`
- `internal/cli/install_cmd.go`
- `internal/cli/download_cmd.go`
- `internal/cli/add_cmd.go`
- `internal/cli/update_cmd.go`
- `internal/cli/config_cmd.go`
- `internal/app/install.go`
- `internal/app/download.go`
- `internal/app/add.go`
- `internal/app/update.go`
- `internal/app/config.go`
- `internal/config/model.go`
- `internal/config/paths.go`
- `internal/config/loader.go`
- `internal/config/writer.go`
- `internal/config/merge.go`
- `internal/install/options.go`
- `internal/install/service.go`
- `internal/install/runner.go`
- `internal/source/github/finder.go`
- `internal/installed/store.go`
- `internal/installed/model.go`
- `internal/installed/upgrade.go`
- `internal/ui/select.go`
- `internal/cli/app_test.go`
- `internal/config/loader_test.go`
- `internal/config/merge_test.go`
- `internal/app/add_test.go`
- `internal/app/config_test.go`
- `internal/app/update_test.go`

### Existing files to modify

- `go.mod`
- `go.sum`
- `Makefile`
- `README.md`
- `DOCS.md`
- `man/eget.md`
- `test/test_eget.go`
- `tools/build-version.go` if main package path assumptions break

### Existing files to retire or migrate from

- `archive.go`
- `config.go`
- `detect.go`
- `dl.go`
- `eget.go`
- `extract.go`
- `find.go`
- `flags.go`
- `installed.go`
- `verify.go`
- `version.go`

这些文件的逻辑要逐步迁移到 `internal/`，最终不再作为主业务实现入口存在。

## Task 1: 引入 `capp` 与新入口骨架

**Files:**
- Create: `cmd/eget/main.go`
- Create: `internal/cli/app.go`
- Create: `internal/cli/install_cmd.go`
- Create: `internal/cli/download_cmd.go`
- Create: `internal/cli/add_cmd.go`
- Create: `internal/cli/update_cmd.go`
- Create: `internal/cli/config_cmd.go`
- Modify: `go.mod`
- Test: `internal/cli/app_test.go`

- [x] **Step 1: 写路由与命令注册失败测试**

在 `internal/cli/app_test.go` 写表驱动测试，覆盖：
- 未提供子命令时返回帮助/错误
- `install --tag nightly inhere/markview` 被识别为 install
- `install inhere/markview --tag nightly` 返回解析错误
- `config --info` 被识别为 config

测试骨架示例：

```go
func TestCommandSyntax(t *testing.T) {
    cases := []struct {
        name    string
        args    []string
        wantErr bool
    }{
        {"install standard order", []string{"install", "--tag", "nightly", "inhere/markview"}, false},
        {"install invalid order", []string{"install", "inhere/markview", "--tag", "nightly"}, true},
    }
}
```

- [x] **Step 2: 运行 CLI 测试确认当前失败**

Run: `go test ./internal/cli -run TestCommandSyntax -v`
Expected: FAIL，因为 `internal/cli` 和 `capp` 入口尚不存在

- [x] **Step 3: 添加 `gookit/cflag/capp` 依赖**

Run: `go get github.com/gookit/goutil/cflag github.com/gookit/goutil/cflag/capp`

并更新 `go.mod` / `go.sum`。

- [x] **Step 4: 实现最小 CLI 骨架**

在 `cmd/eget/main.go` 中调用：

```go
func main() {
    cli.Run(version.Version, version.BuildDate, version.CommitID)
}
```

在 `internal/cli/app.go` 中：
- 构造 `capp.App`
- 注册 `install`、`download`、`add`、`update`、`config`
- 将命令 handler 暂时指向返回 `errors.New("not implemented")` 的占位实现

- [x] **Step 5: 实现 `install` 命令的最小参数绑定**

在 `internal/cli/install_cmd.go` 中先支持：
- `--tag`
- `--system`
- `--to`
- `--file`
- `--asset`
- `--source`
- `--all`
- `--quiet`
- 一个位置参数 `target`

其他命令文件先建立最小命令壳子即可。

- [x] **Step 6: 运行 CLI 测试确认通过**

Run: `go test ./internal/cli -v`
Expected: PASS

- [ ] **Step 7: 提交**

```bash
git add go.mod go.sum cmd/eget/main.go internal/cli
git commit -m "refactor: add capp cli skeleton"
```

## Task 2: 提取安装参数模型与 GitHub/直链识别逻辑

**Files:**
- Create: `internal/install/options.go`
- Create: `internal/install/service.go`
- Create: `internal/source/github/finder.go`
- Modify: `archive.go`
- Modify: `detect.go`
- Modify: `dl.go`
- Modify: `extract.go`
- Modify: `find.go`
- Modify: `verify.go`
- Modify: `eget.go`
- Test: `internal/install/options_test.go`

- [ ] **Step 1: 写安装参数与 target 识别测试**

新建 `internal/install/options_test.go`，覆盖：
- repo target 识别
- GitHub URL 识别
- 直链 URL 识别
- 本地文件识别

测试示例：

```go
func TestDetectTargetKind(t *testing.T) {
    assert.Equal(t, TargetRepo, DetectTargetKind("inhere/markview"))
    assert.Equal(t, TargetGitHubURL, DetectTargetKind("https://github.com/inhere/markview"))
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/install -run TestDetectTargetKind -v`
Expected: FAIL

- [ ] **Step 3: 建立 `internal/install.Options` 与 target 识别函数**

从 `flags.go`、`eget.go` 提取核心字段到：

```go
type Options struct {
    Tag         string
    Prerelease  bool
    Source      bool
    Output      string
    System      string
    ExtractFile string
    All         bool
    Quiet       bool
    DownloadOnly bool
    UpgradeOnly bool
    Asset       []string
    Hash        bool
    Verify      string
    DisableSSL  bool
}
```

并实现：
- `IsURL`
- `IsGitHubURL`
- `IsLocalFile`
- `DetectTargetKind`

- [ ] **Step 4: 将查找/下载/检测/校验/解压逻辑迁出 `main` 流程**

建立 `internal/install/service.go` 和 `internal/source/github/finder.go` 包装旧逻辑，要求：
- 旧实现先通过适配层复用
- `main` 级流程不再是唯一承载者

- [ ] **Step 5: 运行安装层测试**

Run: `go test ./internal/install -v`
Expected: PASS

- [ ] **Step 6: 提交**

```bash
git add internal/install internal/source/github
git commit -m "refactor: extract install option model"
```

## Task 3: 提取配置路径、模型与兼容读取

**Files:**
- Create: `internal/config/model.go`
- Create: `internal/config/paths.go`
- Create: `internal/config/loader.go`
- Create: `internal/config/merge.go`
- Test: `internal/config/loader_test.go`
- Test: `internal/config/merge_test.go`
- Modify: `config.go`
- Modify: `home/home.go`

- [ ] **Step 1: 写配置路径与旧配置读取测试**

在 `internal/config/loader_test.go` 中覆盖：
- `EGET_CONFIG` 优先级
- `~/.eget.toml` 回退
- XDG/LocalAppData 路径回退
- 旧 `global` 和 `["owner/repo"]` 读取成功

在 `internal/config/merge_test.go` 中覆盖：
- `global`
- repo section
- CLI 选项覆盖 repo/global

- [ ] **Step 2: 运行配置测试确认失败**

Run: `go test ./internal/config -v`
Expected: FAIL

- [ ] **Step 3: 建立新配置模型**

定义：

```go
type File struct {
    Global   GlobalConfig
    Repos    map[string]RepoConfig
    Packages map[string]PackageConfig
}
```

要求：
- `Repos` 继续映射 `["owner/repo"]`
- `Packages` 映射 `[packages.<name>]`
- 首版继续兼容没有 `packages` 的旧配置

- [ ] **Step 4: 实现路径解析与读取**

把 `config.go` 中的路径查找逻辑迁到 `internal/config/paths.go`、`loader.go`。

要求：
- 先只实现读取
- 保留现有环境变量和 fallback 行为

- [ ] **Step 5: 实现优先级合并函数**

在 `internal/config/merge.go` 中实现：
- `MergeInstallOptions(global, repo, pkg, cli)`
- 明确优先级：CLI > package > repo > global > default

- [ ] **Step 6: 运行配置测试确认通过**

Run: `go test ./internal/config -v`
Expected: PASS

- [ ] **Step 7: 提交**

```bash
git add internal/config
git commit -m "refactor: extract config loader and merge logic"
```

## Task 4: 建立安装记录存储层

**Files:**
- Create: `internal/installed/model.go`
- Create: `internal/installed/store.go`
- Modify: `installed.go`
- Test: `internal/installed/store_test.go`

- [ ] **Step 1: 写 installed store 读写测试**

覆盖：
- 空配置文件初始化
- 写入一条记录
- 删除一条记录
- 保留旧路径兼容逻辑

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/installed -run TestStore -v`
Expected: FAIL

- [ ] **Step 3: 提取 `InstalledEntry` 与路径/读写逻辑**

将 `installed.go` 中与 store 相关的能力迁入：
- `getInstalledConfigPath`
- `loadInstalledConfig`
- `saveInstalledConfig`
- `recordInstallation`
- `removeInstalled`

保留数据结构兼容。

- [ ] **Step 4: 运行 store 测试**

Run: `go test ./internal/installed -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/installed
git commit -m "refactor: extract installed state store"
```

## Task 5: 建立 app 层并接通 `install`/`download`

**Files:**
- Create: `internal/app/install.go`
- Create: `internal/app/download.go`
- Create: `internal/install/runner.go`
- Modify: `internal/cli/install_cmd.go`
- Modify: `internal/cli/download_cmd.go`
- Modify: `eget.go`
- Test: `internal/app/install_test.go`

- [ ] **Step 1: 写 install/download 用例测试**

对 `internal/app` 建 mockable seam 或最小 fake，覆盖：
- `InstallTarget` 调用完整安装流程并记录 installed 状态
- `DownloadTarget` 调用查找+下载，但不解压、不记录状态

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/app -run TestInstallTarget -v`
Expected: FAIL

- [ ] **Step 3: 实现 `InstallTarget` 用例**

要求：
- 输入 `target` + `install.Options`
- 调用 install runner
- 成功时记录 installed
- 返回可打印结果结构而不是直接 `os.Exit`

- [ ] **Step 4: 实现 `DownloadTarget` 用例**

要求：
- 与 install 复用查找/下载流程
- 不调用 extractor 落盘逻辑以外的“安装语义”
- 不写 installed store

- [ ] **Step 5: 让 `install` / `download` CLI 调用 app 层**

在 `internal/cli/install_cmd.go` 与 `download_cmd.go` 中调用 app service。

- [ ] **Step 6: 运行 app 测试**

Run: `go test ./internal/app -v`
Expected: PASS

- [ ] **Step 7: 提交**

```bash
git add internal/app internal/cli/internal/install
git commit -m "refactor: route install and download through app layer"
```

## Task 6: 增加 `packages` 模型与 `add` 命令

**Files:**
- Create: `internal/app/add.go`
- Create: `internal/app/add_test.go`
- Modify: `internal/config/model.go`
- Modify: `internal/config/writer.go`
- Modify: `internal/cli/add_cmd.go`

- [ ] **Step 1: 写 `add` 用例测试**

覆盖：
- `add junegunn/fzf` 生成 `packages.fzf`
- `add junegunn/fzf --name myfzf` 生成 `packages.myfzf`
- 保存 `target/system/file/asset_filters/tag/verify_sha256/download_source/disable_ssl/all`

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/app -run TestAddPackage -v`
Expected: FAIL

- [ ] **Step 3: 实现配置写回器**

在 `internal/config/writer.go` 中支持：
- 保留已有 `global`
- 保留已有 repo sections
- 新增或覆盖 `[packages.<name>]`

- [ ] **Step 4: 实现 `AddPackage` 用例**

规则：
- 默认 name 取 repo basename
- 支持 `--name`
- 不执行下载
- 只写可复用安装参数

- [ ] **Step 5: 让 `add` CLI 接入 app 层**

在 `internal/cli/add_cmd.go` 中绑定：
- `--name`
- 安装类共享选项
- 位置参数 `repo`

- [ ] **Step 6: 运行 `add` 测试**

Run: `go test ./internal/app -run TestAddPackage -v`
Expected: PASS

- [ ] **Step 7: 提交**

```bash
git add internal/app/add.go internal/app/add_test.go internal/config internal/cli/add_cmd.go
git commit -m "feat: add managed package command"
```

## Task 7: 落地 `config` 命令集

**Files:**
- Create: `internal/app/config.go`
- Create: `internal/app/config_test.go`
- Modify: `internal/cli/config_cmd.go`
- Modify: `internal/config/writer.go`
- Modify: `internal/config/loader.go`

- [ ] **Step 1: 写 config 用例测试**

覆盖：
- `ConfigInfo` 返回生效路径和存在状态
- `ConfigInit` 生成模板文件
- `ConfigList` 返回 global/repos/packages
- `ConfigGet("global.target")`
- `ConfigSet("global.target", "~/.local/bin")`

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/app -run TestConfig -v`
Expected: FAIL

- [ ] **Step 3: 实现 `ConfigInfo` / `ConfigInit`**

模板至少包含：

```toml
[global]
target = ""
system = ""

[packages]
```

避免覆盖已存在文件。

- [ ] **Step 4: 实现 `ConfigList` / `ConfigGet` / `ConfigSet`**

要求：
- 首版支持点路径，如 `global.target`、`packages.fzf.repo`
- 对不存在路径返回明确错误

- [ ] **Step 5: 接通 `config` CLI**

在 `internal/cli/config_cmd.go` 中绑定：
- `--info`
- `--init`
- `--list`
- `get <key>`
- `set <key> <value>`

- [ ] **Step 6: 运行 config 测试**

Run: `go test ./internal/app -run TestConfig -v`
Expected: PASS

- [ ] **Step 7: 提交**

```bash
git add internal/app/config.go internal/app/config_test.go internal/cli/config_cmd.go internal/config
git commit -m "feat: add config command set"
```

## Task 8: 落地 `update` 单项与 `--all`

**Files:**
- Create: `internal/app/update.go`
- Create: `internal/app/update_test.go`
- Create: `internal/installed/upgrade.go`
- Modify: `internal/cli/update_cmd.go`
- Modify: `internal/installed/store.go`
- Modify: `installed.go`

- [ ] **Step 1: 写 update 用例测试**

覆盖：
- `update fzf` 从 `packages.fzf` 解析 repo 和安装参数
- `update junegunn/fzf` 允许直接按 repo 更新
- `update --all` 遍历全部 package 条目
- `--dry-run` 不写文件

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/app -run TestUpdatePackage -v`
Expected: FAIL

- [ ] **Step 3: 提取升级检查逻辑**

从 `installed.go` 中迁出升级相关能力到 `internal/installed/upgrade.go`：
- 候选构造
- 版本/时间检查
- dry-run 结果汇总

- [ ] **Step 4: 实现 `UpdatePackage`**

要求：
- 先查 `packages.<name>`
- 如果未命中且入参含 `/`，按 repo 直接更新
- 将最终解析结果转给 `InstallTarget`

- [ ] **Step 5: 实现 `UpdateAllPackages`**

要求：
- 遍历全部 managed packages
- 支持 `--dry-run`
- 保留 `--interactive` 钩子；若首版不完整，实现为显式 TODO-free 最小行为或清晰报错，不留半成品占位

- [ ] **Step 6: 接通 `update` CLI**

绑定：
- `--all`
- `--dry-run`
- `--interactive`
- 可选目标参数

- [ ] **Step 7: 运行 update 测试**

Run: `go test ./internal/app -run TestUpdatePackage -v`
Expected: PASS

- [ ] **Step 8: 提交**

```bash
git add internal/app/update.go internal/app/update_test.go internal/installed/upgrade.go internal/cli/update_cmd.go
git commit -m "feat: add managed package update workflows"
```

## Task 9: 迁移版本注入、构建入口与 Makefile

**Files:**
- Modify: `Makefile`
- Modify: `version.go`
- Create: `internal/version/version.go`
- Modify: `cmd/eget/main.go`
- Test: `go test ./...`

- [ ] **Step 1: 写构建入口校验步骤**

明确新构建命令必须产出 `cmd/eget` 入口二进制，并保持版本信息可注入。

- [ ] **Step 2: 调整版本信息承载位置**

将当前 `main.Version` 风格迁移为独立包，例如：

```go
package version

var Version = "dev"
var BuildDate = ""
var CommitID = ""
```

- [ ] **Step 3: 更新 `Makefile`**

将以下命令调整为新入口：
- `build`
- `build-dist`
- `install`
- `eget`
- `test`

例如：

```bash
go build -trimpath -ldflags "-s -w $(GOVARS)" -o eget ./cmd/eget
```

- [ ] **Step 4: 运行全量测试**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 5: 运行构建**

Run: `make build`
Expected: 成功生成新入口二进制

- [ ] **Step 6: 提交**

```bash
git add Makefile cmd/eget internal/version
git commit -m "build: point tooling at new command entry"
```

## Task 10: 迁移集成测试到显式子命令语法

**Files:**
- Modify: `test/test_eget.go`
- Modify: `test/eget.toml`
- Test: `make test`

- [ ] **Step 1: 改写现有集成测试命令**

将旧命令：

```go
run(eget, "--system", "linux/amd64", "jgm/pandoc")
```

改为：

```go
run(eget, "install", "--system", "linux/amd64", "jgm/pandoc")
```

并增加最少一条：
- `download`
- `config --init`

- [ ] **Step 2: 运行集成测试**

Run: `make test`
Expected: PASS

- [ ] **Step 3: 提交**

```bash
git add test/test_eget.go test/eget.toml
git commit -m "test: migrate integration flows to explicit commands"
```

## Task 11: 更新文档并移除旧入口叙述

**Files:**
- Modify: `README.md`
- Modify: `DOCS.md`
- Modify: `man/eget.md`

- [ ] **Step 1: 更新 README 用法示例**

将所有示例改为显式子命令风格，例如：

```bash
eget install --tag nightly inhere/markview
eget download --file go --to ~/go1.17.5 https://go.dev/dl/go1.17.5.linux-amd64.tar.gz
eget add --name fzf --to ~/.local/bin junegunn/fzf
eget update --all
eget config --info
```

- [ ] **Step 2: 更新帮助与 man page 文档**

确保文档明确写出：
- 不再支持 `eget owner/repo`
- 命令统一为 `eget <command> --options... arguments...`

- [ ] **Step 3: 运行基础校验**

Run: `rg -n "eget [A-Za-z0-9_./:-]+/[A-Za-z0-9_.-]+( |$)" README.md DOCS.md man/eget.md`
Expected: 不再出现旧单入口示例，或只在迁移说明中出现

- [ ] **Step 4: 提交**

```bash
git add README.md DOCS.md man/eget.md
git commit -m "docs: document explicit subcommand cli"
```

## Task 12: 清理根目录旧实现并完成最终验证

**Files:**
- Modify: `archive.go`
- Modify: `config.go`
- Modify: `detect.go`
- Modify: `dl.go`
- Modify: `eget.go`
- Modify: `extract.go`
- Modify: `find.go`
- Modify: `flags.go`
- Modify: `installed.go`
- Modify: `verify.go`
- Modify: `version.go`
- Test: `go test ./...`
- Test: `make test`

- [ ] **Step 1: 移除旧 `main` 入口与未使用路径**

要求：
- 根目录不再持有主业务入口
- 未被新架构使用的旧函数移除或迁入对应 `internal/` 包
- 不保留双份逻辑

- [ ] **Step 2: 运行 `go test ./...`**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 3: 运行 `make test`**

Run: `make test`
Expected: PASS

- [ ] **Step 4: 手动冒烟验证**

Run:

```bash
./eget install --help
./eget config --info
./eget install --system linux/amd64 jgm/pandoc
```

Expected:
- 帮助输出显示子命令
- config 信息可显示配置路径
- install 命令成功运行

- [ ] **Step 5: 提交**

```bash
git add .
git commit -m "refactor: complete eget cli restructuring"
```

## Notes For Execution

- 每个任务完成后都先运行对应测试，再提交
- 不要在未建立测试或命令壳子的情况下直接大规模搬文件
- `update --interactive` 如果首版无法完整复用现有 bubbletea 逻辑，优先保证 `update` 和 `update --all` 稳定可用，再以清晰错误替代半成品行为
- `config` 写回必须避免抹掉旧 repo section 和 global 配置
- 文档更新必须与最终 CLI 语法一致，避免 README 继续展示旧 `eget owner/repo` 示例
