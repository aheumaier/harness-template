#!/usr/bin/env bash
# gen-check: run `make gen` and fail if any generated file changed, appeared or disappeared.
# Hash-based (not `git diff`) so uncommitted hand-written work in progress does not trip it, while
# stale generated code and hand-edits to generated files do. Rule: docs/harness/rules.md#gen-up-to-date
set -euo pipefail
cd "$(dirname "$0")/../.."

# Every generated path in the repo. Keep in sync with `make gen` (docs/harness/sensors.md).
GEN_RE='^(apps/api/internal/.*\.gen\.go|apps/api/internal/[^/]+/store/(db|models|[^/]+\.sql)\.go|apps/api/db/migrations/atlas\.sum|packages/api-client/src/.*)$'

snapshot() {
  git ls-files -co --exclude-standard | grep -E "$GEN_RE" | sort | while read -r f; do
    [[ -f "$f" ]] && shasum -a 256 "$f"
  done || true
}

before="$(snapshot)"
make gen >/dev/null
after="$(snapshot)"

if [[ "$before" != "$after" ]]; then
  echo "Generated files were stale or hand-edited [gen-up-to-date]. \`make gen\` has now rewritten them:"
  diff <(echo "$before") <(echo "$after") | sed -n 's/^[<>] [0-9a-f]*  /  /p' | sort -u
  exit 1
fi
