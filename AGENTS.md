# AGETNS for AI

**重要** 优先使用中文回复和响应以及编写方案/计划文档，除非明确要求使用英文

- 如果当前任务有进度跟进文件 or 计划文件，需要在完成后更新 checkbox 为已完成
- 多阶段任务，需要在每个子阶段完成后更新进度并提交 Git 提交
- 完成涉及 MVP 主链路的改动后，至少运行 `go test ./...` 再宣称完成。
- 如果有设计或开发方案，需要整理输出到 docs 目录下
- 当前正在进行 v0 版本开发，因此，只要确认进行调整的无需保留兼容处理
- 当前项目的文件路径输出，不要包含绝对路径，只需相对路径
- 没有明确指定时，如果当前功能改动涉及的逻辑文件超过3个或者超过100行代码，需要向用户确认后再实施

## 正在进行的工作

> **IMPORTANT**: 需要实时更新，正在进行的工作和关键md文档链接；**已完成的需要移除**，不能一直累计，总数不能超过10条。

<!-- PROCESSING WORKS:START -->

<!-- PROCESSING WORKS:END -->

## 核心原则

**权衡：** 这些指南倾向于谨慎而非速度。对于琐碎任务，使用判断。

### 1. 编码前思考

**不要假设。不要隐藏困惑。暴露权衡。**

实施前：

- 明确陈述你的假设。如有不确定，请提问。
- 如果存在多种解释，请呈现它们——不要沉默不语。
- 如果存在更简单的方法，请说明。在适当的时候提出反对意见。
- 如果某事不清楚，请停止。指出令人困惑的地方。提问。

### 2. 优先考虑简洁

**解决问题的最小代码量。不要做没有根据的猜测。**

- 不要超出所要求的功能范围。
- 不要为一次性使用的代码添加抽象。
- 不要添加未请求的"灵活性"或"可配置性"。
- 不要为不可能发生的场景添加错误处理。
- 如果你写了200行代码，但实际只需要50行，那就重写它。

问问自己："一个高级工程师会认为这太复杂了吗？" 如果会，那就简化。

### 3. 手术性修改

**只修改必须修改的部分。只清理你自己的烂摊子。**

在编辑现有代码时：

- 不要"改进"相邻的代码、注释或格式。
- 不要重构那些没有问题的东西。
- 保持现有的风格，即使你会有不同的做法。
- 如果你发现无关的废弃代码，请指出来——不要删除它。

当你的修改产生孤儿代码时：

- 删除你修改所导致未使用的导入/变量/函数。
- 除非被要求，否则不要删除现有的废弃代码。

测试：每行修改都应该直接追溯到用户请求。

### 4. 目标驱动执行

**定义成功标准。循环直到验证通过。**

将任务转化为可验证的目标：

- "添加验证" → "为无效输入编写测试，然后使其通过"
- "修复错误" → "编写一个可复现该错误的测试，然后使其通过"
- "重构 X" → "确保在重构前后测试都能通过"

对于多步骤任务，陈述一个简要计划：

```
1. [Step] → verify: [check]
2. [Step] → verify: [check]
3. [Step] → verify: [check]
```

强大的成功标准让你能够独立循环。薄弱的标准（"让它工作"）需要不断的澄清。

## 项目开发规范

- 使用 git worktree 进行开发时，在 `.worktrees` 目录下创建分支
- 一般代码自解释，但是关键的方法或逻辑点需要添加注释说明
- 如果需要通过命令行请求 github api, 优先检查 gh 命令是否可用，不可用再尝试其他方式
- 如果是修复 Github issue，需要在 commit message 中添加 issue number，例如 `fix #1234` `resolve #1234`
- 新增功能如果包含较多主体逻辑文件，应优先按功能在 `internal/app/<feature>` 下建子包，避免继续膨胀 `internal/app` 根目录

### Go单元测试编写

- 使用 `github.com/gookit/goutil/x/assert` 断言结果
- 同一个方法的多个用例使用 `t.Run()` 包裹

require 断言结果的写法：

```go
Require(t, assert.Eq(t, 1, res.ID))

// Standard assertion
assert.Eq(t, expected, actual)
```

<!-- gitnexus:start -->
# GitNexus — Code Intelligence

This project is indexed by GitNexus as **eget** (8175 symbols, 19065 relationships, 300 execution flows). Use the GitNexus MCP tools to understand code, assess impact, and navigate safely.

> If any GitNexus tool warns the index is stale, run `npx gitnexus analyze` in terminal first.

## Always Do

- **MUST run impact analysis before editing any symbol.** Before modifying a function, class, or method, run `gitnexus_impact({target: "symbolName", direction: "upstream"})` and report the blast radius (direct callers, affected processes, risk level) to the user.
- **MUST run `gitnexus_detect_changes()` before committing** to verify your changes only affect expected symbols and execution flows.
- **MUST warn the user** if impact analysis returns HIGH or CRITICAL risk before proceeding with edits.
- When exploring unfamiliar code, use `gitnexus_query({query: "concept"})` to find execution flows instead of grepping. It returns process-grouped results ranked by relevance.
- When you need full context on a specific symbol — callers, callees, which execution flows it participates in — use `gitnexus_context({name: "symbolName"})`.

## Never Do

- NEVER edit a function, class, or method without first running `gitnexus_impact` on it.
- NEVER ignore HIGH or CRITICAL risk warnings from impact analysis.
- NEVER rename symbols with find-and-replace — use `gitnexus_rename` which understands the call graph.
- NEVER commit changes without running `gitnexus_detect_changes()` to check affected scope.

## Resources

| Resource | Use for |
|----------|---------|
| `gitnexus://repo/eget/context` | Codebase overview, check index freshness |
| `gitnexus://repo/eget/clusters` | All functional areas |
| `gitnexus://repo/eget/processes` | All execution flows |
| `gitnexus://repo/eget/process/{name}` | Step-by-step execution trace |

## CLI

| Task | Read this skill file |
|------|---------------------|
| Understand architecture / "How does X work?" | `.claude/skills/gitnexus/gitnexus-exploring/SKILL.md` |
| Blast radius / "What breaks if I change X?" | `.claude/skills/gitnexus/gitnexus-impact-analysis/SKILL.md` |
| Trace bugs / "Why is X failing?" | `.claude/skills/gitnexus/gitnexus-debugging/SKILL.md` |
| Rename / extract / split / refactor | `.claude/skills/gitnexus/gitnexus-refactoring/SKILL.md` |
| Tools, resources, schema reference | `.claude/skills/gitnexus/gitnexus-guide/SKILL.md` |
| Index, status, clean, wiki CLI commands | `.claude/skills/gitnexus/gitnexus-cli/SKILL.md` |

<!-- gitnexus:end -->
