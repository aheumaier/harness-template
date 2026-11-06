# 0005 — Rewrite the harness scripts and their tests in Go

Status: active, rulings taken 2026-11-02 (all option a, see [Rulings](#rulings)) · Owner: @aheumaier · Started:
2026-11-02

## Goal

Move the harness's bash scripts under `scripts/` and `.claude/hooks/` into subcommands of the Go binary
`.bin/harness`, and move their 46 bats test files to `go test`. Keep a small bash bootstrap only where Go cannot
run yet or must not fail open. At the end bats, its pin and `make test-sh` are gone, and the harness has one
language for logic and one test runner. The Makefile stays the single entry point
([ADR 0001](../../adr/0001-tool-pinning-and-single-entry-point.md)).

This file is the plan only. Every slice below changes protected paths (`scripts/harness/*`, `scripts/test/*`,
`apps/api/tools/harness/*`, `Makefile`, `docs/harness/*`, `.github/**`), so each slice is a group of harness PRs
that a human approves, **one script per PR** as [ADR 0012](../../adr/0012-parsing-sensors-in-go.md) requires. The
decision is [ADR 0045](../../adr/0045-harness-scripts-and-tests-in-go.md) (accepted); it amends ADR 0012 (the
clauses it replaces are listed there), ADR 0024 (lint scope) and ADR 0031 (orphan check).

The plan was written and reviewed in the product `prostrength_new` (its exec plan 0005 and ADR 0054, 2026-11-02),
where the user ruled on it. Under ruling R1 the port runs here, so this copy governs from now on; the product's
copy records how each slice arrives there through `make harness-sync-check`.

## Why

The user decided on 2026-11-02 that bats is the wrong tool and that the scripts move to Go, even if that wins
nothing in run time. The evidence, re-measured here at `750c094` on 2026-11-02 on Apple Silicon (load average
8–11 from other sessions) unless a line says otherwise:

- **Size.** `scripts/` holds 40 `.sh` files plus 2 git hooks, 4,860 lines (plus 13 for the hooks);
  `.claude/hooks/` 5 hooks, 319 lines. Largest: `adoption-probe.sh` 441, `agent-gc.sh` 383, `mutation.sh` 316,
  `install-tools.sh` 254, `selftest.sh` 251, `coverage.sh` 188, `pr-review.sh` 172. Not counted, and not in
  scope: the fixture overlays `scripts/harness/fixtures/*/apply.sh`, their shell payloads, the `apply.sh.tmpl`
  files of the adoption probe, and the vendored `.specify/scripts/`.
- **Tests.** 46 bats files, 409 tests, 5,958 lines (6,069 with `helpers.bash`). Run one after another
  (`make test-sh`, inside `make check` on any `scripts/*` or other harness change) they take 192 s. The
  cost is process starts in every test (`git init`, git, make, `node -e` per hook call), not a few heavy tests.
  The pilot's two files take 2.3 s (`check-commits.bats`) and 6.6 s (`protect-harness.bats`), median of 3. The
  suite grew from 120 tests on 2026-10-28.
- **Budgets broken.** In the local `.cache/harness/timings.jsonl` of one worktree of this repo (11 `make check`
  runs on 2026-11-02, 9 in scope `changed` and 2 `full`): `bats` median 173 s, slowest 183 s, every run over
  the 5 s default ([NFR-H5](../../harness/nfr.md); `budgets.txt` has no `bats` row); `harness-tests` median
  12.7 s; `shellcheck` median 7.2 s. NFR-H4 (`make check` ≤ 60 s on any change) is broken by bats alone (TD-7).
- **The Go tool already exists.** `apps/api/tools/harness`: one package, 17 subcommands, 4,516 lines plus 4,075
  test lines, 72 tests, 10.4 s uncached (user 4.4 s, sys 6.2 s). No Go test calls `t.Parallel()`. Four test
  files use env or chdir: `layout_test.go`, `integritycheck_test.go`, `guardbash_test.go`, `specrefs_test.go`.
  Its dispatcher is already `run(args, stdin, stdout, stderr) int` (`main.go:67`), so a test can call any
  subcommand without a process. The module is not linted by golangci
  ([ADR 0024](../../adr/0024-size-and-complexity-limits-are-maintainability-sensors.md) scopes `lint-go` to
  `./...` in `apps/api`, which stops at the nested `go.mod`); `fast-check.sh` runs only `go vet` on it.
- **Bash parses where it should not.** ADR 0012 listed the reasons; the scripts that still parse (`agent-gc.sh`,
  `rule-fires.sh`, `sync-check.sh`, `check-specs.sh`, `affected.sh`, `unchanged.sh`) do it with awk, sed and jq.
- **CI.** `.github/workflows/ci.yml` has one `check-all` job, no shards. NFR-H7 (`check-all` ≤ 10 min) is not
  measured.

## Phases

| # | Phase | Exit criterion | State |
|---|-------|----------------|-------|
| 0 | Write this plan and ADR 0045 | both reviewed here, findings fixed or rejected below | this PR |
| 1 | The user rules on R1–R4 | each has a dated ruling in this file; ADR 0045 accepted; the status lines of ADRs 0012, 0024 and 0031 say "amended by ADR 0045" | done (2026-11-02, ruled on the product's copy, recorded here) |
| 2 | Slice 1, pilot: `check-commits` and `protect-harness` (2 PRs) | the [pilot acceptance criteria](#pilot-acceptance-criteria) hold, measured and written into the [Log](#log) | done (2026-11-03, 2026-11-05) |
| 3 | Slice 2, base: lint, layout, helpers, scope fixes | the [slice 2 list](#port-order) is done, `make check` green | open |
| 4 | Slices 3–10: the rest of `scripts/` in the [port order](#port-order) | each slice's bats files deleted, its Go tests green, its docs updated | open |
| 5 | Slice 11, exit: bats removed | the [exit criteria](#exit-criteria) hold | open |

## Decisions taken

| Date | Decision | Recorded in |
|------|----------|-------------|
| 2026-11-02 | bats is the wrong choice; the harness tests move to Go and `go test`, even if that wins nothing in run time (the user) | ADR 0045 |
| 2026-11-02 | `scripts/` is rewritten in Go; this replaces ADR 0012's "orchestration stays in bash" and the clauses ADR 0045 lists (the user) | ADR 0045 |
| 2026-11-02 | No GNU `parallel` pin and no `bats --jobs`: both are thrown away once bats goes (agent advice; the user did not object) | this file |
| 2026-11-02 | R1–R4 all option (a): template first, everything but the bootstrap, testscript plus table tests, one `.bin/harness` (the user) | [Rulings](#rulings), ADR 0045 |
| 2026-11-02 | The `protect-harness` port keeps every block of the bash heuristic and adds some: globs ignore case, symlinks and `..` are followed, an unresolvable path or an unreadable list blocks, a glob without `*` covers the paths under it, `$(…)` and backquotes are split, `-Ei`/`--in-place` and `cp … 2>/dev/null` are caught. ADR 0045 says "case for case"; whether to reword that sentence is the user's call in the pilot 2 PR (agent) | this file, pilot 2 PR |
| 2026-11-02 | A hook guard's cases are table tests that call `run()` (since slice 2 `runHook`, which dispatches in a test world) with a JSON payload, not testscript files: a payload per case is clearer as a Go value than as a `.txtar` file, and the wrapper is still run as a shell would (`TestProtectHarnessHook`). testscript stays for subcommands driven by arguments and files, as `check-commits` (agent) | this file |
| 2026-11-05 | A: ADR 0045's "keeps the heuristic's behaviour case for case" becomes "keeps every block of the bash version and may add more"; its `:40` stays (the user) | ADR 0045 |
| 2026-11-05 | B: the merged pilot branches `refactor/harness-check-commits-go` and `refactor/harness-protect-go` are deleted on the remote (the user) | this file |
| 2026-11-05 | C: the iOS follow-ups ride in slice 2 as a `docs(ios)` commit: an `xcodebuild` row in `budgets.txt`, TD-151 and TD-155 closed, NFR-R5 dated (the user) | this file, slice 2a PR |
| 2026-11-05 | D: slice 2 is three PRs, each opened after the one before merges: 2a registry, world, shared files and parallel tests; 2b `lint-harness`; 2c TD-129, `layout-check` and three rules (the user) | this file |
| 2026-11-06 | `lint-harness` runs `paralleltest` next to `tparallel` (only `paralleltest` catches a test with no subtests and no `t.Parallel()`), and its process ban is on a use of `exec.Command`, `exec.CommandContext` or the type `exec.Cmd`, by package, not on the import (`world_test.go` needs `exec.ErrNotFound`) (agent) | this file, `docs/harness/rules.md` |

## Rulings

The four questions the user ruled on 2026-11-02, each with the option taken. The options not taken are one line
each in ADR 0045's Alternatives; the product's copy has them in full.

- **R1, where the work happens: (a) in the template first; products take each slice through
  `make harness-sync-check`** ([ADR 0026](../../adr/0026-template-sync-is-a-drift-report.md)). The scripts are
  template files: a rewrite only in a product turns every later upstream change to a script into a hand port
  from bash into Go. The adoption probe ([ADR 0038](../../adr/0038-readme-brownfield-adoption-is-probed.md))
  runs only here (`HARNESS_UPSTREAM` is empty in `scripts/tool-versions.env`), so [trap 3](#three-traps) is proven
  here.
  Products port their own scripts after the template's slice (for example a product's own Sonar or Swift
  wrappers, and the shared scripts it changed locally).
- **R2, scope: (a) everything except a bash bootstrap.** What stays bash, and why:

  | File | Lines | Why it stays bash |
  |------|-------|-------------------|
  | `scripts/install-tools.sh` | 254 | installs the pinned tools before anything else can run |
  | `scripts/setup-worktree.sh` | 153 | `make setup` on a fresh clone, before `.bin/harness` exists |
  | `scripts/harness/layout.sh` | 136 | the Makefile reads `layout.sh --print` at parse time, and CI reads it before `setup-go` (`harness-integrity.yml:48`) |
  | `.claude/hooks/*.sh` (5) | 319 | ADR 0012: a missing binary exits 127, which Claude Code treats as non-blocking, so the wrapper turns it into exit 2 |
  | `scripts/harness/hook-bootstrap.sh` | 14 (34 after pilot 2) | the wrappers' "run make setup" message and the `make setup` / `make .bin/harness` escape |
  | `scripts/harness/timing.sh` | 82 | `harness_time_exit` times each hook through an EXIT trap ([ADR 0013](../../adr/0013-harness-nfr-budgets.md)) |
  | `scripts/git-hooks/*` (2) | 13 | git runs them by name; they call the binary |

  About 970 lines stayed at `750c094`; after the port each hook wrapper is a thin caller of `.bin/harness` like
  `guard-bash.sh` (24 lines). Both `protect-harness.sh` (140 lines: its Bash heuristic) and `fast-check.sh`
  (113 lines: its file-type routing table, which hard-codes layout paths that belong in `layout.env`) move their
  logic into subcommands. `layout.sh` keeps its double in Go (`layout.go`, shared vectors in
  `testdata/stack-vectors.txt`). The fixture overlays and payloads stay bash: they are test data in the format of
  [ADR 0009](../../adr/0009-harness-selftest.md), and the payloads exist to trip shellcheck and shfmt.
- **R3, test framework: (a) `github.com/rogpeppe/go-internal/testscript` for command-level tests, plain Go table
  tests for logic.** A `.txtar` file holds the steps and the fixture files of one case, which reads close to
  today's bats files. `go-internal v1.15.0` is already in the module's `go.sum` (`go.sum:11-12`), so this adds a
  direct `require`, not a new module (`go mod tidy` also lists `golang.org/x/sys` and `golang.org/x/tools`,
  which only the test binary links, as indirect requires; pilot 1); ADR 0045 records it, as
  [ADR 0022](../../adr/0022-approved-api-responses.md) did for its harness dependency. `testscript.Main` does
  *not* run subcommands in-process: it copies the test binary into a temp `PATH` directory under each command
  name (`cloneFile`, a new inode on macOS) and each call is a child process, which pays the macOS first-run check
  (trap 2) per command per run. The in-process route is `Params.Cmds`: a `harness` command that calls
  `commands.dispatch` with a world built from the script's environment (`scriptWorld`, since slice 2). Its built-in `exec` cannot be replaced through `Params.Cmds`, so a script that uses
  `exec` runs a child process that no linter sees.
- **R4, binary layout: (a) all subcommands in the existing `.bin/harness`:** one build, one module, one Make
  target that every check already depends on (`$(HARNESS)`), and `make test-harness` already runs its tests.

## Target shape

- **One binary, one module, one package.** Each ported script becomes a subcommand of `.bin/harness` in
  `apps/api/tools/harness` (R4), in its own file of the existing `package main`. Slice 2 replaces the `switch` in
  `run` with a registry that each subcommand's file fills from `init()` (name, usage line, function), so `run`
  stays a thin dispatcher and a product-only subcommand is one more file, not an edit to `main.go` that every
  template sync touches. `main.go` holds four lists today (the package doc `:5-28`, the `usage` const `:46`,
  `rootSensors` `:50-60`, the `switch` `:72-201`). Afterwards the usage text, `harness help`, `rootSensors` and the
  dispatch come from the registry, and the package doc keeps no list. A duplicate name panics at start-up, so a
  product's variant of a shared subcommand takes its own name and its own make target.
  ADR 0045 records why this is registration rather than explicit wiring.
- **A stable surface for product files.** A product-only subcommand compiles against the package's unexported
  helpers. Slice 2 names the surface such a file may use (the command type, the value carrying root,
  environment and working directory, finding output, the layout reader, the sandbox helper) in the package doc;
  a change to it is a breaking template change and says so in its PR, so `sync-check` readers know to look.
  Sub-packages are not used: each importing file would carry the module path, which differs between the
  template and a product (`go.mod:1`), and `sync-check` would report every such file as changed.
- **The output contract does not change** (ADR 0012): same rule ids, same `file:line: message [rule-id]` lines,
  same exit codes, same flags where a caller passes them.
- **A stale binary blocks with a rebuild message, never a lockout.** A worktree that pulls slice 1 without
  running make still has a binary built before it, which answers `protect-harness` (unknown to it) with the
  usage text and exit 2 (`main.go:200-201`). So the escape cannot depend on the new binary's exit code: a ported
  wrapper reads its input once (`input=$(cat)`), tests it against `hook-bootstrap.sh`'s anchored
  `make setup|make .bin/harness` match *before* it calls the binary, and then passes `$input` to the one binary
  call. Both commands write no protected path, so letting them through matches today's behaviour. From slice 1
  on, every usage error (an unknown subcommand, a wrong argument count, an unknown flag) gets its own exit code,
  64; `run.sh` passes any non-zero code through, so sensors still fail. Wrappers and git hooks treat 64, 126 and
  127, and an exit 2 with a line starting with `usage: harness`, as a stale binary: they block and say "rebuild" (the
  `commit-msg` hook: "run make setup"). `harness_missing` reads the hook input itself (`hook-bootstrap.sh:24`),
  so it gets the input from the wrapper's variable, not from stdin a second time.
- **Subcommands take their world as arguments.** A subcommand reads its root, environment and working directory
  from a value passed in by `run`, never from `os.Getenv`, `os.LookupEnv`, `os.Environ`, `os.Getwd` or `os.Chdir`, and never
  calls `os.Exit`. At `750c094` non-test code had 5 such calls in 3 files: `main.go:173` and `:177` (`HARNESS_ROOT`,
  `BASE_REF`, `HARNESS_CHANGE_APPROVED`), `guardbash.go:79` (`os.Getwd`) and `layout.go:144`
  (`HARNESS_STACK`, in the layout parser every sensor calls, which must keep behaving like `layout.sh`,
  [ADR 0033](../../adr/0033-stack-profiles.md)).
- **Go never runs a ported script's bash.** Non-test Go runs no script under `scripts/` except the R2 bootstrap.
  A callee is ported in the same slice or before, or replaced by existing Go code. The calls between scripts
  today (comments left out), and how the port order handles each:
  - `check-commits.sh:41` runs `merge-base.sh`. The pilot reuses `mergeBase` from `integritycheck.go:153`, and
    slice 4's `merge-base` subcommand wraps the same function, so the "kept in step by hand" of ADR 0012 ends.
  - `fast-check.sh` runs `run.sh` (`:19`) and the slice 6 sensors `migrations-append-only.sh`, `squawk.sh`,
    `swiftlint.sh` and `swift-format.sh`, and sources `file-sensors.sh`. Its slice 5 subcommand only prints the
    routing decision; the wrapper, which stays bash (R2), runs `$RUN` on what it prints.
  - `pre-commit.sh` runs `run.sh` (`:30`): it ports in slice 9 with `run.sh`. It also sources `file-sensors.sh`
    (`:27`) for `harness_file_kind` (`:51`) and runs `trivy.sh secret --staged` (`:65`). The slice 6 PR that
    ports those two swaps both calls for their subcommands (`harness file-kind <path>` gives
    `harness_file_kind` a CLI form) and measures NFR-H8.
  - `pr-review.sh` (`:55`), `scheduled-review.sh` (`:49`) and `agent-gc.sh` (`:76`) source `ci-agent.sh`: all
    port in slice 8 with `ci-agent`. `ci-agent.sh` is deleted in the last slice 8 PR; until then a change to
    one copy is made in the other.
- **Callers change in the same PR**, with one exception: a workflow that runs the PR's own copy against the base
  checkout (`pr-review.yml`, whose header says "Change the script's interface and this file in separate PRs",
  `pr-review.yml:25`). There it is two PRs: the first adds the subcommand, keeps the script as a shim that calls
  it, and adds `setup-go` and the `.bin/harness` build from `base.sha` to the workflow's gate step (today the
  job builds nothing, so a shim without a binary would turn every later review red), with `cache: false` as in
  `harness-integrity.yml:52`, because the job later holds a secret; the second switches the
  call and deletes the script. Make targets that call a subcommand gain `$(HARNESS)` as a prerequisite (for the
  pilot: `commits`, `Makefile:508`). Workflows that run one gain `setup-go` and build `.bin/harness` from the
  trusted checkout, before any step that holds a secret.
- **A thin bash bootstrap** (R2). It is linted by shellcheck and shfmt as now and tested by Go tests that
  execute it.
- **Tests.** Logic: Go table tests next to the code. Commands: testscript files under
  `apps/api/tools/harness/testdata/script/` (R3), with `harness` registered through `Params.Cmds`. One shared
  sandbox helper, an extension of the existing `helpers_test.go`, builds sandboxes: a temp git repo, the repo
  files a case needs, executables hardlinked, `own` to break a hardlink before a test writes to it, a
  before/after checksum of every linked source in `TestMain` so a test that writes through a hardlink fails the
  run (as `helpers.bash` does today), and the environment hygiene of `helpers.bash`
  (`HARNESS_BUDGETS=/dev/null`, no `HARNESS_SCOPE`, no `HARNESS_STACK`) passed as values, not set in the process.
  Ported tests read paths from `layout.env` and never hard-code the template layout (TD-99, TD-133). A test reads
  only files inside the module or matched by the trigger pathspec (Scope and budgets), because only those make
  `test-harness` re-run. One test reads a file outside it today: `qualityrows_test.go:207` (`docs/QUALITY.md`);
  slice 2 moves that input into `testdata/` or keeps the path in the trigger. (`:197` reads
  `scripts/templates/new-domain/quality-row.md.tmpl`, which `'scripts/*'` already matches.)
- **Linted like product code.** Slice 2 adds a `lint-harness` target that runs golangci in
  `apps/api/tools/harness` with its own protected config, and brings the module under the size and complexity
  rules of ADR 0024 with starting numbers measured when it lands (a deliberate second set that only goes down
  until it meets the product thresholds). The plan's `tparallel` and `forbidigo` rules live in that config; on
  `apps/api/.golangci.yml` they would never see this module. ADR 0024 exempts `_test.go` from the per-function
  limits, and this config does the same; a product whose own ADR puts tests under the limits will see this file
  as "changed here too" at every sync (see Risks).

## Inventory

Lines, bats tests and slice per script (at `750c094`). Kind: **o** orchestrates tools, **p** parses text, **m**
both (carried over from the product's read-only inventory; spot-checked, not audited). The bats column covers all
46 files and sums to 409.

| Script | Lines | bats file (tests) | Kind | Slice |
|--------|-------|-------------------|------|-------|
| `harness/check-commits.sh`, `git-hooks/commit-msg` | 49, 7 | check-commits (10) | p | 1 pilot |
| `.claude/hooks/protect-harness.sh` | 140 | protect-harness (10) | m | 1 pilot (wrapper stays) |
| `harness/coverage.sh` | 188 | coverage (7) | o | 3 |
| `harness/unchanged.sh`, `harness/affected.sh`, `harness/merge-base.sh`, `harness/gen-check.sh` | 30, 91, 27, 59 | unchanged (8), affected (11), merge-base (4), gen-check (8) | p | 4 |
| `.claude/hooks/guard-bash.sh`, `only-make-check.sh`, `only-test-files.sh`, `fast-check.sh`, `harness/hook-bootstrap.sh` | 24, 18, 24, 113, 14 | guard-bash (3), only-make-check (3), only-test-files (8), fast-check (11), hook-bootstrap (5) | m | 5 (wrappers stay; `fast-check` logic moves; tests move) |
| `harness/check-specs.sh`, `harness/contract-check.sh`, `harness/upstream-drift.sh`, `harness/migrations-append-only.sh`, `harness/squawk.sh`, `harness/swiftlint.sh`, `harness/swift-format.sh` | 81, 53, 165, 58, 16, 18, 15 | check-specs (15), contract-check (9), upstream-drift (11), migrations-append-only (13), squawk (2), swiftlint (4), swift-format (3) | m | 6 |
| `harness/trivy.sh`, `harness/file-sensors.sh`, `github-branch-protection.sh` | 145, 57, 50 | trivy (16), file-sensors (2), github-branch-protection (4), codeowners (3) | m | 6 |
| `harness/sync-check.sh` | 146 | sync-check (17) | p | 6 (TD-129 already fixed in slice 2) |
| `harness/mutation.sh`, `harness/nfr-report.sh`, `harness/rule-fires.sh`, `harness/ci-timings.sh` | 316, 48, 161, 66 | mutation (10), nfr-report (5), rule-fires (13), ci-timings (7) | m | 7 |
| `obs-query.sh`, `harness/obs-e2e.sh`, `new-domain.sh`, `harness/scaffold-tests.sh`, `gen-schema-doc.sh` | 141, 137, 126, 67, 39 | obs-query (16), obs-e2e (6), new-domain (3), scaffold-tests (5) | o | 7 |
| `harness/pr-review.sh`, `harness/ci-agent.sh`, `harness/agent-gc.sh`, `harness/scheduled-review.sh` | 172, 163, 383, 161 | pr-review (20), agent-gc (28), scheduled-review (13) | m | 8 (security review, two PRs; `scheduled-review` sources `ci-agent`) |
| `harness/run.sh`, `harness/determinism.sh`, `harness/selftest.sh`, `harness/pre-commit.sh` | 151, 55, 251, 95 | run (14), timing (6), determinism (5), selftest (12), make-stack (11), clones-web (2), pre-commit (10) | o | 9 (`pre-commit` runs `run.sh`) |
| `harness/adoption-probe.sh` | 441 | adoption-probe (9) | o | 10 |
| bootstrap (R2 table) | ~970 | install-tools (9), layout (14), helpers (4) | o | 11: tests move, scripts stay |

## Port order

Pilot first. After that, within two constraints, the slowest test files first: a script's callees port in the
same slice or earlier (Go never runs a ported script's bash), and the agent confinement controls do not come
first. A script many others call (`run.sh`, `timing.sh`, `merge-base.sh`, `unchanged.sh`, `layout.sh`) goes late
or stays, because deleting it touches every caller.

1. **Pilot: `check-commits` and `protect-harness`, one PR each.** Small, well tested (10 bats tests each), and
   between them they cover the hard cases: a sensor that CI runs from the base branch against the PR head
   (`HARNESS_ROOT`, `harness-integrity.yml:73-77`) and a git hook (`commit-msg`), and a PreToolUse hook with a
   50 ms budget (NFR-H1) whose wrapper stays bash. Also in this slice:
   - gate `test-harness` on harness changes (trap 1), which fixes a gap that exists today;
   - the stale-binary exit code and wrapper routing (Target shape);
   - correct TD-7's cause (it blames Trivy and golangci-lint; the product measured those files at about 22 s of
     its bats time) and the "Today" cells of NFR-H4 and NFR-H5 in `nfr.md`, with per-file bats timings measured
     here.
2. **Base.** No script is ported; this slice makes the rest safe.
   - Done in 2b: `lint-harness` with its own protected config (`apps/api/tools/harness/.golangci.yml`): the size
     and complexity rules with starting numbers (funlen 77, gocognit 41, 6 parameters, 556 lines), `tparallel`,
     `paralleltest` and a `forbidigo` rule that bans `exec.Command`, `exec.CommandContext` and `exec.Cmd` in
     `_test.go` files other than `helpers_test.go` (the use, by its package, not the `os/exec` import:
     `world_test.go` imports it for `exec.ErrNotFound`); a generated-code header hides no file, except from
     revive, which golangci cannot change (declared in rules.md). Rules
     `harness-tests-parallel` and `harness-tests-exec-in-helpers`; one fixture per measure
     (`harness-function-too-long`, `harness-function-too-complex`, `harness-too-many-params`,
     `harness-file-too-long`) and per test rule (`harness-test-not-parallel`, `harness-test-exec-outside-helpers`,
     `harness-test-exec-disguised`, which pins the evasion review's three closed bypasses). Every subtest now
     calls `t.Parallel()`. `TestRunPhysicalCwd` needs no suppression: `paralleltest` skips a test that calls
     `t.Chdir` or `t.Setenv`. The file-length start is 556, not the 548 measured at `1a1f4b7`: the subtests'
     `t.Parallel()` lines grew `layout_test.go`. `make check` now checks the module's format (TD-171); `make fmt`
     already formatted it, since `golangci-lint fmt ./...` walks into nested modules. The parameter limit stays
     at 6 until the stable surface changes: `printFinding` (`command.go`, on the list in `main.go`'s package
     comment) takes 6, and grouping them into the `finding` struct breaks every product file that calls it.
   - Done in 2a: the subcommand registry (`command.go`) and the stable surface for product files (Target shape,
     listed in `main.go`'s package comment), folding in all four lists of `main.go`; the "world as arguments"
     change (`world.go`) for the 5 calls including `layout.go`'s signature (the shared vectors keep it in step
     with `layout.sh`); `t.Parallel()` in every top-level test but `TestRunPhysicalCwd`. Pilot 2's sixth call,
     `protectharness.go`'s `worktrees`, reads the world's environment, and its test no longer uses `t.Setenv`.
   - Done in 2a: the surface shared by subcommands moved into files of its own, named in the package doc: the
     hook helpers (`blockf`, `set`, `physicalDir`, `segments`) into `hook.go`, the path helpers
     (`protectedPaths`, `pathspecMatch*`, `matchesAny*`, `resolveExisting`, `isRegular`) into `paths.go`, the
     Markdown readers into `markdown.go`, and the test helpers two tests share into `helpers_test.go`.
   - Done in 2a: `TestProtectHarness`'s allowed rows run on a fixed list in `testdata/`, so a product that
     protects more paths keeps them green; its blocked rows still run on the repo's list, so dropping an entry
     they name fails the test.
   - Done in 2a: the sandbox helper from the pilot, generalised (`place`, `gitRepo`, `writeLines`).
   - Fix TD-129 (`sync-check` literal `apps/api` paths) and extend `harness-layout-valid` (`layout-check`) to
     fail when no protected glob matches `<API_DIR>/tools/harness/*` (fixture
     `harness-layout-module-unprotected`): after the port, all harness logic lives under `API_DIR`, so an adopter
     with another `API_DIR` who skipped README's "fix this one first" would have none of it protected.
   - Propose two rules for `rules.md`, each with a fixture: `repo-no-orphan-subcommands` (every key of the
     subcommand registry has an executable caller under ADR 0031, because Go code is never a referencer and
     `deadcode` sees a dispatched subcommand as alive; it reads the keys from the registry in-process) and
     `harness-go-runs-no-script` (non-test harness Go names no `scripts/**/*.sh` outside the bootstrap).
   - Propose `harness-no-new-shell` (ADR 0045's "new harness logic is Go"): no `.sh` under `scripts/` or
     `.claude/hooks/` outside a protected allowlist that holds the bootstrap and the scripts not yet ported and
     only shrinks; fixture data under `scripts/harness/fixtures/` and `scripts/templates/` is exempt. Fixture
     `harness-new-shell-script`.
3. **Coverage:** `coverage.sh`. Products that extend it port their part after this slice.
4. **Scope machinery:** `unchanged`, `affected`, `merge-base`, `gen-check`. These decide what `make check` skips
   ([ADR 0014](../../adr/0014-make-check-scoped-to-the-change.md)), so they keep every fail-closed case of their
   bats files, and the scoped fixtures (`go-tests-dependent-package`, `web-lint-dependent-workspace`,
   `SCOPE=changed`) must still be rejected.
5. **Hooks.** The wrappers stay (R2); `fast-check`'s routing moves into a subcommand that reads `layout.go` and
   prints which sensors to run, and the wrapper runs them through `$RUN`; the 31 bats tests become Go tests that
   execute the wrappers. `node -e` per hook call goes away with them. The stale-binary branch, written out in
   `protect-harness.sh` and `scripts/git-hooks/commit-msg` since slice 1, becomes one `harness_stale` function in
   `hook-bootstrap.sh` that every wrapper calls.
6. **Sensors in `make check` and `check-all`:** specs, contract, upstream drift, migrations, squawk, SwiftLint,
   swift-format, trivy, file sensors, branch protection, sync check. SwiftLint and swift-format start only when
   [plan 0004](0004-ios-stack-group.md)'s phase 2 is done and ADRs 0042–0044 are accepted, so the port does not
   race the iOS work that defines them.
7. **Reports and tools:** mutation, NFR and rule-fires reports, CI timings, the observability scripts,
   `new-domain`, `scaffold-tests`, `gen-schema-doc`.
8. **Agent jobs:** `pr-review` with `ci-agent`, `scheduled-review` (it sources `ci-agent`), and `agent-gc`.
   `pr-review.sh settings` and `screen`, and `agent-gc.sh confined`, `lock` and `patch`, are confinement and
   leak controls: their port needs a human security review. `pr-review` takes the two-PR route (Target shape), and its gate checks that the base has the
   subcommand. `drift.yml` calls `.bin/harness agent-gc` directly after the agent step, never through a make
   target that rebuilds the binary.
9. **The choke point:** `run.sh`, `determinism.sh`, `selftest.sh`, and `pre-commit.sh`, which runs `run.sh`.
   `RUN` in the Makefile becomes
   `.bin/harness run`. Its timing rows must stay byte-compatible with what `timing.sh` writes for the hooks: both
   writers test against one shared vector file, like `layout.sh` and `layout.go` today.
10. **`adoption-probe`.** Today a product skips the probe with two keys: `HARNESS_UPSTREAM` is set, or the script
    is absent and `.harness-version` declines it (`ci.yml:138-147`), so removing it is a config change that needs
    the label. Once the script is gone everywhere, one `# declined` line would skip a required check. This slice
    replaces the file-existence key with one that `harness-integrity` reads, and updates `ci.yml`, its comment
    and README step 6 in the same PR. The key amends ADR 0038, so the slice starts with its own ADR (proposed:
    "the adoption probe is switched off by <key>, not by removing its script").
11. **Exit:** the remaining bootstrap tests move to Go (`install-tools`, `layout`, and new ones for
    `setup-worktree.sh` and both git hooks); bats goes (see [Exit criteria](#exit-criteria)).

An alternative order puts the most-changed files first, so later upstream changes land in Go rather than bash.
In this repo's history (220 commits) the most-changed are `install-tools.sh` (14 commits, bootstrap, stays),
`run.sh` and `selftest.sh` (11 each), `fast-check.sh` (8) and `contract-check.sh` (7). The plan keeps
slowest-first because `run.sh` and `selftest.sh` have the most callers; the user may move slice 9 forward.

## Bats and Go side by side

- Until slice 11, `make test-sh` runs bats over whatever bats files are left, and `make test-harness` runs the Go
  tests. A PR deletes a bats file together with the Go tests that replace it, never later. No test is run by
  both, and no case is dropped: the PR lists each bats `@test` name and the Go test or script case that replaces
  it.
- A ported script is deleted in its PR, with its callers moved (except the `pr-review` shim, deleted in its
  second PR). `make orphans` ([ADR 0031](../../adr/0031-no-orphan-scripts-or-make-targets.md)) must stay green:
  deleting a script that was the only caller of another orphans the second one. For subcommands the check only
  bites once `repo-no-orphan-subcommands` exists (slice 2).
- `sh_sensor` (`Makefile:459`) keeps gating `test-sh`, `lint-sh` and `fmt-check-sh`. Every harness change, the
  Go ports included, triggers `test-sh`, so NFR-H4 stays broken until slice 11. Each slice's Log entry gives the
  remaining bats time, so the overrun is expected, not a surprise.

## Scope and budgets

- **`make check` scoping (ADR 0014).** Today `test-harness` is not gated: in scope `changed` it runs `go test`
  with its cache (`GO_TEST_COUNT` is empty when `CACHED`, `Makefile:58-59`), in scope `full` with `-count=1`.
  Go's test cache only rechecks files inside the module root (`apps/api/tools/harness`), and hashes their size
  and modification time (`cmd/go/internal/test/test.go`, go 1.27.3). A file at the repo root that a test reads
  or runs (`.claude/hooks/*`, `scripts/harness/*`, `protected-paths.txt`, `layout.env`) is not part of the key,
  so `make check` can replay an old pass after it changes. This already applies to existing tests that read repo
  files (`scenarios_test.go:225`, `integritycheck_test.go:386`, `qualityrows_test.go:197`,
  `layoutliterals_test.go`). The fix is the same trigger rule as `sh_sensor`: `test-harness` runs with
  `-count=1` when `unchanged.sh` reports a change to `sh_sensor`'s pathspec (`'*.sh' '*.bash' '*.bats'
  'scripts/*'`, `Makefile:459`) or to another repo file a test reads (today `docs/QUALITY.md`); `unchanged.sh`
  already counts every protected path as a change. It keeps the cache otherwise. The trigger list, not the Go
  cache, decides what re-runs after a harness change. The cost: any protected-path edit, `AGENTS.md` or
  `docs/harness/*` included, runs the whole Go suite uncached, as it runs all of bats today.
- **NFR-H5 (one sensor ≤ 5 s or a `budgets.txt` row).** `harness-tests` already breaks it uncached (10.4 s
  here; median 12.7 s in the 11 `make check` runs of Why). The merged suite will be larger. Each slice records
  `go test` time cached and uncached. Slice 11 proposes a measured `harness-tests` row in `budgets.txt`
  (protected; a human decides). "NFR-H5 met" means met through that row.
- **NFR-H4 (`make check` ≤ 60 s on any change).** Met when a harness change no longer pays 192 s of bats.
  Each slice re-measures `make check` after a one-line change to a ported script.
- **NFR-H3 (`make check` ≤ 10 s without shell or harness changes) and NFR-H6 (rerun with nothing changed
  ≤ 3 s).** Each slice checks that a non-harness edit leaves `test-harness` cached.
- **NFR-H1 (hooks ≤ 50 ms).** The pilot measures `protect-harness` before and after (29 ms Bash, 34 ms Edit in
  `nfr.md` on 2026-10-28); the wrapper keeps no `node` call and gains no new process. On macOS the first hook
  call after each rebuild pays the ~0.35 s executable check once; the median hides it, and the Log says so.
- **NFR-H2 (`fast-check` ≤ 1 s) and NFR-H8 (the pre-commit hook ≤ 2 s, `budgets.txt`).** Slice 5 measures
  `fast-check` before and after, slices 6 and 9 the pre-commit path (Target shape).

## Three traps

Each is an acceptance criterion of the pilot and a rule for every later slice.

1. **Stale cached green.** `go test` replays a pass when nothing it tracked changed, and it tracks only files
   inside `apps/api/tools/harness` (see Scope and budgets). So:
   - `test-harness` runs uncached whenever a harness path changed (the trigger rule above). This is the guard.
   - Ported code is called in-process (`commands.dispatch` through `runTest`, `runHook` or `Params.Cmds`; only
     `TestMain` and `TestRunPhysicalCwd` call `run`), never through a `.bin/harness` built outside the test.
   - A wrapper test that needs `.bin/harness` hardlinks the test binary itself into the sandbox as
     `.bin/harness`, and `TestMain` dispatches on the program name (like `testscript.Main`, but a hardlink, see
     trap 2). The test binary is part of the cache key, so a source change re-runs the test.
   - Every Go test that executes a file goes through `helpers_test.go` (`gitT`, `execIn`, `execInput`), the only
     place a harness test calls `exec.Command` (`forbidigo`, slice 2b); testscript scripts use their own `exec`.
     The helpers do not make the file part of the cache key; the trigger rule does that job.
2. **macOS executable check.** macOS checks every newly written executable on its first run, about 0.35 s each
   (comment in `scripts/test/helpers.bash:31`). The sandbox helper hardlinks executables into the sandbox
   (`os.Link`), falls back to a copy only across volumes, and never writes a new executable per test.
   `testscript.Main`'s `cloneFile` copy is exactly what to avoid, which is one reason the plan uses
   `Params.Cmds` (R3). Because a hardlink writes through, the helper also guards the real files (`own`, and the
   checksum in `TestMain`).
3. **Adopters.** README section (b) step 3 tells adopters to copy `scripts/test/` (`README.md:211`);
   `scripts/templates/adoption-probe/drop.txt` lists 15 bats files a root-module adopter drops (11 of them
   because they hard-code the template layout, TD-99); `adoption-probe.sh` checks that step 3 and step 6 name
   exactly the paths it copies or drops (`step3_copy`, `step6_probe`); `rules.md` names 21 different bats files
   in its **Fixture:** lines; README's "Generic parts" list (`README.md:152-157`) names `check-commits.sh` and
   `protect-harness.sh`. Each PR updates its own references, and the adoption probe stays green (`make
   adoption-probe` runs here). A ported test reads `layout.env` (TD-99's fix), so its `drop.txt` line is deleted,
   not replaced: a new line would be relative to `API_DIR`, which is `.` in the probe repo.

### Pilot acceptance criteria

- **A1 (trap 1).** After an edit to the check-commits Go source, `.claude/hooks/protect-harness.sh`,
  `scripts/harness/hook-bootstrap.sh`, `scripts/harness/timing.sh` or `scripts/harness/protected-paths.txt`,
  `make check` runs the harness tests uncached (no `(cached)` in `go test -v`). After an edit outside the harness
  it prints `(cached)`. The PR shows the runs.
- **A2 (trap 2).** The sandbox helper's own test asserts that each sandboxed executable is the same file as the
  original (`os.SameFile`), and that a test writing through a hardlink without `own` fails the run. First-run
  cost measured on macOS: no test pays the 0.35 s more than once per build.
- **A3 (trap 3).** `rules.md`'s **Fixture:** lines for `git-conventional-commits` and `harness-files-protected`
  name the Go test files by path (`rules-fixture-claims` accepts only tracked paths and `fixtures/<dir>`); no
  `drop.txt`, README or `adoption-probe.sh` line names a deleted file; `make adoption-probe` green.
- **A4 (base branch).** `harness-integrity.yml`'s `pr-conventions` job runs `make commits` today without
  `setup-go`. After the port it sets up Go from the base checkout (`cache: false`, as the job above it) and
  builds `.bin/harness` there, never from `pr-head` ([ADR 0010](../../adr/0010-base-branch-gates.md), ADR 0012).
  `HARNESS_ROOT` keeps pointing at `pr-head` for the commits it reads. The Log records the job's time before and
  after against its 5-minute timeout.
- **A5 (contract).** Every bats case of the two files has a Go counterpart with the same input and the same
  expected output lines and exit code (10 + 10 cases); `fixtures/git-commit-not-conventional` is still rejected
  (`make harness-selftest`); the `commit-msg` hook rejects a bad message and fails closed with "run make setup"
  when the binary is missing.
- **A6 (budget).** `protect-harness` stays ≤ 50 ms per call (NFR-H1, median of 9, timing included).
- **A7 (stale binary).** With a `.bin/harness` built before the slice (no `protect-harness` subcommand), and
  again with one that rejects a new argument, the hook lets `make .bin/harness` and `make setup` through and
  blocks everything else with "rebuild"; the `commit-msg` hook says "run make setup", not the usage text.
- **A8 (numbers).** The Log records: bats time of the two files before, Go test time after (cached and
  uncached), lines removed and added, `make check` time after a one-line harness change. These numbers decide
  whether R3 holds before slice 2.

## Doc and rule updates per slice

Each PR updates every reference to the files it moves:

- `docs/harness/rules.md`: the **Sensor:** and **Fixture:** lines naming the script or its bats file (21 bats
  files are named; `sh-tests-pass` names the whole suite and changes only in slice 11).
- `docs/harness/nfr.md`: NFR-R4's sensor column names `run.bats` and the hook bats tests.
- `docs/harness/ci.md`: the jobs and scripts it names (`run.sh`, `nfr-report.sh`, `pre-commit.sh`,
  `ci-timings.sh`, `rule-fires.sh`, `pr-review.sh`, `ci-agent.sh`, `github-branch-protection.sh`, `agent-gc.sh`,
  `install-tools.sh`).
- `ARCHITECTURE.md`: the codemap rows for `tools/harness`, `scripts/harness` and `scripts/` ("bats tests in
  `scripts/test/`").
- `.github/workflows/*`: the steps that call the script, and the comments that name it
  (`harness-integrity.yml:20`).
- `README.md`: the "Generic parts" list (`:152-157`), section (b) step 3 (`scripts/test/`, `:211`), the table at
  `:340` (bats files and TD-99), step 6 (`scripts/test/adoption-probe.bats`, `:420`).
- `scripts/templates/adoption-probe/drop.txt` and `adoption-probe.sh`'s `step3_copy` and `step6_probe` lists.
- `.claude/agents/reuse-reviewer.md` and `.claude/agents/evasion-reviewer.md`, which name the shell scripts.
- `scripts/harness/protected-paths.txt` and `.github/CODEOWNERS` when a protected path disappears.
- `AGENTS.md` golden principle 5 (`:44`), which lists `scripts/harness/**` as protected but not the module the
  logic moves to (human-owned: proposed in the PR description); `docs/design-docs/core-beliefs.md:11`, whose
  consequence names `run.sh` (slice 9); the package comment of `apps/api/tools/harness/main.go:1-3`, "The shell
  scripts and make stay the entry points" (done in slice 2a).
- ADRs that name a script keep their text (an accepted ADR is history); ADR 0045 lists the ones it amends.
- `docs/exec-plans/tech-debt-tracker.md`: close or reword TD-7, TD-99, TD-129, TD-133, TD-143 (determinism
  latency, which also holds the shellcheck overrun),
  TD-145 and TD-147 when the slice that touches their script lands.

## Exit criteria

- No `.bats` file and no `scripts/test/helpers.bash` in the repo; `make test-sh` is gone.
- `BATS_VERSION` and `BATS_SHA256` are gone from `scripts/tool-versions.env`, and the bats install from
  `scripts/install-tools.sh`.
- `runnerDirs` in `apps/api/tools/harness/orphans.go` no longer lists `scripts/test` (ADR 0031).
- `sh-tests-pass` in `rules.md` is reworded to the Go tests (or retired into `go-tests-pass`), so no rule names a
  sensor that no longer runs ([ADR 0032](../../adr/0032-rules-that-never-fire-are-a-drift-report.md)).
- What stays bash is the bootstrap table of R2 plus the fixture data, and nothing else; each bootstrap file is
  covered by a Go test. `setup-worktree.sh`, `scripts/git-hooks/commit-msg` and `scripts/git-hooks/pre-commit`
  have no bats file today, so their Go tests are new, not ported.
- `lint-harness` runs in `make check` with the size and complexity rules, each with a rule id and a fixture, as
  are its `tparallel` and `os/exec` bans; `repo-no-orphan-subcommands`, `harness-go-runs-no-script` and
  `harness-no-new-shell` are rules with fixtures; `harness-layout-valid` fails when the module is not
  protected.
- NFR-H4 met: `make check` ≤ 60 s after a one-line change to any harness file. NFR-H5 met for `harness-tests`
  through a measured `budgets.txt` row that a human approved.
- The template and its products have the same Go subcommands for every shared script (R1), and a product's
  `make harness-sync-check` shows no file as "changed here too" except the ones it adapted on purpose.

## Risks

- **A port changes behaviour.** Bash quirks (word splitting, `set -e` gaps, `sed` differences) are part of
  today's behaviour, and some callers may rely on them. Mitigation: one-to-one case lists (A5), the selftest
  fixtures, and the nightly determinism run.
- **The trigger rule is the only guard against a replayed pass.** If `test-harness` loses its gate, or a harness
  file is missing from `protected-paths.txt`, `make check` can report an old green. Mitigation: A1, and the full
  uncached run in `check-all` and CI catches it before a merge.
- **A stale binary.** Covered by the escape check before the binary call, plus the 64/126/127 and usage-text
  check (Target shape, A7). A behaviour change behind an
  unchanged interface is not detected until the binary is rebuilt; `.bin/harness` rebuilds on every
  `make check` (ADR 0012).
- **Hand ports into products.** A product that changed a shared script locally gets "deleted upstream, changed
  here too" from `sync-check` when the template deletes it, and must decide before that slice whether to
  upstream its change or keep it as a separate subcommand file under its own name. In the second case it deletes
  the template's file and declines it in `.harness-version`, or keeps a caller for it, or
  `repo-no-orphan-subcommands` fails. Mitigation: freeze changes to a script while its slice is open.
- **Product files break on a template helper change.** A product-only subcommand compiles against unexported
  template helpers, and breaks only when the product syncs. Mitigation: the stable surface (Target shape), and a
  PR that changes it says so.
- **The harness lint config differs between template and product.** A product with its own rule for test
  complexity (for example tests under `gocognit`) adapts the shared config and sees it as "changed here too" at
  every sync. Expected; slice 2's PR says so.
- **Tests become harder to change.** `scripts/test/*` is protected without a class (CODEOWNERS review only,
  `protected-paths.txt:54`); `apps/api/tools/harness/*` is `config` (`:56`), so every harness test change will
  need the `harness-change-approved` label. ADR 0045 keeps it that way (the `test-writer` stays out, ADR 0039);
  a lighter class needs its own ADR.
- **Slices are many PRs.** About 45 PRs in 11 slices, each needing a human. The pilot's numbers (A8) are the
  point to stop if the design does not hold.

## Not in this plan

- `t.Parallel()` for the API's own tests (`apps/api`), and integration-test scoping for many domains.
- The fixture overlays and payloads, `.specify/scripts/`, and any change outside the harness.
- Product-only scripts: products port their own scripts after the template's slice.

## Review

**In the product, 2026-11-02.** `senior-architect-reviewer` and `architecture-reviewer` reviewed the product's
copy, each told to skip the other's lens, with a verification pass each. Their 24 findings (cache key outside the
module, stale-binary lockout, the `pr-review.yml` base checkout, the probe opt-out, layout-dependent paths, the
write-through guard, unmeasured NFRs, the unlinted module, the ADR 0012 clauses, the orphan check, sub-packages,
Go running unported bash, and others) were all fixed in that copy, which this file starts from. None was
rejected. The product's plan lists them one by one.

**Here, 2026-11-02.** The same two reviewers reviewed this copy and ADR 0045, each told to skip the other's
lens. 23 findings; 22 fixed here, one in part. No finding reversed a ruling.

- *Behaviour (`senior-architect-reviewer`, 8).* Fixed:
  - A binary built before a slice answers an unknown subcommand with exit 2, so the exit-64 routing alone did not
    stop the lockout A7 tests. Now the wrapper checks the `make setup` / `make .bin/harness` escape before it
    calls the binary, and an exit 2 with the usage text counts as stale (Target shape, ADR 0045).
  - The port order broke "callees first" three times (`fast-check`, `pre-commit`, `scheduled-review`). Now the
    full callee list is in Target shape, `pre-commit` moves to slice 9, `scheduled-review` to slice 8, and
    `fast-check`'s subcommand only routes.
  - The trigger rule now names its pathspec, and `qualityrows_test.go:197` is listed next to `:207`.
  - A duplicate subcommand name panics; a product's variant takes its own name.
  - NFR-H2 and NFR-H8 are measured in slices 5 and 9.
  - `pr-review`'s new `setup-go` uses `cache: false`.
  - Facts: the timing runs (9 `changed`, 2 `full`), shellcheck 7.2 s, 21 bats files in `rules.md`, 5,958 lines.
  - TD-7's correction gets per-file timings measured here.
  - The cost of the trigger (any protected-path edit runs the Go suite uncached) is now stated.
- *Structure (`architecture-reviewer`, 15 + 1 sensor candidate).* Fixed:
  - The probe's new opt-out key amends ADR 0038; slice 10 now starts with its own ADR, and ADR 0045 says so.
  - "New harness logic is Go" is a decision in ADR 0045; slice 2 proposes `harness-no-new-shell` for it.
  - The registry: all four lists of `main.go` are named, registration from `init()` is a decision in ADR 0045
    with its trade-off against core belief 1, and the orphan check reads the registry in-process.
  - ADR 0045 contradicted itself on test protection. Decided: the module's tests stay `config` (ADR 0039's
    `test-writer` boundary holds); a lighter class needs its own ADR. The Risks line no longer leaves it open.
  - Product files coupled to template helpers: a consequence in ADR 0045, a stable surface in Target shape, a
    Risk.
  - The harness lint config's treatment of tests, and the expected product diff, are stated.
  - The SwiftLint and swift-format ports wait for plan 0004's phase 2.
  - Rule ids and fixtures for every new lint rule; the new rules are `team-convention` under ADR 0015.
  - The R3 (b) and R4 (b) options are in ADR 0045's Alternatives.
  - Product-only facts, "iOS lane" and "trap 3" are reworded; the two remaining code doubles are named.
  - Three files added to the doc updates; TD-143's label corrected; the remaining bootstrap tests, including
    the `pre-commit` git hook, are named in slice 11 and the exit criteria.
  - The PR description lists where this copy differs from the text the user ruled on.
- *Not taken:* the suggestion to add a note to plan 0004 that new iOS harness logic is Go. ADR 0045's decision
  already binds it, and plan 0004 belongs to the iOS work; its next PR picks up the rule.

**Re-check, 2026-11-02.** Both reviewers re-read the fixed copy. All 23 findings read as resolved; 10 new small
ones, all fixed: `pre-commit.sh`'s slice 6 callees (`file-sensors.sh`, `trivy.sh`) and NFR-H8 in slice 6; when
`ci-agent.sh` goes; the package doc cannot be filled from a runtime registry; a product's variant leaves the
template's subcommand uncalled; the analyzers plugin already registers from `init()`; the stale-binary Risk line;
`:197` is inside the trigger; "iOS lane" in Scope; which `pre-commit` has no bats file; a link to trap 3.

## Log

- 2026-11-02: plan and ADR 0054 written, reviewed and ruled on (R1–R4 all (a)) in the product; the port moves
  here under R1.
- 2026-11-02: plan brought here as exec plan 0005 with ADR 0045, numbers re-measured at `750c094`; ADRs 0012,
  0024 and 0031 marked amended.
- 2026-11-02: reviewed here (23 findings, see Review); stale-binary escape, callee order, trigger pathspec, test
  protection and the registry settled in ADR 0045 before it merges.
- 2026-11-02: pilot 1 (`check-commits`), measured on an Apple Silicon Mac.
  - **Tests (A8).** The bats file took 2.66 s (median of 3). Now the testscript file (14 cases: the 10 bats
    cases, plus length boundaries and descriptions that show nothing) takes 0.83 s, and the hook tests 0.56 s.
    The harness module's whole suite takes 11.8–13.9 s uncached (10.2 s before) and 0.6 s cached.
  - **Lines (A8).** Removed 139. Added 317 for `check-commits` (Go 96, testscript 144, hook test 56, hook 21),
    and 287 one-time: sandbox helper 223, testscript runner 50, stub 4, dependencies 10.
  - **`make check` after a one-line harness change (A8): 173 s.** The bats suite is 169 s of it and is
    unchanged by the pilot.
  - **Sandbox (A2).** The first run after a rebuild takes 1.20 s, then 0.54 s.
  - **Cache trigger (A1),** with `BASE_REF=HEAD` and `make test-harness`.
    - A clean tree runs 13.9 s, then 0.6 s from the cache.
    - An edit to `scripts/harness/timing.sh` or to `checkcommits.go` runs uncached (11.8–12.1 s, each time).
    - So does an edit to `protect-harness.sh`, `hook-bootstrap.sh` or `protected-paths.txt`.
    - A `README.md` edit stays cached (0.6 s).
  - **Fixtures (A3, A5).** `make harness-selftest` still rejects `git-commit-not-conventional`, and
    `make adoption-probe` is green.
  - **Base branch (A4).** `pr-conventions` took 9–15 s before. Its time after the change shows only on the first
    PR after the merge, because `pull_request_target` runs main's workflow.
  - **Review.** An evasion review found subjects that show nothing (Unicode spaces, format characters, combining
    marks), a first line of only U+00A0, and a `%s` that joins the first paragraph. They are fixed, and the
    testscript file pins them.
  - **For slice 2.** `getenv` does not reach a child process's environment: in-process `git` reads the test
    process's own. The "world" value that replaces `getenv` should carry the child-process environment as part of
    the stable surface.
- 2026-11-02: pilot 2 (`protect-harness`), measured on an Apple Silicon Mac (load avg 7–10).
  - **Tests (A8).** The bats file took 6.6 s (median of 3). The Go tests take about 1.4 s uncached (0.01 s +
    0.42–0.46 s + 0.89–0.93 s for the three test functions, 3 runs) and nothing when cached.
  - **Lines (A8).** Removed 262: bats 129, the hook's heuristic 127, `hook-bootstrap.sh` 6. Added 641: Go 264,
    Go tests 273, shared test helpers 33, hook 32, `hook-bootstrap.sh` 26 (the duplicate-key check), bats 13.
  - **`make check` after a one-line edit to `protectharness.go` (A8): 174 s.** The bats suite is 172 s of it.
  - **Cache trigger (A1),** with `BASE_REF=HEAD` and `make test-harness`. A clean tree runs 13.8 s, then 0.7 s.
    An edit to `protectharness.go` runs uncached (13.4–13.6 s, each time); a `README.md` edit stays cached
    (0.7 s). The first run after a `.go` file is restored is uncached once, since tests read the source files.
  - **Contract (A5).** Each of the 10 bats cases is a row or subtest of the same name in
    `protectharness_test.go`, plus rows for case folding, symlinks, NotebookEdit, quoting and redirects, an
    unresolvable path, an empty list, and the wrapper's stale-binary and duplicate-key cases. The old and new
    hook, run on 51 shell commands and a duplicate-key payload, differ only where the new one blocks more.
  - **Budget (A6),** median of 9, before → after: Bash 40.1 → 29.8 ms, Edit 36.1 → 29.9 ms, Edit in another
    worktree 47.8 → 36.0 ms, a blocked Edit 39.3 → 30.2 ms.
  - **Stale binary (A7).** With the binaries of `b526181` (exit 2 and usage text) and `cc2c9d5` (exit 64) the
    hook lets `make setup` and `make .bin/harness` through and blocks everything else with "rebuild".
  - **Fixtures (A3).** `rules.md` names `protectharness_test.go`; no `drop.txt`, README or `adoption-probe.sh`
    line named the bats file. `make harness-selftest` (88 passed) and `make adoption-probe` are green.
  - **Review.** Go, shell, code, architecture and reuse reviewers, then one re-run. Fixed: `cp`'s destination
    before a redirect, `rm AGENTS\.md`, a duplicate, differently cased or `\u`-escaped `command` key passing the
    `make setup` escape, git's error text in the block. Still open in TD-73: wrapper words, interpreters, a word
    glued to `>`, an input redirect after the destination, `cp -t`.
- 2026-11-05: the user ruled on slice 2 (A–D in [Decisions taken](#decisions-taken)).
- 2026-11-06: slice 2a (registry, world, shared files, parallel tests), measured on an Apple Silicon Mac (load
  avg 5–8).
  - **Go tests.** The module's suite takes 7.2–7.5 s uncached (3 runs; 12.3 s before) and 0.4–0.5 s cached.
    With `-race` it takes 7.1 s (19.0 s before); `-race -count=3 -shuffle=on` is green.
  - **`make check` after a one-line harness change: 155 s** (at `ee4eaa1`). The bats suite is 151 s of it.
  - **Lines.** Go code +1012 −677, Go tests +639 −292, `testdata/protected-paths.txt` +30, `Makefile` +5 −2.
    `main.go` lost its four lists (276 lines changed); the registry (`command.go`), the world (`world.go`) and
    the shared files (`hook.go`, `paths.go`, `markdown.go`) hold what each subcommand used to repeat.
  - **Behaviour.** Same rule ids and output. A relative or missing working directory in a hook payload now
    blocks in both guards, and `run` resolves a symlinked working directory, so `..` names the physical parent.
  - **Tests.** Every test runs in a world built from the process's environment minus a caller's `HARNESS_*`,
    `BASE_REF` and `PR_TITLE`, so a CI or shell variable no longer changes a result. Subtests are not parallel
    yet (2b).
  - **Review.** Go, code, architecture, reuse, evasion, shell and senior architect reviewers, one re-run each.
    The evasion review found that moving every `TestProtectHarness` row onto the fixed list let 12 entries of the
    repo's list be dropped with the test green; the blocked rows stay on the repo's list. Deferred items are
    TD-165 to TD-171 in the tech-debt tracker.
- 2026-11-06: slice 2b (`lint-harness`), measured on an Apple Silicon Mac (load avg 10–19).
  - **Go tests.** With every subtest parallel, the module's suite takes 6.8–7.0 s uncached and 0.46 s cached;
    `-race -count=3` takes 19.4–21.0 s and is green, with no flaky test.
  - **`lint-harness`:** 1.6 s warm, under the 5 s default budget, so `budgets.txt` has no row for it.
  - **`make check` after a harness change: 206 s** (at `d3d6fd6`). The bats suite is 199 s of it, up from 151 s
    in 2a with no bats file changed; the load average was about twice 2a's.
  - **Lines.** Go tests +53 −2; the config, `Makefile`, `run.sh` and `run.bats` +88 −2; seven fixtures +862, 661
    of them the `harness-file-too-long` file.
  - **Selftest:** 95 passed, 0 failed, 2 skipped (they need Xcode); the seven new fixtures each fail as expected.
  - **Review.** Go, code, architecture, reuse, evasion and shell reviewers, one re-run each. The evasion review
    found nine ways past the new rules. Four are fixed in the config and pinned by fixtures (a generated-code
    header, `zz_helpers_test.go`, `&exec.Cmd{}`, `zz_test.go_fixture.go`). The rest are declared under "Not
    caught" in `rules.md`, as is revive's generated-code skip, which golangci cannot turn off. TD-171 is paid,
    and TD-172 is new.
- 2026-11-06: main went red after slice 2b merged (`74fb02a`): CI's check-all, adoption-probe and generator-probe
  failed with "parallel golangci-lint is running". Under `make -j`, `lint-go` and `lint-harness` start golangci-lint
  together; it allows one run per user's `$TMPDIR` and by default gives up after 5 s. The local gates missed it
  because warm caches finish `lint-go` inside those 5 s; with a cold Go build cache it reproduces locally. Fix:
  `--allow-serial-runners` on both targets and on fast-check's golangci-lint call, and a 3-minute `run.timeout`
  in the harness config. `lint-go`'s measured time can now include the wait (NFR-H3, NFR-H5).
