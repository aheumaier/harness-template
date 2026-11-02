package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCommitMsgHook runs scripts/git-hooks/commit-msg: it checks the message with .bin/harness, and blocks with
// "run make setup" when that binary is missing or older than the hook (ADR 0045).
func TestCommitMsgHook(t *testing.T) {
	const hook = "scripts/git-hooks/commit-msg"
	tests := []struct {
		name    string
		harness string   // "test": the code under test; "stub": testdata/harness-stub.sh; "": none
		env     []string // for the stub
		msg     string
		code    int
		output  string // substring; "" means no output
	}{
		{name: "conventional", harness: "test", msg: "feat(api): add orders\n"},
		{name: "not conventional", harness: "test", msg: "add orders\n", code: 1,
			output: "commit message: 'add orders' is not"},
		{name: "missing binary", msg: "feat: x\n", code: 1, output: "run `make setup`"},
		{name: "binary without the subcommand", harness: "stub", env: []string{"STUB_EXIT=64", "STUB_OUTPUT=usage: harness"},
			msg: "feat: x\n", code: 1, output: "run `make setup`"},
		{name: "binary from before exit 64", harness: "stub", env: []string{"STUB_EXIT=2", "STUB_OUTPUT=usage: harness layout"},
			msg: "feat: x\n", code: 1, output: "run `make setup`"},
		{name: "binary failing for another reason", harness: "stub", env: []string{"STUB_EXIT=2", "STUB_OUTPUT=harness check-commits: no file"},
			msg: "feat: x\n", code: 2, output: "harness check-commits: no file"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := sandbox(t, hook, "scripts/harness/run.sh", "scripts/harness/timing.sh", "docs/harness/rules.md")
			switch tt.harness {
			case "test":
				linkHarness(t, dir)
			case "stub":
				stub, err := filepath.Abs("testdata/harness-stub.sh")
				if err != nil {
					t.Fatal(err)
				}
				place(t, stub, filepath.Join(dir, ".bin", "harness"))
			}
			msg := filepath.Join(t.TempDir(), "COMMIT_EDITMSG")
			if err := os.WriteFile(msg, []byte(tt.msg), 0o644); err != nil {
				t.Fatal(err)
			}
			code, out := execIn(t, dir, tt.env, filepath.Join(dir, hook), msg)
			if code != tt.code || (tt.output == "") != (out == "") || !strings.Contains(out, tt.output) {
				t.Errorf("exit %d, want %d; output:\n%s\nwant it to contain %q", code, tt.code, out, tt.output)
			}
		})
	}
}
