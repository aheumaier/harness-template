package main

// no-new-shell (ADR 0045: new harness logic is Go) checks two rules.
//
// harness-no-new-shell: every git-known shell file under scripts/ and .claude/hooks/ is listed in
// scripts/harness/shell-allowlist.txt; scripts/harness/fixtures/ holds data and is exempt. A shell file ends in
// .sh or .bash (any case), or has no extension (a dotfile has none) and no `#!` line naming another interpreter:
// bash runs a file without one, or with an empty one. The list holds the bootstrap (kind `bootstrap`: what must run before .bin/harness
// exists or without it, ADR 0045 R2) and the scripts not ported yet (kind `unported`); a port takes its script
// off the list, and adding a line is a change to protected config. An entry whose file is missing is no finding:
// an adopted repo copies only part of scripts/ (README (b) step 3), and the list is synced from the template.
//
// harness-go-runs-no-script: no string literal in the non-test Go files of the harness module names a shell
// script other than a bootstrap entry: a path under scripts/ or .claude/hooks/ anywhere in the literal, or any
// other name ending in .sh or .bash (`"run.sh"`, the last part filepath.Join takes, `"./zz.sh"` beside the Go
// code) unless its file name is a bootstrap entry's. Comments do not count.
//
// Run from `make no-new-shell` (make check). Fails closed (exit 2) when the list is missing or malformed, the
// layout cannot be read, git fails, a file it reads (an extensionless shell candidate, a harness Go file) is not a
// regular file, the harness module has no Go file or one does not parse.

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

const (
	newShellRule   = "harness-no-new-shell"
	goRunsShRule   = "harness-go-runs-no-script"
	shellAllowFile = "scripts/harness/shell-allowlist.txt"
)

var (
	// A shell in a `#!` line: sh, bash, zsh, dash or ksh after a slash, a blank or env's -S (`#! /bin/sh`,
	// `#!/usr/bin/env -u X bash`, `#!/usr/bin/env -Sbash -e`, `#!/bin/busybox sh`).
	shellWordRE = regexp.MustCompile(`(?:[/\s]|-S)(?:ba|z|da|k)?sh(?:\s|$)`)
	// A script name or path in a string: name bytes and slashes ending in .sh or .bash, any case.
	scriptTokenRE = regexp.MustCompile(`[A-Za-z0-9_./-]*[A-Za-z0-9_-]\.(?i:sh|bash)\b`)
)

func init() {
	commands.register(rootSensor("no-new-shell", "harness-no-new-shell, harness-go-runs-no-script (make no-new-shell, check)", noNewShell))
}

// noNewShell prints one line per finding to stdout and reports whether there was any. root is the repo.
func noNewShell(w world, root string) (bool, error) {
	allowed, err := readShellAllowlist(root)
	if err != nil {
		return false, err
	}
	unlisted, err := unlistedShell(w, root, allowed)
	if err != nil {
		return false, err
	}
	l, err := loadLayout(w, root)
	if err != nil {
		return false, err
	}
	runs, err := goRunningScripts(w, root, l.apiPath("tools/harness"), allowed)
	if err != nil {
		return false, err
	}
	return unlisted || runs, nil
}

// readShellAllowlist parses shell-allowlist.txt, read like protected-paths.txt: `<path> bootstrap|unported` per
// line, where the pattern is an exact repo path.
func readShellAllowlist(root string) ([]protectedPath, error) {
	entries, err := protectedPaths(filepath.Join(root, shellAllowFile))
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.kind != "bootstrap" && e.kind != "unported" {
			return nil, fmt.Errorf("%s: expected `%s bootstrap|unported`, got kind %q", shellAllowFile, e.pattern, e.kind)
		}
	}
	return entries, nil
}

// unlistedShell prints each shell file the allowlist does not hold.
func unlistedShell(w world, root string, allowed []protectedPath) (bool, error) {
	files, err := w.gitKnown(root, "scripts", ".claude/hooks")
	if err != nil {
		return false, err
	}
	bad := false
	for _, f := range files {
		if strings.HasPrefix(f, fixturesDir+"/") {
			continue
		}
		shell, err := isShellFile(root, f)
		if err != nil {
			return false, err
		}
		if shell && !slices.ContainsFunc(allowed, func(e protectedPath) bool { return e.pattern == f }) {
			printFinding(w.stdout, f, 1, newShellRule, "a new shell script; new harness logic is Go: write it as a .bin/harness subcommand")
			bad = true
		}
	}
	return bad, nil
}

// isShellFile reports whether the repo file f is a shell script: .sh or .bash (any case), or no extension (a
// dotfile has none) and no `#!` line naming another interpreter. Only the first 256 bytes are read, and a file
// with no extension that is not a regular file is an error.
func isShellFile(root, f string) (bool, error) {
	base := path.Base(f)
	switch ext := strings.ToLower(path.Ext(base)); {
	case ext == ".sh" || ext == ".bash":
		return true, nil
	case ext != "" && ext != base:
		return false, nil
	}
	if err := regularFile(root, f); err != nil {
		return false, err
	}
	file, err := os.Open(filepath.Join(root, f))
	if err != nil {
		return false, err
	}
	defer file.Close()
	head := make([]byte, 256)
	n, err := io.ReadFull(file, head)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return false, err
	}
	first, _, whole := strings.Cut(string(head[:n]), "\n")
	if !strings.HasPrefix(first, "#!") {
		return true, nil
	}
	// An empty `#!` line leaves the file to the shell, and a line longer than what was read may name one later.
	return strings.TrimSpace(first[2:]) == "" || !whole && n == len(head) || shellWordRE.MatchString(first), nil
}

// regularFile fails unless the repo file f is a regular file once links are followed: a dangling link, or a
// link to a device or a directory, is an error.
func regularFile(root, f string) error {
	info, err := os.Stat(filepath.Join(root, f))
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s: not a regular file; the harness reads it, so it must be one", f)
	}
	return nil
}

// goRunningScripts prints each string literal in the non-test Go files under dir (repo-relative) that names a
// shell script the allowlist does not mark bootstrap.
func goRunningScripts(w world, root, dir string, allowed []protectedPath) (bool, error) {
	files, err := w.gitKnown(root, dir)
	if err != nil {
		return false, err
	}
	bad, parsed := false, 0
	fset := token.NewFileSet()
	for _, f := range files {
		if path.Ext(f) != ".go" || strings.HasSuffix(f, "_test.go") || slices.Contains(strings.Split(f, "/"), "testdata") {
			continue
		}
		if err := regularFile(root, f); err != nil {
			return false, err
		}
		file, err := parser.ParseFile(fset, filepath.Join(root, f), nil, parser.SkipObjectResolution)
		if err != nil {
			return false, err
		}
		parsed++
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok {
				return true
			}
			s, ok := constString(lit)
			if !ok {
				return true
			}
			for _, name := range namedScripts(s, allowed) {
				printFinding(w.stdout, f, fset.Position(lit.Pos()).Line, goRunsShRule, "names the shell script %s; port what it does to Go instead of running it", name)
				bad = true
			}
			return true
		})
	}
	if parsed == 0 {
		return false, fmt.Errorf("no Go file under %s: check API_DIR in %s", dir, layoutFile)
	}
	return bad, nil
}

// namedScripts returns the scripts s names that are not bootstrap entries: a path under scripts/ or
// .claude/hooks/ (from there on) that is no bootstrap entry, or any other name whose file name is no bootstrap
// entry's.
func namedScripts(s string, allowed []protectedPath) []string {
	var names []string
	for _, t := range scriptTokenRE.FindAllString(s, -1) {
		i := scriptDirIndex(t)
		if i >= 0 {
			t = t[i:]
		}
		if !slices.ContainsFunc(allowed, func(e protectedPath) bool {
			return e.kind == "bootstrap" && (e.pattern == t || i < 0 && path.Base(e.pattern) == path.Base(t))
		}) {
			names = append(names, t)
		}
	}
	return names
}

// constString folds a string literal, or a parenthesized one, to its value.
func constString(e ast.Expr) (string, bool) {
	lit, ok := ast.Unparen(e).(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	return s, err == nil
}

// scriptDirIndex is where scripts/ or .claude/hooks/ starts a path segment in t, or -1.
func scriptDirIndex(t string) int {
	for _, dir := range []string{"scripts/", ".claude/hooks/"} {
		for i := 0; ; {
			j := strings.Index(t[i:], dir)
			if j < 0 {
				break
			}
			if i+j == 0 || t[i+j-1] == '/' {
				return i + j
			}
			i += j + 1
		}
	}
	return -1
}
