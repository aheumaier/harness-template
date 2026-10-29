// Command harness holds the parts of the harness hooks and sensors that parse input (ADR 0012), plus the
// one-shot template rename, which needs the same portable file handling. The shell scripts and make stay
// the entry points and call it; it is built into .bin/harness by make (setup, check).
//
//	harness hook-input <field>...  Claude Code hook JSON on stdin → the fields, one per line
//	harness guard-bash <root>      PreToolUse guard for Bash commands (.claude/hooks/guard-bash.sh)
//	harness check-links <root>     docs-links-resolve, docs-claude-imports-agents (make docs-links)
//	harness fixture-claims <root>  rules-fixture-claims (make fixture-claims)
//	harness rule-lists <root>      harness-rule-lists-match: skip lists in review agents (make rule-lists)
//	harness orphans <root>         repo-no-orphan-scripts (make orphans, check-all)
//	harness quality-rows -no-clock <root>
//	                               docs-quality-rows-well-formed, docs-quality-rows-match-domains
//	                               (make docs-quality-shape, check-all)
//	harness quality-rows <root>    docs-quality-row-fresh against today's UTC date (make docs-quality,
//	                               drift only: it reads the clock)
//	harness references-pinned <root>
//	                               docs-references-pinned (make references-pinned)
//	harness approved-check <root>  api-approved-match-contract (make approved-check)
//	harness layout-check <root>    harness-layout-valid: scripts/harness/layout.env (make layout-check)
//	harness integrity-check <root> harness-suppression-reason, harness-config-not-loosened (make
//	                               harness-integrity; env BASE_REF, HARNESS_ROOT, HARNESS_CHANGE_APPROVED)
//	harness rename [-write] <root> <name> <module|skip> <owner>
//	                               rename the template identity once for a new product (make rename)
//
// Exit 2 is a block (or unusable input) for Claude Code; the hooks pass it on. Sensors exit 1 on findings
// and 2 when they cannot run.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

const usage = "usage: harness hook-input <cwd|tool_name|target|command>... | harness guard-bash <root> | harness check-links <root> | harness fixture-claims <root> | harness rule-lists <root> | harness orphans <root> | harness quality-rows [-no-clock] <root> | harness references-pinned <root> | harness approved-check <root> | harness layout-check <root> | harness integrity-check <root> | harness rename [-write] <root> <name> <module|skip> <owner>"

// rootSensors are the sensors run as `harness <name> <root>`: findings on stdout, whether there was any, and an
// error when they cannot run.
var rootSensors = map[string]func(root string, stdout io.Writer) (bool, error){
	"check-links":       checkLinks,
	"fixture-claims":    fixtureClaims,
	"rule-lists":        ruleLists,
	"orphans":           orphans,
	"references-pinned": referencesPinned,
	"approved-check":    approvedCheck,
	"layout-check":      layoutCheck,
}

// printFinding prints one sensor finding as `<file>:<line>: <message> [<rule>]`, the line run.sh maps to the rule.
func printFinding(out io.Writer, file string, line int, rule, format string, a ...any) {
	fmt.Fprintf(out, "%s:%d: %s [%s]\n", file, line, fmt.Sprintf(format, a...), rule)
}

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
	case "check-links", "fixture-claims", "rule-lists", "orphans", "references-pinned", "approved-check", "layout-check":
		if len(args) != 2 {
			break
		}
		bad, err := rootSensors[args[0]](args[1], stdout)
		switch {
		case err != nil:
			fmt.Fprintf(stderr, "harness %s: %v\n", args[0], err)
			return 2
		case bad:
			return 1
		}
		return 0
	case "quality-rows":
		var bad bool
		var err error
		switch {
		case len(args) == 3 && args[1] == "-no-clock":
			bad, err = qualityShape(args[2], stdout)
		case len(args) == 2:
			bad, err = qualityFresh(args[1], time.Now(), stdout)
		default:
			fmt.Fprintln(stderr, usage)
			return 2
		}
		switch {
		case err != nil:
			fmt.Fprintf(stderr, "harness quality-rows: %v\n", err)
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
	case "rename":
		write := len(args) > 1 && args[1] == "-write"
		if write {
			args = args[1:]
		}
		if len(args) != 5 {
			break
		}
		if err := renameRepo(args[1], args[2], args[3], args[4], write, stdout); err != nil {
			fmt.Fprintf(stderr, "harness rename: %v\n", err)
			return 2
		}
		return 0
	}
	fmt.Fprintln(stderr, usage)
	return 2
}
