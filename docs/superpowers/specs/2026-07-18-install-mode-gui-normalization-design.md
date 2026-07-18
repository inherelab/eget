# Install Mode 与 GUI 选项归一化设计

## 背景

当前 `install` 命令分别绑定 `--gui` 和 `--install-mode`：

- `--gui` 只设置 `InstallOptions.GUI=true`，`InstallMode` 仍为空，后续继续依赖资产文件名自动判断 portable/installer。
- `--install-mode=installer` 只设置 `InstallMode`，`GUI` 仍为 false，GUI 安装链路不会生效。
- `validateInstallMode` 只校验完整值 `portable`、`installer`，不做别名或大小写容错。

这使两个表达同一安装意图的选项可能产生互相矛盾的状态。

## 目标语义

```text
--gui
=> GUI=true
=> InstallMode=installer

--install-mode=p|port|portable
=> GUI=true
=> InstallMode=portable

--install-mode=i|ins|install|installer
=> GUI=true
=> InstallMode=installer
```

解析前后允许空白，匹配时不区分 ASCII 大小写，输出到 `InstallOptions` 的值始终规范为 `portable` 或 `installer`。

## 冲突优先级

显式 `--install-mode` 优先于 `--gui` 的默认值：

```text
--gui --install-mode=portable
=> GUI=true
=> InstallMode=portable
```

因此 `--gui` 只在 `InstallMode` 为空时补默认 `installer`，不会覆盖用户显式选择。

## 实现位置

规则只放在 CLI 边界，不修改 app/install/runner 的配置合并或执行逻辑：

1. 将 `validateInstallMode(string) error` 替换为 `normalizeInstallMode(string) (string, error)`。
2. `normalizeInstallMode` 负责 trim、大小写归一化、别名映射和非法值错误。
3. `newInstallCmd` 在构造 handler snapshot 前调用归一化函数。
4. 规范 mode 非空时设置 `GUI=true`。
5. mode 为空且 `GUI=true` 时设置 `InstallMode=installer`。
6. 保留用户已添加的 `imode` flag alias。

不增加新的配置字段、依赖或下游兼容分支。

## 错误处理

空值合法；未知值继续阻止 handler 执行，错误明确列出规范值和支持的别名。示例：

```text
invalid install mode "silent": use portable (p, port) or installer (i, ins, install)
```

## 测试

在 `internal/cli/app_install_test.go` 使用表驱动用例覆盖：

- 仅 `--gui` 得到 installer。
- `p`、`port`、`portable` 得到 portable 且 GUI=true。
- `i`、`ins`、`install`、`installer` 得到 installer 且 GUI=true。
- 大小写和前后空白得到规范值。
- `--gui --install-mode=portable` 保留 portable。
- 非法值阻止 handler。
- 命令 reset 后 GUI/InstallMode 不残留。

验证命令：

```text
go test ./internal/cli -run 'TestMain_Install.*Mode'
go test ./internal/cli
go test ./...
```
