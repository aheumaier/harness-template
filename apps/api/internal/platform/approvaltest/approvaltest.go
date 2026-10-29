// Package approvaltest pins HTTP responses to approved files, for handler tests (rules.md#api-responses-approved,
// ADR 0022). It is test-only: only _test.go files may import it (rules.md#go-test-helpers-only-in-tests).
//
// Verify renders a recorded response as testdata/approved/<operationId>.<status>[.<case>].json (see File) and
// fails the test when the file differs. `make approve` sets APPROVE_RESPONSES=1, which writes the files
// instead; the diff then shows the change in review, and `make approved-check`
// (apps/api/tools/harness/approvedcheck.go, which reads the same format) checks every file against
// api/openapi.yaml (rules.md#api-approved-match-contract).
package approvaltest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
)

// Dir holds the approved files, relative to the test's package directory.
const Dir = "testdata/approved"

// Env is the variable `make approve` sets to 1 to write the approved files instead of comparing with them.
const Env = "APPROVE_RESPONSES"

var (
	operationIDRE = regexp.MustCompile(`^[a-z][A-Za-z0-9]*$`)
	// caseRE is lower case only: two cases differing in case alone would be one file on macOS.
	caseRE = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

	// written maps each file approved in this run (one test binary, so one package) to its rendering.
	written   = map[string][]byte{}
	writtenMu sync.Mutex
)

// File is the format of an approved file; approvedcheck.go in apps/api/tools/harness decodes the same shape.
// Headers holds one value per header name: several values are joined with ", " (the HTTP list form), which is
// wrong for Set-Cookie, whose values may contain commas. Body is the parsed JSON body, or the raw body as a
// JSON string when the response is not JSON; it is absent for an empty body.
type File struct {
	Headers map[string]string `json:"headers"`
	Body    json.RawMessage   `json:"body,omitempty"`
}

// Option changes what Verify pins.
type Option func(*options)

type options struct{ caseName string }

// Case names one of several responses an operation returns with the same status (two different 404s, say):
// the file becomes <operationId>.<status>.<name>.json. name is lower-case kebab-case.
func Case(name string) Option { return func(o *options) { o.caseName = name } }

// Verify compares rec with the approved file for operationID and rec's status code. The response must be
// deterministic: send a fixed X-Request-Id and fake whatever reads the clock or the network.
func Verify(t testing.TB, operationID string, rec *httptest.ResponseRecorder, opts ...Option) {
	t.Helper()
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	if !operationIDRE.MatchString(operationID) {
		t.Fatalf("approvaltest.Verify: %q is not a camelCase operationId from api/openapi.yaml", operationID)
	}
	name := fmt.Sprintf("%s.%d", operationID, rec.Code)
	if o.caseName != "" {
		if !caseRE.MatchString(o.caseName) {
			t.Fatalf("approvaltest.Verify: case %q is not lower-case kebab-case", o.caseName)
		}
		name += "." + o.caseName
	}
	name += ".json"
	file := path.Join(Dir, name)
	approve, err := approveMode()
	if err != nil {
		t.Fatalf("approvaltest.Verify: %v", err)
	}
	got, err := render(rec)
	if err != nil {
		t.Fatalf("approvaltest.Verify %s: %v", file, err)
	}

	if approve {
		if err := claim(file, got); err != nil {
			t.Fatalf("approvaltest.Verify %v", err)
		}
		if err := write(name, got); err != nil {
			t.Fatalf("approvaltest.Verify %s: %v", file, err)
		}
		return
	}
	want, err := read(name)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		t.Errorf("%s: no approved response for %s %d; run `make approve` and review the new file [api-responses-approved]", file, operationID, rec.Code)
	case err != nil:
		t.Fatalf("approvaltest.Verify %s: %v", file, err)
	case !bytes.Equal(want, got):
		t.Errorf("%s: the response differs from the approved one [api-responses-approved]\n--- approved\n%s+++ received\n%s", file, want, got)
	}
}

// approveMode reports whether Env asks to write the approved files: 1 writes, unset or 0 compares.
func approveMode() (bool, error) {
	switch v := os.Getenv(Env); v {
	case "", "0":
		return false, nil
	case "1":
		return true, nil
	default:
		return false, fmt.Errorf("%s=%q: want 1 (write the approved files) or 0 or unset (compare with them)", Env, v)
	}
}

// claim records got as the rendering of file in this run. A different rendering for a file already written
// means two responses share a name, and the second would silently replace the first.
func claim(file string, got []byte) error {
	abs, err := filepath.Abs(file)
	if err != nil {
		return err
	}
	writtenMu.Lock()
	defer writtenMu.Unlock()
	if prev, ok := written[abs]; ok && !bytes.Equal(prev, got) {
		return fmt.Errorf("%s: two different responses in one run; give each its own approvaltest.Case(\"…\")", file)
	}
	written[abs] = got
	return nil
}

// read returns the approved file name in Dir; an error wrapping fs.ErrNotExist when it or Dir is missing.
func read(name string) ([]byte, error) {
	root, err := os.OpenRoot(Dir)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	return root.ReadFile(name)
}

// write replaces the approved file name in Dir, creating Dir first.
func write(name string, b []byte) error {
	if err := os.MkdirAll(Dir, 0o750); err != nil {
		return err
	}
	root, err := os.OpenRoot(Dir)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	return root.WriteFile(name, b, 0o600)
}

// render is the approved-file form of rec (File): indented, keys sorted, one value per header.
func render(rec *httptest.ResponseRecorder) ([]byte, error) {
	f := File{Headers: map[string]string{}}
	for k, v := range rec.Header() {
		f.Headers[k] = strings.Join(v, ", ")
	}
	if body := rec.Body.Bytes(); len(body) > 0 {
		var err error
		if isJSON(rec.Header().Get("Content-Type")) {
			f.Body, err = normalJSON(body)
		} else {
			f.Body, err = json.Marshal(string(body))
		}
		if err != nil {
			return nil, err
		}
	}
	out, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

// normalJSON re-encodes the single JSON value in body with sorted keys and its numbers as sent.
func normalJSON(body []byte) ([]byte, error) {
	var v any
	d := json.NewDecoder(bytes.NewReader(body))
	d.UseNumber()
	if err := d.Decode(&v); err != nil {
		return nil, fmt.Errorf("JSON body: %w", err)
	}
	if err := d.Decode(new(any)); !errors.Is(err, io.EOF) {
		return nil, errors.New("JSON body: more than one value")
	}
	return json.Marshal(v)
}

// isJSON reports whether contentType is application/json or a +json type such as application/problem+json.
func isJSON(contentType string) bool {
	mt, _, err := mime.ParseMediaType(contentType)
	return err == nil && (mt == "application/json" || strings.HasSuffix(mt, "+json"))
}
