#!/usr/bin/env bash
# Runs in the repo copy: LAYER_GLOB points at a directory with no Go package.
set -euo pipefail
sed -i.bak 's|^LAYER_GLOB=.*|LAYER_GLOB=domains/*/*|' scripts/harness/layout.env && rm scripts/harness/layout.env.bak
