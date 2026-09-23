# Upstream vs Provider-Free Fork Capability Audit — 2026-09-23

## 1. 审计范围与基线

本审计比较当前两个真实 Git 状态，而不是依赖旧聊天摘要：

- Provider-free fork：meilee10006/novel-studio@55a4a26c84fc6615fc5a790fad6c8e7b60e2566a，分支 provider-free-novel-core
- Upstream：Xiaoyangy/novel-studio@882717402b1b9deb96607156176d1aaa96abc977，分支 main
- 二者 merge-base：e3beebbf2f35b9ff55fd781d82055b60d45970c8
- fork 自 merge-base 后有 147 个提交；upstream 自 merge-base 后有 56 个提交
- 当前 Git tree：fork 174 个文件，upstream 1883 个文件；共同路径 51 个

文件数量本身不代表质量。本审计只把源码、协议、测试和当前文档能够证明的产品能力列入结论。

一个关键事实是：Brainstorm、Architect、outline-all、zero-init、角色 Agent、World Arbiter、Editor、web research、craft recall、RAG、render packet、Dashboard、eval/quality、上下文管理和 Project-All 的代表实现路径，在 merge-base 当时已经存在，而在当前 fork 中不存在。因此这些主要属于 provider-free 重构时主动移除的能力，不是 upstream 后来新增而 fork 尚未来得及同步。

## 2. 总结结论

Provider-free fork 不是 upstream 的“轻量版同产品”，而是已经转成了另一个产品边界：

- upstream 是 AI 小说生产系统：负责从想法开发、设计、大纲、角色/世界推演、章节规划、渲染、审核到交付；
- fork 是 provider-free 小说权威状态核心：普通 ChatGPT 负责全部语义创作，Core 负责 Canon、硬约束、事务、恢复、修订和导出。

这个重构显著增强了确定性、恢复、安全和 provider 独立性，但同时产生了一个很大的中间真空：
~~~text
作者的模糊想法
  ↓
普通 ChatGPT 自由讨论 / 自行推断流程
  ↓
最小 Foundation
  ↓
确定性 Novel Core
~~~

upstream 原来位于“想法”和“可进入正文的正式设计”之间的大量语义生产能力没有 provider-free 等价物。当前大纲反复纠偏与这个缺口有直接结构性关系。

## 3. 能力矩阵

| 能力域 | Upstream 当前能力 | Fork 当前能力 | 客观变化 | 影响 |
|---|---|---|---|---|
| 故事开发 / Brainstorm | 专门 Brainstorm Agent；研究、逻辑推敲、核心冲突、主角目标、爽点、差异化、禁区、Architect handoff；产出稳定 brainstorm.md | 无专门阶段；由普通 ChatGPT 自由讨论后直接进入 Foundation | 缺失 | 高：最容易把题材、金手指、爽点误当故事主线 |
| 创作合同稳定化 | brainstorm.md / prompt.md 可作为后续全部规划的稳定依据 | 前置创作判断没有独立稳定 artifact | 弱化 | 高：纠偏容易只存在于聊天上下文 |
| Architect 长篇设计 | premise、人物、世界规则、世界法典、book world、分层大纲、compass 等完整设计 | Foundation 只强制极小字段，可附加任意创作字段但 Core 不要求 | 显著弱化 | 高：故事发动机、中期转向、关系主线等没有前置门禁 |
| 世界机制可推敲性 | world_codex v2 + book_world v2，含机制、代价、失败、可见性、耗时、反事实探针、路线、资源、势力钟 | 通用 world.json + entity；可选 travel constraints；Core 强在已结构化事实连续性 | 设计能力缺失，运行一致性保留/增强 | 高 |
| 全书大纲 | outline-all、全书章位冻结、layered outline、outline contract | book_plan.direction + 可选 current_arc；滚动规划 | 显著弱化 | 高 |
| Zero-init | 把完整 Foundation 转成可推演的开局状态，并做写前设计门禁 | Foundation ACCEPTED 后直接创建 Chapter 1 task | 缺失 | 高 |
| Arc rehearsal | 整弧条件预演，区分预演与正式角色决定 | 无语义预演；只有滚动 Arc 结构与 planning obligation | 缺失 | 中高 |
| Project-All / seal | 全书导航 → 当前窗口细推 → seal → promote → render | 章节 task + chapter contract + rolling planning | 弱化 | 中高 |
| 角色独立决策 | 稳定 Character Agent 身份、私有观察、独立行动、记忆、事件激活 | Core 编译角色局部观察和知识来源；真正行动由同一个 ChatGPT 完成 | 语义能力缺失，知识边界校验增强 | 中高 |
| World Arbiter | 独立裁决角色行动碰撞、世界机制、资源、时间、地点 | Core 只机械验证已提交的结构化变化 | 缺失 | 中高 |
| Planner / Drafter 分离 | Planner 决定事实与章节任务；Drafter 只消费 sealed render packet | 同一个 ChatGPT 在一次 Chapter task 内规划、写作、自审、提交 | 弱化 | 中高 |
| Render packet | 不可变最小渲染输入，限制 Drafter 临场改故事 | 无等价 sealed semantic packet；有 Core context + chapter contract | 部分缺失 | 中 |
| Editor / Reviewer | Editor + 独立 Reviewer + exact-body/digest 绑定审核 | self_review.json 由同一个 ChatGPT 自审；Core 只校验结构 | 显著弱化 | 高 |
| 章节文学质量门禁 | 故事成立、读感、结构、连续性等多层语义审核 | 明确不做文学判断 | 缺失（设计选择） | 高 |
| Web research | Agent 可在 Brainstorm/Architect 等阶段主动联网研究现实支架与题材 | Core 无联网研究；需当前 ChatGPT 会话主动使用外部搜索 | 自动化缺失 | 中 |
| Craft recall | 写法库/套路/大纲模板结构化召回 | 保留 assets/references 和 assets/styles，但不是运行时依赖 | 显著弱化 | 中高 |
| RAG | BM25 + embedding/Qdrant；source ref、content-addressed receipt、snapshot、trace | 基础关键词排序；task context 内检索旧正文；无向量运行时 | 显著弱化 | 中 |
| 上下文治理 | token-aware 压缩、CJK 估算、恢复包、memory policy、context receipt、多 Agent profile | 固定 64 KiB task context budget；Canonical 压缩 + recent prose + keyword retrieval | 简化/弱化 | 中 |
| 短篇专项 | short-story skills、短篇 finalize/review 合同 | 产品明确定位长篇 Core，无短篇专项工作流 | 缺失 | 低到中 |
| 质量审计 / eval | eval harness、quality audit、content lint、风格/重复等质量工具 | 大量 Core 协议与状态测试，但无语义创作 eval 体系 | 创作 eval 缺失；Core test 增强 | 中高 |
| Dashboard | 规划、角色 Agent、正文、RAG、usage、错误、恢复可视化 | CLI status/verify + Drive STATUS；无 Dashboard | 弱化 | 低到中 |
| 诊断 / doctor | doctor、diag、RAG health、模型检查等 | status、verify、协议错误反馈 | 范围缩小 | 低到中 |
| Provider / 模型路由 | 多 provider、角色独立模型、fallback、成本与 token accounting | 完全无 provider/runtime；普通 ChatGPT 是唯一智能层 | 主动移除 | 对自动化是缺失；对 provider-free 目标是优势 |
| 无人值守生产 | pipeline 可自动调多个 Agent 连续推进 | 必须由普通 ChatGPT App 读写 Drive；Core 不主动调用 AI | 缺失（设计选择） | 中 |
| Outline repair / successor generation | 语义级大纲修复、generation successor、rebase 等 | future_plan + historical revision/replay；无独立语义 outline repair Agent | 部分替代，仍有缺口 | 中高 |
| Canon 完整性 | 多层 Store/receipt/accepted canon | 单一 canon_root、receipt chain、重算 verify | fork 增强 | 高正向 |
| Drive 同步一致性 | 非核心产品路径 | manifest + quiet period + immutable local snapshot + post-lock conflict detection | fork 显著增强 | 高正向 |
| Canon ID 生命周期 | 上游有大量稳定 ID/结构，但 provider runtime 管理复杂 | attempt-local ID，只有 ACCEPTED 后 Core 铸造 canonical ID | fork 增强/简化 | 高正向 |
| 历史修订 | upstream 有 rebase/restart 等生产恢复能力 | 明确 historical_revision，从最早受影响章节分叉并 replay 后续；未追平禁止 export | fork 更明确、更确定 | 高正向 |
| 作者裁决 | upstream 有 pipeline 干预/steer | BLOCKED + stable block_id + block_resolution control channel | fork 增强/收敛 | 中高正向 |
| 备份 / 恢复 / 迁移 | 有 archive/recovery 等机制 | 可验证 backup、只恢复到新目录、迁移前强制备份、未知 schema/protocol fail closed | fork 增强 | 高正向 |
| 输入安全 | 常规本地 pipeline 安全 | Drive 输入视为不可信；路径穿越、绝对路径、symlink、未知 artifact、超限输入拒绝 | fork 增强 | 高正向 |
| 最终导出 | upstream 当前也支持经过验证的长篇导出 | Core 从当前 Canon head 确定性导出；未终态冲突/伏笔/承诺或 replay 会阻断 | 能力均存在，fork 更偏确定性 | 正向 |

## 4. 当前 Foundation 为什么不足以替代 upstream 设计阶段

fork 的协议明确允许增加创作字段，但机械必需字段非常少：
- foundation.json：title、protagonist、opening_location
- characters.json：至少一个带 name 的人物
- world.json：可选 entities；实体有类型、local id、name
- book_plan.json：direction
- ending_contract.json：main_resolution
- style_profile.json：language
- platform_profile.json：platform

测试中的最小 Foundation 只要满足这些结构就可以 ACCEPTED 并直接进入 Chapter 1。

相比之下，upstream 写前设计明确覆盖：

- premise
- characters
- world_rules
- world_codex
- book_world
- outline / layered_outline
- timeline
- relationship_state
- foreshadow_ledger
- compass
- world coherence report

因此 fork 的问题不是“不能保存丰富设定”，而是没有任何阶段负责保证这些丰富设定在进入 Chapter 1 前已经被系统性地产生、审核和锁定。

## 5. 与当前“大纲反复纠偏”最直接相关的缺口

### 5.1 缺少 Story Development gate

upstream Brainstorm 要先回答“这本书为什么有人追、靠什么持续兑现”，再交给 Architect。

fork 没有这个 gate，ChatGPT 容易直接从模糊需求跳到题材标签、金手指、升级机制和爽点循环。因此“主线”和“爽感制作手段”容易混层。
### 5.2 缺少稳定 Creative Brief

作者已经纠正过的原则如果只留在聊天里，下一轮发散仍可能重新解释。

需要一个正式 artifact 区分：

- 已锁定原则
- 当前候选
- 已否决方向
- 可重新打开的决策

### 5.3 缺少开发者与审核者的逻辑分工

现在是同一个 ChatGPT：

~~~text
提出方案 → 自己解释方案为什么合理 → 自己继续优化
~~~

缺少 upstream Editor/Reviewer 类似的独立反证阶段，因此更容易局部自洽。

### 5.4 Foundation 接受条件太偏“可解析”，不是“可开写”

Core 的这个边界本身正确，但如果前面没有语义设计门禁，ACCEPTED 很容易被误解成“故事已经设计好了”。

实际含义只是“Core 能证明这组结构化输入合法”。

### 5.5 缺少全书骨架与中期换挡门禁

upstream Architect 明确要求故事引擎、中期转向、升级路径、终局命题等。

fork 可以把这些写进 book_plan.json，但不要求、不生成、不审核。于是早期方向看似成立，几十章后的可持续性没有在写前得到压力测试。

## 6. 除大纲之外，最重要的客观弱化
### 6.1 人物“不会违规”不等于人物“会自主活起来”

fork 对人物知识来源、地点、关系、资源等机械连续性的保护很强。

但 upstream Character Agent + World Arbiter 解决的是另一个问题：人物先按自己的目标和有限信息行动，剧情再接受这些行动的后果。

fork 当前由一个 ChatGPT 同时知道故事计划、人物目标和写作目标，虽然提交时要满足知识边界，但更容易发生“人物为了作者计划而选择最方便行动”的语义偏置。

### 6.2 世界“没有硬矛盾”不等于世界“可推演”

fork 能防时间冲突、资源不足、非法状态跳转。

upstream world_codex/book_world 进一步要求机制的触发、成本、失败模式、可观察性、耗时和反事实测试。缺失后，世界规则可能在文本层面听起来合理，却没有形成稳定的因果机器。

### 6.3 正文“状态正确”不等于正文“故事成立”

fork 的 chapter contract 已经有 purpose、main_conflict、reader_question、ending_hook 等字段，这是很有价值的。

但 Core 明确不判断这些字段在正文里是否真正兑现。没有独立 Editor/Reviewer 时，可能出现“字段写得很好，正文实际没做到”的情况。

### 6.4 参考资料保留了，但召回能力没有保留

fork 保留 assets/references 和 assets/styles，但 README 明确它们不是运行时依赖。

因此“仓库里有资料”和“创作时在正确阶段自动拿到正确资料”是两回事。upstream 的 craft recall/RAG 正是在解决后者。
### 6.5 没有创作能力回归测试

fork 的测试可以很好证明协议、状态、恢复和安全没有退化。

但如果修改 Story Development、Foundation 或写作协议，目前没有 eval 能回答：

- 故事概念是否更稳定
- 是否更少把金手指当主线
- 大纲是否更能支撑中长篇
- 人物是否更少服务剧情
- Review 是否真的能发现开发阶段错误

这是后续恢复语义能力时必须补的测试层。

## 7. Fork 相比 upstream 不应被忽略的增强

这次对比不能得出“回退到 upstream 就更好”。fork 已经建立了一组很有价值、而且符合当前目标的基础：

1. AI 与权威状态彻底解耦：Core 不依赖 provider/model。
2. Canon root 可重算：状态完整性不依赖聊天或模型自述。
3. Drive 稳定快照：多文件非原子同步不会直接污染正式提交。
4. attempt-local → canonical ID：失败尝试不会泄漏永久身份。
5. REWRITE 不污染后态：返工保持 task、切新 attempt。
6. 历史 revision/replay：旧章修改有明确下游失效和重放语义。
7. 作者裁决 channel：只有真正硬冲突才 BLOCKED。
8. 可验证 backup/restore/migrate：恢复和升级都有明确安全边界。
9. 固定 task context budget：避免上下文无限增长。
10. 终局状态机：伏笔、冲突、读者承诺不闭合就不能最终导出。
11. 不伪造模型证据：没有调用就不记录 provider/model/token。
12. 输入视为不可信：文件协议具备明确安全边界。
## 8. 不建议恢复的 upstream 复杂度

### 8.1 Provider / Model runtime

不建议恢复 bootstrap.ModelSet、多 provider adapter、fallback、模型价格、usage accounting、agentcore 主循环。

原因：普通 ChatGPT 已经承担智能层，这些会重新破坏 provider-free 边界。

### 8.2 多 Agent 本地编排基础设施

不必恢复“程序启动多个 LLM Agent”。

应该恢复的是 Agent 的职责分工、阶段边界、artifact 和审核逻辑，让普通 ChatGPT 在不同阶段扮演这些逻辑角色。

### 8.3 Embedding/Qdrant 作为首要依赖

长期可能有价值，但不是当前大纲漂移的根因。优先恢复创作协议和结构化 artifact，比先恢复向量基础设施收益更高。

### 8.4 Dashboard 作为当前优先项

Dashboard 是可观测性增强，不会解决故事开发方向错误。可以后置。

### 8.5 AIGC/AI voice 类自动门禁

fork 当前明确不让 Core 冒充文学审稿人，这个原则应继续保持。语义审核可以由 ChatGPT 负责，Core 只验证审核产物的完整性和版本绑定。

## 9. 推荐的 provider-free 恢复方向

目标不是“恢复 upstream”，而是补齐下面这一层：
~~~text
作者
  ↓
Story Development Protocol
  ├─ Creative Brief
  ├─ Reference / Motherbook Analysis
  ├─ Story Candidates
  ├─ Story Review
  └─ Locked Story Concept
  ↓
Design / Architect Protocol
  ├─ Premise
  ├─ Characters
  ├─ World Rules / Mechanisms
  ├─ Whole-book Skeleton
  ├─ Midpoint / Phase Transitions
  └─ Ending Contract
  ↓
Foundation Gate
  ↓
Novel Core
  ↓
Chapter Planning / Draft / Review Protocol
~~~

关键原则：

- ChatGPT 负责语义判断；
- Core 负责状态和可机械证明的约束；
- 每个语义阶段必须有正式 artifact；
- 阶段之间有输入/输出边界；
- 已锁定决策默认不可被后续阶段偷偷重开；
- Review 是独立阶段，不与 generation 混在一起；
- 不要求恢复 provider、agentcore 或多模型 runtime。

## 10. 建议优先级
### P0 — 直接解决当前反复纠偏

1. 增加 creative_brief 正式 artifact。
2. 增加 Story Development 分阶段协议。
3. 增加 Story Review，至少检查“故事 vs 机制/爽点混层”。
4. 增加 decision lock / rejected direction 记录。
5. Foundation 不再是“想得差不多就提交”的第一道正式门。

### P1 — 恢复写前设计能力

1. 把 upstream premise 中真正有价值的字段映射进 provider-free 设计层：核心冲突、主角目标、核心兑现承诺、故事引擎、关系/成长主线、升级路径、中期转向、终局命题。
2. 恢复 whole-book skeleton / layered planning artifact。
3. 增加 Foundation readiness 的语义自审清单；Core 只检查“已提交并版本绑定”，不判断内容好坏。
4. 把 chapter 的 Plan → Draft → Review 逻辑阶段分开，即使仍由同一个 ChatGPT 完成。

### P2 — 提高长篇质量和效率

1. 增加可选 Reference/Craft retrieval。
2. 增加 Arc rehearsal 的 provider-free 版本。
3. 增加 Character perspective / decision artifact，与全局作者态输入隔离。
4. 增加语义 eval fixtures，专门回归创作能力。
5. 最后再考虑 Dashboard、向量 RAG 或其他可观测性增强。

## 11. 最终判断
从“长篇小说系统”的完整能力看，当前 fork 客观上缺失或弱化的不只是 Brainstorm/Outline Agent，而是 upstream 原有的整个语义生产前半程和语义质量闭环：

~~~text
选题开发
→ 设计
→ 世界可推敲
→ 全书骨架
→ 开局初始化
→ 弧预演
→ 人物独立决策
→ 世界裁决
→ 章节规划
→ 渲染
→ 独立审核
~~~

fork 保留下来的主要是另一半：

~~~text
任务
→ 提交
→ 硬约束验证
→ Canon
→ 状态连续性
→ 返工
→ 历史修订
→ 恢复
→ 备份/迁移
→ 确定性导出
~~~

因此当前最合理的演进方向不是重新引入“庞大的 AI runtime”，而是在现有 Novel Core 前面和章节任务内部补回 provider-free 的语义工作流。

这样可以同时保留 fork 已经得到的确定性优势，又恢复 upstream 中真正影响小说质量、尤其是故事开发与大纲稳定性的客观能力。
