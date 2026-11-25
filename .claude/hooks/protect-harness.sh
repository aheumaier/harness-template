#!/usr/bin/env bash
# PreToolUse hook (Edit|Write|MultiEdit|NotebookEdit|Bash): blocks agent edits to harness files, the
# globs in scripts/harness/protected-paths.txt (AGENTS.md golden principle 5). A human allows edits by
# starting the session with HARNESS_EDIT=1 in the environment.
# The checks live in .bin/harness (apps/api/tools/harness/protectharness.go, ADR 0045): file tools exactly,
# Bash heuristically, in every worktree of the repository. Exit 2 blocks the call; unparseable input is blocked
# too (fail closed).
set -uo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd -P)"
# shellcheck source=../../scripts/harness/timing.sh
source "$ROOT/scripts/harness/timing.sh" 2>/dev/null && harness_time_exit hook protect-harness
[[ "${HARNESS_EDIT:-}" == 1 ]] && exit 0
# shellcheck source=../../scripts/harness/hook-bootstrap.sh
source "$ROOT/scripts/harness/hook-bootstrap.sh" || {
  echo "protect-harness: blocked: scripts/harness/hook-bootstrap.sh is missing; restore it from git" >&2
  exit 2
}
input="$(cat)"
# The commands that build .bin/harness write no harness file; they pass before the binary is asked, so a
# missing or stale one never locks the repo.
harness_builds "$input" && exit 0
# The input is fed again only to reuse the shared message.
[[ -x "$ROOT/.bin/harness" ]] || harness_missing protect-harness <<<"$input"

rc=0
out="$("$ROOT/.bin/harness" protect-harness "$ROOT" <<<"$input" 2>&1)" || rc=$?
((rc == 0)) && exit 0
# Only exit 2 blocks in Claude Code, so a stale binary, one that cannot run (126, 127) or one that failed otherwise
# (1, a panic's 2, killed: 137) is turned into a block too.
harness_stale protect-harness "$rc" "$out" >&2
exit 2
