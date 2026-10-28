// Command harness holds the parts of the harness hooks and sensors that parse input (ADR 0012). The shell
// scripts stay the entry points and call it; it is built into .bin/harness by make (setup, check).
//
//	harness hook-input <field>...  Claude Code hook JSON on stdin → the fields, one per line
//	harness guard-bash <root>      PreToolUse guard for Bash commands (.claude/hooks/guard-bash.sh)
//
// Exit 2 is a block (or unusable input) for Claude Code; the hooks pass it on.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

const usage = "usage: harness hook-input <cwd|tool_name|target|command>... | harness guard-bash <root>"

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, usage)
		return 2
	}
	switch args[0] {
	case "hook-input":
		in, err := parseHookInput(stdin)
		if err == nil {
			var out string
			out, err = in.fields(args[1:])
			if err == nil {
				_, err = io.WriteString(stdout, out)
				if err == nil {
					return 0
				}
			}
		}
		fmt.Fprintf(stderr, "harness hook-input: %v\n", err)
		return 2
	case "guard-bash":
		if len(args) != 2 {
			break
		}
		err := guardBash(args[1], stdin)
		if err == nil {
			return 0
		}
		var b *blockError
		if !errors.As(err, &b) {
			err = blockf("%v", err) // fail closed
		}
		fmt.Fprintf(stderr, "guard-bash: blocked: %v\n", err)
		return 2
	}
	fmt.Fprintln(stderr, usage)
	return 2
}
