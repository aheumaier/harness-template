// Package logging builds the process-wide structured logger.
package logging

import (
	"context"
	"io"
	"log/slog"

	"go.opentelemetry.io/otel/trace"

	"github.com/aheumaier/harness-template/apps/api/internal/platform/requestid"
)

// New returns a JSON slog logger. All logs are structured; never use fmt.Print for diagnostics.
// Every record logged with a request context carries request_id, trace_id and span_id, so the lines of
// one request can be found by its X-Request-Id or its trace.
func New(w io.Writer, level slog.Level) *slog.Logger {
	return slog.New(contextHandler{slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})})
}

// contextHandler adds the correlation IDs from the record's context. Under WithGroup they land inside the
// group, like any attribute added at Handle time.
type contextHandler struct{ slog.Handler }

func (h contextHandler) Handle(ctx context.Context, r slog.Record) error {
	if id := requestid.FromContext(ctx); id != "" {
		r.AddAttrs(slog.String("request_id", id))
	}
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		r.AddAttrs(slog.String("trace_id", sc.TraceID().String()), slog.String("span_id", sc.SpanID().String()))
	}
	return h.Handler.Handle(ctx, r)
}

func (h contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return contextHandler{h.Handler.WithAttrs(attrs)}
}

func (h contextHandler) WithGroup(name string) slog.Handler {
	return contextHandler{h.Handler.WithGroup(name)}
}
