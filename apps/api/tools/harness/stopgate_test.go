package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

const (
	firstStop    = `{"stop_hook_active":false}`
	repeatedStop = `{"stop_hook_active": true}`
)

// checkRuns is how often `make check` ran in a gateRepo.
func checkRuns(t *testing.T, root string) int {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, ".cache/ran"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return strings.Count(string(b), "ran")
}

// gateRepo is a stampRepo whose `make check` counts its runs and then runs recipe, with a change not on main.
func gateRepo(t *testing.T, recipe ...string) string {
	t.Helper()
	mk := []string{"check:", "\t@mkdir -p .cache; echo ran >> .cache/ran"}
	for _, l := range recipe {
		mk = append(mk, "\t"+l)
	}
	root := stampRepo(t, map[string][]string{"Makefile": mk, "a.go": {"a"}})
	writeLines(t, root, "a.go", "b")
	return root
}

// gate runs the gate in root with its two limits and returns its error and the systemMessage on stdout.
func gate(t *testing.T, ctx context.Context, root, stdin string, limit, after time.Duration, env ...string) (msg string, err error) {
	t.Helper()
	var out bytes.Buffer
	in, err := parseStopInput(strings.NewReader(stdin))
	if err == nil {
		err = stopGate(ctx, testWorld(t, &out, env...), root, in, limit, after)
	}
	if out.Len() > 0 {
		var m map[string]string
		if err := json.Unmarshal(out.Bytes(), &m); err != nil || len(m) != 1 {
			t.Fatalf("stdout %q is not one systemMessage: %v", &out, err)
		}
		msg = m["systemMessage"]
	}
	return msg, err
}

func TestStopGate(t *testing.T) {
	t.Parallel()
	const (
		red     = "@echo 'x.go:1: bad'; echo '✗ sensor a failed'; echo 'rule:        go-layers'; exit 1"
		unknown = "is not known to be green"
	)
	task := func(fields string) string {
		return `{"stop_hook_active":false,"background_tasks":[{"status":"running",` + fields + `}]}`
	}
	tests := []struct {
		name   string
		recipe string
		stdin  string
		setup  func(t *testing.T, root string)
		env    []string
		runs   int
		block  []string // substrings of the block; none means the stop goes through
		msg    string   // substring of the systemMessage; "" means none
	}{
		{name: "green", recipe: "@true", stdin: firstStop, runs: 1},
		{name: "red", recipe: red, stdin: firstStop, runs: 1,
			block: []string{"[harness-stop-is-green]", "To proceed: ", "x.go:1: bad", "✗ sensor a failed", "rule:        go-layers"}},
		{name: "red, but the second stop", recipe: red, stdin: repeatedStop, msg: unknown},
		{name: "an input that does not say which stop it is", recipe: red, stdin: `{}`, msg: unknown},
		{name: "null", recipe: red, stdin: `null`, msg: unknown},
		{name: "plan mode", recipe: red, stdin: `{"stop_hook_active":false,"permission_mode":"plan"}`},
		{name: "another mode", recipe: red, stdin: `{"stop_hook_active":false,"permission_mode":"default"}`, runs: 1, block: []string{"red"}},
		{name: "a subagent is running", recipe: red, stdin: task(`"type":"subagent"`)},
		{name: "a background shell runs the check", recipe: red, stdin: task(`"type":"shell","command":"cd x && make check-all"`)},
		{name: "a background shell runs the check, and then more", recipe: red, stdin: task(`"type":"shell","command":"make check && make dev"`), runs: 1, block: []string{"red"}},
		{name: "red under the caller's make -i", recipe: red, stdin: firstStop, env: []string{"MAKEFLAGS=i"}, runs: 1, block: []string{"red"}},
		{name: "red under the caller's make -n", recipe: red, stdin: firstStop, env: []string{"MAKEFLAGS=n"}, runs: 1, block: []string{"red"}},
		{name: "red under the caller's makefile", recipe: red, stdin: firstStop, env: []string{"MAKEFILES=x.mk"}, runs: 1, block: []string{"red"},
			setup: func(t *testing.T, root string) { writeLines(t, root, "x.mk", "SHELL := /usr/bin/true") }},
		{name: "red under the caller's MAKE", recipe: "@$(MAKE) none", stdin: firstStop, env: []string{"MAKE=true"}, runs: 1, block: []string{"red"}},
		{name: "a subagent that is done", recipe: red, runs: 1, block: []string{"red"},
			stdin: `{"stop_hook_active":false,"background_tasks":[{"status":"completed","type":"subagent"}]}`},
		{name: "a stamped tree", recipe: red, stdin: firstStop, setup: func(t *testing.T, root string) {
			if err := keyOf(t, root).stamp(root); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "a tree stamped before another edit", recipe: red, stdin: firstStop, runs: 1, block: []string{"red"},
			setup: func(t *testing.T, root string) {
				if err := keyOf(t, root).stamp(root); err != nil {
					t.Fatal(err)
				}
				writeLines(t, root, "a.go", "c")
			}},
		{name: "a tree stamped with another stack", recipe: red, stdin: firstStop, runs: 1, block: []string{"red"},
			setup: func(t *testing.T, root string) {
				if err := keyOf(t, root, "HARNESS_STACK=go").stamp(root); err != nil {
					t.Fatal(err)
				}
			}},
		{name: "main's tree", recipe: red, stdin: firstStop, setup: func(t *testing.T, root string) { writeLines(t, root, "a.go", "a") }},
		{name: "main's tree on a second stop", recipe: red, stdin: repeatedStop, setup: func(t *testing.T, root string) { writeLines(t, root, "a.go", "a") }},
		{name: "no key, as layout.env is broken", recipe: "@true", stdin: firstStop, runs: 1,
			setup: func(t *testing.T, root string) { writeLines(t, root, layoutFile, "X") }},
		{name: "no key on a second stop", recipe: red, stdin: repeatedStop, msg: "(cannot tell which tree this is: ",
			setup: func(t *testing.T, root string) { writeLines(t, root, layoutFile, "X") }},
		{name: "input that is no JSON", recipe: "@true", stdin: "x", block: []string{"cannot parse"}},
		{name: "no git and no make", recipe: "@true", stdin: firstStop, env: []string{"PATH=/nonexistent"}, block: []string{"cannot run `make check`"}},
		{name: "no git and no make on a second stop", recipe: "@true", stdin: repeatedStop, env: []string{"PATH=/nonexistent"}, msg: unknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := gateRepo(t, tt.recipe)
			if tt.setup != nil {
				tt.setup(t, root)
			}
			msg, err := gate(t, t.Context(), root, tt.stdin, time.Minute, time.Minute, tt.env...)
			if (err != nil) != (len(tt.block) > 0) {
				t.Errorf("error %v, want a block: %v", err, len(tt.block) > 0)
			}
			for _, s := range tt.block {
				if err != nil && !strings.Contains(err.Error(), s) {
					t.Errorf("block %q misses %q", err, s)
				}
			}
			if (msg == "") != (tt.msg == "") || !strings.Contains(msg, tt.msg) {
				t.Errorf("systemMessage %q, want %q in it", msg, tt.msg)
			}
			if got := checkRuns(t, root); got != tt.runs {
				t.Errorf("make check ran %d times, want %d", got, tt.runs)
			}
		})
	}
}

// waitFile waits until file has content and returns it.
func waitFile(t *testing.T, file string) string {
	t.Helper()
	for end := time.Now().Add(time.Minute); time.Now().Before(end); time.Sleep(20 * time.Millisecond) {
		if b, err := os.ReadFile(file); err == nil && len(b) > 0 {
			return strings.TrimSpace(string(b))
		}
	}
	t.Fatalf("%s was never written", file)
	return ""
}

// waitEnded waits until the process whose id is in file is gone.
func waitEnded(t *testing.T, file string) {
	t.Helper()
	pid, err := strconv.Atoi(waitFile(t, file))
	if err != nil {
		t.Fatal(err)
	}
	for end := time.Now().Add(10 * time.Second); syscall.Kill(pid, 0) == nil; time.Sleep(20 * time.Millisecond) {
		if time.Now().After(end) {
			t.Fatalf("process %d still runs", pid)
		}
	}
}

// TestCheckCommand: a command with another one before or after the check is none; the last two trues outlive their
// check (ADR 0048).
func TestCheckCommand(t *testing.T) {
	t.Parallel()
	for command, want := range map[string]bool{
		"make check":             true,
		"cd x && make check-all": true,
		"HARNESS_SCOPE=full make -s check X=1 -k": true,
		"make check > x.log 2>&1":                 true,
		"make check 2>&1 | tail -40":              true,
		"make check>x.log":                        true,
		"make check | sleep 99":                   true,
		"make check > $(sleep 99)":                true,
		"make check && make dev":                  false,
		"make check || make dev":                  false,
		"make check & make dev":                   false,
		"make check; make dev":                    false,
		"make check\nmake dev":                    false,
		"make check dev":                          false,
		"make dev; make check":                    false,
		"make dev # make check":                   false,
		"make dev # `make check` is red":          false,
		"make check-ios":                          false,
		"make checks":                             false,
		"tail -f x | grep -E 'red|make check'":    false,
		"while :; do sleep 60; make check; done":  false,
		"cd x; sleep 60 && make check":            false,
	} {
		if got := checkCommand.MatchString(command); got != want {
			t.Errorf("%q is a check: %v, want %v", command, got, want)
		}
	}
}

// slowFilter makes git add run a filter in root that writes .cache/pid and hangs, with a child that holds git's
// outputs open from a process group of its own. add is where git finds the file it filters.
func slowFilter(t *testing.T, root string) {
	t.Helper()
	gitT(t, root, "config", "filter.slow.clean", "bash .cache/filter.sh")
	writeLines(t, root, ".cache/filter.sh", "set -m", "sleep 60 &", "echo $! > .cache/outside", "echo $$ > .cache/pid", "wait")
	writeLines(t, root, ".gitattributes", "*.slow filter=slow")
	t.Cleanup(func() {
		if b, err := os.ReadFile(filepath.Join(root, ".cache/outside")); err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil {
				_ = syscall.Kill(pid, syscall.SIGKILL)
			}
		}
	})
}

// TestStopGateEnded: a key or a check that is cut off is ended with everything in its process group, leaves no file
// in TMPDIR, and blocks; the message asks for make gen only when the check may have left the tree changed.
func TestStopGateEnded(t *testing.T) {
	t.Parallel()
	const run = `echo $$$$ > .cache/pid; sleep 60 & wait`
	tests := []struct {
		name   string
		recipe string
		setup  func(t *testing.T, root string)
		after  time.Duration // for the second key; 0 is as long as it takes
		gen    bool
		term   bool // the child gets to act on TERM
		unrun  bool // the check does not start
	}{
		{name: "a child that acts on TERM", recipe: `@sh -c 'trap "echo x > .cache/term; exit" TERM; ` + run + `'`, term: true},
		{name: "a child that ignores TERM", recipe: `@sh -c 'trap "" TERM; echo $$$$ > .cache/pid; exec sleep 60'`},
		{name: "a half-written file", recipe: "@echo x > gen.go; sh -c '" + run + "'", gen: true},
		{name: "no first key", recipe: "@sh -c '" + run + "'", gen: true,
			setup: func(t *testing.T, root string) { writeLines(t, root, layoutFile, "X") }},
		{name: "a second key that hangs", recipe: "@echo x > a.slow; sh -c '" + run + "'", setup: slowFilter, after: time.Second, gen: true},
		{name: "a first key that hangs", recipe: "@true", unrun: true, setup: func(t *testing.T, root string) {
			slowFilter(t, root)
			writeLines(t, root, "a.slow", "x")
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root, tmp := gateRepo(t, tt.recipe), t.TempDir()
			if tt.setup != nil {
				tt.setup(t, root)
			}
			if tt.after == 0 {
				tt.after = time.Minute
			}
			pid := filepath.Join(root, ".cache/pid")
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			var ended time.Time
			go func() { // the end waits for the check, however slow this machine is
				for ctx.Err() == nil {
					if b, _ := os.ReadFile(pid); len(b) > 0 {
						ended = time.Now()
						cancel()
					}
					time.Sleep(20 * time.Millisecond)
				}
			}()
			_, err := gate(t, ctx, root, firstStop, time.Minute, tt.after, "TMPDIR="+tmp)
			if err == nil || !strings.Contains(err.Error(), "was ended by a signal") || !strings.Contains(err.Error(), "run `make check` yourself") {
				t.Fatalf("error %v, want a block that says what ended the check", err)
			}
			if got := time.Since(ended); got > 30*time.Second {
				t.Errorf("the gate ended %s after its check, want less", got)
			}
			if got := strings.Contains(err.Error(), "make gen"); got != tt.gen {
				t.Errorf("make gen in the message: %v, want %v: %v", got, tt.gen, err)
			}
			if got := checkRuns(t, root); (got == 0) != tt.unrun {
				t.Errorf("make check ran %d times, want none: %v", got, tt.unrun)
			}
			waitEnded(t, pid)
			if _, err := os.Stat(filepath.Join(root, ".cache/term")); (err == nil) != tt.term {
				t.Errorf("the child acted on TERM: %v, want %v", err == nil, tt.term)
			}
			if left, err := os.ReadDir(tmp); err != nil || len(left) > 0 {
				t.Errorf("left in TMPDIR: %v, %v", left, err)
			}
		})
	}
}

// TestStopGateLimit: the limit covers the check and the key.
func TestStopGateLimit(t *testing.T) {
	t.Parallel()
	root, start := gateRepo(t, "@sleep 60"), time.Now()
	_, err := gate(t, t.Context(), root, firstStop, 3*time.Second, time.Minute)
	if err == nil || !strings.Contains(err.Error(), "was not done after 3s") || time.Since(start) > 30*time.Second {
		t.Errorf("error %v after %s, want a block at the limit", err, time.Since(start))
	}
	msg, err := gate(t, t.Context(), root, repeatedStop, time.Nanosecond, time.Minute)
	if err != nil || !strings.Contains(msg, "(cannot tell which tree this is: ") {
		t.Errorf("systemMessage %q, error %v; want a key that was cut off", msg, err)
	}
}

// TestStopGateTimeout: Claude Code gives the hook longer than it can take (ADR 0048), or it would kill the gate and
// let the stop through.
func TestStopGateTimeout(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), ".claude/settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var s struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Command string
				Timeout int
			}
		}
	}
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatal(err)
	}
	worst, found := stopLimit+killGrace+afterLimit+2*killGrace, false // the check's end; the second key and its end
	for _, e := range s.Hooks["Stop"] {
		for _, h := range e.Hooks {
			if strings.HasSuffix(h.Command, "/.claude/hooks/stop-gate.sh") {
				found = true
				if got := time.Duration(h.Timeout) * time.Second; got <= worst {
					t.Errorf("timeout %s, want more than %s", got, worst)
				}
			}
		}
	}
	if !found {
		t.Error("no Stop hook runs stop-gate.sh")
	}
}

// TestStopGateSignal: the command ends its check on a signal, and blocks.
func TestStopGateSignal(t *testing.T) {
	t.Parallel()
	for _, sig := range []syscall.Signal{syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP} {
		t.Run(sig.String(), func(t *testing.T) {
			t.Parallel()
			root := gateRepo(t, "@sh -c 'echo $$$$ > .cache/pid; exec sleep 60'")
			linkHarness(t, root)
			var out bytes.Buffer
			cmd := startIn(t, root, nil, firstStop, &out, &out, filepath.Join(root, ".bin/harness"), "stop-gate", root)
			waitFile(t, filepath.Join(root, ".cache/pid"))
			if err := cmd.Process.Signal(sig); err != nil {
				t.Fatal(err)
			}
			if code := exitCode(t, cmd); code != 2 || !strings.Contains(out.String(), "was ended by a signal") {
				t.Errorf("exit %d, want 2; output:\n%s", code, &out)
			}
			waitEnded(t, filepath.Join(root, ".cache/pid"))
		})
	}
}

func TestDigest(t *testing.T) {
	t.Parallel()
	var long strings.Builder
	long.WriteString("✗ sensor early failed (exit 1)\nrule:        early-rule\nremediation: do this\n")
	for i := range 100 {
		fmt.Fprintf(&long, "line %d\n", i)
	}
	got := digest([]byte(long.String()))
	for _, s := range []string{"✗ sensor early", "rule:        early-rule", "remediation: do this", "the last 40 of 103 lines", "line 60\n", "line 99"} {
		if !strings.Contains(got, s) {
			t.Errorf("digest misses %q:\n%s", s, got)
		}
	}
	if strings.Contains(got, "line 59\n") {
		t.Errorf("digest is more than the last 40 lines:\n%s", got)
	}
	if got := digest([]byte("a\nb\n")); got != "a\nb" {
		t.Errorf("short digest %q", got)
	}
}

// TestStopGateHook runs .claude/hooks/stop-gate.sh: whatever is wrong with the binary blocks a first stop and
// lets any other go with a systemMessage (ADR 0048).
func TestStopGateHook(t *testing.T) {
	t.Parallel()
	const hook = ".claude/hooks/stop-gate.sh"
	files := []string{hook, "scripts/harness/hook-bootstrap.sh", "scripts/harness/timing.sh"}
	stub, err := filepath.Abs("testdata/harness-stub.sh")
	if err != nil {
		t.Fatal(err)
	}
	const gave = `{"systemMessage":"harness-stop-is-green: the stop gate could not judge this tree`
	tests := []struct {
		name  string
		stub  bool // testdata/harness-stub.sh as .bin/harness
		env   []string
		files []string // instead of files
		block string   // what a first stop says
	}{
		{name: "no binary", block: "stop-gate: blocked: .bin/harness is missing; run `make setup`"},
		{name: "no binary, in a UTF-8 locale", env: []string{"LC_ALL=en_US.UTF-8"}, block: "stop-gate: blocked: .bin/harness is missing"},
		{name: "no bootstrap", files: []string{hook}, block: "hook-bootstrap.sh is missing; restore it from git"},
		{name: "a binary without the subcommand (usage error, ADR 0045)", stub: true,
			env: []string{"STUB_EXIT=64", "STUB_OUTPUT=usage: harness x"}, block: "out of date or failed (exit 64); rebuild it"},
		{name: "a binary from before exit 64", stub: true,
			env: []string{"STUB_EXIT=2", "STUB_OUTPUT=usage: harness x"}, block: "out of date or failed (exit 2); rebuild it"},
		{name: "a binary that cannot run", stub: true, env: []string{"STUB_EXIT=126", "STUB_OUTPUT="}, block: "failed (exit 126)"},
		{name: "a crash", stub: true, env: []string{"STUB_EXIT=2", "STUB_OUTPUT=panic: x"}, block: "panic: x\nstop-gate: blocked: .bin/harness is out of date or failed (exit 2)"},
		{name: "its own block", stub: true, env: []string{"STUB_EXIT=2", "STUB_OUTPUT=stop-gate: blocked: red"}, block: "stop-gate: blocked: red\n"},
		{name: "its own block, which quotes a usage text", stub: true,
			env: []string{"STUB_EXIT=2", "STUB_OUTPUT=stop-gate: blocked: red\nusage : x\nusage: harness x"}, block: "red\nusage : x\nusage: harness x\n"},
	}
	inputs := []struct {
		stdin string
		first bool
	}{
		{firstStop, true},
		{"{\"last_assistant_message\":\"\xff\",\"stop_hook_active\":false}", true},
		{`{"stop_hook_active" : false, "last_assistant_message":"\"stop_hook_active\": true"}`, true},
		{repeatedStop, false},
		{`{"stop_hook_active":true, "last_assistant_message":"\"stop_hook_active\": false"}`, false},
		{`{}`, false},
		{`{"last_assistant_message":"\"stop_hook_active\":false"}`, false},
		{`{"stop_hook_active":"false"}`, false},
		{`{"stop_hook_active":false,"stop_hook_active":true}`, false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.files == nil {
				tt.files = files
			}
			dir := sandbox(t, tt.files...)
			if tt.stub {
				place(t, stub, filepath.Join(dir, ".bin", "harness"))
			}
			for _, in := range inputs {
				code, stdout, stderr := execInput(t, dir, tt.env, in.stdin, filepath.Join(dir, hook))
				if in.first {
					if code != 2 || stdout != "" || !strings.Contains(stderr, tt.block) || !strings.Contains(stderr, "The next stop passes") ||
						strings.Contains(stderr, "usage: harness") && !strings.Contains(stderr, "blocked: red") {
						t.Errorf("%s: exit %d, want 2; stdout %q, want none; stderr, want %q in it:\n%s", in.stdin, code, stdout, tt.block, stderr)
					}
				} else if code != 0 || !strings.HasPrefix(stdout, gave) || !json.Valid([]byte(stdout)) {
					t.Errorf("%s: exit %d, want 0; stdout, want one systemMessage:\n%s", in.stdin, code, stdout)
				}
			}
		})
	}

	t.Run("the code under test", func(t *testing.T) {
		t.Parallel()
		dir := sandbox(t, files...)
		linkHarness(t, dir)
		writeLines(t, dir, ".gitignore", ".bin/", ".cache/")
		writeLines(t, dir, "Makefile", "check:", "\t@echo '✗ sensor a failed'; exit 1")
		for _, c := range []struct {
			stdin, output string
			code          int
		}{
			{firstStop, "stop-gate: blocked: [harness-stop-is-green] `make check` is red in " + dir, 2},
			{repeatedStop, `{"systemMessage":"harness-stop-is-green: ` + dir + " is not known to be green", 0},
			{`{"stop_hook_active":false,"permission_mode":"plan"}`, "", 0},
		} {
			code, out, stderr := execInput(t, dir, nil, c.stdin, filepath.Join(dir, hook))
			if c.code == 2 {
				out, stderr = stderr, out
			}
			if code != c.code || !strings.Contains(out, c.output) || (c.output == "") != (out == "") || stderr != "" {
				t.Errorf("%s: exit %d, want %d; output, want %q in it:\n%s\nthe other stream, want nothing: %q", c.stdin, code, c.code, c.output, out, stderr)
			}
		}
	})
}
