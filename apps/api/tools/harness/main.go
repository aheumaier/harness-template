// Command harness holds the parts of the harness hooks and sensors that parse input (ADR 0012). The shell
// scripts stay the entry points and call it; it is built into .bin/harness by make (setup, check).
//
//	harness hook-input <field>...  Claude Code hook JSON on stdin → the fields, one per line
//	harness guard-bash <root>      PreToolUse guard for Bash commands (.claude/hooks/guard-bash.sh)
//	harness check-links <root>     docs-links-resolve, docs-claude-imports-agents (make docs-links)
//	harness fixture-claims <root>  rules-fixture-claims (make fixture-claims)
//	harness integrity-check <root> harness-suppression-reason, harness-config-not-loosened (make
//	                               harness-integrity; env BASE_REF, HARNESS_ROOT, HARNESS_CHANGE_APPROVED)
//
// Exit 2 is a block (or unusable input) for Claude Code; the hooks pass it on. Sensors exit 1 on findings
// and 2 when they cannot run.
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

const usage = "usage: harness hook-input <cwd|tool_name|target|command>... | harness guard-bash <root> | harness check-links <root> | harness fixture-claims <root> | harness integrity-check <root>"

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
	case "check-links":
		if len(args) != 2 {
			break
		}
		bad, err := checkLinks(args[1], stdout)
		switch {
		case err != nil:
			fmt.Fprintf(stderr, "harness check-links: %v\n", err)
			return 2
		case bad:
			return 1
		}
		return 0
	case "fixture-claims":
		if len(args) != 2 {
			break
		}
		bad, err := fixtureClaims(args[1], stdout)
		switch {
		case err != nil:
			fmt.Fprintf(stderr, "harness fixture-claims: %v\n", err)
			return 2
		case bad:
			return 1
		}
		return 0
	case "integrity-check":
		if len(args) != 2 {
			break
		}
		root := os.Getenv("HARNESS_ROOT")
		if root == "" {
			root = args[1]
		}
		bad, err := integrityCheck(args[1], root, os.Getenv("BASE_REF"), os.Getenv("HARNESS_CHANGE_APPROVED") == "1", stdout)
		switch {
		case err != nil:
			fmt.Fprintf(stderr, "harness integrity-check: %v\n", err)
			return 2
		case bad:
			return 1
		}
		return 0
	}
	fmt.Fprintln(stderr, usage)
	return 2
}
