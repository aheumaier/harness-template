package main

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"go/build/constraint"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// specs-tests-frozen (rules.md#specs-tests-frozen, ADR 0047): the tests of a `Test-Writer: specs/<NNN-slug>` commit
// stay as they are for the rest of the branch, unless the spec changes after it or a later test commit changes them.
//
// Each commit of the branch is compared with its own parent, and git blame says which commit a line it removed came
// from. Comparing a test commit with HEAD instead would also see what the base and later test commits did, and would
// pair up the lines that look alike in every test differently each time.

const frozenRule = "specs-tests-frozen"

// testWriterRE is the line that marks a test commit, wherever it stands in the message: git reads trailers in the
// last paragraph only.
var testWriterRE = regexp.MustCompile(`(?im)^test-writer:(.*)$`)

// branch is what HEAD holds and BASE_REF does not.
type branch struct {
	w         world
	root, ref string
	commits   []branchCommit // a commit comes after its parents
	index     map[string]int // a commit's place in commits
	files     []string       // the test files of all test commits
}

// branchCommit is one commit of the branch; test says the test writer's work went into it.
type branchCommit struct {
	sha     string
	parents []string
	test    bool
	spec    string   // what follows `Test-Writer:`
	files   []string // the product test files a test commit added or changed
}

// origin is a line of a file with the commit it came from and its number there.
type origin struct {
	sha  string
	line int
	text string
}

// removal is a test commit's line that another commit of the branch took away.
type removal struct {
	test, by int // places in branch.commits
	path     string
	origin
}

// fileLine is a text in a file.
type fileLine struct{ path, text string }

func init() {
	commands.register(rootSensor("specs-frozen", "specs-tests-frozen (make specs-frozen; env BASE_REF, HARNESS_ROOT)",
		func(w world, own string) (bool, error) {
			root, err := w.harnessRoot(own)
			if err != nil {
				return false, err
			}
			return specsFrozen(w, own, root, w.getenv("BASE_REF"))
		}))
}

// specsFrozen prints one line per finding to stdout and reports whether there was any. own holds the layout and the
// protected paths, root the commits.
func specsFrozen(w world, own, root, baseRef string) (bool, error) {
	pt, err := loadProductTests(w, own)
	if err != nil {
		return false, err
	}
	// `[id].test.ts` is one file and not a pattern; local replacement commits and grafts are not what CI reads.
	w.environ = append(slices.Clip(w.environ), "GIT_LITERAL_PATHSPECS=1", "GIT_GLOB_PATHSPECS=0", "GIT_NOGLOB_PATHSPECS=0",
		"GIT_ICASE_PATHSPECS=0", "GIT_NO_REPLACE_OBJECTS=1", "GIT_GRAFT_FILE=")
	if _, err := w.mergeBase(root, baseRef); err != nil {
		return false, err
	}
	b := branch{w: w, root: root, index: map[string]int{}}
	if b.ref, err = w.baseRef(root, baseRef); err != nil {
		return false, err
	}
	switch prefix, err := w.git(root, "rev-parse", "--show-prefix"); {
	case err != nil:
		return false, err
	case len(prefix) > 1: // paths in commits start at the top: nothing would match
		return false, errors.New("HARNESS_ROOT is not the top of its repository")
	}
	if err := b.read(pt); err != nil || len(b.files) == 0 {
		return false, err
	}
	head, err := b.head()
	if err != nil {
		return false, err
	}
	removed, err := b.removed(head)
	if err != nil {
		return false, err
	}
	bad := false
	for i, t := range b.commits {
		found, err := b.findings(t, head, slices.DeleteFunc(slices.Clone(removed), func(r removal) bool { return r.test != i }))
		if err != nil {
			return false, err
		}
		if len(found) == 0 {
			continue
		}
		lifted, hint, err := b.lifted(t)
		if err != nil {
			return false, err
		}
		if lifted != "" {
			fmt.Fprintf(w.stdout, "notice: commit %.7s changed %s after test commit %.7s: %d changes of its tests are not reported [%s]\n",
				lifted, specFile(t), t.sha, len(found), frozenRule)
			continue
		}
		bad = true
		for _, f := range found {
			printFinding(w.stdout, f.path, f.line, frozenRule, "%s%s", f.text, hint)
		}
	}
	return bad, nil
}

// read lists the branch's commits. A commit that has the `Test-Writer:` line and changes more than tests and specs
// (tests squashed into the implementation) is no test commit; a notice says so.
func (b *branch) read(pt productTests) error {
	log, err := b.w.git(b.root, "log", "--topo-order", "--reverse", "--no-show-signature", "--encoding=UTF-8", "-z",
		"--format=%H %P%n%B", "HEAD", "^"+b.ref, "--")
	if err != nil {
		return err
	}
	shallow, err := b.w.git(b.root, "rev-parse", "--is-shallow-repository")
	if err != nil {
		return err
	}
	for _, entry := range strings.Split(strings.TrimSuffix(string(log), "\x00"), "\x00") {
		head, msg, _ := strings.Cut(entry, "\n")
		ids := strings.Fields(head)
		if len(ids) == 0 {
			continue
		}
		c := branchCommit{sha: ids[0], parents: ids[1:]}
		if len(c.parents) == 0 && strings.TrimSpace(string(shallow)) == "true" {
			return fmt.Errorf("the clone is cut off at commit %.7s of the branch: fetch full history (git fetch --unshallow)", c.sha)
		}
		if m := testWriterRE.FindStringSubmatch(msg); m != nil && len(c.parents) < 2 {
			c.spec = strings.TrimSpace(m[1])
			if err := b.readTest(&c, pt); err != nil {
				return err
			}
		}
		b.index[c.sha] = len(b.commits)
		b.commits = append(b.commits, c)
		b.files = append(b.files, c.files...)
	}
	slices.Sort(b.files)
	b.files = slices.Compact(b.files)
	return nil
}

// readTest sets c's test and files when c changes only tests and specs.
func (b *branch) readTest(c *branchCommit, pt productTests) error {
	out, err := b.w.git(b.root, "diff-tree", "--root", "--no-commit-id", "-r", "-z", "--no-renames", "--name-status", c.sha)
	if err != nil {
		return err
	}
	var files []string
	fields := strings.Split(string(out), "\x00") // status, path, status, path, …
	for i := 0; i+1 < len(fields); i += 2 {
		status, path := fields[i], fields[i+1]
		switch {
		case pt.why(path) == "":
			if status != "D" {
				files = append(files, path)
			}
		case !strings.HasPrefix(path, "specs/"):
			fmt.Fprintf(b.w.stdout, "notice: commit %.7s has a Test-Writer: line but changes %q, which is neither a test nor a spec: its tests are not frozen [%s]\n",
				c.sha, path, frozenRule)
			return nil
		}
	}
	c.test, c.files = true, files
	return nil
}

// head is the lines of the test files HEAD has.
func (b *branch) head() (map[string][]origin, error) {
	out, err := b.w.git(b.root, append([]string{"ls-tree", "-r", "-z", "--name-only", "HEAD", "--"}, b.files...)...)
	if err != nil {
		return nil, err
	}
	head, present := map[string][]origin{}, strings.Split(string(out), "\x00")
	for _, path := range b.files {
		if !slices.Contains(present, path) {
			continue
		}
		if head[path], err = b.blame("HEAD", path); err != nil {
			return nil, err
		}
	}
	return head, nil
}

// blame is the lines of a file at rev. What the base wrote comes from a commit that is not in b.index. A rename is
// not followed, as in the diffs: a renamed file is a deleted and an added one.
func (b *branch) blame(rev, path string) ([]origin, error) {
	out, err := b.w.git(b.root, "blame", "--minimal", "--porcelain", "--no-follow", "--no-textconv", "--ignore-revs-file=", rev, "^"+b.ref, "--", path)
	if err != nil {
		return nil, err
	}
	var lines []origin
	var o origin
	start := true // of a line's group: `<sha> <number there> …`, then its commit, then the line after a tab
	for _, l := range splitLines(out) {
		if text, last := strings.CutPrefix(l, "\t"); last {
			o.text = text
			lines = append(lines, o)
			start = true
		} else if start {
			f := strings.Fields(l)
			o.sha = f[0]
			if o.line, err = strconv.Atoi(f[1]); err != nil {
				return nil, fmt.Errorf("git blame %s: %w", path, err)
			}
			start = false
		}
	}
	return lines, nil
}

// removed is the lines of test commits that other commits of the branch took away and no commit but a test commit
// brought back, as a move or a later restore does: the same text in the same file. A test commit's line is its own.
// Blank lines check nothing. head is b.head.
func (b *branch) removed(head map[string][]origin) ([]removal, error) {
	var gone []removal
	seen := map[removal]bool{}   // without by: two sides of a merge can take the same line
	back := map[fileLine][]int{} // the commits that added a line which stayed, or which only a test commit removed
	add := func(path string, o origin) {
		if i, ok := b.index[o.sha]; ok && !b.commits[i].test {
			back[fileLine{path, o.text}] = append(back[fileLine{path, o.text}], i)
		}
	}
	for path, lines := range head {
		for _, o := range lines {
			add(path, o)
		}
	}
	for by, c := range b.commits {
		for _, parent := range c.parents {
			if _, ok := b.index[parent]; !ok {
				continue // the base wrote all of it
			}
			lost, err := b.lost(parent, c.sha)
			if err != nil {
				return nil, err
			}
			for _, r := range lost {
				if c.test {
					add(r.path, r.origin)
					continue
				}
				i, ok := b.index[r.sha]
				// a merge may have kept the line a side of it took
				if !ok || !b.commits[i].test || seen[r] || slices.Contains(head[r.path], r.origin) || b.elsewhere(r.sha, c.parents, parent) {
					continue
				}
				seen[r] = true
				r.test, r.by = i, by
				gone = append(gone, r)
			}
		}
	}
	for _, commits := range back {
		slices.Sort(commits)
	}
	return slices.DeleteFunc(gone, func(r removal) bool {
		at := fileLine{r.path, r.text}
		i := slices.IndexFunc(back[at], func(c int) bool { return c >= r.by })
		if i < 0 {
			return false
		}
		back[at] = slices.Delete(back[at], i, i+1) // it brings back one line
		return true
	}), nil
}

// lost is the lines of the test files that a commit's parent has and the commit does not.
func (b *branch) lost(parent, sha string) ([]removal, error) {
	// minimal as in blame: of lines that look alike, both must call the same ones kept
	diff, err := diffLines(b.w, b.root, []string{"--diff-algorithm=minimal", parent, sha}, b.files)
	if err != nil {
		return nil, err
	}
	var lost []removal
	lines := map[string][]origin{}
	for _, c := range diff {
		if c.added || strings.TrimSpace(c.text) == "" {
			continue
		}
		if lines[c.path] == nil {
			if lines[c.path], err = b.blame(parent, c.path); err != nil {
				return nil, err
			}
		}
		if c.line > len(lines[c.path]) || lines[c.path][c.line-1].text != c.text {
			return nil, fmt.Errorf("git diff and git blame disagree on %s at %s", c.path, parent)
		}
		lost = append(lost, removal{path: c.path, origin: lines[c.path][c.line-1]})
	}
	return lost, nil
}

// elsewhere reports whether a merge has the commit sha from another parent too: that side may have removed the line,
// and its own commit answers for it.
func (b *branch) elsewhere(sha string, parents []string, parent string) bool {
	return slices.ContainsFunc(parents, func(p string) bool { return p != parent && b.ancestor(sha, p) })
}

func (b *branch) ancestor(a, of string) bool {
	return b.w.command("git", "-C", b.root, "merge-base", "--is-ancestor", a, of).Run() == nil
}

// findings is what the branch did to t's files after it: the ones it deleted, t's lines it took away (removed) and the
// skips and build constraints it added. head is b.head.
func (b *branch) findings(t branchCommit, head map[string][]origin, removed []removal) ([]change, error) {
	var found []change
	for _, path := range t.files {
		if _, ok := head[path]; ok {
			continue
		}
		by, err := b.newest(t, path, "--no-renames", "--diff-filter=D")
		if err != nil {
			return nil, err
		}
		// none when the base deleted it
		if i, ok := b.index[by]; ok && !b.commits[i].test {
			found = append(found, change{path: path, line: 1,
				text: fmt.Sprintf("commit %.7s deleted this test file of commit %.7s", b.commits[i].sha, t.sha)})
		}
	}
	slices.SortStableFunc(removed, func(x, y removal) int {
		return cmp.Or(strings.Compare(x.path, y.path), x.line-y.line)
	})
	for _, r := range removed {
		if _, ok := head[r.path]; ok {
			found = append(found, change{path: r.path, line: r.line,
				text: fmt.Sprintf("commit %.7s changed or removed this line of test commit %.7s: %q", b.commits[r.by].sha, t.sha, strings.TrimSpace(r.text))})
		}
	}
	for _, path := range t.files {
		for n, o := range head[path] {
			if !skipsTest(path, []byte(o.text)) && !constrainsBuild(path, o.text) {
				continue
			}
			if _, ok := b.index[o.sha]; !ok || o.sha == t.sha || b.ancestor(o.sha, t.sha) {
				continue
			}
			found = append(found, change{path: path, line: n + 1,
				text: fmt.Sprintf("commit %.7s added this line after test commit %.7s; it skips its tests: %q", o.sha, t.sha, strings.TrimSpace(o.text))})
		}
	}
	if len(found) == 0 {
		return nil, nil
	}
	out, err := b.w.git(b.root, append([]string{"diff-tree", "-r", "-z", "--no-renames", "--name-only", t.sha, "HEAD", "--"}, t.files...)...)
	changed := strings.Split(string(out), "\x00")
	// a file HEAD has as t left it was restored, by whatever commit
	return slices.DeleteFunc(found, func(c change) bool { return !slices.Contains(changed, c.path) }), err
}

// newest is the newest commit of the branch after t that touched path, if any.
func (b *branch) newest(t branchCommit, path string, flags ...string) (string, error) {
	// --full-history: a merge that kept the file would hide the side that deleted it
	args := append([]string{"log", "-1", "--no-merges", "--topo-order", "--full-history", "--no-show-signature", "--format=%H"}, flags...)
	out, err := b.w.git(b.root, append(args, t.sha+"..HEAD", "^"+b.ref, "--", path)...)
	return strings.TrimSpace(string(out)), err
}

// specFile is the spec t's line names.
func specFile(t branchCommit) string { return t.spec + "/spec.md" }

// lifted is the last commit of the branch that changed t's spec after t, if HEAD's spec is another than t's. hint is
// what to add to a finding when t's line names no spec that t holds, so that no spec change can lift it.
func (b *branch) lifted(t branchCommit) (by, hint string, err error) {
	spec := specFile(t)
	var held []byte
	if specDirRE.MatchString(spec) {
		if held, err = b.w.git(b.root, "ls-tree", "--object-only", t.sha, "--", spec); err != nil {
			return "", "", err
		}
	}
	if len(held) == 0 {
		return "", fmt.Sprintf(" (no spec change can lift this: the commit's Test-Writer: line says %q, not the specs/<NNN-slug> of a spec it holds; reword it, or commit the spec before it)", t.spec), nil
	}
	now, err := b.w.git(b.root, "ls-tree", "--object-only", "HEAD", "--", spec)
	if err != nil || bytes.Equal(now, held) {
		return "", "", err
	}
	by, err = b.newest(t, spec)
	return by, "", err
}

func constrainsBuild(path, text string) bool {
	line := strings.TrimSpace(strings.TrimPrefix(text, "\ufeff"))
	return strings.HasSuffix(path, ".go") && (constraint.IsGoBuild(line) || constraint.IsPlusBuild(line))
}
