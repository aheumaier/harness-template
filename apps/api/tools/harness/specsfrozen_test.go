package main

import (
	"bytes"
	"os"
	"regexp"
	"strings"
	"testing"
)

const (
	frozenTest  = "apps/api/internal/cart/cart_test.go"
	frozenOther = "apps/api/internal/cart/other_test.go"
	frozenOld   = "apps/api/internal/cart/old_test.go"
	frozenSpec  = "specs/001-pay/spec.md"
	byTester    = "test(cart): pay\n\nTest-Writer: specs/001-pay"
)

var (
	cartTest = []string{"package cart", "", "func TestPay(t *testing.T) {", "\tif pay() != 3 {", "\t\tt.Fatal()", "\t}", "}"}
	oldTest  = []string{"package cart", "", "func TestOld(t *testing.T) {", "\tt.Skip()", "\told()", "}"}
	newTest  = []string{"", "func TestNew(t *testing.T) {", "\tfresh()", "}"}
)

// frozenRepo makes a repo whose main holds the harness files and an old test, and whose branch holds the spec and, as
// its last commit, the test writer's test.
func frozenRepo(t *testing.T) string {
	t.Helper()
	root := templateRepo(t, map[string][]string{"apps/api/internal/cart/cart.go": {"package cart"}, frozenOld: oldTest})
	commitAll(t, root, "chore: base")
	gitT(t, root, "checkout", "-q", "-b", "feat")
	writeLines(t, root, frozenSpec, "# Pay", "1. **Given** a cart")
	commitAll(t, root, "docs(specs): pay")
	writeLines(t, root, frozenTest, cartTest...)
	commitAll(t, root, byTester)
	return root
}

// copyOf is a private copy of repo, cheaper than its commits made again.
func copyOf(t *testing.T, repo string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(repo)); err != nil {
		t.Fatal(err)
	}
	return root
}

// impl commits what edit does as the implementer.
func impl(t *testing.T, root string, edit func()) {
	t.Helper()
	edit()
	commitAll(t, root, "feat(cart): pay")
}

// tester commits what edit does as the test writer.
func tester(t *testing.T, root string, edit func()) {
	t.Helper()
	edit()
	commitAll(t, root, byTester)
}

// onMain commits what edit does on main and merges it into the checked-out branch.
func onMain(t *testing.T, root string, edit func()) {
	t.Helper()
	gitT(t, root, "checkout", "-q", "main")
	edit()
	commitAll(t, root, "chore: main moves")
	gitT(t, root, "checkout", "-q", "-")
	gitT(t, root, "merge", "-q", "--no-edit", "main")
}

// mergeRef is the checkout CI makes: main, with the branch merged into it.
func mergeRef(t *testing.T, root string, onMain func()) {
	t.Helper()
	gitT(t, root, "checkout", "-q", "main")
	onMain()
	commitAll(t, root, "chore: main moves")
	gitT(t, root, "checkout", "-q", "--detach")
	gitT(t, root, "merge", "-q", "--no-edit", "feat")
}

// next lands feat on main and starts the next branch there, with a test commit for the same file.
func next(t *testing.T, root string) {
	t.Helper()
	gitT(t, root, "checkout", "-q", "main")
	gitT(t, root, "merge", "-q", "--ff-only", "feat")
	gitT(t, root, "checkout", "-q", "-b", "next")
	writeLines(t, root, frozenSpec, "# Pay again")
	commitAll(t, root, "docs(specs): more")
	tester(t, root, func() { add(t, root, frozenTest, "func TestRefund(t *testing.T) {}") })
}

func TestSpecsFrozen(t *testing.T) {
	t.Parallel()
	var (
		loosen   = func(t *testing.T, root string) { replace(t, root, frozenTest, "!= 3", "< 0") }
		dropCase = func(t *testing.T, root string) {
			replace(t, root, frozenTest, "\tif pay() != 3 {\n\t\tt.Fatal()\n\t}\n", "")
		}
		skip     = func(t *testing.T, root string) { replace(t, root, frozenTest, "T) {\n", "T) {\n\tt.Skip()\n") }
		editSpec = func(t *testing.T, root string) { add(t, root, frozenSpec, "2. **Given** no cart") }
		code     = func(t *testing.T, root string) {
			add(t, root, "apps/api/internal/cart/cart.go", "func pay() int { return 3 }")
		}
		restore   = func(t *testing.T, root string) { writeLines(t, root, frozenTest, cartTest...) }
		addNew    = func(t *testing.T, root string) { add(t, root, frozenOld, newTest...) }
		loosened  = `cart_test.go:4: commit [0-9a-f]{7} changed or removed this line of test commit [0-9a-f]{7}: "if pay\(\) != 3 \{"`
		skipped   = `cart_test.go:4: commit [0-9a-f]{7} added this line after test commit [0-9a-f]{7}; it skips its tests: "t.Skip\(\)"`
		isDeleted = `cart_test.go:1: commit [0-9a-f]{7} deleted this test file of commit`
		lifted    = `^notice: commit [0-9a-f]{7} changed specs/001-pay/spec.md after test commit [0-9a-f]{7}: 1 changes of its tests are not reported \[specs-tests-frozen\]$`
	)
	repo := frozenRepo(t)
	for name, tc := range map[string]struct {
		change func(t *testing.T, root string)
		want   []string // one pattern per line of output, in order
	}{
		"untouched": {func(t *testing.T, root string) {
			impl(t, root, func() { code(t, root) })
		}, nil},
		"an assertion edited": {func(t *testing.T, root string) {
			impl(t, root, func() { loosen(t, root) })
		}, []string{loosened}},
		"an assertion edited after a spec change": {func(t *testing.T, root string) {
			impl(t, root, func() { editSpec(t, root); loosen(t, root) })
		}, []string{lifted}},
		"a case deleted": {func(t *testing.T, root string) {
			impl(t, root, func() { dropCase(t, root) })
		}, []string{loosened, ":5: commit .* removed this line", ":6: commit .* removed this line"}},
		"a case deleted after a spec change": {func(t *testing.T, root string) {
			impl(t, root, func() { editSpec(t, root) })
			impl(t, root, func() { dropCase(t, root) })
		}, []string{"notice: .*: 3 changes"}},
		"t.Skip added": {func(t *testing.T, root string) {
			impl(t, root, func() { skip(t, root) })
		}, []string{skipped}},
		"t.Skip added after a spec change": {func(t *testing.T, root string) {
			impl(t, root, func() { editSpec(t, root); skip(t, root) })
		}, []string{lifted}},
		"a build constraint added": {func(t *testing.T, root string) {
			impl(t, root, func() { replace(t, root, frozenTest, "package", "//go:build never\n\npackage") })
		}, []string{`cart_test.go:1: commit .* it skips its tests: "//go:build never"`}},
		"a case added": {func(t *testing.T, root string) {
			impl(t, root, func() { add(t, root, frozenTest, "func TestMore(t *testing.T) {}") })
		}, nil},
		"the file deleted": {func(t *testing.T, root string) {
			impl(t, root, func() { gitT(t, root, "rm", "-qf", frozenTest) })
		}, []string{isDeleted}},
		"the file renamed by the branch": {func(t *testing.T, root string) {
			gitT(t, root, "mv", frozenTest, frozenOther)
			commitAll(t, root, "refactor(cart): rename")
		}, []string{isDeleted}},
		"the file deleted after a spec change": {func(t *testing.T, root string) {
			impl(t, root, func() { editSpec(t, root); gitT(t, root, "rm", "-qf", frozenTest) })
		}, []string{lifted}},
		"a journey's locator edited (ADR 0049)": {func(t *testing.T, root string) {
			tester(t, root, func() {
				writeLines(t, root, "apps/web/e2e/pay.e2e.ts", "test('001/US1-AS1 pays', async ({ page }) => {",
					"  await page.getByRole('button', { name: 'Pay' }).click()", "})")
			})
			impl(t, root, func() {
				replace(t, root, "apps/web/e2e/pay.e2e.ts", "getByRole('button', { name: 'Pay' })", "locator('.btn')")
			})
		}, []string{`apps/web/e2e/pay\.e2e\.ts:2: commit [0-9a-f]{7} changed or removed this line of test commit [0-9a-f]{7}: "await page\.getByRole`}},
		"an assertion edited in a file with an odd name": {func(t *testing.T, root string) {
			tester(t, root, func() { writeLines(t, root, "apps/web/src/[id] ü.test.ts", "expect(3)") })
			impl(t, root, func() {
				writeLines(t, root, "apps/web/src/i ü.test.ts", "it.skip('x')") // what the name matches as a pattern
				replace(t, root, "apps/web/src/[id] ü.test.ts", "3", "4")
			})
		}, []string{`apps/web/src/\[id\] ü.test.ts:1: commit .* "expect\(3\)"`}},
		"a file with an odd name deleted": {func(t *testing.T, root string) {
			tester(t, root, func() { writeLines(t, root, "apps/web/src/[id].test.ts", "expect(3)") })
			impl(t, root, func() { gitT(t, root, "--literal-pathspecs", "rm", "-q", "apps/web/src/[id].test.ts") })
		}, []string{`apps/web/src/\[id\].test.ts:1: commit .* deleted this test file`}},
		"a file whose name matches another deleted by its test writer": {func(t *testing.T, root string) {
			tester(t, root, func() { writeLines(t, root, "apps/web/src/[id].test.ts", "expect(3)") })
			impl(t, root, func() { writeLines(t, root, "apps/web/src/i.test.ts", "expect(4)") })
			tester(t, root, func() { gitT(t, root, "--literal-pathspecs", "rm", "-q", "apps/web/src/[id].test.ts") })
			impl(t, root, func() { gitT(t, root, "rm", "-q", "apps/web/src/i.test.ts") })
		}, nil},
		"a file the test writer deleted comes back": {func(t *testing.T, root string) {
			tester(t, root, func() { gitT(t, root, "rm", "-q", frozenOld) })
			impl(t, root, func() { writeLines(t, root, frozenOld, oldTest...) })
		}, nil},
		"a directory named like the base": {func(t *testing.T, root string) {
			impl(t, root, func() { writeLines(t, root, "main/x", "x"); loosen(t, root) })
		}, []string{loosened}},
		"the spec changed and changed back": {func(t *testing.T, root string) {
			impl(t, root, func() { editSpec(t, root); loosen(t, root) })
			impl(t, root, func() { replace(t, root, frozenSpec, "2. **Given** no cart\n", "") })
		}, []string{loosened}},
		"the spec deleted": {func(t *testing.T, root string) {
			impl(t, root, func() { gitT(t, root, "rm", "-qf", frozenSpec); loosen(t, root) })
		}, []string{lifted}},
		"the spec renumbered": {func(t *testing.T, root string) {
			impl(t, root, func() { gitT(t, root, "mv", "specs/001-pay", "specs/002-pay"); loosen(t, root) })
		}, []string{lifted}},
		"another spec changed": {func(t *testing.T, root string) {
			impl(t, root, func() { writeLines(t, root, "specs/002-x/spec.md", "# X"); loosen(t, root) })
		}, []string{loosened}},
		"only the spec's mode changed": {func(t *testing.T, root string) {
			gitT(t, root, "update-index", "--chmod=+x", frozenSpec)
			gitT(t, root, "commit", "-q", "--no-verify", "-m", "docs(specs): mode")
			loosen(t, root)
			gitT(t, root, "commit", "-q", "--no-verify", "-m", "feat(cart): pay", "--", frozenTest)
		}, []string{loosened}},
		"the spec changed in a merge": {func(t *testing.T, root string) {
			impl(t, root, func() { loosen(t, root) })
			gitT(t, root, "checkout", "-q", "main")
			impl(t, root, func() { code(t, root) })
			gitT(t, root, "checkout", "-q", "feat")
			gitT(t, root, "merge", "-q", "--no-commit", "main")
			impl(t, root, func() { editSpec(t, root) })
		}, []string{loosened}},
		"a test commit changes the spec alone": {func(t *testing.T, root string) {
			tester(t, root, func() { editSpec(t, root) })
			impl(t, root, func() { loosen(t, root) })
		}, []string{lifted}},
		"a second test commit corrects the test": {func(t *testing.T, root string) {
			tester(t, root, func() { loosen(t, root) })
		}, nil},
		"a second test commit corrects the test on a side branch": {func(t *testing.T, root string) {
			gitT(t, root, "checkout", "-q", "-b", "side")
			tester(t, root, func() { loosen(t, root) })
			gitT(t, root, "checkout", "-q", "feat")
			impl(t, root, func() { code(t, root) })
			gitT(t, root, "merge", "-q", "--no-edit", "side")
		}, nil},
		"a second test commit deletes the file": {func(t *testing.T, root string) {
			tester(t, root, func() { gitT(t, root, "rm", "-qf", frozenTest) })
		}, nil},
		"a second test commit deletes a case and the implementer adds code": {func(t *testing.T, root string) {
			tester(t, root, func() { dropCase(t, root) })
			impl(t, root, func() { code(t, root); add(t, root, frozenTest, "func TestMore(t *testing.T) {}") })
		}, nil},
		"a second test commit renames the file": {func(t *testing.T, root string) {
			tester(t, root, func() { gitT(t, root, "mv", frozenTest, frozenOther) })
			impl(t, root, func() { replace(t, root, frozenOther, "!= 3", "< 0") })
		}, []string{`other_test.go:4: commit .* "if pay\(\) != 3`}},
		"a second test commit adds to the file": {func(t *testing.T, root string) {
			tester(t, root, func() { add(t, root, frozenTest, "func TestRefund(t *testing.T) {", "\trefund()", "}") })
			impl(t, root, func() { replace(t, root, frozenTest, "\trefund()\n", "") })
		}, []string{`cart_test.go:9: commit .* "refund\(\)"`}},
		"loosened before a test commit adds above": {func(t *testing.T, root string) {
			impl(t, root, func() { loosen(t, root) })
			tester(t, root, func() { replace(t, root, frozenTest, "package cart\n", "package cart\n\nvar paid int\n") })
		}, []string{loosened}},
		"squashed into the implementation": {func(t *testing.T, root string) {
			code(t, root)
			gitT(t, root, "commit", "-q", "-a", "--no-verify", "--amend", "--no-edit")
			impl(t, root, func() { loosen(t, root) })
		}, []string{`^notice: commit .* but changes "apps/api/internal/cart/cart.go", which`}},
		"a skip in a second test commit": {func(t *testing.T, root string) {
			tester(t, root, func() { skip(t, root) })
		}, []string{skipped}},
		"restored": {func(t *testing.T, root string) {
			impl(t, root, func() { loosen(t, root) })
			impl(t, root, func() { restore(t, root) })
		}, nil},
		"restored by a test commit": {func(t *testing.T, root string) {
			impl(t, root, func() { loosen(t, root) })
			tester(t, root, func() { restore(t, root) })
		}, nil},
		"restored by a test commit that adds to the file": {func(t *testing.T, root string) {
			impl(t, root, func() { loosen(t, root) })
			tester(t, root, func() { restore(t, root); add(t, root, frozenTest, "func TestRefund(t *testing.T) {}") })
		}, []string{loosened}},
		"reverted and picked again": {func(t *testing.T, root string) {
			gitT(t, root, "revert", "--no-edit", "HEAD")
			gitT(t, root, "cherry-pick", "HEAD~1")
		}, nil},
		"a skip and a build constraint of the test writer restored": {func(t *testing.T, root string) {
			tester(t, root, func() { writeLines(t, root, frozenOther, "//go:build integration", "", "package cart", "\tt.Skip()") })
			impl(t, root, func() { gitT(t, root, "rm", "-q", frozenOther) })
			gitT(t, root, "revert", "--no-edit", "HEAD")
		}, nil},
		"a merge rejects the loosening": {func(t *testing.T, root string) {
			gitT(t, root, "checkout", "-q", "-b", "side")
			impl(t, root, func() { loosen(t, root) })
			gitT(t, root, "checkout", "-q", "feat")
			impl(t, root, func() { code(t, root); add(t, root, frozenTest, "func TestMore(t *testing.T) {}") })
			gitT(t, root, "merge", "-q", "-s", "ours", "--no-edit", "side")
		}, nil},
		"loosened on both sides of a merge and restored": {func(t *testing.T, root string) {
			gitT(t, root, "checkout", "-q", "-b", "side")
			impl(t, root, func() { code(t, root); loosen(t, root) })
			gitT(t, root, "checkout", "-q", "feat")
			impl(t, root, func() { loosen(t, root) })
			gitT(t, root, "merge", "-q", "--no-edit", "side")
			impl(t, root, func() { restore(t, root); add(t, root, frozenTest, "func TestMore(t *testing.T) {}") })
		}, nil},
		"one of two like lines restored": {func(t *testing.T, root string) {
			tester(t, root, func() { add(t, root, frozenTest, "// want 3", "// and", "// want 3") })
			impl(t, root, func() { replace(t, root, frozenTest, "// want 3\n// and\n// want 3\n", "// and\n") })
			impl(t, root, func() { add(t, root, frozenTest, "// want 3") })
		}, []string{`"// want 3"`}},
		"loosened next to an older line like it": {func(t *testing.T, root string) {
			impl(t, root, func() { add(t, root, frozenTest, "func more() {", "\tif pay() != 3 {", "\t}", "}") })
			impl(t, root, func() { loosen(t, root) })
		}, []string{loosened}},
		"restored and loosened again": {func(t *testing.T, root string) {
			impl(t, root, func() { loosen(t, root) })
			impl(t, root, func() { restore(t, root) })
			impl(t, root, func() { loosen(t, root) })
		}, []string{loosened}},
		"restored, then corrected by the test writer": {func(t *testing.T, root string) {
			impl(t, root, func() { loosen(t, root) })
			impl(t, root, func() { restore(t, root) })
			tester(t, root, func() { loosen(t, root) })
		}, nil},
		"deleted, restored, then deleted by the test writer": {func(t *testing.T, root string) {
			impl(t, root, func() { gitT(t, root, "rm", "-qf", frozenTest) })
			impl(t, root, func() { restore(t, root) })
			tester(t, root, func() { gitT(t, root, "rm", "-qf", frozenTest) })
		}, nil},
		"a blank line next to its test removed": {func(t *testing.T, root string) {
			impl(t, root, func() { replace(t, root, frozenTest, "package cart\n\n", "package cart\n") })
		}, nil},
		"appended to a file without a last newline": {func(t *testing.T, root string) {
			tester(t, root, func() { replace(t, root, frozenTest, "\t}\n}\n", "\t}\n}") })
			impl(t, root, func() { replace(t, root, frozenTest, "\t}\n}", "\t}\n}\n\nfunc TestMore(t *testing.T) {}\n") })
		}, nil},
		"an old test edited in a file the test writer added to": {func(t *testing.T, root string) {
			tester(t, root, func() { addNew(t, root) })
			impl(t, root, func() { replace(t, root, frozenOld, "old()", "pay()") })
		}, nil},
		"an old test deleted in a file the test writer added to": {func(t *testing.T, root string) {
			tester(t, root, func() { addNew(t, root) })
			impl(t, root, func() { replace(t, root, frozenOld, strings.Join(oldTest[2:], "\n")+"\n", "") })
		}, nil},
		"an old line the test writer changed": {func(t *testing.T, root string) {
			tester(t, root, func() { replace(t, root, frozenOld, "old()", "pay()") })
			impl(t, root, func() { replace(t, root, frozenOld, "pay()", "old()") })
		}, []string{`old_test.go:5: commit .* "pay\(\)"`}},
		"a skip the branch added before the test writer came": {func(t *testing.T, root string) {
			impl(t, root, func() { add(t, root, frozenOld, "func TestLater(t *testing.T) {", "\tt.Skip()", "}") })
			tester(t, root, func() { addNew(t, root) })
			impl(t, root, func() { add(t, root, frozenOld, "func TestMore(t *testing.T) {}") })
			impl(t, root, func() { replace(t, root, frozenOld, "\tfresh()\n", "") })
		}, []string{`old_test.go:12: commit .* "fresh\(\)"`}},
		"a skip moved from an old test": {func(t *testing.T, root string) {
			tester(t, root, func() { addNew(t, root) })
			impl(t, root, func() {
				replace(t, root, frozenOld, "\tt.Skip()\n", "")
				replace(t, root, frozenOld, "TestNew(t *testing.T) {\n", "TestNew(t *testing.T) {\n\tt.Skip()\n")
			})
		}, []string{`old_test.go:8: commit .* it skips its tests: "t.Skip\(\)"`}},
		"main changes an old test in a file the test writer added to": {func(t *testing.T, root string) {
			tester(t, root, func() { addNew(t, root) })
			mergeRef(t, root, func() { replace(t, root, frozenOld, "old()", "pay()") })
		}, nil},
		"loosened, seen from CI's merge": {func(t *testing.T, root string) {
			impl(t, root, func() { loosen(t, root) })
			mergeRef(t, root, func() { code(t, root) })
		}, []string{loosened}},
		"loosened on a side branch": {func(t *testing.T, root string) {
			gitT(t, root, "checkout", "-q", "-b", "side")
			impl(t, root, func() { loosen(t, root) })
			gitT(t, root, "checkout", "-q", "feat")
			gitT(t, root, "merge", "-q", "--no-ff", "--no-edit", "side")
		}, []string{loosened}},
		"loosened in a conflict resolution": {func(t *testing.T, root string) {
			tester(t, root, func() { replace(t, root, frozenOld, "old()", "pay()") })
			gitT(t, root, "checkout", "-q", "main")
			impl(t, root, func() { replace(t, root, frozenOld, "old()", "older()") })
			gitT(t, root, "checkout", "-q", "feat")
			gitT(t, root, "merge", "-q", "-s", "ours", "--no-commit", "main")
			tester(t, root, func() { replace(t, root, frozenOld, "pay()", "older()") }) // a merge is no test commit
		}, []string{`old_test.go:5: commit .* "pay\(\)"`}},
		"test commits already on main, after a criss-cross merge": {func(t *testing.T, root string) {
			gitT(t, root, "checkout", "-q", "main")
			impl(t, root, func() { code(t, root) })
			gitT(t, root, "merge", "-q", "--no-edit", "feat")
			gitT(t, root, "checkout", "-q", "feat")
			gitT(t, root, "merge", "-q", "--no-edit", "main~1")
			impl(t, root, func() { loosen(t, root) })
		}, nil},
		"main renames the file": {func(t *testing.T, root string) {
			next(t, root)
			onMain(t, root, func() { gitT(t, root, "mv", frozenTest, frozenOther) })
		}, nil},
		"main adds a skip to the file": {func(t *testing.T, root string) {
			next(t, root)
			onMain(t, root, func() { skip(t, root) })
		}, nil},
		"main changes the spec": {func(t *testing.T, root string) {
			gitT(t, root, "checkout", "-q", "main")
			gitT(t, root, "merge", "-q", "--ff-only", "feat~1")
			gitT(t, root, "checkout", "-q", "feat")
			onMain(t, root, func() { editSpec(t, root) })
			impl(t, root, func() { loosen(t, root) })
		}, []string{loosened}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := copyOf(t, repo)
			tc.change(t, root)
			frozenFindings(t, root, tc.want)
		})
	}
}

// frozenFindings fails unless specs-frozen prints one line per pattern of want. A notice is no finding.
func frozenFindings(t *testing.T, root string, want []string) {
	t.Helper()
	var out bytes.Buffer
	bad, err := specsFrozen(testWorld(t, &out), root, root, "main")
	if err != nil {
		t.Fatal(err)
	}
	lines := splitLines(out.Bytes())
	ok, findings := len(lines) == len(want), false
	for i := 0; ok && i < len(want); i++ {
		ok = regexp.MustCompile(want[i]).MatchString(lines[i])
		ok = ok && strings.HasSuffix(lines[i], " [specs-tests-frozen]")
		findings = findings || !strings.HasPrefix(lines[i], "notice: ")
	}
	if !ok || bad != findings {
		t.Fatalf("got bad=%v:\n%s\nwant %q", bad, out.String(), want)
	}
}

// TestSpecsFrozenMarkers: an added line that skips tests, in each language.
func TestSpecsFrozenMarkers(t *testing.T) {
	t.Parallel()
	repo := frozenRepo(t)
	for name, tc := range map[string]struct {
		path, line string
		found      bool
	}{
		"Go":                              {frozenOther, "\tt.Skipf(\"\")", true},
		"Go build constraint with BOM":    {frozenOther, "\ufeff//go:build never", true},
		"Go's old build constraint":       {frozenOther, "// +build never", true},
		"TypeScript":                      {"apps/web/src/a.test.ts", "it.skip('pays', () => {})", true},
		"TypeScript, as Go would":         {"apps/web/src/a.test.ts", "//go:build never", false},
		"Swift":                           {"apps/ios/Tests/AppCoreTests/PayTests.swift", "@Test(.disabled())", true},
		"Swift, compiled out":             {"apps/ios/Tests/AppCoreTests/PayTests.swift", "#if false", true},
		"Go, as Swift would":              {frozenOther, "// #if false", false},
		"nothing that skips (Go)":         {frozenOther, "// skips nothing", false},
		"nothing that skips (TypeScript)": {"apps/web/src/a.test.ts", "const skipped = 1", false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := copyOf(t, repo)
			tester(t, root, func() { writeLines(t, root, tc.path, "// first") })
			impl(t, root, func() { add(t, root, tc.path, tc.line) })
			var want []string
			if tc.found {
				want = []string{tc.path + ":2: commit .* it skips its tests"}
			}
			frozenFindings(t, root, want)
		})
	}
}
