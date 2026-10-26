package problem

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResponseError_DoesNotLeak(t *testing.T) {
	rec := httptest.NewRecorder()
	ResponseError(rec, httptest.NewRequest(http.MethodGet, "/x", nil), errors.New("secret dsn"))
	var p Problem
	if err := json.NewDecoder(rec.Body).Decode(&p); err != nil {
		t.Fatal(err)
	}
	if rec.Code != 500 || p.Detail != "" || rec.Header().Get("Content-Type") != "application/problem+json" {
		t.Fatalf("unexpected %d %+v", rec.Code, p)
	}
}
