package main

import (
	"bytes"
	"slices"
	"strings"
	"testing"
)

const (
	skillFile = ".claude/skills/review-x/SKILL.md"
	agentFile = ".claude/agents/reuse-reviewer.md"
)

var (
	listRules = []string{
		"# Harness rules", "## Go (apps/api)", "### go-layers", "### go-dead-code",
		"## Duplication (apps/api, apps/web)", "### repo-no-clones", "### web-no-clones",
	}
	listAgent = []string{"## Skip", "- `go-layers`", "## Method"}
	// noneRequired requires nothing of either file.
	noneRequired = map[string][]string{skillFile: nil, agentFile: nil}
)

func TestRuleLists(t *testing.T) {
	tests := []struct {
		name     string
		skill    []string
		required []string // requiredSkips for skillFile
		want     []string // output lines; none means the check passes
	}{
		{"ids and a whole section", []string{
			"# Review", "`not-skipped` outside Skip is free text", "## Skip",
			"- Sensors: `go-layers`, every rule under `## Duplication`: `repo-no-clones`, `web-no-clones`.",
			"- Not ids: `dupl`, `jscpd`, `*.gen.go`, `scripts/harness/fixtures/`, `.specify/`, TD-52, `make check`.",
			"### A subsection stays in Skip: `go-dead-code`",
			"## Look for", "- `free-text` again",
		}, []string{"go-layers", "## Duplication", "go-dead-code"}, nil},
		{"code spans that wrap a line or use double backticks", []string{
			"## Skip", "- `go-layers`, every rule under `##", "  Duplication`: `repo-no-clones`,",
			"  ``web-no-clones``, `` `go-layer` ``", "## Look for",
		}, []string{"## Duplication"}, []string{
			skillFile + ":4: `go-layer` is no rule id in docs/harness/rules.md: use the id of its `### ` heading, or drop it [harness-rule-lists-match]",
		}},
		{"a level-1 heading ends Skip", []string{
			"## Skip", "- `go-layers`", "# Appendix", "- `not-a-rule`",
		}, nil, nil},
		{"an id rules.md does not define", []string{
			"## Skip", "- `go-layers`, `go-layer`", "## Look for",
		}, nil, []string{skillFile + ":2: `go-layer` is no rule id in docs/harness/rules.md: use the id of its `### ` heading, or drop it [harness-rule-lists-match]"}},
		{"a claimed section misses a rule", []string{
			"## Skip", "- `## Duplication`: `repo-no-clones`", "## Look for",
		}, nil, []string{skillFile + ":2: skips `## Duplication` but does not name its rule `web-no-clones`: add it to the Skip list [harness-rule-lists-match]"}},
		{"a section rules.md does not have", []string{
			"## Skip", "- `## Duplication (apps/api, apps/web)`", "## Look for",
		}, nil, []string{skillFile + ":2: `## Duplication (apps/api, apps/web)` is no section of docs/harness/rules.md: name a `## ` heading's text before any ` (` [harness-rule-lists-match]"}},
		// The Phase 3 gate's incident: review-modularity did not skip rules a sensor gates.
		{"a required id or claim dropped", []string{
			"# Review", "## Skip", "- `go-layers`, `repo-no-clones`, `web-no-clones`", "## Look for",
		}, []string{"go-layers", "go-dead-code", "## Duplication"}, []string{
			skillFile + ":2: the Skip list must keep `go-dead-code` (requiredSkips in rulelists.go): add it back, or remove it there in a harness PR that says why the reviewer reports it again [harness-rule-lists-match]",
			skillFile + ":2: the Skip list must keep `## Duplication` (requiredSkips in rulelists.go): add it back, or remove it there in a harness PR that says why the reviewer reports it again [harness-rule-lists-match]",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := linkRepo(t, map[string][]string{rulesFile: listRules, skillFile: tt.skill, agentFile: listAgent})
			var out bytes.Buffer
			bad, err := checkRuleLists(testWorld(t, &out), root, map[string][]string{skillFile: tt.required, agentFile: nil})
			if err != nil {
				t.Fatal(err)
			}
			if want := strings.Join(tt.want, "\n"); strings.TrimSpace(out.String()) != want || bad != (len(tt.want) > 0) {
				t.Errorf("bad=%v, output:\n%s\nwant:\n%s", bad, out.String(), strings.Join(tt.want, "\n"))
			}
		})
	}
}

func TestRuleListsFailsClosed(t *testing.T) {
	both := map[string][]string{rulesFile: listRules, skillFile: {"## Skip"}, agentFile: listAgent}
	tests := []struct {
		name     string
		files    map[string][]string
		required map[string][]string
		want     string
	}{
		{"no rules.md", map[string][]string{skillFile: {"## Skip"}, agentFile: {"## Skip"}}, noneRequired, "rules.md"},
		{"no rules", map[string][]string{rulesFile: {"## Go"}, skillFile: {"## Skip"}, agentFile: {"## Skip"}}, noneRequired, "no `### <rule-id>` sections"},
		{"a repeated rule id", map[string][]string{rulesFile: append(slices.Clone(listRules), "### go-layers"), skillFile: {"## Skip"}, agentFile: listAgent}, noneRequired, "a second `### go-layers` heading"},
		{"no agent file", map[string][]string{rulesFile: listRules, skillFile: {"## Skip"}, ".claude/agents/other.md": {""}}, noneRequired, "no git-known file matches .claude/agents/reuse-reviewer.md"},
		{"no Skip section", map[string][]string{rulesFile: listRules, skillFile: {"## Skip"}, agentFile: {"## Method", "# Skip"}}, noneRequired, "has no `## Skip` section"},
		{"a file without required entries", both, map[string][]string{agentFile: nil}, skillFile + " has no entry in requiredSkips"},
		{"required entries for a missing file", both, map[string][]string{skillFile: nil, agentFile: nil, ".claude/skills/review-gone/SKILL.md": nil}, "names .claude/skills/review-gone/SKILL.md"},
		{"a required id rules.md lost", both, map[string][]string{skillFile: {"go-layer"}, agentFile: nil}, "lists `go-layer` for " + skillFile},
		{"a required section rules.md lost", both, map[string][]string{skillFile: nil, agentFile: {"## Shell"}}, "lists `## Shell` for " + agentFile},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			if _, err := checkRuleLists(testWorld(t, &out), linkRepo(t, tt.files), tt.required); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want it to contain %q", err, tt.want)
			}
		})
	}
}
