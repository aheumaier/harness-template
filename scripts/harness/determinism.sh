#!/usr/bin/env bash
# NFR-R1 sensor (rules.md#harness-deterministic): runs a command twice on the same tree and fails when the two
# runs differ in exit status, in findings, or in the working tree they leave behind.
#
#   scripts/harness/determinism.sh <command...>     (make harness-determinism; nightly in harness-nfr.yml)
#
# Findings are compared as sorted, de-duplicated output lines: `make -k -j4` interleaves sensor output in a
# different order on every run. Durations ("1.2s", "340ms", timing.sh rows) and temp paths are masked first.
# The inner runs have no latency budgets (HARNESS_BUDGETS=/dev/null), because wall time differs by design and
# only warns (NFR-H*), and no output cap (HARNESS_MAX_LINES), because run.sh names its temp log when it
# truncates.
set -euo pipefail

(($# > 0)) || {
  echo "usage: determinism.sh <command...>" >&2
  exit 2
}
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"
tmp="$(mktemp -d -t harness-determinism.XXXXXX)"
trap 'rm -rf "$tmp"' EXIT

normalise() {
  LC_ALL=C sed -E -e 's#(/private)?/(tmp|var/folders)/[^ :]*#<tmp>#g' \
    -e 's/[0-9]+(\.[0-9]+)?(ms|s)([^[:alpha:]]|$)/<t>\3/g' \
    -e 's/"(start_ms|ms)":[0-9]+/"\1":<t>/g' | LC_ALL=C sort -u
}

git status --porcelain >"$tmp/tree0"
for run in 1 2; do
  status=0
  HARNESS_BUDGETS=/dev/null HARNESS_MAX_LINES=1000000 "$@" >"$tmp/out$run" 2>&1 || status=$?
  echo "$status" >"$tmp/exit$run"
  normalise <"$tmp/out$run" >"$tmp/findings$run"
  git status --porcelain >"$tmp/tree$run"
done

fail=0
if ! cmp -s "$tmp/exit1" "$tmp/exit2"; then
  echo "harness-deterministic: verdict differs: run 1 exit $(cat "$tmp/exit1"), run 2 exit $(cat "$tmp/exit2") ($*)"
  fail=1
fi
if ! cmp -s "$tmp/findings1" "$tmp/findings2"; then
  echo "harness-deterministic: findings differ (< run 1, > run 2):"
  diff "$tmp/findings1" "$tmp/findings2" | grep '^[<>]' || true
  fail=1
fi
for run in 1 2; do
  if ! cmp -s "$tmp/tree0" "$tmp/tree$run"; then
    echo "harness-deterministic: run $run changed the working tree (git status --porcelain):"
    diff "$tmp/tree0" "$tmp/tree$run" | grep '^[<>]' || true
    fail=1
  fi
done
exit "$fail"
