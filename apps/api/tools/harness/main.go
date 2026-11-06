// Command harness holds the parts of the harness hooks and sensors that parse input (ADR 0012), plus the
// one-shot template rename, which needs the same portable file handling, and the harness logic ported from
// shell (ADR 0045). It is built into .bin/harness by make (setup, check); `harness help` lists the subcommands
// and their callers.
//
// Each subcommand lives in its own file and registers itself from init() (command.go). Exit 2 is a block (or
// unusable input) for Claude Code; the hooks pass it on. Sensors exit 1 on findings and 2 when they cannot run.
// A usage error (an unknown subcommand, a wrong argument) exits 64, which the wrappers and git hooks read as a
// stale binary (ADR 0045).
//
// # Stable surface
//
// A product's own subcommand is one more file of this package. It may use the names below; a template change
// to any of them is a breaking change and says so in its PR:
//
//   - command.go: the command type (name, usage, about, run), commands.register, sensor and rootSensor,
//     errUsage, exitBadArgs, printFinding. A wrong call returns exitBadArgs (dispatch prints the usage text);
//     exitUsage (main.go) directly is only for a command that prints its own reason, as hook-input does;
//   - world.go: the world type (stdin, stdout, stderr, lookupEnv, environ, cwd), lookupIn, and the methods
//     getenv, harnessRoot, abs, command, git, gitFiles, gitKnown, lsFiles, mergeBase. A subcommand reads the
//     environment and the working directory only through its world, never through os; rootSensor and hookGuard
//     hand it <root> made absolute. A world is built by run or a test helper, lookupEnv always from environ
//     except where testscript cannot list its variables (scriptWorld);
//   - layout.go: loadLayout and parseLayout, the layout type and its methods get, list, has, apiPath and
//     domainsDir, and isDomainDir;
//   - hookinput.go: parseHookInput and the hookInput type; hook.go: hookGuard and blockf;
//   - paths.go: rulesFile, skippedPath, commentRE, the protectedPath type, protectedPaths, pathspecMatch,
//     pathspecMatchFold, matchesAny, matchesAnyFold, resolveExisting;
//   - in tests, testWorld and the sandbox helper of helpers_test.go.
package main

import (
	"io"
	"os"
	"path/filepath"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// exitUsage is the exit code of a usage error.
const exitUsage = 64

// run is the only place that reads the process's environment and working directory: it puts them into the
// world every subcommand gets.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	cwd, _ := os.Getwd() // "" when unreadable: a relative path then fails (world.abs) or blocks
	if p, err := filepath.EvalSymlinks(cwd); err == nil && cwd != "" {
		cwd = p // world.abs cleans `..` lexically; on $PWD through a symlink that would name another tree
	}
	environ := os.Environ()
	w := world{stdin: stdin, stdout: stdout, stderr: stderr, lookupEnv: lookupIn(environ), environ: environ, cwd: cwd}
	return commands.dispatch(w, args)
}
