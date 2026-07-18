# Install Mode 与 GUI 选项归一化实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 `--gui` 与 `--install-mode` 在 CLI 边界形成一致状态，并支持 portable/installer 的便捷别名。

**Architecture:** 使用一个纯函数把用户输入归一化为 `portable`、`installer` 或空值；`newInstallCmd` 在调用 handler 前完成 mode/GUI 联动。下游 app、config、install runner 保持不变。

**Tech Stack:** Go 标准库、gookit/cflag、`github.com/gookit/goutil/x/assert`。

## Global Constraints

- 显式 `--install-mode` 优先；`--gui` 只在 mode 为空时补 `installer`。
- mode 非空时自动设置 `GUI=true`。
- 支持 `p/port/portable` 和 `i/ins/install/installer`，忽略前后空白及 ASCII 大小写。
- handler 只接收规范值 `portable`、`installer` 或空值。
- 保留用户已在 `internal/cli/install_cmd.go` 添加的 `imode` alias，并把该修改纳入最终代码提交。
- 不修改 app/config/install runner，不增加依赖或配置字段。
- 修改现有 symbol 前执行 GitNexus upstream impact；提交前执行 detect-changes。

---

### Task 1: 归一化 install mode 输入

**Files:**

- Modify: `internal/cli/options.go`
- Test: `internal/cli/options_test.go`

**Interfaces:**

- Produces: `normalizeInstallMode(value string) (string, error)`
- Consumes later: `newInstallCmd` 使用规范值更新 snapshot。

- [ ] **Step 1: 对 `validateInstallMode` 做 upstream impact analysis**

确认它只由 `newInstallCmd` 调用；若出现其他生产调用方，先重新评估是否应保留 wrapper。

- [ ] **Step 2: 写归一化失败测试**

在 `options_test.go` 增加表驱动测试：

```go
func TestNormalizeInstallMode(t *testing.T) {
	tests := []struct {
		input, want string
		wantErr     bool
	}{
		{"", "", false},
		{"p", install.InstallModePortable, false},
		{"port", install.InstallModePortable, false},
		{" portable ", install.InstallModePortable, false},
		{"P", install.InstallModePortable, false},
		{"i", install.InstallModeInstaller, false},
		{"ins", install.InstallModeInstaller, false},
		{"install", install.InstallModeInstaller, false},
		{" INSTALLER ", install.InstallModeInstaller, false},
		{"silent", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := normalizeInstallMode(tt.input)
			assert.Eq(t, tt.wantErr, err != nil)
			assert.Eq(t, tt.want, got)
		})
	}
}
```

- [ ] **Step 3: 运行测试确认 RED**

Run: `go test ./internal/cli -run TestNormalizeInstallMode`

Expected: FAIL，提示 `normalizeInstallMode` 未定义。

- [ ] **Step 4: 实现最小归一化函数**

用 `strings.TrimSpace` 和 `strings.ToLower` 实现：

```go
func normalizeInstallMode(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "":
		return "", nil
	case "p", "port", install.InstallModePortable:
		return install.InstallModePortable, nil
	case "i", "ins", "install", install.InstallModeInstaller:
		return install.InstallModeInstaller, nil
	default:
		return "", fmt.Errorf("invalid install mode %q: use portable (p, port) or installer (i, ins, install)", value)
	}
}
```

移除只做完整值校验的 `validateInstallMode`，不要保留重复入口。

- [ ] **Step 5: 验证 Task 1**

Run: `go test ./internal/cli -run TestNormalizeInstallMode`

Expected: PASS。

---

### Task 2: 在 install 命令快照中联动 GUI 与 mode

**Files:**

- Modify: `internal/cli/install_cmd.go`
- Test: `internal/cli/app_install_test.go`
- Modify: `docs/superpowers/plans/2026-07-18-install-mode-gui-normalization.md`
- Modify: `AGENTS.md`

**Interfaces:**

- Consumes: `normalizeInstallMode(string) (string, error)`
- Produces: handler 收到一致的 `InstallOptions.GUI` 与 `InstallOptions.InstallMode`。

- [ ] **Step 1: 对 `newInstallCmd` 做 upstream impact analysis**

报告其进入 `newApp/Main` 的 HIGH/CRITICAL 影响面；修改限定在 install command Func 的 snapshot 构造前。

- [ ] **Step 2: 写 CLI 联动失败测试**

在 `app_install_test.go` 增加 helper，通过 `newApp(...).RunWithArgs(...)` 捕获 handler options，并用表驱动覆盖：

```go
tests := []struct {
	name, mode, wantMode string
	gui, wantGUI         bool
}{
	{"gui defaults installer", "", install.InstallModeInstaller, true, true},
	{"portable alias enables gui", "p", install.InstallModePortable, false, true},
	{"installer alias enables gui", "ins", install.InstallModeInstaller, false, true},
	{"explicit portable beats gui default", "portable", install.InstallModePortable, true, true},
}
```

每个用例构造参数：仅在 `gui=true` 时加入 `--gui`，仅在 mode 非空时加入 `--install-mode <mode>`，最后加入 `owner/repo`；断言 handler 中的 `GUI` 和 `InstallMode`。

- [ ] **Step 3: 运行 CLI 测试确认 RED**

Run: `go test ./internal/cli -run 'TestMain_Install(GUI|Mode)'`

Expected: FAIL：仅 GUI 时 mode 为空，或仅 mode 时 GUI=false。

- [ ] **Step 4: 在 handler 边界规范化并联动**

将原校验替换为：

```go
		mode, err := normalizeInstallMode(opts.InstallMode)
		if err != nil {
			return err
		}
		if mode == "" && opts.GUI {
			mode = install.InstallModeInstaller
		}
		opts.InstallMode = mode
		if mode != "" {
			opts.GUI = true
		}
```

保留现有 `c.StrOpt(&opts.InstallMode, "install-mode", "imode", ...)`。

- [ ] **Step 5: 运行聚焦和 CLI 包测试**

Run: `go test ./internal/cli -run 'Test(NormalizeInstallMode|Main_Install.*Mode)'`

Run: `go test ./internal/cli`

Expected: 全部 PASS。

- [ ] **Step 6: 运行全量验证和 GitNexus**

Run: `go test -count=1 ./...`

Run: `git diff --check`

Run: `npx gitnexus detect-changes --repo eget --scope all`

Expected: 全量 PASS；影响仅限 install CLI options、命令绑定和对应测试。

- [ ] **Step 7: 更新进度并提交**

把本文实际执行步骤改为 `[x]`，从 `AGENTS.md` 移除本进行中事项，然后提交：

```bash
git add internal/cli/options.go internal/cli/options_test.go internal/cli/install_cmd.go internal/cli/app_install_test.go AGENTS.md docs/superpowers/plans/2026-07-18-install-mode-gui-normalization.md
git commit -m "fix: normalize gui install mode options"
```

提交前确认 staged diff 包含用户的 `imode` alias，且不包含其他无关修改。
