package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenamer(t *testing.T) {
	r := newRenamer("acme-shop", "github.com/acme/shop", "@acme/platform")
	for _, tc := range []struct{ in, want string }{
		{"module github.com/aheumaier/harness-template/apps/api", "module github.com/acme/shop/apps/api"},
		{`"name": "@harness-template/api-client"`, `"name": "@acme-shop/api-client"`},
		{"# harness-template Constitution", "# acme-shop Constitution"},
		{"url: https://github.com/aheumaier/harness-template", "url: https://github.com/acme/shop"},
		{"/api/openapi.yaml   @aheumaier", "/api/openapi.yaml   @acme/platform"},
		{"nothing to see", "nothing to see"},
	} {
		if got, _ := r.replace(tc.in); got != tc.want {
			t.Errorf("replace(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	// A new module path that contains the old name is not renamed a second time.
	r = newRenamer("shop", "github.com/acme/harness-template", "@acme")
	if got, _ := r.replace("module github.com/aheumaier/harness-template/apps/api"); got != "module github.com/acme/harness-template/apps/api" {
		t.Errorf("single pass: got %q", got)
	}
}

func TestRenameRepo(t *testing.T) {
	root := linkRepo(t, map[string][]string{
		"apps/api/go.mod":                    {"module github.com/aheumaier/harness-template/apps/api"},
		"package.json":                       {`{"name": "harness-template"}`},
		".github/CODEOWNERS":                 {"/.github/   @aheumaier"},
		"docs/adr/0002-go-service-layout.md": {"module github.com/aheumaier/harness-template/apps/api"},
		"docs/exec-plans/completed/0001.md":  {"Owner: @aheumaier"},
		"README.md":                          {"# harness-template"},
	})
	if err := os.WriteFile(filepath.Join(root, "logo.png"), []byte("harness-template\x00"), 0o644); err != nil {
		t.Fatal(err)
	}
	read := func(p string) string {
		b, err := os.ReadFile(filepath.Join(root, p))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	args := []string{root, "acme-shop", "github.com/acme/shop", "@acme"}

	var out bytes.Buffer
	if code := run(append([]string{"rename"}, args...), nil, &out, &out); code != 0 {
		t.Fatalf("dry run exit %d: %s", code, out.String())
	}
	if !strings.Contains(out.String(), "package.json: 1") || !strings.Contains(out.String(), "dry run") {
		t.Errorf("dry run output: %s", out.String())
	}
	if got := read("package.json"); !strings.Contains(got, "harness-template") {
		t.Errorf("dry run wrote package.json: %s", got)
	}

	out.Reset()
	if code := run(append([]string{"rename", "-write"}, args...), nil, &out, &out); code != 0 {
		t.Fatalf("write exit %d: %s", code, out.String())
	}
	for p, want := range map[string]string{
		"apps/api/go.mod":                    "module github.com/acme/shop/apps/api\n",
		"package.json":                       "{\"name\": \"acme-shop\"}\n",
		".github/CODEOWNERS":                 "/.github/   @acme\n",
		"docs/adr/0002-go-service-layout.md": "module github.com/aheumaier/harness-template/apps/api\n", // history
		"docs/exec-plans/completed/0001.md":  "Owner: @aheumaier\n",                                     // history
		"README.md":                          "# harness-template\n",                                    // describes the template
		"logo.png":                           "harness-template\x00",                                    // binary
	} {
		if got := read(p); got != want {
			t.Errorf("%s = %q, want %q", p, got, want)
		}
	}

	out.Reset()
	if code := run(append([]string{"rename", "-write"}, args...), nil, &out, &out); code != 0 || !strings.Contains(out.String(), "0 files") {
		t.Errorf("second run not a no-op: exit %d: %s", code, out.String())
	}
}

func TestRenameArgs(t *testing.T) {
	root := t.TempDir()
	for _, args := range [][]string{
		{root, "Acme", "github.com/acme/shop", "@acme"},                                   // name not lowercase
		{root, "acme-shop", "github.com/acme/shop/", "@acme"},                             // trailing slash
		{root, "acme-shop", "acme shop", "@acme"},                                         // space in module
		{root, "acme-shop", "github.com/acme/shop", "acme"},                               // owner without @
		{root, "acme-shop", "github.com/acme/shop"},                                       // owner missing
		{root, "harness-template", "github.com/aheumaier/harness-template", "@aheumaier"}, // nothing changes
	} {
		var out bytes.Buffer
		if code := run(append([]string{"rename"}, args...), nil, &out, &out); code != 2 {
			t.Errorf("rename %v: exit %d, want 2: %s", args, code, out.String())
		}
	}
}
