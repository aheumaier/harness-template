package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// renameRun runs harness rename with args and returns the exit code and the combined output.
func renameRun(t *testing.T, args ...string) (int, string) {
	t.Helper()
	var out bytes.Buffer
	code := run(append([]string{"rename"}, args...), nil, &out, &out)
	return code, out.String()
}

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
		if got := r.Replace(tc.in); got != tc.want {
			t.Errorf("replace(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	// A new module path that contains the old name is not renamed a second time.
	r = newRenamer("shop", "github.com/acme/harness-template", "@acme")
	if got := r.Replace("module github.com/aheumaier/harness-template/apps/api"); got != "module github.com/acme/harness-template/apps/api" {
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
	if err := os.WriteFile(filepath.Join(root, "run.sh"), []byte("# harness-template\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("package.json", filepath.Join(root, "link.json")); err != nil {
		t.Fatal(err)
	}
	gitT(t, root, "add", "-A")
	writeLines(t, root, "wip.md", "harness-template") // untracked after the add: work in progress the rename must not touch
	read := func(p string) string {
		b, err := os.ReadFile(filepath.Join(root, p))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	args := []string{root, "acme-shop", "github.com/acme/shop", "@acme"}

	code, out := renameRun(t, args...)
	if code != 0 {
		t.Fatalf("dry run exit %d: %s", code, out)
	}
	if !strings.Contains(out, "package.json: 1") || !strings.Contains(out, "dry run") {
		t.Errorf("dry run output: %s", out)
	}
	if got := read("package.json"); !strings.Contains(got, "harness-template") {
		t.Errorf("dry run wrote package.json: %s", got)
	}

	if code, out := renameRun(t, append([]string{"-write"}, args...)...); code != 0 {
		t.Fatalf("write exit %d: %s", code, out)
	}
	for p, want := range map[string]string{
		"apps/api/go.mod":                    "module github.com/acme/shop/apps/api\n",
		"package.json":                       "{\"name\": \"acme-shop\"}\n",
		".github/CODEOWNERS":                 "/.github/   @acme\n",
		"docs/adr/0002-go-service-layout.md": "module github.com/aheumaier/harness-template/apps/api\n", // history
		"docs/exec-plans/completed/0001.md":  "Owner: @aheumaier\n",                                     // history
		"README.md":                          "# harness-template\n",                                    // describes the template
		"logo.png":                           "harness-template\x00",                                    // binary
		"run.sh":                             "# acme-shop\n",
		"wip.md":                             "harness-template\n", // untracked
	} {
		if got := read(p); got != want {
			t.Errorf("%s = %q, want %q", p, got, want)
		}
	}

	if info, err := os.Stat(filepath.Join(root, "run.sh")); err != nil || info.Mode().Perm() != 0o755 {
		t.Errorf("run.sh mode: %v %v, want 0755", info.Mode(), err)
	}
	if target, err := os.Readlink(filepath.Join(root, "link.json")); err != nil || target != "package.json" {
		t.Errorf("link.json: %q %v, want a symlink to package.json", target, err)
	}

	if code, out := renameRun(t, append([]string{"-write"}, args...)...); code != 0 || !strings.Contains(out, "0 files") {
		t.Errorf("second run not a no-op: exit %d: %s", code, out)
	}
}

func TestRenameLeftovers(t *testing.T) {
	root := linkRepo(t, map[string][]string{
		"apps/api/go.mod":      {"module github.com/aheumaier/harness-template/apps/api"},
		"docs/remote.md":       {"git clone git@github.com:aheumaier/harness-template.git", "fine"},
		"harness-template.txt": {"nothing"},
	})
	gitT(t, root, "add", "-A")
	code, out := renameRun(t, root, "acme-shop", "github.com/acme/shop", "@acme")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	for _, want := range []string{"rename: 2 places still hold", "  docs/remote.md:1\n", "  harness-template.txt: path\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q: %s", want, out)
		}
	}
	if strings.Contains(out, "  apps/api/go.mod:1") {
		t.Errorf("renamed module reported as leftover: %s", out)
	}

	// A kept module path holds the old name on purpose; only the renamed parts are checked.
	code, out = renameRun(t, root, "acme-shop", templateModule, "@acme")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	if strings.Contains(out, "  apps/api/go.mod:1") || !strings.Contains(out, "  harness-template.txt: path") {
		t.Errorf("kept module reported as leftover: %s", out)
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
		{root, "acme-shop", "acme/shop", "@acme"},                                         // no dot in the host
		{root, "acme-shop", "github.com/-acme/shop", "@acme"},                             // element starts with -
		{root, "acme-shop", "github.com/acme/shop", "@-acme"},                             // login starts with -
		{root, "acme-shop", "github.com/acme/shop", "@" + strings.Repeat("a", 40)},        // login over 39
		// A value holding a template token would be renamed again on a second run.
		{root, "acme-shop", "github.com/acme/shop", "@aheumaier/platform"},
		{root, "harness-template-x", "github.com/acme/shop", "@acme"},
		{root, "shop", "github.com/acme/harness-template", "@acme"},
		{root, "acme-shop", "github.com/aheumaier/harness-template/v2", "@acme"},
	} {
		if code, out := renameRun(t, args...); code != 2 {
			t.Errorf("rename %v: exit %d, want 2: %s", args, code, out)
		}
	}
}
