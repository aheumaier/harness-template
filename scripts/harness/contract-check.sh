#!/usr/bin/env bash
# Breaking-change check of api/openapi.yaml against the merge base with BASE_REF (default main).
# Exit 0 when the spec is new or unchanged in a compatible way; exit 1 with oasdiff's findings otherwise.
# Used by `make contract-check` (check-all) and .github/workflows/contract-gate.yml.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
SPEC=api/openapi.yaml
BASE_REF="${BASE_REF:-main}"
cd "$ROOT"

base_commit="$(git merge-base HEAD "$BASE_REF" 2>/dev/null || true)"
if [[ -z "$base_commit" ]] || ! git cat-file -e "$base_commit:$SPEC" 2>/dev/null; then
  exit 0 # no base contract yet: nothing can break
fi

base_spec="$(mktemp -t base-openapi.XXXXXX).yaml"
trap 'rm -f "$base_spec"' EXIT
git show "$base_commit:$SPEC" >"$base_spec"

if ! oasdiff breaking "$base_spec" "$SPEC" --fail-on ERR --format text; then
  echo "[api-no-breaking-change] $SPEC breaks clients compared to $BASE_REF ($base_commit)." >&2
  exit 1
fi
