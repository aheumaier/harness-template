# Shared helpers for scripts/test/*.bats (make test-sh).
# shellcheck shell=bash

REPO="$(cd "$BATS_TEST_DIRNAME/../.." && pwd -P)"
export PATH="$REPO/.bin:$REPO/node_modules/.bin:$PATH"

# has <text>: $output (from bats `run`) contains text. A function, not a bare [[ ]]: bash 3.2 (macOS) does
# not fail a test on a false [[ ]] that is not the last command.
has() {
  # shellcheck disable=SC2154 # reason: $output is assigned by bats `run` (rule: sh-shellcheck)
  [[ "$output" == *"$1"* ]] && return 0
  printf 'expected output to contain: %s\nactual output:\n%s\n' "$1" "$output" >&2
  return 1
}

# hook <script> <command> [cwd]: feeds a PreToolUse Bash payload to .claude/hooks/<script>.
hook() {
  node -e 'process.stdout.write(JSON.stringify({ cwd: process.argv[2], tool_input: { command: process.argv[1] } }))' \
    "$2" "${3:-$REPO}" | "$REPO/.claude/hooks/$1"
}

# sandbox <path...>: throwaway git repo in $SANDBOX with copies of the given repo files (scripts resolve
# the repo root from their own location, so a copy operates on the sandbox).
sandbox() {
  SANDBOX="$BATS_TEST_TMPDIR/repo"
  local f
  for f in "$@"; do
    mkdir -p "$SANDBOX/$(dirname "$f")"
    cp "$REPO/$f" "$SANDBOX/$f"
  done
  git -C "$SANDBOX" init -q
}
