#!/usr/bin/env bash
# Runs in the repo copy: ios dropped from STACK while Swift stays under a PRODUCT_GLOBS entry. In a layout
# without Swift (the adoption probe's, README (b)) the Swift is written first, under the first entry.
set -euo pipefail
# shellcheck source=scripts/harness/layout.sh
source scripts/harness/layout.sh
harness_layout_load scripts/harness/layout.env
if [[ -z "$(git ls-files -co --exclude-standard -- "${PRODUCT_GLOBS[@]/%/.swift}")" ]]; then
  mkdir -p "${PRODUCT_GLOBS[0]%/\*}/ios"
  echo '// swift-tools-version:6.0' >"${PRODUCT_GLOBS[0]%/\*}/ios/Package.swift"
fi
stack=":$STACK:"
stack="${stack//:ios:/:}"
stack="${stack#:}"
sed -i.bak "s|^STACK=.*|STACK=${stack%:}|" scripts/harness/layout.env && rm scripts/harness/layout.env.bak
