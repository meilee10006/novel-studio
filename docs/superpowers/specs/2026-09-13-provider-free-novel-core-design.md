# Provider-free Novel Core 设计规格

状态：待作者审核  
日期：2026-09-13  
目标分支：`provider-free-novel-core`  
上游基线：`Xiaoyangy/novel-studio@e3beebbf2f35b9ff55fd781d82055b60d45970c8`

## 1. 目标

把当前 novel-studio 改造成一个完全不调用 AI 的本地长篇小说状态引擎。

唯一负责创作、人物推演、审稿和改写的 AI 是普通 ChatGPT App。用户在正式连载阶段的正常操作只有一个：

> 继续

每次“继续”只执行当前唯一活动任务。正常情况下，一次“继续”完成一章；如果 Novel Core 判定当前章需要返工，下一次“继续”修同一章，不得跳到下一章。

系统不使用 OpenAI API、其他模型 API、Ollama、MCP、ChatGPT Work，也不通过浏览器或桌面 UI 自动点击 ChatGPT。

## 2. 产品边界

最终系统由四部分组成：

1. **ChatGPT App**：唯一智能层，负责想、写、推演、审稿、修改。
2. **Novel Core**：本地、无 AI、确定性，负责事实、状态、校验、提交、恢复和下一任务。
3. **Google Drive**：ChatGPT 与本地 Core 之间的交换层。
4. **Notion**：可选的人类可读投影，不参与 Canon 裁定，也不作为首发版本的发布门槛。

一句话定义：

> ChatGPT 负责想和写，Novel Core 负责记、查、验、锁和提交。

## 3. 不可破坏的原则

### 3.1 Canon 是唯一事实源

本地 Novel Core 中已经提交的 Canon 是唯一权威事实源。

以下内容都不是 Canon：

- ChatGPT 聊天历史；
- ChatGPT Memory；
- Google Drive 中未验收的文件；
- Notion 页面；
- 未验收的 `state_delta`；
- ChatGPT 自己声称“已经发生”的事情；
- 被 Core 拒绝的稿件；
- 人工直接修改的 published 副本。

只有 Core 成功提交后，状态变化才进入 Canon。

### 3.2 ChatGPT 不能直接改 Canon

ChatGPT 只能提交 Proposal：正文、章节合同、状态变化、审稿结果和必要的滚动规划。

Core 决定这些 Proposal 是否能够进入 Canon。

### 3.3 Core 不冒充文学审稿人

Core 只做能够确定性证明的检查，例如：

- 任务身份；
- digest；
- Canon 前态；
- 章节顺序；
- 人物知识来源；
- 时间和地点约束；
- 资源守恒；
- 锁定事实；
- 伏笔和冲突状态机；
- 本章硬合同。

Core 不判断“这一段够不够感人”“这个爽点够不够爽”“读者一定喜不喜欢”。这些属于 ChatGPT 自审和作者判断。

### 3.4 不伪造模型调用证据

新系统不得伪造：

- provider；
- model；
- token usage；
- tool call id；
- API response id。

新 provenance 只记录系统真实可证明的信息：task、attempt、Canon root、artifact digest、validation result、receipt 和提交时间。

### 3.5 Drive 是交换层，不是数据库

Google Drive 可能存在同步延迟、重复事件、部分文件先到、人工误改和离线状态，因此不得把 Drive 目录当作权威 Store。

本地 Canon 与 Drive workspace 必须分离。

## 4. 用户工作流

### 4.1 首次建书

作者与 ChatGPT 共同确定：

- 创作合同；
- 世界圣经；
- 人物圣经；
- 全书方向；
- 当前卷计划；
- 当前 Arc 计划；
- Ending Contract；
- 平台和文风约束。

冻结后的基础设定由 Core 保存为 Canon。

### 4.2 正常写作

作者输入：

> 继续

ChatGPT 执行当前 READY 指向的任务，在一次回答里完成：

1. 读取根协议和当前 Context Pack；
2. 检查当前卷、Arc 和本章义务；
3. 对活跃人物做独立视角推演；
4. 生成 Chapter Contract；
5. 决定本章冲突、伏笔和阅读回报；
6. 写正文；
7. 做连续性、人物、因果、节奏、语言和阅读体验自审；
8. 必要时在本轮内自行修改；
9. 生成证据化 State Delta；
10. 将完整 Submission 写入 Drive；
11. 最后写 manifest。

本地 Core 看到完整 Submission 后自动校验并产生结果。

### 4.3 ACCEPTED

Core：

- 写入 Accepted Chapter；
- 应用状态变化；
- 生成不可变 Receipt；
- 计算新 Canon Root；
- 写 Checkpoint；
- 推进 READY 到下一任务。

作者下一次继续即可。

### 4.4 REWRITE

Core 不推进章节，生成同一逻辑任务的新 attempt。

作者下一次说“继续”时，ChatGPT 必须修改当前章，不得写下一章。

### 4.5 BLOCKED

只有现有硬规则无法自行满足时才 BLOCK，例如：

- 两条锁定设定互相冲突；
- 作者新要求和已提交历史不可同时成立；
- Ending Contract 自相矛盾；
- 本地状态损坏且没有可靠恢复点。

BLOCKED 才要求作者做剧情级决定。

协议损坏、文件不同步、digest 不一致不属于剧情 BLOCKED；它们属于交换层状态，见第 13 节。

## 5. 系统架构

```text
作者
  │ “继续”
  ▼
普通 ChatGPT App
  │ Google Drive 连接能力
  ▼
Google Drive Cloud
  │
  │ Drive Desktop 同步
  ▼
Drive Workspace
  │
  ▼
Novel Core
  ├── Canon
  ├── State Machine
  ├── Context Compiler
  ├── Validators
  ├── Commit / Recovery
  ├── Retrieval
  └── Export
      │
      ▼
本地权威 Store
```

Notion 位于旁路，只消费投影，不进入上面的关键路径。

## 6. 本地目录与 Drive Workspace

### 6.1 本地权威目录

建议默认：

```text
~/.novel-core/projects/<project-id>/
```

保存：

- Canon；
- Accepted Chapters；
- Characters；
- World；
- Timeline；
- Relationships；
- Resources；
- Foreshadowing；
- Conflicts；
- Reader Promises；
- Planning；
- Checkpoints；
- Receipts；
- 索引和检索数据。

### 6.2 Drive Workspace

```text
NovelStudio/<project-id>/
├── project.json
├── CHATGPT_PROTOCOL.md
├── exchange/
├── published/
├── projection/
└── backup/
```

删除 Drive workspace 不得造成 Canon 丢失。

删除本地权威状态后，Core 不得把 Drive 副本悄悄当成 Canon。

## 7. 控制面、数据面与写入权

### 7.1 控制面

允许覆盖更新的文件只有少量活动指针：

- `project.json`
- `exchange/READY.json`
- `exchange/STATUS.json`

它们回答“当前该干什么”，不保存唯一历史事实。

### 7.2 数据面

任务、提交和回执一旦生成即不可变：

```text
exchange/outbox/<task-id>/<attempt-id>/
exchange/inbox/<task-id>/<attempt-id>/
exchange/result/<task-id>/<attempt-id>.json
receipts/
accepted/
```

重写必须产生新 attempt，不得覆盖旧 attempt。

### 7.3 唯一写入方

每个区域必须只有一个正式写入方：

| 区域 | 正式写入方 | 另一方权限 |
|---|---|---|
| `project.json` | Core | ChatGPT 只读 |
| `CHATGPT_PROTOCOL.md` | Core | ChatGPT 只读 |
| `exchange/READY.json` | Core | ChatGPT 只读 |
| `exchange/STATUS.json` | Core | ChatGPT 只读 |
| `exchange/outbox/**` | Core | ChatGPT 只读 |
| `exchange/inbox/**` | ChatGPT | Core 只读并处理 |
| `exchange/result/**` | Core | ChatGPT 只读 |
| `published/**` | Core | ChatGPT 只读 |
| `projection/**` | Core | ChatGPT / Notion 只读消费 |
| `backup/**` | Core | ChatGPT 不参与写入 |

ChatGPT 不得编辑 READY、STATUS、outbox、result、published 或 backup。

Core 不得替 ChatGPT 修改已经落盘的 inbox submission；只能接受、拒绝或生成新 attempt。

## 8. Production Task 与 Attempt

新系统只保留一个高层任务概念：`Production Task`。

首发版本允许的 task kind：

- `chapter`
- `rewrite`
- `revision`
- `finalize`

不把“人物推演”“伏笔决策”“风格审稿”拆成 Core 流程节点。它们是 ChatGPT 为完成一个任务所做的内部智能工作。

### 8.1 Task 与 Attempt 的区别

- `task_id` 标识一个逻辑工作项，例如“完成第 47 章”。
- `attempt_id` 标识这个逻辑工作项的一次不可变执行版本。
- REWRITE 保持同一个 `task_id`，递增或生成新的 `attempt_id`。
- 每个 attempt 都有自己的 Task Pack 和 `task_digest`。
- 只有 READY 指向的当前 attempt 有资格推进 Canon。

### 8.2 Task 身份

每个 attempt 至少包含：

- schema version；
- project id；
- task id；
- task kind；
- chapter；
- attempt id；
- base canon root；
- task digest；
- required artifacts；
- created at。

同一个项目任何时候只允许一个活动写 attempt。

## 9. READY

`exchange/READY.json` 是 ChatGPT 的唯一活动入口。

至少包含：

- project id；
- task id；
- task kind；
- chapter；
- attempt id；
- base canon root；
- task digest；
- status。

两个 ChatGPT 对话同时写同一个 attempt 时，只能有一个合法内容版本被处理。READY 切换后，旧 attempt 即失去提交资格。

## 10. Task Pack

每个 attempt 输出目录至少包含：

```text
exchange/outbox/<task-id>/<attempt-id>/
├── task.json
├── context.json
├── canon_excerpt.json
├── constraints.json
└── recent_prose.md
```

根目录 `CHATGPT_PROTOCOL.md` 是唯一长期协议；Task Pack 通过 `protocol_version` 引用它，不复制第二份完整协议，避免两套协议漂移。

Task Pack 必须让一个完全新的 ChatGPT 对话在读取根协议后理解本轮操作。

## 11. Submission

章节提交至少包含：

```text
exchange/inbox/<task-id>/<attempt-id>/
├── chapter.md
├── chapter_contract.json
├── state_delta.json
├── self_review.json
└── manifest.json
```

如本章同时需要滚动规划，可以额外包含 `next_arc_proposal.json`。

## 12. manifest-last 协议

ChatGPT 必须最后写 `manifest.json`。

manifest 声明：

- task id；
- attempt id；
- base canon root；
- task digest；
- artifact 列表；
- 每个 artifact 的 digest。

Core 规则：

1. 没有 manifest：视为同步尚未完成；
2. manifest 已出现但文件不齐：继续等待同步，不立即判坏稿；
3. 文件齐全后校验 digest；
4. 相同 attempt + 相同 digest：幂等重复；
5. 同一 attempt 出现第二套不同 digest：协议冲突，不进入剧情验证；
6. 已正式结算的 attempt 永远不重复 Commit。

## 13. 交换层状态与剧情结果必须分开

文件交换和剧情裁定不是同一个状态机。

### 13.1 Reconciliation Status

交换层至少区分：

- `PENDING`：文件尚未同步完整；
- `READY_TO_VALIDATE`：文件齐全且协议校验通过；
- `INVALID`：协议或 digest 冲突，不能进入剧情验证；
- `SETTLED`：attempt 已产生正式结果。

`INVALID` 是系统/协议问题，不等于剧情 BLOCKED，也不会推进 Canon。

### 13.2 Production Result

只有进入剧情验证的 Submission 才会得到：

- `ACCEPTED`
- `REWRITE`
- `BLOCKED`

这样不会把“Drive 少同步了一个文件”和“作者必须决定主角生死”混成同一种错误。

## 14. Canon 模型

Canon 由四部分组成：

1. 当前 Canonical State；
2. Accepted Artifacts；
3. Immutable Commit Receipts；
4. Checkpoints。

不引入完整 Event Sourcing 平台，也不新上数据库作为前置条件。

继续复用当前 novel-studio 的 Store 和 Checkpoint 思路。

### 14.1 Commit 顺序

一次合法提交逻辑上完成：

```text
Validated Submission
→ Apply State Delta
→ Save Accepted Chapter
→ Write Immutable Receipt
→ Calculate Canon Root
→ Write Checkpoint
→ Advance READY
```

要求：

> 要么整个提交可恢复为已完成，要么重启后仍停留在前一个 Canon；不能出现半章进入 Canon 的状态。

实现计划必须根据现有 Store 的落盘机制选择明确的 crash-safe 顺序和恢复标记，不能只靠进程内锁。

## 15. Receipt

Receipt 至少保存：

- schema；
- project id；
- task id；
- attempt id；
- previous canon root；
- task digest；
- submission digest；
- artifact digests；
- validation digest；
- result；
- new canon root；
- committed at。

`source=chatgpt_app` 只能作为描述性 metadata，不能当作模型调用证明。

## 16. Chapter Contract

正文生成前，ChatGPT 必须形成 Chapter Contract。

至少包含：

- chapter；
- POV；
- 起始状态；
- 本章目的；
- 读者当前问题；
- 主冲突；
- 次冲突；
- 必须推进事项；
- 必须承接事项；
- 禁止泄露信息；
- 可执行的伏笔操作；
- Reader Reward；
- 关键 causal beats；
- 预计状态变化；
- 章末钩子；
- 禁止发生事项；
- 目标长度区间。

Contract 不是 Canon，但它属于本章 Production Contract，Core 可以检查硬边界是否被破坏。

## 17. State Delta 必须证据化

ChatGPT 不重新提交完整世界状态，只提交本章变化。

所有影响未来写作的重要变化都必须能指回本章发生的事件或证据。

### 17.1 人物状态

包括：

- location change；
- physical state；
- goal change；
- pressure change；
- commitment change；
- status change。

### 17.2 人物知识

每个 `knowledge_add` 必须记录：

- character；
- fact id；
- source event / source fact；
- evidence。

### 17.3 关系

关系变化记录：

- participants；
- previous state；
- new state；
- evidence event。

不使用虚假的高精度数值，例如 `trust=72`。

优先使用定性状态，例如：

- hostile；
- distrust；
- guarded；
- cooperative；
- intimate。

同一关系允许双方有不同 perception。

### 17.4 资源

记录：

- resource id；
- owner；
- delta；
- reason；
- evidence。

### 17.5 伏笔、冲突、读者承诺

所有生命周期变化必须附 evidence event。

## 18. Story Event 与人物知识边界

关键剧情事实必须结构化为 Story Event。

每个 Event 至少包含：

- event id；
- time；
- location；
- actors；
- observers；
- public/private；
- consequences。

人物获得新知识必须能追到：

- 亲眼观察的事件；
- 合法传播事件；
- 已有明确证据。

没有来源的知识变化必须 REWRITE。

## 19. 人物 Observation

每个本章重要角色拥有自己的 Observation，不共享作者上帝视角。

至少包含：

- current location；
- current goal；
- pressure；
- known facts；
- forbidden / unknown facts；
- resources；
- relationships；
- commitments；
- current action；
- perception。

ChatGPT 必须先按人物可见信息推演，再汇总世界结果。

## 20. 时间、地点和资源

Core 确定性检查：

- 故事时间单调；
- 行程耗时可行；
- 同一个人在同一时间不能出现在冲突地点；
- 实体资源不能凭空出现；
- 金钱和消耗品不能无理由透支；
- 已锁定的权限、伤势和能力限制不能被绕过。

## 21. 伏笔、冲突和 Reader Promise

### 21.1 Foreshadow

推荐状态：

```text
planned
→ seeded
→ reinforced
→ payoff_ready
→ paid_off
→ closed
```

允许额外状态：`misdirected`、`retired`。

每项伏笔至少包含：

- 稳定 ID；
- 表层线索；
- 真义；
- 知情角色；
- 读者当前可能理解；
- 首次出现；
- 强化记录；
- 最早回收章；
- 最晚回收章；
- 假答案；
- 真回收；
- 依赖；
- 后果。

### 21.2 Conflict

必须有明确状态和关闭条件，避免旧冲突在无证据的情况下“自动消失”。

### 21.3 Reader Promise

记录作品向读者作出的跨章承诺，并允许：

- advanced；
- fulfilled；
- deferred（需理由）；
- retired（需作者级理由）。

## 22. Ending Contract

整书必须拥有 Ending Contract。

至少包含：

- 主线必须解决的问题；
- 主角人物弧终点；
- 重要人物弧终点；
- 核心关系终态；
- 核心命题；
- 必须回收伏笔；
- 必须关闭冲突；
- 允许开放的尾声；
- 禁止遗留的重大问题。

进入收束区后，Core 可以按照剩余篇幅禁止明显无法偿还的新长期剧情债务，例如：

- 新终极势力；
- 新世界底层规则；
- 新超长伏笔；
- 无预算的新主线。

存在未满足的硬 Ending Contract 时，不允许标记整书 Complete。

## 23. Context Compiler

Core 不把整本小说重新塞给 ChatGPT。

每个任务编译有限 Context Pack，包括：

### 23.1 全局相关约束

- Story Bible 相关切片；
- Ending Contract 相关条款；
- 用户硬规则；
- 文风和平台规则。

### 23.2 当前结构

- 当前 Volume；
- 当前 Arc；
- Arc 目标；
- 本章骨架；
- 未履行义务；
- Reader Promises。

### 23.3 活跃人物

只加载当前任务相关人物的 Observation 和必要历史。

### 23.4 世界状态

- 当前时间；
- 相关地点；
- 世界规则；
- 相关势力；
- 必要资源。

### 23.5 剧情债务

- 活跃冲突；
- 可强化伏笔；
- 可回收伏笔；
- 禁止提前回收伏笔；
- 即将逾期的承诺。

### 23.6 连续正文

- 最近若干章结构化摘要；
- 上一章必要尾部原文；
- 通过确定性检索命中的相关旧段落。

首发版本优先使用结构化 Canon + BM25/关键词/实体索引，不要求 embedding 和 Qdrant。

第 100 章的 Context Pack 不得随全文长度线性增长。

## 24. 滚动 Arc 规划

Arc 规划不额外消耗正常情况下的“继续”。

正式连载前至少拥有：

- Book Direction；
- Volume Plan；
- Current Arc Plan；
- Ending Contract。

当前 Arc 临近结束时，ChatGPT 可以在普通章节 Submission 中附带 `next_arc_proposal`。

Core 在接受章节时同时登记合法 Proposal。

只有 Proposal 自己违反硬约束时才生成 rewrite / revision 类 task 或 BLOCKED；正常成功路径仍然直接进入下一章节任务。

## 25. CHATGPT_PROTOCOL.md

项目根目录必须有稳定的 `CHATGPT_PROTOCOL.md`，由 Core 版本控制生成。

它定义：

- 如何定位 READY；
- 如何读 Task Pack；
- “继续”的语义；
- 如何完成章节内部智能工作；
- Submission 文件要求；
- manifest-last；
- ACCEPTED / REWRITE / BLOCKED 行为；
- 哪些 Drive 路径只读；
- 禁止直接修改 Canon；
- 新聊天如何恢复。

新 ChatGPT 对话只需用户输入：

> 继续《项目名》

即可从 Drive 中恢复生产状态，不依赖旧聊天记忆。

## 26. 三层验证

### L1 Protocol

检查：

- schema；
- task id；
- attempt id；
- manifest；
- artifact digest；
- task digest；
- base canon root；
- active attempt。

L1 失败进入交换层 `INVALID` 或继续保持 `PENDING`，不直接产生剧情 BLOCKED。

### L2 Canon

检查：

- 人物知识；
- 时间；
- 地点；
- 资源；
- immutable facts；
- relationship transition；
- foreshadow lifecycle；
- conflict lifecycle；
- Ending Contract 约束。

L2 可修问题通常产生 REWRITE。

### L3 Production Contract

检查：

- POV；
- 必须推进事项；
- 必须承接事项；
- 禁止事项；
- 章节身份；
- 目标长度；
- 允许的伏笔操作。

不得把主观文学质量塞进 L3。

## 27. 生产结果状态机

只有通过 L1 的 Submission 才能产生以下结果。

### ACCEPTED

合法提交，推进 Canon。

### REWRITE

同一逻辑任务可修复，Core 生成新 attempt。

REWRITE 必须包含：

- violation code；
- offending entity；
- expected；
- observed；
- evidence refs；
- allowed repair scope。

不得只返回“人物不合理，请修改”这类模糊意见。

### BLOCKED

只有无法在当前硬规则内安全自动处理时使用。

## 28. 幂等、并发和崩溃恢复

### 28.1 幂等

相同 submission 重放任意次数，只允许一次 Commit。

### 28.2 活动 attempt 与 stale submission

只有 READY 当前指向的 attempt 有资格进入生产验证。

旧 attempt 即使基于当前 Canon，也不能在 READY 已经切换后重新抢占提交权。

不同逻辑任务如果基于旧 Canon 提交，则按 stale submission 拒绝。

### 28.3 恢复依据

恢复只依赖：

- Canon root；
- Checkpoint；
- Receipt；
- active task / attempt；
- accepted artifacts。

不得依赖：

- 文件修改时间；
- 聊天历史；
- 内存 goroutine；
- Drive 同步时间顺序。

## 29. 作者主动改稿

### 29.1 未提交章节

废弃当前 attempt，生成同一逻辑任务的新 revision / rewrite attempt。

### 29.2 已进入 Canon 的章节

必须走显式 Revision / Rebase。

历史修改必须：

- 保留旧 revision；
- 产生新 Canon root；
- 标记受影响的未来计划；
- 重新计算必要 Context 和剧情债务状态。

直接修改 `published/*.md` 不能改变 Canon。

## 30. Notion

Notion 不属于核心生产路径，也不是 Release Gate。

Core 首发版本只需生成可选投影，例如：

`projection/notion.json`

投影可包含：

- Chapters；
- Characters；
- Relationships；
- Foreshadowing；
- Conflicts；
- Timeline；
- Volumes / Arcs；
- Reader Promises。

Notion 更新失败不得阻止写作。

## 31. 当前代码迁移方向

### 31.1 保留并深化

重点保留：

- `internal/domain`
- `internal/store`
- Checkpoint
- Digest
- Timeline
- Character state
- World state
- Resource ledger
- Planning data
- 确定性 rules
- zero-init 中与模型无关的数据结构
- 本地检索
- consistency / commit / recovery 中的确定性逻辑

当前 `Store` 作为新 Core 的起点和状态组合根，不重新造一套平行数据库。

到 M6 时，应继续清理只服务旧 AI runtime 的 Store 子项；`Usage`、模型 Session、`AIVoice`、模型调用审计等不得因为历史兼容而成为 Novel Core 的必需状态。

### 31.2 从 Tool 外壳中收回业务逻辑

当前 `internal/tools/commit_chapter.go` 等文件把大量确定性业务逻辑包在 `agentcore.Tool` schema 和执行入口里。

迁移方向：

```text
旧：Agent → Tool → Store
新：CLI / Reconciler → Core → Store
```

Tool schema 不再是业务边界。

Core 公共面要保持很小，优先围绕意图级操作，例如：

- Open Project；
- Prepare Current Task；
- Reconcile Workspace；
- Status；
- Verify；
- Export。

不要为只有一个实现的内部能力预先创建大量 interface/adapter。

### 31.3 不迁移的旧 AI 质量机制

当前代码中如果某些 AIGC、AI voice、模型自评、provider usage 或语义审稿机制依赖模型身份或模型判断，不得为了“兼容”塞入新 Core。

只有能够确定性执行、且确实服务 Canon 或生产合同的规则才迁移。

### 31.4 最终退出生产路径

当新 Core 主干覆盖全部必要行为后，以下内容退出生产运行时：

- `bootstrap.ModelSet`
- Provider runtime
- provider failover
- reasoning effort routing
- Coordinator Agent
- SubAgents
- Writer / Drafter / Reviewer agent runtime
- `agentcore` 主循环
- Codex provider
- Ollama provider
- LiteLLM / provider dispatch
- writer sampler 中只服务模型调用的部分
- model usage / pricing
- provider watchdog
- provider-bound provenance

删除发生在迁移末尾，而不是第一步。

## 32. 新 Core 依赖方向

目标依赖：

```text
cmd/novel-core
    ↓
internal/core
    ↓
internal/domain
internal/store
internal/rules
internal/retrieval
```

`internal/core` 不得依赖：

- `internal/agents`
- `bootstrap.ModelSet`
- `agentcore`
- `internal/llmcodex`
- 任意模型 provider。

## 33. CLI

最终用户需要的核心命令：

### `novel-core init`

初始化项目、本地权威目录、Drive workspace 和第一个正式 Production Task。

### `novel-core serve`

常驻处理：

- submission reconciliation；
- validation；
- commit；
- recovery；
- task generation；
- published / backup / projection 输出。

### `novel-core status`

展示：

- 当前 Canon root；
- 当前章节；
- 当前 task / attempt；
- reconciliation status；
- production result；
- blocked reason。

### `novel-core verify`

完整验证 Canon、Receipt chain、Accepted artifacts 和发布正文。

### `novel-core export`

从权威状态生成唯一正式整书正文。

Core CLI 不提供 `--provider`、`--model`、`--api-key`。

## 34. Google Drive 能力预检

“每章只说一次继续”成立的前提是 ChatGPT App 对项目 Drive 具备足够的读写能力。

能力预检是正式创作前的一次 setup handshake，不属于 Production Task，也不占用章节 attempt。

流程：

1. `novel-core init` 生成固定的 `setup/capability-check.json` 和只读测试文件；
2. 作者在 ChatGPT 中触发一次连接检查；
3. ChatGPT 读取测试文件，并在约定的 setup inbox 创建一份 ack；
4. Drive Desktop 将 ack 同步到本地；
5. Core 校验 ack 内容与 nonce，并记录 capability receipt；
6. 只有检查通过后才生成第一个正式 READY。

能力预检必须证明 ChatGPT 能够：

- 读取 project / protocol / test payload；
- 创建新文件；
- 按约定路径写入；
- 让 Core 通过 Drive Desktop 收到完整内容。

如果只读能力存在而写能力不存在，系统必须明确报告不满足目标运行模式，不能等到正式第 1 章才暴露。

## 35. 测试策略

测试优先从最高公共 seam 进入，不围绕私有 helper 堆 mock。

### 35.1 Core E2E

使用临时本地 Canon 目录和临时 Drive workspace，覆盖：

```text
init
→ task
→ submission
→ validate
→ commit
→ next task
```

### 35.2 必测故障场景

- 只有 chapter，没有 manifest；
- manifest 到达但其他文件尚未同步；
- artifact digest 错误；
- inactive attempt 提交；
- stale Canon；
- 重复 submission；
- 同 attempt 不同内容；
- Commit 中途崩溃；
- READY 损坏；
- 人物知识越界；
- 不可能的旅行时间；
- 资源不足；
- 非法伏笔回收；
- 未履行 Ending Contract 却请求完本；
- revision 后旧计划失效。

### 35.3 AI 不进入 CI

Core 自动 CI 不调用 ChatGPT。

AI 集成通过单独的真实产品验收完成。

## 36. 实际 ChatGPT 产品验收

正式 Release Gate 必须用普通 ChatGPT App 实测：

1. setup capability handshake PASS；
2. Core 生成 Chapter 1 Task；
3. 用户只输入“继续”；
4. ChatGPT 读取 Drive Task；
5. 完成推演、正文、自审和 Submission；
6. Core ACCEPT；
7. READY 进入 Chapter 2；
8. 故意构造一次人物知识越界；
9. Core 返回 REWRITE，并生成 Chapter 2 的新 attempt；
10. 用户再次只输入“继续”；
11. ChatGPT 修 Chapter 2，不写 Chapter 3；
12. Core ACCEPT；
13. 重启 Core 后继续；
14. 新建 ChatGPT 对话，通过项目名恢复；
15. 完成一次滚动 Arc 切换；
16. 完成一次作者 Revision / Rebase；
17. 满足 Ending Contract；
18. 导出完整小说。

Release Gate 只允许 PASS / FAIL，不使用“基本可用”“主体完成”代替。

## 37. 实施里程碑

本设计只定义实现边界，不在这里展开文件级施工步骤。后续 implementation plan 必须按测试先行拆解。

### M1 — Core Spine

结果：

- `novel-core` 无 AI 启动；
- 可以打开现有 Store；
- 新 Core 与 agentcore / ModelSet 隔离；
- 旧 runtime 暂时仍可存在。

### M2 — One Chapter Loop

结果：

```text
init
→ task
→ submission
→ validate
→ commit
→ next task
```

这是第一个生死线。循环不够简单可靠时，在 M2 修正，不继续堆领域功能。

### M3 — Production Reliability

结果：

- serve；
- manifest-last；
- reconciliation status；
- 幂等；
- active attempt / stale 防护；
- 崩溃恢复；
- ACCEPTED / REWRITE / BLOCKED。

### M4 — Long-form Canon

结果：

- Context Compiler；
- Character Observation；
- Knowledge provenance；
- Timeline / Location；
- Resource Ledger；
- Relationships；
- Foreshadow / Conflict / Promise；
- Ending Contract。

### M5 — Authoring Lifecycle

结果：

- 跨聊天恢复；
- 滚动 Arc；
- 作者 Revision / Rebase；
- published / backup；
- 完整导出；
- 可选 Notion projection。

### M6 — Runtime Removal & Acceptance

结果：

- 旧 AI runtime 退出生产路径；
- 模型专属 Store 状态退出 Core 必需路径；
- 无模型配置即可完整运行；
- 真实普通 ChatGPT 产品验收 PASS。

## 38. 明确不做

首发目标明确不包含：

- OpenAI API 自动生成；
- 第三方模型 API；
- MCP；
- ChatGPT Work；
- UI 自动化；
- 无人值守整本生成；
- 每分钟轮询 ChatGPT；
- 完整 Event Sourcing 平台；
- 新数据库作为前置依赖；
- 向量数据库作为必需依赖；
- Notion 双向同步 Canon；
- Notion 作为 Release Gate；
- 多人同时编辑同一 Canon；
- 伪造 provider/model 调用证据；
- AIGC 检测规避；
- 平台审核通过或流量保证。

## 39. 完成定义

项目只有在以下条件全部满足时才算完成：

- Novel Core 生产路径不调用任何 AI；
- 没有任何模型 API Key 也能启动和完整运行；
- Core 不依赖 `agentcore` / `ModelSet` / provider runtime；
- 模型专属 Usage / Session / AIVoice 等状态不是 Core 的必需依赖；
- Canon 是唯一事实源；
- Drive 不是数据库；
- Drive 每个区域的唯一写入方明确且被协议验证；
- setup capability handshake 能提前发现 Drive 无写能力；
- ChatGPT 可以通过普通 Chat + Drive 完成章节提交；
- 正常情况下用户每轮只需输入“继续”；
- 一个章节任务内部的规划、人物推演、正文、自审和状态提议由 ChatGPT 一次完成；
- Core 能区分交换层 PENDING / INVALID 与生产层 ACCEPTED / REWRITE / BLOCKED；
- ACCEPT 后自动准备下一任务；
- REWRITE 保持逻辑 task、生成新 attempt，且不允许跳章；
- 支持人物知识边界；
- 支持人物状态和关系状态；
- 支持时间、地点和资源约束；
- 支持伏笔、冲突和 Reader Promise 生命周期；
- 支持 Ending Contract；
- 支持幂等、inactive attempt 和 stale submission 拒绝；
- 支持崩溃恢复；
- 支持作者 Revision / Rebase；
- 支持新 ChatGPT 对话恢复；
- 支持完整导出；
- Core E2E 全部通过；
- 真实普通 ChatGPT 产品验收 PASS。

## 40. 设计决策摘要

最终只把四个概念放在系统中心：

- **Task**：Core 要 ChatGPT 做什么；
- **Submission**：ChatGPT 声称完成了什么；
- **Canon**：Core 已经接受的小说事实；
- **Receipt**：为什么这次 Canon 变化是合法的。

Attempt 是 Task 的执行版本，不提升为第五个业务中心概念。

任何新模块、协议或抽象，如果不能明显增强这四个概念之一，就不应进入首发范围。
