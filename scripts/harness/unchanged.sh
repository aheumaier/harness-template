#!/usr/bin/env bash
# Scope gate (rules.md#harness-scope, ADR 0013): exits 0 when a sensor may be skipped because none of its inputs
# changed, and 1 when it must run.
#
#   scripts/harness/unchanged.sh <pathspec>...     (Makefile: lint-sh, fmt-check-sh, test-sh)
#
# Skips only in HARNESS_SCOPE=changed (the default, make check). make check-all and CI export HARNESS_SCOPE=full
# and run everything. A change is anything between the merge base (scripts/harness/merge-base.sh) and the working
# tree, staged or not, plus untracked files, in the given pathspecs or in any harness file
# (protected-paths.txt): a harness change can alter what every sensor does. Fails closed: no merge base or any
# git error means run.
set -uo pipefail

(($# > 0)) || {
  echo "usage: unchanged.sh <pathspec>..." >&2
  exit 2
}
[[ "${HARNESS_SCOPE:-changed}" == changed ]] || exit 1
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT" || exit 1

base="$(scripts/harness/merge-base.sh 2>/dev/null)" || exit 1
specs=("$@")
while read -r spec _; do
  [[ -z "$spec" || "$spec" == \#* ]] || specs+=("$spec")
done <scripts/harness/protected-paths.txt

git diff --quiet "$base" -- "${specs[@]}" || exit 1
untracked="$(git ls-files -o --exclude-standard -- "${specs[@]}")" || exit 1
[[ -z "$untracked" ]]
