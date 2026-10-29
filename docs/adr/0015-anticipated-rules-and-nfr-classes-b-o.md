# ADR 0015: Phase 0 rules are adopted as team-convention; NFR classes B and O fail

Status: proposed · 2026-10-28

## Context
Exec plan 0003 (`docs/exec-plans/*/0003-harness-gap-closure.md`) closes gaps between this template and its
sources. Its Phase 0 adds five rules that no incident in this repo has earned yet. Core belief 3
([core-beliefs.md](../design-docs/core-beliefs.md)) says "new rules are earned by real mistakes, not
anticipated", and [rules.md](../harness/rules.md) asks every new rule to link the incident that motivated
it. It also names the other path: **team-convention**, a convention the team already follows, adopted by an
ADR instead of an incident. These five need that ADR.

Plan 0003 also sets targets for the app's behaviour (tests, e2e) and for runtime legibility (logs by request
ID). [ADR 0013](0013-harness-nfr-budgets.md) defines only two NFR classes: latency (H, warn) and reliability
(R, fail). These targets fit neither.

## Decision
- **Adopted as `team-convention`**, each with its source:
  - `repo-no-secrets` and `deps-no-known-vulns`: Böckeler, *Harness engineering for coding agent users*
    (GitLeaks in pre-commit). `deps-no-known-vulns` extends what `go-no-known-vulns` already does for the
    root Go module to JS dependencies and every `go.mod`.
  - `go-sec`: Böckeler (SAST as a computational sensor).
  - `go-structured-logging`: the invariant `apps/api/internal/platform/logging/logging.go:9` already states
    ("All logs are structured; never use fmt.Print for diagnostics"). The sensor enforces it.
  - `rules-fixture-claims`: the doc drift plan 0003 found. rules.md claims fixtures that do not exist, so
    the selftest ([ADR 0009](0009-harness-selftest.md)) does not cover what rules.md says it covers.
- This is consistent with core belief 3. The belief is about invariants and their sensors. These are
  invariants (no secrets in the tree, no known vulns, no unstructured logs, no false fixture claims), not
  style. Each still ships with a rules.md entry, a remediation and a fixture, or `none (<reason>)`. Any
  later rule beyond these five still needs an incident or its own ADR.
- **Amends ADR 0013 with two NFR classes**, both of which **fail** on violation, like R:
  - **B, behaviour:** the app does what it should, proven by tests that run.
  - **O, observability:** an agent can read the running app through `make` targets.
  - The first ones are NFR-B1 (generated domains ship passing tests), NFR-B2 (one e2e journey against
    `make dev`) and NFR-O1 (logs by request ID and a p95 latency, per worktree). They are listed in plan
    0003 until Q5 moves them to `docs/harness/nfr.md`.
- B and O sensors that need Docker, a browser or the running app go in `check-all` and CI, not in
  `make check`.
- **Inferential sensors get no blocking NFR.** An LLM verdict is not reproducible, so it cannot meet NFR-R1.
  Such sensors comment and never fail ([ADR 0019](0019-inferential-sensors-are-advisory.md)).

## Consequences
- The five Phase 0 rules land as `Status: team-convention` with a link to this ADR in place of an incident,
  so rules.md keeps them apart from rules earned by an incident.
- A B or O violation turns `check-all` and CI red: a missing test or a missing `make logs` fails the job
  that finds it, just as a flaky sensor does (R). Latency (H) is still the only class that only warns.
- ADR 0013 stays as written. Its class list is read together with this ADR until both NFR tables live in
  `docs/harness/nfr.md`.
- Anticipating rules stays the exception. Size and complexity limits are a different question: they need
  core belief 3 amended (plan 0003, P4.1), not this ADR.
