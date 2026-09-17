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

- Canon root: `2fcb73a15d275cb5db98a3d44ce8c25fcdc238ba459724aa22eac1ad2115c204`.
- Active task: `task-000010`, `chapter:6`.
- Active attempt: `attempt-000012`, reason `initial`.
- Base Canon root equals the Canon root above.
- Export is not ready; current problem is `ending resolution is missing`.
- Historical revision replay has caught the former head and is no longer active.

Core serve command used for the formal run:

```sh
/opt/sentinelx-cloud-core/novel-core-acceptance/bin/novel-core serve   -project /opt/sentinelx-cloud-core/novel-core-acceptance/project-product-acceptance   -scan-interval 5s
```

The 5s scan interval is intentional: a 500ms scan interval repeatedly rewrote READY/STATUS/outbox faster than the earlier rclone VFS write-back window and caused transport starvation.

## Acceptance status and next work

`docs/provider-free-release-checklist.md` is the authoritative acceptance ledger. Rows 1-12 are complete in the current run. Row 8 was independently re-proven from real Drive files in a genuinely new ChatGPT conversation on 2026-09-17. Remaining rows are:

- Row 13: satisfy ending hard conditions on Chapter 6.
- Row 14: run `novel-core verify` successfully after the final accepted state.
- Row 15: run final export successfully and verify the produced export.

For Chapter 6, do not invent the ending payload from memory. Read the current generated protocol, active outbox, canonical Foundation/ending contract, and existing tests/schema first; then submit through ordinary ChatGPT raw Drive files with manifest last.

## Recovery procedure

1. Restore/fetch the repo and verify local HEAD, tracking HEAD, and GitHub branch HEAD; treat Git as authoritative for design/checklist/handoff state.
2. Read this file and `docs/provider-free-release-checklist.md`.
3. On Vultr, re-read `meta/core/project.json`, `meta/core/production.json`, Canon head, local `exchange/STATUS.json`/`READY.json`, and running processes. Do not assume the runtime checkpoint above is still current if those files disagree.
4. Reconcile the ownership-filtered Drive bridge and independently read real Drive STATUS/READY before creating any submission.
5. Row 8 is complete; continue Chapter 6 ending -> verify -> export from the current Drive/Core identities, re-reading them before each submission.
6. At each stable acceptance milestone, update the release checklist (and this recovery card if the active checkpoint changes), run fresh relevant verification, commit, and push immediately. Non-integration remote CI is asynchronous: trigger if appropriate, but do not wait or poll.
