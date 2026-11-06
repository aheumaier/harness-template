package main

import (
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"slices"
	"strings"
)

// command is one subcommand, `harness <name> <args>...`. Its file registers it from init() (ADR 0045).
type command struct {
	name  string
	usage string // its part of the usage text, starting with `harness <name>`; alternatives joined with " | "
	about string // what it checks and who calls it, for `harness help`
	run   func(w world, args []string) int
}

// exitBadArgs from a command's run makes dispatch print the usage text and exit exitUsage. No child process
// exits with it (exec.ExitError's ExitCode is -1 for one a signal killed), so a command may pass a child's code on.
const exitBadArgs = math.MinInt

// errUsage from a sensor's check is exitBadArgs.
var errUsage = errors.New("usage")

// registry holds the subcommands by name.
type registry map[string]command

// commands is the registry of the binary; every subcommand file adds itself from init().
var commands = newRegistry()

// newRegistry returns a registry that holds only help.
func newRegistry() registry {
	r := registry{}
	r.register(command{name: "help", usage: "harness help", about: "this list (stdout)",
		run: func(w world, args []string) int {
			if len(args) != 0 {
				return exitBadArgs
			}
			r.help(w.stdout)
			return 0
		}})
	return r
}

// register adds c; a second command with the same name panics at start-up.
func (r registry) register(c command) {
	if _, dup := r[c.name]; dup {
		panic("harness: subcommand " + c.name + " registered twice")
	}
	r[c.name] = c
}

// usage is the usage text: every command's usage, sorted by name.
func (r registry) usage() string {
	var parts []string
	for _, name := range slices.Sorted(maps.Keys(r)) {
		parts = append(parts, r[name].usage)
	}
	return "usage: " + strings.Join(parts, " | ")
}

// help prints each command's usage and what it is for.
func (r registry) help(out io.Writer) {
	for _, name := range slices.Sorted(maps.Keys(r)) {
		fmt.Fprintf(out, "%s\n    %s\n", r[name].usage, r[name].about)
	}
}

// dispatch runs the command args[0] names with the rest of args and returns its exit code. A code outside
// 0..255 (a child killed by a signal) is 2: os.Exit would wrap 256 to 0, a pass.
func (r registry) dispatch(w world, args []string) int {
	if len(args) > 0 {
		if c, ok := r[args[0]]; ok {
			switch code := c.run(w, args[1:]); {
			case code == exitBadArgs:
			case code < 0 || code > 255:
				fmt.Fprintf(w.stderr, "harness %s: exit code %d is out of range, exiting 2\n", args[0], code)
				return 2
			default:
				return code
			}
		}
	}
	fmt.Fprintln(w.stderr, r.usage())
	return exitUsage
}

// sensor is a command for a check that prints its findings to stdout: exit 1 when there is any, 2 with
// `harness <name>: <error>` on stderr when it cannot run, and exitBadArgs when check returns errUsage.
func sensor(name, usage, about string, check func(w world, args []string) (bool, error)) command {
	return command{name: name, usage: usage, about: about, run: func(w world, args []string) int {
		bad, err := check(w, args)
		switch {
		case errors.Is(err, errUsage):
			return exitBadArgs
		case err != nil:
			fmt.Fprintf(w.stderr, "harness %s: %v\n", name, err)
			return 2
		case bad:
			return 1
		}
		return 0
	}}
}

// rootSensor is a sensor run as `harness <name> <root>`; check gets root made absolute.
func rootSensor(name, about string, check func(w world, root string) (bool, error)) command {
	return sensor(name, "harness "+name+" <root>", about, func(w world, args []string) (bool, error) {
		if len(args) != 1 {
			return false, errUsage
		}
		root, err := w.abs(args[0])
		if err != nil {
			return false, err
		}
		return check(w, root)
	})
}

// printFinding prints one sensor finding as `<file>:<line>: <message> [<rule>]`, the line run.sh maps to the rule.
func printFinding(out io.Writer, file string, line int, rule, format string, a ...any) {
	fmt.Fprintf(out, "%s:%d: %s [%s]\n", file, line, fmt.Sprintf(format, a...), rule)
}
