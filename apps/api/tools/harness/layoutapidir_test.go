package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLayoutRootModule(t *testing.T) {
	t.Parallel()
	// API_DIR=. puts the module at the repo root: paths are joined without a "./" prefix.
	root := templateRepo(t, map[string][]string{
		layoutFile:                   withLayout("API_DIR", "."),
		"go.mod":                     {"module example.com/api"},
		"internal/health/api/h.go":   {"package api"},
		"tools/harness/main.go":      {"package main"},
		"apps/api/db/migrations/x":   {""},
		"apps/api/db/queries/q.sql":  {""},
		"api/openapi.yaml":           {"openapi: 3.1.0"},
		"packages/api-client/x.json": {"{}"},
		"apps/ios/project.yml":       {"name: App"},
	})
	gitT(t, root, "add", "-A")
	l, err := loadLayout(testWorld(t, io.Discard), root)
	if err != nil {
		t.Fatal(err)
	}
	if got := l.apiPath("go.mod"); got != "go.mod" {
		t.Errorf("apiPath(go.mod) = %q, want go.mod", got)
	}
	// The template's protected-paths.txt still names apps/api/tools/harness/*: README (b) step 5 re-points it.
	var out bytes.Buffer
	bad, err := layoutCheck(testWorld(t, &out), root)
	want := layoutFile + ":1: API_DIR=.: tools/harness/go.mod matches no config entry of " + protectedPathsFile + ", so the harness's own code is unprotected (add or re-point the entry `tools/harness/* config`: README, (b) step 5) [harness-layout-valid]\n"
	if err != nil || !bad || out.String() != want {
		t.Errorf("bad=%v err=%v, output:\n%s\nwant:\n%s", bad, err, out.String(), want)
	}
	writeLines(t, root, protectedPathsFile, "tools/harness/* config")
	out.Reset()
	if bad, err := layoutCheck(testWorld(t, &out), root); bad || err != nil {
		t.Errorf("re-pointed: bad=%v err=%v: %s", bad, err, out.String())
	}
}

// TestLayoutHarnessProtected: every file of the harness module under API_DIR, and its go.mod even before it
// exists, matches a config entry of protected-paths.txt, with case.
func TestLayoutHarnessProtected(t *testing.T) {
	t.Parallel()
	const msg = " matches no config entry of " + protectedPathsFile + ", so the harness's own code is unprotected (add or re-point the entry `apps/api/tools/harness/* config`: README, (b) step 5) [harness-layout-valid]"
	const at = layoutFile + ":3: API_DIR=apps/api: "
	for _, tt := range []struct {
		name      string
		protected []string
		noModule  bool   // no apps/api/tools/harness/ directory at all
		want      string // the unprotected file, "" for none
	}{
		{"the module's glob protects it", []string{"apps/api/tools/harness/* config"}, false, ""},
		{"a parent glob protects it", []string{"apps/api/tools/* config"}, false, ""},
		{"no entry for the module", []string{"scripts/harness/* config", "AGENTS.md"}, false, "apps/api/tools/harness/go.mod"},
		{"no entry and no module yet", []string{"scripts/harness/* config"}, true, "apps/api/tools/harness/go.mod"},
		{"entries for some of its files", []string{"apps/api/tools/harness/go.mod config", "apps/api/tools/harness/*.go config"},
			false, "apps/api/tools/harness/testdata/x.txt"},
		{"an entry without config is not enough", []string{"apps/api/tools/harness/*"}, false, "apps/api/tools/harness/go.mod"},
		{"another case is no match", []string{"APPS/api/tools/harness/* config"}, false, "apps/api/tools/harness/go.mod"},
		{"a commented-out entry is no entry", []string{"# apps/api/tools/harness/* config"}, false, "apps/api/tools/harness/go.mod"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := layoutTree(t, templateLayout)
			writeLines(t, root, protectedPathsFile, tt.protected...)
			if !tt.noModule {
				writeLines(t, root, "apps/api/tools/harness/main.go", "package main")
				writeLines(t, root, "apps/api/tools/harness/testdata/x.txt", "x")
			}
			gitT(t, root, "add", "-A")
			var out bytes.Buffer
			bad, err := layoutCheck(testWorld(t, &out), root)
			want := ""
			if tt.want != "" {
				want = at + tt.want + msg + "\n"
			}
			if err != nil || bad != (tt.want != "") || out.String() != want {
				t.Errorf("bad=%v err=%v, output:\n%s\nwant:\n%s", bad, err, out.String(), want)
			}
		})
	}
}

// TestLayoutHarnessSymlink: a symlink on the way to the module or inside it is refused, never resolved: git
// lists nothing behind a linked directory, so the files there would pass unseen.
func TestLayoutHarnessSymlink(t *testing.T) {
	t.Parallel()
	const at = layoutFile + ":3: API_DIR=apps/api: "
	for _, tt := range []struct {
		name, link, target, entry, want string
	}{
		{"a linked parent directory", "apps/api/tools", "../../hidden", "apps/api/tools/harness/* config",
			at + "apps/api/tools is a symlink, so protect-harness and CI never see the harness's code behind it (make it a real directory) [harness-layout-valid]\n"},
		{"the linked module under a parent glob", "apps/api/tools/harness", "../../../hidden/harness", "apps/api/tools/* config",
			at + "apps/api/tools/harness is a symlink, so protect-harness and CI never see the harness's code behind it (make it a real directory) [harness-layout-valid]\n"},
		{"a linked file in the module", "apps/api/tools/harness/zz.go", "../../../../hidden/harness/main.go", "apps/api/tools/harness/* config",
			at + "apps/api/tools/harness/zz.go is not a regular file (a symlink or a submodule), so protect-harness and CI never see what it points to (commit the file itself) [harness-layout-valid]\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := layoutTree(t, templateLayout)
			writeLines(t, root, protectedPathsFile, tt.entry)
			writeLines(t, root, "hidden/harness/main.go", "package main")
			writeLines(t, root, "apps/api/tools/harness/go.mod", "module harness")
			link := filepath.Join(root, tt.link)
			if err := os.RemoveAll(link); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(tt.target, link); err != nil {
				t.Fatal(err)
			}
			gitT(t, root, "add", "-A")
			var out bytes.Buffer
			bad, err := layoutCheck(testWorld(t, &out), root)
			if err != nil || !bad || out.String() != tt.want {
				t.Errorf("bad=%v err=%v, output:\n%s\nwant:\n%s", bad, err, out.String(), tt.want)
			}
		})
	}
}

// TestLayoutHarnessOtherCase: a path naming the module in another case (ASCII or Unicode, ſ folds to s) is
// refused; a case-insensitive disk builds it into .bin/harness, while git and CI match only the exact path.
func TestLayoutHarnessOtherCase(t *testing.T) {
	t.Parallel()
	for _, file := range []string{"apps/api/tools/Harness/zz.go", "apps/api/tools/harneſs/zz.go"} {
		t.Run(file, func(t *testing.T) {
			t.Parallel()
			root := layoutTree(t, templateLayout)
			writeLines(t, root, protectedPathsFile, "apps/api/tools/harness/* config")
			writeLines(t, root, file, "package main")
			gitT(t, root, "add", "-A")
			var out bytes.Buffer
			bad, err := layoutCheck(testWorld(t, &out), root)
			want := layoutFile + ":3: API_DIR=apps/api: " + file + " names apps/api/tools/harness in another case, so a case-insensitive disk builds it into .bin/harness while protect-harness and CI never match it (rename it to its exact path) [harness-layout-valid]\n"
			if err != nil || !bad || out.String() != want {
				t.Errorf("bad=%v err=%v, output:\n%s\nwant:\n%s", bad, err, out.String(), want)
			}
		})
	}
}

// TestLayoutHarnessNestedRepo: a repository inside the module is a directory git lists as one path, never its files.
func TestLayoutHarnessNestedRepo(t *testing.T) {
	t.Parallel()
	root := layoutTree(t, templateLayout)
	writeLines(t, root, protectedPathsFile, "apps/api/tools/harness/* config")
	writeLines(t, root, "apps/api/tools/harness/go.mod", "module harness")
	writeLines(t, root, "apps/api/tools/harness/nested/x.go", "package nested")
	gitT(t, filepath.Join(root, "apps/api/tools/harness/nested"), "init", "-q")
	var out bytes.Buffer
	bad, err := layoutCheck(testWorld(t, &out), root)
	want := layoutFile + ":3: API_DIR=apps/api: apps/api/tools/harness/nested/ is not a regular file (a symlink or a submodule), so protect-harness and CI never see what it points to (commit the file itself) [harness-layout-valid]\n"
	if err != nil || !bad || out.String() != want {
		t.Errorf("bad=%v err=%v, output:\n%s\nwant:\n%s", bad, err, out.String(), want)
	}
}

// TestLayoutAPIDirClean: the scripts use API_DIR as text, so only its clean form is accepted.
func TestLayoutAPIDirClean(t *testing.T) {
	t.Parallel()
	for _, dir := range []string{"apps/api/", "./apps/api", "apps//api", "apps/x/../api"} {
		t.Run(dir, func(t *testing.T) {
			t.Parallel()
			root := layoutTree(t, withLayout("API_DIR", dir))
			writeLines(t, root, protectedPathsFile, "apps/api/tools/harness/* config")
			var out bytes.Buffer
			bad, err := layoutCheck(testWorld(t, &out), root)
			want := layoutFile + ":3: API_DIR=" + dir + ": not a clean path inside this repo (no leading /, no ./ or .., no trailing /) [harness-layout-valid]\n"
			if err != nil || !bad || out.String() != want {
				t.Errorf("bad=%v err=%v, output:\n%s\nwant:\n%s", bad, err, out.String(), want)
			}
		})
	}
}

func TestLayoutHarnessProtectedFailsClosed(t *testing.T) {
	t.Parallel()
	root := layoutTree(t, templateLayout)
	if err := os.Remove(filepath.Join(root, protectedPathsFile)); err != nil {
		t.Fatal(err)
	}
	if _, err := layoutCheck(testWorld(t, io.Discard), root); err == nil || !strings.Contains(err.Error(), "protected-paths.txt") {
		t.Errorf("err = %v, want one naming protected-paths.txt", err)
	}
}
