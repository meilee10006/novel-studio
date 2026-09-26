# Chapter Quality Artifacts Implementation Plan

**Date:** 2026-09-25
**Spec:** `docs/superpowers/specs/2026-09-25-chapter-quality-artifacts.md`
**Branch:** `chapter-quality-artifacts`
**Development state:** Tasks 1–4 are implemented; development convergence/release gates remain.

## Global constraints

- Base is exact `main@ea3861ff53b27eb568d0a630718cd4d6c34a56ec` at semantic-design product acceptance.
- Local-first; no provider/model/Agent runtime.
- Do not add TaskKind unless an implementation task proves existing chapter/revision attempts cannot express the lifecycle.
- Preserve existing active 1.1 attempts and any unknown or parallel branch work; no reset, clean, reclone, history rewrite, or force push.
- TDD: failing public-behavior test before implementation.
- Stable task commits are pushed promptly.
- GitHub pull-request CI must pass for the exact release SHA before `RELEASE_READY`.
- Real-machine deployment, service start/restart, reboot, deployed health/smoke, and ordinary ChatGPT App + Google Drive product acceptance are post-release operations, not unfinished development.
- Any source defect found after release returns to GitHub development; production tooling does not perform source fixes.

## Task 1 — Exact-snapshot chapter review quality gate — COMPLETE

Evidence: `ab067df9e05c6356993f387f85379655367f7da4`.

### Tests first

Add chapter-review tests covering:

- legacy active five-file attempt remains valid;
- new quality-enabled attempt requires `chapter_review.json`;
- `subject_ref != chapter.md` rewrites;
- missing required dimension rewrites;
- invalid dimension status rewrites;
- `verdict=pass` + revise dimension rewrites;
- `verdict=pass` + blocking issue rewrites;
- blocking issue anchor absent from chapter rewrites;
- coherent `verdict=revise` produces `REWRITE`, leaves Canon unchanged, creates replacement attempt with same required artifact contract;
- coherent `verdict=pass` can proceed to the existing chapter validations.

### Implementation

- Introduce quality-aware chapter artifact selection based on `required_artifacts`.
- Add `validateChapterReview`.
- Add deterministic rewrite-feedback classification for review failures.
- Do not change `self_review` author-decision behavior.
- New chapter/revision attempts should become quality-enabled only after Task 2 adds the complete quality artifact contract; Task 1 implementation remains dormant unless an attempt explicitly requests review.

### Commit

`feat: add exact snapshot chapter review gate`

## Task 2 — Planner / Drafter artifact separation — COMPLETE

Evidence: `9e68ba878007a298df8da383888d7176c245daaf`.

### Tests first

Cover:

- new chapter:1 after Foundation requires `chapter_plan.json` + `chapter_review.json`;
- next normal chapter and new revision attempts require the same quality artifacts;
- legacy active attempt with only the original five remains valid;
- plan chapter/base mismatch rewrites;
- objective/payoff/hook must be non-empty;
- beats require at least two unique IDs with non-empty intents;
- accepted plan/review are stored under the chapter Canon prefix;
- REWRITE/retry/block-resolution preserve the quality required-artifact list.

### Implementation

- Keep legacy `chapterArtifactNames` as the five-file compatibility set.
- Add quality artifact set helper for newly created chapter/revision attempts.
- Add `validateChapterPlan`.
- Canonicalize and commit plan/review with the existing chapter commit journal.
- Make review require `plan_ref=chapter_plan.json`.
- Update test helpers and protocol-chain E2E.

### Commit

`feat: require chapter plans for new production attempts`

## Task 3 — Arc rehearsal before authoritative rolling planning — COMPLETE

Evidence: `6e93a19e41e32eaadd55d68c3f10a23ccae35a10`.

### Tests first

Cover quality-enabled attempts:

- planning patch without rehearsal rewrites;
- rehearsal without planning patch rewrites;
- fewer than two scenarios rewrites;
- duplicate scenario ID rewrites;
- invalid selected ID rewrites;
- selected scenario arc differing from planning patch rewrites;
- valid rehearsal + patch accepts and advances planning;
- planning repair required artifacts include both patch and rehearsal;
- legacy active attempts retain old patch-only behavior.

### Implementation

- Allow `arc_rehearsal.json` only on quality-enabled chapter/revision attempts.
- Add `validateArcRehearsal`.
- Couple it to `validateChapterPlanning`.
- Commit accepted rehearsal as a chapter artifact while only `planning_patch.json` mutates `CorePlanningState`.

### Commit

`feat: rehearse arc alternatives before planning updates`

## Task 4 — Protocol projection, docs, recovery, and whole-chain E2E — COMPLETE

Evidence: `a726005edc0876dc2f076c12bf08a039a375f223`.

### Tests first

- generated `CHATGPT_PROTOCOL.md` describes plan → draft → review order;
- `required_artifacts` is authoritative rather than a hard-coded five-file assumption;
- existing project Reconcile refreshes Core-owned `CHATGPT_PROTOCOL.md` without changing Canon root, active task, attempt ID, or task digest;
- quality E2E: Foundation → chapter plan/draft/review → reviewer REWRITE → replacement ACCEPTED → next READY;
- rolling planning E2E includes rehearsal;
- new-session file recovery instructions still begin from project/status/READY.

### Implementation

- Refresh canonical protocol projection during safe reconcile/startup.
- Update README and release checklist with automated status only; do not claim real product PASS yet.
- Keep protocol number 1.1 because task-specific artifact negotiation is already part of 1.1 and no envelope/schema identity changes.

### Gates

```bash
go test ./...
go vet ./...
if go list -deps ./cmd/novel-core |
  grep -E '/internal/(agents|llmcodex|models|rag)(/|$)|agentcore'; then
  exit 1
fi
git diff --check
```

### Commit

`test: validate chapter quality artifact chain`

## Task 5 — Development convergence and release gates

### Completed review corrections

- Align the quality-enabled marker with the spec: `chapter_review.json` in `required_artifacts` is sufficient.
- Make reviewer-requested REWRITE replace the rejected attempt rather than falsely restricting `allowed_scope` to only `chapter_review.json`.
- Add explicit regression coverage that accepted plan/review/planning/rehearsal artifacts survive verify + backup/restore.

Evidence: `b1ab1baa057b3a412f99f882b630402b4f20d58f`.

### Remaining development gates

1. Remove stale user-facing references to the deleted `provider-free-novel-core` branch.
2. Encode `go test ./...`, `go vet ./...`, provider-free dependency validation, diff hygiene, race-sensitive Core tests, build/smoke, and container checks in GitHub CI.
3. Open a pull request from `chapter-quality-artifacts` to `main`.
4. Review the complete PR diff; resolve every blocking implementation/spec/test/documentation finding in GitHub.
5. Require all GitHub CI jobs for the exact PR head SHA to pass.
6. Re-read branch and PR head after CI. If unchanged and all gates are green, record:
   `SPEC_COMPLETE → PLAN_COMPLETE → IMPLEMENTATION_COMPLETE → TEST/REVIEW/CI_PASS → RELEASE_READY @ exact SHA`.

## Post-release operations — outside this development plan

After `RELEASE_READY`, deployment to a real machine, starting/restarting services, reboot testing, deployed health/smoke checks, and ordinary ChatGPT App + Google Drive product acceptance are separate operational work.

Those operations may discover a new defect, but they do not complete or patch source code. Any source change returns to a new GitHub development task with review and exact-SHA CI.

## Review gates

Before each implementation commit, review from two opposed perspectives:

1. **Completer:** what required behavior/evidence/recovery case is missing?
2. **Rejector:** what new state, abstraction, duplicate authority, compatibility break, or unprovable claim should be removed?

The design should prefer deleting machinery over introducing a new Agent or TaskKind.
