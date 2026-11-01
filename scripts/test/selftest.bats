#!/usr/bin/env bats
# scripts/harness/selftest.sh: its fixture.conf parse and stack and host skips (ADR 0033, ADR 0042). Each
# fixture's verdict is make harness-selftest itself.
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

@test "a fixture's scratch in the gitignored .tmp/ does not leak into the next fixture" {
  fixture writes layout-check
  fixture reads layout-check
  # .tmp/ ignored, as in the repo: else git clean removes it and this test proves nothing.
  printf '.tmp/\n' >"$SANDBOX/.gitignore"
  for f in writes reads; do # a fixture must change a tracked file; .tmp/ alone is ignored
    mkdir -p "$F/$f/files" && echo "$f" >"$F/$f/files/$f.txt"
  done
  printf '#!/usr/bin/env bash\nmkdir -p .tmp/upstream && touch .tmp/upstream/x\n' >"$F/writes/apply.sh"
  printf '#!/usr/bin/env bash\n[[ ! -e .tmp/upstream ]] || { echo "leaked: .tmp/upstream" >&2; exit 1; }\n' \
    >"$F/reads/apply.sh"
  chmod +x "$F/writes/apply.sh" "$F/reads/apply.sh"
  run -1 --separate-stderr "$SANDBOX/scripts/harness/selftest.sh" writes reads
  [[ "$stderr" != *"could not apply fixture"* ]] || false
  grep -qF "✗ reads → layout-check" <<<"$stderr" # reads applied and its sensor ran
}

@test "a fixture.conf STACK naming an unknown group is a bad fixture.conf" {
  fixture typo layout-check wbe
  run -1 --separate-stderr "$SANDBOX/scripts/harness/selftest.sh" typo
  grep -qF "✗ typo: bad fixture.conf (STACK: unknown group wbe; known: go db web ios)" <<<"$stderr"
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

@test "a TEMPLATE_ONLY fixture is skipped in a product (HARNESS_UPSTREAM in the file), a bad TEMPLATE_ONLY is a bad fixture.conf" {
  fixture probe layout-check
  echo TEMPLATE_ONLY=1 >>"$F/probe/fixture.conf"
  fixture typo layout-check
  echo TEMPLATE_ONLY=yes >>"$F/typo/fixture.conf"
  edit scripts/tool-versions.env 's|^HARNESS_UPSTREAM=.*|HARNESS_UPSTREAM=https://example.com/template|'
  HARNESS_UPSTREAM='' run -1 --separate-stderr "$SANDBOX/scripts/harness/selftest.sh" probe typo
  has "- probe: skipped (template-only: HARNESS_UPSTREAM is set, this repo is a product)"
  grep -qF "✗ typo: bad fixture.conf" <<<"$stderr"
  has "harness-selftest: 0 passed, 1 failed, 1 skipped"
}

@test "a TEMPLATE_ONLY fixture runs in the template (HARNESS_UPSTREAM empty in the file), whatever the environment says" {
  fixture probe layout-check
  echo TEMPLATE_ONLY=1 >>"$F/probe/fixture.conf"
  edit scripts/tool-versions.env 's|^HARNESS_UPSTREAM=.*|HARNESS_UPSTREAM=|'
  HARNESS_UPSTREAM=https://example.com/template run --separate-stderr "$SANDBOX/scripts/harness/selftest.sh" probe
  [[ "$output" != *"- probe: skipped"* ]] || false
  has "harness-selftest: 0 passed, 1 failed, 0 skipped"
}

@test "a fixture of an ios target needs NEEDS_SWIFT=1, and NEEDS_SWIFT is 0 or 1 (ADR 0042)" {
  fixture untagged test-ios
  fixture typo layout-check
  echo NEEDS_SWIFT=yes >>"$F/typo/fixture.conf"
  run -1 --separate-stderr "$SANDBOX/scripts/harness/selftest.sh" untagged typo
  grep -qF "✗ untagged: bad fixture.conf (TARGET test-ios needs Swift on macOS: set NEEDS_SWIFT=1)" <<<"$stderr"
  grep -qxF "✗ typo: bad fixture.conf" <<<"$stderr"
  has "harness-selftest: 0 passed, 2 failed, 0 skipped"
}

@test "a NEEDS_SWIFT fixture on a host that is not macOS fails, unless SELFTEST_SKIP_SWIFT=1 skips it" {
  fixture swift test-ios
  echo NEEDS_SWIFT=1 >>"$F/swift/fixture.conf"
  host Linux
  # A STACK with ios, whatever the repo's layout.env declares (an adopted product's may be go only).
  HARNESS_STACK=go,ios SELFTEST_SKIP_SWIFT='' run -1 --separate-stderr "$SANDBOX/scripts/harness/selftest.sh" swift
  grep -qF "✗ swift: needs Swift on macOS (or SELFTEST_SKIP_SWIFT=1)" <<<"$stderr"
  HARNESS_STACK=go,ios SELFTEST_SKIP_SWIFT=1 run -0 "$SANDBOX/scripts/harness/selftest.sh" swift
  has "- swift: skipped (no Swift on macOS here)"
  has "harness-selftest: 0 passed, 0 failed, 1 skipped"
}

@test "a NEEDS_SWIFT fixture on macOS whose swift does not run (Apple's stub without the tools) fails" {
  fixture swift test-ios
  echo NEEDS_SWIFT=1 >>"$F/swift/fixture.conf"
  host Darwin
  printf '#!/bin/sh\nexit 1\n' >"$BATS_TEST_TMPDIR/bin/swift"
  chmod +x "$BATS_TEST_TMPDIR/bin/swift"
  HARNESS_STACK=go,ios SELFTEST_SKIP_SWIFT='' run -1 --separate-stderr "$SANDBOX/scripts/harness/selftest.sh" swift
  grep -qF "✗ swift: needs Swift on macOS (or SELFTEST_SKIP_SWIFT=1)" <<<"$stderr"
}

@test "the full stack is every group layout.env declares: a skip for an undeclared group passes, for a declared one fails" {
  fixture swift test-ios
  echo NEEDS_SWIFT=1 >>"$F/swift/fixture.conf"
  fixture web-target lint-web
  edit scripts/harness/layout.env 's/^STACK=.*/STACK=go:db:web/'
  # A Makefile that leaves out a web sensor although STACK has web: it and selftest.sh disagree.
  edit Makefile 's/^override STACK_OFF := $(strip /&lint-web /'
  run -1 --separate-stderr env -u HARNESS_STACK "$SANDBOX/scripts/harness/selftest.sh" swift web-target
  has "- swift: skipped (STACK=go:db:web leaves out test-ios)"
  grep -qF "✗ web-target: skipped for lint-web under the full STACK=go:db:web" <<<"$stderr"
  has "harness-selftest: 0 passed, 1 failed, 1 skipped"
}
