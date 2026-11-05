package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// The helpers the PreToolUse hooks share (guard-bash, protect-harness).

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

// physicalDir is dir with symlinks resolved, like `cd dir && pwd -P`; the working directory if dir is unusable.
func physicalDir(dir string) string {
	if dir != "" {
		if p, err := filepath.EvalSymlinks(dir); err == nil {
			if st, err := os.Stat(p); err == nil && st.IsDir() {
				return p
			}
		}
	}
	wd, _ := os.Getwd()
	if p, err := filepath.EvalSymlinks(wd); err == nil {
		return p
	}
	return wd
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
