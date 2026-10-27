# 0002 — Fast, reliable harness

Status: proposed · Owner: @aheumaier · Started: 2026-10-27

## Goal

Keep `make check` and the per-edit hooks fast enough that agents run them after every change, today and in a
2–4 MLOC codebase, without loosening any rule. Order of preference: **don't run it** (scope, cache) → **run it
faster** → **rewrite it**. Every item touches protected files, so each ships as its own human-approved
harness PR.

## Baseline (2026-10-27, macOS, warm)

| What | Time | Note |
|------|------|------|
| `make check` | 42 s | wall time ≈ `test-sh` alone |
| `test-sh` (bats, 89 tests) | 42–60 s | ~0.35 s per *copied* script on first exec (below) |
| every other `check` target | 0.06–2.9 s each | run in parallel (`-j4`) |
| `run.sh` overhead | 16 ms | per sensor |
| PreToolUse hooks | 7 ms (protect-harness) + 32 ms (guard-bash) | per Bash call; 20 ms of that is `node` |
| fast-check (PostToolUse) | 70 ms `.sh`, 310 ms `.md`, 340 ms `.go` | `.ts`: eslint + whole-package `tsc --noEmit` |
| session context | ~9 KB (≈2.3k tokens) root guides + 13 skill descriptions | nested AGENTS.md load on demand |

**Root cause of slow bats:** `sandbox()` in `scripts/test/helpers.bash` copies the scripts under test into a
temp repo. macOS assesses every newly written executable on its first exec: a copied `check-specs.sh` takes
340–370 ms, the same script linked takes 30 ms. Measured on a scratch copy of the repo: linking executables
takes the suite **from 60 s to 19 s**. A plain symlink is **unsafe**: `integrity-check.bats` "flags a change to
sensor code" appends to the sandboxed `merge-base.sh` and wrote through to the real file. Linux CI is not
affected.

## Scaling to 2–4 MLOC

Bash cost is a constant (~16 ms per sensor) or scales with the size of the diff (`check-specs`, `check-commits`,
`integrity-check`, `migrations-append-only`). Two exceptions: `check-links` forks per link (≈4 ms each; tens
of thousands of links → minutes), and the bats suite. What scales with the repo are the **full-repo runs**:

| Sensor | Today | At scale | Fix |
|--------|-------|----------|-----|
| `test-go` | `go test -race -count=1 ./...` | `-count=1` disables Go's test cache: every package reruns every time | phase 1 |
| `lint-web` | `eslint .`, no cache | relints every file | phase 1 |
| `arch-web` | `depcruise apps packages`, no cache | rebuilds the whole import graph | phase 1 |
| fast-check `.ts` | `tsc --noEmit` on the whole package per edit | tens of seconds per edit | phase 1 |
| `arch-go`, `lint-go` | whole module (golangci-lint caches; go-arch-lint does not) | grows linearly | phase 2 |
| `gen-check` | regenerates everything | grows with spec and queries | phase 1 |

**Architecture rules can be scoped to the change.** Layer and boundary rules (go-arch-lint, depguard,
custom analyzers, dependency-cruiser, eslint boundaries) judge import edges, and a new edge can only appear in
a changed file. Checking changed packages is therefore sound, as long as a change to a rule config or to
`go.mod`/`package.json` runs the full check and CI keeps running everything (`check-all`).

## NFRs

What the harness promises. Latency is p95 on the reference machine (CI `ubuntu-24.04` runner), warm caches,
measured by `run.sh` (phase 0); the same budgets apply locally, where they only warn. How a violation is
reported: [ADR 0013](../../adr/0013-harness-nfr-budgets.md). Numbers land in `scripts/harness/budgets.txt`
(protected, kind `config`: raising a budget is a loosening and needs a human).

| Id | Requirement | Target | Today | Measured by | On violation |
|----|-------------|--------|-------|-------------|--------------|
| NFR-H1 | PreToolUse hooks, per call | ≤ 50 ms | 39 ms | hook wrapper timing | warn; alert |
| NFR-H2 | fast-check (PostToolUse), per edited file | ≤ 1 s | 0.07–0.34 s (`.ts`: whole-package tsc) | fast-check timing | warn; alert |
| NFR-H3 | `make check`, change without harness files | ≤ 10 s | 42 s | `run.sh` + make wall time | warn; alert |
| NFR-H4 | `make check`, any change | ≤ 60 s | 42–60 s | same | warn; alert |
| NFR-H5 | one sensor in `make check` | ≤ 5 s, or its row in `budgets.txt` | ≤ 2.9 s except `test-sh` | `run.sh` | warn; alert |
| NFR-H6 | `make check` rerun, nothing changed | ≤ 3 s | ≈ first run (no skip, `-count=1`) | two runs back to back | warn; alert |
| NFR-H7 | `make check-all` in CI | ≤ 10 min | not measured | job duration | alert |
| NFR-R1 | Deterministic: same tree, same verdict and same findings | 0 differences | not measured | nightly double run | **fail** (nightly) + alert |
| NFR-R2 | Fail closed: a sensor that cannot run (missing tool, base ref, parse error) exits non-zero | 0 silent passes | partly (merge-base, guard-bash) | fixtures with a broken environment | **fail** |
| NFR-R3 | Detection: every rule with a fixture rejects it ([ADR 0009](../../adr/0009-harness-selftest.md)) | 100 % | 100 % | `make harness-selftest` | **fail** |
| NFR-R4 | Actionable: every failure names its rule id and remediation | 100 % | 100 % via `run.sh` | `run.bats`, selftest | **fail** |
| NFR-R5 | Portable: same verdict on macOS and Linux | 0 differences | Linux only in CI | selftest on a macOS runner (weekly) | alert |

Latency NFRs warn and alert but never fail a build: wall time depends on the machine, and a sensor that fails
under load would itself break NFR-R1. Reliability NFRs fail, because a wrong verdict is the one thing a sensor
must not produce.

## Phases

| # | Phase | Exit criterion | State |
|---|-------|----------------|-------|
| 0 | Measure and alert: timings, budgets, NFR issue | every run of `run.sh` and the hooks records its time; a violation warns locally and reaches the `harness-nfr` issue | proposed |
| 1 | Do less: skip and cache (no new code) | NFR-H3, NFR-H6 met | proposed |
| 2 | Scope `make check` to the affected set | NFR-H3 holds on a 10× larger fixture repo; CI unchanged | proposed |
| 3 | Go harness binary for parsing sensors ([ADR 0012](../../adr/0012-parsing-sensors-in-go.md)) | ported sensors pass their fixtures; bats tests replaced by Go tests; NFR-H1 met without `node` | proposed |
| 4 | Agent loop latency | NFR-H2 met for every file type | proposed |

## Progress

One slice per commit, in this order (the [loop prompt](0002-loop-prompt.md) works through it). `[ ]` open,
`[x]` done (`sha — before → after`), `[!]` blocked (reason).

- [ ] P1 (phase 0) `run.sh` records timings to `.cache/harness/timings.jsonl` and warns over budget;
  `scripts/harness/budgets.txt`; bats tests
- [ ] P2 (phase 0) hook timing helper; CI `::warning` + job summary; timings uploaded as artifact
- [ ] P3 (phase 0) nightly `harness-nfr` workflow (p95 over 7 days, double run for NFR-R1, one issue);
  rules `harness-latency-budget`, `harness-deterministic` with fixtures
- [ ] P4 (phase 1) `lint-sh`, `fmt-check-sh`, `test-sh` in `check` only when shell/harness files changed
- [ ] P5 (phase 1) bats sandbox: symlink executables, copy on write, teardown asserts the repo is unchanged
- [ ] P6 (phase 1) no `-count=1` in `check`; eslint and depcruise caches; `gen-check` skipped on unchanged inputs
- [ ] P7 (phase 1/4) fast-check: no whole-package `tsc` per edit
- [ ] P8 (phase 3) Go module `apps/api/tools/harness` + Makefile staleness rule; port `guard-bash`
- [ ] P9 (phase 3) port `check-links`
- [ ] P10 (phase 3) port `integrity-check`
- [ ] P11 (phase 2) `affected.sh`; scoped Go and web targets; full run on config changes

### Phase 0 — measure and alert

1. `run.sh` already wraps every sensor: it appends one line per run to `.cache/harness/timings.jsonl`
   (gitignored): sensor, milliseconds, exit code, scope (`check`/`check-all`/`fast-check`), git sha, OS. The
   hooks do the same through a shared helper. Cost: one `date` and one append.
2. `scripts/harness/budgets.txt`: `<sensor-or-hook> <budget-ms> <nfr-id>`, default 5000 ms for sensors.
3. Over budget, `run.sh` prints one line to stderr and exits with the sensor's own status:
   `harness: sensor 'lint-web' took 7.2 s (budget 5 s, NFR-H5)`. In CI it also emits a `::warning`
   annotation and a row in the job summary, and uploads `timings.jsonl` as an artifact.
4. A nightly job (next to `drift.yml`) collects the artifacts of the last 7 days of CI runs, computes p95 per
   sensor and hook, runs `make check` twice on `main` and diffs the verdicts and findings (NFR-R1). It opens
   or comments on one issue labelled `harness-nfr` with the violations, same as the `drift` issue. No
   violation, no comment.
5. Rules `harness-latency-budget` (NFR-H*) and `harness-deterministic` (NFR-R1) in `docs/harness/rules.md`,
   each with its remediation and a fixture: a sensor that sleeps past its budget must warn; a sensor with a
   random verdict must fail the double run.

### Phase 1 — do less

1. **Harness sensors only when the harness changed.** Agents cannot edit `scripts/**` or `.claude/**`
   (golden principle 5), so `lint-sh`, `fmt-check-sh` and `test-sh` in `make check` check nothing an agent
   could have broken. Run them in `check` only when `git diff --name-only $(merge-base)` plus untracked files
   touch `*.sh|*.bash|*.bats|scripts/|.claude/hooks/`; `check-all` and CI always run them. Saves ~40 s per
   `make check`.
2. **Bats sandbox without the first-exec penalty**, for harness PRs: `sandbox()` symlinks executables and
   copies data; the write helpers (`add`, `sed -i`) materialize a link (`cp --remove-destination`) before
   writing; a `teardown` asserts `git -C "$REPO" diff --quiet -- scripts .claude` so any write-through fails the
   test. Alternative with no code change (unverified): add the terminal app under macOS *Privacy & Security →
   Developer Tools*, which exempts its child processes from the assessment.
3. **Use the caches tools already have:** drop `-count=1` from `test-go`/`test-analyzers` in `check` (keep it
   in `check-all`/CI; integration tests keep it); `eslint --cache --cache-location node_modules/.cache/eslint`;
   `depcruise --cache node_modules/.cache/depcruise`.
4. **`gen-check` only when its inputs changed:** stamp a hash of `api/openapi.yaml`, `apps/api/db/queries/**`,
   `apps/api/db/migrations/**`, generator configs and `tool-versions.env`; skip when the stamp matches and no
   generated file is modified.
5. **fast-check stops typechecking the whole package per edit:** eslint on the file only; `tsc` stays in
   `make check` (`--incremental` with a tsbuildinfo under `node_modules/.cache`).

### Phase 2 — affected set

- `scripts/harness/affected.sh` prints the change set once (merge base with `BASE_REF` + working tree +
  untracked), the Go packages that changed, their reverse dependencies (`go list -deps -test`), and the JS
  workspaces (`turbo --filter=...[BASE_REF]`).
- Scoped targets: `test-go`, `lint-go`, `arch-go` on affected packages; `lint-web`/`arch-web` on affected
  workspaces; `typecheck-web`/`test-web` via turbo's filter.
- Full run when a rule config (`protected-paths.txt`, kind `config`), `go.mod`, `go.sum`, `package.json`,
  `pnpm-lock.yaml` or `tool-versions.env` changed, when `BASE_REF` cannot be resolved (fail closed to *full*,
  not to *nothing*), and always in `check-all`/CI.
- Fixture per scoped sensor proving that a violation in a changed file is still caught.

### Phase 3 — Go harness binary

Per [ADR 0012](../../adr/0012-parsing-sensors-in-go.md): `guard-bash` (shell AST via `mvdan.cc/sh`), then
`check-links` (goldmark, GitHub slugs, linear time), then `integrity-check` (diff parser). One PR each; same
rule ids and output lines; the old script and its bats file are deleted in the same PR.

### Phase 4 — agent loop latency

- Hook JSON parsed without `node` (phase 3 removes ~20 ms per Bash call).
- fast-check meets NFR-H2 for every file type; the budget check from phase 0 reports any file type that
  does not.
- Remove the deprecated `speckit-taskstoissues` skill (one fewer description in every session's context).
- Keep root guides at their budget (AGENTS.md ≤ 100 lines); area rules stay in nested AGENTS.md, which load
  only when an agent works there.

## Open assumptions

- A1: macOS first-exec assessment is the cause of slow bats (copy 350 ms vs link 30 ms, same script, same
  volume). Not yet confirmed which daemon; the Developer Tools exemption is untested.
- A2: Phase 2 soundness relies on rules being per-import-edge. A rule over the whole graph (cycles, dead code:
  knip, deadcode) stays full-repo and stays in `check-all`.
- A3: Go test cache is safe for `check` because tests read no state outside the module; integration tests
  (`-tags=integration`) keep `-count=1`.
