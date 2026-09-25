# novel-studio — Provider-Free Novel Core

`novel-studio` 这个 fork 现在是一套 **local-first、provider-free 的长篇小说状态核心**。

受支持的运行组合保持精简：

- 普通 **ChatGPT App**：负责讨论、构思、写作、改写与提交文本文件；
- **Google Drive**：只作为文件交换与备份传输层；
- **符合要求的本地 Google Drive transport**：把真实 Drive workspace 暴露成本机普通目录；macOS/Windows 可用 Drive Desktop，Linux/Vultr 可用 rclone；
- 本机 **`novel-core`**：负责确定性状态、校验、事务、恢复、修订、备份和导出。

`novel-core` 不调用 OpenAI API 或其他模型 API，不需要 provider、model、API key、Ollama、MCP、ChatGPT Work、embedding 或 Qdrant。小说内容由普通 ChatGPT App 与作者共同完成；Core 只判断能够机械证明的结构化约束，不冒充文学审稿人。

## 为什么这样设计

聊天很适合创作，却不适合作为百万字连载的唯一数据库。这个 fork 把“真相”从聊天上下文移到本地可验证文件：人物知识、事件证据、地点、资源、关系、伏笔、冲突、读者承诺、滚动规划、章节版本根与回执链都由 `novel-core` 维护。

因此即使换 ChatGPT 对话、重启 Core、Drive 发生延迟，当前活动状态仍可从本地权威文件恢复；历史章节修改则进入显式 revision/replay，而不是偷偷覆盖后文。

## 快速开始

### 1. 准备

需要：

- Go（源码运行时）；或本项目 Release 中的 `novel-core`；
- Google Drive + 一个符合要求的本地 Drive transport（macOS/Windows 可用 Drive Desktop；Linux/Vultr 可用 rclone）；
- 一个普通 ChatGPT App 会话。

不需要任何模型密钥。

Transport 实现示例：macOS/Windows 可继续使用 Google Drive Desktop；Linux/Vultr 推荐使用 rclone Google Drive backend + `rclone mount` + VFS cache。两者都只是传输层，`meta/core/**` 的本地权威不变。

源码运行：

```bash
git clone https://github.com/meilee10006/novel-studio.git
cd novel-studio
git switch provider-free-novel-core
./scripts/run-local.sh --help
```

Release 安装：

```bash
curl -fsSL https://raw.githubusercontent.com/meilee10006/novel-studio/provider-free-novel-core/scripts/install.sh | sh
novel-core --version
```

### 2. 初始化一本书

假设符合要求的本地 Drive transport 把真实 Google Drive workspace 暴露为 `$HOME/novel-drive-workspace`：

```bash
novel-core init \
  --project "$HOME/novels/book-local" \
  --workspace "$HOME/novel-drive-workspace" \
  --project-id book-001
```

初始化会写出能力检查文件和由当前二进制生成的 `CHATGPT_PROTOCOL.md`。不要在仓库里维护另一份手写协议。

### 3. 在 ChatGPT App 中完成 capability check

把 Drive workspace 中的 `CHATGPT_PROTOCOL.md` 和 `setup/capability-challenge.json` 提供给 ChatGPT App，按协议要求让它写回：

- `setup/capability-ack.json`
- `setup/capability-write-test.md`

Core 只有确认普通 UTF-8 JSON/Markdown 读写能力后才会接受后续交换。

fresh protocol 1.1 / core schema 2 项目默认 `design_mode=required`。**第一份 Canon 建立前不会生成 `exchange/READY.json`**；此阶段不要等待 READY，而是按 `CHATGPT_PROTOCOL.md` 使用 `exchange/design/inbox/<submission-id>/` 与 `exchange/design/result/`：

```text
init
→ capability ack
→ creative_brief / story_decisions / story_concept
→ story_review + author_confirmation
→ story_locked
→ 七个建书设计文件 + 完整十槽 bundle
→ foundation_readiness_review
→ foundation_ready
→ Core 内部 Foundation settlement
→ chapter:1 READY
```

`story_locked` 与 `foundation_ready` 都是本地内容寻址 Design authority 的检查点。候选资料或新 source 的导入不会自动移动已锁定的 Design Head；到达 `foundation_ready` 后，建书设计基线冻结。

### 4. 启动 Core

```bash
novel-core serve --project "$HOME/novels/book-local"
```

`serve` 以低频扫描作为正确性机制。文件 watcher 即使漏事件，也不会影响下一次扫描发现完整提交。

查看当前状态：

```bash
novel-core status --project "$HOME/novels/book-local"
```

### 5. 在 ChatGPT App 中继续 Design 或逐任务创作

先读 `project.json`、生成的 `CHATGPT_PROTOCOL.md` 和 `exchange/STATUS.json`。`CHATGPT_PROTOCOL.md` 是操作协议唯一正文来源；不要从 README 或旧聊天推断 machine protocol。

- `design_mode=required` 且还没有 Canon：依据 `STATUS.design_head` / `STATUS.design_checkpoint` 和 `exchange/design/**` 继续 Design，**不读取、不等待 READY**。
- 已建立 Canon 且 STATUS 有 active production attempt：以 `exchange/READY.json` 为当前 production task 指针，并读取对应 `exchange/outbox/<task>/<attempt>/`。
- `design_mode=legacy`：保持兼容旧流程，capability 通过后仍由外部 ChatGPT 提交 Foundation READY/Inbox，再进入 Chapter 链。

`exchange/STATUS.json` 投影当前权威 `design_mode`、`design_head`、`design_checkpoint`、`foundation_design_root`、`canon_root`、capability、活动 task/attempt/block，以及 `export_ready` / `export_problems`。临近结局时以 `export_problems` 检查全量确定性阻塞项；`export_ready=true` 只表示导出前置条件满足，真正 `export` 仍会重新验证 Canon 完整性。

Drive 多文件同步不是原子操作：production 阶段只有当 `STATUS.active_attempt_id == READY.attempt_id`、`STATUS.active_target == READY.target` 且 `STATUS.block_id == READY.block_id`（未阻塞时两边为空）时才继续；不一致就等待同步后重读。把完整 production 提交写到：

```text
exchange/inbox/<task-id>/<attempt-id>/
```

正式提交遵循“先写全部 artifact，最后写 `manifest.json`”。Chapter/Revision 的 `context.json.foundation_reference` 会携带 Core 规范化后的 Foundation 资料与 canonical IDs，新 ChatGPT 对话不需要依赖旧聊天记录猜人物/地点 ID。Core 会把稳定字节锁成本地 snapshot，再产生 `ACCEPTED`、`REWRITE` 或 `BLOCKED`。

- `ACCEPTED`：推进当前 Canon；
- `REWRITE`：同一 task 创建新 attempt，按结构化反馈修改；
- `BLOCKED`：等待作者通过 control channel 裁决。

文学质量、情绪强度、节奏、文风，以及 story/review 中的主观判断由作者与 ChatGPT 负责；Core 只验证内容寻址、引用、结构、证据绑定与事务条件，不声称能够机械判断“故事好不好”。

### 章节质量链

每个 attempt 都以 `outbox/.../task.json.required_artifacts` 为唯一文件合同；不要从旧聊天或固定文件清单猜当前要交什么。新创建的 Chapter/Revision attempt 使用显式 **plan → draft → review** 链：

1. `chapter_plan.json`：先明确本章 objective、reader payoff、至少两个 beats 和 ending hook intent；
2. `chapter.md`：按计划写正文，并继续提交既有 `chapter_contract.json`、`events.json`、`state_delta.json`、`self_review.json`；
3. `chapter_review.json`：精确绑定同一 immutable attempt snapshot 中的 `chapter.md` 与 `chapter_plan.json`，逐项检查 story progress、reader payoff、pacing、character/world consistency、continuity、ending hook。

如果 reviewer 明确给出 `verdict=revise`，Core 不替它做文学判断，而是机械执行现有 `REWRITE`：Canon 不动、task 不变、新建 attempt，下一次必须重新提交 replacement plan/draft/review。

质量链 attempt 提交 `planning_patch.json` 时还必须带 `arc_rehearsal.json`：至少推演两个下一 Arc 候选并明确选择其中一个；只有被选中的 `planning_patch.json` 会改变权威 rolling planning，rehearsal 只是不可变选择证据。旧 active attempt 仍严格按它自己的 `required_artifacts` 完成，不会被升级时追加强制文件。

现有项目的 `CHATGPT_PROTOCOL.md` 是 Core-owned 派生投影；Core Reconcile 会在内容过期时安全刷新它，但不会因此改变 Canon、task、attempt 或 task digest。

Design Store 的本地权威位于 `meta/core/design/`。Google Drive 的 `exchange/design/**` 只是交换层；Drive 文件不会自动成为 Design authority，只有经 Core 导入、内容寻址和 promote 后才进入正式 Design 历史。

## 作者指令与历史修订

作者指令写入：

```text
exchange/control/inbox/<message-id>/
```

control 的 `base_canon_root` 必须从当前 `exchange/STATUS.json.canon_root` 复制；历史 revision 的 READY 可能绑定较早父 root，不能拿它替代当前权威 root。

章节中首次正式出现的新人物/地点/资源可分别通过 `state_delta.character_add` / `state_delta.location_add` / `state_delta.resource_add` 使用本次 attempt 的 `local_id` 声明；同一提交可引用这些 local ID。只有 ACCEPTED 后 Core 才分配永久 character/location/resource ID，并在 result 的 `id_mappings` 中返回；后续章节必须使用 canonical ID。首次创建伏笔同样使用 `state_delta.foreshadow.local_id`；ACCEPTED 后从 `id_mappings` 取得永久 foreshadow ID，后续生命周期推进只使用该 canonical ID。首次创建故事冲突使用 `state_delta.conflict.local_id`，同时声明 canonical character `participants`、升级/关闭条件与事件证据；ACCEPTED 后取得永久 `conflict-*` ID，后续只用 `conflict_id`，合法主路径为 `open → escalated → resolved`，也可从 `open/escalated` 进入 `retired`。首次创建读者承诺也使用 `state_delta.reader_promise.local_id`；ACCEPTED 后取得永久 reader-promise ID，后续推进只使用 canonical `promise_id`。

首发支持：

- `future_plan`：只改变未来规划，不重写已发生事实；
- `historical_revision`：从最早受影响章节建立新分支，并按顺序 replay 后续章节；
- `block_resolution`：解除明确的作者决策阻塞。

历史 replay 未追平原 head 前禁止 final export。

## 验证、备份、恢复与迁移

验证当前 Canon root、活动回执链和权威产物：

```bash
novel-core verify --project "$HOME/novels/book-local"
```

创建可验证备份由 Core API/运行流程生成；恢复 CLI 只允许写入**不存在的新目录**：

```bash
novel-core restore \
  --backup /path/to/verified-backup \
  --project "$HOME/novels/restored-book"
```

本地 schema 或 Drive protocol 升级必须显式迁移，且迁移前先有可验证备份：

```bash
novel-core migrate \
  --project "$HOME/novels/book-local" \
  --backup /path/to/pre-migration-backup
```

未知未来 schema/protocol 会 fail closed。仅协议升级不得改变小说 `canon_root`。

## 最终导出

只有满足确定性结局条件、没有活动 replay、没有未终态伏笔/冲突/读者承诺时才能导出；冲突必须是 `resolved` 或 `retired`。可先用 `novel-core status` 或 Drive `exchange/STATUS.json` 查看 `export_ready` 与完整 `export_problems`；最终导出仍会执行更严格的 Canon 完整性校验：

```bash
novel-core export \
  --project "$HOME/novels/book-local" \
  --out "$HOME/novels/book-final.md"
```

导出只读取当前活动 Canon head，不扫描已被 superseded 的旧章节文件。

## Notion

Notion 不是数据库。Core 可生成只读 `projection/notion.json`，供外部流程同步到 Notion；投影失败不会阻塞写作，Notion 中的人工修改也不会反向覆盖本地权威状态。

## 安全边界

Drive 中的输入全部按不可信文件处理。Core 会拒绝路径穿越、绝对路径、符号链接、未知 artifact、非法 UTF-8、超大 manifest/chapter、过深 JSON 与过长数组。未知文件不会进入 Canon。

项目级跨进程锁保证同一项目同一时刻只有一个 writer；`status` 可并发只读，`verify`/export 使用稳定读快照。

## CLI

```text
novel-core init
novel-core serve
novel-core status
novel-core verify
novel-core export
novel-core restore
novel-core migrate
novel-core --version
```

CLI 没有 `--provider`、`--model` 或 `--api-key`。

## 当前边界

首发是单作者、单活动修订链、普通文件协议。它不提供多人实时协作、分支合并、自动文学质量评分，也不承诺任何发布平台的流量或自动审核结果。

保留的 `assets/references/` 与 `assets/styles/` 是写作参考资料，不是模型运行时依赖。

## 上游与许可证

本 fork 来源于 [Xiaoyangy/novel-studio](https://github.com/Xiaoyangy/novel-studio)，继续遵循仓库中的 **Apache-2.0** `LICENSE` 并保留必要归属。此 fork 的 provider-free 架构与修改由本 fork 维护，不表示得到上游作者背书。

当前 fork：`meilee10006/novel-studio`。
