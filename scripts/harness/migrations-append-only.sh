#!/usr/bin/env bash
# Migrations are append-only: a migration that exists at the merge base with BASE_REF (merge-base.sh) must
# not be modified, renamed or deleted. Used by `make migrations-append-only` (check-all) and migration-gate.yml.
#
# Checks both the base's MIGRATIONS_DIR (from the base copy of scripts/harness/layout.env) and the head's, so a
# PR that moves MIGRATIONS_DIR cannot hide an edit to an applied migration (ADR 0030). When the dirs differ, every
# migration of the base dir, moved or copied out of it, must be in the head dir under the same name and blob
# (as git would store it), and must not be edited where it was either. A base without layout.env (before ADR 0030) uses the head's dir.
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
report() { # <path> <deleted|modified>
  echo "$1: migration on $BASE_REF was $2 [db-append-only-migrations]"
  red=1
}
while read -r status path; do
  [[ -n "$path" ]] || continue
  # Left the base dir after the layout moved: the content pass below judges it against its copy.
  [[ "$status" == D && "$base_dir" != "$MIGRATIONS_DIR" && "$path" == "$base_dir/"* ]] && continue
  if [[ "$status" == D ]]; then report "$path" deleted; else report "$path" modified; fi
done <<<"$changed"

# The layout moved: every base migration must be in the head dir under the same name and blob. hash-object
# hashes the working tree (HEAD in CI) through the eol and clean filters, as git add would, so a checkout with
# CRLF conversion does not flag every copy. Its copy there is an A to the diff above, so an edit to it shows only here (a copied dir, the base's left as it was).
if [[ "$base_dir" != "$MIGRATIONS_DIR" ]]; then
  applied="$(git ls-tree -r --name-only "$base" -- "$base_dir")"
  while IFS= read -r path; do
    [[ "$path" == *.sql ]] || continue
    moved="$MIGRATIONS_DIR/${path#"$base_dir"/}"
    if [[ ! -f "$moved" ]]; then
      report "$path" deleted
    elif [[ "$(git rev-parse "$base:$path")" != "$(git hash-object -- "$moved")" ]]; then
      report "$path" modified
    fi
  done <<<"$applied"
fi
exit "$red"
