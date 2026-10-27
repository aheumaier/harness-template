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
  exec plan [0002](../exec-plans/active/0002-fast-reliable-harness.md#nfrs) until the harness PR moves them to
  `docs/harness/`.
- Budgets live in `scripts/harness/budgets.txt`, a protected config: raising a budget is a loosening
  (`harness-config-not-loosened`) and needs a human.
- `run.sh` and the hooks record their own duration on every run (`.cache/harness/timings.jsonl`). One
  choke point measures every sensor, so no sensor can opt out.
- **Latency violations alert, never fail.** Locally: one warning line naming the sensor, the time, the budget
  and the NFR. CI: a `::warning` annotation and job summary row. Trend: a nightly job computes p95 over the
  last 7 days and opens or comments on one `harness-nfr` issue.
- **Reliability violations fail.** Non-determinism (NFR-R1, nightly double run), a silent pass when a sensor
  cannot run (NFR-R2), a fixture not rejected (NFR-R3) or a failure without rule id and remediation (NFR-R4)
  fail the job that finds them.

## Consequences
- Slowdowns show up as an issue with the sensor name and p95 before anyone complains, and the measurements
  show which phase of the plan to do next.
- Failing on wall time would make the harness flaky on loaded machines, so latency is never a gate; a sensor
  that keeps breaking its budget is fixed through the `harness-nfr` issue, not by a red build.
- Timings stay local or in CI artifacts; nothing leaves the repo's own CI.
- Adds the rules `harness-latency-budget` and `harness-deterministic`, each with a fixture (ADR 0009).
