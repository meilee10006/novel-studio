package protocol

import (
	"encoding/json"
	"fmt"
)

const (
	LegacyVersion      = "0.9"
	CurrentVersion     = "1.0"
	MaxJSONDepth       = 64
	MaxJSONArrayLength = 10000
	DefaultMaxTextSize = 8 << 20
)

func IsKnownVersion(version string) bool {
	return version == LegacyVersion || version == CurrentVersion
}

func RenderChatGPTProtocol(projectID string) string {
	return fmt.Sprintf(`# Novel Core × ChatGPT App 协议

协议版本：%s
项目：%s

## 基本规则

- 只处理普通 UTF-8 .json、.md、.txt 文件。
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

最小形状如下。

chapter_contract.json：

~~~json
{"chapter":1,"declared_pov":"character-000001"}
~~~

hard_constraints 可省略；hard_constraints 一旦出现必须是数组，每一项必须是对象并包含唯一非空 id。offscreen 事件和 author_decision_required.constraint_refs 只能引用这些 id，不要重复、留空或自行引用不存在的约束。

events.json：

~~~json
{"events":[{"local_id":"e1","kind":"onscreen","evidence_anchor":"正文中逐字出现的一小段文本"}]}
~~~

state_delta.json：

~~~json
{"changes":[{"kind":"location","character_id":"character-000001","location_id":"location-000002","start_tick":1,"end_tick":1,"event_ref":"e1"}]}
~~~

self_review.json：

~~~json
{"ok":true}
~~~

self_review.ok 必须是 boolean。ok:true 表示本次自审通过，ok:true 不得同时提供 author_decision_required；ok:false 必须同时提供 author_decision_required 进入 BLOCKED 流程，否则 Core 会 REWRITE。

events.json 的 local_id 只在本次提交中使用；events.json 不得自行提供 canon_id，永久 story-event ID 只由 Core 在 ACCEPTED 时分配。kind 只支持 onscreen / offscreen。actors/observers 可省略；actors/observers 一旦出现必须是字符串数组，数组元素必须是非空人物引用，且 actors/observers 数组内不得重复人物引用。同一提交新人物可先使用 local_id，Core 在 ACCEPTED 前会统一改写并校验 canonical character ID。除 kind=offscreen 外，可见事件必须有 evidence_anchor，且 anchor 必须逐字出现在 chapter.md。离屏事件必须显式使用 kind=offscreen；constraint_refs 必须是非空字符串数组，并至少包含一个元素，其中每个 ID 都必须逐项命中当前 chapter_contract.hard_constraints[].id，表示这个离屏事实已被本章硬约束明确允许。Core 只验证这些引用真实存在，不判断离屏剧情在文学上是否合理。state_delta 中要引用本章事件时使用 event_ref；event_ref 一旦出现必须是非空字符串。Core ACCEPTED 时会把它改成 event_canon_id。event_ref 与 event_canon_id 不得同时提供；event_ref 与 event_canon_id 字段键不得同时出现，即使 event_canon_id 的值写成 null 也不允许；event_canon_id 一旦出现必须是非空字符串。需要引用历史已验收事件时只填写其已有 event_canon_id。state_delta.json.changes 必须是数组；没有结构化状态变化时也必须显式提交 {"changes":[]}。

state_delta 支持的 change kind 包括 character_add、location_add、resource_add、knowledge_add、resource、location、relationship、foreshadow、conflict、reader_promise、ending_resolution；每个 change kind 必须来自支持列表，缺失或未知 kind 会触发 REWRITE，不会被静默忽略。需要事件证据的 change 使用本章 event_ref。不要自行发明 canonical event ID。常用合法形状如下：

~~~json
{"changes":[
  {"kind":"character_add","local_id":"ally","name":"新同伴","description":"本章首次正式出场的人物","event_ref":"e1"},
  {"kind":"location_add","local_id":"station","name":"旧车站","description":"本章首次进入的地点","event_ref":"e1"},
  {"kind":"resource_add","local_id":"money","name":"现金","description":"需要持续追踪数量的故事资源","event_ref":"e1"},
  {"kind":"knowledge_add","character_id":"character-000001","fact_id":"fact-secret","statement":"主角知道密室钥匙在管家手中","source":{"kind":"observed","event_ref":"e1"}},
  {"kind":"resource","resource_id":"money","delta":10,"event_ref":"e1"},
  {"kind":"location","character_id":"character-000001","location_id":"location-000002","start_tick":1,"end_tick":2,"event_ref":"e1"},
  {"kind":"relationship","relationship_id":"character-000001|character-000003","tags":["互相信任"],"event_ref":"e1"},
  {"kind":"foreshadow","local_id":"fs-001","description":"红色纸伞与十年前旧案直接相关","state":"seeded","event_ref":"e1"},
  {"kind":"conflict","local_id":"rivalry","description":"主角与管家争夺同一把钥匙","participants":["character-000001","character-000003"],"state":"open","escalation_condition":"争夺公开化","close_condition":"钥匙归属明确且双方停止争夺","event_ref":"e1"},
  {"kind":"reader_promise","local_id":"promise-001","statement":"读者期待知道红色纸伞真正主人是谁","state":"advanced"},
  {"kind":"ending_resolution","event_ref":"e1"}
]}
~~~

character_add 用本次尝试内的 local_id 声明新人物，并用 event_ref 证明其在本章进入故事；location_add 同理声明新地点；resource_add 同理声明需要持续追踪数量的资源。三类 *_add 的 description 可省略；description 一旦出现必须是字符串。**同一提交**中，chapter_contract.declared_pov、events.json 的 actors/observers、state_delta 的 character_id、relationship_id 两端以及 knowledge source.from_character_id 可以先引用新人物 local_id，location change 的 location_id 可以先引用新地点 local_id，resource change 的 resource_id 可以先引用新资源 local_id；Core ACCEPTED 时会统一改写成 canonical character/location/resource ID。处理结果的 id_mappings 给出 local_id → canon_id；后续任务只能使用 canonical ID，动态人物、地点和资源都会出现在后续 canon_excerpt 的 entities 中。首次创建不得自行提供永久 ID 字段；character/location/resource 不要提交 canon_id，伏笔不要提交 foreshadow_id，读者承诺不要提交 promise_id，冲突不要提交 conflict_id。旧 Canon 已存在的历史资源键继续兼容读取，但新提交不要凭空发明新的 resource_id。

knowledge_add 的 fact_id 是稳定标识，statement 是后续新对话可直接理解的事实文本；新增知识应同时提供两者。同一角色已有非空 statement 时不得改写；后续省略 statement 时 Core 保留既有文本。observed source 要求对应 events.json 事件把该角色列在 observers 中，例如 {"local_id":"e1","evidence_anchor":"...","observers":["character-000001"]}。transmitted source 必须提供 from_character_id；from_character_id 必须出现在 event actors，接收角色必须出现在 event observers，并且来源角色在前态已经知道同一 fact_id。transmitted 不得改写来源 fact 的 statement；来源已有 statement 时可省略该字段，由 Core 继承来源文本。旧 Canon 中没有 statement 的知识仍按 fact_id 兼容读取。relationship_id 必须引用两个不同 canonical characters；relationship 的 tags 可省略，tags 一旦出现必须是唯一非空字符串数组。resource、location、relationship、foreshadow、conflict、ending_resolution 需要事件证据。location.start_tick 不得早于该角色前态 end_tick；地点变化时若存在匹配的 travel_constraints，还必须满足对应 min_ticks。

首次创建伏笔使用 local_id，并提供 foreshadow.description 作为后续新对话可读的伏笔语义；只有 ACCEPTED 时 Core 才分配永久 foreshadow ID，并通过 id_mappings 返回，后续状态推进使用 foreshadow_id 字段填写该 canonical ID。foreshadow.description 一旦出现必须是字符串；description 创建后不可改写；后续推进若省略 description，Core 保留前态描述，显式提供时必须与已有非空 description 一致。旧 Canon 已存在的历史伏笔 key 仍兼容继续推进。foreshadow 合法推进主路径为 planned → seeded → reinforced → payoff_ready → paid_off → closed，也可在允许阶段 retired；final export 前所有 foreshadow 必须是 closed 或 retired。

首次创建冲突使用 local_id，并同时提供 description、participants、state=open、escalation_condition、close_condition 和 event_ref；participants 必须是唯一非空字符串数组，每个元素都引用 canonical character ID。只有 ACCEPTED 时 Core 才分配永久 conflict ID，并通过 id_mappings 返回；后续状态推进使用 conflict_id 字段填写该 canonical ID。transition 如果再次携带 participants，也必须满足同样结构且参与方集合不可变；description、escalation_condition、close_condition 与 participants 都是定义性字段创建后不可改写，transition 可省略，显式提供时必须与前态一致。合法主路径为 open → escalated → resolved，也允许 open/escalated → retired；每次推进都必须引用已验收事件证据。final export 前所有 conflict 必须是 resolved 或 retired。Core 只验证状态边、参与方、条件字段和证据引用，不判断文学意义上的冲突是否精彩或是否真的解决。

首次创建读者承诺使用 local_id，并提供 reader_promise.statement；只有 ACCEPTED 时 Core 才分配永久 reader promise ID，并通过 id_mappings 返回，后续状态推进使用 promise_id 字段填写该 canonical ID。reader_promise.statement 一旦出现必须是字符串；statement 创建后不可改写；后续推进若省略 statement，Core 保留前态文本，显式提供时必须与已有非空 statement 一致。旧 Canon 已存在的历史 promise key 仍兼容继续推进。reader_promise 可用 advanced、deferred、fulfilled、retired；deferred 必须带未来的 deadline_chapter，fulfilled 必须有 event_ref；fulfilled / retired 为终态，不得恢复为 advanced/deferred；final export 前必须是 fulfilled 或 retired。最终主线收束还必须提交带本章事件证据的 ending_resolution。

如果 task 要求 rolling planning 修复，planning_patch.json 最小形状为：

~~~json
{"next_arc":{"id":"arc-2","start_chapter":11,"end_chapter":20,"goal":"下一 Arc 目标"}}
~~~

## REWRITE、BLOCKED 与 result

处理后读取 exchange/result/<attempt_id>.json：

- ACCEPTED：Canon 推进，继续读取新的 READY。
- REWRITE / RETRY：读取 violations/problem，并只向新的 attempt 提交修正版；旧 attempt 不再写。
- BLOCKED：不要继续改旧提交。读取 result 中 block_id、constraint_refs、conflict、options，再通过 control channel 给出作者选择。

如果确实存在不可由 ChatGPT 自行决定的硬约束冲突，可在 self_review.json 中提交 author_decision_required；其中 constraint_refs 必须引用 chapter_contract.json.hard_constraints 中真实存在的 id，并至少给两个 options。

## 作者 control channel

目录固定为 exchange/control/inbox/<message_id>/，包含 control.json 与 manifest.json。message_id 使用该目录名；base_canon_root 必须读取 exchange/STATUS.json.canon_root。

control manifest：

~~~json
{"schema_version":1,"project_id":"当前项目","message_id":"ctrl-001","base_canon_root":"从 STATUS.json 复制","protocol_version":"1.0","files":["control.json"]}
~~~

future_plan 作者指令：

~~~json
{"schema_version":1,"project_id":"当前项目","message_id":"ctrl-001","kind":"author_directive","base_canon_root":"从 STATUS.json 复制","directive_scope":"future_plan","instruction":"后续任务必须遵守的作者指令"}
~~~

historical_revision：

~~~json
{"schema_version":1,"project_id":"当前项目","message_id":"ctrl-002","kind":"author_directive","base_canon_root":"从 STATUS.json 复制","directive_scope":"historical_revision","instruction":"要修改什么以及原因","chapter":3}
~~~

block_resolution：

~~~json
{"schema_version":1,"project_id":"当前项目","message_id":"ctrl-003","kind":"block_resolution","base_canon_root":"从 STATUS.json 复制","block_id":"从 BLOCKED result/STATUS 复制","choice":"从 BLOCKED result 的 options 中原样复制一个值"}
~~~

block_resolution.choice 必须与 BLOCKED result 中某个 options 值完全相等，不要自行改写或追加解释。control 也遵循“先 control.json，最后 manifest.json”。处理结果读取 exchange/control/result/<message_id>.json。historical_revision ACCEPTED 后按新的 revision READY 顺序重写；在 replay 追平原 head 前不要绕过 READY 自行跳章。

## 新对话恢复

新 ChatGPT 对话不依赖旧聊天记录。先读取 project.json、CHATGPT_PROTOCOL.md、exchange/STATUS.json、exchange/READY.json（若存在）以及当前 READY 对应的 outbox。Chapter/Revision 的 canonical Foundation 资料从 context.json.foundation_reference 读取；当前权威根从 STATUS.json.canon_root 读取。只根据这些 Core 生成文件继续工作。
`, CurrentVersion, projectID)
}

func DecodeJSON(data []byte, dst any) error {
	var shape any
	if err := json.Unmarshal(data, &shape); err != nil {
		return fmt.Errorf("decode json: %w", err)
	}
	if err := validateJSONShape(shape, 1); err != nil {
		return err
	}
	if err := json.Unmarshal(data, dst); err != nil {
		return fmt.Errorf("decode json target: %w", err)
	}
	return nil
}

func validateJSONShape(v any, depth int) error {
	if depth > MaxJSONDepth {
		return fmt.Errorf("json exceeds max depth %d", MaxJSONDepth)
	}
	switch x := v.(type) {
	case []any:
		if len(x) > MaxJSONArrayLength {
			return fmt.Errorf("json array exceeds max length %d", MaxJSONArrayLength)
		}
		for _, item := range x {
			if err := validateJSONShape(item, depth+1); err != nil {
				return err
			}
		}
	case map[string]any:
		for _, item := range x {
			if err := validateJSONShape(item, depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}

const MachineSchemaVersion = 1

type SubmissionManifest struct {
	SchemaVersion   int      `json:"schema_version"`
	ProjectID       string   `json:"project_id"`
	TaskID          string   `json:"task_id"`
	AttemptID       string   `json:"attempt_id"`
	BaseCanonRoot   string   `json:"base_canon_root"`
	ProtocolVersion string   `json:"protocol_version"`
	TaskDigest      string   `json:"task_digest"`
	CompletionNonce string   `json:"completion_nonce"`
	Files           []string `json:"files"`
}
