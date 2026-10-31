#!/usr/bin/env bash
# Breaking-change check of the OpenAPI contract (SPEC in scripts/harness/layout.env) against the merge base with BASE_REF (merge-base.sh).
# Exit 0 when the spec is new or unchanged in a compatible way; exit 1 with oasdiff's findings when it breaks
# clients (oasdiff exit 1); exit 2 when oasdiff cannot compare the two (any other exit, e.g. 102: cannot load).
# BREAKING_CHANGE_APPROVED=1 (CI: the `breaking-change-approved` PR label, set by a human) still prints the
# findings but exits 0. Used by `make contract-check` (check-all) and .github/workflows/contract-gate.yml.
# The spec is read from HARNESS_ROOT (default: this repo), so CI can judge a PR with the base branch's script.
# A breaking change also names CONTRACT_CONSUMERS (layout.env next to this script, so in CI the base branch's): the
# repos that vendor this contract, which the approval must cover (ADR 0037). `.bin/harness spec-refs` (next to this
# script's repo, so in CI the base branch's, built by make; HARNESS overrides it) checks the $refs.
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=layout.sh
source "$HERE/layout.sh" || exit 2 # SPEC, CONTRACT_CONSUMERS
cd "${HARNESS_ROOT:-$HERE/../..}"
HARNESS="${HARNESS:-$HERE/../../.bin/harness}"
BASE_REF="${BASE_REF:-$("$HERE/merge-base.sh" --ref)}"
base_commit="$(BASE_REF="$BASE_REF" "$HERE/merge-base.sh")"
# One file, internal refs only: oasdiff would follow an external $ref (URL or path) from the PR's spec, and
# a change in a referenced file would not count as a spec change in contract-gate.yml.
status=0
"$HARNESS" spec-refs "$SPEC" || status=$?
if ((status == 1)); then
  echo "[api-no-breaking-change] $SPEC has an external \$ref (above): keep the contract in one file, refs as '#/...'." >&2
  exit 1
elif ((status != 0)); then
  echo "[api-no-breaking-change] cannot check the \$refs of $SPEC (above)." >&2
  exit 2
fi
git cat-file -e "$base_commit:$SPEC" 2>/dev/null || exit 0 # no base contract yet: nothing can break

tmp="$(mktemp -t base-openapi.XXXXXX)"
base_spec="$tmp.yaml" # oasdiff picks the parser by extension
trap 'rm -f "$tmp" "$base_spec"' EXIT
git show "$base_commit:$SPEC" >"$base_spec"

status=0
oasdiff breaking "$base_spec" "$SPEC" --fail-on ERR --format text || status=$?
if ((status > 1)); then # not a verdict, so the human label cannot approve it
  echo "[api-no-breaking-change] cannot compare $SPEC with $BASE_REF ($base_commit): oasdiff exit $status (above). Make both load with oasdiff breaking, then rerun." >&2
  exit 2
elif ((status == 1)); then
  if ((${#CONTRACT_CONSUMERS[@]} > 0)); then
    echo "[api-no-breaking-change] repos that vendor this contract (CONTRACT_CONSUMERS), which the approval must cover: ${CONTRACT_CONSUMERS[*]}" >&2
  fi
  if [[ "${BREAKING_CHANGE_APPROVED:-0}" == 1 ]]; then
    echo "[api-no-breaking-change] breaking change approved by a human (breaking-change-approved); migrate the clients." >&2
    exit 0
  fi
  echo "[api-no-breaking-change] $SPEC breaks clients compared to $BASE_REF ($base_commit)." >&2
  exit 1
fi
