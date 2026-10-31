package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// The template's identity, as it appears in package names, the Go module path, the OpenAPI URL and
// CODEOWNERS. A new product renames all of it once (make rename).
const (
	templateName   = "harness-template"
	templateModule = "github.com/aheumaier/harness-template"
	templateOwner  = "@aheumaier"
)

// MODULE=skip keeps every Go module path and renames only the name and the owner: a brownfield repo keeps its
// own module (exec plan 0003, P4.8). It shadows a bare module literally named skip.
const skipModule = "skip"

// Go module path syntax like Go's module.CheckPath: no element starting or ending in . or -.
const (
	moduleHost = `[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+`
	moduleBare = `[A-Za-z0-9_~]([A-Za-z0-9_~-]*[A-Za-z0-9_~])?` // a first element without a dot
	moduleElem = `[A-Za-z0-9_~]([A-Za-z0-9._~-]*[A-Za-z0-9_~])?`
)

var (
	renameName = regexp.MustCompile(`^[a-z][a-z0-9-]*[a-z0-9]$`) // npm scope, compose and Go friendly
	// host.tld[/path], or a bare module whose first element has no dot (`go mod init example/hello`).
	renameModule = regexp.MustCompile(`^(` + moduleHost + `|` + moduleBare + `)(/` + moduleElem + `)*$`)
	renameOwner  = regexp.MustCompile(`^@[A-Za-z0-9](-?[A-Za-z0-9]){0,38}(/[a-z0-9_-]+)?$`) // GitHub @user or @org/team
	// History keeps the name it was written under; README.md describes the template itself; this file and
	// its test hold the template identity on purpose, wherever API_DIR puts the harness tool.
	renameSkipped = regexp.MustCompile(`^(docs/adr/|docs/exec-plans/|README\.md$)|(^|/)tools/harness/rename(_test)?\.go$`)
	goModuleLine  = regexp.MustCompile(`(?m)^module[ \t]+(\S+)`)
	// The rename is the moment a repo stops being the template: from then on template sync
	// (scripts/harness/sync-check.sh, ADR 0026) compares it with the template's URL, written here verbatim,
	// neither renamed nor reported as a leftover.
	upstreamFile = "scripts/tool-versions.env"
	upstreamLine = "HARNESS_UPSTREAM=https://" + templateModule
	upstreamRe   = regexp.MustCompile(`(?m)^HARNESS_UPSTREAM=.*$`)
)

// setUpstream points HARNESS_UPSTREAM in tool-versions.env at the template, appending the line if missing.
func setUpstream(s string) string {
	if upstreamRe.MatchString(s) {
		return upstreamRe.ReplaceAllLiteralString(s, upstreamLine)
	}
	if s != "" && !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	return s + upstreamLine + "\n"
}

type renamer struct {
	*strings.Replacer
	module   string   // the new module path: it may keep the old name or owner on purpose, so it is no leftover
	leftover []string // template tokens that should be gone after the rename
}

// newRenamer replaces in one pass, so a module path that contains the old name is not renamed twice in a
// run (checkRenameArgs keeps a second run a no-op). The module path comes first: at the same position the
// Replacer tries the pairs in argument order. A bare module keeps the template's https:// URL, since
// https://<bare module> is no URL; it holds the old tokens, so leftovers lists it for a hand fix. Any other
// form (http://, pkg.go.dev/) is renamed unreported, as README.md says.
func newRenamer(name, module, owner string) renamer {
	pairs := []string{templateModule, module, templateName, name, templateOwner, owner}
	if host, _, _ := strings.Cut(module, "/"); !strings.Contains(host, ".") {
		url := "https://" + templateModule
		pairs = append([]string{url, url}, pairs...)
	}
	r := renamer{Replacer: strings.NewReplacer(pairs...), module: module}
	if name != templateName {
		r.leftover = append(r.leftover, templateName)
	}
	if owner != templateOwner {
		r.leftover = append(r.leftover, strings.TrimPrefix(templateOwner, "@")) // e.g. git@github.com:aheumaier/…
	}
	return r
}

// leftovers lists the lines of s that still hold a template token: a form the three tokens don't cover.
func (r renamer) leftovers(s string) []int {
	var lines []int
	s = stripModule(s, r.module)
	s = strings.ReplaceAll(s, upstreamLine, "")
	for i, l := range strings.Split(s, "\n") {
		for _, tok := range r.leftover {
			if strings.Contains(l, tok) {
				lines = append(lines, i+1)
				break
			}
		}
	}
	return lines
}

// stripModule removes each occurrence of module from s that no letter, digit, -, _ or ~ touches, so a module
// that is part of a longer word (harness in harness-template) hides no leftover.
func stripModule(s, module string) string {
	var b strings.Builder
	for {
		i := strings.Index(s, module)
		if i < 0 {
			return b.String() + s
		}
		j := i + len(module)
		if (i > 0 && isWordByte(s[i-1])) || (j < len(s) && isWordByte(s[j])) {
			j = i + 1 // inside a longer word: keep this byte, look again from the next
			b.WriteString(s[:j])
		} else {
			b.WriteString(s[:i])
		}
		s = s[j:]
	}
}

func isWordByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.IndexByte("-_~", c) >= 0
}

// harnessModule gives the harness tool's go.mod (s, already renamed) the module path <module>/<dir>, dir being
// <API_DIR>/tools/harness (ADR 0030), so it names a directory the product has rather than the template's
// apps/api; a kept module (skip) keeps its path. It returns the content and its module path ("" for none).
// For API_DIR=apps/api this is the path the Replacer already wrote.
func (r renamer) harnessModule(s, dir string) (string, string) {
	if r.module != templateModule {
		s = goModuleLine.ReplaceAllLiteralString(s, "module "+path.Join(r.module, dir))
	}
	if m := goModuleLine.FindStringSubmatch(s); m != nil {
		return s, m[1]
	}
	return s, ""
}

// moduleArg is the module path a MODULE value stands for: skip keeps the template's.
func moduleArg(module string) string {
	if module == skipModule {
		return templateModule
	}
	return module
}

// checkRenameArgs validates the new identity. A value that contains a template token it does not equal is
// refused: a second run would rename it again (@aheumaier/platform → @aheumaier/platform/platform).
func checkRenameArgs(name, module, owner string) error {
	switch {
	case !renameName.MatchString(name):
		return fmt.Errorf("name %q: want lowercase letters, digits and dashes, e.g. acme-shop", name)
	case !renameModule.MatchString(module):
		return fmt.Errorf("module %q: want a Go module path without a trailing slash, host.tld[/path] or bare (e.g. github.com/acme/shop, example.com, shop), or %s to keep every module path", module, skipModule)
	case !renameOwner.MatchString(owner):
		return fmt.Errorf("owner %q: want a GitHub @user or @org/team", owner)
	case name == templateName && module == templateModule && owner == templateOwner:
		return errors.New("name, module and owner are the template's own; nothing to rename")
	}
	ids := []struct{ what, val, own string }{
		{"name", name, templateName}, {"module", module, templateModule}, {"owner", owner, templateOwner},
	}
	for _, v := range ids {
		if v.val == v.own {
			continue
		}
		for _, p := range ids {
			if p.val != p.own && strings.Contains(v.val, p.own) {
				return fmt.Errorf("%s %q contains the template's %q, so a second run would rename it again; pick a value without it", v.what, v.val, p.own)
			}
		}
	}
	return nil
}

// renameRepo renames the template identity in every tracked text file under root. Without write it only
// lists the files that would change. The output names each file and its number of changed lines, then every
// path and line that still holds a template token, for a hand fix. It also names the harness tool's module
// path (harnessModule). Module skip keeps every module path.
func renameRepo(root, name, module, owner string, write bool, stdout io.Writer) error {
	module = moduleArg(module)
	if err := checkRenameArgs(name, module, owner); err != nil {
		return err
	}
	l, err := loadLayout(root) // fails closed, before anything is written
	if err != nil {
		return err
	}
	// Tracked files only: untracked work in progress is not ours to rewrite.
	files, err := lsFiles(root, "-c")
	if err != nil {
		return err
	}
	r := newRenamer(name, module, owner)
	harnessDir := l.apiPath("tools/harness")
	harnessGoMod, harnessMod := path.Join(harnessDir, "go.mod"), ""
	changed := 0
	var left []string
	for _, f := range files {
		if renameSkipped.MatchString(f) {
			continue
		}
		if r.leftovers(f) != nil {
			left = append(left, f+": path")
		}
		abs := filepath.Join(root, f)
		info, err := os.Lstat(abs)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			continue
		}
		b, err := os.ReadFile(abs)
		if err != nil {
			return err
		}
		if bytes.IndexByte(b, 0) >= 0 {
			continue // binary
		}
		out := r.Replace(string(b))
		switch f { // after the rewrite
		case upstreamFile:
			out = setUpstream(out) // the template's URL stays the template's
		case harnessGoMod:
			out, harnessMod = r.harnessModule(out, harnessDir)
		}
		for _, n := range r.leftovers(out) {
			left = append(left, fmt.Sprintf("%s:%d", f, n))
		}
		if out == string(b) {
			continue
		}
		changed++
		fmt.Fprintf(stdout, "%s: %d\n", f, changedLines(string(b), out))
		if write {
			if err := os.WriteFile(abs, []byte(out), info.Mode().Perm()); err != nil {
				return err
			}
		}
	}
	switch {
	case !write:
		fmt.Fprintf(stdout, "rename: %d files would change (dry run; WRITE=1 applies)\n", changed)
	case changed > 0:
		fmt.Fprintf(stdout, "rename: %d files changed; next: make gen check-all harness-selftest, then commit\n", changed)
	default:
		fmt.Fprintln(stdout, "rename: 0 files changed")
	}
	if harnessMod != "" {
		fmt.Fprintf(stdout, "rename: harness tool module: %s (%s)\n", harnessMod, harnessGoMod)
	} else {
		fmt.Fprintf(stdout, "rename: harness tool module: %s is not tracked or has no module line; copy the template's tools/harness there (README.md, \"(b) Brownfield project\", step 2)\n", harnessGoMod)
	}
	if len(left) > 0 {
		fmt.Fprintf(stdout, "rename: %d places still hold %s; rename them by hand:\n", len(left), strings.Join(r.leftover, " or "))
		for _, l := range left {
			fmt.Fprintln(stdout, "  "+l)
		}
	}
	return nil
}

func changedLines(before, after string) int {
	a, b := strings.Split(before, "\n"), strings.Split(after, "\n")
	n := 0
	for i := range a {
		if i < len(b) && a[i] != b[i] {
			n++
		}
	}
	return n
}
