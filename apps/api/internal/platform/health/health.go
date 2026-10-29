// Package health serves GET /healthz (OpenAPI tag "platform"). It is plumbing, not a business domain.
// api.gen.go is generated from api/openapi.yaml by `make gen`; this file implements it.
package health

import (
	"context"
	"net/http"
	"time"

	"github.com/aheumaier/harness-template/apps/api/internal/platform/problem"
	"github.com/aheumaier/harness-template/apps/api/internal/platform/requestid"
)

// Pinger is satisfied by *db.Pool.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Server implements StrictServerInterface.
type Server struct{ db Pinger }

var _ StrictServerInterface = (*Server)(nil)

// Register mounts the generated routes for this tag on mux.
func Register(mux *http.ServeMux, db Pinger) {
	HandlerFromMux(NewStrictHandlerWithOptions(&Server{db: db}, nil, StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  problem.RequestError,
		ResponseErrorHandlerFunc: problem.ResponseError,
	}), mux)
}

// GetHealth reports 200 when the database answers within the timeout, 503 otherwise.
func (s *Server) GetHealth(ctx context.Context, _ GetHealthRequestObject) (GetHealthResponseObject, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := s.db.Ping(ctx); err != nil {
		return GetHealth503ApplicationProblemPlusJSONResponse{ServiceUnavailableApplicationProblemPlusJSONResponse{
			Type: "about:blank", Title: http.StatusText(http.StatusServiceUnavailable), Status: http.StatusServiceUnavailable, Detail: ptr("database unreachable"),
			Instance: nonEmpty(problem.Instance(ctx)),
		}}, nil
	}
	// The contract requires the header on this response; requestid.Middleware re-applies it on the wire anyway.
	return GetHealth200JSONResponse{
		Body:    Health{Status: Ok},
		Headers: GetHealth200ResponseHeaders{XRequestId: requestid.FromContext(ctx)},
	}, nil
}

func ptr[T any](v T) *T { return &v }

// nonEmpty is nil for "", so omitempty drops the field instead of sending an empty string.
func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
