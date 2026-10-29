#!/usr/bin/env bash
# Runs in the repo copy: makes a local upstream (a clone in the gitignored .tmp/, so no network) whose Makefile
# moved on after this commit, records this commit in .harness-version and points HARNESS_UPSTREAM at the clone.
set -euo pipefail
up="$PWD/.tmp/upstream"
git clone -q . "$up"
printf '# upstream: a newer harness\n' >>"$up/Makefile"
git -C "$up" -c core.hooksPath=/dev/null -c user.name=fixture -c user.email=fixture@localhost \
  -c commit.gpgsign=false commit -qam "upstream harness change"
git rev-parse HEAD >.harness-version
sed -i.bak "s|^HARNESS_UPSTREAM=.*|HARNESS_UPSTREAM=$up|" scripts/tool-versions.env
rm -f scripts/tool-versions.env.bak
