#!/usr/bin/env bash
# Runs in the repo copy: a feat commit under apps/ with no specs/ change (selftest commits the leftover).
set -euo pipefail
echo '# Banner' >apps/api/BANNER.md
git add apps/api/BANNER.md
git -c user.name=fixture -c user.email=fixture@localhost -c commit.gpgsign=false -c core.hooksPath=/dev/null \
  commit -qm 'feat(api): add a banner'
echo more >>apps/api/BANNER.md
