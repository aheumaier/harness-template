package main

import (
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
	// None of the developer's git config (hooks, signing, log format): the scripts' `exec git` gets it from
	// Setup, the subcommands' own git, run in-process, from this process's environment.
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	testscript.Run(t, testscript.Params{
		Dir: "testdata/script",
		Setup: func(env *testscript.Env) error {
			env.Vars = append(env.Vars, "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
				"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@localhost",
				"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@localhost")
			return nil
		},
		Cmds: map[string]func(ts *testscript.TestScript, neg bool, args []string){
			"harness": func(ts *testscript.TestScript, neg bool, args []string) {
				if neg || len(args) < 2 {
					ts.Fatalf("usage: harness <exit code> <subcommand> [args...]")
				}
				want, err := strconv.Atoi(args[0])
				ts.Check(err)
				if got := dispatch(args[1:], ts.Getenv, strings.NewReader(""), ts.Stdout(), ts.Stderr()); got != want {
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
