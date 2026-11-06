package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// The helpers the PreToolUse hooks share (guard-bash, protect-harness).

// hookGuard is a command run as `harness <name> <root>` with the hook JSON on stdin: guard gets root made
// absolute, and any error blocks with exit 2 (fail closed).
func hookGuard(name, about string, guard func(w world, root string) error) command {
	return command{name: name, usage: "harness " + name + " <root>", about: about, run: func(w world, args []string) int {
		if len(args) != 1 {
			return exitBadArgs
		}
		root, err := w.abs(args[0])
		if err == nil {
			err = guard(w, root)
		}
		if err != nil {
			fmt.Fprintf(w.stderr, "%s: blocked: %v\n", name, err)
			return 2
		}
		return 0
	}}
}

type blockError struct{ msg string }

func (e *blockError) Error() string { return e.msg }

func blockf(format string, a ...any) error { return &blockError{fmt.Sprintf(format, a...)} }

func set(words ...string) map[string]bool {
	m := make(map[string]bool, len(words))
	for _, w := range words {
		m[w] = true
	}
	return m
}

// physicalDir is dir with symlinks resolved, like `cd dir && pwd -P`; the working directory cwd if dir is
// unusable; "" if cwd is "" too (filepath.EvalSymlinks would make it ".", so a relative path would resolve
// against wherever the process happens to be). A relative dir is "": relative to what is unknown (Claude Code
// sends an absolute cwd), and falling back to cwd would read `../AGENTS.md` from the wrong directory.
func physicalDir(dir, cwd string) string {
	if dir != "" && !filepath.IsAbs(dir) {
		return ""
	}
	if dir != "" {
		if p, err := filepath.EvalSymlinks(dir); err == nil {
			if st, err := os.Stat(p); err == nil && st.IsDir() {
				return p
			}
		}
	}
	if cwd == "" {
		return ""
	}
	if p, err := filepath.EvalSymlinks(cwd); err == nil {
		return p
	}
	return cwd
}

var separator = regexp.MustCompile("[;&|\n`]|\\$\\(")

// segments splits text on `;&|`, newlines, backquotes and `$(`, then each part into whitespace-separated
// fields without leading quotes and grouping (`"`, `'`, `\`, `(`, `{`, backquote, `$(`) or trailing ones.
func segments(text string) [][]string {
	parts := separator.Split(text, -1)
	out := make([][]string, len(parts))
	for i, p := range parts {
		for _, f := range strings.Fields(p) {
			for len(f) > 0 && (strings.ContainsRune("\"'\\({`", rune(f[0])) || strings.HasPrefix(f, "$(")) {
				if strings.HasPrefix(f, "$(") {
					f = f[2:]
				} else {
					f = f[1:]
				}
			}
			f = strings.TrimRight(f, "\"')}`")
			out[i] = append(out[i], f)
		}
	}
	return out
}
