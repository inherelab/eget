# 配置导入导出实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 `eget config` 增加可迁移的 TOML 配置导出与导入能力，默认排除机器相关的 `[global]`，仅在显式指定 `--with-global` 时导出它，并安全替换目标配置。

**Architecture:** 配置包负责保留顶层 section 存在性、按选项序列化 TOML 以及原子保存；应用层负责加载当前/来源配置并应用 `[global]` 保留或替换规则；CLI 层只负责参数、覆盖确认与纯 TOML stdout。沿用现有 `File`、`ConfigService`、gcli 和 prompts，不新增依赖或通用迁移框架。

**Tech Stack:** Go、gcli、gookit/config、`github.com/gookit/goutil/x/assert`、Go 标准库文件 API、GitNexus。

---

## 文件结构

- 修改 `internal/config/model.go`：在非序列化元数据中记录来源是否显式包含 `[global]`。
- 修改 `internal/config/gookit.go`：解码时记录 section 存在性，并提供可省略 `[global]` 的 writer 序列化入口。
- 新建 `internal/config/atomic.go`：同目录临时文件写入及平台替换编排。
- 新建 `internal/config/replace_unix.go`：Unix 使用 `os.Rename` 原子覆盖。
- 新建 `internal/config/replace_windows.go`：Windows 使用 `windows.MoveFileEx` 原子覆盖。
- 修改 `internal/config/gookit_test.go`：覆盖 section 存在性、导出过滤和原子保存失败保护。
- 修改 `internal/app/config.go`：实现导出和导入业务语义。
- 修改 `internal/app/config_test.go`：覆盖默认导出、完整导出、保留/替换 global、非法来源和保存失败。
- 修改 `internal/cli/config_cmd.go`：注册 `export [FILE]`、`import FILE`、`--with-global`、`--force`。
- 修改 `internal/cli/config_handler.go`：连接 stdout/文件导出、覆盖确认和导入结果提示。
- 修改 `internal/cli/config_handler_test.go`：覆盖纯 TOML stdout、文件导出、确认取消与 force。
- 修改 `docs/config.md`、`docs/config.zh-CN.md`：记录命令和迁移语义。
- 修改 `AGENTS.md`：实时维护正在进行工作；功能完成后移除配置导入导出条目。

### Task 1: 配置序列化与原子保存基础能力

**Files:**
- Modify: `internal/config/model.go`
- Modify: `internal/config/gookit.go`
- Create: `internal/config/atomic.go`
- Create: `internal/config/replace_unix.go`
- Create: `internal/config/replace_windows.go`
- Test: `internal/config/gookit_test.go`

- [ ] **Step 1: 写 section 存在性和导出过滤失败测试**

在 `internal/config/gookit_test.go` 增加表驱动测试，使用项目断言库：

```go
func TestLoadFileTracksGlobalSection(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		want       bool
	}{
		{name: "missing", body: "[packages.fzf]\nrepo = 'junegunn/fzf'\n"},
		{name: "present", body: "[global]\n\n[packages.fzf]\nrepo = 'junegunn/fzf'\n", want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "eget.toml")
			assert.NoErr(t, os.WriteFile(path, []byte(tc.body), 0o644))
			cfg, err := LoadFile(path)
			assert.NoErr(t, err)
			assert.Eq(t, tc.want, cfg.Meta.HasGlobal)
		})
	}
}

func TestDumpExportOmitsGlobalByDefault(t *testing.T) {
	cfg := NewFile()
	cfg.Global.Target = util.StringPtr("/machine/bin")
	cfg.Packages["fzf"] = Section{Repo: util.StringPtr("junegunn/fzf")}
	var out bytes.Buffer
	assert.NoErr(t, DumpExport(&out, cfg, false))
	assert.NotContains(t, out.String(), "[global]")
	assert.Contains(t, out.String(), "[packages.fzf]")
	assert.NoErr(t, DumpExport(io.Discard, cfg, true))
}
```

- [ ] **Step 2: 运行测试确认 RED**

Run: `go test ./internal/config -run 'TestLoadFileTracksGlobalSection|TestDumpExportOmitsGlobalByDefault'`

Expected: FAIL，提示 `Meta.HasGlobal` 或 `DumpExport` 尚不存在。

- [ ] **Step 3: 最小实现 section 标记与导出 writer**

在 `File.Meta` 增加 `HasGlobal bool`；`decodeConfigFile` 使用 `cfg.Exists("global", false)` 设置它。新增：

```go
func DumpExport(out io.Writer, file *File, withGlobal bool) error {
	cfg := encodeConfigFile(file)
	if !withGlobal {
		data := cfg.Data()
		delete(data, "global")
		cfg.SetData(data)
	}
	_, err := cfg.DumpTo(out)
	return err
}
```

- [ ] **Step 4: 写原子保存失败不破坏旧文件的失败测试**

通过包内可替换的 `replaceConfigFile` 函数模拟最终替换失败，断言旧文件仍为 `old` 且临时文件已清理；成功用例保存后再 `LoadFile` 验证新内容可解析。

- [ ] **Step 5: 运行原子保存测试确认 RED**

Run: `go test ./internal/config -run TestSaveAtomic`

Expected: FAIL，提示 `SaveAtomic` 尚不存在。

- [ ] **Step 6: 实现同目录临时写入和平台替换**

新增接口：

```go
func SaveAtomic(path string, file *File) error
```

实现必须先 `MkdirAll`，再在目标目录 `CreateTemp`，关闭临时句柄后调用现有 `Save(tempPath, file)`，最后调用平台 `replaceConfigFile(tempPath, path)`；任何错误均删除临时文件。Unix 实现调用 `os.Rename`，Windows 实现调用 `windows.MoveFileEx` 并带 `MOVEFILE_REPLACE_EXISTING|MOVEFILE_WRITE_THROUGH`。

- [ ] **Step 7: 运行配置包测试确认 GREEN**

Run: `go test ./internal/config`

Expected: PASS。

- [ ] **Step 8: 更新计划并提交配置基础阶段**

Run: `npx gitnexus detect-changes --repo eget --scope all`

Expected: 仅影响配置加载、序列化、保存及对应测试。

```bash
git add internal/config docs/superpowers/plans/2026-07-15-config-import-export.md
git commit -m "feat: add config migration primitives"
```

### Task 2: 应用层导入导出语义

**Files:**
- Modify: `internal/app/config.go`
- Test: `internal/app/config_test.go`

- [ ] **Step 1: 写导出与导入规则失败测试**

在 `internal/app/config_test.go` 用 `t.Run()` 覆盖：

```go
func TestConfigExport(t *testing.T) {
	// default: output has packages but no [global]
	// with-global: output contains [global] and its target
}

func TestConfigImport(t *testing.T) {
	// source missing global: retain current Global, replace Packages as a whole
	// source has global: replace current Global and Packages as a whole
	// malformed source: return error and keep target bytes unchanged
}
```

测试使用真实临时 TOML 文件与 `cfgpkg.LoadFile`，不模拟解析器或保存器。

- [ ] **Step 2: 运行测试确认 RED**

Run: `go test ./internal/app -run 'TestConfigExport|TestConfigImport'`

Expected: FAIL，提示 `ConfigExport`、`ConfigImport` 尚不存在。

- [ ] **Step 3: 实现最小业务 API**

新增：

```go
func (s ConfigService) ConfigExport(out io.Writer, withGlobal bool) error
func (s ConfigService) ConfigImport(sourcePath string) (string, error)
```

`ConfigExport` 调用现有 `load()` 后转交 `cfgpkg.DumpExport`。`ConfigImport` 先完整 `cfgpkg.LoadFile(sourcePath)`；仅当 `incoming.Meta.HasGlobal == false` 时加载当前配置并复制 `current.Global`，随后调用 `cfgpkg.SaveAtomic(s.configPath(), incoming)`。来源包含 `[global]` 时不读取目标内容，所有其它顶层 section 按来源文件整体替换。

- [ ] **Step 4: 运行应用层测试确认 GREEN**

Run: `go test ./internal/app -run 'TestConfigExport|TestConfigImport|TestConfigListGetAndSet|TestConfigInit'`

Expected: PASS。

- [ ] **Step 5: 更新计划并提交应用层阶段**

Run: `npx gitnexus detect-changes --repo eget --scope all`

Expected: 新增配置迁移流程，不影响安装、更新等流程。

```bash
git add internal/app/config.go internal/app/config_test.go docs/superpowers/plans/2026-07-15-config-import-export.md
git commit -m "feat: implement config import and export"
```

### Task 3: CLI 命令、确认和输出

**Files:**
- Modify: `internal/cli/config_cmd.go`
- Modify: `internal/cli/config_handler.go`
- Test: `internal/cli/config_handler_test.go`

- [ ] **Step 1: 写命令绑定和 handler 失败测试**

新增测试验证：`export` 接受零或一个 FILE 且绑定 `--with-global`；`import` 必须有一个 FILE 且绑定 `--force`；无 FILE 导出时 stdout 只有可解析 TOML；FILE 导出写文件并只在终端输出成功提示；目标存在且未 `--force` 时拒绝确认会取消且文件不变；`--force` 不读取 stdin。

- [ ] **Step 2: 运行 CLI 测试确认 RED**

Run: `go test ./internal/cli -run 'TestConfig.*Export|TestConfig.*Import|TestHandleConfig.*Export|TestHandleConfig.*Import'`

Expected: FAIL，提示子命令、字段或 action 分支不存在。

- [ ] **Step 3: 注册命令与选项**

扩展 `ConfigOptions`：

```go
File       string
WithGlobal bool
Force      bool
```

新增 `newConfigExportCmd` 和 `newConfigImportCmd`，使用 `AddArg` 限制参数数量，并用 `BoolOpt` 注册 `--with-global`、`--force`；同步更新 config help 和 examples。

- [ ] **Step 4: 实现 handler 分支**

`export` 无 FILE 时直接 `ConfigExport(os.Stdout, opts.WithGlobal)`，不能调用 ccolor；有 FILE 时用 `os.Create`/关闭错误检查写出并显示成功提示。`import` 在目标存在且未 force 时复用确认提示，确认后调用 `ConfigImport`；成功提示输出目标路径。

- [ ] **Step 5: 运行 CLI 测试确认 GREEN**

Run: `go test ./internal/cli -run 'TestConfig|TestHandleConfig'`

Expected: PASS。

- [ ] **Step 6: 更新计划并提交 CLI 阶段**

Run: `npx gitnexus detect-changes --repo eget --scope all`

Expected: 仅新增 config 子命令路由和对应 handler 流程。

```bash
git add internal/cli/config_cmd.go internal/cli/config_handler.go internal/cli/config_handler_test.go docs/superpowers/plans/2026-07-15-config-import-export.md
git commit -m "feat: add config import and export commands"
```

### Task 4: 文档、全量验证与收尾

**Files:**
- Modify: `docs/config.md`
- Modify: `docs/config.zh-CN.md`
- Modify: `AGENTS.md`
- Modify: `docs/superpowers/plans/2026-07-15-config-import-export.md`

- [ ] **Step 1: 更新中英文配置文档**

记录以下命令和精确语义：

```text
eget config export [FILE]
eget config export --with-global [FILE]
eget config import FILE
eget config import --force FILE
```

说明 stdout 是纯 TOML；默认排除 `[global]`；导入不含 `[global]` 时保留目标 global、包含时替换；其它顶层 section 整体替换；导入会重新序列化并丢失原注释/排版。

- [ ] **Step 2: 运行格式化和聚焦测试**

Run: `gofmt -w internal/config/model.go internal/config/gookit.go internal/config/atomic.go internal/config/replace_unix.go internal/config/replace_windows.go internal/config/gookit_test.go internal/app/config.go internal/app/config_test.go internal/cli/config_cmd.go internal/cli/config_handler.go internal/cli/config_handler_test.go`

Run: `go test ./internal/config ./internal/app ./internal/cli`

Expected: PASS。

- [ ] **Step 3: 运行 MVP 全量验证**

Run: `go test ./...`

Expected: PASS，无失败 package。

- [ ] **Step 4: 计划自检与工作项清理**

逐项检查设计文档要求均有对应测试；在 PowerShell 运行 `$bad = @('TB'+'D', 'TO'+'DO', 'implement '+'later', '类似'+'上面', 'similar '+'to'); Select-String -Path docs/superpowers/plans/2026-07-15-config-import-export.md -Pattern $bad`，Expected: 无输出。确认方法名始终为 `DumpExport`、`SaveAtomic`、`ConfigExport`、`ConfigImport`。勾选全部步骤，并从 `AGENTS.md` 的正在进行工作中移除“配置导入导出设计与实施”，保留 Cache Server 设计项。

- [ ] **Step 5: 最终影响检查并提交文档收尾**

Run: `npx gitnexus detect-changes --repo eget --scope all`

Expected: 影响集中在配置管理执行流，无安装/下载核心流程的意外变更。

```bash
git add docs/config.md docs/config.zh-CN.md AGENTS.md docs/superpowers/plans/2026-07-15-config-import-export.md
git commit -m "docs: document config migration"
```

- [ ] **Step 6: 确认最终工作树**

Run: `git status --short`

Expected: 无输出。
