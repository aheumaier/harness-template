package main

// specs-tested (specs-scenarios-tested, ADR 0036): every acceptance scenario of a spec-kit spec is named by a
// test, or is exempted in the spec with a reason.
//
// Scenarios: goldmark parses each git-known specs/<NNN-slug>/spec.md (directories not named NNN-kebab-slug are
// specs-structure's finding and skipped here), so fenced code, HTML blocks and comments hold no scenarios. A
// story is a level 1-3 heading whose text starts with `User Story <n>`; it runs to the next level 1-3 heading.
// Its scenarios are the items of the ordered list right after its top-level `**Acceptance Scenarios**:`
// paragraph; nested lists are steps of an item, not scenarios. A scenario's id is `<NNN>/US<n>-AS<m>`: NNN from
// the directory, n from the heading, m the item's position (1-based, whatever number it is written with);
// nothing is written into the spec (exec plan 0003, A17). Findings, since scenarios would have no id or a shared
// one, or the spec none at all: no story, a story heading without a number, below level 3 or numbered twice, a
// story without a scenario list, and an Acceptance Scenarios paragraph outside a story.
//
// Tests: git-known Go and JS/TS test files (`*_test.go`, `*.test.*`, `*.spec.*`, `*.e2e.*`) under
// PRODUCT_GLOBS and WEB_GLOBS of scripts/harness/layout.env, minus API_CLIENT_DIR, GENERATED_GLOBS and the
// harness files of protected-paths.txt, so the harness's own tests and fixtures cannot satisfy an id. A test
// names an id when the id appears in it as a whole token (no letter, digit, `_`, `/`, `.` or `-` next to it),
// in a title, a string or a `// spec: <id>` comment; test syntax is not parsed. A line that also skips or
// defers its test (`.todo(`, `.skip(`, `.fixme(`, `xit(`, `xtest(`, `xdescribe(`, `t.Skip`) names nothing.
// Whether a test really checks its scenario is review's call (ADR 0019).
//
// A scenario passes when a test names its id or its text ends with `(untested: <reason>)` (the reason may hold
// one level of parentheses); an empty or placeholder reason (n/a, tbd, todo, …) is a finding. Run from
// `make specs-tested` (make check when specs/ or a test file changed, make check-all). Fails closed (exit 2) on
// an unreadable layout, protected-paths.txt or spec, or when git fails; no specs is a pass.

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

const scenarioRule = "specs-scenarios-tested"

var (
	specDirRE       = regexp.MustCompile(`^specs/([0-9]{3,})-[a-z0-9]+(-[a-z0-9]+)*/spec\.md$`)
	testFileRE      = regexp.MustCompile(`(_test\.go|\.(test|spec|e2e)\.[cm]?[jt]sx?)$`)
	scenarioIDRE    = regexp.MustCompile(`[0-9]{3,}/US[0-9]+-AS[0-9]+`)
	skippedTestRE   = regexp.MustCompile(`\.(todo|skip|fixme)\(|\bx(it|test|describe)\(|\bt\.Skip`)
	storyRE         = regexp.MustCompile(`(?i)^user story\b`)
	storyNumberRE   = regexp.MustCompile(`(?i)^user story[ \t]+([0-9]+)\b`)
	scenariosRE     = regexp.MustCompile(`(?i)^acceptance scenarios\b`)
	untestedRE      = regexp.MustCompile(`\(untested:((?:[^()]|\([^()]*\))*)\)[ \t]*$`)
	whitespaceRuns  = regexp.MustCompile(`\s+`)
	placeholderWhys = map[string]bool{"n/a": true, "na": true, "tbd": true, "todo": true, "none": true, "-": true, "x": true, "?": true}
)

// scenario is one acceptance scenario: its id, where it starts and its source text (lines joined).
type scenario struct {
	id   string
	line int
	text string
}

// scenariosTested prints one line per finding to stdout and reports whether there was any. root is the repo.
func scenariosTested(root string, stdout io.Writer) (bool, error) {
	l, err := loadLayout(root)
	if err != nil {
		return false, err
	}
	specs, err := gitFiles(root, "specs/*/spec.md")
	if err != nil {
		return false, err
	}
	sort.Strings(specs)
	var named map[string]bool // read lazily: no spec, no test scan
	bad := false
	for _, spec := range specs {
		m := specDirRE.FindStringSubmatch(spec)
		if m == nil {
			continue
		}
		regular, err := isRegular(root, spec)
		if err != nil {
			return false, err
		}
		if !regular { // CI reads an untrusted tree: never follow a link out of it
			bad = true
			fmt.Fprintf(stdout, "%s:1: is not a regular file (a symlink?); commit the spec itself [%s]\n", spec, scenarioRule)
			continue
		}
		src, err := os.ReadFile(filepath.Join(root, spec))
		if err != nil {
			return false, err
		}
		scenarios, findings, err := parseScenarios(m[1], src)
		if err != nil {
			return false, fmt.Errorf("%s: %w", spec, err)
		}
		if named == nil && len(scenarios) > 0 {
			if named, err = testedIDs(root, l); err != nil {
				return false, err
			}
		}
		for _, s := range scenarios {
			if f, ok := untested(s, named); ok {
				findings = append(findings, f)
			}
		}
		sort.SliceStable(findings, func(i, j int) bool { return findings[i].line < findings[j].line })
		for _, f := range findings {
			bad = true
			fmt.Fprintf(stdout, "%s:%d: %s [%s]\n", spec, f.line, f.msg, scenarioRule)
		}
	}
	return bad, nil
}

// untested returns the finding for s, if any: no test names it and it carries no exemption, or the exemption
// gives no reason.
func untested(s scenario, named map[string]bool) (finding, bool) {
	if m := untestedRE.FindStringSubmatch(s.text); m != nil {
		why := strings.TrimSpace(m[1])
		switch {
		case why == "":
			return finding{s.line, s.id + ": (untested: …) gives no reason; write why no test can check it"}, true
		case placeholderWhys[strings.ToLower(strings.TrimRight(why, "."))]:
			return finding{s.line, fmt.Sprintf("%s: (untested: %s) is a placeholder, not a reason; write why no test can check it", s.id, why)}, true
		}
		return finding{}, false
	}
	if named[s.id] {
		return finding{}, false
	}
	return finding{s.line, fmt.Sprintf("%s has no test naming it: %s", s.id, s.text)}, true
}

// testedIDs returns the scenario ids named in the product's test files.
func testedIDs(root string, l layout) (map[string]bool, error) {
	protected, err := protectedPaths(filepath.Join(root, "scripts/harness/protected-paths.txt"))
	if err != nil {
		return nil, err
	}
	var pathspecs []string
	add := func(prefix string, globs ...string) {
		for _, g := range globs {
			if g != "" { // an empty pathspec would match the whole repo
				pathspecs = append(pathspecs, prefix+g)
			}
		}
	}
	add("", l.list("PRODUCT_GLOBS")...)
	add("", l.list("WEB_GLOBS")...)
	add(":(exclude)", l.get("API_CLIENT_DIR"))
	add(":(exclude)", l.list("GENERATED_GLOBS")...)
	for _, p := range protected {
		add(":(exclude)", p.pattern)
	}
	files, err := gitFiles(root, pathspecs...)
	if err != nil {
		return nil, err
	}
	named := map[string]bool{}
	for _, f := range files {
		if !testFileRE.MatchString(path.Base(f)) {
			continue
		}
		regular, err := isRegular(root, f)
		if err != nil {
			return nil, err
		}
		if !regular {
			continue // a link names nothing: only a test in the tree counts
		}
		b, err := os.ReadFile(filepath.Join(root, f))
		if err != nil {
			return nil, err
		}
		for _, line := range bytes.Split(b, []byte("\n")) {
			if skippedTestRE.Match(line) {
				continue // a skipped or placeholder test checks nothing
			}
			for _, id := range scenarioIDs(line) {
				named[id] = true
			}
		}
	}
	return named, nil
}

// scenarioIDs returns the ids in b that stand as whole tokens: `001/US1-AS1-old` or `x001/US1-AS1` name nothing.
func scenarioIDs(b []byte) []string {
	glued := func(i int) bool {
		if i < 0 || i >= len(b) {
			return false
		}
		c := b[i]
		return c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || strings.IndexByte("_/.-", c) >= 0
	}
	var ids []string
	for _, m := range scenarioIDRE.FindAllIndex(b, -1) {
		if !glued(m[0]-1) && !glued(m[1]) {
			ids = append(ids, string(b[m[0]:m[1]]))
		}
	}
	return ids
}

// isRegular reports whether the repo-relative file f is a regular file, not a link or a directory.
func isRegular(root, f string) (bool, error) {
	fi, err := os.Lstat(filepath.Join(root, f))
	if err != nil {
		return false, err
	}
	return fi.Mode().IsRegular(), nil
}

// specParser holds what parseScenarios needs while it walks one spec.
type specParser struct {
	src        []byte
	lineStarts []int          // byte offset where each line starts
	stories    map[string]int // story number → its heading line
	findings   []finding
}

// parseScenarios returns the acceptance scenarios of spec number num and the findings about its structure.
func parseScenarios(num string, src []byte) ([]scenario, []finding, error) {
	p := &specParser{src: src, lineStarts: []int{0}, stories: map[string]int{}}
	for i, c := range src {
		if c == '\n' {
			p.lineStarts = append(p.lineStarts, i+1)
		}
	}
	doc := markdown.Parser().Parse(text.NewReader(src))
	var scenarios []scenario
	read := map[ast.Node]bool{} // Acceptance Scenarios paragraphs that sit in a story
	sawStory, inStory := false, false
	story, storyLine, count := "", 0, 0 // story is "" outside a story or in one without a usable number
	endStory := func() {
		if story != "" && count == 0 {
			p.add(storyLine, fmt.Sprintf("User Story %s has no numbered list under **Acceptance Scenarios**:, so nothing ties it to a test", story))
		}
		story, inStory = "", false
	}
	for n := doc.FirstChild(); n != nil; n = n.NextSibling() {
		switch n := n.(type) {
		case *ast.Heading:
			title := strings.TrimSpace(plainText(n, src))
			if n.Level > 3 {
				if storyRE.MatchString(title) {
					p.add(p.line(n), fmt.Sprintf(`%q is a level %d heading, so it starts no story; write "### User Story <n> - <title>"`, title, n.Level))
				}
				continue
			}
			endStory()
			if storyRE.MatchString(title) {
				sawStory, inStory = true, true
				story, storyLine, count = p.startStory(p.line(n), title), p.line(n), 0
			}
		case *ast.Paragraph:
			if !inStory || !scenariosRE.MatchString(strings.TrimSpace(plainText(n, src))) {
				continue
			}
			read[n] = true
			list, ok := n.NextSibling().(*ast.List)
			if story == "" || !ok || !list.IsOrdered() {
				continue
			}
			for item := list.FirstChild(); item != nil; item = item.NextSibling() {
				count++
				line := p.line(item)
				if line == 0 { // an empty item
					line = p.line(n)
				}
				scenarios = append(scenarios, scenario{fmt.Sprintf("%s/US%s-AS%d", num, story, count), line, p.itemText(item)})
			}
		}
	}
	endStory()
	if !sawStory {
		p.add(1, `no "### User Story <n> - <title>" section (heading level 1 to 3), so no scenario ties this spec to a test; spec-kit's User Scenarios section is mandatory`)
	}
	err := ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		para, ok := n.(*ast.Paragraph)
		if !ok || !entering {
			return ast.WalkContinue, nil
		}
		if !read[para] && scenariosRE.MatchString(strings.TrimSpace(plainText(para, src))) {
			p.add(p.line(para), "**Acceptance Scenarios** is not a top-level paragraph of a User Story section, so its scenarios have no id; move it under its story's heading")
		}
		return ast.WalkSkipChildren, nil
	})
	return scenarios, p.findings, err
}

func (p *specParser) add(line int, msg string) { p.findings = append(p.findings, finding{line, msg}) }

// startStory returns the number of the story headed title on line n, or "" (with a finding) when it has none
// or another story has it.
func (p *specParser) startStory(n int, title string) string {
	m := storyNumberRE.FindStringSubmatch(title)
	if m == nil {
		p.add(n, `user story heading has no number, so its scenarios have no id; write "### User Story <n> - <title>"`)
		return ""
	}
	num, err := strconv.Atoi(m[1])
	if err != nil { // only an overflow gets here
		p.add(n, "user story number "+m[1]+" is not a number")
		return ""
	}
	story := strconv.Itoa(num)
	if first, dup := p.stories[story]; dup {
		p.add(n, fmt.Sprintf("User Story %s is also the heading on line %d, so their scenario ids clash; renumber one", story, first))
		return ""
	}
	p.stories[story] = n
	return story
}

// line is the 1-based line where block n's first text starts, 0 when it has none.
func (p *specParser) line(n ast.Node) int {
	for ; n != nil; n = n.FirstChild() {
		if n.Type() == ast.TypeBlock && n.Lines().Len() > 0 {
			return sort.SearchInts(p.lineStarts, n.Lines().At(0).Start+1)
		}
	}
	return 0
}

// itemText is the source of a list item's own paragraphs on one line; nested lists are steps, not its text.
func (p *specParser) itemText(item ast.Node) string {
	var parts []string
	for c := item.FirstChild(); c != nil; c = c.NextSibling() {
		switch c.(type) {
		case *ast.Paragraph, *ast.TextBlock:
			for i := 0; i < c.Lines().Len(); i++ {
				seg := c.Lines().At(i)
				parts = append(parts, string(seg.Value(p.src)))
			}
		}
	}
	return strings.TrimSpace(whitespaceRuns.ReplaceAllString(strings.Join(parts, " "), " "))
}
