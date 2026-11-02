#!/usr/bin/env bash
# swift-format lint with the root config on the given Swift files (rules.md#ios-format), for make fmt-check-ios and
# the fast-check hook.
#
#   scripts/harness/swift-format.sh <file.swift>...
#
# swift-format exits 1 for a finding, a config it cannot read and a file that does not parse alike: exit 1 here only
# when it printed a finding line, 2 for any other failure (its errors are printed above), so a broken sensor never
# reads as a finding and `make fmt` is never offered for a syntax error. Without --follow-symlinks it skips a linked
# file and passes.
set -uo pipefail
out="$(swift-format lint --strict --follow-symlinks --configuration .swift-format "$@" 2>&1)" && s=0 || s=$?
[[ -z "$out" ]] || printf '%s\n' "$out"
((s == 0)) || { ((s == 1)) && grep -Eq ':[0-9]+:[0-9]+: (error|warning): \[[A-Za-z]+\]' <<<"$out" && s=1 || s=2; }
exit "$s"
