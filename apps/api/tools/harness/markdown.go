package main

// The Markdown reading the sensors share: goldmark parsing (check-links, specs-tested, rule-lists), plain line
// checks (check-links, integrity-check) and rules.md's rule headings (fixture-claims, rule-lists).

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
)

var markdown = goldmark.New(goldmark.WithExtensions(extension.GFM))

// plainText is the rendered text of an inline tree: text, code spans, link and emphasis text; no raw HTML.
func plainText(n ast.Node, src []byte) string {
	var b strings.Builder
	_ = ast.Walk(n, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := n.(type) {
		case *ast.Text:
			b.Write(n.Segment.Value(src))
		case *ast.String:
			b.Write(n.Value)
		case *ast.RawHTML:
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})
	return b.String()
}

// hasLine reports whether b holds line as a whole line, split on \n only (a CRLF file never matches).
func hasLine(b []byte, line string) bool {
	for _, l := range bytes.Split(b, []byte("\n")) {
		if string(l) == line {
			return true
		}
	}
	return false
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
