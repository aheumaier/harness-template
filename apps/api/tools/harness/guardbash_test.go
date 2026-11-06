package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// guardRun feeds a PreToolUse Bash payload to `harness guard-bash root` and returns the exit code and stderr.
func guardRun(t *testing.T, root, cwd, command string) (int, string) {
	t.Helper()
	return runHook(t, "guard-bash", root, payload(t, cwd, "Bash", command))
}

// gitRepo returns a new repository (symlinks resolved) on branch.
func gitRepo(t *testing.T, branch string) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", dir, "init", "-q", "-b", branch).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	return dir
}

func TestGuardBash(t *testing.T) {
	root := gitRepo(t, "feat")
	onMain := gitRepo(t, "main")
	tests := []struct {
		name     string
		cwd      string // default root
		commands []string
		blocked  string // stderr substring; "" means allowed
	}{
		// The cases of scripts/test/guard-bash.bats before ADR 0012, one row per bats test.
		{name: "force pushes and force refspecs", blocked: "force", commands: []string{
			"git push --force origin feat", "git push -f origin feat", "git push --force-with-lease",
			"git push origin +feat", "git push --mirror", "git push origin --delete feat", "git push -fu origin feat",
			"git push \\\n  --force origin feat",
		}},
		{name: "pushes to main or master", blocked: "Push a feature branch", commands: []string{
			"git push origin main", "git push origin HEAD:main", "git push origin feat:refs/heads/master",
			"FOO=1 git -C . push origin main", "make check && git push origin main", `git push origin "main"`,
			"git push origin 'main'",
		}},
		{name: "bare git push and HEAD/@ refspecs from main", cwd: onMain, blocked: "main", commands: []string{
			"git push", "git push -u origin HEAD", "git push origin @",
		}},
		{name: "pushes of feature branches", commands: []string{
			"git push", "git push -u origin feat", "git -C . push origin feat:feat", "git push -u origin HEAD",
		}},
		{name: "rm -r outside the repo or with unexpanded paths", blocked: "guard-bash: blocked: rm", commands: []string{
			"rm -rf /tmp/x", "rm -rf ../other", "rm -fr /", "FOO=1 rm -r /", `rm -rf "$HOME/x"`, "rm -rf ~",
			"rm -r `pwd`", "cd x && rm -rf /etc", "rm -rf .*", "find . | xargs rm -rf", "rm -rf \\\n/tmp/x",
		}},
		{name: "rm behind wrappers, keywords, grouping and quotes", blocked: "guard-bash: blocked: rm", commands: []string{
			"(rm -rf /)", "{ rm -rf /; }", "echo $(rm -rf /x)", `\rm -rf /x`, `"rm" -rf /x`, "sudo -n rm -rf /x",
			"if true; then rm -rf /tmp/a; fi", "for d in a; do rm -rf /x/$d; done", "env -i rm -rf /x", "time rm -r /x",
		}},
		{name: "rm inside the repo and non-recursive rm", commands: []string{
			"rm -rf node_modules dist", "rm -rf ./build", "rm -rf dist/*", "rm file.txt /etc/x", "rm -f /tmp/x.log",
			"ls; git status", `echo "a; b" | grep x`, "rm -rf ROOT/apps/web/dist",
		}},
		{name: "commands that name a .env file", blocked: ".env files hold", commands: []string{
			"cat .env.local", "grep URL .env.local", "cp .env.local /tmp/x", "source ./.env", "wc -l <.env.local",
			"cat apps/api/.env",
		}},
		{name: ".env.example", commands: []string{"cat .env.example"}},
		{name: "heredoc bodies are data", commands: []string{
			"git commit -F - <<'EOF'\nrm -rf /\ngit push --force\nEOF\necho done",
		}},
		{name: "commands after a heredoc", blocked: "/tmp/after", commands: []string{
			"cat <<-X >f\n\trm -rf /\n\tX\nrm -rf /tmp/after",
		}},
		{name: "a here-string does not hide later commands", blocked: "/tmp/x", commands: []string{
			"grep x <<<foo\nrm -rf /tmp/x",
		}},
		{name: "an unterminated heredoc does not hide later commands", blocked: "/tmp/y", commands: []string{
			"echo \"<<EOF\"\nrm -rf /tmp/y",
		}},

		// Read as bash now (ADR 0012), no looser than the tokenizer.
		{name: "commands in strings after a separator", blocked: "guard-bash: blocked: rm", commands: []string{
			`echo "x; rm -rf /y"`, `echo "ok $X; rm -rf /y"`, "git commit -m 'fix\nrm -rf /y'",
		}},
		{name: "a .env file inside a string", blocked: ".env files hold", commands: []string{
			`git commit -m "ignore .env.local"`, `cat "$D/.env"`, "[[ -f .env ]]", "X=.env.local true",
		}},
		{name: "bash -c, sh -c, zsh -c and eval", blocked: "guard-bash: blocked:", commands: []string{
			`bash -c "rm -rf /x"`, `sh -ec 'git push --force'`, `zsh -lc "cat .env.local"`, `eval rm -rf /x`,
			`bash -c 'bash -c "rm -rf /x"'`,
		}},
		{name: "substitutions in an unquoted heredoc run", blocked: "outside the repository", commands: []string{
			"cat <<EOF\n$(rm -rf /x)\nEOF",
		}},
		{name: "a commit message from a quoted heredoc", commands: []string{
			"git commit -m \"$(cat <<'EOF'\nfeat: x\n\nrm -rf / was the bug; git push --force too\nEOF\n)\"",
		}},
		{name: "rm -r with a dynamic path in the repo", blocked: "unexpanded path ($d)", commands: []string{
			"for d in a b; do rm -rf $d; done",
		}},
		{name: "unparseable command", blocked: "cannot parse the command as bash", commands: []string{
			`echo "unterminated`, "if true; then",
		}},
		{name: "empty command", commands: []string{""}},
	}
	n := 0
	for _, tt := range tests {
		cwd := tt.cwd
		if cwd == "" {
			cwd = root
		}
		for _, c := range tt.commands {
			n++
			c = strings.ReplaceAll(c, "ROOT", root)
			code, stderr := guardRun(t, root, cwd, c)
			if tt.blocked == "" && code != 0 {
				t.Errorf("%s: %q blocked, want allowed:\n%s", tt.name, c, stderr)
			}
			if tt.blocked != "" && (code != 2 || !strings.Contains(stderr, tt.blocked)) {
				t.Errorf("%s: %q exit %d, want 2 with %q:\n%s", tt.name, c, code, tt.blocked, stderr)
			}
		}
	}
	if n < 63 {
		t.Errorf("%d cases, want at least the 63 moved from guard-bash.bats", n)
	}
}

func TestGuardBashFailsClosed(t *testing.T) {
	root := t.TempDir()
	for _, in := range []string{"not json", "", "null", "[]", `{"tool_input":{"command":1}}`, `{} {}`} {
		var stdout, stderr bytes.Buffer
		code := run([]string{"guard-bash", root}, strings.NewReader(in), &stdout, &stderr)
		if code != 2 || !strings.Contains(stderr.String(), "guard-bash: blocked: cannot parse the hook input") {
			t.Errorf("%q: exit %d, stderr %q; want 2, cannot parse", in, code, stderr.String())
		}
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"guard-bash"}, strings.NewReader("{}"), &stdout, &stderr); code != exitUsage {
		t.Errorf("no root: exit %d, want %d", code, exitUsage)
	}
}

// TestGuardBashNoGit: when git cannot start, a bare push may be from main, so it blocks (it used to read as "no
// branch" and pass).
func TestGuardBashNoGit(t *testing.T) {
	onMain := gitRepo(t, "main")
	var stderr bytes.Buffer
	w := testWorld(t, io.Discard, "PATH="+t.TempDir())
	w.stdin, w.stderr = strings.NewReader(payload(t, onMain, "Bash", "git push")), &stderr
	if code := commands.dispatch(w, []string{"guard-bash", onMain}); code != 2 || !strings.Contains(stderr.String(), "git cannot run") {
		t.Errorf("git push without git on PATH: exit %d, stderr %q; want 2", code, stderr.String())
	}
}

// TestGuardBashNoCwd: with neither a usable payload cwd (none, or a relative one) nor a working directory, nothing
// can be judged, so it blocks.
func TestGuardBashNoCwd(t *testing.T) {
	root := gitRepo(t, "main")
	for _, tt := range []struct{ payloadCwd, cwd string }{{"", ""}, {".", root}, {"sub", root}} {
		var stderr bytes.Buffer
		w := testWorld(t, io.Discard)
		w.cwd, w.stdin, w.stderr = tt.cwd, strings.NewReader(payload(t, tt.payloadCwd, "Bash", "git push")), &stderr
		if code := commands.dispatch(w, []string{"guard-bash", root}); code != 2 || !strings.Contains(stderr.String(), "no working directory") {
			t.Errorf("git push from payload cwd %q, cwd %q: exit %d, stderr %q; want 2", tt.payloadCwd, tt.cwd, code, stderr.String())
		}
	}
}

func TestGuardBashUnusableCwd(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	if err := os.Mkdir("sub", 0o755); err != nil {
		t.Fatal(err)
	}
	// A missing cwd falls back to the working directory, like `cd "$cwd" || cwd=$PWD`.
	if code, stderr := guardRun(t, root, filepath.Join(root, "gone"), "rm -rf sub"); code != 0 {
		t.Errorf("rm -rf sub from a missing cwd: exit %d\n%s", code, stderr)
	}
}
