#!/usr/bin/env bash
# Runs in the repo copy: the IOS_DIR package's manifest removed, its Swift tests kept. A project.yml stays so that
# only the missing package, not layout-check, is the finding.
set -euo pipefail
# shellcheck source=scripts/harness/layout.sh
source scripts/harness/layout.sh || exit 2
rm "$IOS_DIR/Package.swift"
echo 'name: App' >"$IOS_DIR/project.yml"
