# Provider-Free Protocol Completion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete the two remaining machine-protocol gaps by adding backward-compatible structured REWRITE feedback and the full chapter contract defined by the approved provider-free design.

**Architecture:** Keep protocol `1.0` backward-compatible: existing accepted submissions and replay remain readable, while newly generated READY/protocol guidance emits the richer structures. Core validates only deterministic structure, references, state transitions, and length; it never evaluates literary quality. Preserve legacy `violations []string` while deriving deterministic `rewrite_feedback` from the same validation failures.

**Tech Stack:** Go 1.25.5, standard library JSON, existing `internal/core`, `internal/protocol`, `internal/domain`, Docker `golang:1.25.5` local gates.

**Spec:** `docs/superpowers/specs/2026-09-16-provider-free-protocol-completion-design.md` (refines the provider-free core design §§17, 20.3, 20.4).

## Global Constraints

- Ordinary ChatGPT App remains the sole AI/creative layer; Core stays provider-free and deterministic.
- Do not add provider/model/Ollama/MCP/embedding/Qdrant/browser/Work dependencies.
- Protocol `1.0` remains readable for existing accepted submissions and historical replay; no version bump in these tasks.
- New validation must be mechanically provable; do not add literary-quality judgment.
- Every behavior change follows RED → minimal GREEN → focused regression → fresh full local gates.
- Stable phases are committed and pushed immediately; non-integration remote CI is asynchronous and is not waited on or polled.

---

### Task 1: Structured REWRITE feedback

**Files:**
- Modify: `internal/core/chapter.go`
- Modify: `internal/core/production.go` only if Foundation REWRITE shares the same feedback adapter
- Modify: `internal/protocol/protocol.go`
- Test: `internal/core/rewrite_feedback_test.go`
- Test: `internal/protocol/protocol_test.go`

**Interfaces:**
- Consumes: existing deterministic `[]string` validation violations and existing `ChapterSettlement` / `FoundationSettlement` REWRITE paths.
- Produces: `RewriteFeedback` with JSON fields `code`, optional `entity_ref`, `expected`, `observed`, `evidence_refs`, `allowed_scope`; REWRITE results retain `violations` and add `rewrite_feedback` in the same deterministic order.

- [ ] **Step 1: Write failing high-level tests**

Create `internal/core/rewrite_feedback_test.go` with a chapter REWRITE that already exists today (invalid `hard_constraints`) and assert the compatibility field plus the new structured field:

```go
func TestRewriteFeedbackIsStructuredAndKeepsLegacyViolations(t *testing.T) {
    project, _, workspace, ready := acceptedFoundationProject(t)
    artifacts := longformChapterArtifacts(1, nil)
    artifacts["chapter_contract.json"] = []byte(`{"chapter":1,"declared_pov":"character-000001","hard_constraints":"hc-a"}`)

    got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
    if got.Result != "REWRITE" || len(got.Violations) == 0 {
        t.Fatalf("settlement=%+v", got)
    }
    if len(got.RewriteFeedback) != len(got.Violations) {
        t.Fatalf("feedback=%+v violations=%v", got.RewriteFeedback, got.Violations)
    }
    item := got.RewriteFeedback[0]
    if item.Code == "" || item.Expected == "" || item.Observed == "" || len(item.EvidenceRefs) == 0 || len(item.AllowedScope) == 0 {
        t.Fatalf("feedback item=%+v", item)
    }
}
```

Also read `exchange/result/<attempt>.json` in the same test and assert it contains both `violations` and `rewrite_feedback`. Add one Foundation REWRITE case using an existing invalid Foundation artifact so both settlement paths use the same type.

- [ ] **Step 2: Run focused tests and verify RED**

Run:

```bash
/usr/local/go/bin/go test ./internal/core -run 'RewriteFeedback' -count=1 -v
```

Expected: FAIL because REWRITE settlements do not yet expose structured feedback.

- [ ] **Step 3: Implement the minimal deterministic adapter**

Add the public result type in `internal/core/chapter.go` (or a focused `rewrite_feedback.go` if the file would become harder to read):

```go
type RewriteFeedback struct {
    Code         string   `json:"code"`
    EntityRef    string   `json:"entity_ref,omitempty"`
    Expected     string   `json:"expected"`
    Observed     string   `json:"observed"`
    EvidenceRefs []string `json:"evidence_refs"`
    AllowedScope []string `json:"allowed_scope"`
}
```

Add a `RewriteFeedback []RewriteFeedback` field with JSON tag `rewrite_feedback,omitempty` to both `ChapterSettlement` and `FoundationSettlement`. Add a deterministic helper with this exact interface:

```go
func buildRewriteFeedback(violations []string, task *domain.CoreTask) []RewriteFeedback
```

The helper must map known deterministic validator messages to stable codes and artifact scopes; unknown deterministic messages use a stable fallback code such as `validation.failed`, preserve the full existing violation in `observed`, use `expected="submission satisfies deterministic Core validation"`, and point evidence/scope at the artifact category selected by an explicit prefix table. Do not derive entity IDs by free-form NLP. Feed the same feedback into `ValidationDigest`, workspace result JSON, and returned settlement so crash/retry paths remain deterministic. Preserve `violations` byte-for-byte after the existing sort.

- [ ] **Step 4: Extend generated ChatGPT protocol text**

Document the new result shape and require ChatGPT to read `rewrite_feedback` when present. Keep `violations` documented as the compatibility field.

- [ ] **Step 5: Run focused GREEN and surrounding regressions**

Run:

```bash
/usr/local/go/bin/go test ./internal/core -run 'Rewrite|Feedback|Outcome|Foundation|Chapter' -count=1
/usr/local/go/bin/go test ./internal/protocol -count=1
```

Expected: PASS.

- [ ] **Step 6: Run fresh full local gates**

Using Docker `golang:1.25.5` and `GOFLAGS=-buildvcs=false`, run `gofmt`, `go test ./... -count=1`, race for core/store/protocol, `go vet ./...`, release build, provider-free dependency boundary, and `git diff --check`.

- [ ] **Step 7: Commit and push**

Commit message:

```text
feat: add structured rewrite feedback
```

Push immediately and verify local/tracking/remote tree identity without waiting for remote CI.

---

### Task 2: Full backward-compatible chapter contract

**Files:**
- Modify: `internal/core/chapter.go`
- Modify: `internal/core/planning.go` only for comparison against already-modeled rolling-planning constraints
- Modify: `internal/protocol/protocol.go`
- Test: `internal/core/chapter_contract_extended_test.go`
- Test: `internal/protocol/protocol_test.go`

**Interfaces:**
- Consumes: existing `chapter_contract.json`, current task/attempt base root, current canon/longform IDs, supported state change kinds, current planning constraints, and `chapter.md` bytes.
- Produces: deterministic validation for optional compatibility fields `start_state`, `purpose`, `main_conflict`, `reader_question`, `obligation_refs`, `immutable_refs`, `foreshadow_operations`, `expected_changes`, `ending_hook`, `target_length`, `planning_obligations`; existing minimal historical contracts remain valid.

- [ ] **Step 1: Write failing structure tests**

Create `internal/core/chapter_contract_extended_test.go`. Start with a table that changes only `chapter_contract.json` on top of `longformChapterArtifacts`:

```go
func TestExtendedChapterContractStructure(t *testing.T) {
    tests := []struct {
        name, contract, want string
    }{
        {"purpose type", `{"chapter":1,"declared_pov":"character-000001","purpose":7}`, "purpose"},
        {"blank reader question", `{"chapter":1,"declared_pov":"character-000001","reader_question":"  "}`, "reader_question"},
        {"duplicate obligations", `{"chapter":1,"declared_pov":"character-000001","obligation_refs":["promise-1","promise-1"]}`, "obligation_refs"},
        {"bad target length", `{"chapter":1,"declared_pov":"character-000001","target_length":{"min_chars":20,"max_chars":10}}`, "target_length"},
        {"unknown expected change", `{"chapter":1,"declared_pov":"character-000001","expected_changes":["teleport"]}`, "expected_changes"},
        {"bad foreshadow operation", `{"chapter":1,"declared_pov":"character-000001","foreshadow_operations":[{"foreshadow_id":"f"}]}`, "foreshadow_operations"},
    }
    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            project, _, workspace, ready := acceptedFoundationProject(t)
            artifacts := longformChapterArtifacts(1, nil)
            artifacts["chapter_contract.json"] = []byte(tc.contract)
            got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
            if got.Result != "REWRITE" || !containsViolation(got.Violations, tc.want) {
                t.Fatalf("settlement=%+v", got)
            }
        })
    }
}
```

Add `TestExtendedChapterContractKeepsLegacyMinimumCompatible`, submitting the untouched legacy `{chapter, declared_pov}` fixture and requiring `ACCEPTED`.

- [ ] **Step 2: Run structure tests and verify RED**

Run:

```bash
/usr/local/go/bin/go test ./internal/core -run 'ExtendedChapterContract' -count=1 -v
```

Expected: FAIL because the new fields are currently ignored.

- [ ] **Step 3: Add minimal structural validation**

Add a focused validator called from `validateAndCanonicalizeChapter` only when the new keys are present:

```go
func validateExtendedChapterContract(contract map[string]any, body string, state *domain.CoreProductionState, task *domain.CoreTask, changes []any, violations *[]string)
```

Use existing `cleanString`, `stringArray`, and exact-integer helpers where they match. Validate non-empty string fields, unique non-empty arrays, `start_state.canon_root == task.BaseCanonRoot` when present, `0 <= min_chars <= max_chars`, supported change kinds, and the structural shape of foreshadow operations. Do not require any new key merely because it is absent; backward compatibility depends on omission remaining valid.

- [ ] **Step 4: Write failing semantic-reference tests**

Add cases that build on accepted canonical state. The minimum required set is:

```go
func TestExtendedChapterContractChecksMechanicalSemantics(t *testing.T) {
    t.Run("length outside range", func(t *testing.T) {
        project, _, workspace, ready := acceptedFoundationProject(t)
        artifacts := longformChapterArtifacts(1, nil)
        artifacts["chapter_contract.json"] = []byte(`{"chapter":1,"declared_pov":"character-000001","target_length":{"min_chars":1,"max_chars":2}}`)
        got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
        if got.Result != "REWRITE" || !containsViolation(got.Violations, "target_length") {
            t.Fatalf("settlement=%+v", got)
        }
    })

    t.Run("expected change missing", func(t *testing.T) {
        project, _, workspace, ready := acceptedFoundationProject(t)
        artifacts := longformChapterArtifacts(1, nil)
        artifacts["chapter_contract.json"] = []byte(`{"chapter":1,"declared_pov":"character-000001","expected_changes":["relationship"]}`)
        got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
        if got.Result != "REWRITE" || !containsViolation(got.Violations, "expected_changes") {
            t.Fatalf("settlement=%+v", got)
        }
    })
}
```

Add separate fixtures for an unknown canonical `obligation_refs`/`immutable_refs` ID, unknown `foreshadow_id`, a real foreshadow transition not in `allowed_states`, and a `planning_obligations` value not already present in the active task constraints. Each must fail for the specific mechanical rule, not because a prerequisite fixture is malformed.

- [ ] **Step 5: Run semantic tests and verify RED**

Run the same focused test target and confirm the new cases fail for the intended missing validation.

- [ ] **Step 6: Implement minimal mechanical semantic validation**

Extend `validateExtendedChapterContract` without adding a second authority store. Build lookup sets from the already-loaded canonical/context state; permit same-attempt local references only where the existing protocol already permits them. Use:

```go
bodyChars := utf8.RuneCountInString(body)
```

for `target_length`. Build a set of submitted `state_delta.json.changes[].kind` values and require every `expected_changes` entry to appear. For each `foreshadow_operations` entry, require an existing foreshadow and, when that foreshadow changes in this attempt, require the submitted next state to be listed in `allowed_states`. Compare `planning_obligations` only with already-issued task/control constraints; never infer an obligation from prose. `purpose`, `main_conflict`, `reader_question`, and `ending_hook` receive type/nonblank validation only.

- [ ] **Step 7: Update generated protocol guidance**

Make new READY guidance show the complete contract shape and state that historical/minimal `1.0` contracts remain readable but newly generated submissions should include the complete fields.

- [ ] **Step 8: Run focused GREEN and full fresh gates**

Run focused contract/chapter/planning/protocol tests, then the same full Docker gates as Task 1.

- [ ] **Step 9: Commit and push**

Commit message:

```text
feat: validate complete chapter contracts
```

Push immediately and verify local/tracking/remote tree identity without waiting for remote CI.

---

### Task 3: Traceability and release-state review

**Files:**
- Modify only if evidence requires it: `docs/provider-free-release-checklist.md`, `README-TECHNICAL.md`, or the existing implementation plan.

**Interfaces:**
- Consumes: completed Task 1 and Task 2 behavior plus current release checklist.
- Produces: an evidence-backed list of any remaining automated gap; manual ordinary ChatGPT + Drive + Drive Desktop checks remain `NOT RUN` unless actually executed.

- [ ] **Step 1: Re-run spec-to-implementation traceability**

Check design completion definition and plan tasks against code/tests. Do not invent additional literary/business requirements.

- [ ] **Step 2: Run final automated gates if documentation or code changed**

Use the same full local gate set and `git diff --check`.

- [ ] **Step 3: Commit/push only if repository content changed**

Do not mark manual product acceptance complete. Do not wait/poll remote CI.
