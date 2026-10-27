#!/usr/bin/env bash
# PreToolUse hook for read-only subagents (code-reviewer): the only Bash command allowed is `make check`
# (optionally `make check-all`). Agent frontmatter `tools:` cannot restrict Bash to one command.
set -uo pipefail
cmd="$(node -e 'let s = ""; process.stdin.on("data", (d) => (s += d)).on("end", () => {
  try { process.stdout.write(JSON.parse(s).tool_input?.command ?? ""); } catch {}
})')"
cmd="$(sed -E 's/^[[:space:]]+|[[:space:]]+$//g' <<<"$cmd")"
[[ "$cmd" =~ ^make\ check(-all)?$ ]] && exit 0
echo "only-make-check: this reviewer may only run \`make check\` or \`make check-all\` (got: $cmd). Use Read/Grep/Glob to inspect code and \`git diff\` output provided in your prompt." >&2
exit 2
