#!/usr/bin/env bats
# scripts/harness/swift-format.sh (rules.md#ios-format) around a stub swift-format: exit 1 only when swift-format
# failed with a finding line in its output; any other failure is exit 2, so a broken sensor never reads as a finding.
bats_require_minimum_version 1.5.0
load helpers

setup() {
  T="$BATS_TEST_TMPDIR"
  # The stub records its arguments, prints $T/out and exits with $T/rc.
  stub swift-format "printf '[%s]' \"\$@\" >'$T/args'; cat '$T/out'; exit \"\$(cat '$T/rc')\""
  PATH="$T/bin:$PATH"
}

# swift_format <rc> [line...]: what the stub prints and how it exits.
swift_format() {
  echo "$1" >"$T/rc"
  shift
  printf '%s\n' "$@" >"$T/out"
}

lint() { "$REPO/scripts/harness/swift-format.sh" "$@"; }

@test "passes clean files to swift-format with the root config, strict and following links" {
  swift_format 0
  run -0 lint 'apps/ios/A b.swift' apps/ios/B.swift
  run -0 cat "$T/args"
  [ "$output" == '[lint][--strict][--follow-symlinks][--configuration][.swift-format][apps/ios/A b.swift][apps/ios/B.swift]' ]
}

@test "a finding is exit 1, with the finding" {
  swift_format 1 'apps/ios/A.swift:2:1: error: [Indentation] remove all leading whitespace'
  run -1 lint apps/ios/A.swift
  has 'apps/ios/A.swift:2:1: error: [Indentation] remove all leading whitespace'
}

@test "a failure without a finding line cannot run (exit 2): an unreadable or missing config, a syntax error, a crash" {
  swift_format 1 '<unknown>: error: Unable to read configuration: The given data was not valid JSON.'
  run -2 lint apps/ios/A.swift
  has 'Unable to read configuration'
  swift_format 1 "apps/ios/A.swift:1:8: error: expected ')' to end parameter clause"
  run -2 lint apps/ios/A.swift
  has "expected ')' to end parameter clause"
  swift_format 134 'apps/ios/A.swift:2:1: error: [Indentation] remove all leading whitespace'
  run -2 lint apps/ios/A.swift
}
