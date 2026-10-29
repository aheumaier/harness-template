#!/usr/bin/env bats
# scripts/obs-query.sh (make logs, metrics, traces): input checks, and the shaping of recorded Loki, Prometheus
# and Tempo responses. A stub curl on PATH records its arguments and answers with $STUB/body, the HTTP status
# in $STUB/code (default 200) or the curl exit status in $STUB/rc. make test-obs reads the live stack (TD-32).
bats_require_minimum_version 1.5.0
load helpers

setup() {
  sandbox scripts/obs-query.sh .bin/jq
  Q="$SANDBOX/scripts/obs-query.sh"
  STUB="$BATS_TEST_TMPDIR/stub"
  mkdir -p "$STUB"
  # Prints the body and, like curl -w '\n%{http_code}', the status as the last line.
  cat >"$STUB/curl" <<'STUB'
#!/usr/bin/env bash
d="$(dirname "$0")"
printf '%s\n' "$@" >"$d/curl.args"
rc="$(cat "$d/rc" 2>/dev/null || echo 0)"
if [[ "$rc" != 0 ]]; then
  echo "curl: ($rc) stubbed failure" >&2
  printf '\n000'
  exit "$rc"
fi
cat "$d/body" 2>/dev/null || true
printf '\n%s' "$(cat "$d/code" 2>/dev/null || echo 200)"
STUB
  chmod +x "$STUB/curl"
  export PATH="$STUB:$PATH" STUB
  export LOKI_PORT=1 PROMETHEUS_PORT=2 TEMPO_PORT=3 OTEL_METRIC_EXPORT_INTERVAL=5000
  unset REQUEST_ID QUERY SINCE LIMIT ID
}

# arg <text>: curl got exactly this argument.
arg() {
  grep -qxF -- "$1" "$STUB/curl.args" && return 0
  printf 'expected curl argument: %s\nactual:\n%s\n' "$1" "$(cat "$STUB/curl.args")" >&2
  return 1
}

loki_empty='{"status":"success","data":{"resultType":"streams","result":[]}}'

@test "usage error without a known command" {
  run -1 "$Q"
  has "usage: obs-query.sh logs|metrics|traces"
}

@test "logs: SINCE must be a number and one of s m h d w" {
  for bad in 1x h 1.5h 30 -1h; do
    SINCE=$bad run -1 "$Q" logs
    has "SINCE='$bad': want a duration"
  done
  echo "$loki_empty" >"$STUB/body"
  SINCE=30m run -0 "$Q" logs
  arg since=30m
}

@test "logs: LIMIT is 1-5000, leading zeros read as decimal" {
  for bad in 0 0000 5001 9999 00001 abc -1; do
    LIMIT=$bad run -1 "$Q" logs
    has "LIMIT='$bad': want 1-5000"
  done
  echo "$loki_empty" >"$STUB/body"
  LIMIT=0100 run -0 "$Q" logs
  arg limit=100
  LIMIT=0010 run -0 "$Q" logs # not octal 8
  arg limit=10
  LIMIT=5000 run -0 "$Q" logs
  arg limit=5000
}

@test "logs: REQUEST_ID takes the contract's charset only, and goes into the LogQL filter" {
  long="$(printf 'a%.0s' $(seq 1 129))"
  for bad in 'a"b' 'a b' 'a|b' "a\`b" '{x}' "$long"; do
    REQUEST_ID=$bad run -1 "$Q" logs
    has "want 1-128 of A-Za-z0-9._-"
  done
  echo "$loki_empty" >"$STUB/body"
  REQUEST_ID=probe-1.x_Y run -3 "$Q" logs
  arg 'query={service_name="api"} | request_id="probe-1.x_Y"'
}

@test "logs: REQUEST_ID and QUERY together are refused" {
  REQUEST_ID=x QUERY='{a="b"}' run -1 "$Q" logs
  has "make logs takes REQUEST_ID or QUERY, not both"
}

@test "a missing port says to run make setup" {
  unset LOKI_PORT
  run -1 "$Q" logs
  has "no LOKI_PORT for this worktree: run make setup"
  TEMPO_PORT=abc ID=bf0b04498d8d0bef59cff061eb57efe4 run -1 "$Q" traces
  has "no TEMPO_PORT for this worktree: run make setup"
}

@test "logs: a Loki streams body becomes JSON lines, oldest first, without the SDK's labels" {
  cat >"$STUB/body" <<'JSON'
{"status":"success","data":{"resultType":"streams","result":[
 {"stream":{"service_name":"api","severity_text":"INFO","severity_number":"9","telemetry_sdk_name":"x",
   "scope_name":"api","request_id":"r1","route":"/healthz"},
  "values":[["1761692983222333444","second"],["1761692983111222333","first"]]},
 {"stream":{"service_name":"api","severity_text":"ERROR","request_id":"r1"},
  "values":[["1761692983150000000","middle"]]}]}}
JSON
  REQUEST_ID=r1 run -0 "$Q" logs
  [[ "$output" == '{"time":"2025-10-28T23:09:43.111222Z","level":"INFO","msg":"first","request_id":"r1","route":"/healthz"}
{"time":"2025-10-28T23:09:43.150000Z","level":"ERROR","msg":"middle","request_id":"r1"}
{"time":"2025-10-28T23:09:43.222333Z","level":"INFO","msg":"second","request_id":"r1","route":"/healthz"}' ]] || {
    echo "$output" >&2
    return 1
  }
  arg http://127.0.0.1:1/loki/api/v1/query_range
  arg direction=backward
}

@test "logs: no records exits 3 for a REQUEST_ID (restart hint), 0 for a QUERY" {
  echo "$loki_empty" >"$STUB/body"
  REQUEST_ID=none run -3 "$Q" logs
  has "no log records of request none in the last 1h"
  has "restart make dev"
  QUERY='{service_name="api"} |= "x"' run -0 "$Q" logs
  has 'no log records match {service_name="api"} |= "x" in the last 1h'
}

@test "logs: a metric query is refused with a pointer to make metrics" {
  echo '{"status":"success","data":{"resultType":"matrix","result":[]}}' >"$STUB/body"
  QUERY='rate({service_name="api"}[1m])' run -1 "$Q" logs
  has "use make metrics for a metric query"
}

@test "metrics: needs QUERY; prints one series per line; no series exits 0 with the export interval" {
  run -1 "$Q" metrics
  has "make metrics needs QUERY='<PromQL>', e.g. the p95 per route"
  echo '{"status":"success","data":{"resultType":"vector","result":[{"metric":{"http_route":"/healthz"},"value":[1,"0.00475"]}]}}' >"$STUB/body"
  QUERY=up run -0 "$Q" metrics
  [[ "$output" == '{"http_route":"/healthz"} 0.00475' ]] || {
    echo "$output" >&2
    return 1
  }
  arg http://127.0.0.1:2/api/v1/query
  arg query=up
  echo '{"status":"success","data":{"resultType":"vector","result":[]}}' >"$STUB/body"
  QUERY=nope run -0 "$Q" metrics
  has "no series match nope (the API exports metrics every 5000 ms)"
}

@test "a 400 prints the server's error and exits 1" {
  echo '{"status":"error","errorType":"bad_data","error":"parse error"}' >"$STUB/body"
  echo 400 >"$STUB/code"
  QUERY='sum((' run -1 "$Q" metrics
  has "HTTP 400 from /api/v1/query:"
  has '"error":"parse error"'
}

@test "curl failures: exit 7 says run make obs-up, others say could not reach" {
  echo 7 >"$STUB/rc"
  run -1 "$Q" logs
  has "no obs stack on 127.0.0.1:1 — run make obs-up"
  echo 28 >"$STUB/rc"
  QUERY=up run -1 "$Q" metrics
  has "could not reach 127.0.0.1:2/api/v1/query (curl exit 28): curl: (28) stubbed failure"
}

@test "traces: ID is lowercased and must be 32 hex digits" {
  for bad in '' bf0b 'bf0b04498d8d0bef59cff061eb57efeg' 'bf0b04498d8d0bef59cff061eb57efe4a'; do
    ID=$bad run -1 "$Q" traces
    has "make traces needs ID=<trace_id>, 32 hex digits"
  done
  echo '{"trace":{}}' >"$STUB/body"
  ID=BF0B04498D8D0BEF59CFF061EB57EFE4 run -3 "$Q" traces
  arg http://127.0.0.1:3/api/v2/traces/bf0b04498d8d0bef59cff061eb57efe4
}

tempo_body() { # tempo_body <base64 traceId>
  cat <<JSON
{"trace":{"resourceSpans":[{"scopeSpans":[{"spans":[
 {"traceId":"$1","spanId":"4rU9pJjFpwc=","parentSpanId":"5oKhpL4aE8A=","name":"pool.acquire",
  "kind":"SPAN_KIND_CLIENT","startTimeUnixNano":"1761692983111058000","endTimeUnixNano":"1761692983111062608",
  "attributes":[{"key":"db.system.name","value":{"stringValue":"postgresql"}}],"status":{}},
 {"traceId":"$1","spanId":"5oKhpL4aE8A=","name":"GET /healthz","kind":"SPAN_KIND_SERVER",
  "startTimeUnixNano":"1761692983111036000","endTimeUnixNano":"1761692983111441760",
  "attributes":[{"key":"http.response.status_code","value":{"intValue":"200"}}],"status":{"code":"STATUS_CODE_ERROR"}}
]}]}]}}
JSON
}

@test "traces: a Tempo body becomes spans in start order with hex IDs" {
  tempo_body 'vwsESY2NC+9Zz/Bh61fv5A==' >"$STUB/body"
  ID=bf0b04498d8d0bef59cff061eb57efe4 run -0 "$Q" traces
  [[ "$output" == '{"start":"2025-10-28T23:09:43.111036Z","duration_ms":0.40576,"name":"GET /healthz","kind":"SERVER","span_id":"e682a1a4be1a13c0","parent_span_id":"","status":"ERROR","attributes":{"http.response.status_code":"200"}}
{"start":"2025-10-28T23:09:43.111058Z","duration_ms":0.004608,"name":"pool.acquire","kind":"CLIENT","span_id":"e2b53da498c5a707","parent_span_id":"e682a1a4be1a13c0","status":"UNSET","attributes":{"db.system.name":"postgresql"}}' ]] || {
    echo "$output" >&2
    return 1
  }
}

@test "traces: an ID that does not decode to the one asked for fails" {
  tempo_body 'AAAAAAAAAAAAAAAAAAAAAA==' >"$STUB/body"
  ID=bf0b04498d8d0bef59cff061eb57efe4 run -1 "$Q" traces
  has "cannot decode Tempo IDs (OBS_IMAGE bump?)"
}

@test "traces: a 404 or an empty trace exits 3 with the spans-arrive-later hint" {
  echo 'trace not found' >"$STUB/body"
  echo 404 >"$STUB/code"
  ID=bf0b04498d8d0bef59cff061eb57efe4 run -3 "$Q" traces
  has "no trace bf0b04498d8d0bef59cff061eb57efe4: spans arrive a few seconds after the request"
  has "restart make dev"
  rm "$STUB/code"
  echo '{"trace":{},"metrics":{}}' >"$STUB/body"
  ID=bf0b04498d8d0bef59cff061eb57efe4 run -3 "$Q" traces
  has "spans arrive a few seconds after the request"
}
