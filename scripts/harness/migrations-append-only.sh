#!/usr/bin/env bash
# Migrations are append-only: a migration that exists at the merge base with BASE_REF (merge-base.sh) must
# not be modified, renamed or deleted. Used by `make migrations-append-only` (check-all) and migration-gate.yml.
#
# Checks both the base's MIGRATIONS_DIR (from the base copy of scripts/harness/layout.env) and the head's, so a
# PR that moves MIGRATIONS_DIR cannot hide an edit to an applied migration (ADR 0030). A migration that left
# the base dir is fine only when the head dir holds it under the same name, byte for byte. A base without
# layout.env (before ADR 0030) uses the head's dir.
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
cd "$HERE/../.."
# shellcheck source=layout.sh
source "$HERE/layout.sh" || exit 2
BASE_REF="${BASE_REF:-$("$HERE/merge-base.sh" --ref)}"
base="$(BASE_REF="$BASE_REF" "$HERE/merge-base.sh")"

base_dir="$MIGRATIONS_DIR"
if git cat-file -e "$base:scripts/harness/layout.env" 2>/dev/null; then
  tmp="$(mktemp)"
  trap 'rm -f "$tmp"' EXIT
  git show "$base:scripts/harness/layout.env" >"$tmp"
  base_dir="$("$HERE/layout.sh" --print "$tmp" | sed -n 's/^MIGRATIONS_DIR=//p')" || {
    echo "scripts/harness/layout.env on $BASE_REF is invalid (above), so its MIGRATIONS_DIR is unknown [db-append-only-migrations]" >&2
    exit 2
  }
fi

# A rename shows up as D (old path) + A (new path) with --no-renames; A alone is the normal case.
changed="$(git diff --name-status --no-renames --diff-filter=MD "$base" -- "$base_dir/*.sql" "$MIGRATIONS_DIR/*.sql")"
red=0
while read -r status path; do
  [[ -n "$path" ]] || continue
  moved="$MIGRATIONS_DIR/${path##*/}"
  if [[ "$status" == D && "$path" != "$moved" && -f "$moved" ]]; then
    [[ "$(git rev-parse "$base:$path")" == "$(git hash-object "$moved")" ]] && continue
    status=M
  fi
  [[ "$status" == D ]] && what=deleted || what=modified
  echo "$path: migration on $BASE_REF was $what [db-append-only-migrations]"
  red=1
done <<<"$changed"
exit "$red"
