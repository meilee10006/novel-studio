#!/usr/bin/env bash
# provider-free Novel Core local runner.
# Examples:
#   ./scripts/run-local.sh --help
#   ./scripts/run-local.sh init --project ./local --workspace ./drive --project-id book-1
#   ./scripts/run-local.sh serve --project ./local
#   ./scripts/run-local.sh status --project ./local
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
BIN=(go run ./cmd/novel-core)
if [ "$#" -eq 0 ]; then
    exec "${BIN[@]}" --help
fi
exec "${BIN[@]}" "$@"
