package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestSpecRefs(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, doc string
		want      string // stdout; "" means no finding
	}{
		{"internal refs pass", "a:\n  $ref: '#/components/schemas/A'\nb: { \"$ref\": \"#/x\" }\n", ""},
		{"plain external ref", "a:\n  $ref: https://example.com/x.yaml\n", `f.yaml:2: $ref "https://example.com/x.yaml" points outside this file (want '#/...')` + "\n"},
		{"value on the next line", "a:\n  $ref:\n    other.yaml\n", `f.yaml:2: $ref "other.yaml" points outside this file (want '#/...')` + "\n"},
		{"space before the colon", "a:\n  $ref : other.yaml\n", `f.yaml:2: $ref "other.yaml" points outside this file (want '#/...')` + "\n"},
		{"quoted key", "a:\n  \"$ref\": other.yaml\n", `f.yaml:2: $ref "other.yaml" points outside this file (want '#/...')` + "\n"},
		{"JSON", "{\"a\": {\n  \"$ref\": \"other.json#/x\"}}\n", `f.yaml:2: $ref "other.json#/x" points outside this file (want '#/...')` + "\n"},
		{"key through an alias", "k: &k $ref\na:\n  *k : other.yaml\n", `f.yaml:3: $ref "other.yaml" points outside this file (want '#/...')` + "\n"},
		{"value through an alias", "v: &v other.yaml\na:\n  $ref: *v\n", `f.yaml:3: $ref "other.yaml" points outside this file (want '#/...')` + "\n"},
		{"not a string", "a:\n  $ref: [x]\n", "f.yaml:2: $ref (not a string) points outside this file (want '#/...')\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "f.yaml"), []byte(tt.doc), 0o600); err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			w := testWorld(t, &out)
			w.cwd = dir // a relative path names a file in the working directory
			bad, err := specRefs(w, []string{"f.yaml"})
			if err != nil || bad != (tt.want != "") || out.String() != tt.want {
				t.Errorf("bad=%v err=%v, output:\n%s\nwant:\n%s", bad, err, out.String(), tt.want)
			}
		})
	}
}

func TestSpecRefsUnparsable(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	f := filepath.Join(dir, "f.yaml")
	if err := os.WriteFile(f, []byte("openapi: [\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := specRefs(testWorld(t, io.Discard), []string{f}); err == nil {
		t.Error("want an error for a document that does not parse")
	}
	if _, err := specRefs(testWorld(t, io.Discard), []string{filepath.Join(dir, "missing.yaml")}); err == nil {
		t.Error("want an error for a missing file")
	}
}
