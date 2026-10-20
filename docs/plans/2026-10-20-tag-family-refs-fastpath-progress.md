# eget tag 家族 refs 快路径 — 执行进度（progress）

> plan: `docs/plans/2026-10-20-tag-family-refs-fastpath-plan.md`（Draft 0.1，candidate `49ef326c…`，已批准 2026-10-20）
> 基线: plan candidate commit `49ef326c`；执行结束 HEAD: `2462a95`

## COMPLETE（2026-10-20）

- 主旨：tag 模式过期检查增加 matching-refs 家族快路径；家族 max==已安装 tag ⇒ 一次轻量请求终判"未过期"，其余一律 fail-open 回落 release 扫描（安装语义未动）。
- 最终状态 COMPLETE：T01–T04 全部完成，build/vet/全量测试（25 包）绿。
- commit 区间 `3adc4e5..2462a95`（4 个原子提交：比较器/家族解析 → 检查接线 → per_page 翻页修正）+ 计划/设计文档 commit；未 push。
- T03 实测（api.github.com 经本机代理，trycua/cua 120 ref 家族）：
  - 快路径冷检 11.3s（2 个 matching-refs 请求，**零** release 历史请求），缓存命中 0.03s；
  - 过期回落路径冷检 65.6s，outdated 结果与发布时间显示和基线一致（v0.35.4 → v0.37.0）；
  - `--all` 检查无任何 `check_failed`。
- 自主决策（Operational Discovery 级）：T02 用例改放 `internal/cli/wiring_latestinfo_test.go` 重写与 `internal/app/list_installedtag_test.go` 新增；T03 前置手工将 installed tag 临时置 0.37.0 后还原（备份 `tmp/installed.toml.bak`）复验双路径；`matching-refs` 显式 `per_page=100` 翻页（家族 120 ref 需 2 页，非冗余探针）。
- 验收偏差披露：计划预估冷检 ~2s（1 请求），实测 11.3s——瓶颈为代理单请求 RTT（~5s/请求）而非请求数；相对 55–65s 基线现实提升 5–6×，缓存命中即瞬完。
- 安全核对（SR1301）：鉴权沿用既有 getter（含 token 回退）未触碰；URL 仅由 repo/prefix 拼接（原样进路径，仅空格剔除，无命令/查询注入面）；无秘密进日志；资源归属/入参校验：prefix 空值短路、refs 空响应短路。其余项不适用。
- 剩余工作：① 线上二进制 `D:\env\tools\eget.exe` 替换待用户授权；② 其它 monorepo 的 refs 行为观察；③ F6 版本解析与比较器的归并评估（维持 deferred）。

## 实施发现分类记录

| 发现 | 分类 | 处理 |
|---|---|---|
| `internal/app/list_installedtag_test.go` 为计划文件清单外新测试文件 | Operational Discovery | 记录后继续（owner 内、仅测试） |
| T02 既有用例曾出现两请求（page=2 探针）与"1 请求"断言不符 | Corrective | 修正用例断言（匹配-refs only、零 releases），recorded；聚焦校验通过 |
| `matching-refs` 显式 per_page=100 翻页修正 | Corrective（owner 内一致性，验收方向不变） | commit `2462a95`；focused 测试 + 真环境复验 |
