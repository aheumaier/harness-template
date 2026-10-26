package health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type pingFunc func(context.Context) error

func (f pingFunc) Ping(ctx context.Context) error { return f(ctx) }

func TestGetHealth(t *testing.T) {
	for name, tc := range map[string]struct {
		ping        pingFunc
		wantStatus  int
		wantContent string
	}{
		"up":   {func(context.Context) error { return nil }, http.StatusOK, "application/json"},
		"down": {func(context.Context) error { return errors.New("boom") }, http.StatusServiceUnavailable, "application/problem+json"},
	} {
		t.Run(name, func(t *testing.T) {
			mux := http.NewServeMux()
			Register(mux, tc.ping)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
			if rec.Code != tc.wantStatus || rec.Header().Get("Content-Type") != tc.wantContent {
				t.Fatalf("got %d %q, want %d %q", rec.Code, rec.Header().Get("Content-Type"), tc.wantStatus, tc.wantContent)
			}
		})
	}
}
