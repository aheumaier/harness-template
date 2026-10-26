// Package problem writes RFC 9457 problem details (application/problem+json).
// Every generated strict server is wired with RequestError/ResponseError so transport errors share one shape.
package problem

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// Problem mirrors components.schemas.Problem in api/openapi.yaml.
type Problem struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
}

// Write sends a problem response with Type "about:blank" and the standard status text as Title.
func Write(w http.ResponseWriter, status int, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Problem{Type: "about:blank", Title: http.StatusText(status), Status: status, Detail: detail})
}

// RequestError handles malformed requests (bad params/body) in generated strict servers.
func RequestError(w http.ResponseWriter, _ *http.Request, err error) {
	Write(w, http.StatusBadRequest, err.Error())
}

// ResponseError handles errors returned by handlers. Details are logged, never leaked to clients.
func ResponseError(w http.ResponseWriter, r *http.Request, err error) {
	slog.ErrorContext(r.Context(), "unhandled handler error", "err", err, "path", r.URL.Path)
	Write(w, http.StatusInternalServerError, "")
}
