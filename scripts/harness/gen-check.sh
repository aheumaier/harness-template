#!/usr/bin/env bash
# gen-check: run `make gen` and fail if any generated file changed, appeared or disappeared.
# Hash-based (not `git diff`) so uncommitted hand-written work in progress does not trip it, while
# stale generated code and hand-edits to generated files do. Rule: docs/harness/rules.md#gen-up-to-date
#
#   gen-check.sh               regenerate and compare; on green, stamp the generator inputs and outputs
#   gen-check.sh --unchanged   exit 0 when scope is `changed` and the stamp matches (make check skips),
#                              else 1: no stamp, a changed input or output, scope `full`, or any error
#                              (rules.md#harness-scope)
set -euo pipefail
cd "$(dirname "$0")/../.."

# Every generated path in the repo. Keep in sync with `make gen`.
GEN_RE='^(apps/api/internal/.*\.gen\.go|apps/api/internal/[^/]+/store/(db|models|[^/]+\.sql)\.go|apps/api/db/migrations/atlas\.sum|packages/api-client/src/gen/.*)$'
# What `make gen` reads: sources, generator configs, tool versions and the gen recipes themselves.
INPUTS=(api/openapi.yaml apps/api/db/queries apps/api/db/migrations 'apps/api/internal/*oapi-codegen.yaml'
  apps/api/sqlc.yaml apps/api/atlas.hcl packages/api-client/orval.config.ts packages/api-client/package.json
  pnpm-lock.yaml scripts/tool-versions.env Makefile scripts/harness/gen-check.sh)
STAMP=.cache/harness/gen-check.stamp

hashes() {
  while read -r f; do
    [[ -f "$f" ]] && shasum -a 256 "$f"
  done || true
}
snapshot() { git ls-files -co --exclude-standard | grep -E "$GEN_RE" | sort | hashes; }
stamp() {
  {
    git ls-files -co --exclude-standard -- "${INPUTS[@]}" | sort | hashes &&
      snapshot
  } | shasum -a 256
}

if [[ "${1:-}" == "--unchanged" ]]; then
  [[ "${HARNESS_SCOPE:-changed}" == changed && -f "$STAMP" ]] || exit 1
  current="$(stamp)" || exit 1
  [[ "$current" == "$(cat "$STAMP")" ]]
  exit
fi

# Stamped before regenerating: an edit made while `make gen` runs leaves the stamp stale, not wrongly green.
pre="$(stamp)" || pre=""
before="$(snapshot)"
make gen >/dev/null
after="$(snapshot)"

if [[ "$before" != "$after" ]]; then
  echo "Generated files were stale or hand-edited [gen-up-to-date]. \`make gen\` has now rewritten them:"
  diff <(echo "$before") <(echo "$after") | sed -n 's/^[<>] [0-9a-f]*  /  /p' | sort -u
  exit 1
fi
# Best effort: without a stamp the next make check only regenerates again.
if [[ -n "$pre" ]] && ! { mkdir -p "$(dirname "$STAMP")" && echo "$pre" >"$STAMP"; } 2>/dev/null; then
  rm -f "$STAMP" 2>/dev/null || true
fi
