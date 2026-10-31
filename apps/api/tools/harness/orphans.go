package main

// orphans (repo-no-orphan-scripts, ADR 0031): nothing under scripts/ and no make target exists without a way
// to run it.
//
// Scripts: every git-known file under scripts/ (scripts/harness/fixtures/ and scripts/templates/ excluded) is
// named by its repo path in a referencer. Referencers are the Makefile, .github/workflows and actions,
// .claude/hooks and settings.json (executable: only their code counts, not `#` comments or make help text),
// .claude/skills and agents and the guides (Markdown outside docs/adr, docs/exec-plans, docs/generated, specs,
// .specify and fixtures; their whole text counts) and, transitively, every named shell script outside
// scripts/test/ (`.sh`, `.bash` or no extension), which is executable too. Go code, package.json and
// compose.yaml are not referencers. A path counts as a whole token; a `/` before it only when what precedes
// the `/` is `.`, `..` or a variable expansion (`$ROOT/scripts/x.sh`, `"${root}"/scripts/x.sh`), so
// `apps/scripts/x.sh` or a blob URL does not name scripts/x.sh. A file directly in a runner directory
// (runnerDirs: `bats scripts/test`, `core.hooksPath scripts/git-hooks`) is also named when the code of an
// executable referencer names that directory. Fixtures never count, so a script named only by a fixture is an
// orphan.
//
// Make targets: every target of the Makefile itself whose name matches `[a-z0-9][a-z0-9-]*` either has a
// `##` help comment on a header line that make help lists (`^[a-z0-9-]+:.*##`: one name, no blank before
// the colon) or is called by name from another target's prerequisites or recipe, a workflow, an action, a
// Claude hook, setting, skill or agent, a guide or a named script. Variable-named, file and double-colon
// targets and included makefiles are skipped; a target-specific variable line is not a call.
//
// Run from `make orphans` (make check-all). Fails closed (exit 2) when the Makefile is missing or git fails.

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const orphanRule = "repo-no-orphan-scripts"

// runnerDirs are the directories whose files a tool runs by naming the directory.
var runnerDirs = map[string]bool{"scripts/test": true, "scripts/git-hooks": true}

var (
	// A rule header: one or more plain target names, a colon that is not an assignment (`:=`, `::=`) or a
	// double-colon rule.
	makeHeaderRE = regexp.MustCompile(`^([a-z0-9][a-z0-9-]*(?:[ \t]+[a-z0-9][a-z0-9-]*)*)[ \t]*:([^:=].*)?$`)
	// The header lines `make help` lists (its awk: `/^[a-z0-9-]+:.*##/`).
	makeHelpRE = regexp.MustCompile(`^[a-z0-9-]+:.*##`)
	// What follows the colon of a target-specific variable line: `VAR = x`, `VAR := x`, `export VAR …`.
	makeAssignRE = regexp.MustCompile(`^[ \t]*((export|override)[ \t]|[A-Za-z_][A-Za-z0-9_.-]*[ \t]*(=|:=|::=|\?=|\+=|!=))`)
	// Markdown that records history or is generated, not a guide anyone follows. Twin of NOT_GUIDE_RE in
	// scripts/harness/agent-gc.sh (findings there are not agent-gc's to fix): change both.
	notGuideRE = regexp.MustCompile(`^(docs/(adr|exec-plans|generated)/|specs/|\.specify/)|(^|/)fixtures/`)
	// What may precede the `/` before a repo path: `.`, `..` or a variable expansion (`$ROOT`, `$1`, `${ROOT}`,
	// `${{ github.workspace }}`, `$(CURDIR)`, `"$root"`).
	pathLeadRE = regexp.MustCompile(`(?:(?:^|[^A-Za-z0-9_.-])\.\.?|\$[A-Za-z0-9_]+|\$\{\{[^}]*\}\}|\$\{[^}]*\}|\$\([^)]*\)|"\$(?:[A-Za-z0-9_]+|\{[^}]*\}|\([^)]*\))")$`)
)

// referencer is the text of a file that can name a script or a make target.
type referencer struct {
	path string
	text string // what names a script or target: the code of an executable file, all of a guide
	exec bool   // runs (Makefile, workflow, action, Claude hook or setting, named script)
}

func newReferencer(path, text string, exec bool) referencer {
	if exec {
		text = stripComments(text)
	}
	return referencer{path: path, text: text, exec: exec}
}

// stripComments drops each line's `#` comment: a `#` at the start of the line or after a blank (`$#` and
// `${#x}` stay). Makefile help text (`## …`) goes with it.
func stripComments(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		for j := 0; j < len(line); j++ {
			if line[j] == '#' && (j == 0 || line[j-1] == ' ' || line[j-1] == '\t') {
				lines[i] = line[:j]
				break
			}
		}
	}
	return strings.Join(lines, "\n")
}

// orphans prints one line per finding to stdout and reports whether there was any. root is the repo.
func orphans(root string, stdout io.Writer) (bool, error) {
	mk, err := os.ReadFile(filepath.Join(root, "Makefile"))
	if err != nil {
		return false, err
	}
	refs, err := collectReferencers(root, string(mk))
	if err != nil {
		return false, err
	}
	orphaned, refs, err := reachScripts(root, refs)
	if err != nil {
		return false, err
	}
	for _, s := range orphaned {
		fmt.Fprintf(stdout, "%s:1: no Makefile target, workflow, hook, skill, agent, guide or named script names this file; wire it up or delete it [%s]\n", s, orphanRule)
	}
	uncalled := uncalledTargets(string(mk), refs)
	for _, t := range uncalled {
		fmt.Fprintf(stdout, "Makefile:%d: make target %s has no '##' help comment and nothing calls it; document it for make help, call it or delete it [%s]\n", t.line, t.name, orphanRule)
	}
	return len(orphaned)+len(uncalled) > 0, nil
}

// collectReferencers reads the Makefile (given), workflows, actions, Claude hooks, settings, skills and agents,
// and the guides.
func collectReferencers(root, mk string) ([]referencer, error) {
	refs := []referencer{newReferencer("Makefile", mk, true)}
	groups := []struct {
		exec      bool
		guide     bool
		pathspecs []string
	}{
		{true, false, []string{".github/workflows", ".github/actions", ".claude/hooks", ".claude/settings.json"}},
		{false, false, []string{".claude/skills", ".claude/agents"}},
		{false, true, []string{"*.md"}},
	}
	for _, g := range groups {
		files, err := gitKnown(root, g.pathspecs...)
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			if g.guide && notGuideRE.MatchString(f) {
				continue
			}
			b, err := os.ReadFile(filepath.Join(root, f))
			if err != nil {
				return nil, err
			}
			refs = append(refs, newReferencer(f, string(b), g.exec))
		}
	}
	return refs, nil
}

// reachScripts returns the scripts no referencer names, sorted, and refs with every named shell script added:
// a named script names others in turn, until nothing new is named.
func reachScripts(root string, refs []referencer) ([]string, []referencer, error) {
	scripts, err := gitKnown(root, "scripts")
	if err != nil {
		return nil, nil, err
	}
	var pending []string
	for _, s := range scripts {
		if !strings.HasPrefix(s, fixturesDir+"/") && !strings.HasPrefix(s, "scripts/templates/") {
			pending = append(pending, s)
		}
	}
	for changed := true; changed; {
		changed = false
		rest := pending[:0:0]
		for _, s := range pending {
			if !namedScript(refs, s) {
				rest = append(rest, s)
				continue
			}
			changed = true
			// Tests call nothing. Data files do not either, and some list script paths without calling them
			// (protected-paths.txt names install-tools.sh, gen-schema-doc.sh, github-branch-protection.sh).
			if ext := path.Ext(s); strings.HasPrefix(s, "scripts/test/") || ext != "" && ext != ".sh" && ext != ".bash" {
				continue
			}
			b, err := os.ReadFile(filepath.Join(root, s))
			if err != nil {
				return nil, nil, err
			}
			refs = append(refs, newReferencer(s, string(b), true))
		}
		pending = rest
	}
	sort.Strings(pending)
	return pending, refs, nil
}

// namedScript reports whether a referencer names s, or the code of an executable one s's runner directory.
func namedScript(refs []referencer, s string) bool {
	dir := path.Dir(s)
	for _, r := range refs {
		if mentions(r.text, s, pathLead) || (runnerDirs[dir] && r.exec && mentions(r.text, dir, pathLead)) {
			return true
		}
	}
	return false
}

// pathLead reports whether before, the text in front of a `/`, ends in `.`, `..` or a variable expansion.
func pathLead(before string) bool { return pathLeadRE.MatchString(before) }

// mentions reports whether text holds name as a whole token: not inside a longer name, not followed by `/`
// (a longer path or a glob) and not preceded by `/` unless lead accepts the text in front of that `/` (nil:
// never).
func mentions(text, name string, lead func(before string) bool) bool {
	for i := 0; ; {
		j := strings.Index(text[i:], name)
		if j < 0 {
			return false
		}
		start, end := i+j, i+j+len(name)
		i = start + 1
		if start > 0 {
			prev := text[start-1]
			if isNameByte(prev) || prev == '/' && (lead == nil || !lead(text[:start-1])) {
				continue
			}
		}
		if !continuesName(text, end) && (end == len(text) || text[end] != '/') {
			return true
		}
	}
}

type makeTarget struct {
	name string
	line int  // first header line
	help bool // a header line is listed by `make help`
}

// makeTargets lists the plain-named targets of a Makefile in order of first appearance.
func makeTargets(mk string) []makeTarget {
	var targets []makeTarget
	index := map[string]int{}
	for n, line := range strings.Split(mk, "\n") {
		m := makeHeaderRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		help := makeHelpRE.MatchString(line)
		for _, name := range strings.Fields(m[1]) {
			if k, ok := index[name]; ok {
				targets[k].help = targets[k].help || help
				continue
			}
			index[name] = len(targets)
			targets = append(targets, makeTarget{name: name, line: n + 1, help: help})
		}
	}
	return targets
}

// uncalledTargets lists the targets of mk that make help does not list and nothing calls.
func uncalledTargets(mk string, refs []referencer) []makeTarget {
	var uncalled []makeTarget
	for _, t := range makeTargets(mk) {
		if !t.help && !calledTarget(mk, refs, t) {
			uncalled = append(uncalled, t)
		}
	}
	return uncalled
}

// calledTarget reports whether anything but t's own rules names t: a prerequisite or recipe line of the
// Makefile outside them (comments, .PHONY, help text and target-specific variables excluded), or any other
// referencer.
func calledTarget(mk string, refs []referencer, t makeTarget) bool {
	for _, r := range refs {
		if r.path != "Makefile" && mentions(r.text, t.name, nil) {
			return true
		}
	}
	own := false
	for _, line := range strings.Split(mk, "\n") {
		if m := makeHeaderRE.FindStringSubmatch(line); m != nil {
			own = false
			for _, name := range strings.Fields(m[1]) {
				own = own || name == t.name
			}
			if own || makeAssignRE.MatchString(m[2]) {
				continue
			}
			line, _, _ = strings.Cut(m[2], "#")
		} else if !strings.HasPrefix(line, "\t") {
			own = false
		}
		if own || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ".PHONY") {
			continue
		}
		if mentions(stripComments(line), t.name, nil) {
			return true
		}
	}
	return false
}

// isNameByte reports whether b can be part of a file or target name.
func isNameByte(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b == '_' || b == '-' || b == '.'
}

// continuesName reports whether the name ending at text[end] goes on: a name byte, or a `.` that starts an
// extension (a `.` ending a sentence does not).
func continuesName(text string, end int) bool {
	if end >= len(text) {
		return false
	}
	if b := text[end]; b != '.' {
		return isNameByte(b)
	}
	return end+1 < len(text) && isNameByte(text[end+1]) && text[end+1] != '.'
}
