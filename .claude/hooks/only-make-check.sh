#!/usr/bin/env bash
# PreToolUse hook (Bash) for subagents whose only shell command is `make check` or `make check-all`
# (code-reviewer, test-writer). Agent frontmatter `tools:` cannot restrict Bash to one command.
set -uo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd -P)"
# shellcheck source=../../scripts/harness/timing.sh
source "$ROOT/scripts/harness/timing.sh" 2>/dev/null && harness_time_exit hook only-make-check
[[ -x "$ROOT/.bin/harness" ]] || {
  # shellcheck source=../../scripts/harness/hook-bootstrap.sh
  source "$ROOT/scripts/harness/hook-bootstrap.sh" && harness_missing only-make-check 'check|check-all'
  exit 2
}
cmd="$("$ROOT/.bin/harness" hook-input command 2>/dev/null)"
cmd="$(sed -E 's/^[[:space:]]+|[[:space:]]+$//g' <<<"$cmd")"
[[ "$cmd" =~ ^make\ check(-all)?$ ]] && exit 0
echo "only-make-check: this agent may only run \`make check\` or \`make check-all\` (got: $cmd). Use Read/Grep/Glob to inspect code, and the \`git diff\` output provided in your prompt." >&2
exit 2
