# 无 AI Novel Core 实施计划

> 状态：已执行的历史实施计划。2026-09-23 的语义设计基线规格新增了建书前流程，并覆盖本计划中“capability 后直接创建 foundation READY”的初始化语义。不要把本文件继续追加成新规格的实施计划；当前新规格尚无实施计划，通过审阅后再单独生成。

目标规格：`docs/superpowers/specs/2026-09-13-provider-free-novel-core-design.md`

这份计划只描述最终实施路径和验收条件，不保留评审过程。执行时按任务顺序推进；每个任务都先写会失败的高层行为测试，再写最小实现，验证通过后再进入下一任务。

## 实施原则

- 上游基线固定为 `e3beebbf2f35b9ff55fd781d82055b60d45970c8`，开发分支为 `provider-free-novel-core`。
- 新生产入口只走 `cmd/novel-core → internal/core`。
- `cmd/novel-core` 与 `internal/core` 不得依赖 `internal/agents`、`bootstrap.ModelSet`、`agentcore`、`internal/llmcodex` 或模型 Provider。
- 不新增数据库、向量数据库、模型服务和云端后端；Drive Desktop 只是目录同步层。
- 不为单一实现提前造 interface/adapter；优先做深模块和窄公开面。
- Core 只做可确定验证，文学判断留给 ChatGPT/作者。
- 正式校验只读取 Core 形成的本地不可变提交快照，不直接在 Drive 文件上判定业务结果。
- 历史修订必须顺序重放下游章节，不能只修状态文件。
- 任何完成声明都以新鲜的测试、vet、依赖图或人工验收记录为依据。

## 目标模块

首发只新增或深化这些模块，避免把一条生产链拆成大量浅文件：

- `cmd/novel-core`：极薄 CLI；
- `internal/core`：项目级工作流、任务推进、提交、恢复、修订、导出；
- `internal/protocol`：Drive 协议、安全文件读取、摘要、本地快照；
- `internal/domain`：任务、尝试版本、故事事件、状态变化、结局约束等领域类型；
- `internal/store`：继续作为本地权威持久化；
- 现有 `rules`、`retrieval`：只复用确定性部分。

`core.Project` 是主要测试和调用边界。除非出现第二个真实实现，不额外抽 `TaskExporter`、`SubmissionImporter`、`Projector` 等接口。

---

## 任务 0：环境与基线

**目标**：确认实施环境可用，并建立不会误报回归的基线。

### 要做的事

- 确认当前分支、上游基线和 spec/plan 是唯一已有改动；
- 确认本机有 Go 1.25.5；
- 下载依赖；
- 跑完整测试和 vet；
- 如果基线失败，先判断是否是环境缺失、稳定的上游既有失败，还是当前分支引入的失败。

### 验证

```bash
git branch --show-current
git merge-base --is-ancestor e3beebbf2f35b9ff55fd781d82055b60d45970c8 HEAD
go version
go mod download
go test ./...
go vet ./...
```

### 完成条件

- Go 版本符合仓库要求；
- 能给出可信基线；
- 当前分支没有未说明的生产代码改动；
- 如果测试基线本来不全绿，受影响包和失败原因已经明确，后续不得把同一失败误算成新回归。

---

## 任务 1：建立真正无 AI 的 Core 主干

**目标**：先让一个最小 `novel-core` 在没有任何模型配置时能启动、打开本地存储、查看状态和校验项目。

### 主要改动

- 新增 `cmd/novel-core`；
- 新增 `internal/core.Project`，只暴露项目级操作；
- 直接复用现有 `store.NewStore` 与确定性一致性检查；
- 增加依赖图测试，禁止新入口把旧 AI runtime 带进来。

### 先写的行为测试

- 清空常见模型 API Key 后，`novel-core status` / `verify` 仍能运行；
- `go list -deps ./cmd/novel-core` 不出现 `agentcore`、`internal/agents`、`internal/llmcodex`；
- `core.Project` 能打开空项目和现有本地存储，并返回明确状态。

### 验证

```bash
go test ./internal/core ./cmd/novel-core -v
go list -deps ./cmd/novel-core
go test ./...
```

### 完成条件

- 新 CLI 不读取模型配置；
- 新依赖方向成立；
- 没有为了测试暴露内部 helper；
- 旧 runtime 仍可暂时存在，但不在新生产路径上。

---

## 任务 2：项目初始化、协议和能力检查

**目标**：打通“本地权威目录 + Drive workspace + capability ack”，但还不写正文。

### 主要改动

- `internal/protocol`：协议版本、安全路径、原子写入、摘要和大小限制；
- `internal/core`：项目初始化、能力检查、`CHATGPT_PROTOCOL.md` 生成；
- `internal/store`：新增任务/尝试版本/回执/控制消息所需的最小生产状态；
- Drive workspace 目录和唯一写入方按 spec 固定下来。

### 先写的行为测试

- 本地权威目录和 Drive workspace 必须不同；
- capability ack 的 nonce 错误时不能进入正式任务；
- 没有 ack 时不得生成 READY；
- 路径穿越、绝对路径、越界符号链接、超大输入必须拒绝；
- `CHATGPT_PROTOCOL.md` 只有一个代码生成源，并写入当前协议版本。

### 验证

```bash
go test ./internal/protocol ./internal/store ./internal/core ./cmd/novel-core -v
go test ./...
```

### 完成条件

- `novel-core init` 在无模型环境可用；
- capability check 能提前发现 Drive 无写能力；
- 能力检查失败时直接报告当前运行模式不可用，不降级使用 Google Docs、Sheets 或其他富文档格式承载协议；
- Drive 还没有任何路径能直接改本地权威状态。


---

## 任务 3：首次建书与第一份权威状态

**目标**：让建书本身也走正式任务/提交/回执，不留“先手工写进本地存储”的旁路。

### 主要改动

- 新增 `foundation` 任务；
- 定义首次建书提交文件和结构化字段；
- Core 在 ACCEPT 时分配永久实体 ID，并重写提交包内交叉引用；
- 建立第一份 `canon_root` 和回执；
- 首次建书通过后自动生成第 1 章 READY。

### 先写的行为测试

- capability ack 之后只生成一个活动 `foundation` 尝试版本；
- 首次建书被拒绝时不能产生永久 ID、`canon_root` 或第 1 章；
- 同一临时 ID 在不同类型实体里不会碰撞；
- 首次建书 `ACCEPTED` 后可以重新计算当前 root，并得到相同结果；
- 回执的 `previous_root` / `new_root` 与当前权威状态一致。

### 验证

```bash
go test ./internal/domain ./internal/store ./internal/core -run 'Foundation|CanonRoot|Receipt|ID' -v
go test ./...
```

### 完成条件

- 首次权威状态完全来自已验收的建书提交；
- 没有 provider/model/usage 字段被伪造；
- 第 1 章任务只会在首次建书 `ACCEPTED` 后出现。

---

## 任务 4：Drive 稳定快照、章节提交与第一次正式提交

**目标**：打通最小纵向闭环：第 1 章任务 → Drive 提交包 → 本地稳定快照 → 校验 → 正式提交 → 第 2 章 READY。

### 主要改动

- 对账状态：`PENDING / READY_TO_VALIDATE / INVALID / SETTLED`；
- manifest-last + 两次相同摘要扫描后形成本地不可变快照；
- 提交包白名单和实际字节摘要；
- 最小三层校验骨架；
- 从现有 `commit_chapter.go` 抽出真正确定性的提交/恢复逻辑，不复用 Tool 外壳和模型门禁；
- Core 在正式提交时为故事事件和新的权威对象分配永久 ID；
- 提交日志和幂等恢复；
- ACCEPT 后最后切 READY。

### 先写的行为测试

- 没有 manifest 保持 PENDING；
- manifest 已到但文件不齐仍保持 PENDING；
- 文件齐全但两次摘要扫描不同，不能进入正式校验；
- 稳定快照形成后，Drive 源文件再变不能替换该快照；
- 同一尝试版本相同内容重复处理只结算一次；
- 同一尝试版本不同内容出现时进入冲突/INVALID；
- 过期前态或非活动尝试版本不能推进状态；
- 第 1 章 `ACCEPTED` 后，正文、事件、状态变化、检查点、回执、`canon_root` 和第 2 章 READY 一致推进；
- 提交中途崩溃后重启只能得到“完整成立”或“仍停在旧 root”两种结果。

### 验证

```bash
go test ./internal/protocol ./internal/core ./internal/store -run 'Submission|Snapshot|Commit|Recover|Rewrite|Stale' -v
go test ./...
```

### 完成条件

- 第一条真实章节闭环跑通；
- 正式校验不直接读 Drive 活文件；
- READY 永远是提交最后一步；
- 一个尝试版本最多推进一次权威状态。

---

## 任务 5：返工、协议重试、作者裁决与控制消息

**目标**：把“可修”“协议坏了”“需要作者决定”三种失败彻底分开。

### 主要改动

- `REWRITE`：同一任务生成新尝试版本；
- `retry`：只处理协议/同步可恢复问题；
- `BLOCKED`：只接受确定性硬约束冲突，或 `self_review.json` 中带硬约束引用的 `author_decision_required`；
- `author_directive`、`block_resolution` 独立控制通道；
- 控制消息带 `base_canon_root`，按消息 ID 幂等，过期前态消息直接拒绝。

### 先写的行为测试

- REWRITE 保持 task id、章节和目标不变，只换 attempt id；
- nonce 错误、未知协议版本等不会伪装成 BLOCKED；
- BLOCKED 不推进 Canon；
- `block_resolution` 引用过期 block id 时拒绝；
- `future_plan` 指令不会直接改权威状态，而是进入下一任务约束；
- `historical_revision` 指令只创建 `revision` 任务，不直接覆盖已验收正文；
- 重复控制消息只处理一次。

### 验证

```bash
go test ./internal/core -run 'Rewrite|Retry|Blocked|Directive|Control' -v
go test ./...
```

### 完成条件

- 三类失败有不同协议语义和恢复路径；
- Core 不需要理解自然语言剧情就能决定是否推进工作流；
- 作者指令没有直接写本地存储的旁路。


---

## 任务 6：长篇一致性与上下文编译

**目标**：把真正适合无 AI Core 做的长篇约束落成确定性规则，并保证上下文不会随全书长度失控。

### 主要改动

- 故事事件证据与知识来源链；
- 人物观察派生；
- 时间/地点冲突和已建模移动约束；
- 资源账；
- 关系变化证据；
- 伏笔、冲突、读者承诺状态机；
- 结局硬约束；
- 固定预算的上下文编译器；
- `planning_patch.json` 与滚动 Arc。

### 先写的行为测试

- 人物获得知识但没有合法来源时 REWRITE；
- 同一人物在重叠时间出现在互斥地点时 REWRITE；
- 有明确 TravelConstraint 时不满足最短耗时会失败；没有约束时 Core 不自行猜距离；
- 资源不能减到负数；
- 关系变化、伏笔推进和 读者承诺标记为 `fulfilled` 都必须有已验收事件证据；
- 未满足结局硬条件时不能 final export；
- 可选 planning patch 失败时，合法正文仍可接受；
- 到 Arc 边界仍缺合法规划时，下一章尝试必须先修规划；
- 100 章与 200 章项目的上下文包都受同一总预算约束，而不是近似线性增长。

### 验证

```bash
go test ./internal/core ./internal/domain ./internal/store -run 'Knowledge|Time|Location|Resource|Relationship|Foreshadow|Promise|Ending|Context|Planning' -v
go test ./...
```

### 完成条件

- Core 只判已经结构化、可证明的约束；
- 没有把文学质量偷偷塞进 validator；
- 上下文预算和裁剪优先级有确定行为；
- 首发不需要 embedding 或 Qdrant。

---

## 任务 7：历史修订与顺序重放

**目标**：把已验收历史的修改做成可追溯的新分支，彻底解决“改一章后后面还算不算成立”的问题。

### 主要改动

- `revision` 任务支持目标章节/基础 artifact；
- 修订通过后从最早受影响点建立新的权威状态分支；
- 旧分支更晚章节标记 `superseded`；
- 按章节顺序生成 `attempt_reason=rebase` 的修订尝试版本；
- 任务包带旧正文候选和新的前态；
- 每个下游章节重新提交事件和状态变化；
- 重放未追上旧 head 前，禁止 normal new chapter 和 final export。

### 先写的行为测试

- 修订第 47 章后，旧第 48 章及之后内容不再属于当前活动权威状态；
- Chapter 48 的旧正文可以作为候选输入，但不能直接继承旧事件/状态变化；
- 新分支必须按 48、49、50……顺序追上原 head；
- 重放期间旧 READY 和旧尝试版本全部失效；
- 每次新分支提交都有清晰 parent root 和 Receipt；
- 只改 `future_plan` 的作者指令不会启动历史重放；
- 首发拒绝同时存在两条活动修订链。

### 验证

```bash
go test ./internal/core -run 'Revision|Rebase|Superseded|Lineage|ExportBlocked' -v
go test ./...
```

### 完成条件

- 历史一致性依赖顺序重放，不依赖“聪明地改几个状态字段”；
- 旧版本仍可追溯，但不会混进当前活动权威状态；
- 修订链追平后才能恢复正常连载。

---

## 任务 8：单写者、备份、恢复、迁移、安全和导出

**目标**：补齐本地长期运行所需的可靠性边界。

### 主要改动

- 项目级跨进程写锁；
- `serve` 低频扫描，watcher 只做唤醒优化；
- 可验证备份与恢复；
- schema/protocol 版本迁移；
- `verify` 重算当前 root 并核对 Receipt chain；
- 路径/大小/JSON 深度等安全上限；
- `export` 只从当前活动权威状态生成整书；
- 可选 Notion 只读投影。

### 先写的行为测试

- 两个 Core 实例不能同时修改同一项目；
- `serve` 丢文件事件也能靠下一次扫描恢复；
- 备份被改一个字节后 restore 必须失败；
- restore 不覆盖正在运行的现有项目目录；
- 已知旧 schema 迁移前先备份，重复迁移幂等；
- 未知更新主版本拒绝打开；
- 协议版本升级本身不改变小说 `canon_root`；
- 路径穿越、符号链接、超大 manifest/chapter、未知产物被拒绝；
- 存在未结算修订/重放或结局硬约束未满足时最终导出失败。

### 验证

```bash
go test ./internal/core ./internal/protocol ./cmd/novel-core -run 'Lock|Serve|Backup|Restore|Migrate|Verify|Security|Export' -v
go vet ./...
go test ./...
```

### 完成条件

- 正确性不依赖 mtime、Drive 到达顺序或进程内存；
- 备份、恢复和迁移都有可验证结果；
- `verify` 能独立证明当前 root 与回执链一致。


---

## 任务 9：退出旧 AI 运行时

**目标**：在新闭环已经覆盖所需行为后，再把旧 Agent/Provider 运行时从受支持生产路径和发布物中移走。

### 主要改动

- 盘点 `internal/agents`、`internal/llmcodex`、`internal/bootstrap`、`cmd/novel-studio` 中仍被引用的确定性逻辑；
- 先把仍有价值的确定性逻辑迁入 domain/store/core/rules；
- 删除或停止构建所有必须创建 `bootstrap.ModelSet`、Coordinator、SubAgent、Writer/Drafter/Reviewer 的生产入口；
- 清理 `go.mod` / `go.sum` 中已经没有消费者的 `agentcore`、LiteLLM 等依赖；
- 更新 `.goreleaser.yml`、Dockerfile、compose 和配置样例，让受支持入口只指向 `novel-core`；
- Usage、模型 Session、AIVoice 等旧数据只保留必要兼容读取，不再成为 本地存储初始化或校验前提。

### 先写的验收测试

- `go list -deps ./cmd/novel-core` 不包含任何 AI runtime；
- 清空所有模型环境变量后，init/status/verify/serve/export 仍能运行；
- 旧模型数据缺失时，新项目和新 Core 不受影响；
- 删除旧运行时后，全量测试仍通过。

### 验证

```bash
go mod tidy
go test ./...
go vet ./...
go list -deps ./cmd/novel-core | grep -E 'agentcore|litellm|internal/agents|internal/llmcodex' && exit 1 || true
```

### 完成条件

- `cmd/novel-core` 是这个 fork 唯一受支持的生产二进制；
- 发布物不再要求模型提供方配置、模型配置或 API Key；
- 没有通过“配置里关掉”来冒充运行时已移除。

---

## 任务 10：文档、许可证与真实产品验收

**目标**：让仓库说明与真实运行方式一致，并完成自动门槛和普通 ChatGPT 实测。

### 主要改动

- 更新 `README.md`、`README-TECHNICAL.md`；若 `README_EN.md` 仍是正式入口，同步更新；
- 明确首发组合只有普通 ChatGPT App + Google Drive + Drive Desktop + `novel-core`；
- 保留 Apache-2.0 `LICENSE` 和必要的上游归属，不暗示得到上游背书；
- 新增简洁发布清单，只记录最终自动验收和人工产品验收结果，不保存评审过程。

### 自动门槛

```bash
go test ./...
go vet ./...
go list -deps ./cmd/novel-core
go run ./cmd/novel-core verify --project <fixture-local>
```

必须确认：

- 依赖图无 AI runtime；
- 无模型环境可运行；
- 完整 Core E2E、恢复、安全、修订、迁移测试通过；
- `LICENSE` 仍在，README 有上游来源说明。

### 普通 ChatGPT 产品验收

按真实产品链逐项记录 `PASS / FAIL / NOT RUN`：

1. capability check；
2. 首次建书 `ACCEPTED`；
3. Chapter 1 ACCEPTED；
4. 连续写到 Chapter 2+；
5. 故意制造一次可确定错误，得到 REWRITE；
6. 同 task 新 attempt 修复并 ACCEPTED；
7. 重启 Core 后继续；
8. 新 ChatGPT 对话通过项目名/ID 恢复；
9. 滚动 Arc 规划生效；
10. `future_plan` 作者指令生效；
11. `historical_revision` 触发下游顺序重放并追平原 head；
12. 制造一次 BLOCKED，通过 block_resolution 恢复；
13. 满足结局硬约束；
14. `verify` 通过；
15. final export 成功。

自动测试不能冒充这里的真实 ChatGPT 验收。

### 完成条件

- 自动门槛全部有新鲜证据；
- 人工产品链全部 PASS；
- README 没有把 API Provider、Ollama、MCP 或 Work 写成必需步骤；
- 没有遗留一份和代码协议互相漂移的手写 `CHATGPT_PROTOCOL.md`。

---

## 最终发布门槛

只有以下条件同时满足，才把迁移标为完成：

- 无 AI Core 的完整生产链可运行；
- Drive 提交先形成稳定本地快照再校验；
- 单写者、幂等、过期前态防护和崩溃恢复通过；
- 长篇硬约束与固定预算上下文通过；
- 历史修订采用顺序重放并能追平；
- 旧 AI runtime 已退出受支持构建与发布路径；
- 全量测试和 vet 通过；
- 普通 ChatGPT + Drive 的真实产品验收通过；
- Apache-2.0 许可证和上游归属保留。
