# 0006 — Close the gaps to Anthropic's harness guidance

Status: active · Owner: @aheumaier · Started: 2026-11-24

## Goal

[Plan 0003](0003-harness-gap-closure.md) compared this template with OpenAI, Böckeler and Thoughtworks.
Anthropic's own writing on harnesses was never read against it: the only trace is one unlinked attribution in
0003's gap row G2. This plan closes the six gaps a comparison on 2026-11-24 (`main` @ 84c552a) found.

Sources:

- [Harness design for long-running application development](https://www.anthropic.com/engineering/harness-design-long-running-apps) (2026-03-24)
- [Effective harnesses for long-running agents](https://www.anthropic.com/engineering/effective-harnesses-for-long-running-agents) (2025-11-26)
- [Building agents: when to fold subagents into your main agent loop](https://claude.com/resources/articles/building-agents-when-to-fold-subagents-into-your-main-agent-loop) (2026-10-22)
- [Agent harness design: 3 patterns](https://claude.com/blog/harnessing-claudes-intelligence) (2026-04-02)
- [How we built our multi-agent research system](https://www.anthropic.com/engineering/multi-agent-research-system) (2025-06-13)
- [Building a C compiler with a team of parallel Claudes](https://www.anthropic.com/engineering/building-c-compiler) (2026-02-05)
- [Claude Code best practices](https://code.claude.com/docs/en/best-practices)

The lead item is the **evaluator that runs the app** (owner's ruling, 2026-11-24). Both long-running-harness
posts report the same thing: unit tests and a read of the diff miss failures a user would see, and an agent
that clicks through the running app "as a human user would" finds them. The repo has the tooling
(`make dev`, `make e2e`, the `drive-app` and `query-runtime` skills) and no agent whose job it is.

Rules for every slice, as in 0003:

- **One slice per PR.** **H** marks a slice that touches `scripts/harness/protected-paths.txt` paths and
  needs a human-approved harness PR.
- **Every new computational sensor ships complete:** a rule in `docs/harness/rules.md` with a remediation, a
  fixture or Go test proving it fires ([ADR 0009](../../adr/0009-harness-selftest.md)), a `budgets.txt` row
  if it may exceed 5 s.
- **New harness logic is a `.bin/harness` subcommand** ([ADR 0045](../../adr/0045-harness-scripts-and-tests-in-go.md)),
  not a new shell script. A slice that changes a script [plan 0005](0005-harness-scripts-in-go.md) is porting
  lands after that script's port.
- **Every new agent earns its place by a trial** with a keep/drop criterion stated before the run, as
  [ADR 0039](../../adr/0039-tests-are-written-apart-from-the-implementation.md) did for `test-writer`.
- **Inferential output stays advisory** ([ADR 0019](../../adr/0019-inferential-sensors-are-advisory.md));
  nothing an LLM judges enters `make check` (NFR-R1).

## Gaps

✅ present · 🟡 partial · ❌ absent.

| Id | Gap | Source | Repo today | Status |
|----|-----|--------|------------|--------|
| A1 | An evaluator, separate from the builder, drives the running app against the acceptance scenarios and grades each one with a hard threshold | Harness design ("Separating the agent doing the work from the agent judging it proves to be a strong lever"); Effective harnesses (browser testing "as a human user would") | `code-reviewer` reads the diff and runs `make check`; `drive-app` is a skill the builder may use on its own work; `make e2e` runs only journeys someone already wrote | 🟡 |
| A2 | "Done" is gated by the harness when the agent stops, not by a guide | Best practices (a Stop hook blocks the stop until the check passes; "CLAUDE.md is for guidelines") | "Done means green" is golden principle 6: prose. `fast-check` covers one file per edit; the `pre-commit` hook runs the cheap sensors on the staged files, CI the rest after the push | ❌ |
| A3 | Agents and their prompts have evals: a small set of cases, run more than once, rerun on a model or prompt change | Fold subagents (about 10 golden examples, run each task more than once); Multi-agent research (start with about 20) | Computational sensors have fixtures; ADR 0019 says of inferential ones "No fixture can prove one fires" and stops there. ADR 0039's trial was one-off, its branch deleted | ❌ |
| A4 | Each piece of scaffolding names the model weakness it assumes and is re-tested when the model changes | Harness design ("every component in a harness encodes an assumption about what the model can't do on its own"); 3 patterns ("Assumptions about what Claude can't do need to be re-tested with each step change") | [ADR 0032](../../adr/0032-rules-that-never-fire-are-a-drift-report.md) reports computational rules that never fire. Agents, skills, hooks and loop prompts have no such record | ❌ |
| A5 | Subagents are kept only where measured to help; cheaper models where they hold up | Fold subagents ("fewer subagents in favor of progressively disclosed context and tools"; fold one at a time and compare evals); 3 patterns ("If you need a cheaper model, use a subagent") | `CLAUDE.md` triggers up to three reviewers per change and `evasion-reviewer` triggers by its own description; none was measured (only `test-writer` was, ADR 0039); no agent file sets `model:` | 🟡 |
| A6 | The implementer cannot weaken the tests it was given, enforced by the harness | Effective harnesses (agents may only flip `passes`; "It is unacceptable to remove or edit tests") | `test-writer` is confined by a hook; a later edit to its tests is found only by `code-reviewer` check 7, advisory, and not at all in CI ([TD-125](../tech-debt-tracker.md)) | 🟡 |

Where the repo already matches or exceeds the sources, no slice is planned: hooks over guides for harness
protection, per-area `AGENTS.md` as progressive disclosure, state in files and git so a compaction costs
nothing (0003 loop prompt), orchestrator and lanes, builder and checker kept apart in the loop.

## Phases

| # | Phase | Exit criterion | State |
|---|-------|----------------|-------|
| 0 | Decision | ADR 0046 proposed: what an app-driving evaluator's verdict counts for, agent evals as a drift report, the assumption ledger (amends 0019, 0032, 0039). The stop gate and the frozen-tests rule get their own ADRs with S2 and T1 | done 2026-11-24 |
| 1 | Evaluator runs the app (A1) | Trial result recorded in ADR 0046 while it is still proposed, then the ADR accepted; if kept: `app-evaluator` agent confined by hooks, wired into `CLAUDE.md` and the loop checker step; an accepted finding is a committed journey in `make e2e` | open |
| 2 | Done gate at stop (A2) | Stopping the main session with a changed tree and a red `make check` is blocked once with the sensor output; budget and escape rules hold | open |
| 3 | Agent evals (A3) | `make agent-evals` reports hit rate, false positives, run-to-run flips and cost per agent; result appears in the weekly drift issue once TD-38 is paid | open |
| 4 | Measure and prune (A4, A5) | Every agent, skill, hook and loop prompt has an assumption row, checked by a sensor; each reviewer has a measured `model:` or is folded; stale rows listed weekly | open |
| 5 | Frozen tests (A6) | A changed or deleted `test-writer` line without a spec change or a later test commit fails `make check-all`; TD-125 paid | open |

Order is by dependency, with the lead item first. Phase 4 needs Phase 3's runner. Phase 3 does not get its
cases from Phase 1: the trial cases are a product's and stay there (ADR 0046). Phases 2 and 5 are
independent, do not wait for ADR 0046 to be accepted, and can run in parallel with the rest. Phases 3 and 4
start after it is accepted.

## Progress

`[ ]` open, `[x]` done (`sha — before → after`), `[!]` blocked (reason). **H** needs a human-approved harness
PR; **–** needs none.

- [x] D0 – f186bda — no decision record for A1, A3 and A4 → ADR 0046 proposed, amending
  0019, 0032 and 0039
- [ ] V1 – evaluator trial run on a clone of `strength_planner`, numbers recorded in ADR 0046 (design below;
  the owner approves the design and cost before the run)
- [ ] V2 H `app-evaluator` agent file and its hook confinement
- [ ] V3 H finding-to-journey rule: `drive-app` skill and `code-reviewer` updated
- [ ] V4 H wiring: `CLAUDE.md`, loop prompts' checker step, PR template line
- [x] S1 – 2d78eb2 — Stop-hook behaviour taken from a summary → verified against the hooks reference on
  2026-11-24; `stop_hook_active` tells a repeated stop, so no marker file is needed
- [ ] S2 H `.bin/harness stop-gate`, hook entry in `.claude/settings.json`, rule and tests
- [ ] E1 H eval corpus for `code-reviewer`; case format for product-owned `app-evaluator` cases
- [ ] E2 H `.bin/harness agent-evals` and `make agent-evals`
- [ ] E3 H corpora for the other reviewers; weekly run in `drift.yml`
- [ ] M1 H assumption ledger and `harness-assumptions-listed`
- [ ] M2 H model trial per reviewer, numbers in the ledger (`docs/harness/*` is protected)
- [ ] M3 H `model:` set, reviewers folded or retired as M2 decides; `CLAUDE.md` trigger list updated
- [ ] M4 H stale assumption rows in the weekly drift report
- [ ] T1 H `specs-tests-frozen` sensor in `check-all`
- [ ] T2 H `pr-review` passes the since-test-commit diff (pays TD-125)

### Phase 0 — decision

**D0.** [ADR 0046](../../adr/0046-agent-verdicts-are-advisory-and-measured.md), amending 0019, 0032 and
0039. It stays `proposed` until V1's numbers are in it, since an accepted ADR is not edited in place. It
decides three things and holds the keep bar and the defaults of Phases 1, 3 and 4:

- An app-driving evaluator is an inferential sensor: advisory. What makes its finding binding is not its
  verdict but the journey that reproduces it (V3), which is computational. The journey is committed only
  when the builder accepts the finding.
- Agent evals are a trend and a drift report, like coverage ([ADR 0020](../../adr/0020-test-coverage-is-a-trend.md))
  and mutation ([ADR 0023](../../adr/0023-mutation-testing-is-a-drift-report.md)). They never gate a PR.
  ADR 0019's "no fixture can prove one fires" stays true; an eval measures how often it fires.
- ADR 0032's question (does this rule still earn its place?) is extended from rules to agents, skills,
  hooks and loop prompts through the assumption ledger. Retirement stays a human PR.

Not in ADR 0046 (owner's ruling, 2026-11-24): the stop gate (S2) and the frozen-tests rule (T1) each get
their own ADR with their slice. T1's ADR also decides whether a journey the evaluator adds is frozen like a
`test-writer` test, and under which commit trailer.

### Phase 1 — evaluator runs the app

The builder may already use `drive-app`. That is self-evaluation, and the Harness design post is blunt about
it: "agents reliably skew positive when grading their own work." It is equally blunt about the fix needing
work: "Out of the box, Claude is a poor QA agent." So the agent is trialled and its prompt tuned before it
is kept.

**V1 — trial.** Designed on 2026-11-24, not run. The owner ruled that the trial ground is the product
`strength_planner` (Go API, React app, spec-kit specs, Playwright journeys), not a scratch domain here. The
pilot starts only after the owner approves this design and the pilot's cost; the round starts only after the
owner approves the cost the pilot measured.

- **Where.** A separate local clone of `strength_planner` `main` at a pinned commit (`3b0a312` on
  2026-11-24). Nothing is written in the product's own checkout: no branch, worktree, file or stash. Planted
  defects exist only as uncommitted patches in the clone. The clone and its Docker project are removed when
  the numbers are recorded and the cases are handed over (Record).
- **Stack.** The product has no `make dev` or `make e2e`. The clone starts its own stack: the database from
  its `docker-compose.yml`, `make migrate-up`, the API container, then `npm run dev` in `apps/web`; the users
  come from the journeys' global setup. The product's default ports are in use on the owner's machine,
  so one commit in the clone moves the trial to its own ports (compose file, Playwright config,
  global setup, the CORS origin) and its own compose project name. The AI key is a dummy value. Runs go one
  at a time; each starts from an empty database and a reset tree. A pilot (below) proves the recipe.
- **Cases.** A case is one or two user stories. Four get two planted defects each: `002-editable-names`
  (stories 1 and 3), `003-microcycle-session-count` (its only story), `019-live-training-session` story 1,
  and its story 2. Two stay clean: `019-live-training-session` story 3 and `004-mesocycle-order-sort-fix`
  (its only story). 003 and 004 have about two counted scenarios each. The other specs on the product's
  `main` describe mostly API calls, need the AI key, or are in work, so there is no reserve. Before the run,
  each case lists the scenario ids a user can reach in the browser (one that names only an HTTP request or
  status does not count), its existing journeys are green on the pinned commit, and no open bug breaks a
  counted scenario. If a case fails this, stop and ask the owner.
- **Defects.** Of the kind a user sees: a control wired to the wrong operation, a feature that renders but
  does nothing, an error state never shown, a list that ignores its filter, a value lost on reload. Each
  breaks one counted scenario, a different one for each defect of a case, is not described by the spec's
  Edge Cases or Assumptions, and leaves the product's own checks green on the planted tree: Go tests, lint,
  vitest, `tsc` and the existing journeys. At least four of the eight come from classes the two
  long-running-harness posts report finding (the first two in this list); the case table marks which.
- **Arms.** Each case is judged twice by each arm, on the same model. (a) `code-reviewer`, with this repo's
  agent file as its prompt, gets the feature's diff with the defects squashed in (the feature's commits
  where they can be told apart, otherwise its files as added), not a diff of the defects alone. The two
  planted stories of 019 share a commit; each gets that diff with its own defects. (b) The evaluator gets
  the base URL, a browser and the case's user stories and their acceptance scenarios. It works from an empty
  directory: no source, no git history, no account of what was built. A trial-only `--settings` file limits
  it to its browser tools and to writing in that directory. A recorded table says what replaces
  each harness piece the product lacks.
- **Evaluator output.** One line per acceptance scenario of the case, `PASS`, `FAIL` or `NOT EXERCISED`,
  each with evidence (journey step, screenshot or accessibility-tree path, request id). Each `FAIL` comes
  with a journey that shows it. Any `FAIL` or `NOT EXERCISED` makes the verdict `CHANGES REQUESTED`: a hard
  threshold per criterion, no averaging.
- **Keep bar** (ADR 0046 holds the binding text). A hit is a `FAIL` on the scenario a planted defect breaks,
  with evidence, in both runs; for `code-reviewer`, a finding naming the defect's file and behaviour, in both
  runs. `NOT EXERCISED` is a miss on the scenario a planted defect breaks and a `FAIL` on a counted scenario
  of a clean case; any other `FAIL` or `NOT EXERCISED` on a planted case is reported apart, with its count,
  and counts for neither side.
  **Keep** if the evaluator hits at least two planted defects `code-reviewer` did not, and in each run fails
  at most one counted scenario across the two clean cases. A `FAIL` on a clean case is a found bug, reported
  apart and counting for neither side, only when its journey reproduces it on the pinned commit and the
  product's owner confirms that the journey expects what the scenario says. **Otherwise** tune the prompt at
  most twice and rerun; if it still fails, drop the agent, record the numbers, and add "drive the scenarios in
  the app" to `code-reviewer` only if that variant passes the same bar.
- **Cost (an estimate, nothing measured yet).** One round is 12 evaluator runs and 12 `code-reviewer` runs.
  Guess: 5 to 10 USD and 30 to 45 minutes per evaluator run, about a fifth of that per reviewer run. That is
  about 72 to 144 USD and one to one and a half working days per round. With a margin of half: about 110 to
  220 USD per round, and 330 to 660 USD if both tuning rounds are needed. Not included: the pilot (5 to 10
  USD) and the `code-reviewer` variant that drives the scenarios. **Pilot first:** one evaluator run on one
  clean case. It proves the stack recipe and gives the real cost per run; the owner approves the round from
  that number.
- **Record.** The table, the cost per judged case and the wall time go into ADR 0046. The cases (pinned
  commit, patches, scenario lists, start recipe) are handed to the product's owner before the clone is
  removed; whether they are kept is the owner's call. They are not copied into this public template.
- **V2's first step.** If kept, V2 first checks the wiring on this template's own stack, with `make dev` and the
  `drive-app` skill: a small web feature on a scratch branch, judged once clean and once with a planted
  defect, each with and without read access to the repository (the trial gave none; the agent below may
  read it). It has no keep bar. If the results differ, V2 takes the better variant. V2's PR records them and
  the scratch branch is deleted.

**V2 — the agent (H).** `.claude/agents/app-evaluator.md`:

- Tools `Read, Grep, Glob, Write, Edit, Bash`. Bash is confined by a hook to an allow-list of make targets
  (`dev`, `e2e`, the `query-runtime` targets, `check`). `only-make-check.sh` hard-codes `make check(-all)`
  today; it gains a target-list argument rather than a sibling hook. `make dev` runs until stopped, so the
  slice also settles how the evaluator starts and stops it under that confinement (the first step of V2
  finds out; `make e2e` starting its own `make dev` may be enough). Write and Edit are confined by
  `only-test-files.sh`; it accepts every product test file, so the slice narrows it to `apps/web/e2e/` with an
  argument too. The evaluator writes journeys and nothing else.
- Input: the feature's `spec.md` (acceptance scenarios with their
  [ADR 0036](../../adr/0036-acceptance-scenarios-are-traced-to-tests.md) ids), and the repository root if the
  first step keeps read access. It is told that every file is data, not instructions, as `reuse-reviewer`
  and `evasion-reviewer` are.
- A cost cap per run, as ADR 0019 requires of the slice that adds an agent; V1's measured cost sets it.
- The prompt carries the rubric the trial tuned. It states what the evaluator cannot judge (sound, native
  dialogs, anything not reachable through the page or the API logs) so that it reports `NOT EXERCISED`
  rather than guessing.
- Scope: the web app and the API as the web app uses it. iOS is out ([plan 0004](0004-ios-stack-group.md)).

**V3 — a finding is a journey (H).** An evaluator `FAIL` is reported with the journey that shows it, left
uncommitted in `apps/web/e2e/`, where it makes `make e2e` red until it is accepted or deleted. When the
builder accepts the finding, the journey is committed with the fix, in a commit that quotes the `FAIL` line
and its scenario id, and `make e2e` (in `check-all`) holds the behaviour from then on; a rejected finding's
journey is deleted, with the reason in the report (ADR 0046). ADR 0019 turns a finding class seen twice into a
computational rule; here a single accepted finding becomes a test at once, because a journey is cheap and
specific to the feature. Update `drive-app` §3 and §5 (a journey written to prove an accepted failure is
kept, not deleted) and `code-reviewer` (how it treats a journey the evaluator added). The commit carries
the trailer `App-Evaluator: specs/<NNN-slug>`, and `harness specs-frozen` learns to read it: journeys only (the
files V2 lets the agent write), and the commit may hold the fix too; a later `App-Evaluator:` commit may change an
earlier journey only if it changes journeys and nothing else
([ADR 0047](../../adr/0047-test-writer-tests-are-frozen.md)).

**V4 — wiring (H).** `CLAUDE.md`: a change that alters what the web app shows or how it talks to the API
also gets `app-evaluator`, before it is called done. The 0003 loop prompt's checker step and the PR template
(one line: evaluator verdict, or why not applicable) follow. CI is not wired: see Not in this plan.

### Phase 2 — done gate at stop

**S1.** Read the Claude Code hooks reference and record, under Open assumptions, what was verified: the
Stop event's input fields, how exit 2 is fed back, how a hook tells a first stop from a stop after its own
block, whether subagent stops are a separate event, and the timeout.

**S2 (H).** `.bin/harness stop-gate`, called from a `Stop` entry in `.claude/settings.json` through the usual
bootstrap wrapper:

- **Runs** `make check` when tracked files differ from the tree of the last green `make check` (a stamp of
  the tree hash under `.cache/harness/`, written by `make check` on success). No difference: silent, exit 0.
- **Red:** prints the failing sensor's output and remediation, exit 2. The agent continues.
- **Escape:** a stop that follows the gate's own block passes with a one-line notice. The gate blocks once
  per stop, never twice in a row, so a red tree the agent cannot fix (a `test-writer` commit awaiting its
  implementation, a blocked slice) does not trap the session.
- **Main session only.** Subagent stops are not gated: reviewers change nothing, and `test-writer` must leave
  red tests.
- **Budget.** `make check` is ≤ 10 s only for a change without shell or harness files (NFR-H3); with them the
  shell tests alone took 155–198 s ([TD-7](../tech-debt-tracker.md)). Until plan 0005 retires bats, the gate does not run the
  check when the affected scope includes the shell tests: it prints "run `make check` before calling this
  done" and exits 0. A `budgets.txt` row holds the gate itself to the NFR-H3 figure.
- **Missing or stale binary:** the wrapper blocks with the bootstrap's notice, as every hook does (ADR 0045,
  NFR-R2). The block-once rule must therefore also hold in the wrapper, so that a broken gate cannot keep a
  session from ending. S1 showed the wrapper can tell a repeated stop; S2's ADR argues why letting it
  pass meets ADR 0045 and NFR-R2, or records the exception.
- Rule `agent-stop-is-green` in `rules.md`; Go tests for each branch above; its own ADR, written with this
  slice.

### Phase 3 — agent evals

**E1 (H).** A corpus under `scripts/harness/agent-evals/<agent>/<case>/`: the input the agent gets (a diff,
or a spec plus a patch to apply to the template's tree), the findings it must report and, for clean cases, that it
must report none. About ten cases per agent, at least three clean. Sources: real catches from the 0003 and
0005 logs and review sections; planted violations of each `code-reviewer` check. The template ships only
cases that apply to its own tree. `app-evaluator` cases, the V1 trial's among them, belong to a product and
stay there; E2's runner accepts a case folder outside the template (ADR 0046).

**E2 (H).** `.bin/harness agent-evals [agent]`, behind `make agent-evals`:

- Runs each case **twice** with the agent's file as it is in the tree.
- Grades without an LLM judge: `code-reviewer` prints a verdict line
  and `path:line — problem — fix (rule id or principle #)`, so a case names the expected verdict, path and
  id. The other reviewers print looser lines; E3 gives each a gradable format before its corpus is written.
  `app-evaluator` cases name the expected verdict per scenario id.
- Reports, per agent: cases hit in both runs, in one, in none; findings on clean cases; verdict flips between
  runs; tokens and cost. Writes the row to `.cache/harness/`, as the timings are.
- Needs credentials, the network and money, so it is in neither `check` nor `check-all`. Without a key it
  skips with a notice.

**E3 (H).** Corpora for `architecture-reviewer`, `reuse-reviewer`, `evasion-reviewer`. A weekly `drift.yml`
job runs the evals of the read-only agents and adds the table to the drift issue, confined as the scheduled
reviews are (`ci-agent`, tools `Read,Grep,Glob` per ADR 0019). `evasion-reviewer` and `app-evaluator` need
Bash, so their evals run locally only. The job is blocked on [TD-38](../tech-debt-tracker.md) like every CI
agent run. The PR template gains a line: a PR that changes `.claude/agents/*` states the before and after
`make agent-evals` rows.

### Phase 4 — measure and prune

**M1 (H).** `docs/harness/assumptions.md`: one row per agent, non-vendored skill, hook and loop prompt.
Columns: component; the model weakness it assumes; the evidence it was added on (ADR, trial, incident); last
tested (model id, date, result); how to test its removal. Sensor `harness-assumptions-listed`
(`.bin/harness`, in `make check`): every file under `.claude/agents/`, `.claude/hooks/`, `.claude/skills/`
(spec-kit's excluded) and every `*-loop-prompt.md` has a row, and every row names a file that exists.

**M2 (H).** For each reviewer, `make agent-evals` with the agent on the session's model and on the two cheaper
tiers. Record the three rows in the ledger. A cheaper model is taken when it hits the same cases in both
runs and adds no finding on a clean case.

**M3 (H).** Apply M2: set `model:` in the agent files. Then the fold test, one agent at a time: add the
agent's checks to `code-reviewer`, rerun both corpora, and keep the fold only if no case flips to a miss.
Candidates by overlap of scope: `reuse-reviewer` and `architecture-reviewer`. `evasion-reviewer` (needs a
scratch worktree and Bash) and `test-writer` and `app-evaluator` (need permissions the reviewer must not
have) are kept apart on the fold-subagents article's own grounds: verification and modular permissioning.
Update the `CLAUDE.md` trigger list.

**M4 (H).** The weekly drift report lists ledger rows whose "last tested" names a model other than the one
the CI agents run on, or is older than 90 days, next to ADR 0032's never-fired rules. Removal is tried one
component at a time and is a human PR.

### Phase 5 — frozen tests

**T1 (H).** Sensor `specs-tests-frozen` (`.bin/harness specs-frozen`, in `check-all`, since it reads history):
what a `Test-Writer: specs/<NNN-slug>` commit wrote in test files may not be edited, deleted or skipped later in
the branch without a change to that spec. [ADR 0047](../../adr/0047-test-writer-tests-are-frozen.md) decides the
details, and differs from this slice's first text in five points: every test commit counts, not only the first;
"changed in the same range" is the range from the test commit to `HEAD`; only what the branch itself did counts;
a later test commit may change an earlier one's tests; a dropped spec lifts the freeze too. It is coarser than
`code-reviewer` check 7 by design: the reviewer still judges whether the spec diff explains the change, and
loosening through another file. Go tests: an assertion edited, a case deleted, a `t.Skip` added, each with and
without a spec change; one fixture ([ADR 0009](../../adr/0009-harness-selftest.md)) proves the rule
fires. ADR 0047 also freezes a journey the evaluator added, under `App-Evaluator:`; V3 builds that part.

**T2 (H).** TD-125: `pr-review` passes the diff since each test commit to the CI reviewer. It lands in
the Go port of `pr-review` (plan 0005), not in the shell script.

## Open assumptions

- **Stop-hook behaviour: verified on 2026-11-24 (S1)** against the
  [hooks reference](https://code.claude.com/docs/en/hooks). What S2 builds on:
  - **Input.** The common fields (among them `session_id`, `transcript_path`, `cwd`, `permission_mode` and
    `hook_event_name`) plus `stop_hook_active`, `last_assistant_message`, `background_tasks` (the tasks
    still running) and `session_crons`.
  - **Exit 2** blocks the stop and the hook's stderr goes to the model as the reason. Exit 0 lets the
    session stop. Any other exit code is a non-blocking error: the stop goes ahead. Exit 0 with
    `{"decision":"block","reason":"…"}` on stdout blocks too; a hook uses one form.
  - **First stop or repeated stop.** `stop_hook_active` is `true` "when Claude Code is already continuing as
    a result of a stop hook". The escape rule reads it; no marker file is needed. The field is about any
    stop hook, not this one: when a user-level Stop hook blocked first, the gate sees `true` and lets the
    stop pass without having run. S2 accepts that or says why not.
  - **A built-in cap.** After eight stop-hook continuations in a row Claude Code overrides the next block and
    ends the turn; `CLAUDE_CODE_STOP_HOOK_BLOCK_CAP` raises the cap. The count resets each time the model calls
    a tool, so the cap does not stop a broken gate whose agent keeps working. The block-once rule must.
  - **Subagents.** `Stop` runs "when the main Claude Code agent has finished responding"; a subagent's stop
    is the separate `SubagentStop` event (with `agent_id`, `agent_type`, `agent_transcript_path`). So a `Stop`
    entry in `.claude/settings.json` should be the main-session gate the plan wants; S2 tests that it does not
    run for a subagent.
  - **Timeout.** 600 s by default for a command hook, set per hook with `timeout` (seconds). A hook that
    times out is cancelled and its output discarded.
  - **Not fired** when the user interrupts. A turn that ends on an API error fires `StopFailure`, whose
    output and exit code are ignored.
  - **The wrapper can tell a repeated stop without the binary.** `scripts/harness/hook-bootstrap.sh`
    already matches the raw hook input when `.bin/harness` is missing; the same kind of match on
    `"stop_hook_active": true` gives block-once there. A repeated stop that passes without the binary is still
    a hook that fails open: S2's ADR argues why that meets ADR 0045 and NFR-R2, or records the exception.
  - **Not stated in the reference, to be tested in S2:** that a timed-out Stop hook lets the stop go ahead
    (the reference says so for "most events"); whether `stop_hook_active` stays `true` when the model
    calls tools between the block and its next stop; that `agent_id` is absent from a main-session `Stop`;
    what a Stop hook's plain output at exit 0 reaches (the events whose stdout reaches the model are listed,
    and Stop is not one of them).
- **The trial's planted defects are chosen by the plan's author.** They may flatter the evaluator. V1
  therefore takes at least half of them from defect classes the two long-running-harness posts report, and
  the keep bar is relative to `code-reviewer` on the same cases.
- **A trial on another product says something about the evaluator here.** `strength_planner` is not a
  harness adopter: no `make dev`, no `make e2e`, none of this repo's hooks or skills. The trial judges the
  evaluator's prompt and method, not its wiring; V2 first checks the wiring on the template. A product
  that adopts the harness should rerun E2 on its own cases.
- **The trial's cases are assumed healthy.** Read on 2026-11-24: spec 019's feature commits are on the
  product's `main` (its status line still says otherwise). Judged by title, no open entry in the bug list
  breaks a counted scenario of 002, 003, 004 or 019; one to watch is #131 (finishing a session can make AI
  calls, and the trial's AI key is a dummy). Not known: whether the journeys are green. V1 checks both on
  the pinned commit first.
- **Deterministic grading is possible.** If reviewers' wording of path and id proves too loose to match, E2
  first tightens the output format in the agent files rather than adding an LLM judge.
- **Cost.** Unknown until V1 and E2 report it; V1's figures are a guess until its pilot run. The Harness
  design post's full harness cost about 20 times its solo run; an evaluator pass per UI change must be far
  below that to be kept as a default.
- **A tree-hash stamp is a sound "last green" marker** given `make check` is scoped to the change
  ([ADR 0014](../../adr/0014-make-check-scoped-to-the-change.md)). S2 checks which scope the stamp may claim.

## Not in this plan

- **The evaluator in CI.** It needs a running stack next to an agent that holds a secret and reads PR
  content. That is a new confinement question on top of TD-38; revisit once Phase 1 has a kept agent and
  TD-38 is paid.
- **A sprint contract negotiated between builder and evaluator.** Spec-kit's acceptance scenarios with
  traced ids already fix what done means before work starts; the Harness design post itself removed sprints
  as the model improved.
- **A planner agent, or loosening `plan.md`.** Noted as a tension (the Harness design post keeps the planner
  at product level because detailed errors cascade); no evidence here yet that pinned signatures hurt.
- **Loop-harness changes** (feature list in JSON, an initializer session). The 0003 loop's Progress list and
  git cover the same ground.

## Log

- 2026-11-24 — plan written from the comparison with the Anthropic sources; evaluator-runs-the-app ruled the
  lead item by the owner.
- 2026-11-24 — plan reviewed by `senior-architect-reviewer` (behaviour) and `architecture-reviewer`
  (structure). Owner's rulings on their findings: the V1 trial runs on `strength_planner`, not a scratch
  domain; ADR 0046 decides three things, and the stop gate and the frozen-tests rule get their own ADRs; the
  keep bar counts only browser-reachable scenarios, defines a hit, and treats `NOT EXERCISED` as a miss or a
  fail; trial cases stay with the product, so Phase 3 is no longer seeded by Phase 1; an evaluator journey is
  committed only with an accepted finding; Phases 3 and 4 wait for ADR 0046 to be accepted; trial cases are
  user stories of specs 002, 003, 004 and 019.
- 2026-11-24 — D0: ADR 0046 written, status `proposed`. V1 designed, not run; a `strength_planner` session
  said a separate clone with other ports fits, and that it could not vouch for the cases. Findings left for
  their slices: what a Stop hook's exit-0 output reaches, an explicit timeout, the stamp's key and a second
  blocking Stop hook (S2); the rule id `agent-stop-is-green` against the `harness-` prefix; this file's
  "Status: proposed" (now `active`).
- 2026-11-24 — S1 and D0 rebased and reviewed again by `code-reviewer`; S1's claims checked against the hooks
  reference. Fixed: what the continuation cap bounds (S1), and in D0 the keep bar, what
  `only-test-files.sh` accepts, the red working tree an uncommitted journey causes, and what a drop leaves
  standing. Owner's rulings: the keep bar excuses a `FAIL` on a clean case only when the owner confirms the
  journey's expectation, and a verdict on a planted case's other scenarios counts for neither side; V1's
  design and pilot are approved, with a trial-only `--settings` file; S1 and D0 are two PRs; ticks keep their
  branch SHAs, whatever the merge method. Confirmed: approval of V1 in two steps; a drop still accepts ADR
  0046; its empty "Trial result" section.
- 2026-11-25 — owner's ruling of 2026-11-24 on V1's case 019 story 2, whose session detail page has a bug of
  its own on the pinned commit: the case stays, and a hit counts only if its evidence shows the planted symptom.
  A `FAIL` whose evidence is that page's bug is not a hit. ADR 0046's keep bar is unchanged.
