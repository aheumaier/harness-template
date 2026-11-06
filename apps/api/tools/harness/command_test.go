package main

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

// TestRegistry checks the contract every subcommand file relies on (ADR 0045): a name registers once, the usage
// text and `harness help` list every command, and a wrong call prints the usage text and exits exitUsage.
func TestRegistry(t *testing.T) {
	t.Run("a second registration panics", func(t *testing.T) {
		r := newRegistry()
		defer func() {
			if recover() == nil {
				t.Error("registering help twice did not panic")
			}
		}()
		r.register(command{name: "help"})
	})

	t.Run("usage and help name every command", func(t *testing.T) {
		usage := commands.usage()
		if !strings.HasPrefix(usage, "usage: harness ") {
			t.Errorf("usage text %q does not start with `usage: harness ` (protect-harness.sh and commit-msg match it)", usage)
		}
		var help bytes.Buffer
		if code := commands.dispatch(testWorld(t, &help), []string{"help"}); code != 0 {
			t.Fatalf("harness help: exit %d", code)
		}
		for name, c := range commands {
			if !strings.HasPrefix(c.usage, "harness "+name) || c.about == "" || c.run == nil {
				t.Errorf("%s: usage %q, about %q: want usage `harness %s ...`, an about and a run", name, c.usage, c.about, name)
			}
			if !strings.Contains(usage, c.usage) || !strings.Contains(help.String(), c.usage+"\n    "+c.about+"\n") {
				t.Errorf("%s: missing from the usage text or harness help", name)
			}
		}
	})

	// The cases below run on a registry of their own, so they hold whichever template subcommands a product keeps.
	r := newRegistry()
	r.register(rootSensor("stub", "a sensor", func(world, string) (bool, error) { return false, nil }))
	r.register(command{name: "pass-on", usage: "harness pass-on <code>", about: "exits with a child's code",
		run: func(_ world, args []string) int {
			return map[string]int{"killed": -1, "wraps": 256, "fails": 1}[args[0]]
		}})

	t.Run("a wrong call prints the usage text", func(t *testing.T) {
		for _, args := range [][]string{nil, {"no-such-command"}, {"help", "extra"}, {"stub"}, {"stub", "a", "b"}} {
			var stdout, stderr bytes.Buffer
			w := testWorld(t, &stdout)
			w.stderr = &stderr
			if code := r.dispatch(w, args); code != exitUsage || stderr.String() != r.usage()+"\n" || stdout.Len() != 0 {
				t.Errorf("harness %q: exit %d, stdout %q, stderr %q; want exit %d and the usage text on stderr",
					args, code, stdout.String(), stderr.String(), exitUsage)
			}
		}
	})

	t.Run("an exit code outside 0..255 is 2", func(t *testing.T) {
		for arg, want := range map[string]int{"killed": 2, "wraps": 2, "fails": 1} {
			var stderr bytes.Buffer
			w := testWorld(t, io.Discard)
			w.stderr = &stderr
			if code := r.dispatch(w, []string{"pass-on", arg}); code != want || (want == 2) != strings.Contains(stderr.String(), "out of range") {
				t.Errorf("pass-on %s: exit %d, stderr %q; want %d", arg, code, stderr.String(), want)
			}
		}
	})
}
