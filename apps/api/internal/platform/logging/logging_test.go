package logging

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"

	"go.opentelemetry.io/otel/trace"

	"github.com/aheumaier/harness-template/apps/api/internal/platform/requestid"
)

func TestNew_AddsCorrelationIDsFromContext(t *testing.T) {
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: trace.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		SpanID:  trace.SpanID{1, 2, 3, 4, 5, 6, 7, 8},
	})
	ctx := trace.ContextWithSpanContext(requestid.NewContext(t.Context(), "req-1"), sc)

	var buf bytes.Buffer
	New(&buf, slog.LevelInfo).With("k", "v").InfoContext(ctx, "hello")

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{
		"msg":        "hello",
		"k":          "v",
		"request_id": "req-1",
		"trace_id":   sc.TraceID().String(),
		"span_id":    sc.SpanID().String(),
	} {
		if line[key] != want {
			t.Errorf("%s = %v, want %q (line %s)", key, line[key], want, buf.String())
		}
	}
}

func TestNew_AlsoHandlersGetSameRecordsAtSameLevel(t *testing.T) {
	var stdout, export bytes.Buffer
	// The export handler accepts every level, as the OTLP bridge does; New must apply LOG_LEVEL to it.
	all := slog.NewJSONHandler(&export, &slog.HandlerOptions{Level: slog.LevelDebug})
	log := New(&stdout, slog.LevelInfo, nil, all)
	ctx := requestid.NewContext(t.Context(), "req-1")
	log.DebugContext(ctx, "dropped")
	log.InfoContext(ctx, "kept")

	for name, buf := range map[string]*bytes.Buffer{"stdout": &stdout, "export": &export} {
		var line map[string]any
		if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
			t.Fatalf("%s: want exactly one JSON line, got %q: %v", name, buf.String(), err)
		}
		if line["msg"] != "kept" || line["request_id"] != "req-1" {
			t.Errorf("%s: got %s", name, buf.String())
		}
	}
}

func TestNew_NoRequestContext(t *testing.T) {
	var buf bytes.Buffer
	New(&buf, slog.LevelInfo).InfoContext(t.Context(), "boot")

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"request_id", "trace_id", "span_id"} {
		if _, ok := line[key]; ok {
			t.Errorf("%s present outside a request: %s", key, buf.String())
		}
	}
}
