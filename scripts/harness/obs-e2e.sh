#!/usr/bin/env bash
# NFR-O1 end to end (rules.md#obs-request-queryable; `make test-obs`, in make check-all). Boots the API on a
# free port with its exporters on this worktree's obs stack, sends one request with its own X-Request-Id, stops
# the API (which flushes its telemetry) and reads the request back only through make, as an agent would:
#   make logs REQUEST_ID=<id>   its access line (msg "request", route /healthz, status 200)
#   make traces ID=<trace_id>   its server span, by the trace_id of that line
#   make metrics QUERY=<p95>    the p95 latency of /healthz from this API process alone (its
#                               service.instance.id is the request ID, Prometheus label `instance`)
# Attaches to an obs stack that is already up and leaves it; one it started (make obs-up), it removes.
#   obs-e2e.sh pull   only makes sure OBS_IMAGE (2.58 GB) is present, so that make test-obs pulls it outside
#                     the timed test (budgets.txt rows obs-pull and obs-e2e).
# Needs Docker, perl (for a free port) and this worktree's env (make test-obs loads it); MAKE is the calling
# make (macOS ships 3.81). The API exports with the worktree's own OTEL_* settings (scripts/setup-worktree.sh),
# so a broken default fails here too. Fails closed, never a skip: exit 1 the request is not queryable, 2 it
# cannot run (no Docker, image, env or free port), each with the fix.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
# shellcheck source=layout.sh
source "$ROOT/scripts/harness/layout.sh" || exit 2
JQ="$ROOT/.bin/jq"
WAIT=60 # seconds each signal may take to arrive

die() { # cannot run
  echo "obs-e2e: $*" >&2
  exit 2
}

command -v docker >/dev/null || die "no docker on PATH: install Docker (NFR-O1 reads the obs stack), then rerun"
docker info >/dev/null 2>&1 || die "Docker is not running: start it, then rerun"

if [[ "${1:-}" == pull ]]; then
  [[ -n "${OBS_IMAGE:-}" ]] || die "no OBS_IMAGE: run this through make test-obs (it reads scripts/tool-versions.env)"
  docker image inspect "$OBS_IMAGE" >/dev/null 2>&1 && exit 0
  docker pull -q "$OBS_IMAGE" >/dev/null ||
    die "could not pull OBS_IMAGE ($OBS_IMAGE): check its pin in scripts/tool-versions.env, the network and Docker's disk space, then rerun"
  exit 0
fi

[[ -x "$JQ" ]] || die "no .bin/jq: run make setup"
for v in COMPOSE_PROJECT_NAME OTLP_PORT LOKI_PORT TEMPO_PORT PROMETHEUS_PORT; do
  [[ -n "${!v:-}" ]] || die "no $v: run make setup, then run this through make test-obs"
done

# m <target> [NAME=value...]: make <target> with only the query inputs given; a caller's QUERY, SINCE, ...
# (environment, or make's command line, which reaches this make as MAKEFLAGS) must not change what is asked.
m() {
  local target="$1"
  shift
  env -u REQUEST_ID -u QUERY -u SINCE -u LIMIT -u ID -u MAKEFLAGS -u MFLAGS "$@" \
    "${MAKE:-make}" -C "$ROOT" --no-print-directory "$target"
}

tmp="$(mktemp -d)"
api_pid=
started=
cleanup() {
  if [[ -n "$api_pid" ]]; then kill "$api_pid" 2>/dev/null || true; fi
  if [[ -n "$started" ]] && ! m obs-down >/dev/null 2>&1; then
    echo "obs-e2e: could not remove the obs stack it started: run make obs-down" >&2
  fi
  rm -rf "$tmp"
}
trap cleanup EXIT

# fail <message>: die with the last output of the failed step (err; got: what make logs, traces or metrics
# printed) and of the API.
fail() {
  local f
  for f in "$tmp/err" "$tmp/got" "$tmp/api.log"; do
    [[ -s "$f" ]] && printf -- '--- %s (last 20 lines)\n%s\n' "${f##*/}" "$(tail -n 20 "$f")" >&2
  done
  echo "obs-e2e: $1" >&2
  exit 1
}

# poll <name> <function>: reruns the function (stdout in $tmp/out, stderr in $tmp/err) until it succeeds,
# for at most WAIT seconds.
poll() {
  local deadline=$((SECONDS + WAIT))
  until "$2" >"$tmp/out" 2>"$tmp/err"; do
    ((SECONDS < deadline)) || fail "no $1 within ${WAIT}s"
    sleep 1
  done
}

m db-up >"$tmp/err" 2>&1 || fail "make db-up failed: this worktree's Postgres did not start"

# The stack: attach, or start it and remember to remove it. A failed probe must not pass for "down", or the
# cleanup would remove a stack that someone else started.
ids="$(cd "$ROOT" && docker compose --profile obs ps --status running -q obs 2>"$tmp/err")" ||
  fail "docker compose ps failed: cannot tell whether this worktree's obs stack is up"
[[ -n "$ids" ]] || started=1
m obs-up >"$tmp/err" 2>&1 ||
  fail "make obs-up failed: Docker could not start the obs stack (OBS_IMAGE ${OBS_IMAGE:-unset}); check its network and disk space"

# The API, built (killing `go run` can orphan its child) and listening on a port nobody else uses.
(cd "$ROOT/$API_DIR" && go build -o "$tmp/api" ./cmd/api) >"$tmp/err" 2>&1 || fail "the API does not build"
port="$(perl -MIO::Socket::INET -e 'print IO::Socket::INET->new(Listen => 1, LocalAddr => "127.0.0.1:0")->sockport')" ||
  die "could not find a free port (perl IO::Socket::INET)"
id="nfr-o1-$(date +%s)-$$"
# service.instance.id (merged into the resource, exec plan 0003 A16) ties the metric to this process.
HTTP_ADDR="127.0.0.1:$port" OTEL_RESOURCE_ATTRIBUTES="service.instance.id=$id" "$tmp/api" >"$tmp/api.log" 2>&1 &
api_pid=$!
listening() { kill -0 "$api_pid" 2>/dev/null && (: <"/dev/tcp/127.0.0.1/$port") 2>/dev/null; }
deadline=$((SECONDS + 30))
until listening; do
  kill -0 "$api_pid" 2>/dev/null || fail "the API exited at start"
  ((SECONDS < deadline)) || fail "the API did not listen on 127.0.0.1:$port within 30s"
  sleep 0.2
done

code="$(curl -sS --noproxy '*' --max-time 10 -o /dev/null -D "$tmp/headers" -w '%{http_code}' \
  -H "X-Request-Id: $id" "http://127.0.0.1:$port/healthz")" || fail "GET /healthz failed"
[[ "$code" == 200 ]] || fail "GET /healthz answered $code, want 200"
grep -qiF "x-request-id: $id" "$tmp/headers" || fail "GET /healthz did not return X-Request-Id: $id"

kill -TERM "$api_pid" 2>/dev/null || fail "the API exited before SIGTERM"
rc=0
wait "$api_pid" || rc=$?
api_pid=
((rc == 0)) || fail "the API exited with $rc on SIGTERM"

access_line() {
  m logs REQUEST_ID="$id" SINCE=15m | tee "$tmp/got" | "$JQ" -ce --arg id "$id" \
    'select(.msg == "request" and .request_id == $id and .route == "/healthz" and (.status | tostring) == "200")'
}
poll "access line of request $id (make logs REQUEST_ID=$id)" access_line
trace="$("$JQ" -r '.trace_id // empty' "$tmp/out" | head -n 1)"
[[ "$trace" =~ ^[0-9a-f]{32}$ ]] || fail "the access line of request $id has no trace_id"

server_span() { m traces ID="$trace" | tee "$tmp/got" | "$JQ" -ce 'select(.kind == "SERVER" and .name == "GET /healthz")'; }
poll "server span of trace $trace (make traces ID=$trace)" server_span

# One line `{labels} <seconds>`; NaN or no series (empty output) is not a latency.
p95q="histogram_quantile(0.95, sum by (le) (http_server_request_duration_seconds_bucket{http_route=\"/healthz\",instance=\"$id\"}))"
p95() { m metrics QUERY="$p95q" | tee "$tmp/got" | grep -E ' [0-9]+(\.[0-9]+)?(e[-+]?[0-9]+)?$'; }
poll "p95 latency of /healthz of this API process (make metrics QUERY='$p95q')" p95
