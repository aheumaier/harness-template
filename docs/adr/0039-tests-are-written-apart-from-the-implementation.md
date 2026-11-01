# ADR 0039: Tests are written apart from the implementation, from the spec (amends 0019)

Status: accepted · 2026-10-30 · amends [ADR 0019](0019-inferential-sensors-are-advisory.md) · amended 2026-11-01 by [ADR 0042](0042-swift-unit-tests-in-make-check.md) (the `test-writer` writes Swift unit tests)

## Context
Gap G16 of exec plan 0003 (`docs/exec-plans/*/0003-harness-gap-closure.md`, slice P5.4), from Nandyala and
Pani, [Engineering the harness](https://www.thoughtworks.com/insights/blog/architecture/engineering-the-harness-a-practical-pattern-for-reliable-coding-agents)
(Thoughtworks, 2026-09-29): a RED agent writes the tests, a GREEN agent the code. Here one agent writes both, so
it can fit the tests to its own code; the weekly mutation report
([ADR 0023](0023-mutation-testing-is-a-drift-report.md)) finds weak tests only afterwards, and
`specs-scenarios-tested` ([ADR 0036](0036-acceptance-scenarios-are-traced-to-tests.md)) proves a test names
each scenario, not that it checks it. The plan made the agent earn its place: build two features with a
spec-only test writer and two the usual way, and keep it only with fewer surviving mutants or fewer missed
scenarios.

**The trial (2026-10-30).** Four pure Go model functions of ~40 lines, each with a hand-written spec of two
stories × three acceptance scenarios plus two edge-case bullets, a plan.md pinning package, names and
signatures, and the same tasks.md (tests first, ids named). Each feature was built one way only. All builders
were the same model, and the test-quality instruction was the same sentence in every prompt ("write tests
that would catch a wrong implementation: cover every acceptance scenario and every edge case in the spec").
Usual: one agent wrote tests and code. Test-first: a test writer got spec, plan and tasks and was told someone
else would implement from the same spec; it wrote only the test file, the tests were committed red (undefined
names), then a second agent implemented without touching them (`git diff <test commit> HEAD -- *_test.go`
empty for both). The test writers were ad-hoc prompts, not the agent file below. Mutation: `make mutation`
(gremlins, scope = the trial commits). Review: a blind read-only pass applying `code-reviewer.md`, also
judging per scenario whether its test checks it and naming bugs the tests would miss.

The trial ran on a scratch branch from 90e3e9f: specs in 2330357, test-first tests in 035b517 (001) and
9e4df61 (004), usual builds in c87d9a9 (002) and 142935e (003). The branch was deleted after the measurement,
so these commits are not retained; the numbers below are the record.

| Feature | Build | Test cases | Mutants | Survived | Ids untraced | Not checked (review) | Review findings | Bugs tests miss (review) |
|---|---|---:|---:|---:|---:|---:|---:|---:|
| 001 shipping rate | test-first | 31 | 17 | 0 | 0 | 0 | 0 | 0 |
| 004 loyalty points | test-first | 21 | 5 | 0 | 0 | 0 | 1 | 0 |
| 002 late fee | usual | 15 | 16 | 2 (both equivalent) | 0 | 0 | 1 | 0 |
| 003 invoice total | usual | 17 | 18 | 0 | 0 | 1 edge case (weak) | 2 | 3 |

The two survivors cannot be killed by any test (`amountCents*flatPercent` with `flatPercent = 1` becoming
`/`; `fee > limit` becoming `>=` where both return `limit`). Of the invoice's three missed bugs, one was
confirmed by a hand mutant: rounding VAT and discount per line instead of on the net passes all 17 usual-mode
tests, because every rounding case has one line and the two-line scenario sums to the same total by chance.
The other two (discount floored per line; the price check reading only the first line) the reviewer asserted
from the tests without a mutant.

## Decision
- **Keep, as `test-writer` (`.claude/agents/test-writer.md`), proposed until the re-measure below.**
  Mutation is a tie: 0 against 0 killable survivors, since both usual-mode survivors are equivalent. Scenario
  tracing is a tie: 0 untraced ids in both arms. The keep rests on the review, all of it in feature 003: one
  bug confirmed by a hand mutant, two the reviewer asserted, and one weakly checked edge case, against none
  for the test-first features. That is fewer missed scenarios (0 against 1), which meets the plan's rule.
- **Scope: unit tests that `make check` runs**, in Go `model`/`service` packages and `packages/domain`, which
  is all the trial covered. E2e journeys, `integration`-tagged tests, approval-test handler tests (their
  approved files come from the implementation and `make approve`, [ADR 0022](0022-approved-api-responses.md))
  and tests of an HTTP change before the contract and `make gen` (golden principle 1) stay with the
  implementer.
- **The flow:** the test writer gets `specs/<NNN-slug>/` and the test task ids, reads spec, plan, tasks and
  the area guides but no function body, writes only test files, and confirms red. The caller commits those
  tests with the trailer `Test-Writer: specs/<NNN-slug>` (one commit per story is fine); the implementer then
  makes them pass. A later change to them needs a spec change first, and code-reviewer check 7 reports it.
- **plan.md pins every new name** (package path, exported names, signatures); names that already exist are
  pinned by the current code. Without that the test writer stops and says what is missing: tests written
  against guessed names do not compile against the code.
- **Cost cap per run** (defaults, decided here): at most 4 test files, at most 3 `make check` runs, 30
  minutes of wall clock.
- **code-reviewer check 7** reports each assertion removed or loosened since the first `Test-Writer:` commit.
  It cannot run git, so the caller passes the diff of the test files, testdata, the spec and the test-runner
  config since that commit. Advisory, like check 6.
- **Amendment (2026-10-30, exec plan 0003 merge): accepted ahead of the re-measure.** The harness owner
  accepted this ADR before the first-adopter re-measure that "proposed until the re-measure" above waited for.
  The re-measure still runs ([TD-126](../exec-plans/tech-debt-tracker.md)); a reject now takes a new ADR that
  supersedes this one.

**Relationship to ADR 0019 (amended).** 0019 makes the inferential sensors advisory. `test-writer` is not one:
it is a builder, so it gets no rules.md entry, and check 7 is one more advisory check of the existing
code-reviewer sensor. The agent's write guard is computational: `.claude/hooks/only-test-files.sh` on
`Edit|Write|MultiEdit|NotebookEdit` asks `harness test-file` whether the path is a product test file, times
itself and fails closed like the other hooks; its shell is limited by `only-make-check.sh`. The predicate is
the one `specs-tested` reads tests with (`productTests` in `apps/api/tools/harness/testfile.go`, held to
both by the shared vectors in `testdata/test-files.txt`): a test file name under `PRODUCT_GLOBS` or
`WEB_GLOBS`, outside `node_modules/`, `fixtures/`, `API_CLIENT_DIR`, `GENERATED_GLOBS` and the harness files
of `scripts/harness/protected-paths.txt`. So the guard itself keeps the agent out of harness tests such as
`apps/api/tools/harness/*_test.go`, without relying on the session's `protect-harness` hook firing for
subagent calls. The exclusions ignore case, since on a case-insensitive disk `tools/Harness/` is
`tools/harness/`, and a path with a `..` part is refused before anything resolves it. It is wider than the
scope above (it allows `api` package tests and e2e files too); the agent's prompt narrows it. It reads no git
state, so a git-ignored test file passes it although `specs-tested` never sees one (TD-128).

## Alternatives considered
- **The test-quality guidance in `apps/api/internal/AGENTS.md` and code-reviewer check 4, for every builder,
  with no second agent.** Cheaper: no extra run or commit per feature. The trial cannot tell the two apart. Its
  test-quality instruction was identical in both arms, so the gap comes from the separation and the role
  framing, not from guidance one arm lacked. But the guidance now in `test-writer.md` ("several items, not
  only one, when it sums or applies a rule per item"; both sides of each threshold) was written after the
  invoice finding and was in no trial prompt. Whether that guidance alone, given to a single builder, would
  have caught the invoice bug is untested. Not chosen, because the plan asked for the agent, and code-reviewer
  check 7 needs a test commit that came from someone else. A re-measure that loses (TD-126) falls back to this
  alternative.

## Consequences
- The evidence is thin, and this is written down so nobody reads more into it. There were two features per
  arm and one model, and the trial's author wrote the specs. Each feature was built one way only, so a
  difference in difficulty could explain the whole gap: 003 sums over a list, and none of the others does.
  gremlins' five mutators make 5 to 18 mutants on a 40-line function and none of them is a per-item rounding
  change (TD-123). The test-first tables were larger (31 and 21 cases against 15 and 17). The trial covered
  pure functions only, hence the narrow scope above.
- A feature costs one more agent run and one more commit. The test writer cannot fix a red test caused by a
  spec gap; it can only report it as a spec question.
- Until plan-template.md has an interface section, many plans will not pin names, and the agent will often
  stop with `plan.md does not pin …`. That stop is intended: the alternative is guessing.
- Hand-off, for human harness PRs and not in this change (TD-124). ADR 0011 keeps the speckit skills and
  `.specify/templates/` vendored and unedited, so the wiring goes elsewhere. CLAUDE.md names `test-writer`
  for unit-test tasks, and `.specify/memory/constitution.md` says plan.md pins package, exported names and
  signatures for each test task. Until both land, only the agent's description makes anyone use it.
- Also open: CI's code-reviewer run does not get the test-commit diff, so check 7 is always skipped there
  (TD-125). The mutation report counts equivalent survivors as weak tests (TD-123). The re-measure on the
  first adopter decides whether it stays or a new ADR supersedes it (TD-126).
