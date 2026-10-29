#!/usr/bin/env bats
# scripts/harness/selftest.sh: its fixture.conf parse and stack skips (ADR 0033). Each fixture's verdict is
# make harness-selftest itself.
bats_require_minimum_version 1.5.0
load helpers

setup() {
  sandbox scripts/harness/selftest.sh scripts/tool-versions.env Makefile
  F="$SANDBOX/scripts/harness/fixtures"
}

# fixture <name> <TARGET> [STACK]: a fixture.conf in the sandbox.
fixture() {
  mkdir -p "$F/$1"
  printf '%s\n' RULE=harness-layout-valid "TARGET=$2" "EXPECT='x'" ${3:+"STACK=$3"} >"$F/$1/fixture.conf"
}

@test "a fixture.conf STACK naming an unknown group is a bad fixture.conf" {
  fixture typo layout-check wbe
  run -1 --separate-stderr "$SANDBOX/scripts/harness/selftest.sh" typo
  grep -qF "✗ typo: bad fixture.conf (STACK: unknown group wbe; known: go db web)" <<<"$stderr"
  has "harness-selftest: 0 passed, 1 failed, 0 skipped"
}

@test "a fixture whose target or STACK tag the stack leaves out is skipped, from the Makefile's STACK_OFF" {
  fixture web-target lint-web
  fixture db-tag layout-check db
  HARNESS_STACK=go run -0 "$SANDBOX/scripts/harness/selftest.sh" web-target db-tag
  has "- web-target: skipped (STACK=go leaves out lint-web)"
  has "- db-tag: skipped (STACK=go leaves out db)"
  has "harness-selftest: 0 passed, 0 failed, 2 skipped"
}
