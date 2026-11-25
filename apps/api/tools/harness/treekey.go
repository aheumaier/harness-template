package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// treeKey names what `make check` judges (ADR 0048): the working tree as `git add -A` would stage it, the commit
// scope `changed` compares it with, and the stack. HEAD is not in it: committing a green tree needs no new check.
type treeKey struct {
	tree  string
	base  string // the merge base with BASE_REF, or noBase
	stack string
}

const noBase = "none"

func (k treeKey) String() string { return "tree=" + k.tree + " base=" + k.base + " stack=" + k.stack }

// treeKey computes the key of root and writes nothing to the repository: the index is a copy, and new objects go
// to a temporary directory.
func (w world) treeKey(root string) (treeKey, error) {
	l, err := loadLayout(w, root)
	if err != nil {
		return treeKey{}, err
	}
	k := treeKey{base: noBase, stack: l.get("STACK")}
	if base, err := w.mergeBase(root, w.getenv("BASE_REF")); err == nil {
		k.base = base
	}
	k.tree, err = w.workingTree(root)
	return k, err
}

// workingTree is the id of the tree `git add -A` would stage.
func (w world) workingTree(root string) (string, error) {
	tmp, err := os.MkdirTemp(w.getenv("TMPDIR"), "harness-tree-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	var stderr bytes.Buffer
	w.stderr = &stderr
	git := func(args ...string) (string, error) {
		// A split index would write its shared part to the repository, and a hook could write anything.
		out, err := w.git(root, append([]string{"-c", "core.splitIndex=false", "-c", "core.hooksPath=/dev/null"}, args...)...)
		if err != nil {
			err = fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
		}
		return strings.TrimSpace(string(out)), err
	}
	index, err := git("rev-parse", "--path-format=absolute", "--git-path", "index")
	if err != nil {
		return "", err
	}
	objects, err := git("rev-parse", "--path-format=absolute", "--git-path", "objects")
	if err != nil {
		return "", err
	}
	copied, added := filepath.Join(tmp, "index"), filepath.Join(tmp, "objects")
	if err := errors.Join(copyIndex(index, copied), os.Mkdir(added, 0o700)); err != nil {
		return "", err
	}
	w.environ = append(slices.Clone(w.environ), "GIT_INDEX_FILE="+copied, "GIT_OBJECT_DIRECTORY="+added,
		"GIT_ALTERNATE_OBJECT_DIRECTORIES="+objects)
	if _, err := git("add", "-A"); err != nil {
		return "", err
	}
	return git("write-tree")
}

// copyIndex copies a git index with its modification time: git reads the content of a file that is not older than
// the index, and under a newer index an edit of the same size in the second of the last commit would read as
// unchanged. A repository without an index (nothing staged yet) needs no copy.
func copyIndex(from, to string) error {
	info, err := os.Stat(from) // before the read: an index replaced in between gets the older time, which is safe
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	b, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	if err := os.WriteFile(to, b, 0o600); err != nil {
		return err
	}
	return os.Chtimes(to, info.ModTime(), info.ModTime())
}

// checkStampFile holds the key of the tree `make check` last called green.
const checkStampFile = ".cache/harness/check.stamp"

// stamped reports whether `make check` last called k green.
func (k treeKey) stamped(root string) bool {
	b, err := os.ReadFile(filepath.Join(root, checkStampFile))
	return err == nil && string(b) == k.String()+"\n"
}

// stamp records k as green. The write is a rename, so two runs at once never leave a mix of both.
func (k treeKey) stamp(root string) error {
	file := filepath.Join(root, checkStampFile)
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(file), "check.stamp.")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // gone after the rename
	_, err = fmt.Fprintln(tmp, k)
	if err := errors.Join(err, tmp.Close()); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), file)
}
