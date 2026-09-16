# Provider-Free Protocol Completion Design

状态：已批准，待实现
日期：2026-09-16
目标分支：`provider-free-novel-core`

本设计细化 `2026-09-13-provider-free-novel-core-design.md` 的 §17、§20.3、§20.4；不改变 provider-free 总体边界，也不单独提升协议版本。

## 1. 兼容策略

协议 `1.0` 保持可读取。已经验收的旧提交、历史 replay 和已有最小 `chapter_contract.json` 不因本次扩展失效。新生成的 READY/ChatGPT 协议指导使用完整结构；Core 对新增字段采用“出现则严格机械验证”的兼容策略。

`REWRITE` 继续保留现有 `violations: []string`，同时增加结构化 `rewrite_feedback`。旧结果不回填，新结果同时输出两者。普通 ChatGPT App 仍是唯一 AI/创作层，Core 不增加文学判断。

## 2. 完整 chapter_contract

新生成章节应使用以下结构：

```json
{
  "chapter": 12,
  "declared_pov": "character-000001",
  "start_state": {"canon_root": "..."},
  "purpose": "本章要完成的叙事目的",
  "main_conflict": "本章主冲突",
  "reader_question": "本章持续推动的读者问题",
  "obligation_refs": ["foreshadow-000010", "promise-000011"],
  "immutable_refs": ["character-000001"],
  "foreshadow_operations": [
    {"foreshadow_id": "foreshadow-000010", "allowed_states": ["reinforced", "payoff_ready"]}
  ],
  "expected_changes": ["location", "relationship"],
  "ending_hook": "章末钩子说明",
  "target_length": {"min_chars": 1800, "max_chars": 2600},
  "planning_obligations": ["rolling_planning_due"],
  "hard_constraints": [{"id": "hc-001"}]
}
```

字段语义固定如下：

- `chapter`、`declared_pov`、`hard_constraints` 保持现有语义；历史最小 contract 仍合法。
- `start_state` 必须是对象；首发允许 `canon_root`。若出现 `canon_root`，必须等于该 attempt 的 `base_canon_root`。它只是前态声明，不是第二份权威状态。
- `purpose`、`main_conflict`、`reader_question`、`ending_hook` 一旦出现必须是非空字符串。Core 只检查结构，不判断文学质量。
- `obligation_refs`、`immutable_refs`、`planning_obligations` 一旦出现必须是唯一非空字符串数组；引用只允许指向当前上下文中已经存在、可机械证明的对象或已下发任务约束。
- `foreshadow_operations` 一旦出现必须是对象数组；每项包含非空 `foreshadow_id` 和唯一非空字符串数组 `allowed_states`。Core 验证 ID 存在，并验证本次结构化 foreshadow 变化未超出允许状态集合。
- `expected_changes` 一旦出现必须是唯一非空字符串数组；元素只能来自 Core 支持的 state change kind。实际 `state_delta.json` 至少覆盖这些类别；额外合法变化仍允许。
- `target_length` 一旦出现必须包含非负整数 `min_chars`、`max_chars`，且 `min_chars <= max_chars`。Core 使用 `chapter.md` 的 Unicode rune 数机械校验。
- `planning_obligations` 只承载当前 task/constraints 已经给出的滚动规划义务；Core 不从自然语言推断新义务。

新 READY 的 ChatGPT 协议文本要求生成这些完整字段；为兼容既有 `1.0` 提交和 replay，Core 不因新增字段缺失本身拒绝旧最小 contract。

## 3. 结构化 REWRITE

新产生的 `REWRITE` 结果形状：

```json
{
  "result": "REWRITE",
  "violations": ["chapter_contract.target_length is invalid"],
  "rewrite_feedback": [
    {
      "code": "chapter_contract.target_length.invalid",
      "entity_ref": "chapter:12",
      "expected": "0 <= min_chars <= max_chars",
      "observed": "min_chars=2600 max_chars=1800",
      "evidence_refs": ["chapter_contract.json#/target_length"],
      "allowed_scope": ["chapter_contract.json"]
    }
  ]
}
```

字段语义固定如下：

- `code`：稳定、机器可读、非空字符串；同一种确定性校验失败使用同一 code，不嵌入动态 ID。
- `entity_ref`：可选非空字符串，指向最直接相关的 canonical/local/task/chapter 对象；没有单一实体时省略。
- `expected`：非空字符串，描述 Core 实际执行的机械规则。
- `observed`：非空字符串，描述本次提交观察到的结构化值或状态，不推测文学动机。
- `evidence_refs`：唯一非空字符串数组，使用文件名或 `file.json#/json/pointer` 指向 Core 已读取证据；不能细分时至少指向相关文件。
- `allowed_scope`：唯一非空字符串数组，只列出修复该 violation 允许修改的提交 artifact；不得授权修改 Core 权威状态文件。

`violations` 与 `rewrite_feedback` 一一对应并保持确定性顺序。`REWRITE` 仍保持 task、切换 attempt；结构化反馈只是兼容扩展，不改变工作流语义。

## 4. 确定性边界

Core 只验证结构、类型、唯一性、可证明引用、已有状态机、实际 state change kind、已下发 planning obligation 和 Unicode rune 长度。Core 不判断 `purpose` 是否精彩、`main_conflict` 是否强、`reader_question` 是否吸引人、`ending_hook` 是否有效，也不从自然语言自动生成新约束。

结构化反馈同样只描述已经执行的确定性校验。不能把自由文本 violation 送给模型或 NLP 再“推断” code/entity/scope。

## 5. 验收边界

本设计完成的自动化标准：

- 旧最小 `chapter_contract` 仍可通过既有兼容路径；
- 新字段一旦出现，错误类型/重复值/未知引用/非法状态/非法长度等产生确定性 `REWRITE`；
- 每个新 `REWRITE` 同时保留 `violations` 并输出一一对应的 `rewrite_feedback`；
- 反馈进入 validation digest、workspace result 和返回 settlement，崩溃恢复/重试结果保持一致；
- generated ChatGPT protocol 展示完整 contract 和结构化 REWRITE；
- 全量 test/vet/race/build/provider-free dependency boundary 通过。

普通 ChatGPT App + Google Drive + Drive Desktop 的真实产品验收仍是独立人工发布门槛；自动测试不能替代，也不能提前标记为 PASS。
