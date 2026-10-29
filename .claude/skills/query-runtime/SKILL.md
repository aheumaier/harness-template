---
name: query-runtime
description: Answer questions about the running app (logs of a request, errors since a time, latency or error rate of a route, the trace of a request) through this worktree's make targets. Use whenever you need runtime evidence from the API instead of guessing.
---

# Query the runtime

Reach runtime data **only through `make` targets**. They load this worktree's ports themselves; never read,
print or source the env file (`.env.local`), and never curl the stack's ports by hand.

## 1. Get something running

- `make obs-up` starts this worktree's obs stack (logs, traces, metrics). Then start `make dev` **in the
  background** (it runs until stopped); a `make dev` started before `make obs-up` prints `telemetry off` and
  exports nothing: restart it.
- `make dev` also writes the output of this run to `.tmp/dev/api.log` (the API's JSON log lines) and
  `.tmp/dev/web.log` (Vite), emptied at each start. Without the obs stack these files are all there is.
- To find your own request later, send it with an ID: `curl -H 'X-Request-Id: my-check-1' http://…/healthz`
  (`make dev` prints the address). Every response carries `X-Request-Id`; every log line written while
  serving a request carries its `request_id` and `trace_id` (startup and shutdown lines have neither).

## 2. Map the question to a target

| Question | Command |
|----------|---------|
| Logs of request X | `make logs REQUEST_ID=X` (one JSON line per record; the `request` line has method, route, status, `duration_ms`) |
| The trace of request X | `make traces ID=<trace_id>` with the `trace_id` from its log line (spans in start order, DB calls as children) |
| Errors and warnings since T | `make logs QUERY='{service_name="api"} \| severity_text=~"ERROR\|WARN"' SINCE=30m` |
| Failed requests | `make logs QUERY='{service_name="api"} \| status=~"4..\|5.."' SINCE=1h` |
| Slow requests | `make logs QUERY='{service_name="api"} \| duration_ms > 100' SINCE=1h` |
| p95 latency per route | `make metrics QUERY='histogram_quantile(0.95, sum by (le, http_route) (rate(http_server_request_duration_seconds_bucket[5m])))'` |
| p95 latency of /x | `make metrics QUERY='histogram_quantile(0.95, sum by (le) (rate(http_server_request_duration_seconds_bucket{http_route="/x"}[5m])))'` |
| Requests per route and status | `make metrics QUERY='sum by (http_route, http_response_status_code) (increase(http_server_request_duration_seconds_count[15m]))'` |
| Same, without the obs stack | `.bin/jq -cR 'fromjson? \| select(.request_id == "X")' .tmp/dev/api.log` (or `.status >= 500`, `.level == "ERROR"`, `.duration_ms > 100`) |

In the table, `\|` is a plain `|` (Markdown escaping). `SINCE` defaults to `1h`, `LIMIT` to 100 records.
Routes are templates without the method (`/healthz`); an unmatched path has no `route` and no `http_route`.

## 3. Read the answer right

- `make logs REQUEST_ID=…` and `make traces` fail with "no log records of request …" / "no trace …" when the
  ID has nothing: telemetry arrives a few seconds after the request; retry, then check that `make dev` was
  restarted after `make obs-up`. A `QUERY` that matches nothing prints a note and succeeds.
- `make metrics` prints `{labels} value`. `NaN` from a `rate(…[5m])` means too few samples yet: metrics are
  exported every 5 s, so send more requests or wait. "no series match" means no such metric or label.
- Make expands `$` in a command-line value: write `$$`, or pass the value from the environment
  (`QUERY='…' make logs`).
- `make obs-down` drops the stack and its telemetry; `make down` removes every container and volume of
  this worktree, Postgres data too.

## 4. When done

Stop the background `make dev` you started, and run `make obs-down` if you started the stack with
`make obs-up` (leave one that was already running).
