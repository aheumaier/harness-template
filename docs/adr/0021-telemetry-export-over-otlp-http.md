# ADR 0021: Telemetry leaves the process over OTLP/HTTP, one exporter per signal

Status: accepted · 2026-10-28

Amends the dependency list of [ADR 0018](0018-http-and-db-instrumentation.md) and the "Standard library only
for transport and logging" bullet of [ADR 0002](0002-go-service-layout.md), as far as logging goes.

## Context
Slice P1.3 of exec plan 0003 (`docs/exec-plans/*/0003-harness-gap-closure.md`) has to get the API's traces,
logs and RED metrics per route into the worktree's observability stack, a pinned `grafana/otel-lgtm`
container with an OTLP receiver ([ADR 0017](0017-per-worktree-observability-stack.md)). Until now spans were
created but only ever printed (`stdout`) or dropped, logs went to stdout only, and there were no metrics.
otelhttp, already the server instrumentation (ADR 0018), records `http.server.request.duration` once a meter
provider exists.

## Decision
- **Telemetry leaves the process over OTLP/HTTP only** (protobuf over HTTP, port 4318 in the collector). No
  gRPC exporter, no Prometheus scrape endpoint, no collector tailing files.
- **One exporter per signal, chosen with the spec's variables** `OTEL_TRACES_EXPORTER`,
  `OTEL_METRICS_EXPORTER` and `OTEL_LOGS_EXPORTER`: `none` (the default, also outside dev) or `otlp`; traces
  also take `stdout`, a pretty-printed local exporter. `stdout` differs from the spec's `console`, which is
  rejected like any unknown value. `OTEL_EXPORTER_OTLP_ENDPOINT` is the base URL and is required once any
  signal is `otlp`; `telemetry` appends `/v1/<signal>`.
- **Logs go to stdout always, and through the otelslog bridge when `OTEL_LOGS_EXPORTER=otlp`**, with the
  same records at the same level and the same request and trace IDs (`logging.New(..., tel.LogHandler)`).
  stdout stays the source of truth: the SDK's own export errors are logged there only, never over OTLP.
- **RED per route is otelhttp's `http.server.request.duration`** histogram (count = rate,
  `http.response.status_code` = errors, buckets = duration), labelled with `http.route`,
  `http.request.method` and a fixed `server.address` (`otelhttp.WithServerName`), so a client-set `Host`
  cannot multiply series. No instruments of our own.
- **Config owns** the three `OTEL_*_EXPORTER` variables and `OTEL_EXPORTER_OTLP_ENDPOINT`: `platform/config`
  parses them into `telemetry.Config` and is the only place they are validated, so a typo fails at startup.
  The endpoint passed in code also wins over `OTEL_EXPORTER_OTLP_{TRACES,METRICS,LOGS}_ENDPOINT`, and the
  protocol is always HTTP/protobuf (`OTEL_EXPORTER_OTLP_PROTOCOL` has no effect).
- **The SDK reads its own tuning variables**, unvalidated by us: OTLP headers, timeout, compression and
  certificates (`OTEL_EXPORTER_OTLP_[SIGNAL_]{HEADERS,TIMEOUT,COMPRESSION,CERTIFICATE,CLIENT_CERTIFICATE,
  CLIENT_KEY}`), the metric export interval and timeout (`OTEL_METRIC_EXPORT_INTERVAL`, default 60 s), the
  span and log batchers (`OTEL_BSP_*`, `OTEL_BLRP_*`), the sampler (`OTEL_TRACES_SAMPLER[_ARG]`) and the
  attribute limits.
- **`OTEL_SERVICE_NAME` is ignored**: `service.name` is always `api` (`telemetry.ServiceName`). The SDK
  providers still merge the other keys of `OTEL_RESOURCE_ATTRIBUTES` into the resource, but never its
  `service.name` (`TestSetup_ServiceNameIsFixed`).
- **Six new direct dependencies**, all from the OTel Go project at the line of otel v1.47:
  `go.opentelemetry.io/otel/sdk/metric` v1.47, `go.opentelemetry.io/otel/sdk/log` v1.47,
  `.../exporters/otlp/otlptrace/otlptracehttp` v1.47, `.../exporters/otlp/otlpmetric/otlpmetrichttp` v1.47,
  and two pre-1.0 ones, `.../exporters/otlp/otlplog/otlploghttp` v0.23 and
  `go.opentelemetry.io/contrib/bridges/otelslog` v0.21. They add to ADR 0018's list (otelhttp, otelpgx,
  httpsnoop).

## Consequences
- The HTTP exporters still bring `google.golang.org/grpc`, `google.golang.org/genproto`,
  `google.golang.org/protobuf` and `golang.org/x/net` into the binary: the generated OTLP collector packages
  (`go.opentelemetry.io/proto/otlp/collector/*`) carry gRPC service stubs and `grpc/status`. Their
  advisories reach `govulncheck` and Trivy (`deps-no-known-vulns`) like our own code.
- The logs signal and its bridge are pre-1.0: a minor bump may change the API or the record mapping.
  `TestSetup_OTLPSendsEverySignalToItsPath` and `TestNew_AlsoHandlersGetSameRecordsAtSameLevel` guard it.
- Exporters connect lazily: the API starts without a collector, failed exports are logged to stdout, and
  shutdown flushes the three providers concurrently within 5 s.
- `none` stays the default outside dev; the per-worktree setup turns `otlp` on once the collector port
  exists (exec plan 0003 P1.4).
- **Supersede this ADR** when the collector tails stdout instead of receiving logs over OTLP, when the
  export moves to gRPC, or when the logs bridge or exporter breaks on a bump and is dropped.
