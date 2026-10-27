#!/usr/bin/env bats
# scripts/harness/check-commits.sh: commit subjects and the PR title follow Conventional Commits.
bats_require_minimum_version 1.5.0
load helpers

setup() {
  sandbox scripts/harness/check-commits.sh scripts/harness/merge-base.sh
  printf '%s\n' x >"$SANDBOX/x"
  sandbox_base
  git -C "$SANDBOX" checkout -qb work
}

commit() { # subject
  printf '%s\n' "$1" >>"$SANDBOX/x"
  git -C "$SANDBOX" -c user.name=test -c user.email=test@localhost -c commit.gpgsign=false commit -qam "$1"
}
check() { BASE_REF=main run "$@" "$SANDBOX/scripts/harness/check-commits.sh"; }
msg() { # status message
  printf '%s\n' "$2" >"$BATS_TEST_TMPDIR/msg"
  run "$1" "$SANDBOX/scripts/harness/check-commits.sh" --msg "$BATS_TEST_TMPDIR/msg"
}

@test "passes conventional commits, with scope and breaking marker" {
  commit 'feat(web): add a banner'
  commit 'fix!: drop the v0 route'
  commit 'chore(deps/go): bump pgx'
  check -0
}

@test "flags non-conventional subjects with the sha" {
  commit 'feat: fine'
  commit 'update stuff'
  commit 'Feat: capitalised type'
  check -1
  has "$(git -C "$SANDBOX" rev-parse --short HEAD~1): 'update stuff' is not"
  has "'Feat: capitalised type' is not"
  has '[git-conventional-commits]'
}

@test "flags subjects over 100 characters" {
  commit "docs: $(printf 'x%.0s' {1..100})"
  check -1
  has 'subject is 106 characters, max 100'
}

@test "checks PR_TITLE when set" {
  commit 'feat: fine'
  PR_TITLE='Add banner' check -1
  has "PR title: 'Add banner' is not"
}

@test "ignores merge commits" {
  git -C "$SANDBOX" checkout -qb side main
  commit 'fix: side'
  git -C "$SANDBOX" checkout -q work
  git -C "$SANDBOX" -c user.name=test -c user.email=test@localhost -c commit.gpgsign=false merge -q --no-ff -m "Merge branch 'side'" side
  check -0
}

@test "exits 2 without a base" {
  BASE_REF=nope run -2 "$SANDBOX/scripts/harness/check-commits.sh"
}

@test "--msg checks the first non-comment line" {
  msg -0 $'# comment\n\nfeat(api): add orders\n\nbody'
  msg -1 $'# comment\nadd orders'
  has "commit message: 'add orders' is not"
}

@test "--msg lets git's own merge, revert and autosquash subjects through" {
  msg -0 "Merge branch 'main' into work"
  msg -0 'Revert "feat: x"'
  msg -0 'fixup! feat: x'
}
