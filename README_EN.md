# novel-studio — Provider-Free Novel Core

This fork is a **local-first, provider-free authority core for long-form fiction**.

The supported first-release stack is intentionally small:

- the ordinary **ChatGPT App** for discussion, planning, prose, rewrites, and file submissions;
- **Google Drive** as an exchange/backup transport only;
- **Drive Desktop** to expose that workspace as ordinary local files;
- local **`novel-core`** for deterministic state, validation, transactions, recovery, revision, backup, and export.

`novel-core` does not call OpenAI APIs or any other model provider. It requires no provider configuration, API key, Ollama, MCP, ChatGPT Work, embeddings, or Qdrant. Creative quality remains a responsibility of the author and ChatGPT; Core enforces only mechanically provable structured constraints.

## Quick start

Requirements: Go or a `novel-core` release binary, Google Drive + Drive Desktop, and an ordinary ChatGPT App conversation.

```bash
git clone https://github.com/meilee10006/novel-studio.git
cd novel-studio
git switch provider-free-novel-core
./scripts/run-local.sh --help
```

Initialize a local authority directory and a Drive workspace:

```bash
novel-core init \
  --project "$HOME/novels/book-local" \
  --workspace "$HOME/Google Drive/My Drive/novel-book" \
  --project-id book-001
```

`init` generates the current `CHATGPT_PROTOCOL.md` plus a capability challenge inside the Drive workspace. Do not maintain a second handwritten protocol in the repository.

Give `CHATGPT_PROTOCOL.md` and `setup/capability-challenge.json` to ChatGPT App. After ChatGPT writes the requested acknowledgement and Markdown probe, run:

```bash
novel-core serve --project "$HOME/novels/book-local"
```

Use `exchange/READY.json` as the only pointer to the current task. `exchange/STATUS.json` exposes the current authoritative `canon_root`, capability state, and active task/attempt/block. ChatGPT reads the matching outbox task/context and writes the submission to `exchange/inbox/<task-id>/<attempt-id>/`, with `manifest.json` written last.

For Chapter/Revision tasks, `context.json.foundation_reference` carries the normalized Foundation reference and canonical character/location IDs. A fresh ChatGPT conversation can therefore recover from Core-generated files instead of relying on prior chat history. Control messages must copy `base_canon_root` from the current `exchange/STATUS.json.canon_root`, not from a historical revision READY.

Core snapshots stable bytes locally, validates them, and returns one of `ACCEPTED`, `REWRITE`, or `BLOCKED`. A rewrite keeps the task but creates a fresh attempt. Author directives and block decisions travel through `exchange/control/inbox/`.

## Revision and replay

A `future_plan` directive affects future planning only. A `historical_revision` creates a new authority branch at the earliest affected accepted chapter, supersedes old downstream chapters, and replays them in order. Final export is blocked until replay catches up with the former head.

## Verify, restore, migrate, export

```bash
novel-core status  --project "$HOME/novels/book-local"
novel-core verify  --project "$HOME/novels/book-local"
novel-core restore --backup /path/to/backup --project "$HOME/novels/restored-book"
novel-core migrate --project "$HOME/novels/book-local" --backup /path/to/pre-migration-backup
novel-core export  --project "$HOME/novels/book-local" --out "$HOME/novels/book-final.md"
```

Restore never overwrites an existing target. Known migrations require a verified pre-migration backup and are idempotent; unknown future schema/protocol versions are rejected. A protocol-only upgrade must not change the novel Canon root.

Final export reads only the active Canon inventory and is rejected while historical replay is unsettled or deterministic ending obligations remain open.

## Notion projection

Notion is optional and read-only. Core can write `projection/notion.json` for an external Notion sync. Projection failures never block authority commits, and edits made in Notion never flow back into local authority state.

## Safety

Drive input is untrusted. Core rejects traversal, absolute paths, symlinks, unknown artifacts, invalid UTF-8, oversized inputs, excessive JSON depth, and oversized arrays. Unknown files never enter Canon.

A cross-process project lock enforces a single writer. `status` stays concurrently readable; verify/export use stable read snapshots.

## Supported CLI

```text
novel-core init
novel-core serve
novel-core status
novel-core verify
novel-core export
novel-core restore
novel-core migrate
novel-core --version
```

There are no `--provider`, `--model`, or `--api-key` options.

## Scope

The first release is single-author and single-active-revision-chain. It does not provide multi-user live editing, branch merging, automatic literary-quality scoring, or guarantees about acceptance/traffic on any publishing platform.

The retained `assets/references/` and `assets/styles/` directories are writing references, not model-runtime dependencies.

## Upstream and license

This fork originates from [Xiaoyangy/novel-studio](https://github.com/Xiaoyangy/novel-studio) and retains the repository's **Apache-2.0** license and required attribution. The provider-free architecture is maintained by this fork and does not imply endorsement by the upstream authors.

Current fork: `meilee10006/novel-studio`.
