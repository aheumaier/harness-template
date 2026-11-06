package main

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
)

// TestScripts runs the scripts in testdata/script (ADR 0045): a subcommand's cases as the shell a developer
// would type, with two commands of their own:
//
//	harness <exit code> <subcommand> [args...]   runs the subcommand in-process and checks its exit code;
//	                                             stdout and stderr then hold its output
//	shortsha <var> <rev>                         sets $var to the abbreviated sha of rev in $WORK
func TestScripts(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir: "testdata/script",
		Setup: func(env *testscript.Env) error {
			// None of the developer's git config (hooks, signing, log format), for `exec git` and the subcommands'.
			env.Vars = append(env.Vars, "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
				"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@localhost",
				"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@localhost")
			env.Values[scriptVars{}] = slices.Clone(env.Vars)
			return nil
		},
		Cmds: map[string]func(ts *testscript.TestScript, neg bool, args []string){
			"harness": func(ts *testscript.TestScript, neg bool, args []string) {
				if neg || len(args) < 2 {
					ts.Fatalf("usage: harness <exit code> <subcommand> [args...]")
				}
				want, err := strconv.Atoi(args[0])
				ts.Check(err)
				if got := commands.dispatch(scriptWorld(ts), args[1:]); got != want {
					ts.Fatalf("harness %s: exit %d, want %d", strings.Join(args[1:], " "), got, want)
				}
			},
			"shortsha": func(ts *testscript.TestScript, neg bool, args []string) {
				if neg || len(args) != 2 {
					ts.Fatalf("usage: shortsha <var> <rev>")
				}
				ts.Check(ts.Exec("git", "rev-parse", "--short", args[1]))
				ts.Setenv(args[0], strings.TrimSpace(ts.ReadFile("stdout")))
			},
		},
	})
}

// scriptVars keys the script's environment from Setup in testscript.Env.Values.
type scriptVars struct{}

// scriptWorld is the world of a script's `harness` command: its working directory and environment. testscript
// says no more than ts.Getenv, so a variable set to "" reads as unset, and the processes the subcommand runs get
// Setup's variables at their current values, without the ones a script adds. Its stdin is always empty (a
// script's `stdin` does not reach it), so a hook test there would only see the empty-input block.
func scriptWorld(ts *testscript.TestScript) world {
	var environ []string
	for _, kv := range ts.Value(scriptVars{}).([]string) {
		k, _, _ := strings.Cut(kv, "=")
		environ = append(environ, k+"="+ts.Getenv(k))
	}
	return world{stdin: strings.NewReader(""), stdout: ts.Stdout(), stderr: ts.Stderr(), environ: environ, cwd: ts.MkAbs("."),
		lookupEnv: func(key string) (string, bool) {
			v := ts.Getenv(key)
			return v, v != ""
		}}
}
