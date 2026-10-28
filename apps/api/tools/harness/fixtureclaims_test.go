package main

import (
	"bytes"
	"strings"
	"testing"
)

// claimRepo is a repo whose rules.md holds lines, with the fixtures (dir → RULE= line) and other files given.
func claimRepo(t *testing.T, rules []string, fixtures map[string]string, other ...string) string {
	t.Helper()
	files := map[string][]string{rulesFile: rules}
	for dir, rule := range fixtures {
		files[fixturesDir+"/"+dir+"/fixture.conf"] = []string{"# comment", rule, "TARGET=t", "EXPECT='x'"}
	}
	for _, f := range other {
		files[f] = []string{""}
	}
	return linkRepo(t, files)
}

func TestFixtureClaims(t *testing.T) {
	const status = " · Status: seed"
	fixtures := map[string]string{
		"a-one":   "RULE=a",
		"a-two":   "RULE='a'",
		"b-fx":    `RULE="b"`,
		"a-x/sub": "RULE=a", // nested deeper: not a fixture
		"no-rule": "# RULE= missing",
	}
	tests := []struct {
		name  string
		rules []string
		want  []string // output lines; none means the check passes
	}{
		{"every valid form", []string{
			"# Harness rules", "Format: `**Fixture:**` in prose is not a claim.", "## Go",
			"### a", "- **Rule:** r", "- **Fixture:** `fixtures/a-one`, `fixtures/a-two`" + status,
			"### b", "- **Fixture:** `fixtures/b-fx`; `scripts/test/b.bats`, `scripts/test/*.bats`" + status,
			"## Other", "### c", "- **Fixture:** none (formatter, auto-fixable)" + status,
			"### d", "- **Fixture:** none (a hook; see `fixtures/a-one` (nested)); `scripts/test/gone.bats`, free text" + status,
			"### e", "- **Fixture:** `fixtures/a-one`", // no Status: the claim is the rest of the line
		}, []string{
			"docs/harness/rules.md:15: e: fixtures/a-one tests rule \"a\", not this one (RULE= in its fixture.conf) [rules-fixture-claims]",
		}},
		// The findings against rules.md before exec plan 0003 Q3.
		{"free text instead of a fixture", []string{
			"### a", "- **Fixture:** covered by `fixtures/a-one` side effects" + status,
		}, []string{
			"docs/harness/rules.md:2: a: \"covered by `fixtures/a-one` side effects\" is neither none (<reason>) nor `fixtures/<dir>`, … [; `<test path>`, …] [rules-fixture-claims]",
		}},
		{"missing fixtures/ prefix", []string{
			"### a", "- **Fixture:** `a-one`, `a-nope`" + status,
		}, []string{
			"docs/harness/rules.md:2: a: `a-one` is not a fixture claim: write `fixtures/a-one` [rules-fixture-claims]",
			"docs/harness/rules.md:2: a: `a-nope` is not `fixtures/<dir>` [rules-fixture-claims]",
		}},
		{"none needs a reason in parentheses", []string{
			"### a", "- **Fixture:** none" + status,
			"### b", "- **Fixture:** none ()" + status,
			"### c", "- **Fixture:** none (unbalanced" + status,
			"### d", "- **Fixture:** none, see the docs" + status,
		}, []string{
			"docs/harness/rules.md:2: a: `none` without a reason: write none (<why this rule has no fixture>) [rules-fixture-claims]",
			"docs/harness/rules.md:4: b: `none` without a reason: write none (<why this rule has no fixture>) [rules-fixture-claims]",
			"docs/harness/rules.md:6: c: `none` without a reason: write none (<why this rule has no fixture>) [rules-fixture-claims]",
			"docs/harness/rules.md:8: d: \"none, see the docs\" is neither none (<reason>) nor `fixtures/<dir>`, … [; `<test path>`, …] [rules-fixture-claims]",
		}},
		{"missing fixture, another rule's fixture, nested fixture", []string{
			"### a", "- **Fixture:** `fixtures/a-gone`, `fixtures/b-fx`, `fixtures/a-x`" + status,
		}, []string{
			"docs/harness/rules.md:2: a: fixtures/a-gone does not exist (no scripts/harness/fixtures/a-gone/fixture.conf) [rules-fixture-claims]",
			"docs/harness/rules.md:2: a: fixtures/b-fx tests rule \"b\", not this one (RULE= in its fixture.conf) [rules-fixture-claims]",
			"docs/harness/rules.md:2: a: fixtures/a-x does not exist (no scripts/harness/fixtures/a-x/fixture.conf) [rules-fixture-claims]",
		}},
		{"test paths must exist and be plain paths", []string{
			"### a", "- **Fixture:** `fixtures/a-one`; `scripts/test/gone.bats`, `:!:x`, `../x`, `fixtures/a-two`, `.`" + status,
		}, []string{
			"docs/harness/rules.md:2: a: test path scripts/test/gone.bats matches no git-known file [rules-fixture-claims]",
			"docs/harness/rules.md:2: a: `:!:x` is not a test path (fixtures come before the `;`) [rules-fixture-claims]",
			"docs/harness/rules.md:2: a: `../x` is not a test path (fixtures come before the `;`) [rules-fixture-claims]",
			"docs/harness/rules.md:2: a: `fixtures/a-two` is not a test path (fixtures come before the `;`) [rules-fixture-claims]",
			"docs/harness/rules.md:2: a: `.` is not a test path (fixtures come before the `;`) [rules-fixture-claims]",
		}},
		{"fixture.conf without RULE=", []string{
			"### a", "- **Fixture:** `fixtures/no-rule`" + status,
		}, []string{
			"docs/harness/rules.md:2: a: fixtures/no-rule tests rule \"\", not this one (RULE= in its fixture.conf) [rules-fixture-claims]",
		}},
		{"malformed lists", []string{
			"### a", "- **Fixture:** `fixtures/a-one` and more" + status,
			"### b", "- **Fixture:** `fixtures/b-fx`; `scripts/test/b.bats`; `scripts/test/b.bats`" + status,
			"### c", "- **Fixture:**" + status,
		}, []string{
			"docs/harness/rules.md:2: a: \"`fixtures/a-one` and more\" is neither none (<reason>) nor `fixtures/<dir>`, … [; `<test path>`, …] [rules-fixture-claims]",
			"docs/harness/rules.md:4: b: \"`fixtures/b-fx`; `scripts/test/b.bats`; `scripts/test/b.bats`\" is neither none (<reason>) nor `fixtures/<dir>`, … [; `<test path>`, …] [rules-fixture-claims]",
			"docs/harness/rules.md:6: c: \"\" is neither none (<reason>) nor `fixtures/<dir>`, … [; `<test path>`, …] [rules-fixture-claims]",
		}},
		{"one Fixture line per rule, inside a rule", []string{
			"## Go", "- **Fixture:** `fixtures/a-one`" + status,
			"### a", "- **Rule:** r",
			"### b", "- **Fixture:** `fixtures/b-fx`" + status, "- **Fixture:** `fixtures/b-fx`" + status,
			"## Docs", "### a-last",
		}, []string{
			"docs/harness/rules.md:2: a `**Fixture:**` line outside a `### <rule-id>` section [rules-fixture-claims]",
			"docs/harness/rules.md:3: a: no `- **Fixture:**` line [rules-fixture-claims]",
			"docs/harness/rules.md:7: b: a second `**Fixture:**` line (one per rule) [rules-fixture-claims]",
			"docs/harness/rules.md:9: a-last: no `- **Fixture:**` line [rules-fixture-claims]",
		}},
		// The fixture (scripts/harness/fixtures/rules-bogus-fixture-claim) appends a rule to rules.md.
		{"fixture rules-bogus-fixture-claim", []string{
			"### a", "- **Fixture:** `fixtures/a-one`" + status,
			"", "### fx-uncovered", "- **Fixture:** `fixtures/fx-never-written`" + status,
		}, []string{
			"docs/harness/rules.md:5: fx-uncovered: fixtures/fx-never-written does not exist (no scripts/harness/fixtures/fx-never-written/fixture.conf) [rules-fixture-claims]",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			bad, err := fixtureClaims(claimRepo(t, tt.rules, fixtures, "scripts/test/b.bats"), &out)
			if err != nil {
				t.Fatal(err)
			}
			got := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
			if out.Len() == 0 {
				got = nil
			}
			if bad != (len(tt.want) > 0) || strings.Join(got, "\n") != strings.Join(tt.want, "\n") {
				t.Errorf("bad=%v, output:\n%s\nwant:\n%s", bad, out.String(), strings.Join(tt.want, "\n"))
			}
		})
	}
}

func TestFixtureClaimsExitCodes(t *testing.T) {
	ok := claimRepo(t, []string{"### a", "- **Fixture:** none (no sensor)"}, nil)
	bad := claimRepo(t, []string{"### a", "- **Fixture:** none"}, nil)
	noRules := claimRepo(t, []string{"# Harness rules"}, nil)
	noFile := linkRepo(t, map[string][]string{"a.md": {"# A"}})
	for _, tt := range []struct {
		args []string
		want int
	}{
		{[]string{"fixture-claims", ok}, 0},
		{[]string{"fixture-claims", bad}, 1},
		{[]string{"fixture-claims", noRules}, 2}, // fail closed: nothing to check is not a pass
		{[]string{"fixture-claims", noFile}, 2},
		{[]string{"fixture-claims", t.TempDir()}, 2}, // not a git repo
		{[]string{"fixture-claims"}, 2},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(tt.args, strings.NewReader(""), &stdout, &stderr); code != tt.want {
			t.Errorf("%v: exit %d, want %d (%s%s)", tt.args, code, tt.want, stdout.String(), stderr.String())
		}
	}
}
