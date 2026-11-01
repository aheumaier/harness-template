#!/usr/bin/env bash
# Runs in the repo copy: a SignOutTests.swift naming 900/US1-AS1 in an app source folder, not a *Tests one,
# under the first PRODUCT_GLOBS entry (any layout, README (b)); a real test names 900/US1-AS2.
set -euo pipefail
# shellcheck source=scripts/harness/layout.sh
source scripts/harness/layout.sh || exit 2
ios="${PRODUCT_GLOBS[0]%/\*}/ios"
mkdir -p "$ios/Sources/Fixture" "$ios/Tests/FixtureTests"
printf '%s\n' 'import Testing' '' '@Test("900/US1-AS1 shows the sign-in screen") func showsSignIn() {}' \
  >"$ios/Sources/Fixture/SignOutTests.swift"
printf '%s\n' 'import Testing' '' '@Test("900/US1-AS2 clears the cached week") func clearsCache() {}' \
  >"$ios/Tests/FixtureTests/CacheTests.swift"
