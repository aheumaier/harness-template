# ADR 0049: Scenarios a user sees are traced by tests that run the app (amends 0036, 0039)

Status: proposed · 2026-11-26 · amends [ADR 0036](0036-acceptance-scenarios-are-traced-to-tests.md) and
[ADR 0039](0039-tests-are-written-apart-from-the-implementation.md)

Slice V5 of [exec plan 0006](../exec-plans/active/0006-anthropic-harness-gaps.md), gap A1, built first in Phase 1
by the owner's ruling of 2026-11-26 ([ADR 0046](0046-agent-verdicts-are-advisory-and-measured.md), Outcome of the
trial). On acceptance the notes on ADR 0036 and ADR 0039 lose their "proposed" and name what was ruled; if this
ADR is withdrawn, the notes are removed. Each question with two options ends in the owner's ruling; on
2026-11-26 the owner ruled the recommended option on all five, the fourth for now (question 4).

## Context
- **ADR 0036 counts any test.** `specs-scenarios-tested` passes a scenario when its id appears in any product test
  file. A scenario about an HTTP header, a status code or what a page shows passes with a unit test that never
  starts the app.
- **V1 saw that gap** (ADR 0046, Trial result). On `strength_planner`'s spec 002 the product's own checks were
  green, and the evaluator failed three scenarios in the running app: no `ETag` header, 409 where the spec asks
  for 412, an athlete can open the rename box. The product's owner has not confirmed them yet. The owner moved
  the weight from the evaluator's verdict to fixed tests against the running app.
- **ADR 0039 keeps those tests with the implementer.** `test-writer` writes only unit tests, because its trial
  covered only those. Journeys and integration tests are written by the agent that also writes the code, so they
  can be fitted to it.
- **What exists, on `main` and this branch:**
  - `make e2e` ([ADR 0025](0025-browser-e2e-is-the-nfr-b2-sensor.md)): Playwright journeys, collected from
    `apps/web/e2e/` by `testMatch: '*.e2e.ts'`, against this worktree's `make dev`, in `make check-all`, only with
    `db` and `web` in `STACK` (`e2e` is in both `DB_TARGETS` and `WEB_TARGETS`,
    [ADR 0033](0033-stack-profiles.md)). One journey today.
  - `make test-integration` (sensor id `go-integration`): Go tests under `//go:build integration` against this
    worktree's Postgres, in `make check-all`, only with `db`. One today,
    `apps/api/internal/platform/db/db_integration_test.go`. A new domain's scaffold adds
    `internal/<domain>/api/api_integration_test.go`, which sends HTTP requests through the domain's routes and
    `requestid.Middleware`, with every layer on the real Postgres.
  - `make check` type-checks both without running them: `apps/api/.golangci.yml` sets `build-tags: [integration]`
    for `lint-go`, and `apps/web/tsconfig.json` includes `e2e/` for `typecheck-web`.
  - `specs-tests-frozen` ([ADR 0047](0047-test-writer-tests-are-frozen.md)) freezes every product test file a test
    commit wrote. A journey and an `integration`-tagged `*_test.go` are product test files, so a journey or
    integration test from `test-writer` is frozen today, with no code change. Any change to spec.md lifts the
    freeze for that spec's tests, with a `notice:`.
  - `test-writer`'s write hook (`only-test-files.sh`) already lets it write journeys and integration tests; its
    prompt forbids them. Its shell hook (`only-make-check.sh`) allows `make check` and `make check-all` only.

## Questions
The plan asks six questions. Its first one (how a scenario is recognised, and which test kinds count) is split
here into questions 1 and 2, so there are seven. Each answer gives a recommendation and its cost. Where two answers
are both sound, it gives two options with an example and leaves the ruling to the owner. The template has no
`specs/`, so the spec examples are made up.

### 1. Which scenarios are ones a user sees
Spec-kit's template writes every story as a user journey (`### User Story 1 - [Brief Title] (Priority: P1)`), and
ADR 0036 already reads a marker at the end of a scenario, `(untested: <reason>)`. The plan forbids ids in the spec
(ADR 0036, A17), not markers. So the choice is the default. Under either option, a marker ends the scenario (as
`(untested: …)` does), and it is in spec.md before the test commit: added later, it is a spec change, which lifts
the freeze on every test of that spec (ADR 0047).

- **Option A (recommended): every scenario is one a user sees, unless it says otherwise.** A scenario that a unit
  test may prove ends with `(unit: <reason>)`, under the same rules as `(untested: …)`: an empty reason or a
  placeholder (`n/a`, `tbd`, …) is a finding. A scenario carries at most one of the two markers; both is a finding.
  Example, in a spec `specs/001-cart/spec.md`: `3. **Given** a cart of 3 items at 9.99, **When** the total is
  shown, **Then** it is 29.97 (unit: a pricing rule of packages/domain; the page prints the value it gets)`.
  Cost: every other scenario needs a journey or an integration test. Spec-kit never writes the marker, so each new
  spec needs one pass over its scenarios before `test-writer` runs, and a forgotten marker shows as a red
  `make check` after the test commit. Then either `test-writer` adds a journey or an integration test in a new
  test commit, or the marker is added and the freeze on that spec's tests lifts until its next test commit. An adopter with specs sees `make check` red on upgrade, one finding per
  scenario only unit tests trace. The template has no specs, so `make adoption-probe` is not affected. This fails
  closed: the V1 scenarios (a header, a status code) are the kind nobody would have marked as "a user sees this".
- **Option B: only a scenario marked as one a user sees.** It ends with `(app)`; every other scenario keeps ADR
  0036's "any test". Example: `2. **Given** a stale programme, **When** the coach renames it, **Then** the API
  answers 412 (app)`. Cost: nothing breaks on upgrade. But spec-kit's template and `/speckit-specify` are vendored
  and unedited ([ADR 0011](0011-conventional-commits-and-spec-kit.md)) and never write the marker, so until a
  constitution line asks for it (a human harness PR, like [TD-124](../exec-plans/tech-debt-tracker.md)) the rule
  does not fire, and a forgotten marker is silent. Spec 002's `ETag` scenario would most likely not have been
  marked.

A constitution line would cut the cost of either option. It is TD-124's kind of change and not part of this slice.

Ruling (owner, 2026-11-26): A.

### 2. Which test kinds count
Both options count journeys: a test file Playwright collects (`apps/web/e2e/*.e2e.ts`), with `db` and `web` in
`STACK`. A journey's `request` fixture also checks what the API returns, as `critical-path.e2e.ts` does for
`/healthz`. They differ in the Go tests that count for what the API returns.

- **Option A (recommended): integration tests in a domain's `api` package.** A `*_test.go` under
  `apps/api/internal/<domain>/api/` whose build constraint selects it with the tag `integration` and drops it
  without (decided as `go test -tags=integration` decides, by `go/build` on the host's OS and architecture; a
  `//go:build` line below `package` is no constraint). Example: the scaffold's `api_integration_test.go`, which
  sends `GET /<domain>/status` through the domain's routes against Postgres. It runs in `make check-all`. Cost:
  the kind is read from the path and the build line, not from what the test does. A test there that calls the
  service instead of sending a request counts too. `cmd/api`'s middleware chain (authentication, logging,
  telemetry) is not in the path, so a header set there, or a 401 without a token, is traced only by a journey. This is "an API test against the running server"
  of the plan and ADR 0046 in its nearest readable form: the domain's routes and the real database, not the whole
  binary.
- **Option B: also the untagged handler tests in the same package.** The scaffold's `api_test.go` sends requests
  through the generated strict server and the real service with a fake store. They run in `make check`, so a 412
  or a missing `ETag` goes red in seconds, and they work without `db`. Cost: a fake store hides what the database
  decides (a version check in SQL); the approval tests in the same package, written by the implementer, would count
  too; and it is not "against the running app" as the owner ruled.

Not an option: any Go test that uses `httptest`. `apps/api/cmd/api/main_test.go` wraps the real middleware chain
around its own mux, and `internal/platform/requestid/requestid_test.go` wraps a stub handler; no line tells them
apart.

Ruling (owner, 2026-11-26): A.

### 3. How a frozen journey survives the page it was written against
ADR 0047 already gives a way out: a later test commit may change what an earlier one wrote. So the implementer
never edits a journey. It changes the page, or the plan, and sends the journey back to `test-writer`. The question
is how to make that rare.

- **Option A (recommended): plan.md pins what a journey depends on,** as it pins Go names (ADR 0039):
  - the route, and the role and accessible name of each control the scenarios use, the texts the spec states, a
    `data-testid` only when plan.md names it. Journeys find elements by role, label or text
    (`page.getByRole('button', { name: 'Rename' })`), never by CSS structure;
  - how a journey signs in as each role the spec names (V1's "an athlete can open the rename box" is a role
    scenario), and the principal an integration test puts in the request context with `auth.WithPrincipal`
    (the domain's routes have no sign-in: `cmd/api` wires authentication);
  - nothing about data: each test creates its own uniquely named data and checks nothing global, since tests
    share the worktree's database with `make dev`.

  A name plan.md does not pin makes `test-writer` stop with `plan.md does not pin …`, as today. Cost: a longer
  plan.md; one more `test-writer` run when a pinned name turns out wrong. Spec-kit's plan template has no section
  for it (as for Go names, TD-124), so the stop will be frequent at first.
- **Option B: `test-writer`'s journeys are not frozen.** `specs-tests-frozen` skips `*.e2e.*` files under
  `Test-Writer:` (one condition) and keeps freezing an `App-Evaluator:` journey; `code-reviewer` check 7 judges
  edits. Cost: the implementer may loosen what a journey expects as freely as how it finds the page, so the gap ADR
  0047 closed opens again for exactly the tests this ADR adds. It also amends ADR 0047 (its header, a note there,
  this title).

Ruling (owner, 2026-11-26): A.

### 4. What "confirms red" means for a test of a page that does not exist yet
- An integration test is type-checked by `make check` (`lint-go` with `build-tags: [integration]`). A pinned name
  that does not exist is `undefined: …`, the red ADR 0039 already accepts.
- A journey has no such signal. It type-checks green against a page that is not there; it is red only when it
  runs, and then possibly for the wrong reason (no server, a sign-in that fails, a typo in an earlier step, or no
  test selected at all). `test-writer` cannot run `make e2e` or `make test-integration`: its shell hook allows
  `make check` and `make check-all`, and the plan gives the widening of that hook to slice V2.

For a new or changed operation, the API does not build after `make gen` until the new `StrictServerInterface`
method exists (add-endpoint, step 4): `make dev` does not start, and the domain's `api` package does not compile.
So the commit with the contract change also holds the smallest body for each new method, one that returns an
error (the strict server answers 500). Then the server builds and each test can fail at its own step.

The right reason: the test fails at the first step that needs what the feature adds (a locator for a pinned name
that times out, a 404 from a new route, a status or header the spec states), not at its setup. Two rules make that
checkable. A journey carries its scenario id in its `test('…')` title, not only in a `// spec:` comment, because
`E2E_GREP` selects by title. And no test is written with `test.fixme`, `.skip` or `t.Skip`: a skip on the line
that names the id makes the test name nothing for `specs-scenarios-tested`, and `specs-tests-frozen` reports a skip
added later.

- **Option A (recommended): the caller confirms the run.** `test-writer` confirms what `make check` shows, as
  today. Before committing the tests, the caller runs `make e2e E2E_GREP='<NNN>/'` and `make test-integration`,
  with no `make dev` running (Playwright attaches to one, and its API does not reload), checks that each new test
  ran, and quotes, per test, the step that failed in the test commit's message. Cost: one
  manual run per test commit. A red journey costs 5 to 30 s: a failing `expect` gives up after 5 s, an action
  on a missing element waits out the 30 s test timeout (the config sets neither), so eight new journeys cost up to
  about four minutes; the integration tests add seconds. The caller judges
  "right reason".
- **Option B: `test-writer` runs them itself.** Its shell hook gains `make e2e` and `make test-integration` now,
  through the target list V2 plans for `only-make-check.sh`. Cost: each run needs Docker and starts `make dev`; the
  agent judges its own red, unreviewed. It moves part of V2 into this slice (plan 0006 changes), and the hook change
  lands only after [plan 0005](../exec-plans/active/0005-harness-scripts-in-go.md) slice 5 has ported that hook
  (plan 0006, Rules for every slice).

Ruling (owner, 2026-11-26): A, for now. Not the final answer: the owner sees running and judging these tests as
the job of a tester agent ([TD-179](../exec-plans/tech-debt-tracker.md)).

### 5. Whether `test-writer`'s wider scope needs its own trial
ADR 0039 limited the scope to unit tests because its trial covered only those, and plan 0006 makes every new agent
earn its place by a trial. Widening `test-writer` is not a new agent, but it is a scope no trial covered.

- **Option A (recommended): no trial before building.** The prompt is widened, the ADR says the wider scope is
  unmeasured, and the re-measure on the first adopter ([TD-126](../exec-plans/tech-debt-tracker.md)) is widened to
  cover journeys and integration tests too. If it shows that they cost more corrections than they catch, the scope
  falls back to ADR 0039's, rather than the agent being dropped. Reasons: on 2026-11-26 the owner chose measuring
  over further trial rounds for the evaluator; the template has no business domain to trial on, so a trial needs a
  product again. Cost: the wider scope rests on no evidence of its own until TD-126 runs.
- **Option B: a trial first, before `test-writer` changes.** Two features of a product with journeys, each built
  once with journeys from `test-writer` and once with journeys from the implementer, with a keep bar fixed before
  the run (for example, fewer scenarios that fail in the running app while their journeys are green). The sensor of
  question 2 could still be built now. Cost: about what V1 cost (28.15 USD for the round, 1.28 USD for the pilot)
  and a working day, and a product owner's consent. If ruled, V5 writes the design and stops before running it.

Ruling (owner, 2026-11-26): A.

### 6. What traces these scenarios on a stack without `db` and `web`
No option needed: a test kind counts only where its make target runs
([NFR-B2](../harness/nfr.md) applies journeys only with `db` and `web`).

| `STACK` has | Counts for a scenario a user sees |
|-------------|-----------------------------------|
| `db` and `web` (the template) | journeys and integration tests |
| `db`, no `web` (an API-only product) | integration tests; there is no web app to show anything |
| no `db` (`go`, `go:web`, `go:ios`) | any test, as ADR 0036 says; the sensor prints one `notice:` line, which `run.sh` shows for a green sensor |

- Under option B of question 1, an `(app)` marker on a stack without `db` is a finding instead, since nothing
  could ever trace it.
- **iOS**: what the iOS app shows is out of scope, as it is for the evaluator (plan 0006, V2). Its UI tests are
  not run by `make check` ([ADR 0042](0042-swift-unit-tests-in-make-check.md)). An integration test covers the API
  half; otherwise the scenario says `(unit: shown in the iOS app, whose UI tests make check does not run)`.
- `HARNESS_STACK` overrides `STACK` for the sensor as for every other one.

Cost: on a product without `db` this ADR changes nothing, which is honest, since no test there runs the app.

### 7. What the extra tests cost in the budget rows
No option needed.
- **`e2e`** (30 s, NFR-H7): measured on 2026-10-29, one journey, 5.4 s when Playwright starts `make dev` itself (Go
  cache warm), 1.1 s attached to a running one; `playwright.config.ts` runs one worker. A green journey with one
  page load and one API call costs well under 1 s, so the row warns at a few dozen journeys. A red one costs 5 to
  30 s (question 4). ADR 0046 already notes that evaluator journeys grow the same row.
- **`go-integration`** has no row, so the 5 s default (NFR-H5) applies. `go test -race -count=1` with Postgres
  reaches it sooner than journeys reach 30 s.
- `make check` does not change: journeys and integration tests run in `make check-all` only.

Decided: leave both alone; V5's tick records the measured times; a `go-integration` row comes when its
warning first fires, with the measurement. When a row warns, first look at running tests in parallel or sharing
their setup (the Playwright config is protected: a harness PR), before raising the budget.

## Decision
Written to the rulings of 2026-11-26.

- **`specs-scenarios-tested` gains one finding** (question 1 A, 2 A, 6). With `db` in `STACK`, a scenario that
  carries neither `(unit: …)` nor `(untested: …)` passes only when a test of a kind question 6 allows names its id.
  A scenario named only by other tests is a finding:
  `specs/<NNN-slug>/spec.md:<line>: <id> is a scenario a user sees and only other tests name it (<file>); name it
  in a journey (apps/web/e2e/*.e2e.ts) or an integration test in internal/<domain>/api/ (//go:build integration),
  or end it with (unit: <reason>); a marker added after the test commit lifts the freeze on this spec's tests
  (specs-tests-frozen), so prefer a new test commit [specs-scenarios-tested]`.
  A scenario no test names keeps today's finding. Both markers on one scenario, found anywhere in its text, is a
  finding. Same rule id, same sensor (`.bin/harness specs-tested`, [ADR 0045](0045-harness-scripts-and-tests-in-go.md)),
  same place in `make check`.
- **Why the same rule id.** It reads the same input with the same marker family, and the sensor already counts
  tests per stack group (Swift tests only with `ios`). The cost: [ADR 0032](0032-rules-that-never-fire-are-a-drift-report.md)
  counts findings per rule id, so its drift report cannot tell whether the new finding ever fires on an adopter.
  Accepted: the fixture and the Go tests prove it fires, and an adopter sees it in `make check`'s output.
- **It ships complete** (plan 0006, Rules for every slice):
  - rules.md: the rule text, the remediation, and the Status line linking this ADR next to ADR 0036;
  - Go tests in `scenarios_test.go` for each branch: journey, integration test in `api/`, one outside `api/`, a
    negated tag, a tag below `package`, unit test only, each marker, both markers, a stack without `db`, one
    without `web`; with `GOOS` and `GOARCH` fixed, so they pass alike on every machine;
  - the fixture `scripts/harness/fixtures/specs-user-scenario-unit-tested` with `STACK=db` in its `fixture.conf`
    ([ADR 0009](0009-harness-selftest.md)), so a stack without `db` skips it;
  - a Go test that keeps the sensor's table equal to what runs it: `test-integration` in `DB_TARGETS`, `e2e` in
    `DB_TARGETS` and `WEB_TARGETS`, and Playwright's `testDir` and `testMatch`. Otherwise a test no target runs, or
    a `*.e2e.ts` outside `apps/web/e2e/`, would count. It is the pattern TD-149 proposes for the test-file names.
    The web app's directory has no `layout.env` key yet ([TD-46](../exec-plans/tech-debt-tracker.md)), so the
    sensor names `apps/web` as the Makefile's `e2e` target does, and TD-46 lists it;
  - the surfaces that state the old rule: the Makefile's `specs-tested` help text, `test-writer.md`,
    `apps/ios/AGENTS.md` (a Swift test no longer traces a scenario a user sees when `db` is in `STACK`),
    `README.md` (which groups change what the rule counts) and the rule's row in `docs/applying-the-harness.md`.

  Time target: today 0.01 s without specs and 0.03 s with one; the change reads the header of Go test files the
  sensor reads already, so it stays far below the 5 s default and needs no `budgets.txt` row.
- **`test-writer` writes journeys and integration tests too** (question 3 A, 4 A, 5 A), from the spec, before the
  implementation, after the contract change and `make gen` are committed (golden principle 1: a journey's setup and
  an integration test use the generated client and types). It locates page elements only by names plan.md pins,
  puts the scenario id in the test title, and writes each integration test in a new file: ADR 0047 reports a
  `//go:build` line added to a file a test commit wrote. It confirms red as today. Its reply tells the caller to run
  `make e2e E2E_GREP='<NNN>/'` and `make test-integration`, check that each test ran, and quote the failing step per
  test in the test commit; and, after the implementation, to run them again before calling the story done, since
  `make check` does not run them. Handler tests with `approvaltest` and XCTest UI tests stay with the implementer.
  The hooks do not change. The cost cap per run stays (4 test files, 3 `make check` runs, 30 minutes); a feature
  with more tests gets one run per story, as ADR 0039 already allows.
- **Budget rows** (question 7): no change; a `go-integration` row comes with its first warning.
- **`specs-tests-frozen` needs no code.** A journey or an integration test in a test commit is frozen like a unit test.
  A Go test in `specsfrozen_test.go` proves it for an edited journey line.
- **Tracker edits:** TD-126's re-measure covers journeys and integration tests, and a loss narrows the scope back
  instead of dropping the agent; TD-124's proposed constitution and CLAUDE.md lines also name the `(unit: …)`
  marker and the routes, roles, accessible names and sign-in plan.md pins; TD-46 adds the sensor to the places
  that name `apps/web`; TD-149 stays open, with the stack-group test as a second user of its pattern.

## What this changes in ADR 0036 and ADR 0039
- **ADR 0036, "any whole-token occurrence in a test file counts".** With `db` in `STACK`, for a scenario without
  `(unit: …)`, only an occurrence in a test of a kind question 6 allows counts.
- **ADR 0036, the exemption.** A second marker, `(unit: <reason>)`, with the same rules for its reason. It does not
  exempt the scenario from a test; it lets any test trace it.
- **ADR 0039, "Scope: unit tests that `make check` runs".** Journeys and integration tests in a domain's `api`
  package are in scope, after the contract and `make gen` are committed. Approval-test handler tests stay out.
- **ADR 0039, the flow and "confirms red".** The caller confirms a journey's and an integration test's red by
  running them, and quotes the failing step in the test commit.
- **ADR 0039, "plan.md pins every new name".** Also the routes, roles, accessible names, texts and sign-in a
  journey uses.
- **ADR 0039, the write guard.** Unchanged: it already allows these files. Slice V2 narrows the same hook for
  `app-evaluator` with an argument, after plan 0005 has ported it; `test-writer` keeps the wide default.

## Consequences
- An agent that traces a scenario a user sees with a unit test sees `make check` red with the id, the scenario and
  the test file, and goes back to the test step on its own.
- A feature costs more tests and more time in `make check-all`: about one journey or integration test per scenario
  a user sees. `make check` stays as fast as it is.
- **Between the test commit and `make check-all`, `make check` stays green while a journey is red.** The stop gate
  ([ADR 0048](0048-stop-gate.md)) stamps `make check` only. `make check-all` before a PR (golden principle 6) and
  `test-writer`'s reply close that gap; nothing enforces it per story.
- **Blind spots, left to review** ([ADR 0019](0019-inferential-sensors-are-advisory.md)):
  - the kind is read from the path, the file name and the build line: an integration test in `api/` that never
    sends a request, or a journey that only checks the page title, counts;
  - who wrote the test is not checked: a journey the implementer fitted to its code counts. Keeping tests apart
    rests on `test-writer`'s prompt and `code-reviewer` check 7;
  - whether a `(unit: …)` reason holds is the reviewer's call, as for `(untested: …)`.
- An adopter with specs pays once on upgrade, and each new spec pays one pass over its markers (question 1,
  option A). The finding names each scenario and the fix.
- The wider `test-writer` scope is unmeasured until TD-126 runs (question 5, option A).
- The caller's manual run to confirm red is provisional (question 4): TD-179 holds the owner's direction, a tester
  agent that runs these tests and judges their red, and is decided with slice V2's hook argument or after it.
- Not decided here: the evaluator's agent file, its hooks and its wiring (plan 0006, V2 to V4).

## Alternatives considered
- **Guess from the scenario's words** ("page", "shows", "clicks", "returns"). Rejected: it fails both ways, and
  silently.
- **A marker on the story heading or its `**Independent Test**:` paragraph.** Rejected: one story often holds a
  rule scenario next to a screen one, and the heading's form is spec-kit's.
- **A list of scenario ids kept outside the spec.** Rejected for the reason ADR 0036 gives: a marker in the spec
  stays visible in review.
- **Journeys only.** Simpler, and a journey can check the API through its `request` fixture. Rejected: an API-only
  product (`go:db`) would have no kind left.
- **A new rule id for the new finding.** Lets ADR 0032's drift report see it. Rejected: a second rule over the same
  scenarios and markers, documented and suppressed apart, for one line in a weekly report.
- **`specs-tests-frozen` ignores a spec change that only adds or removes a marker.** Would let a marker come after the
  test commit. Rejected for now: more code in a sensor for a step the flow can order.
- **Leave it to the app evaluator.** Rejected by the owner (ADR 0046, Outcome of the trial): its verdict is
  advisory and changes with the model; a committed test does not.
