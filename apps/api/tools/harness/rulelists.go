package main

// rule-lists (harness-rule-lists-match): the agent prompts that copy rules.md by hand (ruleListFiles) skip
// only what a rule really gates, and keep skipping what requiredSkips says they must. Each file's `## Skip`
// section runs to the next heading of level 1 or 2; goldmark parses it, so a code span may wrap lines or use
// double backticks. In it, a code span holding
//
//	<rule-id>      a lowercase token with at least one hyphen (`go-layers`, not `dupl`, `*.gen.go` or a path)
//	               is a rule id: it must have a `### <rule-id>` heading in rules.md;
//	## <section>   a rules.md section title (the heading text before any ` (`) claims the whole section:
//	               every `### <rule-id>` under it must also be named in the Skip section.
//
// Every rule id and claim requiredSkips lists for the file must be there. Run from `make rule-lists` (make
// check). Fails closed (exit 2) when rules.md holds no rules or repeats an id, a pattern matches no git-known
// file, a file has no `## Skip` section, or requiredSkips and the files or rules.md disagree.

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

const ruleListRule = "harness-rule-lists-match"

// ruleListFiles are the hand-copied rule lists (git pathspec globs); every file they match has an entry in
// requiredSkips.
var ruleListFiles = []string{".claude/skills/review-*/SKILL.md", ".claude/agents/reuse-reviewer.md"}

// requiredSkips is what each file's Skip section must keep naming: rule ids, and `## <section>` claims (whose
// rules must then all be named, so a new rule there joins the list). The lists drifted by omission (exec plan
// 0003, Phase 3 gate: review-modularity never skipped `repo-no-clones` and `repo-no-orphan-scripts`), which no
// check of what a list names can see. A file may skip more. An entry joins when a new sensor takes over a
// reviewer's finding class, and leaves only in a harness PR that says why the reviewer reports it again.
var requiredSkips = map[string][]string{
	".claude/agents/reuse-reviewer.md": {
		"## Duplication", "go-no-dead-code", "web-no-dead-code", "repo-no-orphan-scripts", "go-layers",
		"go-domain-isolation", "go-db-only-in-store", "go-http-only-in-api", "web-domain-pure", "web-no-cross-app",
		"web-no-direct-http",
	},
	".claude/skills/review-data-handling/SKILL.md": {
		"go-structured-logging", "go-db-only-in-store", "repo-no-secrets", "api-errors-problem-json",
		"api-camel-case-properties", "db-no-destructive-migration",
	},
	".claude/skills/review-modularity/SKILL.md": {
		"## Duplication", "go-layers", "go-domain-isolation", "go-db-only-in-store", "go-http-only-in-api",
		"go-test-helpers-only-in-tests", "web-no-direct-http", "web-domain-pure", "web-no-cross-app",
		"go-no-dead-code", "web-no-dead-code", "repo-no-orphan-scripts",
	},
	".claude/skills/review-security/SKILL.md": {
		"## Secrets and dependencies", "go-sec", "go-no-known-vulns", "api-errors-problem-json",
		"go-structured-logging", "ci-workflows-valid",
	},
}

var ruleIDRE = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)+$`)

// ruleListsRepo holds rules.md's rules for checking the lists.
type ruleListsRepo struct {
	sections map[string][]string // section title → its rule ids, in order
	known    map[string]bool     // rule id → true
}

func init() {
	commands.register(rootSensor("rule-lists", "harness-rule-lists-match: skip lists in review agents (make rule-lists)", ruleLists))
}

// ruleLists prints one line per finding to stdout and reports whether there was any. root is the repo.
func ruleLists(w world, root string) (bool, error) {
	return checkRuleLists(w, root, requiredSkips)
}

// checkRuleLists is ruleLists with the required entries as a parameter (the tests use their own).
func checkRuleLists(w world, root string, required map[string][]string) (bool, error) {
	src, err := os.ReadFile(filepath.Join(root, rulesFile))
	if err != nil {
		return false, err
	}
	headings, err := ruleHeadings(src)
	if err != nil {
		return false, err
	}
	r := ruleListsRepo{sections: map[string][]string{}, known: map[string]bool{}}
	for _, h := range headings {
		if h.first != 0 {
			return false, fmt.Errorf("%s:%d: a second `### %s` heading (run make fixture-claims)", rulesFile, h.line, h.id)
		}
		r.sections[h.section] = append(r.sections[h.section], h.id)
		r.known[h.id] = true
	}
	var files []string
	for _, p := range ruleListFiles {
		matched, err := w.gitKnown(root, p)
		if err != nil {
			return false, err
		}
		if len(matched) == 0 {
			return false, fmt.Errorf("no git-known file matches %s: update ruleListFiles in rulelists.go", p)
		}
		files = append(files, matched...)
	}
	if err := r.checkRequired(files, required); err != nil {
		return false, err
	}
	bad := false
	for _, f := range files {
		b, err := r.file(root, f, required[f], w.stdout)
		if err != nil {
			return false, err
		}
		bad = bad || b
	}
	return bad, nil
}

// checkRequired fails closed unless required has an entry for exactly the files, each naming a rule id or a
// section rules.md has: a renamed rule must not silently stop being required.
func (r ruleListsRepo) checkRequired(files []string, required map[string][]string) error {
	for _, f := range files {
		if _, ok := required[f]; !ok {
			return fmt.Errorf("%s has no entry in requiredSkips (rulelists.go): list the rules its Skip section must keep", f)
		}
	}
	for f, entries := range required {
		if !slices.Contains(files, f) {
			return fmt.Errorf("requiredSkips (rulelists.go) names %s, which no git-known file in ruleListFiles is", f)
		}
		for _, e := range entries {
			title, claim := strings.CutPrefix(e, "## ")
			if claim && r.sections[title] == nil || !claim && !r.known[e] {
				return fmt.Errorf("requiredSkips (rulelists.go) lists `%s` for %s, which %s does not have: update the entry", e, f, rulesFile)
			}
		}
	}
	return nil
}

// file checks one file's `## Skip` sections against rules.md and its required entries.
func (r ruleListsRepo) file(root, file string, required []string, out io.Writer) (bool, error) {
	src, err := os.ReadFile(filepath.Join(root, file))
	if err != nil {
		return false, err
	}
	bad := false
	report := func(line int, format string, a ...any) {
		printFinding(out, file, line, ruleListRule, format, a...)
		bad = true
	}
	var (
		skipLine int                // line of the first `## Skip` heading, 0 for none
		named    []string           // rule ids the Skip sections name
		claims   = map[string]int{} // claimed section → line
		inSkip   bool
	)
	doc := markdown.Parser().Parse(text.NewReader(src))
	for n := doc.FirstChild(); n != nil; n = n.NextSibling() {
		if h, ok := n.(*ast.Heading); ok && h.Level <= 2 {
			inSkip = h.Level == 2 && strings.TrimSpace(plainText(h, src)) == "Skip"
			if inSkip && skipLine == 0 {
				skipLine = lineAt(src, h.Lines().At(0).Start)
			}
			continue
		}
		if !inSkip {
			continue
		}
		_ = ast.Walk(n, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
			span, ok := n.(*ast.CodeSpan)
			if !ok || !entering {
				return ast.WalkContinue, nil
			}
			tok, line := codeSpanText(span, src)
			switch title, claim := strings.CutPrefix(tok, "## "); {
			case claim && r.sections[title] == nil:
				report(line, "`%s` is no section of %s: name a `## ` heading's text before any ` (`", tok, rulesFile)
			case claim:
				claims[title] = line
			case ruleIDRE.MatchString(tok):
				named = append(named, tok)
				if !r.known[tok] {
					report(line, "`%s` is no rule id in %s: use the id of its `### ` heading, or drop it", tok, rulesFile)
				}
			}
			return ast.WalkSkipChildren, nil
		})
	}
	if skipLine == 0 {
		return false, fmt.Errorf("%s has no `## Skip` section", file)
	}
	titles := make([]string, 0, len(claims))
	for title := range claims {
		titles = append(titles, title)
	}
	slices.Sort(titles) // deterministic output
	for _, title := range titles {
		for _, id := range r.sections[title] {
			if !slices.Contains(named, id) {
				report(claims[title], "skips `## %s` but does not name its rule `%s`: add it to the Skip list", title, id)
			}
		}
	}
	for _, e := range required {
		if title, claim := strings.CutPrefix(e, "## "); claim && claims[title] == 0 || !claim && !slices.Contains(named, e) {
			report(skipLine, "the Skip list must keep `%s` (requiredSkips in rulelists.go): add it back, or remove it there in a harness PR that says why the reviewer reports it again", e)
		}
	}
	return bad, nil
}

// codeSpanText is a code span's content with its whitespace (a wrapped line, the padding of a
// double-backtick span) collapsed and backticks inside it trimmed, and the line it starts on.
func codeSpanText(span *ast.CodeSpan, src []byte) (string, int) {
	var b strings.Builder
	start := -1
	for c := span.FirstChild(); c != nil; c = c.NextSibling() {
		if t, ok := c.(*ast.Text); ok {
			if start < 0 {
				start = t.Segment.Start
			}
			b.Write(t.Segment.Value(src))
			b.WriteByte(' ')
		}
	}
	tok := strings.Join(strings.Fields(strings.Trim(strings.TrimSpace(b.String()), "`")), " ")
	return tok, lineAt(src, max(start, 0))
}

// lineAt is the 1-based line of byte offset off in src.
func lineAt(src []byte, off int) int { return 1 + bytes.Count(src[:off], []byte("\n")) }
