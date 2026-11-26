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
- [How Anthropic builds software in 2026, with Katelyn Lesse](https://newsletter.pragmaticengineer.com/p/how-anthropic-builds-software-in)
  (The Pragmatic Engineer, 2026-11-25), the newsletter and its podcast transcript
- [Migrating our Messages API to Rust](https://www.anthropic.com/research/migrating-api-to-rust) (2026-11-25)

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
  (2026-11-26: the app evaluator is the one exception, by the owner's override after V1; see ADR 0046,
  Outcome of the trial.)
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
| 1 | Evaluator runs the app (A1) | Trial result recorded in ADR 0046 while it is still proposed, then the ADR accepted (result 2026-11-26: bar missed; the owner overrode the outcome, see V1). V5's ADR decided and its sensor built: how a scenario a user sees is proven against the running app; `app-evaluator` confined by hooks and run on demand as an explorer; an accepted finding is a committed journey in `make e2e` | open |
| 2 | Done gate at stop (A2) | Stopping the main session with a changed tree and a red `make check` is blocked once with the sensor output or, past 15 s, with the request to run it; budget and escape rules hold | open |
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
- [ ] V1 – evaluator trial run on a clone of `strength_planner`, numbers recorded in ADR 0046 (run
  2026-11-26; open: hand the cases to the product's owner, then remove the clone)
- [x] V5 H e0d2fab, aa705d0 — with `db`, any test traced a scenario a user sees → only a journey or an
  integration test does, unless it ends with `(unit: <reason>)`; `test-writer` writes them; ADR 0049, proposed
  (`specs-tested` 0.03 s → 0.03 s warm on a tree with a spec)
- [ ] V2 H `app-evaluator` agent file, its hook confinement and a 3 USD cost cap per run
- [ ] V3 H finding-to-journey rule: `drive-app` skill and `code-reviewer` updated
- [ ] V4 H wiring: when to run the explorer (`CLAUDE.md`, loop prompts' checker step, PR template line)
- [x] S1 – 2d78eb2 — Stop-hook behaviour taken from a summary → verified against the hooks reference on
  2026-11-24; `stop_hook_active` tells a repeated stop, so no marker file per session is needed
- [x] S2 – 93dc5f2 — `.bin/harness stop-gate`, its Stop hook, rule `harness-stop-is-green` and ADR 0048, proposed;
  one stop on 2026-11-25: 0.45 s on a stamped tree (0.2 s before the key held ignored files), 2.6 s (red) and 3.1 s
  (green) on a one-file Go edit with warm caches (both before it), 15.3–15.7 s at the limit (harness edits, cold Go caches)
- [ ] E1 H eval corpus for `code-reviewer`; case format for product-owned `app-evaluator` cases
- [ ] E2 H `.bin/harness agent-evals` and `make agent-evals`
- [ ] E3 H corpora for the other reviewers; weekly run in `drift.yml`
- [ ] M1 H assumption ledger and `harness-assumptions-listed`
- [ ] M2 H model trial per reviewer, numbers in the ledger (`docs/harness/*` is protected)
- [ ] M3 H `model:` set, reviewers folded or retired as M2 decides; `CLAUDE.md` trigger list updated
- [ ] M4 H stale assumption rows in the weekly drift report
- [x] T1 – ca125d8 — `specs-tests-frozen` in `check-all` and ADR 0047, proposed; time of `make specs-frozen` on
  2026-11-25: 0.15 s without test commits, 1.4 s with 5 test commits in 105, 2.1 s with 20 loosened lines
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
work: "Out of the box, Claude is a poor QA agent." So the agent is trialled, and its prompt tuned, before it
is kept. (2026-11-26: it was trialled, missed the bar and was kept as an explorer without tuning, by the
owner's override; see V1.)

**V1 — trial.** Designed on 2026-11-24, run on 2026-11-26 (28.15 USD for one round). Result: both arms hit
7 of 8 defects, the evaluator hit none that `code-reviewer` missed, and it failed no clean scenario, so the
keep bar is missed. The owner overrode the rule for a missed bar (tune or drop): no tuning round, the
evaluator explores on demand instead of gating, and scenarios a user sees get tests against the running app
(V5). ADR 0046, Trial result, holds the table and costs. What follows is the design; ADR 0046 lists where
the run differed. The owner ruled that the trial ground is the product
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
  (its only story). 003 and 004 have about two counted scenarios each (2026-11-26: the run counted three on
  003 and two on 004). The other specs on the product's
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
- **Cost (an estimate made before the run; measured figures at the end).** One round is 12 evaluator runs and 12 `code-reviewer` runs.
  Guess: 5 to 10 USD and 30 to 45 minutes per evaluator run, about a fifth of that per reviewer run. That is
  about 72 to 144 USD and one to one and a half working days per round. With a margin of half: about 110 to
  220 USD per round, and 330 to 660 USD if both tuning rounds are needed. Not included: the pilot (5 to 10
  USD) and the `code-reviewer` variant that drives the scenarios. **Pilot first:** one evaluator run on one
  clean case. It proves the stack recipe and gives the real cost per run; the owner approves the round from
  that number. Measured: the pilot cost 1.28 USD; the round, approved up to 40 USD, cost 28.15 USD (evaluator
  mean 1.28 USD and 285 s per run, `code-reviewer` mean 1.06 USD and 124 s).
- **Record.** The table, the cost per judged case and the wall time go into ADR 0046. (2026-11-26: each run
  judged one case, so the cost per run is the cost per judged case.) The cases (pinned
  commit, patches, scenario lists, start recipe) are handed to the product's owner before the clone is
  removed; whether they are kept is the owner's call. They are not copied into this public template.
- **V2's first step.** V2 first checks the wiring on this template's own stack, with `make dev` and the
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
- A cost cap per run, as ADR 0019 requires of the slice that adds an agent: 3 USD, from V1's highest run
  (2.30 USD). V1's cases were one or two stories on an empty database, so a whole feature may need more;
  V2 settles how a run that hits the cap ends: its report is partial, and every scenario it did not reach is
  `NOT EXERCISED`. V2's first measured run on a whole feature sets the cap again, so that this stays rare.
- Its role is an explorer (ADR 0046, Outcome of the trial): it looks for what the scenario tests of V5 do not
  check, and each accepted finding becomes a journey (V3). The trial denied its Write tool by mistake, so V2's
  first step also checks that it can write its journey files.
- The prompt carries the rubric the trial used (it was not tuned). It states what the evaluator cannot judge (sound, native
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

**V4 — wiring (H).** `CLAUDE.md`: a feature that changes what the web app shows or how it talks to the API
gets one `app-evaluator` run, on demand, when its stories are built; it is not a step before every change is
called done (ADR 0046, Outcome of the trial). The builder decides when, and the PR template makes a skipped
run visible (one line: evaluator run and verdict, or why none). The 0003 loop prompt's checker step follows. CI is not wired: see Not in this plan.

**V5 — scenarios a user sees run against the app (H).** Built first in this phase. ADR 0036 traces each
acceptance scenario to a test, but any test counts, so a scenario about an HTTP header or what a page shows
can pass with a unit test that never starts the app. In V1 the product's own checks were green while the
evaluator found three scenarios failing in the running app (not yet confirmed by the product's owner). The
direction, which the slice's own ADR (amending ADR 0036 and ADR 0039) decides first:

- A scenario about what the web app shows, or what the API returns to it, is traced only by a journey
  (`*.e2e.*`) or an API test against the running server (an integration test); `specs-scenarios-tested`
  checks that.
- `test-writer` writes these tests from the spec before the implementation, as it writes unit tests now
  (ADR 0039). They are frozen by `specs-tests-frozen` (ADR 0047).

That ADR answers:

- How a scenario is recognised as one a user sees (by the story, a marker or a stated reason, without
  writing ids into the spec), and which test kinds count.
- How a frozen journey survives the page it was written against: selectors written before the page exists
  will change, and ADR 0047 allows that only with a spec change.
- What "confirms red" means for a journey against a page that does not exist yet: it fails, but for the
  wrong reason (ADR 0039).
- Whether `test-writer`'s wider scope needs its own trial: ADR 0039 limited it to unit tests because its
  trial covered only those.
- What traces these scenarios on a stack without `db` and `web` (NFR-B2), where journeys do not apply.
- What the extra journeys cost in the `e2e` budget row.

### Phase 2 — done gate at stop

**S1.** Read the Claude Code hooks reference and record, under Open assumptions, what was verified: the
Stop event's input fields, how exit 2 is fed back, how a hook tells a first stop from a stop after its own
block, whether subagent stops are a separate event, and the timeout.

**S2 (H).** `.bin/harness stop-gate`, called from a `Stop` entry in `.claude/settings.json` through the usual
bootstrap wrapper:

- **Runs** `make check` when the tree is neither the one `make check` last called green (a stamp under
  `.cache/harness/`, written by `make check` on success) nor the merge base's. The stamp's key is the tree as
  `git add -A` would stage it, untracked files included, with the merge base and `STACK`. The
  key also holds the files as they are on disk and the ignored files by size and time (owner's ruling, 2026-11-25).
  No difference: silent, exit 0. Plan mode and a turn that ends while a subagent or a background `make check` runs
  are not judged.
- **Red:** prints the failed sensors, their rules and the last lines of the output, exit 2. The agent continues.
- **Escape:** a stop that follows a Stop hook's block passes with a one-line `systemMessage` to the person (plain
  output at exit 0 reaches nobody). The gate blocks once per tree (owner's ruling, 2026-11-25), so a red tree
  the agent cannot fix (a `test-writer` commit awaiting its implementation, a blocked slice) does not trap the session. A second
  blocking Stop hook of the person's own counts as the block for this one too. Both run on the first stop, so the gate
  has judged by then, unless that stop passed as busy; the ADR accepts that.
- **Main session only.** Subagent stops are not gated: reviewers change nothing, and `test-writer` must leave
  red tests. `Stop` does not fire for them (tested, Open assumptions).
- **Budget.** The gate ends a check that is not done after 15 s and blocks once with "run `make check`
  yourself"; the next stop goes through. This covers the shell tests (155–198 s,
  [TD-7](../tech-debt-tracker.md)) and every other slow check. `timeout` in `settings.json` is 30 s, above it. The
  gate has a `budgets.txt` row and an NFR of its own, NFR-H10. (Before 2026-11-25 this text held the gate to NFR-H3's
  10 s, which `make check` breaks on a Go edit, and had it skip the check and exit 0 when shell tests were in
  scope, with a message the model would never see.)
- **Missing or stale binary:** the wrapper blocks with the bootstrap's notice, once per turn: it reads
  `stop_hook_active` from the raw input, so that a broken gate cannot keep a session from ending. ADR 0048 records
  the exception to ADR 0013, ADR 0045 and NFR-R2.
- Rule `harness-stop-is-green` in `rules.md` (named `agent-stop-is-green` before 2026-11-25; no rule has that
  prefix); Go tests for each branch above; [ADR 0048](../../adr/0048-stop-gate.md), written with this slice.

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
    a result of a stop hook". The escape rule reads it; no marker file per session is needed. The field is about any
    stop hook, not this one: when a user-level Stop hook blocked, the gate sees `true` at the next stop
    and lets it pass. It has judged the first stop, where both run, unless that one passed as busy. ADR 0048 accepts
    that.
  - **A built-in cap.** After eight stop-hook continuations in a row Claude Code overrides the next block and
    ends the turn; `CLAUDE_CODE_STOP_HOOK_BLOCK_CAP` raises the cap. The count resets each time the model calls
    a tool, so the cap does not stop a broken gate whose agent keeps working. The block-once rule must.
  - **Subagents.** `Stop` runs "when the main Claude Code agent has finished responding"; a subagent's stop
    is the separate `SubagentStop` event (with `agent_id`, `agent_type`, `agent_transcript_path`). So a `Stop`
    entry in `.claude/settings.json` is the main-session gate the plan wants; it does not run for a subagent
    (tested, below).
  - **Timeout.** 600 s by default for a command hook, set per hook with `timeout` (seconds). A hook that
    times out is cancelled and its output discarded. The gate's is 30 s (ADR 0048).
  - **Not fired** when the user interrupts. A turn that ends on an API error fires `StopFailure`, whose
    output and exit code are ignored.
  - **The wrapper can tell a repeated stop without the binary.** `scripts/harness/hook-bootstrap.sh`
    already matches the raw hook input when `.bin/harness` is missing; the same kind of match on
    `stop_hook_active` gives block-once there: it blocks only on `false` and nowhere `true`. A repeated stop that passes without the binary is still
    a hook that fails open: ADR 0048 records the exception.
  - **Not stated in the reference, tested in S2** on 2026-11-25 (Claude Code 2.1.311, `claude -p` in a scratch
    repo, project settings only):
    - `stop_hook_active` stays `true` when the model calls tools between the block and its next stop, and is
      `false` again after the next prompt.
    - A Stop hook that runs into its `timeout` (2 s, the hook slept 20 s) lets the stop go ahead.
    - `agent_id` and `agent_type` are absent from a main-session `Stop`.
    - `Stop` does not fire for a subagent; `SubagentStop` does. `Stop` fires each time the main turn ends while
      background tasks still run, and again when they have finished. `background_tasks` holds one entry per
      running task: `id`, `type` (`subagent` or `shell`), `status`, `description`, and `agent_type` or `command`.
    - Plain output at exit 0 does not reach the model in the next turn.
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
  the pinned commit first. (2026-11-24, on the pin: 160 journeys passed, 3 failed, none of them a case
  journey; 12 skipped and 15 did not run.)
- **Deterministic grading is possible.** If reviewers' wording of path and id proves too loose to match, E2
  first tightens the output format in the agent files rather than adding an LLM judge.
- **Cost.** Unknown until V1 and E2 report it; V1's figures are a guess until its pilot run. (2026-11-26:
  V1 measured 1.28 USD per evaluator run on average and 2.30 USD at most; E2 is still open.) The Harness
  design post's full harness cost about 20 times its solo run; an evaluator pass per UI change must be far
  below that to be kept as a default. (2026-11-26: it is not a default; the owner ruled one run on demand
  per feature.)
- **A tree-hash stamp is a sound "last green" marker** given `make check` is scoped to the change
  ([ADR 0014](../../adr/0014-make-check-scoped-to-the-change.md)). Checked in S2 (2026-11-25): the key needs the
  merge base, `STACK`, the files on disk and the ignored files beside the tree, and all but two sensors read the working tree, not the index
  ([ADR 0048](../../adr/0048-stop-gate.md), Consequences). Not seen: a red that comes from outside the tree
  or from a flaky test on a tree that was green once.

## Not in this plan

- **The evaluator in CI.** It needs a running stack next to an agent that holds a secret and reads PR
  content. That is a new confinement question on top of TD-38; revisit once Phase 1 has a kept agent and
  TD-38 is paid.
- **An always-on QA explorer.** Anthropic runs QA agents, still experimental, that keep testing the running system
  ("constantly hammering on that thing") against a description a person wrote, and open a PR when something is
  broken (The Pragmatic Engineer, 2026-11-25). That is the evaluator as a standing explorer, not a gate on each
  change, which fits the owner's V1 override (explorer on demand, capped per run). A standing run has the same
  confinement question as the evaluator in CI; revisit with it.
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
- 2026-11-25 — S2 built, ADR 0048 written, status `proposed`. It answers D0's findings: plain exit-0 output does not
  reach the model, so a passing notice is a `systemMessage`; the `timeout` is explicit (30 s, the gate's own limit is
  15 s by the owner's ruling); the stamp's key holds untracked files, the merge base and `STACK`
  (`tool-versions.env` is in the tree); a second blocking Stop hook is accepted; the rule id is
  `harness-stop-is-green`. S2's text above was changed to match. The owner ruled on 2026-11-25:
  an unchanged tree that is red or not done blocks once; the key also covers ignored files, the tools in `.bin/` and git filters
  (0.45 s a stop instead of 0.2 s); TD-178 is a harness PR of its own.
- 2026-11-26 — V1 run: pilot (1.28 USD), then one round, approved up to 40 USD, for 28.15 USD. The keep bar was
  missed once 019 story 2 was scored (19.03 USD); the owner chose to finish the round for full data. Result
  and deviations in ADR 0046. Owner's rulings, which override ADR 0046's rule for a missed bar (tune or
  drop): no tuning round; `code-reviewer` does not gain the
  scenario-driving variant; the evaluator stays as an explorer run on demand, cap 3 USD per run, measured by
  Phase 3's evals instead of a second trial; new slice V5, built first, gives scenarios a user sees tests
  against the running app. Still open from V1: handing the cases to the product's owner, then removing the
  clone.
- 2026-11-26 — V5 built: ADR 0049 proposed, with the owner's rulings (all A; question 4 for now, TD-179: a
  later tester agent confirms the red). `specs-tested` traces a scenario a user sees only by a journey or an
  integration test, `test-writer` writes them, and `specs-tests-frozen` freezes them unchanged. Reviewed in two
  rounds; the evasion review's bypasses (the race tag, directories `./...` skips, a nested `go.mod`, a marker in
  a link, disguised placeholders) are closed, except `go.mod` `ignore` lines (TD-180). No budget row: 0.03 s.
- 2026-11-26 — read The Pragmatic Engineer's newsletter and podcast transcript with Katelyn Lesse and
  Anthropic's Rust migration post (both 2026-11-25); added to Sources. They change no slice. They back the
  explorer override of V1 (QA agents run all the time, not per change; "An always-on QA explorer" above) and
  keeping human review as a gate (in the Rust migration "the agent found the cheapest way" to match outputs).
  The migration's lessons on ported tests went to plan 0005's Risks.
