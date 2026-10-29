// Package requestid gives every request an ID that correlates its response, log lines and problem details.
// The ID ends up in logs, so an incoming X-Request-Id is parsed here, at the boundary (golden principle 2):
// it is honoured only if it matches components.headers.XRequestId in api/openapi.yaml, else replaced.
package requestid

import (
	"context"
	"crypto/rand"
	"io"
	"net/http"

	"github.com/felixge/httpsnoop"
)

// Header is the request and response header that carries the ID.
const Header = "X-Request-Id"

// maxLen mirrors maxLength of components.headers.XRequestId.
const maxLen = 128

type ctxKey struct{}

// Valid reports whether id is 1-128 characters of A-Za-z0-9._- (the contract's pattern).
func Valid(id string) bool {
	if id == "" || len(id) > maxLen {
		return false
	}
	for i := 0; i < len(id); i++ {
		switch c := id[i]; {
		case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', '0' <= c && c <= '9', c == '.', c == '_', c == '-':
		default:
			return false
		}
	}
	return true
}

// Middleware puts the request ID into the context and the response header. It must be the outermost
// middleware, so that every response and log line carries the ID. The header is set again when the headers
// are sent (WriteHeader, or the first Write or ReadFrom), so a handler or a generated Visit…Response that
// blanks or replaces it cannot change the ID on the wire.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(Header)
		if !Valid(id) {
			id = rand.Text() // 26 base32 characters, within the contract's charset
		}
		w.Header().Set(Header, id)
		sent := false
		apply := func() {
			if !sent {
				sent = true
				w.Header().Set(Header, id)
			}
		}
		hooked := httpsnoop.Wrap(w, httpsnoop.Hooks{
			WriteHeader: func(next httpsnoop.WriteHeaderFunc) httpsnoop.WriteHeaderFunc {
				return func(code int) { apply(); next(code) }
			},
			Write: func(next httpsnoop.WriteFunc) httpsnoop.WriteFunc {
				return func(b []byte) (int, error) { apply(); return next(b) }
			},
			ReadFrom: func(next httpsnoop.ReadFromFunc) httpsnoop.ReadFromFunc {
				return func(src io.Reader) (int64, error) { apply(); return next(src) }
			},
		})
		next.ServeHTTP(hooked, r.WithContext(NewContext(r.Context(), id)))
	})
}

// NewContext returns ctx carrying id.
func NewContext(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// FromContext returns the request ID, or "" outside a request.
func FromContext(ctx context.Context) string {
	id, _ := ctx.Value(ctxKey{}).(string)
	return id
}
