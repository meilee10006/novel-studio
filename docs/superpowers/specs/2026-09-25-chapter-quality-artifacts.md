# Chapter Quality Artifacts Design

**Status:** Implementation complete; exact-SHA development release gates pending
**Date:** 2026-09-25
**Branch:** `chapter-quality-artifacts`

## 1. Goal

Improve novel quality in the order:

1. chapter-level Editor / Reviewer;
2. Planner / Drafter separation;
3. Arc rehearsal.

Do this without restoring Agent runtime, provider/model routing, a workflow engine, or a new quality state machine. Reuse the provider-free Core boundaries already proven in production:

- immutable submission snapshots;
- exact attempt binding;
- canonical artifacts;
- existing `Task / Attempt / Receipt`;
- `REWRITE` and `BLOCKED`;
- existing rolling `planning_patch.json`.

Core still does not judge whether prose is exciting, emotional, elegant, or commercially strong. ChatGPT makes those judgments and records them as artifacts; Core only enforces structure, exact-version binding, internal consistency, and the consequences of ChatGPT's explicit verdict.

## 2. Non-goals

This phase does not add:

- Reviewer Agent, Editor Agent, Planner Agent, Drafter Agent;
- Character Agent or World Arbiter;
- model/provider APIs;
- model-specific routing or token accounting;
- a new TaskKind for review/planning/rehearsal;
- a second Canon or quality authority;
- sealed render packets;
- embedding/Qdrant;
- Dashboard.

### 2.1 Development release boundary

This specification's development phase ends at an exact Git SHA once the spec and plan match the implementation, implementation and regression tests are complete, review has no blocking findings, and the GitHub pull-request CI for that exact SHA passes.

Real-machine deployment, service start/restart, reboot, deployed health/smoke checks, and ordinary ChatGPT App + Google Drive product acceptance are post-release operational validation. They do not block development `RELEASE_READY`.

If post-release validation discovers a source defect, that defect returns to a new GitHub development task and must pass the same code/review/CI boundary. Production tooling must not patch or complete source implementation outside GitHub.

## 3. Compatibility strategy

Protocol envelope and machine schema remain `1.1` / schema `1`.

The reason is that chapter artifact requirements are already attempt-negotiated and bound into `CoreAttempt.RequiredArtifacts` and `TaskDigest`. The extension uses that existing negotiation mechanism rather than changing manifest identity or settlement semantics.

Compatibility rules:

1. Existing active chapter/revision attempts that do not list the new artifacts remain valid under their original five-file contract.
2. New chapter/revision attempts created by the upgraded Core are quality-enabled and require the new artifacts.
3. A quality-enabled attempt is mechanically identified by `chapter_review.json` in `required_artifacts`; no new persisted mode flag is introduced.
4. `REWRITE`, retry, block-resolution and protocol-replacement attempts preserve the active attempt's artifact contract.
5. Core-owned `CHATGPT_PROTOCOL.md` is a derived projection. Reconcile may refresh it from the canonical renderer so an existing 1.1 project receives the current task-artifact guidance without mutating Canon or production authority.

## 4. Quality-enabled chapter artifact set

Legacy active attempt:

```text
chapter.md
chapter_contract.json
events.json
state_delta.json
self_review.json
```

New quality-enabled attempt:

```text
chapter_plan.json
chapter.md
chapter_contract.json
events.json
state_delta.json
self_review.json
chapter_review.json
```

`planning_patch.json` remains conditional. `arc_rehearsal.json` is coupled to it only for quality-enabled attempts.

`self_review.json` is retained for compatibility and author-decision signaling. `chapter_review.json` has a different responsibility: explicit literary/reader-experience review of the exact draft in the immutable attempt snapshot.

## 5. Chapter review

### 5.1 Exact-version binding

A chapter submission is locally snapshotted before settlement. Therefore:

```json
{
  "subject_ref": "chapter.md",
  "plan_ref": "chapter_plan.json"
}
```

binds the review to the exact `chapter.md` and `chapter_plan.json` bytes in that immutable attempt snapshot. ChatGPT does not calculate or provide a trusted digest.

### 5.2 Minimum shape

```json
{
  "subject_ref": "chapter.md",
  "plan_ref": "chapter_plan.json",
  "verdict": "pass",
  "dimensions": {
    "story_progress": {"status": "pass", "note": "本章推进了主线目标"},
    "reader_payoff": {"status": "pass", "note": "本章兑现了一个明确期待"},
    "pacing": {"status": "pass", "note": "场景推进没有明显停滞"},
    "character_consistency": {"status": "pass", "note": "核心人物行为与既有状态一致"},
    "world_consistency": {"status": "pass", "note": "没有突破既有规则"},
    "continuity": {"status": "pass", "note": "与前态和当前 Arc 连续"},
    "ending_hook": {"status": "pass", "note": "章末形成下一章驱动力"}
  },
  "issues": []
}
```

Required dimensions are fixed in this phase:

- `story_progress`
- `reader_payoff`
- `pacing`
- `character_consistency`
- `world_consistency`
- `continuity`
- `ending_hook`

Each dimension must contain:

- `status`: `pass` or `revise`;
- non-empty `note`.

Issues, when present:

```json
{
  "code": "payoff-too-weak",
  "severity": "blocking",
  "description": "承诺只被提及，没有实际兑现",
  "evidence_anchor": "正文中的短原文锚点",
  "suggested_fix": "让主角在本章真正取得阶段性结果"
}
```

`severity` is `blocking` or `advisory`. A blocking issue must have a non-empty `evidence_anchor` that occurs in `chapter.md`. Advisory issues may omit the anchor.

### 5.3 Verdict semantics

Core does not decide whether a dimension should pass. It only enforces the review's self-consistency:

- `verdict=pass`: every required dimension is `pass` and no blocking issue exists.
- `verdict=revise`: at least one required dimension is `revise` or at least one blocking issue exists.
- any other verdict is invalid.

A structurally valid `verdict=revise` produces chapter `REWRITE`. The next attempt keeps the same task and receives deterministic rewrite feedback pointing to `chapter_review.json`. A quality-enabled chapter cannot be `ACCEPTED` with an explicit reviewer `revise`.

Author-choice conflicts still use existing `self_review.author_decision_required` and existing `BLOCKED` semantics. Review does not invent a second blocking mechanism.

## 6. Planner / Drafter separation

### 6.1 Principle

The plan is a first-class immutable artifact, not hidden chain-of-thought and not a new task. Protocol order is:

```text
read task/context
→ write chapter_plan.json
→ draft chapter.md from that plan
→ produce structured events/state
→ review exact draft against exact plan
→ revise before submission when necessary
→ submit one immutable package
```

Core cannot prove the model literally wrote the plan first. It proves that an accepted chapter contains a concrete plan artifact and an exact-snapshot review referencing it.

### 6.2 Minimum shape

```json
{
  "chapter": 1,
  "base_canon_root": "copy READY.base_canon_root",
  "objective": "本章必须完成的主要故事推进",
  "reader_payoff": "本章给读者的明确获得感/信息/结果",
  "beats": [
    {"id": "beat-1", "intent": "建立当前压力"},
    {"id": "beat-2", "intent": "让主角采取动作"},
    {"id": "beat-3", "intent": "形成结果并推出下一问题"}
  ],
  "ending_hook_intent": "章末让读者继续读的具体动力"
}
```

Mechanical rules:

- `chapter` exactly matches task target;
- `base_canon_root` exactly matches task base;
- `objective`, `reader_payoff`, `ending_hook_intent` are non-empty strings;
- `beats` contains at least two items;
- every beat has unique non-empty `id` and non-empty `intent`.

The artifact is saved under the accepted chapter's Canon artifact prefix together with the draft and review. It does not create a separate planning state.

## 7. Arc rehearsal

### 7.1 Principle

Arc rehearsal explores alternatives before one next Arc becomes authoritative. It does not modify `CorePlanningState`; only an accepted `planning_patch.json` may do that.

For a quality-enabled attempt:

- `planning_patch.json` present ⇒ `arc_rehearsal.json` must also be present;
- `arc_rehearsal.json` present without `planning_patch.json` is invalid;
- when rolling planning repair makes `planning_patch.json` required, the attempt also requires `arc_rehearsal.json`.

Legacy active attempts retain the old planning behavior.

### 7.2 Minimum shape

```json
{
  "subject_ref": "planning_patch.json",
  "scenarios": [
    {
      "id": "route-a",
      "next_arc": {
        "id": "arc-2",
        "start_chapter": 11,
        "end_chapter": 20,
        "goal": "进入更高压的资源争夺"
      },
      "opportunity": "扩大主角可见收益",
      "risk": "可能重复上一 Arc 的竞争结构"
    },
    {
      "id": "route-b",
      "next_arc": {
        "id": "arc-2b",
        "start_chapter": 11,
        "end_chapter": 18,
        "goal": "切入关系冲突与身份暴露"
      },
      "opportunity": "制造新鲜冲突",
      "risk": "需要控制信息揭示速度"
    }
  ],
  "selected_scenario_id": "route-a",
  "selection_reason": "更贴合当前读者期待，同时保留下一阶段升级空间"
}
```

Mechanical rules:

- `subject_ref` exactly `planning_patch.json`;
- at least two scenarios;
- scenario IDs unique and non-empty;
- each scenario has a structurally valid `next_arc`, non-empty `opportunity`, non-empty `risk`;
- `selected_scenario_id` exists;
- `selection_reason` non-empty;
- selected scenario's `next_arc` is structurally equal to `planning_patch.next_arc`.

This proves that the authoritative patch is one of the rehearsed alternatives without asking Core to decide which alternative is better.

## 8. Canon and receipts

Accepted quality artifacts are stored under the existing chapter prefix:

```text
chapters/000001/chapter_plan.json
chapters/000001/chapter.md
...
chapters/000001/chapter_review.json
```

If accepted with planning:

```text
chapters/000010/planning_patch.json
chapters/000010/arc_rehearsal.json
```

No new root is created. Existing Canon root computation automatically commits them because they are ordinary canonical artifacts.

Receipts keep their existing shape. `ArtifactDigests` already records the received immutable submission. `ValidationDigest` remains the deterministic proof of settlement result; review-triggered rewrite violations become part of it.

## 9. Rewrite behavior

Review-driven rewrite must preserve the normal invariant:

- same `task_id`;
- new `attempt_id`;
- same `base_canon_root`;
- same required artifact contract;
- rejected snapshot stays immutable/auditable;
- Canon does not move.

Rewrite feedback should classify review failures as a chapter-review problem and tell ChatGPT to fix `chapter.md` and regenerate `chapter_review.json` against the replacement draft. Plan changes are allowed in the new attempt because the whole rejected attempt is superseded.

## 10. Revision behavior

New historical revision attempts are quality-enabled exactly like new chapter attempts. Existing active revision attempts keep their original required artifact list.

During replay, accepted `chapter_plan.json` and `chapter_review.json` are versioned under the rewritten chapter prefix just like the rewritten prose. No separate review history pointer is needed because Canon history already records the exact artifact set at every revision root.

## 11. Security and limits

New JSON files use the existing UTF-8, JSON depth, array length, file size, traversal, symlink and immutable snapshot protections.

No review note, issue description, opportunity, risk, or selection reason is interpreted as executable instructions by Core.

## 12. Development acceptance and post-release validation

Development automated tests must prove:

1. legacy active five-file attempt still settles;
2. new chapter attempt requires plan/review;
3. invalid/missing plan rewrites;
4. review `pass` accepts only when structurally consistent;
5. review `revise` always rewrites and preserves Canon root;
6. blocking review issue anchor must exist in exact draft;
7. rewrite attempt preserves quality artifact contract;
8. optional planning patch on quality attempt requires rehearsal;
9. selected rehearsal scenario must equal the patch;
10. planning repair requires both patch and rehearsal;
11. accepted quality artifacts survive verify/backup/restore;
12. historical revision uses the same quality artifact contract.

The development release gate additionally requires the exact release SHA to pass GitHub pull-request CI, including repository tests, vet, provider-free dependency checks, race-sensitive Core tests, container build/smoke, and diff hygiene.

Ordinary ChatGPT App + real Google Drive validation remains valuable post-release product validation, including new-session recovery and real-machine verify/backup/restore. It is tracked separately from development readiness and does not delay `RELEASE_READY`.

## 13. Success criterion

The phase succeeds if it demonstrably improves the writing process without reintroducing AI runtime state:

- every newly accepted chapter has an explicit pre-draft plan;
- every newly accepted chapter has an explicit review of that exact immutable snapshot;
- reviewer-requested revision cannot be silently accepted;
- next-Arc authority cannot be updated on a quality-enabled attempt without an explicit multi-option rehearsal;
- all authority still flows through existing Canon + Task/Attempt/Receipt;
- the reviewed exact Git SHA passes all required GitHub development CI checks and is recorded as `RELEASE_READY` before any production deployment begins.
