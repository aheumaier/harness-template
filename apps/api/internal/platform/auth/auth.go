// Package auth is a stub authentication middleware. It establishes the seam (principal in context)
// so handlers are written against it from day one; replace the body, not the interface.
//
// It fails closed: New only accepts ModeDevStub, so a process configured for anything else refuses to
// start until a real verifier (JWT signature/issuer/audience/expiry, or token introspection) exists.
package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// ModeDevStub is the only implemented mode. Never use it outside local development.
const ModeDevStub = "dev-stub"

// Principal is the authenticated caller.
type Principal struct {
	Subject string
}

type ctxKey struct{}

// Anonymous is the principal used when no credentials are presented.
var Anonymous = Principal{Subject: "anonymous"}

// New returns the authentication middleware for mode, or an error if no verifier exists for it.
func New(mode string) (func(http.Handler) http.Handler, error) {
	if mode != ModeDevStub {
		return nil, fmt.Errorf("auth: mode %q not implemented; only %q (local development) is available", mode, ModeDevStub)
	}
	return devStub, nil
}

// devStub attaches a Principal to the request context.
// INSECURE: it trusts any bearer token as its own subject. Development only; see New.
func devStub(next http.Handler) http.Handler {
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
