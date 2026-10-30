#!/usr/bin/env bash
# Runs in the repo copy: the adoption probe's layout.env keeps a LAYER_GLOB the brownfield repo does not have.
set -euo pipefail
f=scripts/templates/adoption-probe/layout.env
sed -i.bak 's|^LAYER_GLOB=.*|LAYER_GLOB=pkg/*/*|' "$f" && rm "$f.bak"
