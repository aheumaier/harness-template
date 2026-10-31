#!/usr/bin/env bats
# scripts/harness/rule-fires.sh: the ledger of rule fires carried between drift runs, and the report of rules
# that never fired in the window (ADR 0032).
bats_require_minimum_version 1.5.0
load helpers

setup() {
  sandbox scripts/harness/rule-fires.sh
  R="$SANDBOX/scripts/harness/rule-fires.sh"
  mkdir -p "$SANDBOX/docs/harness"
  printf '%s\n' '# Harness rules' '### rule-a' 'x' '### rule-b' '### rule-c' >"$SANDBOX/docs/harness/rules.md"
  L="$BATS_TEST_TMPDIR/ledger"
  D="$BATS_TEST_TMPDIR/timings" # one directory per artifact, as ci-timings.sh downloads them
  mkdir -p "$D"
  export RULE_FIRES_DAYS=90
}

# at <day>: RULE_FIRES_NOW for 23:00 of that day (days since the epoch, UTC).
at() { export RULE_FIRES_NOW=$(($1 * 86400 + 82800)); }

# rowms <artifact-id> <start-ms> <findings-json> [sensor]: a run.sh timing row in that artifact.
rowms() {
  mkdir -p "$D/$1"
  printf '{"sensor":"%s","start_ms":%s,"ms":5,"exit":0,"scope":"full","sha":"abc","os":"linux-gnu","findings":%s}\n' \
    "${4:-lint}" "$2" "$3" >>"$D/$1/timings.jsonl"
}

# row <artifact-id> <day> <findings-json> [sensor]: the same, starting at noon of that day.
row() { rowms "$1" "$(($2 * 86400000 + 43200000))" "$3" "${4:-}"; }

@test "starts tracking with no ledger and lists nothing before a rule is tracked for the whole window" {
  row 11 1000 '{"rule-a":0,"rule-b":3}'
  at 1000
  run -0 --separate-stderr "$R" "" "$L" 8 "$D"
  [ -z "$output" ]
  [ "$stderr" = "rule-fires: merged 1 rows from 1 new artifacts (0 already merged)" ]
  run cat "$L"
  [ "${lines[0]}" = "# rule-fires v2 merged_day=1000" ]
  [ "${lines[1]}" = "rule rule-a 1000 1000 - 1" ]
  [ "${lines[2]}" = "rule rule-b 1000 1000 1000 0" ]
  [ "${lines[3]}" = "rule rule-c 1000 - - 0" ]
  [ "${lines[4]}" = "artifact 11 1000" ]
}

@test "after the window: lists a rule that ran without a finding, names unmeasured ones, skips one that fired" {
  printf '%s\n' '# rule-fires v2 merged_day=1095' 'rule rule-a 900 950 - 40' 'rule rule-b 900 950 905 40' \
    'rule rule-c 900 - - 0' >"$L.in"
  row 11 1100 '{"rule-a":0,"rule-b":2}'
  at 1100
  run -0 "$R" "$L.in" "$L" 8 "$D"
  has '### Rules that never fired in 90 days'
  has '| `rule-a` | never | 41 | today |'
  run ! has '| `rule-b`'
  has 'Not measured: no run in the ci.yml check-all timings of the last 90 days'
  has '`rule-c`.'
  run ! has 'longer than the 8 days'
}

@test "a rule that last fired before the window is listed with its age; one added later stays silent" {
  printf '%s\n' '# rule-fires v2 merged_day=1196' 'rule rule-a 900 1190 1000 80' 'rule rule-b 1150 1190 - 5' \
    'rule rule-c 900 1190 1150 9' >"$L.in"
  at 1200
  run -0 "$R" "$L.in" "$L" 8
  has '| `rule-a` | 200 days ago | 80 | 10 days ago |'
  run ! has 'rule-b'
  run ! has 'rule-c'
  run ! has 'Not measured'
}

@test "merges each artifact once, and one uploaded after a merge the week after" {
  row 11 1000 '{"rule-a":0}'
  at 1000
  run -0 "$R" "" "$L" 8 "$D"
  row 12 999 '{"rule-a":0}' # a run still going at the first merge: older rows, uploaded later
  row 13 1003 '{"rule-a":0}'
  at 1007
  run -0 --separate-stderr "$R" "$L" "$L.2" 8 "$D" # artifact 11 is downloaded again
  [ "$stderr" = "rule-fires: merged 2 rows from 2 new artifacts (1 already merged)" ]
  run cat "$L.2"
  has 'rule rule-a 1000 1003 - 3'
  has 'artifact 11 1000'
  has 'artifact 12 1007'
  has 'artifact 13 1007'
}

@test "forgets a merged artifact once it has certainly expired, after the retention plus a day" {
  printf '%s\n' '# rule-fires v2 merged_day=1000' 'rule rule-a 1000 1000 - 1' 'artifact 11 1000' 'artifact 12 991' \
    >"$L.in"
  at 1009
  run -0 "$R" "$L.in" "$L" 8
  run cat "$L"
  has 'artifact 11 1000'
  run ! has 'artifact 12'
}

@test "a fire resets the runs without a finding, which then count the runs after it" {
  printf '%s\n' '# rule-fires v2 merged_day=1000' 'rule rule-a 900 1000 - 30' >"$L.in"
  row 11 1002 '{"rule-a":0}'
  row 11 1003 '{"rule-a":2}'
  row 12 1004 '{"rule-a":0}'
  row 12 1005 '{"rule-a":0}'
  at 1006
  run -0 "$R" "$L.in" "$L" 8 "$D"
  run grep '^rule rule-a ' "$L"
  [ "$output" = "rule rule-a 900 1005 1003 2" ]
}

@test "keeps a row starting now and drops one a minute later (a forged future row would hide a rule)" {
  at 1000
  rowms 11 "$((RULE_FIRES_NOW * 1000 + 999))" '{"rule-a":1}' # now in ms: later in the same whole second
  rowms 11 "$((RULE_FIRES_NOW * 1000 + 60000))" '{"rule-b":1}'
  run -0 "$R" "" "$L" 8 "$D"
  run cat "$L"
  has 'rule rule-a 1000 1000 1000 0'
  has 'rule rule-b 1000 - - 0'
}

@test "drops rows that are not timing.sh's shape, forged ids or counts, symlinks and odd directories" {
  row 11 1000 '{"rule-a":1}' 'bad sensor'
  row 11 1000 '{"rule-a":"1"}'
  row 11 1000 '{"rule-a":1,"Rule_B":1}'
  row 11 1000 '{"rule-a":1}{"x":1}'
  rowms 11 12345678901234567 '{"rule-a":1}'
  printf '{"sensor":"guard-bash","start_ms":%s,"ms":1,"exit":0,"scope":"hook","sha":"","os":"darwin"}\n' \
    "$((1000 * 86400000))" >>"$D/11/timings.jsonl"
  row 11 1000 '{"rule-b":0,"retired-rule":4}'
  row notanid 1000 '{"rule-c":1}'
  row 12 1000 '{"rule-c":1}'
  mv "$D/12/timings.jsonl" "$BATS_TEST_TMPDIR/elsewhere.jsonl"
  ln -s "$BATS_TEST_TMPDIR/elsewhere.jsonl" "$D/12/timings.jsonl"
  ln -s "$D/notanid" "$D/13"
  at 1000
  run -0 "$R" "" "$L" 8 "$D"
  run cat "$L"
  has 'rule rule-a 1000 - - 0'
  has 'rule rule-b 1000 1000 - 1'
  has 'rule rule-c 1000 - - 0'
  run ! has 'retired-rule'
  run ! has 'artifact 12'
  run ! has 'artifact 13'
}

@test "says in the report when new artifacts gave no row (the writer and the reader disagree)" {
  rowms 11 "$((1000 * 86400000))" '{"rule-a":1}' 'bad sensor'
  at 1000
  run -0 --separate-stderr "$R" "" "$L" 8 "$D"
  [ "$stderr" = "rule-fires: merged 0 rows from 1 new artifacts (0 already merged)" ]
  has 'No row merged from 1 new ci.yml timing artifacts'
}

@test "drops ledger rows of rules no longer in rules.md and tracks a new rule from today" {
  printf '%s\n' '# rule-fires v2 merged_day=999' 'rule rule-a 900 950 - 40' 'rule gone-rule 900 950 - 40' >"$L.in"
  at 1000
  run -0 "$R" "$L.in" "$L" 8
  run cat "$L"
  has 'rule rule-a 900 950 - 40'
  has 'rule rule-c 1000 - - 0'
  run ! has 'gone-rule'
}

@test "says when the previous ledger is older than the timings retention" {
  printf '%s\n' '# rule-fires v2 merged_day=1080' 'rule rule-a 1000 1080 1080 3' >"$L.in"
  at 1100
  run -0 "$R" "$L.in" "$L" 8
  has 'The previous ledger is from 20 days ago, longer than the 8 days ci.yml keeps timings'
  run -0 --separate-stderr "$R" "$L.in" "$L" 30
  [ -z "$output" ]
}

@test "fails closed on a ledger or heading that does not parse, an unreadable input or a usage error" {
  at 1000
  printf '%s\n' 'rule rule-a 1 2 3 4' >"$L.in"
  run -2 "$R" "$L.in" "$L" 8
  has 'not a rule-fires v2 ledger header'
  printf '%s\n' '# rule-fires v2 merged_day=1' 'rule rule-a 1 two - 4' >"$L.in"
  run -2 "$R" "$L.in" "$L" 8
  has ':2: not rule <rule-id>'
  : >"$L.in"
  run -2 "$R" "$L.in" "$L" 8
  has 'empty ledger'
  run -2 "$R" "" "$L" 8 "$BATS_TEST_TMPDIR/missing"
  has 'cannot read'
  run -2 "$R" "" "$L"
  has 'usage:'
  run -2 "$R" "" "$L" eight
  RULE_FIRES_NOW=soon run -2 "$R" "" "$L" 8
  printf '%s\n' '### rule-a' '### Rule A (old name)' >"$SANDBOX/docs/harness/rules.md"
  run -2 "$R" "" "$L" 8
  has 'not a rule id in heading: ### Rule A (old name)'
  [ ! -e "$L" ]
}

@test "reads the rows the real run.sh writes, as nfr-report.sh does (writer and readers agree)" {
  sandbox scripts/harness/run.sh scripts/harness/timing.sh scripts/harness/nfr-report.sh
  printf '%s\n' '### rule-a' '- **Remediation:** Fix A.' '### rule-b' '- **Remediation:** Fix B.' '### rule-c' \
    >"$SANDBOX/docs/harness/rules.md"
  unset HARNESS_BUDGETS GITHUB_ACTIONS GITHUB_STEP_SUMMARY
  run -1 "$SANDBOX/scripts/harness/run.sh" lint rule-a,rule-b -- bash -c 'echo "x: rule-b broke"; exit 1'
  mkdir -p "$D/11"
  cp "$SANDBOX/.cache/harness/timings.jsonl" "$D/11/timings.jsonl"
  HARNESS_BUDGETS=/dev/null run -0 "$SANDBOX/scripts/harness/nfr-report.sh" "$D/11/timings.jsonl"
  # Now is the row's own second (no clock read here, so no midnight flake): the latest a row may start.
  local start_ms
  start_ms="$(sed -n 's/.*"start_ms":\([0-9]*\),.*/\1/p' "$D/11/timings.jsonl")"
  export RULE_FIRES_NOW=$((start_ms / 1000))
  local today=$((RULE_FIRES_NOW / 86400))
  run -0 --separate-stderr "$R" "" "$L" 8 "$D"
  [ "$stderr" = "rule-fires: merged 1 rows from 1 new artifacts (0 already merged)" ]
  run cat "$L"
  has "rule rule-a $today $today - 1"
  has "rule rule-b $today $today $today 0"
}
