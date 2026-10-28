package problem

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aheumaier/harness-template/apps/api/internal/platform/requestid"
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

func TestRequestError_SetsInstanceFromRequestID(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	RequestError(rec, req.WithContext(requestid.NewContext(req.Context(), "req-1")), errors.New("bad param"))
	var p Problem
	if err := json.NewDecoder(rec.Body).Decode(&p); err != nil {
		t.Fatal(err)
	}
	if rec.Code != 400 || p.Instance != "urn:request-id:req-1" || p.Detail != "bad param" {
		t.Fatalf("unexpected %d %+v", rec.Code, p)
	}
}

func TestInstance_OutsideRequest(t *testing.T) {
	if got := Instance(t.Context()); got != "" {
		t.Fatalf("got %q", got)
	}
}
