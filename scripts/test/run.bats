#!/usr/bin/env bats
# scripts/harness/run.sh: silent on success; on failure the tool output plus each rule's remediation.
bats_require_minimum_version 1.5.0
load helpers

setup() {
  unset HARNESS_BUDGETS GITHUB_ACTIONS GITHUB_STEP_SUMMARY # budgets from the sandbox, no CI annotations
  sandbox scripts/harness/run.sh scripts/harness/timing.sh
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

SIZE=code-function-length,code-complexity,code-parameters,code-file-length
GO_IDS=rule-a,go-sec,go-structured-logging,go-format,$SIZE,repo-no-clones # as lint-go passes them
WEB_IDS=rule-a,web-lint,$SIZE                                             # as lint-web passes them

# remediations <ids>: a "Fix <id>." remediation in the sandbox rules.md for each id that has none yet.
remediations() {
  local id ids
  IFS=',' read -ra ids <<<"$1"
  for id in "${ids[@]}"; do
    grep -qx "### $id" "$SANDBOX/docs/harness/rules.md" ||
      printf '%s\n' '' "### $id" "- **Remediation:** Fix $id." >>"$SANDBOX/docs/harness/rules.md"
  done
}

# recipe_of <ids> <finding> <rule-id>: a failure printing only <finding> shows exactly <rule-id>'s remediation.
recipe_of() {
  remediations "$1"
  printf '%s\n' "$2" >"$SANDBOX/finding"
  local status=0
  "$RUN" lint "$1" -- bash -c 'cat "$0"; exit 1' "$SANDBOX/finding" 2>"$SANDBOX/out" || status=$?
  [ "$status" -eq 1 ]
  if ! grep -qx "remediation: Fix $3." "$SANDBOX/out" || [ "$(grep -c '^remediation:' "$SANDBOX/out")" -ne 1 ]; then
    printf 'expected only the remediation of %s for: %s\nactual output:\n' "$3" "$2" >&2
    cat "$SANDBOX/out" >&2
    return 1
  fi
  # The timing row counts the finding for <rule-id> alone (scripts/harness/rule-fires.sh reads it, ADR 0032).
  local id ids row want
  row="$(tail -n 1 "$SANDBOX/.cache/harness/timings.jsonl")"
  IFS=',' read -ra ids <<<"$1"
  for id in "${ids[@]}"; do
    want=0
    [[ $id != "$3" ]] || want=1
    [[ $row == *"\"$id\":$want"[,\}]* ]] || {
      printf 'expected "%s":%s in the timing row for: %s\nrow: %s\n' "$id" "$want" "$2" "$row" >&2
      return 1
    }
  done
}

@test "maps each golangci finding that cannot carry its rule id to that rule alone (ADR 0024)" {
  # Lines as golangci-lint prints them (copied from a run over the selftest fixtures and probes).
  recipe_of "$GO_IDS" "internal/fx/service/service.go:4:6: Function 'FixtureTooLong' has too many statements (36 > 35) (funlen)" code-function-length
  recipe_of "$GO_IDS" 'internal/fx/service/service.go:5:1: cognitive complexity 14 of func `FixtureTooComplex` is high (> 13) (gocognit)' code-complexity
  recipe_of "$GO_IDS" 'internal/fx/service/service.go:5:1: argument-limit: maximum number of arguments per function exceeded; max 4 but got 5 (revive)' code-parameters
  recipe_of "$GO_IDS" 'internal/fx/service/service.go:206: file-length-limit: file length is 206 lines, which exceeds the limit of 205 (revive)' code-file-length
  recipe_of "$GO_IDS" 'internal/fx/service/e.go:6:46: G304: Potential file inclusion via variable (gosec)' go-sec
  recipe_of "$GO_IDS" 'internal/fx/service/f.go:6:50: key-value pairs and attributes should not be mixed (sloglint)' go-structured-logging
  recipe_of "$GO_IDS" 'internal/fx/service/g.go:5:1: File is not properly formatted (gofmt)' go-format
  recipe_of "$GO_IDS" 'internal/fx/service/service.go:10: 10-23 lines are duplicate of `internal/fx/service/service.go:26-39` (dupl)' repo-no-clones
}

@test "maps each ESLint finding to its rule alone; web-lint takes the unmapped ones (ADR 0024)" {
  # Lines as ESLint's stylish format prints them; max-lines must not match max-lines-per-function, nor the reverse.
  recipe_of "$WEB_IDS" "  2:8  error  Function 'fixtureTooComplex' has a complexity of 21. Maximum allowed is 20  complexity" code-complexity
  recipe_of "$WEB_IDS" "  2:8  error  Function 'fixtureTooManyParams' has too many parameters (4). Maximum allowed is 3  max-params" code-parameters
  recipe_of "$WEB_IDS" "  1:8  error  Function 'fixtureTooLong' has too many lines (51). Maximum allowed is 50  max-lines-per-function" code-function-length
  recipe_of "$WEB_IDS" '  301:1  error  File has too many lines (301). Maximum allowed is 300  max-lines' code-file-length
  recipe_of "$WEB_IDS" '  2:16  error  Unexpected any. Specify a different type   @typescript-eslint/no-explicit-any' web-lint
  # A finding that names another given rule is that rule's, not web-lint's.
  recipe_of "$WEB_IDS,web-x" "  1:1  error  [web-x] fetch is not allowed  no-restricted-globals" web-x
}

@test "prints a split recipe only when its finding is there, even when nothing is named (ADR 0024)" {
  remediations "$GO_IDS,$WEB_IDS"
  run -1 "$RUN" golangci "$GO_IDS" -- bash -c "echo 'x.go:1:1: Error return value is not checked (errcheck)'; exit 1"
  has 'Fix A.'
  run ! grep -qE 'Fix (code|go|repo)-' <<<"$output"
  # web-lint is the catch-all, so it stays in the fallback (e.g. ESLint crashing before any finding).
  run -2 "$RUN" eslint "$WEB_IDS" -- bash -c 'echo "Oops! Something went wrong!"; exit 2'
  has 'Fix A.'
  has 'Fix web-lint.'
  run ! grep -q 'Fix code-' <<<"$output"
  # When every rule given has a pattern (the pre-commit hook's go-format alone), they all print.
  run -1 "$RUN" golangci-fmt go-format -- bash -c 'echo "--- x.go.orig"; exit 1'
  has 'Fix go-format.'
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

@test "records each rule's finding count: its lines, at least 1 when its remediation prints, else 0 (ADR 0032)" {
  T="$SANDBOX/.cache/harness/timings.jsonl"
  run -0 "$RUN" quick rule-a,rule-b -- true
  run tail -n 1 "$T"
  has '","findings":{"rule-a":0,"rule-b":0}}'
  run -1 "$RUN" named rule-a,rule-b -- bash -c 'printf "x:1: [rule-b]\nx:2: [rule-b]\nnoise\n"; exit 1'
  run tail -n 1 "$T"
  has '"findings":{"rule-a":0,"rule-b":2}}'
  # A failing test names no rule: each rule whose remediation prints fired once, so it is never a retirement
  # candidate for want of a rule id in its tool's output.
  run -1 "$RUN" tests rule-a,rule-b -- bash -c 'echo "--- FAIL: TestX"; exit 1'
  run tail -n 1 "$T"
  has '"findings":{"rule-a":1,"rule-b":1}}'
  remediations "$GO_IDS"
  run -1 "$RUN" golangci "$GO_IDS" -- bash -c "echo 'x.go:1:1: Error return value is not checked (errcheck)'; exit 1"
  run tail -n 1 "$T"
  has '"findings":{"rule-a":1,"go-sec":0,"go-structured-logging":0,"go-format":0,"code-function-length":0,'
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

@test "runs the sensor untimed when the timing helper is missing" {
  rm "$SANDBOX/scripts/harness/timing.sh"
  run -3 "$RUN" sensor rule-a -- bash -c 'exit 3'
  has 'remediation: Fix A.'
  run ! has 'timing.sh'
  [ ! -e "$SANDBOX/.cache/harness/timings.jsonl" ]
}
