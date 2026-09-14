# 无 AI Novel Core 设计规格

状态：已收敛，待实施
日期：2026-09-13  
目标分支：`provider-free-novel-core`  
上游基线：`Xiaoyangy/novel-studio@e3beebbf2f35b9ff55fd781d82055b60d45970c8`

## 1. 目标

把当前 novel-studio 改造成一个完全不调用 AI 的本地长篇小说状态引擎。唯一负责创作、人物推演、审稿和改写的 AI 是普通 ChatGPT App；本地 Novel Core 只负责权威状态、硬约束、版本、恢复和任务推进。

正式连载后，正常情况下作者每章只需要输入一次“继续”。ChatGPT 完成当前任务所需的规划、人物推演、正文、自审和结构化提交；Core 随后完成对账、校验、提交，并准备下一任务。

首发完成时必须真实跑通：

```text
能力检查
→ 首次建书
→ 第 1 章
→ 连续多章
→ 至少一次返工
→ Core 重启恢复
→ 新 ChatGPT 对话恢复
→ 一次历史章节改稿与下游重放
→ Arc 滚动衔接
→ 满足结局约束
→ 导出完整书稿
```

整个 Core 不需要模型配置、API Key、Ollama、MCP、ChatGPT Work，也不通过浏览器或桌面自动化去点击 ChatGPT。

## 2. 产品边界

首发核心只有三部分：

1. **ChatGPT App**：唯一智能层，负责想、写、推演、自审和改稿。
2. **Novel Core**：本地、无 AI、确定性，负责权威状态、校验、提交、恢复和下一任务。
3. **Google Drive**：ChatGPT 与本地 Core 之间的文件交换层；Drive Desktop 只负责把云端文件同步到本机目录。

Notion 只是可选的只读投影，不参与生产闭环，也不是发布门槛。

一句话定义：

> ChatGPT 负责创作，Novel Core 负责记住什么已经成立、什么不能被破坏，以及下一步该做什么。

## 3. 统一术语

文档正文尽量使用中文；英文只保留给协议字段、枚举和代码名。

| 中文术语 | 协议/代码名 | 含义 |
|---|---|---|
| 权威状态 | Canon | Core 已正式接受的小说事实、硬约束、滚动规划和生产状态 |
| 任务 | Task | Core 要 ChatGPT 完成的一项逻辑工作 |
| 尝试版本 | Attempt | 同一任务的一次不可变执行版本 |
| 提交包 | Submission | ChatGPT 对当前尝试版本的完整交付物 |
| 回执 | Receipt | Core 对一次正式处理结果留下的不可变记录 |
| 故事事件 | Story Event | 本章中会影响后续状态的结构化事件 |
| 作者指令 | Author Directive | 作者要求改变未来方向或历史内容的结构化指令 |
| 章节约束 | chapter_contract | 本章必须满足、不得破坏的可检查要求 |
| 状态变化 | state_delta | 本章相对前态产生的结构化变化 |
| 结局约束 | ending_contract | 全书收束时必须满足的明确硬条件 |
| 上下文包 | context pack | Core 为当前任务编译的有限工作集 |

`ACCEPTED`、`REWRITE`、`BLOCKED` 等大写英文仅作为机器协议枚举使用。

## 4. 用户场景

1. 作为作者，我希望新书一开始就有明确的世界、人物、主线和结局约束，避免连载后设定漂移。
2. 作为作者，我希望正常写作时每章只说一次“继续”，不用反复解释流程。
3. 作为作者，我希望人物只能根据自己真正知道的信息行动，不会突然获得上帝视角。
4. 作为作者，我希望时间、地点、伤势、金钱、道具、关系和承诺跨几百章保持连续。
5. 作为作者，我希望伏笔、冲突和读者承诺有明确生命周期，不会被遗忘或无故消失。
6. 作为作者，我希望可修问题只返工当前任务，不污染后续状态。
7. 作为作者，我希望只有真正需要创作选择时系统才停下来找我裁决。
8. 作为作者，我希望改旧章节时能明确知道哪些后续章节已经失效，并安全重放。
9. 作为作者，我希望关掉电脑、重启 Core 或换一个 ChatGPT 对话后仍能继续。
10. 作为作者，我希望 Drive、Notion 或聊天记录出问题时不会破坏本地权威状态。
11. 作为作者，我希望最后能导出唯一、顺序正确、版本明确的完整书稿。
12. 作为维护者，我希望没有任何模型配置时 Core 仍能构建、测试和运行。
13. 作为维护者，我希望重复同步、重复提交、程序崩溃和两个聊天同时写入都不会造成双重提交。
14. 作为维护者，我希望主要行为从 CLI 和文件协议的公共入口测试，而不是靠大量内部 mock。

## 5. 不可破坏的原则

### 5.1 本地存储是唯一权威来源

聊天历史、ChatGPT Memory、Drive 中尚未验收的文件、Notion 页面、被拒绝的尝试版本、人工修改的发布副本，都不是权威状态。

只有 Core 完成正式提交后，变化才算成立。

### 5.2 ChatGPT 只能提案，不能直接改权威状态

ChatGPT 可以提交正文、章节约束、故事事件、状态变化、自审结果、滚动规划和作者指令。Core 决定这些内容是否能进入权威状态。

### 5.3 Core 只做能确定判断的事

Core 可以检查任务身份、前态、事件引用、时间与资源约束、状态机和硬合同；它不能判断文字是否感人、爽点是否够强、人物是否“有灵魂”、读者一定会不会喜欢，也不能保证平台审核和流量。

文学质量由 ChatGPT 自审和作者判断负责。

### 5.4 不伪造模型证据

新系统不得伪造 `provider`、`model`、token 用量、tool call id 或 API response id。新回执只记录系统真正能证明的任务、尝试版本、摘要、校验结果、权威状态根和时间。

### 5.5 Drive 只是交换层

Drive 可能延迟、离线、重复同步、改变到达顺序，也可能被人工误改。Core 不得把 Drive 目录当成数据库，更不能在本地权威目录丢失后把普通 Drive 副本悄悄升级成权威状态。

### 5.6 不为单一实现预造抽象层

首发只有一个本地存储、一个文件交换协议和一个 Core 运行时。没有第二种实现之前，不为它们额外建立通用接口或适配层；复杂性留在深模块内部，对外公开面保持小而稳定。

## 6. 权威状态的组成

“权威状态”不是把所有东西都当成永远不能改的故事事实。Core 至少区分三类内容：

1. **已发生事实**：已验收正文、故事事件、人物知识、时间线、资源、关系和已发生的世界变化。只能通过后续已验收事件或历史修订改变。
2. **硬约束**：作者明确锁定的设定、禁区、结局条件和不可越过的规则。普通章节不能自行改写；需要作者指令和正式修订。
3. **滚动规划**：未来卷纲、Arc 计划、章节义务、风格和平台偏好。它们属于当前有效生产状态，但可以被后续已验收规划更新替换，不等同于“故事里已经发生”。

这样既保留单一权威来源，又避免把未来计划误当成不可变历史。

## 7. 总体架构

```text
作者
  │ “继续” / 改稿 / 裁决
  ▼
ChatGPT App
  │ Google Drive 文件读写
  ▼
Google Drive Cloud
  │ Drive Desktop
  ▼
本地 Drive 工作区
  │
  ▼
Novel Core
  ├── 协议对账与本地快照
  ├── 任务/尝试版本状态机
  ├── 确定性校验
  ├── 提交、恢复与版本根
  ├── 上下文编译
  └── 导出与只读投影
      │
      ▼
本地权威存储
```

Notion 只消费 Core 生成的投影文件，不反向写权威状态。

目标依赖方向：

```text
cmd/novel-core
    ↓
internal/core
    ↓
internal/protocol   internal/domain   internal/store   internal/retrieval
```

`internal/core` 不得依赖 `internal/agents`、`bootstrap.ModelSet`、`agentcore`、`internal/llmcodex` 或任何模型提供方代码。

## 8. 项目生命周期

### 8.1 能力检查

`novel-core init` 先生成一次性的能力检查。ChatGPT 读取挑战文件，在约定目录写回包含 nonce 的确认文件；Drive Desktop 同步到本地后，Core 校验成功才允许进入正式建书。

能力检查至少证明：

- ChatGPT 能读项目文件；
- 能创建普通 UTF-8 `.json`、`.md` 文件；
- 能写入指定目录；
- Drive Desktop 能把文件完整同步到本地；
- Core 能按协议读取这些文件。

如果账号只能读不能写，必须在这里失败，不能拖到第 1 章才暴露。首发不把 Google Docs、Sheets 等富文档格式当作协议载体；不能稳定读写普通 UTF-8 文件时，就判定当前环境不支持这一运行模式。能力检查不是正式任务，也不进入权威状态。

### 8.2 首次建书

能力检查通过后，Core 生成 `foundation` 任务。ChatGPT 与作者共同提交：创作合同、世界设定、人物设定、全书方向、卷级规划、当前 Arc、结局约束、风格和平台约束。

Core 只检查结构、引用和明确硬约束。通过后形成第一份权威状态，并生成第 1 章任务。

### 8.3 正常写作

作者输入“继续”后，ChatGPT 只处理 READY 指向的当前尝试版本，并在同一轮里完成：

1. 读取上下文包和当前约束；
2. 按人物各自可见信息推演；
3. 形成章节约束；
4. 写正文并自审；
5. 必要时自行修改；
6. 形成故事事件和状态变化；
7. 如当前任务要求，附带滚动规划更新；
8. 写完整提交包，最后写 `manifest.json`。

Core 对账并形成本地不可变快照，再做校验和提交。只有 `ACCEPTED` 才推进权威状态并生成下一任务。

### 8.4 返工与协议重试

- 可修复的故事/状态硬错误产生 `REWRITE`：保留 `task_id`，生成新的 `attempt_id`，下一次“继续”仍修当前任务。
- 文件不完整、版本错误、nonce 错误等协议问题不算剧情返工；如果能恢复，Core 生成 `retry` 尝试版本。
- 被拒绝或作废的尝试版本永远不能污染后续权威状态。

### 8.5 需要作者裁决

`BLOCKED` 只用于“继续写之前必须由作者做选择”的情况，来源只有两类：

1. Core 能确定证明两个硬约束无法同时满足；
2. ChatGPT 在 `self_review.json` 中明确提出 `author_decision_required`，并引用具体硬约束、冲突点和可选方案。Core 只检查引用是否真实存在，不替 ChatGPT 判断文学层面的对错。

Core 为阻塞生成稳定 `block_id`。作者选择后，ChatGPT 通过控制消息提交 `block_resolution`；Core 校验后生成新的尝试版本。同步损坏、摘要漂移、文件缺失不属于 `BLOCKED`。

### 8.6 作者主动改稿

作者指令必须结构化声明作用范围：

- `future_plan`：只改变未来方向，不改已发生历史。Core 把它变成下一任务的待落实要求，由后续提交中的规划更新正式生效。
- `historical_revision`：要改已经验收的章节、事实或硬约束。Core 创建 `revision` 任务，进入历史修订与下游重放。

作者的自然语言指令本身永远不能直接修改权威状态。

## 9. 本地目录、Drive 工作区与写入权

### 9.1 本地权威目录

默认位置：

```text
~/.novel-core/projects/<project-id>/
```

保存权威状态、已验收正文、回执、提交日志、检查点、本地索引和导出元数据。删除 Drive workspace 不得造成这些内容丢失。

### 9.2 Drive 工作区

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

### 9.3 唯一写入方

| 区域 | 正式写入方 | 另一方 |
|---|---|---|
| `project.json`、`CHATGPT_PROTOCOL.md` | Core | ChatGPT 只读 |
| `exchange/READY.json`、`STATUS.json` | Core | ChatGPT 只读 |
| `exchange/outbox/**` | Core | ChatGPT 只读 |
| `exchange/inbox/**` | ChatGPT | Core 只读并处理 |
| `exchange/result/**` | Core | ChatGPT 只读 |
| `exchange/control/inbox/**` | ChatGPT | Core 只读并处理 |
| `exchange/control/result/**` | Core | ChatGPT 只读 |
| `published/**`、`projection/**`、`backup/**` | Core | ChatGPT/Notion 只读 |

ChatGPT 不得修改 READY、STATUS、outbox、result、published、projection 或 backup。Core 也不得回写并“修补”已经落盘的提交包；它只能接受、拒绝、作废或生成新尝试版本。

## 10. 任务与尝试版本

首发只保留三类正式任务：

- `foundation`：建立或重建基础权威状态；
- `chapter`：写一个新的章节；
- `revision`：修改已经进入权威状态的历史内容，并按需要重放后续章节。

不设置独立的 `rewrite`、`review`、`character_simulation`、`arc_plan`、`finalize` 任务。返工只是同一任务的新尝试版本；人物推演、自审和滚动规划都是完成当前任务时的内部工作。

每个尝试版本至少包含：

- project id；
- task id；
- task kind；
- attempt id；
- attempt reason：`initial` / `rewrite` / `retry` / `rebase`；
- target；
- base canon root；
- protocol version；
- task digest；
- required artifacts；
- completion nonce；
- created at。

同一项目任何时刻只允许一个活动正式尝试版本。只有 `exchange/READY.json` 当前指向的尝试版本有资格推进权威状态。

## 11. READY 与任务包

`exchange/READY.json` 是 ChatGPT 的唯一生产入口。它只包含定位当前工作的最小信息：项目、任务、尝试版本、目标、前态根、协议版本、任务摘要、完成 nonce 和状态。

每个尝试版本对应一个只读任务包：

```text
exchange/outbox/<task-id>/<attempt-id>/
├── task.json
├── context.json
├── constraints.json
├── canon_excerpt.json
└── recent_prose.md
```

首次建书任务可以按同一结构输出，只是正文相关内容为空或换成建书上下文。

`CHATGPT_PROTOCOL.md` 是长期协议的唯一正文来源，由 Core 按 `protocol_version` 生成。任务包只给当前任务的事实和约束，不复制第二套完整操作手册。

## 12. 提交包

### 12.1 章节与历史修订

```text
exchange/inbox/<task-id>/<attempt-id>/
├── chapter.md
├── chapter_contract.json
├── events.json
├── state_delta.json
├── self_review.json
├── planning_patch.json        # 当前任务要求或主动提供时才出现
└── manifest.json              # 最后写
```

历史修订仍使用同一提交形状；目标章节、修订模式和旧版本由任务包决定，不再造一套平行协议。

### 12.2 首次建书

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

### 12.3 manifest

`manifest.json` 必须最后写入，至少回显：

- project id；
- task id；
- attempt id；
- base canon root；
- protocol version；
- task digest；
- completion nonce；
- 实际文件清单。

ChatGPT 不负责计算 SHA-256。manifest 的作用只是声明“这个尝试版本已经写完”。

当前任务包会给出允许出现的文件白名单；未声明文件、绝对路径、目录型 artifact、路径规范化后重名的文件一律按协议错误处理。

## 13. Drive 对账与本地不可变快照

“最后写 manifest”只能表达写作顺序，不能假设 Drive 一定按同样顺序同步到本地。因此 Core 不直接在 Drive 路径上做正式校验。

处理顺序固定为：

1. 没有 manifest：`PENDING`；
2. manifest 已到但声明文件不齐：继续 `PENDING`；
3. 文件齐全后，Core 计算每个文件的实际摘要；
4. 经过可配置的静默间隔后再次扫描；只有文件集合和摘要完全相同，才形成本地不可变快照；
5. 正式校验只读取这份本地快照；
6. 同一尝试版本一旦锁定快照，Drive 后续再出现不同内容，标记为冲突，不替换已锁定快照；如果该尝试版本已经 `SETTLED`，只记录篡改/冲突，不反向改变已结算结果。

静默间隔只是防止读到正在同步的半文件，不依赖 mtime，也不用于判断业务先后。

交换层至少区分：

- `PENDING`：还不能形成稳定快照；
- `READY_TO_VALIDATE`：稳定快照已经形成；
- `INVALID`：身份、版本、nonce、文件清单或快照一致性出错；
- `SETTLED`：尝试版本已经得到正式处理结果。

相同尝试版本、相同快照摘要重复出现时必须幂等；READY 已切换后，旧尝试版本永远不能重新抢占。

## 14. 摘要、版本根与回执

Core 统一使用 SHA-256，并对需要结构化计算的 JSON 使用稳定字段顺序和规范化编码。

- `artifact_digest`：对收到的原始文件字节计算；
- `task_digest`：对只读任务包的规范化清单计算；
- `submission_digest`：对本地不可变快照的“逻辑路径 + 文件摘要”清单计算；
- `validation_digest`：对规范化校验结果计算；
- `canon_root`：对当前权威状态清单计算。

权威状态清单至少包含：

- schema version；
- 当前 revision；
- parent canon root；
- 当前结构化状态摘要；
- 当前已验收 artifact 摘要清单。

时间戳、日志文本、Drive mtime、机器路径等不稳定信息不得进入 `canon_root`。协议版本可以记录在回执和项目元数据中，但协议升级本身不应无缘无故改变小说状态根。

每次正式结算都写不可变回执，至少包含：任务/尝试版本、前态根、任务摘要、提交摘要、artifact 摘要、校验摘要、结果、后态根和结算时间。

`verify` 必须能够重新计算当前根，并验证回执链中的 `previous_root → new_root` 连续关系。回执不能与新 root 互相循环参与同一次 root 计算。

## 15. 提交事务、恢复与并发

### 15.1 单写者

一个本地项目同一时刻只能有一个会修改状态的 Core 进程。`serve`、`migrate`、`restore`、提交和修订必须竞争同一项目锁；只读 `status` 可以并行。

不能只依赖进程内 mutex。项目锁必须能阻止两个 Core 实例同时提交。

### 15.2 提交不变量

任何时候都必须满足：

- 一个尝试版本最多推进一次权威状态；
- 正文、事件、状态变化、进度、检查点和版本根属于同一次提交；
- READY 最后更新；
- 崩溃恢复后，要么确认本次提交完整成立，要么仍停在前一个根；
- 不能出现“正文已正式发布，但人物/时间线/资源仍是旧状态”的半提交。

Core 在本地维护可恢复的提交日志。具体阶段名可以服从现有存储层，但至少要能区分“准备”“应用中”“已经结算”，并允许幂等重放。

### 15.3 过期前态与并发提交

- 提交包的 `base_canon_root` 必须等于当前活动尝试版本绑定的前态；
- 相同提交重复处理只允许一次结算；
- 两个聊天同时写同一个尝试版本时，第一份形成稳定本地快照的内容锁定该尝试版本；之后不同内容按冲突处理；
- 控制消息也必须带前态根；前一个控制消息改变状态后，后到的旧前态消息按过期前态拒绝，不按文件到达时间强行套用。

## 16. 故事事件、ID 与状态变化

### 16.1 尝试版本内 ID 与永久 ID

ChatGPT 在提交包里只使用当前尝试版本内的临时 ID。新人物、地点、资源、冲突、伏笔、读者承诺和故事事件真正进入权威状态时，由 Core 分配永久 ID，并把 `local_id → canon_id` 映射写入回执。

被拒绝、作废或冲突的尝试版本不产生可引用的永久 ID。后续任务只能引用已经进入权威状态的永久 ID。

### 16.2 故事事件

凡是会影响未来写作的关键变化，都应有故事事件作为证据锚点。事件至少记录：

- 尝试版本内 event id；
- 故事时间或顺序；
- 地点；
- 参与者；
- 观察者或明确接收者；
- 可见范围；
- 后果；
- 证据类型。

正文中直接发生的事件应给一个短 `evidence_anchor`，Core 只检查这个片段是否能在 `chapter.md` 中机械匹配。

离屏事件必须显式标记 `offscreen`，并引用允许它发生的章节约束或既有硬规则。Core 只检查引用，不假装理解这段离屏剧情写得是否合理。

### 16.3 状态变化

ChatGPT 不重新提交完整世界状态，只提交相对前态的变化。会影响后续写作的变化必须引用：

- 本尝试版本中的故事事件；或
- 已经进入权威状态的事实/事件。

状态变化至少覆盖实际涉及的：时间、地点、身体状态、目标与承诺、知识、关系、资源、世界状态、伏笔、冲突和读者承诺。

Core 根据前态和变化计算新状态，禁止 ChatGPT 直接用一份“完整新世界状态”覆盖本地存储。

## 17. 章节约束

写正文前，ChatGPT 必须形成结构化 `chapter_contract.json`。首发至少包含：

- 章节号和 POV；
- 起始状态；
- 本章目的与主冲突；
- 当前读者问题；
- 必须推进/承接的 obligation id；
- 禁止改变的权威对象 ID；
- 允许的伏笔操作；
- 预期状态变化；
- 章末钩子；
- 目标长度区间；
- 当前任务要求的滚动规划义务。

Core 只检查可机械验证的字段。它能检查章节号、POV 声明、ID、字数和状态机，却不能判断正文是否真的“足够悬”“足够爽”。

## 18. 长篇一致性的确定性规则

### 18.1 人物知识

每个新增知识都必须有明确来源：

- 角色亲眼观察到的故事事件；
- 明确传播给该角色的事件；
- 已经进入权威状态的事实。

Core 可以检查观察者、接收者、来源链和前态是否存在。没有来源的结构化知识变化必须 `REWRITE`。

正文里是否偷偷泄露了 POV 不该知道的信息，仍由 ChatGPT 自审；Core 不做自然语言理解。

### 18.2 人物观察

Core 为当前重要角色编译局部观察，至少包含：当前位置、目标、压力、已知事实、明确未知/禁止知道的事实、资源、关系、承诺和当前行动。

人物观察只是当前上下文的派生物，不是第二套权威状态。

### 18.3 时间、地点和资源

Core 只验证已经结构化的规则：

- 故事时间不能违反已接受顺序；
- 同一人物不能在重叠时间区间出现在互斥地点；
- 已定义移动约束时必须满足最短耗时；
- 已建账的金钱、道具和消耗品不能凭空透支；
- 已锁定的伤势、能力、权限限制不能无事件依据消失。

没有建模两个地点之间的移动规则时，Core 不凭现实常识自行估算交通时间。

### 18.4 关系

关系不使用 `trust=72` 这类虚假高精度数字。至少保存：双方实际关系、各自认知、未履行承诺、最近证据事件和必要标签。

关系变化必须有事件证据；标签只是描述，不构成统一数轴。

### 18.5 伏笔、冲突和读者承诺

伏笔至少有稳定 ID、表层线索、真义、首次出现、强化记录、回收边界、依赖和后果。状态机应显式定义，例如：

```text
planned → seeded → reinforced → payoff_ready → paid_off → closed
```

允许明确的 `retired` / `misdirected` 分支，但不得跳过协议未允许的状态边。

冲突必须有稳定 ID、参与方、状态、升级/关闭条件和证据事件，不能无证据自动消失。

读者承诺至少允许 `advanced`、`fulfilled`、`deferred`、`retired`。标记 `fulfilled` 时必须引用已经验收的事件证据；Core 只证明“有明确证据引用”，不宣称语义上真的让读者满意。

### 18.6 结局约束

整书必须有结构化结局约束，至少覆盖：主线必须解决的问题、重要人物弧终点、核心关系终态、必须回收的伏笔、必须关闭的冲突、允许开放的尾声和明确禁止遗留的问题。

Core 只执行已经编码的硬条件，例如“某伏笔最晚在第 N 章前回收”“某 Arc 前必须关闭某冲突”“某阶段后禁止新增 major mystery”。它不能凭无 AI 代码判断“剩余篇幅肯定收不回来”。

硬结局约束未满足时，项目不能标记为完成，也不能进行 final export。

## 19. 上下文编译与滚动规划

### 19.1 上下文包

Core 不把整本小说重新塞给 ChatGPT。每个任务只带当前工作真正需要的内容：

- 相关世界与人物设定；
- 当前硬约束和结局约束；
- 当前卷、Arc 和章节义务；
- 活跃人物观察；
- 当前时间、地点、资源和世界状态；
- 活跃冲突、伏笔和读者承诺；
- 最近若干章摘要；
- 上一章必要尾部正文；
- 本地检索命中的旧片段。

预算按 UTF-8 字节数或 Unicode 字符数配置，不绑定某个模型 tokenizer。硬约束、知识边界和当前状态有保留额度，不能被旧正文挤掉。

第 100 章以后，上下文包大小仍受固定总上限约束，不能随全书长度线性增长。

首发优先复用结构化索引、关键词和现有本地检索能力；不要求 embedding、Qdrant 或新向量数据库。

### 19.2 滚动 Arc

正式连载前至少已有全书方向、卷级规划、当前 Arc 和结局约束。Core 在当前 Arc 结束前给任务加上滚动规划义务，ChatGPT 可通过 `planning_patch.json` 提交下一 Arc 方案。

滚动规划和正文分开验收：

- 规划更新合法：与本章一起进入当前有效生产状态；
- 规划更新有硬错误，但当前章不依赖这份新规划：正文仍可 `ACCEPTED`，规划更新单独拒绝，并把“补齐下一 Arc”列入后续任务硬前置；
- 到了下一 Arc 起点仍没有合法规划：下一章任务必须先修规划，再写正文；规划仍不合法时该尝试版本 `REWRITE`；
- 只有硬约束需要作者选择时才进入 `BLOCKED`。

回执必须单独记录可选规划更新的处理结果：`accepted`、`rejected` 或 `not_present`。被拒绝的规划文件仍保留在该次提交的审计摘要里，但不进入当前有效规划，也不进入权威状态的已验收产物清单。

这样不会为了“一章一次继续”把错误规划硬塞进权威状态，也不会为了一个可晚一章修的规划问题推翻已经合法的正文。

## 20. 三层校验与正式结果

### 20.1 协议层

检查版本、project/task/attempt 身份、manifest、nonce、文件白名单、任务摘要、前态根、活动尝试版本和本地快照完整性。

协议层失败只会得到 `PENDING`、`INVALID` 或新的 `retry` 尝试版本，不直接变成剧情 `BLOCKED`。

### 20.2 权威状态层

检查已经结构化的事实和约束，例如人物知识来源、时间地点、资源、不可改事实、关系、伏笔/冲突状态机和结局硬条件。

可修问题通常产生 `REWRITE`。

### 20.3 章节约束层

只检查本章能机械验证的硬要求，例如章节身份、POV 声明、目标长度、obligation 是否有事件证据、禁止修改的权威对象 ID 和允许的伏笔操作。

文学质量、情绪强度、节奏好坏和“读起来爽不爽”不属于这一层。

### 20.4 正式结果

只有通过协议层并形成稳定本地快照的提交包，才有资格得到正式结果：

- `ACCEPTED`：合法，推进权威状态；
- `REWRITE`：同一任务可修，生成新尝试版本；
- `BLOCKED`：继续前必须由作者裁决，不推进权威状态。

`REWRITE` 必须给出可执行反馈：violation code、涉及实体、expected、observed、证据引用和允许修改范围。不得只写“人物不合理，请修改”。

## 21. 控制消息

作者指令和阻塞裁决走独立控制通道：

```text
exchange/control/inbox/<message-id>/
├── control.json
└── manifest.json
```

首发 `kind` 只允许：

- `author_directive`；
- `block_resolution`。

控制消息必须带 project id、message id、base canon root，以及它引用的 task/chapter/block id。Core 按 message id 幂等处理；过期前态或过期 block id 必须拒绝。

控制消息不能直接改权威状态，只能改变任务队列、作废当前尝试版本，或创建正式修订任务。

## 22. 历史修订与下游重放

修改已经验收的历史章节，不能只改一章正文再“重算几个 JSON”。首发采用保守但正确的线性重放：

1. `revision` 任务明确最早受影响的已验收章节或基础 artifact；
2. 修订通过后，从该点建立新的权威状态分支；
3. 旧分支上更晚的已验收章节全部标记为 `superseded`，不再属于当前活动权威状态；
4. Core 按章节顺序生成后续 `revision` 尝试版本，任务包同时给出旧正文候选和新的前态；
5. ChatGPT 可以保留、局部改写或重写旧正文，但必须重新提交事件和状态变化；
6. 每一章重新 `ACCEPTED` 后，新分支才向前推进；
7. 重放追上原来的最新章节后，项目才恢复正常新章生产。

历史重放期间禁止 final export。首发只支持一条活动修订链，不做多分支合并，也不支持多人同时改同一历史。

只改变未来规划、没有碰已发生事实的作者指令，不启动历史重放。

## 23. 安全边界

Drive 中的所有文件都按不可信输入处理。Core 必须：

- 只读取协议白名单内的相对路径；
- 拒绝 `..`、绝对路径、路径穿越和越出 workspace 的链接；
- 不跟随提交包中的符号链接；
- 只接受协议允许的 UTF-8 文本文件；
- 对单文件大小、提交包总大小、JSON 深度和数组长度设置上限；
- 不执行提交包中的脚本、命令、HTML 或宏；
- 未知文件不进入权威状态；
- 日志不得记录连接凭证；
- Drive、Notion 等凭证不得写进小说项目、Prompt 或 Git 仓库。

## 24. 备份、恢复、迁移与投影

### 24.1 备份与恢复

可验证备份至少包含：当前权威状态快照、已验收 artifact 清单、回执链、schema version 和 canon root。

Drive 中的 `backup/` 只是备份副本，不自动拥有权威身份。恢复必须显式执行，并在写入新本地目录前完整验证摘要和回执链；校验失败时拒绝恢复，不做“尽量猜”。

### 24.2 schema 与协议迁移

本地 schema version 和 Drive protocol version 分开管理：

- 已知旧本地 schema：先生成可验证备份，再执行幂等迁移，并留下迁移回执；
- 未知更新主版本：拒绝打开；
- 协议不兼容升级后，旧尝试版本不得被新协议静默解释成另一种含义；
- 协议升级本身不应改变小说内容根，只有真实权威状态变化才产生新的权威状态 revision。

### 24.3 Notion

首发只要求 Core 能生成可选 `projection/notion.json` 或其他只读投影。Notion 更新失败不能阻止写作，Notion 中的人工修改也不能反向覆盖本地权威状态。

## 25. 当前代码的迁移方向

### 25.1 保留真正有价值的部分

重点保留现有：

- `internal/domain` 中可复用的小说领域数据；
- `internal/store` 及文件型持久化；
- Checkpoint、Timeline、Character/World state、Resource ledger、Planning data；
- 与模型无关的确定性规则、恢复逻辑和本地检索能力。

不另起一套平行数据库。

### 25.2 从 Tool 外壳里抽回确定性业务逻辑

现有 `internal/tools/commit_chapter.go` 同时包含 Tool schema、模型时代门禁和大量真正有用的提交/恢复逻辑。迁移时先把确定性部分收回 Core/Store，再删除旧壳，不能照搬整个 Tool。

目标调用方向：

```text
CLI / Reconciler → internal/core → domain/store/rules
```

首发不为了“以后可能还有别的后端”提前造 `TaskExporter`、`SubmissionImporter`、`Projector` 等单实现接口。出现第二个真实实现后再决定是否抽象。

### 25.3 旧 AI 状态

Usage、模型 Session、AIVoice 等旧数据可以为了读取历史项目短期保留，但新 Core 必须满足：

- 不要求这些数据存在；
- 不再写新的模型 Usage；
- 不把它们算进 canon root；
- 不让它们参与任务、校验或提交；
- 迁移完成后只作为兼容数据，或在确认无引用后删除。

### 25.4 最终退出生产运行时

以下内容最终从受支持的生产路径移除：`bootstrap.ModelSet`、Provider/failover、Coordinator/SubAgents、Writer/Drafter/Reviewer 运行时、`agentcore` 主循环、Codex/Ollama/LiteLLM dispatch、模型价格/Usage、provider-bound provenance 和依赖模型判断的 AIGC/AI voice 门禁。

删除顺序必须服从依赖图：先有新的无 AI 闭环，再移旧运行时，避免迁移过程中仓库长期不可运行。

Fork 继续保留上游 Apache-2.0 许可证和必要归属，不暗示得到上游作者背书。

## 26. CLI

首发支持：

- `novel-core init`：初始化本地项目、Drive workspace 和能力检查；
- `novel-core serve`：低频扫描、恢复、对账、校验、提交和下一任务准备；
- `novel-core status`：显示当前 revision、章节、活动任务/尝试版本、对账状态和阻塞原因；
- `novel-core verify`：重新验证权威状态根、回执链、已验收产物和活动指针；
- `novel-core export`：从当前活动权威状态导出书稿；
- `novel-core restore`：从可验证备份恢复到新的本地目录；
- `novel-core migrate`：执行显式 schema 迁移。

Core CLI 不提供 `--provider`、`--model`、`--api-key`。

## 27. 测试与真实产品验收

### 27.1 自动测试原则

优先从 `cmd/novel-core`、`core.Project` 和文件协议等公共入口测试。私有 helper 可以通过更高层行为覆盖，不为测试方便额外暴露内部接口。

自动测试至少覆盖：

- 无模型环境启动；
- capability ack；
- 首次建书 → 第 1 章；
- 正常章节 `ACCEPTED`；
- `REWRITE` 保持 task、切换 attempt；
- manifest 先到/文件后到；
- Drive 内容在稳定快照前变化；
- manifest 后内容变化；
- 非活动尝试版本和过期前态提交；
- 同一提交重复处理；
- 两个 Core 实例争抢写锁；
- 提交各阶段崩溃后的恢复；
- 人物知识无来源；
- 时间地点冲突；
- 资源不足；
- 非法伏笔状态跳转；
- 读者承诺没有证据却标记为 `fulfilled`；
- 未满足结局约束却请求 final export；
- 作者指令、`BLOCKED` 裁决和过期前态控制消息；
- 历史修订后下游章节没有重放；
- 备份损坏、未知 schema/protocol 主版本；
- 路径穿越、符号链接和超大输入；
- 上下文包在长篇下仍受固定预算约束。

CI 不调用 ChatGPT。

### 27.2 真实 ChatGPT 验收

发布前必须使用普通 ChatGPT App + Google Drive + Drive Desktop 实测完整链路：能力检查、首次建书、第 1 章、连续章节、一次 REWRITE、Core 重启、新聊天恢复、滚动 Arc、一次历史修订与下游重放、一次 BLOCKED 裁决、结局收束、`verify` 和完整导出。

人工项只能记 `PASS`、`FAIL` 或 `NOT RUN`，不能拿 Core fixture 测试冒充真实 ChatGPT 验收。

## 28. 明确不做

首发不包含：

- OpenAI API 或其他模型 API 自动生成；
- Ollama、本地模型、MCP、ChatGPT Work；
- 浏览器/UI 自动化和无人值守整本生成；
- 高频轮询模拟 ChatGPT 自动触发；
- 完整 Event Sourcing 平台或新数据库前置依赖；
- 必需的向量数据库；
- Notion 双向同步权威状态；
- 多人并发编辑和历史分支合并；
- Core 用自然语言理解判断文学质量；
- Core 自动推断未建模的现实世界常识；
- 伪造 provider/model 调用证据；
- AIGC 检测规避；
- 平台审核、读者接受或推荐流量保证。

## 29. 完成定义

以下条件必须全部满足：

- Novel Core 的受支持生产路径完全不调用 AI；
- 没有任何模型 API Key 也能初始化、运行、校验、恢复和导出；
- `cmd/novel-core` 的依赖图不包含 `agentcore`、`ModelSet`、`internal/agents`、`internal/llmcodex` 或模型提供方代码；
- 本地存储是唯一权威来源，Drive/Notion/聊天历史都不能直接改它；
- 任务、尝试版本、提交包、回执和版本根只有一套一致语义；
- Drive 提交先形成稳定本地不可变快照，再进入正式校验；
- 同一项目有本地单写者保护，重复/并发提交不会双重结算；
- 首次建书、第 1 章、返工、恢复、滚动规划、作者裁决、历史修订和下游重放形成完整闭环；
- 人物知识、时间地点、资源、关系、伏笔、冲突、读者承诺和结局约束有明确可检查边界；
- 上下文包受固定预算约束，不随全书长度线性增长；
- 备份、恢复和迁移可验证；
- `verify` 能重算当前根并验证回执链；
- 自动测试通过；
- 真实普通 ChatGPT 产品验收通过。

设计收束只有四个核心业务概念：**任务、提交包、权威状态、回执**。尝试版本只是任务的执行版本；故事事件是提交包进入权威状态时的证据；控制消息只是作者意图进入正式任务的入口。任何新增模块或协议，如果不能明显增强这四个概念之一，就不进入首发范围。
