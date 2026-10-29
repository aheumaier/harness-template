package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/aheumaier/harness-template/apps/api/internal/platform/auth"
	"github.com/aheumaier/harness-template/apps/api/internal/platform/health"
	"github.com/aheumaier/harness-template/apps/api/internal/platform/logging"
	"github.com/aheumaier/harness-template/apps/api/internal/platform/requestid"
	"github.com/aheumaier/harness-template/apps/api/internal/platform/telemetry"
)

type pingFunc func(context.Context) error

func (f pingFunc) Ping(ctx context.Context) error { return f(ctx) }

// serve sends one request through the full middleware chain around /healthz and returns the response, the
// single access log line, the single recorded server span and the HTTP server duration histogram.
func serve(t *testing.T, ping pingFunc, header http.Header) (*httptest.ResponseRecorder, map[string]any, sdktrace.ReadOnlySpan, metricdata.Histogram[float64]) {
	t.Helper()
	prevTP, prevMP, prevProp, prevEH := otel.GetTracerProvider(), otel.GetMeterProvider(), otel.GetTextMapPropagator(), otel.GetErrorHandler()
	t.Cleanup(func() {
		otel.SetTracerProvider(prevTP)
		otel.SetMeterProvider(prevMP)
		otel.SetTextMapPropagator(prevProp)
		otel.SetErrorHandler(prevEH)
	})
	tel, err := telemetry.Setup(t.Context(), telemetry.Config{}, slog.New(slog.DiscardHandler)) // installs the propagator main uses
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), time.Second)
		defer cancel()
		if err := tel.Shutdown(ctx); err != nil {
			t.Error(err)
		}
	})
	spans := tracetest.NewSpanRecorder()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spans)))
	metrics := sdkmetric.NewManualReader()
	otel.SetMeterProvider(sdkmetric.NewMeterProvider(sdkmetric.WithReader(metrics)))

	authn, err := auth.New(auth.ModeDevStub)
	if err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	mux := http.NewServeMux()
	health.Register(mux, ping)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Host = "client-chosen.example:1234" // must not reach server.address
	for k, v := range header {
		req.Header[k] = v
	}
	rec := httptest.NewRecorder()
	handler(logging.New(&logs, slog.LevelInfo), authn, mux).ServeHTTP(rec, req)

	var line map[string]any
	if err := json.Unmarshal(logs.Bytes(), &line); err != nil {
		t.Fatalf("want exactly one JSON access line, got %q: %v", logs.String(), err)
	}
	ended := spans.Ended()
	if len(ended) != 1 {
		t.Fatalf("want 1 span, got %d", len(ended))
	}
	var rm metricdata.ResourceMetrics
	if err := metrics.Collect(t.Context(), &rm); err != nil {
		t.Fatal(err)
	}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if h, ok := m.Data.(metricdata.Histogram[float64]); ok && m.Name == "http.server.request.duration" {
				return rec, line, ended[0], h
			}
		}
	}
	t.Fatalf("no http.server.request.duration histogram in %+v", rm)
	return nil, nil, nil, metricdata.Histogram[float64]{}
}

func TestHandler_CorrelatesResponseLogAndSpan(t *testing.T) {
	const traceID = "0af7651916cd43dd8448eb211c80319c"
	for name, tc := range map[string]struct {
		ping       pingFunc
		wantStatus int
	}{
		"200": {func(context.Context) error { return nil }, http.StatusOK},
		"503": {func(context.Context) error { return errors.New("down") }, http.StatusServiceUnavailable},
	} {
		t.Run(name, func(t *testing.T) {
			rec, line, span, duration := serve(t, tc.ping, http.Header{
				requestid.Header: {"req-1"},
				"Traceparent":    {"00-" + traceID + "-b7ad6b7169203331-01"},
			})

			if rec.Code != tc.wantStatus || rec.Header().Get(requestid.Header) != "req-1" {
				t.Fatalf("got %d %s=%q, want %d req-1", rec.Code, requestid.Header, rec.Header().Get(requestid.Header), tc.wantStatus)
			}
			if tc.wantStatus != http.StatusOK {
				var p health.Problem
				if err := json.NewDecoder(rec.Body).Decode(&p); err != nil {
					t.Fatal(err)
				}
				if p.Instance == nil || *p.Instance != "urn:request-id:req-1" {
					t.Fatalf("problem instance = %v, want urn:request-id:req-1", p.Instance)
				}
			}

			for key, want := range map[string]any{
				"msg": "request", "method": "GET", "route": "/healthz", "status": float64(tc.wantStatus),
				"request_id": "req-1", "trace_id": traceID, "span_id": span.SpanContext().SpanID().String(),
			} {
				if line[key] != want {
					t.Errorf("log %s = %v, want %v", key, line[key], want)
				}
			}
			if _, ok := line["duration_ms"].(float64); !ok {
				t.Errorf("log duration_ms missing: %v", line)
			}

			if span.Name() != "GET /healthz" || span.SpanContext().TraceID().String() != traceID {
				t.Errorf("span %q in trace %s, want GET /healthz in %s", span.Name(), span.SpanContext().TraceID(), traceID)
			}
			if !slices.Contains(span.Attributes(), attribute.String("http.route", "/healthz")) {
				t.Errorf("span lacks http.route=/healthz: %v", span.Attributes())
			}

			// RED per route template: one histogram point (count = rate, status = errors, buckets = duration).
			if len(duration.DataPoints) != 1 || duration.DataPoints[0].Count != 1 {
				t.Fatalf("want one data point counting 1 request, got %+v", duration.DataPoints)
			}
			attrs := duration.DataPoints[0].Attributes
			for _, want := range []attribute.KeyValue{
				attribute.String("http.route", "/healthz"),
				attribute.String("http.request.method", "GET"),
				attribute.Int("http.response.status_code", tc.wantStatus),
				attribute.String("server.address", telemetry.ServiceName), // fixed, whatever Host the client sent
			} {
				if got, ok := attrs.Value(want.Key); !ok || got != want.Value {
					t.Errorf("metric %s = %v, want %v", want.Key, got.String(), want.Value.String())
				}
			}
		})
	}
}

func TestHandler_ReplacesInvalidRequestID(t *testing.T) {
	rec, line, _, _ := serve(t, func(context.Context) error { return nil }, http.Header{requestid.Header: {"bad id"}})
	got := rec.Header().Get(requestid.Header)
	if got == "bad id" || !requestid.Valid(got) || line["request_id"] != got {
		t.Fatalf("response ID %q, logged %v", got, line["request_id"])
	}
}
