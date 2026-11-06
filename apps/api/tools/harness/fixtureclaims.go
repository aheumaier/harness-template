package main

// fixture-claims (rules-fixture-claims): every `### <rule-id>` in docs/harness/rules.md has exactly one
// `- **Fixture:**` line, and that line claims only what the selftest (ADR 0009) really proves. The claim is
// the text between `**Fixture:** ` and ` · Status:`:
//
//	none (<reason>)…   no fixture; the reason is not empty and, like anything after it, free text (it may
//	                   name other rules' fixtures or tests)
//	`fixtures/<dir>`[, `fixtures/<dir>`…][; `<test path>`[, `<test path>`…]]
//
// Each <dir> is a git-known scripts/harness/fixtures/<dir>/fixture.conf whose RULE= is the enclosing rule id;
// each test path is a git pathspec that matches a git-known file. A rule id has one `### <id>` heading: a
// second one is a finding. Run from `make fixture-claims` (make
// check). Fails closed (exit 2) when rules.md is missing or holds no rules.

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

const fixturesDir = "scripts/harness/fixtures"

var (
	fixtureDirRE = regexp.MustCompile(`^fixtures/([a-z0-9][a-z0-9-]*)$`)
	// A plain path or glob: no pathspec magic (`:`), nothing outside the repo.
	testPathRE = regexp.MustCompile(`^[A-Za-z0-9_.-][A-Za-z0-9_./*-]*$`)
)

type claimChecker struct {
	w        world
	root     string
	out      io.Writer
	bad      bool
	fixtures map[string]string // fixture dir → RULE= of its fixture.conf
}

func (c *claimChecker) report(line int, format string, a ...any) {
	printFinding(c.out, rulesFile, line, "rules-fixture-claims", format, a...)
	c.bad = true
}

// ruleHeading is one `### <rule-id>` heading of rules.md: the id, the `## ` section it sits in (that heading's
// text before any ` (`), its line, and first: the line of an earlier heading with the same id (0 for none).
type ruleHeading struct {
	id, section string
	line, first int
}

// ruleHeadings lists rules.md's `### <rule-id>` headings in order; an error when there is none.
func ruleHeadings(src []byte) ([]ruleHeading, error) {
	var (
		rules   []ruleHeading
		section string
		seen    = map[string]int{} // rule id → line of its first heading
	)
	for i, line := range strings.Split(string(src), "\n") {
		switch {
		case strings.HasPrefix(line, "### "):
			id := strings.TrimSpace(line[4:])
			rules = append(rules, ruleHeading{id: id, section: section, line: i + 1, first: seen[id]})
			if seen[id] == 0 {
				seen[id] = i + 1
			}
		case strings.HasPrefix(line, "## "):
			section, _, _ = strings.Cut(strings.TrimSpace(line[3:]), " (")
		}
	}
	if len(rules) == 0 {
		return nil, fmt.Errorf("%s has no `### <rule-id>` sections", rulesFile)
	}
	return rules, nil
}

func init() {
	commands.register(rootSensor("fixture-claims", "rules-fixture-claims (make fixture-claims)", fixtureClaims))
}

// fixtureClaims prints one line per finding to stdout and reports whether there was any. root is the repo.
func fixtureClaims(w world, root string) (bool, error) {
	src, err := os.ReadFile(filepath.Join(root, rulesFile))
	if err != nil {
		return false, err
	}
	headings, err := ruleHeadings(src)
	if err != nil {
		return false, err
	}
	c := &claimChecker{w: w, root: root, out: w.stdout}
	if c.fixtures, err = fixtureRules(w, root); err != nil {
		return false, err
	}
	at := map[int]ruleHeading{} // line → the rule heading there
	for _, h := range headings {
		at[h.line] = h
	}

	var (
		rule             string // the enclosing `### <id>`, "" outside one
		ruleLine, claims int
	)
	endRule := func() {
		if rule != "" && claims == 0 {
			c.report(ruleLine, "%s: no `- **Fixture:**` line", rule)
		}
	}
	for i, line := range strings.Split(string(src), "\n") {
		n := i + 1
		h, isRule := at[n]
		switch {
		case isRule:
			endRule()
			rule, ruleLine, claims = h.id, n, 0
			// Anchors, run.sh's remediation lookup and fixture RULE= all resolve an id to one section.
			if h.first != 0 {
				c.report(n, "%s: a second `### %s` heading (first at line %d; rule ids are unique)", rule, rule, h.first)
			}
		case strings.HasPrefix(line, "#"):
			endRule()
			rule = ""
		case strings.HasPrefix(line, "- **Fixture:**"):
			claims++
			switch {
			case rule == "":
				c.report(n, "a `**Fixture:**` line outside a `### <rule-id>` section")
			case claims > 1:
				c.report(n, "%s: a second `**Fixture:**` line (one per rule)", rule)
			default:
				claim, _, _ := strings.Cut(strings.TrimPrefix(line, "- **Fixture:**"), " · Status:")
				if err := c.claim(rule, n, strings.TrimSpace(claim)); err != nil {
					return false, err
				}
			}
		}
	}
	endRule()
	return c.bad, nil
}

// claim checks one rule's claim against the git-known fixtures and files.
func (c *claimChecker) claim(rule string, line int, claim string) error {
	if rest, ok := strings.CutPrefix(claim, "none"); ok && (rest == "" || rest[0] == ' ') {
		if reason, ok := parenthesized(strings.TrimPrefix(rest, " ")); !ok || strings.TrimSpace(reason) == "" {
			c.report(line, "%s: `none` without a reason: write none (<why this rule has no fixture>)", rule)
		}
		return nil
	}
	dirs, tests, ok := claimList(claim)
	if !ok {
		c.report(line, "%s: %q is neither none (<reason>) nor `fixtures/<dir>`, … [; `<test path>`, …]", rule, claim)
		return nil
	}
	for _, d := range dirs {
		m := fixtureDirRE.FindStringSubmatch(d)
		if m == nil {
			if _, exists := c.fixtures[d]; exists {
				c.report(line, "%s: `%s` is not a fixture claim: write `fixtures/%s`", rule, d, d)
			} else {
				c.report(line, "%s: `%s` is not `fixtures/<dir>`", rule, d)
			}
			continue
		}
		switch got, exists := c.fixtures[m[1]]; {
		case !exists:
			c.report(line, "%s: %s does not exist (no %s/%s/fixture.conf)", rule, d, fixturesDir, m[1])
		case got != rule:
			c.report(line, "%s: %s tests rule %q, not this one (RULE= in its fixture.conf)", rule, d, got)
		}
	}
	for _, p := range tests {
		if !testPathRE.MatchString(p) || p == "." || strings.Contains(p, "..") || strings.HasPrefix(p, "fixtures/") {
			c.report(line, "%s: `%s` is not a test path (fixtures come before the `;`)", rule, p)
			continue
		}
		files, err := c.w.gitKnown(c.root, p)
		if err != nil {
			return err
		}
		if len(files) == 0 {
			c.report(line, "%s: test path %s matches no git-known file", rule, p)
		}
	}
	return nil
}

// parenthesized returns the text inside the balanced parentheses s starts with.
func parenthesized(s string) (string, bool) {
	if !strings.HasPrefix(s, "(") {
		return "", false
	}
	depth := 0
	for i, r := range s {
		switch r {
		case '(':
			depth++
		case ')':
			if depth--; depth == 0 {
				return s[1:i], true
			}
		}
	}
	return "", false
}

// claimList splits "`a`, `b`; `t1`, `t2`" into the fixture items before the `; ` and the test paths after
// it. ok is false unless the whole claim is backticked items joined by ", " with at most one "; ".
func claimList(claim string) (dirs, tests []string, ok bool) {
	s, list := claim, &dirs
	for {
		if !strings.HasPrefix(s, "`") {
			return nil, nil, false
		}
		item, rest, found := strings.Cut(s[1:], "`")
		if !found || item == "" {
			return nil, nil, false
		}
		*list = append(*list, item)
		switch {
		case rest == "":
			return dirs, tests, true
		case strings.HasPrefix(rest, ", "):
			s = rest[2:]
		case strings.HasPrefix(rest, "; ") && list == &dirs:
			s, list = rest[2:], &tests
		default:
			return nil, nil, false
		}
	}
}

// fixtureRules maps each git-known fixture dir to the RULE= of its fixture.conf, read as selftest.sh reads
// it (the last RULE= line, one pair of quotes stripped).
func fixtureRules(w world, root string) (map[string]string, error) {
	confs, err := w.gitKnown(root, fixturesDir+"/*/fixture.conf")
	if err != nil {
		return nil, err
	}
	rules := map[string]string{}
	for _, conf := range confs {
		dir := path.Dir(conf)
		if path.Dir(dir) != fixturesDir {
			continue // nested deeper: not a fixture the selftest runs
		}
		b, err := os.ReadFile(filepath.Join(root, conf))
		if err != nil {
			return nil, err
		}
		rule := ""
		for _, l := range strings.Split(string(b), "\n") {
			if v, ok := strings.CutPrefix(l, "RULE="); ok {
				if v != "" && strings.ContainsRune(`'"`, rune(v[0])) {
					v = v[1:]
				}
				if v != "" && strings.ContainsRune(`'"`, rune(v[len(v)-1])) {
					v = v[:len(v)-1]
				}
				rule = v
			}
		}
		rules[path.Base(dir)] = rule
	}
	return rules, nil
}
