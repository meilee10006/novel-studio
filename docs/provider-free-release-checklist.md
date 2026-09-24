# Provider-Free Release Checklist

> 本清单中的既有 PASS 记录属于 protocol 1.0 / core schema 1 的
> 2026-09-16 产品验收。protocol 1.1 / core schema 2 的语义设计链
> 必须完成新的真实 ChatGPT App + Google Drive 验收后，才能宣称
> 新流程通过产品验收；不得继承旧 PASS。

This file records final release acceptance status only. It is not a design log or review history.

## Protocol 1.1 / core schema 2 semantic-design acceptance

Automated E2E may validate Core mechanics, but it does not replace the real ordinary ChatGPT App + Google Drive product boundary.

| Check | Status |
| --- | --- |
| Fresh required semantic-design automated E2E | PASS |
| Real ChatGPT App + Google Drive protocol 1.1 product chain | PASS |
| New-session recovery from protocol 1.1 project files | PASS |
| protocol 1.1 verify + backup/restore product acceptance | PASS |

### Protocol 1.1 real product acceptance record

Acceptance completed on 2026-09-25 (+08:00) against the real Google Drive workspace:

`Google Drive/Novel Core Semantic Design Acceptance 2026-09-24/novel-core-workspace/`

- `project_id`: `semantic-product-acceptance-20260924`
- core schema: `2`
- protocol version: `1.1`
- `design_mode`: `required`
- final `design_root`: `db18ebd3e51bcbf91f7bb0d01be0a31b715b61e9a4e3acdca2f4006ecff726b8`
- final `canon_root`: `4845e5e8db57a16f6ee7bf74b1e587c080dd6b9d31610405b167652eb534cb89`
- active production authority after first Canon: `chapter:1` / `attempt-000002`
- story-lock result: `exchange/design/result/accept-s7-story-promote.json`
- foundation-ready result: `exchange/design/result/accept-f10-foundation-promote.json`
- story-lock receipt: `meta/core/design/receipts/accept-s7-story-promote.json`
- foundation-ready receipt: `meta/core/design/receipts/accept-f10-foundation-promote.json`
- Foundation ACCEPTED receipt: `meta/core/receipts/attempt-000001.json`
- production pointer after internal Foundation settlement: `exchange/READY.json`

The ordinary ChatGPT App new-session recovery check was performed without old chat context or a handoff summary. The new session read only the real project files (`project.json`, `CHATGPT_PROTOCOL.md`, `exchange/STATUS.json`, `exchange/READY.json`, plus the referenced current outbox) and independently recovered the same project/protocol/design/Canon roots and the same active `chapter:1` attempt.

`novel-core verify` passed on the original acceptance project. A verified backup containing 100 Core files restored successfully to a new project root; the restored project preserved both final roots above and passed `novel-core verify`. Core serve and the real Drive transport were then restarted, and `STATUS.json` / `READY.json` resumed with the same authority.

One fail-closed case occurred during the product run: the initial `accept-s1-brief` submission used Design manifest schema version `2` instead of machine schema version `1`, and Core rejected it as `INVALID` with `unsupported design manifest schema version 2`. It was corrected and resubmitted as `accept-s1-brief-v2`; no invalid submission was promoted or admitted to Design authority. No unresolved acceptance failures remain.

## Automated acceptance

| Check | Status |
| --- | --- |
| `go test ./...` | PASS |
| `go vet ./...` | PASS |
| `go list -deps ./cmd/novel-core` contains no AI runtime | PASS |
| `novel-core` runs without model credentials | PASS |
| Core capability/Foundation/chapter E2E tests | PASS |
| Self-describing workspace file E2E | PASS |
| Protocol file full-chain E2E | PASS |
| Core restart + file-only resume E2E | PASS |
| REWRITE / new-attempt tests | PASS |
| Structured REWRITE feedback keeps legacy violations and persists deterministic feedback in digest/result/settlement | PASS |
| Backward-compatible complete `chapter_contract` deterministic validation + generated protocol guidance | PASS |
| BLOCKED / block-resolution tests | PASS |
| historical revision + downstream replay tests | PASS |
| rolling planning / future-plan tests | PASS |
| crash recovery / single-writer tests | PASS |
| backup / restore / schema+protocol migration tests | PASS |
| path / symlink / size / JSON-shape security tests | PASS |
| final-export hard-condition tests | PASS |
| `verify` recomputes Canon and active receipt chain | PASS |
| GoReleaser snapshot archives + checksums | PASS |
| Dockerfile arm64 cross-build | PASS |
| Installer GitHub API parsing + checksum enforcement | PASS |
| Apache-2.0 `LICENSE` retained | PASS |

Automated fixtures prove Core mechanics only. They do not prove the ordinary ChatGPT product workflow.

## Manual ChatGPT product acceptance

Each product-chain row uses one of `PASS`, `FAIL`, or `NOT RUN`; completed rows keep their verification evidence inline.

Required environment: ordinary ChatGPT App + real Google Drive + a conforming local Google Drive transport + local `novel-core`.

Transport evidence:
- Implementation/version: PASS — Vultr Linux using rclone v1.75.1. Initial transport acceptance and product rows 1–12 used FUSE3 `rclone mount`; after the FUSE read path stalled, acceptance continued on the same real Drive workspace with an ownership-filtered `rclone copy` bridge (ChatGPT-owned inbox/control paths Drive→local; Core-owned READY/STATUS/outbox/result paths local→Drive). READY/STATUS SHA-256 remained identical across the transport switchover, and the OAuth token remained host-local and outside Git.
- Google Drive workspace: PASS — `Novel Core Product Acceptance 2026-09-16/novel-core-workspace`.
- Byte-preserving probes: PASS — ordinary `.json`, `.md`, and `.txt` files preserved exact bytes/SHA-256 in both tested directions.
- Bidirectional propagation: PASS — Vultr mount -> Google Drive -> ChatGPT readback, and ChatGPT raw `upload_file` -> Google Drive -> Vultr mount.
- Transport restart/recovery: PASS — after stopping Core, unmounting/restarting rclone, remounting, and restarting Core, READY/STATUS SHA-256, Canon root, task, attempt, and target were unchanged.
- Eventual-consistency safety: PASS — partial Foundation uploads (3/7 artifacts, then 7/7 artifacts) produced no settlement without `manifest.json`; uploading manifest last was the transition that allowed ACCEPTED.

| # | Product-chain check | Status |
| ---: | --- | --- |
| 1 | capability check | PASS — ordinary ChatGPT App read the real Drive challenge, uploaded raw `capability-ack.json` + `capability-write-test.md`, Core reached `capability=passed`, and ChatGPT read back READY/STATUS with matching `foundation` / `attempt-000001`. |
| 2 | first-book Foundation accepted | PASS — ChatGPT uploaded the 7 required raw Foundation artifacts plus manifest to the real Drive inbox; Core returned `ACCEPTED`, allocated canonical IDs, advanced Canon to `92dcb26d…`, and ChatGPT read the result back from Drive. |
| 3 | Chapter 1 accepted | PASS — ChatGPT read the canonical Foundation context from Drive, submitted a full Chapter 1 contract/body/events/state/self-review with manifest last, Core returned `ACCEPTED` at Canon `a3c38b11…`, and Drive READY/STATUS converged to `chapter:2` / `attempt-000003`. |
| 4 | continue through Chapter 2+ | PASS — ChatGPT submitted Chapter 2 through real Drive with a full contract and optional `planning_patch.json`; Core returned `ACCEPTED` with `planning_status=accepted`, advanced Canon to `5e6b22b3…`, and Drive READY/STATUS converged to `chapter:3` / `attempt-000004`. |
| 5 | intentionally trigger one deterministic REWRITE | PASS — Chapter 3 intentionally declared a target length excluding the actual body length; Core returned a single `REWRITE` with code `chapter_contract.invalid`, preserved legacy `violations`, limited `allowed_scope` to `chapter_contract.json`, kept Canon at `5e6b22b3…`, and opened `attempt-000005`. |
| 6 | repair on a new attempt and accept | PASS — ChatGPT used the REWRITE feedback to create `attempt-000005`, changed only the allowed `chapter_contract.json` target-length range, kept body/events/state/self-review byte-identical, and Core returned `ACCEPTED` at Canon `ad0db185…`; ChatGPT read the accepted result back from Drive. |
| 7 | restart Core and continue | PASS — the formal product Core was stopped and restarted during the Chapter 3 rewrite at `attempt-000005`; READY/STATUS SHA-256, Canon root, target, and attempt were identical before/after restart, then the repaired attempt was accepted and the chain continued to Chapter 4. |
| 8 | recover from a new ChatGPT conversation by project ID/files | PASS — in this genuinely new ChatGPT conversation, the project was re-located on real Google Drive by matching `project_id=vultr-product-acceptance-20260916` in Drive `project.json`; ChatGPT then read the Drive-native `project.json`, full `CHATGPT_PROTOCOL.md`, `exchange/STATUS.json`, `exchange/READY.json`, and the active `task-000010/attempt-000012` outbox, independently reconstructing `chapter:6`, base Canon `2fcb73a1…`, and the five required Chapter artifacts without relying on prior chat history. |
| 9 | rolling Arc planning takes effect | PASS — Chapter 2 received `rolling_planning_due`, its `planning_patch.json` was accepted, and the real Drive Chapter 4 context shows `current_arc.id=arc-2` with chapters 4–6 and the submitted Arc-2 goal. |
| 10 | `future_plan` author directive takes effect | PASS — while Chapter 4 was active, ChatGPT submitted `ctrl-future-001`; Core returned `ACCEPTED`, Chapter 4 remained `control_constraints:null`, and the real Drive Chapter 5 constraints/context contain the same `future_plan` instruction starting from the next task. |
| 11 | `historical_revision` replays downstream and catches the old head | PASS — ChatGPT submitted `ctrl-revision-001` targeting Chapter 4; Core branched from Chapter 3 Canon, accepted revised Chapter 4 at Canon `ccd3e165…`, issued Chapter 5 as `attempt_reason=rebase`, accepted that replay at Canon `2fcb73a1…`, cleared `revision_replay`, and real Drive READY/STATUS returned to Chapter 6, thereby catching the former accepted head. |
| 12 | trigger BLOCKED and recover with `block_resolution` | PASS — Chapter 5 returned `BLOCKED` with stable `block-2e08d531fb741000` and two exact options; ChatGPT submitted `ctrl-block-001` using the offered choice verbatim, Core opened `attempt-000008` with the resolution constraint, the repaired Chapter 5 was `ACCEPTED` at Canon `1ec6b423…`, and Drive advanced to Chapter 6. |
| 13 | satisfy ending hard conditions | PASS — in Chapter 6, ordinary ChatGPT uploaded the five required raw artifacts to the real Drive inbox and uploaded `manifest.json` last; the ownership-filtered Drive→Vultr bridge exposed that exact attempt to Core, which returned `ACCEPTED`, mapped `e1` to `story-event-000012`, advanced Canon to `40ec21cb…`, and the Core-owned Drive result/STATUS readback converged with `export_ready=true` and no export problems. |
| 14 | `novel-core verify` passes | PASS — the formal Vultr `serve` writer was stopped cleanly because the single-writer design holds an exclusive project lock; `novel-core verify --project /opt/sentinelx-cloud-core/novel-core-acceptance/project-product-acceptance` then returned `ok:true` against the live authority, with production/head still at revision 9 and Canon `40ec21cb…`. |
| 15 | final export succeeds | PASS — with the same Vultr authority offline from `serve`, `novel-core export` produced `/opt/sentinelx-cloud-core/novel-core-acceptance/final-export/vultr-product-acceptance-20260916.md` at Canon `40ec21cb…` with 6 chapters; an independent reconstruction from the six active canonical `chapter.md` artifacts matched the 15,943-byte export exactly at SHA-256 `6497161b1c20674b80670b23c105908c8ff202637c472db9316789f096c13ca6`, post-export `verify` remained `ok:true`, and restarting `serve` at 5s preserved revision 9, Canon, `task-000011` / `attempt-000013`, `export_ready=true`, and Drive READY/STATUS hash identity. |

A release is not product-accepted until all manual rows are PASS. Automated test results must not be copied into this table as substitutes.
