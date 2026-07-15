# 配置导入导出设计

## 背景

`eget config` 已支持初始化、查看、诊断、路径查询和单字段读写，但缺少跨机器迁移入口。用户只能手工定位并复制配置文件，新机器初始化成本较高。

现有配置层已经具备 TOML 加载、保存和序列化能力。本功能只补充 CLI 与应用层编排，不新增配置格式，也不引入新的归档格式。

## 目标

新增以下命令：

```text
eget config export [FILE]
eget config export --with-global [FILE]
eget config import FILE
eget config import --force FILE
```

默认导出适合在机器之间共享的配置；只有显式传入 `--with-global` 才导出机器相关的 `[global]` 配置。

## 非目标

- 首版不支持逐字段或逐 package 合并。
- 首版不支持从 URL 导入。
- 首版不支持 ZIP、加密包或多文件配置包。
- 首版不提供自动 secret 脱敏；默认通过排除 `[global]` 避免导出其中的 token、路径和代理设置。
- 首版不保留原配置文件的注释、字段顺序和排版。

## 导出语义

### 默认导出

```text
eget config export [FILE]
```

导出当前有效配置，但清空整个 `Global` section。导出内容包括现有的其它顶层 section，例如：

```text
api_cache
http_proxy
ghproxy
cache_mirror
repos
packages
pkg_templates
sdk
```

默认不导出：

```text
global
```

这样可避免把当前机器的安装目录、缓存目录、SDK 目录、代理、GitHub token 和系统选择带到新机器。

如果用户在 package/repo 等非 global section 中直接配置敏感字段，这些字段仍会随对应 section 导出。首版不做跨 section 的隐式脱敏。

### 完整导出

```text
eget config export --with-global [FILE]
```

完整导出当前有效配置，包括 `[global]`。

### 输出目标

- 指定 `FILE`：将 TOML 写入该文件。
- 省略 `FILE`：将纯 TOML 写入 stdout，stdout 不混入成功提示，方便重定向。
- 人类可读的成功提示只在写入文件时输出。

序列化复用现有 config dump 能力，避免维护第二套 TOML 编码逻辑。

## 导入语义

### 基本流程

```text
eget config import FILE
```

导入必须按以下顺序执行：

1. 读取源文件。
2. 完整解析为现有 `config.File`，任何语法或字段类型错误都立即返回。
3. 读取目标机器当前配置；目标配置不存在时使用空配置。
4. 判断导入文件是否实际包含 `[global]`。
5. 导入文件不包含 `[global]` 时，保留目标机器当前的 `Global`。
6. 导入文件包含 `[global]` 时，使用导入文件的 `Global`。
7. 其它顶层 section 以导入文件为准，整体替换，不逐项合并。
8. 通过同目录临时文件写入并原子替换目标配置，避免失败时留下半文件。

### 覆盖确认

- 目标配置已存在时默认要求确认。
- `--force` 跳过确认，适合自动化和无人值守初始化。
- 用户取消时不修改任何文件。
- 源文件与目标文件是同一路径时拒绝执行，避免无意义覆盖。

### Global 存在性

不能只通过解析后的字段是否为空判断 `[global]` 是否存在，因为显式空 section 与缺失 section 在 Go struct 中可能相同。导入流程需要从 TOML 文档或 config manager 中记录顶层 `global` key 是否存在。

## CLI 设计

扩展现有 `config` 子命令：

```text
config
├── init
├── list
├── doctor
├── path
├── get
├── set
├── export [FILE] [--with-global]
└── import FILE [--force]
```

`ConfigOptions` 增加最少字段：

```text
File
WithGlobal
Force
```

CLI 只负责参数绑定和校验；读取、合并、验证及原子写入放在 `app.ConfigService`，避免把文件语义塞进 handler。

## 错误处理

- 源配置不存在：明确返回文件不存在。
- TOML 无法解析：返回源文件路径和原始解析错误。
- 目标目录不可写：在替换前返回，不删除旧配置。
- stdout 导出失败：原样返回 writer 错误。
- 原子替换失败：清理临时文件，保留旧配置。

## 测试范围

至少覆盖：

1. 默认导出不包含 `[global]`，其它 section 保留。
2. `--with-global` 导出包含 `[global]`。
3. 导出到 stdout 只有 TOML。
4. 导入无 global 的配置时保留目标 global。
5. 导入含 global 的配置时替换目标 global。
6. 其它顶层 section 整体替换，不混入目标旧值。
7. 非法 TOML 不修改目标配置。
8. 已有配置的确认、取消和 `--force`。
9. CLI `config` / `cfg` 路由和未知参数校验。

## 成功标准

- 新机器可以通过一次 export/import 获得 packages、templates、cache mirror 等共享配置。
- 默认迁移不会覆盖新机器当前的 `[global]`。
- 完整迁移可通过 `--with-global` 明确执行。
- 导入失败不会破坏现有配置。
- 不新增配置文件格式或第三方依赖。
