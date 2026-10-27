#!/usr/bin/env bats
# scripts/harness/run.sh: silent on success; on failure the tool output plus each rule's remediation.
bats_require_minimum_version 1.5.0
load helpers

setup() {
  sandbox scripts/harness/run.sh
  mkdir -p "$SANDBOX/docs/harness" "$SANDBOX/sub"
  printf '%s\n' '### rule-a' '- **Remediation:** Fix A.' '' '### rule-b' '- **Remediation:** Fix B.' \
    >"$SANDBOX/docs/harness/rules.md"
  RUN="$SANDBOX/scripts/harness/run.sh"
}

@test "is silent and exits 0 when the sensor passes" {
  run -0 "$RUN" sensor rule-a -- echo noisy
  [ -z "$output" ]
}

@test "keeps the exit code and shows the remediation of the rule named in the output" {
  run -3 "$RUN" sensor rule-a,rule-b -- bash -c 'echo "x.go:1: rule-b violated"; exit 3'
  has "✗ sensor 'sensor' failed (exit 3)"
  has 'x.go:1: rule-b violated'
  has 'remediation: Fix B.'
  has 'docs:        docs/harness/rules.md#rule-b'
  run ! has 'Fix A.'
}

@test "shows every rule when the output names none" {
  run -1 "$RUN" sensor rule-a,rule-b -- false
  has 'Fix A.'
  has 'Fix B.'
}

@test "runs in --dir and truncates long output" {
  HARNESS_MAX_LINES=2 run -1 "$RUN" sensor rule-a --dir sub -- bash -c 'seq 6; exit 1'
  has 'cwd: sub'
  has '… 4 more lines'
}

@test "records every run in .cache/harness/timings.jsonl" {
  run -0 "$RUN" quick rule-a -- true
  run -2 "$RUN" broken rule-a -- bash -c 'exit 2'
  run cat "$SANDBOX/.cache/harness/timings.jsonl"
  has '{"sensor":"quick","start_ms":'
  has '"exit":0,"scope":"changed"'
  has '{"sensor":"broken","start_ms":'
  has '"exit":2,'
}

@test "warns over budget and keeps the sensor's exit status" {
  printf '%s\n' '# comment' 'default 600000 NFR-H5' 'slow 1 NFR-H5' >"$SANDBOX/scripts/harness/budgets.txt"
  run -0 --separate-stderr "$RUN" slow rule-a -- sleep 0.1
  [ -z "$output" ]
  output="$stderr"
  has "harness: sensor 'slow' took "
  has " s (budget 0.001 s, NFR-H5)"
  run -1 "$RUN" slow rule-a -- bash -c 'sleep 0.1; exit 1'
  has "harness: sensor 'slow' took"
  has 'remediation: Fix A.'
}

@test "uses the default budget for a sensor without a row and is silent under budget" {
  printf '%s\n' 'default 1 NFR-H5' 'roomy 600000 NFR-H5' >"$SANDBOX/scripts/harness/budgets.txt"
  run -0 "$RUN" other rule-a -- sleep 0.1
  has "harness: sensor 'other' took"
  run -0 "$RUN" roomy rule-a -- true
  [ -z "$output" ]
}

@test "a timing that cannot be written does not fail or print" {
  printf 'not a dir' >"$SANDBOX/.cache"
  run -0 "$RUN" sensor rule-a -- true
  [ -z "$output" ]
}
