package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// testFile is the test-writer subagent's write guard (.claude/hooks/only-test-files.sh, ADR 0039): p (absolute,
// or relative to the working directory) must name a file inside root whose name testFileRE matches, the same
// predicate specs-tested uses to find tests. Symlinks in the part of the path that exists are resolved first,
// as protect-harness's protected() does, so a link cannot carry the write out of the repo or onto a non-test
// file. It returns nil for an allowed path and an error saying why for any other.
func testFile(root, p string) error {
	r, err := filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("repository root: %w", err)
	}
	if p == "" {
		return errors.New("no file path in the hook input")
	}
	resolved, err := resolveExisting(p)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(r, resolved)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%s is outside the repository", p)
	}
	if !testFileRE.MatchString(filepath.Base(resolved)) {
		return fmt.Errorf("%s is not a test file", filepath.ToSlash(rel))
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
