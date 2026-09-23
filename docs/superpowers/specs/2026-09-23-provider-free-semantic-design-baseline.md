# Provider-Free 语义设计基线与证据链规格

状态：设计已收敛，待审阅
日期：2026-09-23
目标分支：provider-free-novel-core
依赖规格：docs/superpowers/specs/2026-09-13-provider-free-novel-core-design.md
能力审计：docs/upstream-fork-capability-audit-20260923.md

## 1. 目标

在不恢复任何模型提供方、Agent runtime 或本地 AI 调度的前提下，把 upstream 中最影响建书质量的语义能力重新接回当前 provider-free 流程：

- Brainstorm：先把“想写什么故事”想清楚，再进入设定和大纲；
- Architect：把已确认的故事概念展开成人物、世界、全书结构、结局、风格和平台设计；
- outline-all：在第 1 章之前形成可支撑长篇的全书骨架；
- zero-init / architect-check：在正式建立 Canon 前完成一次独立的建书就绪审查；
- web research / craft recall：允许研究资料和母本分析作为可追溯来源参与设计，但不把它们变成 Core 的强制 AI 依赖。

本规格只解决“建书前的语义设计缺口”。章节级 Character Agent、World Arbiter、Planner/Drafter 分离、Editor/Reviewer、Arc rehearsal 和运行时 RAG 不在本期恢复范围。

目标不是复刻 upstream 的执行拓扑，而是保留当前 fork 的确定性边界，同时找回 upstream 已经证明有价值的创作方法和检查点。

## 2. 与既有规格的关系

本规格补充 2026-09-13 的 Provider-Free Novel Core 设计，并覆盖其中“能力检查通过后直接生成 foundation 任务”的旧流程。

以下原则保持不变：

1. ChatGPT App 是唯一智能层；
2. Novel Core 不调用任何模型；
3. Google Drive 仍然只是交换层；
4. Canon 仍然只有 foundation、chapter、revision 三类正式结算；
5. Core 只判断可以确定证明的结构、引用、版本和硬约束；
6. 文学质量、故事判断和审稿意见由 ChatGPT 与作者负责；
7. chapter / revision / historical replay / backup / restore / migrate / export 的既有语义不因本规格重写。

本规格新增的语义设计层发生在第一份 Canon 形成之前。第一份 Foundation 一旦 ACCEPTED，设计基线冻结；连载中的未来方向调整和历史改稿继续使用既有 future_plan 与 historical_revision，不再建立第二套长期滚动规划权威。

## 3. 核心结论

系统增加一个很小的“语义设计存储”，但不增加 Workflow Engine、Capability Graph、Scheduler 或通用 DSL。

ChatGPT 仍然按创作协议自由讨论、发散、比较、审稿和收敛。Core 只保存重要的不可变产物，并在跨越关键检查点时核对：

- 引用的是不是精确版本；
- 审稿是不是针对这一版；
- 作者确认是不是针对这一版；
- 当前设计包里的硬依赖是不是彼此一致；
- 是否缺少规定的检查点证据；
- 是否满足现有 Foundation 的确定性结构约束。

整体关系：

~~~text
作者
  │
  ▼
ChatGPT App
  │ 创作方法：讨论 / 发散 / 比较 / 审稿 / 收敛
  ▼
语义设计层
  ├─ 不可变设计产物
  ├─ 审稿与作者确认
  ├─ 设计包
  ├─ 设计提交
  └─ design_root
       │
       │ 确定性建书转换
       ▼
Novel Core
  ├─ foundation
  ├─ chapter
  ├─ revision
  └─ canon_root
~~~

design_root 回答“这本书在正式开写前决定怎么写”。

canon_root 回答“这本书已经正式成立了什么”。

二者不能互相替代。

## 4. 明确不做什么

首版禁止为了这项能力顺手建设以下系统：

- 不恢复 provider/model 配置；
- 不恢复 Agent runtime、agent loop 或多 Agent 本地调度；
- 不增加 story_development、brainstorm、architect、review 等 Core TaskKind；
- 不建设通用 Workflow/Run/Phase 状态机；
- 不保存 current_phase、phase_attempt、phase_transition 之类的权威游标；
- 不建设通用 DAG resolver；
- 不建设 YAML/JSON Capability DSL；
- 不建设 Scheduler、Queue、Worker；
- 不因为参考资料发生变化就自动使整个设计树失效；
- 不要求 embedding、Qdrant 或向量数据库；
- 不恢复 Dashboard 作为本期前置条件；
- 不把 ChatGPT 的 PASS 当成 Core 对文学质量的证明；
- 不让 design_root 在正式连载期间与 Canon 的滚动规划形成双重权威。

如果将来需要自动推荐下一步，只能作为从当前设计事实推导出的非权威投影加入，不能改变本规格的持久状态模型。

## 5. 统一术语

| 中文术语 | 协议/代码名 | 含义 |
|---|---|---|
| 设计产物 | design artifact | ChatGPT 产生、Core 内容寻址保存的不可变语义材料 |
| 设计引用 | artifact_ref | Core 根据真实内容生成的稳定引用；ChatGPT 不自行计算 |
| 硬输入 | inputs | 当前产物成立所依赖的精确上游版本；参与一致性检查 |
| 来源 | sources | 对当前产物有帮助的资料来源；只做追溯，不自动触发失效 |
| 设计包 | design bundle | 一组被提议共同组成当前设计的精确产物引用 |
| 证据 | evidence | 针对精确产物或设计包的审稿、作者确认或检查结果 |
| 设计提交 | design commit | 把一个设计包及其必要证据正式确认为当前设计基线的不可变记录 |
| 设计根 | design_root | 对设计提交的确定性摘要 |
| 设计头 | design head | 当前正式采用的 design_root；Foundation 接受后冻结 |
| 建书设计根 | foundation_design_root | 产生第一份 Canon 的最终 design_root |
| 设计状态投影 | DESIGN_STATUS | Core 根据持久事实生成的只读状态，不是流程游标 |

“证据”只表示“存在一份绑定到精确版本的结构化判断或确认”，不表示 Core 能证明文学判断客观正确。

## 6. 三层信息边界

### 6.1 探索材料

包括：

- 尚未采用的故事候选；
- 母本和参考资料分析；
- 被否决方向；
- 临时比较；
- 研究笔记；
- 尚未通过审稿的设计草稿。

探索材料可以长期保存，也可以丢弃。它们不会进入 Canon，也不会因为存在就自动改变当前设计。

### 6.2 设计基线

由当前 设计提交明确选择的产物组成。

设计基线代表作者和 ChatGPT 在正式开书前已经确认的设计，不等于小说中已经发生的事实。

设计提交只能引用 Core 已经内容寻址保存的产物和证据。

### 6.3 Canon

Canon 仍然由既有 Novel Core 管理。

语义设计层不能直接修改人物状态、故事事件、关系、资源、知识、伏笔、冲突、读者承诺或已验收正文。

Foundation 接受后，建书设计根作为来源信息冻结；后续章节造成的滚动规划变化仍属于既有 Canon 生产状态。

## 7. 设计产物模型

### 7.1 内容寻址

ChatGPT 不计算 SHA-256。

ChatGPT 把文件交给 Design Inbox 后，Core：

1. 对 Drive 文件做与现有 Submission 相同级别的路径、类型、大小和稳定快照检查；
2. 在本地不可变快照上计算真实摘要；
3. 保存对象；
4. 返回 artifact_ref。

建议对外引用形式：

~~~text
<artifact-type>@sha256:<digest>
~~~

具体本地目录布局属于实现细节，但必须支持：

- 同内容去重；
- 不可变读取；
- 摘要重算；
- backup / restore；
- verify；
- schema version 检查。

### 7.2 设计产物的最小元数据

每个设计产物至少具有：

- artifact_type；
- schema_version；
- payload；
- inputs：零个或多个精确 artifact_ref；
- sources：零个或多个精确 artifact_ref；互联网、书目、母本等外部来源必须先导入为 source_record 设计产物；
- supersedes：可选，表示它明确取代哪个旧设计产物。

Core 计算 artifact_ref 时必须把 artifact_type、schema_version、inputs、sources、supersedes 和真实 payload 一起纳入确定性摘要。

因此同一段文字如果改用了不同的硬输入，仍然是不同的设计产物。

### 7.3 inputs 与 sources 必须区分

inputs 表示“如果这个上游版本换了，当前产物不能直接拿来证明新设计仍然成立”。

sources 表示“这个资料曾经帮助形成当前判断，但资料列表后来变化，不会自动让已经确认的设计失效”。

例子：

~~~text
story_concept
  inputs:
    creative_brief@...
    design_decisions@...
  sources:
    motherbook_analysis@...
~~~

更换母本分析不会自动废掉已经锁定的 story_concept。

如果作者决定根据新母本重新开发故事，应产生新的 story_concept，而不是修改旧产物。

这个区分是硬约束，不能用一个笼统的 dependency 字段代替。

## 8. 首版设计产物

首版只定义支撑建书所需的最小集合。

### 8.1 creative_brief

记录作者真正想解决的问题和边界，不负责写故事答案。

至少包含：

- 写作目的；
- 目标类型/平台约束；
- 已明确偏好；
- 已明确禁区；
- 当前必须保留的作者原则。

### 8.2 design_decisions

记录已经做出的设计决定。

至少区分：

- locked：后续设计必须遵守；
- rejected：已经明确否决，后续不得在没有重新确认的情况下偷偷恢复；
- open：尚未决定，但可以继续探索。

每条决定必须有稳定本地 id。重新打开旧决定时，不改旧记录，而是新建一条决定并通过 supersedes 指向旧决定。

建书就绪时不得存在标记为 blocking 的 open 决定。

### 8.3 story_concept

回答“这本书到底讲一个什么故事”，而不是把题材、金手指、爽点循环或升级手段本身当作主线。

至少包含：

- story：用直接语言说明故事本身；
- protagonist_goal；
- central_conflict；
- story_engine：什么持续制造事件和选择；
- change_path：主角、关系或处境会如何发生长期变化；
- ending_direction。

story_concept 必须把当前 creative_brief 和 design_decisions 作为 inputs。

母本、市场资料和写作参考放在 sources，不默认成为硬输入。

### 8.4 七个建书设计文件

Architect 能力不再单独对应 Agent 或 Task，而是把 story_concept 展开成现有 Foundation 能消费的七个文件：

- foundation.json；
- characters.json；
- world.json；
- book_plan.json；
- ending_contract.json；
- style_profile.json；
- platform_profile.json。

这些文件在进入 Canon 前只是设计产物。

它们可以比当前 Core 最低结构要求丰富，但最终必须仍然能通过现有 Foundation 的确定性校验。

### 8.5 book_plan 的额外建书要求

语义设计层的 book_plan 不能只写 direction。

首版至少要求包含：

- direction；
- whole_book_skeleton；
- 至少两个明确阶段；本项目定位长篇，不为单阶段作品放宽这条建书门槛；
- 每个阶段的主要目标或问题；
- 阶段之间发生了什么不可忽略的变化；
- 结局如何与 ending_contract 接上。

Core 只检查这些字段存在、类型正确、引用有效，不判断结构是否精彩。

### 8.6 world 的额外建书要求

如果世界机制会直接决定剧情可行性，设计稿应描述：

- 触发条件；
- 前置条件；
- 代价或限制；
- 效果；
- 失败边界；
- 可观察性；
- 时间条件。

并非所有世界都必须人为造复杂机制。建书就绪审查负责判断哪些机制需要这些说明，Core 不猜。

## 9. 审稿与作者确认

### 9.1 Review 必须是独立产物

Review 不与被审核内容写在同一个文件。

Review 至少包含：

- review_type；
- subject_ref；
- policy_version；
- verdict：PASS / REVISE / AUTHOR_DECISION_REQUIRED；
- findings；
- created_from_context_refs。

Core 只接受指向已经存在 artifact_ref 的 review。

Review 对其他版本没有效力。

### 9.2 Story Review

story_review 至少检查：

1. story 字段是否真的是故事，而不是题材、设定、金手指或爽点手段；
2. 主角是否有明确而可持续推进的目标；
3. central_conflict 是否能制造长期选择与后果；
4. story_engine 是否能持续产生事件，而不是重复同一种爽点；
5. change_path 是否提供长期变化空间；
6. ending_direction 是否与前面的故事自然相连；
7. 是否违反当前 locked / rejected decisions；
8. 是否只是参考母本的表层换皮。

这些判断由 ChatGPT 完成。Core 只核对 review 的结构、版本和 subject_ref。

### 9.3 建书就绪审查（foundation_readiness_review）

foundation_readiness_review 针对整个 design bundle，而不是单个文件。

至少检查：

1. story_concept 与七个建书设计文件是否互相一致；
2. book_plan 是否真的承接 story_concept；
3. 全书骨架是否能走到 ending_contract；
4. 主要人物的目标、关系和资源是否足够支撑开局；
5. 世界机制是否足够明确，不需要写到中途再临时发明关键规则；
6. 第 1 章开局条件是否已经具备；
7. locked / rejected decisions 是否被遵守；
8. 是否仍有会阻止开写的 open decisions；
9. 是否存在明显依赖旧版本设计的文件；
10. 是否把商业手段、爽点、金手指或设定误当成故事本身。

### 9.4 作者确认

story_concept 进入故事锁定检查点前必须有作者明确确认。

作者确认是一个单独的结构化 evidence，绑定精确 story_concept 的 artifact_ref。

协议必须明确：只有作者在当前对话中明确选择或确认该版本后，ChatGPT 才能写这份 evidence。Core 能证明的是“这个确认文件针对哪个版本”，不能证明自然语言确认本身的真实性。

不得把“作者没有反对”当成确认。

### 9.5 不伪装成独立模型审核

首版允许同一个 ChatGPT 在不同步骤完成生成和 Review。

所谓“独立 Review”只表示：

- 单独产物；
- 单独输入边界；
- 精确版本绑定；
- 不允许 Review 顺手修改 subject。

系统不得声称使用了第二模型、独立 Agent 或隐藏评审器。

## 10. 设计包

设计包（Design Bundle）是一个不可变清单，只保存精确引用，不复制正文。

示意：

~~~json
{
  "schema_version": 1,
  "selections": {
    "creative_brief": "creative_brief@sha256:...",
    "design_decisions": "design_decisions@sha256:...",
    "story_concept": "story_concept@sha256:...",
    "foundation": "foundation@sha256:...",
    "characters": "characters@sha256:...",
    "world": "world@sha256:...",
    "book_plan": "book_plan@sha256:...",
    "ending_contract": "ending_contract@sha256:...",
    "style_profile": "style_profile@sha256:...",
    "platform_profile": "platform_profile@sha256:..."
  }
}
~~~

故事锁定时允许只选择前三类建书前产物。

建书就绪时必须选择完整集合。

### 10.1 一致性闭包

Core 校验设计包时：

1. 所有引用对象必须真实存在；
2. artifact_type 必须和 selection slot 匹配；
3. 每个对象的 inputs 必须真实存在；
4. 如果某个 input 的 artifact_type 也出现在当前 selections 中，则 input_ref 必须等于当前 bundle 选择的那个 ref；
5. supersedes 只记录版本关系；导入一个后继候选不会自动废掉当前 design head，只有新的 设计提交才能改变活动选择；
6. 同一个 selection slot 只能选择一个 artifact_ref，不能同时选择前后两个版本；
7. sources 只要求可追溯，不参与第 4 条的一致性替换规则。

因此：

~~~text
bundle 选择 story_concept v5
premise / book_plan 仍硬依赖 story_concept v4
~~~

必须被机械拒绝。

Core 不需要知道“下一步应该重写哪一个文件”，只返回不一致的精确引用。

## 11. 两个正式设计检查点

首版不建立通用阶段系统，只定义两个检查点。

### 11.1 故事锁定（story_locked）

必须满足：

- creative_brief 已选；
- design_decisions 已选；
- story_concept 已选；
- story_concept 的 inputs 指向当前 creative_brief 与 design_decisions；
- 有 PASS 的 story_review，subject_ref 精确指向当前 story_concept；
- 有作者确认，subject_ref 精确指向当前 story_concept。

通过后 Core 可以生成一个 设计提交，并把 design head 指向新的 design_root。

故事锁定之后仍允许作者在 Foundation 前反悔。反悔不修改旧提交，而是产生新产物、新 Review、新确认和新的 设计提交。

### 11.2 建书就绪（foundation_ready）

必须满足：

- 当前链上已经存在有效 故事锁定；
- 建书就绪的设计包中，creative_brief、design_decisions、story_concept 必须与当前 story_locked 设计提交选择的三个引用完全相同；如果其中任一版本改变，必须先重新完成故事锁定；
- 设计包选择完整十个设计槽位；
- bundle 通过一致性闭包；
- 七个建书设计文件能通过现有 Foundation 的确定性预校验；
- book_plan 满足本规格的 whole_book_skeleton 最低结构要求；
- 没有 blocking open decision；
- 有 PASS 的 foundation_readiness_review，subject_ref 精确指向当前 bundle_ref。

通过后 Core 生成新的 设计提交。

没有第三个通用检查点系统。将来若要恢复章节级语义能力，另开规格论证，不能直接把首版扩成任意工作流平台。

## 12. 设计提交与 design_root

设计提交（Design Commit）至少包含：

- schema_version；
- checkpoint：story_locked 或 foundation_ready；
- parent_design_root；
- bundle_ref；
- evidence_refs；
- policy_versions。

设计提交不包含时间戳等与语义无关的可变元数据。

design_root 使用 Canonical JSON 对 设计提交的语义字段做确定性序列化后计算 SHA-256。

提交时间、处理机器、日志位置写入独立 receipt，不参与 design_root。

### 12.1 Design Head 的 CAS 规则

推动 design head 必须带 expected_design_root。

只有：

~~~text
expected_design_root == 当前 design head
~~~

才允许更新。

陈旧 ChatGPT 会话、重复提交或两个会话并行尝试推动设计时，后到的陈旧提交必须失败，不得覆盖新 head。

导入普通设计产物不需要持有 design head；只有 promote 设计提交需要 CAS。

### 12.2 Foundation 后冻结

第一份 Foundation ACCEPTED 后：

- design head 不再允许推进；
- 最终 head 固化为 foundation_design_root；
- 项目状态永久保留 foundation_design_root；
- 后续 future_plan、planning_patch、historical_revision 不重写 foundation_design_root。

这样不会与现有 Canon 中的滚动规划形成双重权威。

## 13. Foundation 的衔接

### 13.1 不让 ChatGPT重复抄一次已批准设计

建书就绪已经选择了七个精确设计文件，因此不应再要求 ChatGPT 把同样内容重新写一遍。

Core 在建书就绪推进前先对七个文件运行现有 Foundation 的确定性预校验。

推进成功后，Core 从本地 Design Store 读取这七个精确 payload，创建一个正常的内部 foundation Task/Attempt，并把七个 payload 物化为本地不可变输入快照。这个内部尝试不发布 ChatGPT-facing READY，也不要求 ChatGPT 再写一份 manifest。

随后实现应把现有 Foundation 的“校验”和“提交”逻辑抽成可复用入口，由内部 foundation attempt 与旧的外部 FoundationSubmission 路径共同调用。同一套 ID mint、ref rewrite、receipt 和 Canon commit 规则只能有一份。

不得为了复用旧接口伪造“ChatGPT 已提交 manifest”的证据，也不得另写第二套 Foundation 提交器。

### 13.2 可恢复顺序

建书就绪的落地顺序：

1. 所有设计对象和 evidence 已经本地不可变保存；
2. 对候选设计提交和七个 Foundation 文件做完整 dry validation；
3. CAS 推进 design head；
4. 写设计提交回执；
5. reconcile 发现当前 head=foundation_ready 且尚无 Canon；
6. 创建或恢复唯一的内部 foundation Task/Attempt，并从 Design Store 物化本地不可变输入快照；
7. 通过共享的 Foundation validator/commit 路径创建第一份 Canon；
8. 把 foundation_design_root 写入项目状态与 Foundation receipt；
9. 创建 chapter:1 READY。

如果在第 3 至第 7 步任意位置崩溃，重启后必须根据 design head、Design receipt 和 Canon 是否存在恢复，不创建第二份 Foundation。

Foundation settlement 必须保持幂等。

### 13.3 Foundation 校验失败不应发生在推进之后

建书就绪推进必须复用与真正 settlement 相同的确定性验证逻辑。

如果 dry validation 失败：

- 不推进 design head；
- 不创建 Canon；
- 返回具体机械错误；
- ChatGPT 产生新的设计产物后重试。

这要求把现有 Foundation 校验从“只能边结算边验证”收敛为可复用的纯校验入口，而不是复制规则。

## 14. ChatGPT 创作协议

Core 不保存创作流程游标，但 CHATGPT_PROTOCOL.md 必须给出方法顺序。

建议的默认方法：

~~~text
理解作者意图
→ creative_brief
→ 必要时做母本 / 现实资料研究
→ 发散故事候选
→ 与作者比较和收敛
→ design_decisions
→ story_concept
→ story_review
→ 作者确认
→ 故事锁定
→ Architect：人物 / 世界 / 全书骨架 / 结局 / 风格 / 平台
→ foundation_readiness_review
→ 建书就绪
→ Core 自动建立第一份 Canon
→ Chapter 1
~~~

这是“推荐创作方法”，不是持久状态机。

作者可以在 Foundation 前回到前面的讨论重新考虑；系统只通过新不可变产物和新设计提交记录最终变化。

### 14.1 不允许偷偷跳过检查点

ChatGPT 可以自由安排探索对话，但不能：

- 没有 故事审稿和作者确认就声称故事已锁定；
- 建书就绪使用没有被故事锁定采用的 story_concept；
- Review 一个旧版本后提交另一个版本；
- Foundation Readiness Review 后偷偷改七个文件；
- 用聊天记忆替代 design_decisions；
- 把被明确 rejected 的方向当成未决定方向继续推进。

## 15. 研究资料与母本

母本、市场资料、现实资料和 craft reference 属于可选 source artifact。外部 URL、书名或其他来源先形成 source_record，再被其他设计产物通过 sources 引用。

首版支持的最低能力：

- 保存来源名称、来源类型、定位信息、必要摘录摘要和分析结论；
- 允许 story_concept 或其他设计产物通过 sources 引用；
- 保留来源追溯；
- 不要求 embedding；
- 不自动把 source 变化传播成设计失效。

如果来源来自互联网，ChatGPT 负责研究和引用；Core 不内建浏览器，也不验证网页内容真实性。

如果未来恢复本地 craft recall/RAG，只能作为帮助 ChatGPT找到 source artifact 的工具，不能改变 design_root 的权威规则。

## 16. Drive 交换协议

Design 层复用现有 Drive 的“外部输入不可信”原则，但不复用 READY Task 语义。

新增：

~~~text
exchange/design/
├── STATUS.json
├── inbox/
│   └── <submission-id>/
│       ├── ...
│       └── manifest.json
└── result/
    └── <submission-id>.json
~~~

### 16.1 写入权

- STATUS.json：Core 写，ChatGPT 只读；
- design/inbox：ChatGPT 写，Core 只读并快照；
- design/result：Core 写，ChatGPT 只读。

### 16.2 Design submission 操作

首版只允许两种操作：

1. import：导入一个或多个设计产物、evidence 或 design bundle；Core 对 design bundle 返回 bundle_ref；
2. promote：提交一个已存在 bundle_ref + evidence_refs，请求推进某个检查点。

不增加通用 action language。

### 16.3 导入（import）

导入（import）可以批量带多个互不依赖或只引用既有 artifact_ref 的文件。

首版不支持 submission-local ref 自动解析。

如果 Review 需要绑定刚产生的 story_concept：

1. 先 import story_concept；
2. Core 返回 artifact_ref；
3. 再 import review。

这会多一次交换，但能避免 Core 维护第二套临时引用解析和重写逻辑。

### 16.4 推进（promote）

promote 请求至少包含：

- project_id；
- submission_id；
- expected_design_root；
- checkpoint；
- bundle_ref；
- evidence_refs；
- protocol_version；
- completion_nonce。

Core 验证成功后返回：

- result；
- previous_design_root；
- new_design_root；
- receipt_ref；
- 如为建书就绪，返回后续 Canon settlement 状态。

### 16.5 STATUS 不是流程游标

STATUS 可以包含：

- design_head；
- foundation_design_root；
- 是否已有故事锁定；
- 是否已有建书就绪；
- 是否已形成 Canon；
- 最近一次 design result；
- 当前协议版本。

不得包含 current_phase 或“强制下一步节点”。

协议文档可以给 ChatGPT建议，但 status 只投影已经成立的事实。

## 17. 上下文与跨会话恢复

新 ChatGPT 会话恢复时不依赖旧聊天摘要。

它读取：

1. CHATGPT_PROTOCOL.md；
2. exchange/design/STATUS.json；
3. 当前 design head 指向的设计提交、bundle 和必要产物；
4. 尚未采用但作者希望继续处理的候选 artifact_ref，如有。

正式恢复依据是 Design Store，不是“上次聊到第几步”。

为了方便继续未确认探索，STATUS 可以列出最近若干未被设计提交选择的候选引用，但这只是便利投影。候选列表丢失不能改变 design head。

## 18. 与现有 Canon 任务的边界

### 18.1 新项目

新项目流程改为：

~~~text
capability check
→ semantic design
→ 故事锁定
→ 建书就绪
→ Core 自动完成 Foundation 结算
→ chapter:1 READY
~~~

能力检查通过后不再立即创建 ChatGPT-facing foundation READY。

### 18.2 已有 Canon 的项目

升级时绝不伪造历史设计证据。

已有 Foundation 或章节的项目：

- 保持现有 Canon、task、revision 和 export 行为；
- foundation_design_root 为空；
- design_origin 标记为 legacy；
- 不要求补造 Story Review、作者确认或设计提交；
- 本规格不阻塞其继续写作。

首版不提供“把旧书逆向补齐 design_root”的自动工具。

如以后确有需求，应单独设计 legacy adoption，并明确它只是“登记当前设计”，不能冒充过去已经做过的 Review。

## 19. 与 upstream 能力的映射

本期只恢复职责，不恢复运行时。

| Upstream 能力 | 本规格中的落点 |
|---|---|
| Brainstorm | creative_brief + design_decisions + story_concept + 故事审稿 |
| Architect | 七个建书设计文件 |
| outline-all | book_plan.whole_book_skeleton |
| zero-init / architect-check | 建书就绪审查 + Foundation 确定性预校验 |
| web research | 来源产物（source artifact），由 ChatGPT 使用现有联网能力完成 |
| craft recall | 可选来源产物；不要求 RAG runtime |
| Agent handoff | artifact_ref / bundle_ref / 设计提交，不靠聊天口头交接 |
| exact-version review | Review.subject_ref |
| recovery | 内容寻址 Design Store + 设计提交链 + design head |

以下能力明确延期：

- Character Agent；
- World Arbiter；
- Arc rehearsal；
- Planner / Drafter 分离；
- chapter-level Editor / Reviewer；
- sealed render packet；
- embedding / Qdrant；
- Dashboard；
- provider/model routing；
- token/cost accounting。

延期不是放弃。后续恢复必须先证明它不能用现有 Task/Artifact/Receipt 边界表达，再考虑增加新抽象。

## 20. 本地存储与完整性

建议在现有项目本地权威目录下增加：

~~~text
meta/core/design/
├── objects/
├── bundles/
├── commits/
├── receipts/
└── HEAD.json
~~~

具体分片目录可由实现决定，但逻辑对象必须分开。

### 20.1 HEAD.json

只保存：

- schema_version；
- design_root；
- checkpoint。

Foundation 后增加冻结标志或由项目状态推导 FROZEN，不在 HEAD 内保存流程信息。

### 20.2 Verify

novel-core verify 至少新增：

- 所有 Design object 摘要可重算；
- bundle 引用对象存在；
- commit bundle/evidence 引用存在；
- 设计提交摘要可重算；
- parent chain 无环、无断裂；
- HEAD 指向真实 commit；
- Foundation receipt 中的 foundation_design_root 与冻结 head 一致；
- legacy 项目允许 foundation_design_root 为空。

Design verify 失败不允许悄悄从 Drive 重建。

### 20.3 Backup / Restore

Core backup 必须包含 Design Store、Design receipts 和 HEAD。

Restore 完成后必须同时验证 design_root 链和 canon_root 链。

### 20.4 Migration

未知 Design schema version 必须 fail closed。

任何会改写 Design Store 格式的迁移必须遵守现有“先备份再迁移”原则。

不得为了迁移方便重新计算语义内容或制造缺失 evidence。

## 21. 安全边界

Design Inbox 使用与现有 Submission 至少同级的安全策略：

- 禁止绝对路径；
- 禁止 .. 路径穿越；
- 禁止 symlink；
- 禁止目录型 artifact；
- 禁止路径规范化后重名；
- 限制 manifest、单文件、总提交大小和 JSON 深度；
- 只接受 UTF-8 文本和协议明确允许的扩展名；
- manifest 必须最后写；
- Drive 文件稳定后才锁本地快照；
- 同一 submission_id 锁定后内容改变，标记冲突而不是覆盖；
- 不信任 ChatGPT 提供的摘要。

promote 是状态变更，必须在项目锁内完成。

导入（import）可以并行存在，但对象最终仍以真实内容摘要去重。

## 22. 失败语义

### 22.1 导入失败

协议或安全错误只拒绝本次 import，不改变 design head 和 Canon。

### 22.2 Review 版本错误

Review.subject_ref 不存在或类型不匹配时直接 INVALID。

旧版本 Review 不自动迁移到新版本。

### 22.3 Bundle 不一致

返回精确的 slot、artifact_ref 和冲突 input_ref。

不得把它升级成作者 BLOCKED，因为这是可修复的设计包问题。

### 22.4 promote CAS 冲突

返回 STALE_DESIGN_HEAD，并给出当前 design_root。

ChatGPT 必须重新读取 STATUS 再决定是否重做，不允许 Core 自动 rebase 语义设计。

### 22.5 建书就绪的机械校验失败

不推进 design head，不创建 Canon。

返回现有 Foundation validator 的具体错误。

### 22.6 Foundation settlement 崩溃

依赖既有事务恢复与新增 design receipt 恢复。

恢复后只能产生一个 Canon root 和一个 Foundation receipt。

## 23. 可观察性

首版不建 Dashboard。

novel-core status 至少增加：

- design_mode：required / legacy；
- design_head；
- design_checkpoint；
- foundation_design_root；
- design_object_count；
- design_verified；
- foundation_state。

Drive 的 DESIGN STATUS 只投影这些必要信息。

不展示假 token、假模型、假 Agent 进度。

## 24. 测试要求

实现必须覆盖以下高风险场景。

### 24.1 精确版本

- story_review(v1) 不能证明 story_concept(v2)；
- foundation_readiness_review(bundle-A) 不能证明 bundle-B；
- 修改 payload 后 artifact_ref 必须变化；
- inputs 改变而 payload 不变时 artifact_ref 仍必须变化。

### 24.2 决策与来源

- design_decisions 改变后，仍绑定旧 decisions 的 story_concept 不能组成新 故事锁定；
- 新增 source artifact 不自动使已锁定 story_concept 失效；
- 仅仅导入一个带 supersedes 的新候选不能改变已有 design head；只有成功 promote 的新设计提交才能改变活动选择。

### 24.3 Bundle 一致性

- 新 story_concept + 仍依赖旧 story_concept 的 book_plan 必须拒绝；
- slot 类型错配必须拒绝；
- 缺失 input object 必须拒绝；
- source_ref 指向不存在对象时必须拒绝；新增其他 source artifact 不得自动使已有设计提交失效。

### 24.4 检查点

- 无 PASS story_review 不能 故事锁定；
- 无明确作者确认不能 故事锁定；
- 有 blocking open decision 不能 建书就绪；
- 审稿 PASS 后修改七个文件必须导致旧 readiness review 失效；
- 建书就绪必须跑现有 Foundation dry validation。

### 24.5 并发与恢复

- 两个会话从同一 design_root 同时 推进，只允许一个成功；
- 重复 推进同一 commit 必须幂等；
- design head 已写、Foundation 尚未 settlement 时崩溃，重启必须继续而不是重复建书；
- Drive submission 锁定后被改写不能污染本地对象。

### 24.6 兼容

- 已有 Canon 的旧项目升级后继续写作；
- legacy 项目不生成虚假 design_root；
- 新项目没有 建书就绪时不能进入 chapter:1；
- 新项目 Foundation 成功后 design head 冻结；
- 冻结后 Design 推进必须拒绝，并指向既有 future_plan / historical_revision 路径。

### 24.7 完整性

- backup / restore 后 design_root 与 canon_root 都保持一致；
- verify 能发现 Design object、bundle、commit、receipt 任一被篡改；
- 不存在 provider/model/token/tool-call 等伪造字段。

## 25. 完成标准

本规格实现完成的最低标准：

1. 新项目能力检查通过后进入语义设计，而不是直接 Foundation；
2. ChatGPT 能导入 creative_brief、design_decisions、story_concept 并获得 Core 生成的 artifact_ref；
3. 故事审稿和作者确认必须绑定精确 story_concept；
4. 故事锁定可以跨会话恢复；
5. Architect 产出的七个建书设计文件进入 Design Store；
6. 建书就绪审查必须绑定精确设计包；
7. Core 能发现新旧设计文件混用；
8. 建书就绪推进 使用 design head CAS；
9. Core 从已批准七文件自动复用既有 Foundation settlement 建立第一份 Canon，不要求 ChatGPT重复抄写；
10. Foundation receipt 保存 foundation_design_root；
11. chapter:1 只有在 Foundation ACCEPTED 后才出现；
12. Foundation 后 Design Head 冻结；
13. 已有项目升级不伪造历史设计证据；
14. verify、backup、restore 覆盖 Design Store；
15. 全流程没有任何模型/provider/runtime 依赖。

## 26. 后续演进原则

只有当前语义设计层经过真实建书验证后，才考虑恢复章节级 upstream 能力。

未来优先复用本规格已经验证的三个机制：

- 不可变 artifact；
- 精确版本 review/evidence；
- 既有 Core Task/Attempt/Receipt。

例如 Character Decision 可以先尝试作为 chapter 内的不可变 artifact，而不是新增 Character Task；Editor Review 可以先尝试绑定 exact chapter draft，而不是新增 Reviewer Agent。

只有当真实用例证明这三个机制无法表达所需生命周期时，才允许提出新 TaskKind 或新的状态机抽象。

本规格的长期约束是：

> 创作方法可以持续进化，持久状态只记录已经形成的产物、精确版本和正式检查点；不要把“怎么想”重新做成一个需要维护的 AI runtime。
