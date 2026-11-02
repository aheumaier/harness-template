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

// The rename points template sync at the template (ADR 0026): written after the rewrite, never renamed, never
// a leftover, and a second run changes nothing.
func TestRenameSetsUpstream(t *testing.T) {
	root := linkRepo(t, map[string][]string{
		upstreamFile:   {"# harness-template sync", "GO_TOOLCHAIN=go1", "HARNESS_UPSTREAM="},
		"package.json": {`{"name": "harness-template"}`},
		layoutFile:     templateLayout,
	})
	gitT(t, root, "add", "-A")
	args := []string{"-write", root, "acme-shop", "github.com/acme/shop", "@acme"}
	code, out := renameRun(t, args...)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	want := "# acme-shop sync\nGO_TOOLCHAIN=go1\nHARNESS_UPSTREAM=https://github.com/aheumaier/harness-template\n"
	if b, err := os.ReadFile(filepath.Join(root, upstreamFile)); err != nil || string(b) != want {
		t.Errorf("%s = %q, %v; want %q", upstreamFile, b, err, want)
	}
	if strings.Contains(out, "still hold") {
		t.Errorf("upstream line reported as a leftover: %s", out)
	}
	if code, out := renameRun(t, args...); code != 0 || !strings.Contains(out, "0 files") {
		t.Errorf("second run not a no-op: exit %d: %s", code, out)
	}
	for in, want := range map[string]string{
		"A=1\n":                      "A=1\n" + upstreamLine + "\n", // a missing line is appended
		"A=1":                        "A=1\n" + upstreamLine + "\n",
		"HARNESS_UPSTREAM=x\nB=2\n":  upstreamLine + "\nB=2\n",
		"HARNESS_UPSTREAM=\n# end\n": upstreamLine + "\n# end\n",
	} {
		if got := setUpstream(in); got != want {
			t.Errorf("setUpstream(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRenameRepo(t *testing.T) {
	root := linkRepo(t, map[string][]string{
		"apps/api/go.mod":                    {"module github.com/aheumaier/harness-template/apps/api"},
		"apps/api/tools/harness/go.mod":      {"module " + templateHarnessModule, "", "go 1.27"},
		"package.json":                       {`{"name": "harness-template"}`},
		".github/CODEOWNERS":                 {"/.github/   @aheumaier"},
		layoutFile:                           templateLayout,
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
	args := []string{root, "acme-shop", "github.com/acme/shop", "@acme"}

	code, out := renameRun(t, args...)
	if code != 0 {
		t.Fatalf("dry run exit %d: %s", code, out)
	}
	if !strings.Contains(out, "package.json: 1") || !strings.Contains(out, "dry run") ||
		!strings.Contains(out, "rename: harness tool module: github.com/acme/shop/apps/api/tools/harness (apps/api/tools/harness/go.mod)\n") {
		t.Errorf("dry run output: %s", out)
	}
	if got := renameFile(t, root, "package.json"); !strings.Contains(got, "harness-template") {
		t.Errorf("dry run wrote package.json: %s", got)
	}

	if code, out := renameRun(t, append([]string{"-write"}, args...)...); code != 0 {
		t.Fatalf("write exit %d: %s", code, out)
	}
	for p, want := range map[string]string{
		"apps/api/go.mod":                    "module github.com/acme/shop/apps/api\n",
		"apps/api/tools/harness/go.mod":      "module github.com/acme/shop/apps/api/tools/harness\n\ngo 1.27\n",
		"package.json":                       "{\"name\": \"acme-shop\"}\n",
		".github/CODEOWNERS":                 "/.github/   @acme\n",
		"docs/adr/0002-go-service-layout.md": "module github.com/aheumaier/harness-template/apps/api\n", // history
		"docs/exec-plans/completed/0001.md":  "Owner: @aheumaier\n",                                     // history
		"README.md":                          "# harness-template\n",                                    // describes the template
		"logo.png":                           "harness-template\x00",                                    // binary
		"run.sh":                             "# acme-shop\n",
		"wip.md":                             "harness-template\n", // untracked
	} {
		if got := renameFile(t, root, p); got != want {
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

// templateHarnessModule is the harness tool's module path in the template (apps/api/tools/harness/go.mod).
const templateHarnessModule = templateModule + "/apps/api/tools/harness"

// renameFile reads root/p after a rename.
func renameFile(t *testing.T, root, p string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, p))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// A bare Go module (no dot in its first element, as `go mod init example/hello` writes) is a valid MODULE
// (exec plan 0003, P4.8). The harness tool's module follows API_DIR; the template URL is kept, since
// https://<bare module> is no URL, and listed for a hand fix.
func TestRenameBareModule(t *testing.T) {
	for _, tc := range []struct {
		name, module, apiDir, apiGoMod, apiBefore, apiAfter, harnessGoMod, harnessModule string
	}{
		{"template layout", "emergencies", "apps/api", "apps/api/go.mod", "module " + templateModule + "/apps/api",
			"module emergencies/apps/api\n", "apps/api/tools/harness/go.mod", "emergencies/apps/api/tools/harness"},
		// The product's own root module holds no template token and stays as it is.
		{"root module", "example/hello", ".", "go.mod", "module emergencies",
			"module emergencies\n", "tools/harness/go.mod", "example/hello/tools/harness"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := linkRepo(t, map[string][]string{
				layoutFile:         withLayout("API_DIR", tc.apiDir),
				tc.apiGoMod:        {tc.apiBefore},
				tc.harnessGoMod:    {"module " + templateHarnessModule, "", "go 1.27"},
				"api/openapi.yaml": {"servers:", "  - url: https://" + templateModule},
				"package.json":     {`{"name": "harness-template"}`},
			})
			gitT(t, root, "add", "-A")
			line := "rename: harness tool module: " + tc.harnessModule + " (" + tc.harnessGoMod + ")\n"
			args := []string{root, "acme-shop", tc.module, "@acme"}
			if code, out := renameRun(t, args...); code != 0 || !strings.Contains(out, line) {
				t.Fatalf("dry run: exit %d, want 0 and %q: %s", code, line, out)
			}
			code, out := renameRun(t, append([]string{"-write"}, args...)...)
			if code != 0 || !strings.Contains(out, line) || !strings.Contains(out, "  api/openapi.yaml:2\n") {
				t.Fatalf("write: exit %d, want 0, %q and the URL as a leftover: %s", code, line, out)
			}
			for p, want := range map[string]string{
				tc.apiGoMod:        tc.apiAfter,
				tc.harnessGoMod:    "module " + tc.harnessModule + "\n\ngo 1.27\n",
				"api/openapi.yaml": "servers:\n  - url: https://" + templateModule + "\n",
				"package.json":     "{\"name\": \"acme-shop\"}\n",
			} {
				if got := renameFile(t, root, p); got != want {
					t.Errorf("%s = %q, want %q", p, got, want)
				}
			}
			if code, out := renameRun(t, append([]string{"-write"}, args...)...); code != 0 || !strings.Contains(out, "0 files") {
				t.Errorf("second run not a no-op: exit %d: %s", code, out)
			}
		})
	}
}

// MODULE=skip renames only the name and the owner: Go module paths, the harness tool's included, stay as
// they are (exec plan 0003, P4.8).
func TestRenameModuleSkip(t *testing.T) {
	root := linkRepo(t, map[string][]string{
		layoutFile:             withLayout("API_DIR", "."),
		"go.mod":               {"module emergencies"},
		"tools/harness/go.mod": {"module " + templateHarnessModule},
		// The tool's own template constants stay, wherever API_DIR puts it.
		"tools/harness/rename.go": {`templateName = "harness-template"`},
		"package.json":            {`{"name": "harness-template"}`},
		".github/CODEOWNERS":      {"/.github/   @aheumaier"},
		// The owner's other forms are leftovers; the template's URL holds the kept module, so it is none.
		"docs/clone.md": {"git clone git@github.com:aheumaier/x.git", "https://github.com/aheumaier/harness-template"},
	})
	gitT(t, root, "add", "-A")
	args := []string{"-write", root, "acme-shop", "skip", "@acme"}
	code, out := renameRun(t, args...)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	for _, want := range []string{
		"rename: harness tool module: " + templateHarnessModule + " (tools/harness/go.mod)\n",
		"rename: 1 places still hold harness-template or aheumaier; rename them by hand:\n  docs/clone.md:1\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q: %s", want, out)
		}
	}
	for p, want := range map[string]string{
		"go.mod":                  "module emergencies\n",
		"tools/harness/go.mod":    "module " + templateHarnessModule + "\n",
		"tools/harness/rename.go": "templateName = \"harness-template\"\n",
		"package.json":            "{\"name\": \"acme-shop\"}\n",
		".github/CODEOWNERS":      "/.github/   @acme\n",
	} {
		if got := renameFile(t, root, p); got != want {
			t.Errorf("%s = %q, want %q", p, got, want)
		}
	}
	if code, out := renameRun(t, args...); code != 0 || !strings.Contains(out, "0 files") {
		t.Errorf("second run not a no-op: exit %d: %s", code, out)
	}
	if code, out := renameRun(t, root, templateName, "skip", templateOwner); code != 2 || !strings.Contains(out, "nothing to rename") {
		t.Errorf("skip with the template's name and owner: exit %d, want 2: %s", code, out)
	}
}

// The rename reads API_DIR from layout.env and fails closed without it, before it writes anything.
func TestRenameNeedsLayout(t *testing.T) {
	for name, lines := range map[string][]string{
		"missing":       nil,
		"no API_DIR":    withLayout("API_DIR", ""),
		"empty API_DIR": append(withLayout("API_DIR", ""), "API_DIR="),
		"API_DIR twice": append(withLayout("API_DIR", "."), "API_DIR=apps/api"),
		"not KEY=value": append(withLayout("API_DIR", "."), "API_DIR apps/api"),
	} {
		t.Run(name, func(t *testing.T) {
			files := map[string][]string{"package.json": {`{"name": "harness-template"}`}}
			if lines != nil {
				files[layoutFile] = lines
			}
			root := linkRepo(t, files)
			gitT(t, root, "add", "-A")
			if code, out := renameRun(t, "-write", root, "acme-shop", "github.com/acme/shop", "@acme"); code != 2 || !strings.Contains(out, layoutFile) {
				t.Errorf("exit %d, want 2 naming %s: %s", code, layoutFile, out)
			}
			if got := renameFile(t, root, "package.json"); got != "{\"name\": \"harness-template\"}\n" {
				t.Errorf("package.json written without a valid layout: %q", got)
			}
		})
	}
}

func TestRenameLeftovers(t *testing.T) {
	root := linkRepo(t, map[string][]string{
		"apps/api/go.mod":      {"module github.com/aheumaier/harness-template/apps/api"},
		"docs/remote.md":       {"git clone git@github.com:aheumaier/harness-template.git", "fine"},
		"harness-template.txt": {"nothing"},
		layoutFile:             templateLayout,
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

	// A new module under the template owner's account keeps the owner's name on purpose.
	code, out = renameRun(t, root, "acme-shop", "github.com/aheumaier/shop", "@acme")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	if strings.Contains(out, "  apps/api/go.mod:1") || !strings.Contains(out, "  docs/remote.md:1\n") {
		t.Errorf("module under the template owner reported as leftover, or the clone URL missed: %s", out)
	}

	// A kept module path holds the old name on purpose; only the renamed parts are checked.
	code, out = renameRun(t, root, "acme-shop", templateModule, "@acme")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	if strings.Contains(out, "  apps/api/go.mod:1") || !strings.Contains(out, "  harness-template.txt: path") {
		t.Errorf("kept module reported as leftover: %s", out)
	}

	// A module that is part of a template token (harness in harness-template) hides no leftover.
	code, out = renameRun(t, root, "acme-shop", "harness", "@acme")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	if !strings.Contains(out, "  harness-template.txt: path") || !strings.Contains(out, "  docs/remote.md:1\n") {
		t.Errorf("leftover hidden by a short module: %s", out)
	}
}

// TestCheckRenameArgs tables the argument check itself, so a refused row fails for its own reason and an
// accepted row proves the check does not refuse too much.
func TestCheckRenameArgs(t *testing.T) {
	const (
		badName   = "want lowercase letters"
		badModule = "want a Go module path"
		badOwner  = "want a GitHub @user"
		again     = "so a second run would rename it again"
		nothing   = "nothing to rename"
	)
	for _, tc := range []struct{ name, module, owner, err string }{
		{"acme-shop", "github.com/acme/shop", "@acme", ""},
		{"acme-shop", "github.com/aheumaier/shop", "@acme/platform", ""}, // under the template owner's account
		{"acme-shop", "emergencies", "@acme", ""},                        // bare
		{"acme-shop", "example/hello", "@acme", ""},
		{"acme-shop", "example/hello.v2", "@acme", ""},
		{"acme-shop", "example.com", "@acme", ""}, // one dotted element
		{"acme-shop", "mycorp.internal", "@acme", ""},
		{"acme-shop", "harness", "@acme", ""},  // part of a template token; stripModule keeps leftovers visible
		{"acme-shop", skipModule, "@acme", ""}, // keeps the template's module
		{templateName, skipModule, "@acme", ""},
		{"acme-shop", skipModule, templateOwner, ""},

		{"Acme", "github.com/acme/shop", "@acme", badName},
		{"acme-shop", "github.com/acme/shop/", "@acme", badModule}, // trailing slash
		{"acme-shop", "acme/shop/", "@acme", badModule},            // bare, trailing slash
		{"acme-shop", "example.com/", "@acme", badModule},
		{"acme-shop", "acme shop", "@acme", badModule},
		{"acme-shop", "-acme", "@acme", badModule},                 // bare, starts with -
		{"acme-shop", ".acme/shop", "@acme", badModule},            // bare, starts with .
		{"acme-shop", "github.com/-acme/shop", "@acme", badModule}, // element starts with -
		{"acme-shop", "github.com/acme/shop.", "@acme", badModule}, // element ends with .
		{"acme-shop", "", "@acme", badModule},
		{"acme-shop", "github.com/acme/shop", "acme", badOwner},                        // no @
		{"acme-shop", "github.com/acme/shop", "@-acme", badOwner},                      // login starts with -
		{"acme-shop", "github.com/acme/shop", "@" + strings.Repeat("a", 40), badOwner}, // login over 39
		{templateName, templateModule, templateOwner, nothing},
		{templateName, skipModule, templateOwner, nothing},
		// A value holding a template token would be renamed again on a second run.
		{"acme-shop", "github.com/acme/shop", "@aheumaier/platform", again},
		{"harness-template-x", "github.com/acme/shop", "@acme", again},
		{"shop", "github.com/acme/harness-template", "@acme", again},
		{"acme-shop", "github.com/aheumaier/harness-template/v2", "@acme", again},
		{"acme-shop", "harness-template", "@acme", again}, // bare
	} {
		err := checkRenameArgs(tc.name, moduleArg(tc.module), tc.owner)
		switch {
		case tc.err == "" && err != nil:
			t.Errorf("%s %q %s: refused, want accepted: %v", tc.name, tc.module, tc.owner, err)
		case tc.err != "" && (err == nil || !strings.Contains(err.Error(), tc.err)):
			t.Errorf("%s %q %s: got %v, want an error with %q", tc.name, tc.module, tc.owner, err, tc.err)
		}
	}
}

// The command refuses bad arguments before it writes a file.
func TestRenameArgs(t *testing.T) {
	root := linkRepo(t, map[string][]string{"package.json": {`{"name": "harness-template"}`}, layoutFile: templateLayout})
	gitT(t, root, "add", "-A")
	for _, tc := range []struct {
		args []string
		code int
		want string
	}{
		{[]string{"-write", root, "acme-shop", "github.com/acme/shop"}, exitUsage, "usage:"}, // owner missing
		{[]string{"-write", root, "acme-shop", "acme/shop/", "@acme"}, 2, "want a Go module path"},
		{[]string{"-write", root, "acme-shop", "github.com/acme/shop", "@aheumaier/platform"}, 2, "second run"},
	} {
		if code, out := renameRun(t, tc.args...); code != tc.code || !strings.Contains(out, tc.want) {
			t.Errorf("rename %v: exit %d, want %d with %q: %s", tc.args, code, tc.code, tc.want, out)
		}
	}
	if got := renameFile(t, root, "package.json"); got != "{\"name\": \"harness-template\"}\n" {
		t.Errorf("package.json written despite refused arguments: %q", got)
	}
}
