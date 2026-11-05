package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// The path helpers the hooks and sensors share: protected-paths.txt entries, git-style pathspec matching and
// symlink resolution.

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
