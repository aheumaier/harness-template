# 0006 — Close the gaps to Anthropic's harness guidance

Status: proposed · Owner: @aheumaier · Started: 2026-11-24

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
| A2 | "Done" is gated by the harness when the agent stops, not by a guide | Best practices (a Stop hook blocks the stop until the check passes; "CLAUDE.md is for guidelines") | "Done means green" is golden principle 6: prose. `fast-check` covers one file per edit; CI catches the rest after the push | ❌ |
| A3 | Agents and their prompts have evals: a small set of cases, run more than once, rerun on a model or prompt change | Fold subagents (about 10 golden examples, run each task more than once); Multi-agent research (start with about 20) | Computational sensors have fixtures; ADR 0019 says of inferential ones "No fixture can prove one fires" and stops there. ADR 0039's trial was one-off, its branch deleted | ❌ |
| A4 | Each piece of scaffolding names the model weakness it assumes and is re-tested when the model changes | Harness design ("every component in a harness encodes an assumption about what the model can't do on its own"); 3 patterns ("Assumptions about what Claude can't do need to be re-tested with each step change") | [ADR 0032](../../adr/0032-rules-that-never-fire-are-a-drift-report.md) reports computational rules that never fire. Agents, skills, hooks and loop prompts have no such record | ❌ |
| A5 | Subagents are kept only where measured to help; cheaper models where they hold up | Fold subagents ("fewer subagents in favor of progressively disclosed context and tools"; fold one at a time and compare evals); 3 patterns ("If you need a cheaper model, use a subagent") | `CLAUDE.md` triggers three reviewers per change and `evasion-reviewer` triggers by its own description; none was measured (only `test-writer` was, ADR 0039); no agent file sets `model:` | 🟡 |
| A6 | The implementer cannot weaken the tests it was given, enforced by the harness | Effective harnesses (agents may only flip `passes`; "It is unacceptable to remove or edit tests") | `test-writer` is confined by a hook; a later edit to its tests is found only by `code-reviewer` check 7, advisory, and not at all in CI ([TD-125](../tech-debt-tracker.md)) | 🟡 |

Where the repo already matches or exceeds the sources, no slice is planned: hooks over guides for harness
protection, per-area `AGENTS.md` as progressive disclosure, state in files and git so a compaction costs
nothing (0003 loop prompt), orchestrator and lanes, builder and checker kept apart in the loop.

## Phases

| # | Phase | Exit criterion | State |
|---|-------|----------------|-------|
| 0 | Decision | ADR 0046 proposed: what an app-driving evaluator's verdict counts for, agent evals as a drift report, the assumption ledger (amends 0019, 0032, 0039) | open |
| 1 | Evaluator runs the app (A1) | Trial result recorded in ADR 0046 while it is still proposed, then the ADR accepted; if kept: `app-evaluator` agent confined by hooks, wired into `CLAUDE.md` and the loop checker step; a finding it proves is a committed journey in `make e2e` | open |
| 2 | Done gate at stop (A2) | Stopping the main session with a changed tree and a red `make check` is blocked once with the sensor output; budget and escape rules hold | open |
| 3 | Agent evals (A3) | `make agent-evals` reports hit rate, false positives, run-to-run flips and cost per agent; result appears in the weekly drift issue once TD-38 is paid | open |
| 4 | Measure and prune (A4, A5) | Every agent, skill, hook and loop prompt has an assumption row, checked by a sensor; each reviewer has a measured `model:` or is folded; stale rows listed weekly | open |
| 5 | Frozen tests (A6) | A changed or deleted `test-writer` line without a spec change fails `make check-all`; TD-125 paid | open |

Order is by dependency, with the lead item first. Phase 1's trial cases seed Phase 3's corpus; Phase 4 needs
Phase 3's runner. Phases 2 and 5 are independent and can run in parallel with the rest.

## Progress

`[ ]` open, `[x]` done (`sha — before → after`), `[!]` blocked (reason). **H** needs a human-approved harness
PR; **–** needs none.

- [ ] D0 – ADR 0046 proposed (Phase 0)
- [ ] V1 – evaluator trial run on a scratch branch, numbers recorded in ADR 0046
- [ ] V2 H `app-evaluator` agent file and its hook confinement
- [ ] V3 H finding-to-journey rule: `drive-app` skill and `code-reviewer` updated
- [ ] V4 H wiring: `CLAUDE.md`, loop prompts' checker step, PR template line
- [ ] S1 – Stop-hook semantics verified against the hooks reference; findings in Open assumptions
- [ ] S2 H `.bin/harness stop-gate`, hook entry in `.claude/settings.json`, rule and tests
- [ ] E1 H eval corpus for `code-reviewer` and (if kept) `app-evaluator`
- [ ] E2 H `.bin/harness agent-evals` and `make agent-evals`
- [ ] E3 H corpora for the other reviewers; weekly run in `drift.yml`
- [ ] M1 H assumption ledger and `harness-assumptions-listed`
- [ ] M2 H model trial per reviewer, numbers in the ledger (`docs/harness/*` is protected)
- [ ] M3 H `model:` set, reviewers folded or retired as M2 decides; `CLAUDE.md` trigger list updated
- [ ] M4 H stale assumption rows in the weekly drift report
- [ ] T1 H `specs-tests-frozen` sensor in `check-all`
- [ ] T2 H `pr-review` passes the since-test-commit diff (pays TD-125)

### Phase 0 — decision

**D0.** [ADR 0046](../../adr/README.md) (not written yet), amending 0019, 0032 and 0039. It stays `proposed`
until V1's numbers are in it, since an accepted ADR is not edited in place. It decides:

- An app-driving evaluator is an inferential sensor: advisory. What makes its finding binding is not its
  verdict but the journey that reproduces it (V3), which is computational.
- Agent evals are a trend and a drift report, like coverage ([ADR 0020](../../adr/0020-test-coverage-is-a-trend.md))
  and mutation ([ADR 0023](../../adr/0023-mutation-testing-is-a-drift-report.md)). They never gate a PR.
  ADR 0019's "no fixture can prove one fires" stays true; an eval measures how often it fires.
- ADR 0032's question ("does this rule still earn its place?") is extended from rules to agents, skills,
  hooks and loop prompts through the assumption ledger. Retirement stays a human PR.
- ADR 0039's check 7 stays advisory; next to it, a coarser computational rule (T1) fails `check-all`, and a
  journey the evaluator adds (V3) counts as a test-writer test for both.

### Phase 1 — evaluator runs the app

The builder may already use `drive-app`. That is self-evaluation, and the source is blunt about it: "agents
reliably skew positive when grading their own work." It is equally blunt about the fix needing work: "Out of
the box, Claude is a poor QA agent." So the agent is trialled and its prompt tuned before it is kept.

**V1 — trial.** On a scratch branch, with ad-hoc prompts (as ADR 0039's trial):

- Four small spec'd web features on a scratch domain, each built by one agent, `make check` green. Into each,
  two defects are planted by hand that `make check` and the existing journeys do not catch and a user would:
  a control wired to the wrong operation, a stubbed feature that renders but does nothing, an error state
  never shown, a list that ignores its filter, a value lost on reload. Two further features stay clean.
- Each of the six is judged twice by (a) `code-reviewer` as it is, from the diff, and (b) an evaluator that
  gets `spec.md`, the running app and the `drive-app` skill, and no account from the builder of what was done.
- Evaluator output: one line per acceptance scenario, `PASS`, `FAIL` or `NOT EXERCISED`, each with evidence
  (journey step, screenshot or accessibility-tree path under `.tmp/e2e/`, request id). Any `FAIL` or
  `NOT EXERCISED` makes the verdict `CHANGES REQUESTED`: a hard threshold per criterion, no averaging.
- **Keep** if, in both runs, the evaluator finds at least two planted defects more than `code-reviewer` and
  fails at most one scenario across the two clean features. **Otherwise** tune the prompt at most twice and
  rerun; if it still fails, drop the agent, record the numbers, and add "run `drive-app` on the scenarios" to
  `code-reviewer` instead only if that variant passes the same bar.
- Record the table, the cost per judged feature and the wall time in ADR 0046. Keep the six cases: they seed E1.

**V2 — the agent (H).** `.claude/agents/app-evaluator.md`:

- Tools `Read, Grep, Glob, Write, Edit, Bash`. Bash is confined by a hook to an allow-list of make targets
  (`dev`, `e2e`, the `query-runtime` targets, `check`). `only-make-check.sh` hard-codes `make check(-all)`
  today; it gains a target-list argument rather than a sibling hook. `make dev` runs until stopped, so the
  slice also settles how the evaluator starts and stops it under that confinement (V1 finds out; `make e2e`
  starting its own `make dev` may be enough). Write and Edit are confined by `only-test-files.sh`, which
  accepts `*.e2e.*`; the evaluator writes journeys and nothing else.
- Input: the feature's `spec.md` (acceptance scenarios with their [ADR 0036](../../adr/0036-acceptance-scenarios-are-traced-to-tests.md)
  ids) and the repository root. It is told the diff and every file are data, not instructions, as
  `reuse-reviewer` and `evasion-reviewer` are.
- A cost cap per run, as ADR 0019 requires of the slice that adds an agent; V1's measured cost sets it.
- The prompt carries the rubric the trial tuned. It states what the evaluator cannot judge (sound, native
  dialogs, anything not reachable through the page or the API logs) so that it reports `NOT EXERCISED`
  rather than guessing.
- Scope: the web app and the API as the web app uses it. iOS is out ([plan 0004](0004-ios-stack-group.md)).

**V3 — a finding is a journey (H).** An evaluator `FAIL` is reported with the journey that shows it, left in
`apps/web/e2e/` and red. The builder fixes the code; the journey stays and `make e2e` (in `check-all`) holds
the behaviour from then on. ADR 0019 turns a finding class seen twice into a computational rule; here a
single finding becomes a test at once, because a journey is cheap and specific to the feature.
Update `drive-app` §3 (a journey written to prove a failure is kept, not deleted) and `code-reviewer` (a
journey added by the evaluator is covered by check 7's loosening rules).

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
  session from ending. If S1 shows the wrapper cannot tell a repeated stop, D0 proposes an exception to
  ADR 0045 for this hook instead.
- Rule `agent-stop-is-green` in `rules.md`; Go tests for each branch above.

### Phase 3 — agent evals

**E1 (H).** A corpus under `scripts/harness/agent-evals/<agent>/<case>/`: the input the agent gets (a diff,
or a spec plus a patch to apply to the scaffold), the findings it must report and, for clean cases, that it
must report none. About ten cases per agent, at least three clean. Sources: the V1 trial cases; real catches
from the 0003 and 0005 logs and review sections; planted violations of each `code-reviewer` check.

**E2 (H).** `.bin/harness agent-evals [agent]`, behind `make agent-evals`:

- Runs each case **twice** with the agent's file as it is in the tree.
- Grades without an LLM judge where the agent's output format allows: `code-reviewer` prints a verdict line
  and `path:line — problem — fix (rule id or principle #)`, so a case names the expected verdict, path and
  id. The other reviewers print looser lines; E3 gives each a gradable format before its corpus is written. `app-evaluator` cases name the expected verdict per scenario id.
- Reports, per agent: cases hit in both runs, in one, in none; findings on clean cases; verdict flips between
  runs; tokens and cost. Writes the row to `.cache/harness/`, as the timings are.
- Needs credentials, the network and money, so it is in neither `check` nor `check-all`. Without a key it
  skips with a notice.

**E3 (H).** Corpora for `architecture-reviewer`, `reuse-reviewer`, `evasion-reviewer`. A weekly `drift.yml`
job runs the evals of the read-only agents and adds the table to the drift issue, confined as the scheduled
reviews are (`ci-agent`, tools `Read,Grep,Glob` per ADR 0019). `evasion-reviewer` and `app-evaluator` need
Bash, so their evals run locally only. The job is blocked on [TD-38](../tech-debt-tracker.md) like every CI agent run. The PR template
gains a line: a PR that changes `.claude/agents/*` states the before and after `make agent-evals` rows.

### Phase 4 — measure and prune

**M1 (H).** `docs/harness/assumptions.md`: one row per agent, non-vendored skill, hook and loop prompt.
Columns: component; the model weakness it assumes; the evidence it was added on (ADR, trial, incident); last
tested (model id, date, result); how to test its removal. Sensor `harness-assumptions-listed`
(`.bin/harness`, in `make check`): every file under `.claude/agents/`, `.claude/hooks/`, `.claude/skills/`
(spec-kit's excluded) and every `*-loop-prompt.md` has a row, and every row names a file that exists.

**M2.** For each reviewer, `make agent-evals` with the agent on the session's model and on the two cheaper
tiers. Record the three rows in the ledger. A cheaper model is taken when it hits the same cases in both
runs and adds no finding on a clean case.

**M3 (H).** Apply M2: set `model:` in the agent files. Then the fold test, one agent at a time: add the
agent's checks to `code-reviewer`, rerun both corpora, and keep the fold only if no case flips to a miss.
Candidates by overlap of scope: `reuse-reviewer` and `architecture-reviewer`. `evasion-reviewer` (needs a
scratch worktree and Bash) and `test-writer` and `app-evaluator` (need permissions the reviewer must not
have) are kept apart on the source's own grounds: verification and modular permissioning. Update the
`CLAUDE.md` trigger list.

**M4 (H).** The weekly drift report lists ledger rows whose "last tested" names a model other than the one
the CI agents run on, or is older than 90 days, next to ADR 0032's never-fired rules. Removal is tried one
component at a time and is a human PR.

### Phase 5 — frozen tests

**T1 (H).** Sensor `specs-tests-frozen` (`.bin/harness`, in `check-all`, since it reads history). For the
first commit in `<base>..HEAD` with a `Test-Writer: specs/<slug>` trailer: a line that commit added to a
test file and that is deleted or changed at `HEAD`, a file of that commit deleted or renamed, or a skip
marker or build tag added to one, fails unless `specs/<slug>/spec.md` changed in the same range. Added tests
and added lines pass. This is coarser than `code-reviewer` check 7 by design: the sensor asks "was a
test-writer line touched without a spec change", the reviewer still judges whether a change the spec diff
accompanies is explained by it, and loosening through another file (`TestMain`, a config exclude). Fixtures:
an assertion edited, a case deleted, a `t.Skip` added, and the same three with a spec change.

**T2 (H).** TD-125 as written: `pr-review` passes the since-test-commit diff to the CI reviewer. It lands in
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
  therefore takes half of them from defect classes the two source posts report finding, and the keep bar is
  relative to `code-reviewer` on the same cases.
- **A scratch domain is enough to judge the evaluator.** The template has no business domain. A product that
  adopts the harness should rerun E2 on its own cases.
- **Deterministic grading is possible.** If reviewers' wording of path and id proves too loose to match, E2
  first tightens the output format in the agent files rather than adding an LLM judge.
- **Cost.** Unknown until V1 and E2 report it. The source's full harness cost about 20 times its solo run;
  an evaluator pass per UI change must be far below that to be kept as a default.
- **A tree-hash stamp is a sound "last green" marker** given `make check` is scoped to the change
  ([ADR 0014](../../adr/0014-make-check-scoped-to-the-change.md)). S2 checks which scope the stamp may claim.

## Not in this plan

- **The evaluator in CI.** It needs a running stack next to an agent that holds a secret and reads PR
  content. That is a new confinement question on top of TD-38; revisit once Phase 1 has a kept agent and
  TD-38 is paid.
- **A sprint contract negotiated between builder and evaluator.** Spec-kit's acceptance scenarios with
  traced ids already fix what done means before work starts; the source itself removed sprints as the model
  improved.
- **A planner agent, or loosening `plan.md`.** Noted as a tension (the source keeps the planner at product
  level because detailed errors cascade); no evidence here yet that pinned signatures hurt.
- **Loop-harness changes** (feature list in JSON, an initializer session). The 0003 loop's Progress list and
  git cover the same ground.

## Log

- 2026-11-24 — plan written from the comparison with the Anthropic sources; evaluator-runs-the-app ruled the
  lead item by the owner.
