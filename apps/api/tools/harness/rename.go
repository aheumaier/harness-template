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
	renameName   = regexp.MustCompile(`^[a-z][a-z0-9-]*[a-z0-9]$`)           // npm scope, compose and Go friendly
	renameModule = regexp.MustCompile(`^[a-z0-9.-]+(/[A-Za-z0-9._~-]+)+$`)   // host/path, no trailing slash
	renameOwner  = regexp.MustCompile(`^@[A-Za-z0-9-]+(/[A-Za-z0-9._-]+)?$`) // @user or @org/team
	// History keeps the name it was written under; README.md describes the template itself; this file and
	// its test hold the template identity on purpose.
	renameSkipped = regexp.MustCompile(`^(docs/adr/|docs/exec-plans/|README\.md$|apps/api/tools/harness/rename(_test)?\.go$)`)
)

type renamer struct{ r *strings.Replacer }

// newRenamer replaces in one pass, so a new module path that contains the old name is not renamed twice.
// The module path comes first: at the same position the Replacer tries the pairs in argument order.
func newRenamer(name, module, owner string) renamer {
	return renamer{strings.NewReplacer(templateModule, module, templateName, name, templateOwner, owner)}
}

// replace returns s renamed and whether anything changed.
func (r renamer) replace(s string) (string, bool) {
	out := r.r.Replace(s)
	return out, out != s
}

// renameRepo renames the template identity in every git-known text file under root. Without write it only
// lists the files that would change. The output names each file and its number of changed lines.
func renameRepo(root string, args []string, write bool, stdout io.Writer) error {
	if len(args) != 3 {
		return errors.New("want <name> <module> <owner>")
	}
	name, module, owner := args[0], args[1], args[2]
	switch {
	case !renameName.MatchString(name):
		return fmt.Errorf("name %q: want lowercase letters, digits and dashes, e.g. acme-shop", name)
	case !renameModule.MatchString(module):
		return fmt.Errorf("module %q: want a Go module path without a trailing slash, e.g. github.com/acme/shop", module)
	case !renameOwner.MatchString(owner):
		return fmt.Errorf("owner %q: want a GitHub @user or @org/team", owner)
	case name == templateName && module == templateModule && owner == templateOwner:
		return errors.New("name, module and owner are the template's own; nothing to rename")
	}
	files, err := gitKnown(root)
	if err != nil {
		return err
	}
	r := newRenamer(name, module, owner)
	changed := 0
	for _, f := range files {
		if renameSkipped.MatchString(f) {
			continue
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
		out, ok := r.replace(string(b))
		if !ok {
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
