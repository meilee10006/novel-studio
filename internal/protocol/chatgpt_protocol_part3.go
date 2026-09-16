package protocol

const chatGPTProtocolPart3 = `knowledge_add 的 fact_id 是全局稳定事实标识，statement 是后续新对话可直接理解的事实文本；knowledge statement 一旦出现必须是字符串；新增知识应同时提供两者。同一 fact_id 跨角色共享同一 statement 语义，不同事实或不同解释应使用不同 fact_id。同一角色已有非空 statement 时不得改写；后续省略 statement 时 Core 保留既有文本。observed source 要求对应 events.json 事件把该角色列在 observers 中，例如 {"local_id":"e1","evidence_anchor":"...","observers":["character-000001"]}。transmitted source 必须提供 from_character_id；from_character_id 必须出现在 event actors，接收角色必须出现在 event observers，并且来源角色在前态已经知道同一 fact_id。transmitted 不得改写来源 fact 的 statement；来源已有 statement 时可省略该字段，由 Core 继承来源文本。旧 Canon 中没有 statement 的知识仍按 fact_id 兼容读取。relationship_id 必须引用两个不同 canonical characters；relationship 的 tags 可省略，tags 一旦出现必须是唯一非空字符串数组。resource、location、relationship、foreshadow、conflict、ending_resolution 需要事件证据。location.start_tick 不得早于该角色前态 end_tick；地点变化时若存在匹配的 travel_constraints，还必须满足对应 min_ticks。

首次创建伏笔使用 local_id，并提供 foreshadow.description 作为后续新对话可读的伏笔语义；只有 ACCEPTED 时 Core 才分配永久 foreshadow ID，并通过 id_mappings 返回，后续状态推进使用 foreshadow_id 字段填写该 canonical ID。foreshadow.description 一旦出现必须是字符串；description 创建后不可改写；后续推进若省略 description，Core 保留前态描述，显式提供时必须与已有非空 description 一致。旧 Canon 已存在的历史伏笔 key 仍兼容继续推进。foreshadow 合法推进主路径为 planned → seeded → reinforced → payoff_ready → paid_off → closed，也可在允许阶段 retired；final export 前所有 foreshadow 必须是 closed 或 retired。

首次创建冲突使用 local_id，并同时提供 description、participants、state=open、escalation_condition、close_condition 和 event_ref；participants 必须是唯一非空字符串数组，每个元素都引用 canonical character ID。只有 ACCEPTED 时 Core 才分配永久 conflict ID，并通过 id_mappings 返回；后续状态推进使用 conflict_id 字段填写该 canonical ID。transition 如果再次携带 participants，也必须满足同样结构且参与方集合不可变；description、escalation_condition、close_condition 与 participants 都是定义性字段创建后不可改写，transition 可省略，显式提供时必须与前态一致。合法主路径为 open → escalated → resolved，也允许 open/escalated → retired；每次推进都必须引用已验收事件证据。final export 前所有 conflict 必须是 resolved 或 retired。Core 只验证状态边、参与方、条件字段和证据引用，不判断文学意义上的冲突是否精彩或是否真的解决。

首次创建读者承诺使用 local_id，并提供 reader_promise.statement；只有 ACCEPTED 时 Core 才分配永久 reader promise ID，并通过 id_mappings 返回，后续状态推进使用 promise_id 字段填写该 canonical ID。reader_promise.statement 一旦出现必须是字符串；statement 创建后不可改写；后续推进若省略 statement，Core 保留前态文本，显式提供时必须与已有非空 statement 一致。旧 Canon 已存在的历史 promise key 仍兼容继续推进。reader_promise 可用 advanced、deferred、fulfilled、retired；deferred 必须带未来的 deadline_chapter，fulfilled 必须有 event_ref；fulfilled / retired 为终态，不得恢复为 advanced/deferred；final export 前必须是 fulfilled 或 retired。最终主线收束还必须提交带本章事件证据的 ending_resolution。

如果 constraints.json.control_constraints 包含 kind=rolling_planning_due，表示当前章节已进入 current_arc.planning_lead_chapters 的提前规划窗口；应尽量随本章提交合法 planning_patch.json，但提前规划阶段仍可省略 planning_patch.json，正文继续独立验收。只有越过当前 Arc 边界仍没有合法下一 Arc 时，后续 task 才会以 planning_repair_required 把 planning_patch.json 列入 required_artifacts，变成写正文前必须修复的硬前置。

如果 task 要求 rolling planning 修复，planning_patch.json 最小形状为：

~~~json
{"next_arc":{"id":"arc-2","start_chapter":11,"end_chapter":20,"goal":"下一 Arc 目标"}}
~~~

## REWRITE、BLOCKED 与 result

处理后读取 exchange/result/<attempt_id>.json：

- ACCEPTED：Canon 推进，继续读取新的 READY。
- REWRITE / RETRY：读取 violations/problem，并只向新的 attempt 提交修正版；旧 attempt 不再写。REWRITE 会继续保留 violations 兼容字段，并同时给出 rewrite_feedback 结构化反馈；优先按 rewrite_feedback 精确修复。
- BLOCKED：不要继续改旧提交。读取 result 中 block_id、constraint_refs、conflict、options，再通过 control channel 给出作者选择。

REWRITE 的 rewrite_feedback 与 violations 一一对应且顺序一致。每项包含稳定机器可读 code、可选 entity_ref、expected、observed、evidence_refs、allowed_scope；evidence_refs 指向 Core 实际读取的证据文件或 JSON pointer，allowed_scope 只列允许修改的提交 artifact，不授权修改 Core 权威状态。示例：

~~~json
{"result":"REWRITE","violations":["chapter_contract.hard_constraints must be an array"],"rewrite_feedback":[{"code":"chapter_contract.hard_constraints.invalid","entity_ref":"chapter:12","expected":"chapter_contract.hard_constraints is a valid unique-id array","observed":"chapter_contract.hard_constraints must be an array","evidence_refs":["chapter_contract.json#/hard_constraints"],"allowed_scope":["chapter_contract.json"]}]}
~~~

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
`
