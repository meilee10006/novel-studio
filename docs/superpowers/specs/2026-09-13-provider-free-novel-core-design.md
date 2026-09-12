# Provider-free Novel Core 设计规格

状态：待作者确认  
日期：2026-09-13  
目标分支：`provider-free-novel-core`  
上游基线：`Xiaoyangy/novel-studio@e3beebbf2f35b9ff55fd781d82055b60d45970c8`

## 1. 目标

把当前 novel-studio 改造成一个完全不调用 AI 的本地长篇小说状态引擎。

唯一负责创作、人物推演、审稿和改写的 AI 是普通 ChatGPT App。正式连载后，正常情况下作者每章只需要输入一次：

> 继续

一次“继续”完成当前章节所需的规划、人物推演、正文、自审、修改和结构化提交。Novel Core 随后在本地完成校验、提交、状态更新和下一任务准备。

如果本章违反可确定验证的硬规则，Core 不推进章节，而是为同一个任务生成新的返工尝试；作者下一次“继续”修当前章，不得跳章。

系统不使用 OpenAI API、其他模型 API、Ollama、MCP、ChatGPT Work，也不通过浏览器或桌面 UI 自动点击 ChatGPT。

### 1.1 完成标准

项目完成时，必须能真实跑通：

```text
首次建书
→ 基础设定进入 Canon
→ 第 1 章
→ 连续多章
→ 至少一次返工
→ Core 重启恢复
→ 新 ChatGPT 对话恢复
→ 一次作者改稿与历史重算
→ Arc 滚动衔接
→ 满足结局合同
→ 导出完整书稿
```

整个 Novel Core 运行过程不需要任何模型配置、模型密钥或模型服务。

## 2. 产品边界

首发核心只有三部分：

1. **ChatGPT App**：唯一智能层，负责想、写、推演、审稿和改稿。
2. **Novel Core**：本地、无 AI、确定性，负责事实、状态、校验、提交、恢复和下一任务。
3. **Google Drive**：ChatGPT 与本地 Core 之间的文件交换层。

Notion 只是可选投影，不属于核心生产链，也不是发布门槛。

一句话定义：

> ChatGPT 负责创作，Novel Core 负责记住事实、检查硬约束、提交版本和恢复现场。

## 3. 术语

为避免实现阶段出现同词不同义，本规格统一使用以下概念。

### Canon

Core 已正式接受的小说事实与状态。只有成功提交后才能进入 Canon。

### Task

Core 要 ChatGPT 完成的一项逻辑工作，例如“写第 47 章”或“重做已经提交的第 47 章”。

### Attempt

同一个 Task 的一次不可变执行版本。第一次写、返工和协议重试都属于不同 Attempt。

### Submission

ChatGPT 对当前 Attempt 的完整交付物。

### Receipt

Core 对一次正式结果的不可变回执，用来说明哪个输入在什么 Canon 前态上得到什么结果。

### Story Event

本章发生的结构化事件，是知识变化、关系变化、伏笔推进等状态变化的证据锚点。

### Author Directive

作者明确要求改变创作方向或历史内容的指令。它不是 Canon，必须经过 Core 转换成正式任务后才能影响 Canon。

## 4. 用户场景

1. 作为作者，我希望建立一本新书并冻结世界、人物、主线和结局约束，避免连载后设定漂移。
2. 作为作者，我希望正常写作时每章只说一次“继续”，不用重复解释工作流程。
3. 作为作者，我希望人物只依据自己真正知道的信息行动，避免突然获得上帝视角。
4. 作为作者，我希望时间、地点、伤势、金钱、道具和承诺跨几百章保持连续。
5. 作为作者，我希望伏笔、冲突和读者承诺有明确状态，避免忘记、提前回收或无故消失。
6. 作为作者，我希望可修问题只返工当前章，不污染后续 Canon。
7. 作为作者，我希望真正需要我做剧情选择时系统才停下来，而不是每一步都找我确认。
8. 作为作者，我希望改已经提交的旧章节时能知道哪些后续状态因此失效，并能安全重算。
9. 作为作者，我希望关掉电脑后重新启动仍能准确恢复，而不是依赖聊天记忆。
10. 作为作者，我希望换一个新的 ChatGPT 对话也能继续当前项目。
11. 作为作者，我希望 Drive 或 Notion 出问题时不会破坏本地 Canon。
12. 作为作者，我希望最后能导出唯一、顺序正确、版本明确的完整书稿。
13. 作为维护者，我希望 Core 在没有任何模型配置的机器上也能构建、测试和运行。
14. 作为维护者，我希望重复同步、重复提交和程序崩溃不会造成双重提交。
15. 作为维护者，我希望主要行为通过 CLI 与文件协议端到端测试，而不是依赖大量内部 mock。

## 5. 不可破坏的原则

### 5.1 Canon 是唯一事实源

以下内容都不是 Canon：

- ChatGPT 聊天历史；
- ChatGPT Memory；
- 未验收的 Drive 文件；
- Notion 页面；
- 未验收的状态变化；
- 被拒绝或被废弃的 Attempt；
- 人工直接修改的 `published` 副本。

只有 Core 成功提交后的状态才属于 Canon。

### 5.2 ChatGPT 不能直接改 Canon

ChatGPT 只能提交提案，包括正文、章节合同、Story Event、状态变化、自审结果、滚动规划和作者指令。

Core 决定哪些提案能够进入 Canon。

### 5.3 Core 不冒充文学编辑

Core 只检查能够确定验证的事情，例如任务身份、前态、事件引用、时间约束、资源账、状态机和硬合同。

以下判断不属于 Core：

- 这段是否足够感人；
- 爽点是否足够强；
- 人物是否“有灵魂”；
- 读者一定会不会喜欢；
- 平台一定会不会给流量。

这些由 ChatGPT 自审和作者判断负责。

### 5.4 不伪造模型调用证据

新系统不得伪造 `provider`、`model`、token 用量、tool call id 或 API response id。

新 provenance 只记录系统真正能证明的内容：Task、Attempt、Canon root、文件摘要、验证结果、Receipt 和时间。

### 5.5 Drive 是交换层，不是数据库

Drive 可能出现同步延迟、重复事件、到达顺序变化、离线和人工误改。Core 不得把 Drive 目录当作权威 Store。

## 6. 总体架构

```text
作者
  │ “继续” / 改稿指令 / 裁决
  ▼
普通 ChatGPT App
  │ Google Drive 读写连接
  ▼
Google Drive Cloud
  │
  │ Drive Desktop 同步
  ▼
本地 Drive Workspace
  │
  ▼
Novel Core
  ├── Task 编译
  ├── Submission 对账
  ├── 确定性验证
  ├── Canon 提交与恢复
  ├── Context 编译
  ├── 本地检索
  └── 导出
      │
      ▼
本地权威 Store
```

Notion 位于旁路，只消费 Core 生成的投影。

## 7. 项目生命周期

### 7.1 能力检查

正式建书前先检查 ChatGPT 对项目 Drive 是否真的具备读写能力。

`novel-core init` 生成一次性的 setup challenge。作者在 ChatGPT 中触发连接检查，ChatGPT 读取 nonce 并在约定目录写回 ack。Core 收到并校验后生成 capability receipt。

能力检查至少证明：

- ChatGPT 能读项目文件；
- ChatGPT 能创建普通 `.json`、`.md` 文件；
- ChatGPT 能写入指定目录；
- Drive Desktop 能把内容原样同步到本地；
- Core 能读到完整 UTF-8 内容。

如果账号只有 Drive 读取能力，系统必须在正式建书前明确失败，不能等到第 1 章才暴露。

能力检查不是 Production Task，也不进入 Canon。

### 7.2 首次建书

能力检查通过后，Core 生成 `foundation` Task。

ChatGPT 与作者共同确定并提交：

- 创作合同；
- 世界圣经；
- 人物圣经；
- 题材和目标读者；
- 全书方向；
- 卷级规划；
- 当前 Arc 规划；
- Ending Contract；
- 文风与平台约束。

Core 只做结构和硬约束检查。Foundation Accepted 后，这些内容进入 Canon，Core 才生成第一章任务。

### 7.3 正常写作

作者输入：

> 继续

ChatGPT 读取根协议和 READY，执行当前 Attempt，并在这一轮内完成：

1. 读取 Context Pack；
2. 检查当前卷、Arc 和本章义务；
3. 分人物做可见信息范围内的推演；
4. 形成章节合同；
5. 设计冲突、推进、伏笔和阅读回报；
6. 写正文；
7. 做连续性、人物、因果、节奏、语言和阅读体验自审；
8. 必要时自行修改；
9. 形成 Story Event 与证据化状态变化；
10. 写入完整 Submission；
11. 最后写 manifest。

Core 随后自动对账、验证和提交。

### 7.4 返工

如果正文或状态提案违反可修复的硬规则，Core 返回 `REWRITE`，保持原 `task_id`，生成新的 `attempt_id`。

下一次“继续”必须修当前任务，不能跳到下一章。

### 7.5 阻塞与作者裁决

`BLOCKED` 只用于现有硬规则无法自行满足的创作冲突，例如两个锁定设定互相矛盾。

Core 为阻塞生成稳定 `block_id`。作者在 ChatGPT 中做决定后，ChatGPT 通过控制消息提交 `block_resolution`。Core 校验引用关系后解除阻塞并生成新的 Attempt。

文件损坏、同步不完整、digest 不一致不属于剧情 BLOCKED。

### 7.6 作者主动改稿

作者可以直接在聊天里提出：

> 第 47 章不要这样处理，顾清瑶不能这么快原谅主角，黑色钥匙伏笔保留。

ChatGPT 把这个意图写成 `author_directive` 控制消息。Core 根据目标章节和当前 Canon 判断：

- 尚未提交：废弃当前 Attempt，生成新的 Attempt；
- 已进入 Canon：生成正式 `revision` Task，并进入显式 Rebase 流程。

作者指令本身不能直接改 Canon。

## 8. 本地权威目录与 Drive Workspace

### 8.1 本地权威目录

默认位置：

```text
~/.novel-core/projects/<project-id>/
```

保存 Canon、Accepted Chapters、人物、世界、时间线、关系、资源、剧情债务、规划、Checkpoint、Receipt 和本地索引。

### 8.2 Drive Workspace

```text
NovelStudio/<project-id>/
├── project.json
├── CHATGPT_PROTOCOL.md
├── setup/
├── exchange/
│   ├── READY.json
│   ├── STATUS.json
│   ├── outbox/
│   ├── inbox/
│   ├── result/
│   └── control/
├── published/
├── projection/
└── backup/
```

删除 Drive workspace 不得造成 Canon 丢失。

删除本地权威目录后，Core 也不得把 Drive 中的普通副本悄悄当成 Canon。恢复必须走显式 restore。

## 9. 写入权

每个区域只有一个正式写入方。

| 区域 | 正式写入方 | 另一方 |
|---|---|---|
| `project.json` | Core | ChatGPT 只读 |
| `CHATGPT_PROTOCOL.md` | Core | ChatGPT 只读 |
| `exchange/READY.json` | Core | ChatGPT 只读 |
| `exchange/STATUS.json` | Core | ChatGPT 只读 |
| `exchange/outbox/**` | Core | ChatGPT 只读 |
| `exchange/inbox/**` | ChatGPT | Core 只读并处理 |
| `exchange/result/**` | Core | ChatGPT 只读 |
| `exchange/control/inbox/**` | ChatGPT | Core 只读并处理 |
| `exchange/control/result/**` | Core | ChatGPT 只读 |
| `published/**` | Core | ChatGPT 只读 |
| `projection/**` | Core | ChatGPT / Notion 只读消费 |
| `backup/**` | Core | ChatGPT 不写 |

ChatGPT 不得修改 READY、STATUS、outbox、result、published 或 backup。

Core 不得替 ChatGPT 修改已经落盘的 Submission，只能接受、拒绝、作废或生成新的 Attempt。

## 10. 版本与摘要规则

### 10.1 版本

所有机器协议都必须带 `schema_version`；项目协议带 `protocol_version`。

规则：

- 同一主版本内允许向后兼容的新增字段；
- 不兼容变化必须提高主版本；
- Core 不得猜测未知主版本；
- 升级本地 Canon 前必须先生成可验证备份；
- 协议升级后，旧 Attempt 不得被新协议静默解释成另一种含义。

### 10.2 摘要

Core 统一使用 SHA-256。

- `artifact_digest`：对收到的原始文件字节计算 SHA-256；
- `submission_digest`：按逻辑路径排序后，对“路径 + artifact_digest”清单计算 SHA-256；
- `task_digest`：Core 对 Task Pack 的确定性清单计算 SHA-256；
- `canon_root`：Core 对当前 Canon 的确定性状态清单、Accepted Artifact 摘要和协议版本计算 SHA-256。

ChatGPT **不需要自己计算 SHA-256**。它只需在 manifest 中回显 Core 已提供的 `task_digest`、`base_canon_root`、完成 nonce 和实际写入的文件清单。

如果 ChatGPT 能提供文件摘要，可作为附加校验；Core 自己计算的摘要才是权威值。

## 11. Task 与 Attempt

### 11.1 Task 类型

首发版只允许三类 Production Task：

- `foundation`：建立或重建基础 Canon；
- `chapter`：写一个新的章节；
- `revision`：重做已经进入 Canon 的历史内容并重算受影响状态。

不设置 `rewrite` Task。返工只是同一个 Task 的新 Attempt。

不把人物推演、伏笔决策、风格审稿、Arc 推演拆成独立 Task；它们是 ChatGPT 为完成一个 Task 所做的内部工作。

### 11.2 Attempt

每个 Attempt 至少包含：

- project id；
- task id；
- task kind；
- attempt id；
- attempt reason：`initial` / `rewrite` / `retry`；
- target chapter 或 foundation target；
- base canon root；
- protocol version；
- task digest；
- required artifacts；
- completion nonce；
- created at。

同一项目任何时刻只允许一个活动 Production Attempt。

只有 READY 当前指向的 Attempt 有资格推进 Canon。

## 12. READY 与 Task Pack

`exchange/READY.json` 是 ChatGPT 的唯一生产入口。

READY 至少包含：

- project id；
- task id；
- task kind；
- attempt id；
- attempt reason；
- target；
- base canon root；
- protocol version；
- task digest；
- completion nonce；
- status。

每个 Attempt 对应：

```text
exchange/outbox/<task-id>/<attempt-id>/
├── task.json
├── context.json
├── canon_excerpt.json
├── constraints.json
└── recent_prose.md
```

Foundation 可使用同一目录结构，但正文相关文件可以为空或替换成 foundation context。

根目录 `CHATGPT_PROTOCOL.md` 是唯一长期协议。Task Pack 只引用 `protocol_version`，不得复制第二套完整协议。

## 13. Submission

### 13.1 Chapter / Revision Submission

```text
exchange/inbox/<task-id>/<attempt-id>/
├── chapter.md
├── chapter_contract.json
├── events.json
├── state_delta.json
├── self_review.json
├── next_arc_proposal.json      # 仅在任务要求时出现
└── manifest.json               # 最后写
```

### 13.2 Foundation Submission

至少包含：

```text
foundation.json
characters.json
world.json
book_plan.json
ending_contract.json
style_profile.json
platform_profile.json
manifest.json
```

### 13.3 manifest

manifest 必须最后写入，并至少回显：

- project id；
- task id；
- attempt id；
- base canon root；
- protocol version；
- task digest；
- completion nonce；
- 实际文件清单。

manifest 的作用是声明“这一 Attempt 已经写完”，不是让 ChatGPT 充当哈希工具。

## 14. 控制消息

作者改稿和 BLOCKED 裁决走独立控制通道，不伪装成章节 Submission。

```text
exchange/control/inbox/<message-id>/
├── control.json
└── manifest.json
```

`control.json` 的 `kind` 首发只允许：

- `author_directive`；
- `block_resolution`。

控制消息必须包含项目 id、message id、当前 Canon root，以及所引用的 task / chapter / block id。

Core 处理结果写入：

```text
exchange/control/result/<message-id>.json
```

控制消息不能直接修改 Canon，只能生成、废弃或恢复正式 Production Task。

## 15. manifest-last 与同步对账

Core 不依赖文件监听事件顺序。文件监听只用于唤醒扫描；正确性来自目录内容、身份和摘要。

### 15.1 对账状态

交换层至少区分：

- `PENDING`：manifest 尚未出现，或声明文件仍未同步完整；
- `READY_TO_VALIDATE`：文件齐全，协议身份正确；
- `INVALID`：结构、身份、nonce、版本或文件清单冲突；
- `SETTLED`：Attempt 已产生正式生产结果。

`PENDING` 不因单纯超时自动变成 `INVALID`。Core 可以提示“等待时间异常”，但不能因为网络慢就判坏稿。

### 15.2 对账规则

1. 没有 manifest：保持 PENDING；
2. manifest 已到但文件不齐：继续 PENDING；
3. 文件齐全后 Core 计算 artifact / submission digest；
4. 相同 Attempt + 相同 submission digest：视为幂等重复；
5. manifest 后内容再次变化：标记协议漂移，不重新打开已经结算的 Attempt；
6. READY 已切换后，旧 Attempt 不再具有提交资格；
7. INVALID 不进入剧情验证，也不等于 BLOCKED。

如果当前活动 Attempt 因可恢复的协议错误 INVALID，Core 可以生成同一 Task 的 `retry` Attempt；这不计入剧情 REWRITE。

## 16. Canon 与提交事务

Canon 由四部分组成：

1. 当前 Canonical State；
2. Accepted Artifacts；
3. Immutable Receipts；
4. Checkpoints。

不引入完整 Event Sourcing 平台，也不要求新数据库。

### 16.1 提交不变量

任何时候都必须满足：

- 一个 Attempt 最多推进 Canon 一次；
- Canon root 只在完整提交后改变；
- READY 最后更新；
- 崩溃恢复后，要么确认本次提交完整成立，要么仍停留在前一个 Canon；
- 不允许出现正文已进入正式版本但人物、时间线或资源仍停在旧状态的半提交。

### 16.2 提交日志

Core 在本地维护可恢复的提交日志，至少记录 `prepared`、`applying`、`committed` 三个阶段及目标 Canon root。

具体落盘方式由 implementation plan 根据现有 Store 与 Checkpoint 机制决定，但不得只依赖进程内锁。

Receipt 只代表已经正式结算的结果，不拿半成品 Receipt 充当事务日志。

## 17. Receipt

Receipt 至少包含：

- schema version；
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

`source=chatgpt_app` 只能是描述性 metadata，不能当成模型调用证明。

## 18. Story Event 与证据

### 18.1 events.json

章节中影响未来状态的关键事件必须进入 `events.json`。

每个 Event 至少包含：

- attempt 内唯一 event id；
- 故事时间；
- 地点；
- actors；
- observers；
- 可见范围；
- consequences；
- evidence kind。

### 18.2 正文证据

正文直接发生的事件应提供短 `evidence_anchor`。它必须是 `chapter.md` 中可机械匹配的原文片段，Core 校验它确实存在。

对于正文未展示但允许发生的离屏事件，必须标记 `offscreen`，并引用允许它发生的章节合同或已存在 Canon 约束。Core 只能检查结构和硬边界，不假装理解离屏事件写得是否合理。

### 18.3 ID 所有权

Canon 中的永久实体 ID 由 Core 管理。

ChatGPT 新增人物、地点、资源、冲突或伏笔时使用 Attempt 内临时 ID。Commit 时 Core 生成永久 ID，并把映射写入 Receipt。

这样可以避免两个聊天自行创造相同永久 ID。

Story Event 自身属于 Accepted Attempt，可以使用 Attempt 作用域内稳定 ID；被拒绝 Attempt 的 Event 永远不能被后续 Canon 引用。

## 19. Chapter Contract

正文生成前，ChatGPT 必须形成 Chapter Contract。

至少包含：

- chapter；
- POV 声明；
- 起始状态；
- 本章目的；
- 当前读者问题；
- 主冲突；
- 必须推进的 obligation id；
- 必须承接的 obligation id；
- 禁止改变的 Canon id；
- 允许的伏笔操作；
- 目标状态变化；
- 章末钩子；
- 目标长度区间。

Contract 不是 Canon，但属于本章生产合同。

Core 只检查可机械验证的部分。例如它能检查 POV 声明、obligation id、字数和伏笔状态机，却不能靠无 AI 代码判断正文是否真的“写出了足够强的悬念”。

## 20. State Delta

ChatGPT 不重新提交完整世界状态，只提交本章变化。

每个影响未来写作的重要变化必须引用 Accepted Event 或本 Attempt 的 Event。

至少覆盖：

- 时间变化；
- 人物地点和身体状态；
- 目标、压力和承诺；
- 知识变化；
- 关系变化；
- 资源变化；
- 世界状态；
- 伏笔；
- 冲突；
- 读者承诺。

Core 根据 Delta 计算新 Canon，而不是让 ChatGPT直接覆盖整个状态文件。

## 21. 人物知识边界

每个 `knowledge_add` 必须引用明确来源：

- 角色亲眼观察的 Event；
- 合法传播 Event；
- 已存在 Canon Fact。

Core 可以机械检查角色是否在 observers 中、是否存在传播链，以及来源是否已进入 Canon。

没有来源的知识变化必须 REWRITE。

Core 不能仅凭正文语义判断“叙述里是否偷偷泄露了 POV 不该知道的信息”。这类正文层面的越界由 ChatGPT 自审负责；Core 负责结构化知识状态不越界。

## 22. 人物 Observation

Core 根据 Canon 为本章重要角色编译 Observation，至少包含：

- 当前地点；
- 当前目标；
- 当前压力；
- 已知事实；
- 明确未知或禁止知道的事实；
- 资源；
- 关系；
- 承诺；
- 当前行动；
- 对关键关系和事件的个人认知。

ChatGPT 先按这些局部视角推演，再写正文。

Observation 是 Context 的派生物，不是第二套 Canon。

## 23. 时间、地点和资源

Core 只能检查已经结构化的规则，不能凭常识自动推断所有现实世界距离和耗时。

可确定检查包括：

- 故事时间顺序；
- 已定义路线或移动规则的最短耗时；
- 同一人物在时间重叠区间的地点冲突；
- 已建账的金钱、道具和消耗品；
- 已锁定的权限、伤势和能力限制。

如果某两个地点之间没有任何已知移动约束，Core 不能假装知道真实交通时间；ChatGPT 自审可以提出合理性问题，Core 只负责已建模规则。

## 24. 关系模型

关系不使用 `trust=72` 这类虚假的高精度数字，也不强迫所有关系进入同一条“敌对→亲密”线性阶梯。

关系状态至少包含：

- 双方当前关系合同或实际关系；
- 各自对关系的 perception；
- 关键未履行承诺；
- 最近证据 Event；
- 可选标签，例如敌对、戒备、合作、依赖、亲密，但标签不表示统一数轴。

关系变化必须有 Event 证据。

## 25. 伏笔、冲突和读者承诺

### 25.1 伏笔

推荐状态：

```text
planned
→ seeded
→ reinforced
→ payoff_ready
→ paid_off
→ closed
```

允许 `misdirected`、`retired`。

每项伏笔至少保存：稳定 ID、表层线索、真义、知情角色、首次出现、强化记录、最早/最晚回收边界、假答案、真回收、依赖和后果。

Core 只执行显式状态机和章节边界，不判断线索是否“高级”。

### 25.2 冲突

冲突必须有稳定 ID、参与方、当前状态、升级/关闭条件和证据 Event，不能无证据自动消失。

### 25.3 读者承诺

读者承诺允许：`advanced`、`fulfilled`、`deferred`、`retired`。

`fulfilled` 必须引用 Event；`deferred` 必须保留新的最迟处理边界；`retired` 必须有作者级 Directive 或 Ending Contract 依据。

## 26. Ending Contract

整书必须拥有 Ending Contract，至少包含：

- 主线必须解决的问题；
- 主角和重要人物弧终点；
- 核心关系终态；
- 核心命题；
- 必须回收伏笔；
- 必须关闭冲突；
- 允许开放的尾声；
- 禁止遗留的问题。

Core 只执行明确编码的收束规则，例如：

- 某章后禁止新增 major mystery；
- 某章后禁止新增 major character；
- 某伏笔最晚回收章；
- 某冲突必须在指定 Arc 前关闭。

Core **不能**凭无 AI 代码判断“这个新谜团太大，剩余篇幅肯定收不回来”。这类语义收束由 ChatGPT 根据 Ending Contract 自审。

存在未满足的硬 Ending Contract 时，不允许把项目标记为 Completed。

## 27. Context Compiler

Core 不把整本小说重新塞给 ChatGPT。

每个 Task Pack 只包含完成当前任务必要的内容：

- Story Bible 相关切片；
- Ending Contract 相关条款；
- 当前卷与 Arc；
- obligation；
- 活跃人物 Observation；
- 时间、地点、资源和世界规则；
- 活跃冲突和剧情债务；
- 可强化/可回收/禁止回收的伏笔；
- 最近若干章摘要；
- 上一章必要尾部正文；
- 本地检索命中的旧段落。

### 27.1 预算

Core 不绑定某个模型 tokenizer，因此 Context 预算按 UTF-8 字节数或 Unicode 字符数配置，不使用虚构的“精确 token 预算”。

不同栏目必须有独立上限和总上限。超过预算时按预先定义的优先级裁剪，硬合同、知识边界和当前状态不能被普通旧正文挤掉。

第 100 章以后，Context Pack 大小不得随全书长度线性增长。

### 27.2 检索

首发版优先使用结构化 Canon、BM25、关键词和实体索引。

不要求 embedding，不要求 Qdrant。

## 28. 滚动 Arc 规划

Arc 规划正常情况下不额外消耗一次“继续”。

正式连载前至少有：

- Book Direction；
- Volume Plan；
- Current Arc Plan；
- Ending Contract。

Core 应在当前 Arc 结束前预留规划提前量，并在 Chapter Contract 中要求 ChatGPT 附带 `next_arc_proposal.json`。

滚动规划规则：

- Proposal 合法：与本章一起登记；
- Proposal 有可修硬错误：本章如果仍处于当前 Arc，可接受正文并把“修复下一 Arc 规划”作为下一章节 Task 的前置要求；
- 已到 Arc 边界仍没有合法下一 Arc 规划：下一章节 Task 必须把规划修复列为硬前置，ChatGPT 在同一次“继续”里先修规划，再写章节；
- 如果硬约束本身互相冲突，才进入 BLOCKED。

这样正常成功路径仍然是一章一次“继续”，但不会为了守这个口号而把错误规划强行写进 Canon。

## 29. CHATGPT_PROTOCOL.md

项目根目录必须有稳定的 `CHATGPT_PROTOCOL.md`，由 Core 根据协议版本生成。

它定义：

- 如何定位项目与 READY；
- “继续”的正式语义；
- 如何读取 Task Pack；
- 如何完成章节内部智能工作；
- Submission 和控制消息格式；
- manifest-last；
- 哪些路径只读；
- ACCEPTED / REWRITE / BLOCKED 行为；
- 新聊天如何恢复；
- 禁止直接修改 Canon。

新的 ChatGPT 对话只需：

> 继续《项目名》

即可从 Drive 恢复生产状态，不依赖旧聊天记忆。

如果同名项目不止一个，必须使用 project id 消歧，不能猜。

## 30. 三层验证

### L1：协议层

检查：

- schema / protocol version；
- project / task / attempt identity；
- manifest；
- completion nonce；
- 文件清单；
- task digest；
- base canon root；
- active attempt。

失败进入 PENDING、INVALID 或 retry，不直接产生剧情 BLOCKED。

### L2：Canon 层

检查结构化事实：

- 人物知识来源；
- 时间与地点；
- 资源；
- immutable facts；
- 关系状态；
- 伏笔和冲突状态机；
- Ending Contract 硬约束。

可修问题通常产生 REWRITE。

### L3：生产合同层

只检查能够机械验证的本章硬要求，例如：

- 章节身份；
- 声明 POV；
- 目标长度；
- obligation id 是否有 Event 证据；
- 禁止修改的 Canon id；
- 允许的伏笔操作。

不得把主观文学质量、真实情绪强度或“读起来爽不爽”塞进 L3。

## 31. 生产结果

只有通过 L1 的 Submission 才能得到生产结果。

### ACCEPTED

合法提交，推进 Canon。

### REWRITE

同一 Task 可修复，Core 生成新的 Attempt。

REWRITE 至少包含：

- violation code；
- offending entity；
- expected；
- observed；
- evidence refs；
- allowed repair scope。

不得只返回“人物不合理，请修改”这类模糊意见。

### BLOCKED

只有硬约束无法同时满足、且需要作者作创作选择时使用。

## 32. 幂等、并发和 stale submission

- 相同 Submission 重放任意次数，只允许一次 Commit；
- 只有 READY 当前 Attempt 有资格进入生产验证；
- READY 切换后，旧 Attempt 不能重新抢占；
- 两个聊天同时提交同一个 Attempt 时，第一份完整且合法的内容版本锁定该 Attempt；后续不同内容标记冲突；
- 基于旧 Canon 的新 Task 提交按 stale submission 拒绝；
- 所有判断依据身份、摘要和 Receipt，不依赖文件修改时间。

## 33. Revision / Rebase

### 33.1 未进入 Canon

废弃当前 Attempt，保持同一个 Task，生成新 Attempt。

### 33.2 已进入 Canon

Core 创建 `revision` Task，目标必须明确到一个已接受版本或章节范围。

Revision Accepted 后：

- 保留旧版本和旧 Receipt；
- 产生新的 Canon root；
- 标记受影响的后续摘要、计划、Observation 和剧情债务为待重算；
- 重算必须按章节顺序推进，不能只改一个 JSON 数字后假装历史一致；
- 所有受影响的已发布章节必须明确保留旧 revision，直到新的派生状态验证完成。

首发版不要求支持多人并发历史编辑。

## 34. 安全边界

Drive 中的任何文件都按不可信输入处理。

Core 必须：

- 只读取协议白名单内的相对路径；
- 拒绝 `..`、绝对路径、路径穿越和越出 workspace 的链接；
- 不跟随 Submission 中的符号链接；
- 只接受声明允许的 UTF-8 文本文件；
- 对单文件大小、Submission 总大小、JSON 深度和数组长度设置可配置上限；
- 不执行 Submission 中的脚本、命令、HTML 或宏；
- 未知文件不进入 Canon；
- 日志不得记录连接凭证；
- Drive、Notion 等凭证不得写进小说项目、Prompt 或 Git 仓库。

## 35. 备份、恢复与迁移

### 35.1 备份

备份至少包含：

- Canon snapshot；
- Accepted Artifact manifest；
- Receipt chain；
- schema / protocol version；
- canon root。

Drive 中的 `backup/` 只是备份副本，不自动拥有 Canon 权威。

### 35.2 恢复

恢复必须显式执行，例如：

```text
novel-core restore --from <backup>
```

Core 在恢复前完整验证摘要和 Receipt chain。验证失败时拒绝恢复，不做“尽量猜”。

### 35.3 迁移

不兼容本地 schema 升级必须先备份，再显式迁移。迁移后重新计算并验证 Canon root 映射，保留迁移 Receipt。

## 36. Notion

Notion 不属于核心生产路径，也不是 Release Gate。

Core 首发只需能生成可选 `projection/notion.json`。Notion 更新失败不得阻止写作。

Notion 的人工备注不得反向覆盖 Canon。

## 37. 当前代码迁移方向

### 37.1 保留并深化

重点保留：

- `internal/domain`；
- `internal/store`；
- Checkpoint；
- Timeline；
- Character / World state；
- Resource ledger；
- Planning data；
- 确定性 rules；
- zero-init 中与模型无关的数据结构；
- 本地检索；
- consistency / commit / recovery 中可确定执行的逻辑。

当前 `Store` 作为新 Core 的起点，不重造一套平行数据库。

### 37.2 把业务逻辑从 Tool 外壳里收回来

当前 `internal/tools/commit_chapter.go` 把大量确定性提交逻辑包在 `agentcore.Tool` schema 与执行入口里。

迁移目标：

```text
旧：Agent → Tool → Store
新：CLI / Reconciler → internal/core → Store
```

`Tool` schema 不再是业务边界。

新 Core 对外公开面保持很小，围绕项目级意图操作，不为单一实现提前建立一层层 interface / adapter。

### 37.3 旧 AI 状态的处理

当前 `Store` 中的 Usage、模型 Session、AIVoice 等历史子项在迁移期间可以为了读取旧项目而暂时存在，但必须满足：

- 新 Core 不要求它们存在；
- 新 Core 不写新的模型 Usage；
- 它们不参与 Canon root；
- 它们不影响 Task、验证或 Commit；
- M6 后只能作为历史兼容数据，或者安全删除。

### 37.4 最终退出生产路径

以下内容最终退出 Novel Core 生产运行时：

- `bootstrap.ModelSet`；
- Provider runtime / failover；
- reasoning effort routing；
- Coordinator / SubAgents；
- Writer / Drafter / Reviewer agent runtime；
- `agentcore` 主循环；
- Codex / Ollama / LiteLLM provider dispatch；
- writer sampler 中只服务模型调用的部分；
- model usage / pricing；
- provider watchdog；
- provider-bound provenance；
- 依赖模型判断的 AIGC / AI voice 门禁。

删除发生在新 Core 主干已经覆盖必要行为之后，而不是迁移第一步。

### 37.5 许可证

Fork 继续遵守上游 Apache-2.0 许可证及必要 NOTICE / copyright 要求，不暗示得到上游作者背书。

## 38. 目标依赖方向

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

- `internal/agents`；
- `bootstrap.ModelSet`；
- `agentcore`；
- `internal/llmcodex`；
- 任意模型 provider。

## 39. CLI

首发核心命令：

### `novel-core init`

初始化项目、本地权威目录、Drive workspace 和能力检查。

### `novel-core serve`

常驻扫描 Submission 和控制消息，执行对账、验证、提交、恢复和下一任务生成。

文件系统 watcher 可以用于降低延迟，但正确性不能依赖 watcher 不丢事件。

### `novel-core status`

展示 Canon root、当前章节、活动 Task / Attempt、对账状态、生产结果和 BLOCK 原因。

### `novel-core verify`

完整验证 Canon、Receipt chain、Accepted Artifacts、活动指针和发布正文。

### `novel-core export`

从权威状态生成唯一正式整书正文。

### `novel-core restore`

从经过验证的备份显式恢复。

### `novel-core migrate`

执行需要改变本地 schema 的显式迁移。

Core CLI 不提供 `--provider`、`--model`、`--api-key`。

## 40. 测试策略

测试优先从最高公共入口进入，不围绕私有 helper 堆 mock。

### 40.1 Core E2E

用临时 Canon 目录和临时 Drive workspace 跑：

```text
init
→ capability ack
→ foundation task
→ foundation accepted
→ chapter task
→ submission
→ validate
→ commit
→ next task
```

### 40.2 必测故障

至少覆盖：

- 没有 manifest；
- manifest 先到、其他文件后到；
- completion nonce 错误；
- task digest 错误；
- 未知协议主版本；
- inactive Attempt；
- stale Canon；
- 重复 Submission；
- 同 Attempt 在 manifest 后内容改变；
- Commit 各恢复边界崩溃；
- READY 损坏；
- 控制消息重复；
- BLOCKED 裁决引用错误 block id；
- Story Event 证据片段不在正文；
- 人物知识没有来源；
- 已建模路线下旅行时间不可能；
- 资源不足；
- 非法伏笔回收；
- Reader Promise 无证据却标 fulfilled；
- 未满足 Ending Contract 却请求完本；
- Revision 后旧派生状态未重算；
- 备份摘要损坏；
- 路径穿越、符号链接和超大输入。

### 40.3 AI 不进入 CI

Core 自动 CI 不调用 ChatGPT。

真实 AI 只进入产品级人工验收。

## 41. 真实 ChatGPT 产品验收

Release Gate 必须使用普通 ChatGPT App 实测：

1. 能力检查 PASS；
2. Foundation 正式进入 Canon；
3. Core 生成 Chapter 1；
4. 用户只输入“继续”；
5. ChatGPT 读取 Task Pack 并完成正文、自审、events、delta 和 manifest；
6. Core ACCEPT，READY 进入 Chapter 2；
7. 故意制造一次人物知识越界；
8. Core REWRITE，并生成同 Task 新 Attempt；
9. 用户再次只输入“继续”；
10. ChatGPT 修 Chapter 2，不写 Chapter 3；
11. Core ACCEPT；
12. 重启 Core 后继续；
13. 新建 ChatGPT 对话，通过项目名恢复；
14. 完成一次滚动 Arc 规划；
15. 通过 Author Directive 完成一次已提交章节 Revision / Rebase；
16. 人为制造一次 BLOCKED，并通过 block_resolution 恢复；
17. 满足 Ending Contract；
18. `verify` 通过；
19. 导出完整书稿。

Release Gate 只有 PASS / FAIL，不使用“基本可用”“主体完成”代替。

## 42. 实施里程碑

### M1 — Core Spine

- `novel-core` 无 AI 启动；
- 能打开现有 Store；
- 新 Core 与 agentcore / ModelSet 隔离；
- 旧 runtime 暂时可继续存在。

### M2 — One Chapter Loop

打通：

```text
foundation accepted
→ chapter task
→ submission
→ validate
→ commit
→ next task
```

这是第一个生死线。主循环不够简单可靠时，在 M2 修，不继续堆领域功能。

### M3 — Production Reliability

完成 manifest-last、对账状态、幂等、retry、stale 防护、提交恢复、控制消息和 ACCEPTED / REWRITE / BLOCKED。

### M4 — Long-form Canon

完成人物知识、Observation、时间地点、资源、关系、伏笔、冲突、Reader Promise、Ending Contract 和 Context Compiler。

### M5 — Authoring Lifecycle

完成跨聊天恢复、滚动 Arc、Author Directive、Revision / Rebase、备份恢复和完整导出。

### M6 — Runtime Removal & Acceptance

旧 AI runtime 退出生产路径，模型专属 Store 状态退出必需路径，真实普通 ChatGPT 产品验收 PASS。

## 43. 明确不做

首发不包含：

- OpenAI API 自动生成；
- 第三方模型 API；
- MCP；
- ChatGPT Work；
- UI 自动化；
- 无人值守整本生成；
- 以高频轮询模拟 ChatGPT 自动触发；
- 完整 Event Sourcing 平台；
- 新数据库作为前置依赖；
- 向量数据库作为必需依赖；
- Notion 双向同步 Canon；
- Notion 作为 Release Gate；
- 多人同时修改同一 Canon；
- Core 通过自然语言理解判断文学质量；
- Core 自动推断没有建模的现实世界交通常识；
- 伪造 provider / model 调用证据；
- AIGC 检测规避；
- 平台审核、读者接受或推荐流量保证。

## 44. 完成定义

以下条件必须全部满足：

- Novel Core 生产路径不调用任何 AI；
- 没有模型 API Key 也能完整运行；
- Core 不依赖 `agentcore`、`ModelSet` 或 provider runtime；
- 模型 Usage / Session / AIVoice 不是 Core 必需状态；
- Canon 是唯一事实源；
- Drive 不是数据库；
- 各交换目录有明确唯一写入方；
- 能力检查能提前发现 Drive 无写能力；
- Foundation 有正式 Task / Submission / Receipt；
- 正常情况下作者每章只需输入一次“继续”；
- Chapter Submission 包含正文、事件、状态变化和自审；
- Core 能区分同步问题、协议问题和剧情 BLOCKED；
- Core 自己计算文件摘要，不要求 ChatGPT 可靠充当哈希工具；
- ACCEPT 后自动准备下一 Task；
- REWRITE 保持 Task、生成新 Attempt，不跳章；
- Author Directive 和 BLOCKED 裁决都有正式回写通道；
- 人物知识有来源链；
- 关系变化不依赖虚假高精度数值；
- 时间、地点和资源只按已建模规则确定验证；
- 伏笔、冲突和 Reader Promise 有生命周期；
- Ending Contract 只执行显式硬规则，不假装做语义判断；
- Context 大小不随全书长度线性增长；
- 支持幂等、并发冲突、inactive Attempt 和 stale submission；
- 支持崩溃恢复；
- 支持 Revision / Rebase；
- 支持显式备份恢复和 schema 迁移；
- 支持新 ChatGPT 对话恢复；
- 支持完整导出；
- Core E2E 全部通过；
- 真实普通 ChatGPT 产品验收 PASS。

## 45. 设计收束

系统中心只保留四个业务概念：

- **Task**：Core 要 ChatGPT 做什么；
- **Submission**：ChatGPT 交付了什么；
- **Canon**：Core 已经接受了什么；
- **Receipt**：为什么这次 Canon 变化成立。

Attempt 是 Task 的执行版本；Story Event 是 Submission 与 Canon 之间的证据；控制消息只是作者意图进入正式 Task 的入口，都不另起一套生产系统。

任何新增模块、协议或抽象，如果不能明显增强这四个概念之一，就不进入首发范围。
