# eget 匹配与转换规则整合 实施计划 评审轮次记录

> 汇总产物。权威结构化记录见同目录 `review-ledger.json`；逐轮报告以会话内独立子代理报告形式产出，未逐个落盘。
> 评审对象：`docs/plans/2026-10-17-rule-consolidation-plan.md`；Git root `inhere-tools/eget`；Delivery Track=Full、暴露度=高。

## 轮次摘要

| 轮 | 候选 commit | revision | 轴 | 结论 | open CORE_BLOCKING |
|---|---|---|---|---|---|
| 1 | `3ec68c9` | 0.1 | Standards/Governance + Spec/Executability | 两轴 BLOCKED | S-01；E-01/E-02/E-03 |
| 2 | `f239389` | 0.2 | 两轴 | Standards PASS / Executability BLOCKED | E2-01~E2-05（S-04 亦未闭合） |
| 3 | `8f95fab` | 0.3 | 合并轴 | BLOCKED | R3-01、R3-02 |
| 4 | `1c16917` | 0.4 | 合并轴 | BLOCKED | R4-01 |
| 5 | `c29ac3e` | 0.5 | 合并轴 | BLOCKED | R5-01、R5-02、R5-03 |
| — | `47876e4` | 0.6 | 未复核（预算用尽） | FIXED_NOT_REREVIEWED | 无（3 项修复未验证） |

## 第 1 轮（0.1）

- **S-01** Semantic Amendment 失败路径未返回 design review Gate。→ 0.2 改为「design/plan review + Human Approval Gate，先 design 修订再升 plan revision」。
- **S-02** 验证命令未声明运行目录。→ 0.2 在「任务」节声明 Git root，并逐条标注。
- **S-03** T03 站点计数 18 与实际列举 15 不一致。→ 0.2 改为 15 并按站点 id 列名。
- **S-04** 引用了未定义的 `ExtExact`。→ 0.3 改为 `KindExact`。
- **S-05** 可追溯性漏 T06、T13 owner 过泛。→ 0.2 补两行并列出 T13 目录 owner。
- **S-06** ledger `human_gate.pending` 与用户批准证据冲突。→ 0.2 在「输入与批准证据」说明该 pending 为评审时点历史状态，已由用户批准取代。
- **E-01** F3 漂移处理是未获批准的设计决策。→ 0.2 明确「不补齐、不折叠」，归一路径写为「design 修订 + 用户批准」。
- **E-02** 合并 `requiresSystem7z` 与完整 SystemArchiver 判定会改变 `.7z/.rar/.msi/.cab/.iso` 行为。→ 0.2 拆开为两个独立判定。
- **E-03** 未定义 API（`ExtExact`/`KindExact`）与 S-A07 两阶段。→ 0.3 补齐。
- **E-04** characterization 覆盖与计数。→ 0.2/0.3 逐站点落到 owner 文件。
- **E-05** 「每集合 × 两轴」不可执行。→ 0.2 按消费者实际轴限定。
- **E-06** 测试文件未列入 owner、T10 未跑 detect。→ 0.2/0.3 补齐。
- **E-07** filter 调用适配不具体。→ 0.3~0.6 逐入口固定。
- **E-08** 运行目录与 grep 覆盖。→ 0.2 补齐。
- **E-09** 安全无冲突（INFO）。

## 第 2 轮（0.2）

- **E2-01 / S-04** S-A01 仍引用未定义 API。→ 0.3 改为 `KindExact`。
- **E2-02** `MultiExtExact` 返回格式与 sdk 无点协议冲突。→ 0.3 固定 `MultiExtFold`（原大小写切片）/`MultiExtExact`（字面量 + ok）与调用点 `TrimPrefix`。
- **E2-03** `filter.Parse` 未固定 trim 与错误文案。→ 0.3/0.4 逐入口固定；0.4 增加 `ParseRegex` 供 chooser 保持「先剥前缀再 trim」。
- **E2-04** T10 测试 owner 无法覆盖包私有函数。→ 0.3 按消费者包拆分 owner。
- **E2-05** platform / `ClassifyAsset` API 不足以唯一实现。→ 0.3 补齐 `OSSpec`/`ArchSpec`/两个索引与判定顺序。
- **S2-01** 单文件 checkout 做站点级回退不可靠。→ 0.3 改为按任务粒度提交 + `git revert`。

## 第 3 轮（0.3）

- **R3-01** 人工 Gate 仍写 Draft 0.2。→ 0.4 改为当前 revision 并写明三元组来源。
- **R3-02** 四调用点 trim 责任未完全固定（`finder.go:109` 遗漏）。→ 0.4 逐点固定，含 `:109` 的 trim 上提。

## 第 4 轮（0.4）

- **R4-01** `filter.Parse` 默认剥 `^` 会改变 GitHub tag 的 `^` 字面量语义。→ 0.5 取消默认剥离，导出独立 `filter.StripAnti` 由 asset/sdk 显式调用。

## 第 5 轮（0.5）

- **R5-01** `Spec` → `Regex` 映射未规定（`PRE:`/`SUF:` 会失去匹配能力）。→ 0.6 固定填充映射与 service 的 `assetFilter` 映射。
- **R5-02** sdk 的 `^PRE:` characterization 与现状/站点清单矛盾。→ 0.6 明确 sdk 仅 `REG:` 走正则，`PRE:`/`SUF:` 仍作子串，并修正表述。
- **R5-03** 错误文案的表达式来源未固定。→ 0.6 统一取 `*filter.RegexError.Expr` 并逐入口断言完整错误字符串。

## 预算与限制

- 严谨型预算：1 轮 discovery + 最多 4 轮 blocking closure。第 5 轮已用尽 closure 预算；0.6 的三项修复**未经第 6 轮独立复核**（`REVIEW_LIMIT_REACHED`）。
- 所有轮次均为静态阅读，未运行 `go build`/`go test`/validator；等价性不构成行为证明。
- 第 1、2 轮为两轴独立；第 3~5 轮为合并轴（单评审者同时覆盖两轴），已在 ledger 中如实标注。
