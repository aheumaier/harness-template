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
