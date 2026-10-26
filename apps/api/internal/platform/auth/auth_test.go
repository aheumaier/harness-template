package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMiddleware(t *testing.T) {
	var got Principal
	h := Middleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { got = FromContext(r.Context()) }))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(httptest.NewRecorder(), req)
	if got != Anonymous {
		t.Fatalf("want anonymous, got %+v", got)
	}

	req.Header.Set("Authorization", "Bearer alice")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if got.Subject != "alice" {
		t.Fatalf("want alice, got %+v", got)
	}
}
