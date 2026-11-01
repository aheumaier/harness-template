#!/usr/bin/env bash
# Runs in the repo copy: ios dropped from STACK while Swift is under a PRODUCT_GLOBS entry. The Swift is
# written here, under the first entry, so the fixture holds in any layout (the adoption probe's has none).
set -euo pipefail
# shellcheck source=scripts/harness/layout.sh
source scripts/harness/layout.sh || exit 2
dir="${PRODUCT_GLOBS[0]%/\*}/hidden-ios"
mkdir -p "$dir"
echo 'struct Hidden {}' >"$dir/Hidden.swift"
stack=":$STACK:" # wrapped in `:`, so ios drops from any position
stack="${stack//:ios:/:}"
stack="${stack#:}"
sed -i.bak "s|^STACK=.*|STACK=${stack%:}|" scripts/harness/layout.env && rm scripts/harness/layout.env.bak
