package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// shellRepo is a linkRepo with the layout (API_DIR=svc), a shell allowlist, one harness Go file and files
// (path → lines).
func shellRepo(t *testing.T, files map[string][]string) string {
	t.Helper()
	all := map[string][]string{
		layoutFile:                  withLayout("API_DIR", "svc"),
		shellAllowFile:              {"# test", "scripts/harness/layout.sh bootstrap", "", "scripts/old.sh unported"},
		"scripts/harness/layout.sh": {"#!/usr/bin/env bash"},
		"scripts/old.sh":            {"#!/usr/bin/env bash"},
		"svc/tools/harness/ok.go":   {"package main"},
	}
	for p, lines := range files {
		all[p] = lines
	}
	return linkRepo(t, all)
}

func TestNoNewShell(t *testing.T) {
	t.Parallel()
	root := shellRepo(t, map[string][]string{
		shellAllowFile:                          {"# test", "scripts/harness/layout.sh bootstrap", "", "scripts/old.sh unported", "scripts/gone.sh unported"},
		".claude/hooks/new-hook.sh":             {"echo"},
		".claude/hooks/hook.py":                 {"#!/usr/bin/env python3"},
		"scripts/data":                          {"#!/bin/shx"},
		"scripts/empty-shebang":                 {"#! "},
		"scripts/harness/.zz":                   {"#!/bin/bash"},
		"scripts/long-shebang":                  {"#!/usr/bin/env" + strings.Repeat(" ", 250) + " bash"},
		"scripts/lib.bash":                      {"f() { :; }"},
		"scripts/new.sh":                        {"echo"},
		"scripts/notes":                         {"echo run with bash"},
		"scripts/readme.md":                     {"Run scripts/new.sh."},
		"scripts/tool":                          {"#!/usr/bin/env -S bash -e"},
		"scripts/UP.SH":                         {"echo"},
		"scripts/zsh-tool":                      {"#! /bin/zsh"},
		"scripts/harness/fixtures/x/files/a.sh": {"echo"},
		"scripts/templates/t.sh":                {"echo"},
		"scripts/templates/t.tmpl":              {"echo"},
		"docs/a.sh":                             {"echo"},
		"svc/tools/harness/run_test.go":         {"package main", `var x = "scripts/new.sh"`},
		"svc/tools/harness/testdata/bad.go":     {"not go"},
		"svc/tools/harness/README.md":           {"Runs scripts/new.sh."},
		"svc/tools/harness/run.go": {
			"package main",
			"",
			`import "path/filepath"`,
			"",
			"// Comments name scripts/old.sh and do not count.",
			"var (",
			`	a = filepath.Join("scripts", "old.sh")`,
			`	b = "scripts/harness/layout.sh"`,
			`	c = filepath.Join("x", "layout.sh")`,
			`	d = "$ROOT/scripts/old.sh --flag"`,
			`	e = "cd scripts && ./OLD.SH --all"`,
			"	f = `bash .claude/hooks/foo.sh`",
			`	g = "myscripts/x.sh"`,
			`	h = "harness/layout.sh"`,
			`	i = "bash zz.sh"`,
			`	j = "harness/LAYOUT.SH"`,
			")",
		},
	})
	newShell := func(f string) string {
		return f + ":1: a new shell script; new harness logic is Go: write it as a .bin/harness subcommand [harness-no-new-shell]"
	}
	runs := func(line, name string) string {
		return "svc/tools/harness/run.go:" + line + ": names the shell script " + name + "; port what it does to Go instead of running it [harness-go-runs-no-script]"
	}
	wantFindings(t, func(out io.Writer) (bool, error) { return noNewShell(testWorld(t, out), root) }, []string{
		newShell(".claude/hooks/new-hook.sh"), newShell("scripts/UP.SH"), newShell("scripts/empty-shebang"),
		newShell("scripts/harness/.zz"), newShell("scripts/lib.bash"), newShell("scripts/long-shebang"),
		newShell("scripts/new.sh"), newShell("scripts/notes"), newShell("scripts/templates/t.sh"),
		newShell("scripts/tool"), newShell("scripts/zsh-tool"),
		runs("7", "old.sh"), runs("10", "scripts/old.sh"), runs("11", "./OLD.SH"), runs("12", ".claude/hooks/foo.sh"),
		runs("13", "myscripts/x.sh"), runs("15", "zz.sh"), runs("16", "harness/LAYOUT.SH"),
	})
}

func TestNoNewShellClean(t *testing.T) {
	t.Parallel()
	wantFindings(t, func(out io.Writer) (bool, error) { return noNewShell(testWorld(t, out), shellRepo(t, nil)) }, nil)
}

func TestNoNewShellListedLink(t *testing.T) {
	t.Parallel()
	root := shellRepo(t, map[string][]string{
		shellAllowFile: {"# test", "scripts/harness/layout.sh bootstrap", "scripts/hook bootstrap", "scripts/old.sh unported"},
	})
	if err := os.Symlink("old.sh", filepath.Join(root, "scripts/hook")); err != nil {
		t.Fatal(err)
	}
	wantFindings(t, func(out io.Writer) (bool, error) { return noNewShell(testWorld(t, out), root) }, nil)
}

func TestNoNewShellFailsClosed(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		setup func(root string) error
		want  string
	}{
		"no allowlist": {func(root string) error { return os.Remove(filepath.Join(root, shellAllowFile)) }, "shell-allowlist.txt"},
		"malformed line": {func(root string) error {
			return os.WriteFile(filepath.Join(root, shellAllowFile), []byte("scripts/old.sh ported\n"), 0o644)
		}, "bootstrap|unported"},
		"no layout": {func(root string) error { return os.Remove(filepath.Join(root, layoutFile)) }, "layout.env"},
		"no harness Go": {func(root string) error {
			return os.Remove(filepath.Join(root, "svc/tools/harness/ok.go"))
		}, "no Go file under svc/tools/harness"},
		"unparsable Go": {func(root string) error {
			return os.WriteFile(filepath.Join(root, "svc/tools/harness/bad.go"), []byte("package main\nfunc {\n"), 0o644)
		}, "bad.go"},
		"dangling link": {func(root string) error {
			return os.Symlink("missing", filepath.Join(root, "scripts/dangling"))
		}, "scripts/dangling: no such file"},
		"link to a device": {func(root string) error {
			return os.Symlink(os.DevNull, filepath.Join(root, "scripts/zz"))
		}, "scripts/zz: not a regular file"},
		"Go file linked to a device": {func(root string) error {
			return os.Symlink(os.DevNull, filepath.Join(root, "svc/tools/harness/zz.go"))
		}, "svc/tools/harness/zz.go: not a regular file"},
		"not a git repo": {func(root string) error { return os.RemoveAll(filepath.Join(root, ".git")) }, "git"},
	} {
		root := shellRepo(t, nil)
		if err := c.setup(root); err != nil {
			t.Fatal(err)
		}
		if _, err := noNewShell(testWorld(t, io.Discard), root); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: got error %v, want one holding %q", name, err, c.want)
		}
	}
}

func TestShellShebang(t *testing.T) {
	t.Parallel()
	for line, want := range map[string]bool{
		"#!/bin/sh": true, "#!/bin/bash -e": true, "#! /bin/sh": true, "#!/usr/bin/env bash": true,
		"#!/usr/bin/env -S bash -e": true, "#!/usr/bin/env -Sbash -e": true, "#!/usr/bin/env -u FOO bash": true,
		"#!/bin/busybox sh": true, "#!/bin/zsh": true, "#!/bin/dash\r": true, "#!/bin/ksh": true,
		"#!/bin/shx": false, "#!/usr/bin/env python3": false, "#!/usr/bin/env node": false,
	} {
		if got := shellWordRE.MatchString(line); got != want {
			t.Errorf("%q: got %v, want %v", line, got, want)
		}
	}
}
