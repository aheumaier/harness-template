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
| PreToolUse hooks | 37 ms (protect-harness) + 32 ms (guard-bash) | per Bash call; ~20 ms of each is `node`. Measured without `HARNESS_EDIT`, which short-circuits protect-harness to 7 ms |
| fast-check (PostToolUse) | 70 ms `.sh`, 310 ms `.md`, 340 ms `.go` | `.ts`: eslint + whole-package `tsc --noEmit` |
| session context | ~9 KB (≈2.3k tokens) root guides + 13 skill descriptions | nested AGENTS.md load on demand |

**Root cause of slow bats:** `sandbox()` in `scripts/test/helpers.bash` copies the scripts under test into a
temp repo. macOS assesses every newly written executable on its first exec: a copied `check-specs.sh` takes
340–370 ms, the same script linked takes 30 ms. Measured on a scratch copy of the repo: linking executables
takes the suite **from 60 s to 19 s**. A plain link is **unsafe**: `integrity-check.bats` "flags a change to
sensor code" appends to the sandboxed `merge-base.sh` and wrote through to the real file. Linux CI is not
affected.

## Scaling to 2–4 MLOC

Bash cost is a constant (~16 ms per sensor) or scales with the size of the diff (`check-specs`, `check-commits`,
`integrity-check`, `migrations-append-only`). Two exceptions: `check-links` forks per anchor link (≈4 ms each;
tens of thousands of links → minutes), and the bats suite. What scales with the repo are the **full-repo
runs**:

| Sensor | Today | At scale | Fix |
|--------|-------|----------|-----|
| `test-go` | `go test -race -count=1 -short ./...` | `-count=1` disables Go's test cache: every package reruns every time | phase 1 |
| `lint-web` | `eslint .`, no cache | relints every file; `--cache` is unsafe with the type-aware rules in `eslint.config.mjs` | phase 2 |
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
| NFR-H1 | Each PreToolUse hook, per call | ≤ 50 ms | 30 ms (protect-harness), 21 ms (guard-bash), 24 ms (only-make-check), timing included (2026-10-28) | hook wrapper timing | warn; alert |
| NFR-H2 | fast-check (PostToolUse), per edited file | ≤ 1 s | 0.02–0.5 s (`.md` 0.08 s, was 0.37); `.tsx` in apps/web 1.05 s, over budget (type-aware eslint 0.97 s alone; 2026-10-28) | fast-check timing | warn; alert |
| NFR-H3 | `make check`, change without shell or harness files | ≤ 10 s | 2.9 s (2026-10-28, warm; Go test cache, gen-check stamp) | `run.sh` + make wall time | warn; alert |
| NFR-H4 | `make check`, any change | ≤ 60 s | 36 s (2026-10-28, shell change, `bats`-bound) | same | warn; alert |
| NFR-H5 | one sensor in `make check` | ≤ 5 s, or its row in `budgets.txt` | `bats` 27.9 s (2026-10-28), `go-test` 0.37 s, `analyzer-tests` 0.37 s (Go test cache, 2026-10-28; uncached in `full`); rest ≤ 5 s | `run.sh` | warn; alert |
| NFR-H6 | `make check` rerun, nothing changed | ≤ 3 s | 2.9 s (2026-10-28; was 3.7 s) | two runs back to back | warn; alert |
| NFR-H7 | `make check-all` in CI | ≤ 10 min | not measured | job duration | alert |
| NFR-R1 | Deterministic: same tree, same verdict and same findings | 0 differences | 0 (`make check` ×2, local, 2026-10-28) | nightly double run | **fail** (nightly) + alert |
| NFR-R2 | Fail closed: a sensor that cannot run (missing tool, base ref, parse error) exits non-zero | 0 silent passes | partly (merge-base, guard-bash) | fixtures with a broken environment | **fail** |
| NFR-R3 | Detection: every rule with a fixture rejects it ([ADR 0009](../../adr/0009-harness-selftest.md)) | 100 % | 100 % | `make harness-selftest` | **fail** |
| NFR-R4 | Actionable: every sensor failure through `run.sh` names its rule id and remediation; every hook block names its reason | 100 % | 100 % | `run.bats`, selftest, hook bats tests | **fail** |
| NFR-R5 | Portable: same verdict on macOS and Linux | 0 differences | Linux only in CI | selftest on a macOS runner (weekly) | **fail** (weekly) + alert |

Latency NFRs warn and alert but never fail a build: wall time depends on the machine, and a sensor that fails
under load would itself break NFR-R1. Reliability NFRs fail, because a wrong verdict is the one thing a sensor
must not produce.

## Phases

| # | Phase | Exit criterion | State |
|---|-------|----------------|-------|
| 0 | Measure and alert: timings, budgets, NFR issue | every run of `run.sh` and the hooks records its time; a violation warns locally and reaches the `harness-nfr` issue | proposed |
| 1 | Do less: skip and cache (no new code) | NFR-H3, NFR-H6 met; `check-all` still runs everything | proposed |
| 2 | Scope `make check` to the affected set | NFR-H3 holds on a 10× larger fixture repo; CI unchanged | proposed |
| 3 | Go harness binary for parsing sensors ([ADR 0012](../../adr/0012-parsing-sensors-in-go.md)) | ported sensors pass their fixtures; bats tests replaced by Go tests; no hook starts `node` to parse its input | proposed |
| 4 | Agent loop latency | NFR-H2 met for every file type | proposed |

## Progress

One slice per commit, in this order (the [loop prompt](0002-loop-prompt.md) works through it). `[ ]` open,
`[x]` done (`sha — before → after`), `[!]` blocked (reason).

- [x] P1 — 5a52a22 — `run.sh` overhead 16 → 27 ms per sensor; over-budget sensors now reported (`bats`
  52.8 s, `go-test` 6.1 s, `analyzer-tests` 6.0 s under `-j4`)
- [x] P2 — 2d8c53f — hooks 36/34/30 ms → 43/43/42 ms (timed now, budget 50 ms); CI annotations, summary, `harness-timings-check-all` artifact
- [x] P3 — f52c9be — NFR-R1 not measured → 0 differences (`make harness-determinism`, 112 s local); nightly
  `harness-nfr.yml`, rule `harness-deterministic`, fixture `harness-nondeterministic`
- [x] P4 — d1d7443 — `make check` without shell or harness changes 67 → 4 s warm (`bats` 64 s skipped);
  `HARNESS_SCOPE` (`changed` in `check`, `full` in `check-all`/CI), `scripts/harness/unchanged.sh`, rule `harness-scope`
- [x] P5 — 715693e — `make test-sh` 65.8 → 27.9 s warm (126 tests); executables hardlinked, `own` before
  a write, teardown checksum of the real `scripts/` and `.claude/hooks/`
- [x] P6 — c34a098 — `make check` rerun 3.7 → 2.9 s; `go-test` 2.6 → 0.37 s, `analyzer-tests` 1.2 → 0.37 s,
  `gen-check` 0.67 s → skipped (stamp), `depcruise` 0.43 → 0.45 s (cache gains nothing at this size)
- [x] P7 — e6e282c — fast-check `apps/web/src/App.tsx` 1.5 → 1.05 s, `packages/domain/src/index.ts` 0.59 → 0.50 s;
  eslint on the file only, tsc stays in `make check`
- [x] P8 — 3b35ea1 — PreToolUse hooks protect-harness 50 → 30 ms (Edit), 45 → 25 ms (Bash), guard-bash 45 → 21 ms,
  only-make-check 43 → 24 ms, fast-check stdin 40 → 22 ms; `.bin/harness` parses hook input and guard-bash commands
- [x] P9 — a52802e — `make docs-links` 0.37 → 0.06 s (sensor 0.40 → 0.02 s), fast-check on a Markdown file 374 → 80 ms;
  `.bin/harness check-links` (goldmark, GitHub slugs)
- [ ] P10 (phase 3) port `integrity-check`; workflow builds the binary from the base branch
- [ ] P11 (phase 2) `affected.sh`; scoped Go and web targets; full run on config changes

### Phase 0 — measure and alert

1. `run.sh` already wraps every sensor: it appends one line per run to `.cache/harness/timings.jsonl`
   (gitignored): sensor, milliseconds, exit code, scope (`HARNESS_SCOPE` or `fast-check`), git sha, OS. The
   hooks do the same through a shared helper. Milliseconds: `EPOCHREALTIME` when set (bash ≥ 5), else
   `perl -MTime::HiRes` (macOS `/bin/bash` 3.2 has no `EPOCHREALTIME`, BSD `date` has no `%N`). A failure to
   time or append is ignored.
2. `scripts/harness/budgets.txt`: `<sensor-or-hook> <budget-ms> <nfr-id>`, default 5000 ms for sensors.
3. Over budget, `run.sh` prints one line to stderr and exits with the sensor's own status:
   `harness: sensor 'lint-web' took 7.2 s (budget 5 s, NFR-H5)`. In CI it also emits a `::warning`
   annotation and a row in the job summary, and uploads `timings.jsonl` as an artifact.
4. A nightly job (next to `drift.yml`) collects the artifacts of the last 7 days of CI runs, computes p95 per
   sensor and hook, runs `make check` twice on `main` and diffs the verdicts and findings (NFR-R1). It opens
   or comments on one issue labelled `harness-nfr` with the violations, same as the `drift` issue. No
   violation, no comment. A determinism difference also fails the workflow.
5. Rules in `docs/harness/rules.md`, each with its remediation:
   - `harness-deterministic` (NFR-R1), with a selftest fixture: a sensor whose verdict alternates through a
     counter file in its sandbox, so the double run must report a difference. No randomness.
   - `harness-latency-budget` (NFR-H*), marked "no fixture" like the formatter rules: it only warns, and the
     selftest asserts a non-zero exit (ADR 0009). `run.bats` covers it: a sensor that sleeps past a tiny
     budget prints the warning and keeps its own exit status.

### Phase 1 — do less

1. **One switch for full runs.** `HARNESS_SCOPE` is `changed` by default and `full` in `check-all` and CI.
   `check-all` depends on `check`, so it sets the variable target-specifically and exported (`check-all:
   export HARNESS_SCOPE := full`): make passes it on to its prerequisites, and `export` puts it in the recipe
   shells where `run.sh` reads it. Without `export`, `check-all` would silently run in `changed` scope. CI also
   exports it. A bats or selftest case asserts that `make check-all` runs the shell sensors on a change
   without shell files. Every skip and cache below applies
   only in `changed` scope.
2. **Shell sensors only when their inputs changed.** `lint-sh`, `fmt-check-sh` and `test-sh` judge shell files
   and the harness itself; if neither changed, their verdict cannot change. (Not because agents cannot edit
   harness files: unprotected scripts exist, and the hook only stops Claude Code.) In `changed` scope they
   run when the change set (`git diff --name-only` against the merge base, plus staged, unstaged and untracked
   files) contains `*.sh`, `*.bash`, `*.bats`, any path in `protected-paths.txt` (this covers `rules.md`,
   `.shellcheckrc`, `.editorconfig`, `.squawk.toml`, CODEOWNERS, `scripts/**`, `.claude/**`), or
   `scripts/test/**`. If `merge-base.sh` fails, they run (fail closed to *full*, not to *nothing*). Saves
   ~40 s per `make check`.
3. **Bats sandbox without the first-exec penalty**, for harness PRs: `sandbox()` hardlinks executables
   (`ln "$src" "$dst" 2>/dev/null || cp "$src" "$dst"`; a hardlink is a regular file to git, so commits in
   `sandbox_base` see no type change) and copies data. Every helper that writes a sandboxed file first breaks
   the link with `rm -f "$f" && cp "$src" "$f"` (portable; GNU `cp --remove-destination` is not on macOS).
   `setup_file` records a checksum of the real `scripts/` and `.claude/hooks/` and `teardown_file` asserts
   it is unchanged, so a write-through fails the suite even when the developer has uncommitted harness edits
   (a `git diff --quiet` check would fail on those). Alternative with no code change (unverified): add the
   terminal app under macOS *Privacy & Security → Developer Tools*, which exempts its child processes from
   the assessment.
4. **Use the caches tools already have:** in `changed` scope, drop `-count=1` from `test-go`/`test-analyzers`
   (keep `-short`; `full` scope and integration tests keep `-count=1`); `depcruise --cache
   node_modules/.cache/depcruise`. No `eslint --cache`: `eslint.config.mjs` uses type-aware rules, whose
   results depend on files the cache does not track (phase 2 scopes eslint instead).
5. **`gen-check` only when nothing it covers changed:** the stamp hashes the inputs (`api/openapi.yaml`,
   `apps/api/db/queries/**`, `apps/api/db/migrations/**`, generator configs, `tool-versions.env`,
   `pnpm-lock.yaml`, `packages/api-client/package.json`) **and** the generated outputs. It skips only when
   both hashes match the last green run, so a committed hand edit of generated code still fails.
6. **fast-check stops typechecking the whole package per edit:** eslint on the file only; `tsc` stays in
   `make check` (`--incremental` with a tsbuildinfo under `node_modules/.cache`).

### Phase 2 — affected set

- `scripts/harness/affected.sh` prints the change set once (merge base with `BASE_REF` + working tree +
  untracked), the Go packages that changed, their reverse dependencies, and the JS workspaces
  (`turbo --filter=...[BASE_REF]`). Reverse deps: invert `go list -test -f '{{.ImportPath}} {{join .Deps " "}}'
  ./...` (`go list -deps` gives forward deps only).
- Scoped targets: `test-go` and `lint-go` on affected packages; `lint-web`/`arch-web` on affected
  workspaces; `typecheck-web`/`test-web` via turbo's filter. `arch-go` is scoped only if go-arch-lint can check
  a subset of packages (A4); otherwise it stays full.
- Full run when `HARNESS_SCOPE=full`, when a rule config (`protected-paths.txt`, kind `config`), `go.mod`,
  `go.sum`, `package.json`, `pnpm-lock.yaml` or `tool-versions.env` changed, and when `BASE_REF` cannot be
  resolved.
- Fixture per scoped sensor proving that a violation in a changed file is still caught.

### Phase 3 — Go harness binary

Per [ADR 0012](../../adr/0012-parsing-sensors-in-go.md): `guard-bash` and every hook's JSON input, then
`check-links`, then `integrity-check`. One PR each; same rule ids and output lines.

- `apps/api/tools/harness/*` goes into `protected-paths.txt` (`config`) and CODEOWNERS in P8.
- `.claude/hooks/*.sh` stay as thin wrappers that `exec .bin/harness …`; missing binary → exit 2 with
  "run make setup", except `make setup` and `make .bin/harness` pass guard-bash so the repo cannot lock itself.
  Bats tests cover the wrappers; the parsing tests move to Go.
- Ported sensors (`check-links`, `integrity-check`): old script and bats file deleted in the same PR, every
  bats case kept as a Go table test.
- `integrity-check` runs from `pull_request_target` (ADR 0010): the workflow builds `.bin/harness` from the
  base-branch checkout only.

### Phase 4 — agent loop latency

- Hook JSON parsed without `node` (phase 3 removes ~20 ms from each of the four hooks).
- fast-check meets NFR-H2 for every file type; the budget check from phase 0 reports any file type that
  does not.
- Keep root guides at their budget (AGENTS.md ≤ 100 lines); area rules stay in nested AGENTS.md, which load
  only when an agent works there. The deprecated `speckit-taskstoissues` skill is vendored and comes back on
  every spec-kit upgrade; it stays.

## Open assumptions

- A1: macOS first-exec assessment is the cause of slow bats (copy 350 ms vs link 30 ms, same script, same
  volume). Not yet confirmed which daemon; the Developer Tools exemption is untested. Hardlinks need the bats
  temp dir on the same volume as the repo; the `cp` fallback keeps the suite correct, only slower.
- A2: Phase 2 soundness relies on rules being per-import-edge. A rule over the whole graph (cycles, dead code:
  knip, deadcode) stays full-repo and stays in `check-all`.
- A3: Go test cache is safe for `check` because tests read no state outside the module; integration tests
  (`-tags=integration`) keep `-count=1`.
- A4: go-arch-lint may not support checking a subset of packages; verify before scoping `arch-go`.

## Log

- 2026-10-27: plan, ADR 0012 and ADR 0013 drafted; code-reviewer found 15 issues (warn-only rule vs
  selftest, fail-open hook binary, protection gap, full-run switch, trigger list, bats sandbox portability,
  timing on bash 3.2, gen-check stamp, reverse deps, eslint cache, base-branch build). All folded in above.
  Hook baseline re-measured without `HARNESS_EDIT` (37 ms, not 7 ms).
- 2026-10-27: P1 (5a52a22). `make check` 55 s wall. Under `-j4` `go-test` and `analyzer-tests` exceed 5 s
  (2.9 s alone), so P6 matters beyond `bats`. `check-all` is red only on `harness-config-not-loosened` for the
  slice's own files (the human gate); its other targets and `test-integration` are green. One selftest run
  stalled 915 s on `db-edited-migration` (1 s when rerun alone): not reproducible, noted for NFR-R1.
- 2026-10-27: P2 (2d8c53f). Timing moved to `scripts/harness/timing.sh`, sourced by `run.sh` and all four
  hooks; it costs ~8 ms per hook call on macOS bash 3.2 (two perl clock reads), 0 on bash ≥ 5. Hook numbers
  are A/B medians of 11 calls via a bench script whose own perl read adds ~4 ms to both sides; another
  session's Go tests had the load average at 20–40 during this iteration, so absolute numbers vary. Headroom
  under NFR-H1 is now 7 ms until P8 removes `node` (~20 ms). Bats tests set `HARNESS_BUDGETS=/dev/null`.
  `check-all`: integrity gate as in P1; other targets green.
- 2026-10-28: P3 (f52c9be). `make harness-determinism` (two `make check` runs, 112 s; budget row 120 s) found
  one real difference on its first run: `run.bats` asserted timing scope `changed` and failed under
  `HARNESS_SCOPE=full`, which the nightly sets; `helpers.bash` now unsets it. Findings are compared as
  sorted lines with durations, timing rows and temp paths masked, since `-j4` reorders output. p95 report
  reads untrusted PR-run artifacts, so only plain sensor ids are kept. Selftest 30 fixtures, 29 s (load
  ~3). The workflow is untested on GitHub until merged (scheduled workflows run from `main`); actionlint green.
- 2026-10-28: P4 (d1d7443). Baseline `make check BASE_REF=HEAD` 131 s cold, 67 s warm, with `bats` 64 s of it;
  after, on a tree without shell or harness changes, 4.3 s and 3.8 s, and the three shell targets print
  `skipped`. A change that touches a shell or harness file still pays the full ~55–65 s (NFR-H4), and so does
  this branch against `main`, since every slice of it is a harness change. Triggers are `*.sh`, `*.bash`,
  `*.bats`, `scripts/**` and every protected path; bats inputs outside those (node, `package.json`) are left to
  `check-all`. The new bats test leaked `BASE_REF=HEAD` from the calling `make` and now pins `BASE_REF=main`.
  Selftest 30/30, 22 s. `check-all`: integrity gate as in P1; other targets and `test-integration` green.
- 2026-10-28: P5 (715693e). `HARNESS_SCOPE=full make test-sh` 63.0/65.8 s before, 28.1/27.9 s after (load ~4);
  `make check` with a shell change 31/36 s, still `bats`-bound. Three hardlinks from the P0 measurement were
  still in this session's scratchpad (link count 4 on `merge-base.sh`, `check-specs.sh`,
  `protected-paths.txt`) and were removed; nothing had written through them. The guard was proven before the
  fix: with plain links, `integrity-check.bats` failed in `teardown_file` naming `merge-base.sh`, and the file
  was restored from git. `integrity-check.bats` `add()` now calls `own` on existing files. A1 holds: links
  remove the first-exec cost. The link test skips when the temp dir is on another volume (copy fallback).
  Selftest 30/30. Integrity green (only `scripts/test/`, not config).
- 2026-10-28: P6 (c34a098). In scope `changed` (`make check`; unset counts as changed) Go tests run without `-count=1`,
  `arch-web` uses `--cache node_modules/.cache/depcruise`, and `gen-check` skips while a hash of its inputs
  (sources, generator configs, tool versions, lockfile, Makefile, the script) and of the generated files
  matches the stamp of its last green run (`gen-check.sh --unchanged`, outside `run.sh` so the target prints
  the skip). Any other scope runs uncached. The depcruise cache caught a new untracked and an edited tracked
  violation when warm, but saves nothing at this size; kept for the 2–4 MLOC case. `harness-determinism` now
  exports scope `full` (as nightly): in `changed` scope run 2 would skip what run 1 stamped. Mutation check:
  dropping the scope test, an input, the outputs from the hash, or stamping before the compare each fails
  `gen-check.bats`. Determinism 0 differences (75 s, full). Selftest 30/30. Integrity names only `Makefile`
  and `gen-check.sh`; the other check-all targets and `test-integration` green.
- 2026-10-28: P7 (e6e282c). fast-check on a web file ran eslint (0.93 s in apps/web) and whole-package
  `tsc --noEmit` (0.52 s): 1.5 s per edit. Now eslint on the file only: 1.05 s, still over NFR-H2 (1 s),
  because type-aware eslint (`strictTypeChecked`) builds the package's type program per call; the rule keeps
  warning. Not fixed here: `eslint --cache` stays banned (type-aware results), and a persistent eslint or tsc
  server is out of scope for phase 1. No `tsc --incremental` in `make check`: Turborepo already caches
  `typecheck` (tsc 0.06 s warm) and a tsbuildinfo would also apply in scope `full`. New sandboxed tests in
  `fast-check.bats` put fake eslint and tsc on PATH; the "file only" test fails against the old hook.
  Selftest 30/30. Integrity names only `.claude/hooks/fast-check.sh`; rest of check-all and integration green.
- 2026-10-28: P8 (3b35ea1). Every hook started `node` to read its JSON input (~20 ms of each call); guard-bash
  tokenized commands with regexes. `.bin/harness` (module `apps/api/tools/harness`, built by `make check`,
  `make setup` and `make test-sh`) now does `hook-input` for all four hooks and guard-bash on a bash AST
  (`mvdan.cc/sh/v3`). Deviations from ADR 0012: the wrappers call the binary without `exec`, so the bash
  timing EXIT trap still records the hook; and there is no zsh fallback: a command that does not parse as
  bash is blocked (fail closed; the message asks for simpler commands). Not looser than the old tokenizer:
  a static string is still read as commands after `;&|`, a newline, a backquote or `$(`; tighter: `bash -c`,
  `sh -c`, `zsh -c` and `eval` strings are parsed recursively (depth 4), an rm -r on a dynamic path blocks.
  Without the binary every hook blocks except `make setup` / `make .bin/harness`
  (`scripts/harness/hook-bootstrap.sh`, 4 tests in `hook-bootstrap.bats`). The 63 guard-bash bats cases
  moved 1:1 into Go table tests (with a count check) plus new cases; `guard-bash.bats` keeps 3 wrapper tests.
  The module is protected and code-owned; its tests run as `make test-harness` (sensor `harness-tests`).
  Gap: `vuln-go` and `lint-go` cover only `apps/api`, not the nested `tools/` modules (TD-3). Selftest
  30/30. Integrity names only this slice's files; rest of check-all and integration green.
- 2026-10-28: P9 (a52802e). `check-links.sh` forked awk/tr/sed/grep per anchor link and approximated GitHub
  slugs in ASCII: 0.4 s for 77 links, linear in links times processes. `.bin/harness check-links` parses each
  Markdown file once (goldmark, GFM) and caches each target's slugs: 0.02 s. Same rule ids, lines and exit
  codes; stricter where the regexes guessed (github-slugger rules with Unicode letters and `-1`/`-2` for
  duplicates, rendered heading text, setext headings, `~~~` and indented code, images, reference links); the
  repo's docs pass unchanged. 3 bats cases plus both fixtures and 9 new cases as Go table tests; 5 mutants of
  the checker all killed. Script and bats file deleted. fast-check reports both docs rules and gained a bats
  test for the Markdown route (sandbox excludes `rules.md`, whose links leave the sandbox). Selftest 30/30
  (both docs fixtures rejected). Integrity names only this slice's files; rest of check-all and integration
  green.
