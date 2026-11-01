#!/usr/bin/env bash
# SwiftLint with the root config on the given Swift files (rules.md#ios-lint and the size rules), for make lint-ios
# and the fast-check hook.
#
#   scripts/harness/swiftlint.sh <file.swift>...
#
# Exit 1 is a finding only when SwiftLint printed one (its exit 2 for an error, 3 for a warning under --strict); any
# other failure (134 for a bad config, 133 when it cannot load SourceKit) is exit 2, so a broken sensor never reads
# as a finding. Paths stay relative, so a rule id in the checkout's own path matches no finding line in run.sh. With
# only the command-line tools selected, SwiftLint finds SourceKit only through TOOLCHAIN_DIR. No cache: it keys on a
# linked file's own mtime, so an edit behind the link would pass.
set -uo pipefail
dev="$(xcode-select -p 2>/dev/null)" || dev=
[[ "$dev" != */CommandLineTools ]] || export TOOLCHAIN_DIR="$dev"
out="$(swiftlint lint --strict --quiet --no-cache --reporter relative-path --config .swiftlint.yml "$@" 2>&1)" && s=0 || s=$?
[[ -z "$out" ]] || printf '%s\n' "$out"
((s == 0)) || { ((s == 2 || s == 3)) && grep -Eq ': (error|warning): .* \([a-z_]+\)$' <<<"$out" && s=1 || s=2; }
exit "$s"
