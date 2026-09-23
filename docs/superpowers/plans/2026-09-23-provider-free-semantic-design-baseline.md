# Provider-Free 语义设计基线 Implementation Plan

> 状态：已完成计划级 self-review，待按任务实施。

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不恢复任何模型 Provider、Agent runtime 或新 TaskKind 的前提下，为新项目加入建书前语义设计证据链，并把通过建书就绪检查的七个 Foundation 文件可靠地接入现有 Canon 结算。

**Architecture:** 新增一个仅存在于第一份 Canon 之前的内容寻址 Design Store，保存不可变设计产物、设计包、设计提交、设计头与提交回执。ChatGPT 继续负责语义创作和审稿；Core 只验证版本绑定、固定 provenance、两个检查点和 Foundation 确定性结构。现有 foundation/chapter/revision Canon 生产链保持唯一，Foundation 建立后 Design Head 冻结。

**Tech Stack:** Go 1.25.5，标准库 JSON/文件系统/SHA-256，现有 `internal/core`、`internal/domain`、`internal/store`、`internal/protocol`，Google Drive 文件交换协议。

**Spec:** `docs/superpowers/specs/2026-09-23-provider-free-semantic-design-baseline.md`

## Global Constraints

- 固定执行主机：MBP2015。
- 固定仓库：`/Users/agent/workspace/novel-studio`，分支 `provider-free-novel-core`。
- 以 repo/Git 与当前环境为准，优先本地验证。
- SentinelX 执行只使用 `background=true`；长任务脱离会话运行，避免重复启动。
- 稳定阶段成果验证后及时 commit/push；非集成 CI 不等待、不轮询。
- ChatGPT App 仍是唯一智能层；Novel Core 不调用任何模型。
- 不新增 story_development / brainstorm / architect / review 等 Core TaskKind。
- 不建设 Workflow Engine、通用 DAG resolver、Capability DSL、Scheduler、Queue 或 Worker。
- `exchange/STATUS.json` 仍是项目唯一状态投影。
- Canon 仍只通过 foundation / chapter / revision 正式结算。
- Design 操作不得消耗 `NextTaskSeq`、`NextAttemptSeq`、`NextEntitySeq`。
- 第一份 Foundation ACCEPTED 后 Design Head 冻结；本期不实现 Foundation revision。
- 当前发布基线是 core schema 1 / protocol 1.0；目标版本使用 core schema 2 / protocol 1.1。
- 现有 schema 0 与 protocol 0.9 的兼容入口不得被无意删除；迁移实现使用少量显式分支，不建设通用迁移图。
- `go.mod` 要求 Go 1.25.5。计划编写时 MBP2015 没有 `go`、Docker、Podman、Colima 或 Homebrew；开始 Task 1 前必须先恢复本机 Go 1.25.5 工具链，之后才能声称 Go 测试通过。

### Execution preflight

在改代码前执行：

```bash
cd /Users/agent/workspace/novel-studio
git status --short --branch
git fetch origin provider-free-novel-core
test "$(git rev-parse HEAD)" = "$(git rev-parse origin/provider-free-novel-core)"
uname -m
```

MBP2015 预期 `uname -m` 为 `x86_64`。若 `go` 仍不存在，用本地工作区安装官方 Go 1.25.5，不写入 repo：

```bash
mkdir -p /Users/agent/workspace/.tools
curl -fsSL https://go.dev/dl/go1.25.5.darwin-amd64.tar.gz -o /tmp/go1.25.5.darwin-amd64.tar.gz
rm -rf /Users/agent/workspace/.tools/go
tar -C /Users/agent/workspace/.tools -xzf /tmp/go1.25.5.darwin-amd64.tar.gz
export PATH="/Users/agent/workspace/.tools/go/bin:$PATH"
go version
```

Expected:

```text
go version go1.25.5 darwin/amd64
```

随后建立新鲜基线：

```bash
go test ./...
go vet ./...
go list -deps ./cmd/novel-core | grep -E 'internal/agents|internal/llmcodex|agentcore' && exit 1 || true
```

如果基线本身失败，先记录既有失败并停止功能实现；不要把红基线带进后续任务。

## File Map

新增文件：

- `internal/domain/core_design.go`：Design artifact / bundle / commit / head / design submission record 领域类型和固定常量。
- `internal/protocol/design.go`：Drive Design manifest 与 promote 请求机器协议。
- `internal/store/core_design.go`：Design Store 的不可变对象、bundle、commit、HEAD、receipt、snapshot/reconcile 持久化。
- `internal/core/inbox_snapshot.go`：Submission 与 Design Inbox 共用的安全读取、静默期观察和冲突判定。
- `internal/core/design.go`：设计对象规范化、ref 解析、固定 artifact 结构检查、bundle 读取。
- `internal/core/design_submission.go`：Design Inbox 扫描、import、promote、幂等回放和结果写入。
- `internal/core/design_checkpoint.go`：story_locked / foundation_ready 两个固定检查点。
- `internal/core/design_test.go`：内容寻址、checkpoint、provenance、CAS 的行为测试。
- `internal/core/design_submission_test.go`：Drive Design Inbox 的安全、静默期、幂等与 serve 行为测试。
- `internal/core/design_test_helpers_test.go`：语义设计测试共用的建项目、导入、推进和合法 payload fixture；只服务测试，不进入生产 API。

主要修改文件：

- `internal/domain/core_project.go`
- `internal/domain/core_production.go`
- `internal/protocol/protocol.go`
- `internal/protocol/chatgpt_protocol_part1.go`
- `internal/protocol/chatgpt_protocol_part3.go`
- `internal/store/core_submission.go`
- `internal/core/init.go`
- `internal/core/project.go`
- `internal/core/submission.go`
- `internal/core/serve.go`
- `internal/core/production.go`
- `internal/core/production_part2.go`
- `internal/core/production_part3.go`
- `internal/core/production_part5.go`
- `internal/core/migration.go`
- `internal/core/lock.go`
- `internal/core/backup.go`
- 相关现有 `*_test.go`
- `README.md`
- `docs/provider-free-release-checklist.md`

## Review Focus

1. **陈旧/重复 Design promote**：同一 `submission_id` 相同快照必须幂等返回原结果；内容变化必须冲突；不同 submission 使用旧 `expected_design_root` 必须得到 `STALE_DESIGN_HEAD`。Task 3 覆盖。
2. **required 与 legacy 边界**：升级前任何已初始化项目必须保持旧 foundation/READY 语义；只有升级后新项目进入 required，且 capability 通过后无 active task/READY 是合法状态。Task 6 覆盖。
3. **精确版本与 sources/inputs 混淆**：旧 review 不能证明新 artifact；新 source 不自动废掉已锁定设计；七个建书文件不能省略固定硬输入来混用旧版本。Task 3/4 覆盖。
4. **Foundation handoff 崩溃恢复**：Design Head 已到 foundation_ready、内部 foundation attempt 已建但 Canon 尚未提交时，重启只能得到一份 Foundation receipt / Canon root / chapter:1 READY。Task 5 覆盖。
5. **Design Store 篡改与备份恢复**：object、bundle、commit、HEAD、Foundation receipt 任一不一致都必须被 verify/backup/restore 发现；legacy 项目没有 Design Store 仍合法。Task 7 覆盖。

---

### Task 1: 建立内容寻址 Design Store

**Files:**
- Create: `internal/domain/core_design.go`
- Create: `internal/store/core_design.go`
- Create: `internal/core/design.go`
- Create: `internal/core/design_test.go`
- Modify: `internal/domain/core_project.go`

**Interfaces:**
- Consumes: 现有 `protocol.DecodeJSON([]byte, any)`、`digestJSON(any)`、`store.CoreStore`。
- Produces:
  - `canonicalDesignArtifact(raw []byte) (domain.CoreDesignArtifact, []byte, string, error)`
  - `canonicalDesignBundle(raw []byte) (domain.CoreDesignBundle, []byte, string, error)`
  - `parseDesignArtifactRef(ref string) (artifactType string, digest string, err error)`
  - `parseDesignBundleRef(ref string) (digest string, err error)`
  - `(*CoreStore).SaveCoreDesignArtifact(digest string, data []byte) error`
  - `(*CoreStore).ReadCoreDesignArtifact(digest string) ([]byte, error)`
  - `(*CoreStore).SaveCoreDesignBundle(digest string, data []byte) error`
  - `(*CoreStore).ReadCoreDesignBundle(digest string) ([]byte, error)`
  - `(*CoreStore).SaveCoreDesignCommit(root string, commit *domain.CoreDesignCommit) error`
  - `(*CoreStore).LoadCoreDesignCommit(root string) (*domain.CoreDesignCommit, error)`
  - `(*CoreStore).SaveCoreDesignHead(head *domain.CoreDesignHead) error`
  - `(*CoreStore).LoadCoreDesignHead() (*domain.CoreDesignHead, error)`

- [ ] **Step 1: 写内容寻址失败测试**

在 `internal/core/design_test.go` 增加：

```go
func TestDesignArtifactRefBindsPayloadAndHardInputs(t *testing.T) {
    a := []byte(`{"schema_version":1,"artifact_type":"story_concept","inputs":["creative_brief@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"],"sources":[],"payload":{"story":"同一个故事"}}`)
    b := []byte(`{
      "payload":{"story":"同一个故事"},
      "sources":[],
      "inputs":["creative_brief@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"],
      "artifact_type":"story_concept",
      "schema_version":1
    }`)

    _, _, refA, err := canonicalDesignArtifact(a)
    if err != nil {
        t.Fatal(err)
    }
    _, _, refB, err := canonicalDesignArtifact(b)
    if err != nil {
        t.Fatal(err)
    }
    if refA == refB {
        t.Fatalf("hard input change did not change artifact ref: %s", refA)
    }
}

func TestDesignBundleRefIsStableAcrossJSONFormatting(t *testing.T) {
    a := []byte(`{"schema_version":1,"selections":{"story_concept":"story_concept@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}`)
    b := []byte("{\n  \"selections\": {\"story_concept\": \"story_concept@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\"},\n  \"schema_version\": 1\n}")

    _, _, refA, err := canonicalDesignBundle(a)
    if err != nil {
        t.Fatal(err)
    }
    _, _, refB, err := canonicalDesignBundle(b)
    if err != nil {
        t.Fatal(err)
    }
    if refA != refB {
        t.Fatalf("same bundle got refs %q and %q", refA, refB)
    }
}
```

- [ ] **Step 2: 运行测试确认失败**

Run:

```bash
go test ./internal/core -run 'TestDesignArtifactRefBindsPayloadAndHardInputs|TestDesignBundleRefIsStableAcrossJSONFormatting' -v
```

Expected: FAIL，因为 Design 类型和 canonical helpers 尚不存在。

- [ ] **Step 3: 定义最小领域类型**

在 `internal/domain/core_design.go` 写入：

```go
package domain

const (
    DesignModeRequired = "required"
    DesignModeLegacy   = "legacy"

    DesignCheckpointStoryLocked     = "story_locked"
    DesignCheckpointFoundationReady = "foundation_ready"
)

type CoreDesignArtifact struct {
    SchemaVersion int      `json:"schema_version"`
    ArtifactType  string   `json:"artifact_type"`
    Inputs        []string `json:"inputs,omitempty"`
    Sources       []string `json:"sources,omitempty"`
    Supersedes    string   `json:"supersedes,omitempty"`
    Payload       any      `json:"payload"`
}

type CoreDesignBundle struct {
    SchemaVersion int               `json:"schema_version"`
    Selections    map[string]string `json:"selections"`
}

type CoreDesignCommit struct {
    SchemaVersion           int               `json:"schema_version"`
    Checkpoint              string            `json:"checkpoint"`
    ParentDesignRoot        string            `json:"parent_design_root,omitempty"`
    BundleRef               string            `json:"bundle_ref"`
    Evidence                map[string]string `json:"evidence"`
    CheckpointPolicyVersion int               `json:"checkpoint_policy_version"`
}

type CoreDesignHead struct {
    SchemaVersion int    `json:"schema_version"`
    DesignRoot    string `json:"design_root"`
    Checkpoint    string `json:"checkpoint"`
}

type CoreDesignReceipt struct {
    SchemaVersion       int    `json:"schema_version"`
    SubmissionID       string `json:"submission_id"`
    Operation          string `json:"operation"`
    Result             string `json:"result"`
    SnapshotDigest     string `json:"snapshot_digest"`
    PreviousDesignRoot string `json:"previous_design_root,omitempty"`
    NewDesignRoot      string `json:"new_design_root,omitempty"`
    Problem            string `json:"problem,omitempty"`
    CommittedAt        string `json:"committed_at,omitempty"`
}
```

并在 `CoreProjectState` 先加入但暂不激活：

```go
DesignMode string `json:"design_mode,omitempty"`
```

此时不要改 `coreSchemaVersion`，不要改 `InitProject` 默认行为。

- [ ] **Step 4: 实现 canonical ref 与不可变存储**

`internal/core/design.go` 的关键逻辑固定为：

```go
var sha256HexPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func designArtifactRef(artifactType, digest string) string {
    return artifactType + "@sha256:" + digest
}

func designBundleRef(digest string) string {
    return "design_bundle@sha256:" + digest
}

func canonicalDesignArtifact(raw []byte) (domain.CoreDesignArtifact, []byte, string, error) {
    var artifact domain.CoreDesignArtifact
    if err := protocol.DecodeJSON(raw, &artifact); err != nil {
        return artifact, nil, "", err
    }
    artifact.ArtifactType = strings.TrimSpace(artifact.ArtifactType)
    if artifact.SchemaVersion != 1 || artifact.ArtifactType == "" || artifact.Payload == nil {
        return artifact, nil, "", fmt.Errorf("invalid design artifact envelope")
    }
    sort.Strings(artifact.Inputs)
    sort.Strings(artifact.Sources)
    if hasDuplicateStrings(artifact.Inputs) || hasDuplicateStrings(artifact.Sources) {
        return artifact, nil, "", fmt.Errorf("design refs must be unique")
    }
    canonical, err := json.Marshal(artifact)
    if err != nil {
        return artifact, nil, "", err
    }
    digest := sha256Bytes(canonical)
    return artifact, canonical, designArtifactRef(artifact.ArtifactType, digest), nil
}

func canonicalDesignBundle(raw []byte) (domain.CoreDesignBundle, []byte, string, error) {
    var bundle domain.CoreDesignBundle
    if err := protocol.DecodeJSON(raw, &bundle); err != nil {
        return bundle, nil, "", err
    }
    if bundle.SchemaVersion != 1 || len(bundle.Selections) == 0 {
        return bundle, nil, "", fmt.Errorf("invalid design bundle")
    }
    canonical, err := json.Marshal(bundle)
    if err != nil {
        return bundle, nil, "", err
    }
    digest := sha256Bytes(canonical)
    return bundle, canonical, designBundleRef(digest), nil
}

func hasDuplicateStrings(items []string) bool {
    seen := make(map[string]bool, len(items))
    for _, item := range items {
        if seen[item] {
            return true
        }
        seen[item] = true
    }
    return false
}
```

`parseDesignArtifactRef` 必须验证 `@sha256:`、64 位小写 hex；`parseDesignBundleRef` 只接受 `design_bundle@sha256:<digest>`。

`internal/store/core_design.go` 使用：

```text
meta/core/design/objects/<digest>.json
meta/core/design/bundles/<digest>.json
meta/core/design/commits/<design-root>.json
meta/core/design/HEAD.json
meta/core/design/receipts/<submission-id>.json
```

所有 Save 都遵循“已存在且字节完全相同 => 幂等；已存在但内容不同 => error”，不覆盖旧对象。

- [ ] **Step 5: 补存储不可变测试并运行**

增加测试：同 digest 重写相同 bytes 成功；同 digest 不同 bytes 失败；HEAD 读写 round-trip。

Run:

```bash
go test ./internal/core -run 'TestDesign' -v
go test ./internal/store -v
```

Expected: PASS。

- [ ] **Step 6: 提交**

```bash
git add internal/domain/core_design.go internal/domain/core_project.go internal/store/core_design.go internal/core/design.go internal/core/design_test.go
git commit -m "feat: add content addressed design store"
git push origin provider-free-novel-core
```

---

### Task 2: 复用稳定 Drive 快照并实现 Design import

**Files:**
- Create: `internal/protocol/design.go`
- Create: `internal/core/inbox_snapshot.go`
- Create: `internal/core/design_submission.go`
- Create: `internal/core/design_submission_test.go`
- Create: `internal/core/design_test_helpers_test.go`
- Modify: `internal/domain/core_design.go`
- Modify: `internal/store/core_design.go`
- Modify: `internal/core/submission.go`
- Test: `internal/core/submission_snapshot_test.go`
- Test: `internal/core/security_test.go`

**Interfaces:**
- Consumes: Task 1 canonical artifact/bundle helpers和 Design Store。
- Produces:
  - `protocol.DesignManifest`
  - `ScanDesignSubmission(submissionID string) (domain.CoreDesignSubmissionRecord, error)`
  - `scanDesignSubmissionLocked(submissionID string) (domain.CoreDesignSubmissionRecord, error)`
  - `ProcessDesignSubmission(submissionID string) (domain.CoreDesignSubmissionResult, error)`
  - `processDesignSubmissionLocked(submissionID string) (domain.CoreDesignSubmissionResult, error)`
  - `readInboxCandidate(workspaceRoot, baseRel string, files []string, maxTotal int) (map[string][]byte, string, error)`
  - `advanceStableObservation(current stableObservation, scanDigest string, now time.Time, quiet time.Duration) (stableObservation, string)`

- [ ] **Step 1: 先写稳定快照回归测试**

在 `internal/core/submission_snapshot_test.go` 增加一个测试，确保重构后现有 Submission 的语义不变：

```go
func TestSubmissionStillRejectsMutationAfterSharedSnapshotRefactor(t *testing.T) {
    project, _, workspace := newCapabilityPassedProject(t)
    project.submissionQuietPeriod = 0
    ready := readReady(t, workspace)
    files := validFoundationArtifacts()
    writeSubmission(t, workspace, ready, files, false)

    first, err := project.ScanActiveSubmission()
    if err != nil || first.State != "READY_TO_VALIDATE" {
        t.Fatalf("first scan=%+v err=%v", first, err)
    }

    path := filepath.Join(workspace, "exchange", "inbox", ready.TaskID, ready.AttemptID, "foundation.json")
    if err := os.WriteFile(path, []byte(`{"title":"changed"}`), 0o644); err != nil {
        t.Fatal(err)
    }

    second, err := project.ScanActiveSubmission()
    if err != nil {
        t.Fatal(err)
    }
    if !second.Conflict || second.State != "INVALID" {
        t.Fatalf("mutated locked submission=%+v", second)
    }
}
```

- [ ] **Step 2: 运行现有快照测试基线**

```bash
go test ./internal/core -run 'TestSubmission.*Snapshot|TestSubmissionStillRejectsMutationAfterSharedSnapshotRefactor' -v
```

新测试应先 FAIL 或无法编译，既有测试应保持当前结果。

- [ ] **Step 3: 提取共享底层实现**

`internal/core/inbox_snapshot.go` 定义：

```go
type stableObservation struct {
    ObservedDigest string
    ObservedAt     string
    SnapshotDigest string
}

func readInboxCandidate(workspaceRoot, baseRel string, files []string, maxTotal int) (map[string][]byte, string, error)
func advanceStableObservation(current stableObservation, scanDigest string, now time.Time, quiet time.Duration) (stableObservation, string)
```

`readInboxCandidate` 必须：

- 只允许 `manifest.json` + manifest 声明的 basename 文件；
- 拒绝目录和 symlink；
- 通过 `protocol.ReadUTF8` 读取；
- 计算总字节上限；
- 返回包含 manifest 的文件 map 和 `digestArtifactManifest(digestArtifacts(files))`。

`advanceStableObservation` 只返回四种决定字符串：`PENDING`、`READY_TO_SNAPSHOT`、`LOCKED_SAME`、`LOCKED_CONFLICT`。现有 `scanActiveSubmissionLocked` 改用这两个 helper，但现有 Record/路径/错误文字保持兼容。

- [ ] **Step 4: 定义 Design machine manifest 与 reconciliation record**

`internal/protocol/design.go`：

```go
package protocol

type DesignManifest struct {
    SchemaVersion   int      `json:"schema_version"`
    ProjectID       string   `json:"project_id"`
    SubmissionID    string   `json:"submission_id"`
    ProtocolVersion string   `json:"protocol_version"`
    Operation       string   `json:"operation"`
    Files           []string `json:"files"`
}
```

`internal/domain/core_design.go` 增加：

```go
type CoreDesignSubmissionRecord struct {
    SchemaVersion       int               `json:"schema_version"`
    SubmissionID       string            `json:"submission_id"`
    Operation          string            `json:"operation,omitempty"`
    State              string            `json:"state"`
    ObservedDigest     string            `json:"observed_digest,omitempty"`
    ObservedAt         string            `json:"observed_at,omitempty"`
    SnapshotDigest     string            `json:"snapshot_digest,omitempty"`
    Conflict           bool              `json:"conflict,omitempty"`
    Problem            string            `json:"problem,omitempty"`
    Result             string            `json:"result,omitempty"`
    Refs               map[string]string `json:"refs,omitempty"`
    PreviousDesignRoot string            `json:"previous_design_root,omitempty"`
    NewDesignRoot      string            `json:"new_design_root,omitempty"`
    ReceiptPath        string            `json:"receipt_path,omitempty"`
}

type CoreDesignSubmissionResult struct {
    SchemaVersion       int               `json:"schema_version"`
    SubmissionID       string            `json:"submission_id"`
    Result             string            `json:"result"`
    Refs               map[string]string `json:"refs,omitempty"`
    PreviousDesignRoot string            `json:"previous_design_root,omitempty"`
    NewDesignRoot      string            `json:"new_design_root,omitempty"`
    ReceiptRef         string            `json:"receipt_ref,omitempty"`
    Problem            string            `json:"problem,omitempty"`
}
```

Design snapshot/reconcile 路径固定：

```text
meta/core/design/reconcile/<submission-id>.json
meta/core/design/snapshots/<submission-id>/*
```

公开的 `ScanDesignSubmission` / `ProcessDesignSubmission` 必须先 `acquireProjectMutationLock`，然后分别调用 locked 版本；后续 `servePassLocked` 已经持锁，只能调用 `scanDesignSubmissionLocked` / `processDesignSubmissionLocked`，禁止重入 public API。


- [ ] **Step 5: 写 Design import 失败测试**

`internal/core/design_submission_test.go`：

```go
func TestDesignImportLocksSnapshotAndReturnsCoreRefs(t *testing.T) {
    project, _, workspace := newCapabilityPassedProject(t)
    state, err := project.store.LoadCoreProjectState()
    if err != nil {
        t.Fatal(err)
    }
    state.DesignMode = domain.DesignModeRequired
    if err := project.store.SaveCoreProjectState(state); err != nil {
        t.Fatal(err)
    }
    project.submissionQuietPeriod = 0

    writeDesignImport(t, workspace, "design-001", map[string]any{
        "concept.json": map[string]any{
            "kind": "artifact",
            "artifact": map[string]any{
                "schema_version": 1,
                "artifact_type": "story_concept",
                "inputs": []string{},
                "sources": []string{},
                "payload": map[string]any{
                    "story": "测试故事",
                    "protagonist_goal": "完成目标",
                    "central_conflict": "持续冲突",
                    "story_engine": "持续选择",
                    "change_path": "长期变化",
                    "ending_direction": "明确收束",
                },
            },
        },
    })

    if _, err := project.ScanDesignSubmission("design-001"); err != nil {
        t.Fatal(err)
    }
    status, err := project.ScanDesignSubmission("design-001")
    if err != nil || status.State != "READY_TO_VALIDATE" {
        t.Fatalf("status=%+v err=%v", status, err)
    }
    result, err := project.ProcessDesignSubmission("design-001")
    if err != nil || result.Result != "IMPORTED" || result.Refs["concept.json"] == "" {
        t.Fatalf("result=%+v err=%v", result, err)
    }
}
```

同时添加：symlink、未知文件、超大总提交、同 submission 锁定后改写必须 INVALID/Conflict。

- [ ] **Step 6: 实现 import，不实现 promote**

Import 文件 envelope 固定为：

```json
{
  "kind": "artifact",
  "artifact": {
    "schema_version": 1,
    "artifact_type": "creative_brief",
    "inputs": [],
    "sources": [],
    "payload": {}
  }
}
```

或：

```json
{
  "kind": "bundle",
  "bundle": {
    "schema_version": 1,
    "selections": {
      "story_concept": "story_concept@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    }
  }
}
```

`ProcessDesignSubmission` 在 `operation=import` 时：

1. 验证 `DesignModeRequired`、capability passed、没有 Canon；
2. 从本地 snapshot 读取，不直接读 Drive；
3. canonicalize；
4. 对 artifact 的每个 inputs/sources 先执行 `parseDesignArtifactRef`，并确认对应对象已经存在；首版禁止 submission-local ref；
5. supersedes 非空时必须是已存在、且 artifact_type 与当前对象相同的 artifact_ref；它只记录版本关系，不改变 Design Head；
6. bundle 中每个 selection 必须是已存在且 slot/type 可识别的 artifact_ref；
7. 保存不可变 object/bundle；
8. `Refs[fileName]=ref`；
9. 写 `exchange/design/result/<submission-id>.json`；
10. Record => SETTLED；
11. 同 submission_id + 同 snapshot 重放返回同一 result；
12. `operation=promote` 暂时返回 `unsupported design operation`。

- [ ] **Step 7: 建立后续任务共用测试 helper**

在 `internal/core/design_test_helpers_test.go` 定义下列 helper；所有 helper 都调用真实 Project/Design API，不直接写 `meta/core/design` 绕过被测代码：

```go
func newRequiredDesignProjectForTest(t *testing.T) (*Project, string, string)
func importDesignArtifactForTest(t *testing.T, p *Project, artifactType string, inputs []string, payload any) string
func importDesignBundleForTest(t *testing.T, p *Project, selections map[string]string) string
func promoteForTest(t *testing.T, p *Project, submissionID, expectedRoot, checkpoint, bundleRef string, evidence map[string]string) domain.CoreDesignSubmissionResult

func validCreativeBriefPayload() map[string]any
func validStoryDecisionsPayload() map[string]any
func validStoryConceptPayload() map[string]any
```

`newRequiredDesignProjectForTest` 固定使用 fresh `InitProject`，直接按 capability challenge 写 ack/probe，然后把本地 `CoreProjectState.DesignMode` 显式设为 `required`。Tasks 1–5 期间不得调用 `Reconcile` 来“确认 capability”，因为当前 1.0 的旧 reconcile 会创建 foundation task；直接用 `capabilityStatus(state)` 断言 passed 即可。Task 6 激活默认 required 后，删除手工 DesignMode 覆盖，并新增断言确认 fresh project 仍无 production task。

三个合法 payload fixture 必须严格按 spec 的固定字段返回，不得用空 map 代替：

```go
func validCreativeBriefPayload() map[string]any {
    return map[string]any{
        "purpose": "验证长篇语义设计链",
        "target_genre": "都市男频",
        "platform_constraints": []any{},
        "preferences": []any{"故事主线优先"},
        "exclusions": []any{"不把金手指当故事本身"},
        "author_principles": []any{"先定故事再定手段"},
    }
}

func validStoryDecisionsPayload() map[string]any {
    return map[string]any{
        "decisions": []any{
            map[string]any{
                "id": "story-decision-001",
                "status": "locked",
                "statement": "故事主线优先于爽点机制",
                "blocking": true,
            },
        },
    }
}

func validStoryConceptPayload() map[string]any {
    return map[string]any{
        "story": "主角为解决迫近的个人危机被迫进入新的竞争环境，并逐步改变自己与核心关系。",
        "protagonist_goal": "解决眼前危机并获得持续生存空间",
        "central_conflict": "个人目标与竞争环境的规则持续冲突",
        "story_engine": "每次推进目标都会制造新的选择、代价和关系变化",
        "change_path": "主角从被动求生变成能主动选择并承担后果",
        "ending_direction": "核心危机和长期关系获得明确收束",
    }
}
```

- [ ] **Step 8: 跑回归与 import 测试**

```bash
go test ./internal/core -run 'TestSubmission|TestDesignImport|TestDesign.*Symlink|TestDesign.*Oversize' -v
go test ./internal/protocol ./internal/store -v
```

Expected: PASS。

- [ ] **Step 9: 提交**

```bash
git add internal/protocol/design.go internal/domain/core_design.go internal/store/core_design.go internal/core/inbox_snapshot.go internal/core/submission.go internal/core/design_submission.go internal/core/design_submission_test.go internal/core/design_test_helpers_test.go internal/core/submission_snapshot_test.go internal/core/security_test.go
git commit -m "feat: import immutable semantic design artifacts"
git push origin provider-free-novel-core
```

---

### Task 3: 实现 story_locked Promote、精确审稿与 CAS

**Files:**
- Modify: `internal/protocol/design.go`
- Modify: `internal/domain/core_design.go`
- Modify: `internal/store/core_design.go`
- Create: `internal/core/design_checkpoint.go`
- Modify: `internal/core/design_submission.go`
- Modify: `internal/core/design_test_helpers_test.go`
- Test: `internal/core/design_test.go`
- Test: `internal/core/design_submission_test.go`

**Interfaces:**
- Consumes: Task 1 refs/store、Task 2 Design snapshot/import 与测试 helper。
- Produces:
  - `protocol.DesignPromoteRequest`
  - `validateStoryLocked(project *Project, bundleRef string, evidence map[string]string) error`
  - `validateCreativeBriefPayload(payload any) error`
  - `validateStoryDecisionsPayload(payload any) error`
  - `validateStoryConceptPayload(payload any) error`
  - `validateBundleClosure(project *Project, bundle domain.CoreDesignBundle) error`
  - `validateReviewArtifact(artifact domain.CoreDesignArtifact, wantType, subjectRef string) error`
  - `validateAuthorConfirmation(artifact domain.CoreDesignArtifact, subjectRef string) error`
  - `promoteDesignLocked(request protocol.DesignPromoteRequest, record *domain.CoreDesignSubmissionRecord) (domain.CoreDesignSubmissionResult, error)`
  - `finishPreparedDesignPromote(record *domain.CoreDesignSubmissionRecord) (domain.CoreDesignSubmissionResult, error)`
  - `computeDesignRoot(commit domain.CoreDesignCommit) (string, error)`

- [ ] **Step 1: 写 story_locked 成功与精确版本失败测试**

在 `internal/core/design_submission_test.go` 增加：

```go
func TestStoryLockedRequiresExactReviewAndAuthorConfirmation(t *testing.T) {
    project, _, _ := newRequiredDesignProjectForTest(t)

    briefRef := importDesignArtifactForTest(
        t, project, "creative_brief", nil, validCreativeBriefPayload(),
    )
    decisionsRef := importDesignArtifactForTest(
        t, project, "story_decisions", nil, validStoryDecisionsPayload(),
    )
    conceptRef := importDesignArtifactForTest(
        t, project, "story_concept",
        []string{briefRef, decisionsRef},
        validStoryConceptPayload(),
    )
    bundleRef := importDesignBundleForTest(t, project, map[string]string{
        "creative_brief": briefRef,
        "story_decisions": decisionsRef,
        "story_concept": conceptRef,
    })

    reviewRef := importDesignArtifactForTest(
        t, project, "story_review", nil,
        map[string]any{
            "review_type": "story_review",
            "subject_ref": conceptRef,
            "policy_version": 1,
            "verdict": "PASS",
            "findings": []any{},
            "created_from_context_refs": []any{briefRef, decisionsRef},
        },
    )
    approvalRef := importDesignArtifactForTest(
        t, project, "author_confirmation", nil,
        map[string]any{
            "subject_ref": conceptRef,
            "decision": "APPROVED",
        },
    )

    result := promoteForTest(
        t, project, "design-promote-story-001", "",
        domain.DesignCheckpointStoryLocked,
        bundleRef,
        map[string]string{
            "story_review": reviewRef,
            "author_confirmation": approvalRef,
        },
    )
    if result.Result != "PROMOTED" || result.NewDesignRoot == "" {
        t.Fatalf("result=%+v", result)
    }
}

func TestStoryLockedRejectsReviewForDifferentConcept(t *testing.T) {
    project, _, _ := newRequiredDesignProjectForTest(t)
    briefRef := importDesignArtifactForTest(t, project, "creative_brief", nil, validCreativeBriefPayload())
    decisionsRef := importDesignArtifactForTest(t, project, "story_decisions", nil, validStoryDecisionsPayload())

    oldPayload := validStoryConceptPayload()
    oldPayload["story"] = "旧故事版本"
    oldConceptRef := importDesignArtifactForTest(
        t, project, "story_concept", []string{briefRef, decisionsRef}, oldPayload,
    )
    currentConceptRef := importDesignArtifactForTest(
        t, project, "story_concept", []string{briefRef, decisionsRef}, validStoryConceptPayload(),
    )
    bundleRef := importDesignBundleForTest(t, project, map[string]string{
        "creative_brief": briefRef,
        "story_decisions": decisionsRef,
        "story_concept": currentConceptRef,
    })
    reviewRef := importDesignArtifactForTest(t, project, "story_review", nil, map[string]any{
        "review_type": "story_review",
        "subject_ref": oldConceptRef,
        "policy_version": 1,
        "verdict": "PASS",
        "findings": []any{},
        "created_from_context_refs": []any{briefRef, decisionsRef},
    })
    approvalRef := importDesignArtifactForTest(t, project, "author_confirmation", nil, map[string]any{
        "subject_ref": currentConceptRef,
        "decision": "APPROVED",
    })

    result := promoteForTest(
        t, project, "design-promote-story-stale-review", "",
        domain.DesignCheckpointStoryLocked, bundleRef,
        map[string]string{
            "story_review": reviewRef,
            "author_confirmation": approvalRef,
        },
    )
    if result.Result != "INVALID" || !strings.Contains(result.Problem, "story_review") {
        t.Fatalf("result=%+v", result)
    }
}
```

再增加四个具名测试：

- `TestStoryLockedRejectsAuthorConfirmationForDifferentConcept`；
- `TestStoryLockedRejectsBlockingOpenDecision`；
- `TestStoryLockedRejectsInvalidCreativeBriefShape`；
- `TestStoryLockedRejectsEmptyStoryConceptField`。

每个测试只破坏一个目标约束，其余 bundle/evidence 必须保持合法，避免一个用例同时命中多条错误路径。

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./internal/core -run 'TestStoryLocked' -v
```

Expected: FAIL，因为 promote/checkpoint validator 尚不存在。

- [ ] **Step 3: 定义 promote machine request**

在 `internal/protocol/design.go` 增加：

```go
type DesignPromoteRequest struct {
    SchemaVersion      int               `json:"schema_version"`
    ProjectID          string            `json:"project_id"`
    SubmissionID       string            `json:"submission_id"`
    ProtocolVersion    string            `json:"protocol_version"`
    ExpectedDesignRoot string            `json:"expected_design_root,omitempty"`
    Checkpoint         string            `json:"checkpoint"`
    BundleRef          string            `json:"bundle_ref"`
    Evidence           map[string]string `json:"evidence"`
}
```

promote submission 固定为 `manifest.operation="promote"`、`manifest.files=["promote.json"]`。manifest 的 submission_id 与 promote.json 的 submission_id 必须完全相同。

- [ ] **Step 4: 实现 story_locked 固定结构校验**

`internal/core/design_checkpoint.go` 实现上面 Interfaces 的五个 validator。确定性规则固定为：

- bundle selections 恰好是 `creative_brief/story_decisions/story_concept`；
- creative_brief 的 purpose/target_genre 是非空字符串，四个数组只含唯一非空字符串；
- story_decisions 的 id 唯一；status 只允许 locked/rejected/open；statement 非空；blocking 必须 bool；supersedes 只能引用同 payload 的另一个 id 且不得自指；
- 不允许 `blocking=true,status=open`；
- story_concept 的六个字段都必须是非空字符串；
- story_concept.inputs 必须至少包含当前 briefRef 与 decisionsRef，不允许用 sources 替代；额外 hard input 允许存在，但必须通过 bundle 一致性闭包；
- evidence key 恰好是 `story_review` 和 `author_confirmation`；
- story_review 必须 artifact_type=story_review、payload.review_type=story_review、payload.subject_ref=current concept、payload.policy_version 为正整数、payload.verdict=PASS，findings/created_from_context_refs 为数组；
- author_confirmation 必须 artifact_type=author_confirmation、subject_ref=current concept、decision=APPROVED；
- Core 不判断 review finding 的文学内容。

增加固定闭包 helper：

```go
func containsAllRefs(got []string, required ...string) bool {
    seen := make(map[string]bool, len(got))
    for _, ref := range got {
        seen[ref] = true
    }
    for _, ref := range required {
        if !seen[ref] {
            return false
        }
    }
    return true
}

func validateBundleClosure(project *Project, bundle domain.CoreDesignBundle) error {
    for slot, selectedRef := range bundle.Selections {
        artifact, err := project.loadDesignArtifactRef(selectedRef)
        if err != nil {
            return err
        }
        if artifact.ArtifactType != slot {
            return fmt.Errorf("bundle slot %s selects artifact type %s", slot, artifact.ArtifactType)
        }
        for _, inputRef := range artifact.Inputs {
            inputType, _, err := parseDesignArtifactRef(inputRef)
            if err != nil {
                return err
            }
            if selectedInput, ok := bundle.Selections[inputType]; ok && selectedInput != inputRef {
                return fmt.Errorf("%s depends on stale %s", slot, inputType)
            }
        }
    }
    return nil
}
```

`story_locked` 与 `foundation_ready` 都必须先调用 `validateBundleClosure`；sources 只要求对象存在，不参与“必须等于当前 selection”的替换规则。

- [ ] **Step 5: 实现线性 Design commit、CAS 与可恢复 APPLYING**

`CoreDesignSubmissionRecord` 增加：

```go
PreparedCommit     *CoreDesignCommit `json:"prepared_commit,omitempty"`
PreparedDesignRoot string            `json:"prepared_design_root,omitempty"`
PreviousDesignRoot string            `json:"previous_design_root,omitempty"`
```

`computeDesignRoot` 只对 `CoreDesignCommit` 的 Canonical JSON 做 SHA-256，返回 64 位小写 hex；receipt 时间不进入 root。

story_locked promote 的 parent 规则：

```go
if head == nil {
    if request.ExpectedDesignRoot != "" {
        return stale
    }
} else {
    if head.Checkpoint != domain.DesignCheckpointStoryLocked {
        return invalid
    }
    if request.ExpectedDesignRoot != head.DesignRoot {
        return stale
    }
}
```

Promote 顺序固定：

1. 读取当前 HEAD 并做 expected root CAS；
2. 跑 checkpoint validator；
3. 构造 commit；
4. 计算 root；
5. Record 保存 `State="APPLYING"`、previous root、prepared commit/root；
6. 保存 immutable commit；
7. CAS 写 HEAD；
8. 写 immutable Design receipt，其中 `SnapshotDigest=record.SnapshotDigest`；
9. 写 Drive result，`receipt_ref` 使用本地 receipt 相对路径；
10. Record => SETTLED。

`finishPreparedDesignPromote` 的恢复规则：

- HEAD 仍等于 PreviousDesignRoot：继续写 prepared commit/head；
- HEAD 已等于 PreparedDesignRoot：只补 receipt/result/SETTLED；
- HEAD 是第三个 root：Record => INVALID，Problem=`design head changed while prepared promote was recovering`，不得覆盖。

- [ ] **Step 6: 写 CAS、同 submission 幂等与重新 story lock 测试**

在 `design_test_helpers_test.go` 增加一个真实 API helper：

```go
func validStoryLockInputsForTest(
    t *testing.T,
    p *Project,
) (bundleRef string, evidence map[string]string) {
    t.Helper()
    briefRef := importDesignArtifactForTest(t, p, "creative_brief", nil, validCreativeBriefPayload())
    decisionsRef := importDesignArtifactForTest(t, p, "story_decisions", nil, validStoryDecisionsPayload())
    conceptRef := importDesignArtifactForTest(
        t, p, "story_concept", []string{briefRef, decisionsRef}, validStoryConceptPayload(),
    )
    bundleRef = importDesignBundleForTest(t, p, map[string]string{
        "creative_brief": briefRef,
        "story_decisions": decisionsRef,
        "story_concept": conceptRef,
    })
    reviewRef := importDesignArtifactForTest(t, p, "story_review", nil, map[string]any{
        "review_type": "story_review",
        "subject_ref": conceptRef,
        "policy_version": 1,
        "verdict": "PASS",
        "findings": []any{},
        "created_from_context_refs": []any{briefRef, decisionsRef},
    })
    approvalRef := importDesignArtifactForTest(t, p, "author_confirmation", nil, map[string]any{
        "subject_ref": conceptRef,
        "decision": "APPROVED",
    })
    return bundleRef, map[string]string{
        "story_review": reviewRef,
        "author_confirmation": approvalRef,
    }
}
```

再写：

```go
func TestDesignPromoteRejectsStaleExpectedRoot(t *testing.T) {
    project, _, _ := newRequiredDesignProjectForTest(t)
    bundleRef, evidence := validStoryLockInputsForTest(t, project)

    first := promoteForTest(
        t, project, "design-promote-cas-1", "",
        domain.DesignCheckpointStoryLocked, bundleRef, evidence,
    )
    if first.Result != "PROMOTED" {
        t.Fatalf("first=%+v", first)
    }

    stale := promoteForTest(
        t, project, "design-promote-cas-2", "",
        domain.DesignCheckpointStoryLocked, bundleRef, evidence,
    )
    if stale.Result != "STALE_DESIGN_HEAD" {
        t.Fatalf("stale=%+v", stale)
    }
}

func TestDesignPromoteReplayUsesSettledSubmissionResult(t *testing.T) {
    project, _, _ := newRequiredDesignProjectForTest(t)
    bundleRef, evidence := validStoryLockInputsForTest(t, project)

    first := promoteForTest(
        t, project, "design-promote-replay", "",
        domain.DesignCheckpointStoryLocked, bundleRef, evidence,
    )
    second := promoteForTest(
        t, project, "design-promote-replay", "",
        domain.DesignCheckpointStoryLocked, bundleRef, evidence,
    )
    if !reflect.DeepEqual(first, second) {
        t.Fatalf("replay changed result: first=%+v second=%+v", first, second)
    }
}
```

另外增加 `TestStoryLockedCanAdvanceToNewStoryLockedWithCurrentExpectedRoot`，第二次使用新的 concept/review/confirmation 和第一次的 NewDesignRoot，断言线性 parent 正确。

- [ ] **Step 7: 运行测试**

```bash
go test ./internal/core -run 'TestStoryLocked|TestDesignPromote' -v
```

Expected: PASS。

- [ ] **Step 8: 提交**

```bash
git add internal/protocol/design.go internal/domain/core_design.go internal/store/core_design.go internal/core/design_checkpoint.go internal/core/design_submission.go internal/core/design_test_helpers_test.go internal/core/design_test.go internal/core/design_submission_test.go
git commit -m "feat: lock story concepts with exact evidence"
git push origin provider-free-novel-core
```

---

### Task 4: 实现 foundation_ready 与无副作用 Foundation 预校验

**Files:**
- Modify: `internal/core/design_checkpoint.go`
- Modify: `internal/core/design.go`
- Modify: `internal/core/production_part2.go`
- Modify: `internal/core/design_test_helpers_test.go`
- Test: `internal/core/design_test.go`
- Test: `internal/core/foundation_test.go`

**Interfaces:**
- Consumes: story_locked Design Head、Task 3 CAS、现有 `validateAndCanonicalizeFoundation` / `foundationLongformState` / `foundationPlanningState`。
- Produces:
  - `prepareFoundationArtifacts(artifacts map[string][]byte, state *domain.CoreProductionState) (preparedFoundation, []string, error)`
  - `validateWholeBookSkeleton(bookPlanPayload any) []string`
  - `(*Project).foundationArtifactsFromDesignBundle(bundle domain.CoreDesignBundle) (map[string][]byte, error)`
  - `(*Project).validateFoundationReady(head *domain.CoreDesignHead, bundleRef string, evidence map[string]string, state *domain.CoreProductionState) error`

- [ ] **Step 1: 先锁定“预校验无副作用”的现有行为**

在 `internal/core/foundation_test.go` 增加：

```go
func TestPrepareFoundationArtifactsDoesNotConsumeEntitySequence(t *testing.T) {
    state := newCoreProductionState()
    before := *state

    prepared, violations, err := prepareFoundationArtifacts(validFoundationArtifacts(), state)
    if err != nil {
        t.Fatal(err)
    }
    if len(violations) != 0 {
        t.Fatalf("violations=%v", violations)
    }
    if len(prepared.Canonical) != len(foundationArtifactNames) {
        t.Fatalf("canonical=%v", prepared.Canonical)
    }
    if !reflect.DeepEqual(before, *state) {
        t.Fatalf("prevalidation mutated state: before=%+v after=%+v", before, *state)
    }
}
```

这个测试必须验证整个 `CoreProductionState` 不变，而不只检查 `NextEntitySeq`。

- [ ] **Step 2: 提取唯一 Foundation preparation 入口**

在 `internal/core/production_part2.go` 定义：

```go
type preparedFoundation struct {
    Canonical map[string][]byte
    Mappings  []IDMapping
    Longform  domain.CoreLongformState
    Planning  domain.CorePlanningState
}

func prepareFoundationArtifacts(
    artifacts map[string][]byte,
    state *domain.CoreProductionState,
) (preparedFoundation, []string, error) {
    canonical, mappings, violations, err := validateAndCanonicalizeFoundation(artifacts, state)
    if err != nil || len(violations) != 0 {
        return preparedFoundation{}, violations, err
    }

    longform, violations, err := foundationLongformState(canonical["world.json"])
    if err != nil || len(violations) != 0 {
        return preparedFoundation{}, violations, err
    }

    planning, violations, err := foundationPlanningState(canonical["book_plan.json"])
    if err != nil || len(violations) != 0 {
        return preparedFoundation{}, violations, err
    }

    return preparedFoundation{
        Canonical: canonical,
        Mappings: mappings,
        Longform: longform,
        Planning: planning,
    }, nil, nil
}
```

把现有 `settleFoundationLocked` 改成：

1. 仍然先跑 `validateSubmissionIdentity`；
2. 调用 `prepareFoundationArtifacts(sub.Artifacts, state)`；
3. violations 非空仍走现有 `rejectFoundation`；
4. PASS 后调用 Task 5 将重构的共享 `acceptFoundation`。

此时 **不要** 改 legacy Foundation 的最低字段要求，也不要把 whole_book_skeleton 塞进 `foundationPlanningState`。

- [ ] **Step 3: 实现 Design-only 的 whole_book_skeleton 校验**

在 `internal/core/design_checkpoint.go` 定义：

```go
func validateWholeBookSkeleton(payload any) []string
```

只检查：

- payload 必须 object；
- `direction` 非空字符串；
- `whole_book_skeleton` object；
- `stages` 至少两个；
- 每个 stage 的 `id/objective/transition` 都是非空字符串；
- stage id 唯一；
- `ending_connection` 非空字符串。

这个 validator 只从 `validateFoundationReady` 调用。增加回归测试：

```go
func TestLegacyFoundationStillAcceptsBookPlanWithoutWholeBookSkeleton(t *testing.T) {
    project, _, workspace := newCapabilityPassedProject(t)
    if err := project.Reconcile(); err != nil {
        t.Fatal(err)
    }
    ready := readReady(t, workspace)
    artifacts := validFoundationArtifacts()

    settlement, err := project.SettleFoundation(FoundationSubmission{
        Manifest: manifestForReady(ready, artifacts),
        Artifacts: artifacts,
    })
    if err != nil {
        t.Fatal(err)
    }
    if settlement.Result != "ACCEPTED" {
        t.Fatalf("settlement=%+v", settlement)
    }
}
```

这条测试防止 required 新门禁误伤 legacy 1.0 行为。

- [ ] **Step 4: 固定七个设计槽到 Foundation 文件的唯一映射**

在 `internal/core/design.go` 定义：

```go
var foundationDesignSlots = map[string]string{
    "foundation":       "foundation.json",
    "characters":       "characters.json",
    "world":            "world.json",
    "book_plan":        "book_plan.json",
    "ending_contract":  "ending_contract.json",
    "style_profile":    "style_profile.json",
    "platform_profile": "platform_profile.json",
}

func (p *Project) foundationArtifactsFromDesignBundle(
    bundle domain.CoreDesignBundle,
) (map[string][]byte, error) {
    out := make(map[string][]byte, len(foundationDesignSlots))
    for slot, fileName := range foundationDesignSlots {
        ref, ok := bundle.Selections[slot]
        if !ok {
            return nil, fmt.Errorf("design bundle is missing %s", slot)
        }
        artifact, err := p.loadDesignArtifactRef(ref)
        if err != nil {
            return nil, err
        }
        if artifact.ArtifactType != slot {
            return nil, fmt.Errorf("%s selects artifact type %s", slot, artifact.ArtifactType)
        }
        raw, err := json.Marshal(artifact.Payload)
        if err != nil {
            return nil, err
        }
        out[fileName] = raw
    }
    return out, nil
}
```

如果 Task 1 尚未提供，补一个唯一读取入口：

```go
func (p *Project) loadDesignArtifactRef(ref string) (domain.CoreDesignArtifact, error)
func (p *Project) loadDesignBundleRef(ref string) (domain.CoreDesignBundle, error)
```

两个入口都负责 parse ref、从 store 读取、重算 ref 一致性；后续 checkpoint/verify 都复用它们。

- [ ] **Step 5: 建立完整 Foundation Design 测试 fixture**

在 `design_test_helpers_test.go` 增加：

```go
type foundationDesignFixture struct {
    StoryRoot      string
    FoundationRoot string
    BriefRef       string
    DecisionsRef   string
    ConceptRef     string
    BundleRef      string
    ReviewRef      string
}

func makeStoryLockedProjectForTest(t *testing.T) (*Project, string, string, foundationDesignFixture)
func importValidFoundationDesignForTest(
    t *testing.T,
    p *Project,
    briefRef, decisionsRef, conceptRef string,
) (bundleRef string, reviewRef string)
func importFoundationReadinessReviewForTest(
    t *testing.T,
    p *Project,
    bundleRef string,
) string
func decodeFoundationPayloadForTest(t *testing.T, name string) any
func readDesignBundleForTest(t *testing.T, p *Project, ref string) domain.CoreDesignBundle
func readDesignArtifactForTest(t *testing.T, p *Project, ref string) domain.CoreDesignArtifact
func replaceRefForTest(items []string, oldRef, newRef string) []string
func mustDesignHeadForTest(t *testing.T, p *Project) *domain.CoreDesignHead
```

`importValidFoundationDesignForTest` 必须通过真实 import API 依次创建：

- characters/world/ending_contract，inputs = current story_concept；
- foundation，inputs = current story_concept + characters + world；
- book_plan，inputs = current story_concept + characters + world + ending_contract；
- style_profile/platform_profile，inputs = current creative_brief。

`decodeFoundationPayloadForTest` 从 `validFoundationArtifacts()` 解码指定文件；当 `name=="book_plan.json"` 时，只在返回的测试对象中加入合法 `whole_book_skeleton`。不要修改 `validFoundationArtifacts()` 本身，因为它是 legacy 回归 fixture。其他 read/replace/head helper 都是薄包装，不直接改 Design Store。

最后 import 完整十槽 bundle，再 import：

```json
{
  "review_type": "foundation_readiness_review",
  "subject_ref": "<bundle-ref>",
  "policy_version": 1,
  "verdict": "PASS",
  "findings": [],
  "created_from_context_refs": []
}
```

- [ ] **Step 6: 先写 foundation_ready provenance / version 失败测试**

至少增加：

```go
func TestFoundationReadyRejectsBookPlanBuiltFromOldStoryConcept(t *testing.T) {
    project, _, _, fixture := makeStoryLockedProjectForTest(t)

    oldConceptPayload := validStoryConceptPayload()
    oldConceptPayload["story"] = "未被当前故事锁定采用的旧故事"
    oldConceptRef := importDesignArtifactForTest(
        t, project, "story_concept",
        []string{fixture.BriefRef, fixture.DecisionsRef},
        oldConceptPayload,
    )

    bundleRef, _ := importValidFoundationDesignForTest(
        t, project, fixture.BriefRef, fixture.DecisionsRef, fixture.ConceptRef,
    )
    bundle := readDesignBundleForTest(t, project, bundleRef)
    bookPlanRef := bundle.Selections["book_plan"]
    bookPlan := readDesignArtifactForTest(t, project, bookPlanRef)
    bookPlan.Inputs = replaceRefForTest(bookPlan.Inputs, fixture.ConceptRef, oldConceptRef)
    staleBookPlanRef := importDesignArtifactForTest(
        t, project, "book_plan", bookPlan.Inputs, bookPlan.Payload,
    )
    bundle.Selections["book_plan"] = staleBookPlanRef
    staleBundleRef := importDesignBundleForTest(t, project, bundle.Selections)
    staleReviewRef := importFoundationReadinessReviewForTest(
        t, project, staleBundleRef,
    )

    result := promoteForTest(
        t, project, "design-promote-foundation-stale-plan",
        fixture.StoryRoot,
        domain.DesignCheckpointFoundationReady,
        staleBundleRef,
        map[string]string{"foundation_readiness_review": staleReviewRef},
    )
    if result.Result != "INVALID" {
        t.Fatalf("result=%+v", result)
    }
}
```

并分别覆盖：

- 前三槽与当前 story_locked bundle 不一致；
- characters/world/ending_contract 缺 current story_concept input；
- foundation 缺 story_concept / characters / world 任一 input；
- book_plan 缺四个硬输入任一项；
- style/profile 缺 creative_brief；
- whole_book_skeleton 少于 2 stage；
- readiness review.subject_ref 指向另一个 bundle；
- evidence key 多一个或少一个；
- sources 新增或 source_record 新导入，不会让**当前已锁定** story_concept 自动失效。

除“专门测试 review 版本不匹配”的用例外，每个被修改后的 bundle 都必须重新 import 一个 `subject_ref` 精确指向该 bundle 的 readiness review，避免测试先因旧 review 失败而产生假阳性。

- [ ] **Step 7: 实现 foundation_ready validator**

`(*Project).validateFoundationReady` 固定执行：

1. current head 非空且 checkpoint == story_locked；
2. 读取 parent story_locked commit 与 bundle；
3. 当前 bundle 必须恰好 10 个固定 slot；
4. creative_brief/story_decisions/story_concept 三个 ref 与 parent bundle 完全相同；
5. 所有 selection ref 类型匹配；
6. 每个槽位必须包含 spec 规定的最低 hard inputs；额外 hard input 允许存在，但 bundle closure 必须保证其中已被 bundle 选择的 artifact_type 精确指向当前 selection；sources 不替代 hard inputs；
7. 当前 story_decisions 仍无 blocking open；
8. book_plan.payload 通过 `validateWholeBookSkeleton`；
9. `foundationArtifactsFromDesignBundle` 成功；
10. `prepareFoundationArtifacts` PASS；
11. evidence key 恰好 `foundation_readiness_review`；
12. review 的 artifact_type/review_type/subject_ref/policy_version/verdict 精确有效。

在 `promoteDesignLocked` 中：

- foundation_ready 只允许 parent=current story_locked root；
- PASS 后 commit.checkpoint=foundation_ready；
- HEAD 到达 foundation_ready 后任何后续 promote 都返回 INVALID，Problem 明确为建书设计基线已冻结。

- [ ] **Step 8: 确认 Design 阶段完全不消耗生产序号**

新增：

```go
func TestDesignOperationsDoNotConsumeProductionSequences(t *testing.T) {
    project, _, _, fixture := makeStoryLockedProjectForTest(t)
    before, err := project.store.LoadCoreProductionState()
    if err != nil {
        t.Fatal(err)
    }
    if before == nil {
        before = newCoreProductionState()
    }
    snapshot := *before

    bundleRef, reviewRef := importValidFoundationDesignForTest(
        t, project, fixture.BriefRef, fixture.DecisionsRef, fixture.ConceptRef,
    )
    result := promoteForTest(
        t, project, "design-promote-foundation-seq",
        fixture.StoryRoot,
        domain.DesignCheckpointFoundationReady,
        bundleRef,
        map[string]string{"foundation_readiness_review": reviewRef},
    )
    if result.Result != "PROMOTED" {
        t.Fatalf("result=%+v", result)
    }

    after, err := project.store.LoadCoreProductionState()
    if err != nil {
        t.Fatal(err)
    }
    if after == nil {
        after = newCoreProductionState()
    }
    if !reflect.DeepEqual(snapshot, *after) {
        t.Fatalf("design changed production state: before=%+v after=%+v", snapshot, *after)
    }
}
```

Tasks 1–4 期间语义设计路径不得调用 `newTask` / `newAttempt` / 修改 production state。

- [ ] **Step 9: 跑测试**

```bash
go test ./internal/core -run 'TestPrepareFoundation|TestLegacyFoundationStill|TestFoundationReady|TestDesignOperations' -v
```

Expected: PASS。

- [ ] **Step 10: 提交**

```bash
git add internal/core/design_checkpoint.go internal/core/design.go internal/core/production_part2.go internal/core/design_test_helpers_test.go internal/core/design_test.go internal/core/foundation_test.go
git commit -m "feat: validate foundation ready design bundles"
git push origin provider-free-novel-core
```

---

### Task 5: 用现有 commit journal 把 foundation_ready 可靠接入 Canon

**Files:**
- Modify: `internal/domain/core_production.go`
- Modify: `internal/domain/core_commit.go`
- Modify: `internal/core/production.go`
- Modify: `internal/core/production_part2.go`
- Modify: `internal/core/production_part5.go`
- Modify: `internal/core/commit.go`
- Modify: `internal/core/submission.go`
- Modify: `internal/store/core_submission.go`
- Modify: `internal/core/design_test_helpers_test.go`
- Test: `internal/core/foundation_test.go`
- Test: `internal/core/commit_recovery_test.go`
- Test: `internal/core/serve_test.go`

**Interfaces:**
- Consumes: foundation_ready frozen Design Head、Task 4 `preparedFoundation`。
- Produces:
  - `CoreAttempt.InputSource/InputRef`
  - `CoreReceipt.FoundationDesignRoot`
  - `newAttemptWithInput(state *domain.CoreProductionState, task *domain.CoreTask, reason string, required []string, protocolVersion, inputSource, inputRef string) (*domain.CoreAttempt, error)`
  - `attemptInputSource(attempt *domain.CoreAttempt) string`
  - `(*Project).ensureDesignFoundationAttemptLocked(project *domain.CoreProjectState, state *domain.CoreProductionState, head *domain.CoreDesignHead) error`
  - `(*Project).settleDesignFoundationAttemptLocked() (FoundationSettlement, error)`
  - `(*Project).prepareFoundationCommit(project *domain.CoreProjectState, state *domain.CoreProductionState, task *domain.CoreTask, attempt *domain.CoreAttempt, prepared preparedFoundation, foundationDesignRoot string) (*domain.CoreCommitJournal, error)`
  - `(*Project).applyFoundationCommit(project *domain.CoreProjectState, journal *domain.CoreCommitJournal) (FoundationSettlement, error)`

- [ ] **Step 1: 先给 attempt 输入来源写兼容测试**

在 `internal/core/foundation_test.go` 增加：

```go
func TestAttemptInputSourceTreatsEmptyLegacyValueAsDrive(t *testing.T) {
    if got := attemptInputSource(&domain.CoreAttempt{}); got != "drive" {
        t.Fatalf("input source=%q", got)
    }
}
```

这保证 schema 1 已持久化的 attempt 不需要为了新增字段被重写。

- [ ] **Step 2: 给 CoreAttempt 增加来源绑定，并统一 TaskDigest 生成**

`internal/domain/core_production.go`：

```go
type CoreAttempt struct {
    AttemptID         string   `json:"attempt_id"`
    Reason            string   `json:"reason"`
    ProtocolVersion   string   `json:"protocol_version"`
    TaskDigest        string   `json:"task_digest"`
    CompletionNonce   string   `json:"completion_nonce"`
    RequiredArtifacts []string `json:"required_artifacts"`
    InputSource       string   `json:"input_source,omitempty"`
    InputRef          string   `json:"input_ref,omitempty"`
}
```

在 `production.go` 定义：

```go
func attemptInputSource(attempt *domain.CoreAttempt) string {
    if attempt == nil || attempt.InputSource == "" {
        return "drive"
    }
    return attempt.InputSource
}

func newAttempt(
    state *domain.CoreProductionState,
    task *domain.CoreTask,
    reason string,
    required []string,
    protocolVersion string,
) (*domain.CoreAttempt, error) {
    return newAttemptWithInput(
        state, task, reason, required, protocolVersion, "drive", "",
    )
}

func newAttemptWithInput(
    state *domain.CoreProductionState,
    task *domain.CoreTask,
    reason string,
    required []string,
    protocolVersion, inputSource, inputRef string,
) (*domain.CoreAttempt, error)
```

`newAttemptWithInput` 复用当前 nonce/排序/sequence 逻辑，并让 TaskDigest 的输入固定为：

```go
struct {
    Task              *domain.CoreTask `json:"task"`
    ProtocolVersion   string           `json:"protocol_version"`
    RequiredArtifacts []string         `json:"required_artifacts"`
    InputSource       string           `json:"input_source"`
    InputRef          string           `json:"input_ref,omitempty"`
}
```

旧 attempt 的现有 TaskDigest 不重算；只有新创建 attempt 使用新摘要格式。

- [ ] **Step 3: 让外部 submission 明确只接受 drive attempt**

在 `validateSubmissionIdentity` 和 `ScanActiveSubmission` 的入口增加：

```go
if attemptInputSource(attempt) != "drive" {
    return fmt.Errorf("active attempt does not accept Drive submission")
}
```

不要为 internal design foundation 生成 manifest/READY，也不要让猜到 attempt id 的 Drive 文件进入 settlement。

- [ ] **Step 4: 泛化现有 CoreCommitJournal，而不是新增第二套 Foundation journal**

`internal/domain/core_commit.go` 增加：

```go
Kind string `json:"kind,omitempty"`
```

兼容规则：

- 旧 journal 的 `kind==""` 按现有 chapter/revision journal 处理；
- 新 chapter/revision journal 保存 `Kind=task.Kind`；
- 新 Foundation journal 保存 `Kind="foundation"`。

修改 `recoverPendingCommit`：

```go
switch journal.Kind {
case "", "chapter", "revision":
    _, err = p.applyChapterCommit(project, journal)
case "foundation":
    _, err = p.applyFoundationCommit(project, journal)
default:
    err = fmt.Errorf("unsupported pending commit kind %q", journal.Kind)
}
return err
```

这一步不改变现有 chapter/revision journal 的持久化路径和 fault stages。

- [ ] **Step 5: 把 Foundation 直接写入重构为 prepare/apply journal**

`CoreReceipt` 增加：

```go
FoundationDesignRoot string `json:"foundation_design_root,omitempty"`
```

在 `production_part5.go` 用下面的职责替换当前直接 `acceptFoundation`：

```go
func (p *Project) prepareFoundationCommit(
    project *domain.CoreProjectState,
    state *domain.CoreProductionState,
    task *domain.CoreTask,
    attempt *domain.CoreAttempt,
    prepared preparedFoundation,
    foundationDesignRoot string,
) (*domain.CoreCommitJournal, error)

func (p *Project) applyFoundationCommit(
    project *domain.CoreProjectState,
    journal *domain.CoreCommitJournal,
) (FoundationSettlement, error)
```

`prepareFoundationCommit`：

1. 从 `prepared.Canonical` 计算 artifact digests 与 submission digest，保持当前 Foundation receipt 语义；
2. 创建 revision 1 Canon state/head；
3. receipt 在**首次构造时**写入 `FoundationDesignRoot=foundationDesignRoot`；
4. 对 state 做浅拷贝为 `next`；
5. `next.NextEntitySeq += len(prepared.Mappings)`；
6. 在 `next` 上创建 chapter:1 task + drive attempt；
7. 构造完整的 Foundation `CoreCommitJournal`，其中 `Kind="foundation"`，并写入 task/attempt/root/artifact/canon/receipt/next-state 全部字段；
8. 用现有 `SaveCorePreparedArtifacts(attempt.AttemptID, prepared.Canonical)` 保存准备产物；
9. 保存 pending journal。

`applyFoundationCommit`：

1. journal.State => applying；
2. 从 prepared artifacts 读回七文件；
3. `SaveCoreCanon`；
4. 调用现有 `maybeCommitFault("canon")`；
5. `SaveCoreReceipt`；
6. `maybeCommitFault("receipt")`；
7. `SaveCoreProductionState(&journal.NextState)`；
8. `maybeCommitFault("production")`；
9. 根据 journal 中已经固化的 `Receipt.FoundationDesignRoot` 判定来源：空值表示 legacy Drive Foundation，结算现有 submission record 并写 `exchange/result/<attempt>.json`；非空表示 required internal Foundation，不创建伪造的外部 Foundation result。恢复时不得依赖当前 production state 推断旧 attempt 来源；
10. `writeActiveAttempt(project, &journal.NextState)`，此时第一次对 ChatGPT 发布的是 chapter:1 READY；
11. `maybeCommitFault("ready")`；
12. journal.State => committed。

`settleFoundationLocked` 在 Task 4 preparation PASS 后只做：

```go
journal, err := p.prepareFoundationCommit(
    projectState, state, task, attempt, prepared, "",
)
if err != nil {
    return FoundationSettlement{}, err
}
return p.applyFoundationCommit(projectState, journal)
```

这样旧 Drive Foundation 与新设计 Foundation 共享同一 Canon/receipt/state 事务路径。

- [ ] **Step 6: 先写旧 Foundation commit recovery 回归**

在 `commit_recovery_test.go` 增加参数化 Foundation 恢复测试，对 `canon/receipt/production/ready` 每个现有 fault stage：

1. 新建 legacy test project；
2. capability ack + Reconcile 得到 Foundation READY；
3. 写合法 Foundation submission；
4. 设置 `project.commitFault` 在目标 stage 第一次返回注入错误；
5. settlement 失败；
6. `OpenProject(local)`；
7. `Reconcile()`；
8. 断言只有一份 Foundation ACCEPTED receipt、一个 Canon root、一个 chapter:1 active attempt；
9. 再 `Reconcile()` 一次仍完全相同。

这一步先证明 journal 泛化没有只服务新设计路径。

- [ ] **Step 7: 从 frozen Design Head 创建唯一内部 Foundation attempt**

在 `production.go` 实现：

```go
func newDesignFoundationAttempt(
    state *domain.CoreProductionState,
    task *domain.CoreTask,
    designRoot, protocolVersion string,
) (*domain.CoreAttempt, error) {
    return newAttemptWithInput(
        state,
        task,
        "design-foundation",
        foundationArtifactNames,
        protocolVersion,
        "design",
        designRoot,
    )
}
```

`ensureDesignFoundationAttemptLocked` 固定：

1. 只接受 `head.Checkpoint==foundation_ready`；
2. 如果 state 没有 active task/attempt，创建唯一 foundation task + design attempt 并先保存 production state；
3. 如果已有 active task/attempt，必须恰好是 foundation + `InputSource=design` + `InputRef=head.DesignRoot`，否则 fail closed；
4. 从 frozen design commit/bundle 读取七文件；
5. 调用现有 `SaveCoreSnapshot(attempt.AttemptID, files)`；已存在相同 snapshot 幂等，不同内容报错；
6. **不**调用 `writeActiveAttempt`。

在 `design_test_helpers_test.go` 增加：

```go
func makeFoundationReadyProjectForTest(
    t *testing.T,
) (*Project, string, string, foundationDesignFixture)

func foundationAcceptedReceiptsForTest(
    receipts []domain.CoreReceipt,
) []domain.CoreReceipt
```

`makeFoundationReadyProjectForTest` 只通过真实 Design import/promote API 得到 foundation_ready，不调用 Reconcile 自动结算；它必须把成功 promote 返回的 `NewDesignRoot` 写进 `fixture.FoundationRoot`，并保留最终 bundle/review ref。

`foundationAcceptedReceiptsForTest` 只筛选 `Result=="ACCEPTED" && PreviousRoot==""` 的第一份 Foundation receipt；legacy receipt 的 `FoundationDesignRoot` 可以为空，required receipt 必须非空。

- [ ] **Step 8: 实现 internal Foundation settlement**

`settleDesignFoundationAttemptLocked`：

1. 读取 project/state/head；
2. 验证 required、pre-Canon、head=foundation_ready；
3. active task=foundation；
4. `attemptInputSource(attempt)=="design"`；
5. `attempt.InputRef==head.DesignRoot`；
6. 从本地 Core snapshot 精确读取 `foundationArtifactNames` 七文件；
7. 调用 `prepareFoundationArtifacts`；
8. 如果出现 err 或 violations，返回 integrity error；**不得**生成 rewrite attempt，因为 Foundation Ready 已经证明过同一七文件；
9. 调用 `p.prepareFoundationCommit(project, state, task, attempt, prepared, head.DesignRoot)`；
10. 调用 `applyFoundationCommit`。

在 required pre-Canon reconcile 中：

- head nil/story_locked => 不创建 Foundation task；
- head foundation_ready => 建/恢复 internal attempt，随后自动 settlement；
- 设计 Foundation 结算完成后继续由 journal.NextState 的 chapter:1 READY 接回现有生产链。

- [ ] **Step 9: 写 internal Foundation 的崩溃恢复测试**

```go
func TestDesignFoundationCommitRecoversAfterCanonFault(t *testing.T) {
    project, local, workspace, fixture := makeFoundationReadyProjectForTest(t)

    fired := false
    project.commitFault = func(stage string) error {
        if stage == "canon" && !fired {
            fired = true
            return errors.New("injected foundation canon crash")
        }
        return nil
    }

    if err := project.Reconcile(); err == nil {
        t.Fatal("expected injected crash")
    }

    if _, err := os.Stat(filepath.Join(workspace, "exchange", "READY.json")); !os.IsNotExist(err) {
        t.Fatalf("internal foundation leaked READY before recovery: %v", err)
    }

    reopened, err := OpenProject(local)
    if err != nil {
        t.Fatal(err)
    }
    if err := reopened.Reconcile(); err != nil {
        t.Fatal(err)
    }
    if err := reopened.Reconcile(); err != nil {
        t.Fatal(err)
    }

    head, err := reopened.store.LoadCoreDesignHead()
    if err != nil {
        t.Fatal(err)
    }
    if head == nil || head.DesignRoot != fixture.FoundationRoot {
        t.Fatalf("design head=%+v fixture=%+v", head, fixture)
    }

    receipts, err := reopened.store.ListCoreReceipts()
    if err != nil {
        t.Fatal(err)
    }
    accepted := foundationAcceptedReceiptsForTest(receipts)
    if len(accepted) != 1 {
        t.Fatalf("foundation accepted receipts=%v", accepted)
    }
    if accepted[0].FoundationDesignRoot != fixture.FoundationRoot {
        t.Fatalf("receipt root=%q", accepted[0].FoundationDesignRoot)
    }

    ready := readReady(t, workspace)
    if ready.TaskKind != "chapter" || ready.Target != "chapter:1" {
        t.Fatalf("READY=%+v", ready)
    }
}
```

同一参数化测试至少再覆盖 `receipt/production/ready` fault stage，确保不会产生第二份 Foundation receipt / Canon / chapter attempt。

- [ ] **Step 10: 跑 Foundation、commit recovery 与 chapter 回归**

```bash
go test ./internal/core -run 'TestAttemptInputSource|TestFoundation|TestDesignFoundation|TestCommitRecovery|TestChapter' -v
```

Expected: PASS。现有 chapter/revision commit recovery 必须保持绿色。

- [ ] **Step 11: 提交**

```bash
git add internal/domain/core_production.go internal/domain/core_commit.go internal/core/production.go internal/core/production_part2.go internal/core/production_part5.go internal/core/commit.go internal/core/submission.go internal/store/core_submission.go internal/core/design_test_helpers_test.go internal/core/foundation_test.go internal/core/commit_recovery_test.go internal/core/serve_test.go
git commit -m "feat: settle approved designs through foundation canon"
git push origin provider-free-novel-core
```

---

### Task 6: 激活 required/legacy、新协议与显式迁移

**Files:**
- Modify: `internal/core/init.go`
- Modify: `internal/core/project.go`
- Modify: `internal/core/production.go`
- Modify: `internal/core/serve.go`
- Modify: `internal/core/migration.go`
- Modify: `internal/core/lock.go`
- Modify: `internal/protocol/protocol.go`
- Modify: `internal/protocol/chatgpt_protocol_part1.go`
- Modify: `internal/protocol/chatgpt_protocol_part2.go`
- Modify: `internal/protocol/chatgpt_protocol_part3.go`
- Modify: `internal/core/init_test.go`
- Modify: `internal/core/migration_test.go`
- Modify: `internal/core/serve_test.go`
- Modify: `internal/protocol/protocol_test.go`

**Interfaces:**
- Consumes: Tasks 1–5 已完成但尚未默认启用的 Design 能力。
- Produces: core schema 2、protocol 1.1、新项目 required、升级项目 legacy、无 active task 的 required idle lifecycle、Design Inbox serve 扫描、唯一 STATUS 设计投影。

- [ ] **Step 1: 写 fresh required 生命周期失败测试**

修改现有 `TestWorkspaceStatusTracksCapabilityAndActiveAuthority`：capability 通过后的新项目不再期待 foundation READY，而是：

```go
if readyStatus["capability"] != "passed" {
    t.Fatalf("capability=%v", readyStatus["capability"])
}
if readyStatus["design_mode"] != "required" {
    t.Fatalf("design_mode=%v", readyStatus["design_mode"])
}
if readyStatus["active_task_kind"] != nil ||
    readyStatus["active_attempt_id"] != nil {
    t.Fatalf(
        "required project created production task before foundation ready: %+v",
        readyStatus,
    )
}
if _, err := os.Stat(
    filepath.Join(workspace, "exchange", "READY.json"),
); !os.IsNotExist(err) {
    t.Fatalf("required project must not publish READY before canon: %v", err)
}
```

新增：

```go
func TestNewProjectStartsInRequiredDesignModeWithoutProductionTask(t *testing.T) {
    local := t.TempDir()
    workspace := t.TempDir()
    project, err := InitProject(InitOptions{
        ProjectID: "required-new",
        LocalRoot: local,
        WorkspaceRoot: workspace,
    })
    if err != nil {
        t.Fatal(err)
    }

    writeCapabilityAckForTest(t, workspace)
    if err := project.Reconcile(); err != nil {
        t.Fatal(err)
    }

    state, err := project.store.LoadCoreProjectState()
    if err != nil {
        t.Fatal(err)
    }
    if state.DesignMode != domain.DesignModeRequired {
        t.Fatalf("design_mode=%q", state.DesignMode)
    }

    production, err := project.store.LoadCoreProductionState()
    if err != nil {
        t.Fatal(err)
    }
    if production != nil {
        t.Fatalf("required idle project created production state: %+v", production)
    }
}
```

`writeCapabilityAckForTest` 从现有 `newCapabilityPassedProject` 中提取 ack/probe 写入逻辑，后续测试共用；它不调用 Reconcile。

- [ ] **Step 2: 提升 core/protocol 版本并让新 Init 固定 required**

`internal/core/init.go`：

```go
const coreSchemaVersion = 2
```

`internal/protocol/protocol.go`：

```go
const (
    LegacyVersion   = "0.9"
    PreviousVersion = "1.0"
    CurrentVersion  = "1.1"
)
```

`IsKnownVersion` 明确接受 0.9 / 1.0 / 1.1。

fresh `CoreProjectState` 写：

```go
DesignMode: domain.DesignModeRequired,
```

workspace 初始化增加：

```text
exchange/design/inbox
exchange/design/result
```

此时删除 `newRequiredDesignProjectForTest` 中 Task 6 之前的手工 DesignMode 覆盖，并断言 fresh Init 已经是 required。

- [ ] **Step 3: 把 schema migration 改成显式 0|1 → 2，并写 design_mode=legacy**

`Migrate` 不再固定查 `0 -> coreSchemaVersion` receipt。若 `state.SchemaVersion != coreSchemaVersion`：

```go
schemaReceipt, err := p.store.LoadCoreMigrationReceipt(
    state.SchemaVersion,
    coreSchemaVersion,
)
```

`migrateSchemaLocked` 只接受 from schema 0 或 1，receipt.FromSchema 使用真实旧版本；成功时：

```go
state.SchemaVersion = coreSchemaVersion
state.DesignMode = domain.DesignModeLegacy
```

`finishPreparedSchemaMigration` 不再写死 FromSchema=0，只要求：

- receipt.State=prepared；
- receipt.ToSchema=2；
- receipt.FromSchema 是 0 或 1；
- 当前 state.SchemaVersion=2；
- 备份按 receipt.FromSchema 验证；
- Canon root 前后相同。

`acquireProjectMutationLock` 对 target schema 2 的 prepared receipt 显式检查两个可能来源：

```go
for _, from := range []int{0, 1} {
    receipt, err := p.store.LoadCoreMigrationReceipt(
        from, coreSchemaVersion,
    )
    // prepared => require recovery
}
```

在 `migration_test.go` 保留现有 schema 0 case，并复制其真实文件编辑模式新增 schema 1 case。schema 1 case必须记录 migration 前的 `CoreProductionState` JSON bytes，迁移后重新读取并比较完全相同；只允许 `CoreProjectState.SchemaVersion/DesignMode` 改变。


当前正式发布项目同时需要 schema 1→2 和 protocol 1.0→1.1。本期不把 `Migrate` 改造成通用迁移编排器：一次调用仍只提交一种 migration receipt。升级操作明确分两段，并使用两个独立预迁移备份目录：

```bash
novel-core migrate --project <project> --backup <backup-before-schema-2>
novel-core migrate --project <project> --backup <backup-before-protocol-1.1>
```

第一段完成后项目处于 schema 2 / protocol 1.0 / design_mode=legacy，生产 mutation 仍因旧 protocol fail closed；第二段才进入 schema 2 / protocol 1.1。不能复用同一个 backup 路径，因为第二段备份必须精确对应 schema 已升级、protocol 尚未升级的中间状态。

- [ ] **Step 4: 把 protocol migration 改成显式 0.9|1.0 → 1.1，保留 replacement-attempt 安全语义**

若 state.ProtocolVersion != 1.1，receipt lookup 使用：

```go
p.store.LoadCoreProtocolMigrationReceipt(
    state.ProtocolVersion,
    protocol.CurrentVersion,
)
```

`migrateProtocolLocked` 只接受 0.9 或 1.0。继续复用当前实现：

- verified pre-migration backup；
- 新 capability nonce/probe；
- 若有 active production attempt，为**同一 active task**创建 reason=`protocol_upgrade` 的 replacement attempt；
- replacement attempt 绑定 1.1，task kind/target/base root/constraints 不变；
- Canon root 不变；
- 删除旧 READY，等待新 capability ack 后为 replacement attempt 重发 READY；
- DesignMode 保持 legacy。

不要修改 spec 去要求 protocol migration 保留旧 attempt ID；本规格已经明确沿用当前安全重绑行为。

`acquireProjectMutationLock` 同样显式检查：

```go
for _, from := range []string{
    protocol.LegacyVersion,
    protocol.PreviousVersion,
} {
    receipt, err := p.store.LoadCoreProtocolMigrationReceipt(
        from, protocol.CurrentVersion,
    )
    // prepared => require recovery
}
```

现有 `TestMigrateKnownLegacyProtocolPreservesCanonAndRebindsAttempt` table 化为 0.9 和 1.0 两个 from 版本，断言 task ID/target/base canon 保持、attempt ID 更新、Canon 不变。


再增加一个当前正式发布组合的两段迁移测试。不要伪造 migration internals；按现有测试方式直接把 fixture 的 project metadata 降为 schema 1 / protocol 1.0，然后：

```go
func TestCurrentReleaseMigratesInTwoExplicitStages(t *testing.T) {
    project, _, _, ready := acceptedFoundationProject(t)

    state, err := project.store.LoadCoreProjectState()
    if err != nil {
        t.Fatal(err)
    }
    state.SchemaVersion = 1
    state.ProtocolVersion = protocol.PreviousVersion
    state.DesignMode = ""
    if err := project.store.SaveCoreProjectState(state); err != nil {
        t.Fatal(err)
    }

    before, err := project.store.LoadCoreProductionState()
    if err != nil {
        t.Fatal(err)
    }
    beforeTaskID := before.ActiveTask.TaskID
    beforeTarget := before.ActiveTask.Target
    beforeRoot := before.CanonRoot

    schemaBackup := filepath.Join(t.TempDir(), "before-schema-2")
    first, err := project.Migrate(schemaBackup)
    if err != nil {
        t.Fatal(err)
    }
    if first.FromSchema != 1 || first.ToSchema != 2 {
        t.Fatalf("schema migration=%+v", first)
    }

    middle, err := project.store.LoadCoreProjectState()
    if err != nil {
        t.Fatal(err)
    }
    if middle.DesignMode != domain.DesignModeLegacy ||
        middle.ProtocolVersion != protocol.PreviousVersion {
        t.Fatalf("middle state=%+v", middle)
    }

    protocolBackup := filepath.Join(t.TempDir(), "before-protocol-1-1")
    second, err := project.Migrate(protocolBackup)
    if err != nil {
        t.Fatal(err)
    }
    if second.FromProtocol != protocol.PreviousVersion ||
        second.ToProtocol != protocol.CurrentVersion {
        t.Fatalf("protocol migration=%+v", second)
    }

    after, err := project.store.LoadCoreProductionState()
    if err != nil {
        t.Fatal(err)
    }
    if after.ActiveTask.TaskID != beforeTaskID ||
        after.ActiveTask.Target != beforeTarget ||
        after.CanonRoot != beforeRoot {
        t.Fatalf("migration changed business progress: before=%+v after=%+v", before, after)
    }
    if after.ActiveAttempt.AttemptID == ready.AttemptID {
        t.Fatal("protocol migration did not rebind active attempt")
    }
}
```

- [ ] **Step 5: 重排 reconcile，让 required idle 不创建 production state**

capability passed 后先读取 production + Design HEAD，但不要立刻 `newCoreProductionState`：

```go
state, err := p.store.LoadCoreProductionState()
if err != nil {
    return err
}
head, err := p.store.LoadCoreDesignHead()
if err != nil {
    return err
}

if projectState.DesignMode == domain.DesignModeRequired {
    if state == nil || state.CanonRoot == "" {
        if head == nil ||
            head.Checkpoint != domain.DesignCheckpointFoundationReady {
            return p.writeWorkspaceStatus(projectState, state)
        }
        if state == nil {
            state = newCoreProductionState()
        }
        if err := p.ensureDesignFoundationAttemptLocked(
            projectState, state, head,
        ); err != nil {
            return err
        }
        if _, err := p.settleDesignFoundationAttemptLocked(); err != nil {
            return err
        }
        state, err = p.store.LoadCoreProductionState()
        if err != nil {
            return err
        }
        return p.writeWorkspaceStatus(projectState, state)
    }
}
```

只有 legacy 分支继续执行旧：

```go
if state == nil {
    state = newCoreProductionState()
}
if state.ActiveTask == nil || state.ActiveAttempt == nil {
    // create external Foundation task/attempt
}
```

required + story_locked/no head 时没有 production state 仍是正常状态。

- [ ] **Step 6: 在 Serve 中加入 locked Design Inbox 扫描**

新增：

```go
var designSubmissionIDPattern =
    regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,127}$`)

func (p *Project) serveDesignInboxLocked(project *domain.CoreProjectState) error
```

它只在：

```text
design_mode=required
capability=passed
Canon 为空
HEAD 尚未 foundation-ready/frozen
```

时扫描 `exchange/design/inbox`；跳过 symlink/non-directory/非法 submission id；按 id 排序；每个 id 调 `scanDesignSubmissionLocked`，READY_TO_VALIDATE 时调 `processDesignSubmissionLocked`。

`servePassLocked` 顺序固定：

1. `reconcileLocked`；
2. load project/production；
3. 如果 required 且 Canon 为空，调用 `serveDesignInboxLocked`；
4. 再 `reconcileLocked` 一次，让刚得到 foundation_ready 的项目走 Task 5 Foundation journal；
5. 重新 load production；
6. 没 active attempt 则本轮结束；
7. 有 Canon 后才走 controls / active production submission / projection。

新增 Serve 测试：启动 required idle Serve，稍后写一个合法 design import；无需文件事件，后续 scan 必须生成 result，同时仍没有 READY。

- [ ] **Step 7: 扩展唯一 STATUS，只投影事实，不再造 foundation_state**

`Status` 与 `workspaceStatus` 增加：

```go
DesignMode           string `json:"design_mode,omitempty"`
DesignHead           string `json:"design_head,omitempty"`
DesignCheckpoint     string `json:"design_checkpoint,omitempty"`
FoundationDesignRoot string `json:"foundation_design_root,omitempty"`
```

投影规则：

- DesignMode 直接来自 CoreProjectState；
- required 且有 HEAD：DesignHead/DesignCheckpoint 来自 HEAD；
- pre-Canon foundation_ready：DesignCheckpoint 已足够表达“已建书就绪”，FoundationDesignRoot 仍为空；
- Foundation ACCEPTED 后，从非空 `CoreReceipt.FoundationDesignRoot` 投影 FoundationDesignRoot；
- legacy：DesignHead/Checkpoint/FoundationDesignRoot 为空；
- CanonRoot/ActiveTask 继续来自 production state。

不要新增 foundation_state 或任何流程阶段游标。required accepted 项目的 receipt/head 交叉一致性由 Task 7 Verify 负责；STATUS 不自己修复不一致。

- [ ] **Step 8: 更新生成的 CHATGPT_PROTOCOL.md**

协议正文必须真实描述两种模式和 machine schema，不维护第二份手写协议。至少加入以下 JSON 示例，继续由现有 `TestChatGPTProtocolJSONExamplesAreValid` 自动解析：

```json
{
  "schema_version": 1,
  "project_id": "book-1",
  "submission_id": "design-001",
  "protocol_version": "1.1",
  "operation": "import",
  "files": ["concept.json"]
}
```

```json
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
```

并明确新会话恢复顺序：

```text
project.json
→ CHATGPT_PROTOCOL.md
→ exchange/STATUS.json
→ 如果 design_mode=required 且还没有 Canon：按 Design Head/候选继续，不读取 READY
→ 如果 STATUS 有 active production attempt：读取 READY + outbox
```

协议测试至少搜索：`design_mode`、`exchange/design/inbox`、`story_locked`、`foundation_ready`、`author_confirmation`、`foundation_readiness_review`、`expected_design_root`、“没有 READY”。

- [ ] **Step 9: 跑 activation / migration / protocol 测试**

```bash
go test ./internal/core -run 'TestNewProject|TestWorkspaceStatus|TestServe|TestMigrate|TestSchema|TestLegacyProtocol|TestPreparedProtocol' -v
go test ./internal/protocol -v
```

Expected: PASS。

- [ ] **Step 10: 提交**

```bash
git add internal/core/init.go internal/core/project.go internal/core/production.go internal/core/serve.go internal/core/migration.go internal/core/lock.go internal/protocol/protocol.go internal/protocol/chatgpt_protocol_part1.go internal/protocol/chatgpt_protocol_part2.go internal/protocol/chatgpt_protocol_part3.go internal/core/init_test.go internal/core/migration_test.go internal/core/serve_test.go internal/protocol/protocol_test.go
git commit -m "feat: activate semantic design for new projects"
git push origin provider-free-novel-core
```

---

### Task 7: 把 Design Store 纳入 verify / backup / restore / security

**Files:**
- Modify: `internal/core/project.go`
- Modify: `internal/core/backup.go`
- Modify: `internal/store/core_design.go`
- Modify: `internal/core/design_test_helpers_test.go`
- Modify: `internal/core/verify_integrity_test.go`
- Modify: `internal/core/backup_test.go`
- Modify: `internal/core/security_test.go`

**Interfaces:**
- Consumes: active/frozen Design Store、Design receipts、Foundation receipt。
- Produces:
  - `(*CoreStore).ListCoreDesignArtifactDigests() ([]string, error)`
  - `(*CoreStore).ListCoreDesignBundleDigests() ([]string, error)`
  - `internal/store/core_design.go` 内定义 `type CoreDesignCommitEntry struct { Root string; Commit domain.CoreDesignCommit }`
  - `(*CoreStore).ListCoreDesignCommits() ([]CoreDesignCommitEntry, error)`；
  - `(*CoreStore).ListCoreDesignReceipts() ([]domain.CoreDesignReceipt, error)`
  - `verifyDesignStore(project *domain.CoreProjectState) []string`
  - backup 前 Design integrity gate；restore 复用现有最终 Verify。

- [ ] **Step 1: 建立 accepted required fixture 并写 object tamper 测试**

在 `design_test_helpers_test.go` 增加：

```go
func acceptedRequiredDesignProjectForTest(
    t *testing.T,
) (*Project, string, string, string) {
    t.Helper()
    project, local, workspace, fixture :=
        makeFoundationReadyProjectForTest(t)
    if err := project.Reconcile(); err != nil {
        t.Fatal(err)
    }
    status, err := project.Status()
    if err != nil {
        t.Fatal(err)
    }
    if status.CanonRoot == "" {
        t.Fatal("required project did not create first canon")
    }
    return project, local, workspace, fixture.FoundationRoot
}

func currentStoryConceptRefForTest(
    t *testing.T,
    p *Project,
) string {
    t.Helper()
    head, err := p.store.LoadCoreDesignHead()
    if err != nil || head == nil {
        t.Fatalf("design head=%+v err=%v", head, err)
    }
    commit, err := p.store.LoadCoreDesignCommit(head.DesignRoot)
    if err != nil || commit == nil {
        t.Fatalf("design commit=%+v err=%v", commit, err)
    }
    bundle, err := p.loadDesignBundleRef(commit.BundleRef)
    if err != nil {
        t.Fatal(err)
    }
    ref := bundle.Selections["story_concept"]
    if ref == "" {
        t.Fatal("story_concept is missing")
    }
    return ref
}
```

在 `verify_integrity_test.go` 增加：

```go
func TestVerifyDetectsDesignArtifactTamper(t *testing.T) {
    project, local, _, _ := acceptedRequiredDesignProjectForTest(t)
    ref := currentStoryConceptRefForTest(t, project)
    _, digest, err := parseDesignArtifactRef(ref)
    if err != nil {
        t.Fatal(err)
    }

    path := filepath.Join(
        local, "meta", "core", "design", "objects", digest+".json",
    )
    if err := os.WriteFile(
        path, []byte(`{"tampered":true}`), 0o644,
    ); err != nil {
        t.Fatal(err)
    }

    verification, err := project.Verify()
    if err != nil {
        t.Fatal(err)
    }
    if verification.OK ||
        !containsProblem(verification.Problems, "design artifact") {
        t.Fatalf("verification=%+v", verification)
    }
}
```

同文件再写 bundle bytes、commit bytes、HEAD 指向不存在 root、Foundation receipt.FoundationDesignRoot 改成另一个合法/非法 root 四类 tamper。

- [ ] **Step 2: 为 Design Store 增加只读枚举，不增加第二套索引**

`internal/store/core_design.go` 的 List 方法只枚举现有目录中的 `.json` regular files，拒绝目录项 symlink/non-regular，不维护额外 index：

```text
objects/*.json
bundles/*.json
commits/*.json
receipts/*.json
```

artifact/bundle 列表返回文件名中的 digest；commit 列表必须同时返回文件名 root 和解码后的 commit，Verify 才能比较“文件名 root == 重算 root”。

所有结果排序，保证验证错误稳定。

- [ ] **Step 3: 实现 verifyDesignStore 并接入 Project.Verify**

required 项目检查：

1. 枚举**所有** objects：读取后 canonicalize，要求文件名 digest == 重算 digest，artifact_ref 中 artifact_type 与对象一致；
2. 每个 artifact.inputs/sources/supersedes 非空 ref 都必须存在；supersedes 的 artifact_type 必须相同；
3. 枚举**所有** bundles：重算 bundle_ref；每个 selection ref 存在且 artifact_type == slot；调用 `validateBundleClosure`；
4. 枚举**所有** commits：重算 design_root 必须等于文件名；bundle/evidence refs 存在；checkpoint 只允许 story_locked/foundation_ready；parent 非空时对应 commit 存在；
5. HEAD 为空仅在 Canon 为空时允许；HEAD 非空必须指向真实 commit，HEAD.Checkpoint == commit.Checkpoint；
6. 从 HEAD 沿 parent 逆行，检测 cycle/断链；所有 commit 文件都必须在这条正式线性历史上，避免落下幽灵正式 commit；
7. 所有 Design receipt 的 submission_id 与文件名一致；PROMOTED receipt 的 NewDesignRoot 必须存在，PreviousDesignRoot 必须等于该 commit.ParentDesignRoot；
8. Canon 为空时，不要求 Foundation receipt；
9. required 且 Canon 已存在：HEAD checkpoint 必须 foundation_ready；必须恰好找到一个 ACCEPTED Foundation receipt 的 `FoundationDesignRoot` 非空并等于 HEAD.DesignRoot；
10. 不从 Drive 自动重建任何缺失对象。

legacy 项目：

- 没有 Design Store 完全合法；
- 候选 objects/bundles 若意外存在仍按摘要完整性检查，但不成为权威；
- HEAD 或 commits 一旦存在，报 `legacy project unexpectedly contains authoritative design history`；
- Foundation receipt.FoundationDesignRoot 必须为空。

`Project.Verify` 在现有 Canon/receipt chain 检查后 执行 `problems = append(problems, verifyDesignStore(projectState)...)`。

- [ ] **Step 4: backup 只增加前置 Design 校验，继续复制整个 meta/core**

`createBackupUnlocked` 在创建 staging 目录之前增加：

```go
if problems := p.verifyDesignStore(projectState); len(problems) > 0 {
    return BackupResult{}, fmt.Errorf(
        "verify design store before backup: %s",
        strings.Join(problems, "; "),
    )
}
```

不要新增 Design 文件收集器，也不要把 design_root 再复制进 backup manifest；现有 `copyCoreBackupPayload` 已递归复制整个 `meta/core`。

- [ ] **Step 5: 用现有 RestoreBackup → Verify 钩子证明双根恢复**

当前 `RestoreBackup` 已经在最终 rename 前：

```go
verification, err := staged.Verify()
```

因此**不修改 restore 流程**；Task 7 只让新的 `Verify` 自然覆盖 Design Store。

新增：

```go
func TestBackupRestoreRoundTripPreservesDesignAndCanonRoots(t *testing.T) {
    project, _, _, designRoot :=
        acceptedRequiredDesignProjectForTest(t)
    before, err := project.Status()
    if err != nil {
        t.Fatal(err)
    }

    backupDir := filepath.Join(t.TempDir(), "backup")
    if _, err := project.CreateBackup(backupDir); err != nil {
        t.Fatal(err)
    }
    restored, err := RestoreBackup(
        backupDir,
        filepath.Join(t.TempDir(), "restored"),
    )
    if err != nil {
        t.Fatal(err)
    }

    after, err := restored.Status()
    if err != nil {
        t.Fatal(err)
    }
    if before.CanonRoot != after.CanonRoot ||
        after.FoundationDesignRoot != designRoot {
        t.Fatalf("before=%+v after=%+v", before, after)
    }
    verification, err := restored.Verify()
    if err != nil || !verification.OK {
        t.Fatalf("verification=%+v err=%v", verification, err)
    }
}
```

再增加：篡改 backup payload 内某个 Design object 且同步修改 backup manifest 使文件级 digest 看似一致，Restore 仍必须因为 Design content address 不匹配而拒绝。这证明不是只靠 backup manifest。

- [ ] **Step 6: 补齐 Design Inbox 安全边界**

在 `security_test.go` / `design_submission_test.go` 真实创建并测试：

- absolute path；
- `..` path traversal；
- symlink file；
- nested directory；
- manifest 未声明文件；
- invalid UTF-8；
- JSON depth > `MaxJSONDepth`；
- array length > `MaxJSONArrayLength`；
- 总提交 > 32 MiB；
- 同 submission 第一次锁定 snapshot 后 Drive 内容改变；
- 非法 submission id 目录不被 Serve 执行。

这些测试必须走 `ScanDesignSubmission` 或 Serve，不直接调用 `protocol.ReadUTF8` 代替端到端边界。

- [ ] **Step 7: 跑 verify/backup/security 回归**

```bash
go test ./internal/core -run 'TestVerify|TestBackup|TestRestore|TestDesign.*Reject|TestDesign.*Tamper|TestServe.*Design' -v
```

Expected: PASS。

- [ ] **Step 8: 提交**

```bash
git add internal/core/project.go internal/core/backup.go internal/store/core_design.go internal/core/design_test_helpers_test.go internal/core/verify_integrity_test.go internal/core/backup_test.go internal/core/security_test.go internal/core/design_submission_test.go
git commit -m "feat: verify and back up semantic design authority"
git push origin provider-free-novel-core
```

---

### Task 8: 完整 E2E、用户文档与发布门禁

**Files:**
- Modify: `internal/core/protocol_chain_e2e_test.go`
- Modify: `internal/core/design_test.go`
- Modify: `cmd/novel-core/main_test.go`
- Modify: `cmd/novel-core/docs_contract_test.go`
- Modify: `README.md`
- Modify: `docs/provider-free-release-checklist.md`
- Modify: `docs/superpowers/specs/2026-09-23-provider-free-semantic-design-baseline.md` only if implementation exposes a genuine requirement ambiguity; never loosen the spec merely to make a failing implementation pass.

**Interfaces:**
- Consumes: Tasks 1–7 全部接口。
- Produces: fresh init → real Drive-shaped Design submissions → story_locked → foundation_ready → internal Foundation journal → chapter:1 READY 的自动 E2E；用户文档；本地发布门禁；真实 ChatGPT App + Drive 产品验收入口。

- [ ] **Step 1: 在 protocol E2E 中增加真实 Design Drive helper**

复用文件内已有 `writeProtocolJSON`，新增：

```go
func writeProtocolDesignSubmission(
    t *testing.T,
    p *Project,
    workspace string,
    submissionID string,
    operation string,
    files map[string]any,
) {
    t.Helper()
    status, err := p.Status()
    if err != nil {
        t.Fatal(err)
    }

    dir := filepath.Join(
        workspace, "exchange", "design", "inbox", submissionID,
    )
    if err := os.MkdirAll(dir, 0o755); err != nil {
        t.Fatal(err)
    }

    names := make([]string, 0, len(files))
    for name, value := range files {
        names = append(names, name)
        writeProtocolJSON(t, filepath.Join(dir, name), value)
    }
    sort.Strings(names)

    // manifest last
    writeProtocolJSON(t, filepath.Join(dir, "manifest.json"),
        protocol.DesignManifest{
            SchemaVersion: protocol.MachineSchemaVersion,
            ProjectID: status.ProjectID,
            SubmissionID: submissionID,
            ProtocolVersion: protocol.CurrentVersion,
            Operation: operation,
            Files: names,
        },
    )
}

func settleProtocolDesignSubmission(
    t *testing.T,
    p *Project,
    submissionID string,
) domain.CoreDesignSubmissionResult {
    t.Helper()
    var record domain.CoreDesignSubmissionRecord
    for i := 0; i < 2; i++ {
        got, err := p.ScanDesignSubmission(submissionID)
        if err != nil {
            t.Fatal(err)
        }
        record = got
    }
    if record.State != "READY_TO_VALIDATE" {
        t.Fatalf("design record=%+v", record)
    }
    result, err := p.ProcessDesignSubmission(submissionID)
    if err != nil {
        t.Fatal(err)
    }
    return result
}
```

再增加三个薄 wrapper；它们全部通过上面 Drive 路径，不直接调用 canonical/store：

```go
func driveImportArtifactForProtocolChain(
    t *testing.T,
    p *Project,
    workspace, submissionID, artifactType string,
    inputs []string,
    payload any,
) string {
    t.Helper()
    writeProtocolDesignSubmission(
        t, p, workspace, submissionID, "import",
        map[string]any{
            "artifact.json": map[string]any{
                "kind": "artifact",
                "artifact": map[string]any{
                    "schema_version": 1,
                    "artifact_type": artifactType,
                    "inputs": inputs,
                    "sources": []string{},
                    "payload": payload,
                },
            },
        },
    )
    result := settleProtocolDesignSubmission(t, p, submissionID)
    if result.Result != "IMPORTED" {
        t.Fatalf("design artifact import=%+v", result)
    }
    ref := result.Refs["artifact.json"]
    if ref == "" {
        t.Fatalf("design artifact import returned no ref: %+v", result)
    }
    return ref
}

func driveImportBundleForProtocolChain(
    t *testing.T,
    p *Project,
    workspace, submissionID string,
    selections map[string]string,
) string {
    t.Helper()
    writeProtocolDesignSubmission(
        t, p, workspace, submissionID, "import",
        map[string]any{
            "bundle.json": map[string]any{
                "kind": "bundle",
                "bundle": map[string]any{
                    "schema_version": 1,
                    "selections": selections,
                },
            },
        },
    )
    result := settleProtocolDesignSubmission(t, p, submissionID)
    if result.Result != "IMPORTED" {
        t.Fatalf("design bundle import=%+v", result)
    }
    ref := result.Refs["bundle.json"]
    if ref == "" {
        t.Fatalf("design bundle import returned no ref: %+v", result)
    }
    return ref
}

func drivePromoteForProtocolChain(
    t *testing.T,
    p *Project,
    workspace, submissionID, expectedRoot, checkpoint, bundleRef string,
    evidence map[string]string,
) domain.CoreDesignSubmissionResult {
    t.Helper()
    status, err := p.Status()
    if err != nil {
        t.Fatal(err)
    }
    writeProtocolDesignSubmission(
        t, p, workspace, submissionID, "promote",
        map[string]any{
            "promote.json": protocol.DesignPromoteRequest{
                SchemaVersion: protocol.MachineSchemaVersion,
                ProjectID: status.ProjectID,
                SubmissionID: submissionID,
                ProtocolVersion: protocol.CurrentVersion,
                ExpectedDesignRoot: expectedRoot,
                Checkpoint: checkpoint,
                BundleRef: bundleRef,
                Evidence: evidence,
            },
        },
    )
    return settleProtocolDesignSubmission(t, p, submissionID)
}
```

Import wrapper 必须断言 Core 返回 `IMPORTED` 和非空 ref；promote wrapper 不预先断言结果，这样同一个 helper 既能用于成功 E2E，也能用于 STALE/INVALID 负例。

- [ ] **Step 2: 写完整 fresh required E2E**

新增：

```go
func TestSemanticDesignProtocolChainCreatesFirstCanonWithoutFoundationTaskReady(
    t *testing.T,
) {
    local := t.TempDir()
    workspace := t.TempDir()
    project, err := InitProject(InitOptions{
        ProjectID: "semantic-e2e",
        LocalRoot: local,
        WorkspaceRoot: workspace,
    })
    if err != nil {
        t.Fatal(err)
    }
    writeProtocolCapabilityAck(t, workspace)
    project.submissionQuietPeriod = 0

    if err := project.Reconcile(); err != nil {
        t.Fatal(err)
    }
    if _, err := os.Stat(
        filepath.Join(workspace, "exchange", "READY.json"),
    ); !os.IsNotExist(err) {
        t.Fatalf("READY exists before Canon: %v", err)
    }

    briefRef := driveImportArtifactForProtocolChain(
        t, project, workspace, "e2e-brief", "creative_brief",
        nil, validCreativeBriefPayload(),
    )
    decisionsRef := driveImportArtifactForProtocolChain(
        t, project, workspace, "e2e-decisions", "story_decisions",
        nil, validStoryDecisionsPayload(),
    )
    conceptRef := driveImportArtifactForProtocolChain(
        t, project, workspace, "e2e-concept", "story_concept",
        []string{briefRef, decisionsRef},
        validStoryConceptPayload(),
    )
    storyBundle := driveImportBundleForProtocolChain(
        t, project, workspace, "e2e-story-bundle",
        map[string]string{
            "creative_brief": briefRef,
            "story_decisions": decisionsRef,
            "story_concept": conceptRef,
        },
    )
    storyReview := driveImportArtifactForProtocolChain(
        t, project, workspace, "e2e-story-review", "story_review", nil,
        map[string]any{
            "review_type": "story_review",
"subject_ref": conceptRef,
            "policy_version": 1,
            "verdict": "PASS",
            "findings": []any{},
            "created_from_context_refs": []any{briefRef, decisionsRef},
        },
    )
    authorConfirmation := driveImportArtifactForProtocolChain(
        t, project, workspace, "e2e-author-confirm", "author_confirmation", nil,
        map[string]any{
            "subject_ref": conceptRef,
            "decision": "APPROVED",
        },
    )
    storyLocked := drivePromoteForProtocolChain(
        t, project, workspace, "e2e-story-promote", "",
        domain.DesignCheckpointStoryLocked,
        storyBundle,
        map[string]string{
            "story_review": storyReview,
            "author_confirmation": authorConfirmation,
        },
    )
    if storyLocked.Result != "PROMOTED" {
        t.Fatalf("story locked=%+v", storyLocked)
    }

    charactersRef := driveImportArtifactForProtocolChain(
        t, project, workspace, "e2e-characters", "characters",
        []string{conceptRef},
        decodeFoundationPayloadForTest(t, "characters.json"),
    )
    worldRef := driveImportArtifactForProtocolChain(
        t, project, workspace, "e2e-world", "world",
        []string{conceptRef},
        decodeFoundationPayloadForTest(t, "world.json"),
    )
    endingRef := driveImportArtifactForProtocolChain(
        t, project, workspace, "e2e-ending", "ending_contract",
        []string{conceptRef},
        decodeFoundationPayloadForTest(t, "ending_contract.json"),
    )
    foundationRef := driveImportArtifactForProtocolChain(
        t, project, workspace, "e2e-foundation", "foundation",
        []string{conceptRef, charactersRef, worldRef},
        decodeFoundationPayloadForTest(t, "foundation.json"),
    )
    bookPlanRef := driveImportArtifactForProtocolChain(
        t, project, workspace, "e2e-book-plan", "book_plan",
        []string{conceptRef, charactersRef, worldRef, endingRef},
        decodeFoundationPayloadForTest(t, "book_plan.json"),
    )
    styleRef := driveImportArtifactForProtocolChain(
        t, project, workspace, "e2e-style", "style_profile",
        []string{briefRef},
        decodeFoundationPayloadForTest(t, "style_profile.json"),
    )
    platformRef := driveImportArtifactForProtocolChain(
        t, project, workspace, "e2e-platform", "platform_profile",
        []string{briefRef},
        decodeFoundationPayloadForTest(t, "platform_profile.json"),
    )

    foundationBundle := driveImportBundleForProtocolChain(
        t, project, workspace, "e2e-foundation-bundle",
        map[string]string{
            "creative_brief": briefRef,
            "story_decisions": decisionsRef,
            "story_concept": conceptRef,
            "foundation": foundationRef,
            "characters": charactersRef,
            "world": worldRef,
            "book_plan": bookPlanRef,
            "ending_contract": endingRef,
            "style_profile": styleRef,
            "platform_profile": platformRef,
        },
    )
    readinessReview := driveImportArtifactForProtocolChain(
        t, project, workspace, "e2e-readiness-review",
        "foundation_readiness_review", nil,
        map[string]any{
            "review_type": "foundation_readiness_review",
            "subject_ref": foundationBundle,
            "policy_version": 1,
            "verdict": "PASS",
            "findings": []any{},
            "created_from_context_refs": []any{
                conceptRef,
                foundationBundle,
            },
        },
    )
    readyResult := drivePromoteForProtocolChain(
        t, project, workspace, "e2e-foundation-promote",
        storyLocked.NewDesignRoot,
        domain.DesignCheckpointFoundationReady,
        foundationBundle,
        map[string]string{
            "foundation_readiness_review": readinessReview,
        },
    )
    if readyResult.Result != "PROMOTED" {
        t.Fatalf("foundation ready=%+v", readyResult)
    }

    if err := project.Reconcile(); err != nil {
        t.Fatal(err)
    }

    ready := readReady(t, workspace)
    if ready.TaskKind != "chapter" || ready.Target != "chapter:1" {
        t.Fatalf("READY=%+v", ready)
    }
    status, err := project.Status()
    if err != nil {
        t.Fatal(err)
    }
    if status.CanonRoot == "" ||
        status.FoundationDesignRoot != readyResult.NewDesignRoot ||
        status.DesignCheckpoint != domain.DesignCheckpointFoundationReady {
        t.Fatalf("status=%+v", status)
    }
}
```


- [ ] **Step 3: 把 Review Focus 五类风险全部钉进测试**

已有 Tasks 3–7 已覆盖 stale CAS、exact review、Foundation crash、tamper/backup。补两个尚未直接覆盖的行为：

```go
func TestImportingNewSourceDoesNotMoveStoryLockedHead(t *testing.T) {
    project, _, _ := newRequiredDesignProjectForTest(t)
    bundleRef, evidence := validStoryLockInputsForTest(t, project)
    locked := promoteForTest(
        t, project, "source-stability-lock", "",
        domain.DesignCheckpointStoryLocked, bundleRef, evidence,
    )
    if locked.Result != "PROMOTED" {
        t.Fatalf("locked=%+v", locked)
    }

    _ = importDesignArtifactForTest(
        t, project, "source_record", nil,
        map[string]any{
            "name": "新资料",
            "source_type": "web",
            "locator": "example",
            "summary": "新增但尚未采用的资料",
            "analysis": "只作为候选 source",
        },
    )

    head := mustDesignHeadForTest(t, project)
    if head.DesignRoot != locked.NewDesignRoot {
        t.Fatalf("source import moved head: %+v", head)
    }
}

func TestFoundationReadyFreezesFurtherPromote(t *testing.T) {
    project, _, _, fixture := makeFoundationReadyProjectForTest(t)
    head := mustDesignHeadForTest(t, project)
    commit, err := project.store.LoadCoreDesignCommit(head.DesignRoot)
    if err != nil || commit == nil {
        t.Fatalf("commit=%+v err=%v", commit, err)
    }

    result := promoteForTest(
        t, project, "frozen-promote", fixture.FoundationRoot,
        domain.DesignCheckpointFoundationReady,
        commit.BundleRef,
        commit.Evidence,
    )
    if result.Result != "INVALID" ||
        !strings.Contains(result.Problem, "建书设计基线已冻结") {
        t.Fatalf("result=%+v", result)
    }
}
```

legacy schema/protocol migration 行为继续由 Task 6 的 migration tests 负责，不在 E2E 复制第三套 fixture。

- [ ] **Step 4: 更新 README 与 CLI/docs contract**

README 快速开始改成：

```text
init
→ capability ack
→ required 项目进入 exchange/design
→ story_locked
→ foundation_ready
→ Core 内部 Foundation settlement
→ chapter:1 READY
```

必须明确：

- fresh 1.1 项目在第一份 Canon 前不读取/等待 READY；
- legacy 项目仍走外部 Foundation READY；
- `CHATGPT_PROTOCOL.md` 是操作协议唯一正文来源；
- Design Store 在本地 `meta/core/design`，Drive 只是交换层；
- subjective story/review 仍由作者 + ChatGPT 完成，Core 不声称文学判断。

`cmd/novel-core/docs_contract_test.go` 加断言，防 README 再写回“capability 后直接 Foundation READY”的旧流程。

- [ ] **Step 5: 把旧 1.0 release checklist 标成历史验收**

`docs/provider-free-release-checklist.md` 顶部增加：

```markdown
> 本清单中的既有 PASS 记录属于 protocol 1.0 / core schema 1 的
> 2026-09-16 产品验收。protocol 1.1 / core schema 2 的语义设计链
> 必须完成新的真实 ChatGPT App + Google Drive 验收后，才能宣称
> 新流程通过产品验收；不得继承旧 PASS。
```

保留旧事实原文，不把 1.0 PASS 改写成 1.1 PASS。

- [ ] **Step 6: 跑局部完整测试**

```bash
go test ./internal/domain ./internal/store ./internal/protocol ./internal/core ./cmd/novel-core -v
```

Expected: PASS。

- [ ] **Step 7: 跑全仓、vet、provider-free 与 fresh CLI 门禁**

```bash
go test ./...
go vet ./...

if go list -deps ./cmd/novel-core |
  grep -E '/internal/(agents|llmcodex|models|rag)(/|$)|agentcore'; then
  echo "novel-core regained AI runtime dependency" >&2
  exit 1
fi

git diff --check

tmp="$(mktemp -d)"
mkdir -p "$tmp/local" "$tmp/drive"
go run ./cmd/novel-core init   --project "$tmp/local"   --workspace "$tmp/drive"   --project-id semantic-smoke   >"$tmp/init.json"

go run ./cmd/novel-core status   --project "$tmp/local"   >"$tmp/status.json"

grep -q '"design_mode":"required"' "$tmp/status.json"
test ! -e "$tmp/drive/exchange/READY.json"
test -d "$tmp/drive/exchange/design/inbox"
test -d "$tmp/drive/exchange/design/result"
```

Expected: 全部 exit 0。

- [ ] **Step 8: whole-branch standards/spec review**

本 implementation plan 文件第一次加入 Git 的提交就是功能基线：

```bash
base="$(
  git log --format=%H --diff-filter=A --     docs/superpowers/plans/2026-09-23-provider-free-semantic-design-baseline.md |
  tail -1
)"
test -n "$base"
git diff --check "$base"..HEAD
git diff --stat "$base"..HEAD
```

按 Superpowers code-review 对 `$base..HEAD` 做两轴审查：

- standards：错误处理、事务恢复、文件安全、测试质量、兼容；
- spec：逐节核对 2026-09-23 semantic design baseline。

阻断项直接修复，并重新执行 Step 6–7。

- [ ] **Step 9: 提交自动化 E2E 与用户文档**

```bash
git add   internal/core/protocol_chain_e2e_test.go   internal/core/design_test.go   cmd/novel-core/main_test.go   cmd/novel-core/docs_contract_test.go   README.md   docs/provider-free-release-checklist.md   docs/superpowers/specs/2026-09-23-provider-free-semantic-design-baseline.md

git commit -m "test: validate semantic design production chain"
git push origin provider-free-novel-core
```

不等待、不轮询非集成 CI。

- [ ] **Step 10: 在发布前做真实 ChatGPT App + Google Drive 集成验收**

这一步是明确的集成边界，不用远端普通 CI 代替。使用 fresh protocol 1.1 项目至少完成：

```text
init
→ ordinary ChatGPT App 读取真实 CHATGPT_PROTOCOL.md / capability challenge
→ capability ack/probe 经真实 Drive 同步被 Core 接受
→ creative_brief / story_decisions / story_concept import
→ story_review + author_confirmation
→ story_locked
→ 七个建书设计文件
→ foundation_readiness_review
→ foundation_ready
→ Core 内部 Foundation settlement
→ chapter:1 READY
→ 新 ChatGPT 会话仅靠 project files 恢复当前权威状态
→ novel-core verify
→ backup / restore 后 verify
```

验收必须记录真实 project_id、core/protocol version、最终 design_root、canon_root、关键 result/receipt 路径和失败项；不要记录聊天私密正文或伪造模型/provider 字段。

如果真实集成链任一关键步骤失败：

- 不把 release checklist 标成 PASS；
- 修代码后重跑相关本地门禁；
- 再重做真实集成边界。

验收通过后只更新现有 `docs/provider-free-release-checklist.md` 的 protocol 1.1 区段，不再新建一份过程型交接文档，然后：

```bash
git add docs/provider-free-release-checklist.md
git commit -m "docs: record semantic design product acceptance"
git push origin provider-free-novel-core
```

---

## Final Acceptance Checklist

实现完成后必须逐项确认：

- [ ] 新项目使用 core schema 2 / protocol 1.1 / design_mode=required。
- [ ] 当前正式发布项目可按“两段显式迁移”从 schema 1 / protocol 1.0 升级到 schema 2 / protocol 1.1；schema 阶段写 design_mode=legacy 且不改变生产状态，protocol 阶段只允许为同一 active task 重绑 replacement attempt。
- [ ] schema 0 与 protocol 0.9 的既有显式兼容入口仍受测试保护。
- [ ] required 项目 capability 通过后没有 active task 是合法状态，且不生成 READY。
- [ ] Design Import 复用现有安全读取、静默期和本地不可变快照 primitive。
- [ ] artifact_ref 与 bundle_ref 由 Core 计算；ChatGPT 不提供可信摘要。
- [ ] story_locked 只接受精确 story_review + author_confirmation。
- [ ] creative_brief、story_decisions、story_concept 的首版最低机器结构有确定性校验。
- [ ] story_decisions 中 blocking open 能机械阻止故事锁定。
- [ ] foundation_ready 绑定完整十槽、固定 required inputs、bundle closure 和精确 readiness review。
- [ ] whole_book_skeleton 至少两个 stage；该门槛只用于 required Design readiness，不反向破坏 legacy Foundation。
- [ ] 第一份 Canon 前的 Design import/promote 不创建 Core production state，也不消耗 task/attempt/entity sequence。
- [ ] Foundation 无副作用预校验与正式 settlement 使用同一 preparation 逻辑。
- [ ] internal Foundation attempt 只有在 foundation_ready 后才 mint，不发布 ChatGPT-facing Foundation READY，也不伪造 manifest。
- [ ] legacy external Foundation 与 required internal Foundation 共用同一 Foundation commit journal。
- [ ] Foundation receipt 首次写入就携带 foundation_design_root；旧 legacy Foundation receipt 保持为空。
- [ ] Foundation journal 在 canon / receipt / production / ready 各故障阶段都能重启恢复，不产生第二份 Foundation Canon、receipt 或 chapter:1 attempt。
- [ ] Foundation ACCEPTED 后 Design Head 冻结；现有 historical_revision 不被伪装成 Foundation revision。
- [ ] exchange/STATUS.json 是唯一状态投影，不新增流程阶段游标 / foundation_state 等第二套流程状态。
- [ ] verify 覆盖所有 Design object/bundle/commit/receipt、正式 HEAD 线性链和 Foundation receipt/head 交叉一致性。
- [ ] backup 继续复用 meta/core 全量复制，不新增第二套 Design 收集器。
- [ ] restore 复用现有 staged Verify，恢复后 design_root/canon_root 都验证通过。
- [ ] Design Inbox 的 traversal/symlink/unknown file/UTF-8/JSON shape/size/snapshot mutation 边界有端到端测试。
- [ ] provider/model/Agent runtime/RAG 依赖没有回到 novel-core。
- [ ] Character Agent、World Arbiter、Arc rehearsal、Planner/Drafter 分离、章节级 Editor/Reviewer 等延期能力没有被本期顺手引入。
- [ ] README 与生成的 CHATGPT_PROTOCOL.md 不互相冲突，旧 protocol 1.0 产品验收记录明确标成历史结果。
- [ ] `go test ./...`、`go vet ./...`、provider-free dependency gate、fresh CLI smoke、`git diff --check` 全部通过。
- [ ] 真实 ordinary ChatGPT App + Google Drive 的 protocol 1.1 建书链完成集成验收；在此之前不得把自动化 E2E 等同于产品验收。

## Implementation Commit Sequence

执行时保持以下提交边界，不把多个 reviewer gate 压成一个巨型 commit：

1. `feat: add content addressed design store`
2. `feat: import immutable semantic design artifacts`
3. `feat: lock story concepts with exact evidence`
4. `feat: validate foundation ready design bundles`
5. `feat: settle approved designs through foundation canon`
6. `feat: activate semantic design for new projects`
7. `feat: verify and back up semantic design authority`
8. `test: validate semantic design production chain`
9. `docs: record semantic design product acceptance`

前八个提交都必须在各自本地验证通过后立即 push；第九个只在真实 ChatGPT App + Drive 集成边界通过后提交。不要等待、不轮询非集成 CI。
