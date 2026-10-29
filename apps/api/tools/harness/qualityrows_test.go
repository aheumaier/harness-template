package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	qualityTestHeader = "| Domain | model | store | service | api | Tests | Docs | Updated | Notes |"
	qualityTestSep    = "|:-------|-------|-------|---------|-----|-------|------|---------|------:|"
	qualityTestMarker = "<!-- harness:quality-rows (generator marker, keep) -->"
)

func qualityTestRow(domain, grades, updated string) string {
	return "| " + domain + " | " + grades + " | t | AGENTS.md | " + updated + " | n |"
}

func qualityTestLink(d string) string { return "[" + d + "](../apps/api/internal/" + d + "/AGENTS.md)" }

// qualityTestRepo is a git repo with QUALITY.md holding rows and a file under apps/api/internal/<d>/ per domain.
func qualityTestRepo(t *testing.T, rows, domains []string) string {
	t.Helper()
	lines := append(append([]string{qualityTestHeader, qualityTestSep}, rows...), qualityTestMarker, "", "Grades: …")
	files := map[string][]string{qualityFile: lines}
	for _, d := range domains {
		files[domainsDir+d+"/x.go"] = []string{"package x"}
	}
	return linkRepo(t, files)
}

// wantFindings fails unless check printed exactly want (nothing when want is empty) and reported it.
func wantFindings(t *testing.T, check func(io.Writer) (bool, error), want []string) {
	t.Helper()
	var out bytes.Buffer
	bad, err := check(&out)
	if err != nil {
		t.Fatal(err)
	}
	if len(want) == 0 {
		if bad || out.Len() != 0 {
			t.Fatalf("want no findings, got bad=%v:\n%s", bad, out.String())
		}
		return
	}
	if got := strings.TrimSuffix(out.String(), "\n"); !bad || got != strings.Join(want, "\n") {
		t.Fatalf("got bad=%v:\n%s\nwant:\n%s", bad, got, strings.Join(want, "\n"))
	}
}

func TestQualityShape(t *testing.T) {
	link := qualityTestLink
	row := qualityTestRow
	tests := []struct {
		name    string
		rows    []string
		domains []string // apps/api/internal/<d>/x.go exists
		want    []string // output lines; none means the check passes
	}{
		{"no domains, no rows", nil, nil, nil},
		{"well-formed, bare name and link, any date", []string{
			row(link("billing"), "A | B | – | F", "2001-01-01"),
			"",
			row("orders", "– | – | – | –", "2099-12-31"),
		}, []string{"billing", "orders", "platform"}, nil},
		{"not well-formed", []string{
			row(link("billing"), "A+ | B | – | -", "2026-10-29"),
			"| " + link("orders") + " | A | A | A | A | 2026-10-29 |",
			row("Orders Team", "A | A | A | A", "2026-10-29"),
			row(link("users"), "A | A | A | A", "29.10.2026"),
			row("[users](../apps/api/internal/orders/AGENTS.md)", "A | A | A | A", "2026-13-01"),
		}, []string{"billing", "users"}, []string{
			`docs/QUALITY.md:3: model grade "A+" is not one of A–F or – (en dash U+2013, not -) [docs-quality-rows-well-formed]`,
			`docs/QUALITY.md:3: api grade "-" is not one of A–F or – (en dash U+2013, not -) [docs-quality-rows-well-formed]`,
			"docs/QUALITY.md:4: the row has 6 cells, the header 9: one cell per column, and no | inside a cell [docs-quality-rows-well-formed]",
			`docs/QUALITY.md:5: Domain "Orders Team" is not a domain name or a link to one: write [<domain>](../apps/api/internal/<domain>/AGENTS.md) [docs-quality-rows-well-formed]`,
			`docs/QUALITY.md:6: Updated "29.10.2026" is not a date (YYYY-MM-DD) [docs-quality-rows-well-formed]`,
			"docs/QUALITY.md:7: Domain link [users](../apps/api/internal/orders/AGENTS.md) does not point at its domain: write [users](../apps/api/internal/users/AGENTS.md) [docs-quality-rows-well-formed]",
			`docs/QUALITY.md:7: Updated "2026-13-01" is not a date (YYYY-MM-DD) [docs-quality-rows-well-formed]`,
			"docs/QUALITY.md:7: a second row for domain users (first at line 6): keep one [docs-quality-rows-match-domains]",
		}},
		{"gone, twice, missing, platform", []string{
			row(link("billing"), "A | A | A | A", "2026-10-29"),
			row(link("billing"), "B | B | B | B", "2026-10-29"),
			row(link("platform"), "A | A | A | A", "2026-10-29"),
		}, []string{"orders", "platform"}, []string{
			"docs/QUALITY.md:3: domain billing is gone (no git-known file under apps/api/internal/billing/): delete its row [docs-quality-rows-match-domains]",
			"docs/QUALITY.md:4: a second row for domain billing (first at line 3): keep one [docs-quality-rows-match-domains]",
			"docs/QUALITY.md:5: domain platform is gone (no git-known file under apps/api/internal/platform/): delete its row [docs-quality-rows-match-domains]",
			"docs/QUALITY.md:6: domain orders (apps/api/internal/orders/) has no row: add one before the marker and grade it [docs-quality-rows-match-domains]",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := qualityTestRepo(t, tt.rows, tt.domains)
			wantFindings(t, func(w io.Writer) (bool, error) { return qualityShape(root, w) }, tt.want)
		})
	}
}

func TestQualityFresh(t *testing.T) {
	// 2026-10-30 locally, 2026-10-29 23:30 in UTC: the rows are measured against the UTC date.
	today := time.Date(2026, 10, 30, 0, 30, 0, 0, time.FixedZone("CET", 3600))
	row := qualityTestRow
	tests := []struct {
		name string
		rows []string
		want []string
	}{
		{"30 days old is fresh", []string{row("billing", "A | A | A | A", "2026-09-29"), row("orders", "– | – | – | –", "2026-10-29")}, nil},
		{"stale, future, year 1", []string{
			row("billing", "A | A | A | A", "2026-09-28"),
			row("orders", "A | A | A | A", "2026-10-30"),
			row("users", "A | A | A | A", "0001-01-01"),
		}, []string{
			"docs/QUALITY.md:3: Updated 2026-09-28 is 31 days old (more than 30): re-grade the row [docs-quality-row-fresh]",
			"docs/QUALITY.md:4: Updated 2026-10-30 is after today (2026-10-29) [docs-quality-row-fresh]",
			"docs/QUALITY.md:5: Updated 0001-01-01 is 739917 days old (more than 30): re-grade the row [docs-quality-row-fresh]",
		}},
		{"rows that are not well-formed are the shape check's", []string{
			"| billing | A | A | 2001-01-01 |",
			row("orders", "A | A | A | A", "29.10.2001"),
		}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := qualityTestRepo(t, tt.rows, nil)
			wantFindings(t, func(w io.Writer) (bool, error) { return qualityFresh(root, today, w) }, tt.want)
		})
	}
}

// The row scripts/new-domain.sh renders from its template, under the real QUALITY.md header, passes both modes
// on the day it is written: the generator and the sensor agree on the table's shape.
func TestQualityRowTemplateMatchesHeader(t *testing.T) {
	const repo = "../../../.."
	tmpl, err := os.ReadFile(filepath.Join(repo, "scripts/templates/new-domain/quality-row.md.tmpl"))
	if err != nil {
		t.Fatal(err)
	}
	quality, err := os.ReadFile(filepath.Join(repo, qualityFile))
	if err != nil {
		t.Fatal(err)
	}
	// The placeholders render() in scripts/new-domain.sh fills in for this template.
	rendered := strings.NewReplacer("__NAME__", "billing_run", "__TODAY__", "2026-10-29").Replace(string(tmpl))
	if strings.Contains(rendered, "__") {
		t.Fatalf("quality-row.md.tmpl has a placeholder this test does not fill in:\n%s", rendered)
	}
	lines := strings.Split(string(quality), "\n")
	var table []string // the real header and separator, the rendered row, the marker
	for i, l := range lines {
		if strings.HasPrefix(l, "| Domain |") && i+1 < len(lines) {
			table = append(table, l, lines[i+1])
			table = append(table, strings.Split(strings.TrimSuffix(rendered, "\n"), "\n")...)
			table = append(table, qualityTestMarker)
			break
		}
	}
	if table == nil {
		t.Fatalf("%s has no line starting with `| Domain |`", qualityFile)
	}
	root := linkRepo(t, map[string][]string{qualityFile: table, domainsDir + "billing_run/AGENTS.md": {"# billing_run"}})
	today := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	wantFindings(t, func(w io.Writer) (bool, error) { return qualityShape(root, w) }, nil)
	wantFindings(t, func(w io.Writer) (bool, error) { return qualityFresh(root, today, w) }, nil)
}

func TestQualityRowsFailsClosed(t *testing.T) {
	today := time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name  string
		lines []string // nil: no QUALITY.md
		want  string
	}{
		{"no file", nil, "no such file"},
		{"no header", []string{"| Area | Grade |", "<!-- harness:quality-rows -->"}, "no table header"},
		{"no Updated column", []string{"| Domain | model | store | service | api |", "<!-- harness:quality-rows -->"}, `no "Updated" column`},
		{"no marker", []string{"| Domain | model | store | service | api | Updated |", "|---|"}, "no `harness:quality-rows` marker"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := map[string][]string{"README.md": {""}}
			if tt.lines != nil {
				files[qualityFile] = tt.lines
			}
			root := linkRepo(t, files)
			for mode, check := range map[string]func(io.Writer) (bool, error){
				"shape": func(w io.Writer) (bool, error) { return qualityShape(root, w) },
				"fresh": func(w io.Writer) (bool, error) { return qualityFresh(root, today, w) },
			} {
				if _, err := check(io.Discard); err == nil || !strings.Contains(err.Error(), tt.want) {
					t.Fatalf("%s: want an error containing %q, got %v", mode, tt.want, err)
				}
			}
		})
	}
}
