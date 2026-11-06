package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// protect runs `harness protect-harness root` on a payload and returns the exit code and stderr.
func protect(t *testing.T, root, stdin string) (int, string) {
	t.Helper()
	return runHook(t, "protect-harness", root, stdin)
}

// wantProtect checks that tool on each arg, from cwd, is blocked as a harness file (or allowed).
func wantProtect(t *testing.T, root, cwd, tool string, blocked bool, args ...string) {
	t.Helper()
	for _, a := range args {
		code, out := protect(t, root, payload(t, cwd, tool, a))
		if blocked && (code != 2 || !strings.Contains(out, " is a harness file") ||
			!strings.Contains(out, "propose the change in the PR description")) {
			t.Errorf("%s %q: exit %d, want 2 with the reason and what to do instead; stderr:\n%s", tool, a, code, out)
		}
		if !blocked && code != 0 {
			t.Errorf("%s %q: exit %d, want 0; stderr:\n%s", tool, a, code, out)
		}
	}
}

// TestProtectHarness holds the cases of the former scripts/test/protect-harness.bats, split into rows, against this
// repository and its protected-paths.txt, plus the paths the bash heuristic let through.
func TestProtectHarness(t *testing.T) {
	root := repoRoot(t)
	tmp, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for link, to := range map[string]string{"agents": "AGENTS.md", "scripts": "scripts", "harness": "scripts/harness"} {
		if err := os.Symlink(filepath.Join(root, to), filepath.Join(tmp, link)); err != nil {
			t.Fatal(err)
		}
	}
	tests := []struct {
		name    string
		tool    string
		blocked bool
		args    []string
	}{
		// The blocked paths under apps/ match `*` globs of the live list; the rest are root, scripts/, .claude/ or
		// .github/ paths, so none depends on layout.env. The allowed rows assume this template's list; moving them onto a fixed list in testdata/ is exec plan 0005 slice 2.
		{name: "blocks file tools on harness files", tool: "Edit", blocked: true, args: []string{
			".golangci.yml", "apps/api/.go-arch-lint.yml", "apps/web/eslint.config.js", ".claude/settings.json",
			".claude/hooks/guard-bash.sh", ".github/workflows/ci.yml", ".github/workflows/pr-review.yml",
			".github/actions/setup/action.yml", ".github/CODEOWNERS", ".github/pull_request_template.md",
			"scripts/harness/run.sh", "AGENTS.md", root + "/Makefile", "scripts/tool-versions.env",
			"apps/web/tsconfig.json", "scripts/test/timing.bats"}},
		{name: "Write", tool: "Write", blocked: true, args: []string{"atlas.hcl"}},
		{name: "MultiEdit", tool: "MultiEdit", blocked: true, args: []string{".spectral.yaml"}},
		{name: "NotebookEdit", tool: "NotebookEdit", blocked: true, args: []string{"Makefile"}},
		{name: "allows file tools on product code and docs outside the harness", tool: "Edit", args: []string{
			"apps/api/internal/platform/x.go", "apps/web/src/App.tsx", "api/openapi.yaml", "docs/QUALITY.md",
			"ARCHITECTURE.md", "apps/api/AGENTS.md", tmp + "/elsewhere/AGENTS.md", ".github/workflows/deploy.yml",
			".github/actions/x/action.yml", ""}},
		{name: "blocks shell writes to harness files", tool: "Bash", blocked: true, args: []string{
			"echo x >> .golangci.yml", "echo x >AGENTS.md", "echo x > .squawk.toml", `sed -i "" s/a/b/ Makefile`,
			"cp /tmp/x .claude/settings.json", "rm scripts/harness/run.sh", "make check && tee -a knip.json <x",
			"git checkout -- .golangci.yml", `mv a "AGENTS.md"`, "cp /tmp/x .claude/settings.json 2>/dev/null",
			"cp a Makefile > log", `rm AGENTS\.md`}},
		{name: "allows shell reads of harness files and writes elsewhere", tool: "Bash", args: []string{
			"cat AGENTS.md", "grep -n x .golangci.yml", "sed -n 1,5p Makefile", "echo x > apps/api/x.go",
			"make check", "git diff -- AGENTS.md", "cp AGENTS.md " + tmp + "/x", "sed -e s/i/x/ Makefile 2>&1",
			"perl -ne print Makefile", "echo x >"}},
		// Stricter than the bash version (exec plan 0005, pilot 2).
		{name: "any case: macOS's disk folds it", tool: "Edit", blocked: true, args: []string{
			"agents.md", ".GOLANGCI.yml", strings.ToUpper("scripts/harness/run.sh")}},
		{name: "a symlink or a `..` through one does not carry the write past the list", tool: "Edit", blocked: true,
			args: []string{tmp + "/agents", tmp + "/scripts/harness/run.sh", tmp + "/harness/../../AGENTS.md",
				"nothere/../AGENTS.md"}},
		{name: "in-place flags in a cluster, a redirect first, quotes inside a word, a substitution", tool: "Bash",
			blocked: true, args: []string{"perl -pi -e s/a/b/ Makefile", "sed -Ei s/a/b/ Makefile",
				"sed --in-place s/a/b/ Makefile", ">AGENTS.md echo x", `echo x > AGENTS".md"`, "echo $(rm AGENTS.md)",
				"cp a b " + tmp + "/agents"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wantProtect(t, root, root, tt.tool, tt.blocked, tt.args...)
		})
	}

	t.Run("fails closed on a path it cannot resolve", func(t *testing.T) {
		if err := os.Symlink(tmp+"/nowhere/x", tmp+"/dangling"); err != nil {
			t.Fatal(err)
		}
		if code, out := protect(t, root, payload(t, root, "Edit", tmp+"/dangling")); code != 2 ||
			!strings.Contains(out, "protect-harness: blocked: cannot resolve") {
			t.Errorf("exit %d, want 2 with cannot resolve; stderr:\n%s", code, out)
		}
	})

	t.Run("fails closed on unparseable input", func(t *testing.T) {
		if code, out := protect(t, root, "not json"); code != 2 || !strings.Contains(out, "protect-harness: blocked: cannot parse") {
			t.Errorf("exit %d, want 2 with cannot parse; stderr:\n%s", code, out)
		}
	})
}

// worktreeSandbox returns a committed sandbox repository with this repository's protected-paths.txt and a
// directory for sibling worktrees and other repositories.
func worktreeSandbox(t *testing.T) (repo, tmp string) {
	t.Helper()
	repo = sandbox(t, "scripts/harness/protected-paths.txt")
	commitAll(t, repo, "x")
	tmp, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return repo, tmp
}

func TestProtectHarnessWorktrees(t *testing.T) {
	t.Run("guards every worktree of the repo with this checkout's list: a sibling lane, a nested agent worktree", func(t *testing.T) {
		repo, tmp := worktreeSandbox(t)
		lane, agent, other := tmp+"/lane", repo+"/.claude/worktrees/agent", tmp+"/other"
		gitT(t, repo, "worktree", "add", "-q", "-b", "lane", lane)
		gitT(t, repo, "worktree", "add", "-q", "-b", "agent", agent)
		gitT(t, tmp, "init", "-q", other)
		// A worktree cannot loosen its own copy.
		if err := os.WriteFile(lane+"/scripts/harness/protected-paths.txt", nil, 0o644); err != nil {
			t.Fatal(err)
		}
		wantProtect(t, repo, repo, "Edit", true, lane+"/Makefile", lane+"/.claude/settings.json", agent+"/AGENTS.md")
		wantProtect(t, repo, repo, "Edit", false, lane+"/apps/api/x.go", agent+"/apps/api/x.go", other+"/Makefile")
		wantProtect(t, repo, repo, "Bash", true, "echo x >> "+lane+"/AGENTS.md")
		gitT(t, tmp, "init", "-q", tmp+"/lane2") // shares the lane's prefix, is no worktree
		wantProtect(t, repo, repo, "Edit", false, tmp+"/lane2/Makefile")
		// A session rooted in the nested agent worktree guards the main checkout too.
		wantProtect(t, agent, agent, "Edit", true, repo+"/Makefile")
	})

	t.Run("a worktree registered inside this checkout does not hide its harness files", func(t *testing.T) {
		repo, tmp := worktreeSandbox(t)
		gitT(t, repo, "worktree", "add", "-q", "-b", "fake", tmp+"/fake")
		// Repoint the fake worktree's admin entry at scripts/, so git lists repo/scripts as a worktree.
		for file, text := range map[string]string{repo + "/.git/worktrees/fake/gitdir": repo + "/scripts/.git\n",
			repo + "/scripts/.git": "gitdir: " + repo + "/.git/worktrees/fake\n"} {
			if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if roots, err := (&protector{w: testWorld(t, io.Discard), root: repo}).worktrees(); err != nil || !slices.Contains(roots, repo+"/scripts") {
			t.Fatalf("git worktree list: %v, %v; want it to list %s/scripts", roots, err, repo)
		}
		wantProtect(t, repo, repo, "Edit", true, "scripts/harness/run.sh", repo+"/scripts/harness/protected-paths.txt")
	})

	t.Run("moving this checkout's .git away does not unprotect it", func(t *testing.T) {
		repo, _ := worktreeSandbox(t)
		if err := os.Rename(repo+"/.git", repo+"/.git.x"); err != nil {
			t.Fatal(err)
		}
		wantProtect(t, repo, repo, "Edit", true, "AGENTS.md", ".claude/settings.json")
		wantProtect(t, repo, repo, "Edit", false, "apps/api/x.go")
	})

	t.Run("fails closed when git cannot list the worktrees a path needs", func(t *testing.T) {
		repo, tmp := worktreeSandbox(t)
		gitT(t, repo, "worktree", "add", "-q", "-b", "lane", tmp+"/lane")
		nogit := tmp + "/nogit"
		stub, err := filepath.Abs("testdata/harness-stub.sh")
		if err != nil {
			t.Fatal(err)
		}
		place(t, stub, nogit+"/git")
		// worktrees reads the process environment; passing it in as a value is exec plan 0005 slice 2.
		t.Setenv("PATH", nogit+":"+os.Getenv("PATH"))
		t.Setenv("STUB_EXIT", "1")
		t.Setenv("STUB_OUTPUT", "fatal: stub git")
		if code, out := protect(t, repo, payload(t, repo, "Edit", tmp+"/lane/apps/x.go")); code != 2 ||
			!strings.Contains(out, "cannot list the worktrees") || !strings.Contains(out, "fatal: stub git") {
			t.Errorf("exit %d, want 2 with cannot list the worktrees; stderr:\n%s", code, out)
		}
		// Paths in this checkout or in no repository need no list.
		wantProtect(t, repo, repo, "Edit", false, "apps/x.go", nogit+"/x")
	})

	t.Run("fails closed without the list of protected paths", func(t *testing.T) {
		repo, _ := worktreeSandbox(t)
		if err := os.Remove(repo + "/scripts/harness/protected-paths.txt"); err != nil {
			t.Fatal(err)
		}
		if code, out := protect(t, repo, payload(t, repo, "Edit", "apps/x.go")); code != 2 ||
			!strings.Contains(out, "cannot read the protected paths") {
			t.Errorf("exit %d, want 2 with cannot read the protected paths; stderr:\n%s", code, out)
		}
		wantProtect(t, repo, repo, "Bash", false, "ls") // a command that writes nothing needs no list
	})

	t.Run("fails closed on a relative path without a working directory", func(t *testing.T) {
		repo, _ := worktreeSandbox(t)
		for tool, arg := range map[string]string{"Write": ".claude/settings.json", "Bash": "echo x > .claude/settings.json"} {
			var stderr bytes.Buffer
			w := testWorld(t, io.Discard)
			w.cwd, w.stderr = "", &stderr // os.Getwd failed, and the payload names no cwd
			w.stdin = strings.NewReader(payload(t, "", tool, arg))
			if code := commands.dispatch(w, []string{"protect-harness", repo}); code != 2 ||
				!strings.Contains(stderr.String(), "no working directory") {
				t.Errorf("%s %q: exit %d, want 2 with no working directory; stderr:\n%s", tool, arg, code, stderr.String())
			}
		}
	})

	t.Run("a relative payload cwd counts as no working directory", func(t *testing.T) {
		repo, _ := worktreeSandbox(t)
		for cwd, file := range map[string]string{".": ".claude/settings.json", "docs": "../.claude/settings.json"} {
			var stderr bytes.Buffer
			w := testWorld(t, io.Discard)
			w.cwd, w.stderr = repo, &stderr // falling back to it would read docs/../ from the repo's parent
			w.stdin = strings.NewReader(payload(t, cwd, "Write", file))
			if code := commands.dispatch(w, []string{"protect-harness", repo}); code != 2 ||
				!strings.Contains(stderr.String(), "no working directory") {
				t.Errorf("Write %s from payload cwd %q: exit %d, stderr %q; want 2", file, cwd, code, stderr.String())
			}
		}
	})
}

// TestProtectHarnessHook runs .claude/hooks/protect-harness.sh: HARNESS_EDIT=1 lets a human through, and a
// binary older than the hook blocks with "rebuild" but never the commands that build it (exec plan 0005, A7).
func TestProtectHarnessHook(t *testing.T) {
	const hook = ".claude/hooks/protect-harness.sh"
	files := []string{hook, "scripts/harness/hook-bootstrap.sh", "scripts/harness/timing.sh",
		"scripts/harness/protected-paths.txt"}
	stub, err := filepath.Abs("testdata/harness-stub.sh")
	if err != nil {
		t.Fatal(err)
	}
	type call struct {
		tool, arg string
		code      int
		output    string // substring; "" means no output
	}
	builds := []call{{"Bash", "make setup", 0, ""}, {"Bash", "make .bin/harness", 0, ""}}
	rebuild := func(code int) []call {
		return slices.Concat(builds, []call{{"Bash", "make setup; rm AGENTS.md", 2, "rebuild it with `make .bin/harness`"},
			{"Edit", "apps/x.go", 2, fmt.Sprintf("out of date or failed (exit %d)", code)}})
	}
	tests := []struct {
		name  string
		stub  bool     // testdata/harness-stub.sh as .bin/harness; else the code under test
		raw   string   // a payload sent as written, for every call
		env   []string // for the stub, or HARNESS_EDIT
		calls []call
	}{
		{name: "the code under test", calls: slices.Concat(builds, []call{{"Edit", "AGENTS.md", 2, "AGENTS.md is a harness file"},
			{"Edit", "apps/x.go", 0, ""}, {"Bash", "make setup; rm AGENTS.md", 2, "AGENTS.md is a harness file"}})},
		{name: "a second command key is no way out", raw: `{"tool_name":"Bash","tool_input":{"command":"make setup","command":"rm AGENTS.md"}}`,
			calls: []call{{code: 2, output: "AGENTS.md is a harness file"}}},
		{name: "HARNESS_EDIT=1 (set by a human) allows the edit", env: []string{"HARNESS_EDIT=1"},
			calls: []call{{"Edit", "AGENTS.md", 0, ""}, {"Bash", "echo x >> Makefile", 0, ""}}},
		{name: "a binary that rejects a new argument (usage error, ADR 0045)", stub: true,
			env: []string{"STUB_EXIT=64", "STUB_OUTPUT=usage: harness hook-input"}, calls: rebuild(64)},
		{name: "a binary from before exit 64, without the subcommand", stub: true,
			env: []string{"STUB_EXIT=2", "STUB_OUTPUT=usage: harness hook-input"}, calls: rebuild(2)},
		{name: "a binary that cannot run", stub: true, env: []string{"STUB_EXIT=126", "STUB_OUTPUT="}, calls: rebuild(126)},
		{name: "a block from the binary is passed on", stub: true,
			env:   []string{"STUB_EXIT=2", "STUB_OUTPUT=protect-harness: blocked: x is a harness file"},
			calls: []call{{"Edit", "x", 2, "protect-harness: blocked: x is a harness file"}}},
		{name: "a binary that fails for another reason: its message and the way out", stub: true,
			env:   []string{"STUB_EXIT=1", "STUB_OUTPUT=protect-harness: cannot read x"},
			calls: []call{{"Edit", "apps/x.go", 2, "protect-harness: cannot read x\nprotect-harness: blocked: .bin/harness is out of date or failed (exit 1)"}}},
		{name: "a crash that exits 2 is no block message", stub: true,
			env:   []string{"STUB_EXIT=2", "STUB_OUTPUT=panic: x"},
			calls: []call{{"Edit", "apps/x.go", 2, "panic: x\nprotect-harness: blocked: .bin/harness is out of date or failed (exit 2)"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := sandbox(t, files...)
			if tt.stub {
				place(t, stub, filepath.Join(dir, ".bin", "harness"))
			} else {
				linkHarness(t, dir)
			}
			for _, c := range tt.calls {
				in := tt.raw
				if in == "" {
					in = payload(t, dir, c.tool, c.arg)
				}
				code, out := execInput(t, dir, tt.env, in, filepath.Join(dir, hook))
				if code != c.code || (c.output == "") != (out == "") || !strings.Contains(out, c.output) {
					t.Errorf("%s %q: exit %d, want %d; output:\n%s\nwant it to contain %q", c.tool, c.arg, code, c.code, out, c.output)
				}
				if strings.Contains(out, "usage: harness") {
					t.Errorf("%s %q: the usage text reached the agent:\n%s", c.tool, c.arg, out)
				}
			}
		})
	}
}
