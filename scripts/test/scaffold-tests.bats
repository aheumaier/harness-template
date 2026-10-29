#!/usr/bin/env bats
# scripts/harness/scaffold-tests.sh: every scaffolded test of a new domain ran and passed (NFR-B1, CI generator probe).
bats_require_minimum_version 1.5.0
load helpers

setup() {
  sandbox scripts/harness/scaffold-tests.sh scripts/tool-versions.env \
    scripts/templates/new-domain/service_test.go.tmpl scripts/templates/new-domain/api_test.go.tmpl \
    scripts/templates/new-domain/api_integration_test.go.tmpl
  S="$SANDBOX/scripts/harness/scaffold-tests.sh"
  T="$BATS_TEST_TMPDIR"
  mkdir -p "$SANDBOX/apps/api/internal/zzz_probe" "$T/bin"
  # A go stub: prints $T/go.json as go test -json would, exits with $GO_EXIT, logs its arguments.
  cat >"$T/bin/go" <<'EOF'
#!/usr/bin/env bash
echo "$PWD $*" >"$GO_ARGS"
echo "go: downloading example.com/x v1.0.0" >&2
cat "$GO_JSON"
exit "${GO_EXIT:-0}"
EOF
  chmod +x "$T/bin/go"
  export PATH="$T/bin:$PATH" GO_JSON="$T/go.json" GO_ARGS="$T/go.args"
}

# events <action:test...>: go test -json lines, one per argument, plus a package without test files.
events() {
  local e
  : >"$GO_JSON"
  printf '{"Action":"start","Package":"m/internal/zzz_probe/model"}\n' >>"$GO_JSON"
  printf '{"Action":"skip","Package":"m/internal/zzz_probe/model","Elapsed":0}\n' >>"$GO_JSON"
  for e in "$@"; do
    printf '{"Action":"output","Package":"m/internal/zzz_probe/api","Test":"%s","Output":"=== RUN   %s\\n"}\n' "${e#*:}" "${e#*:}" >>"$GO_JSON"
    printf '{"Action":"%s","Package":"m/internal/zzz_probe/api","Test":"%s","Elapsed":0}\n' "${e%%:*}" "${e#*:}" >>"$GO_JSON"
  done
}

ALL=(pass:TestReady pass:TestGetZzzProbeStatus_OK pass:TestGetZzzProbeStatus_StorageDown pass:TestGetZzzProbeStatus_Postgres)

@test "silent when every scaffolded test passed; runs the domain's tests with -json and the integration tag in apps/api" {
  events "${ALL[@]}" pass:TestGetZzzProbeStatus_OK/sub
  run -0 "$S" zzz_probe
  [ -z "$output" ]
  [ "$(cat "$GO_ARGS")" = "$SANDBOX/apps/api test -json -race -count=1 -tags=integration ./internal/zzz_probe/..." ]
}

@test "fails on a skipped or failed test, subtests included" {
  events "${ALL[@]}" skip:TestGetZzzProbeStatus_OK/db
  run -1 "$S" zzz_probe
  has 'skipped or failed'
  has 'skip: TestGetZzzProbeStatus_OK/db'
  has 'rule go-tests-pass'
  events pass:TestReady pass:TestGetZzzProbeStatus_OK fail:TestGetZzzProbeStatus_StorageDown pass:TestGetZzzProbeStatus_Postgres
  run -1 "$S" zzz_probe
  has 'fail: TestGetZzzProbeStatus_StorageDown'
}

@test "fails when a test function of the templates did not run" {
  events pass:TestReady pass:TestGetZzzProbeStatus_OK pass:TestGetZzzProbeStatus_StorageDown
  run -1 "$S" zzz_probe
  has 'did not run: TestGetZzzProbeStatus_Postgres'
  events
  run -1 "$S" zzz_probe
  has 'did not run: TestGetZzzProbeStatus_OK TestGetZzzProbeStatus_Postgres TestGetZzzProbeStatus_StorageDown TestReady'
}

@test "a failing go test prints its output without the JSON envelope" {
  events "${ALL[@]}"
  printf '# m/internal/zzz_probe/api\napi.go:1: syntax error\n' >>"$GO_JSON"
  GO_EXIT=1 run -1 "$S" zzz_probe
  has 'go test of apps/api/internal/zzz_probe failed'
  has '=== RUN   TestReady'
  has 'api.go:1: syntax error'
  has 'go: downloading'
  run ! has '"Action"'
}

@test "fails closed on bad usage, a missing domain and unparseable output" {
  run -2 "$S"
  has 'usage: scaffold-tests.sh <name>'
  run -2 "$S" 'Bad-Name'
  run -2 "$S" billing
  has 'run make new-domain NAME=billing first'
  printf 'not json\n' >"$GO_JSON"
  run -2 "$S" zzz_probe
  has 'unparseable go test -json output'
}
