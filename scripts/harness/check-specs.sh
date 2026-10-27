#!/usr/bin/env bash
# spec-kit specs (specs/NNN-slug/, written by /speckit-specify, /speckit-plan, /speckit-tasks).
#
#   scripts/harness/check-specs.sh      (make specs)
#
# specs-structure: every specs/<dir> is NNN-kebab-slug with a unique NNN, has spec.md, has plan.md once it
#   has tasks.md, and no .md in it still says NEEDS CLARIFICATION.
# specs-required-for-features: when a commit since the merge base with BASE_REF (or PR_TITLE) is a `feat`
#   and the change touches apps/ or api/, it must also add or change a specs/<dir>/. NO_SPEC_NEEDED=1 (the
#   `no-spec-needed` PR label) skips this rule only.
#
# Reads from HARNESS_ROOT (default: this repo) so CI runs the base branch's copy against the PR head.
# Fails closed (exit 2) when the base or the range cannot be read.
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
cd "${HARNESS_ROOT:-$HERE/../..}"
fail=0
err() {
  echo "$1"
  fail=1
}

# --- specs-structure ----------------------------------------------------------------------------------
seen=' '
shopt -s nullglob
for d in specs/*/; do
  d="${d%/}" name="${d#specs/}"
  if [[ ! "$name" =~ ^([0-9]{3,})-[a-z0-9]+(-[a-z0-9]+)*$ ]]; then
    err "$d: name is not NNN-kebab-slug (create it with /speckit-specify) [specs-structure]"
    continue
  fi
  num="${BASH_REMATCH[1]}"
  [[ "$seen" == *" $num "* ]] && err "$d: number $num is used by another spec [specs-structure]"
  seen+="$num "
  [[ -f "$d/spec.md" ]] || err "$d: no spec.md [specs-structure]"
  [[ ! -f "$d/tasks.md" || -f "$d/plan.md" ]] || err "$d: tasks.md without plan.md [specs-structure]"
  while IFS=: read -r f line _; do
    err "$f:$line: open NEEDS CLARIFICATION (resolve it with /speckit-clarify) [specs-structure]"
  done < <(grep -rn --include='*.md' 'NEEDS CLARIFICATION' "$d" || true)
done

# --- specs-required-for-features ----------------------------------------------------------------------
if [[ "${NO_SPEC_NEEDED:-0}" != 1 ]]; then
  base="$("$HERE/merge-base.sh")"
  feat='^feat(\([^)]*\))?!?:'
  # Files of every commit in the range whose subject is a feat; with a feat PR title, the whole change.
  log="$(git log --no-merges --format='%H %s' "$base..HEAD")" || exit 2
  feat_files=''
  while read -r sha subject; do
    [[ "$subject" =~ $feat ]] && feat_files+="$(git show --format= --name-only "$sha")"$'\n'
  done <<<"$log"
  if grep -Eq "$feat" <<<"${PR_TITLE:-}"; then
    feat_files+=$'\n'"$(git diff --name-only "$base")" || exit 2
  fi
  changed="$(git diff --name-only "$base" && git ls-files -o --exclude-standard)" || exit 2
  if grep -Eq '^(apps|api)/' <<<"$feat_files" && ! grep -Eq '^specs/[^/]+/' <<<"$changed"; then
    err "a feat change touches apps/ or api/ but adds or changes no specs/<NNN-slug>/ (write it with /speckit-specify, or label the PR no-spec-needed) [specs-required-for-features]"
  fi
fi

exit "$fail"
