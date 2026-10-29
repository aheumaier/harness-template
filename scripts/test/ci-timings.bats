#!/usr/bin/env bats
# scripts/harness/ci-timings.sh: ci.yml's timing artifacts for harness-nfr.yml and drift.yml, listed once and
# downloaded by id, with every failure counted.
bats_require_minimum_version 1.5.0
load helpers

setup() {
  sandbox scripts/harness/ci-timings.sh
  C="$SANDBOX/scripts/harness/ci-timings.sh"
  S="$BATS_TEST_TMPDIR/stub"
  D="$BATS_TEST_TMPDIR/timings"
  mkdir -p "$S/bin" "$SANDBOX/.github/workflows"
  # gh stub: the artifact list from $S/ids ($S/list-fails fails it), each zip from $S/<id>.zip; logs its args.
  cat >"$S/bin/gh" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"$STUB/log"
case "$*" in
*"/artifacts?name="*) [[ ! -e "$STUB/list-fails" ]] && cat "$STUB/ids" ;;
*/zip) id="${2%/zip}" && id="${id##*/}" && cat "$STUB/$id.zip" ;;
*) exit 9 ;;
esac
EOF
  chmod +x "$S/bin/gh"
  export STUB="$S" PATH="$S/bin:$PATH"
}

# artifact <id> <row>: a zip holding timings.jsonl at its root, as ci.yml uploads it.
artifact() {
  mkdir -p "$S/src/$1"
  printf '%s\n' "$2" >"$S/src/$1/timings.jsonl"
  (cd "$S/src/$1" && zip -q "$S/$1.zip" timings.jsonl)
  printf '%s\n' "$1" >>"$S/ids"
}

@test "retention: reads retention-days of the timings artifact in the real ci.yml" {
  cp "$REPO/.github/workflows/ci.yml" "$SANDBOX/.github/workflows/ci.yml"
  run -0 "$C" retention
  [[ "$output" =~ ^[0-9]+$ ]]
  grep -A6 'name: harness-timings-check-all' "$REPO/.github/workflows/ci.yml" | grep -qx "  *retention-days: $output"
}

@test "retention: fails closed when the timings upload has no retention-days" {
  printf '%s\n' 'steps:' '  - uses: x' '    with:' '      name: harness-timings-check-all' '  - uses: y' \
    '    with:' '      retention-days: 3' >"$SANDBOX/.github/workflows/ci.yml"
  run -2 "$C" retention
  has 'no retention-days for harness-timings-check-all'
}

@test "download: lists the artifacts once and unpacks each by id" {
  artifact 101 '{"row":1}'
  artifact 102 '{"row":2}'
  run -0 "$C" download "$D" 8
  has 'downloaded 2 of 2 harness-timings-check-all artifacts'
  [ "$(cat "$D/101/timings.jsonl")" = '{"row":1}' ]
  [ "$(cat "$D/102/timings.jsonl")" = '{"row":2}' ]
  [ "$(grep -c 'artifacts?name=harness-timings-check-all' "$S/log")" -eq 1 ]
  grep -q 'select(.expired | not)' "$S/log"
  [ -z "$(find "$D" -name '*.zip')" ]
}

@test "download: a failed list or download fails, counted, so the caller can say so" {
  artifact 101 '{"row":1}'
  printf '%s\n' 103 >>"$S/ids" # listed, but its download fails
  run -1 "$C" download "$D" 8
  has 'downloading artifact 103 failed'
  has 'downloaded 1 of 2'
  touch "$S/list-fails"
  run -1 "$C" download "$D" 8
  has 'listing the harness-timings-check-all artifacts failed'
}

@test "usage errors exit 2" {
  run -2 "$C"
  has 'usage:'
  run -2 "$C" download "$D"
  run -2 "$C" download "$D" week
  run -2 "$C" retention extra
}
