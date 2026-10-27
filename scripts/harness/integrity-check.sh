#!/usr/bin/env bash
# Harness integrity: the change (working tree vs the merge base with BASE_REF, default main) must not
# weaken the harness. Used by `make harness-integrity` (check-all) and .github/workflows/harness-integrity.yml.
#
#   [harness-suppression-reason]   an added suppression lacks `reason: … (rule: <id>)` on its line, or an
#                                  @ts-ignore was added (never allowed)
#   [harness-config-not-loosened]  a harness config (`config` in protected-paths.txt) lost a rule line or
#                                  gained a disable/exclude/ignore/skip/off/warn line. Waived only by a human:
#                                  HARNESS_CHANGE_APPROVED=1 (CI: the `harness-change-approved` PR label).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
BASE_REF="${BASE_REF:-main}"
cd "$ROOT"

base="$(git merge-base HEAD "$BASE_REF" 2>/dev/null || true)"
[[ -n "$base" ]] || exit 0 # no common history with BASE_REF: nothing to compare

# changes <pathspec...>: one "path<TAB>line<TAB>+|-<TAB>text" row per added/removed line (removed lines carry
# their line number on the base side); untracked files count as added.
changes() {
  git ls-files -o --exclude-standard -- "$@" | while IFS= read -r f; do
    awk -v path="$f" '{ print path "\t" NR "\t+\t" $0 }' "$f"
  done
  git -c core.quotePath=false diff -U0 --no-color --no-ext-diff --no-renames --src-prefix=a/ --dst-prefix=b/ \
    "$base" -- "$@" | awk '
      /^diff --git / { header = 1; next }
      header && /^(---|\+\+\+) / { if ($2 != "/dev/null") path = substr($2, 3); next }
      /^@@/ { header = 0; split(substr($2, 2), o, ","); split(substr($3, 2), n, ","); old = o[1]; new = n[1]; next }
      header { next }
      /^\+/ { print path "\t" new++ "\t+\t" substr($0, 2); next }
      /^-/ { print path "\t" old++ "\t-\t" substr($0, 2) }
    '
}

fail=0

# --- suppressions -------------------------------------------------------------------------------------
# Hand-written source only: fixture overlays are deliberate violations, generated code is not ours, and
# this script names the markers it looks for.
SOURCES=('*.go' '*.ts' '*.tsx' '*.js' '*.mjs' '*.cjs' '*.sh' '*.bash' '*.bats'
  ':!:scripts/harness/fixtures/*/files/*' ':!:scripts/harness/integrity-check.sh'
  ':!:*.gen.go' ':!:apps/api/internal/*/store/*' ':!:packages/api-client/src/gen/*')
SUPPRESSION='(//[[:space:]]*nolint|eslint-disable|@ts-expect-error|#[[:space:]]*shellcheck[[:space:]]+disable)'
REASON='reason: .+\(rule: [a-z0-9-]+\)'
while IFS=$'\t' read -r path line sign text; do
  [[ "$sign" == + ]] || continue
  if [[ "$text" == *@ts-ignore* ]]; then
    echo "$path:$line: @ts-ignore is never allowed [harness-suppression-reason]"
    fail=1
  elif [[ "$text" =~ $SUPPRESSION && ! "$text" =~ $REASON ]]; then
    echo "$path:$line: suppression without 'reason: … (rule: <id>)' on the same line [harness-suppression-reason]"
    fail=1
  fi
done < <(changes "${SOURCES[@]}")

# --- harness configs ----------------------------------------------------------------------------------
configs=()
while read -r pattern kind; do
  [[ "$kind" == config ]] && configs+=("$pattern")
done < <(grep -Ev '^[[:space:]]*(#|$)' scripts/harness/protected-paths.txt)
if [[ "${HARNESS_CHANGE_APPROVED:-0}" != 1 && ${#configs[@]} -gt 0 ]]; then
  COMMENT='^[[:space:]]*(#|//|/\*|\*|$)'
  LOOSENING='(disable|exclude|ignore|skip|(^|[^a-z])(off|warn)([^a-z]|$))'
  shopt -s nocasematch
  while IFS=$'\t' read -r path line sign text; do
    [[ "$text" =~ $COMMENT ]] && continue
    if [[ "$sign" == - ]]; then
      echo "$path:$line: removed harness config line: $text [harness-config-not-loosened]"
      fail=1
    elif [[ "$text" =~ $LOOSENING ]]; then
      echo "$path:$line: harness config line looks like a loosening: $text [harness-config-not-loosened]"
      fail=1
    fi
  done < <(changes "${configs[@]}")
  shopt -u nocasematch
fi

exit "$fail"
