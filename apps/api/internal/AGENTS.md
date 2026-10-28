# apps/api/internal

- `platform/` is plumbing (config, logging, requestid, accesslog, db, auth, telemetry, problem, health). It imports no domain.
  Add to it only what every domain needs; domain logic never goes here.
- Every other directory is a domain created by `make new-domain NAME=<name>` with `model/ store/ service/ api/`.
  Read the domain's own `AGENTS.md` before changing it.
- Dependencies: `api → service → store → model`; `api` and `service` may use `platform`. Another domain may use
  your `service` or `model` only ([rules](../../../docs/harness/rules.md#go-layers)).
- `api/api.gen.go` and `store/*.go` are generated (`make gen`); edit `api/openapi.yaml` or `db/queries/` instead.
- `api.go` maps requests to service calls and service errors to the problem responses declared in the spec.
  Services return plain Go values and typed errors, never HTTP status codes.
- Pass `ctx` everywhere; never `context.Background()` outside `cmd/` and tests.
- Log with the `*Context` methods so lines carry `request_id` and `trace_id`. Don't `WithGroup` a request
  logger (the IDs would land inside the group); add attrs instead.
- A `service` declares the narrow interface over `*store.Queries` it uses, asserted at compile time; tests
  pass a fake. A test may wire `store` itself (like `cmd/api/routes.go`); `api/` proper never imports `store`.
- Unit tests live next to the code (`*_test.go`, `-short`); tests that need Postgres use the
  `integration` build tag and run in `make check-all`.
