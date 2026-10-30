package main

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

var (
	testFileRE = regexp.MustCompile(`(_test\.go|\.(test|spec|e2e)\.[cm]?[jt]sx?)$`)
	// testFileForms says in words what testFileRE matches; TestTestFileForms holds the two together.
	testFileForms = "*_test.go, or *.test.*, *.spec.* or *.e2e.* ending in .js, .ts, .jsx or .tsx (also .mjs, .cjs, .mts, .cts)"
	// testFilePathspecs selects, by name alone, every file testFileRE can match (`*` also matches `/`): git's
	// prefilter for specs-tested, never the decision.
	testFilePathspecs = []string{"*_test.go", "*.test.*", "*.spec.*", "*.e2e.*"}
)

// productTests is the one predicate for a product test file: specs-tested reads only the files it allows (ADR
// 0036), and `harness test-file`, the test-writer's write guard, lets the agent write only those (ADR 0039).
// The exclusions ignore case: on a case-insensitive disk (macOS by default) `apps/api/tools/Harness/x_test.go`
// is a file in the harness package, so every case variant of an excluded path is excluded on every disk. The
// include globs keep their case, so a variant there only rejects more.
type productTests struct {
	include   []string        // PRODUCT_GLOBS and WEB_GLOBS
	apiClient string          // API_CLIENT_DIR
	generated []string        // GENERATED_GLOBS
	protected []protectedPath // scripts/harness/protected-paths.txt: the harness's own tests and fixtures
}

// loadProductTests reads the predicate from root's layout.env and protected-paths.txt. It fails closed on a
// protected-paths.txt without entries (the harness would not be excluded) and on a pattern pathspecMatch cannot
// read the way git does (`?`, `[` or `\`).
func loadProductTests(root string) (productTests, error) {
	l, err := loadLayout(root)
	if err != nil {
		return productTests{}, err
	}
	file := filepath.Join(root, "scripts/harness/protected-paths.txt")
	protected, err := protectedPaths(file)
	if err != nil {
		return productTests{}, err
	}
	if len(protected) == 0 {
		return productTests{}, fmt.Errorf("no entries in %s: refusing to decide without excluding the harness files", file)
	}
	var include []string
	for _, g := range append(l.list("PRODUCT_GLOBS"), l.list("WEB_GLOBS")...) {
		if !slices.Contains(include, g) { // apps/* is in both in the template
			include = append(include, g)
		}
	}
	pt := productTests{
		include:   include,
		apiClient: l.get("API_CLIENT_DIR"),
		generated: l.list("GENERATED_GLOBS"),
		protected: protected,
	}
	patterns := append(append(slices.Clone(pt.include), pt.apiClient), pt.generated...)
	for _, p := range protected {
		patterns = append(patterns, p.pattern)
	}
	for _, p := range patterns {
		if strings.ContainsAny(p, `?[\`) {
			return productTests{}, fmt.Errorf("pattern %q in layout.env or protected-paths.txt has ?, [ or \\: only * is supported", p)
		}
	}
	return pt, nil
}

// why returns "" when rel (repo-relative, `/`-separated) is a product test file, else why it is not.
func (pt productTests) why(rel string) string {
	switch {
	case !testFileRE.MatchString(path.Base(rel)):
		return "is not a test file (" + testFileForms + ")"
	case skippedPath.MatchString(strings.ToLower(rel)):
		return "is under node_modules/ or fixtures/"
	case !matchesAny(pt.include, rel):
		return "is not product code (PRODUCT_GLOBS or WEB_GLOBS: " + strings.Join(pt.include, ", ") + ")"
	case pathspecMatchFold(pt.apiClient, rel):
		return "is in the generated API client (API_CLIENT_DIR: " + pt.apiClient + ")"
	case matchesAnyFold(pt.generated, rel):
		return "is generated code (GENERATED_GLOBS: " + strings.Join(pt.generated, ", ") + ")"
	}
	for _, p := range pt.protected {
		if pathspecMatchFold(p.pattern, rel) {
			return "is a harness file (" + p.pattern + " in scripts/harness/protected-paths.txt)"
		}
	}
	return ""
}

func matchesAny(patterns []string, rel string) bool {
	for _, p := range patterns {
		if pathspecMatch(p, rel) {
			return true
		}
	}
	return false
}

func matchesAnyFold(patterns []string, rel string) bool {
	for _, p := range patterns {
		if pathspecMatchFold(p, rel) {
			return true
		}
	}
	return false
}

// pathspecMatchFold is pathspecMatch ignoring case (ASCII and Unicode simple folding).
func pathspecMatchFold(p, rel string) bool {
	return pathspecMatch(strings.ToLower(p), strings.ToLower(rel))
}

// pathspecMatch reports whether git's default pathspec matching matches rel with p, for the patterns
// layout.env and protected-paths.txt use: `*` matches any characters, `/` included; a pattern without `*` also
// matches what is under it. An empty pattern matches nothing (as a pathspec it would be the whole repo). It
// does not read `?`, `[` or `\`, so loadProductTests refuses patterns with them.
func pathspecMatch(p, rel string) bool {
	switch {
	case p == "":
		return false
	case !strings.Contains(p, "*"):
		return rel == p || strings.HasPrefix(rel, strings.TrimSuffix(p, "/")+"/")
	}
	ok, err := regexp.MatchString("^"+strings.ReplaceAll(regexp.QuoteMeta(p), `\*`, ".*")+"$", rel)
	return ok && err == nil
}

// testFile is the test-writer subagent's write guard (.claude/hooks/only-test-files.sh, ADR 0039): p (absolute,
// or relative to the working directory) must name a file inside root that productTests allows, the predicate
// specs-tested reads tests with. Symlinks in the part of the path that exists are resolved first, as
// protect-harness's protected() does, so a link cannot carry the write out of the repo or onto another file.
// It returns nil for an allowed path and an error saying why for any other; an unreadable layout.env or
// protected-paths.txt is an error too (fail closed).
//
// Limit: it reads no git state, so a git-ignored product test file (apps/web/dist/x.test.ts, coverage/, .tmp/)
// is allowed although specs-tested, which reads git-known files only, never sees it (TD-128). Writing one is
// harmless: it traces no scenario and is not committed.
func testFile(root, p string) error {
	r, err := filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("repository root: %w", err)
	}
	if p == "" {
		return errors.New("no file path in the hook input")
	}
	// filepath.Abs would drop `x/..` before the symlinks are resolved, but the OS resolves a link x first.
	if slices.Contains(strings.Split(filepath.ToSlash(p), "/"), "..") {
		return fmt.Errorf("%s has a `..` part, which can lead outside the repository: give the path without it", p)
	}
	resolved, err := resolveExisting(p)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(r, resolved)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%s is outside the repository", p)
	}
	pt, err := loadProductTests(r)
	if err != nil {
		return err
	}
	rel = filepath.ToSlash(rel)
	if why := pt.why(rel); why != "" {
		return fmt.Errorf("%s %s", rel, why)
	}
	return nil
}

// resolveExisting makes p absolute and resolves symlinks in its longest existing prefix; the rest (a file or
// directories still to be created) is appended as given. A dangling link is an error: writing through it
// would create its target, wherever that is.
func resolveExisting(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	dir, rest := abs, ""
	for {
		if _, err := os.Lstat(dir); err == nil {
			r, err := filepath.EvalSymlinks(dir)
			if err != nil {
				return "", fmt.Errorf("cannot resolve %s: %w", p, err)
			}
			return filepath.Join(r, rest), nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return abs, nil
		}
		rest = filepath.Join(filepath.Base(dir), rest)
		dir = parent
	}
}
