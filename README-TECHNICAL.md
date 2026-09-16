# Provider-Free Novel Core — Technical Reference

This document describes the supported **ordinary ChatGPT App + real Google Drive + conforming local Google Drive transport + `novel-core`** architecture. Drive Desktop (macOS/Windows) and rclone (Linux/Vultr) are transport implementations, not Core dependencies. Legacy provider/agent runtimes are not part of the production path.

## 1. Authority model

The ordinary ChatGPT App is the only creative/AI layer. `novel-core` is deliberately AI-free and deterministic.

Authority lives in the local project directory under `meta/core/**`. Google Drive is transport, not a database. Notion is an optional read-only projection. Timestamps, Drive mtimes, machine paths, logs, and model metadata are excluded from the Canon content root.

The current Canon head binds:

- local schema/revision;
- parent root;
- structured Canon state digest;
- accepted artifact digest inventory.

Each accepted transition writes an immutable receipt with `previous_root -> new_root` continuity. `verify` recomputes the current root and walks the active receipt lineage.

## 2. Workspace protocol

`novel-core init` creates a Drive workspace with the current machine-readable files plus a generated `CHATGPT_PROTOCOL.md`. The repository does not ship a second handwritten copy.

Key paths:

```text
setup/                         capability challenge/ack
exchange/READY.json            current task pointer
exchange/STATUS.json           current authority/capability/task summary
exchange/outbox/<task>/<try>/  Core-owned task/context
exchange/inbox/<task>/<try>/   ChatGPT submission
exchange/result/               settlement results
exchange/control/inbox/        author directives / block decisions
published/                     Core-owned published views
projection/notion.json         optional read-only projection
backup/                        non-authoritative backup copies
```

ChatGPT writes only allowed inbox/control/setup response files. Core-owned READY/STATUS/outbox/result/published/projection data must not be edited by ChatGPT. `STATUS.json` also publishes `export_ready` and the full deterministic `export_problems` set so a bounded task context cannot hide an outstanding ending obligation; readiness is only a prerequisite signal, not a substitute for export-time Canon verification. Drive does not make multi-file updates atomic, so consumers must require `STATUS.active_attempt_id == READY.attempt_id`, `STATUS.active_target == READY.target`, and `STATUS.block_id == READY.block_id` before acting; a mismatch means the workspace is still converging. `STATUS.json.canon_root` is the authority root to bind control messages; it is intentionally distinct from a revision task's historical `READY.base_canon_root`.

A submission is considered only after its files and manifest are stable across scans. The local snapshot is then authoritative for validation; later Drive mutation becomes a conflict rather than silently changing the attempt.

## 3. Production loop

`novel-core serve --project <local>` holds the project writer lock for its lifetime. Correctness uses periodic scanning; watcher events are optional wakeups only.

Each pass performs, in order:

1. recover incomplete local commit state;
2. reconcile the active task/attempt and READY state;
3. scan/process control messages;
4. scan the active submission;
5. settle Foundation, Chapter, or Revision snapshots;
6. create retry attempts for protocol-invalid submissions when appropriate;
7. best-effort refresh of `projection/notion.json`.

Projection failure is non-authoritative and never rolls back a successful commit.

## 4. Tasks and attempts

A task is the semantic unit (Foundation, chapter, revision). An attempt is a versioned submission binding with protocol version, task digest, completion nonce, base Canon root, and an exact required artifact set. Chapter/Revision `context.json` includes `foundation_reference`, a budgeted canonical Foundation reference containing normalized characters/world/style/platform data so a fresh chat can discover valid Canon IDs without historical conversation state.

`REWRITE` preserves the task but creates a new attempt. Rejected/invalid attempts do not mint permanent Canon IDs.

Two chats racing the same attempt are resolved by stable local snapshot: the first stable byte set locks the attempt; different later bytes are a conflict.

## 5. Structured validation

Core validates only facts it can mechanically prove, including:

- protocol identity and exact artifact set;
- chapter identity / declared POV fields;
- evidence anchors for events;
- knowledge provenance;
- locations and travel constraints;
- resources;
- relationship/evidence transitions;
- foreshadow, story-conflict, and reader-promise state machines;
- rolling arc planning contracts;
- ending-resolution evidence.

Chapter submissions may introduce new characters, locations, or tracked resources with `state_delta.character_add` / `state_delta.location_add` / `state_delta.resource_add`. Core allocates the permanent entity ID only on ACCEPTED, rewrites same-attempt local character/location/resource references to the canonical ID, records the mapping in the result/receipt, and carries the dynamic entity in Canon for later task contexts and historical revision snapshots. New foreshadows use an attempt-local ID on first creation and receive a permanent foreshadow ID only on ACCEPTED; later lifecycle transitions must reference that canonical ID. Story conflicts follow the same permanent-ID rule and store canonical-character participants, readable description, escalation/close conditions, current state, and the latest evidence event. Their deterministic lifecycle is `open → escalated → resolved`, with `open/escalated → retired`; every transition requires accepted event evidence. Reader promises use the same lifecycle-ID rule: an attempt-local ID on first creation, a permanent ID only on ACCEPTED, then canonical `promise_id` references thereafter.

Core does not score prose quality, emotional strength, pacing, commercial potential, or “AI-ness”. Those are creative/editorial decisions for the author and ChatGPT App.

## 6. Rolling planning

Foundation seeds a deterministic planning state. Chapter submissions may carry `planning_patch.json` when required. Planning settlement is part of the same authority transaction as the chapter, so a crash cannot leave prose accepted with planning still on the old state.

Future-plan author directives attach constraints to future tasks without changing already accepted Canon facts.

## 7. Historical revision

`historical_revision` branches from the target chapter's parent root. The old target and all later accepted chapters are marked superseded for the active branch.

Core then emits revision tasks sequentially. Old prose may be supplied as a candidate, but old events/state deltas are not trusted as new evidence. Each replayed chapter must be resubmitted and accepted against the new preceding root.

The replay is linear, single-chain, and conservative. Final export is blocked until it catches up with the original head.

## 8. Author controls and BLOCKED

Control messages are independently snapshotted and idempotent. Supported first-release semantics include future planning directives, historical revisions, and block resolutions.

A control message is bound to a base Canon root. Stale-root messages are rejected instead of being applied according to filesystem arrival order.

## 9. Locking and crash recovery

All state-mutating public Core operations use the same cross-process project lock. `serve`, migration, commit/revision settlement, and control processing cannot mutate one project concurrently.

Export and verify use a shared read lock so they see a stable authority snapshot while still allowing concurrent readers. `status` remains lightweight read-only access.

Chapter commits and migrations use durable staged state/receipts so recovery is idempotent. An attempt can advance authority at most once.

## 10. Backup and restore

A Core backup contains a manifest with project/schema/protocol/Canon identity and an exact list of persistent authority files with SHA-256 digests.

Restore:

- validates the entire backup before publishing anything;
- rejects symlinks/unlisted files/digest changes;
- stages into a temporary directory;
- opens and verifies the restored project;
- renames only into a previously nonexistent target.

A one-byte mutation therefore fails restore instead of producing a best-effort project.

## 11. Schema and protocol migration

Local schema and Drive protocol versions are independent.

Known legacy versions migrate only under the project writer lock and only after a verified backup exists. Migration uses prepared/committed receipts and is safe to resume after a crash. Re-running a committed migration is a no-op.

Unknown future versions fail closed.

A protocol upgrade invalidates the old active attempt binding and creates a fresh attempt/capability challenge under the new protocol, but the novel Canon content root must remain unchanged.

## 12. Export

Final export is allowed only when:

- historical revision/replay is settled;
- ending resolution has accepted event evidence on the latest accepted chapter;
- foreshadows are terminal (`closed`/`retired`);
- story conflicts are terminal (`resolved`/`retired`);
- reader promises are terminal (`fulfilled`/`retired`);
- a recomputed Canon root matches the current head.

Export enumerates chapter Markdown from the active Canon head digest inventory. It never scans raw artifact directories for “whatever files exist”, so superseded downstream prose cannot leak into the final manuscript.

## 13. Security limits

All Drive inputs are untrusted. Protocol I/O rejects traversal, absolute paths, escaping symlinks, submission symlinks/directories, unknown artifacts, invalid UTF-8, oversized files/total submissions, excessive JSON depth, and excessive array length.

No credential is stored in project protocol files because `novel-core` has no model credentials to manage.

## 14. Supported CLI

```text
novel-core init     initialize local authority + Drive protocol
novel-core serve    long-running polling/recovery/settlement loop
novel-core status   inspect current project/task state
novel-core verify   recompute Canon and active receipt lineage
novel-core export   create final manuscript from active Canon
novel-core restore  restore a verified backup into a new directory
novel-core migrate  explicitly migrate known schema/protocol versions
```

No provider/model/API-key flags exist.

## 15. Dependency boundary

`go list -deps ./cmd/novel-core` must not include `agentcore`, legacy `internal/agents`, `internal/bootstrap`, `internal/llmcodex`, LiteLLM, provider dispatch, embedding, or Qdrant runtime dependencies.

The old provider/agent runtime, old `cmd/novel-studio`, and vendored LiteLLM implementation have been removed from the supported codebase. Static writing references under `assets/references/` and `assets/styles/` remain as content resources only.

## 16. Release acceptance

Automated acceptance requires fresh `go test ./...`, `go vet ./...`, dependency-boundary checks, Core recovery/security/revision/migration tests, and `verify` on a generated fixture.

A real release still requires a separate manual product run using the ordinary ChatGPT App + real Google Drive + a conforming local Google Drive transport + local `novel-core`. Automated fixtures must not be reported as a substitute for that manual chain.

See `docs/provider-free-release-checklist.md` for the final status table.
