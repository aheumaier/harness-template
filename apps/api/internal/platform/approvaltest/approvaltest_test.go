package approvaltest

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
)

// recorder captures what Verify reports instead of failing the real test.
type recorder struct {
	testing.TB
	errs []string
}

func (r *recorder) Helper() {}

func (r *recorder) Errorf(format string, args ...any) {
	r.errs = append(r.errs, fmt.Sprintf(format, args...))
}

// Fatalf stops Verify as the real one does; verify recovers.
func (r *recorder) Fatalf(format string, args ...any) {
	r.Errorf(format, args...)
	panic(r)
}

func (r *recorder) verify(operationID string, rec *httptest.ResponseRecorder, opts ...Option) {
	defer func() {
		if p := recover(); p != nil && p != r {
			panic(p)
		}
	}()
	Verify(r, operationID, rec, opts...)
}

// take returns the reports since the last call.
func (r *recorder) take() []string {
	errs := r.errs
	r.errs = nil
	return errs
}

func response(status int, contentType, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	if contentType != "" {
		rec.Header().Set("Content-Type", contentType)
	}
	rec.WriteHeader(status)
	_, _ = rec.WriteString(body)
	return rec
}

func wantOne(t *testing.T, what string, errs []string, parts ...string) {
	t.Helper()
	if len(errs) != 1 {
		t.Fatalf("%s: %q", what, errs)
	}
	for _, p := range parts {
		if !strings.Contains(errs[0], p) {
			t.Fatalf("%s: %q lacks %q", what, errs[0], p)
		}
	}
}

func TestVerify(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv(Env, "") // also under `make approve`
	r := &recorder{TB: t}

	r.verify("getThing", response(200, "application/json", `{"b": 1.50, "a": [true]}`))
	wantOne(t, "missing file", r.take(), "testdata/approved/getThing.200.json: no approved response for getThing 200; run `make approve`")

	t.Setenv(Env, "1")
	r.verify("getThing", response(200, "application/json", `{"b": 1.50, "a": [true]}`))
	r.verify("getThing", response(200, "application/json", `{"b": 1.50, "a": [true]}`)) // the same response again
	r.verify("getThing", response(406, "text/plain; charset=utf-8", "nope\n"))
	r.verify("getThing", response(404, "text/plain", "  "), Case("gone"))
	r.verify("deleteThing", response(204, "", ""))
	if errs := r.take(); len(errs) != 0 {
		t.Fatalf("approve: %q", errs)
	}
	for name, want := range map[string]string{
		"getThing.200.json":      "{\n  \"headers\": {\n    \"Content-Type\": \"application/json\"\n  },\n  \"body\": {\n    \"a\": [\n      true\n    ],\n    \"b\": 1.50\n  }\n}\n",
		"getThing.406.json":      "{\n  \"headers\": {\n    \"Content-Type\": \"text/plain; charset=utf-8\"\n  },\n  \"body\": \"nope\\n\"\n}\n",
		"getThing.404.gone.json": "{\n  \"headers\": {\n    \"Content-Type\": \"text/plain\"\n  },\n  \"body\": \"  \"\n}\n",
		"deleteThing.204.json":   "{\n  \"headers\": {}\n}\n",
	} {
		got, err := read(name)
		if err != nil || string(got) != want {
			t.Errorf("%s: %v\n%s\nwant\n%s", name, err, got, want)
		}
	}
	r.verify("getThing", response(406, "text/plain; charset=utf-8", "no\n"))
	wantOne(t, "two responses, one name", r.take(), "testdata/approved/getThing.406.json: two different responses in one run", `approvaltest.Case("…")`)

	t.Setenv(Env, "")
	r.verify("getThing", response(200, "application/json", `{"a":[true],"b":1.50}`)) // same JSON, other bytes
	r.verify("getThing", response(404, "text/plain", "  "), Case("gone"))
	if errs := r.take(); len(errs) != 0 {
		t.Fatalf("unchanged: %q", errs)
	}
	r.verify("getThing", response(200, "application/json", `{"a": [false], "b": 1.50}`))
	wantOne(t, "changed body", r.take(), "getThing.200.json: the response differs from the approved one [api-responses-approved]", "+++ received", "false")
	changed := response(200, "application/json", `{"a":[true],"b":1.50}`)
	changed.Header().Set("Cache-Control", "no-store")
	r.verify("getThing", changed)
	wantOne(t, "changed header", r.take(), "getThing.200.json: the response differs from the approved one [api-responses-approved]", `"Cache-Control": "no-store"`)

	r.verify("GetThing", response(200, "application/json", `{}`))
	r.verify("getThing", response(200, "application/json", `{`))
	r.verify("getThing", response(200, "application/json", `{} {}`))
	r.verify("getThing", response(200, "application/json", `{}`), Case("Gone"))
	t.Setenv(Env, "true")
	r.verify("getThing", response(200, "application/json", `{}`))
	errs := r.take()
	for i, want := range []string{
		"not a camelCase operationId",
		"JSON body: unexpected EOF",
		"JSON body: more than one value",
		`case "Gone" is not lower-case kebab-case`,
		`APPROVE_RESPONSES="true": want 1`,
	} {
		if i >= len(errs) || !strings.Contains(errs[i], want) {
			t.Fatalf("bad input %d: want %q in %q", i, want, errs)
		}
	}
}
