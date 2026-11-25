package main

import (
	"bytes"
	"path/filepath"
	"testing"
)

// TestSpecsFrozenTestCommits: which commits are test commits, and what a bad value does.
func TestSpecsFrozenTestCommits(t *testing.T) {
	t.Parallel()
	const bodyOf = "test(cart): pay\n\n"
	repo := frozenRepo(t)
	for name, tc := range map[string]struct {
		msg, want string
		editSpec  bool
	}{
		"line before the last paragraph": {bodyOf + "Test-Writer: specs/001-pay\n\nCo-Authored-By: X <x@localhost>", "this line of test commit", false},
		"another case, no blank":         {bodyOf + "test-writer:specs/001-pay", "this line of test commit", false},
		"no line":                        {bodyOf + "See Test-Writer: specs/001-pay", "", false},
		"the spec file instead of its directory": {bodyOf + "Test-Writer: specs/001-pay/spec.md",
			`Test-Writer: line says "specs/001-pay/spec\.md", not the specs/<NNN-slug> of a spec it holds; reword it`, true},
		"a spec that does not exist": {bodyOf + "Test-Writer: specs/001-pya", "reword it", true},
		"no value":                   {bodyOf + "Test-Writer:", "reword it", true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := copyOf(t, repo)
			gitT(t, root, "commit", "-q", "--no-verify", "--amend", "-m", tc.msg)
			replace(t, root, frozenTest, "!= 3", "< 0")
			if tc.editSpec {
				add(t, root, frozenSpec, "# changed")
			}
			commitAll(t, root, "feat(cart): pay")
			if tc.want == "" {
				frozenFindings(t, root, nil)
				return
			}
			frozenFindings(t, root, []string{tc.want})
		})
	}
}

// TestSpecsFrozenRootCommit: a test commit without a parent (history merged in from elsewhere) is frozen as well.
func TestSpecsFrozenRootCommit(t *testing.T) {
	t.Parallel()
	root := frozenRepo(t)
	gitT(t, root, "checkout", "-q", "--orphan", "elsewhere")
	gitT(t, root, "rm", "-rfq", ".")
	writeLines(t, root, "specs/003-x/spec.md", "# X")
	writeLines(t, root, frozenOther, "package cart", "// want 3")
	commitAll(t, root, "test(cart): x\n\nTest-Writer: specs/003-x")
	gitT(t, root, "checkout", "-q", "feat")
	gitT(t, root, "merge", "-q", "--no-edit", "--allow-unrelated-histories", "elsewhere")
	frozenFindings(t, root, nil)
	impl(t, root, func() { replace(t, root, frozenOther, "3", "4") })
	frozenFindings(t, root, []string{`other_test.go:2: commit .* "// want 3"`})
}

// TestSpecsFrozenCommand: arguments, environment, exit codes, and which root the predicate comes from.
func TestSpecsFrozenCommand(t *testing.T) {
	t.Parallel()
	clean := frozenRepo(t)
	none := copyOf(t, clean)
	gitT(t, none, "checkout", "-q", "main")
	dirty := copyOf(t, clean)
	tester(t, dirty, func() { replace(t, dirty, frozenOld, "old()", "pay()") })
	impl(t, dirty, func() {
		replace(t, dirty, frozenOld, "package cart", "package cart // moved")
		replace(t, dirty, frozenOld, "pay()", "old()")
	})
	shallow := filepath.Join(t.TempDir(), "shallow") // it has the merge base and not the test commit
	onMain(t, dirty, func() { writeLines(t, dirty, "x", "x") })
	gitT(t, dirty, "clone", "-q", "--depth=2", "--no-single-branch", "file://"+dirty, shallow)
	own := templateRepo(t, map[string][]string{"scripts/harness/protected-paths.txt": {"apps/api/internal/cart/*"}})
	// what only this clone and this environment hold does not count
	gitT(t, dirty, "config", "diff.suppressBlankEmpty", "true")
	gitT(t, dirty, "config", "diff.interHunkContext", "9")
	gitT(t, dirty, "config", "blame.ignoreRevsFile", ".git/ignored")
	gitT(t, dirty, "log", "-1", "--format=%H", "--output="+filepath.Join(dirty, ".git/ignored"), "HEAD~2")
	gitT(t, dirty, "replace", "--graft", "HEAD~1", "main")
	writeLines(t, dirty, ".git/info/grafts")
	gitT(t, dirty, "log", "-1", "--format=%H", "--output="+filepath.Join(dirty, ".git/info/grafts"), "HEAD~1")
	for _, tt := range []struct {
		name       string
		args       []string
		root, base string
		want       int
	}{
		{"clean", []string{clean}, "", "main", 0},
		{"finding", []string{dirty}, "", "main", 1},
		{"default BASE_REF is main without origin/main", []string{dirty}, "", "", 1},
		{"HARNESS_ROOT is read", []string{clean}, dirty, "main", 1},
		{"HARNESS_ROOT is not the top of its repository", []string{clean}, filepath.Join(dirty, "apps"), "main", 2},
		{"the predicate is the one of <root>", []string{own}, dirty, "main", 0},
		{"no test commits", []string{none}, "", "main", 0},
		{"the clone is cut off after the test commit", []string{shallow}, "", "origin/main", 2},
		{"base does not resolve", []string{clean}, "", "nope", 2},
		{"no args", nil, "", "", exitUsage},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			if code := runTest(t, append([]string{"specs-frozen"}, tt.args...), nil, &stdout, &stderr,
				"HARNESS_ROOT="+tt.root, "BASE_REF="+tt.base, "GIT_ICASE_PATHSPECS=1"); code != tt.want {
				t.Errorf("exit %d, want %d (%s%s)", code, tt.want, stdout.String(), stderr.String())
			}
		})
	}
}
