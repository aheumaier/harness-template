// Package health serves GET /healthz (OpenAPI tag "platform"). It is plumbing, not a business domain.
// api.gen.go is generated from api/openapi.yaml by `make gen`; this file implements it.
package health

import (
	"context"
	"net/http"
	"time"

	"github.com/aheumaier/harness-template/apps/api/internal/platform/problem"
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
			Type: "about:blank", Title: "Service Unavailable", Status: http.StatusServiceUnavailable, Detail: ptr("database unreachable"),
		}}, nil
	}
	return GetHealth200JSONResponse{Status: Ok}, nil
}

func ptr[T any](v T) *T { return &v }
