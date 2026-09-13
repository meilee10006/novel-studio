# Provider-free Novel Core Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把当前 novel-studio 改造成一个不调用任何 AI、可由普通 ChatGPT App 通过 Google Drive 文件交换驱动的本地长篇小说状态引擎，并完成“Foundation → 连续写章 → 返工 → 恢复 → 改稿 → 完本导出”的完整闭环。

**Architecture:** 保留现有 `internal/domain`、`internal/store` 和确定性规则，把新的生产入口收敛到 `cmd/novel-core -> internal/core`。ChatGPT 只写 Drive `exchange/inbox/**`，Core 只读 Submission 并通过协议校验、Canon 校验和生产合同校验决定 ACCEPTED / REWRITE / BLOCKED；只有 ACCEPTED 能推进 Canon。旧 Agent/Provider 运行时在新闭环稳定后再退出生产路径。

**Tech Stack:** Go 1.25.5；现有文件型 Store；SHA-256；标准库 `encoding/json` / `crypto/sha256` / `fs` / `filepath`；Google Drive Desktop 仅作为目录同步层；首发不新增数据库、向量数据库或模型依赖。

**Spec:** `docs/superpowers/specs/2026-09-13-provider-free-novel-core-design.md`

## Global Constraints

- 上游基线固定为 `e3beebbf2f35b9ff55fd781d82055b60d45970c8`。
- 开发分支为 `provider-free-novel-core`。
- `cmd/novel-core` 与 `internal/core` 不得依赖 `internal/agents`、`bootstrap.ModelSet`、`agentcore`、`internal/llmcodex` 或任何模型 Provider。
- 没有任何模型配置和 API Key 时，`novel-core init/status/verify/serve/export` 必须能运行。
- Google Drive 是交换层，不是 Canon 数据库；本地 Store 是唯一事实源。
- ChatGPT 只能写 `exchange/inbox/**` 和 setup/control 专用 inbox；不得修改 READY、STATUS、outbox、result、published、backup。
- Core 只做可确定验证；文学质量、人物感染力、节奏好坏等语义判断留给 ChatGPT/作者。
- 所有生产任务都有稳定 `task_id`；返工保持 `task_id`，生成新的 `attempt_id`。
- `manifest.json` 必须最后出现；Core 自己计算实际文件摘要，不信任 ChatGPT 自报摘要。
- 未知协议主版本必须拒绝；协议升级必须有显式迁移策略。
- Notion 不属于 Release Gate。
- 每项代码改动先写失败测试，再写最小实现，再跑测试，再提交。
- 正式编码前必须先完成 Task 0 基线门禁；基线失败时停止实现并报告，不把上游失败算到本分支。
- 新永久 ID 只能由 Core 在 ACCEPTED 提交时分配；ChatGPT 只在 Attempt 内使用临时 ID。
- Reader Promise 标记 fulfilled 时必须引用已接受的 Story Event 证据。

---

## File Map

### 新增核心入口

- `cmd/novel-core/main.go`：极薄 CLI 入口，只负责参数解析和调用 `internal/core`。
- `cmd/novel-core/main_test.go`：CLI 级冒烟测试和无模型配置测试。

### 新增 Core 深模块

- `internal/core/project.go`：`Project` 聚合根；打开项目、状态读取、公开操作入口。
- `internal/core/init.go`：项目初始化、目录创建、项目元数据和能力检查任务。
- `internal/core/task.go`：Task/Attempt 生成、READY 推进、Task Pack 输出。
- `internal/core/reconcile.go`：扫描 inbox、manifest-last、协议状态、幂等与 stale 检查。
- `internal/core/foundation.go`：Foundation 提交和首次 Canon 建立。
- `internal/core/chapter.go`：章节提交应用、Accepted Chapter、Receipt、下一章准备。
- `internal/core/validate.go`：L1/L2/L3 确定性验证编排。
- `internal/core/context.go`：有限 Context Pack 编译。
- `internal/core/control.go`：Author Directive、BLOCKED 裁决、Revision/Rebase 控制消息。
- `internal/core/recovery.go`：pending commit、重启恢复、备份恢复。
- `internal/core/export.go`：正式书稿、published、backup、projection 输出。
- `internal/core/serve.go`：低频目录对账循环；正确性不依赖文件事件顺序。
- `internal/core/ids.go`：把 Attempt 内临时 ID 映射为 Core 分配的永久 Canon ID。
- `internal/core/migrate.go`：本地 schema 与协议版本的显式迁移。
- `internal/core/chatgpt_protocol.go`：生成项目根唯一的 `CHATGPT_PROTOCOL.md`。

### 新增协议与领域类型

- `internal/protocol/types.go`：ProjectManifest、ReadyPointer、TaskEnvelope、SubmissionManifest、ResultEnvelope。
- `internal/protocol/version.go`：本地 schema version、协议 major/minor 与兼容性判断。
- `internal/protocol/io.go`：安全 JSON/Markdown 读取、原子写入、路径约束、大小上限。
- `internal/protocol/digest.go`：SHA-256、Submission 实际摘要、稳定 Canon root 链。
- `internal/domain/production_task.go`：TaskKind、ProductionResult、ReconciliationStatus、Violation。
- `internal/domain/story_event.go`：StoryEvent、EvidenceRef、KnowledgeChange。
- `internal/domain/author_directive.go`：AuthorDirective、BlockResolution、RevisionRequest。
- `internal/domain/ending_contract.go`：EndingContract 的显式硬规则字段。

### 扩展 Store

- `internal/store/production.go`：任务、Attempt、READY、Receipt、CanonHead、控制消息的本地持久化。
- `internal/store/production_test.go`：持久化与幂等测试。
- `internal/store/store.go`：把 `Production` 接入 Store 组合根，并创建 `meta/production/**`。

### 复用/迁移现有代码

- `internal/tools/commit_chapter.go`：只在迁移阶段用于对照现有确定性提交行为；新 Core 不调用 Agent Tool schema。
- `internal/store/chapter_render_transaction.go`：复用其中已经验证过的事务/恢复思路，不复用模型绑定。
- `internal/store/checkpoints.go` 与 `internal/store/checkpoints_test.go`：复用已有 checkpoint 能力。
- `internal/store/store.go`：继续作为状态组合根。

### 端到端测试

- `internal/core/core_e2e_test.go`：Foundation → Chapter → Rewrite → Next Chapter。
- `internal/core/recovery_e2e_test.go`：崩溃边界、重复提交、stale、恢复。
- `internal/core/longform_e2e_test.go`：知识、时间、资源、伏笔、Ending Contract。
- `internal/core/authoring_e2e_test.go`：Author Directive、Revision/Rebase、BLOCKED 恢复。
- `internal/core/security_e2e_test.go`：路径穿越、符号链接、超大输入、manifest 篡改。

---

### Task 0: 实施前基线门禁

**Files:**
- No repository changes.

**Purpose:** 在任何生产代码修改前建立可信基线，避免把上游既有失败误判成迁移回归。

- [ ] **Step 1: 建立隔离工作区并核对分支**

Run:
```bash
git branch --show-current
git merge-base --is-ancestor e3beebbf2f35b9ff55fd781d82055b60d45970c8 HEAD
git diff --name-only e3beebbf2f35b9ff55fd781d82055b60d45970c8...HEAD
```

Expected: 当前分支为 `provider-free-novel-core`；审计基线是 HEAD 祖先；开始编码前相对基线只允许已有 spec/plan 文档改动。

- [ ] **Step 2: 拉取依赖**

Run: `go mod download`

Expected: exit 0。

- [ ] **Step 3: 跑完整基线测试**

Run: `go test ./...`

Expected: exit 0。若失败，立即停止实现，保存失败测试名和输出并报告；未经作者明确决定，不进入 Task 1。

- [ ] **Step 4: 跑基线 vet**

Run: `go vet ./...`

Expected: exit 0。失败处理与 Step 3 相同。

---

### Task 1: 建立无 AI 的 Core Spine 和依赖防线

**Files:**
- Create: `cmd/novel-core/main.go`
- Create: `cmd/novel-core/main_test.go`
- Create: `internal/core/project.go`
- Create: `internal/core/project_test.go`
- Create: `internal/core/dependency_test.go`

**Interfaces:**
- Produces: `core.Open(localRoot, workspaceRoot string) (*core.Project, error)`
- Produces: `(*core.Project).Status() (core.Status, error)`
- Produces: `(*core.Project).Verify() error`
- Consumes: existing `store.NewStore(dir)` and `Store.CheckConsistency()`

- [ ] **Step 1: 写依赖防线失败测试**

```go
func TestNovelCoreDependencyGraphHasNoAIRuntime(t *testing.T) {
    cmd := exec.Command("go", "list", "-deps", "./cmd/novel-core")
    out, err := cmd.Output()
    if err != nil {
        t.Fatal(err)
    }
    forbidden := []string{
        "github.com/voocel/agentcore",
        "internal/agents",
        "internal/llmcodex",
    }
    for _, needle := range forbidden {
        if bytes.Contains(out, []byte(needle)) {
            t.Fatalf("novel-core depends on forbidden AI runtime %q", needle)
        }
    }
}
```

- [ ] **Step 2: 运行测试确认先失败**

Run: `go test ./internal/core -run TestNovelCoreDependencyGraphHasNoAIRuntime -v`

Expected: FAIL，因为 `cmd/novel-core` 尚不存在。

- [ ] **Step 3: 写最小 Core 和 CLI**

```go
package core

type Project struct {
    localRoot     string
    workspaceRoot string
    store         *store.Store
}

func Open(localRoot, workspaceRoot string) (*Project, error) {
    if strings.TrimSpace(localRoot) == "" || strings.TrimSpace(workspaceRoot) == "" {
        return nil, errors.New("local root and workspace root are required")
    }
    return &Project{
        localRoot: localRoot,
        workspaceRoot: workspaceRoot,
        store: store.NewStore(localRoot),
    }, nil
}
```

CLI 只支持 `status`、`verify` 两个入口，不读取 bootstrap 模型配置。

- [ ] **Step 4: 添加 CLI 无模型环境测试**

```go
func TestStatusDoesNotRequireModelConfiguration(t *testing.T) {
    t.Setenv("OPENAI_API_KEY", "")
    t.Setenv("ANTHROPIC_API_KEY", "")
    t.Setenv("DEEPSEEK_API_KEY", "")
    code := run([]string{"status", "--local", t.TempDir(), "--workspace", t.TempDir()})
    if code != 0 {
        t.Fatalf("status exit=%d", code)
    }
}
```

- [ ] **Step 5: 运行 Core 和 CLI 测试**

Run: `go test ./internal/core ./cmd/novel-core -v`

Expected: PASS。

- [ ] **Step 6: 跑完整回归**

Run: `go test ./...`

Expected: PASS；若出现新失败，先修复再提交。

- [ ] **Step 7: Commit**

```bash
git add cmd/novel-core internal/core
git commit -m "feat: add provider-free novel core spine"
```

---

### Task 2: 项目初始化、协议类型和 Drive 能力检查

**Files:**
- Create: `internal/protocol/types.go`
- Create: `internal/protocol/version.go`
- Create: `internal/protocol/io.go`
- Create: `internal/protocol/digest.go`
- Create: `internal/protocol/protocol_test.go`
- Create: `internal/store/production.go`
- Create: `internal/store/production_test.go`
- Modify: `internal/store/store.go`
- Create: `internal/core/init.go`
- Create: `internal/core/init_test.go`
- Create: `internal/core/chatgpt_protocol.go`
- Create: `internal/core/chatgpt_protocol_test.go`
- Modify: `cmd/novel-core/main.go`

**Interfaces:**
- Produces: `core.InitProject(ctx context.Context, opts InitOptions) (*Project, error)`
- Produces: `(*Project).CheckCapabilityAck() (bool, error)`
- Produces: `protocol.ProjectManifest`, `protocol.ReadyPointer`
- Produces Store operations: `LoadCanonHead`, `SaveCanonHead`, `LoadActiveAttempt`, `SaveActiveAttempt`, `AppendReceipt`

- [ ] **Step 1: 写初始化失败测试**

```go
func TestInitSeparatesLocalCanonFromWorkspace(t *testing.T) {
    local := filepath.Join(t.TempDir(), "local")
    workspace := filepath.Join(t.TempDir(), "drive")
    p, err := InitProject(context.Background(), InitOptions{
        ProjectName: "测试小说",
        LocalRoot: local,
        WorkspaceRoot: workspace,
    })
    if err != nil { t.Fatal(err) }
    if _, err := os.Stat(filepath.Join(local, "meta", "production")); err != nil { t.Fatal(err) }
    if _, err := os.Stat(filepath.Join(workspace, "project.json")); err != nil { t.Fatal(err) }
    if p.localRoot == p.workspaceRoot { t.Fatal("local canon and workspace must differ") }
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/core -run TestInitSeparatesLocalCanonFromWorkspace -v`

Expected: FAIL，`InitProject` 尚不存在。

- [ ] **Step 3: 定义稳定协议类型**

```go
type ProjectManifest struct {
    SchemaVersion   int    `json:"schema_version"`
    ProjectID       string `json:"project_id"`
    ProjectName     string `json:"project_name"`
    ProtocolVersion int    `json:"protocol_version"`
}

type ReadyPointer struct {
    ProjectID     string `json:"project_id"`
    TaskID        string `json:"task_id"`
    AttemptID     string `json:"attempt_id"`
    TaskKind      string `json:"task_kind"`
    Chapter       int    `json:"chapter,omitempty"`
    BaseCanonRoot string `json:"base_canon_root"`
    TaskDigest    string `json:"task_digest"`
    Status        string `json:"status"`
}
```

协议文件写入必须采用临时文件 + rename；读取必须拒绝越出 workspace 根目录的路径。

- [ ] **Step 4: 接入 Store**

在 `Store` 增加：

```go
Production *ProductionStore
```

`Init()` 增加：

```text
meta/production/
meta/production/receipts/
meta/production/tasks/
meta/production/control/
```

- [ ] **Step 5: 实现 capability handshake**

`init` 只生成：

```text
setup/capability-check.json
setup/inbox/
CHATGPT_PROTOCOL.md
project.json
```

`CHATGPT_PROTOCOL.md` 必须由 `internal/core/chatgpt_protocol.go` 的单一模板生成，并写入当前 `protocol_version`。在收到带正确 nonce 的 `setup/inbox/capability-ack.json` 前，不生成正式 READY。

- [ ] **Step 6: 加安全测试**

```go
func TestSafeJoinRejectsTraversal(t *testing.T) {
    root := t.TempDir()
    if _, err := protocol.SafeJoin(root, "../escape.json"); err == nil {
        t.Fatal("expected traversal rejection")
    }
}
```

同时覆盖符号链接跳出根目录和单文件大小上限。

- [ ] **Step 7: 运行测试**

Run: `go test ./internal/protocol ./internal/store ./internal/core ./cmd/novel-core -v`

Expected: PASS。

- [ ] **Step 8: Commit**

```bash
git add internal/protocol internal/store internal/core cmd/novel-core
git commit -m "feat: initialize novel core projects and drive handshake"
```

---

### Task 3: Foundation Task、首次 Canon 和 Receipt 链

**Files:**
- Create: `internal/domain/production_task.go`
- Create: `internal/domain/ending_contract.go`
- Create: `internal/core/task.go`
- Create: `internal/core/foundation.go`
- Create: `internal/core/foundation_test.go`
- Create: `internal/core/ids.go`
- Create: `internal/core/ids_test.go`
- Modify: `internal/protocol/types.go`
- Modify: `internal/store/production.go`

**Interfaces:**
- Produces: `(*Project).PrepareCurrentTask() (*protocol.ReadyPointer, error)`
- Produces: `(*Project).AcceptFoundation(sub FoundationSubmission) (protocol.ResultEnvelope, error)`
- Produces domain enums: `TaskKindFoundation`, `TaskKindChapter`, `TaskKindRevision`, `TaskKindFinalize`

- [ ] **Step 1: 写 Foundation 闭环失败测试**

```go
func TestCapabilityAckCreatesFoundationTask(t *testing.T) {
    p := newTestProjectWithCapabilityAck(t)
    ready, err := p.PrepareCurrentTask()
    if err != nil { t.Fatal(err) }
    if ready.TaskKind != "foundation" { t.Fatalf("kind=%s", ready.TaskKind) }
    if ready.BaseCanonRoot != "" { t.Fatalf("foundation must start before first canon") }
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/core -run TestCapabilityAckCreatesFoundationTask -v`

Expected: FAIL。

- [ ] **Step 3: 定义 Foundation Submission**

Foundation 必须至少包含：

```go
type FoundationSubmission struct {
    CreativeContract json.RawMessage
    StoryBible       json.RawMessage
    CharacterBible   json.RawMessage
    BookDirection    json.RawMessage
    VolumePlan       json.RawMessage
    CurrentArcPlan   json.RawMessage
    EndingContract   domain.EndingContract
}
```

不得直接写现有 Store 内部文件；必须经过 `AcceptFoundation`。

- [ ] **Step 4: 定义 CanonHead 与 root 链**

```go
type CanonHead struct {
    Revision uint64 `json:"revision"`
    Root     string `json:"root"`
    Chapter  int    `json:"chapter"`
}
```

新 root 按稳定顺序计算：

```text
SHA256(
  previous_root || 0x00 ||
  task_digest || 0x00 ||
  submission_digest || 0x00 ||
  validation_digest || 0x00 ||
  accepted_artifact_digest
)
```

Foundation 的 `previous_root` 使用空字符串。

- [ ] **Step 5: Foundation ACCEPT 时分配永久实体 ID**

Foundation Submission 中人物、势力、地点、初始伏笔等新实体使用 Attempt-local ID。Core 在首次 ACCEPT 时分配永久 Canon ID，把映射写入 Foundation Receipt，并在进入 Store 前重写交叉引用。测试必须覆盖不同类别同名/同 local ID 不碰撞，以及拒绝的 Foundation Attempt 不占用永久 ID。

- [ ] **Step 6: Foundation ACCEPT 后生成 Chapter 1 Task**

测试必须断言：

```go
if result.Result != "ACCEPTED" { ... }
if result.NewCanonRoot == "" { ... }
ready := mustLoadReady(t, workspace)
if ready.TaskKind != "chapter" || ready.Chapter != 1 { ... }
```

- [ ] **Step 7: 运行测试**

Run: `go test ./internal/domain ./internal/store ./internal/core -v`

Expected: PASS。

- [ ] **Step 8: Commit**

```bash
git add internal/domain internal/core internal/protocol internal/store
git commit -m "feat: establish foundation and initial canon"
```

---

### Task 4: manifest-last 对账与一章 Submission 协议

**Files:**
- Create: `internal/domain/story_event.go`
- Create: `internal/core/reconcile.go`
- Create: `internal/core/reconcile_test.go`
- Modify: `internal/protocol/types.go`
- Modify: `internal/protocol/version.go`
- Modify: `internal/protocol/io.go`
- Modify: `internal/protocol/digest.go`

**Interfaces:**
- Produces: `(*Project).ReconcileWorkspace(ctx context.Context) (ReconcileReport, error)`
- Produces: `protocol.SubmissionManifest`
- Produces: `domain.ReconciliationStatus` = PENDING / READY_TO_VALIDATE / INVALID / SETTLED

- [ ] **Step 1: 写 manifest-last 测试**

```go
func TestSubmissionWithoutManifestStaysPending(t *testing.T) {
    p := newProjectAtChapter(t, 1)
    writeInboxFile(t, p, "chapter.md", "正文")
    report, err := p.ReconcileWorkspace(context.Background())
    if err != nil { t.Fatal(err) }
    if report.Status != domain.ReconciliationPending {
        t.Fatalf("status=%s", report.Status)
    }
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/core -run TestSubmissionWithoutManifestStaysPending -v`

Expected: FAIL。

- [ ] **Step 3: 定义章节 Submission 文件集**

```text
chapter.md
chapter_contract.json
events.json
state_delta.json
self_review.json
manifest.json
```

`manifest.json` 只声明文件名、任务身份、Attempt 身份、base Canon root、task digest 和 completion nonce；artifact digest 由 Core 读取实际字节后计算并写入 Result/Receipt。

允许的 artifact 集合必须由当前 Task Pack 明确给出。章节首发集合只允许上述五个必需文件和可选 `next_arc_proposal.json`；出现未声明 artifact、绝对路径、重复规范化文件名或目录型 artifact 一律 `INVALID`。Story Event 在 Submission 内只使用 Attempt-local ID。

- [ ] **Step 4: 实现 immutable-after-manifest 检查**

第一次完整读取后保存实际 `submission_digest`。同一 Attempt 再次对账：

- digest 相同：幂等；
- digest 不同：`INVALID`；
- READY 已切换：inactive Attempt，不进入 L2/L3。

- [ ] **Step 5: 加协议故障测试**

覆盖：

```text
manifest 先到、文件后到 -> PENDING
completion nonce 错 -> INVALID
unknown newer major version -> INVALID
未声明 artifact -> INVALID
same attempt same bytes -> idempotent
same attempt changed bytes -> INVALID
inactive attempt -> INVALID
```

- [ ] **Step 6: 运行测试**

Run: `go test ./internal/protocol ./internal/core -run 'TestSubmission|TestManifest|TestInactive' -v`

Expected: PASS。

- [ ] **Step 7: Commit**

```bash
git add internal/domain internal/protocol internal/core
git commit -m "feat: reconcile immutable chapter submissions"
```

---

### Task 5: 第一条真实章节 Commit 闭环

**Files:**
- Create: `internal/core/chapter.go`
- Create: `internal/core/chapter_test.go`
- Create: `internal/core/core_e2e_test.go`
- Create: `internal/core/validate.go`
- Modify: `internal/core/ids.go`
- Modify: `internal/core/ids_test.go`
- Modify: `internal/store/production.go`
- Reuse: `internal/store/store.go`
- Reuse behavior from: `internal/tools/commit_chapter.go`

**Interfaces:**
- Produces: `(*Project).ValidateCurrentSubmission(...) ValidationReport`
- Produces: internal `commitAcceptedChapter(...)`
- Produces: ResultEnvelope ACCEPTED / REWRITE / BLOCKED

- [ ] **Step 1: 写端到端失败测试**

```go
func TestOneChapterLoopAdvancesCanonAndReady(t *testing.T) {
    p := newProjectWithAcceptedFoundation(t)
    ready1 := mustReady(t, p)
    if ready1.Chapter != 1 { t.Fatal("expected chapter 1") }

    writeValidChapterSubmission(t, p, ready1, validChapterFixture(1))
    report, err := p.ReconcileWorkspace(context.Background())
    if err != nil { t.Fatal(err) }
    if report.Result != domain.ResultAccepted { t.Fatalf("result=%s", report.Result) }

    head := mustCanonHead(t, p)
    if head.Chapter != 1 || head.Root == ready1.BaseCanonRoot { t.Fatal("canon did not advance") }
    ready2 := mustReady(t, p)
    if ready2.Chapter != 2 { t.Fatalf("next chapter=%d", ready2.Chapter) }
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/core -run TestOneChapterLoopAdvancesCanonAndReady -v`

Expected: FAIL。

- [ ] **Step 3: 实现 L1/L2/L3 编排骨架**

```go
type ValidationReport struct {
    ProtocolViolations   []domain.Violation
    CanonViolations      []domain.Violation
    ContractViolations   []domain.Violation
}

func (r ValidationReport) ProductionResult() domain.ProductionResult {
    if len(r.ProtocolViolations) > 0 { return "" }
    if hasBlocking(r.CanonViolations, r.ContractViolations) { return domain.ResultBlocked }
    if len(r.CanonViolations)+len(r.ContractViolations) > 0 { return domain.ResultRewrite }
    return domain.ResultAccepted
}
```

- [ ] **Step 4: 先迁移最小确定性提交集合**

从现有 `commit_chapter.go` 复用 Store 写入顺序，但首个新闭环只处理确定性且必需的数据：

```text
SaveFinalChapter
SaveSummary
AppendTimelineEvents
UpdateForeshadow
UpdateRelationships
AppendStateChanges
MergeClaims
MarkChapterComplete
Checkpoint
Receipt
CanonHead
READY next attempt
```

明确不迁移：AIVoice、AIGCReport、provider usage、模型审批、模型身份绑定、向量 embedding。

- [ ] **Step 5: 由 Core 分配永久 Canon ID**

ACCEPTED 前，`events.json`、新增伏笔、冲突、Promise 等只允许使用 Attempt-local ID。Commit 时由 Core 分配稳定永久 ID，并把 `local_id -> canon_id` 映射写入 Receipt；后续 State Delta 应用到 Canon 时使用永久 ID。

先写测试：两个不同 Attempt 都使用 `event-1`，最终必须得到不同 Canon Event ID；REWRITE 未接受的 Attempt 不得消耗或占用永久 ID。

- [ ] **Step 6: 保证 Commit 前后可恢复**

提交前写本地 `PendingCommit`，阶段至少包括：

```text
prepared
artifacts_saved
state_applied
progress_marked
checkpointed
settled
```

每一步必须可幂等重放。

- [ ] **Step 7: 加 REWRITE 测试**

```go
func TestRewriteKeepsTaskAndCreatesNewAttempt(t *testing.T) {
    p := newProjectWithAcceptedFoundation(t)
    r1 := mustReady(t, p)
    writeSubmissionWithWrongChapter(t, p, r1)
    _ = mustReconcile(t, p)
    r2 := mustReady(t, p)
    if r2.TaskID != r1.TaskID { t.Fatal("rewrite changed logical task") }
    if r2.AttemptID == r1.AttemptID { t.Fatal("rewrite must create new attempt") }
    if r2.Chapter != r1.Chapter { t.Fatal("rewrite skipped chapter") }
}
```

- [ ] **Step 8: 运行端到端测试**

Run: `go test ./internal/core -run 'TestOneChapterLoop|TestRewrite' -v`

Expected: PASS。

- [ ] **Step 9: Commit**

```bash
git add internal/core internal/store
git commit -m "feat: commit accepted chapters and advance canon"
```

---

### Task 6: 长篇 Canon 约束和有限 Context Compiler

**Files:**
- Create: `internal/core/context.go`
- Create: `internal/core/context_test.go`
- Modify: `internal/core/validate.go`
- Create: `internal/core/longform_e2e_test.go`
- Modify/Create domain files: `internal/domain/story_event.go`, `internal/domain/ending_contract.go`
- Reuse: existing World/Character/Resource/Planning stores

**Interfaces:**
- Produces: internal `compileContext(attempt ActiveAttempt) (protocol.ContextPack, error)`
- Produces validators for knowledge provenance, modeled time/location constraints, resource ledger, relationship transitions, foreshadow/conflict/promise lifecycles, Ending Contract explicit rules

- [ ] **Step 1: 写人物知识来源失败测试**

```go
func TestKnowledgeChangeRequiresObservableSourceEvent(t *testing.T) {
    p := newProjectAtChapter(t, 7)
    sub := validChapterFixture(7)
    sub.Events = []domain.StoryEvent{{ID: "e1", Actors: []string{"林舟"}, Observers: []string{"林舟"}}}
    sub.Delta.KnowledgeAdd = []domain.KnowledgeChange{{
        Character: "顾清瑶", FactID: "lab-entry", SourceEventID: "e1",
    }}
    result := submitAndReconcile(t, p, sub)
    if result.Result != domain.ResultRewrite { t.Fatalf("result=%s", result.Result) }
    assertViolation(t, result, "knowledge_boundary_violation")
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/core -run TestKnowledgeChangeRequiresObservableSourceEvent -v`

Expected: FAIL。

- [ ] **Step 3: 实现 Story Event 证据规则**

一个知识变化合法，当且仅当 source event 满足以下之一：

```text
character in observers
OR event.public == true
OR source event 是明确传播事件且 recipients 包含 character
```

Core 不做自然语言推理。

- [ ] **Step 4: 实现时间/地点/资源确定性验证**

只验证已经建模的数据：

```text
时间不能倒退
同一人物时间区间不能冲突地点重叠
TravelConstraint 明确存在时必须满足最小耗时
ResourceLedger 中可计量资源不能减到负数
```

没有 TravelConstraint 时不使用现实世界常识自行估算。

- [ ] **Step 5: 实现伏笔/冲突/Promise 状态机**

Foreshadow 允许边：

```text
planned -> seeded
seeded -> reinforced | payoff_ready | retired
reinforced -> reinforced | payoff_ready | retired
payoff_ready -> paid_off | retired
paid_off -> closed
```

任何未列出的跃迁返回 REWRITE。

Reader Promise 标记 `fulfilled` 时必须引用当前已接受 Story Event；事件不存在、证据锚点缺失或 Promise 已 retired 时返回 REWRITE。先补 `TestReaderPromiseFulfilledRequiresAcceptedEventEvidence`。

- [ ] **Step 6: 实现 Ending Contract 显式硬规则**

Finalize 只能在结构化字段全部满足时通过，例如 `required_foreshadow_ids` 全部 closed、`required_conflict_ids` 全部 closed、`required_character_arc_ids` 全部 terminal。不要加入“剩余篇幅看起来不够”之类语义判断。

- [ ] **Step 7: 实现 Context Compiler 预算测试**

```go
func TestContextPackSizeDoesNotGrowLinearlyWithBookLength(t *testing.T) {
    p100 := seededProjectWithChapters(t, 100)
    p200 := seededProjectWithChapters(t, 200)
    c100 := mustCompileContext(t, p100)
    c200 := mustCompileContext(t, p200)
    if len(c200.JSON) > len(c100.JSON)*2 {
        t.Fatalf("context grew linearly: %d -> %d", len(c100.JSON), len(c200.JSON))
    }
}
```

Context 预算使用字符/字节上限，不依赖某个模型 tokenizer。

- [ ] **Step 8: 运行长篇测试**

Run: `go test ./internal/core -run 'TestKnowledge|TestTime|TestResource|TestForeshadow|TestEnding|TestContext' -v`

Expected: PASS。

- [ ] **Step 9: Commit**

```bash
git add internal/core internal/domain
git commit -m "feat: enforce long-form canon invariants"
```

---

### Task 7: Author Directive、BLOCKED 裁决和 Revision/Rebase

**Files:**
- Create: `internal/domain/author_directive.go`
- Create: `internal/core/control.go`
- Create: `internal/core/authoring_e2e_test.go`
- Modify: `internal/core/task.go`
- Modify: `internal/store/production.go`

**Interfaces:**
- Produces: `(*Project).IngestControlMessage(path string) error`
- Produces control message types `AuthorDirective`, `BlockResolution`, `RevisionRequest`
- Produces revision Task but never mutates Canon directly from author prose

- [ ] **Step 1: 写 Author Directive 失败测试**

```go
func TestAuthorDirectiveBecomesRevisionTaskWithoutMutatingCanon(t *testing.T) {
    p := newProjectAtChapter(t, 47)
    before := mustCanonHead(t, p)
    writeAuthorDirective(t, p, domain.AuthorDirective{
        ID: "directive-1",
        TargetChapter: 47,
        Instruction: "顾清瑶不能在本章原谅林舟，保留黑色钥匙伏笔",
    })
    if err := p.IngestControlMessage(controlPath(t, p)); err != nil { t.Fatal(err) }
    after := mustCanonHead(t, p)
    if after.Root != before.Root { t.Fatal("directive mutated canon directly") }
    if mustReady(t, p).TaskKind != "revision" { t.Fatal("expected revision task") }
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/core -run TestAuthorDirectiveBecomesRevisionTaskWithoutMutatingCanon -v`

Expected: FAIL。

- [ ] **Step 3: 实现 Author Directive 控制通道**

Drive 使用独立控制目录：

```text
control/inbox/<message-id>.json
control/result/<message-id>.json
```

Core 按 message ID 幂等处理。作者文字只进入新的正式 Task，不直接修改 Store。

- [ ] **Step 4: 实现 BLOCKED 裁决**

`BlockResolution` 必须引用当前 `block_id`；过期 block ID 拒绝。合法裁决生成新 Attempt 或 Revision Task，而不是直接继续旧 Attempt。

- [ ] **Step 5: 实现 Revision/Rebase**

已提交章节发生 Revision 后：

```text
保留旧 accepted artifact
建立新 revision lineage
生成新 Canon root
标记目标章之后的派生计划 stale
重建必要摘要/Context/剧情债务投影
```

不允许直接覆盖旧 `published/chapter-047.md` 作为事实变更手段。

- [ ] **Step 6: 加历史重算测试**

测试 Chapter 47 Revision 后：Chapter 48+ 旧 Task 全部 inactive；新的 READY 从需要重写的最早章节开始；Canon lineage 可追到 revision receipt。

- [ ] **Step 7: 运行测试**

Run: `go test ./internal/core -run 'TestAuthorDirective|TestBlockResolution|TestRevision' -v`

Expected: PASS。

- [ ] **Step 8: Commit**

```bash
git add internal/domain internal/core internal/store
git commit -m "feat: add author directives and canon revision flow"
```

---

### Task 8: Serve、崩溃恢复、版本迁移、备份和完整导出

**Files:**
- Create: `internal/core/recovery.go`
- Create: `internal/core/recovery_e2e_test.go`
- Create: `internal/core/serve.go`
- Create: `internal/core/export.go`
- Create: `internal/core/export_test.go`
- Create: `internal/core/migrate.go`
- Create: `internal/core/migrate_test.go`
- Modify: `internal/protocol/version.go`
- Modify: `cmd/novel-core/main.go`

**Interfaces:**
- Produces: `(*Project).Serve(ctx context.Context, interval time.Duration) error`
- Produces: `(*Project).Recover() error`
- Produces: `(*Project).Export(dst string) error`
- Produces: `(*Project).CreateBackup(dst string) error`
- Produces: `RestoreBackup(src, localRoot string) error`

- [ ] **Step 1: 写崩溃恢复测试**

对 commit 阶段注入失败：

```go
for _, stage := range []string{"artifacts_saved", "state_applied", "progress_marked", "checkpointed"} {
    t.Run(stage, func(t *testing.T) {
        p := newProjectAtChapter(t, 3)
        p.testFailCommitAt = stage
        submitValidChapter(t, p, 3)
        _ = p.ReconcileWorkspace(context.Background())
        reopened := reopenProject(t, p)
        if err := reopened.Recover(); err != nil { t.Fatal(err) }
        assertExactlyOneSettlement(t, reopened, 3)
    })
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/core -run TestRecover -v`

Expected: FAIL。

- [ ] **Step 3: 实现恢复规则**

恢复只读：PendingCommit、Receipt、CanonHead、Checkpoint、Accepted Artifacts。禁止用 mtime、Drive 到达顺序或内存状态决定真相。

- [ ] **Step 4: 实现 serve**

`serve` 每次循环只调用 `Recover()` + `ReconcileWorkspace()` + 输出刷新。文件 watcher 可以以后优化，但不得成为正确性前提。

- [ ] **Step 5: 实现 backup/restore**

Backup 包含：

```text
project manifest
canon head
receipt chain
accepted artifacts
canonical store snapshot
schema version
backup digest manifest
```

Restore 必须先完整校验，再写入全新 local root；禁止覆盖当前运行中的项目目录。

- [ ] **Step 6: 实现显式 schema / protocol 迁移**

项目本地 schema 版本和 Drive 协议版本分别管理。打开已知旧版本时先创建可验证 backup，再执行幂等迁移并写 migration receipt；未知的更新主版本必须拒绝打开，禁止“尽量解析”。先覆盖 `TestKnownSchemaMigratesAfterBackup`、`TestUnknownNewerMajorIsRejected`、`TestMigrationIsIdempotent`。

- [ ] **Step 7: 实现 export**

只从 Accepted Chapters 按 Canon 顺序导出；存在未结算 Revision、Receipt 链错误或 Ending Contract 未满足且请求 final export 时返回错误。

- [ ] **Step 8: 运行测试**

Run: `go test ./internal/core ./cmd/novel-core -run 'TestRecover|TestBackup|TestMigrate|TestUnknownNewerMajor|TestExport|TestServe' -v`

Expected: PASS。

- [ ] **Step 9: Commit**

```bash
git add internal/core internal/protocol cmd/novel-core
git commit -m "feat: recover migrate backup and export novel core projects"
```

---

### Task 9: 安全边界、完整 E2E 与 ChatGPT 协议生成

**Files:**
- Create: `internal/core/security_e2e_test.go`
- Modify: `internal/core/init.go`
- Modify: `internal/core/task.go`
- Modify: `internal/protocol/io.go`
- Modify: `internal/core/chatgpt_protocol.go`
- Modify: `internal/core/chatgpt_protocol_test.go`
- Modify: `cmd/novel-core/main.go`

**Interfaces:**
- No new public Core methods; this task hardens existing seams.

- [ ] **Step 1: 写 hostile workspace 测试**

必须拒绝：

```text
../ 路径穿越
绝对路径 artifact
symlink 指向 workspace 外
超大 manifest
超大 chapter.md
重复同名 artifact
未知 artifact 名称
manifest 后内容变化
```

- [ ] **Step 2: 加完整 Core E2E**

同一个测试中跑：

```text
init
→ capability ack
→ foundation ACCEPTED
→ chapter 1 ACCEPTED
→ chapter 2 REWRITE
→ chapter 2 new attempt ACCEPTED
→ restart/reopen
→ author directive revision
→ block + resolution
→ finalize constraints
→ verify
→ export
```

- [ ] **Step 3: 从唯一代码模板生成项目 `CHATGPT_PROTOCOL.md`**

`internal/core/chatgpt_protocol.go` 是协议正文的唯一生成源；`novel-core init` 把带 `protocol_version` 的固定模板写到每个项目根目录。仓库不再维护第二份完整协议文档，避免两份内容漂移。

模板必须明确写出：

```text
1. 只读取 READY 当前 Attempt。
2. 只写 inbox 当前 Attempt。
3. 一次“继续”完成本 Task 所有智能工作。
4. events.json 先描述本章结构化事件，state_delta 只能引用这些事件或既有 Canon fact。
5. 最后写 manifest.json。
6. 不修改 READY/STATUS/outbox/result/published/backup。
7. REWRITE 修同一 Task 的新 Attempt，不跳章。
8. BLOCKED 等待作者裁决，不自行修改硬设定。
```

- [ ] **Step 4: CLI 验收命令**

确保：

```bash
novel-core init
novel-core status
novel-core verify
novel-core serve
novel-core export
```

都不读取模型配置。

- [ ] **Step 5: 全量测试**

Run: `go test ./...`

Expected: PASS。旧 runtime 在 Contract 前仍必须保持可测试；若本任务导致旧测试失败，先修复再提交。

- [ ] **Step 6: Commit**

```bash
git add internal/core internal/protocol cmd/novel-core
git commit -m "test: harden novel core production protocol"
```

---

### Task 10: Contract 旧 AI Runtime 生产路径

**Files:**
- Delete or retire from release path: `internal/agents/**`
- Delete or retire from release path: `internal/llmcodex/**`
- Delete or split AI-only code from: `internal/bootstrap/models.go`, `internal/bootstrap/model_observer.go`, `internal/bootstrap/model_snapshot.go`, `internal/bootstrap/config.go`, `internal/bootstrap/configfile.go`
- Migrate any still-needed deterministic CLI behavior, then remove `cmd/novel-studio/**` from the supported/release build; audit at minimum `main.go`, `deepseek_ai_judge.go`, `draft_ai_judge.go`, `pipeline_cmd.go`, `pipeline_planning_stages.go` before deletion
- Modify: `go.mod`
- Modify: `go.sum`
- Modify: `internal/store/store.go`
- Modify: `config.example.jsonc` and any bootstrap config examples that still require provider/model setup
- Audit/modify release packaging at minimum `.goreleaser.yml`, `Dockerfile`, `docker-compose.yml`，使首发入口指向 `novel-core`
- Keep only domain/store/rules code still used by `cmd/novel-core`; any deterministic logic found in the files above must move before deletion

**Interfaces:**
- `cmd/novel-core` becomes the supported production binary for this fork.
- Existing Canon data that is model-independent remains readable.

- [ ] **Step 1: 写最终依赖验收脚本/测试**

```bash
go list -deps ./cmd/novel-core | grep -E 'agentcore|litellm|internal/agents|internal/llmcodex' && exit 1 || true
```

Expected: no output。

- [ ] **Step 2: 列出实际 AI runtime 依赖面**

Run:

```bash
rg -n 'agentcore|ModelSet|ForRole\(|ForDefaultPurpose\(|llmcodex|litellm|provider.*model|reasoning_effort' --glob '*.go' .
```

把命中分成两类：

```text
A. 仅旧 AI runtime 使用 -> 删除/退出 release path
B. 同时含可复用确定性逻辑 -> 先把确定性逻辑留在 domain/store/rules/core，再删除旧壳
```

这一步不是另做架构设计；分类必须服从现有 spec，不能重新引入 Provider 抽象。

- [ ] **Step 3: 移除旧模型启动路径**

删除所有必须创建 `bootstrap.ModelSet`、Coordinator、SubAgent、Writer/Drafter/Reviewer 模型实例的受支持生产入口。确定性 CLI 行为迁到 `cmd/novel-core` 后，旧 `cmd/novel-studio` 不再作为可发布二进制；`internal/agents/**` 和 `internal/llmcodex/**` 在确定性逻辑迁出后从首发构建图消失，而不是仅靠运行时配置“不开启”。

- [ ] **Step 4: 清理 Store 必需依赖**

`Usage`、模型 Session、`AIVoice` 等可以暂时作为旧数据兼容读取，但 `store.NewStore` 和 `novel-core` 的 Init/Verify/Commit 不得要求它们存在。

- [ ] **Step 5: `go mod tidy`**

Run: `go mod tidy`

如果仓库已经没有任何 `agentcore/litellm` 引用，则 `go.mod`/`go.sum` 必须移除相应依赖；如果为了明确保留的非生产兼容代码仍有引用，必须把该兼容代码从首发构建路径隔离，并在本任务内给出测试证明 `cmd/novel-core` 不加载它。

- [ ] **Step 6: 全量验证**

Run:

```bash
go test ./...
go vet ./...
go list -deps ./cmd/novel-core
```

Expected: 测试和 vet 退出码 0；最后一个命令输出中没有 `agentcore`、`litellm`、`internal/agents`、`internal/llmcodex`。

- [ ] **Step 7: 无模型环境真实 CLI 验证**

Run:

```bash
env -u OPENAI_API_KEY -u ANTHROPIC_API_KEY -u DEEPSEEK_API_KEY \
  go run ./cmd/novel-core init --name smoke --local /tmp/novel-core-smoke/local --workspace /tmp/novel-core-smoke/drive
```

Expected: 创建项目并进入 capability-check 状态，而不是要求模型设置。

- [ ] **Step 8: Commit**

```bash
git add -A
git commit -m "refactor: retire ai runtime from novel core production path"
```

---

### Task 11: Release Gate 与真实普通 ChatGPT 验收清单

**Files:**
- Create: `docs/novel-core-release-checklist.md`
- Modify: `README.md`
- Modify: `README-TECHNICAL.md`
- Modify if still user-facing: `README_EN.md`
- Verify unchanged/preserved: `LICENSE`

**Interfaces:**
- No new code interfaces.

- [ ] **Step 1: 写发布清单**

必须包含两个分区：自动验证和人工 ChatGPT 产品验证。

自动验证：

```bash
go test ./...
go vet ./...
go run ./cmd/novel-core verify --local <fixture-local> --workspace <fixture-workspace>
```

人工产品验证必须逐项记录 PASS/FAIL：

```text
能力检查
Foundation
Chapter 1 ACCEPTED
Chapter 2 人物知识越界 -> REWRITE
Chapter 2 新 Attempt -> ACCEPTED
Core 重启恢复
新 ChatGPT 对话恢复
滚动 Arc
Author Directive + Revision/Rebase
BLOCKED + block_resolution
Ending Contract
verify
完整导出
```

- [ ] **Step 2: 更新 README**

README 只描述首发真实运行方式：

```text
普通 ChatGPT App
Google Drive
Drive Desktop
novel-core
```

不得把 API Provider、Ollama、MCP 或 Work 写成新 Core 的必需步骤。

README 必须说明这是基于 `Xiaoyangy/novel-studio` 的 fork/改造，保留 Apache-2.0 `LICENSE`，不得暗示上游作者或项目为本 fork 背书；如果仓库中存在上游 NOTICE/第三方归属文件，必须继续保留并随分发带上。

- [ ] **Step 3: 验证许可证和归属**

Run: `test -f LICENSE && grep -q 'Apache License' LICENSE`

Expected: exit 0；README 中存在上游来源说明。

- [ ] **Step 4: 跑最终自动验证**

Run:

```bash
go test ./...
go vet ./...
```

Expected: exit 0。

- [ ] **Step 5: 不伪造人工验收**

如果当前环境无法真实完成普通 ChatGPT + Drive 的人工链路，发布清单必须明确保持对应项为 `NOT RUN`；不得把 Core fixture 测试当作真实 ChatGPT PASS。

- [ ] **Step 6: Commit**

```bash
git add docs/novel-core-release-checklist.md README.md README-TECHNICAL.md README_EN.md
git commit -m "docs: define novel core release gate"
```

---

## Plan Self-Review Checklist

执行者在开始实现前再核对一次：

- [ ] Foundation 是正式 Task/Submission/Receipt，不存在“先手工把设定写进 Store”的旁路。
- [ ] Task 和 Attempt 没有混用；REWRITE 不创建新的逻辑 Task。
- [ ] Drive 每个目录只有一个正式写入方。
- [ ] Core 从实际字节计算 digest，不要求 ChatGPT 充当哈希工具。
- [ ] Story Event 是知识、关系、伏笔、资源变化的结构化证据锚点。
- [ ] Core 不承担文学语义判断。
- [ ] 时间/旅行只验证已建模约束，不臆测现实常识。
- [ ] Context Compiler 有稳定大小预算，不随全书长度线性增长。
- [ ] Author Directive 和 BLOCKED 裁决都经正式控制消息进入 Task。
- [ ] Revision/Rebase 不覆盖旧 Accepted Artifact。
- [ ] Recovery 不依赖 mtime、聊天历史或 Drive 到达顺序。
- [ ] Notion 不在 Release Gate。
- [ ] 旧 AI runtime 只在最后 Contract 阶段退出，避免迁移期间仓库长期不可运行。
- [ ] 永久实体 ID 只在 ACCEPTED 时由 Core 分配，Attempt-local ID 不泄漏为 Canon ID。
- [ ] Reader Promise 的 fulfilled 状态有已接受 Story Event 证据。
- [ ] schema/protocol 迁移先备份、可幂等重跑，未知更新主版本拒绝。
- [ ] `CHATGPT_PROTOCOL.md` 只有一个生成源，不维护第二份完整协议。
- [ ] Apache-2.0 LICENSE 和上游归属得到保留。
- [ ] 最终真实 ChatGPT 产品链路没有被自动测试冒充。
