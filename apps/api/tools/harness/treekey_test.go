package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTreeKey(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		change func(t *testing.T, root string)
		env    []string
		same   bool
	}{
		{name: "nothing", change: func(*testing.T, string) {}, same: true},
		{name: "a tracked file", change: func(t *testing.T, root string) { writeLines(t, root, "a.go", "b") }},
		{name: "an untracked file", change: func(t *testing.T, root string) { writeLines(t, root, "new.go", "x") }},
		{name: "a file that git reads as unchanged", change: func(t *testing.T, root string) {
			gitT(t, root, "update-index", "--skip-worktree", "a.go")
			writeLines(t, root, "a.go", "b")
		}},
		{name: "a file that a filter stages as it was", change: func(t *testing.T, root string) {
			gitT(t, root, "config", "filter.x.clean", "echo a")
			writeLines(t, root, ".git/info/attributes", "*.go filter=x")
			writeLines(t, root, "a.go", "b")
		}},
		{name: "a tool", change: func(t *testing.T, root string) { writeLines(t, root, ".bin/tool", "x") }},
		{name: "an ignored file that a test reads", change: func(t *testing.T, root string) { writeLines(t, root, ".env.local", "x=2") }},
		{name: "an ignored link", change: func(t *testing.T, root string) {
			if err := os.Symlink("elsewhere", filepath.Join(root, ".bin/link")); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "a dependency", change: func(t *testing.T, root string) { writeLines(t, root, "node_modules/.pnpm/x@1/dist/x.js", "x") }},
		{name: "the mode of an ignored file", change: func(t *testing.T, root string) {
			if err := os.Chmod(filepath.Join(root, ".env.local"), 0); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "a cache", change: func(t *testing.T, root string) { writeLines(t, root, ".cache/x", "x") }, same: true},
		{name: "a tool's cache", change: func(t *testing.T, root string) { writeLines(t, root, "node_modules/.cache/x/y.json", "x") }, same: true},
		{name: "what the work after a check rewrites", change: func(t *testing.T, root string) {
			writeLines(t, root, "node_modules/.vitest-cache/x", "x")
			writeLines(t, root, "node_modules/.vite-temp/x", "x")
			writeLines(t, root, ".claude/settings.local.json", "x")
		}, same: true},
		{name: "a log", change: func(t *testing.T, root string) { writeLines(t, root, "node_modules/x.log", "x") }, same: true},
		{name: "a stamp", change: func(t *testing.T, root string) {
			if err := keyOf(t, root).stamp(root, heldStamp); err != nil {
				t.Fatal(err)
			}
		}, same: true},
		{name: "a deleted file", change: func(t *testing.T, root string) {
			if err := os.Remove(filepath.Join(root, "a.go")); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "a mode", change: func(t *testing.T, root string) {
			if err := os.Chmod(filepath.Join(root, "a.go"), 0o755); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "the merge base", change: func(t *testing.T, root string) {
			gitT(t, root, "commit", "-q", "--allow-empty", "--no-verify", "-m", "more")
			gitT(t, root, "branch", "-f", "main", "HEAD")
		}},
		{name: "the stack", change: func(*testing.T, string) {}, env: []string{"HARNESS_STACK=go"}},
		{name: "a BASE_REF that is the default one", change: func(*testing.T, string) {}, env: []string{"BASE_REF=main"}, same: true},
		{name: "another BASE_REF", change: func(t *testing.T, root string) {
			gitT(t, root, "commit", "-q", "--allow-empty", "--no-verify", "-m", "more")
		}, env: []string{"BASE_REF=HEAD"}},
		{name: "a commit", change: func(t *testing.T, root string) {
			gitT(t, root, "commit", "-q", "--allow-empty", "--no-verify", "-m", "more")
		}, same: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := stampRepo(t, map[string][]string{"a.go": {"a"}})
			writeLines(t, root, ".gitignore", ".bin/", ".cache/", ".claude/", ".env.*", "*.log", "node_modules/")
			for _, f := range []string{".bin/tool", ".env.local", "node_modules/x.js"} {
				writeLines(t, root, f, "x=1")
			}
			before := keyOf(t, root)
			tt.change(t, root)
			if after := keyOf(t, root, tt.env...); (after == before) != tt.same {
				t.Errorf("key %s, then %s; want the same: %v", before, after, tt.same)
			}
		})
	}
}

// TestDiskHashLimit: the end of the context ends the read of a file, not only git.
func TestDiskHashLimit(t *testing.T) {
	t.Parallel()
	root := stampRepo(t, map[string][]string{"a.go": {"a"}})
	writeLines(t, root, "zz", "")                                         // the last file: no other one's turn ends the hashing
	if err := os.Truncate(filepath.Join(root, "zz"), 1<<36); err != nil { // sparse: it takes no space
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	w := testWorld(t, io.Discard)
	w.ctx = ctx
	if _, err := w.diskHash(root); !errors.Is(err, os.ErrClosed) {
		t.Errorf("diskHash read on after its context ended: %v", err)
	}
}

// TestHashFiles: a file that is gone is skipped; one that cannot be read gives no hash.
func TestHashFiles(t *testing.T) {
	t.Parallel()
	root, w := t.TempDir(), testWorld(t, io.Discard)
	writeLines(t, root, "a", "x")
	for _, want := range []error{nil, fs.ErrPermission} {
		_, err := w.hashFiles(root, []string{"a", "gone"}, func(string, fs.FileInfo) (string, error) { return "", want })
		if !errors.Is(err, want) {
			t.Errorf("error %v, want %v", err, want)
		}
	}
}

// TestTreeKeyCommit: committing a changed tree leaves its key as it is, staged or not.
func TestTreeKeyCommit(t *testing.T) {
	t.Parallel()
	root := stampRepo(t, map[string][]string{"a.go": {"a"}})
	writeLines(t, root, "a.go", "b")
	writeLines(t, root, "new.go", "x")
	want := keyOf(t, root)
	gitT(t, root, "add", "new.go")
	if got := keyOf(t, root); got != want {
		t.Errorf("staged: %s, want %s", got, want)
	}
	commitAll(t, root, "more")
	if got := keyOf(t, root); got != want {
		t.Errorf("committed: %s, want %s", got, want)
	}
}

// TestTreeKeySameSecond: an edit of the same size in the second of the commit changes the key, also when the key
// is taken a second later (copyIndex keeps the index's time).
func TestTreeKeySameSecond(t *testing.T) {
	t.Parallel()
	root := stampRepo(t, map[string][]string{"a.go": {"a"}})
	before := keyOf(t, root).tree // the base's is compared alone (isBase)
	info, err := os.Stat(filepath.Join(root, ".git", "index"))
	if err != nil {
		t.Fatal(err)
	}
	writeLines(t, root, "a.go", "b")
	for _, f := range []string{"a.go", ".git/index"} { // the same second; a slow machine shows in ctime, which git reads too
		if err := os.Chtimes(filepath.Join(root, f), info.ModTime(), info.ModTime().Truncate(time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(time.Until(info.ModTime().Truncate(time.Second).Add(1100 * time.Millisecond)))
	if after := keyOf(t, root).tree; after == before {
		t.Errorf("tree %s did not change", after)
	}
}

// TestTreeKeyWritesNothing: the repository has the same objects, index and files in .git after a key is taken, also
// with a split index and a hook on index changes.
func TestTreeKeyWritesNothing(t *testing.T) {
	t.Parallel()
	root := stampRepo(t, map[string][]string{"a.go": {"a"}})
	gitT(t, root, "config", "core.splitIndex", "true")
	if err := os.Symlink("/usr/bin/touch", filepath.Join(root, ".git/hooks/post-index-change")); err != nil {
		t.Fatal(err)
	}
	writeLines(t, root, "new.go", "x")
	state := func() string {
		_, objects := execIn(t, root, nil, "git", "count-objects", "-v")
		_, index := execIn(t, root, nil, "git", "ls-files", "-s")
		_, files := execIn(t, root, nil, "find", ".")
		return objects + index + files
	}
	before := state()
	keyOf(t, root)
	if after := state(); after != before {
		t.Errorf("before:\n%s\nafter:\n%s", before, after)
	}
}

// TestTreeKeyNoBase: without a main branch the base is none; without git there is no key.
func TestTreeKeyNoBase(t *testing.T) {
	t.Parallel()
	root := templateRepo(t, nil)
	if k := keyOf(t, root); k.base != noBase || k.tree == "" {
		t.Errorf("key %s, want base=none and a tree", k)
	}
	if k, err := testWorld(t, &bytes.Buffer{}, "PATH=/nonexistent").treeKey(root); err == nil {
		t.Errorf("key %s without git, want an error", k)
	}
}

// makeSandbox is a repo with the real Makefile and the code under test as .bin/harness, and what it takes to
// read the layout; `make check MAKE=true` runs check with every sensor green, MAKE=false with every one red.
func makeSandbox(t *testing.T) string {
	t.Helper()
	dir := sandbox(t, "Makefile", "scripts/tool-versions.env", "scripts/harness/merge-base.sh")
	old := time.Unix(0, 0) // older than .bin/harness, so make does not rebuild it
	if err := os.Chtimes(filepath.Join(dir, "scripts/tool-versions.env"), old, old); err != nil {
		t.Fatal(err)
	}
	linkHarness(t, dir)
	writeLines(t, dir, ".gitignore", ".bin/", ".cache/")
	commitAll(t, dir, "start")
	return dir
}

// TestMakeCheckStamps runs the real check recipe: which runs stamp the tree.
func TestMakeCheckStamps(t *testing.T) {
	t.Parallel()
	const changed = "this tree is not stamped green: it changed while make check ran"
	tests := []struct {
		name    string
		args    []string
		env     []string
		code    int    // -1: any
		stamped bool   // as the gate, without args and env, sees the tree
		stack   string // the stack it is stamped for
		output  string
	}{
		{name: "green", args: []string{"MAKE=true"}, stamped: true},
		{name: "green with -k", args: []string{"-k", "MAKE=true"}, stamped: true},
		{name: "green with a variable that holds an i", args: []string{"MAKE=true", "VAR=is"}, stamped: true},
		{name: "green with the base ref given", args: []string{"MAKE=true", "BASE_REF=main"}, stamped: true},
		{name: "red", args: []string{"MAKE=false"}, code: 2},
		{name: "red under -k", args: []string{"-k", "MAKE=false"}, code: 2},
		{name: "red under -i, which ends green", args: []string{"-i", "MAKE=false"}},
		{name: "red under -k -i with a variable", args: []string{"-k", "-i", "VAR=x", "MAKE=false"}},
		{name: "red under -k -i -j, which make splits", args: []string{"-ki", "-j4", "MAKE=false"}},
		{name: "red under --ignore-errors", args: []string{"--ignore-errors", "MAKE=false"}},
		{name: "red under MAKEFLAGS=i", args: []string{"MAKE=false"}, env: []string{"MAKEFLAGS=i"}},
		{name: "dry run", args: []string{"-n", "MAKE=true"}},
		{name: "touch, which make 3.81 fails and make 4 does not", args: []string{"-t", "MAKE=true"}, code: -1},
		{name: "another stack on the command line", args: []string{"MAKE=true", "HARNESS_STACK=go"}, stack: "go"},
		{name: "another stack in the environment", args: []string{"MAKE=true"}, env: []string{"HARNESS_STACK=go"}, stack: "go"},
		{name: "green with a tool built meanwhile", args: []string{"MAKE=touch $(HARNESS).x #"}, stamped: true},
		{name: "the tree changes meanwhile", args: []string{"MAKE=echo x >> changed.txt #"}, output: changed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := makeSandbox(t)
			env := append([]string{"MAKEFLAGS=", "MAKELEVEL="}, tt.env...)
			code, out := execIn(t, dir, env, "make", append([]string{"check"}, tt.args...)...)
			if (tt.code >= 0 && code != tt.code) || !strings.Contains(out, tt.output) {
				t.Errorf("exit %d, want %d; output, want %q in it:\n%s", code, tt.code, tt.output, out)
			}
			if got := keyOf(t, dir).stamped(dir, greenStamp); got != tt.stamped {
				t.Errorf("stamped is %v, want %v; output:\n%s", got, tt.stamped, out)
			}
			if tt.stack != "" && !keyOf(t, dir, "HARNESS_STACK="+tt.stack).stamped(dir, greenStamp) {
				t.Errorf("not stamped for stack %s; output:\n%s", tt.stack, out)
			}
		})
	}
}

// TestCheckStamp: the stamp never turns a green check red, and a wrong call is a usage error.
func TestCheckStamp(t *testing.T) {
	t.Parallel()
	root := stampRepo(t, map[string][]string{"a.go": {"a"}})
	key := keyOf(t, root).source()
	tests := []struct {
		name, stderr string
		args, env    []string
		code         int
	}{
		{name: "no key", args: []string{root, ""}},
		{name: "another tree's key", args: []string{root, "tree=x"}, stderr: "it changed while make check ran\n"},
		{name: "no git", args: []string{root, key}, env: []string{"PATH=/nonexistent"}, stderr: "this tree is not stamped green"},
		{name: "no root", code: exitUsage, stderr: "usage: harness "},
		{name: "too many", args: []string{root, key, "x"}, code: exitUsage, stderr: "usage: harness "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			code := runTest(t, append([]string{"check-stamp"}, tt.args...), nil, &stdout, &stderr, tt.env...)
			if code != tt.code || stdout.Len() != 0 || !strings.Contains(stderr.String(), tt.stderr) || (tt.stderr == "") != (stderr.Len() == 0) {
				t.Errorf("exit %d, want %d; stdout %q; stderr %q, want %q in it", code, tt.code, &stdout, &stderr, tt.stderr)
			}
			if keyOf(t, root).stamped(root, greenStamp) {
				t.Error("the tree is stamped")
			}
		})
	}
}
