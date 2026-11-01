package main

// layout: scripts/harness/layout.env says where the product's code lives (exec plan 0003, P4.6; ADR 0030). The
// scripts and the Makefile (through scripts/harness/layout.sh), CI and the domainiso analyzer read the same file;
// it is parsed here with layout.sh's rules: `KEY=value` lines, `#` comments, every known key exactly once, no
// unknown key, a plain value (no spaces, quotes or `$`), lists separated by single `:` (no empty entry).
//
// STACK, the sensor groups the product has (ADR 0033), is optional (go:db:web without it, as in a layout.env
// written before the key existed) and replaced by the environment's HARNESS_STACK when that is set (`,` read as
// `:`); it names each of go, db, web and ios (ADR 0041) at most once, and go always. Without db, layout-check
// leaves MIGRATIONS_DIR and QUERIES_DIR unchecked; without web, API_CLIENT_DIR and WEB_GLOBS. A stack must match
// the tree: a group left out while its git-known files exist (a *.sql in MIGRATIONS_DIR or a file in QUERIES_DIR;
// a package.json in a WEB_GLOBS workspace or an API_CLIENT_DIR; a *.swift or an Xcode project under a
// PRODUCT_GLOBS or WEB_GLOBS entry) is a finding, so a narrowed stack never silently drops sensors whose code is still there.
//
// IOS_DIR (the iOS app's directory, ADR 0042), UPSTREAM_SPECS and CONTRACT_CONSUMERS (cross-repo contracts, ADR
// 0037) are optional and the only keys that may be empty. With ios in STACK, IOS_DIR must hold a git-known
// Package.swift or project.yml, so the iOS sensors never run on nothing. Each UPSTREAM_SPECS entry is
// `<vendored spec>@<source repo>@<spec path in the source>@<revision>`, read by scripts/harness/upstream-drift.sh, which repeats these checks at run time (drift.yml runs it without make
// check): the shape; a vendored spec inside this repo (not absolute, no `..`) that is a file; a source that is a
// host path (github.com/<owner>/<repo>, fetched over https) or an absolute local path (tests and fixtures only),
// never starting with `-` and without `..`; a spec path that is plain (not absolute, no `.`, `..` or empty
// segment); a 7 to 40 digit hex revision. testdata/upstream-spec-vectors.txt holds both to the same rule.
// CONTRACT_CONSUMERS names repos for people (contract-check.sh prints it), so it is not checked.
//
// layout-check (harness-layout-valid, make layout-check in make check) prints one line per finding:
//
//	scripts/harness/layout.env[:<line>]: <message> [harness-layout-valid]
//
// for a malformed line, a missing, unknown or repeated key, an UPSTREAM_SPECS entry that is refused, a STACK that is invalid or leaves out a group the tree has, API_DIR without a go.mod, SPEC that is not a file,
// MIGRATIONS_DIR or API_CLIENT_DIR that is not a directory (QUERIES_DIR: nor its parent), with ios an IOS_DIR
// without a git-known Package.swift or project.yml, a WEB_GLOBS entry
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
	"io/fs"
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
	layoutKeys   = []string{"API_DIR", "SPEC", "MIGRATIONS_DIR", "QUERIES_DIR", "API_CLIENT_DIR", "WEB_GLOBS", "GENERATED_GLOBS", "PRODUCT_GLOBS", "LAYER_GLOB", "STACK", "UPSTREAM_SPECS", "CONTRACT_CONSUMERS", "IOS_DIR"}
	optionalKeys = []string{"STACK", "UPSTREAM_SPECS", "CONTRACT_CONSUMERS", "IOS_DIR"} // absent: the default stack, no entries, no iOS app
	emptyKeys    = []string{"UPSTREAM_SPECS", "CONTRACT_CONSUMERS", "IOS_DIR"}          // the only values that may be empty
	stackGroups  = []string{"go", "db", "web", "ios"}
	defaultStack = "go:db:web" // a layout.env without STACK
	stackRE      = regexp.MustCompile(`^[a-z]+(?::[a-z]+)*$`)
	layoutLineRE = regexp.MustCompile(`^([A-Z_]+)=((?:[A-Za-z0-9_./*@+-]+(?::[A-Za-z0-9_./*@+-]+)*)?)$`)
	revisionRE   = regexp.MustCompile(`^[0-9a-f]{7,40}$`)
	hostPathRE   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.-]*(?:/[A-Za-z0-9_.+-]+)+$`) // github.com/<owner>/<repo>
	layoutSkipRE = regexp.MustCompile(`^[[:space:]]*(#|$)`)
)

// layout is a parsed layout.env: each key's value and line; lists are split with list. stackAt names where the
// effective STACK came from, for findings: "<layoutFile>:<line>: STACK=<value>" or "HARNESS_STACK=<value>".
type layout struct {
	values  map[string]string
	lines   map[string]int
	stackAt string
}

func (l layout) get(key string) string { return l.values[key] }

// list splits a list value at `:`; an empty (or absent) one has no entries.
func (l layout) list(key string) []string {
	if l.values[key] == "" {
		return nil
	}
	return strings.Split(l.values[key], ":")
}

// has reports whether STACK names group.
func (l layout) has(group string) bool { return slices.Contains(l.list("STACK"), group) }

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
		case m == nil, m[2] == "" && !slices.Contains(emptyKeys, m[1]):
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
		if l.lines[k] == 0 && !slices.Contains(optionalKeys, k) {
			problems = append(problems, fmt.Sprintf("%s: missing %s", layoutFile, k))
		}
	}
	if len(problems) == 0 {
		problems = l.applyStack()
	}
	return l, problems, sc.Err()
}

// applyStack defaults a missing STACK, replaces it with HARNESS_STACK when that is set, then validates it
// (layout.sh's harness_layout_parse and harness_stack_apply).
func (l *layout) applyStack() []string {
	if l.lines["STACK"] == 0 {
		l.values["STACK"] = defaultStack
		l.stackAt = fmt.Sprintf("%s: STACK unset, so %s", layoutFile, defaultStack)
	} else {
		l.stackAt = fmt.Sprintf("%s:%d: STACK=%s", layoutFile, l.lines["STACK"], l.get("STACK"))
	}
	if env, ok := os.LookupEnv("HARNESS_STACK"); ok {
		l.values["STACK"], l.stackAt = strings.ReplaceAll(env, ",", ":"), "HARNESS_STACK="+env
	}
	where := l.stackAt
	if !stackRE.MatchString(l.get("STACK")) {
		return []string{where + ": not a list of groups such as go:db:web (HARNESS_STACK may use , instead of :)"}
	}
	var problems []string
	seen := map[string]bool{}
	for _, g := range l.list("STACK") {
		switch {
		case !slices.Contains(stackGroups, g):
			problems = append(problems, fmt.Sprintf("%s: unknown group %s (known: %s)", where, g, strings.Join(stackGroups, " ")))
		case seen[g]:
			problems = append(problems, fmt.Sprintf("%s: %s named twice", where, g))
		}
		seen[g] = true
	}
	if !seen["go"] {
		problems = append(problems, where+": no go (every stack has the Go API and the harness)")
	}
	return problems
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
	if l.has("db") {
		if !isDir(l.get("MIGRATIONS_DIR")) {
			report("MIGRATIONS_DIR", "no such directory")
		}
		// The first domain's queries create QUERIES_DIR (make new-domain): until then only its parent exists.
		if !isDir(path.Dir(l.get("QUERIES_DIR"))) {
			report("QUERIES_DIR", "neither it nor its parent directory exists")
		}
	}
	for _, e := range l.list("UPSTREAM_SPECS") {
		if problem := upstreamEntryProblem(e); problem != "" {
			report("UPSTREAM_SPECS", "%s: %s", e, problem)
		} else if vendored, _, _ := strings.Cut(e, "@"); !isFile(vendored) {
			report("UPSTREAM_SPECS", "%s: no such file %s (the vendored copy)", e, vendored)
		}
	}
	if l.has("web") {
		if !isDir(l.get("API_CLIENT_DIR")) {
			report("API_CLIENT_DIR", "no such directory")
		}
		for _, e := range l.list("WEB_GLOBS") {
			if dir, ok := strings.CutSuffix(e, "/*"); !ok || !isDir(dir) {
				report("WEB_GLOBS", "%s is not <existing directory>/*", e)
			}
		}
	}
	if l.has("ios") {
		// A clean relative path: git refuses one outside the repo, and affected.sh compares IOS_DIR as text.
		dir := l.get("IOS_DIR")
		switch {
		case l.lines["IOS_DIR"] == 0:
			out = append(out, layoutFile+": IOS_DIR unset, but STACK has ios (set it to the iOS app's directory)")
		case dir == "":
			report("IOS_DIR", "no git-known Package.swift or project.yml there, but STACK has ios (set it to the iOS app's directory)")
		case dir == "." || !fs.ValidPath(dir): // fs.ValidPath holds for "." (the repo root, no app directory)
			report("IOS_DIR", "not a clean path inside this repo (no leading /, no ./ or .., no trailing /)")
		default:
			want := []string{path.Join(dir, "Package.swift"), path.Join(dir, "project.yml")}
			apps, err := gitKnown(root, ":(literal)"+want[0], ":(literal)"+want[1])
			if err != nil {
				return nil, err
			}
			// One of exactly those paths (a literal pathspec also lists the files under a directory of that name),
			// and a regular file, not a symlink: a link to /dev/null or to any other file is no app.
			found := slices.ContainsFunc(apps, func(f string) bool {
				fi, err := os.Lstat(filepath.Join(root, f))
				return slices.Contains(want, f) && err == nil && fi.Mode().IsRegular()
			})
			if !found {
				report("IOS_DIR", "no git-known Package.swift or project.yml there, but STACK has ios (set it to the iOS app's directory)")
			} else if !matchesAny(append(l.list("PRODUCT_GLOBS"), l.list("WEB_GLOBS")...), path.Join(dir, "Package.swift")) {
				// Outside them, specs-tested and the test-writer's write guard (productTests) never see its tests.
				report("IOS_DIR", "not under a PRODUCT_GLOBS or WEB_GLOBS entry, so its Swift tests would not count")
			}
		}
	}
	hidden, err := l.hiddenGroups(root, isDir)
	if err != nil {
		return nil, err
	}
	out = append(out, hidden...)
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

// upstreamEntryProblem says why an UPSTREAM_SPECS entry is refused, or "" (upstream-drift.sh's entry_problem, with
// the same messages: keep the two in step, testdata/upstream-spec-vectors.txt checks both).
func upstreamEntryProblem(e string) string {
	f := strings.Split(e, "@")
	if len(f) != 4 || slices.Contains(f, "") {
		return "not <vendored spec>@<source repo>@<spec path in the source>@<revision>"
	}
	vendored, src, specPath, revision := f[0], f[1], f[2], f[3]
	segs := func(p string) []string { return strings.Split(p, "/") }
	switch {
	case strings.HasPrefix(vendored, "/") || slices.Contains(segs(vendored), ".."):
		return fmt.Sprintf("vendored spec %s is not a path inside this repo", vendored)
	case !strings.HasPrefix(src, "/") && !hostPathRE.MatchString(src) || slices.Contains(segs(src), ".."):
		return fmt.Sprintf("source %s is neither a host path (github.com/<owner>/<repo>) nor an absolute local path", src)
	case strings.HasPrefix(specPath, "/") || slices.ContainsFunc(segs(specPath), func(s string) bool { return s == "" || s == "." || s == ".." }):
		return fmt.Sprintf("spec path %s is not a plain path inside the source", specPath)
	case !revisionRE.MatchString(revision):
		return fmt.Sprintf("revision %s is not a 7 to 40 digit hex commit", revision)
	}
	return ""
}

// hiddenGroups reports each group the stack leaves out while the tree still has its code: its sensors would leave
// make check and make check-all without a trace.
func (l layout) hiddenGroups(root string, isDir func(string) bool) ([]string, error) {
	var out []string
	hide := func(group, found string) {
		out = append(out, fmt.Sprintf("%s: leaves out %s, but %s exists: add %s to STACK or delete its directories (do not narrow HARNESS_STACK on a full tree)", l.stackAt, group, found, group))
	}
	if !l.has("db") {
		files, err := gitFiles(root, ":(glob)"+l.get("MIGRATIONS_DIR")+"/*.sql", l.get("QUERIES_DIR")+"/*")
		if err != nil {
			return nil, err
		}
		if len(files) > 0 {
			hide("db", files[0])
		}
	}
	if !l.has("web") {
		var specs []string
		for _, e := range l.list("WEB_GLOBS") {
			specs = append(specs, ":(glob)"+e+"/package.json")
		}
		files, err := gitFiles(root, specs...)
		if err != nil {
			return nil, err
		}
		switch {
		case len(files) > 0:
			hide("web", files[0])
		case isDir(l.get("API_CLIENT_DIR")):
			hide("web", l.get("API_CLIENT_DIR"))
		}
	}
	if !l.has("ios") { // any Swift counts, hybrid shells and server Swift too (ADR 0041)
		files, err := gitKnown(root, "*.swift", "*.xcodeproj/project.pbxproj") // not gitFiles: a fixtures/ folder in the app is still the app
		if err != nil {
			return nil, err
		}
		scope := append(l.list("PRODUCT_GLOBS"), l.list("WEB_GLOBS")...) // where productTests counts Swift tests
		for _, f := range files {
			if matchesAny(scope, f) {
				hide("ios", f)
				break
			}
		}
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

// isDomainDir reports whether a directory directly under the domains' parent is a domain: every one but
// platform, the shared plumbing (AGENTS.md#layer-rule). analyzers/domainiso has its own copy of this
// definition (TD-55).
func isDomainDir(name string) bool { return name != "platform" }

// domainsDir is the domains' parent directory, repo-relative and without a trailing `/`: LAYER_GLOB without
// its two `*` segments, inside API_DIR (the Makefile's DOMAINS_DIR). A LAYER_GLOB of another shape is an
// error, so its readers fail closed.
func (l layout) domainsDir() (string, error) {
	segs, err := layerSegments(l.get("LAYER_GLOB"))
	if err != nil {
		return "", fmt.Errorf("%s:%d: LAYER_GLOB=%s: %v (run make layout-check)", layoutFile, l.lines["LAYER_GLOB"], l.get("LAYER_GLOB"), err)
	}
	return l.layerDir(segs), nil
}

// layerDir is the directory the literal LAYER_GLOB segments name, inside API_DIR.
func (l layout) layerDir(segs []string) string {
	return l.apiPath(strings.Join(segs[:len(segs)-2], "/"))
}

// layerPackages counts the directories under API_DIR that match the LAYER_GLOB segments and hold a git-known
// .go file, and lists the flat packages: <layer dir>/<pkg> with a .go file of its own, which domainiso skips
// (it needs a domain and a layer). <layer dir>/platform is no domain (isDomainDir): domainiso skips it too, so
// it is never flat; its packages count, since a tree without a domain yet (the template) is valid.
func (l layout) layerPackages(root string, segs []string) (int, []string, error) {
	prefix := l.layerDir(segs)
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
		case len(rest) == 1 && isDomainDir(rest[0]):
			flat[path.Dir(f)] = true
		}
	}
	return len(dirs), slices.Sorted(maps.Keys(flat)), nil
}
