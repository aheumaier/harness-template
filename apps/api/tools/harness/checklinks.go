package main

// check-links: every relative Markdown link and #anchor in tracked *.md files resolves (docs-links-resolve),
// and every AGENTS.md has a sibling CLAUDE.md with an `@AGENTS.md` line (docs-claude-imports-agents; Claude
// Code reads CLAUDE.md, other agents AGENTS.md). Run from `make docs-links` and the fast-check hook.
// goldmark parses each file once (links in code spans and blocks are not links), so the cost grows with the
// docs, not with a few processes per anchor link as in the shell version it replaces (ADR 0012).
// External links (scheme:) are not checked: they are not deterministic. Anchors are GitHub's heading slugs.

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

var (
	// Vendored spec-kit (ADR 0011) is not ours to fix; gitFiles skips fixtures (skippedPath).
	mdSpec   = []string{"*.md", ":!:.specify/templates/*", ":!:.claude/skills/speckit-*"}
	schemeRE = regexp.MustCompile(`^[a-z][a-z0-9+.-]*:`)
)

func init() {
	commands.register(rootSensor("check-links", "docs-links-resolve, docs-claude-imports-agents (make docs-links)", checkLinks))
}

// checkLinks prints one line per finding to stdout and reports whether there was any. root is the repo.
func checkLinks(w world, root string) (bool, error) {
	c := &linkChecker{root: root, out: w.stdout, slugs: map[string]map[string]bool{}}
	mds, err := w.gitFiles(root, mdSpec...)
	if err != nil {
		return false, err
	}
	for _, md := range mds {
		if err := c.file(md); err != nil {
			return false, err
		}
	}
	agents, err := w.gitFiles(root, "*AGENTS.md")
	if err != nil {
		return false, err
	}
	for _, a := range agents {
		if path.Base(a) != "AGENTS.md" {
			continue
		}
		claude := path.Join(path.Dir(a), "CLAUDE.md")
		if b, err := os.ReadFile(filepath.Join(root, claude)); err != nil || !hasLine(b, "@AGENTS.md") {
			c.report("%s: no %s with an '@AGENTS.md' line next to it [docs-claude-imports-agents]", a, claude)
		}
	}
	return c.bad, nil
}

type linkChecker struct {
	root  string
	out   io.Writer
	bad   bool
	slugs map[string]map[string]bool // file (repo-relative, cleaned) → its heading slugs
}

func (c *linkChecker) report(format string, a ...any) {
	fmt.Fprintf(c.out, format+"\n", a...)
	c.bad = true
}

// file checks the links of one Markdown file; md is relative to the repo.
func (c *linkChecker) file(md string) error {
	src, err := os.ReadFile(filepath.Join(c.root, md))
	if err != nil {
		return err
	}
	doc := markdown.Parser().Parse(text.NewReader(src))
	var targets []string
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			switch n := n.(type) {
			case *ast.Link:
				targets = append(targets, string(n.Destination))
			case *ast.Image:
				targets = append(targets, string(n.Destination))
			}
		}
		return ast.WalkContinue, nil
	})
	for _, target := range targets {
		if target == "" || schemeRE.MatchString(target) {
			continue
		}
		p, anchor, hasAnchor := strings.Cut(target, "#")
		file := md
		if p != "" {
			file = path.Dir(md) + "/" + p // as written, for the message
			if _, err := os.Stat(filepath.Join(c.root, file)); err != nil {
				c.report("%s: broken link (%s): %s does not exist [docs-links-resolve]", md, target, file)
				continue
			}
		}
		if !hasAnchor || anchor == "" || !strings.HasSuffix(file, ".md") {
			continue
		}
		slugs, ok, err := c.headingSlugs(path.Clean(file))
		if err != nil {
			return err
		}
		if ok && !slugs[anchor] {
			c.report("%s: broken anchor (%s): no heading #%s in %s [docs-links-resolve]", md, target, anchor, file)
		}
	}
	return nil
}

// headingSlugs returns the GitHub anchors of file's headings; ok is false when file is not a regular file.
func (c *linkChecker) headingSlugs(file string) (slugs map[string]bool, ok bool, err error) {
	if s, cached := c.slugs[file]; cached {
		return s, s != nil, nil
	}
	full := filepath.Join(c.root, file)
	if fi, err := os.Stat(full); err != nil || !fi.Mode().IsRegular() {
		c.slugs[file] = nil
		return nil, false, nil
	}
	src, err := os.ReadFile(full)
	if err != nil {
		return nil, false, err
	}
	slugs = map[string]bool{}
	seen := map[string]int{}
	doc := markdown.Parser().Parse(text.NewReader(src))
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if h, ok := n.(*ast.Heading); ok && entering {
			slugs[uniqueSlug(slug(plainText(h, src)), seen)] = true
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})
	c.slugs[file] = slugs
	return slugs, true, nil
}

// slug is GitHub's heading anchor (github-slugger): lower case, punctuation and symbols dropped except `-`
// and `_`, every space a `-`.
func slug(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r == ' ':
			b.WriteByte('-')
		case r == '-' || r == '_' || unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsMark(r):
			b.WriteRune(r)
		}
	}
	return b.String()
}

// uniqueSlug numbers repeated slugs as GitHub does: x, x-1, x-2, ….
func uniqueSlug(s string, seen map[string]int) string {
	u := s
	for {
		if _, dup := seen[u]; !dup {
			break
		}
		seen[s]++
		u = fmt.Sprintf("%s-%d", s, seen[s])
	}
	seen[u] = 0
	return u
}
