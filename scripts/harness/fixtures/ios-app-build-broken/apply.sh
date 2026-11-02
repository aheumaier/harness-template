#!/usr/bin/env bash
# Runs in the repo copy: a Swift file in the app target (project.yml's sources: App) that does not compile.
set -euo pipefail
# shellcheck source=scripts/harness/layout.sh
source scripts/harness/layout.sh || exit 2
[[ -f "$IOS_DIR/project.yml" && -d "$IOS_DIR/App" ]] || {
  echo "ios-app-build-broken: no $IOS_DIR/project.yml with the app's sources in $IOS_DIR/App/, the shape build-ios expects (ADR 0044)" >&2
  exit 2
}
echo 'let harnessSelftestBroken: Int = harnessSelftestUndefined' >"$IOS_DIR/App/HarnessSelftestBroken.swift"
