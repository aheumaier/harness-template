package main

// integrity-check: the change (working tree vs the merge base with BASE_REF) must not weaken the harness.
// Run from `make harness-integrity` (check-all) and .github/workflows/harness-integrity.yml.
//
//	[harness-suppression-reason]   an added suppression lacks `reason: … (rule: <id>)` on its line, names an
//	                               id that is not a `### <id>` heading in docs/harness/rules.md, or is an
//	                               @ts-ignore/@ts-nocheck/jscpd:ignore (never allowed)
//	[harness-config-not-loosened]  a harness config (`config` in protected-paths.txt) gained or lost a
//	                               non-comment line (a shebang is not a comment), or changed its file
//	                               mode (a hook without its x bit no longer runs). Waived only by a human: HARNESS_CHANGE_APPROVED=1 (CI:
//	                               the `harness-change-approved` PR label).
//
// The binary, its marker lists, protected-paths.txt and the rules.md that rule ids are looked up in come from
// its own checkout (own); the change is read from HARNESS_ROOT (default: the same repo). CI builds the binary
// from the base branch and runs it against the PR head, so a PR cannot weaken the check that judges it (a
// suppression may cite a rule only once that rule is on the base branch; ADR 0010). Fails closed (exit 2)
// when the base or the diff cannot be read.

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	// Hand-written source only: fixture overlays are deliberate violations and vendored code (.specify/,
	// spec-kit: ADR 0011) is not ours; suppressionPathspecs also drops generated code (GENERATED_GLOBS) and this
	// file, which names the markers it looks for.
	suppressionSources = []string{"*.go", "*.ts", "*.tsx", "*.mts", "*.cts", "*.js", "*.jsx", "*.mjs", "*.cjs",
		"*.sh", "*.bash", "*.bats", "*.sql", ".github/*.yml", ".github/*.yaml", "scripts/git-hooks/*",
		":!:scripts/harness/fixtures/*/files/*", ":!:.specify/*"}
	// jscpd honours `jscpd:ignore-start`/`-end` comments; repo-no-clones has no sanctioned exception there.
	neverRE       = regexp.MustCompile(`(@ts-ignore|@ts-nocheck|jscpd:ignore)`)
	suppressionRE = regexp.MustCompile(`(//[[:space:]]*(nolint|lint:(file-)?ignore)|eslint-disable|/\*[[:space:]]*eslint[[:space:]]|@ts-expect-error|#[[:space:]]*shellcheck[[:space:]]+disable|atlas:nolint|squawk-ignore)`)
	reasonRE      = regexp.MustCompile(`reason: .+\(rule: ([a-z0-9-]+)\)`)
	// Only `#` lines and blank lines are comments: `//` and `/* */` can comment out a JSONC, JS or Go line
	// without the commented line itself looking like code.
	commentRE = regexp.MustCompile(`^[[:space:]]*(#|$)`)
	hunkRE    = regexp.MustCompile(`^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@`)
)

// change is one added (+) or removed (-) line; removed lines carry their line number on the base side.
type change struct {
	path  string
	line  int
	added bool
	text  string
}

// integrityCheck prints one line per finding to stdout and reports whether there was any.
func integrityCheck(own, root, baseRef string, approved bool, stdout io.Writer) (bool, error) {
	base, err := mergeBase(root, baseRef)
	if err != nil {
		return false, err
	}
	bad := false
	report := func(format string, a ...any) {
		fmt.Fprintf(stdout, format+"\n", a...)
		bad = true
	}

	rules, err := os.ReadFile(filepath.Join(own, "docs/harness/rules.md"))
	if err != nil {
		return false, err
	}
	l, err := loadLayout(own)
	if err != nil {
		return false, err
	}
	rows, err := changes(root, base, suppressionPathspecs(l))
	if err != nil {
		return false, err
	}
	for _, c := range rows {
		if !c.added {
			continue
		}
		if m := neverRE.FindStringSubmatch(c.text); m != nil {
			report("%s:%d: %s is never allowed [harness-suppression-reason]", c.path, c.line, m[1])
		} else if !suppressionRE.MatchString(c.text) {
			continue
		} else if m := reasonRE.FindStringSubmatch(c.text); m == nil {
			report("%s:%d: suppression without 'reason: … (rule: <id>)' on the same line [harness-suppression-reason]", c.path, c.line)
		} else if !hasLine(rules, "### "+m[1]) {
			report("%s:%d: suppression names '%s', not a rule id in docs/harness/rules.md [harness-suppression-reason]", c.path, c.line, m[1])
		}
	}

	// Any non-comment change counts, tightening included: telling a loosening from a tightening needs
	// judgment, and judgment is the human's (the label).
	protected := filepath.Join(own, "scripts/harness/protected-paths.txt")
	configs, err := configPatterns(protected)
	if err != nil {
		return false, err
	}
	if len(configs) == 0 {
		return false, fmt.Errorf("no `config` entries in %s: refusing to pass without checking harness configs", protected)
	}
	if !approved {
		rows, err := changes(root, base, configs)
		if err != nil {
			return false, err
		}
		for _, c := range rows {
			// A shebang is code: `#!/usr/bin/true` turns a sensor script into a pass.
			if commentRE.MatchString(c.text) && (c.line != 1 || !strings.HasPrefix(c.text, "#!")) {
				continue
			}
			what := "removed"
			if c.added {
				what = "added"
			}
			report("%s:%d: %s harness config line: %s [harness-config-not-loosened]", c.path, c.line, what, c.text)
		}
		modes, err := modeChanges(root, base, configs)
		if err != nil {
			return false, err
		}
		for _, m := range modes {
			report("%s: file mode changed from %s to %s [harness-config-not-loosened]", m[2], m[0], m[1])
		}
	}
	return bad, nil
}

// suppressionPathspecs is suppressionSources without the layout's generated code and this file.
func suppressionPathspecs(l layout) []string {
	specs := append([]string{":!:" + l.apiPath("tools/harness/integritycheck.go")}, suppressionSources...)
	for _, g := range l.list("GENERATED_GLOBS") {
		specs = append(specs, ":!:"+g)
	}
	return specs
}

// mergeBase is scripts/harness/merge-base.sh in root: the merge base of HEAD with ref (default: origin/main
// if it exists, else main). A sensor that cannot find its base would compare nothing and pass.
func mergeBase(root, ref string) (string, error) {
	verify := func(r string) bool {
		return exec.Command("git", "-C", root, "rev-parse", "-q", "--verify", r+"^{commit}").Run() == nil
	}
	if ref == "" {
		ref = "main"
		if verify("origin/main") {
			ref = "origin/main"
		}
	}
	if !verify(ref) {
		return "", fmt.Errorf("BASE_REF '%s' is not a commit in this clone: fetch it (git fetch origin main) or pass BASE_REF=<branch|sha>.", ref)
	}
	out, err := exec.Command("git", "-C", root, "merge-base", "HEAD", ref).Output()
	if err != nil {
		return "", fmt.Errorf("HEAD and BASE_REF '%s' have no merge base: fetch full history (git fetch --unshallow; CI: actions/checkout with fetch-depth: 0).", ref)
	}
	return strings.TrimSpace(string(out)), nil
}

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

// configPatterns returns the pathspecs marked `config` in protected-paths.txt.
func configPatterns(file string) ([]string, error) {
	entries, err := protectedPaths(file)
	if err != nil {
		return nil, err
	}
	var patterns []string
	for _, e := range entries {
		if e.kind == "config" {
			patterns = append(patterns, e.pattern)
		}
	}
	return patterns, nil
}

// git runs git in root and returns its stdout.
func git(root string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"-C", root, "-c", "core.quotePath=false"}, args...)...)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w", args[0], err)
	}
	return out, nil
}

// modeChanges lists [old mode, new mode, path] for the files matching pathspecs whose mode changed since
// base; a diff without content lines (`chmod -x`) has no rows in changes. Added and deleted files (mode
// 000000 on one side) are left to changes.
func modeChanges(root, base string, pathspecs []string) ([][3]string, error) {
	out, err := git(root, append([]string{"diff", "--raw", "-z", "--no-renames", "--no-ext-diff", base, "--"}, pathspecs...)...)
	if err != nil {
		return nil, err
	}
	var modes [][3]string
	f := strings.Split(string(out), "\x00")
	for i := 0; i+1 < len(f); i += 2 { // `:old new sha sha status` NUL path NUL
		meta := strings.Fields(strings.TrimPrefix(f[i], ":"))
		if len(meta) >= 2 && meta[0] != meta[1] && meta[0] != "000000" && meta[1] != "000000" {
			modes = append(modes, [3]string{meta[0], meta[1], f[i+1]})
		}
	}
	return modes, nil
}

// changes lists the lines added and removed since base in the files matching pathspecs; untracked files
// count as added. --text --no-textconv: the change's .gitattributes (`-diff`, `binary`, textconv) must not
// turn a hunk into "Binary files differ".
func changes(root, base string, pathspecs []string) ([]change, error) {
	var rows []change
	untracked, err := git(root, append([]string{"ls-files", "-z", "-o", "--exclude-standard", "--"}, pathspecs...)...)
	if err != nil {
		return nil, err
	}
	for _, f := range strings.Split(string(untracked), "\x00") {
		if f == "" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(root, f))
		if err != nil {
			return nil, err
		}
		for i, l := range splitLines(b) {
			rows = append(rows, change{f, i + 1, true, trimTabs(l)})
		}
	}
	diff, err := git(root, append([]string{"diff", "-U0", "--text", "--no-textconv", "--no-color", "--no-ext-diff",
		"--no-renames", "--src-prefix=a/", "--dst-prefix=b/", base, "--"}, pathspecs...)...)
	if err != nil {
		return nil, err
	}
	return append(rows, parseDiff(diff)...), nil
}

// parseDiff reads `git diff -U0` output. The path comes from the `--- a/` and `+++ b/` lines (a deleted
// file's `+++ /dev/null` keeps the `a/` path).
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
			rows = append(rows, change{path, cur, true, trimTabs(l[1:])})
			cur++
		case strings.HasPrefix(l, "-"):
			rows = append(rows, change{path, old, false, trimTabs(l[1:])})
			old++
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

// trimTabs strips the leading and trailing tabs that the shell version's `IFS=$'\t' read` dropped, so a
// Makefile recipe line reads as before.
func trimTabs(s string) string { return strings.Trim(s, "\t") }

func splitLines(b []byte) []string {
	if len(b) == 0 {
		return nil
	}
	return strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
}
