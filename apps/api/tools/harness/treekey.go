package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

// treeKey names what `make check` judges (ADR 0048): the working tree as `git add -A` would stage it, the commit
// scope `changed` compares it with, the stack, and what git does not stage as it is on disk. HEAD is not in it:
// committing a green tree needs no new check.
type treeKey struct {
	tree    string
	base    string // the merge base with BASE_REF, or noBase
	stack   string
	disk    string // the files git knows, as they are on disk
	ignored string // the ignored files, the tools in .bin among them
}

const noBase = "none"

func (k treeKey) String() string { return k.source() + " ignored=" + k.ignored }

// source is the part of the key a check must not change. The ignored files are what it left: it may build tools.
func (k treeKey) source() string {
	return "tree=" + k.tree + " base=" + k.base + " stack=" + k.stack + " disk=" + k.disk
}

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
	if k.tree, err = w.workingTree(root); err != nil {
		return treeKey{}, err
	}
	w.stderr = io.Discard
	if k.disk, err = w.diskHash(root); err != nil {
		return treeKey{}, err
	}
	k.ignored, err = w.ignoredHash(root)
	return k, err
}

// diskHash hashes the files git knows by content: git stages what filters and core.* settings make of them, and
// nothing of a change to a skip-worktree or assume-unchanged one.
func (w world) diskHash(root string) (string, error) {
	files, err := w.gitKnown(root)
	if err != nil {
		return "", err
	}
	return w.hashFiles(root, files, func(path string, _ fs.FileInfo) (string, error) {
		f, err := os.Open(path)
		if err != nil {
			return "", err
		}
		defer f.Close()
		if w.ctx != nil {
			defer context.AfterFunc(w.ctx, func() { _ = f.Close() })() // ends the read of a huge file
		}
		h := sha256.New()
		_, err = io.Copy(h, f)
		return fmt.Sprintf("%x", h.Sum(nil)), err
	})
}

// churn matches ignored files that the work after a check rewrites: caches, logs, SwiftPM and Xcode output, editor
// state, Claude Code's own settings.
var churn = regexp.MustCompile(`(^|/)(\.cache|\.turbo|\.vite(st-cache|-temp)?|\.build|\.tmp|\.swiftpm|DerivedData|xcuserdata|\.DS_Store)(/|$)` +
	`|\.(log|xcuserstate)$|(^|/)\.claude/settings\.local\.json$`)

// ignoredHash hashes the ignored files but churn by size and time: it opens none, as .env.local holds secrets.
func (w world) ignoredHash(root string) (string, error) {
	files, err := w.lsFiles(root, "-oi", "--exclude-standard")
	if err != nil {
		return "", err
	}
	files = slices.DeleteFunc(files, churn.MatchString)
	return w.hashFiles(root, files, func(_ string, info fs.FileInfo) (string, error) {
		return fmt.Sprint(info.Size(), info.ModTime().UnixNano()), nil
	})
}

// hashFiles hashes files in root by path, mode and what of returns for a regular one or the target of a link.
// A directory, which is a nested repository, is its path and mode.
func (w world) hashFiles(root string, files []string, of func(string, fs.FileInfo) (string, error)) (string, error) {
	slices.Sort(files) // git lists untracked files first, so staging one would change the order
	h := sha256.New()
	for _, f := range slices.Compact(files) {
		if w.ctx != nil && w.ctx.Err() != nil {
			return "", w.ctx.Err()
		}
		path := filepath.Join(root, f)
		info, err := os.Lstat(path)
		var content string
		switch {
		case err != nil:
		case info.Mode().IsRegular():
			content, err = of(path, info)
		case info.Mode()&fs.ModeSymlink != 0:
			content, err = os.Readlink(path)
		}
		if errors.Is(err, fs.ErrNotExist) {
			continue // gone since git listed it
		}
		if err != nil {
			return "", err
		}
		fmt.Fprintf(h, "%q %v %q\n", f, info.Mode(), content)
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// treeKeyWithin is treeKey with a limit of its own, for a key taken when the gate's is used up.
func (w world) treeKeyWithin(root string, limit time.Duration) (treeKey, error) {
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()
	w.ctx = ctx
	return w.treeKey(root)
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

// The stamps: the key of the tree `make check` last called green, and of the one the stop gate last held back.
const (
	greenStamp = ".cache/harness/check.stamp"
	heldStamp  = ".cache/harness/stop-gate.stamp"
)

// stamped reports whether k is the key in the stamp.
func (k treeKey) stamped(root, stamp string) bool {
	b, err := os.ReadFile(filepath.Join(root, stamp))
	return err == nil && string(b) == k.String()+"\n"
}

// stamp writes k to the stamp. The write is a rename, so two runs at once never leave a mix of both.
func (k treeKey) stamp(root, stamp string) error {
	file := filepath.Join(root, stamp)
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(file), filepath.Base(file)+".")
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
