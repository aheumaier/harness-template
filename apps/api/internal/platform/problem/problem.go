// Package problem writes RFC 9457 problem details (application/problem+json).
// Every generated strict server is wired with RequestError/ResponseError so transport errors share one shape.
package problem

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/aheumaier/harness-template/apps/api/internal/platform/requestid"
)

// Problem mirrors components.schemas.Problem in api/openapi.yaml.
type Problem struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
}

// Write sends a problem response with Type "about:blank", the standard status text as Title and the
// request's Instance.
func Write(w http.ResponseWriter, r *http.Request, status int, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Problem{
		Type: "about:blank", Title: http.StatusText(status), Status: status, Detail: detail, Instance: Instance(r.Context()),
	})
}

// Instance identifies this occurrence of a problem: "urn:request-id:<X-Request-Id>" (components.schemas.Problem
// in api/openapi.yaml), or "" outside a request. Handlers building a generated problem response set it too.
func Instance(ctx context.Context) string {
	if id := requestid.FromContext(ctx); id != "" {
		return "urn:request-id:" + id
	}
	return ""
}

// RequestError handles malformed requests (bad params/body) in generated strict servers.
func RequestError(w http.ResponseWriter, r *http.Request, err error) {
	Write(w, r, http.StatusBadRequest, err.Error())
}

// ResponseError handles errors returned by handlers. Details are logged, never leaked to clients.
func ResponseError(w http.ResponseWriter, r *http.Request, err error) {
	slog.ErrorContext(r.Context(), "unhandled handler error", "err", err, "path", r.URL.Path)
	Write(w, r, http.StatusInternalServerError, "")
}
