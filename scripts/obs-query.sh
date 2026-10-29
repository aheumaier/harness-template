#!/usr/bin/env bash
# Reads this worktree's observability stack (compose profile obs, ADR 0017) for make logs, metrics and traces.
# Inputs come from the environment: the *_PORT values `make` loads from .env.local, and the target's own
# REQUEST_ID, QUERY, SINCE, LIMIT or ID, each validated here before it reaches a query (golden principle 2).
#   obs-query.sh logs     REQUEST_ID=<id> | QUERY=<LogQL> [SINCE=1h] [LIMIT=100]: JSON lines, oldest first
#   obs-query.sh metrics  QUERY=<PromQL>: one series per line, `{labels} value`
#   obs-query.sh traces   ID=<32 hex trace id>: one JSON line per span, in start order
# Exit codes, for direct calls: 0 found (for a QUERY also: nothing matched), 1 error, 3 the REQUEST_ID or trace
# ID has no records. Through make every failure is make's exit 2: the message on stderr tells them apart.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
JQ="$ROOT/.bin/jq"

die() {
  echo "$*" >&2
  exit 1
}
[[ -x "$JQ" ]] || die "no .bin/jq: run make setup"
# jq: a Unix-nanosecond string as UTC RFC 3339 with microseconds.
TS='def ts: .[10:16] as $us | .[0:10] | tonumber | todate | sub("Z$"; ".\($us)Z");'

port() { # NAME_PORT: the value, or fail with the fix
  local v="${!1:-}"
  [[ "$v" =~ ^[0-9]+$ ]] || die "no $1 for this worktree: run make setup"
  echo "$v"
}

# not_found <message>: the looked-up ID has no records; exit 3.
not_found() {
  echo "$1 (an API started before make obs-up exports nothing: if that is make dev, restart make dev)" >&2
  exit 3
}

# get <port> <path> [curl args...]: the body on stdout. Exits 1 when the stack is down or answers non-2xx
# (printing the server's error), 4 on a 404 (internal: callers turn it into 3 or 1).
get() {
  local p=$1 path=$2 out rc=0 code
  shift 2
  out="$(curl -sS -G --max-time 30 --noproxy '*' -w '\n%{http_code}' "$@" "http://127.0.0.1:$p$path" 2>&1)" || rc=$?
  code="${out##*$'\n'}" # -w appends the status as the last line, also after a failed transfer (000)
  out="${out%$'\n'*}"
  ((rc != 7)) || die "no obs stack on 127.0.0.1:$p — run make obs-up (then restart make dev)"
  ((rc == 0)) || die "could not reach 127.0.0.1:$p$path (curl exit $rc): ${out%$'\n'}"
  if [[ "$code" != 2* ]]; then
    printf 'HTTP %s from %s:\n%s\n' "$code" "$path" "$out" >&2
    [[ "$code" != 404 ]] || exit 4
    exit 1
  fi
  printf '%s\n' "$out"
}

logs() {
  local request_id="${REQUEST_ID:-}" query="${QUERY:-}" since="${SINCE:-1h}" limit="${LIMIT:-100}" p body out
  [[ -z "$request_id" || -z "$query" ]] || die "make logs takes REQUEST_ID or QUERY, not both"
  [[ "$since" =~ ^[0-9]+[smhdw]$ ]] || die "SINCE='$since': want a duration such as 30m, 1h or 2d"
  if ! [[ "$limit" =~ ^[0-9]{1,4}$ ]] || ((10#$limit < 1 || 10#$limit > 5000)); then
    die "LIMIT='$limit': want 1-5000"
  fi
  limit=$((10#$limit))
  local selector='{service_name="api"}'
  if [[ -n "$request_id" ]]; then
    # The contract's X-Request-Id pattern (platform/requestid): nothing else can reach the LogQL string.
    [[ "$request_id" =~ ^[A-Za-z0-9._-]{1,128}$ ]] || die "REQUEST_ID='$request_id': want 1-128 of A-Za-z0-9._-"
    query="$selector | request_id=\"$request_id\""
  fi
  [[ -n "$query" ]] || query="$selector"
  p="$(port LOKI_PORT)"
  body="$(get "$p" /loki/api/v1/query_range --data-urlencode "query=$query" --data-urlencode "since=$since" \
    --data-urlencode "limit=$limit" --data-urlencode direction=backward)" || exit 1
  # Loki keeps the slog message as the line and every attribute as structured metadata; the newest LIMIT
  # records, printed oldest first, without the SDK's bookkeeping labels.
  out="$(printf '%s' "$body" | "$JQ" -c "$TS"'
    if .data.resultType != "streams" then
      ("a \(.data.resultType) result: make logs takes a log query; use make metrics for a metric query\n"
        | halt_error(1))
    else . end |
    [.data.result[] | .stream as $s | .values[] | {ns: .[0], msg: .[1], s: $s}] | sort_by(.ns) | .[] |
    {time: (.ns | ts), level: .s.severity_text, msg}
      + (.s | del(.severity_text, .severity_number, .detected_level, .flags, .observed_timestamp,
                  .scope_name, .service_name, .telemetry_sdk_language, .telemetry_sdk_name,
                  .telemetry_sdk_version))')"
  if [[ -z "$out" ]]; then
    [[ -z "$request_id" ]] || not_found "no log records of request $request_id in the last $since"
    echo "no log records match $query in the last $since" >&2
    return 0
  fi
  printf '%s\n' "$out"
}

metrics() {
  local query="${QUERY:-}" p body
  [[ -n "$query" ]] || die "make metrics needs QUERY='<PromQL>', e.g. the p95 per route:
  make metrics QUERY='histogram_quantile(0.95, sum by (le, http_route) (rate(http_server_request_duration_seconds_bucket[5m])))'"
  p="$(port PROMETHEUS_PORT)"
  body="$(get "$p" /api/v1/query --data-urlencode "query=$query")" || exit 1
  printf '%s' "$body" | "$JQ" -r --arg q "$query" --arg every "${OTEL_METRIC_EXPORT_INTERVAL:-60000}" '
    .data | if .resultType == "vector" or .resultType == "matrix" then
      if (.result | length) == 0 then
        ("no series match \($q) (the API exports metrics every \($every) ms)\n" | halt_error(0))
      else .result[] | "\(.metric | tojson) \(.value[1] // (.values | map(.[1]) | join(" ")))" end
    else .result[1] end'
}

traces() {
  local id p body out rc=0
  id="$(printf '%s' "${ID:-}" | tr '[:upper:]' '[:lower:]')"
  [[ "$id" =~ ^[0-9a-f]{32}$ ]] || die "make traces needs ID=<trace_id>, 32 hex digits (trace_id in make logs)"
  p="$(port TEMPO_PORT)"
  body="$(get "$p" "/api/v2/traces/$id")" || rc=$?
  ((rc != 4)) || not_found "no trace $id: spans arrive a few seconds after the request"
  ((rc == 0)) || exit "$rc"
  # Tempo returns OTLP JSON with base64 IDs, printed here as hex like the logs carry them. Not @base64d: it
  # decodes to a UTF-8 string, which mangles bytes that are not valid UTF-8.
  out="$(printf '%s' "$body" | "$JQ" -c --arg id "$id" "$TS"'
    def hex: "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/" as $a
      | [explode[] | select(. != 61) | . as $c | $a | index([$c] | implode)]
      | [.[] | [(. / 32 | floor) % 2, (. / 16 | floor) % 2, (. / 8 | floor) % 2, (. / 4 | floor) % 2,
                (. / 2 | floor) % 2, . % 2][]]
      | .[0:(length / 8 | floor) * 8] | [range(0; length; 4) as $i | .[$i:$i + 4] | .[0] * 8 + .[1] * 4 + .[2] * 2 + .[3]]
      | map("0123456789abcdef"[.:. + 1]) | join("");
    def val: to_entries[0].value | if type == "object" then tojson else . end;
    # Unix nanoseconds exceed a double'"'"'s 53 bits: compare as strings, subtract seconds and nanos apart.
    def ns_since($from): ((.[0:10] | tonumber) - ($from[0:10] | tonumber)) * 1e9
      + ((.[10:] | tonumber) - ($from[10:] | tonumber));
    [.trace.resourceSpans[]?.scopeSpans[].spans[]] | sort_by(.startTimeUnixNano | [length, .]) |
    if length > 0 and (.[0].traceId | hex) != $id then
      ("cannot decode Tempo IDs (OBS_IMAGE bump?)\n" | halt_error(1))
    else .[] end |
    {start: (.startTimeUnixNano | ts),
     duration_ms: (.startTimeUnixNano as $s | .endTimeUnixNano | ns_since($s) / 1e6),
     name, kind: (.kind | sub("^SPAN_KIND_"; "")), span_id: (.spanId | hex),
     parent_span_id: (.parentSpanId // "" | hex), status: (.status.code // "STATUS_CODE_UNSET" | sub("^STATUS_CODE_"; "")),
     attributes: (.attributes // [] | map({key, value: (.value | val)}) | from_entries)}')"
  [[ -n "$out" ]] || not_found "no trace $id: spans arrive a few seconds after the request"
  printf '%s\n' "$out"
}

case "${1:-}" in
  logs | metrics | traces) "$1" ;;
  *) die "usage: obs-query.sh logs|metrics|traces (run it through make logs, make metrics, make traces)" ;;
esac
