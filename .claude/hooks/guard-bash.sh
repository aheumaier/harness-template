#!/usr/bin/env bash
# PreToolUse hook (Bash): blocks force pushes, pushes to main, recursive rm outside the repo and commands
# that name a .env file. Backs up the permissions.deny patterns in .claude/settings.json, which only match
# command text (e.g. a bare `git push` while on main, or `rm -rf "$HOME/x"`, slip past them).
# The checks live in .bin/harness (apps/api/tools/harness/guardbash.go, ADR 0012), which parses the command
# as bash and never runs it. Exit 2 blocks the call; unparseable input is blocked too (fail closed).
set -uo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd -P)"
# shellcheck source=../../scripts/harness/timing.sh
source "$ROOT/scripts/harness/timing.sh" 2>/dev/null && harness_time_exit hook guard-bash

[[ -x "$ROOT/.bin/harness" ]] || {
  # shellcheck source=../../scripts/harness/hook-bootstrap.sh
  source "$ROOT/scripts/harness/hook-bootstrap.sh" && harness_missing guard-bash
  exit 2
}
# Not exec: the timing EXIT trap above must run after it.
"$ROOT/.bin/harness" guard-bash "$ROOT"
