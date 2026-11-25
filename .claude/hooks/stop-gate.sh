#!/usr/bin/env bash
# Stop hook: holds a session back once when its tree is not known to be green (harness-stop-is-green, ADR 0048).
# The check lives in .bin/harness (apps/api/tools/harness/stopgate.go). Exit 2 blocks the stop. A hook that cannot
# judge blocks too, but only a first stop: on a repeated one it says so to the person and lets go, or a checkout
# that cannot build the binary would never hand back.
set -uo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd -P)"
# shellcheck source=../../scripts/harness/timing.sh
source "$ROOT/scripts/harness/timing.sh" 2>/dev/null && harness_time_exit hook stop-gate
input="$(cat)"

# The match is on the raw JSON: a quote inside a JSON string is escaped, so only the key matches. Anything but a
# plain `false` counts as a repeated stop.
block_once() { # <message>
  local LC_ALL=C key='"stop_hook_active"[[:space:]]*:[[:space:]]*'
  printf '%s\n' "$1" >&2
  if [[ "$input" =~ ${key}false && ! "$input" =~ ${key}true ]]; then
    echo '[harness-stop-is-green] The next stop passes either way.' >&2
    exit 2
  fi
  echo '{"systemMessage":"harness-stop-is-green: the stop gate could not judge this tree (most likely `make setup` helps, or `make .bin/harness` when the binary is old); it is not known to be green"}'
  exit 0
}

# shellcheck source=../../scripts/harness/hook-bootstrap.sh
source "$ROOT/scripts/harness/hook-bootstrap.sh" ||
  block_once "stop-gate: blocked: scripts/harness/hook-bootstrap.sh is missing; restore it from git"
[[ -x "$ROOT/.bin/harness" ]] || block_once "$(harness_missing stop-gate 2>&1 <<<"$input")"

# At exit 0 the binary writes nothing to stderr, so the systemMessage stays one JSON object.
rc=0
out="$("$ROOT/.bin/harness" stop-gate "$ROOT" <<<"$input" 2>&1)" || rc=$?
((rc == 0)) || block_once "$(harness_stale stop-gate "$rc" "$out")"
[[ -z "$out" ]] || printf '%s\n' "$out"
