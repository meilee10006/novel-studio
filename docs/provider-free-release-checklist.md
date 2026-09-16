# Provider-Free Release Checklist

This file records final release acceptance status only. It is not a design log or review history.

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

Required environment: ordinary ChatGPT App + real Google Drive + a conforming local Google Drive transport + local `novel-core`.

Transport evidence:
- Implementation/version: PASS — Vultr Linux using rclone v1.75.1 + FUSE3 3.14.0, `rclone mount --vfs-cache-mode full`; OAuth token remained host-local and outside Git.
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
| 8 | recover from a new ChatGPT conversation by project ID/files | NOT RUN |
| 9 | rolling Arc planning takes effect | PASS — Chapter 2 received `rolling_planning_due`, its `planning_patch.json` was accepted, and the real Drive Chapter 4 context shows `current_arc.id=arc-2` with chapters 4–6 and the submitted Arc-2 goal. |
| 10 | `future_plan` author directive takes effect | PASS — while Chapter 4 was active, ChatGPT submitted `ctrl-future-001`; Core returned `ACCEPTED`, Chapter 4 remained `control_constraints:null`, and the real Drive Chapter 5 constraints/context contain the same `future_plan` instruction starting from the next task. |
| 11 | `historical_revision` replays downstream and catches the old head | NOT RUN |
| 12 | trigger BLOCKED and recover with `block_resolution` | PASS — Chapter 5 returned `BLOCKED` with stable `block-2e08d531fb741000` and two exact options; ChatGPT submitted `ctrl-block-001` using the offered choice verbatim, Core opened `attempt-000008` with the resolution constraint, the repaired Chapter 5 was `ACCEPTED` at Canon `1ec6b423…`, and Drive advanced to Chapter 6. |
| 13 | satisfy ending hard conditions | NOT RUN |
| 14 | `novel-core verify` passes | NOT RUN |
| 15 | final export succeeds | NOT RUN |

A release is not product-accepted until all manual rows are PASS. Automated test results must not be copied into this table as substitutes.
