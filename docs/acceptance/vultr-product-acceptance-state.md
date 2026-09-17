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

On 2026-09-17, live rclone operations also emitted a notice that the shared Google Drive `client_id` used by this remote is being retired during 2026. During the Chapter 8 continuation, that shared client also began returning Google Drive `403 rateLimitExceeded` query-quota errors and remained rate-limited after a bounded backoff. The already-connected Google Drive transport was used as a one-run fallback to finish the ChatGPT-owned Chapter 8 inbox upload without changing Core authority or protocol. Migrate the host-local rclone configuration to a dedicated client ID when transport maintenance is scheduled; never commit credentials or OAuth tokens.

## Current verified runtime checkpoint

Freshly re-verified on 2026-09-17 after the completed product-acceptance run:

- Core `serve` is running as `/opt/sentinelx-cloud-core/novel-core-acceptance/bin/novel-core serve -project /opt/sentinelx-cloud-core/novel-core-acceptance/project-product-acceptance -scan-interval 5s`. Keep the 5s scan interval unless there is fresh evidence to change it; the earlier 500ms interval caused READY/STATUS/outbox churn faster than the Drive transport could settle.
- Live authority is revision 11 at Canon root `61b52e6f4b99cceb4e82b1f038db61f56256f5f6c3336c3b33f2a2e72bbe1da9`.
- Post-acceptance Chapter 8 task `task-000012` / attempt `attempt-000014` is `ACCEPTED`; local event `e1` became canonical `story-event-000014` and planning status was `not_present` as expected.
- Chapter 8 intentionally exercised the current per-chapter length contract without changing code: `target_length` was `2000..2400` Unicode runes and the accepted `chapter.md` measured 2020 runes. This proves an approximately-2200-character Chapter can be enforced today through `chapter_contract.json`.
- Active task is now `task-000013`, `chapter:9`; active attempt is `attempt-000015`, reason `initial`, base Canon root `61b52e6f4b99cceb4e82b1f038db61f56256f5f6c3336c3b33f2a2e72bbe1da9`. Re-read its live outbox before writing; do not infer its required artifacts from this checkpoint.
- Live `exchange/STATUS.json` now reports `export_ready: false` with the sole problem `ending resolution is from chapter 6, not latest accepted chapter 8`. This is the expected consequence of continuing the live novel after the already-completed acceptance export; it does not retroactively invalidate the historical product-acceptance evidence.
- The formal acceptance export remains the historical artifact `/opt/sentinelx-cloud-core/novel-core-acceptance/final-export/vultr-product-acceptance-20260916.md`, 15,943 bytes, SHA-256 `6497161b1c20674b80670b23c105908c8ff202637c472db9316789f096c13ca6`, produced at the former acceptance Canon `40ec21cb14bc763bc41eb5900e82eb8a455e5e86bd4bd9606431ca12d3eed52c`.
- Chapter 7 was submitted through the real Drive transport with the six non-manifest artifacts copied first and `manifest.json` last; Core accepted it normally. Core-owned `exchange/result/attempt-000013.json`, `published/chapters/000007.md`, `projection/notion.json`, current READY/STATUS, and the complete `task-000012/attempt-000014` outbox were then copied back only through their ownership-allowed Drive paths. Local and real-Drive SHA-256 matched for every propagated file.
- Chapter 8 was also submitted through the real Google Drive workspace. Rclone uploaded `chapter.md`, `chapter_contract.json`, and `events.json` before its shared-client quota failed; the connected Google Drive transport then uploaded `self_review.json`, `state_delta.json`, and `manifest.json`, with the manifest still written last. All five non-manifest artifacts were read back from Drive and matched the staged SHA-256 values; `manifest.json` read back with SHA-256 `442d2d4749830f405391f3226ba1b42f780d38dc255ae0aef9e2dd46165b602b`. The verified identical bytes were bridged into the local ChatGPT-owned inbox with local manifest-last ordering, and Core settled attempt `attempt-000014` as `ACCEPTED`.
- Rclone remained quota-limited when attempting the post-acceptance Core-owned result propagation, so do not assume real Drive READY/STATUS/outbox have caught up to revision 11. Before Chapter 9, reconcile Core-owned outputs to Drive and independently require Drive and local task identities to match. This is a transport lag, not a Canon or acceptance failure.
- No persistent rclone process is required. Keep ownership-filtered transport and do not replace it with whole-workspace bidirectional sync.

## Acceptance status and next work

`docs/provider-free-release-checklist.md` remains the authoritative acceptance ledger. All 15 manual product-chain rows are complete in the accepted run; do not repeat those rows merely because the live novel has continued beyond the accepted export checkpoint.

The live post-acceptance chain has now advanced through Chapter 8 and local Core is ready for Chapter 9. Before creating any Chapter 9 submission, first reconcile the currently lagging Core-owned Drive outputs, then re-read the current generated protocol, READY/STATUS, and `task-000013/attempt-000015` outbox from both real Drive and local Core authority and require their identities to agree. Do not infer Chapter 9 artifacts or planning obligations from the Chapter 8 task.

The historical final export at Canon `40ec21cb…` is still the completed product-acceptance artifact. The current live Canon is newer, so `export_ready:false` is a live-authority condition that must be resolved by future story work before producing a new current final export; it is not an acceptance regression.

## Recovery procedure

1. Restore/fetch the repo and verify local HEAD, tracking HEAD, and GitHub branch HEAD; treat Git as authoritative for design/checklist/handoff state.
2. Read this file and `docs/provider-free-release-checklist.md`.
3. On Vultr, re-read `meta/core/project.json`, `meta/core/production.json`, Canon head, local `exchange/STATUS.json`/`READY.json`, and running processes. Do not assume the runtime checkpoint above is still current if those files disagree.
4. Reconcile the ownership-filtered Drive bridge and independently read real Drive STATUS/READY before creating any submission.
5. All manual rows are complete. `serve` is running again with the original 5s scan interval. Preserve the current authority/Drive identities unless a new protocol-valid task is intentionally submitted.
6. At each stable post-acceptance runtime milestone, update this recovery card when the active checkpoint changes; update the release checklist only if acceptance evidence itself changes. Run fresh relevant verification, commit, and push immediately. Non-integration remote CI is asynchronous: trigger if appropriate, but do not wait or poll.
