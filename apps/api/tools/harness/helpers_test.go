package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
)

// payload is a PreToolUse payload: Bash's command, NotebookEdit's notebook_path or another file tool's file_path.
func payload(t *testing.T, cwd, tool, arg string) string {
	t.Helper()
	key := map[string]string{"Bash": "command", "NotebookEdit": "notebook_path"}[tool]
	if key == "" {
		key = "file_path"
	}
	b, err := json.Marshal(map[string]any{"cwd": cwd, "tool_name": tool, "tool_input": map[string]string{key: arg}})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// runHook runs `harness <hook> root` in-process on a PreToolUse payload and returns the exit code and stderr.
func runHook(t *testing.T, hook, root, stdin string) (int, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run([]string{hook, root}, strings.NewReader(stdin), &stdout, &stderr)
	return code, stderr.String()
}

// TestMain runs the test binary as the harness when a sandbox links it in as .bin/harness (linkHarness), so a
// test of a wrapper runs the code under test, not whatever .bin/harness was last built. After the tests it fails
// the run if one wrote through a sandbox hardlink into a real file (place).
func TestMain(m *testing.M) {
	if filepath.Base(os.Args[0]) == "harness" {
		os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
	}
	code := m.Run()
	if changed := writtenThrough(); len(changed) > 0 {
		fmt.Fprintf(os.Stderr, "a test wrote through a sandbox hardlink into %s (restore it with git; call own before writing)\n",
			strings.Join(changed, ", "))
		code = 1
	}
	os.Exit(code)
}

// repoRoot is the repository the tests run in: the nearest directory above the package that holds
// layout.env, wherever API_DIR puts this tool (README "(b) Brownfield project" step 2).
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, layoutFile)); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("no %s above the test's directory", layoutFile)
		}
		dir = parent
	}
}

// linked maps every real file a sandbox hardlinks to its checksum when it was first linked.
var linked sync.Map

// checksum is a file's content and mode.
func checksum(file string) (string, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(file)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x %v", sha256.Sum256(b), info.Mode()), nil
}

// writtenThrough returns the linked real files whose checksum changed since they were linked.
func writtenThrough() []string {
	var changed []string
	linked.Range(func(file, sum any) bool {
		if now, err := checksum(file.(string)); err != nil || now != sum {
			changed = append(changed, file.(string))
		}
		return true
	})
	sort.Strings(changed)
	return changed
}

// sandbox returns a new git repository on main with the given repo files at the same paths, plus
// layout.sh and layout.env, which the scripts read the repo layout from. Scripts resolve the repo root from
// their own location, so they run on the sandbox. sandboxFrom takes the files from src instead of this repo.
func sandbox(t *testing.T, files ...string) string {
	t.Helper()
	return sandboxFrom(t, repoRoot(t), files...)
}

func sandboxFrom(t *testing.T, src string, files ...string) string {
	t.Helper()
	dir := gitRepo(t, "main")
	for _, f := range append([]string{"scripts/harness/layout.sh", layoutFile}, files...) {
		if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
			continue // a caller naming the layout files too
		}
		place(t, filepath.Join(src, f), filepath.Join(dir, f))
	}
	return dir
}

// linkHarness puts the test binary into the sandbox dir as .bin/harness; TestMain then runs it as the harness.
func linkHarness(t *testing.T, dir string) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	place(t, exe, filepath.Join(dir, ".bin", "harness"))
}

// place puts the file src at dst. An executable is hardlinked: macOS assesses every newly written executable on
// its first exec, about 0.35 s each (exec plan 0002). A data file is copied, and so is an executable when the
// link fails (another volume). A link writes through to src: call own before writing to it; TestMain fails the
// run otherwise.
func place(t *testing.T, src, dst string) {
	t.Helper()
	info, err := os.Stat(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0o100 != 0 && os.Link(src, dst) == nil {
		if _, ok := linked.Load(src); !ok {
			sum, err := checksum(src)
			if err != nil {
				t.Fatal(err)
			}
			linked.LoadOrStore(src, sum)
		}
		return
	}
	if err := copyFile(src, dst, info.Mode()); err != nil {
		t.Fatal(err)
	}
}

// own replaces a sandboxed file with a private copy (same content and mode), breaking its hardlink to the real
// file before a test writes to it.
func own(t *testing.T, file string) {
	t.Helper()
	info, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := copyFile(file, file+".own", info.Mode()); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(file+".own", file); err != nil {
		t.Fatal(err)
	}
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// testEnv is the test process's environment with env added (env wins). It has no latency budgets (a loaded
// machine must not add warnings to tested output), no scope or stack override, like scripts/test/helpers.bash,
// and no HARNESS_EDIT (a human's session would let every protect-harness case through).
func testEnv(env ...string) []string {
	var all []string
	for _, kv := range os.Environ() {
		switch k, _, _ := strings.Cut(kv, "="); k {
		case "HARNESS_BUDGETS", "HARNESS_SCOPE", "HARNESS_STACK", "HARNESS_EDIT":
		default:
			all = append(all, kv)
		}
	}
	return append(append(all, "HARNESS_BUDGETS=/dev/null"), env...)
}

// testWorld is the world of an in-process subcommand: testEnv(env...) as its environment, both for lookupEnv and
// the processes it runs, the test's working directory, empty stdin, stdout into out and stderr to the test's.
func testWorld(t *testing.T, out io.Writer, env ...string) world {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	environ := testEnv(env...)
	return world{stdin: strings.NewReader(""), stdout: out, stderr: os.Stderr, lookupEnv: lookupIn(environ), environ: environ, cwd: cwd}
}

// execIn runs a file in dir with testEnv(env...) and returns its exit code and combined output. execInput also
// gives it stdin.
func execIn(t *testing.T, dir string, env []string, name string, args ...string) (int, string) {
	t.Helper()
	return execInput(t, dir, env, "", name, args...)
}

func execInput(t *testing.T, dir string, env []string, stdin, name string, args ...string) (int, string) {
	t.Helper()
	var out bytes.Buffer
	cmd := exec.Command(name, args...)
	cmd.Dir, cmd.Env, cmd.Stdin, cmd.Stdout, cmd.Stderr = dir, testEnv(env...), strings.NewReader(stdin), &out, &out
	var exit *exec.ExitError
	switch err := cmd.Run(); {
	case err == nil:
		return 0, out.String()
	case errors.As(err, &exit):
		return exit.ExitCode(), out.String()
	default:
		t.Fatalf("%s: %v", name, err)
		return 0, ""
	}
}

// TestSandbox checks the sandbox helper itself (exec plan 0005, A2): executables are the real files, a write
// through one is caught, and own breaks the link.
func TestSandbox(t *testing.T) {
	root := repoRoot(t)
	dir := sandbox(t, "scripts/git-hooks/commit-msg", "docs/harness/rules.md")
	for f, same := range map[string]bool{"scripts/git-hooks/commit-msg": true, "scripts/harness/layout.sh": true,
		"docs/harness/rules.md": false, layoutFile: false} {
		real, err := os.Stat(filepath.Join(root, f))
		if err != nil {
			t.Fatal(err)
		}
		boxed, err := os.Stat(filepath.Join(dir, f))
		if err != nil {
			t.Fatal(err)
		}
		if os.SameFile(real, boxed) != same {
			t.Errorf("%s: same file as the real one is %v, want %v", f, !same, same)
		}
	}

	tool := func(name string) (src, dst string) {
		src = filepath.Join(t.TempDir(), name)
		if err := os.WriteFile(src, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		dst = filepath.Join(dir, name)
		place(t, src, dst)
		t.Cleanup(func() { linked.Delete(src) }) // t.TempDir removes src
		return src, dst
	}
	src, dst := tool("written-through")
	if err := os.WriteFile(dst, []byte("changed\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := writtenThrough(); !slices.Contains(got, src) {
		t.Errorf("writtenThrough() = %v, want it to hold %s", got, src)
	}

	src, dst = tool("owned")
	own(t, dst)
	if err := os.WriteFile(dst, []byte("changed\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := writtenThrough(); slices.Contains(got, src) {
		t.Errorf("writtenThrough() = %v after own, want it without %s", got, src)
	}
}
