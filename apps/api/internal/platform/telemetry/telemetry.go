// Package telemetry configures OpenTelemetry tracing. Default exporter is none (spans are created but not
// exported); set OTEL_TRACES_EXPORTER=stdout for local debugging. An incoming W3C traceparent header is
// honoured, so a request continues its caller's trace.
package telemetry

import (
	"context"
	"fmt"
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Setup installs the global tracer provider and returns its shutdown func.
func Setup(_ context.Context, exporter string) (shutdown func(context.Context) error, err error) {
	var opts []sdktrace.TracerProviderOption
	switch exporter {
	case "", "none":
		// No span processor: spans are created (so instrumentation is exercised) but never exported.
	case "stdout":
		exp, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			return nil, fmt.Errorf("telemetry: stdout exporter: %w", err)
		}
		opts = append(opts, sdktrace.WithBatcher(exp))
	default:
		return nil, fmt.Errorf("telemetry: unknown exporter %q", exporter)
	}
	tp := sdktrace.NewTracerProvider(opts...)
	otel.SetTracerProvider(tp)
	// otelhttp extracts the parent span with the global propagator, which is a no-op until set.
	otel.SetTextMapPropagator(propagation.TraceContext{})
	return tp.Shutdown, nil
}

// Middleware starts an OTel server span per request (operation name until the route is known, then
// "METHOD /pattern") and continues an incoming traceparent. It is the only place otelhttp is used.
func Middleware(operation string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler { return otelhttp.NewHandler(next, operation) }
}
