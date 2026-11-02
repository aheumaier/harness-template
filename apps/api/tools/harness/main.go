// Command harness holds the parts of the harness hooks and sensors that parse input (ADR 0012), plus the
// one-shot template rename, which needs the same portable file handling, and the harness logic ported from
// shell (ADR 0045). Make, the hook wrappers and the git hooks call it; it is built into .bin/harness by make
// (setup, check).
//
//	harness hook-input <field>...  Claude Code hook JSON on stdin → the fields, one per line
//	harness guard-bash <root>      PreToolUse guard for Bash commands (.claude/hooks/guard-bash.sh)
//	harness test-file <root> <path>
//	                               test-writer's write guard: path is a product test file inside root, as
//	                               specs-tested counts them (.claude/hooks/only-test-files.sh)
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
//	harness specs-tested <root>    specs-scenarios-tested (make specs-tested)
//	harness layout-check <root>    harness-layout-valid: scripts/harness/layout.env (make layout-check)
//	harness swift-resolved <root>  deps-swift-resolved: Package.resolved in git (make swift-resolved)
//	harness swift-tests <root> <dir>
//	                               ios-tests-pass: every Swift test file is in a declared test target, from
//	                               `swift package describe --type json` on stdin (make test-ios)
//	harness spec-refs <file>...    every $ref internal: contract-check.sh, upstream-drift.sh (ADR 0037)
//	harness check-commits <root>   git-conventional-commits: commits since the merge base, PR_TITLE (make
//	                               commits; env BASE_REF, HARNESS_ROOT, PR_TITLE)
//	harness check-commits --msg <file>
//	                               git-conventional-commits: one commit message (scripts/git-hooks/commit-msg)
//	harness integrity-check <root> harness-suppression-reason, harness-config-not-loosened (make
//	                               harness-integrity; env BASE_REF, HARNESS_ROOT, HARNESS_CHANGE_APPROVED)
//	harness rename [-write] <root> <name> <module|skip> <owner>
//	                               rename the template identity once for a new product (make rename)
//
// Exit 2 is a block (or unusable input) for Claude Code; the hooks pass it on. Sensors exit 1 on findings
// and 2 when they cannot run. A usage error (an unknown subcommand, a wrong argument) exits 64, which the
// wrappers and git hooks read as a stale binary (ADR 0045).
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

// exitUsage is the exit code of a usage error.
const exitUsage = 64

const usage = "usage: harness hook-input <cwd|tool_name|target|command>... | harness guard-bash <root> | harness test-file <root> <path> | harness check-links <root> | harness fixture-claims <root> | harness rule-lists <root> | harness orphans <root> | harness quality-rows [-no-clock] <root> | harness references-pinned <root> | harness approved-check <root> | harness specs-tested <root> | harness layout-check <root> | harness swift-resolved <root> | swift package describe --type json | harness swift-tests <root> <dir> | harness spec-refs <file>... | harness check-commits <root> | harness check-commits --msg <file> | harness integrity-check <root> | harness rename [-write] <root> <name> <module|skip> <owner>"

// rootSensors are the sensors run as `harness <name> <root>`: findings on stdout, whether there was any, and an
// error when they cannot run.
var rootSensors = map[string]func(root string, stdout io.Writer) (bool, error){
	"check-links":       checkLinks,
	"fixture-claims":    fixtureClaims,
	"rule-lists":        ruleLists,
	"orphans":           orphans,
	"references-pinned": referencesPinned,
	"approved-check":    approvedCheck,
	"specs-tested":      scenariosTested,
	"layout-check":      layoutCheck,
	"swift-resolved":    swiftResolved,
}

// printFinding prints one sensor finding as `<file>:<line>: <message> [<rule>]`, the line run.sh maps to the rule.
func printFinding(out io.Writer, file string, line int, rule, format string, a ...any) {
	fmt.Fprintf(out, "%s:%d: %s [%s]\n", file, line, fmt.Sprintf(format, a...), rule)
}

// harnessRoot is the tree a git-reading sensor checks: HARNESS_ROOT when set (CI runs the base branch's binary
// on the PR head, harness-integrity.yml), else root.
func harnessRoot(getenv func(string) string, root string) string {
	if r := getenv("HARNESS_ROOT"); r != "" {
		return r
	}
	return root
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return dispatch(args, os.Getenv, stdin, stdout, stderr)
}

// dispatch is run with the environment as a value, so a test can run a subcommand in-process with its own
// (testscript's Params.Cmds).
func dispatch(args []string, getenv func(string) string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, usage)
		return exitUsage
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
		if errors.Is(err, errUnknownField) {
			return exitUsage
		}
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
	case "test-file":
		if len(args) != 3 {
			break
		}
		if err := testFile(args[1], args[2]); err != nil {
			fmt.Fprintf(stderr, "harness test-file: %v\n", err)
			return 1
		}
		return 0
	case "check-links", "fixture-claims", "rule-lists", "orphans", "references-pinned", "approved-check", "specs-tested", "layout-check", "swift-resolved":
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
	case "swift-tests":
		if len(args) != 3 {
			break
		}
		bad, err := swiftTests(args[1], args[2], stdin, stdout)
		switch {
		case err != nil:
			fmt.Fprintf(stderr, "harness swift-tests: %v\n", err)
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
			return exitUsage
		}
		switch {
		case err != nil:
			fmt.Fprintf(stderr, "harness quality-rows: %v\n", err)
			return 2
		case bad:
			return 1
		}
		return 0
	case "spec-refs":
		if len(args) < 2 {
			break
		}
		bad, err := specRefs(args[1:], stdout)
		switch {
		case err != nil:
			fmt.Fprintf(stderr, "harness spec-refs: %v\n", err)
			return 2
		case bad:
			return 1
		}
		return 0
	case "check-commits":
		var bad bool
		var err error
		switch {
		case len(args) == 3 && args[1] == "--msg":
			bad, err = checkMessage(args[2], stdout)
		case len(args) == 2 && args[1] != "--msg":
			bad, err = checkCommits(harnessRoot(getenv, args[1]), getenv("BASE_REF"), getenv("PR_TITLE"), stdout)
		default:
			fmt.Fprintln(stderr, usage)
			return exitUsage
		}
		switch {
		case err != nil:
			fmt.Fprintf(stderr, "harness check-commits: %v\n", err)
			return 2
		case bad:
			return 1
		}
		return 0
	case "integrity-check":
		if len(args) != 2 {
			break
		}
		bad, err := integrityCheck(args[1], harnessRoot(getenv, args[1]), getenv("BASE_REF"), getenv("HARNESS_CHANGE_APPROVED") == "1", stdout)
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
	return exitUsage
}
