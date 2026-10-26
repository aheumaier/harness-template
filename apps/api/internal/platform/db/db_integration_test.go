//go:build integration

package db_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/aheumaier/harness-template/apps/api/internal/platform/db"
	"github.com/aheumaier/harness-template/apps/api/internal/platform/health"
)

// Runs against the worktree's Postgres (make test-integration loads .env.local).
func TestPoolAndHealthz(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Fatal("DATABASE_URL not set; run scripts/setup-worktree.sh")
	}
	pool, err := db.NewPool(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	var ext string
	if err := pool.QueryRow(t.Context(), "SELECT extname FROM pg_extension WHERE extname = 'pgcrypto'").Scan(&ext); err != nil {
		t.Fatalf("pgcrypto missing; migrations not applied? %v", err)
	}

	rec := httptest.NewRecorder()
	health.Handler(pool).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("healthz = %d", rec.Code)
	}
}
