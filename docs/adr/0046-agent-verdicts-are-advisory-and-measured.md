# ADR 0046: Agent verdicts stay advisory and are measured: the app evaluator, agent evals, the assumption ledger (amends 0019, 0032, 0039)

Status: proposed · 2026-11-24 · amends [ADR 0019](0019-inferential-sensors-are-advisory.md),
[ADR 0032](0032-rules-that-never-fire-are-a-drift-report.md),
[ADR 0039](0039-tests-are-written-apart-from-the-implementation.md)

The work is planned in [exec plan 0006](../exec-plans/active/0006-anthropic-harness-gaps.md), slices D0, V1–V4,
E1–E3 and M1–M4 (V5, added 2026-11-26, gets its own ADR). This ADR stays `proposed` until the trial below has run. Its result is added here as a dated
amendment, and then the ADR is accepted, whether the result is keep or drop: a drop is recorded, and the
decisions on evals and the ledger stand. On a drop the decisions about the app evaluator and its journeys,
and the amendments that rest on them, take no effect. On acceptance the notes on ADRs 0019, 0032 and 0039
lose their "proposed:" and keep only what took effect; if the ADR is withdrawn instead, they are removed.
Slices E1–E3 and M1–M4 start after acceptance. (2026-11-26: the trial missed the bar, which by this ADR
means tune or drop. The owner overrode that rule and did neither: the decisions about the app evaluator and
its journeys stand, in the changed form of Decision, "Outcome of the trial". The trial did not show that the
agent is worth keeping; the keep rests on the owner's judgement.)

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

**The trial (designed 2026-11-24, run 2026-11-26: see Trial result).** The owner ruled on 2026-11-24 that the trial runs on a
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
**2026-11-26, round 1 of 1: the keep bar is missed.** Both arms ran on `claude-opus-5-5` at effort high,
on `strength_planner` at the pinned commit `3b0a312`, one run at a time, each on an empty database.

| Defect | Class (plan 0006 V1) | Evaluator, run 1 / 2 | `code-reviewer`, run 1 / 2 |
|---|---|---|---|
| 002-a: renaming the programme title renames its first mesocycle | control wired to the wrong operation | hit / hit | hit / hit |
| 002-b: renaming a row in the session list sends the wrong id, the row reverts | control wired to the wrong operation | hit / hit | hit / hit |
| 003-a: every week lists all sessions of the mesocycle | list ignores its filter | hit / hit | hit / hit |
| 003-b: Create Microcycle is disabled once a week exists, with no duration set | renders but does nothing | hit / hit | hit / hit |
| 019 s1-a: editing a ticked set's load stores the value from before the edit | value lost on reload | hit / hit | hit / hit |
| 019 s1-b: unticking a set leaves it saved | renders but does nothing | hit / **miss** | hit / **miss** |
| 019 s2-a: finishing stores unticked rows too | list ignores its filter (closest) | hit / hit | hit / hit |
| 019 s2-b: a load typed without a target is stored as the reps | value lost on reload (closest) | hit / hit | hit / hit |

- **Hits in both runs:** evaluator 7 of 8, `code-reviewer` 7 of 8. Both missed 019 s1-b in run 2. So the
  evaluator hit **no** defect that `code-reviewer` missed; the bar needs at least two.
- **Clean cases:** the evaluator failed 0 of 6 counted scenarios in each run (019 story 3: four; 004: two).
  `code-reviewer` is not scored on clean cases.
- **Reported apart** (Decision, "Keep or drop": counts for neither side). In both runs, on 002, `FAIL` on
  three more scenarios: the API sends no `ETag` header, a stale rename gets 409 instead of 412, an athlete
  can open the rename box. In run 1 also: on 019 story 1, the session detail page bug that the scenario
  check had found before the run (it shows "— kg" and raw exercise ids); on 002, `NOT EXERCISED` on two
  scenarios it could not reach through the page (a missing name, a failing rename); on 003 and 019 story 1,
  `NOT EXERCISED` on one scenario each that names no step in the page and was not counted. The run-2 log
  repeats the three 002 `FAIL`s and records the rest only where they were hits. The 002 findings describe
  the pinned commit, not a planted defect, and the product's owner has not confirmed them. `code-reviewer`
  named two of them from the code, the missing `ETag` in one run and the athlete's rename box. Found by the
  evaluator alone, and new: the 409.
- **Cost** is the `total_cost_usd` estimate that Claude Code reports per run (list prices). Evaluator: mean
  1.28 USD (0.32 to 2.30), mean 285 s wall time. `code-reviewer`: mean 1.06 USD (0.09 to 2.47, growing with
  the diff), mean 124 s. Round 28.15 USD in all, the pilot (1.28 USD) not included. The plan's guess was 5 to
  10 USD per evaluator run.
- **Where the run differed from the design.**
  - The evaluator could not write files: the trial's settings denied Write. It put each journey's code in
    its report instead. No score depends on the file.
  - Besides the diff, `code-reviewer` got the case's user stories with their acceptance scenarios, and the
    statement that the product's checks are green on the tree. They were proven green on the pinned commit
    with the defects, not on the older feature commits its diffs end at; the journeys' baseline on the pin
    had three failures, none in a case.
  - The two 019 story cases diff the live session's five commits, because two defects do not apply to its
    first commit alone.
  - One reviewer run read nothing because the trial wiring was broken (0.07 USD). One was stopped by a
    timeout; it left no cost record. Both were rerun, with the prompts unchanged.
- **Reading.** The bar was fixed before the run, and the result stands as measured. Two things in the design
  favoured `code-reviewer`. Every planted defect sat in a few changed lines of the diff. And the reviewer saw
  the scenarios, which a CI review of a PR does not.

**Outcome (owner's ruling, 2026-11-26).** The owner was asked when the bar could no longer be met (at 19.03
USD) and chose to finish the round for full data. Then the owner overrode this ADR's rule for a missed bar:
no tuning round and no drop, and the evaluator is not kept as a step every UI change gets. The reasons are
the owner's judgement, not the trial's result: the case design could not tell the two arms apart (Reading),
and three 002 scenarios were green in the product's checks yet failed in the running app (unconfirmed;
the evaluator alone found one of them, the 409). That is what a fixed test against the running app
catches. So the weight moves from the agent's verdict to those tests: Decision,
"Outcome of the trial", below. The case material goes to the product's owner and is not kept here.

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
  decides on each finding before running them. Until then it also counts as a test for its scenario (the
  scenario check reads untracked files), so the builder reruns `make check` after deleting one.
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
- **Outcome of the trial (2026-11-26, owner's override).** The bar was missed (Trial result). The owner
  set aside "Otherwise" above and replaced it, and "If kept", with this:
  - **No tuning round.** `code-reviewer` does not gain "drive the scenarios in the app". That variant was
    never run, and the scenario tests below are meant to cover the same ground with a fixed check.
  - **The app evaluator explores; it does not gate.** It becomes `.claude/agents/app-evaluator.md`,
    advisory, and its findings still bind only through an accepted journey, as above. It is run on demand,
    about once per feature that changes what the web app shows, not before every change is called done. The
    cost cap per run is 3 USD (the highest measured run was 2.30 USD).
  - **It was kept without passing a trial**, the one exception to "ADR 0039, the trial as the way an agent
    earns its place" below. What the trial does show: 7 of 8 defects hit in both runs, as `code-reviewer`
    did, and 0 false `FAIL`s in 12 verdicts on clean scenarios. From then on it is measured like every agent,
    by Phase 3's evals on cases the product owns, and by its row in the assumption ledger. That row names a
    review date and what removes the agent; the owner sets both on acceptance, since the product may not keep
    the cases.
  - **Direction: scenarios a user sees are tested against the running app.** A scenario about what the web
    app shows, or what the API returns to it, would be proven by a journey or an API test against the running
    server, written by `test-writer` from the spec before the implementation. Whether and how a unit test
    stops counting for such a scenario is decided by V5's own ADR, amending ADR 0036 and ADR 0039, not here. (2026-11-26, proposed:
    [ADR 0049](0049-user-seen-scenarios-are-traced-by-tests-that-run-the-app.md) answers it: a journey, or an integration test in the domain's `api` package, unless the scenario ends with `(unit: <reason>)`.)
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
  judge, so the measurement can be repeated. For this trial that is the product's call. (2026-11-26: the
  app evaluator itself is the one exception, by the owner's override in Decision, "Outcome of the trial".
  The rule holds for every other new agent.)

Not decided here: the done gate at stop (plan 0006 S2) and the frozen-tests rule (T1). Each gets its own ADR
with its slice, so that neither waits for this trial. Whether an evaluator journey is frozen like a
`test-writer` test is T1's question. (2026-11-25, proposed:
[ADR 0047](0047-test-writer-tests-are-frozen.md) answers it: yes, under the trailer `App-Evaluator:`.)

## Consequences
- Until the trial has run, nothing of this is built: no agent file, no hook change, no wiring in
  `CLAUDE.md`. Slices V2–V4 start only after this ADR is accepted with a keep. (2026-11-26: they start
  after acceptance, in the changed form of Decision, "Outcome of the trial"; V5 comes first.)
- The trial judges the evaluator on another stack than the template's. `strength_planner` has no
  `make dev`, no `make e2e` and none of this repo's hooks or skills, so the evaluator runs from an
  ad-hoc prompt and `code-reviewer` from its agent file outside this repo's hooks, with a recorded table of
  what replaces what. V2's first step checks its wiring on the template itself.
- The evidence will be thin, as ADR 0039's was: one model, defects chosen by the trial's author, and six
  cases from only four specs, because most of the product's specs describe API calls. Three cases come from
  one spec, and two specs have about two counted scenarios each (2026-11-26: the run counted three on 003 and
two on 004). At least half of the planted defects come
  from classes that the Harness design post or
  [Effective harnesses for long-running agents](https://www.anthropic.com/engineering/effective-harnesses-for-long-running-agents)
  report, and the bar is relative to `code-reviewer` on the same cases, to limit how far the choice can flatter the
  evaluator.
- An evaluator pass costs an agent run with a browser per UI change. The Harness design post's full harness
  cost about 20 times its solo run; if the measured cost is near that, the agent is not a default step even
  if kept. (2026-11-26: measured at 1.28 USD per run on average; the owner ruled it runs on demand, about
  once per feature, not per UI change.)
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
  default because it is self-evaluation; it is the fallback if the trial drops the agent. (2026-11-26: the
  trial matched the evaluator to `code-reviewer`, not to this fallback, which it never measured. The owner
  chose a separate agent over the builder's own check, because it is not the builder grading its work; the
  agent file, its hooks and V2–V4 are the price of that.)
- **Tune the prompt and rerun** (2026-11-26), the default this ADR set for a missed bar. Rejected by the
  owner: the planted defects were visible in the diff, so a better prompt cannot show an advantage the case
  design does not allow. The money goes into the scenario tests instead.
- **Drop the evaluator altogether** (2026-11-26), what this ADR prescribed after a missed bar and no tuning.
  Rejected by the owner's judgement, not by the numbers: on equal hits it costs a little more than
  `code-reviewer` (1.28 against 1.06 USD per run). It failed no clean scenario, and it found one new thing,
  not yet confirmed, that `code-reviewer` did not: a 409 where the spec asks for 412.
- **Trial on scratch features in the template**, as the plan first said. Cheaper to set up and on the right
  stack, but the template has no business domain and four toy features say little about a real app. The
  owner ruled for `strength_planner`.
- **An LLM judge for the evals.** It grades loose output without a format change. Rejected: it adds a second
  unmeasured agent to measure the first. A reviewer whose lines cannot be matched gets a stricter output
  format instead.
- **One ADR for all of plan 0006**, including the stop gate and the frozen-tests rule. Rejected: this ADR
  waits for a trial, and those two slices do not depend on it.
