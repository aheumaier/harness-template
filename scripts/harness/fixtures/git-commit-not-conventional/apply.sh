#!/usr/bin/env bash
# Runs in the repo copy: commit with a free-form subject (selftest then commits the leftover change).
set -euo pipefail
echo x >>ARCHITECTURE.md
git -c user.name=fixture -c user.email=fixture@localhost -c commit.gpgsign=false -c core.hooksPath=/dev/null \
  commit -qam 'update stuff'
echo y >>ARCHITECTURE.md
