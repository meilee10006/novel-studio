package protocol

const chatGPTProtocolPart1 = `# Novel Core × ChatGPT App 协议

协议版本：%s
项目：%s

## 基本规则

- 只处理普通 UTF-8 .json、.md、.txt 文件。
- JSON 对象中的键不得重复；重复键会被 Core 拒绝。
- JSON 数值如果数学上是整数，该整数值必须能被 Core 精确保留；会在通用解码中发生精度损失的超大整数会被拒绝。
- Core 写 project.json、CHATGPT_PROTOCOL.md、exchange/READY.json、exchange/STATUS.json、outbox、result、published、projection、backup。
- ChatGPT 只写 exchange/inbox、exchange/control/inbox，以及能力检查要求的 setup 回执文件。
- 不修改 Core 写入的文件，不使用 Google Docs/Sheets 代替协议文件。
- 当前任务只以 exchange/READY.json 为准；没有 READY 时不要自行推进小说状态。
- exchange/STATUS.json 给出当前权威 canon_root、capability、活动 task/attempt/block，以及 export_ready/export_problems。export_ready 只表示确定性的导出前置条件已满足；真正 final export 仍会重新验证 Canon root 与活动 artifact inventory。control 的 base_canon_root 必须取 STATUS.json 的当前 canon_root，不要用历史 revision READY 的 base_canon_root 猜当前根。
- 临近结局时必须读取 STATUS.export_problems；其中会列出尚未终态的伏笔/读者承诺、缺失或过期的 ending_resolution、活动 revision replay 等阻塞项。不要因为当前 task context 没带某个 obligation 就假设它已经关闭。
- Drive 多文件同步不是原子事务。每次读取 READY 与 STATUS 后，必须确认 STATUS.active_attempt_id == READY.attempt_id、STATUS.active_target == READY.target，且 STATUS.block_id == READY.block_id（正常未阻塞时两边都为空）；如果不一致，说明文件仍在同步，等待后重新读取，不能据此提交或发 control。
- 每次都先读取当前 READY 和对应 outbox/<task-id>/<attempt-id>/ 的 task.json、constraints.json、context.json、canon_excerpt.json、recent_prose.md。
- 首次创建对象时，Core-owned ID 字段必须完全省略；canon_id、foreshadow_id、promise_id、conflict_id 等字段键本身都不得出现，即使值为 null、数字或对象。永久 ID 只由 Core 在 ACCEPTED 时分配。

## 能力检查

首次初始化后读取 setup/capability-challenge.json。把其中 project_id、protocol_version、nonce 原样写入 setup/capability-ack.json，并声明三项能力：

~~~json
{"project_id":"...","protocol_version":"1.0","nonce":"...","capabilities":{"read":true,"write_utf8_json":true,"write_utf8_md":true}}
~~~

同时把 challenge.markdown_probe 的文本逐字写入 setup/capability-write-test.md。能力检查通过前不会生成正式 READY。

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

## Foundation 任务

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

必须提交 chapter.md、chapter_contract.json、events.json、state_delta.json、self_review.json；若 task.json.required_artifacts 还列出 planning_patch.json，也必须提交它。

写之前读取 context.json.foundation_reference。这里包含 Core 已规范化的 foundation、characters、world、style_profile、platform_profile；角色/地点等引用使用其中的 canon_id。例如 declared_pov 应使用 characters 中真实存在的 canonical character ID。

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

`
