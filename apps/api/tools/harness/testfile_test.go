package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestTestFile(t *testing.T) {
	root := scenarioRepo(t, map[string][]string{"apps/api/internal/x/model/x.go": {"package model"}})
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "apps/web/src"), 0o755); err != nil {
		t.Fatal(err)
	}
	for link, target := range map[string]string{
		"out":                                    outside,                                               // a directory link out of the repo
		"apps/api/internal/x/model/fake_test.go": filepath.Join(root, "apps/api/internal/x/model/x.go"), // a test name on a non-test file
		"apps/api/internal/x/model/gone_test.go": filepath.Join(outside, "missing_test.go"),             // dangling
	} {
		if err := os.Symlink(target, filepath.Join(root, link)); err != nil {
			t.Fatal(err)
		}
	}
	tests := []struct {
		path string
		want string // "" allowed, else a substring of the error
	}{
		{"apps/api/internal/x/model/x_test.go", ""},
		{"apps/api/internal/y/service/new_test.go", ""}, // directories not created yet
		{"apps/web/src/a.test.tsx", ""},
		{"packages/domain/src/a.test.ts", ""},
		{"apps/web/src/a.test.mjs", ""},
		{"apps/web/src/a.spec.ts", ""}, // testFileRE's set; the agent's prompt forbids it in apps/web
		{"apps/web/e2e/a.e2e.ts", ""},
		{"apps/api/internal/x/model/x.go", "x/model/x.go is not a test file (" + testFileForms + ")"},
		{"apps/api/internal/x/store/q_test.go", "is generated code (GENERATED_GLOBS: *.gen.go, apps/api/internal/*/store/*"},
		{"packages/api-client/src/gen/a.test.ts", "is in the generated API client (API_CLIENT_DIR: packages/api-client)"},
		{"apps/api/tools/harness/a_test.go", "is a harness file (apps/api/tools/harness/* in scripts/harness/protected-paths.txt)"},
		{"apps/web/node_modules/x/a.test.ts", "is under node_modules/ or fixtures/"},
		{"tools/a_test.go", "is not product code (PRODUCT_GLOBS or WEB_GLOBS: apps/*, api/*, packages/*)"},
		{"apps/api/internal/x/model/x_test.go.orig", "is not a test file"},
		{"packages/domain/src/test.ts", "is not a test file"},
		{"apps/api/internal/x/model/fake_test.go", "x/model/x.go is not a test file"},
		{"apps/api/internal/x/model/gone_test.go", "cannot resolve"},
		{"out/x_test.go", "outside the repository"},
		{"../x_test.go", "has a `..` part"},
		{"apps/../../x_test.go", "has a `..` part"},
		{"apps/api/internal/x/model/../model/y_test.go", "has a `..` part"}, // inside, but `..` is refused as such
		{"out/../apps/api/internal/x/model/z_test.go", "has a `..` part"},   // Abs: inside; the OS: beside outside
		// Case variants of excluded paths: on a case-insensitive disk they are the excluded files themselves.
		{"apps/api/tools/Harness/zz_test.go", "is a harness file (apps/api/tools/harness/*"},
		{"apps/api/tools/HARNESS/testfile_test.go", "is a harness file"},
		{"apps/api/internal/x/Store/q_test.go", "is generated code"},
		{"packages/API-client/src/gen/a.test.ts", "is in the generated API client"},
		{"apps/web/Node_modules/x/a.test.ts", "is under node_modules/ or fixtures/"},
		{"apps/api/internal/x/Fixtures/a_test.go", "is under node_modules/ or fixtures/"},
		{".", "outside the repository"},
		{filepath.Join(outside, "x_test.go"), "outside the repository"},
		{"", "no file path"},
	}
	for _, tt := range tests {
		p := tt.path
		if p != "" && !filepath.IsAbs(p) {
			p = root + "/" + p
		}
		err := testFile(root, p)
		switch {
		case tt.want == "" && err != nil:
			t.Errorf("%s: %v, want allowed", tt.path, err)
		case tt.want != "" && (err == nil || !strings.Contains(err.Error(), tt.want)):
			t.Errorf("%s: %v, want an error containing %q", tt.path, err, tt.want)
		}
	}
}

// TestProductTestFileVectors runs every row of testdata/test-files.txt through the test-writer's write guard
// (testFile) and through the files specs-tested reads (testedIDs), so the agent may write exactly the test files
// that specs-tested counts (ADR 0039). The rows are template paths, so it uses the template's layout and the
// template protected paths the rows meet; only-test-files.bats runs the same rows through the hook with the
// repo's own layout.env and protected-paths.txt.
func TestProductTestFileVectors(t *testing.T) {
	protected := append(slices.Clone(protectedList), "*playwright.config.* config", "scripts/test/*",
		"apps/api/tools/analyzers/* config", "apps/api/internal/platform/approvaltest/* config")
	var hinges []string // the third fields a row may carry: entries the template has
	for _, l := range templateLayout {
		if globs, ok := strings.CutPrefix(l, "GENERATED_GLOBS="); ok {
			for _, g := range strings.Split(globs, ":") {
				hinges = append(hinges, "generated:"+g)
			}
		}
	}
	for _, l := range protected {
		if p, _, _ := strings.Cut(l, " "); !strings.HasPrefix(p, "#") {
			hinges = append(hinges, "protected:"+p)
		}
	}
	b, err := os.ReadFile("testdata/test-files.txt")
	if err != nil {
		t.Fatal(err)
	}
	type row struct {
		path, id string
		allow    bool
	}
	var rows []row
	files := map[string][]string{"scripts/harness/protected-paths.txt": protected}
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Fields(line)
		if len(f) == 0 || strings.HasPrefix(line, "#") {
			continue
		}
		switch {
		case len(f) == 2 && (f[1] == "allow" || f[1] == "reject"):
		case len(f) == 3 && f[1] == "reject" && slices.Contains(hinges, f[2]):
		default:
			t.Fatalf("%q: want <path> allow|reject, or <path> reject <entry of the template it hinges on: %s>",
				line, strings.Join(hinges, ", "))
		}
		p := f[0]
		r := row{p, fmt.Sprintf("001/US1-AS%d", len(rows)+1), f[1] == "allow"}
		files[p] = []string{"// spec: " + r.id} // each file names its own id, so testedIDs shows whether it read it
		rows = append(rows, r)
	}
	if len(rows) == 0 {
		t.Fatal("no vectors in testdata/test-files.txt")
	}
	root := scenarioRepo(t, files)
	pt, err := loadProductTests(root)
	if err != nil {
		t.Fatal(err)
	}
	named, err := testedIDs(root, pt)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if err := testFile(root, filepath.Join(root, r.path)); (err == nil) != r.allow {
			t.Errorf("%s: harness test-file: %v; want allow=%v", r.path, err, r.allow)
		}
		if named[r.id] != r.allow {
			t.Errorf("%s: specs-tested reads it: %v; want %v", r.path, named[r.id], r.allow)
		}
	}
}

func TestTestFileExitCodes(t *testing.T) {
	root := scenarioRepo(t, nil)
	noLayout := t.TempDir() // fails closed: without the layout no path is a product test
	for _, tt := range []struct {
		args []string
		code int
	}{
		{[]string{"test-file", root, filepath.Join(root, "apps/api/a_test.go")}, 0},
		{[]string{"test-file", root, filepath.Join(root, "apps/api/a.go")}, 1},
		{[]string{"test-file", filepath.Join(root, "missing"), filepath.Join(root, "apps/api/a_test.go")}, 1},
		{[]string{"test-file", noLayout, filepath.Join(noLayout, "apps/api/a_test.go")}, 1},
		{[]string{"test-file", root}, 2},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(tt.args, strings.NewReader(""), &stdout, &stderr); code != tt.code {
			t.Errorf("%v: exit %d, want %d (%s)", tt.args[1:], code, tt.code, stderr.String())
		}
		if stdout.Len() != 0 {
			t.Errorf("%v: printed %q on stdout", tt.args[1:], stdout.String())
		}
	}
}

// TestTestFileForms holds testFileForms, the words every reject prints, to testFileRE: each form it names is a
// test file, and the near misses are not.
func TestTestFileForms(t *testing.T) {
	forms := []string{"a_test.go"}
	for _, kind := range []string{"test", "spec", "e2e"} {
		for _, ext := range []string{"js", "ts", "jsx", "tsx", "mjs", "cjs", "mts", "cts"} {
			if !strings.Contains(testFileForms, "*."+kind+".*") || !strings.Contains(testFileForms, "."+ext) {
				t.Errorf("testFileForms does not name *.%s.* or .%s", kind, ext)
			}
			forms = append(forms, "a."+kind+"."+ext)
		}
	}
	for _, f := range forms {
		if !testFileRE.MatchString(f) {
			t.Errorf("%s: testFileForms names it, testFileRE does not match it", f)
		}
	}
	for _, f := range []string{"a.go", "a_test.go.orig", "test.ts", "a.test.go", "a.tests.ts", "a.test.snap", "a.spec.json"} {
		if testFileRE.MatchString(f) {
			t.Errorf("%s: testFileRE matches a near miss", f)
		}
	}
	if !strings.Contains(testFileForms, "*Tests.swift under a *Tests directory, not under Sources/") {
		t.Errorf("testFileForms does not name the Swift form")
	}
	for _, f := range []string{"Tests/AppCoreTests/PayTests.swift", "apps/ios/AppUITests/PayUITests.swift", "apps/ios/AppTests/Flows/PayTests.swift"} {
		if !swiftTestRE.MatchString(f) {
			t.Errorf("%s: testFileForms names it, swiftTestRE does not match it", f)
		}
	}
	for _, f := range []string{"PayTests.swift", "apps/ios/Tests/PayTests.swift", "apps/ios/Sources/AppCore/PayTests.swift", "apps/ios/Tests/AppCoreTests/Helpers.swift", "apps/ios/AppTests/PayTests.swift.orig", "apps/ios/Tests/PayTest.swift", "apps/ios/MyTests.swift"} {
		if swiftTestRE.MatchString(f) {
			t.Errorf("%s: swiftTestRE matches a near miss", f)
		}
	}
	pt := productTests{include: []string{"apps/*"}}
	for _, f := range []string{"apps/ios/Sources/AppCore/MockTests/PayTests.swift", "apps/ios/Sources/AppCoreTests/PayTests.swift"} {
		if pt.why(f) == "" {
			t.Errorf("%s: a *Tests folder under Sources/ ships in the app, but why allows it", f)
		}
	}
}

// TestLoadProductTestsFailsClosed: without harness entries, or with a pattern pathspecMatch cannot read as git
// does, the predicate refuses to decide rather than allow too much.
func TestLoadProductTestsFailsClosed(t *testing.T) {
	for name, files := range map[string]map[string][]string{
		"comments only": {"scripts/harness/protected-paths.txt": {"# harness files", "", "  # none"}},
		"protected ?":   {"scripts/harness/protected-paths.txt": {"apps/api/tools/harnes? tool"}},
		"protected [":   {"scripts/harness/protected-paths.txt": {"apps/api/tools/[h]arness/* tool"}},
		"protected \\":  {"scripts/harness/protected-paths.txt": {`apps/api/tools/harness/\* tool`}},
		"layout [":      {layoutFile: replaced(t, templateLayout, "GENERATED_GLOBS=", "GENERATED_GLOBS=[a]*.gen.go:")},
	} {
		root := scenarioRepo(t, files)
		if _, err := loadProductTests(root); err == nil {
			t.Errorf("%s: loaded, want an error", name)
		}
		if err := testFile(root, filepath.Join(root, "apps/api/internal/x/model/x_test.go")); err == nil {
			t.Errorf("%s: harness test-file allowed a path, want it to fail closed", name)
		}
	}
}

// replaced returns lines with the one line starting with prefix rewritten to start with repl instead.
func replaced(t *testing.T, lines []string, prefix, repl string) []string {
	t.Helper()
	out := slices.Clone(lines)
	for i, l := range out {
		if strings.HasPrefix(l, prefix) {
			out[i] = repl + strings.TrimPrefix(l, prefix)
			return out
		}
	}
	t.Fatalf("no line starts with %q", prefix)
	return nil
}
