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
// Tests: the git-known files productTests allows (testfile.go, the predicate `harness test-file` guards the
// test-writer's writes with): Go, JS/TS and Swift test files (testFileRE, swiftTestRE) under PRODUCT_GLOBS and WEB_GLOBS of
// scripts/harness/layout.env, minus node_modules/, fixtures/, API_CLIENT_DIR, GENERATED_GLOBS and the harness
// files of protected-paths.txt, so the harness's own tests and fixtures cannot satisfy an id. A test
// names an id when the id appears in it as a whole token (no letter, digit, `_`, `/`, `.` or `-` next to it),
// in a title, a string or a `// spec: <id>` comment; test syntax is not parsed. A line that also skips or
// defers its test (skipsTest), and in a .swift file every line inside a bare `#if false`, names nothing. Other ways to compile code out are not read (tech-debt TD-150). The check is per line: a skip on another line than
// the id (an XCTSkip in the test body under a `// spec:` comment) is not seen.
// Whether a test really checks its scenario is review's call (ADR 0019).
//
// A scenario passes when its text ends with `(untested: <reason>)`, or a test names its id. With db in STACK, a
// scenario that does not end with `(unit: <reason>)` is one a user sees, and only a test that runs the app
// counts (scenariokinds.go, ADR 0049); one only other tests name is a finding that names one of them. A reason
// may hold one level of parentheses; an empty or placeholder reason (n/a, tbd, todo, …) is a finding, and so
// are both markers in one scenario. Run from
// `make specs-tested` (make check when specs/ or a test file changed, make check-all). Fails closed (exit 2) on
// an unreadable layout, protected-paths.txt or spec, or when git fails; no specs is a pass.

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

const scenarioRule = "specs-scenarios-tested"

var (
	scenarioIDRE    = regexp.MustCompile(`[0-9]{3,}/US[0-9]+-AS[0-9]+`)
	swiftIfRE       = regexp.MustCompile(`^\s*#if\b`)
	swiftEndifRE    = regexp.MustCompile(`^\s*#endif\b`)
	swiftElseRE     = regexp.MustCompile(`^\s*#else(if)?\b`)
	storyRE         = regexp.MustCompile(`(?i)^user story\b`)
	storyNumberRE   = regexp.MustCompile(`(?i)^user story[ \t]+([0-9]+)\b`)
	scenariosRE     = regexp.MustCompile(`(?i)^acceptance scenarios\b`)
	markerRE        = regexp.MustCompile(`(?:^|[^\]])\((untested|unit):((?:[^()]|\([^()]*\))*)\)[ \t]*$`)
	markerNameRE    = regexp.MustCompile(`\((untested|unit):`)
	whitespaceRuns  = regexp.MustCompile(`\s+`)
	placeholderWhys = map[string]bool{"": true, "na": true, "tbd": true, "todo": true, "none": true, "x": true} // keyed by placeholderKey
	htmlTagRE       = regexp.MustCompile(`<[^>]*>`)
)

// markerWhy says what the reason of each marker gives.
var markerWhy = map[string]string{"untested": "write why no test can check it", "unit": "write why a unit test proves it"}

// tracing is what the product's tests say about the scenario ids they name.
type tracing struct {
	named    map[string]string // id → the first test file naming it
	app      map[string]bool   // ids a test that runs the app names
	at       appTests          // with at.integration (db), a scenario without (unit: …) needs such a test
	unmarked bool              // a scenario without a marker was read: without db, the notice is due
}

// scenario is one acceptance scenario: its id, where it starts and its source text (lines joined).
type scenario struct {
	id   string
	line int
	text string
}

func init() {
	commands.register(rootSensor("specs-tested", "specs-scenarios-tested (make specs-tested)", scenariosTested))
}

// scenariosTested prints one line per finding to stdout and reports whether there was any. root is the repo.
func scenariosTested(w world, root string) (bool, error) {
	pt, err := loadProductTests(w, root)
	if err != nil {
		return false, err
	}
	specs, err := w.gitFiles(root, "specs/*/spec.md")
	if err != nil {
		return false, err
	}
	sort.Strings(specs)
	var tr *tracing // read lazily: no spec, no test scan
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
			printFinding(w.stdout, spec, 1, scenarioRule, "is not a regular file (a symlink?); commit the spec itself")
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
		if tr == nil && len(scenarios) > 0 {
			at, err := loadAppTests(w, root)
			if err != nil {
				return false, err
			}
			if tr, err = testedIDs(w, root, pt, at); err != nil {
				return false, err
			}
		}
		for _, s := range scenarios {
			if f, ok := tr.finding(s); ok {
				findings = append(findings, f)
			}
		}
		sort.SliceStable(findings, func(i, j int) bool { return findings[i].line < findings[j].line })
		for _, f := range findings {
			bad = true
			printFinding(w.stdout, spec, f.line, scenarioRule, "%s", f.msg)
		}
	}
	if tr != nil && tr.unmarked && !tr.at.integration {
		fmt.Fprintln(w.stdout, tr.at.notice())
	}
	return bad, nil
}

// finding returns the finding for s, if any: it carries both markers, a marker that does not end it or gives no
// reason, no marker and no test naming it, or no (unit: …) marker in a STACK that runs the app and no test that
// runs it naming it.
func (tr *tracing) finding(s scenario) (finding, bool) {
	names := markerNameRE.FindAllStringSubmatch(s.text, -1)
	if len(names) > 1 && slices.ContainsFunc(names, func(m []string) bool { return m[1] != names[0][1] }) {
		return finding{s.line, s.id + ": carries both (unit: …) and (untested: …); keep the one that holds"}, true
	}
	marker := ""
	if m := markerRE.FindStringSubmatch(s.text); m != nil {
		why := strings.TrimSpace(m[2])
		marker = m[1]
		switch {
		case why == "":
			return finding{s.line, fmt.Sprintf("%s: (%s: …) gives no reason; %s", s.id, marker, markerWhy[marker])}, true
		case placeholderWhys[placeholderKey(why)]:
			return finding{s.line, fmt.Sprintf("%s: (%s: %s) is a placeholder, not a reason; %s", s.id, marker, why, markerWhy[marker])}, true
		case marker == "untested":
			return finding{}, false
		}
	} else if len(names) > 0 && strings.Contains(s.text, "]("+names[0][1]+":") {
		return finding{s.line, fmt.Sprintf("%s: (%s: …) in a link target is no marker; end the scenario with (%s: <reason>)", s.id, names[0][1], names[0][1])}, true
	} else if len(names) > 0 {
		return finding{s.line, fmt.Sprintf("%s: (%s: …) must end the scenario, with at most one level of parentheses in its reason", s.id, names[0][1])}, true
	} else {
		tr.unmarked = true
	}
	file, ok := tr.named[s.id]
	switch {
	case !ok:
		return finding{s.line, fmt.Sprintf("%s has no test naming it: %s", s.id, s.text)}, true
	case marker == "unit" || !tr.at.integration || tr.app[s.id]:
		return finding{}, false
	}
	return finding{s.line, fmt.Sprintf("%s is a scenario a user sees and only other tests name it (%s); name it in %s, or end it with (unit: <reason>); "+
		"a marker added after the test commit lifts the freeze on this spec's tests (specs-tests-frozen), so prefer a new test commit", s.id, file, tr.at.kinds())}, true
}

// placeholderKey keeps a reason's letters and digits, lowercased, so no punctuation, symbol, invisible character
// or HTML tag makes a placeholder a reason.
func placeholderKey(why string) string {
	return strings.ToLower(strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return -1
	}, htmlTagRE.ReplaceAllString(why, "")))
}

// testedIDs reads the scenario ids the product's test files name, and which of them a test that runs the app
// names.
func testedIDs(w world, root string, pt productTests, at appTests) (*tracing, error) {
	files, err := w.gitFiles(root, testFilePathspecs...)
	if err != nil {
		return nil, err
	}
	tr := &tracing{named: map[string]string{}, app: map[string]bool{}, at: at}
	for _, f := range files {
		if pt.why(f) != "" {
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
		ids := fileIDs(f, b)
		if len(ids) == 0 {
			continue
		}
		app := false
		if tr.at.integration {
			if app, err = at.runsApp(root, f); err != nil {
				return nil, fmt.Errorf("%s: %w", f, err)
			}
		}
		for _, id := range ids {
			if _, ok := tr.named[id]; !ok {
				tr.named[id] = f
			}
			tr.app[id] = tr.app[id] || app
		}
	}
	return tr, nil
}

// fileIDs returns the scenario ids the test file f with content b names, on lines that skip no test and are not
// compiled out.
func fileIDs(f string, b []byte) []string {
	var ids []string
	swift := strings.HasSuffix(f, ".swift")
	off := 0 // #if depth inside a Swift `#if false`: compiled out, so it names nothing
	for _, line := range bytes.Split(b, []byte("\n")) {
		if swift {
			switch {
			case off > 0 && swiftIfRE.Match(line):
				off++
			case off > 0 && swiftEndifRE.Match(line):
				off--
			case off == 1 && swiftElseRE.Match(line):
				off = 0 // the #else branch of `#if false` is compiled
			case off == 0 && swiftIfFalseRE.Match(line):
				off = 1
			}
			if off > 0 {
				continue
			}
		}
		if skipsTest(f, line) {
			continue // a skipped or placeholder test checks nothing
		}
		ids = append(ids, scenarioIDs(line)...)
	}
	return ids
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
