#!/usr/bin/env bash
# Deliberate violation: unquoted expansion in rm (word splitting, globbing).
set -euo pipefail
target=$1
rm -rf $target/build
