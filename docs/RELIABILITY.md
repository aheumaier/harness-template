# Reliability

## In place

- **Context propagation:** every function below `cmd/` receives the request context (rule
  [go-no-root-context](harness/rules.md#go-no-root-context)); cancellation and deadlines reach Postgres.
- **Graceful shutdown:** SIGINT/SIGTERM stop accepting requests and drain for up to 10 s; `ReadHeaderTimeout`
  is 5 s.
- **Health:** `GET /healthz` checks the database and returns a problem with 503 when it is unreachable.
- **Migrations** are lint-checked for destructive and locking operations and set `lock_timeout` /
  `statement_timeout` ([db-safe-locking](harness/rules.md#db-safe-locking)).
- **Observability:** structured JSON logs (slog) on stdout, OpenTelemetry traces and RED metrics per route,
  correlated by request ID and trace ID; logs, traces and RED metrics are exported over OTLP when configured
  ([ARCHITECTURE.md](../ARCHITECTURE.md#cross-cutting-concerns), [ADR 0021](adr/0021-telemetry-export-over-otlp-http.md)). `/healthz` is instrumented like any route.
  A failed export is logged to stdout and never blocks a request; shutdown waits at most 5 s for the flush.
- **Tests** run with the race detector; integration tests run against a real Postgres per worktree.

## Before production

- Define SLOs per user-facing operation and alert on them (from the `http.server.request.duration` histogram).
- When a liveness or readiness prober calls `/healthz`, keep its calls out of spans and metrics
  (`otelhttp.WithFilter` in `telemetry.Middleware`) and decide whether they still get access lines.
- Readiness vs liveness split, request timeouts per route, connection pool sizing, backups and restore drills.
