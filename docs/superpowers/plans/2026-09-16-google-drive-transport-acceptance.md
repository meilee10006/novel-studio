# Google Drive Transport Acceptance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the Drive-Desktop-only release gate with an implementation-independent Google Drive transport contract, then complete real product acceptance on Vultr using rclone without removing Mac/Windows Drive Desktop support.

**Architecture:** `novel-core` still consumes an ordinary-file workspace while local `meta/core/**` remains sole authority. Acceptance requires ordinary ChatGPT App + real Google Drive + a conforming local Drive transport; Drive Desktop and rclone are implementations, never Core dependencies.

**Tech Stack:** Go 1.25.x, Markdown, Google Drive, rclone Drive backend, FUSE3.

**Spec:** `docs/superpowers/specs/2026-09-16-google-drive-transport-acceptance-design.md`

## Global Constraints

- Google Drive remains mandatory; ordinary ChatGPT App remains the sole AI/creative layer.
- Mac/Windows Drive Desktop remains supported; Linux/Vultr rclone is an additional conforming implementation.
- `meta/core/**` remains sole authority; no Core protocol bump or rclone runtime dependency.
- Protocol files remain ordinary UTF-8 `.json/.md/.txt`; rclone tokens/OAuth secrets never enter Git.
- Manual rows remain `NOT RUN` until actually executed over real Google Drive.
- Remote CI stays asynchronous; do not wait/poll.

---

### Task 1: Migrate product documentation

**Files:** `README.md`, `README_EN.md`, `README-TECHNICAL.md`, `docs/provider-free-release-checklist.md`, the transport design, and the protocol-completion design. Older provider-free specs/plans remain historical records and are superseded by the transport design.

**Produces:** consistent boundary: `ordinary ChatGPT App + real Google Drive + conforming local Google Drive transport + local novel-core`.

- [ ] **Step 1: Verify RED wording exists**

```bash
grep -nE 'Required environment:.*Drive Desktop|requires a separate manual product run using.*Drive Desktop|发布前必须使用普通 ChatGPT App \+ Google Drive \+ Drive Desktop|首发组合只有普通 ChatGPT App \+ Google Drive \+ Drive Desktop' \
  README.md README_EN.md README-TECHNICAL.md docs/provider-free-release-checklist.md \
  docs/superpowers/specs/2026-09-16-provider-free-protocol-completion-design.md
```

Expected: one or more mandatory Drive-Desktop-only matches.

- [ ] **Step 2: Apply documentation migration**

All mandatory release wording uses the generic transport boundary. README guidance must explicitly retain:

```text
macOS/Windows: Google Drive Desktop is supported.
Linux/Vultr: rclone Google Drive transport is supported; rclone mount + VFS cache is preferred for acceptance.
```

Replace fixed `Drive Desktop` architecture edges/capability wording with `conforming local Drive transport`, naming Drive Desktop/rclone as implementations.

Add to `docs/provider-free-release-checklist.md` before the 15 rows:

```markdown
Transport evidence:
- Implementation/version: NOT RUN
- Google Drive workspace: NOT RUN
- Byte-preserving probes: NOT RUN
- Bidirectional propagation: NOT RUN
- Transport restart/recovery: NOT RUN
```

Do not change the 15 manual row statuses.

- [ ] **Step 3: Verify documentation GREEN**

Re-run Step 1 prefixed with `!`; then verify both implementations remain documented and manual row count is unchanged:

```bash
grep -n 'Drive Desktop' README.md README_EN.md docs/superpowers/specs/2026-09-16-google-drive-transport-acceptance-design.md
grep -n 'rclone' README.md README_EN.md docs/superpowers/specs/2026-09-16-google-drive-transport-acceptance-design.md
test "$(grep -c '| NOT RUN |' docs/provider-free-release-checklist.md)" -ge 15
git diff --check
```

- [ ] **Step 4: Run fresh full gates**

```bash
gofmt -l $(find . -name '*.go' -not -path './.git/*')
go test ./... -count=1
go vet ./...
go build -trimpath -o /tmp/novel-core ./cmd/novel-core
go list -deps ./cmd/novel-core | grep -E 'agentcore|litellm|internal/agents|internal/llmcodex|internal/bootstrap' && exit 1 || true
git diff --check
go test -race -count=1 -timeout=10m ./internal/core ./internal/store ./internal/protocol
```

Expected: all PASS; no Go file needs formatting.

- [ ] **Step 5: Commit/push**

Commit `docs: generalize google drive transport acceptance`; push non-force; verify local/tracking/remote tree identity; do not poll CI.

---

### Task 2: Configure Vultr rclone and prove transport contract

**Git:** no credentials/config in repo. Host paths: `/srv/novel-core-acceptance/{drive,project}` and host rclone config/cache.

- [ ] **Step 1: Install transport**

```bash
sudo apt-get update && sudo apt-get install -y fuse3 curl unzip
curl https://rclone.org/install.sh | sudo bash
rclone version
fusermount3 --version
```

Record exact rclone version.

- [ ] **Step 2: Authenticate real Google Drive remote**

Configure `gdrive` using the real acceptance Google account/folder; prefer rooting at the dedicated acceptance workspace. Secrets stay outside Git. Verify:

```bash
rclone config file
rclone lsd gdrive:
rclone lsjson gdrive:
```

If OAuth needs user authorization, stop only at that authorization boundary, complete the Google login, then resume; never substitute another account/fake filesystem.

- [ ] **Step 3: Mount workspace**

```bash
sudo mkdir -p /srv/novel-core-acceptance/{drive,project} /var/cache/rclone/novel-core-acceptance
sudo chown -R "$USER:$USER" /srv/novel-core-acceptance /var/cache/rclone/novel-core-acceptance
rclone mount gdrive: /srv/novel-core-acceptance/drive \
  --vfs-cache-mode full --cache-dir /var/cache/rclone/novel-core-acceptance \
  --vfs-cache-max-size 2G --vfs-cache-max-age 24h --vfs-write-back 2s \
  --dir-cache-time 30s --poll-interval 5s --buffer-size 8M \
  --log-file /var/log/rclone-novel-core-acceptance.log --log-level INFO --daemon
```

- [ ] **Step 4: Prove local -> Drive -> ChatGPT byte fidelity**

Write `.json/.md/.txt` via mount, record SHA-256, read the same ordinary files through ChatGPT's real Drive connection, and compare exact content. No Google Docs conversion.

- [ ] **Step 5: Prove ChatGPT -> Drive -> local byte fidelity**

Create distinct ordinary `.json/.md/.txt` through ChatGPT/Drive; verify names/content/SHA on `/srv/novel-core-acceptance/drive`.

- [ ] **Step 6: Prove eventual-consistency safety**

Initialize Core against the real mount; stage a multi-file submission with manifest last. Confirm Core does not settle incomplete/unstable content and only consumes the stable READY-bound attempt.

- [ ] **Step 7: Prove restart/recovery**

Record READY/STATUS/Canon; restart rclone mount and `novel-core`; after convergence verify authority and active identity did not drift without a valid transition.

- [ ] **Step 8: Record evidence**

Update only the transport evidence fields from actual observations; leave product rows unchanged unless executed. `git diff --check`; commit `docs: record vultr drive transport acceptance`; push/verify without CI polling.

---

### Task 3: Run 15-item real product acceptance on Vultr

**Consumes:** PASS transport contract. **Produces:** evidence-backed PASS/FAIL for the existing 15 rows.

- [ ] **Step 1:** capability handshake through ordinary ChatGPT App + real Drive + rclone; require Core `capability=passed` and ChatGPT readback of READY/STATUS.
- [ ] **Step 2:** Foundation, Chapter 1, Chapter 2+ through the real Drive workspace; mark rows 2-4 only from actual settlements.
- [ ] **Step 3:** intentionally trigger deterministic REWRITE, inspect structured feedback, repair only on new attempt, ACCEPTED; rows 5-6.
- [ ] **Step 4:** restart Core and continue; then recover in a fresh ChatGPT conversation using only project/protocol/STATUS/READY/outbox; rows 7-8.
- [ ] **Step 5:** prove rolling Arc planning and `future_plan` applying only to future tasks; rows 9-10.
- [ ] **Step 6:** run `historical_revision` through downstream replay and a separate BLOCKED -> exact `block_resolution` -> new attempt flow; rows 11-12.
- [ ] **Step 7:** satisfy ending hard conditions, then run:

```bash
novel-core verify --project /srv/novel-core-acceptance/project
novel-core export --project /srv/novel-core-acceptance/project
```

Mark rows 13-15 only from actual results.

- [ ] **Step 8:** update checklist strictly from observed evidence, run fresh final repo gates, commit `docs: record provider-free product acceptance`, push and verify local/tracking/remote; never convert local preflight/fixtures into manual PASS.
