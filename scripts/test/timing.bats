#!/usr/bin/env bats
# scripts/harness/timing.sh: every sensor run and hook call lands in .cache/harness/timings.jsonl; over its
# budget it warns (and, in GitHub Actions, annotates) without changing the exit status.
bats_require_minimum_version 1.5.0
load helpers

setup() {
  unset GITHUB_ACTIONS GITHUB_STEP_SUMMARY HARNESS_EDIT
  sandbox scripts/harness/timing.sh scripts/harness/protected-paths.txt scripts/tool-versions.env \
    .claude/hooks/protect-harness.sh .claude/hooks/guard-bash.sh .claude/hooks/only-make-check.sh \
    .claude/hooks/only-test-files.sh .claude/hooks/fast-check.sh .bin/harness
  export HARNESS_BUDGETS="$SANDBOX/budgets.txt"
  printf '%s\n' 'default 600000 NFR-H5' >"$HARNESS_BUDGETS"
  TIMINGS="$SANDBOX/.cache/harness/timings.jsonl"
}

# payload <command>: writes a PreToolUse Bash payload for the sandbox to $PAYLOAD.
payload() {
  PAYLOAD="$BATS_TEST_TMPDIR/payload.json"
  node -e 'process.stdout.write(JSON.stringify({ cwd: process.argv[2], tool_name: "Bash",
    tool_input: { command: process.argv[1] } }))' "$1" "$SANDBOX" >"$PAYLOAD"
}

@test "every hook records its call with scope hook and no sha" {
  payload 'make check'
  for h in protect-harness guard-bash only-make-check; do
    run -0 "$SANDBOX/.claude/hooks/$h.sh" <"$PAYLOAD"
  done
  run -0 "$SANDBOX/.claude/hooks/only-test-files.sh" <<<"{\"tool_name\":\"Write\",\"tool_input\":{\"file_path\":\"$SANDBOX/a_test.go\"}}"
  run -0 "$SANDBOX/.claude/hooks/fast-check.sh" budgets.txt
  run cat "$TIMINGS"
  for h in protect-harness guard-bash only-make-check only-test-files fast-check; do
    has "{\"sensor\":\"$h\",\"start_ms\":"
  done
  has '"exit":0,"scope":"hook","sha":"",'
}

@test "a blocked hook call keeps exit 2 and its reason, and warns over budget" {
  printf '%s\n' 'only-make-check 1 NFR-H1' >"$HARNESS_BUDGETS"
  payload 'make gen'
  run -2 --separate-stderr "$SANDBOX/.claude/hooks/only-make-check.sh" <"$PAYLOAD"
  [ -z "$output" ]
  output="$stderr"
  has 'only-make-check: this agent may only run'
  has "harness: hook 'only-make-check' took "
  has " s (budget 0.001 s, NFR-H1)"
  run cat "$TIMINGS"
  has '"exit":2,"scope":"hook"'
}

@test "in GitHub Actions a warning is also an annotation and a job-summary line" {
  printf '%s\n' 'slow 1 NFR-H5' >"$HARNESS_BUDGETS"
  export GITHUB_ACTIONS=true GITHUB_STEP_SUMMARY="$BATS_TEST_TMPDIR/summary.md"
  run -0 --separate-stderr bash -c 'ROOT="$1"; source "$1/scripts/harness/timing.sh"; harness_record sensor slow 0 1' _ "$SANDBOX"
  has "::warning title=harness-latency-budget::harness: sensor 'slow' took "
  output="$stderr"
  has "harness: sensor 'slow' took "
  run cat "$GITHUB_STEP_SUMMARY"
  has "- harness: sensor 'slow' took "
}

@test "is silent under budget and outside GitHub Actions" {
  run -0 bash -c 'ROOT="$1"; source "$1/scripts/harness/timing.sh"; harness_record sensor fast "$(harness_now_ms)" 0' _ "$SANDBOX"
  [ -z "$output" ]
  printf '%s\n' 'slow 1 NFR-H5' >"$HARNESS_BUDGETS"
  run -0 --separate-stderr bash -c 'ROOT="$1"; source "$1/scripts/harness/timing.sh"; harness_record sensor slow 0 0' _ "$SANDBOX"
  [ -z "$output" ]
}

@test "formats milliseconds as seconds without trailing zeros" {
  run -0 bash -c 'source "$1"; for ms in 1 50 999 5000 52830; do harness_seconds "$ms"; done' _ "$SANDBOX/scripts/harness/timing.sh"
  [ "$output" = $'0.001\n0.05\n0.999\n5\n52.83' ]
}

@test "a hook without the helper still works" {
  rm "$SANDBOX/scripts/harness/timing.sh"
  payload 'make gen'
  run -2 "$SANDBOX/.claude/hooks/only-make-check.sh" <"$PAYLOAD"
  has 'only-make-check: this agent may only run'
  run ! has 'timing.sh'
  [ ! -e "$TIMINGS" ]
}
