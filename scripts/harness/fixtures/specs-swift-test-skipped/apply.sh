#!/usr/bin/env bash
# Runs in the repo copy: a Swift test file under the first PRODUCT_GLOBS entry (any layout, README (b)) that
# names 900/US1-AS1 in a test that runs and 900/US1-AS2 in one disabled on the same line.
set -euo pipefail
# shellcheck source=scripts/harness/layout.sh
source scripts/harness/layout.sh
harness_layout_load scripts/harness/layout.env
dir="${PRODUCT_GLOBS[0]%/\*}/ios/Tests/FixtureTests"
mkdir -p "$dir"
cat >"$dir/SignOutTests.swift" <<'SWIFT'
import Testing

@Test("900/US1-AS1 shows the sign-in screen") func showsSignIn() {}
@Test("900/US1-AS2 clears the cached week", .disabled("offline cache not built yet")) func clearsCache() {}
SWIFT
