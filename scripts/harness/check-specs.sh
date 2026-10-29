#!/usr/bin/env bash
# spec-kit specs (specs/NNN-slug/, written by /speckit-specify, /speckit-plan, /speckit-tasks).
#
#   scripts/harness/check-specs.sh      (make specs)
#
# specs-structure: every specs/<dir> is a real directory named NNN-kebab-slug with a unique NNN, has spec.md,
#   has plan.md once it has tasks.md, no .md in it has an open clarification (`[NEEDS CLARIFICATION: …]` from
#   /speckit-specify, a plan field left as `: NEEDS CLARIFICATION`; checklists/ only names the marker), and it
#   holds no OpenAPI document (the contract lives only in the layout's SPEC).
# specs-required-for-features: when a `feat` commit since the merge base with BASE_REF (or a feat PR_TITLE)
#   changes product code (PRODUCT_GLOBS of scripts/harness/layout.env) outside the harness files
#   (protected-paths.txt), the change must also add or change (not just delete) a committed specs/<dir>/.
#   NO_SPEC_NEEDED=1 (the `no-spec-needed` PR label) skips this rule only.
#
# Reads from HARNESS_ROOT (default: this repo) so CI runs the base branch's copy against the PR head.
# Fails closed (exit 2) when the base or the range cannot be read.
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=layout.sh
source "$HERE/layout.sh" || exit 2
cd "${HARNESS_ROOT:-$HERE/../..}"
OPEN_CLARIFICATION='\[NEEDS CLARIFICATION:|: NEEDS CLARIFICATION[[:space:]]*$'
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
  if [[ -L "$d" ]]; then # CI reads an untrusted tree: never follow a link out of it
    err "$d: is a symlink, not a directory [specs-structure]"
    continue
  fi
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
  done < <(grep -rnE --include='*.md' --exclude-dir=checklists "$OPEN_CLARIFICATION" "$d" || true)
  while IFS= read -r f; do
    err "$f: is an OpenAPI document: change $SPEC instead and describe the change here [specs-structure]"
  done < <(grep -rlE --include='*.yaml' --include='*.yml' --include='*.json' '^(openapi:|[[:space:]]*"openapi"[[:space:]]*:)' "$d" || true)
done

# --- specs-required-for-features ----------------------------------------------------------------------
if [[ "${NO_SPEC_NEEDED:-0}" != 1 ]]; then
  base="$("$HERE/merge-base.sh")"
  feat='^feat(\([^)]*\))?!?:'
  # Product code: PRODUCT_GLOBS minus harness files (analyzers, linter configs), planned in exec-plans.
  product=("${PRODUCT_GLOBS[@]}")
  while read -r glob _; do product+=(":(exclude)$glob"); done < <(grep -Ev '^[[:space:]]*(#|$)' "$HERE/protected-paths.txt")
  # Product files of every commit in the range whose subject is a feat; with a feat PR title, the whole change.
  log="$(git log --no-merges --format='%H %s' "$base..HEAD")" || exit 2
  feat_files=''
  while read -r sha subject; do
    if [[ "$subject" =~ $feat ]]; then
      feat_files+="$(git show --format= --name-only "$sha" -- "${product[@]}")"$'\n' || exit 2
    fi
  done <<<"$log"
  if grep -Eq "$feat" <<<"${PR_TITLE:-}"; then
    feat_files+="$(git diff --name-only "$base" HEAD -- "${product[@]}")" || exit 2
  fi
  # Committed only: CI sees no untracked files, so a local pass must not depend on them.
  changed="$(git diff --name-only --diff-filter=d "$base" HEAD)" || exit 2
  if grep -q . <<<"$feat_files" && ! grep -Eq '^specs/[^/]+/' <<<"$changed"; then
    err "a feat change touches product code (${PRODUCT_GLOBS[*]}) but adds or changes no committed specs/<NNN-slug>/ (write it with /speckit-specify and commit it, or label the PR no-spec-needed) [specs-required-for-features]"
  fi
fi

exit "$fail"
