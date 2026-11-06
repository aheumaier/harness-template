package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckLinks(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		files map[string][]string
		want  []string // output lines; none means the check passes
	}{
		// The cases of the former scripts/test/check-links.bats.
		{"links and anchors resolve", map[string][]string{
			"docs/a.md": {"# Title", "## Some Section", `See [b](../b.md#other-heading "title") and [self](#some-section).`},
			"b.md":      {"## Other Heading"},
		}, nil},
		{"missing files and missing anchors", map[string][]string{
			"docs/a.md": {"See [gone](missing.md) and [b](../b.md#nope)."},
			"b.md":      {"## Real"},
		}, []string{
			"docs/a.md: broken link (missing.md): docs/missing.md does not exist [docs-links-resolve]",
			"docs/a.md: broken anchor (../b.md#nope): no heading #nope in docs/../b.md [docs-links-resolve]",
		}},
		{"external links, code spans and fenced blocks are ignored", map[string][]string{
			"a.md": {"Visit [x](https://example.com/nope) and `[y](gone.md)`.", "```", "[z](gone.md)", "```"},
		}, nil},
		// The fixtures (scripts/harness/fixtures/docs-*).
		{"fixture docs-broken-link", map[string][]string{
			"ARCHITECTURE.md": {"# Architecture", "## Layers"},
			"docs/moved.md":   {"# Moved doc", "", "See [the old rules](harness/old-rules.md#go-layers) and [layers](../ARCHITECTURE.md#no-such-heading)."},
		}, []string{
			"docs/moved.md: broken link (harness/old-rules.md#go-layers): docs/harness/old-rules.md does not exist [docs-links-resolve]",
			"docs/moved.md: broken anchor (../ARCHITECTURE.md#no-such-heading): no heading #no-such-heading in docs/../ARCHITECTURE.md [docs-links-resolve]",
		}},
		{"AGENTS.md without CLAUDE.md", map[string][]string{
			"apps/zz/AGENTS.md": {"# Area"},
			"AGENTS.md":         {"# Root"},
			"CLAUDE.md":         {"@AGENTS.md"},
			"api/AGENTS.md":     {"# Api"},
			"api/CLAUDE.md":     {"see @AGENTS.md"},
		}, []string{
			"api/AGENTS.md: no api/CLAUDE.md with an '@AGENTS.md' line next to it [docs-claude-imports-agents]",
			"apps/zz/AGENTS.md: no apps/zz/CLAUDE.md with an '@AGENTS.md' line next to it [docs-claude-imports-agents]",
		}},
		{"a root file names its link with ./", map[string][]string{"a.md": {"[x](gone.md)"}},
			[]string{"a.md: broken link (gone.md): ./gone.md does not exist [docs-links-resolve]"}},
		// What the shell version approximated.
		{"GitHub slugs: punctuation, code, links, Unicode, duplicates", map[string][]string{
			"a.md": {
				"## Go: layers & `make check`", "## [Linked](x.md) *heading*", "## Größe", "## Same", "## Same",
				"Setext", "------",
				"[a](#go-layers--make-check) [b](#linked-heading) [c](#größe) [d](#same) [e](#same-1) [f](#setext)",
			},
			"x.md": {""},
		}, nil},
		{"a third duplicate heading is -2, not -3", map[string][]string{
			"a.md": {"# S", "# S", "# S", "[a](#s-2) [b](#s-3)"},
		}, []string{"a.md: broken anchor (#s-3): no heading #s-3 in a.md [docs-links-resolve]"}},
		{"anchors are case sensitive and only for Markdown files", map[string][]string{
			"a.md":  {"# Title", "[a](#Title) [b](c.txt#anything) [c](dir#x) [d](a.md#)"},
			"c.txt": {"text"},
			"dir/x": {""},
		}, []string{"a.md: broken anchor (#Title): no heading #Title in a.md [docs-links-resolve]"}},
		{"headings in code are not anchors; ~~~ fences and indented code are code", map[string][]string{
			"a.md": {"```", "# Fenced", "```", "~~~", "[x](gone.md)", "~~~", "", "    [y](gone.md)", "", "[a](#fenced)"},
		}, []string{"a.md: broken anchor (#fenced): no heading #fenced in a.md [docs-links-resolve]"}},
		{"images and reference links are checked", map[string][]string{
			"a.md": {"![img](gone.png) [ref][r]", "", "[r]: also-gone.md"},
		}, []string{
			"a.md: broken link (gone.png): ./gone.png does not exist [docs-links-resolve]",
			"a.md: broken link (also-gone.md): ./also-gone.md does not exist [docs-links-resolve]",
		}},
		{"fixtures, node_modules and vendored spec-kit are skipped", map[string][]string{
			"scripts/harness/fixtures/x/files/a.md": {"[x](gone.md)"},
			"node_modules/p/a.md":                   {"[x](gone.md)"},
			".specify/templates/a.md":               {"[x](gone.md)"},
			".claude/skills/speckit-plan/SKILL.md":  {"[x](gone.md)"},
			"fixtures/AGENTS.md":                    {""},
		}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			bad, err := checkLinks(testWorld(t, &out), linkRepo(t, tt.files))
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

func TestCheckLinksTrackedFileDeleted(t *testing.T) {
	t.Parallel()
	root := linkRepo(t, map[string][]string{"a.md": {"[x](gone.md)"}, "gone.md": {"# Gone"}})
	gitT(t, root, "add", "-A")
	if err := os.Remove(filepath.Join(root, "gone.md")); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	bad, err := checkLinks(testWorld(t, &out), root)
	want := "a.md: broken link (gone.md): ./gone.md does not exist [docs-links-resolve]\n"
	if err != nil || !bad || out.String() != want {
		t.Errorf("bad=%v err=%v, output:\n%swant:\n%s", bad, err, out.String(), want)
	}
}

func TestCheckLinksExitCodes(t *testing.T) {
	t.Parallel()
	ok := linkRepo(t, map[string][]string{"a.md": {"# A"}})
	broken := linkRepo(t, map[string][]string{"a.md": {"[x](gone.md)"}})
	for _, tt := range []struct {
		args []string
		want int
	}{
		{[]string{"check-links", ok}, 0},
		{[]string{"check-links", broken}, 1},
		{[]string{"check-links", t.TempDir()}, 2}, // not a git repo: fail closed
		{[]string{"check-links"}, exitUsage},
	} {
		var stdout, stderr bytes.Buffer
		if code := runTest(t, tt.args, nil, &stdout, &stderr); code != tt.want {
			t.Errorf("%v: exit %d, want %d (%s%s)", tt.args, code, tt.want, stdout.String(), stderr.String())
		}
	}
}
