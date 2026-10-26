# ADR 0002: Go service layout

Status: accepted · 2026-10-27

## Context
The service must be legible to agents with minimal framework magic, and support many domains that can be
generated mechanically.

## Decision
- Go module at `apps/api`, path `github.com/aheumaier/harness-template/apps/api` (repo-path-correct for a
  monorepo subdirectory). The analyzers are a nested module so linter deps never enter the app's go.mod.
- Standard library only for transport (`net/http` 1.22+ patterns) and logging (`log/slog` JSON).
- `pgx/v5` pool owned by `internal/platform/db`; exposed as the alias `db.Pool` so `cmd/` wires stores
  without importing the driver.
- `internal/platform` holds every provider (config, logging, db, auth stub, telemetry, health). It imports
  no domain. Domains live in `internal/<domain>/{model,store,service,api}` and are created only by
  `make new-domain`.
- `cmd/api` is the only place a root context is created; route registration lives in `cmd/api/routes.go`
  with a generator marker.

## Consequences
- No DI framework: wiring is explicit Go in `cmd/api`, readable and greppable.
- Config fails fast on missing env vars; there are no silent defaults for required values.
