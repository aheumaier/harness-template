#!/usr/bin/env bash
# Runs in the repo copy: the file below in the first source target of the IOS_DIR package, so the fixture holds in
# any layout.
set -euo pipefail
# shellcheck source=scripts/harness/layout.sh
source scripts/harness/layout.sh || exit 2
targets=("$IOS_DIR"/Sources/*/)
[[ -d "${targets[0]}" ]] || {
  echo "no source target in $IOS_DIR/Sources/" >&2
  exit 1
}
printf '%s\n' 'public func harnessSelftestForceCast(_ value: Any) -> Int {' '    value as! Int' '}' \
  >"${targets[0]}HarnessSelftestForceCast.swift"
