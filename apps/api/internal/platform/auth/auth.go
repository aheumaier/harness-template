// Package auth is a stub authentication middleware. It establishes the seam (principal in context)
// so handlers are written against it from day one; replace the body, not the interface.
package auth

import (
	"context"
	"net/http"
	"strings"
)

// Principal is the authenticated caller.
type Principal struct {
	Subject string
}

type ctxKey struct{}

// Anonymous is the principal used when no credentials are presented.
var Anonymous = Principal{Subject: "anonymous"}

// Middleware attaches a Principal to the request context.
// STUB: accepts any bearer token as its own subject. Do not ship to production unchanged.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := Anonymous
		if tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok && tok != "" {
			p = Principal{Subject: tok}
		}
		next.ServeHTTP(w, r.WithContext(WithPrincipal(r.Context(), p)))
	})
}

// WithPrincipal returns ctx carrying p.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, ctxKey{}, p)
}

// FromContext returns the request principal, or Anonymous.
func FromContext(ctx context.Context) Principal {
	if p, ok := ctx.Value(ctxKey{}).(Principal); ok {
		return p
	}
	return Anonymous
}
