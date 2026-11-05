package main

import (
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

var (
	testFileRE = regexp.MustCompile(`(_test\.go|\.(test|spec|e2e)\.[cm]?[jt]sx?)$`)
	// swiftTestRE matches a whole path: Swift has no build rule like _test.go, and Xcode compiles any file in an
	// app target's folder, so a Swift test also needs a directory named *Tests (SwiftPM's Tests/<X>Tests/,
	// Xcode's <App>Tests/ and <App>UITests/; a bare Tests/ is in no target; ADR 0041).
	swiftTestRE = regexp.MustCompile(`(^|/)[^/]+Tests/([^/]+/)*[^/]*Tests\.swift$`)
	// swiftSourcesRE: SwiftPM compiles everything under a source target's folder (Sources/, Source/, src/ or
	// srcs/ by default), a *Tests subfolder too, so a file there ships unless Package.swift says otherwise; a
	// test target placed there is refused as well (fails closed: move it to Tests/).
	swiftSourcesRE = regexp.MustCompile(`(?i)(^|/)(sources?|srcs?)/`) // any case: macOS disks fold it, as matchesAnyFold says
	// testFileForms says in words what testFileRE and swiftTestRE match; TestTestFileForms holds them together.
	testFileForms = "*_test.go, or *.test.*, *.spec.* or *.e2e.* ending in .js, .ts, .jsx or .tsx (also .mjs, .cjs, .mts, .cts), or *Tests.swift under a *Tests directory, not under Sources/"
	// testFilePathspecs selects, by name alone, every file the two can match (`*` also matches `/`): git's
	// prefilter for specs-tested, never the decision.
	testFilePathspecs = []string{"*_test.go", "*.test.*", "*.spec.*", "*.e2e.*", "*Tests.swift"}
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
	case !testFileRE.MatchString(path.Base(rel)) && (!swiftTestRE.MatchString(rel) || swiftSourcesRE.MatchString(rel)):
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

// testFile is the test-writer subagent's write guard (.claude/hooks/only-test-files.sh, ADR 0039): p (absolute,
// or relative to the working directory) must name a file inside root that productTests allows, the predicate
// specs-tested reads tests with. Symlinks in the part of the path that exists are resolved first, as
// protect-harness's check does, so a link cannot carry the write out of the repo or onto another file.
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
