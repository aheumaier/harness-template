// Package telemetry configures OpenTelemetry tracing. Default exporter is none (no-op);
// set OTEL_TRACES_EXPORTER=stdout for local debugging.
package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
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
	return tp.Shutdown, nil
}
