#!/usr/bin/env bash
# Runs in the repo copy: STACK narrowed to go on a tree that still has the web app and packages.
set -euo pipefail
sed -i.bak 's|^STACK=.*|STACK=go|' scripts/harness/layout.env && rm scripts/harness/layout.env.bak
