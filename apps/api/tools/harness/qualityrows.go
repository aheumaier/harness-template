package main

// quality-rows: docs/QUALITY.md grades every backend domain per layer, and the grades are recent (exec plan
// 0003 P3.5, ADR 0028). The rows sit between the table header (the line starting `| Domain |`) and the
// `harness:quality-rows` marker that `make new-domain` inserts before. Columns are found by header name:
//
//	Domain   `[<domain>](../<domains>/<domain>/AGENTS.md)` or a bare <domain>, where <domains> is the
//	         domains' parent directory from scripts/harness/layout.env (API_DIR plus LAYER_GLOB without its
//	         /*/*; apps/api/internal in the template)
//	model, store, service, api   A–F, or – (en dash) until the first grade
//	Updated  YYYY-MM-DD, the day of the last grading
//
// Cells hold no `|` (QUALITY.md says so): a row is split on every pipe. Two modes:
//
//	qualityShape (-no-clock; make docs-quality-shape, in check-all and the CI generator probe), deterministic:
//	  docs-quality-rows-well-formed    one cell per column, a domain name or a link to its AGENTS.md, grades
//	                                   A–F or –, Updated in YYYY-MM-DD
//	  docs-quality-rows-match-domains  one row per domain: none for a domain without git-known files under
//	                                   <domains>/<domain>/ (platform is no domain), none missing, none twice
//	qualityFresh (make docs-quality, the weekly drift sweep only), reads the clock:
//	  docs-quality-row-fresh           Updated is at most qualityMaxAgeDays before today and not after it;
//	                                   rows that are not well-formed are left to the shape mode
//
// today is a parameter, so the tests never read the clock. Both modes fail closed (exit 2) when QUALITY.md, its
// table header, a needed column or the marker is missing; the shape mode also when layout.env is missing or
// malformed (it reads <domains> from there; the age check needs no domain).

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

const (
	qualityFile       = "docs/QUALITY.md"
	qualityMarker     = "harness:quality-rows"
	qualityMaxAgeDays = 30
)

var (
	qualityGrades  = []string{"model", "store", "service", "api"}
	qualityGradeRE = regexp.MustCompile(`^([A-F]|–)$`)
	qualityLinkRE  = regexp.MustCompile(`^\[([^\]]*)\]\(([^)]*)\)$`)
	qualityDateRE  = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`)
	domainNameRE   = regexp.MustCompile(`^[a-z][a-z0-9_]*[a-z0-9]$`) // scripts/new-domain.sh's NAME
	tableSepRE     = regexp.MustCompile(`^\|[\s:|-]*-[\s:|-]*$`)     // `|---|`, `| :-- |`
)

// qualityTable is the parsed table: header columns by name, the rows with their line numbers, the marker line.
type qualityTable struct {
	col    map[string]int // header name → cell index
	width  int            // number of header cells
	rows   []qualityRow
	marker int
}

type qualityRow struct {
	line  int
	cells []string
}

func (t *qualityTable) cell(r qualityRow, name string) string { return r.cells[t.col[name]] }

type qualityChecker struct {
	out     io.Writer
	domains string // the domains' parent directory with a trailing `/` (layout.domainsDir)
	bad     bool
}

func (c *qualityChecker) report(line int, rule, format string, a ...any) {
	fmt.Fprintf(c.out, "%s:%d: %s [%s]\n", qualityFile, line, fmt.Sprintf(format, a...), rule)
	c.bad = true
}

// qualityShape prints one line per well-formed or match-domains finding and reports whether there was any.
func qualityShape(root string, stdout io.Writer) (bool, error) {
	t, err := readQualityTable(root)
	if err != nil {
		return false, err
	}
	l, err := loadLayout(root)
	if err != nil {
		return false, err
	}
	dir, err := l.domainsDir()
	if err != nil {
		return false, err
	}
	c := &qualityChecker{out: stdout, domains: dir + "/"}
	domains, err := backendDomains(root, c.domains)
	if err != nil {
		return false, err
	}
	seen := map[string]int{} // domain → line of its first row
	for _, r := range t.rows {
		d := c.wellFormed(t, r)
		if d == "" {
			continue
		}
		switch first, dup := seen[d]; {
		case dup:
			c.report(r.line, "docs-quality-rows-match-domains", "a second row for domain %s (first at line %d): keep one", d, first)
		case !domains[d]:
			seen[d] = r.line
			c.report(r.line, "docs-quality-rows-match-domains", "domain %s is gone (no git-known file under %s%s/): delete its row", d, c.domains, d)
		default:
			seen[d] = r.line
		}
	}
	for _, d := range slices.Sorted(maps.Keys(domains)) {
		if _, ok := seen[d]; !ok {
			c.report(t.marker, "docs-quality-rows-match-domains", "domain %s (%s%s/) has no row: add one before the marker and grade it", d, c.domains, d)
		}
	}
	return c.bad, nil
}

// qualityFresh prints one line per row whose Updated date is after today's UTC date or more than
// qualityMaxAgeDays before it. Rows that are not well-formed are skipped: qualityShape reports them.
func qualityFresh(root string, today time.Time, stdout io.Writer) (bool, error) {
	t, err := readQualityTable(root)
	if err != nil {
		return false, err
	}
	today = today.UTC()
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	c := &qualityChecker{out: stdout}
	for _, r := range t.rows {
		if len(r.cells) != t.width {
			continue
		}
		updated := t.cell(r, "Updated")
		day, err := parseQualityDate(updated)
		switch {
		case err != nil:
		case day.After(today):
			c.report(r.line, "docs-quality-row-fresh", "Updated %s is after today (%s)", updated, today.Format(time.DateOnly))
		case day.AddDate(0, 0, qualityMaxAgeDays).Before(today):
			c.report(r.line, "docs-quality-row-fresh", "Updated %s is %d days old (more than %d): re-grade the row", updated, daysBetween(day, today), qualityMaxAgeDays)
		}
	}
	return c.bad, nil
}

// daysBetween counts the days between two UTC midnights through Unix seconds, which, unlike time.Duration
// (±292 years), cannot overflow for any date from 0001-01-01 on.
func daysBetween(from, to time.Time) int64 { return (to.Unix() - from.Unix()) / 86400 }

func parseQualityDate(s string) (time.Time, error) {
	if !qualityDateRE.MatchString(s) {
		return time.Time{}, errors.New("not YYYY-MM-DD")
	}
	return time.Parse(time.DateOnly, s)
}

// readQualityTable reads the rows between the table header and the marker.
func readQualityTable(root string) (*qualityTable, error) {
	src, err := os.ReadFile(filepath.Join(root, qualityFile))
	if err != nil {
		return nil, err
	}
	var t *qualityTable
	header := 0
	sc := bufio.NewScanner(bytes.NewReader(src))
	sc.Buffer(nil, 1<<20)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		switch {
		case t == nil:
			if strings.HasPrefix(line, "| Domain |") {
				if t, err = qualityHeader(line); err != nil {
					return nil, fmt.Errorf("%s:%d: %v", qualityFile, n, err)
				}
				header = n
			}
		case strings.Contains(line, qualityMarker):
			t.marker = n
			return t, sc.Err()
		case line == "" || tableSepRE.MatchString(line):
			// the separator row, or blank lines inside the table
		default:
			t.rows = append(t.rows, qualityRow{line: n, cells: tableCells(line)})
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if t == nil {
		return nil, fmt.Errorf("%s has no table header line starting with `| Domain |`", qualityFile)
	}
	return nil, fmt.Errorf("%s: no `%s` marker after the table header at line %d (%d rows read)", qualityFile, qualityMarker, header, len(t.rows))
}

// qualityHeader maps the header's cell names to their index; every column the checks read must be there.
func qualityHeader(line string) (*qualityTable, error) {
	cells := tableCells(line)
	t := &qualityTable{col: map[string]int{}, width: len(cells)}
	for i, name := range cells {
		t.col[name] = i
	}
	for _, need := range append([]string{"Domain", "Updated"}, qualityGrades...) {
		if _, ok := t.col[need]; !ok {
			return nil, fmt.Errorf("the table header has no %q column", need)
		}
	}
	return t, nil
}

// wellFormed checks one row's shape and returns its domain, or "" when the row names none it can match.
func (c *qualityChecker) wellFormed(t *qualityTable, r qualityRow) string {
	const rule = "docs-quality-rows-well-formed"
	if len(r.cells) != t.width {
		c.report(r.line, rule, "the row has %d cells, the header %d: one cell per column, and no | inside a cell", len(r.cells), t.width)
		return ""
	}
	domain := c.domainCell(r.line, t.cell(r, "Domain"))
	for _, layer := range qualityGrades {
		if g := t.cell(r, layer); !qualityGradeRE.MatchString(g) {
			c.report(r.line, rule, "%s grade %q is not one of A–F or – (en dash U+2013, not -)", layer, g)
		}
	}
	updated := t.cell(r, "Updated")
	if _, err := parseQualityDate(updated); err != nil {
		c.report(r.line, rule, "Updated %q is not a date (YYYY-MM-DD)", updated)
	}
	return domain
}

// domainCell returns the domain a Domain cell names: a bare name, or [name](../<domains>/name/AGENTS.md).
func (c *qualityChecker) domainCell(line int, cell string) string {
	const rule = "docs-quality-rows-well-formed"
	name, target, linked := cell, "", false
	if m := qualityLinkRE.FindStringSubmatch(cell); m != nil {
		name, target, linked = m[1], m[2], true
	}
	if !domainNameRE.MatchString(name) {
		c.report(line, rule, "Domain %q is not a domain name or a link to one: write [<domain>](../%s<domain>/AGENTS.md)", cell, c.domains)
		return ""
	}
	if want := "../" + c.domains + name + "/AGENTS.md"; linked && target != want {
		c.report(line, rule, "Domain link [%s](%s) does not point at its domain: write [%s](%s)", name, target, name, want)
	}
	return name
}

// tableCells splits a Markdown table row into its trimmed cells, without the outer pipes.
func tableCells(line string) []string {
	line = strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(line), "|"), "|")
	cells := strings.Split(line, "|")
	for i := range cells {
		cells[i] = strings.TrimSpace(cells[i])
	}
	return cells
}

// backendDomains is the set of directories under dir (the domains' parent, with a trailing `/`) that hold a
// git-known file and are domains (isDomainDir).
func backendDomains(root, dir string) (map[string]bool, error) {
	files, err := gitKnown(root, dir+"*")
	if err != nil {
		return nil, err
	}
	domains := map[string]bool{}
	for _, f := range files {
		d, rest, nested := strings.Cut(strings.TrimPrefix(f, dir), "/")
		if nested && rest != "" && isDomainDir(d) {
			domains[d] = true
		}
	}
	return domains, nil
}
