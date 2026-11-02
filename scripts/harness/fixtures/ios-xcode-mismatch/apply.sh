#!/usr/bin/env bash
# Runs in the repo copy: pins an Xcode version no host has.
set -euo pipefail
sed -i.bak 's/^XCODE_VERSION=.*/XCODE_VERSION=0.0/' scripts/tool-versions.env
rm scripts/tool-versions.env.bak
grep -qx 'XCODE_VERSION=0.0' scripts/tool-versions.env
