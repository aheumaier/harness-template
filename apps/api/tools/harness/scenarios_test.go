package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// protectedList is the protected-paths.txt of the repos below: the harness's own tests and fixtures are in it.
var protectedList = []string{"# harness files", "apps/api/tools/harness/* config", "scripts/harness/* config", "AGENTS.md"}

// scenarioRepo is a repo with the template's layout and protected paths plus files (path → lines).
func scenarioRepo(t *testing.T, files map[string][]string) string {
	t.Helper()
	noHarnessStack(t)
	all := map[string][]string{layoutFile: templateLayout, "scripts/harness/protected-paths.txt": protectedList}
	for p, lines := range files {
		all[p] = lines
	}
	return linkRepo(t, all)
}

// story is a spec-kit user story section with its acceptance scenarios; they start on its line 7.
func story(heading string, scenarios ...string) []string {
	lines := []string{"### " + heading, "", "Plain words.", "", "**Acceptance Scenarios**:", ""}
	lines = append(lines, scenarios...)
	return append(lines, "", "---", "")
}

// spec is a spec of sections; the first starts on line 5.
func spec(sections ...[]string) []string {
	lines := []string{"# Feature Specification: Demo", "", "## User Scenarios & Testing *(mandatory)*", ""}
	for _, s := range sections {
		lines = append(lines, s...)
	}
	return lines
}

func specFinding(line int, msg string) string {
	return fmt.Sprintf("specs/001-demo/spec.md:%d: %s [specs-scenarios-tested]", line, msg)
}

func untestedAt(line int, id, text string) string {
	return specFinding(line, fmt.Sprintf("%s has no test naming it: %s", id, text))
}

func TestScenariosTested(t *testing.T) {
	const (
		g1 = "1. **Given** a cart, **When** I pay, **Then** I get a receipt"
		g2 = "2. **Given** an empty cart, **When** I pay, **Then** I am told it is empty"
	)
	var (
		g1Untested = untestedAt(11, "001/US1-AS1", strings.TrimPrefix(g1, "1. "))
		noStory    = specFinding(1, `no "### User Story <n> - <title>" section (heading level 1 to 3), so no scenario ties this spec to a test; spec-kit's User Scenarios section is mandatory`)
		outside    = specFinding(7, "**Acceptance Scenarios** is not a top-level paragraph of a User Story section, so its scenarios have no id; move it under its story's heading")
		named      = map[string][]string{"apps/api/a_test.go": {"// spec: 001/US1-AS1"}}
	)
	with := func(files map[string][]string, specLines []string) map[string][]string {
		all := map[string][]string{"specs/001-demo/spec.md": specLines}
		for p, l := range files {
			all[p] = l
		}
		return all
	}
	tests := []struct {
		name  string
		files map[string][]string
		want  []string
	}{
		{"no specs", nil, nil},
		{"named by a Go subtest, a TS title, an e2e comment or a domain package test, or exempted", map[string][]string{
			"specs/001-demo/spec.md": spec(
				story("User Story 1 - Pay (Priority: P1)", g1, g2),
				story("User Story 2 - Refund (Priority: P2)", "1. **Given** a paid order, **When** I refund, **Then** money returns",
					"1. **Given** a refunded order, **When** I refund again, **Then** nothing happens (untested: needs the payment provider's sandbox)",
					"1. **Given** a card, **When** the bank declines, **Then** I see why (untested: needs the bank's sandbox (v2))",
					"1. **Given** a voucher, **When** I refund, **Then** it is restored"),
			),
			"apps/api/internal/cart/service/pay_test.go": {`t.Run("001/US1-AS1 pays", func(t *testing.T) {})`},
			"apps/web/src/Cart.test.tsx":                 {`it('001/US1-AS2: empty cart', () => {})`},
			"apps/web/e2e/refund.e2e.ts":                 {"// spec: 001/US2-AS1"},
			"packages/domain/src/voucher.test.ts":        {`it("restores the voucher (001/US2-AS4)", () => {})`},
		}, nil},
		{"untested scenarios name their id and text; the position is the id, not the written number", map[string][]string{
			"specs/001-demo/spec.md": spec(story("User Story 1 - Pay", "1. **Given** a cart,", "   **When** I pay, **Then** I get a receipt", "1. **Given** b")),
		}, []string{
			untestedAt(11, "001/US1-AS1", "**Given** a cart, **When** I pay, **Then** I get a receipt"),
			untestedAt(13, "001/US1-AS2", "**Given** b"),
		}},
		{"nested ordered steps belong to their scenario", with(named, spec(story("User Story 1 - Pay",
			"1. **Given** a", "   1. step one", "   2. step two", "2. **Given** b"))),
			[]string{untestedAt(14, "001/US1-AS2", "**Given** b")}},
		{"a bullet after a scenario is not its continuation, so its exemption is not the scenario's", map[string][]string{
			"specs/001-demo/spec.md": spec(story("User Story 1 - Pay", "1. **Given** a", "- **Given** b (untested: manual)")),
		}, []string{untestedAt(11, "001/US1-AS1", "**Given** a")}},
		{"a line over 1 MiB is read whole", map[string][]string{
			"specs/001-demo/spec.md": spec(story("User Story 1 - Pay", "1. **Given** "+strings.Repeat("a ", 600000)+"(untested: a line over 1 MiB)", "2. **Given** b")),
		}, []string{untestedAt(12, "001/US1-AS2", "**Given** b")}},
		{"an exemption may wrap, but needs a real reason and must end the scenario", map[string][]string{
			"specs/001-demo/spec.md": spec(story("User Story 1 - Pay",
				"1. **Given** a, **When** b, **Then** c", "   (untested: manual only)",
				"2. **Given** d (untested: )",
				"3. **Given** e (untested: x) and more",
				"4. **Given** f (untested: n/a)",
				"5. **Given** g (untested: TBD)",
				"6. **Given** h (untested:  todo. )",
				"7. **Given** i (untested: -)",
				"8. **Given** j (untested: ?)")),
		}, []string{
			specFinding(13, "001/US1-AS2: (untested: …) gives no reason; write why no test can check it"),
			untestedAt(14, "001/US1-AS3", "**Given** e (untested: x) and more"),
			specFinding(15, "001/US1-AS4: (untested: n/a) is a placeholder, not a reason; write why no test can check it"),
			specFinding(16, "001/US1-AS5: (untested: TBD) is a placeholder, not a reason; write why no test can check it"),
			specFinding(17, "001/US1-AS6: (untested: todo.) is a placeholder, not a reason; write why no test can check it"),
			specFinding(18, "001/US1-AS7: (untested: -) is a placeholder, not a reason; write why no test can check it"),
			specFinding(19, "001/US1-AS8: (untested: ?) is a placeholder, not a reason; write why no test can check it"),
		}},
		{"ids match as whole tokens only", map[string][]string{
			"specs/001-demo/spec.md": spec(story("User Story 1 - Pay", g1)),
			"apps/api/a_test.go":     {"// 001/US1-AS12 1001/US1-AS1 001/US11-AS1 x001/US1-AS1 001/US1-AS1-old 001/US1-AS1.5 a/001/US1-AS1"},
		}, []string{g1Untested}},
		{"a skipped or placeholder test names nothing", map[string][]string{
			"specs/001-demo/spec.md": spec(story("User Story 1 - Pay", g1, g2)),
			"apps/web/src/a.test.ts": {"it.todo('001/US1-AS1')", "describe.skip('001/US1-AS2', () => {})", "xit('001/US1-AS1', () => {})"},
			"apps/web/src/b.test.ts": {"xdescribe('001/US1-AS2', () => {})", "test.fixme('001/US1-AS1', () => {})", "xtest('001/US1-AS2')"},
			"apps/api/a_test.go":     {`t.Run("001/US1-AS1", func(t *testing.T) { t.Skip("later") })`, `t.Run("001/US1-AS2", func(t *testing.T) { t.Skipf("%d", 1) })`},
		}, []string{g1Untested, untestedAt(12, "001/US1-AS2", strings.TrimPrefix(g2, "2. "))}},
		{"a Swift test under a *Tests directory names its scenario (Swift Testing title or // spec: comment)", map[string][]string{
			"specs/001-demo/spec.md":                     spec(story("User Story 1 - Pay", g1, g2)),
			"apps/ios/Tests/AppCoreTests/PayTests.swift": {`@Test("001/US1-AS1 pays") func pays() {}`},
			"apps/ios/AppUITests/Flows/PayUITests.swift": {"// spec: 001/US1-AS2", "func testPay() throws {}"},
		}, nil},
		{"a skipped Swift test names nothing; .disabled( is a skip in Swift only", map[string][]string{
			"specs/001-demo/spec.md": spec(story("User Story 1 - Pay", g1, g2, "3. **Given** c", "4. **Given** d")),
			"apps/ios/Tests/AppCoreTests/PayTests.swift": {
				`@Test("001/US1-AS1 pays", .disabled("later")) func pays() {}`,
				`try XCTSkipIf(true) // 001/US1-AS3`, `throw XCTSkip("001/US1-AS3")`,
				`@Test("001/US1-AS4 refunds", .enabled(if: false)) func refunds() {}`,
			},
			"apps/web/src/c.test.ts": {"it('001/US1-AS2', () => expect(b.disabled()).toBe(true))"},
		}, []string{g1Untested, untestedAt(13, "001/US1-AS3", "**Given** c"), untestedAt(14, "001/US1-AS4", "**Given** d")}},
		{"a Swift test inside #if false names nothing; its #else branch does; .disabled may be spelled loosely", map[string][]string{
			"specs/001-demo/spec.md": spec(story("User Story 1 - Pay", g1, g2, "3. **Given** c", "4. **Given** d")),
			"apps/ios/Tests/AppCoreTests/PayTests.swift": {
				"#if false", `@Test("001/US1-AS1 pays") func pays() {}`, "#if os(iOS)", `// spec: 001/US1-AS1`, "#endif",
				"#else", `@Test("001/US1-AS2 refunds") func refunds() {}`, "#endif",
				`@Test("001/US1-AS3 voids", .disabled ("later")) func voids() {}`,
				"@Test(\"001/US1-AS4 splits\", .`disabled`(\"later\")) func splits() {}",
			},
		}, []string{g1Untested, untestedAt(13, "001/US1-AS3", "**Given** c"), untestedAt(14, "001/US1-AS4", "**Given** d")}},
		{"a Swift test outside a *Tests directory is not a test file", map[string][]string{
			"specs/001-demo/spec.md":                    spec(story("User Story 1 - Pay", g1)),
			"apps/ios/Sources/AppCore/PayTests.swift":   {"// spec: 001/US1-AS1"},
			"apps/ios/Tests/AppCoreTests/Helpers.swift": {"// spec: 001/US1-AS1"},
		}, []string{g1Untested}},
		{"only product and web test files count: not the api client, generated code, harness tests, fixtures or other files", map[string][]string{
			"specs/001-demo/spec.md":                              spec(story("User Story 1 - Pay", g1)),
			"packages/api-client/src/client.test.ts":              {"// 001/US1-AS1"},
			"packages/api-client/src/gen/hooks.test.ts":           {"// 001/US1-AS1"},
			"apps/api/internal/cart/store/query_test.go":          {"// 001/US1-AS1"},
			"apps/api/tools/harness/x_test.go":                    {"// 001/US1-AS1"},
			"scripts/harness/fixtures/f/files/apps/api/x_test.go": {"// 001/US1-AS1"},
			"apps/api/internal/cart/service/pay.go":               {"// 001/US1-AS1"},
			"apps/api/node_modules/x/y.test.ts":                   {"// 001/US1-AS1"},
			"tools/z_test.go":                                     {"// 001/US1-AS1"},
			"specs/001-demo/tasks.md":                             {"001/US1-AS1"},
		}, []string{g1Untested}},
		{"edge cases, comments and code blocks hold no scenarios", with(named, spec(
			[]string{"<!--", "### User Story 9 - Example", "**Acceptance Scenarios**:", "1. **Given** x", "-->", ""},
			[]string{"```text", "**Acceptance Scenarios**:", "1. **Given** fenced", "```", ""},
			story("User Story 1 - Pay", g1, "", "**Independent Test**: by paying.", "", "1. not a scenario"),
			[]string{"### Edge Cases", "", "1. What happens when the card is declined?"},
		)), nil},
		{"a bold story heading is a story", with(named, spec(story("**User Story 1** - Pay", g1))), nil},
		{"stories need a unique number and a numbered scenario list", with(named, spec(
			story("User Story - Pay"),
			story("User Story 1 - Pay", g1),
			story("User Story 1 - Again", g1),
			story("User Story 2 - Bullets", "- **Given** a bullet"),
		)), []string{
			specFinding(5, `user story heading has no number, so its scenarios have no id; write "### User Story <n> - <title>"`),
			specFinding(24, "User Story 1 is also the heading on line 14, so their scenario ids clash; renumber one"),
			specFinding(34, "User Story 2 has no numbered list under **Acceptance Scenarios**:, so nothing ties it to a test"),
		}},
		{"a level-4 story heading starts no story", with(named, spec([]string{"#### User Story 1 - Pay", "", "**Acceptance Scenarios**:", "", g1})), []string{
			noStory,
			specFinding(5, `"User Story 1 - Pay" is a level 4 heading, so it starts no story; write "### User Story <n> - <title>"`),
			outside,
		}},
		{"a story heading without the words User Story starts no story", with(named, spec([]string{"### Story 1 - Pay", "", "**Acceptance Scenarios**:", "", g1})),
			[]string{noStory, outside}},
		{"a story inside an HTML comment is no story", with(named, spec([]string{"<!--", "### User Story 1 - Pay", "", "**Acceptance Scenarios**:", "", g1, "-->"})),
			[]string{noStory}},
		{"a story after an unclosed fence is no story", with(named, spec([]string{"```", "### User Story 1 - Pay", "", "**Acceptance Scenarios**:", "", g1})),
			[]string{noStory}},
		{"a directory that is not NNN-slug is specs-structure's finding", map[string][]string{
			"specs/demo/spec.md":       spec(story("User Story 1 - Pay", g1)),
			"specs/001-demo/x/spec.md": spec(story("User Story 1 - Pay", g1)),
		}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := scenarioRepo(t, tt.files)
			wantFindings(t, func(w io.Writer) (bool, error) { return scenariosTested(root, w) }, tt.want)
		})
	}
}

// The vendored spec-kit template is what /speckit-specify copies: its stories and scenarios must keep their ids.
func TestScenariosTestedReadsSpecKitTemplate(t *testing.T) {
	src, err := os.ReadFile(filepath.Join(repoRoot(t), ".specify/templates/spec-template.md"))
	if err != nil {
		t.Fatal(err)
	}
	scenarios, findings, err := parseScenarios("001", src)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, s := range scenarios {
		ids = append(ids, s.id)
	}
	if want := []string{"001/US1-AS1", "001/US1-AS2", "001/US2-AS1", "001/US3-AS1"}; !reflect.DeepEqual(ids, want) || len(findings) > 0 {
		t.Errorf("ids %v, findings %v; want ids %v and no findings", ids, findings, want)
	}
}

func TestScenariosTestedLinks(t *testing.T) {
	root := scenarioRepo(t, map[string][]string{
		"outside/spec.md":     spec(story("User Story 1 - Pay", "1. **Given** a")),
		"outside/a_test.go":   {"// spec: 002/US1-AS1"},
		"specs/002-b/spec.md": spec(story("User Story 1 - Pay", "1. **Given** b")),
	})
	for link, target := range map[string]string{"specs/001-a/spec.md": "outside/spec.md", "apps/api/a_test.go": "outside/a_test.go"} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, link)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(root, target), filepath.Join(root, link)); err != nil {
			t.Fatal(err)
		}
	}
	wantFindings(t, func(w io.Writer) (bool, error) { return scenariosTested(root, w) }, []string{
		"specs/001-a/spec.md:1: is not a regular file (a symlink?); commit the spec itself [specs-scenarios-tested]",
		"specs/002-b/spec.md:11: 002/US1-AS1 has no test naming it: **Given** b [specs-scenarios-tested]",
	})
}

func TestScenariosTestedExitCodes(t *testing.T) {
	ok := scenarioRepo(t, nil)
	bad := scenarioRepo(t, map[string][]string{"specs/001-demo/spec.md": spec(story("User Story 1 - Pay", "1. **Given** a"))})
	noLayout := linkRepo(t, map[string][]string{"README.md": {"x"}})
	noProtected := linkRepo(t, map[string][]string{layoutFile: templateLayout, "specs/001-demo/spec.md": spec(story("User Story 1 - Pay", "1. **Given** a"))})
	noGit := t.TempDir() // layout and protected paths are readable, so only git can fail
	for p, lines := range map[string][]string{layoutFile: templateLayout, "scripts/harness/protected-paths.txt": protectedList} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(noGit, p)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(noGit, p), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, tt := range []struct {
		args []string
		want int
	}{
		{[]string{"specs-tested", ok}, 0},
		{[]string{"specs-tested", bad}, 1},
		{[]string{"specs-tested", noLayout}, 2},
		{[]string{"specs-tested", noProtected}, 2},
		{[]string{"specs-tested", noGit}, 2},
		{[]string{"specs-tested"}, 2},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(tt.args, strings.NewReader(""), &stdout, &stderr); code != tt.want {
			t.Errorf("%v: exit %d, want %d (%s%s)", tt.args, code, tt.want, stdout.String(), stderr.String())
		}
	}
}
