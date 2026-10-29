package requestid

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestValid(t *testing.T) {
	for id, want := range map[string]bool{
		"abc":                     true,
		"A-z_0.9":                 true,
		strings.Repeat("a", 128):  true,
		"":                        false,
		strings.Repeat("a", 129):  false,
		"has space":               false,
		"new\nline":               false,
		`{"json":1}`:              false,
		"urn:colon":               false,
		"é":                       false,
		"trailing/slash":          false,
		"0af7651916cd43dd8448eb2": true,
	} {
		if got := Valid(id); got != want {
			t.Errorf("Valid(%q) = %v, want %v", id, got, want)
		}
	}
}

func TestMiddleware(t *testing.T) {
	for name, tc := range map[string]struct {
		incoming string
		keep     bool
	}{
		"valid is honoured":    {"req-1.a_B", true},
		"missing is generated": {"", false},
		"invalid is replaced":  {"bad id\r\nX-Injected: 1", false},
		"too long is replaced": {strings.Repeat("x", 129), false},
	} {
		t.Run(name, func(t *testing.T) {
			var seen string
			h := Middleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				seen = FromContext(r.Context())
			}))
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.incoming != "" {
				req.Header[Header] = []string{tc.incoming}
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			got := rec.Header().Get(Header)
			if got != seen {
				t.Fatalf("header %q != context %q", got, seen)
			}
			if !Valid(got) {
				t.Fatalf("response ID %q is not valid", got)
			}
			if tc.keep != (got == tc.incoming) {
				t.Fatalf("incoming %q, got %q, want kept=%v", tc.incoming, got, tc.keep)
			}
		})
	}
}

// A handler (or a generated Visit…Response) that blanks or replaces the header cannot change the ID sent.
func TestMiddleware_HeaderSurvivesHandler(t *testing.T) {
	for name, write := range map[string]func(http.ResponseWriter){
		"blank then WriteHeader": func(w http.ResponseWriter) {
			w.Header().Set(Header, "")
			w.WriteHeader(http.StatusOK)
		},
		"replace then Write": func(w http.ResponseWriter) {
			w.Header().Set(Header, "other")
			_, _ = w.Write([]byte("ok"))
		},
	} {
		t.Run(name, func(t *testing.T) {
			h := Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { write(w) }))
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set(Header, "req-1")
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if got := rec.Result().Header.Get(Header); got != "req-1" {
				t.Fatalf("sent %s = %q, want req-1", Header, got)
			}
		})
	}
}

// TestValid_MatchesContract fails when components.headers.XRequestId changes without Valid.
func TestValid_MatchesContract(t *testing.T) {
	spec, err := os.ReadFile("../../../../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	_, headers, ok := strings.Cut(string(spec), "\n  headers:\n")
	if ok {
		_, headers, ok = strings.Cut("\n"+headers, "\n    XRequestId:\n")
	}
	if !ok {
		t.Fatal("components.headers.XRequestId not found in api/openapi.yaml")
	}
	// The block ends at the first non-blank line indented 4 spaces or less (the next header or section).
	var block []string
	for line := range strings.Lines(headers) {
		if strings.TrimSpace(line) != "" && len(line)-len(strings.TrimLeft(line, " ")) <= 4 {
			break
		}
		block = append(block, line)
	}
	header := strings.Join(block, "")
	for _, want := range []string{"minLength: 1\n", "maxLength: " + strconv.Itoa(maxLen) + "\n", "pattern: '^[A-Za-z0-9._-]+$'"} {
		if !strings.Contains(header, want) {
			t.Errorf("XRequestId schema lacks %q; update Valid and the spec together", want)
		}
	}
}

func TestFromContext_Empty(t *testing.T) {
	if id := FromContext(t.Context()); id != "" {
		t.Fatalf("got %q outside a request", id)
	}
}
