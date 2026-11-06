package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// The path helpers the hooks and sensors share: harness file names, protected-paths.txt entries, git-style
// pathspec matching, file-type checks and symlink resolution.

const (
	// rulesFile is the rule catalogue (fixture-claims, rule-lists, integrity-check).
	rulesFile = "docs/harness/rules.md"
	// protectedPathsFile lists the harness files (protect-harness, integrity-check, test-file).
	protectedPathsFile = "scripts/harness/protected-paths.txt"
	// fixturesDir holds the selftest fixtures (fixture-claims, orphans).
	fixturesDir = "scripts/harness/fixtures"
)

var (
	// skippedPath is what world.gitFiles and test-file leave out: fixtures hold deliberate violations, and
	// node_modules is not ours.
	skippedPath = regexp.MustCompile(`(^|/)(node_modules|fixtures)/`)
	// commentRE is a line that does nothing: only `#` lines and blank lines are comments. `//` and `/* */` can
	// comment out a JSONC, JS or Go line without the commented line itself looking like code. A `#` line that
	// holds another line break (CR, VT, FF, U+0085, U+2028, U+2029; one CR may end it) is not: YAML, SwiftLint
	// and swift-format read a line after it. Nor is one ending in `\`: make continues the comment onto the next
	// line, commenting that line out.
	commentRE = regexp.MustCompile(`^[[:space:]]*(#([^\r\v\f\x{85}\x{2028}\x{2029}]*[^\\\r\v\f\x{85}\x{2028}\x{2029}])?\r?)?$`)
)

// protectedPath is one entry of protected-paths.txt: a pathspec and what follows it (`config`, or nothing).
type protectedPath struct{ pattern, kind string }

// protectedPaths returns the entries of protected-paths.txt.
func protectedPaths(file string) ([]protectedPath, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var entries []protectedPath
	for _, l := range strings.Split(string(b), "\n") {
		if f := strings.Fields(l); !commentRE.MatchString(l) {
			entries = append(entries, protectedPath{f[0], strings.Join(f[1:], " ")})
		}
	}
	return entries, nil
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

// isRegular reports whether the repo-relative file f is a regular file, not a link or a directory.
func isRegular(root, f string) (bool, error) {
	fi, err := os.Lstat(filepath.Join(root, f))
	if err != nil {
		return false, err
	}
	return fi.Mode().IsRegular(), nil
}

// resolveExisting resolves symlinks in the longest existing prefix of the absolute path p (world.abs makes one);
// the rest (a file or directories still to be created) is appended as given. A dangling link is an error:
// writing through it would create its target, wherever that is.
func resolveExisting(p string) (string, error) {
	if !filepath.IsAbs(p) {
		return "", fmt.Errorf("%s is not an absolute path", p)
	}
	abs := filepath.Clean(p)
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
