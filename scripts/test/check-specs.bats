#!/usr/bin/env bats
# scripts/harness/check-specs.sh: spec-kit specs are well formed; feat changes to apps/ or api/ come with one.
bats_require_minimum_version 1.5.0
load helpers

setup() {
  sandbox scripts/harness/check-specs.sh scripts/harness/merge-base.sh
  mkdir -p "$SANDBOX/apps/web" "$SANDBOX/docs"
  printf '%s\n' x | tee "$SANDBOX/apps/web/x" >"$SANDBOX/docs/x"
  sandbox_base
  git -C "$SANDBOX" checkout -qb work
}

check() { BASE_REF=main run "$@" "$SANDBOX/scripts/harness/check-specs.sh"; }
add() { mkdir -p "$(dirname "$SANDBOX/$1")" && printf '%s\n' "${@:2}" >>"$SANDBOX/$1"; }
commit() { # subject
  git -C "$SANDBOX" add -A
  git -C "$SANDBOX" -c user.name=test -c user.email=test@localhost -c commit.gpgsign=false commit -qm "$1"
}

@test "passes without specs" {
  check -0
}

@test "passes a complete spec" {
  add specs/001-user-signup/spec.md '# Spec'
  add specs/001-user-signup/plan.md '# Plan'
  add specs/001-user-signup/tasks.md '# Tasks'
  check -0
}

@test "flags a bad name, a duplicate number, a missing spec.md and tasks.md without plan.md" {
  add specs/Signup/spec.md '# Spec'
  add specs/001-a/spec.md '# Spec'
  add specs/001-b/tasks.md '# Tasks'
  check -1
  has 'specs/Signup: name is not NNN-kebab-slug'
  has 'specs/001-b: number 001 is used by another spec'
  has 'specs/001-b: no spec.md'
  has 'specs/001-b: tasks.md without plan.md'
  has '[specs-structure]'
}

@test "flags an open NEEDS CLARIFICATION with file and line" {
  add specs/002-x/spec.md '# Spec' '- FR-1: [NEEDS CLARIFICATION: which auth?]'
  check -1
  has 'specs/002-x/spec.md:2: open NEEDS CLARIFICATION'
}

@test "a feat commit touching apps/ needs a spec" {
  add apps/web/y 'y'
  commit 'feat(web): add banner'
  check -1
  has '[specs-required-for-features]'
}

@test "a feat commit touching apps/ passes with a spec, or with NO_SPEC_NEEDED=1" {
  add apps/web/y 'y'
  commit 'feat(web): add banner'
  NO_SPEC_NEEDED=1 check -0
  add specs/003-banner/spec.md '# Spec'
  check -0 # untracked spec counts
}

@test "a feat PR title counts as a feat" {
  add apps/web/y 'y'
  commit 'fix(web): banner'
  check -0
  PR_TITLE='feat(web): banner' check -1
}

@test "feat outside apps/ and api/, and fix inside, need no spec" {
  add docs/y 'y'
  commit 'feat(docs): new guide'
  add apps/web/y 'y'
  commit 'fix(web): typo'
  check -0
}

@test "exits 2 without a base" {
  BASE_REF=nope run -2 "$SANDBOX/scripts/harness/check-specs.sh"
}
