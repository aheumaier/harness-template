#!/usr/bin/env bash
# Migrations are append-only: a migration that exists at the merge base with BASE_REF (default main) must
# not be modified, renamed or deleted. Used by `make migrations-append-only` (check-all) and migration-gate.yml.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
BASE_REF="${BASE_REF:-main}"
cd "$ROOT"

base="$(git merge-base HEAD "$BASE_REF" 2>/dev/null || true)"
[[ -n "$base" ]] || exit 0 # no common history with BASE_REF: nothing to compare

# A rename shows up as D (old path) + A (new path) with --no-renames; A alone is the normal case.
changed="$(git diff --name-status --no-renames --diff-filter=MD "$base" -- 'apps/api/db/migrations/*.sql')"
[[ -n "$changed" ]] || exit 0
while read -r status path; do
  [[ "$status" == D ]] && what=deleted || what=modified
  echo "$path: migration on $BASE_REF was $what [db-append-only-migrations]"
done <<<"$changed"
exit 1
