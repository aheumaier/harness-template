# Harness NFRs

What the harness promises, as numbered, measurable requirements. Latency is p95 on the reference machine (CI
`ubuntu-24.04` runner), warm caches, measured by `run.sh`; the same budgets apply locally, where they only
warn. Budgets land in `scripts/harness/budgets.txt` (protected, kind `config`: raising a budget is a
loosening and needs a human, [harness-config-not-loosened](rules.md#harness-config-not-loosened)).

| Class | Meaning | On violation | Defined by (proposed) |
|-------|---------|--------------|------------|
| H, latency | the harness is fast enough to run after every change | warn; alert ([harness-latency-budget](rules.md#harness-latency-budget)) | [ADR 0013](../adr/0013-harness-nfr-budgets.md) |
| R, reliability | a sensor's verdict can be trusted | **fail** | [ADR 0013](../adr/0013-harness-nfr-budgets.md) |
| B, behaviour | the app does what it should, proven by tests that run | **fail** | [ADR 0015](../adr/0015-anticipated-rules-and-nfr-classes-b-o.md) |
| O, observability | an agent can read the running app through `make` targets | **fail** | [ADR 0015](../adr/0015-anticipated-rules-and-nfr-classes-b-o.md) |

Latency NFRs warn and alert but never fail a build: wall time depends on the machine, and a sensor that fails
under load would itself break NFR-R1. Reliability NFRs fail, because a wrong verdict is the one thing a sensor
must not produce. Inferential sensors get **no** blocking NFR: an LLM verdict is not reproducible, so it
cannot meet NFR-R1. They comment, never fail.

The rows come from [exec plan 0002](../exec-plans/active/0002-fast-reliable-harness.md) (NFR-H1–H7, NFR-R1–R5)
and [exec plan 0003](../exec-plans/active/0003-harness-gap-closure.md) (NFR-H8, NFR-H9, NFR-B1, NFR-B2,
NFR-O1). "Today" is the last measurement, with its date and the slice that took it; update it in place when
you re-measure.

## Plan 0002: latency and reliability

| Id | Requirement | Target | Today | Measured by | On violation |
|----|-------------|--------|-------|-------------|--------------|
| NFR-H1 | Each PreToolUse hook, per call | ≤ 50 ms | 29 ms (protect-harness, Bash), 34 ms (Edit), 26 ms (guard-bash), 25 ms (only-make-check); median of 9, timing included (2026-10-28, step 8, load avg 11–17) | hook wrapper timing | warn; alert |
| NFR-H2 | fast-check (PostToolUse), per edited file | ≤ 1 s | `.md` 0.08 s, `.sh` 0.14 s, `.go` 0.33 s, `.tsx` in apps/web 0.90 s (was 1.05 s, over budget; 2026-10-28, step 8, load avg 11–17) | fast-check timing | warn; alert |
| NFR-H3 | `make check`, change without shell or harness files | ≤ 10 s | tsx edit 2.1 s; Go edit 11.2 s, **over budget**: `arch-go` is not scoped (plan 0002, A4) and go-arch-lint took 9.4 s (load avg 11–17; 2.2 s at P11) (2026-10-28, step 8) | `run.sh` + make wall time | warn; alert |
| NFR-H4 | `make check`, any change | ≤ 60 s | 52 s (shell change, `bats` 49 s; load avg 11–17; 36 s at P11) (2026-10-28, step 8) | same | warn; alert |
| NFR-H5 | one sensor in `make check` | ≤ 5 s, or its row in `budgets.txt` | `bats` 36–58 s (120 tests), `go-arch-lint` 7–9 s (load avg 11–17; 2.2 s at P11); rest ≤ 5 s (2026-10-28, step 8) | `run.sh` | warn; alert |
| NFR-H6 | `make check` rerun, nothing changed | ≤ 3 s | 1.8 s (2026-10-28, step 8, load avg 11–17; was 3.7 s) | two runs back to back | warn; alert |
| NFR-H7 | `make check-all` in CI | ≤ 10 min | not measured | job duration | alert |
| NFR-R1 | Deterministic: same tree, same verdict and same findings | 0 differences | 0 (`make harness-determinism`, scope full; since step 8 tsc and vitest run uncached, `turbo --force`; 342 s, load avg 11–17, 2026-10-28) | nightly double run | **fail** (nightly) + alert |
| NFR-R2 | Fail closed: a sensor that cannot run (missing tool, base ref, parse error) exits non-zero | 0 silent passes | merge-base, affected.sh, gen-check stamp, integrity-check (shebang and mode changes count), every hook on a missing or failing binary (2026-10-28); no fixtures with a broken environment yet | fixtures with a broken environment | **fail** |
| NFR-R3 | Detection: every rule with a fixture rejects it ([ADR 0009](../adr/0009-harness-selftest.md)) | 100 % | 100 % (selftest 32/32, also in scope full as CI runs it, 2026-10-28) | `make harness-selftest` | **fail** |
| NFR-R4 | Actionable: every sensor failure through `run.sh` names its rule id and remediation; every hook block names its reason | 100 % | 100 % | `run.bats`, selftest, hook bats tests | **fail** |
| NFR-R5 | Portable: same verdict on macOS and Linux | 0 differences | Linux only in CI | selftest on a macOS runner (weekly) | **fail** (weekly) + alert |

## Plan 0003: latency, behaviour and observability

| Id | Requirement | Target | Today | Measured by | On violation |
|----|-------------|--------|-------|-------------|--------------|
| NFR-H8 | `pre-commit` git hook, typical commit (≤ 20 staged files) | ≤ 2 s | 0.57 s warm, 10 staged files (Q2) | `timing.sh` row `pre-commit`, local only (the hook never runs in CI) | warn |
| NFR-H9 | `repo-no-secrets` in `make check` | ≤ 5 s | 0.43 s warm, full tree (Q1) | `run.sh`, budgets row `repo-no-secrets 5000 NFR-H9` | warn; alert. If it stays over budget, the full scan moves to `check-all` and `check` scans changed files only |
| NFR-B1 | Every domain created by `make new-domain` has a passing service test, handler test and integration test before its first edit | 100 % | 0 domains, scaffold has no tests | CI `generator-probe` job ([ci.md](ci.md)) runs the generated tests | **fail** |
| NFR-B2 | The app's critical path is covered by an e2e test that runs against this worktree's `make dev` | ≥ 1 journey (`/healthz` + web shell renders) | none | `make e2e` in `check-all` and CI | **fail** |
| NFR-O1 | An agent in a fresh worktree can fetch the log lines of one request by its ID and a p95 latency, through `make` targets only | yes | no | integration test in `check-all`: request → `make logs REQUEST_ID=…` | **fail** |
