# ADR 0046: Agent verdicts stay advisory and are measured: the app evaluator, agent evals, the assumption ledger (amends 0019, 0032, 0039)

Status: proposed · 2026-11-24 · amends [ADR 0019](0019-inferential-sensors-are-advisory.md),
[ADR 0032](0032-rules-that-never-fire-are-a-drift-report.md),
[ADR 0039](0039-tests-are-written-apart-from-the-implementation.md)

The work is planned in [exec plan 0006](../exec-plans/active/0006-anthropic-harness-gaps.md), slices D0, V1–V4,
E1–E3 and M1–M4. This ADR stays `proposed` until the trial below has run. Its result is added here as a dated
amendment, and then the ADR is accepted, whether the result is keep or drop: a drop is recorded, and the
decisions on evals and the ledger stand. On a drop the decisions about the app evaluator and its journeys,
and the amendments that rest on them, take no effect. On acceptance the notes on ADRs 0019, 0032 and 0039
lose their "proposed:" and keep only what took effect; if the ADR is withdrawn instead, they are removed.
Slices E1–E3 and M1–M4 start after acceptance.

## Context
Plan 0006 compares the template with Anthropic's writing on harnesses and finds three gaps that ask the same
question: what does an agent's output count for, and how do we know an agent still earns its place?

- **A1.** No agent runs the app. `code-reviewer` reads the diff, and `make e2e`
  ([ADR 0025](0025-browser-e2e-is-the-nfr-b2-sensor.md)) runs only journeys someone already wrote. The builder
  may use the `drive-app` skill, but then it grades its own work. Anthropic's
  [Harness design for long-running application development](https://www.anthropic.com/engineering/harness-design-long-running-apps)
  reports that agents "reliably skew positive when grading their own work", and that an evaluator which
  clicks through the running app finds what tests and a diff read miss. It also says: "Out of the box,
  Claude is a poor QA agent."
- **A3.** ADR 0019 says of inferential sensors "No fixture can prove one fires" and stops there. Nothing
  measures how often a reviewer finds what it should, or how much its verdict changes from run to run.
  ADR 0039's trial was run once and its branch deleted.
- **A4.** [ADR 0032](0032-rules-that-never-fire-are-a-drift-report.md) asks of every computational rule
  whether it still earns its place. Agents, skills, hooks and loop prompts have no such record, although
  each one exists because of something a model could not do when it was added.

**The trial (designed 2026-11-24, not run yet).** The owner ruled on 2026-11-24 that the trial runs on a
real product, `strength_planner`, not on scratch features in the template. Six cases, each one or two user
stories of a spec'd web feature, are judged at a pinned commit in a separate local clone. Four get two
hand-planted defects each, of the kind a user would see and the product's own checks (Go tests, lint, unit
tests, type check, the existing Playwright journeys) do not catch. Two stay clean. Each case is judged twice
by `code-reviewer`, from the diff, and twice by an evaluator that gets the case's user stories and their
acceptance scenarios, the running app and a browser, and neither the source, the git history nor an account
of what was built. The evaluator prints one line per acceptance scenario: `PASS`, `FAIL` or `NOT EXERCISED`,
each with evidence, and a journey for each `FAIL`. The plan's V1 section holds the full design. The result
goes into the section Trial result below; the clone and its commits are not kept by this repository, so the
numbers in this ADR are the record.

## Trial result
Not run yet (2026-11-24). The result table, the cost per judged case, the wall time and the keep-or-drop
decision go here as a dated amendment.

## Decision
- **The app evaluator is an inferential sensor, and its verdict is advisory** (ADR 0019). It never gates
  `make check`, `make check-all` or a merge. It is a local workflow step, as the reviewers are.
- **A finding binds through its journey, not through the verdict.** The evaluator reports a `FAIL` together
  with a Playwright journey that shows it. The journey cites the scenario id it covers
  ([ADR 0036](0036-acceptance-scenarios-are-traced-to-tests.md)). The evaluator leaves it uncommitted. When
  the builder accepts the finding (ADR 0019's meaning: the tree changes because of it), the builder commits
  the journey with the fix, quoting the `FAIL` line and its scenario id in the commit, and from then on
  `make e2e` holds the behaviour, which is computational. A finding the builder rejects has its journey
  deleted, with the reason in the report. A journey is never committed red and never before the finding is
  accepted, so a wrong verdict blocks no commit and no merge. Until it is accepted or deleted, the
  uncommitted journey makes `make e2e`, and so `make check-all`, red in that working tree: the builder
  decides on each finding before running them.
- **Keep or drop, decided by the trial** (defaults, decided here):
  - Before the run, each case lists the scenario ids a user can reach in the browser. Only those count.
  - A **hit** is a `FAIL` on the scenario a planted defect breaks, with evidence, in both runs. For
    `code-reviewer`, a hit is a finding that names the defect's file and behaviour, in both runs.
    `NOT EXERCISED` on the scenario a planted defect breaks is a miss; on a counted scenario of a clean case
    it counts as a `FAIL`. Any other `FAIL` or `NOT EXERCISED` on a planted case is reported apart, with its
    count, and counts for neither side.
  - **Keep** when the evaluator hits at least two planted defects that `code-reviewer` did not hit, and in
    each run fails at most one counted scenario across the two clean cases. A `FAIL` on a clean case is a
    found bug, reported apart and counting for neither side, only when its journey reproduces it on the
    pinned commit and the product's owner confirms that the journey expects what the scenario says.
    Otherwise it counts as a `FAIL`.
  - **Otherwise** the prompt is tuned at most twice and the trial rerun. If it still misses the bar, the
    agent is dropped and the numbers are recorded here. `code-reviewer` then gains "drive the scenarios in
    the app" only if that variant passes the same bar.
  - If kept, the agent is `.claude/agents/app-evaluator.md`, confined by hooks, with a cost cap per run set
    from the trial's measured cost (ADR 0019 asks the slice that adds an agent for one).
- **Agent evals are a drift report**, like coverage ([ADR 0020](0020-test-coverage-is-a-trend.md)) and
  mutation testing ([ADR 0023](0023-mutation-testing-is-a-drift-report.md)). `make agent-evals` runs each
  agent on a small set of cases and reports, per agent, the cases hit in every run, in some and in none, the
  findings on clean cases, the verdicts that flip between runs, and the cost. It needs credentials, the
  network and money, so it runs in neither `make check` nor `make check-all` and never gates a PR. Defaults,
  decided here: about ten cases per agent, at least three of them clean; two runs per case; grading by
  matching the agent's output lines, with no LLM judge.
- **Eval cases live with the code they judge.** The template ships the runner, the case format and cases
  that apply to the template's own tree. A product's cases stay in the product, and the runner accepts a
  case folder outside the template. The six trial cases therefore belong to `strength_planner`; whether it
  keeps them is its owner's call. The template is public and does not copy a product's specs or source.
- **Every agent, non-vendored skill, hook and loop prompt has a row in an assumption ledger**
  (`docs/harness/assumptions.md`): the model weakness it assumes, the evidence it was added on, when it was
  last tested (model, date, result) and how to test its removal. The sensor `harness-assumptions-listed`
  checks that every such file has a row and every row names a file that exists. It is adopted as
  `team-convention` under [ADR 0015](0015-anticipated-rules-and-nfr-classes-b-o.md); this ADR is its
  decision record. The weekly drift report lists rows last tested on another model than the CI agents run
  on, or more than 90 days ago (ADR 0032's window). As in ADR 0032, a person decides and removal is a human
  PR, one component at a time.

### What this changes in the ADRs it amends
- **ADR 0019, "Their output is one of three things".** A fourth is added for the app evaluator: an
  uncommitted journey file in `apps/web/e2e/`, in addition to its report.
- **ADR 0019, the local reviewers whose accepted finding is quoted in the fixing commit.** `app-evaluator` is
  added; its finding's class is the scenario id.
- **ADR 0019, "A finding class accepted twice becomes a computational rule".** An accepted evaluator finding
  becomes a test at once, with no second acceptance. A journey is a test of one feature, not a rule for all
  code, so it does not need the two-acceptances guard against one-off rules.
- **ADR 0019, "No fixture can prove one fires".** It stays true. An eval does not prove an agent fires; it
  measures how often it does.
- **ADR 0032.** Its question is extended from computational rules to agents, skills, hooks and loop prompts.
  Its report, ledger and artifacts are unchanged; the assumption ledger is a second, hand-kept file.
- **ADR 0039, "E2e journeys … stay with the implementer".** The evaluator may also write a journey, to prove
  a failure. `only-test-files.sh` allows e2e files and every other product test file; the slice that adds the
  agent narrows it to journeys.
- **ADR 0039, the trial as the way an agent earns its place.** It becomes the rule for every new agent: a
  keep-or-drop bar written before the run, and the cases kept, by the repository that owns the code they
  judge, so the measurement can be repeated. For this trial that is the product's call.

Not decided here: the done gate at stop (plan 0006 S2) and the frozen-tests rule (T1). Each gets its own ADR
with its slice, so that neither waits for this trial. Whether an evaluator journey is frozen like a
`test-writer` test is T1's question.

## Consequences
- Until the trial has run, nothing of this is built: no agent file, no hook change, no wiring in
  `CLAUDE.md`. Slices V2–V4 start only after this ADR is accepted with a keep.
- The trial judges the evaluator on another stack than the template's. `strength_planner` has no
  `make dev`, no `make e2e` and none of this repo's hooks or skills, so the evaluator runs from an
  ad-hoc prompt and `code-reviewer` from its agent file outside this repo's hooks, with a recorded table of
  what replaces what. V2's first step checks its wiring on the template itself.
- The evidence will be thin, as ADR 0039's was: one model, defects chosen by the trial's author, and six
  cases from only four specs, because most of the product's specs describe API calls. Three cases come from
  one spec, and two specs have about two counted scenarios each. At least half of the planted defects come
  from classes that the Harness design post or
  [Effective harnesses for long-running agents](https://www.anthropic.com/engineering/effective-harnesses-for-long-running-agents)
  report, and the bar is relative to `code-reviewer` on the same cases, to limit how far the choice can flatter the
  evaluator.
- An evaluator pass costs an agent run with a browser per UI change. The Harness design post's full harness
  cost about 20 times its solo run; if the measured cost is near that, the agent is not a default step even
  if kept.
- A journey added for each accepted finding makes `make e2e` slower over time (the `e2e` row of
  `scripts/harness/budgets.txt`, under NFR-H7).
- `make agent-evals` costs money per run and is run by hand, and weekly for the read-only agents once the
  CI agent runs are enabled (TD-38). A PR that changes an agent file states the eval rows before and after.
- The ledger is one more hand-kept file. Its sensor checks that rows exist, not that they are true.
- Phase 3 of plan 0006 does not get its cases from Phase 1: the template's corpus starts with
  `code-reviewer` cases, and `app-evaluator` cases are a product's own.

## Alternatives considered
- **Make the evaluator's verdict a gate.** The Harness design post uses a hard threshold per criterion.
  Rejected: an LLM verdict changes with model, prompt and sampling (ADR 0019, NFR-R1). The hard threshold is
  kept inside the report (any `FAIL` or `NOT EXERCISED` makes the verdict `CHANGES REQUESTED`), and what
  binds is the journey.
- **Leave it to the builder and the `drive-app` skill.** No new agent and no extra run. Rejected as the
  default because it is self-evaluation; it is the fallback if the trial drops the agent.
- **Trial on scratch features in the template**, as the plan first said. Cheaper to set up and on the right
  stack, but the template has no business domain and four toy features say little about a real app. The
  owner ruled for `strength_planner`.
- **An LLM judge for the evals.** It grades loose output without a format change. Rejected: it adds a second
  unmeasured agent to measure the first. A reviewer whose lines cannot be matched gets a stricter output
  format instead.
- **One ADR for all of plan 0006**, including the stop gate and the frozen-tests rule. Rejected: this ADR
  waits for a trial, and those two slices do not depend on it.
