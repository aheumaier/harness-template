#!/usr/bin/env bats
# scripts/harness/merge-base.sh: the base of every diff-based sensor; fails closed when there is none.
bats_require_minimum_version 1.5.0
load helpers

setup() {
  sandbox scripts/harness/merge-base.sh
  printf '%s\n' x >"$SANDBOX/x"
  sandbox_base
  cd "$SANDBOX" || return
}

@test "--ref prefers origin/main and falls back to main" {
  run -0 scripts/harness/merge-base.sh --ref
  [ "$output" = main ]
  git update-ref refs/remotes/origin/main HEAD
  run -0 scripts/harness/merge-base.sh --ref
  [ "$output" = origin/main ]
}

@test "prints the merge base with BASE_REF" {
  BASE_REF=main run -0 scripts/harness/merge-base.sh
  [ "$output" = "$(git rev-parse HEAD)" ]
}

@test "exits 2 when BASE_REF does not resolve" {
  BASE_REF=nope run -2 scripts/harness/merge-base.sh
  has "BASE_REF 'nope' is not a commit in this clone"
}

@test "exits 2 when there is no merge base (unrelated or shallow history)" {
  git checkout -q --orphan other
  git -c user.name=t -c user.email=t@localhost -c commit.gpgsign=false commit -qm other
  BASE_REF=main run -2 scripts/harness/merge-base.sh
  has 'have no merge base: fetch full history'
}
