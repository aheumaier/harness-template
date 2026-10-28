package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// guard-bash blocks force pushes, pushes to main, recursive rm outside the repo and commands that name a
// .env file. It backs up the permissions.deny patterns in .claude/settings.json, which only match command
// text. The command is parsed as bash (quoting, grouping, substitutions, heredocs) and never run or
// expanded. It errs towards blocking:
//   - an rm -r path that needs expansion ($x, `…`, ~, .*) is refused;
//   - a string argument is also read as commands after its first `;&|`, newline, backquote or `$(`, as the
//     awk tokenizer this replaces did (`echo "x; rm -rf /"` is blocked);
//   - the script of `bash|sh|zsh -c` and the arguments of `eval` are parsed and checked in turn;
//   - a command that does not parse as bash is blocked (fail closed).
//
// Heredoc bodies are data (commit messages, file contents), not commands, except for the substitutions
// in an unquoted one, which run.

type blockError struct{ msg string }

func (e *blockError) Error() string { return e.msg }

func blockf(format string, a ...any) error { return &blockError{fmt.Sprintf(format, a...)} }

// maxDepth bounds nested `bash -c` / `eval` strings.
const maxDepth = 4

var (
	envFile    = regexp.MustCompile(`(^|/)\.env(\.[A-Za-z0-9_.-]+)?$`)
	assignment = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)
	recursive  = regexp.MustCompile(`^-[a-zA-Z]*[rR]`)
	mainBranch = regexp.MustCompile(`^(refs/heads/)?(main|master)$`)
	shellC     = regexp.MustCompile(`^-[a-zA-Z]*c[a-zA-Z]*$`) // -c, -lc, -ec
	// Words that only prefix the real command (`sudo rm`, `xargs rm`); their options are skipped too.
	prefixWords = set("if", "then", "else", "elif", "do", "while", "until", "time", "sudo", "env", "command",
		"exec", "nohup", "nice", "xargs", "!")
	shells = set("bash", "sh", "zsh")
)

func set(words ...string) map[string]bool {
	m := make(map[string]bool, len(words))
	for _, w := range words {
		m[w] = true
	}
	return m
}

func guardBash(root string, stdin io.Reader) error {
	in, err := parseHookInput(stdin)
	if err != nil {
		return blockf("%v", err)
	}
	if in.ToolInput.Command == "" {
		return nil
	}
	g := &guard{root: root, cwd: physicalDir(in.Cwd)}
	return g.check(in.ToolInput.Command, 0)
}

// physicalDir is dir with symlinks resolved, like `cd dir && pwd -P`; the working directory if dir is unusable.
func physicalDir(dir string) string {
	if dir != "" {
		if p, err := filepath.EvalSymlinks(dir); err == nil {
			if st, err := os.Stat(p); err == nil && st.IsDir() {
				return p
			}
		}
	}
	wd, _ := os.Getwd()
	if p, err := filepath.EvalSymlinks(wd); err == nil {
		return p
	}
	return wd
}

type guard struct {
	root, cwd string
	branch    *string // current branch of cwd, looked up on the first git push
}

// word is one shell word: text with quotes removed and escapes resolved; static when nothing in it
// needs expansion; raw as typed (for messages).
type word struct {
	text, raw string
	static    bool
}

// check parses src as bash and checks every simple command and word in it.
func (g *guard) check(src string, depth int) error {
	if depth > maxDepth {
		return blockf("commands nested more than %d levels deep (bash -c, eval). Run them directly.", maxDepth)
	}
	f, err := syntax.NewParser(syntax.Variant(syntax.LangBash)).Parse(strings.NewReader(src), "")
	if err != nil {
		return blockf("cannot parse the command as bash (%v). Run simpler commands, one per call.", err)
	}
	hdocs := map[*syntax.Word]bool{}
	syntax.Walk(f, func(n syntax.Node) bool {
		if r, ok := n.(*syntax.Redirect); ok && r.Hdoc != nil {
			hdocs[r.Hdoc] = true
		}
		return true
	})
	var found error
	syntax.Walk(f, func(n syntax.Node) bool {
		if found != nil {
			return false // Walk still visits the siblings of a node that returned false
		}
		switch n := n.(type) {
		case *syntax.Word:
			if !hdocs[n] {
				found = checkEnv(wordOf(n).text)
			}
		case *syntax.CallExpr:
			ws := make([]word, len(n.Args))
			for i, a := range n.Args {
				ws[i] = wordOf(a)
			}
			found = g.command(ws, depth)
			for _, w := range ws {
				for _, seg := range embedded(w.text) {
					if found == nil {
						found = g.command(seg, depth)
					}
				}
			}
		}
		return found == nil
	})
	return found
}

// command checks one simple command.
func (g *guard) command(ws []word, depth int) error {
	ws = skipPrefix(ws)
	if len(ws) == 0 {
		return nil
	}
	switch name := ws[0].text; {
	case path.Base(name) == "git":
		return g.gitPush(ws)
	case name == "rm" || strings.HasSuffix(name, "/rm"):
		return g.rm(ws)
	case shells[path.Base(name)]:
		for i := 1; i+1 < len(ws); i++ {
			if shellC.MatchString(ws[i].text) && ws[i+1].static {
				return g.check(ws[i+1].text, depth+1)
			}
		}
	case name == "eval":
		src := make([]string, 0, len(ws)-1)
		for _, w := range ws[1:] {
			src = append(src, w.text)
		}
		return g.check(strings.Join(src, " "), depth+1)
	}
	return nil
}

// skipPrefix drops env assignments (FOO=1 git push), prefix words and the options of a prefix word.
func skipPrefix(ws []word) []word {
	prefixed := false
	for i, w := range ws {
		switch {
		case w.text == "" || assignment.MatchString(w.text):
		case prefixWords[w.text]:
			prefixed = true
		case !prefixed || !strings.HasPrefix(w.text, "-"):
			return ws[i:]
		}
	}
	return nil
}

// gitPush checks `git [-C dir] [-c k=v] push …`.
func (g *guard) gitPush(ws []word) error {
	j := 1
	for j < len(ws) && strings.HasPrefix(ws[j].text, "-") {
		if ws[j].text == "-C" || ws[j].text == "-c" {
			j++
		}
		j++
	}
	if j >= len(ws) || ws[j].text != "push" {
		return nil
	}
	var positional []string
	for _, w := range ws[j+1:] {
		a := w.text
		switch {
		case a == "--force" || strings.HasPrefix(a, "--force-with-lease") || a == "--force-if-includes" ||
			a == "--mirror" || a == "--delete":
			return blockf("force/destructive push (%s). Push a normal branch and open a PR.", a)
		case strings.HasPrefix(a, "--"):
		case strings.HasPrefix(a, "-") && strings.ContainsAny(a[1:], "fd"):
			return blockf("force/destructive push (%s). Push a normal branch and open a PR.", a)
		case strings.HasPrefix(a, "-"):
		case strings.HasPrefix(a, "+"):
			return blockf("force refspec (%s). Push a normal branch and open a PR.", a)
		default:
			positional = append(positional, a)
		}
	}
	if len(positional) > 1 {
		for _, ref := range positional[1:] {
			dst := ref[strings.LastIndex(ref, ":")+1:]
			if dst == "HEAD" || dst == "@" {
				dst = g.currentBranch()
			}
			if mainBranch.MatchString(dst) {
				return blockf("push to %s. Push a feature branch and open a PR.", dst)
			}
		}
		return nil
	}
	// No refspec: pushes the current branch.
	if b := g.currentBranch(); b == "main" || b == "master" {
		return blockf("`git push` from %s pushes main. Create a branch first.", b)
	}
	return nil
}

func (g *guard) currentBranch() string {
	if g.branch == nil {
		out, _ := exec.Command("git", "-C", g.cwd, "symbolic-ref", "--short", "-q", "HEAD").Output()
		b := strings.TrimSpace(string(out))
		g.branch = &b
	}
	return *g.branch
}

// rm checks that every path of a recursive rm is a literal path inside the repo.
func (g *guard) rm(ws []word) error {
	rec := false
	for _, w := range ws[1:] {
		rec = rec || recursive.MatchString(w.text) || w.text == "--recursive"
	}
	if !rec {
		return nil
	}
	paths := 0
	for _, w := range ws[1:] {
		a := w.text
		if a == "" || strings.HasPrefix(a, "-") {
			continue
		}
		paths++
		if !w.static || strings.HasPrefix(a, "~") || strings.Contains(a, ".*") {
			return blockf("rm -r with an unexpanded path (%s). Use a literal path inside the repo.", w.raw)
		}
		abs := a
		if !strings.HasPrefix(a, "/") {
			abs = g.cwd + "/" + a
		}
		norm := path.Clean(abs) // lexically: the target may not exist
		if !strings.HasPrefix(norm, g.root+"/") {
			return blockf("rm -r outside the repository (%s → %s). Only delete paths inside %s.", a, norm, g.root)
		}
	}
	if paths == 0 {
		return blockf("rm -r without a literal path (paths from xargs or stdin). Name the paths inside the repo.")
	}
	return nil
}

// checkEnv blocks a word that names a .env file (other than .env.example), also inside a string.
func checkEnv(text string) error {
	for _, seg := range segments(text) {
		for _, a := range seg {
			a = a[strings.LastIndexAny(a, "<>")+1:]
			if envFile.MatchString(a) && !strings.HasSuffix(a, ".env.example") {
				return blockf("%s: .env files hold this worktree's secrets; never read, copy or print them. "+
					"Use `make` targets, which load them.", a)
			}
		}
	}
	return nil
}

// embedded returns the commands a string would hold after its first separator, for the strictness of the
// earlier tokenizer: "x; rm -rf /" → [rm -rf /]. A field with `$` or a backquote is not static.
func embedded(text string) [][]word {
	segs := segments(text)
	if len(segs) < 2 {
		return nil
	}
	out := make([][]word, 0, len(segs)-1)
	for _, seg := range segs[1:] {
		ws := make([]word, len(seg))
		for i, f := range seg {
			ws[i] = word{text: f, raw: f, static: !strings.ContainsAny(f, "$`")}
		}
		out = append(out, ws)
	}
	return out
}

var separator = regexp.MustCompile("[;&|\n`]|\\$\\(")

// segments splits text on `;&|`, newlines, backquotes and `$(`, then each part into whitespace-separated
// fields without leading quotes and grouping (`"`, `'`, `\`, `(`, `{`, backquote, `$(`) or trailing ones.
func segments(text string) [][]string {
	parts := separator.Split(text, -1)
	out := make([][]string, len(parts))
	for i, p := range parts {
		for _, f := range strings.Fields(p) {
			for len(f) > 0 && (strings.ContainsRune("\"'\\({`", rune(f[0])) || strings.HasPrefix(f, "$(")) {
				if strings.HasPrefix(f, "$(") {
					f = f[2:]
				} else {
					f = f[1:]
				}
			}
			f = strings.TrimRight(f, "\"')}`")
			out[i] = append(out[i], f)
		}
	}
	return out
}

// wordOf returns w's text as bash would see it without expansion. A part that needs expansion keeps its
// source text ($HOME, ${x}, $((1+1))), a command substitution becomes `$(…)` (its commands are checked on
// their own, and its heredocs must not be read as commands), and the word is not static.
func wordOf(w *syntax.Word) word {
	var b strings.Builder
	static := parts(&b, w.Parts, false)
	return word{text: b.String(), raw: source(w), static: static}
}

func parts(b *strings.Builder, ps []syntax.WordPart, quoted bool) bool {
	static := true
	for _, p := range ps {
		switch p := p.(type) {
		case *syntax.Lit:
			b.WriteString(unescape(p.Value, quoted))
		case *syntax.SglQuoted:
			b.WriteString(p.Value)
			static = static && !p.Dollar // $'…' has escapes of its own
		case *syntax.DblQuoted:
			static = parts(b, p.Parts, true) && static
		case *syntax.CmdSubst, *syntax.ProcSubst:
			b.WriteString("$(…)")
			static = false
		default:
			b.WriteString(source(p))
			static = false
		}
	}
	return static
}

// unescape drops the backslash of an escaped character (in double quotes only before $ ` " \) and joins
// line continuations.
func unescape(s string, quoted bool) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			next := s[i+1]
			if next == '\n' {
				i++
				continue
			}
			if !quoted || strings.IndexByte("$`\"\\", next) >= 0 {
				i++
				b.WriteByte(next)
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func source(n syntax.Node) string {
	var b strings.Builder
	if err := syntax.NewPrinter().Print(&b, n); err != nil {
		return "?"
	}
	return b.String()
}
