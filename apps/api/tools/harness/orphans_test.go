package main

import (
	"bytes"
	"fmt"
	"io"
	"runtime"
	"strings"
	"testing"
)

// orphanScript and orphanTarget are the findings orphans prints.
func orphanScript(s string) string {
	return s + ":1: no Makefile target, workflow, hook, skill, agent, guide or named script names this file; wire it up or delete it [repo-no-orphan-scripts]"
}

func orphanTarget(line int, name string) string {
	return fmt.Sprintf("Makefile:%d: make target %s has no '##' help comment and nothing calls it; document it for make help, call it or delete it [repo-no-orphan-scripts]", line, name)
}

func TestOrphans(t *testing.T) {
	t.Parallel()
	const ok = "ok: ## An entry point needs no caller"
	tests := []struct {
		name  string
		files map[string][]string
		want  []string // output lines; none means the check passes
	}{
		{"scripts named by Makefile, workflow, action, hook, setting, skill, agent and guide", map[string][]string{
			"Makefile":                         {ok, "\t@scripts/a.sh"},
			".github/workflows/ci.yml":         {"run: ./scripts/b.sh --flag"},
			".github/actions/x/action.yml":     {"run: scripts/f.sh"},
			".claude/hooks/fast.sh":            {`"$ROOT/scripts/c.sh"`},
			".claude/settings.json":            {`{"command": "scripts/g.sh"}`},
			".claude/skills/x/SKILL.md":        {"Run `scripts/d.sh`."},
			".claude/agents/r.md":              {"Run scripts/h.sh."},
			"docs/guide.md":                    {"See scripts/e.sh."},
			"scripts/a.sh":                     {"echo"},
			"scripts/b.sh":                     {"echo"},
			"scripts/c.sh":                     {"echo"},
			"scripts/d.sh":                     {"echo"},
			"scripts/e.sh":                     {"echo"},
			"scripts/f.sh":                     {"echo"},
			"scripts/g.sh":                     {"echo"},
			"scripts/h.sh":                     {"echo"},
			"scripts/templates/x/y.go":         {"package y"},
			"scripts/harness/fixtures/f/a.txt": {"x"},
		}, nil},
		{"a / before a path counts only after ., .. or a variable expansion", map[string][]string{
			"Makefile": {ok, "\t@$(CURDIR)/scripts/p1.sh; $$root/scripts/p2.sh", "\t@apps/scripts/n1.sh; .specify/scripts/n2.sh"},
			".claude/hooks/x.sh": {
				`"$ROOT/scripts/p3.sh"`, `"${ROOT}/scripts/p4.sh"`, `"$root"/scripts/p5.sh`, "./scripts/p6.sh", "../../scripts/p7.sh",
				"${{ github.workspace }}/scripts/p8.sh",
			},
			"docs/guide.md": {"https://github.com/o/r/blob/main/scripts/n3.sh"},
			"scripts/p1.sh": {"echo"}, "scripts/p2.sh": {"echo"}, "scripts/p3.sh": {"echo"}, "scripts/p4.sh": {"echo"},
			"scripts/p5.sh": {"echo"}, "scripts/p6.sh": {"echo"}, "scripts/p7.sh": {"echo"}, "scripts/p8.sh": {"echo"},
			"scripts/n1.sh": {"echo"}, "scripts/n2.sh": {"echo"}, "scripts/n3.sh": {"echo"},
		}, []string{orphanScript("scripts/n1.sh"), orphanScript("scripts/n2.sh"), orphanScript("scripts/n3.sh")}},
		{"a named script names others; tests and data files do not", map[string][]string{
			"Makefile":                     {ok, "\t@scripts/run.sh", "test-sh: ## Bats", "\tbats scripts/test"},
			"scripts/run.sh":               {`. "$ROOT/scripts/lib.sh"`, "cat scripts/list.txt"},
			"scripts/lib.sh":               {"echo"},
			"scripts/list.txt":             {"scripts/data-only.sh"},
			"scripts/data-only.sh":         {"echo"},
			"scripts/test/a.bats":          {"run scripts/test-only.sh"},
			"scripts/test/helpers.bash":    {"x"},
			"scripts/test-only.sh":         {"echo"},
			"scripts/git-hooks/commit-msg": {"x"},
			"scripts/setup.sh":             {"git config core.hooksPath scripts/git-hooks"},
			"docs/setup.md":                {"Run scripts/setup.sh."},
		}, []string{orphanScript("scripts/data-only.sh"), orphanScript("scripts/test-only.sh")}},
		{"only runner directories, named in code, cover their files", map[string][]string{
			"Makefile": {
				"##@ Shell sensors (scripts/, scripts/tools/, scripts/test)",
				"lint: ## Lint every file in scripts/tools",
				"\tshellcheck 'scripts/tools/*' # and scripts/test",
				"\tcd scripts/tools && ./x.sh; cd scripts && ./top.sh",
			},
			".github/workflows/ci.yml": {`run: die "the scripts of x"`},
			"README.md":                {"Hooks live in scripts/git-hooks."},
			"scripts/top.sh":           {"echo"},
			"scripts/tools/x.sh":       {"echo"},
			"scripts/test/a.bats":      {"x"},
			"scripts/git-hooks/pre":    {"x"},
		}, []string{
			orphanScript("scripts/git-hooks/pre"), orphanScript("scripts/test/a.bats"),
			orphanScript("scripts/tools/x.sh"), orphanScript("scripts/top.sh"),
		}},
		{"a commented-out caller keeps nothing alive; a guide's whole text counts", map[string][]string{
			"Makefile":                {ok, "#\t@scripts/old.sh", "\t@echo # scripts/old.sh"},
			".claude/hooks/x.sh":      {`# "$ROOT/scripts/old.sh"`},
			".github/workflows/x.yml": {"# run: scripts/old.sh"},
			"docs/guide.md":           {"<!-- scripts/doc.sh -->"},
			"scripts/old.sh":          {"echo"},
			"scripts/doc.sh":          {"echo"},
		}, []string{orphanScript("scripts/old.sh")}},
		{"names inside longer names, history, fixtures and self-references do not count", map[string][]string{
			"Makefile":                                {ok, "\t@scripts/gen.sh.bak; scripts/gen.shx; myscripts/gen.sh-old"},
			"docs/adr/0001-x.md":                      {"scripts/gen.sh"},
			"docs/exec-plans/active/0001.md":          {"scripts/gen.sh"},
			"specs/001-x/spec.md":                     {"scripts/gen.sh"},
			"scripts/harness/fixtures/f/apply.sh":     {"scripts/gen.sh"},
			"scripts/harness/fixtures/f/fixture.conf": {"TARGET=gen"},
			"scripts/gen.sh":                          {"# scripts/gen.sh"},
		}, []string{orphanScript("scripts/gen.sh")}},
		{"make targets: help comments, prerequisites, recipes and other callers", map[string][]string{
			"Makefile": {
				"API := apps/api",
				".PHONY: check lint-go test-go gen-x ci-only docs-only skip-me",
				"check: lint-go ##Everything (no space after ## still lists it in make help)",
				"\t@$(MAKE) test-go",
				"lint-go:",
				"\t@echo lint",
				"test-go:",
				"\t@echo test",
				"gen-x:",
				"\t@echo gen",
				"ci-only:",
				"\t@echo ci",
				"docs-only:",
				"\t@echo docs",
				"$(API)/bin:",
				"\t@mkdir -p $@",
				"debug: FLAGS = -v",
				"script-only:",
				"\t@echo s",
				"dead-call:",
				"\t@echo d",
			},
			".github/workflows/ci.yml": {"run: make ci-only && scripts/ci.sh"},
			"docs/guide.md":            {"Run `make docs-only`."},
			"docs/other.md":            {"scripts/gen-x.sh is not gen-x/ either"},
			"scripts/ci.sh":            {"make script-only"},
			"scripts/dead.sh":          {"make dead-call"},
		}, []string{
			orphanScript("scripts/dead.sh"),
			orphanTarget(9, "gen-x"), orphanTarget(17, "debug"), orphanTarget(20, "dead-call"),
		}},
		{"make help lists one name with no blank before the colon", map[string][]string{
			"Makefile": {ok, "a b: ## Two names", "name : ## A blank before the colon", "ab-c: ## Listed"},
		}, []string{orphanTarget(2, "a"), orphanTarget(2, "b"), orphanTarget(3, "name")}},
		{"a target-specific variable line is not a call", map[string][]string{
			"Makefile": {
				"check-all: ## Everything",
				"check-all: export HARNESS_SCOPE := full",
				"foo: VAR = lint-x",
				"bar: override MODE ?= lint-x",
				"lint-x:",
				"\t@echo lint",
				"run: ## Runs foo and bar",
				"\t@$(MAKE) foo bar",
			},
		}, []string{orphanTarget(5, "lint-x")}},
		{"a target named only in its own rule, a comment or .PHONY is uncalled", map[string][]string{
			"Makefile": {
				ok,
				"# lonely is old",
				".PHONY: lonely",
				"lonely: ; @echo lonely",
				"\t@echo still lonely",
				"other: ## Its help text names lonely, which is prose",
			},
		}, []string{orphanTarget(4, "lonely")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var out bytes.Buffer
			bad, err := orphans(testWorld(t, &out), linkRepo(t, tt.files), newRegistry()) // help needs no caller
			if err != nil {
				t.Fatal(err)
			}
			got := strings.Split(strings.TrimSpace(out.String()), "\n")
			if out.Len() == 0 {
				got = nil
			}
			if bad != (len(tt.want) > 0) || strings.Join(got, "\n") != strings.Join(tt.want, "\n") {
				t.Errorf("bad=%v, output:\n%s\nwant:\n%s", bad, out.String(), strings.Join(tt.want, "\n"))
			}
		})
	}
}

func TestOrphansFailsClosed(t *testing.T) {
	t.Parallel()
	if _, err := orphans(testWorld(t, io.Discard), linkRepo(t, map[string][]string{"scripts/a.sh": {"echo"}}), registry{}); err == nil {
		t.Error("no Makefile: want an error")
	}
	if _, err := orphans(testWorld(t, io.Discard), t.TempDir(), registry{}); err == nil {
		t.Error("not a git repo: want an error")
	}
	// An orphan subcommand needs the layout to say where it lives.
	reg := registry{"lonely": {name: "lonely", file: "lonely.go", line: 1}}
	if _, err := orphans(testWorld(t, io.Discard), linkRepo(t, map[string][]string{"Makefile": {"ok: ## x"}}), reg); err == nil {
		t.Error("orphan subcommand, no layout.env: want an error")
	}
}

func TestOrphanSubcommands(t *testing.T) {
	t.Parallel()
	reg := registry{}
	for i, name := range []string{"mk-call", "env-call", "hook-call", "bare-call", "var-call", "brace-call", "script-call",
		"guide-only", "comment-only", "test-only", "prefix-only"} {
		reg[name] = command{name: name, file: name + ".go", line: i + 1}
	}
	root := linkRepo(t, map[string][]string{
		layoutFile: withLayout("API_DIR", "svc"),
		"Makefile": {
			"ok: ## Runs them", "\t@$(HARNESS) mk-call .", "\t@env X=1 $(HARNESS) env-call", "\t@scripts/s.sh",
			"\t@$(HARNESS) prefix-only_x", "\tbats scripts/test",
		},
		".claude/hooks/h.sh": {
			`"$ROOT/.bin/harness" hook-call "$ROOT"`, `x="$(.bin/harness bare-call target)"`, "# .bin/harness comment-only",
		},
		"scripts/s.sh":        {`"$HARNESS" var-call x`, "${HARNESS} brace-call", `"$ROOT/.bin/harness" script-call`},
		"scripts/test/x.bats": {"run .bin/harness test-only"},
		"docs/guide.md":       {"Run `.bin/harness guide-only`.", "Edit scripts/harness/layout.env."},
	})
	finding := func(name string, line int) string {
		return fmt.Sprintf("svc/tools/harness/%s.go:%d: harness subcommand %s has no caller: no Makefile target, workflow, hook or named script runs it as $(HARNESS) %s or .bin/harness %s; call it or delete it [repo-no-orphan-subcommands]", name, line, name, name, name)
	}
	wantFindings(t, func(out io.Writer) (bool, error) { return orphans(testWorld(t, out), root, reg) }, []string{
		finding("comment-only", 9), finding("guide-only", 8), finding("prefix-only", 11), finding("test-only", 10),
	})
}

func TestRegisterRecordsCaller(t *testing.T) {
	t.Parallel()
	r := registry{}
	r.register(command{name: "x"}) // the line below must stay next to this one
	_, _, line, _ := runtime.Caller(0)
	if c := r["x"]; c.file != "orphans_test.go" || c.line != line-1 {
		t.Errorf("registered at %s:%d, want orphans_test.go:%d", c.file, c.line, line-1)
	}
}
