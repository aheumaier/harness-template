#!/usr/bin/env bash
# Harness integrity: the change (working tree vs the merge base with BASE_REF, merge-base.sh) must not
# weaken the harness. Used by `make harness-integrity` (check-all) and .github/workflows/harness-integrity.yml.
#
#   [harness-suppression-reason]   an added suppression lacks `reason: … (rule: <id>)` on its line, names an
#                                  id that is not a `### <id>` heading in docs/harness/rules.md, or is an
#                                  @ts-ignore/@ts-nocheck (never allowed)
#   [harness-config-not-loosened]  a harness config (`config` in protected-paths.txt) gained or lost a
#                                  non-comment line. Waived only by a human: HARNESS_CHANGE_APPROVED=1 (CI:
#                                  the `harness-change-approved` PR label).
#
# This script, its marker lists, protected-paths.txt and the rules.md that rule ids are looked up in come
# from the script's own checkout; the change is read from HARNESS_ROOT (default: the same repo). CI runs the
# base branch's copy against the PR head, so a PR cannot weaken the check that judges it (a suppression may
# cite a rule only once that rule is on the base branch). Fails closed (exit 2) when the diff cannot be read.
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
cd "${HARNESS_ROOT:-$HERE/../..}"
BASE_REF="${BASE_REF:-$("$HERE/merge-base.sh" --ref)}"
base="$(BASE_REF="$BASE_REF" "$HERE/merge-base.sh")"

# changes <pathspec...>: one "path<TAB>line<TAB>+|-<TAB>text" row per added/removed line (removed lines carry
# their line number on the base side); untracked files count as added. --text --no-textconv: the PR's
# .gitattributes (`-diff`, `binary`, textconv) must not turn a hunk into "Binary files differ".
changes() {
  git -c core.quotePath=false ls-files -o --exclude-standard -- "$@" | while IFS= read -r f; do
    awk -v path="$f" '{ print path "\t" NR "\t+\t" $0 }' "$f"
  done || return
  git -c core.quotePath=false diff -U0 --text --no-textconv --no-color --no-ext-diff --no-renames --src-prefix=a/ --dst-prefix=b/ \
    "$base" -- "$@" | awk '
      /^diff --git / { header = 1; next }
      header && /^(---|\+\+\+) [ab]\// { path = substr($0, 7); sub(/\t$/, "", path); next } # "a/" or "b/" + path
      /^@@/ { header = 0; split(substr($2, 2), o, ","); split(substr($3, 2), n, ","); old = o[1]; new = n[1]; next }
      header { next }
      /^\+/ { print path "\t" new++ "\t+\t" substr($0, 2); next }
      /^-/ { print path "\t" old++ "\t-\t" substr($0, 2) }
    '
}

RULES="$HERE/../../docs/harness/rules.md"
fail=0

# --- suppressions -------------------------------------------------------------------------------------
# Hand-written source only: fixture overlays are deliberate violations, generated and vendored code
# (.specify/, spec-kit: ADR 0011) is not ours, and this script names the markers it looks for.
SOURCES=('*.go' '*.ts' '*.tsx' '*.mts' '*.cts' '*.js' '*.jsx' '*.mjs' '*.cjs' '*.sh' '*.bash' '*.bats' '*.sql'
  '.github/*.yml' '.github/*.yaml' 'scripts/git-hooks/*'
  ':!:scripts/harness/fixtures/*/files/*' ':!:scripts/harness/integrity-check.sh' ':!:.specify/*'
  ':!:*.gen.go' ':!:apps/api/internal/*/store/*' ':!:packages/api-client/src/gen/*')
NEVER='(@ts-ignore|@ts-nocheck)'
SUPPRESSION='(//[[:space:]]*(nolint|lint:(file-)?ignore)|eslint-disable|/\*[[:space:]]*eslint[[:space:]]|@ts-expect-error|#[[:space:]]*shellcheck[[:space:]]+disable|atlas:nolint|squawk-ignore)'
REASON='reason: .+\(rule: ([a-z0-9-]+)\)'
rows="$(changes "${SOURCES[@]}")" || exit 2
while IFS=$'\t' read -r path line sign text; do
  [[ "$sign" == + ]] || continue
  if [[ "$text" =~ $NEVER ]]; then
    echo "$path:$line: ${BASH_REMATCH[1]} is never allowed [harness-suppression-reason]"
  elif [[ ! "$text" =~ $SUPPRESSION ]]; then
    continue
  elif [[ ! "$text" =~ $REASON ]]; then
    echo "$path:$line: suppression without 'reason: … (rule: <id>)' on the same line [harness-suppression-reason]"
  elif ! grep -qxF "### ${BASH_REMATCH[1]}" "$RULES"; then
    echo "$path:$line: suppression names '${BASH_REMATCH[1]}', not a rule id in docs/harness/rules.md [harness-suppression-reason]"
  else
    continue
  fi
  fail=1
done <<<"$rows"

# --- harness configs ----------------------------------------------------------------------------------
# Any non-comment change counts, tightening included: telling a loosening from a tightening needs judgment,
# and judgment is the human's (the label). Only `#` lines and blank lines are comments: `//` and `/* */` can
# comment out a JSONC, JS or Go line without the commented line itself looking like code.
configs=()
while read -r pattern kind; do
  [[ "$kind" == config ]] && configs+=("$pattern")
done < <(grep -Ev '^[[:space:]]*(#|$)' "$HERE/protected-paths.txt")
((${#configs[@]})) || {
  echo "no \`config\` entries in $HERE/protected-paths.txt: refusing to pass without checking harness configs" >&2
  exit 2
}
if [[ "${HARNESS_CHANGE_APPROVED:-0}" != 1 ]]; then
  rows="$(changes "${configs[@]}")" || exit 2
  COMMENT='^[[:space:]]*(#|$)'
  while IFS=$'\t' read -r path line sign text; do
    [[ "$text" =~ $COMMENT ]] && continue
    [[ "$sign" == + ]] && what=added || what=removed
    echo "$path:$line: $what harness config line: $text [harness-config-not-loosened]"
    fail=1
  done <<<"$rows"
fi

exit "$fail"
