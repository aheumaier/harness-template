package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// world is everything a subcommand reads from outside its arguments (ADR 0045): run builds it from the process,
// a test builds its own, so subcommands run in-process and in parallel. Nothing else reads the environment or the
// working directory. A <root> argument is resolved with abs too.
//
// The environment is two fields because a testscript world can look a variable up but not list them: run and
// testWorld build lookupEnv from environ (lookupIn).
type world struct {
	stdin          io.Reader
	stdout, stderr io.Writer
	lookupEnv      func(key string) (string, bool) // the environment the subcommand reads
	environ        []string                        // the environment of the processes it runs (git)
	cwd            string                          // where relative paths start
}

func (w world) getenv(key string) string {
	v, _ := w.lookupEnv(key)
	return v
}

// lookupIn is os.LookupEnv on environ: the last entry of a key wins, as for exec.Cmd.
func lookupIn(environ []string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		for _, kv := range slices.Backward(environ) {
			if v, ok := strings.CutPrefix(kv, key+"="); ok {
				return v, true
			}
		}
		return "", false
	}
}

// harnessRoot is the tree a git-reading sensor checks, made absolute: HARNESS_ROOT when set (CI runs the base
// branch's binary on the PR head, harness-integrity.yml), else root. HARNESS_ROOT is the change under review:
// a sensor reads its own config (layout.env, protected-paths.txt) from root, never from harnessRoot, or the
// change would rewrite the rules that gate it (integrity-check's own and root).
func (w world) harnessRoot(root string) (string, error) {
	if r := w.getenv("HARNESS_ROOT"); r != "" {
		return w.abs(r)
	}
	return w.abs(root)
}

// abs is p joined to the working directory when it is relative; an error when there is none (run could not
// read it).
func (w world) abs(p string) (string, error) {
	switch {
	case filepath.IsAbs(p):
		return p, nil
	case w.cwd == "":
		return "", fmt.Errorf("%s: no working directory to resolve the relative path against", p)
	}
	return filepath.Join(w.cwd, p), nil
}

// command is exec.Command for the world: name is looked up in its PATH, not the process's, and the process gets
// its environment and starts in its working directory. A name that is not found fails when the command starts,
// as with exec.Command.
func (w world) command(name string, args ...string) *exec.Cmd {
	cmd := &exec.Cmd{Path: name, Args: append([]string{name}, args...), Env: w.environ, Dir: w.cwd}
	if !strings.Contains(name, string(filepath.Separator)) {
		cmd.Path, cmd.Err = lookPath(name, w.environ)
	}
	return cmd
}

// lookPath finds an executable name in environ's PATH (its last PATH=, the one exec.Cmd passes on), with
// exec.LookPath's test of each candidate (one this user may run). Relative PATH entries are skipped, as
// exec.LookPath refuses what they find (exec.ErrDot).
func lookPath(name string, environ []string) (string, error) {
	dirs, _ := lookupIn(environ)("PATH")
	for _, dir := range filepath.SplitList(dirs) {
		if filepath.IsAbs(dir) {
			if p, err := exec.LookPath(filepath.Join(dir, name)); err == nil {
				return p, nil
			}
		}
	}
	return "", &exec.Error{Name: name, Err: exec.ErrNotFound}
}

// git runs git in root and returns its stdout; git's stderr goes to the world's.
func (w world) git(root string, args ...string) ([]byte, error) {
	cmd := w.command("git", append([]string{"-C", root, "-c", "core.quotePath=false"}, args...)...)
	cmd.Stderr = w.stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w", args[0], err)
	}
	return out, nil
}

// gitFiles is gitKnown without skippedPath (paths under fixtures/ or node_modules/).
func (w world) gitFiles(root string, pathspecs ...string) ([]string, error) {
	known, err := w.gitKnown(root, pathspecs...)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, f := range known {
		if !skippedPath.MatchString(f) {
			files = append(files, f)
		}
	}
	return files, nil
}

// gitKnown lists tracked and untracked, not ignored files matching the pathspecs, without tracked files
// deleted from the working tree (`rm docs/x.md` leaves them in ls-files -c): their links are gone, and links
// to them are reported where they are.
func (w world) gitKnown(root string, pathspecs ...string) ([]string, error) {
	return w.lsFiles(root, append([]string{"-co", "--exclude-standard", "--"}, pathspecs...)...)
}

// lsFiles runs git ls-files with args and keeps the files that still exist in the working tree.
func (w world) lsFiles(root string, args ...string) ([]string, error) {
	out, err := w.git(root, append([]string{"ls-files", "-z"}, args...)...)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, f := range strings.Split(string(out), "\x00") {
		if f == "" {
			continue
		}
		if _, err := os.Lstat(filepath.Join(root, f)); err == nil {
			files = append(files, f)
		}
	}
	return files, nil
}

// mergeBase is scripts/harness/merge-base.sh in root: the merge base of HEAD with ref (default: origin/main
// if it exists, else main). A sensor that cannot find its base would compare nothing and pass.
func (w world) mergeBase(root, ref string) (string, error) {
	verify := func(r string) bool {
		return w.command("git", "-C", root, "rev-parse", "-q", "--verify", r+"^{commit}").Run() == nil
	}
	if _, err := lookPath("git", w.environ); err != nil { // else every ref below would read as missing
		return "", err
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
	out, err := w.command("git", "-C", root, "merge-base", "HEAD", ref).Output()
	if err != nil {
		return "", fmt.Errorf("HEAD and BASE_REF '%s' have no merge base: fetch full history (git fetch --unshallow; CI: actions/checkout with fetch-depth: 0).", ref)
	}
	return strings.TrimSpace(string(out)), nil
}
