package protocol

const chatGPTProtocolPart1 = `# Novel Core × ChatGPT App 协议

协议版本：%s
项目：%s

## 基本规则

- 只处理普通 UTF-8 .json、.md、.txt 文件。
- JSON 对象中的键不得重复；重复键会被 Core 拒绝。
- JSON 数值如果数学上是整数，该整数值必须能被 Core 精确保留；会在通用解码中发生精度损失的超大整数会被拒绝。
- Core 写 project.json、CHATGPT_PROTOCOL.md、exchange/READY.json、exchange/STATUS.json、outbox、result、published、projection、backup。
- ChatGPT 只写 exchange/inbox、exchange/control/inbox、exchange/design/inbox，以及能力检查要求的 setup 回执文件。
- 不修改 Core 写入的文件，不使用 Google Docs/Sheets 代替协议文件。
- production 任务只以 exchange/READY.json 为准；required Design 阶段没有 READY，必须按 STATUS 的 design_mode、design_head、design_checkpoint 与 exchange/design/inbox 继续。
- exchange/STATUS.json 给出当前权威 canon_root、capability、活动 task/attempt/block，以及 export_ready/export_problems。export_ready 只表示确定性的导出前置条件已满足；真正 final export 仍会重新验证 Canon root 与活动 artifact inventory。control 的 base_canon_root 必须取 STATUS.json 的当前 canon_root，不要用历史 revision READY 的 base_canon_root 猜当前根。
- 临近结局时必须读取 STATUS.export_problems；其中会列出尚未终态的伏笔/读者承诺、缺失或过期的 ending_resolution、活动 revision replay 等阻塞项。不要因为当前 task context 没带某个 obligation 就假设它已经关闭。
- Drive 多文件同步不是原子事务。每次读取 READY 与 STATUS 后，必须确认 STATUS.active_attempt_id == READY.attempt_id、STATUS.active_target == READY.target，且 STATUS.block_id == READY.block_id（正常未阻塞时两边都为空）；如果不一致，说明文件仍在同步，等待后重新读取，不能据此提交或发 control。
- STATUS 有 active production attempt 时，先读取当前 READY 和对应 outbox/<task-id>/<attempt-id>/ 的 task.json、constraints.json、context.json、canon_excerpt.json、recent_prose.md。
- 首次创建对象时，Core-owned ID 字段必须完全省略；canon_id、foreshadow_id、promise_id、conflict_id 等字段键本身都不得出现，即使值为 null、数字或对象。永久 ID 只由 Core 在 ACCEPTED 时分配。

## 能力检查

首次初始化后读取 setup/capability-challenge.json。把其中 project_id、protocol_version、nonce 原样写入 setup/capability-ack.json，并声明三项能力：

~~~json
{"project_id":"...","protocol_version":"1.1","nonce":"...","capabilities":{"read":true,"write_utf8_json":true,"write_utf8_md":true}}
~~~

同时把 challenge.markdown_probe 的文本逐字写入 setup/capability-write-test.md。能力检查通过前不会生成正式 READY。

## Design 模式与 pre-Canon 语义设计

exchange/STATUS.json 的 design_mode 只有两种语义：required 表示新项目必须先完成语义设计，legacy 表示升级项目继续沿用既有 Foundation production 流程。required 且还没有 Canon 时，Design submission 写入 exchange/design/inbox/<submission_id>/，结果读取 exchange/design/result/<submission_id>.json。此阶段没有 READY；不要创建 Foundation production submission，也不要读取历史 READY 推断下一步。

Design import 的 manifest.json 使用 machine schema 1，例如：

~~~json
{
  "schema_version": 1,
  "project_id": "book-1",
  "submission_id": "design-001",
  "protocol_version": "1.1",
  "operation": "import",
  "files": ["concept.json"]
}
~~~

import 文件把不可变 semantic artifact 或 bundle 导入本地内容寻址 Design Store。artifact ref 与 bundle ref 都由 Core 返回；后续 inputs、sources、supersedes、bundle selections 只能引用已经成功导入的 ref，不要自行计算摘要。

story_locked promote 必须显式提供 expected_design_root 做 CAS，并提供精确绑定当前 story_concept 的 story_review 与 author_confirmation：

~~~json
{
  "schema_version": 1,
  "project_id": "book-1",
  "submission_id": "design-promote-001",
  "protocol_version": "1.1",
  "expected_design_root": "",
  "checkpoint": "story_locked",
  "bundle_ref": "design_bundle@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  "evidence": {
    "story_review": "story_review@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
    "author_confirmation": "author_confirmation@sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
  }
}
~~~

story_locked 之后，foundation_ready 必须保留已经锁定的 creative_brief、story_decisions、story_concept，并提交完整 Foundation Design bundle；evidence 必须精确只有 foundation_readiness_review，且其 subject_ref 必须是当前 bundle ref。foundation_ready 成功后 Design baseline 冻结，Core 会在本地通过 commit journal 自动把 approved Design Foundation 接入第一份 Canon，然后才发布 chapter:1 的 READY。

## 正式提交与 manifest.json

提交目录固定为 exchange/inbox/<task_id>/<attempt_id>/。先写完全部 artifact，最后才写 manifest.json；不要计算 SHA-256。manifest.json 的绑定字段全部从当前 READY 原样复制，files 列出本次实际提交的 artifact，不包含 manifest.json：

~~~json
{
  "schema_version": 1,
  "project_id": "从 READY 复制",
  "task_id": "从 READY 复制",
  "attempt_id": "从 READY 复制",
  "base_canon_root": "从 READY 复制",
  "protocol_version": "从 READY 复制",
  "task_digest": "从 READY 复制",
  "completion_nonce": "从 READY 复制",
  "files": ["按 task.json.required_artifacts 填写"]
}
~~~

不要重用旧 attempt 的 manifest。REWRITE、RETRY 或 block_resolution 后必须重新读取新的 READY。

## legacy Foundation production 任务

Foundation 必须提交 task.json.required_artifacts 列出的 7 个 JSON 文件。最小合法形状如下；可以增加创作字段，但不要省略这些硬字段。

foundation.json：

~~~json
{"title":"书名","protagonist":{"entity_type":"character","local_ref":"protagonist"},"opening_location":{"entity_type":"location","local_ref":"start"}}
~~~

characters.json：

~~~json
{"characters":[{"local_id":"protagonist","name":"主角"}]}
~~~

world.json：

~~~json
{"entities":[{"entity_type":"location","local_id":"start","name":"起点"}]}
~~~

book_plan.json：

~~~json
{"direction":"全书主线方向"}
~~~

ending_contract.json：

~~~json
{"main_resolution":"主线最终必须如何收束"}
~~~

style_profile.json：

~~~json
{"language":"zh-CN"}
~~~

platform_profile.json：

~~~json
{"platform":"fanqie"}
~~~

characters.json.characters 必须是数组并至少包含一个人物；world.json.entities 可省略，但 world.entities 一旦出现必须是数组。characters.json 中角色用 local_id 定义；world.json 的实体必须有 entity_type + local_id。每个 Foundation entity 必须有非空字符串 name。foundation.protagonist 必须引用 character，foundation.opening_location 必须引用 location；二者都必须使用非空 local_ref 指向本次 Foundation 已声明实体。Foundation 内部引用使用 {"entity_type":"...","local_ref":"..."}；local_ref 一旦出现必须是非空字符串，并与 entity_type 一起指向已经声明的本地实体。Foundation 首次定义不得自行提供 canon_id，typed reference 也不要预填 canon_id；Core 只在 ACCEPTED 后分配并写入 canon_id。后续任务不要继续使用 Foundation local_id/local_ref。

book_plan.json 可选 current_arc：

~~~json
{"direction":"主线方向","current_arc":{"id":"arc-1","start_chapter":1,"end_chapter":10,"goal":"当前 Arc 目标","planning_lead_chapters":2}}
~~~

world.json 可选 travel_constraints，用 Foundation local_ref 连接地点，min_ticks 必须为正整数：

~~~json
{"entities":[{"entity_type":"location","local_id":"a","name":"甲地"},{"entity_type":"location","local_id":"b","name":"乙地"}],"travel_constraints":[{"from":{"entity_type":"location","local_ref":"a"},"to":{"entity_type":"location","local_ref":"b"},"min_ticks":3}]}
~~~

## Chapter / Revision 任务

task.json.required_artifacts 是当前 attempt 的唯一文件合同；不要根据旧聊天、旧协议示例或固定“五文件”假设增删 artifact。旧 active attempt 可能仍只有 chapter.md、chapter_contract.json、events.json、state_delta.json、self_review.json；新质量链 attempt 会明确把 chapter_plan.json 与 chapter_review.json 列入 required_artifacts。若 required_artifacts 还列出 planning_patch.json 或 arc_rehearsal.json，也必须提交。manifest.json.files 必须与当前 attempt 的 required_artifacts 加上协议允许的可选 planning artifact 精确匹配。

写之前读取 context.json.foundation_reference。这里包含 Core 已规范化的 foundation、characters、world、style_profile、platform_profile；角色/地点等引用使用其中的 canon_id。例如 declared_pov 应使用 characters 中真实存在的 canonical character ID。

质量链的操作顺序是：先写 chapter_plan.json，再写 chapter.md，然后生成 events.json / state_delta.json / self_review.json，最后针对这个不可变 attempt snapshot 写 chapter_review.json。Core 不检查私有思考过程，只检查提交中确实存在计划产物、正文产物和精确绑定的审稿产物。

chapter_plan.json 最小形状：

~~~json
{
  "chapter": 1,
  "base_canon_root": "从当前 READY.base_canon_root 复制",
  "objective": "本章主要故事推进",
  "reader_payoff": "本章明确给读者的结果/信息/满足",
  "beats": [
    {"id":"beat-1","intent":"建立压力"},
    {"id":"beat-2","intent":"行动并形成结果"}
  ],
  "ending_hook_intent": "推出下一章问题"
}
~~~

chapter_review.json 必须绑定同一 snapshot 内的 chapter.md；质量链还必须 plan_ref=chapter_plan.json。固定检查维度为 story_progress、reader_payoff、pacing、character_consistency、world_consistency、continuity、ending_hook，每项都提供 status=pass/revise 与非空 note：

~~~json
{
  "subject_ref": "chapter.md",
  "plan_ref": "chapter_plan.json",
  "verdict": "pass",
  "dimensions": {
    "story_progress": {"status":"pass","note":"主线有推进"},
    "reader_payoff": {"status":"pass","note":"读者获得明确结果"},
    "pacing": {"status":"pass","note":"推进没有明显停滞"},
    "character_consistency": {"status":"pass","note":"人物行为与前态一致"},
    "world_consistency": {"status":"pass","note":"没有突破既有规则"},
    "continuity": {"status":"pass","note":"与前态和 Arc 连续"},
    "ending_hook": {"status":"pass","note":"章末形成下一章驱动力"}
  },
  "issues": []
}
~~~

如果 reviewer 明确给出 verdict=revise，Core 会使用现有 REWRITE 语义保留同一 task、创建新 attempt、保持 Canon 不动；不要用旧 attempt 的 review 或 manifest 修补新 attempt。blocking issue 的 evidence_anchor 必须逐字出现在当前 chapter.md 中。文学判断仍由 ChatGPT/作者完成，Core 只验证 review 的结构、精确 snapshot 引用和 verdict 自洽性。

历史最小 1.0 chapter_contract 仍可读取，例如 {"chapter":1,"declared_pov":"character-000001"}；为获得完整的确定性保护，新提交应使用完整 chapter_contract。新增字段采用“出现则严格机械验证”，Core 不判断文学质量。

chapter_contract.json 推荐完整形状：

~~~json
{
  "chapter": 1,
  "declared_pov": "character-000001",
  "start_state": {"canon_root": "从当前 READY.base_canon_root 复制"},
  "purpose": "本章要完成的叙事目的",
  "main_conflict": "本章主冲突",
  "reader_question": "本章持续推动的读者问题",
  "obligation_refs": ["foreshadow-000010", "promise-000011"],
  "immutable_refs": ["character-000001"],
  "foreshadow_operations": [{"foreshadow_id":"foreshadow-000010","allowed_states":["reinforced","payoff_ready"]}],
  "expected_changes": ["location", "relationship"],
  "ending_hook": "章末钩子说明",
  "target_length": {"min_chars":1800,"max_chars":2600},
  "planning_obligations": ["rolling_planning_due"],
  "hard_constraints": [{"id":"hc-001"}]
}
~~~

start_state.canon_root 一旦出现必须等于当前 attempt 的 base_canon_root。purpose、main_conflict、reader_question、ending_hook 一旦出现必须是非空字符串。obligation_refs、immutable_refs、planning_obligations 一旦出现必须是唯一非空字符串数组并引用 Core 当前可机械证明的对象或任务约束。foreshadow_operations 每项必须包含已存在的 foreshadow_id 和唯一非空 allowed_states；本次对应伏笔状态变化不得超出 allowed_states。expected_changes 只能使用 Core 支持的 state change kind，并且每个声明类别都必须实际出现在本次 state_delta.json.changes 中。target_length.min_chars/max_chars 必须是非负整数且 min_chars <= max_chars，Core 按 chapter.md 的 Unicode rune 数检查。planning_obligations 只能填写当前 constraints.json.control_constraints 已经下发的滚动规划 kind，不能从正文自行推断。

质量链 attempt 如果提交 planning_patch.json，还必须同时提交 arc_rehearsal.json；反过来 rehearsal 也不能脱离 patch 单独提交。rehearsal 至少两个候选 scenario，每个都有唯一 id、结构合法的 next_arc、非空 opportunity 与 risk，并通过 selected_scenario_id 选择一个；selection_reason 必须非空。selected scenario 的 next_arc 必须与 planning_patch.json.next_arc 结构相同。只有 planning_patch.json 改变权威 planning，arc_rehearsal.json 只是记录候选与选择证据，不建立第二套 planning 状态。

`
