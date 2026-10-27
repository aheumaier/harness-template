#!/usr/bin/env bash
# Runs in the repo copy: point make harness-determinism at a target whose verdict alternates through a counter
# in the gitignored .cache/ (passes on odd runs, fails on even ones; no randomness, so the fixture is stable).
set -euo pipefail
printf '%s\n' '' 'DETERMINISM_TARGET := flaky' 'flaky:' \
  $'\t@n=$$(cat .cache/flaky 2>/dev/null || echo 0); mkdir -p .cache; echo $$((n + 1)) >.cache/flaky; ((n % 2 == 0))' \
  >>Makefile
