package main

// layout: scripts/harness/layout.env says where the product's code lives (exec plan 0003, P4.6; ADR 0030). The
// scripts and the Makefile (through scripts/harness/layout.sh), CI and the domainiso analyzer read the same file;
// it is parsed here with layout.sh's rules: `KEY=value` lines, `#` comments, every known key exactly once, no
// unknown key, a plain value (no spaces, quotes or `$`), lists separated by single `:` (no empty entry).
//
// layout-check (harness-layout-valid, make layout-check in make check) prints one line per finding:
//
//	scripts/harness/layout.env[:<line>]: <message> [harness-layout-valid]
//
// for a malformed line, a missing, unknown or repeated key, API_DIR without a go.mod, SPEC that is not a file,
// MIGRATIONS_DIR or API_CLIENT_DIR that is not a directory (QUERIES_DIR: nor its parent), a WEB_GLOBS entry
// that is not `<existing dir>/*`, and a LAYER_GLOB that is not `<dir>[/<dir>…]/*/*`-shaped (literal segments,
// then exactly two `*` segments: domain and layer, as the analyzer reads them), matches no `<dir>/*/*`
// directory with a git-known .go file under API_DIR (fixtures skipped), or has a flat package `<dir>/<pkg>`
// that the analyzer would skip (`<dir>/platform` is shared plumbing, skipped by the analyzer, never flat), so
// the domain-isolation analyzer is never a silent no-op.

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"maps"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

const layoutFile = "scripts/harness/layout.env"

var (
	layoutKeys   = []string{"API_DIR", "SPEC", "MIGRATIONS_DIR", "QUERIES_DIR", "API_CLIENT_DIR", "WEB_GLOBS", "GENERATED_GLOBS", "PRODUCT_GLOBS", "LAYER_GLOB"}
	layoutLineRE = regexp.MustCompile(`^([A-Z_]+)=([A-Za-z0-9_./*@+-]+(?::[A-Za-z0-9_./*@+-]+)*)$`)
	layoutSkipRE = regexp.MustCompile(`^[[:space:]]*(#|$)`)
)

// layout is a parsed layout.env: each key's value and line; lists are split with list.
type layout struct {
	values map[string]string
	lines  map[string]int
}

func (l layout) get(key string) string { return l.values[key] }

func (l layout) list(key string) []string { return strings.Split(l.values[key], ":") }

// apiPath joins API_DIR and a path inside it, as a repo-relative path ("." joins to the path itself).
func (l layout) apiPath(p string) string { return path.Join(l.get("API_DIR"), p) }

// parseLayout reads root's layout.env; problems are findings (`<layoutFile>[:<line>]: <message>`), err means it
// is unreadable.
func parseLayout(root string) (layout, []string, error) {
	b, err := os.ReadFile(filepath.Join(root, layoutFile))
	if err != nil {
		return layout{}, nil, err
	}
	l := layout{values: map[string]string{}, lines: map[string]int{}}
	known := map[string]bool{}
	for _, k := range layoutKeys {
		known[k] = true
	}
	var problems []string
	sc := bufio.NewScanner(bytes.NewReader(b))
	for n := 1; sc.Scan(); n++ {
		line := sc.Text()
		if layoutSkipRE.MatchString(line) {
			continue
		}
		m := layoutLineRE.FindStringSubmatch(line)
		switch {
		case m == nil:
			problems = append(problems, fmt.Sprintf("%s:%d: not KEY=value (no spaces, quotes, $ or empty list entries)", layoutFile, n))
		case !known[m[1]]:
			problems = append(problems, fmt.Sprintf("%s:%d: unknown key %s (known: %s)", layoutFile, n, m[1], strings.Join(layoutKeys, " ")))
		case l.lines[m[1]] != 0:
			problems = append(problems, fmt.Sprintf("%s:%d: %s set twice", layoutFile, n, m[1]))
		default:
			l.values[m[1]], l.lines[m[1]] = m[2], n
		}
	}
	for _, k := range layoutKeys {
		if l.lines[k] == 0 {
			problems = append(problems, fmt.Sprintf("%s: missing %s", layoutFile, k))
		}
	}
	return l, problems, sc.Err()
}

// loadLayout is parseLayout for the sensors that only read the layout: any problem is an error (exit 2).
func loadLayout(root string) (layout, error) {
	l, problems, err := parseLayout(root)
	if err == nil && len(problems) > 0 {
		err = fmt.Errorf("%s (run make layout-check)", problems[0])
	}
	return l, err
}

// layoutCheck prints one line per finding to stdout and reports whether there was any.
func layoutCheck(root string, stdout io.Writer) (bool, error) {
	l, problems, err := parseLayout(root)
	if err != nil {
		return false, err
	}
	if len(problems) == 0 {
		problems, err = l.problems(root)
		if err != nil {
			return false, err
		}
	}
	for _, p := range problems {
		fmt.Fprintf(stdout, "%s [harness-layout-valid]\n", p)
	}
	return len(problems) > 0, nil
}

// problems checks a well-formed layout against the tree at root.
func (l layout) problems(root string) ([]string, error) {
	var out []string
	report := func(key, format string, a ...any) {
		out = append(out, fmt.Sprintf("%s:%d: %s=%s: %s", layoutFile, l.lines[key], key, l.get(key), fmt.Sprintf(format, a...)))
	}
	isDir := func(p string) bool { fi, err := os.Stat(filepath.Join(root, p)); return err == nil && fi.IsDir() }
	isFile := func(p string) bool {
		fi, err := os.Stat(filepath.Join(root, p))
		return err == nil && fi.Mode().IsRegular()
	}

	if !isFile(l.apiPath("go.mod")) {
		report("API_DIR", "no go.mod there (the Go API module; `.` for the repo root)")
	}
	if !isFile(l.get("SPEC")) {
		report("SPEC", "no such file (the OpenAPI contract)")
	}
	for _, k := range []string{"MIGRATIONS_DIR", "API_CLIENT_DIR"} {
		if !isDir(l.get(k)) {
			report(k, "no such directory")
		}
	}
	// The first domain's queries create QUERIES_DIR (make new-domain): until then only its parent exists.
	if !isDir(path.Dir(l.get("QUERIES_DIR"))) {
		report("QUERIES_DIR", "neither it nor its parent directory exists")
	}
	for _, e := range l.list("WEB_GLOBS") {
		if dir, ok := strings.CutSuffix(e, "/*"); !ok || !isDir(dir) {
			report("WEB_GLOBS", "%s is not <existing directory>/*", e)
		}
	}
	segs, err := layerSegments(l.get("LAYER_GLOB"))
	if err != nil {
		report("LAYER_GLOB", "%v", err)
		return out, nil
	}
	n, flat, err := l.layerPackages(root, segs)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		report("LAYER_GLOB", "matches no package (no directory %s with a .go file): the domain-isolation analyzer would check nothing", l.apiPath(l.get("LAYER_GLOB")))
	}
	for _, dir := range flat {
		report("LAYER_GLOB", "%s is a package with no layer (expected %s/<domain>/<layer>): the domain-isolation analyzer skips it", dir, path.Dir(dir))
	}
	return out, nil
}

// layerSegments splits LAYER_GLOB: literal directories, then exactly two `*` segments (domain, layer).
func layerSegments(glob string) ([]string, error) {
	segs := strings.Split(glob, "/")
	if len(segs) < 3 || segs[len(segs)-1] != "*" || segs[len(segs)-2] != "*" {
		return nil, fmt.Errorf("not <dir>[/<dir>…]/*/* (literal directories, then the domain and the layer)")
	}
	for _, s := range segs[:len(segs)-2] {
		if s == "" || s == "." || s == ".." || strings.ContainsAny(s, "*?[") {
			return nil, fmt.Errorf("segment %q is not a literal directory name", s)
		}
	}
	return segs, nil
}

// layerPackages counts the directories under API_DIR that match the LAYER_GLOB segments and hold a git-known
// .go file, and lists the flat packages: <layer dir>/<pkg> with a .go file of its own, which domainiso skips
// (it needs a domain and a layer). <layer dir>/platform is shared plumbing, not a domain: domainiso skips it
// too, so it is never flat; its packages count, since a tree without a domain yet (the template) is valid.
func (l layout) layerPackages(root string, segs []string) (int, []string, error) {
	prefix := l.apiPath(strings.Join(segs[:len(segs)-2], "/"))
	if fi, err := os.Stat(filepath.Join(root, prefix)); err != nil || !fi.IsDir() {
		return 0, nil, nil // also keeps git ls-files -o from warning about a missing directory
	}
	files, err := gitFiles(root, prefix+"/*.go")
	if err != nil {
		return 0, nil, err
	}
	dirs, flat := map[string]bool{}, map[string]bool{}
	for _, f := range files {
		rest := strings.Split(strings.TrimPrefix(path.Dir(f), prefix+"/"), "/")
		switch {
		case len(rest) == 2:
			dirs[path.Dir(f)] = true
		case len(rest) == 1 && rest[0] != "platform":
			flat[path.Dir(f)] = true
		}
	}
	return len(dirs), slices.Sorted(maps.Keys(flat)), nil
}
