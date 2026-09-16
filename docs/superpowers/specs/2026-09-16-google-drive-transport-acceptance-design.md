# Google Drive Transport Acceptance Design

## 1. Decision

Provider-free Novel Core release acceptance no longer requires Google Drive Desktop specifically.
The supported production boundary is:

> ordinary ChatGPT App + real Google Drive cloud + a conforming local Google Drive transport + local `novel-core`.

This changes the transport/release-acceptance boundary only. It does not change Core authority, protocol ownership, AI boundaries, or Canon semantics.

`meta/core/**` in the local project remains the sole authority. Google Drive and every transport implementation remain non-authoritative exchange layers.

## 2. Supported transport implementations

A transport implementation is supported when it satisfies the contract in this design.

First-release supported examples:

- macOS / Windows: Google Drive Desktop exposing the workspace as ordinary local files;
- Linux / Vultr: rclone Google Drive transport, with `rclone mount` + VFS cache as the preferred acceptance implementation;
- future implementations may be accepted without changing the product architecture if they satisfy the same contract.

Drive Desktop remains fully supported on Mac and Windows. It is no longer the only implementation allowed by the release gate.

The release checklist must record the transport implementation and version used for the manual product run.

## 3. Transport contract

A conforming local Google Drive transport MUST satisfy all of the following.

### 3.1 Real Google Drive cloud

The remote side MUST be a real Google Drive folder accessible to the ordinary ChatGPT App account used for acceptance.
A local-only mirror, fake Drive fixture, temporary filesystem, or automated test double does not qualify.

### 3.2 Ordinary-file fidelity

The workspace MUST expose ordinary UTF-8 `.json`, `.md`, and `.txt` files.
The transport MUST preserve file names and file bytes; it MUST NOT convert protocol files to Google Docs, Sheets, Slides, or another rich-document representation.

### 3.3 Bidirectional propagation

Acceptance MUST demonstrate both directions on the same real workspace:

1. ChatGPT App -> Google Drive -> local transport -> Core-visible filesystem;
2. Core-visible filesystem -> local transport -> Google Drive -> ChatGPT App.

A one-way uploader/downloader does not qualify as the production transport.

### 3.4 Eventual consistency and non-atomic multi-file behavior

The transport may be eventually consistent and may expose multi-file updates non-atomically.
Core MUST continue to rely on existing READY/STATUS identity checks, stable-file observation, immutable attempt binding, local snapshots, and conflict handling rather than transport mtimes or arrival order.

The transport MUST eventually converge without changing file contents.

### 3.5 Restart and recovery

Acceptance MUST restart the transport process/service and `novel-core`, then continue against the same Google Drive workspace without changing the authoritative Canon root or active task/attempt identity except through valid protocol transitions.

### 3.6 No transport authority

Transport caches, remote mtimes, mount metadata, logs, implementation-specific IDs, and sync state are never Canon authority and are excluded from the Canon content root.

## 4. Vultr / rclone acceptance profile

For Linux/Vultr release acceptance, the preferred implementation is:

```text
ordinary ChatGPT App
        |
        v
real Google Drive folder
        |
        v
rclone Google Drive remote
        |
        v
rclone mount + VFS cache
        |
        v
local workspace directory
        |
        v
novel-core
        |
        v
local project/meta/core/** (authority)
```

Recommended profile:

- rclone Google Drive backend authenticated to the same Drive account/folder used by ChatGPT App;
- remote rooted to the dedicated acceptance workspace where practical;
- `rclone mount` using `--vfs-cache-mode full` for normal filesystem write semantics;
- local Core project directory outside the mount;
- transport and Core restart tested independently;
- transport implementation/version captured in acceptance evidence.

The rclone config/token is host-local secret material and MUST NOT be committed to Git.

## 5. Mac / Windows profile

Mac and Windows remain supported production platforms.

Google Drive Desktop is a conforming implementation when the same contract is demonstrated against its local synchronized workspace. No rclone dependency is required on Mac or Windows.

A Mac may also use rclone if desired; acceptance is based on the transport contract, not the brand of transport software.

## 6. Capability check interpretation

`novel-core init` capability challenge remains unchanged.
The product acceptance interpretation changes from "Drive Desktop copied the files" to "the selected conforming Drive transport propagated the files".

Capability acceptance must demonstrate:

- ChatGPT App can read the project protocol files from real Google Drive;
- ChatGPT App can create the required ordinary UTF-8 JSON/Markdown response files;
- the selected transport propagates those bytes to the local Core workspace;
- Core accepts the nonce-bound capability response;
- Core-generated READY/STATUS/outbox files propagate back through Google Drive and can be read by ChatGPT App.

## 7. Release acceptance

The 15 manual product-chain checks remain required and retain their current meanings.
They may be marked `PASS` only when executed using:

- ordinary ChatGPT App as the sole AI/creative layer;
- one real Google Drive workspace;
- one conforming local Google Drive transport;
- local `novel-core` against that transport workspace.

Automated fixtures and local-only preflight runs still cannot substitute for this product run.

Before row 1 is marked `PASS`, transport evidence MUST record:

- transport implementation and version;
- Google Drive workspace identity or acceptance-folder name;
- successful byte-preserving probes for `.json`, `.md`, and `.txt` where used;
- successful bidirectional propagation;
- transport restart/recovery result.

A release is not product-accepted until the transport contract passes and all 15 manual rows are `PASS`.

## 8. Documentation changes required

The implementation phase updates product documentation so that:

- `README.md` and `README_EN.md` describe Google Drive transport generically and list Drive Desktop/rclone as implementations;
- `README-TECHNICAL.md` describes a conforming Google Drive transport rather than requiring Drive Desktop;
- the original provider-free design replaces Drive-Desktop-specific release-gate wording with the contract defined here;
- the protocol-completion design/plan no longer names Drive Desktop as mandatory acceptance infrastructure;
- `docs/provider-free-release-checklist.md` records the actual transport implementation/version and uses the generic required environment.

No Core protocol version bump is required because the file protocol and authority model do not change.

## 9. Non-goals

This change does not:

- make Google Drive optional;
- allow non-Drive cloud storage to satisfy the release gate;
- move authority into Google Drive, rclone, or Drive Desktop;
- add rclone as a runtime dependency of `novel-core`;
- add provider/model APIs, browser automation, MCP, Ollama, or background AI generation;
- permit Google Docs/Sheets/Slides as protocol-file substitutes;
- convert automated fixture PASS results into manual product acceptance.
