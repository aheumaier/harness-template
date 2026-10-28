package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
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

var (
	renameName = regexp.MustCompile(`^[a-z][a-z0-9-]*[a-z0-9]$`) // npm scope, compose and Go friendly
	// host.tld/path like Go's module.CheckPath: a dot in the host, no element starting or ending in . or -.
	renameModule = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+(/[A-Za-z0-9_~]([A-Za-z0-9._~-]*[A-Za-z0-9_~])?)+$`)
	renameOwner  = regexp.MustCompile(`^@[A-Za-z0-9](-?[A-Za-z0-9]){0,38}(/[a-z0-9_-]+)?$`) // GitHub @user or @org/team
	// History keeps the name it was written under; README.md describes the template itself; this file and
	// its test hold the template identity on purpose.
	renameSkipped = regexp.MustCompile(`^(docs/adr/|docs/exec-plans/|README\.md$|apps/api/tools/harness/rename(_test)?\.go$)`)
)

type renamer struct {
	*strings.Replacer
	keptMod  bool     // the module path stays, so the name inside it is no leftover
	leftover []string // template tokens that should be gone after the rename
}

// newRenamer replaces in one pass, so a module path that contains the old name is not renamed twice in a
// run (checkRenameArgs keeps a second run a no-op). The module path comes first: at the same position the
// Replacer tries the pairs in argument order.
func newRenamer(name, module, owner string) renamer {
	r := renamer{
		Replacer: strings.NewReplacer(templateModule, module, templateName, name, templateOwner, owner),
		keptMod:  module == templateModule,
	}
	if name != templateName {
		r.leftover = append(r.leftover, templateName)
	}
	if !r.keptMod && owner != templateOwner {
		r.leftover = append(r.leftover, strings.TrimPrefix(templateOwner, "@")) // e.g. git@github.com:aheumaier/…
	}
	return r
}

// leftovers lists the lines of s that still hold a template token: a form the three tokens don't cover.
func (r renamer) leftovers(s string) []int {
	var lines []int
	if r.keptMod {
		s = strings.ReplaceAll(s, templateModule, "")
	}
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

// checkRenameArgs validates the new identity. A value that contains a template token it does not equal is
// refused: a second run would rename it again (@aheumaier/platform → @aheumaier/platform/platform).
func checkRenameArgs(name, module, owner string) error {
	switch {
	case !renameName.MatchString(name):
		return fmt.Errorf("name %q: want lowercase letters, digits and dashes, e.g. acme-shop", name)
	case !renameModule.MatchString(module):
		return fmt.Errorf("module %q: want a Go module path host.tld/path without a trailing slash, e.g. github.com/acme/shop", module)
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
// path and line that still holds a template token, for a hand fix.
func renameRepo(root, name, module, owner string, write bool, stdout io.Writer) error {
	if err := checkRenameArgs(name, module, owner); err != nil {
		return err
	}
	// Tracked files only: untracked work in progress is not ours to rewrite.
	files, err := lsFiles(root, "-c")
	if err != nil {
		return err
	}
	r := newRenamer(name, module, owner)
	changed := 0
	var left []string
	for _, f := range files {
		if renameSkipped.MatchString(f) {
			continue
		}
		if r.leftovers(f) != nil {
			left = append(left, f+": path")
		}
		path := filepath.Join(root, f)
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			continue
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if bytes.IndexByte(b, 0) >= 0 {
			continue // binary
		}
		out := r.Replace(string(b))
		for _, n := range r.leftovers(out) {
			left = append(left, fmt.Sprintf("%s:%d", f, n))
		}
		if out == string(b) {
			continue
		}
		changed++
		fmt.Fprintf(stdout, "%s: %d\n", f, changedLines(string(b), out))
		if write {
			if err := os.WriteFile(path, []byte(out), info.Mode().Perm()); err != nil {
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
