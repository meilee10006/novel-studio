package protocol

const chatGPTProtocolPart2 = `hard_constraints 可省略；hard_constraints 一旦出现必须是数组，每一项必须是对象并包含唯一非空 id。author_decision_required.constraint_refs 只能引用这些 id，不要重复、留空或自行引用不存在的约束。offscreen constraint_refs 也只能引用这些 id；为兼容既有提交，offscreen constraint_refs 的重复引用会被兼容接受，但重复项不表达额外语义。

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

events.json 的 local_id 只在本次提交中使用；events.json 不得自行提供 canon_id，永久 story-event ID 只由 Core 在 ACCEPTED 时分配。kind 只支持 onscreen / offscreen。actors/observers 可省略；actors/observers 一旦出现必须是字符串数组，数组元素必须是非空人物引用，且 actors/observers 数组内不得重复人物引用。同一提交新人物可先使用 local_id，Core 在 ACCEPTED 前会统一改写并校验 canonical character ID。除 kind=offscreen 外，可见事件必须有 evidence_anchor，且 anchor 必须逐字出现在 chapter.md。离屏事件必须显式使用 kind=offscreen；constraint_refs 必须是非空字符串数组，并至少包含一个元素，其中每个 ID 都必须逐项命中当前 chapter_contract.hard_constraints[].id，表示这个离屏事实已被本章硬约束明确允许。Core 只验证这些引用真实存在，不判断离屏剧情在文学上是否合理。state_delta 中要引用本章事件时使用 event_ref；event_ref 一旦出现必须是非空字符串。Core ACCEPTED 时会把它改成 event_canon_id。event_ref 与 event_canon_id 不得同时提供；event_ref 与 event_canon_id 字段键不得同时出现，即使 event_canon_id 的值写成 null 也不允许；event_canon_id 一旦出现必须是非空字符串，且 event_canon_id 必须引用已验收 story event，不能自行发明不存在的 canonical event ID。当前 attempt 新事件只能使用 event_ref，不能预猜将分配的 story-event-*；需要引用历史已验收事件时只填写其已有 event_canon_id。state_delta.json.changes 必须是数组；没有结构化状态变化时也必须显式提交 {"changes":[]}。

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

`
