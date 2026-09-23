# Provider-Free 语义设计基线与证据链规格

状态：设计已收敛，实施计划已生成，待实现
日期：2026-09-23
目标分支：provider-free-novel-core
依赖规格：docs/superpowers/specs/2026-09-13-provider-free-novel-core-design.md
能力审计：docs/upstream-fork-capability-audit-20260923.md
实施计划：docs/superpowers/plans/2026-09-23-provider-free-semantic-design-baseline.md

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
| 设计头 | 设计头 | 当前正式采用的 design_root；Foundation 接受后冻结 |
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

ChatGPT 把文件交给 design/inbox 后，Core：

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
    story_decisions@...
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

creative_brief.payload 固定使用：

~~~json
{
  "purpose": "为什么要写这本书",
  "target_genre": "目标类型",
  "platform_constraints": [],
  "preferences": [],
  "exclusions": [],
  "author_principles": []
}
~~~

确定性要求：

- purpose、target_genre 必须是非空字符串；
- platform_constraints、preferences、exclusions、author_principles 必须是字符串数组；
- 数组内条目必须是非空字符串且不得重复；
- 数组允许为空，Core 不替作者发明偏好、禁区或原则。

### 8.2 story_decisions

只记录在“故事锁定”之前、会直接约束 story_concept 的作者决定，不充当全书通用决策日志。Architect 阶段的人物、世界和结构细化直接进入对应七个设计文件；只有细化结果反过来改变故事本身时，才重新故事锁定并产生新的 story_decisions。

记录已经做出的故事层决定。

至少区分：

- locked：后续设计必须遵守；
- rejected：已经明确否决，后续不得在没有重新确认的情况下偷偷恢复；
- open：尚未决定，但可以继续探索。

story_decisions.payload 使用固定结构：

~~~json
{
  "decisions": [
    {
      "id": "story-decision-001",
      "status": "locked",
      "statement": "故事层决定",
      "blocking": true,
      "supersedes": ""
    }
  ]
}
~~~

- decisions 必须是数组；
- id 必须是本产物内唯一的非空字符串；
- status 只允许 locked / rejected / open；
- statement 必须是非空字符串；
- blocking 必须是 boolean；
- supersedes 可省略或为空；非空时只能引用同一 payload 中另一个 decision.id，且不能自指；
- 重新打开旧决定时不改旧条目，而是新增一条决定并用 supersedes 指向被取代的 decision.id。

故事锁定和建书就绪时都不得存在 blocking=true 且 status=open 的故事层决定。

### 8.3 story_concept

回答“这本书到底讲一个什么故事”，而不是把题材、金手指、爽点循环或升级手段本身当作主线。

story_concept.payload 固定包含：

- story：用直接语言说明故事本身；
- protagonist_goal；
- central_conflict；
- story_engine：什么持续制造事件和选择；
- change_path：主角、关系或处境会如何发生长期变化；
- ending_direction。

这六个字段都必须是非空字符串。Core 只检查字段和类型，不判断“story 是否真的是好故事”；这项语义判断由 story_review 负责。

story_concept 必须把当前 creative_brief 和 story_decisions 作为 inputs。

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

首版不建设通用依赖图，但为这七类文件固定最小 provenance：

- characters、world、ending_contract 的 inputs 必须包含当前 story_concept；
- foundation 的 inputs 必须包含当前 story_concept、characters 和 world；
- book_plan 的 inputs 必须包含当前 story_concept、characters、world 和 ending_contract；
- style_profile、platform_profile 的 inputs 必须包含当前 creative_brief；需要时可额外引用 story_concept；
- 任何文件都可以增加 sources，但 sources 不替代上述 inputs。

因此故事版本变化后，旧 characters/world/book_plan 等不能靠省略依赖关系继续混入新的建书设计。

### 8.5 book_plan 的额外建书要求

语义设计层的 book_plan 不能只写 direction。

首版固定使用下面的最低机器结构；可以增加其他创作字段，但不能换名替代这些字段：

~~~json
{
  "direction": "全书总方向",
  "whole_book_skeleton": {
    "stages": [
      {
        "id": "stage-1",
        "objective": "这一阶段要解决的主要目标或问题",
        "transition": "这一阶段结束后发生的不可忽略变化"
      },
      {
        "id": "stage-2",
        "objective": "下一阶段的主要目标或问题",
        "transition": "如何进入最终收束"
      }
    ],
    "ending_connection": "全书骨架如何接到 ending_contract"
  }
}
~~~

确定性要求：

- direction 必须是非空字符串；
- whole_book_skeleton 必须是对象；
- stages 必须是长度至少 2 的数组；
- 每个 stage 的 id、objective、transition 都必须是非空字符串，id 在数组内唯一；
- ending_connection 必须是非空字符串。

Core 只检查这些字段存在、类型正确和局部结构有效，不判断阶段设计是否精彩，也不判断 ending_connection 的文学合理性；后者属于建书就绪审查。

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

### 9.1 审稿必须独立成产物

Review 不与被审核内容写在同一个文件。

Review 至少包含：

- review_type；
- subject_ref：按 review_type 指向精确 artifact_ref 或 bundle_ref；
- policy_version；
- verdict：PASS / REVISE / AUTHOR_DECISION_REQUIRED；
- findings；
- created_from_context_refs。

Core 只接受指向已经存在且类型符合 review_type 的不可变 subject。story_review 必须指向 story_concept 的 artifact_ref；foundation_readiness_review 必须指向 design bundle 的 bundle_ref。

审稿结果对其他版本没有效力。AUTHOR_DECISION_REQUIRED 只是设计层审稿结果，不创建 CoreBlock，也不使用现有 Canon 任务的 BLOCKED 状态。

### 9.2 故事审稿（story_review）

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

作者确认是一个单独的 author_confirmation 设计产物，至少包含：

- subject_ref：精确指向被确认的 story_concept artifact_ref；
- decision：固定为 APPROVED；
- note：可选，用于记录作者确认时的简短说明。

协议必须明确：只有作者在当前对话中明确选择或确认该版本后，ChatGPT 才能写 author_confirmation。Core 能证明的是“这个确认文件针对哪个版本”，不能证明自然语言确认本身的真实性。

不得把“作者没有反对”当成确认。

### 9.5 不伪装成独立模型审核

首版允许同一个 ChatGPT 在不同步骤完成生成和 Review。

所谓“独立审稿”只表示：

- 单独产物；
- 单独输入边界；
- 精确版本绑定；
- 不允许 Review 顺手修改 subject。

系统不得声称使用了第二模型、独立 Agent 或隐藏评审器。

## 10. 设计包

设计包是一个不可变清单，只保存精确引用，不复制正文。ChatGPT 不计算 bundle_ref；Core 对校验后的 selections 做确定性序列化并计算摘要，返回 bundle_ref。相同 selections 得到相同 bundle_ref。

示意：

~~~json
{
  "schema_version": 1,
  "selections": {
    "creative_brief": "creative_brief@sha256:...",
    "story_decisions": "story_decisions@sha256:...",
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
5. supersedes 只记录版本关系；导入一个后继候选不会自动废掉当前设计头，只有新的设计提交才能改变活动选择；
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
- story_decisions 已选；
- story_concept 已选；
- story_concept 的 inputs 指向当前 creative_brief 与 story_decisions；
- story_decisions 中没有 blocking=true 且 status=open 的决定；
- 有 PASS 的 story_review，subject_ref 精确指向当前 story_concept；
- 有作者确认，subject_ref 精确指向当前 story_concept。

通过后 Core 可以生成一个设计提交，并把 设计头指向新的 design_root。

故事锁定之后仍允许作者在 Foundation 前反悔。反悔不修改旧提交，而是产生新产物、新审稿、新确认和新的设计提交。

### 11.2 建书就绪（foundation_ready）

必须满足：

- 当前链上已经存在有效故事锁定；
- 建书就绪的设计包中，creative_brief、story_decisions、story_concept 必须与当前 story_locked 设计提交选择的三个引用完全相同；如果其中任一版本改变，必须先重新完成故事锁定；
- 设计包选择完整十个设计槽位；
- bundle 通过一致性闭包；
- 七个建书设计文件能通过现有 Foundation 的确定性预校验；
- book_plan 满足本规格的 whole_book_skeleton 最低结构要求；
- 没有 blocking open decision；
- 有 PASS 的 foundation_readiness_review，subject_ref 精确指向当前 bundle_ref。

通过后 Core 生成新的设计提交。

检查点推进只允许一条线性历史：

- 初始 设计头 为空，只能推进到 story_locked；
- story_locked 可以基于当前 story_locked 再次推进到新的 story_locked，用于 Foundation 前重新锁定故事；
- foundation_ready 的 parent_design_root 必须是当前 story_locked；
- foundation_ready 成功后不再接受新的设计提交，只允许完成随后的 Foundation 结算与崩溃恢复；
- 首版没有设计分支、merge、自动 rebase 或并行 head。多个候选可以同时存在于 设计存储，但只有 CAS 成功的线性 head 是正式设计历史。

没有第三个通用检查点系统。将来若要恢复章节级语义能力，另开规格论证，不能直接把首版扩成任意工作流平台。

## 12. 设计提交与 design_root

设计提交至少包含：

- schema_version；
- checkpoint：story_locked 或 foundation_ready；
- parent_design_root；
- bundle_ref；
- evidence：按固定角色名映射到精确 evidence artifact_ref；
- checkpoint_policy_version。

设计提交不包含时间戳等与语义无关的可变元数据。

design_root 使用 Canonical JSON 对设计提交的语义字段做确定性序列化后计算 SHA-256。

提交时间、处理机器、日志位置写入独立 receipt，不参与 design_root。

### 12.1 设计头的 CAS 规则

推动 设计头 必须带 expected_design_root。

只有：

~~~text
expected_design_root == 当前设计头
~~~

才允许更新。

陈旧 ChatGPT 会话、重复提交或两个会话并行尝试推动设计时，后到的陈旧提交必须失败，不得覆盖新 head。

导入普通设计产物不需要持有 设计头；只有 promote 设计提交需要 CAS。

### 12.2 Foundation 后冻结

第一份 Foundation ACCEPTED 后：

- 设计头不再允许推进；
- 最终 head 固化为 foundation_design_root；
- 冻结的设计头永久保留最终 design_root，Foundation 回执同时记录该 foundation_design_root；
- foundation_design_root 只表示“第一份 Canon 来自哪套建书设计”，不再充当连载期的当前规划权威；
- 后续 future_plan 和 planning_patch 继续改变未来生产状态，但不重写 foundation_design_root；
- 当前 historical_revision 只覆盖已验收章节及其下游重放，也不重写 foundation_design_root。

首版不声称支持“重新结算已经 ACCEPTED 的 Foundation”。如果连载后确实需要重做原始 Foundation，而现有未来规划、章节事件或章节历史修订无法表达，应另开 foundation revision 规格；不能借设计层偷偷覆盖第一份 Canon。

这样既保留建书来源，又不会与现有 Canon 中的滚动规划形成双重权威。

## 13. Foundation 的衔接

### 13.1 不让 ChatGPT重复抄一次已批准设计

建书就绪已经选择了七个精确设计文件，因此不应再要求 ChatGPT 把同样内容重新写一遍。

Core 在建书就绪推进前先对七个文件运行现有 Foundation 的确定性预校验。

推进成功后，Core 从本地 设计存储读取这七个精确 payload，创建一个正常的内部 foundation Task/Attempt，并把七个 payload 物化为本地不可变输入快照。这个内部尝试不发布 对 ChatGPT 可见的 READY，也不要求 ChatGPT 再写一份 manifest。

随后实现应把现有 Foundation 的“校验”和“提交”逻辑抽成可复用入口，由内部 foundation attempt 与旧的外部 FoundationSubmission 路径共同调用。同一套 ID mint、ref rewrite、receipt 和 Canon commit 规则只能有一份。

不得为了复用旧接口伪造“ChatGPT 已提交 manifest”的证据，也不得另写第二套 Foundation 提交器。

### 13.2 可恢复顺序

建书就绪的落地顺序：

1. 所有设计对象和 evidence 已经本地不可变保存；
2. 对候选设计提交和七个 Foundation 文件做完整无副作用预校验；
3. CAS 推进设计头；
4. 写设计提交回执；
5. reconcile 发现当前 head=foundation_ready 且尚无 Canon；
6. 创建或恢复唯一的内部 foundation Task/Attempt，并从 设计存储物化本地不可变输入快照；
7. 共享的 Foundation commit 在写第一份 Canon 时读取当前冻结设计头，并在首次写 Foundation 回执时一并记录 foundation_design_root；receipt 写入后不得回头修改；
8. reconcile 交叉确认 Foundation 回执的 foundation_design_root 与冻结设计头 一致；
9. 创建 chapter:1 READY。

如果在第 3 至第 8 步任意位置崩溃，重启后必须根据 设计头、设计提交回执、Foundation 回执 和 Canon 是否存在恢复，不创建第二份 Foundation，也不通过修改旧 receipt 修复状态。

Foundation settlement 必须保持幂等。

### 13.3 Foundation 校验失败不应发生在推进之后

建书就绪推进必须复用与真正 settlement 相同的确定性验证逻辑。预校验必须无副作用，不消耗 task、attempt、entity 序号；正式内部 foundation attempt 使用与预校验相同、尚未被设计操作改变的 entity sequence 基准。

如果预校验失败：

- 不推进设计头；
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
→ story_decisions
→ story_concept
→ story_review
→ 作者确认
→ 故事锁定
→ 建书设计：人物 / 世界 / 全书骨架 / 结局 / 风格 / 平台
→ foundation_readiness_review
→ 建书就绪
→ Core 自动建立第一份 Canon
→ Chapter 1
~~~

这是“推荐创作方法”，不是持久状态机。

作者可以在 Foundation 前回到前面的讨论重新考虑；系统只通过新不可变产物和新设计提交记录最终变化。

### 14.1 不允许偷偷跳过检查点

ChatGPT 可以自由安排探索对话，但不能：

- 没有故事审稿和作者确认就声称故事已锁定；
- 建书就绪使用没有被故事锁定采用的 story_concept；
- 审稿针对旧版本，却提交另一个版本；
- 建书就绪审查 后偷偷改七个文件；
- 用聊天记忆替代 story_decisions；
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

设计层复用现有 Drive 的“外部输入不可信”原则，但不复用 READY Task 语义。实现时必须把现有 Submission 的安全读取、静默期稳定扫描和本地不可变快照抽成共享深模块；design/inbox 调用同一套底层实现，不复制第二套静默期扫描器。

新增：

~~~text
exchange/design/
├── inbox/
│   └── <submission-id>/
│       ├── ...
│       └── manifest.json
└── result/
    └── <submission-id>.json
~~~

设计状态不新增第二个 STATUS 文件。现有 exchange/STATUS.json 继续作为项目唯一状态投影，扩展设计字段即可。

### 16.1 写入权

- exchange/STATUS.json：Core 写，ChatGPT 只读；
- design/inbox：ChatGPT 写，Core 只读并快照；
- design/result：Core 写，ChatGPT 只读。

### 16.2 设计提交请求

设计提交请求只在 design_mode=required、capability=passed、且第一份 Canon 尚未形成时处理。legacy 项目、capability 未通过的项目、以及已经冻结 foundation_design_root 的项目不得通过 design/inbox 改设计权威。

现有 exchange/control 仍只服务 Canon 生产任务；第一份 Canon 形成前不使用 author_directive / block_resolution 驱动语义设计。故事确认、审稿和候选变更全部走 design/inbox 的不可变产物与 promote。

首版只允许两种操作：

1. import：导入一个或多个设计产物、evidence 或 design bundle；Core 对 design bundle 返回 bundle_ref；
2. promote：提交一个已存在 bundle_ref + evidence，请求推进某个检查点。

不增加通用 action language。

同一 submission_id 第一次锁定本地快照后即成为幂等键：重复提交完全相同的内容返回原结果；同一 submission_id 后续出现不同内容必须标记冲突。不同 submission_id 即使请求语义相同，也按各自的 expected_design_root 正常执行 CAS，不做隐式全局去重。

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
- evidence：按当前 checkpoint 的固定角色提交精确 evidence artifact_ref；
- protocol_version。

Core 验证成功后返回：

- result；
- previous_design_root；
- new_design_root；
- receipt_ref；
- 如为建书就绪，返回后续 Canon settlement 状态。

### 16.5 STATUS 不是流程游标

STATUS 可以包含：

- design_head；
- foundation_design_root：required 项目从冻结设计头 / Foundation 回执 交叉投影，legacy 为空；
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
2. exchange/STATUS.json；
3. 当前设计头 指向的设计提交、bundle 和必要产物；
4. 尚未采用但作者希望继续处理的候选 artifact_ref，如有。

正式恢复依据是 设计存储，不是“上次聊到第几步”。

为了方便继续未确认探索，STATUS 可以列出最近若干未被设计提交选择的候选引用，但这只是便利投影。候选列表丢失不能改变设计头。

## 18. 与现有 Canon 任务的边界

### 18.1 新项目

项目运行元数据新增 design_mode：

- required：按本规格完成语义设计后才能建立第一份 Canon；
- legacy：保留旧的直接 Foundation 流程。

使用本规格版本新建的项目固定为 required。

新项目流程：

~~~text
capability check
→ semantic design
→ 故事锁定
→ 建书就绪
→ Core 自动完成 Foundation 结算
→ chapter:1 READY
~~~

required 模式下，能力检查通过但尚未建书就绪时：

- 没有 active Core Task/Attempt 是合法状态；
- 设计导入、设计包、审稿和推进 使用独立 submission_id / artifact_ref，不消耗 Core 的 NextTaskSeq、NextAttemptSeq 或 NextEntitySeq；
- 不创建也不发布 exchange/READY.json；
- Reconcile 只刷新 exchange/STATUS.json，不得自动创建 foundation task；
- serve 即使没有 active production attempt，也必须扫描 design/inbox；
- 设计导入 / 推进 只有 capability=passed 时才处理；
- 设计头 到达 foundation_ready 后，reconcile 才创建唯一内部 foundation Task/Attempt，并且不把这个内部 attempt 发布到 READY；
- Foundation ACCEPTED 后立即回到现有 chapter/revision 生产链。

这意味着现有 reconcile 中“capability 通过 + 无 active task ⇒ 自动 foundation”的规则必须按 design_mode 分支，而不是在旁边再叠一套初始化循环。

### 18.2 现有项目与迁移

本规格同时改变本地项目元数据和 Drive 协议，因此实施时必须显式提升 core schema version 与 protocol version，并继续遵守现有“先备份、再迁移、未知新版本 fail closed”的规则。

迁移时绝不伪造历史设计证据。所有在升级前已经初始化的项目——无论是否已经有 Canon、是否正停在旧 foundation attempt、还是只有 capability 状态——统一写入 design_mode=legacy。

core schema migration 只补本地运行元数据，不改变 Canon、active task/attempt 或现有 READY 指向。protocol migration 继续沿用当前已经存在的安全语义：为了让旧协议 submission 失效，可以为同一个 active task 生成绑定新 protocol_version 的 replacement attempt，并在重新完成 capability check 后发布对应的新 READY；这个重绑不得改变 task 的 kind、target、base_canon_root、约束、Canon 或业务进度，也不得把 legacy 项目切换到 required。

因此这里的“保持旧流程”指不推进或重写小说生产事实，不要求保留旧协议 attempt 的字节级身份。

只有升级后新建的项目进入 design_mode=required。首版不自动把 legacy 项目切换成 required，也不取消或重写旧项目已经存在的 foundation attempt。

legacy 项目：

- foundation_design_root 为空；
- 不要求补造 Story Review、作者确认或设计提交；
- verify、backup、restore 继续接受“没有 设计存储”的合法状态；
- 本规格不阻塞其继续写作。

首版不提供“把旧书逆向补齐 design_root”的自动工具。

如以后确有需求，应单独设计 legacy adoption，并明确它只是“登记当前设计”，不能冒充过去已经做过的 Review。

## 19. 与 upstream 能力的映射

本期只恢复职责，不恢复运行时。

| Upstream 能力 | 本规格中的落点 |
|---|---|
| Brainstorm | creative_brief + story_decisions + story_concept + 故事审稿 |
| Architect | 七个建书设计文件 |
| outline-all | book_plan.whole_book_skeleton |
| zero-init / architect-check | 建书就绪审查 + Foundation 确定性预校验 |
| web research | 来源产物（source artifact），由 ChatGPT 使用现有联网能力完成 |
| craft recall | 可选来源产物；不要求 RAG runtime |
| Agent handoff | artifact_ref / bundle_ref / 设计提交，不靠聊天口头交接 |
| exact-version review | review.subject_ref |
| recovery | 内容寻址设计存储 + 设计提交链 + 设计头 |

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

CoreProjectState 只增加一个运行元数据字段：

- design_mode：required / legacy。

foundation_design_root 不再复制进 CoreProjectState。required 项目的最终建书设计根由冻结的设计头 保存，并由 Foundation 回执 再记录一次作为 Canon 来源证明；status 从这两处读取并交叉检查。这样不在第三个文件里维护同一值。

design_mode 属于项目运行元数据，不进入小说 Canon。

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

- 所有设计对象摘要可重算；
- bundle 引用对象存在；
- commit bundle/evidence 引用存在；
- 设计提交摘要可重算；
- parent chain 无环、无断裂；
- HEAD 指向真实 commit；
- required 项目 Foundation 回执 中的 foundation_design_root 与冻结 head 一致；
- legacy 项目允许不存在设计头，Foundation 回执 也允许没有 foundation_design_root。

设计完整性校验 失败不允许悄悄从 Drive 重建。

### 20.3 Backup / Restore

设计存储位于现有 meta/core 下，应继续由当前“复制整个本地权威目录”的 backup 路径自然覆盖，不另写第二套备份收集器。backup 在落盘前必须先验证 设计存储；恢复完成后必须同时验证 design_root 链和 canon_root 链。

### 20.4 Migration

未知设计 schema version 必须 fail closed。

core schema migration 负责为旧 CoreProjectState 明确写入 design_mode=legacy，并且不得改变现有 Canon、active task/attempt 或 READY 指向。

protocol migration 负责生成与新 exchange/design 协议匹配的 CHATGPT_PROTOCOL.md，并继续使用当前 provider-free Core 已有的 replacement-attempt 机制重新绑定 protocol_version：允许替换 active attempt 和重新发布 READY，但必须保持同一个 active task、target、base canon、约束和 Canon，不得推进章节或 Foundation 业务状态。

本期只新增“当前已发布版本 → 本规格版本”的显式迁移路径，不为了将来可能出现的多版本组合建设通用迁移图。


当前发布版本同时需要 core schema 与 protocol 升级，而现有迁移器一次只提交一种迁移。因此当前发布版本升级到本规格版本固定分两段执行：先完成 core schema 迁移并写入 design_mode=legacy，再完成 protocol migration 与 replacement attempt 重绑。两段各自使用与当时本地状态精确匹配的独立预迁移备份；第一段完成到第二段开始之间，普通生产 mutation 继续 fail closed。首版不为把两段压成一个命令而引入通用迁移编排器。

任何会改写 设计存储 格式的迁移必须遵守现有“先备份再迁移”原则。

不得为了迁移方便重新计算语义内容、制造缺失 evidence，或把旧项目悄悄升级成 required。

## 21. 安全边界

design/inbox 使用与现有 Submission 至少同级的安全策略：

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

协议或安全错误只拒绝本次 import，不改变 设计头 和 Canon。

### 22.2 审稿版本错误

review.subject_ref 不存在或类型不匹配时直接 INVALID。

旧版本审稿结果不自动迁移到新版本。

### 22.3 Bundle 不一致

返回精确的 slot、artifact_ref 和冲突 input_ref。

不得把它升级成作者 BLOCKED，因为这是可修复的设计包问题。

### 22.4 promote CAS 冲突

返回 STALE_DESIGN_HEAD，并给出当前 design_root。

ChatGPT 必须重新读取 STATUS 再决定是否重做，不允许 Core 自动 rebase 语义设计。

### 22.5 建书就绪的机械校验失败

不推进设计头，不创建 Canon。

返回现有 Foundation validator 的具体错误。

### 22.6 Foundation settlement 崩溃

依赖既有事务恢复与新增 design receipt 恢复。

恢复后只能产生一个 Canon root 和一个 Foundation 回执。

## 23. 可观察性

首版不建 Dashboard。

novel-core status 至少增加：

- design_mode：required / legacy；
- design_head；
- design_checkpoint；
- foundation_design_root；
- foundation_state。

现有 exchange/STATUS.json 只投影这些必要信息。

不展示假 token、假模型、假 Agent 进度。

## 24. 测试要求

实现必须覆盖以下高风险场景。

### 24.1 精确版本

- story_review 的 subject_ref 只能是 story_concept artifact_ref，不能拿 bundle_ref 或其他 artifact 顶替；
- foundation_readiness_review 的 subject_ref 只能是 bundle_ref；
- story_review(v1) 不能证明 story_concept(v2)；
- foundation_readiness_review(bundle-A) 不能证明 bundle-B；
- 修改 payload 后 artifact_ref 必须变化；
- inputs 改变而 payload 不变时 artifact_ref 仍必须变化。

### 24.2 决策与来源

- story_decisions 改变后，仍绑定旧 decisions 的 story_concept 不能组成新故事锁定；
- 新增 source artifact 不自动使已锁定 story_concept 失效；
- 仅仅导入一个带 supersedes 的新候选不能改变已有设计头；只有成功 promote 的新设计提交才能改变活动选择。

### 24.3 Bundle 一致性

- characters/world/ending_contract 缺少当前 story_concept 硬输入必须拒绝；
- foundation 缺少当前 story_concept、characters 或 world 硬输入必须拒绝；
- book_plan 缺少当前 story_concept、characters、world 或 ending_contract 硬输入必须拒绝；
- style_profile/platform_profile 缺少当前 creative_brief 硬输入必须拒绝；
- 新 story_concept + 仍依赖旧 story_concept 的 book_plan 必须拒绝；
- slot 类型错配必须拒绝；
- 缺失 input object 必须拒绝；
- source_ref 指向不存在对象时必须拒绝；新增其他 source artifact 不得自动使已有设计提交失效。

### 24.4 检查点

- 无 PASS story_review 不能故事锁定；
- 无明确作者确认不能故事锁定；
- 有 blocking open decision 不能建书就绪；
- 审稿 PASS 后修改七个文件必须导致旧 readiness review 失效；
- 建书就绪必须跑现有 Foundation 无副作用预校验。

### 24.5 并发与恢复

- 两个会话从同一 design_root 同时推进，只允许一个成功；
- 同一 submission_id、同一锁定快照的 promote 重放必须返回原结果；同一 submission_id 内容变化必须冲突；
- 设计头已写、Foundation 尚未 settlement 时崩溃，重启必须继续而不是重复建书；
- Drive submission 锁定后被改写不能污染本地对象。

### 24.6 兼容

- 所有升级前已初始化项目迁移后都是 legacy，包括尚未结算的旧 foundation attempt；
- core schema migration 后原 active task/attempt/READY 指向保持不变；随后若执行 protocol migration，可以按现有安全机制为同一 active task 重绑 replacement attempt，并在 capability 重新通过后发布新 READY，但不得改变 target/base canon/业务进度；
- legacy 项目不生成虚假 design_root；
- 新 required 项目 capability 通过但尚未建书就绪时没有 active task 是合法状态，且不得生成 READY；
- required 项目的 serve 在没有 active task 时仍能处理 design/inbox；
- required 项目的设计操作不得改变 NextTaskSeq、NextAttemptSeq、NextEntitySeq；
- 新项目没有建书就绪时不能进入 chapter:1；
- 新项目 Foundation 成功后 设计头冻结；
- 冻结后 design promote 必须拒绝；错误信息只能说明“建书设计基线已冻结”，不能谎称现有 historical_revision 支持修改 Foundation。

### 24.7 完整性

- backup / restore 后 design_root 与 canon_root 都保持一致；
- verify 能发现 设计对象、bundle、commit、receipt 任一被篡改；
- 不存在 provider/model/token/tool-call 等伪造字段。

## 25. 完成标准

本规格实现完成的最低标准：

1. 新 required 项目能力检查通过后进入语义设计，而不是直接 Foundation；legacy 项目保持旧流程；
2. ChatGPT 能导入 creative_brief、story_decisions、story_concept 并获得 Core 生成的 artifact_ref；
3. 故事审稿和作者确认必须绑定精确 story_concept；
4. 故事锁定可以跨会话恢复；
5. Architect 产出的七个建书设计文件进入 设计存储；
6. 建书就绪审查必须绑定精确设计包；
7. Core 能发现新旧设计文件混用；
8. 建书就绪推进使用 设计头 CAS；
9. Core 从已批准七文件创建内部 foundation attempt，并复用共享 Foundation 校验/提交逻辑建立第一份 Canon，不要求 ChatGPT 重复抄写或伪造 manifest；
10. Foundation 回执在首次写入时保存 foundation_design_root，并与冻结设计头 一致；
11. chapter:1 只有在 Foundation ACCEPTED 后才出现；
12. Foundation 后设计头 冻结；
13. 已有项目升级不伪造历史设计证据；
14. verify、backup、restore 覆盖 设计存储，且 backup 继续复用现有 meta/core 整体复制路径；
15. exchange/STATUS.json 仍是唯一状态投影，设计模式不增加第二个 STATUS；
16. 全流程没有任何模型/provider/runtime 依赖。

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
