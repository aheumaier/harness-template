# ADR 0024: Size and complexity limits are maintainability sensors, not style (amends core belief 3)

Status: proposed · 2026-10-29

## Context
Gap G5 of exec plan 0003 (`docs/exec-plans/*/0003-harness-gap-closure.md`, slice P4.1). The plan records
that both of its primary sources name the same four limits for agent-written code: function length, file
length, complexity and parameter count. Its gap table cites OpenAI's
[Harness engineering](https://openai.com/index/harness-engineering/) for them as taste invariants, and
Böckeler's follow-up to [Harness engineering for coding agent users](https://martinfowler.com/articles/harness-engineering.html),
*Maintainability sensors for coding agents*, for them as maintainability sensors.

The repo has none of them. golangci-lint runs `standard` plus the rules in `apps/api/.golangci.yml`;
ESLint has no `max-lines`, `max-lines-per-function`, `complexity` or `max-params`. The reason is
[core belief 3](../design-docs/core-beliefs.md#3-enforce-invariants-not-implementations), whose consequences
read "no style rules beyond formatters; new rules are earned by real mistakes, not anticipated".
[ADR 0015](0015-anticipated-rules-and-nfr-classes-b-o.md) adopted five anticipated rules as
`team-convention` and said that size and complexity limits need the belief amended, not that ADR. Plan 0003's
Q4 deferred sloglint's key-case check for the same reason.

What the limits would bound today, measured at `1b33d8e` with the linters named below, at a limit of 1 so
every function reports its value. Generated code (`*.gen.go`, `internal/*/store/`,
`packages/api-client/src/gen/`) is excluded, as golangci (`exclusions.generated`) and `eslint.config.mjs`
already exclude it.

| Measure | Linter | Worst, non-test | Worst, test |
|---------|--------|-----------------|-------------|
| Go statements per function | `funlen` | 35, `internal/platform/telemetry/telemetry.go:91` | 40, `internal/platform/approvaltest/approvaltest_test.go:66` |
| Go cognitive complexity | `gocognit` | 13, `approvaltest.go:65` and `telemetry.go:91` | 46, `cmd/api/main_test.go:98` |
| Go parameters per function | `revive` `argument-limit` | 4, `approvaltest.go:65` and `internal/platform/problem/problem.go:25` | 4, `approvaltest_test.go:54` |
| Go lines per file | `revive` `file-length-limit` | 205, `approvaltest.go` | 196, `internal/platform/telemetry/telemetry_test.go` |
| TS lines per function | `max-lines-per-function` | 15, `apps/web/src/App.tsx:3` | 9, `App.test.tsx:7` |
| TS cyclomatic complexity | `complexity` | 5, `App.tsx:3` and `packages/api-client/src/index.ts:12` | 1 |
| TS parameters per function | `max-params` | 1, `index.ts:12` | 0 |
| TS lines per file | `max-lines` | 62, `eslint.config.mjs` | 15, `App.test.tsx` |

Go paths are relative to `apps/api`. Two facts shape the decision:

- **The Go numbers are real; the TS numbers are a scaffold.** `apps/web` is one 17-line component and one
  test. A limit of one parameter or 15 lines would fail the next ordinary component.
- **The tests' worst values are far above production's.** Table-driven tests and `t.Run` closures add nesting:
  `main_test.go:98` scores 46 against production's 13. One threshold at the test maximum would let
  production code grow 3.5 times worse before the sensor fired.

## Decision
- **Amend core belief 3.** Function length, file length, complexity and parameter count are
  **maintainability sensors**, not style. Each bounds how much code an agent with no memory of yesterday
  must hold in one read to change it safely ([core belief 1](../design-docs/core-beliefs.md#1-agent-legibility-first)).
  A formatter cannot fix them and a reviewer judges them inconsistently. The invariant is "no unit of code
  outgrows what one read can hold"; how to split it stays open. Naming style (sloglint's key case, identifier
  case) is not part of this amendment and stays outside the rules.
- **Adopted as `team-convention`**, with this ADR in place of an incident, like the rules in ADR 0015. P4.2
  adds them.
- **One linter per measure.** P4.2 uses these, so each threshold uses the same linter and counting as the
  table:
  - Go function length: `funlen` counts **statements**, and its line check is off. A statement count does not
    change with formatting or long literals.
  - Go complexity: `gocognit`, not `cyclop`. Cognitive complexity penalises nesting, which is what makes a
    function hard to read.
  - Go parameters: `revive` `argument-limit`. A variadic parameter counts as one.
  - Go file length: `revive` `file-length-limit`, counting every line (comments and blanks included, as
    `wc -l` does). No custom analyzer is needed.
  - `revive` is not enabled today. Enabled without a `rules:` list it turns on its default rules
    (`exported`, `var-naming`, `package-comments`, …), which are the naming style this ADR keeps out. So
    revive runs with only these two rules.
  - TS: ESLint `max-lines-per-function`, `complexity`, `max-params` and `max-lines`, all counting every line
    (the ESLint defaults for `skipBlankLines` and `skipComments`).
  - No new tool, so [ADR 0016](0016-tool-supply-chain.md) does not apply: `funlen`, `gocognit` and `revive`
    ship in the pinned golangci-lint, and the four TS rules are ESLint core.
- **Scope is what the sensors already lint.** Go: the `apps/api` module that `make lint-go` runs golangci on
  (`./...` in `apps/api`), so not the separate `tools/analyzers` and `tools/harness` modules. TS: what
  `make lint-web` runs ESLint on. Generated code is excluded.
- **Tests are exempt from the three per-function limits** (`*_test.go`; `*.test.ts`, `*.test.tsx`), and
  stay under the file-length limit.
  - Go: golangci cannot give one linter a second threshold by path, and a shared one would be set by the
    tests. `funlen` and `gocognit` are excluded for `_test\.go` by path. `revive` carries both a per-function
    and a per-file rule, so its test exemption is a text-matched `exclusions.rules` entry, in the form of the
    existing `std log package outside platform` entry: `path: _test\.go`, `linters: [revive]`,
    `text: "argument-limit"`. `file-length-limit` stays on for tests.
  - TS: ESLint can set per-glob thresholds, but TS follows Go so the repo has one policy.
- **Starting thresholds: today's worst; for TS, the linter's documented default while the code is a
  scaffold.** Nothing existing fails either way.
  - Go measures at P4.2's base commit, with the linters and scope above. The three per-function limits start
    at the non-test worst. File length starts at the worst over all linted files, tests included, because
    tests are under that limit and a lane could make a test the longest file before P4.2 lands (today
    `approvaltest.go` 205, `telemetry_test.go` 196). Other lanes add code before P4.2, so the values in the
    table (`funlen` statements 35, `gocognit` 13, `argument-limit` 4, `file-length-limit` 205) are the
    evidence for the method, not the final numbers.
  - TS starts at ESLint's own defaults for the pinned ESLint 10.13: `max-lines-per-function` 50,
    `complexity` 20, `max-params` 3, `max-lines` 300. A threshold comes from a measurement or a documented
    external number, never from taste; for a scaffold, as `apps/web` and `packages/*` are today, the
    linter's documented default is the external number used. The measured worst (1 parameter, 15 lines)
    would not bound the next change but block it: because thresholds are never raised (below), the next
    ordinary component could pass only with a suppression. This exception applies only to the TS code; it
    does not raise any Go threshold.
- **Thresholds only go down.** Each one is lowered once the code that set it is split. Lowering is a harness
  PR that needs a human's approval like any change to a `config` path: `apps/api/.golangci.yml` and
  `eslint.config.mjs` are `config`, and [harness-config-not-loosened](../harness/rules.md#harness-config-not-loosened)
  counts tightening too. Raising a threshold is forbidden by this ADR; only an ADR that supersedes it can
  lift that. A unit that truly cannot be split takes a suppression with `reason: … (rule: <id>)` (golden
  principle 4), so the exception stays local and visible.
- **Every finding tells the agent how to split, not only that the code is too long.** The linters' own
  messages ("too many statements (36 > 35)") do not, and golangci cannot rewrite them. `run.sh` already
  prints the rules.md remediation of each rule id found in the output, or of the only rule it was given. So
  P4.2 gives each measure its own rule id, wires it so that `run.sh` prints that measure's recipe and no
  other, and writes these recipes as the remediations. golangci prints only the linter name, so that wiring
  needs a linter-to-rule-id map in `run.sh` or one golangci run per measure. Both touch protected `config`
  paths (`scripts/harness/*`, the Makefile), so the wiring is itself a harness change under P4.2's human
  gate. The recipes:
  - **Function length:** extract each step the function performs into a named helper in the same file, so
    the function reads as a list of calls. Keep a helper next to its caller.
  - **Complexity:** return early for error and edge cases instead of nesting `else`; move each branch body of
    a long `if`/`switch` into a function named after the case; replace a flag parameter with two functions.
  - **Parameters:** group parameters that travel together into a struct (Go) or an object parameter (TS), or
    make them fields of the receiver they belong to. Use options structs or functional options for optional
    values.
  - **File length:** split the file by responsibility into sibling files in the same package or directory
    (`handlers.go`, `handlers_export.go`), with no new package and no change to the import graph. In TS, move a
    component's helpers into their own module next to it.

## Consequences
- Core belief 3 keeps "no style rules beyond formatters", and links this ADR for the four measures it now
  counts as invariants. Any other anticipated rule still needs an incident, a finding accepted twice
  ([ADR 0019](0019-inferential-sensors-are-advisory.md)) or its own ADR.
- P4.2 lands the rules as `Status: team-convention` linking this ADR, each with a fixture
  ([ADR 0009](0009-harness-selftest.md)) that adds one function or line past its threshold.
- The Go thresholds are set by platform code (`approvaltest`, `telemetry`), not by a domain. The first
  domain function that crosses one is split, not waved through.
- `approvaltest.go` sets the `argument-limit` and `file-length-limit` values and is protected
  (`apps/api/internal/platform/approvaltest/*` in `scripts/harness/protected-paths.txt`). Lowering either
  threshold means splitting a protected file, so it is a harness PR too.
- Plan 0003 Q4's deferral of sloglint's key-case check is resolved: not adopted, since naming style stays
  out. P4.2, which edits `apps/api/.golangci.yml` anyway, rewrites the stale comment there ("no key-case
  check yet (plan 0003, P4.1)", line 36) to point at this ADR.
- The TS thresholds are loose until `apps/web` grows. Each later harness PR lowers them toward the measured
  worst; nothing lowers them automatically.
- Test code is bounded only by file length. If long or deeply nested tests become a finding class seen twice,
  ADR 0019's path turns it into a rule with its own threshold.
- The harness's own Go code (`tools/harness`, `tools/analyzers`) is not linted by golangci today and is out
  of scope here. Its values are higher (`tools/harness/main.go:32`: 85 statements, cognitive complexity 42).
  Bringing it under these limits is its own change with its own starting numbers.
