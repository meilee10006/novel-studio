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
| REWRITE / new-attempt tests | PASS |
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
| Apache-2.0 `LICENSE` retained | PASS |

Automated fixtures prove Core mechanics only. They do not prove the ordinary ChatGPT product workflow.

## Manual ChatGPT product acceptance

Required environment: ordinary ChatGPT App + Google Drive + Drive Desktop + local `novel-core`.

| # | Product-chain check | Status |
| ---: | --- | --- |
| 1 | capability check | NOT RUN |
| 2 | first-book Foundation accepted | NOT RUN |
| 3 | Chapter 1 accepted | NOT RUN |
| 4 | continue through Chapter 2+ | NOT RUN |
| 5 | intentionally trigger one deterministic REWRITE | NOT RUN |
| 6 | repair on a new attempt and accept | NOT RUN |
| 7 | restart Core and continue | NOT RUN |
| 8 | recover from a new ChatGPT conversation by project ID/files | NOT RUN |
| 9 | rolling Arc planning takes effect | NOT RUN |
| 10 | `future_plan` author directive takes effect | NOT RUN |
| 11 | `historical_revision` replays downstream and catches the old head | NOT RUN |
| 12 | trigger BLOCKED and recover with `block_resolution` | NOT RUN |
| 13 | satisfy ending hard conditions | NOT RUN |
| 14 | `novel-core verify` passes | NOT RUN |
| 15 | final export succeeds | NOT RUN |

A release is not product-accepted until all manual rows are PASS. Automated test results must not be copied into this table as substitutes.
