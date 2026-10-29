//go:build integration

package db_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"github.com/aheumaier/harness-template/apps/api/internal/platform/db"
	"github.com/aheumaier/harness-template/apps/api/internal/platform/health"
	"github.com/aheumaier/harness-template/apps/api/internal/platform/requestid"
)

func databaseURL(t *testing.T) string {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Fatal("DATABASE_URL not set; run scripts/setup-worktree.sh")
	}
	return url
}

// Every query is a child span of the span in its context (the HTTP server span in a request).
func TestPool_QueryIsChildSpan(t *testing.T) {
	spans := tracetest.NewSpanRecorder()
	t.Cleanup(func(prev trace.TracerProvider) func() {
		return func() { otel.SetTracerProvider(prev) }
	}(otel.GetTracerProvider()))
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spans)))

	pool, err := db.NewPool(t.Context(), databaseURL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	ctx, parent := otel.Tracer("test").Start(t.Context(), "parent")
	var one int
	if err := pool.QueryRow(ctx, "SELECT 1").Scan(&one); err != nil {
		t.Fatal(err)
	}
	parent.End()

	query := attribute.String("db.query.text", "SELECT 1")
	for _, s := range spans.Ended() {
		if s.Parent().SpanID() == parent.SpanContext().SpanID() && slices.Contains(s.Attributes(), query) {
			return
		}
	}
	t.Fatalf("no span with %v under the parent among %d spans", query, len(spans.Ended()))
}

// Runs against the worktree's Postgres (make test-integration loads .env.local).
func TestPoolAndHealthz(t *testing.T) {
	pool, err := db.NewPool(t.Context(), databaseURL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	var ext string
	if err := pool.QueryRow(t.Context(), "SELECT extname FROM pg_extension WHERE extname = 'pgcrypto'").Scan(&ext); err != nil {
		t.Fatalf("pgcrypto missing; migrations not applied? %v", err)
	}

	mux := http.NewServeMux()
	health.Register(mux, pool)
	rec := httptest.NewRecorder()
	requestid.Middleware(mux).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("healthz = %d", rec.Code)
	}
	if id := rec.Header().Get(requestid.Header); !requestid.Valid(id) {
		t.Fatalf("%s = %q, want a valid ID", requestid.Header, id)
	}
}
