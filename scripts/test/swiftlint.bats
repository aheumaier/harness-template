#!/usr/bin/env bats
# scripts/harness/swiftlint.sh (rules.md#ios-lint) around a stub swiftlint: exit 1 only when SwiftLint failed with
# a finding line in its output; any other failure is exit 2, so a broken sensor never reads as a finding.
bats_require_minimum_version 1.5.0
load helpers

setup() {
  T="$BATS_TEST_TMPDIR"
  # The stub records its arguments and TOOLCHAIN_DIR, prints $T/out and exits with $T/rc.
  stub swiftlint "printf '[%s]' \"\$@\" >'$T/args'; echo \"\${TOOLCHAIN_DIR-unset}\" >'$T/toolchain'; cat '$T/out'; exit \"\$(cat '$T/rc')\""
  stub xcode-select 'echo /Applications/Xcode.app/Contents/Developer'
  PATH="$T/bin:$PATH"
}

# swiftlint <rc> [line...]: what the stub prints and how it exits.
swiftlint() {
  echo "$1" >"$T/rc"
  shift
  printf '%s\n' "$@" >"$T/out"
}

lint() { "$REPO/scripts/harness/swiftlint.sh" "$@"; }

@test "passes clean files to SwiftLint with the root config, strict and with relative paths" {
  swiftlint 0
  run -0 lint 'apps/ios/A b.swift' apps/ios/B.swift
  run -0 cat "$T/args"
  [ "$output" == '[lint][--strict][--quiet][--no-cache][--reporter][relative-path][--config][.swiftlint.yml][apps/ios/A b.swift][apps/ios/B.swift]' ]
  run -0 cat "$T/toolchain"
  [ "$output" == unset ]
}

@test "a finding (SwiftLint's exit 2 for an error, 3 for a warning) is exit 1, with the finding" {
  swiftlint 2 'apps/ios/A.swift:2:11: error: Force Cast Violation: Force casts should be avoided (force_cast)'
  run -1 lint apps/ios/A.swift
  has 'apps/ios/A.swift:2:11: error: Force Cast Violation: Force casts should be avoided (force_cast)'
  swiftlint 3 'apps/ios/A.swift:1:1: warning: Todo Violation: TODOs should be resolved (todo)'
  run -1 lint apps/ios/A.swift
}

@test "a failure without a finding line cannot run (exit 2): a bad config, no SourceKit, a missing tool" {
  swiftlint 134 'error: Cannot parse YAML file' 'Could not read configuration: file Configuration.swift, line 280'
  run -2 lint apps/ios/A.swift
  has 'Could not read configuration'
  swiftlint 2 'error: something else'
  run -2 lint apps/ios/A.swift
  swiftlint 133 'apps/ios/A.swift:2:11: error: Force Cast Violation: Force casts should be avoided (force_cast)'
  run -2 lint apps/ios/A.swift
}

@test "with only the command-line tools selected, SwiftLint gets TOOLCHAIN_DIR" {
  stub xcode-select 'echo /Library/Developer/CommandLineTools'
  swiftlint 0
  run -0 lint apps/ios/A.swift
  run -0 cat "$T/toolchain"
  [ "$output" == /Library/Developer/CommandLineTools ]
}
