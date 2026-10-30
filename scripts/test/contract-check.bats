#!/usr/bin/env bats
# scripts/harness/contract-check.sh: oasdiff breaking changes vs the merge base; a human label waives them.
bats_require_minimum_version 1.5.0
load helpers

setup() {
  sandbox scripts/harness/contract-check.sh scripts/harness/merge-base.sh
  export HARNESS="$REPO/.bin/harness" # spec-refs; the sandbox has no .bin
  contract /a /b
  sandbox_base
}

# contract <path...>: the sandbox's api/openapi.yaml with one GET per path.
contract() { spec "$SANDBOX/api/openapi.yaml" "$@"; }

check() { BASE_REF=main run "$@" "$SANDBOX/scripts/harness/contract-check.sh"; }

@test "passes an additive change" {
  contract /a /b /c
  check -0
}

@test "fails a removed path" {
  contract /a
  check -1
  has '[api-no-breaking-change] api/openapi.yaml breaks clients compared to main'
}

@test "BREAKING_CHANGE_APPROVED=1 (the human label) prints the break but passes" {
  contract /a
  BREAKING_CHANGE_APPROVED=1 check -0
  has 'breaking change approved by a human'
}

@test "passes when the base has no contract yet" {
  git -C "$SANDBOX" rm -q api/openapi.yaml
  commit_all "$SANDBOX" rm
  contract /a
  check -0
}

@test "judges the spec under HARNESS_ROOT with this script (CI: base script, PR head)" {
  pr="$BATS_TEST_TMPDIR/pr"
  git clone -q "$SANDBOX" "$pr"
  spec "$pr/api/openapi.yaml" /a
  HARNESS_ROOT="$pr" BASE_REF=origin/main run -1 "$SANDBOX/scripts/harness/contract-check.sh"
  has 'breaks clients compared to origin/main'
}

@test "fails an external \$ref: oasdiff would fetch or read it" {
  printf '%s\n' 'components:' '  schemas:' '    X: { $ref: "https://example.com/x.yaml" }' >>"$SANDBOX/api/openapi.yaml"
  BREAKING_CHANGE_APPROVED=1 check -1
  has 'has an external $ref'
}

@test "an external \$ref that a line grep misses still fails: key quoted or spaced, value on the next line" {
  printf '%s\n' 'components:' '  schemas:' '    A:' '      $ref:' '        other.yaml' '    B:' '      $ref : other.yaml' \
    '    C: { "$ref": "other.yaml" }' >>"$SANDBOX/api/openapi.yaml"
  BREAKING_CHANGE_APPROVED=1 check -1
  has 'api/openapi.yaml:17: $ref "other.yaml" points outside this file'
  has 'api/openapi.yaml:20: $ref "other.yaml" points outside this file'
  has 'api/openapi.yaml:21: $ref "other.yaml" points outside this file'
}

@test "a break names the repos in CONTRACT_CONSUMERS that vendor the contract (ADR 0037)" {
  edit scripts/harness/layout.env '/^CONTRACT_CONSUMERS=/d' # the key is optional: absent in many repos
  contract /a
  check -1
  [ "${output/CONTRACT_CONSUMERS/}" = "$output" ] # none listed: nothing to name
  echo 'CONTRACT_CONSUMERS=github.com/acme/web:github.com/acme/app' >>"$SANDBOX/scripts/harness/layout.env"
  BREAKING_CHANGE_APPROVED=1 check -0
  has 'vendor this contract (CONTRACT_CONSUMERS), which the approval must cover: github.com/acme/web github.com/acme/app'
}

@test "a spec oasdiff cannot load fails closed with exit 2, even with the human label" {
  printf 'openapi: 3.0.3\ninfo: { title: t, version: "1" }\npaths: 7\n' >"$SANDBOX/api/openapi.yaml"
  BREAKING_CHANGE_APPROVED=1 check -2
  has '[api-no-breaking-change] cannot compare api/openapi.yaml with main'
  has 'oasdiff exit 102'
  [ "${output/approved by a human/}" = "$output" ]
}
