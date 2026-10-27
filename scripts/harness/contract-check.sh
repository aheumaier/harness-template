#!/usr/bin/env bash
# Breaking-change check of api/openapi.yaml against the merge base with BASE_REF (merge-base.sh).
# Exit 0 when the spec is new or unchanged in a compatible way; exit 1 with oasdiff's findings otherwise.
# BREAKING_CHANGE_APPROVED=1 (CI: the `breaking-change-approved` PR label, set by a human) still prints the
# findings but exits 0. Used by `make contract-check` (check-all) and .github/workflows/contract-gate.yml.
# The spec is read from HARNESS_ROOT (default: this repo), so CI can judge a PR with the base branch's script.
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
SPEC=api/openapi.yaml
cd "${HARNESS_ROOT:-$HERE/../..}"
BASE_REF="${BASE_REF:-$("$HERE/merge-base.sh" --ref)}"
base_commit="$(BASE_REF="$BASE_REF" "$HERE/merge-base.sh")"
# One file, internal refs only: oasdiff would follow an external $ref (URL or path) from the PR's spec, and
# a change in a referenced file would not count as a spec change in contract-gate.yml.
if grep -nE '\$ref["'"'"']?:[[:space:]]*["'"'"']?[^"'"'"'#[:space:]]' "$SPEC"; then
  echo "[api-no-breaking-change] $SPEC has an external \$ref (above): keep the contract in one file, refs as '#/...'." >&2
  exit 1
fi
git cat-file -e "$base_commit:$SPEC" 2>/dev/null || exit 0 # no base contract yet: nothing can break

tmp="$(mktemp -t base-openapi.XXXXXX)"
base_spec="$tmp.yaml" # oasdiff picks the parser by extension
trap 'rm -f "$tmp" "$base_spec"' EXIT
git show "$base_commit:$SPEC" >"$base_spec"

if ! oasdiff breaking "$base_spec" "$SPEC" --fail-on ERR --format text; then
  if [[ "${BREAKING_CHANGE_APPROVED:-0}" == 1 ]]; then
    echo "[api-no-breaking-change] breaking change approved by a human (breaking-change-approved); migrate the clients." >&2
    exit 0
  fi
  echo "[api-no-breaking-change] $SPEC breaks clients compared to $BASE_REF ($base_commit)." >&2
  exit 1
fi
