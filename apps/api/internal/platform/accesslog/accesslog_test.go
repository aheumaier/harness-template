package accesslog

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The matched case (route template, span name, IDs) is covered through the full chain in cmd/api.
func TestMiddleware_RouteThroughCopiedRequest(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /items/{id}", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	// copyReq stands in for auth, which hands the mux a copy of the request.
	copyReq := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { next.ServeHTTP(w, r.WithContext(r.Context())) })
	}

	for path, want := range map[string]struct {
		route  string
		status float64
	}{
		"/items/42": {"/items/{id}", http.StatusTeapot},
		"/nope":     {"", http.StatusNotFound},
	} {
		t.Run(path, func(t *testing.T) {
			var buf bytes.Buffer
			req := httptest.NewRequest(http.MethodGet, path, nil)
			Middleware(slog.New(slog.NewJSONHandler(&buf, nil)))(copyReq(Route(mux))).ServeHTTP(httptest.NewRecorder(), req)

			var line map[string]any
			if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
				t.Fatal(err)
			}
			if line["route"] != want.route || line["status"] != want.status || line["method"] != "GET" {
				t.Fatalf("got %s", buf.String())
			}
			if want.route != "" && req.Pattern != "GET "+want.route {
				t.Fatalf("outer request Pattern = %q", req.Pattern)
			}
		})
	}
}
