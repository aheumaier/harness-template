package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Conventional Commits (rules.md#git-conventional-commits): every commit subject is
// `<type>[(<scope>)][!]: <description>`, at most 100 characters.
const commitTypes = "build|chore|ci|docs|feat|fix|perf|refactor|revert|style|test"

var (
	conventionalRE = regexp.MustCompile(`^(` + commitTypes + `)(\([a-z0-9][a-z0-9._/-]*\))?!?: [^[:space:]]`)
	// gitOwnRE is git's own Merge/Revert/fixup!/squash!/amend! subjects, which the commit-msg hook lets through
	// and the range check catches if they reach a PR.
	gitOwnRE = regexp.MustCompile(`^(Merge |Revert "|fixup! |squash! |amend! )`)
)

// asciiBlank is what git's message cleanup treats as blank: a line of only U+00A0 survives into the commit and
// becomes part of its subject, so the hook must not skip it (the range check reads the subject git stored).
const asciiBlank = " \t"

// checkSubject prints a finding for subject, found at where, if it is not a Conventional Commit of at most max
// characters, and reports whether it did.
func checkSubject(out io.Writer, where, subject string, max int) bool {
	_, description, _ := strings.Cut(subject, ": ")
	if !conventionalRE.MatchString(subject) || strings.IndexFunc(description, visible) < 0 {
		fmt.Fprintf(out, "%s: '%s' is not '<type>(<scope>)!: <description>' with type one of %s and a lowercase scope [git-conventional-commits]\n",
			where, subject, strings.ReplaceAll(commitTypes, "|", ", "))
		return true
	}
	if n := utf8.RuneCountInString(subject); n > max {
		fmt.Fprintf(out, "%s: subject is %d characters, max %d [git-conventional-commits]\n", where, n, max)
		return true
	}
	return false
}

// visible is false for what shows nothing on its own: Unicode spaces (RE2's [:space:] is ASCII only), format and
// control characters such as U+200B, combining marks and fillers such as U+3164. A description of only those is
// empty.
func visible(r rune) bool {
	return unicode.IsGraphic(r) && !unicode.IsSpace(r) &&
		!unicode.In(r, unicode.Mn, unicode.Me, unicode.Other_Default_Ignorable_Code_Point)
}

func init() {
	commands.register(sensor("check-commits", "harness check-commits <root> | harness check-commits --msg <file>",
		"git-conventional-commits: commits since the merge base and PR_TITLE (make commits; env BASE_REF, HARNESS_ROOT, PR_TITLE), "+
			"or one commit message (scripts/git-hooks/commit-msg)",
		func(w world, args []string) (bool, error) {
			switch {
			case len(args) == 2 && args[0] == "--msg":
				return checkMessage(w, args[1])
			case len(args) == 1 && args[0] != "--msg":
				root, err := w.harnessRoot(args[0])
				if err != nil {
					return false, err
				}
				return checkCommits(w, root, w.getenv("BASE_REF"), w.getenv("PR_TITLE"))
			}
			return false, errUsage
		}))
}

// checkCommits is `make commits`: every non-merge commit in root since its merge base with baseRef, plus
// prTitle when set, at most 90 characters (GitHub's default squash commit is the title + " (#N)"). CI passes
// the PR head as root and runs the base branch's binary (harness-integrity.yml).
func checkCommits(w world, root, baseRef, prTitle string) (bool, error) {
	base, err := w.mergeBase(root, baseRef)
	if err != nil {
		return false, err
	}
	// %B, not %s: %s joins the lines of the first paragraph, so `feat(api):` + newline + `add orders` would pass.
	log, err := w.git(root, "log", "--no-merges", "-z", "--format=%h %B", base+"..HEAD")
	if err != nil {
		return false, err
	}
	bad := false
	for _, entry := range strings.Split(string(log), "\x00") {
		if sha, msg, _ := strings.Cut(entry, " "); sha != "" {
			subject, _, _ := strings.Cut(msg, "\n")
			bad = checkSubject(w.stdout, sha, subject, 100) || bad
		}
	}
	if prTitle != "" {
		bad = checkSubject(w.stdout, "PR title", prTitle, 90) || bad
	}
	return bad, nil
}

// checkMessage is the commit-msg hook's check of one commit message file: its first line that is not a
// comment and not blank.
func checkMessage(w world, file string) (bool, error) {
	p, err := w.abs(file)
	if err != nil {
		return false, err
	}
	msg, err := os.ReadFile(p)
	if err != nil {
		return false, err
	}
	subject := ""
	for _, line := range strings.Split(string(msg), "\n") {
		if line = strings.TrimSuffix(line, "\r"); !strings.HasPrefix(line, "#") && strings.Trim(line, asciiBlank) != "" {
			subject = line
			break
		}
	}
	if gitOwnRE.MatchString(subject) {
		return false, nil
	}
	return checkSubject(w.stdout, "commit message", subject, 100), nil
}
