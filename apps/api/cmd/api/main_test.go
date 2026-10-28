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

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
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
// single access log line and the single recorded server span.
func serve(t *testing.T, ping pingFunc, header http.Header) (*httptest.ResponseRecorder, map[string]any, sdktrace.ReadOnlySpan) {
	t.Helper()
	prevTP, prevProp := otel.GetTracerProvider(), otel.GetTextMapPropagator()
	t.Cleanup(func() { otel.SetTracerProvider(prevTP); otel.SetTextMapPropagator(prevProp) })
	shutdown, err := telemetry.Setup(t.Context(), "none") // installs the propagator main uses
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = shutdown(context.WithoutCancel(t.Context())) })
	spans := tracetest.NewSpanRecorder()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spans)))

	authn, err := auth.New(auth.ModeDevStub)
	if err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	mux := http.NewServeMux()
	health.Register(mux, ping)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
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
	return rec, line, ended[0]
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
			rec, line, span := serve(t, tc.ping, http.Header{
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
		})
	}
}

func TestHandler_ReplacesInvalidRequestID(t *testing.T) {
	rec, line, _ := serve(t, func(context.Context) error { return nil }, http.Header{requestid.Header: {"bad id"}})
	got := rec.Header().Get(requestid.Header)
	if got == "bad id" || !requestid.Valid(got) || line["request_id"] != got {
		t.Fatalf("response ID %q, logged %v", got, line["request_id"])
	}
}
