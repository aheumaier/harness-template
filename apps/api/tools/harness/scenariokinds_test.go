package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

func userSeenAt(line int, id, file, kinds string) string {
	return specFinding(line, fmt.Sprintf("%s is a scenario a user sees and only other tests name it (%s); name it in %s, or end it with (unit: <reason>); "+
		"a marker added after the test commit lifts the freeze on this spec's tests (specs-tests-frozen), so prefer a new test commit", id, file, kinds))
}

// Which tests trace a scenario a user sees (ADR 0049). GOOS and GOARCH are fixed, so a build constraint decides
// alike on every host.
func TestScenariosTestedUserSeen(t *testing.T) {
	t.Parallel()
	const (
		api          = "apps/api/internal/cart/api/"
		both         = "a journey (apps/web/e2e/**/*.e2e.ts) or an integration test in apps/api/internal/<domain>/api/ (//go:build integration)"
		apiOnly      = "an integration test in apps/api/internal/<domain>/api/ (//go:build integration)"
		unitTest     = "apps/api/internal/cart/service/pay_test.go"
		tagged       = "//go:build integration"
		linuxOnly    = "//go:build integration && linux"
		noDB         = "notice: HARNESS_STACK=go:web:ios: no db, so no test runs the app and any test traces a scenario a user sees (ADR 0049) [specs-scenarios-tested]"
		defaultStack = ""
	)
	// one scenario per test file in names, each file naming its own id, so each line shows whether that file counts
	scenarios := func(n int) []string {
		var lines []string
		for i := 1; i <= n; i++ {
			lines = append(lines, fmt.Sprintf("%d. **Given** s%d", i, i))
		}
		return spec(story("User Story 1 - Pay", lines...))
	}
	repo := func(specLines []string, files ...[]string) map[string][]string {
		all := map[string][]string{"specs/001-demo/spec.md": specLines}
		for i, f := range files {
			all[f[0]] = append(slices.Clone(f[1:]), fmt.Sprintf("// spec: 001/US1-AS%d", i+1))
		}
		return all
	}
	id := func(i int) string { return fmt.Sprintf("001/US1-AS%d", i) }
	tests := []struct {
		name  string
		stack string // HARNESS_STACK, "" for the template's go:db:web:ios
		goos  string
		files map[string][]string
		want  []string
	}{
		{"journeys and integration tests in a domain's api package trace it", defaultStack, "linux", repo(scenarios(4),
			[]string{"apps/web/e2e/pay.e2e.ts"},
			[]string{"apps/web/e2e/cart/refund.e2e.ts"},
			[]string{api + "pay_integration_test.go", tagged, "", "package api_test"},
			[]string{api + "refund_test.go", "// +build integration", "", "package api_test"},
		), nil},
		{"a test outside those kinds, or one only another build runs, does not", defaultStack, "linux", repo(scenarios(16),
			[]string{unitTest},
			[]string{"apps/web/src/Cart.test.tsx"},
			[]string{"apps/web/src/pay.e2e.ts"},
			[]string{"apps/web/e2e/pay.e2e.tsx"},
			[]string{"apps/web/e2e/pay.spec.ts"},
			[]string{api + "pay_test.go", "package api_test"},
			[]string{api + "neg_test.go", "//go:build !integration", "", "package api_test"},
			[]string{api + "late_test.go", "package api_test", "", tagged},
			[]string{"apps/api/internal/cart/service/pay_integration_test.go", tagged, "", "package service_test"},
			[]string{api + "routes/deep_test.go", tagged, "", "package routes_test"},
			[]string{"apps/api/internal/platform/api/x_test.go", tagged, "", "package api_test"},
			[]string{"apps/api/internal/_draft/api/x_test.go", tagged, "", "package api_test"},
			[]string{api + "norace_test.go", "//go:build integration && !race", "", "package api_test"},
			[]string{"apps/api/internal/vendor/api/x_test.go", tagged, "", "package api_test"},
			[]string{"apps/api/internal/.cart/api/x_test.go", tagged, "", "package api_test"},
			[]string{"apps/api/internal/testdata/api/x_test.go", tagged, "", "package api_test"},
		), []string{
			userSeenAt(11, id(1), unitTest, both),
			userSeenAt(12, id(2), "apps/web/src/Cart.test.tsx", both),
			userSeenAt(13, id(3), "apps/web/src/pay.e2e.ts", both),
			userSeenAt(14, id(4), "apps/web/e2e/pay.e2e.tsx", both),
			userSeenAt(15, id(5), "apps/web/e2e/pay.spec.ts", both),
			userSeenAt(16, id(6), api+"pay_test.go", both),
			userSeenAt(17, id(7), api+"neg_test.go", both),
			userSeenAt(18, id(8), api+"late_test.go", both),
			userSeenAt(19, id(9), "apps/api/internal/cart/service/pay_integration_test.go", both),
			userSeenAt(20, id(10), api+"routes/deep_test.go", both),
			userSeenAt(21, id(11), "apps/api/internal/platform/api/x_test.go", both),
			userSeenAt(22, id(12), "apps/api/internal/_draft/api/x_test.go", both),
			userSeenAt(23, id(13), api+"norace_test.go", both),
			userSeenAt(24, id(14), "apps/api/internal/vendor/api/x_test.go", both),
			userSeenAt(25, id(15), "apps/api/internal/.cart/api/x_test.go", both),
			userSeenAt(26, id(16), "apps/api/internal/testdata/api/x_test.go", both),
		}},
		{"go/build decides for GOOS: linux builds both, darwin neither", defaultStack, "linux", repo(scenarios(2),
			[]string{api + "os_test.go", linuxOnly, "", "package api_test"},
			[]string{api + "pay_linux_test.go", tagged, "", "package api_test"},
		), nil},
		{"on darwin the linux-only integration tests do not run", defaultStack, "darwin", repo(scenarios(2),
			[]string{api + "os_test.go", linuxOnly, "", "package api_test"},
			[]string{api + "pay_linux_test.go", tagged, "", "package api_test"},
		), []string{userSeenAt(11, id(1), api+"os_test.go", both), userSeenAt(12, id(2), api+"pay_linux_test.go", both)}},
		{"API_DIR's own go.mod does not make a domain another module", defaultStack, "linux", map[string][]string{
			"specs/001-demo/spec.md":        scenarios(1),
			"apps/api/go.mod":               {"module example.com/api"},
			api + "pay_integration_test.go": {tagged, "", "package api_test", "// spec: 001/US1-AS1"},
		}, nil},
		{"a domain with its own go.mod is another module, which go test ./... skips", defaultStack, "linux", map[string][]string{
			"specs/001-demo/spec.md":        scenarios(1),
			"apps/api/internal/cart/go.mod": {"module example.com/cart"},
			api + "pay_integration_test.go": {tagged, "", "package api_test", "// spec: 001/US1-AS1"},
		}, []string{userSeenAt(11, id(1), api+"pay_integration_test.go", both)}},
		{"one test that runs the app is enough, beside other tests", defaultStack, "linux", map[string][]string{
			"specs/001-demo/spec.md":  scenarios(1),
			unitTest:                  {"// spec: 001/US1-AS1"},
			"apps/web/e2e/pay.e2e.ts": {"// spec: 001/US1-AS1"},
		}, nil},
		{"(unit: …) lets any test trace it, (untested: …) still exempts it; both, or no reason, is a finding", defaultStack, "linux", map[string][]string{
			"specs/001-demo/spec.md": spec(story("User Story 1 - Pay",
				"1. **Given** a (unit: a pricing rule of packages/domain)",
				"2. **Given** b (unit: (a rule) of the service)",
				"3. **Given** c (unit: rounding)",
				"4. **Given** d (untested: needs the bank's sandbox)",
				"5. **Given** e (unit: a rule) (untested: no sandbox)",
				"6. **Given** f (untested: see (unit: x))",
				"7. **Given** g (unit: )",
				"8. **Given** h (unit: TBD)",
				"9. **Given** i (unit: x) and more",
				"10. **Given** j [see the note](unit:pricing)",
				"11. **Given** k (unit: **tbd**)",
				"12. **Given** l (unit: (n/a))",
				"13. **Given** m (unit: tbd!)")),
			unitTest: {"// spec: 001/US1-AS1", "// spec: 001/US1-AS7", "// spec: 001/US1-AS8", "// spec: 001/US1-AS9",
				"// spec: 001/US1-AS10", "// spec: 001/US1-AS11", "// spec: 001/US1-AS12", "// spec: 001/US1-AS13"},
			"packages/domain/src/rule.test.ts":       {"it('001/US1-AS2 rounds', () => {})"},
			"apps/api/internal/cart/model/x_test.go": {"// spec: 001/US1-AS5"},
		}, []string{
			untestedAt(13, id(3), "**Given** c (unit: rounding)"),
			specFinding(15, id(5)+": carries both (unit: …) and (untested: …); keep the one that holds"),
			specFinding(16, id(6)+": carries both (unit: …) and (untested: …); keep the one that holds"),
			specFinding(17, id(7)+": (unit: …) gives no reason; write why a unit test proves it"),
			specFinding(18, id(8)+": (unit: TBD) is a placeholder, not a reason; write why a unit test proves it"),
			specFinding(19, id(9)+": (unit: …) must end the scenario, with at most one level of parentheses in its reason"),
			specFinding(20, id(10)+": (unit: …) in a link target is no marker; end the scenario with (unit: <reason>)"),
			specFinding(21, id(11)+": (unit: **tbd**) is a placeholder, not a reason; write why a unit test proves it"),
			specFinding(22, id(12)+": (unit: (n/a)) is a placeholder, not a reason; write why a unit test proves it"),
			specFinding(23, id(13)+": (unit: tbd!) is a placeholder, not a reason; write why a unit test proves it"),
		}},
		{"without web a journey does not count, an integration test does", "go:db:ios", "linux", repo(scenarios(2),
			[]string{"apps/web/e2e/pay.e2e.ts"},
			[]string{api + "pay_integration_test.go", tagged, "", "package api_test"},
		), []string{userSeenAt(11, id(1), "apps/web/e2e/pay.e2e.ts", apiOnly)}},
		{"without db any test traces it, and a notice says so once", "go:web:ios", "linux", repo(scenarios(2),
			[]string{unitTest},
			[]string{"apps/web/src/Cart.test.tsx"},
		), []string{noDB}},
		{"without db and a scenario no test names, the finding comes first", "go:web:ios", "linux", repo(scenarios(2),
			[]string{unitTest},
		), []string{untestedAt(12, id(2), "**Given** s2"), noDB}},
		{"without db the notice needs a scenario without a marker", "go:web:ios", "linux", map[string][]string{
			"specs/001-demo/spec.md": spec(story("User Story 1 - Pay", "1. **Given** a (untested: manual)", "2. **Given** b (unit: a rule)")),
			unitTest:                 {"// spec: 001/US1-AS2"},
		}, nil},
		{"without db and without specs there is no notice", "go:web:ios", "linux", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			env := []string{"GOOS=" + tt.goos, "GOARCH=amd64"}
			if tt.stack != "" {
				env = append(env, "HARNESS_STACK="+tt.stack)
			}
			root := templateRepo(t, tt.files)
			check := func(w io.Writer) (bool, error) { return scenariosTested(testWorld(t, w, env...), root) }
			if len(tt.want) == 1 && strings.HasPrefix(tt.want[0], "notice: ") {
				// a notice alone is a pass: wantFindings expects bad for any output
				var out strings.Builder
				if bad, err := check(&out); err != nil || bad || out.String() != tt.want[0]+"\n" {
					t.Fatalf("bad=%v err=%v:\n%s\nwant only:\n%s", bad, err, out.String(), tt.want[0])
				}
				return
			}
			wantFindings(t, check, tt.want)
		})
	}
}

// An api test whose build line go/build cannot parse stops the run (exit 2): it cannot say whether the test runs.
func TestScenariosTestedBadBuildLine(t *testing.T) {
	t.Parallel()
	root := templateRepo(t, map[string][]string{
		"specs/001-demo/spec.md":                 spec(story("User Story 1 - Pay", "1. **Given** s1")),
		"apps/api/internal/cart/api/pay_test.go": {"//go:build integration &&", "", "package api_test", "// spec: 001/US1-AS1"},
	})
	_, err := scenariosTested(testWorld(t, io.Discard), root)
	if err == nil || !strings.Contains(err.Error(), "apps/api/internal/cart/api/pay_test.go") || !strings.Contains(err.Error(), "parsing //go:build line") {
		t.Fatalf("want a parse error naming the file, got %v", err)
	}
}

// A test that runs the app counts only where a make target runs it: the kinds of scenariokinds.go must match the
// Makefile's groups and recipes and Playwright's testDir and testMatch. Only the kinds this repo's STACK counts are
// checked: an adopter without web has no playwright.config.ts.
func TestAppTestKindsMatchMake(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	at, err := loadAppTests(testWorld(t, io.Discard), root)
	if err != nil {
		t.Fatal(err)
	}
	if !at.integration {
		t.Skip("no db in STACK: no test runs the app")
	}
	read := func(p string) string {
		b, err := os.ReadFile(filepath.Join(root, p))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	makefile := read("Makefile")
	targets := func(group string) []string {
		m := regexp.MustCompile(`(?m)^` + group + `\s*:=(.*)$`).FindStringSubmatch(makefile)
		if m == nil {
			t.Fatalf("Makefile: no %s := line", group)
		}
		return strings.Fields(m[1])
	}
	recipe := func(target string) string {
		m := regexp.MustCompile(`(?m)^` + target + `:.*\n((?:\t.*\n)+)`).FindStringSubmatch(makefile)
		if m == nil {
			t.Fatalf("Makefile: no %s target with a recipe", target)
		}
		return m[1]
	}
	type groupTarget struct{ group, target string }
	kinds := []groupTarget{{"DB_TARGETS", "test-integration"}}
	if at.journeys {
		kinds = append(kinds, groupTarget{"DB_TARGETS", "e2e"}, groupTarget{"WEB_TARGETS", "e2e"})
	}
	for _, c := range kinds {
		if !slices.Contains(targets(c.group), c.target) {
			t.Errorf("Makefile: %s does not hold %s, so scenariokinds.go counts a test STACK may not run", c.group, c.target)
		}
	}
	if r := recipe("test-integration"); !strings.Contains(r, "--dir $(API_DIR) ") || !strings.Contains(r, " -race ") ||
		!strings.Contains(r, "-tags="+integrationTag+" ./...\n") {
		t.Errorf("Makefile: test-integration does not run go test -race -tags=%s ./... in API_DIR, as runsApp assumes:\n%s", integrationTag, r)
	}
	if !at.journeys {
		return
	}
	if r := recipe("e2e"); !strings.Contains(r, "--dir "+webAppDir+" ") {
		t.Errorf("Makefile: e2e does not run Playwright in %s:\n%s", webAppDir, r)
	}
	config := read(webAppDir + "/playwright.config.ts")
	testDir := strings.TrimSuffix(strings.TrimPrefix(journeyDir, webAppDir+"/"), "/")
	for _, want := range []string{"testDir: '" + testDir + "',", "testMatch: '*" + journeySuffix + "',"} {
		if !strings.Contains(config, want) {
			t.Errorf("%s/playwright.config.ts has no %s: Playwright collects other files than scenariokinds.go counts", webAppDir, want)
		}
	}
	for _, unwanted := range []string{"testIgnore", "projects"} {
		if strings.Contains(config, unwanted) {
			t.Errorf("%s/playwright.config.ts sets %s: Playwright may skip files scenariokinds.go counts", webAppDir, unwanted)
		}
	}
}
