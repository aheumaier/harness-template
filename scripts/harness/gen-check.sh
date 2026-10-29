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
# shellcheck source=layout.sh
source scripts/harness/layout.sh || exit 2

# Every generated path in the repo (layout.env's GENERATED_GLOBS, plus Atlas's checksum file), fixture overlays
# aside. Keep GENERATED_GLOBS in sync with `make gen`.
GENERATED=("${GENERATED_GLOBS[@]}" "$MIGRATIONS_DIR/atlas.sum" ':!:scripts/harness/fixtures/*')
# What `make gen` reads: sources, generator configs, tool versions and the gen recipes themselves.
INPUTS=("$SPEC" "$QUERIES_DIR" "$MIGRATIONS_DIR" "$DOMAINS_DIR/*oapi-codegen.yaml"
  "${API_PREFIX}sqlc.yaml" "${API_PREFIX}atlas.hcl" "$API_CLIENT_DIR/orval.config.ts" "$API_CLIENT_DIR/package.json"
  pnpm-lock.yaml scripts/tool-versions.env Makefile scripts/harness/gen-check.sh scripts/harness/layout.env)
STAMP=.cache/harness/gen-check.stamp

hashes() {
  while read -r f; do
    [[ -f "$f" ]] && shasum -a 256 "$f"
  done || true
}
snapshot() { git ls-files -co --exclude-standard -- "${GENERATED[@]}" | sort | hashes; }
stamp() { # the effective STACK too: a HARNESS_STACK run of make gen leaves generators out (ADR 0033)
  {
    echo "STACK=$STACK" &&
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
