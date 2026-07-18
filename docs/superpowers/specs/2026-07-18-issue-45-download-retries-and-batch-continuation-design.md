# Issue #45 下载重试与批量安装容错设计

## 目标

分两个提交处理 [issue #45](https://github.com/inherelab/eget/issues/45)：先修复 `install --all` 中单个包失败会中断后续任务的问题，再增加 `--retries` 文件下载重试选项。

## 已确认语义

- `install --all` 无论串行还是并发，都应尝试处理全部配置包。
- 单个包失败不阻止其他包；命令结束时仍聚合错误并返回非零退出码。
- 成功项保留在结果中，失败项不生成空结果。
- `update --all` 已具备相同行为，不重复修改。
- `--retries N` 表示每个下载请求 URL 的总尝试次数，而不是首次请求后的额外次数。
- 默认值为 `1`；显式输入必须大于等于 `1`。
- 选项覆盖 `install`、`update`、`download`、`add`，并沿现有 options 链传到文件下载客户端。
- 只重试工具文件下载请求，不重试 GitHub 等 provider API 元数据请求。
- ghproxy 有多个候选地址时，每个候选地址最多尝试 `N` 次，失败后再走现有 fallback。
- 不增加退避、延迟、状态码策略、配置文件字段或外部依赖。

## 提交 1：批量安装继续执行

`InstallAllPackages` 的串行路径把包级错误收集起来并继续循环；全局配置加载或并发参数非法等启动前错误仍立即返回。

`installAllPackagesConcurrent` 不再因第一个包错误取消任务分发。worker 记录包级错误，所有任务结束后过滤成功结果并返回聚合错误。实现沿用 `UpdateCandidates` 已验证的成功标记和失败聚合模式，不抽取新的通用批处理框架。

测试分别覆盖 batch=1 和 batch>1：第一个包失败，后续包仍被调用；返回成功结果以及包含失败数量的错误。

## 提交 2：文件下载请求重试

在现有 install/client options 中增加 `Retries` 字段，并由四个 CLI 命令绑定 `--retries`。CLI 默认传入 `1`，非法值在开始网络请求前返回。

下载既通过 `client.GetWithOptions` 执行普通 GET，也通过 `client.requestWithOptions` 执行 Range、探测和断点续传请求，因此两个现有请求循环都在单个候选 URL 内增加 transport retry。`GetWithOptions` 使用已有 `isProviderMetadataRequest` 判定：provider API 元数据请求始终只尝试一次，其他文件下载请求使用 `Retries`。每次重试重新构造请求；只有 `httpDo` 返回错误时重试，已有 HTTP 响应按当前逻辑处理。

测试覆盖默认只请求一次、前两次连接错误后成功、耗尽次数后返回最后错误、非法 CLI 值，以及四个命令的参数传播。

## 风险与验证

GitNexus 显示 `InstallAllPackages` 和下载请求链路进入 CLI 主流程，风险为 HIGH/CRITICAL。修改只落在批量安装、CLI options 传播和下载请求边界；不修改 provider API、安装器或配置结构。

每个提交前运行聚焦测试和 GitNexus detect-changes；两个提交完成后运行 `go test -count=1 ./...`。提交信息分别引用 `#45`，仅第二个提交使用 `fix #45` 关闭 issue。

## 提交边界

1. `fix: continue install all after package failures (#45)`：包含本文档、进度链接、批量安装修复及测试。
2. `feat: add configurable download retries (fix #45)`：包含 CLI/option/client 实现、测试和进度收尾。
