#!/usr/bin/env bash
# PreToolUse hook (Edit|Write|MultiEdit|NotebookEdit) for the test-writer subagent (ADR 0039): it may write
# only product test files, as `harness test-file` decides with the predicate specs-tested reads tests with
# (productTests in apps/api/tools/harness/testfile.go), which also prints why a path is refused and the test file
# forms. Agent frontmatter `tools:` cannot restrict a file tool to some paths. Exit 2 blocks; unparseable input too.
set -uo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd -P)"
# shellcheck source=../../scripts/harness/timing.sh
source "$ROOT/scripts/harness/timing.sh" 2>/dev/null && harness_time_exit hook only-test-files
[[ -x "$ROOT/.bin/harness" ]] || {
  # shellcheck source=../../scripts/harness/hook-bootstrap.sh
  source "$ROOT/scripts/harness/hook-bootstrap.sh" && harness_missing only-test-files
  exit 2
}
input="$("$ROOT/.bin/harness" hook-input cwd target 2>/dev/null)" || {
  echo "only-test-files: blocked: cannot parse the hook input" >&2
  exit 2
}
cwd="${input%%$'\n'*}" target="${input#*$'\n'}"
[[ -z "$target" || "$target" == /* ]] || target="${cwd:-$PWD}/$target"
why="$("$ROOT/.bin/harness" test-file "$ROOT" "$target" 2>&1)" && exit 0
echo "only-test-files: blocked: ${why#harness test-file: }. test-writer writes only the product test files its tasks name; the implementer writes everything else." >&2
exit 2
