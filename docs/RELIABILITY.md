# Reliability

## In place

- **Context propagation:** every function below `cmd/` receives the request context (rule
  [go-no-root-context](harness/rules.md#go-no-root-context)); cancellation and deadlines reach Postgres.
- **Graceful shutdown:** SIGINT/SIGTERM stop accepting requests and drain for up to 10 s; `ReadHeaderTimeout`
  is 5 s.
- **Health:** `GET /healthz` checks the database and returns a problem with 503 when it is unreachable.
- **Migrations** are lint-checked for destructive and locking operations and set `lock_timeout` /
  `statement_timeout` ([db-safe-locking](harness/rules.md#db-safe-locking)).
- **Observability:** structured JSON logs (slog) and OpenTelemetry traces (`OTEL_TRACES_EXPORTER`).
- **Tests** run with the race detector; integration tests run against a real Postgres per worktree.

## Before production

- Define SLOs per user-facing operation and alert on them; add metrics export.
- Readiness vs liveness split, request timeouts per route, connection pool sizing, backups and restore drills.
