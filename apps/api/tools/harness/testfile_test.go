package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTestFile(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	for _, d := range []string{"apps/api/internal/x/model", "apps/web/src"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "apps/api/internal/x/model/x.go"), nil, 0o644); err != nil {
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
		{"apps/api/internal/x/model/x.go", "is not a test file"},
		{"apps/api/internal/x/model/x_test.go.orig", "is not a test file"},
		{"packages/domain/src/test.ts", "is not a test file"},
		{"apps/api/internal/x/model/fake_test.go", "x/model/x.go is not a test file"},
		{"apps/api/internal/x/model/gone_test.go", "cannot resolve"},
		{"out/x_test.go", "outside the repository"},
		{"../x_test.go", "outside the repository"},
		{"apps/../../x_test.go", "outside the repository"},
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

func TestTestFileExitCodes(t *testing.T) {
	root := t.TempDir()
	for _, tt := range []struct {
		args []string
		code int
	}{
		{[]string{"test-file", root, filepath.Join(root, "a_test.go")}, 0},
		{[]string{"test-file", root, filepath.Join(root, "a.go")}, 1},
		{[]string{"test-file", filepath.Join(root, "missing"), filepath.Join(root, "a_test.go")}, 1},
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
