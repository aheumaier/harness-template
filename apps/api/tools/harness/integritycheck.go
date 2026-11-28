package main

// integrity-check: the change (working tree vs the merge base with BASE_REF) must not weaken the harness.
// Run from `make harness-integrity` (check-all) and .github/workflows/harness-integrity.yml.
//
//	[harness-suppression-reason]   an added suppression lacks `reason: … (rule: <id>)` on its line, names an
//	                               id that is not a `### <id>` heading in docs/harness/rules.md, or is an
//	                               @ts-ignore/@ts-nocheck/jscpd:ignore/swift-format-ignore (never allowed)
//	[harness-config-not-loosened]  a harness config (`config` in protected-paths.txt) gained or lost a
//	                               non-comment line (a shebang is not a comment), was added without one, or
//	                               changed its file mode (a hook without its x bit no longer runs).
//	                               Waived only by a human: HARNESS_CHANGE_APPROVED=1 (CI: the
//	                               `harness-change-approved` PR label).
//
// The binary, its marker lists, protected-paths.txt and the rules.md that rule ids are looked up in come from
// its own checkout (own); the change is read from HARNESS_ROOT (default: the same repo). CI builds the binary
// from the base branch and runs it against the PR head, so a PR cannot weaken the check that judges it (a
// suppression may cite a rule only once that rule is on the base branch; ADR 0010). Fails closed (exit 2)
// when the base or the diff cannot be read.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	// Hand-written source only: fixture overlays are deliberate violations and vendored code (.specify/,
	// spec-kit: ADR 0011) is not ours; suppressionPathspecs also drops generated code (GENERATED_GLOBS) and this
	// file, which names the markers it looks for.
	suppressionSources = []string{"*.go", "*.ts", "*.tsx", "*.mts", "*.cts", "*.js", "*.jsx", "*.mjs", "*.cjs",
		"*.sh", "*.bash", "*.bats", "*.sql", "*.swift", ".github/*.yml", ".github/*.yaml", "scripts/git-hooks/*",
		":!:scripts/harness/fixtures/*/files/*", ":!:.specify/*"}
	// jscpd honours `jscpd:ignore-start`/`-end` comments; repo-no-clones has no sanctioned exception there.
	// swift-format honours `swift-format-ignore` only with nothing after the rule names, so no reason fits on
	// its line (ADR 0043).
	neverRE = regexp.MustCompile(`(@ts-ignore|@ts-nocheck|jscpd:ignore|swift-format-ignore)`)
	// SwiftLint skips Foundation's whitespace and newlines (U+200B too, on macOS) before `disable`: any run of
	// non-letters and non-digits there counts, so no invisible character hides a suppression.
	suppressionRE = regexp.MustCompile(`(//[[:space:]]*(nolint|lint:(file-)?ignore)|eslint-disable|/\*[[:space:]]*eslint[[:space:]]|@ts-expect-error|#[[:space:]]*shellcheck[[:space:]]+disable|atlas:nolint|squawk-ignore|swiftlint:[^\pL\pN]*disable)`)
	reasonRE      = regexp.MustCompile(`reason: .+\(rule: ([a-z0-9-]+)\)`)
)

func init() {
	commands.register(rootSensor("integrity-check",
		"harness-suppression-reason, harness-config-not-loosened (make harness-integrity; env BASE_REF, HARNESS_ROOT, HARNESS_CHANGE_APPROVED)",
		func(w world, own string) (bool, error) {
			root, err := w.harnessRoot(own)
			if err != nil {
				return false, err
			}
			return integrityCheck(w, own, root, w.getenv("BASE_REF"), w.getenv("HARNESS_CHANGE_APPROVED") == "1")
		}))
}

// integrityCheck prints one line per finding to stdout and reports whether there was any.
func integrityCheck(w world, own, root, baseRef string, approved bool) (bool, error) {
	base, err := w.mergeBase(root, baseRef)
	if err != nil {
		return false, err
	}
	bad := false
	report := func(format string, a ...any) {
		fmt.Fprintf(w.stdout, format+"\n", a...)
		bad = true
	}

	rules, err := os.ReadFile(filepath.Join(own, rulesFile))
	if err != nil {
		return false, err
	}
	l, err := loadLayout(w, own)
	if err != nil {
		return false, err
	}
	rows, err := changes(w, root, base, suppressionPathspecs(l))
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
	protected := filepath.Join(own, protectedPathsFile)
	configs, err := configPatterns(protected)
	if err != nil {
		return false, err
	}
	if len(configs) == 0 {
		return false, fmt.Errorf("no `config` entries in %s: refusing to pass without checking harness configs", protected)
	}
	if !approved {
		rows, err := changes(w, root, base, configs)
		if err != nil {
			return false, err
		}
		seen := map[string]bool{}
		for _, c := range rows {
			// A shebang is code: `#!/usr/bin/true` turns a sensor script into a pass.
			if commentRE.MatchString(c.text) && (c.line != 1 || !strings.HasPrefix(c.text, "#!")) {
				continue
			}
			seen[c.path] = true
			what := "removed"
			if c.added {
				what = "added"
			}
			report("%s:%d: %s harness config line: %s [harness-config-not-loosened]", c.path, c.line, what, c.text)
		}
		modes, err := modeChanges(w, root, base, configs)
		if err != nil {
			return false, err
		}
		for _, m := range modes {
			if m[0] == "000000" && m[1] != "120000" { // a regular file, reported above when it has a config line
				if !seen[m[2]] {
					report("%s: empty or comment-only harness config added [harness-config-not-loosened]", m[2])
				}
			} else if m[0] == m[1] {
				report("%s: symlink target changed [harness-config-not-loosened]", m[2])
			} else {
				report("%s: file mode changed from %s to %s [harness-config-not-loosened]", m[2], m[0], m[1])
			}
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

// configPatterns returns the pathspecs marked `config` in protected-paths.txt.
func configPatterns(file string) ([]string, error) {
	entries, err := protectedPaths(file)
	if err != nil {
		return nil, err
	}
	var patterns []string
	for _, e := range entries {
		if e.kind == "config" {
			// Any case: on macOS's file system `.SWIFT-FORMAT-IGNORE` is the `.swift-format-ignore` swift-format reads.
			patterns = append(patterns, ":(icase)"+e.pattern)
		}
	}
	return patterns, nil
}

// modeChanges lists [old mode, new mode, path] for the files matching pathspecs whose mode changed since
// base; a diff without content lines (`chmod -x`, an added empty file) has no rows in changes: make reads an empty
// `GNUmakefile` instead of `Makefile`. Deleted files are left to changes. A symlink (120000 on either side) is listed
// whenever it changed: its one line is the target's name, which can look like a comment (`#notes`) while the tool
// reads the target.
func modeChanges(w world, root, base string, pathspecs []string) ([][3]string, error) {
	out, err := w.git(root, append([]string{"diff", "--raw", "-z", "--no-renames", "--no-ext-diff", base, "--"}, pathspecs...)...)
	if err != nil {
		return nil, err
	}
	var modes [][3]string
	f := strings.Split(string(out), "\x00")
	for i := 0; i+1 < len(f); i += 2 { // `:old new sha sha status` NUL path NUL
		meta := strings.Fields(strings.TrimPrefix(f[i], ":"))
		if len(meta) < 2 {
			continue
		}
		link := meta[0] == "120000" || meta[1] == "120000"
		if link || meta[0] != meta[1] && meta[1] != "000000" {
			modes = append(modes, [3]string{meta[0], meta[1], f[i+1]})
		}
	}
	// An untracked file has no raw row, and changes reads a symlink's target.
	untracked, err := w.git(root, append([]string{"ls-files", "-z", "-o", "--exclude-standard", "--"}, pathspecs...)...)
	if err != nil {
		return nil, err
	}
	for _, p := range strings.Split(string(untracked), "\x00") {
		fi, err := os.Lstat(filepath.Join(root, p))
		if p == "" || err != nil {
			continue
		}
		mode := "100644"
		if fi.Mode()&os.ModeSymlink != 0 {
			mode = "120000"
		}
		modes = append(modes, [3]string{"000000", mode, p})
	}
	return modes, nil
}

// changes lists the lines added and removed since base in the files matching pathspecs; untracked files
// count as added.
func changes(w world, root, base string, pathspecs []string) ([]change, error) {
	var rows []change
	untracked, err := w.git(root, append([]string{"ls-files", "-z", "-o", "--exclude-standard", "--"}, pathspecs...)...)
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
	diff, err := diffLines(w, root, []string{base}, pathspecs)
	if err != nil {
		return nil, err
	}
	for _, c := range diff {
		c.text = trimTabs(c.text)
		rows = append(rows, c)
	}
	return rows, nil
}

// trimTabs strips the leading and trailing tabs that the shell version's `IFS=$'\t' read` dropped, so a
// Makefile recipe line reads as before.
func trimTabs(s string) string { return strings.Trim(s, "\t") }
