package main

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// protect-harness blocks agent edits to harness files, the globs in scripts/harness/protected-paths.txt
// (AGENTS.md golden principle 5), for .claude/hooks/protect-harness.sh; the wrapper lets a human through with
// HARNESS_EDIT=1. File tools are matched exactly. Bash is a heuristic: it blocks a protected path that is a
// redirection target or an argument of a file-changing command (sed -i, perl -i, mv, cp, rm, ln, install,
// chmod, touch, truncate, tee, git checkout/restore/rm/mv/apply). Writes from inside an interpreter
// (`python -c`, `node -e`) are not seen; CODEOWNERS and the harness-integrity check catch them in review.
//
// Paths are judged inside every worktree of the repository (git worktree list), not only this checkout: a
// session rooted in one worktree that edits a sibling lane or a nested `.claude/worktrees/<agent>` meets the
// same globs, from this checkout's list (a worktree cannot loosen its own copy). The globs ignore case, as
// macOS's disk does. A path is judged as written (symlinks in its directory resolved) and as the file it leads
// to, so a link cannot carry the write onto a harness file. It errs towards blocking: unparseable input, a
// path it cannot resolve, an unreadable list and a failed worktree listing all block (fail closed).

const harnessFileAdvice = ` is a harness file (AGENTS.md golden principle 5).
Linter configs, sensors, hooks and CI define what "good" means here; changing them to get a green check
hides the problem instead of fixing it. Fix the code the sensor points at. If the rule itself is wrong,
do not edit it: propose the change in the PR description (Harness change: Y, the file, the diff, why).`

var (
	// fileWriters: 1 = every argument is written, 2 = only the last one (the destination).
	fileWriters = map[string]int{"cp": 2, "ln": 2, "install": 2,
		"mv": 1, "rm": 1, "chmod": 1, "touch": 1, "truncate": 1, "tee": 1}
	inPlace    = regexp.MustCompile(`^(-[a-zA-Z]*i|--in-place)`) // sed -i, -Ei, --in-place; perl -i, -pi
	gitWriters = set("checkout", "restore", "rm", "mv", "apply")
	unquote    = strings.NewReplacer(`"`, "", `'`, "", `\`, "") // AGENTS".md", AGENTS\.md
)

type protector struct {
	root, cwd string
	patterns  []string // protected-paths.txt, read on first use
	roots     []string // root and every worktree, physical, listed on first use
}

func protectHarness(root string, stdin io.Reader) error {
	in, err := parseHookInput(stdin)
	if err != nil {
		return blockf("%v", err)
	}
	r, err := filepath.EvalSymlinks(root)
	if err != nil {
		return blockf("repository root: %v", err)
	}
	p := &protector{root: r, cwd: physicalDir(in.Cwd)}
	if in.ToolName != "Bash" {
		target := in.ToolInput.FilePath
		if target == "" {
			target = in.ToolInput.NotebookPath
		}
		return p.check(target, strings.TrimPrefix(target, root+"/"))
	}
	return p.checkCommand(in.ToolInput.Command)
}

// checkCommand looks at each simple command of cmd (split as segments does) and blocks on the first word that
// writes a protected path.
func (p *protector) checkCommand(cmd string) error {
	for _, seg := range segments(cmd) {
		w := make([]string, len(seg))
		for i, f := range seg {
			w[i] = unquote.Replace(f)
		}
		if len(w) == 0 {
			continue
		}
		writer := fileWriters[w[0]]
		switch w[0] {
		case "sed", "perl":
			if slices.ContainsFunc(w[1:], inPlace.MatchString) {
				writer = 1
			}
		case "git":
			if slices.ContainsFunc(w[1:], func(a string) bool { return gitWriters[a] }) {
				writer = 1
			}
		}
		dest := 0 // the last word that is neither an option nor part of a redirect: cp's destination
		for i := 1; i < len(w); i++ {
			if !strings.HasPrefix(w[i], "-") && !strings.Contains(w[i], ">") && !strings.HasSuffix(w[i-1], ">") {
				dest = i
			}
		}
		prev := ""
		for i, a := range w {
			target := ""
			switch j := strings.LastIndexByte(a, '>'); {
			case j >= 0 && j < len(a)-1: // >file, >>file, 2>file
				target = a[j+1:]
			case strings.HasSuffix(prev, ">"),
				i > 0 && !strings.HasPrefix(a, "-") && (writer == 1 || writer == 2 && i == dest):
				target = a
			}
			prev = a
			if err := p.check(target, target); err != nil {
				return err
			}
		}
	}
	return nil
}

// check blocks when path (absolute, or relative to cwd) is a harness file; name is how the message shows it.
func (p *protector) check(path, name string) error {
	if path == "" {
		return nil
	}
	full := osPath(p.cwd, path)
	dir, err := resolveExisting(filepath.Dir(full))
	if err != nil {
		return blockf("%v", err)
	}
	target, err := resolveExisting(full)
	if err != nil {
		return blockf("%v", err)
	}
	for _, f := range []string{filepath.Join(dir, filepath.Base(full)), target} {
		if ok, err := p.protected(f); err != nil || ok {
			if err == nil {
				err = blockf("%s%s", name, harnessFileAdvice)
			}
			return err
		}
	}
	return nil
}

// osPath is p (absolute, or relative to cwd) as the OS walks it: a `..` part goes up from where the symlinks
// before it lead, not lexically (filepath.Clean would drop `link/..` before the link is followed).
func osPath(cwd, p string) string {
	if !filepath.IsAbs(p) {
		p = cwd + "/" + p
	}
	cur := "/"
	for _, c := range strings.Split(p, "/") {
		switch c {
		case "", ".":
		case "..":
			if r, err := filepath.EvalSymlinks(cur); err == nil {
				cur = r
			}
			cur = filepath.Dir(cur)
		default:
			cur = filepath.Join(cur, c)
		}
	}
	return cur
}

// protected reports whether file (physical) is inside a worktree of the repo and, relative to any worktree that
// holds it, matches a protected glob. Every holder counts, so registering a fake worktree inside this one hides
// nothing; only an agent worktree nested at .claude/worktrees/<name> is judged as itself rather than as files
// under .claude/. The nearest .git above the file decides whether git is asked: none (this checkout still
// counts with its .git moved away) or this checkout's own means only this checkout, without a git call
// (NFR-H1); anything else asks for every worktree.
func (p *protector) protected(file string) (bool, error) {
	d := filepath.Dir(file)
	for {
		if _, err := os.Lstat(filepath.Join(d, ".git")); err == nil {
			break
		}
		if parent := filepath.Dir(d); parent != d {
			d = parent
			continue
		}
		d = ""
		break
	}
	if d == "" || strings.EqualFold(d, p.root) {
		rel, ok := under(p.root, file)
		if !ok {
			return false, nil
		}
		return p.globbed(rel)
	}
	roots, err := p.worktrees()
	if err != nil {
		return false, err
	}
	for _, r := range roots {
		rel, ok := under(r, file)
		if !ok {
			continue
		}
		if n, ok := strings.CutPrefix(rel, ".claude/worktrees/"); ok {
			if name, _, nested := strings.Cut(n, "/"); nested && slices.Contains(roots, r+"/.claude/worktrees/"+name) {
				continue
			}
		}
		if ok, err := p.globbed(rel); err != nil || ok {
			return ok, err
		}
	}
	return false, nil
}

// under returns file relative to dir when file is inside it, ignoring case as macOS's disk does.
func under(dir, file string) (string, bool) {
	if len(file) <= len(dir)+1 || file[len(dir)] != '/' || !strings.EqualFold(file[:len(dir)], dir) {
		return "", false
	}
	return file[len(dir)+1:], true
}

// globbed reports whether rel matches a glob of this checkout's protected-paths.txt.
func (p *protector) globbed(rel string) (bool, error) {
	if p.patterns == nil {
		file := filepath.Join(p.root, "scripts/harness/protected-paths.txt")
		entries, err := protectedPaths(file)
		if err != nil {
			return false, blockf("cannot read the protected paths in %s (%v)", file, err)
		}
		if len(entries) == 0 {
			return false, blockf("cannot read the protected paths in %s (it lists none)", file)
		}
		for _, e := range entries {
			p.patterns = append(p.patterns, e.pattern)
		}
	}
	return matchesAnyFold(p.patterns, rel), nil
}

// worktrees returns root and the physical paths of every worktree of its repository, asked of git once.
// It fails closed (NFR-R2): without the list a sibling worktree would be unguarded.
func (p *protector) worktrees() ([]string, error) {
	if p.roots != nil {
		return p.roots, nil
	}
	cmd := exec.Command("git", "-C", p.root, "worktree", "list", "--porcelain", "-z")
	// The repository is root's, whatever a caller's environment points git at.
	cmd.Env = slices.DeleteFunc(os.Environ(), func(kv string) bool {
		k, _, _ := strings.Cut(kv, "=")
		return k == "GIT_DIR" || k == "GIT_WORK_TREE" || k == "GIT_COMMON_DIR"
	})
	out, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && len(exit.Stderr) > 0 {
			err = errors.New(strings.TrimSpace(string(exit.Stderr))) // -z needs git 2.36
		}
		return nil, blockf("cannot list the worktrees of %s (git worktree list: %v)", p.root, err)
	}
	p.roots = []string{p.root}
	for _, f := range strings.Split(string(out), "\x00") {
		if w, ok := strings.CutPrefix(f, "worktree "); ok {
			if r, err := filepath.EvalSymlinks(w); err == nil {
				p.roots = append(p.roots, r)
			}
		}
	}
	return p.roots, nil
}
