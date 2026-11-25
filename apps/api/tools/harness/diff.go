package main

import (
	"regexp"
	"strconv"
	"strings"
)

// The line diff the sensors that judge a change read (integrity-check, specs-frozen).

var hunkRE = regexp.MustCompile(`^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@`)

// change is one added (+) or removed (-) line; removed lines carry their line number on the base side.
type change struct {
	path  string
	line  int
	added bool
	text  string
}

// diffLines lists the lines added and removed in the files matching pathspecs. revs is one revision (compared with
// the working tree) or two, after any further flags. The flags set here keep the answer independent of the user's git
// config and of the change's .gitattributes (`-diff`, `binary`, textconv), which must not turn a hunk into "Binary
// files differ".
func diffLines(w world, root string, revs, pathspecs []string) ([]change, error) {
	args := append([]string{"diff", "-U0", "--text", "--no-textconv", "--no-color", "--no-ext-diff",
		"--no-renames", "--src-prefix=a/", "--dst-prefix=b/"}, revs...)
	diff, err := w.git(root, append(append(args, "--"), pathspecs...)...)
	if err != nil {
		return nil, err
	}
	return parseDiff(diff), nil
}

// parseDiff reads `git diff -U0` output; a user's config or environment can still ask for context lines. The path
// comes from the `--- a/` and `+++ b/` lines (a deleted file's `+++ /dev/null` keeps the `a/` path).
func parseDiff(diff []byte) []change {
	var rows []change
	var path string
	var old, cur int
	header := false
	for _, l := range splitLines(diff) {
		switch {
		case strings.HasPrefix(l, "diff --git "):
			header = true
		case header && (strings.HasPrefix(l, "--- a/") || strings.HasPrefix(l, "+++ b/") ||
			strings.HasPrefix(l, `--- "a/`) || strings.HasPrefix(l, `+++ "b/`)):
			path = diffPath(l[4:])
		case strings.HasPrefix(l, "@@"):
			header = false
			if m := hunkRE.FindStringSubmatch(l); m != nil {
				old, _ = strconv.Atoi(m[1])
				cur, _ = strconv.Atoi(m[2])
			}
		case header:
		case strings.HasPrefix(l, "+"):
			rows = append(rows, change{path, cur, true, l[1:]})
			cur++
		case strings.HasPrefix(l, "-"):
			rows = append(rows, change{path, old, false, l[1:]})
			old++
		case l == "", strings.HasPrefix(l, " "): // diff.suppressBlankEmpty leaves a blank context line empty
			old++
			cur++
		}
	}
	return rows
}

// diffPath turns `a/x`, `b/x\t` (git appends a tab to paths with spaces) or `"b/x\303\244"` (git's C quoting
// of unusual characters) into x.
func diffPath(p string) string {
	p = strings.TrimSuffix(p, "\t")
	if strings.HasPrefix(p, `"`) {
		if u, err := strconv.Unquote(p); err == nil {
			p = u
		}
	}
	if len(p) < 2 {
		return p
	}
	return p[2:]
}

func splitLines(b []byte) []string {
	if len(b) == 0 {
		return nil
	}
	return strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
}
