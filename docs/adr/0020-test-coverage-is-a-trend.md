# ADR 0020: Test coverage is a trend, never a gate

Status: proposed · 2026-10-28

## Context
Exec plan 0003 (P2.5) asks for test coverage per package. Nothing reports it today, so a package whose
tests are deleted or never written goes unnoticed. A coverage threshold is the usual answer, and it has a
known failure mode, sharper with coding agents: once the number blocks a merge, the cheapest way to green
is a test that executes code without asserting anything. The number then rises while the tests get worse,
and the gate hides exactly the gap it was meant to show.

## Decision
- Unit-test statement coverage is **reported per package**: per Go package of `apps/api` and per JS
  workspace, by `make coverage` (`scripts/harness/coverage.sh`, [ci.md](../harness/ci.md#coverage)).
  - Every PR and push to `main`: the table in the job summary of the `coverage` workflow, which is not a
    required status check: a failure to measure turns it red but never blocks a merge (amended 2026-10-29,
    exec plan 0003 Phase 2 gate: `continue-on-error` hid that failure behind a green check).
  - Weekly: `drift.yml` compares it with the last scheduled run's and adds the packages whose coverage fell
    to the `drift` issue.
- **No threshold anywhere**: not in `make check` or `check-all`, not in a required check, not in a
  workspace's test config (`thresholds` in Vitest, a minimum in any Go tool). `coverage.sh` fails only when
  it cannot measure (missing tool, failing tests, unparseable input), never on a percentage. It is a report,
  not a sensor, so it has no rule id in [rules.md](../harness/rules.md).
- What is measured is fixed in protected code: `coverage.sh` sets the Vitest provider and reporter and
  excludes generated Go files; a workspace only provides a `coverage` script.
- A **ratchet** (coverage may not fall) or any other gate on the number needs its own ADR, justified by a
  finding: a defect that a covered test would have caught, and that the weekly drift issue failed to surface
  in time ([ADR 0015](0015-anticipated-rules-and-nfr-classes-b-o.md)).

## Consequences
- A drop shows up once a week in the `drift` issue, with the package and the change in percentage points;
  a human or the garbage-collection agent decides whether it matters.
- Nothing forces coverage up, so a low number can stay low; that is deliberate until a finding says
  otherwise.
- Unit tests only (`go test -short`, Vitest): code covered only by integration tests reads as uncovered,
  for example `platform/db`.
- The `coverage` job re-runs the unit suites outside `check-all`, so it does not count against the
  `check-all` job's time budget (NFR-H7).
