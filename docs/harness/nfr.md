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
cannot meet NFR-R1. They comment, never fail ([ADR 0019](../adr/0019-inferential-sensors-are-advisory.md)).

The rows come from [exec plan 0002](../exec-plans/active/0002-fast-reliable-harness.md) (NFR-H1–H7, NFR-R1–R5)
and [exec plan 0003](../exec-plans/active/0003-harness-gap-closure.md) (NFR-H8, NFR-H9, NFR-B1, NFR-B2,
NFR-O1). "Today" is the last measurement, with its date and the slice that took it; update it in place when
you re-measure.

## Plan 0002: latency and reliability

| Id | Requirement | Target | Today | Measured by | On violation |
|----|-------------|--------|-------|-------------|--------------|
| NFR-H1 | Each PreToolUse hook, per call | ≤ 50 ms | 29 ms (protect-harness, Bash), 34 ms (Edit), 26 ms (guard-bash), 25 ms (only-make-check); median of 9, timing included (2026-10-28, step 8, load avg 11–17); protect-harness Edit 38 ms in this checkout, 49 ms for a path in another worktree (one `git worktree list`), mean of 20 (2026-10-29, load avg 7–8); 24 ms (only-test-files, allowed and blocked Write; median of 9, timing included; 2026-10-30, plan 0003 P5 gate, load avg 3–4); protect-harness in Go: 30 ms (Bash), 30 ms (Edit, allowed or blocked), 36 ms (Edit in another worktree), median of 9, timing included (2026-11-02, plan 0005 pilot 2, load avg 6–8; 40, 36 and 48 ms before on the same machine) | hook wrapper timing | warn; alert |
| NFR-H2 | fast-check (PostToolUse), per edited file | ≤ 1 s | `.md` 0.08 s, `.sh` 0.14 s, `.go` 0.33 s, `.tsx` in apps/web 0.90 s (was 1.05 s, over budget; 2026-10-28, step 8, load avg 11–17); `.swift` in apps/ios 0.24–0.30 s, SwiftLint and swift-format (2026-11-01, command-line tools only, load avg 11) | fast-check timing | warn; alert |
| NFR-H3 | `make check`, change without shell or harness files | ≤ 10 s | tsx edit 2.1 s; Go edit 11.2 s, **over budget**: `arch-go` is not scoped (plan 0002, A4) and go-arch-lint took 9.4 s (load avg 11–17; 2.2 s at P11) (2026-10-28, step 8) | `run.sh` + make wall time | warn; alert |
| NFR-H4 | `make check`, any change | ≤ 60 s | 173 s, **over budget**: harness change, `bats` 169 s (load avg 7–12; 2026-11-02, exec plan 0005 pilot); was 52 s with 120 bats tests (2026-10-28, step 8) | same | warn; alert |
| NFR-H5 | one sensor in `make check` | ≤ 5 s, or its row in `budgets.txt` | `bats` 169 s (399 tests in 45 files, 390 in 44 since plan 0005 pilot 2; file by file 172 s, the slowest `agent-gc` 12.0 s, `pre-commit` 11.8 s, `pr-review` 11.7 s, `scheduled-review` 10.5 s, `mutation` 9.0 s), `harness-tests` 11.3 s uncached, 0.5 s cached (load avg 7–12; 2026-11-02, exec plan 0005 pilot); `go-arch-lint` 7–9 s (load avg 11–17; 2.2 s at P11; 2026-10-28, step 8); rest ≤ 5 s | `run.sh` | warn; alert |
| NFR-H6 | `make check` rerun, nothing changed | ≤ 3 s | 1.8 s (2026-10-28, step 8, load avg 11–17; was 3.7 s) | two runs back to back | warn; alert |
| NFR-H7 | `make check-all` in CI | ≤ 10 min | not measured | job duration | alert |
| NFR-R1 | Deterministic: same tree, same verdict and same findings | 0 differences | 0 (`make harness-determinism`, scope full; since step 8 tsc and vitest run uncached, `turbo --force`; 342 s, load avg 11–17, 2026-10-28) | nightly double run | **fail** (nightly) + alert |
| NFR-R2 | Fail closed: a sensor that cannot run (missing tool, base ref, parse error) exits non-zero | 0 silent passes | merge-base, affected.sh, gen-check stamp, integrity-check (shebang and mode changes count), every hook on a missing or failing binary (2026-10-28); no fixtures with a broken environment yet | fixtures with a broken environment | **fail** |
| NFR-R3 | Detection: every rule with a fixture rejects it ([ADR 0009](../adr/0009-harness-selftest.md)); measured on the full stack (every group `layout.env` declares), where a skipped fixture fails ([ADR 0033](../adr/0033-stack-profiles.md), [ADR 0042](../adr/0042-swift-unit-tests-in-make-check.md)) | 100 % | 100 % (selftest 44/44, 2026-10-28, gate of Phase 0) | `make harness-selftest` | **fail** |
| NFR-R4 | Actionable: every sensor failure through `run.sh` names its rule id and remediation; every hook block names its reason | 100 % | 100 % | `run.bats`, selftest, hook bats tests and Go hook tests (`protectharness_test.go`) | **fail** |
| NFR-R5 | Portable: same verdict on macOS and Linux, except the targets in `HOST_OFF` ([ADR 0042](../adr/0042-swift-unit-tests-in-make-check.md)) and, on a Mac with only the command-line tools, `IOS_XCODE_TARGETS` ([ADR 0043](../adr/0043-swift-lint-format-and-xcode-pin.md)) | 0 differences | 0: Linux in CI; macOS, with `ios` in `STACK`, green in every finished `check-ios` run on main since the first, 2026-11-02 (`b526181`), last checked 2026-11-05 (`360b672`); `check-ios` skips the fixtures that need Docker (ADR 0044) | `make harness-selftest` on Ubuntu (`ci.yml`) and on macOS with Xcode selected (`ios.yml`, job `check-ios`), on each PR that touches Swift, `IOS_DIR` or a harness file and on each push to main ([ADR 0044](../adr/0044-ios-ci-on-macos.md)) | **fail** |

## Plan 0003: latency, behaviour and observability

| Id | Requirement | Target | Today | Measured by | On violation |
|----|-------------|--------|-------|-------------|--------------|
| NFR-H8 | `pre-commit` git hook, typical commit (≤ 20 staged files) | ≤ 2 s | 0.57 s warm, 10 staged files (Q2) | `timing.sh` row `pre-commit`, local only (the hook never runs in CI) | warn |
| NFR-H9 | `repo-no-secrets` in `make check` | ≤ 5 s | 0.43 s warm, full tree (Q1) | `run.sh`, budgets row `repo-no-secrets 5000 NFR-H9` | warn; alert. In place: `check` scans only the changed files (`HARNESS_SCOPE=changed`), the full tree only when a harness file changed; if that full scan stays over budget, it moves to `check-all` |
| NFR-B1 | Every domain created by `make new-domain` has a passing service test, handler test and integration test before its first edit; applies only with `db` in `STACK` ([ADR 0033](../adr/0033-stack-profiles.md)) | 100 % | 100 %: the `zzz_probe` scaffold's 3 tests green on generation (2026-10-28, P2.1, local run) | CI `generator-probe` job ([ci.md](ci.md)): `make test-scaffold` runs the generated tests, red unless each ran and passed, none skipped | **fail** |
| NFR-B2 | The app's critical path is covered by an e2e test that runs against this worktree's `make dev`; applies only with `db` and `web` in `STACK` ([ADR 0033](../adr/0033-stack-profiles.md)) | ≥ 1 journey (`/healthz` + web shell renders) | 1 journey, green: `make e2e` 5.4 s starting `make dev` itself (Go build cache warm), 1.1 s attached to a running one (2026-10-29, P2.3, local; CI not measured yet) | `make e2e` in `check-all` and CI ([web-e2e-journeys](rules.md#web-e2e-journeys)) | **fail** |
| NFR-O1 | An agent in a fresh worktree can fetch the log lines of one request by its ID and a p95 latency, through `make` targets only | yes | yes, tested: `make test-obs` green, image present: `obs-pull` 0.1 s, `obs-e2e` 13.8 s with the stack started and removed, 5 s attached to a running stack; its request's access line via `make logs`, its server span via `make traces`, the p95 of `/healthz` from that API process (label `instance`) via `make metrics` (2026-10-29, P1.5, local; CI pull not measured) | `make test-obs` in `check-all` and CI ([obs-request-queryable](rules.md#obs-request-queryable)): request → `make logs REQUEST_ID=…`, `make traces`, `make metrics` | **fail** |
