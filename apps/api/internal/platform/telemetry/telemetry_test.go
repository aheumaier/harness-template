package telemetry

import (
	"bytes"
	"context"
	"encoding/pem"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// restoreGlobals puts back the process-wide OTel state Setup replaces. Only partly: OTel's global delegate is
// bound to the first provider ever set, so code that captured a tracer earlier keeps it (TD-19).
func restoreGlobals(t *testing.T) {
	tp, mp, prop, eh := otel.GetTracerProvider(), otel.GetMeterProvider(), otel.GetTextMapPropagator(), otel.GetErrorHandler()
	t.Cleanup(func() {
		otel.SetTracerProvider(tp)
		otel.SetMeterProvider(mp)
		otel.SetTextMapPropagator(prop)
		otel.SetErrorHandler(eh)
	})
}

// shutdown flushes tel within d and returns Shutdown's result.
func shutdown(t *testing.T, tel *Telemetry, d time.Duration) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), d)
	defer cancel()
	return tel.Shutdown(ctx)
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

// closedEndpoint is an http:// URL nothing listens on.
func closedEndpoint(t *testing.T) *url.URL {
	t.Helper()
	srv := httptest.NewServer(http.NotFoundHandler())
	u := mustURL(t, srv.URL)
	srv.Close()
	return u
}

// syncBuffer is a bytes.Buffer the SDK's export goroutines may write to while the test reads it.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestSetup_OTLPSendsEverySignalToItsPath(t *testing.T) {
	restoreGlobals(t)
	var mu sync.Mutex
	var paths []string
	collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/x-protobuf")
	}))
	defer collector.Close()

	tel, err := Setup(t.Context(), Config{Traces: ExporterOTLP, Metrics: ExporterOTLP, Logs: ExporterOTLP,
		Endpoint: mustURL(t, collector.URL)}, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	_, span := otel.Tracer("test").Start(t.Context(), "op")
	span.End()
	counter, err := otel.Meter("test").Int64Counter("test.count")
	if err != nil {
		t.Fatal(err)
	}
	counter.Add(t.Context(), 1)
	slog.New(tel.LogHandler).InfoContext(t.Context(), "hello")

	if err := shutdown(t, tel, 5*time.Second); err != nil { // flushes all three
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	for _, want := range []string{"/v1/traces", "/v1/metrics", "/v1/logs"} {
		if !slices.Contains(paths, want) {
			t.Errorf("collector got %v, want %s", paths, want)
		}
	}
}

func TestSetup_ReportsFailedExportsToErrLog(t *testing.T) {
	restoreGlobals(t)
	var errLog syncBuffer
	tel, err := Setup(t.Context(), Config{Traces: ExporterOTLP, Metrics: ExporterOTLP, Endpoint: closedEndpoint(t)},
		slog.New(slog.NewJSONHandler(&errLog, nil)))
	if err != nil {
		t.Fatalf("Setup must not dial: %v", err)
	}
	_, span := otel.Tracer("test").Start(t.Context(), "op")
	span.End()
	counter, err := otel.Meter("test").Int64Counter("test.count")
	if err != nil {
		t.Fatal(err)
	}
	counter.Add(t.Context(), 1)

	// The span batcher hands its export error to the SDK's error handler (errLog); the metric reader
	// returns its own from Shutdown.
	if err := shutdown(t, tel, 200*time.Millisecond); err == nil || !strings.Contains(err.Error(), "connection refused") {
		t.Fatalf("want Shutdown to report the failed flush, got %v", err)
	}
	if !strings.Contains(errLog.String(), "telemetry export failed") {
		t.Fatalf("errLog got %q, want the export failure", errLog.String())
	}
}

func TestSetup_FailureLeavesGlobalsUntouched(t *testing.T) {
	restoreGlobals(t)
	// A CA certificate for the log exporter only, against an http:// endpoint: otlploghttp.New refuses that
	// combination, after the trace and metric exporters were built.
	tlsSrv := httptest.NewTLSServer(http.NotFoundHandler())
	ca := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(ca, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: tlsSrv.Certificate().Raw}), 0o600); err != nil {
		t.Fatal(err)
	}
	tlsSrv.Close()
	t.Setenv("OTEL_EXPORTER_OTLP_LOGS_CERTIFICATE", ca)

	for name, cfg := range map[string]Config{
		"metrics built, logs fail":            {Metrics: ExporterOTLP, Logs: ExporterOTLP},
		"traces and metrics built, logs fail": {Traces: ExporterOTLP, Metrics: ExporterOTLP, Logs: ExporterOTLP},
	} {
		t.Run(name, func(t *testing.T) {
			cfg.Endpoint = closedEndpoint(t)
			tp, mp := otel.GetTracerProvider(), otel.GetMeterProvider()
			_, err := Setup(t.Context(), cfg, slog.New(slog.DiscardHandler))
			if err == nil || !strings.HasPrefix(err.Error(), "telemetry: otlp log exporter:") {
				t.Fatalf("want the log exporter error, got %v", err)
			}
			if otel.GetTracerProvider() != tp || otel.GetMeterProvider() != mp {
				t.Fatal("a failed Setup replaced a global provider")
			}
		})
	}
}

// The SDK providers merge OTEL_RESOURCE_ATTRIBUTES into the resource; service.name stays ServiceName (ADR 0021).
func TestSetup_ServiceNameIsFixed(t *testing.T) {
	restoreGlobals(t)
	t.Setenv("OTEL_SERVICE_NAME", "other")
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "service.name=other,deployment.environment.name=dev")
	tel, err := Setup(t.Context(), Config{}, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := shutdown(t, tel, time.Second); err != nil {
			t.Error(err)
		}
	})
	_, span := otel.Tracer("test").Start(t.Context(), "op")
	defer span.End()
	res := span.(sdktrace.ReadOnlySpan).Resource()
	for key, want := range map[attribute.Key]string{"service.name": ServiceName, "deployment.environment.name": "dev"} {
		if got, _ := res.Set().Value(key); got.AsString() != want {
			t.Errorf("resource %s = %q, want %q", key, got.AsString(), want)
		}
	}
}
