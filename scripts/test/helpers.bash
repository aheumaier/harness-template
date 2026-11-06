# Shared helpers for scripts/test/*.bats (make test-sh).
# shellcheck shell=bash

REPO="$(cd "$BATS_TEST_DIRNAME/../.." && pwd -P)"
export PATH="$REPO/.bin:$REPO/node_modules/.bin:$PATH"
# No latency budgets (scripts/harness/timing.sh): a loaded machine must not add warnings to tested output.
# Tests of the budgets themselves unset this or point it at their own file.
export HARNESS_BUDGETS=/dev/null
# Timing rows default to scope "changed"; make check-all and the nightly double run export HARNESS_SCOPE=full.
unset HARNESS_SCOPE
# A run's stack override (ADR 0033) would re-point the scripts under test: they read their sandbox's layout.env.
unset HARNESS_STACK

# has <text>: $output (from bats `run`) contains text. A function, not a bare [[ ]]: bash 3.2 (macOS) does
# not fail a test on a false [[ ]] that is not the last command.
has() {
  # shellcheck disable=SC2154 # reason: $output is assigned by bats `run` (rule: sh-shellcheck)
  [[ "$output" == *"$1"* ]] && return 0
  printf 'expected output to contain: %s\nactual output:\n%s\n' "$1" "$output" >&2
  return 1
}

# lacks <text>: $output does not contain text (has's opposite).
lacks() {
  [[ "$output" != *"$1"* ]] && return 0
  printf 'expected output not to contain: %s\nactual output:\n%s\n' "$1" "$output" >&2
  return 1
}

# hook <script> <command> [cwd]: feeds a PreToolUse Bash payload to .claude/hooks/<script>.
hook() {
  node -e 'process.stdout.write(JSON.stringify({ cwd: process.argv[2], tool_input: { command: process.argv[1] } }))' \
    "$2" "${3:-$REPO}" | "$REPO/.claude/hooks/$1"
}

# sandbox <path...>: throwaway git repo in $SANDBOX with the given repo files (scripts resolve the repo root
# from their own location, so they operate on the sandbox). Executables are hardlinked: macOS assesses every
# newly written executable on its first exec (~0.35 s each; exec plan 0002). Data files are copied, and so is
# an executable when the link fails (another volume). A link writes through to the real file: call `own` before
# a test writes to a sandboxed executable; teardown_file fails the file if one did not. Every sandbox also gets
# scripts/harness/layout.sh and layout.env: the scripts read the repo layout from them.
sandbox() {
  SANDBOX="$BATS_TEST_TMPDIR/repo"
  local f
  for f in scripts/harness/layout.sh scripts/harness/layout.env "$@"; do
    [[ ! -e "$SANDBOX/$f" ]] || continue # a second sandbox call, or a caller naming the layout files too
    mkdir -p "$SANDBOX/$(dirname "$f")"
    if [[ -x "$REPO/$f" ]] && ln "$REPO/$f" "$SANDBOX/$f" 2>/dev/null; then continue; fi
    cp "$REPO/$f" "$SANDBOX/$f"
  done
  git -C "$SANDBOX" init -q
}

# edit <file> <sed-expr>: edits a sandbox file in place (portable sed -i). A sandboxed executable is a hardlink:
# call `own` on it first.
edit() { sed -i.bak "$2" "$SANDBOX/$1" && rm -f "$SANDBOX/$1.bak"; }

# host <Darwin|Linux>: a uname on PATH whose -s names that host, so a Makefile's HOST_OFF follows it; every
# other uname call goes to the real one.
host() {
  stub uname "[ \"\$1\" = -s ] && echo $1 || exec /usr/bin/uname \"\$@\""
  PATH="$BATS_TEST_TMPDIR/bin:$PATH"
}

# stub <command> <sh body>: a command in $BATS_TEST_TMPDIR/bin, for this test only.
stub() {
  mkdir -p "$BATS_TEST_TMPDIR/bin" && printf '#!/bin/sh\n%s\n' "$2" >"$BATS_TEST_TMPDIR/bin/$1"
  chmod +x "$BATS_TEST_TMPDIR/bin/$1"
}

# own <path...>: replaces sandboxed files with private copies (same content and mode), breaking the hardlink
# to the real file before a test writes to them.
own() {
  local f
  for f in "$@"; do
    cp "$SANDBOX/$f" "$SANDBOX/$f.own" && mv -f "$SANDBOX/$f.own" "$SANDBOX/$f"
  done
}

# Write-through guard for the hardlinks above, in every bats file that loads this one: the real harness
# scripts must be byte-identical, with the same executable bits, after the file's tests. A checksum, not `git diff --quiet`, so uncommitted
# harness edits of the developer do not fail it. A file that needs its own setup_file/teardown_file calls these.
harness_cksum() {
  (cd "$REPO" && find scripts .claude/hooks -type f -exec cksum {} + | LC_ALL=C sort -k3 &&
    find scripts .claude/hooks -type f -perm -u+x | LC_ALL=C sort)
}
setup_file() { harness_cksum >"$BATS_FILE_TMPDIR/harness.cksum"; }
teardown_file() {
  harness_cksum >"$BATS_FILE_TMPDIR/harness.cksum.after"
  cmp -s "$BATS_FILE_TMPDIR/harness.cksum" "$BATS_FILE_TMPDIR/harness.cksum.after" && return 0
  echo "a test wrote through a sandbox hardlink into the real harness files (restore them with git; call own <path> before writing):" >&2
  diff "$BATS_FILE_TMPDIR/harness.cksum" "$BATS_FILE_TMPDIR/harness.cksum.after" >&2
  return 1
}

# commit_all <repo> <msg>: commits everything in <repo> with a test identity, unsigned and without hooks.
commit_all() {
  git -C "$1" add -A
  git -C "$1" -c core.hooksPath=/dev/null -c user.name=test -c user.email=test@localhost -c commit.gpgsign=false \
    commit -qm "$2"
}

# sandbox_base: commit everything in $SANDBOX as branch main (the BASE_REF for diff-based checks).
sandbox_base() {
  commit_all "$SANDBOX" base
  git -C "$SANDBOX" branch -M main
}

# spec <file> <path...>: writes a minimal OpenAPI document with one GET per path to <file>.
spec() {
  local f="$1"
  shift
  mkdir -p "$(dirname "$f")"
  {
    printf 'openapi: 3.0.3\ninfo: { title: t, version: "1" }\npaths:\n'
    printf '  %s:\n    get:\n      responses:\n        "200":\n          description: ok\n' "$@"
  } >"$f"
}
