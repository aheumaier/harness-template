#!/usr/bin/env bash
# Runs in the repo copy: a failing Swift Testing test in the first test target of the IOS_DIR package, so the
# fixture holds in any layout.
set -euo pipefail
# shellcheck source=scripts/harness/layout.sh
source scripts/harness/layout.sh || exit 2
targets=("$IOS_DIR"/Tests/*/)
[[ -d "${targets[0]}" ]] || {
  echo "no test target in $IOS_DIR/Tests/" >&2
  exit 1
}
printf '%s\n' 'import Testing' '' '@Test func harnessSelftestFails() {' '    #expect(1 + 1 == 3)' '}' \
  >"${targets[0]}HarnessSelftestTests.swift"
