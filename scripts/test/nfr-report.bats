#!/usr/bin/env bats
# scripts/harness/nfr-report.sh: p95 per sensor or hook over timings.jsonl files, only names over budget.
bats_require_minimum_version 1.5.0
load helpers

setup() {
  sandbox scripts/harness/nfr-report.sh
  R="$SANDBOX/scripts/harness/nfr-report.sh"
  export HARNESS_BUDGETS="$BATS_TEST_TMPDIR/budgets.txt"
  printf '%s\n' '# comment' 'default 100 NFR-H5' 'fast-hook 1000 NFR-H1' >"$HARNESS_BUDGETS"
}

# rows <file> <name> <ms...>: timing.sh rows for one name.
rows() {
  local f="$1" name="$2" ms
  shift 2
  for ms in "$@"; do
    printf '{"sensor":"%s","start_ms":1,"ms":%s,"exit":0,"scope":"changed","sha":"abc","os":"linux"}\n' "$name" "$ms" >>"$f"
  done
}

@test "reports the nearest-rank p95 of every name over budget, across files" {
  rows "$BATS_TEST_TMPDIR/a.jsonl" slow 10 20 30 40 50 60 70 80 90 100
  rows "$BATS_TEST_TMPDIR/b.jsonl" slow 110 120 130 140 150 160 170 180 190 200
  rows "$BATS_TEST_TMPDIR/b.jsonl" fast-hook 900 900 900
  run -0 "$R" "$BATS_TEST_TMPDIR/a.jsonl" "$BATS_TEST_TMPDIR/b.jsonl"
  has '| Sensor or hook | Runs | p95 (s) | Budget (s) | NFR |'
  has '| slow | 20 | 0.19 | 0.1 | NFR-H5 |'
  run ! has 'fast-hook'
}

@test "one outlier in twenty runs stays under the p95" {
  rows "$BATS_TEST_TMPDIR/a.jsonl" mostly-fast 50 50 50 50 50 50 50 50 50 50 50 50 50 50 50 50 50 50 50 9000
  run -0 "$R" "$BATS_TEST_TMPDIR/a.jsonl"
  [ -z "$output" ]
}

@test "fails closed when there are no timing rows" {
  : >"$BATS_TEST_TMPDIR/empty.jsonl"
  run -2 "$R" "$BATS_TEST_TMPDIR/empty.jsonl"
  has 'nfr-report: no timing rows'
  run -2 "$R"
  has 'usage: nfr-report.sh'
}

@test "drops rows whose name is not a plain id (artifacts from PR runs are untrusted)" {
  rows "$BATS_TEST_TMPDIR/a.jsonl" '<img src=x>' 9000
  rows "$BATS_TEST_TMPDIR/a.jsonl" slow 9000
  run -0 "$R" "$BATS_TEST_TMPDIR/a.jsonl"
  has '| slow | 1 | 9 | 0.1 | NFR-H5 |'
  run ! has 'img'
}

@test "reads the time of run.sh rows carrying findings, never a number inside them" {
  printf '{"sensor":"lint","start_ms":1,"ms":%s,"exit":1,"scope":"full","sha":"abc","os":"linux","findings":{"ms":9000,"rule-a":1}}\n' \
    50 150 >"$BATS_TEST_TMPDIR/a.jsonl"
  run -0 "$R" "$BATS_TEST_TMPDIR/a.jsonl"
  has '| lint | 2 | 0.15 | 0.1 | NFR-H5 |'
}
