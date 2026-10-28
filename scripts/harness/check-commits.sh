#!/usr/bin/env bash
# Conventional Commits: every commit subject is `<type>[(<scope>)][!]: <description>`, at most 100 characters.
#
#   scripts/harness/check-commits.sh              every non-merge commit since the merge base with BASE_REF
#                                                 (`make commits`), plus PR_TITLE when set, max 90 characters
#                                                 (GitHub's default squash commit is the title + " (#N)")
#   scripts/harness/check-commits.sh --msg FILE   one commit message (scripts/git-hooks/commit-msg); also
#                                                 lets git's own Merge/Revert/fixup!/squash!/amend! through,
#                                                 which the range check catches if they reach a PR
#
# The commits are read from HARNESS_ROOT (default: this repo), like `harness integrity-check`, so CI runs
# the base branch's copy against the PR head. Fails closed (exit 2) when the range cannot be read.
# rules.md#git-conventional-commits
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
TYPES='build|chore|ci|docs|feat|fix|perf|refactor|revert|style|test'
CONVENTIONAL="^($TYPES)(\([a-z0-9][a-z0-9._/-]*\))?!?: [^[:space:]]"
GIT_OWN='^(Merge |Revert "|fixup! |squash! |amend! )'
fail=0

check() { # where subject [max]
  local max="${3:-100}"
  if [[ ! "$2" =~ $CONVENTIONAL ]]; then
    echo "$1: '$2' is not '<type>(<scope>)!: <description>' with type one of ${TYPES//|/, } and a lowercase scope [git-conventional-commits]"
    fail=1
  elif ((${#2} > max)); then
    echo "$1: subject is ${#2} characters, max $max [git-conventional-commits]"
    fail=1
  fi
}

if [[ "${1:-}" == --msg ]]; then
  subject="$(awk '!/^#/ && NF { print; exit }' "${2:?usage: check-commits.sh --msg FILE}")"
  [[ "$subject" =~ $GIT_OWN ]] && exit 0
  check "commit message" "$subject"
  exit "$fail"
fi

cd "${HARNESS_ROOT:-$HERE/../..}"
base="$("$HERE/merge-base.sh")"
log="$(git log --no-merges --format='%h %s' "$base..HEAD")" || exit 2
while read -r sha subject; do
  [[ -n "$sha" ]] && check "$sha" "$subject"
done <<<"$log"
# A squash merge appends " (#N)" to the title, so leave it room under the 100-character limit on main.
[[ -z "${PR_TITLE:-}" ]] || check "PR title" "$PR_TITLE" 90

exit "$fail"
