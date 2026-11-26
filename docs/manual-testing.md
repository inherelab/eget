# 手工复现与验证清单

用于两类问题，都不需要下载真实资产（典型情况：reporter 的包有 130MB 资产，只关心 eget 的**判定**而不是文件内容）：

1. `list` / `list --outdated` / `update` 对某个已安装包的版本判定（是否提示更新、来源 tag、时间列）
2. 解压 / `--to` / `--rename` / `--file` 等 CLI 行为

原则：**状态全部用隔离的配置目录构造，真实仓库只用来核对 API 返回**。所有产物放在 `tmp/` 下（已在 `.gitignore`）。

---

## 1. 隔离环境：`EGET_CONFIG_DIR`

配置与已安装记录的路径解析（`internal/config/paths.go`、`internal/installed/store.go`）：

| 项 | 查找顺序 |
| --- | --- |
| 配置文件 | `EGET_CONFIG`（文件路径）→ `EGET_CONFIG_DIR/eget.toml` → `~/.eget.toml` → `./eget.toml` → `<config dir>/eget.toml` |
| 已安装记录 | `~/.eget.installed.toml`（旧路径，**存在即优先**）→ `<config dir>/installed.toml` |
| `config dir` | `EGET_CONFIG_DIR` → `$XDG_CONFIG_HOME/eget` → `~/.config/eget` |

所以只要设了 `EGET_CONFIG_DIR`，且本机没有 `~/.eget.installed.toml`，就能完全隔离：

```powershell
$env:EGET_CONFIG_DIR = (Resolve-Path 'tmp/issue66/a').Path
```

```bash
export EGET_CONFIG_DIR="$PWD/tmp/issue66/a"
```

> 本机若已存在 `~/.eget.installed.toml`，它会被读取而不是隔离目录里的 `installed.toml`；先确认 `Test-Path ~/.eget.installed.toml` 为 False。

目录内容最少两个文件：

```
tmp/issue66/a/
  eget.toml        # 可以只有一行 [global]
  installed.toml   # 手工构造的安装记录
```

---

## 2. 构造 `installed.toml`

```toml
[installed."colitu/linux"]
repo = "colitu/linux"                 # 归一化后的 repo key
target = "colitu/linux"
tool = "colitu-vpn"
installed_at = 2026-11-13T10:00:00Z   # 首次安装时间（list 的回退值）
updated_at = 2026-11-22T09:00:00Z     # 最近更新时间（list 的 Update Time）
url = "https://github.com/colitu/linux/releases/download/v1.9.0/colitu-vpn-1.9.0-linux-x64.tar.gz"
asset = "colitu-vpn-1.9.0-linux-x64.tar.gz"
tag = "v1.9.0"                        # 安装时的 tag，outdated 判定拿它和最新 tag 比较
version = "v1.9.0"
extracted_files = ["~/.local/bin/colitu-vpn"]

[installed."colitu/linux".options]    # 安装时记录的命令行选项，update/list 会读取
prerelease = true                     # 决定最新版本检查是否包含 prerelease
system = "linux/amd64"
extract_file = "colitu-vpn"
output = "~/.local/bin"
```

判定相关的字段（其余只影响显示）：

| 字段 | 作用 |
| --- | --- |
| `tag` | `list --outdated` 的“已安装版本”，空值会直接报 `installed tag is empty` |
| `tag_policy` / `options.tag_policy` | `latest`：跟踪最新；`tag`：把 `tag` 当显式跟踪目标；`pattern`：`tag` 是 `PRE:`/`SUF:`/`REG:` 模式 |
| `options.prerelease` | 最新版本检查是否包含 prerelease |
| `asset` / `asset_id` / `asset_size` / `asset_digest` / `asset_updated_at` | 仅在显式 tag 跟踪时用于判断同 tag 资产是否被重传 |
| `installed_at` / `updated_at` | `list` 的“Update Time”取 `updated_at`，零值回退 `installed_at` |

版本判定的两条分支（`internal/app/list.go` + `internal/cli/wiring.go`）：

- `item.Tag == ""`（普通版本 tag 且非 `--track-tag`）→ `LatestReleaseInfo(repo, prerelease)`：
  - `prerelease=false`：`GET /repos/{repo}/releases/latest`；该仓库没有稳定版时 404 → **回退**到 `releases?per_page=30` 按 `published_at` 取最新
  - `prerelease=true`：直接 `releases?per_page=30` 按 `published_at` 取最新
- `item.Tag != ""`（`--track-tag` / tag 模式）→ `githubTagLatestInfo`：tag family 快路径 → `releases/tags/{tag}` + 资产比较
- 比较规则：`sameReleaseTag` 先归一化（去 `v` 前缀、忽略大小写），所以 `1.9.0` ≡ `v1.9.0`；仅当显式 tag 且资产元信息变化才继续报更新

---

## 3. 验证命令

```powershell
.\tmp\eget.exe list                      # 版本 / 来源 / Update Time
.\tmp\eget.exe list --outdated           # 是否提示更新
.\tmp\eget.exe -v list --outdated        # 打印实际请求的 URL 与缓存命中
```

注意：`-v` 是**全局**参数，必须放在子命令前；`eget list --outdated -v` 会报 `option provided but not defined: -v`。

`update` 与 `list --outdated` 共用同一个判定函数（`checkOutdatedItem`），所以 `list --outdated` 的结论就是 `update --all` 的结论；`update --dry-run` 目前未实现。

---

## 4. 用例矩阵（#66 实战，可直接复制）

| case | 记录 | 期望 |
| --- | --- | --- |
| A | `tag = "v1.9.0"` + `options.prerelease = true` | `list` 显示 `updated_at`；`--outdated` = 已是最新 |
| B | `tag = "v1.9.0"`，**去掉** `options.prerelease`（老版本 eget 写的记录） | 仓库全部是 prerelease 时曾报 `check_failed ... 404`；修复后回退到最新 prerelease，判定为已最新 |
| C | `tag = "v1.8.0"` + `prerelease = true` | 正确报 `v1.8.0 → v1.9.0`，Published At 为发布时间 |
| D | `tag = "v1.9.0"` + `options.tag = "v1.9.0"` + `tag_policy = "tag"` | 走显式 tag 分支（family 快路径），不误报 |
| E | `tag = "1.9.0"`（缺 `v`）+ `prerelease = true` | 修复前误报 `1.9.0 → v1.9.0`；修复后为已最新 |

生成 C/E 这类变体最快的方式是基于 A 复制目录再替换字符串（PowerShell）：

```powershell
Copy-Item -Recurse tmp/issue66/a tmp/issue66/c
(Get-Content tmp/issue66/c/installed.toml -Raw) -replace 'v1\.9\.0', 'v1.8.0' |
  Set-Content tmp/issue66/c/installed.toml -NoNewline
```

---

## 5. 用真实仓库核对（不下载资产）

```bash
gh release list -R colitu/linux --limit 5                       # 看是否全是 Pre-release
gh release view v1.9.0 -R colitu/linux --json tagName,isPrerelease,publishedAt,assets
gh api repos/colitu/linux/releases/latest                        # 全 prerelease 时为 404
```

已知事实：`colitu/linux` 的全部 release 都是 prerelease，`releases/latest` 恒 404 → 正好覆盖“无稳定版”路径。构造记录时资产名要从 `assets[].name` 原样抄（例如 `colitu-vpn-1.9.0-linux-x64.tar.gz`）。

只想验证**解析链路**、不想下载 130MB 时，用一个不存在的资产过滤让流程在下载前失败：

```powershell
.\tmp\eget.exe -v install --system linux/amd64 --asset 'no-such-asset' --to tmp/issue66/out colitu/linux
```

预期输出（关键三行）：

```
[verbose] github finder request: https://api.github.com/repos/colitu/linux/releases/latest   # 404 Not Found
[verbose] github finder: no stable latest release, using v1.9.0
[verbose] github finder request: https://api.github.com/repos/colitu/linux/releases/tags/v1.9.0  # 200 OK
❌ ERROR: asset `no-such-asset` not found
```

---

## 6. 本地归档 + 本地 HTTP：验证解压 / `--to` / `--rename`

不依赖任何外部仓库，覆盖“多文件归档”“单文件提取”“文件目标 vs 目录目标”等分支。

```powershell
# 1) 造归档（MobaXterm 结构：exe + 插件 + 文本）
New-Item -ItemType Directory -Force tmp/repro67 | Out-Null
Set-Content tmp/repro67/MobaXterm_Personal_26.6.exe 'exe'
Set-Content tmp/repro67/CygUtils.plugin 'plugin'
Set-Content tmp/repro67/readme.txt 'readme'
Compress-Archive -Path tmp/repro67/* -DestinationPath tmp/repro67.zip -Force

# 2) 本地 HTTP（用后台任务跑，工作目录必须是 tmp/）
python -m http.server 18067 --bind 127.0.0.1

# 3) 关键：访问 127.0.0.1 必须绕过代理
$env:NO_PROXY = '127.0.0.1,localhost'
```

```powershell
$zip = 'http://127.0.0.1:18067/repro67.zip'

# a) extract-all + rename glob
.\tmp\eget.exe download --extract-all --rename 'MobaXterm_Personal_*.exe=MobaXterm.exe' $zip

# b) extract-all + 文件目标（多文件 → 明确报错，不建同名目录）
.\tmp\eget.exe download --extract-all --to target.exe $zip

# c) 单文件提取 + 文件目标（必须是文件）
.\tmp\eget.exe download --file 'MobaXterm_Personal_*.exe' --to single.exe $zip

# d) 零命中的 rename 规则（应输出 warning）
.\tmp\eget.exe download --extract-all --rename 'nope*.dll=x.dll' $zip
```

坑位记录：

- 主机设置了 `http_proxy` 时，请求 `127.0.0.1` 会被代理转发并返回 502，必须 `NO_PROXY`（本次踩过）。
- 同一 URL 会命中包缓存（输出 ` - Using cached file repro67-...zip`），改了 zip 内容要用新文件名或 `eget cache clean --pkg --all`。
- API 结果也会缓存（`- Using api_cache file: api.github.com-repos-...json`），改了构造记录却看不到变化时先清：`eget cache clean --api --all`。
- `Compress-Archive` 会带上目录结构，验证“归档内路径匹配”时正好可用。

---

## 7. 收尾

- 后台 `python -m http.server` 记得结束（否则端口占用，下次启动静默失败，表现为连接被拒绝）。
- `tmp/` 全部内容不入库；需要长期保留的用例请把 `installed.toml` 片段直接贴进对应 issue 或本文档的矩阵里。
- 复现结论写回 issue 时，带上：`eget --version`、`eget -v list --outdated` 原始输出、以及构造用的 `tag` / `options` 两段。
