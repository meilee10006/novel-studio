# Vultr Product Acceptance Recovery State

This is the durable recovery card for the in-progress manual product acceptance. Repo/Git is the handoff source; host/Drive state must be re-read and verified before continuing. Do not copy credentials or OAuth tokens into this file.

## Identity and locations

- Branch: `provider-free-novel-core`.
- Acceptance project ID: `vultr-product-acceptance-20260916`.
- Vultr acceptance root: `/opt/sentinelx-cloud-core/novel-core-acceptance`.
- Core authority project: `/opt/sentinelx-cloud-core/novel-core-acceptance/project-product-acceptance`.
- Local exchange workspace: `/opt/sentinelx-cloud-core/novel-core-acceptance/drive/vultr-product-acceptance`.
- Binary: `/opt/sentinelx-cloud-core/novel-core-acceptance/bin/novel-core`.
- Google Drive rclone remote: `gdrive:` is rooted at the acceptance `novel-core-workspace`; the product workspace is `gdrive:vultr-product-acceptance`.
- OAuth/rclone configuration is host-local and must remain outside Git.

## Transport mode

The initial acceptance used `rclone mount`. After the FUSE read path stalled, the same real Drive workspace continued through an ownership-filtered `rclone copy` bridge without changing Core authority. Do not reintroduce a bidirectional blind sync.

Direction boundaries:

- Drive -> local: ChatGPT-owned inputs only (`setup` responses when needed, `exchange/inbox/**`, `exchange/control/inbox/**`).
- Local -> Drive: Core-owned outputs (`project.json`, `CHATGPT_PROTOCOL.md`, `exchange/READY.json`, `exchange/STATUS.json`, `exchange/outbox/**`, `exchange/result/**`, `exchange/control/result/**`, plus Core-owned published/projection output when needed).
- Before acting on a task, re-read local Core authority and Drive READY/STATUS and require identities to agree.

Use `rclone copy`/`copyto` on the specific ownership paths rather than syncing the whole workspace in either direction. The exact host-local rclone config is discoverable with `rclone listremotes`; never print or commit its token.

## Current verified runtime checkpoint

At handoff, Core authority reports:

- Canon root: `40ec21cb14bc763bc41eb5900e82eb8a455e5e86bd4bd9606431ca12d3eed52c`.
- Chapter 6 ending attempt `task-000010` / `attempt-000012` is `ACCEPTED`; its ending evidence event is canonical `story-event-000012`.
- Active task is now `task-000011`, `chapter:7`; active attempt is `attempt-000013`, reason `initial`, with `planning_repair_required`.
- Active task base Canon root equals the Canon root above.
- `exchange/STATUS.json` reports `export_ready: true` with no `export_problems`; Chapter 7 planning repair is a next-chapter workflow state and does not undo the satisfied final-export hard conditions.
- Historical revision replay has caught the former head and is no longer active.
- Final export: `/opt/sentinelx-cloud-core/novel-core-acceptance/final-export/vultr-product-acceptance-20260916.md`, 15,943 bytes, SHA-256 `6497161b1c20674b80670b23c105908c8ff202637c472db9316789f096c13ca6`; independent reconstruction from active Canon was byte-identical and post-export `novel-core verify` returned `ok:true`.
- Core `serve` was restarted with `-scan-interval 5s`; revision 9 / Canon `40ec21cb…` / `task-000011` / `attempt-000013` remained unchanged and Drive READY/STATUS SHA-256 matched local after restart.

Core serve command used for the formal run:

```sh
/opt/sentinelx-cloud-core/novel-core-acceptance/bin/novel-core serve   -project /opt/sentinelx-cloud-core/novel-core-acceptance/project-product-acceptance   -scan-interval 5s
```

The 5s scan interval is intentional: a 500ms scan interval repeatedly rewrote READY/STATUS/outbox faster than the earlier rclone VFS write-back window and caused transport starvation.

## Acceptance status and next work

`docs/provider-free-release-checklist.md` is the authoritative acceptance ledger. All 15 manual product-chain rows are complete in the current run. Row 8 was independently re-proven from real Drive files in a genuinely new ChatGPT conversation on 2026-09-17; Row 13 completed the real Drive Chapter 6 ending; Row 14 verified the live authority; Row 15 produced and independently byte-verified the final export.

For Chapter 6, do not invent the ending payload from memory. Read the current generated protocol, active outbox, canonical Foundation/ending contract, and existing tests/schema first; then submit through ordinary ChatGPT raw Drive files with manifest last.

## Recovery procedure

1. Restore/fetch the repo and verify local HEAD, tracking HEAD, and GitHub branch HEAD; treat Git as authoritative for design/checklist/handoff state.
2. Read this file and `docs/provider-free-release-checklist.md`.
3. On Vultr, re-read `meta/core/project.json`, `meta/core/production.json`, Canon head, local `exchange/STATUS.json`/`READY.json`, and running processes. Do not assume the runtime checkpoint above is still current if those files disagree.
4. Reconcile the ownership-filtered Drive bridge and independently read real Drive STATUS/READY before creating any submission.
5. All manual rows are complete. `serve` is running again with the original 5s scan interval. Preserve the current authority/Drive identities unless a new protocol-valid task is intentionally submitted.
6. At each stable acceptance milestone, update the release checklist (and this recovery card if the active checkpoint changes), run fresh relevant verification, commit, and push immediately. Non-integration remote CI is asynchronous: trigger if appropriate, but do not wait or poll.
