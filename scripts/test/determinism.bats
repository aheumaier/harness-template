#!/usr/bin/env bats
# scripts/harness/determinism.sh: two runs of a command on the same tree must agree on exit status, findings
# (order, durations and temp paths aside) and the tree they leave behind.
bats_require_minimum_version 1.5.0
load helpers

setup() {
  sandbox scripts/harness/determinism.sh
  sandbox_base
  D="$SANDBOX/scripts/harness/determinism.sh"
  # cmd.sh: a fake sensor run; $n is 1 on the first run and 2 on the second (counter outside the tree).
  CMD="$BATS_TEST_TMPDIR/cmd.sh"
  printf '%s\n' '#!/usr/bin/env bash' "c='$BATS_TEST_TMPDIR/n'" 'n=$(($(cat "$c" 2>/dev/null || echo 0) + 1)); echo "$n" >"$c"' >"$CMD"
  chmod +x "$CMD"
}

body() { printf '%s\n' "$@" >>"$CMD"; }

@test "passes when only output order, durations and temp paths differ" {
  body 'echo "{\"sensor\":\"s\",\"start_ms\":17931417865$n,\"ms\":$n,\"exit\":0}"'
  body 'if ((n == 1)); then echo "a.go:1: bad"; echo "FAIL pkg 0.41s, /tmp/x.$n/log"; else echo "FAIL pkg 12ms, /tmp/x.$n/log"; echo "a.go:1: bad"; fi' 'exit 1'
  run -0 "$D" "$CMD"
  [ -z "$output" ]
}

@test "fails when the verdict alternates" {
  body 'exit $((n - 1))'
  run -1 "$D" "$CMD"
  has 'harness-deterministic: verdict differs: run 1 exit 0, run 2 exit 1'
}

@test "fails when the findings differ" {
  body 'echo "a.go:$n: bad"' 'exit 1'
  run -1 "$D" "$CMD"
  has 'findings differ (< run 1, > run 2):'
  has '< a.go:1: bad'
  has '> a.go:2: bad'
  run ! has 'verdict differs'
}

@test "fails when a run changes the working tree" {
  body 'cd "$(git rev-parse --show-toplevel)" && echo "$n" >written-by-sensor'
  run -1 "$D" "$CMD"
  has 'run 1 changed the working tree'
  has '> ?? written-by-sensor'
}

@test "needs a command" {
  run -2 "$D"
  has 'usage: determinism.sh <command...>'
}
