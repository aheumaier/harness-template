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

func TestHandler(t *testing.T) {
	for name, tc := range map[string]struct {
		ping pingFunc
		want int
	}{
		"up":   {func(context.Context) error { return nil }, http.StatusOK},
		"down": {func(context.Context) error { return errors.New("boom") }, http.StatusServiceUnavailable},
	} {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			Handler(tc.ping).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
			if rec.Code != tc.want {
				t.Fatalf("status %d, want %d", rec.Code, tc.want)
			}
		})
	}
}
