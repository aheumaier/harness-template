#!/usr/bin/env bats
# scripts/harness/contract-check.sh: oasdiff breaking changes vs the merge base; a human label waives them.
bats_require_minimum_version 1.5.0
load helpers

setup() {
  sandbox scripts/harness/contract-check.sh scripts/harness/merge-base.sh
  spec /a /b
  sandbox_base
}

# spec <path...>: a minimal api/openapi.yaml with one GET per path.
spec() {
  mkdir -p "$SANDBOX/api"
  {
    printf 'openapi: 3.0.3\ninfo: { title: t, version: "1" }\npaths:\n'
    printf '  %s:\n    get:\n      responses:\n        "200":\n          description: ok\n' "$@"
  } >"$SANDBOX/api/openapi.yaml"
}

check() { BASE_REF=main run "$@" "$SANDBOX/scripts/harness/contract-check.sh"; }

@test "passes an additive change" {
  spec /a /b /c
  check -0
}

@test "fails a removed path" {
  spec /a
  check -1
  has '[api-no-breaking-change] api/openapi.yaml breaks clients compared to main'
}

@test "BREAKING_CHANGE_APPROVED=1 (the human label) prints the break but passes" {
  spec /a
  BREAKING_CHANGE_APPROVED=1 check -0
  has 'breaking change approved by a human'
}

@test "passes when the base has no contract yet" {
  git -C "$SANDBOX" rm -q api/openapi.yaml
  git -C "$SANDBOX" -c user.name=t -c user.email=t@localhost -c commit.gpgsign=false commit -qm rm
  spec /a
  check -0
}
