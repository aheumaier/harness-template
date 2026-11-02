#!/usr/bin/env bash
# Runs in the repo copy: a Swift test in a Tests/ folder no test target of the IOS_DIR package declares.
set -euo pipefail
# shellcheck source=scripts/harness/layout.sh
source scripts/harness/layout.sh || exit 2
mkdir -p "$IOS_DIR/Tests/LooseTests"
cat >"$IOS_DIR/Tests/LooseTests/LooseTests.swift" <<'SWIFT'
import Testing

@Test func looseTestNeverRuns() {
    #expect(Bool(false))
}
SWIFT
