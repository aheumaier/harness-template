#!/usr/bin/env bats
# .github/CODEOWNERS mirrors scripts/harness/protected-paths.txt: every harness file needs an owner's review.
bats_require_minimum_version 1.5.0
load helpers

@test "every protected path has a code owner" {
  owners="$(awk '!/^#/ && NF { p = $1; sub(/^\//, "", p); sub(/\/$/, "", p); print p }' "$REPO/.github/CODEOWNERS")"
  missing=()
  while read -r glob _; do
    p="${glob#\*}" p="${p%/\*}"
    covered=0
    while IFS= read -r o; do
      [[ "$p" == "$o" || "$p" == "$o"/* ]] && covered=1
    done <<<"$owners"
    ((covered)) || missing+=("$glob")
  done < <(grep -Ev '^[[:space:]]*(#|$)' "$REPO/scripts/harness/protected-paths.txt")
  output="${missing[*]-}"
  [[ -z "$output" ]] || {
    echo "no CODEOWNERS line covers: $output" >&2
    return 1
  }
}
