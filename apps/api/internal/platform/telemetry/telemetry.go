// Package telemetry configures OpenTelemetry traces, metrics and logs (ADR 0021). Each signal has its own
// exporter, none by default; ExporterOTLP sends it over OTLP/HTTP to the worktree's collector (ADR 0017), and
// traces also take ExporterStdout for local debugging. An incoming W3C traceparent header is honoured, so a
// request continues its caller's trace.
//
// The RED metrics per route are otelhttp's own server metrics, not instruments of ours:
// http.server.request.duration (a histogram in seconds; its count is the rate) labelled with http.route,
// http.request.method and http.response.status_code (5xx are the errors). In Prometheus it is
// http_server_request_duration_seconds_{bucket,count,sum} with label http_route.
package telemetry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sync"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// ServiceName is the OTel service.name of every signal, the label the telemetry is queried by.
const ServiceName = "api"

// Exporter is where one signal's telemetry goes. config.Load parses OTEL_*_EXPORTER into it and is the only
// place it is validated; the zero value exports nothing, like ExporterNone.
type Exporter string

const (
	ExporterNone   Exporter = "none"
	ExporterStdout Exporter = "stdout" // traces only, pretty-printed for local debugging; not the spec's "console"
	ExporterOTLP   Exporter = "otlp"   // OTLP/HTTP to Config.Endpoint
)

// Config selects the exporter per signal. Endpoint is the OTLP/HTTP base URL (the per-signal path
// /v1/<signal> is appended); it must be set when any signal is ExporterOTLP.
type Config struct {
	Traces, Metrics, Logs Exporter
	Endpoint              *url.URL
}

// Telemetry is what Setup installed.
type Telemetry struct {
	// LogHandler sends slog records to the OTLP log exporter; nil unless Logs is ExporterOTLP.
	LogHandler slog.Handler
	shutdown   []func(context.Context) error
}

// Shutdown flushes and stops every provider Setup created, concurrently, so one stalled export does not use
// up the others' time. Bound ctx: a down collector is retried until it expires.
func (t *Telemetry) Shutdown(ctx context.Context) error {
	errs := make([]error, len(t.shutdown))
	var wg sync.WaitGroup
	for i, f := range t.shutdown {
		wg.Go(func() { errs[i] = f(ctx) })
	}
	wg.Wait()
	return errors.Join(errs...)
}

// Setup installs the global tracer provider, the meter provider (when metrics are exported) and the W3C
// propagator, and reports the SDK's export errors to errLog. errLog must not export over OTLP itself, or each
// failed log export would queue the record of its own failure.
//
// Exporters connect lazily: Setup succeeds without a collector, and failed exports reach errLog.
func Setup(ctx context.Context, cfg Config, errLog *slog.Logger) (*Telemetry, error) {
	t := &Telemetry{}
	if err := t.setup(ctx, cfg); err != nil {
		return nil, errors.Join(err, t.Shutdown(ctx))
	}
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) { errLog.Error("telemetry export failed", "err", err) }))
	// otelhttp extracts the parent span with the global propagator, which is a no-op until set.
	otel.SetTextMapPropagator(propagation.TraceContext{})
	return t, nil
}

func (t *Telemetry) setup(ctx context.Context, cfg Config) error {
	res, err := resource.New(ctx, resource.WithTelemetrySDK(),
		resource.WithAttributes(attribute.String("service.name", ServiceName)))
	if err != nil {
		return fmt.Errorf("telemetry: resource: %w", err)
	}

	// Without a span processor spans are still created (so IDs reach the logs) but never exported.
	traceOpts := []sdktrace.TracerProviderOption{sdktrace.WithResource(res)}
	switch cfg.Traces {
	case ExporterStdout:
		exp, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			return fmt.Errorf("telemetry: stdout trace exporter: %w", err)
		}
		traceOpts = append(traceOpts, sdktrace.WithBatcher(exp))
	case ExporterOTLP:
		exp, err := otlptracehttp.New(ctx, otlptracehttp.WithEndpointURL(signalURL(cfg.Endpoint, "traces")))
		if err != nil {
			return fmt.Errorf("telemetry: otlp trace exporter: %w", err)
		}
		traceOpts = append(traceOpts, sdktrace.WithBatcher(exp))
	}
	tp := sdktrace.NewTracerProvider(traceOpts...)
	t.shutdown = append(t.shutdown, tp.Shutdown)

	// Without a meter provider the global one stays a no-op: otelhttp records nothing.
	var mp *sdkmetric.MeterProvider
	if cfg.Metrics == ExporterOTLP {
		exp, err := otlpmetrichttp.New(ctx, otlpmetrichttp.WithEndpointURL(signalURL(cfg.Endpoint, "metrics")))
		if err != nil {
			return fmt.Errorf("telemetry: otlp metric exporter: %w", err)
		}
		// The SDK's export interval (60 s) applies unless OTEL_METRIC_EXPORT_INTERVAL (ms) overrides it.
		mp = sdkmetric.NewMeterProvider(sdkmetric.WithResource(res), sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exp)))
		t.shutdown = append(t.shutdown, mp.Shutdown)
	}

	if cfg.Logs == ExporterOTLP {
		exp, err := otlploghttp.New(ctx, otlploghttp.WithEndpointURL(signalURL(cfg.Endpoint, "logs")))
		if err != nil {
			return fmt.Errorf("telemetry: otlp log exporter: %w", err)
		}
		lp := sdklog.NewLoggerProvider(sdklog.WithResource(res), sdklog.WithProcessor(sdklog.NewBatchProcessor(exp)))
		t.shutdown = append(t.shutdown, lp.Shutdown)
		t.LogHandler = otelslog.NewHandler(ServiceName, otelslog.WithLoggerProvider(lp))
	}

	// Installed only once every exporter is built, so a failed Setup leaves the globals untouched.
	otel.SetTracerProvider(tp)
	if mp != nil {
		otel.SetMeterProvider(mp)
	}
	return nil
}

// signalURL is the OTLP/HTTP URL of one signal under the base endpoint, e.g. http://host:4318/v1/traces.
func signalURL(endpoint *url.URL, signal string) string {
	return endpoint.JoinPath("v1", signal).String()
}

// Middleware starts an OTel server span per request (operation name until the route is known, then
// "METHOD /pattern"), records otelhttp's server metrics labelled with http.route, and continues an incoming
// traceparent. It is the only place otelhttp is used. The server name is fixed, so a client-set Host header
// cannot add server.address values, and with them RED series.
func Middleware(operation string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return otelhttp.NewHandler(next, operation, otelhttp.WithServerName(ServiceName))
	}
}
