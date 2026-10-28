// Package accesslog writes one structured log line per request: method, route template, status and
// duration (the request and trace IDs come from the context, see platform/logging).
//
// It is two middlewares because http.ServeMux records the matched route template in Request.Pattern of the
// request it is given, and auth (between the two) hands the mux a copy of the request. Route, directly
// around the mux, reports the pattern back to Middleware, which sits directly inside otelhttp.
package accesslog

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/felixge/httpsnoop"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type routeKey struct{}

// Middleware logs one "request" line per request after it completes. Wrap it directly in otelhttp, with
// nothing between them that copies the request: it sets the pattern on the request otelhttp holds, so
// otelhttp names the server span "GET /route" (and labels its metrics with http.route).
func Middleware(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			pattern := new(string)
			m := httpsnoop.CaptureMetrics(next, w, r.WithContext(context.WithValue(r.Context(), routeKey{}, pattern)))
			if r.Pattern == "" {
				r.Pattern = *pattern // as http.ServeMux does: the matched pattern is recorded on the request in place
			}
			rt := route(*pattern)
			if rt != "" {
				// otelhttp derives the span's own http.route only when the span starts, before the mux ran.
				trace.SpanFromContext(r.Context()).SetAttributes(attribute.String("http.route", rt))
			}
			log.LogAttrs(r.Context(), slog.LevelInfo, "request",
				slog.String("method", r.Method),
				slog.String("route", rt),
				slog.Int("status", m.Code),
				slog.Float64("duration_ms", float64(m.Duration.Microseconds())/1000),
			)
		})
	}
}

// Route wraps the http.ServeMux and reports the route template it matched to Middleware.
func Route(mux http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mux.ServeHTTP(w, r)
		if p, ok := r.Context().Value(routeKey{}).(*string); ok {
			*p = r.Pattern
		}
	})
}

// route strips the method from a ServeMux pattern ("GET /healthz" → "/healthz"); "" when no route matched.
func route(pattern string) string {
	if i := strings.IndexByte(pattern, '/'); i >= 0 {
		return pattern[i:]
	}
	return ""
}
