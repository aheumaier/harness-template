# ADR 0013: Harness NFRs are budgets; latency alerts, reliability fails

Status: proposed · 2026-10-27

## Context
Agents run the hooks on every tool call and `make check` after every change, so harness speed is agent
throughput, and a wrong verdict undermines every rule. Today nothing states what "fast" or "reliable" means,
and nothing notices when it degrades: `make check` grew to 42 s (bats on macOS) without any sensor reporting
it, and the documented "< 60 s warm" is checked by nobody. Degradation is gradual (more code, more tests,
tool upgrades), so no single PR is to blame and no PR review catches it.

## Decision
- The harness has numbered NFRs (`NFR-H*` latency, `NFR-R*` reliability) with measurable targets, listed in
  exec plan 0002 (`docs/exec-plans/*/0002-fast-reliable-harness.md`, section NFRs) until the harness PR moves
  them to `docs/harness/`.
- Budgets live in `scripts/harness/budgets.txt`, a protected config: raising a budget is a loosening
  (`harness-config-not-loosened`) and needs a human.
- `run.sh` and the hooks record their own duration on every run (`.cache/harness/timings.jsonl`,
  gitignored). One choke point measures every sensor, so no sensor can opt out. Milliseconds come from
  `EPOCHREALTIME` where bash has it and from `perl -MTime::HiRes` on macOS `/bin/bash` 3.2; a failure to time
  or write never changes a sensor's exit status.
- **Latency violations alert, never fail.** Locally: one warning line naming the sensor, the time, the budget
  and the NFR. CI: a `::warning` annotation and job summary row. Trend: a nightly job computes p95 over the
  last 7 days and opens or comments on one `harness-nfr` issue.
- **Reliability violations fail** the job that finds them: non-determinism (NFR-R1, nightly double run), a
  silent pass when a sensor cannot run (NFR-R2), a fixture not rejected (NFR-R3), a failure without rule id
  and remediation (NFR-R4), a different verdict on macOS (NFR-R5, weekly macOS job). Jobs that run off the PR
  path (nightly, weekly) fail their workflow and open or comment on the `harness-nfr` issue.

## Consequences
- Slowdowns show up as an issue with the sensor name and p95 before anyone complains, and the measurements
  show which phase of the plan to do next.
- Failing on wall time would make the harness flaky on loaded machines, so latency is never a gate; a sensor
  that keeps breaking its budget is fixed through the `harness-nfr` issue, not by a red build.
- Timings stay local or in CI artifacts; nothing leaves the repo's own CI.
- Adds two rules. `harness-deterministic` gets a selftest fixture (ADR 0009): a sensor whose verdict
  alternates through a counter file, which the double run must reject; no randomness, so the fixture itself
  stays deterministic. `harness-latency-budget` only warns, and the selftest asserts a non-zero exit, so it is
  marked "no fixture" in rules.md and tested by `run.bats` instead (warning printed, exit status unchanged).
