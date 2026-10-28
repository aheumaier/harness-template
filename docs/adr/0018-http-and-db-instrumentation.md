# ADR 0018: HTTP and database instrumentation through otelhttp and otelpgx

Status: proposed · 2026-10-28

Amends the "Standard library only for transport and logging" bullet of
[ADR 0002](0002-go-service-layout.md).

## Context
Gap G1 of exec plan 0003 (`docs/exec-plans/*/0003-harness-gap-closure.md`, slice P1.2) needs every
request to be traceable across its response, log lines and SQL queries: a request ID, one access log line, an
OTel server span and one child span per query. ADR 0002 keeps transport and logging on the standard library.
The standard library has neither OTel spans nor a pgx tracer, and its `http.ResponseWriter` exposes no
status or byte count to middleware.

## Decision
- **Transport stays `net/http`** (`ServeMux` patterns, generated strict server); logging stays `log/slog`.
  Instrumentation is added around it, never in place of it:
  - `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp` v0.72 (matches otel v1.47) for server
    spans and W3C `traceparent` extraction. It is used only in `platform/telemetry` (`telemetry.Middleware`).
  - `github.com/exaring/otelpgx` v0.13 as the pgx `QueryTracer` in `platform/db`, so every query is a child
    span of the span in its context.
  - `github.com/felixge/httpsnoop` captures status and duration for `platform/accesslog` and re-applies
    `X-Request-Id` in `platform/requestid`. It was already in the graph through otelhttp; it is promoted to a
    direct dependency.
- **otelpgx, not an in-repo `pgx.QueryTracer`.** It follows the OTel database semantic conventions
  (`db.query.text`; it also spans pool acquires), and it does not record query arguments by
  default, which keeps parameter values out of traces. Writing and keeping a tracer in step with semconv
  ourselves is the code nobody should write. The cost: otelpgx is third-party (not OTel contrib) and pre-1.0,
  so a minor bump may break it.
- **Middleware order is an invariant**, outermost first:
  `requestid → telemetry.Middleware (otelhttp) → accesslog.Middleware → authn → accesslog.Route → mux`.
  The request ID comes first so every response, log line and span has it; the access log sits outside auth,
  so rejected requests are logged too; `accesslog.Route` sits inside auth because auth copies the request
  and only the innermost copy sees the matched `r.Pattern`. `cmd/api/main_test.go` guards the order.
- **`accesslog` writes `r.Pattern` onto otelhttp's request copy**, so that otelhttp names the span
  `METHOD /pattern` after the handler returns. That relies on an otelhttp internal (it reads `Pattern` from
  the request it passed down). Any otelhttp bump must keep `TestHandler_CorrelatesResponseLogAndSpan` green.
- **Client-supplied correlation keys are trusted** until production: an incoming `traceparent` (trace ID,
  sampled flag) and a valid `X-Request-Id` are kept. [SECURITY.md](../SECURITY.md#before-production) lists
  the change for an edge deployment (`otelhttp.WithPublicEndpointFn`, generated request IDs for untrusted
  callers).

## Consequences
- Three new direct dependencies in `apps/api/go.mod`, each scanned by `govulncheck` and Trivy like any other.
  Bumps of otelhttp must move with the `go.opentelemetry.io/otel` version they are built for.
- Domains get spans and correlated logs without code of their own: they pass `ctx` and log with the
  `*Context` slog methods.
- The tracer provider and propagator are process globals read implicitly
  ([TD-19](../exec-plans/tech-debt-tracker.md)).
- **Supersede this ADR** when otelpgx is unmaintained or diverges from the semconv, when a Go standard
  library or OTel contrib pgx tracer replaces it, or when an otelhttp release stops honouring
  `Pattern` on the request it passes down (then the route capture moves into the span itself).
