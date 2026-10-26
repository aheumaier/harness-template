package main

import (
	"net/http"

	"github.com/aheumaier/harness-template/apps/api/internal/platform/db"
	"github.com/aheumaier/harness-template/apps/api/internal/platform/health"
)

// routes wires every HTTP surface. `make new-domain` inserts domain registrations at the marker.
func routes(pool *db.Pool) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /healthz", health.Handler(pool))
	// harness:domain-routes (generator marker, keep)
	return mux
}
