# ADR 0045: Harness scripts and their tests move to Go (amends 0012, 0024, 0031)

Status: accepted · 2026-11-02 · amends [ADR 0012](0012-parsing-sensors-in-go.md),
[ADR 0024](0024-size-and-complexity-limits-are-maintainability-sensors.md),
[ADR 0031](0031-no-orphan-scripts-or-make-targets.md)

The work is planned in [exec plan 0005](../exec-plans/active/0005-harness-scripts-in-go.md). The plan and this
decision were written in the product `prostrength_new` (its ADR 0054), where the user ruled option (a) on the
plan's four open questions on 2026-11-02: the port happens in `harness-template` first and reaches products
through template sync (R1), everything moves except a bash bootstrap (R2), tests use `testscript` plus table tests
(R3), and all subcommands live in `.bin/harness` (R4).

## Context
ADR 0012 moved the sensors that parse into one Go binary, `.bin/harness`, and kept orchestration in bash on the
grounds that bash is not the cost and Go would only add a build step. Measured here on 2026-11-02 at `750c094`
(plan 0005 has the details):

- `scripts/` holds 40 `.sh` files plus 2 git hooks (4,860 lines), `.claude/hooks/` 5 hooks (319 lines). The Go
  tool has 17 subcommands, 4,516 lines and 72 tests.
- The bash scripts are tested by 46 bats files with 409 tests. Run one after another, as `make test-sh` does
  inside `make check` on any harness change, they take 192 s. Every test starts processes (`git init`, git,
  make, `node -e` per hook call); that, not a few heavy tests, is the cost. bats was over the 5 s default of
  NFR-H5 in every recorded run (median 173 s in 11 `make check` runs in one worktree of this repo on
  2026-11-02), and it alone breaks NFR-H4 (`make check` ≤ 60 s on any change).
- Several "orchestration" scripts parse after all (`agent-gc.sh`, `rule-fires.sh`, `sync-check.sh`,
  `affected.sh`, `unchanged.sh`, `check-specs.sh`), with awk, sed and jq.
- Every harness script has two languages to read: bash for the logic, bats for its tests, plus Go for the parts
  ADR 0012 moved.

On 2026-11-02 the user decided that bats is the wrong choice and that the harness tests move to `go test`, even
if that wins nothing in run time, and that `scripts/` is rewritten in Go.

## Decision
- **Harness scripts become Go code in the harness module `apps/api/tools/harness`**, one script per PR. Their
  callers (Makefile recipes, workflows, hooks, other scripts) change in the same PR, except where a workflow runs
  the PR's own copy against the base checkout (`pr-review.yml`): there a shim keeps the old interface for one PR.
  They are subcommands of the one binary `.bin/harness` (R4), one file per subcommand in the existing package.
  The Makefile stays the single entry point ([ADR 0001](0001-tool-pinning-and-single-entry-point.md)).
- **Their tests become Go tests.** A ported script's bats file is deleted in the same PR as the Go tests that
  replace it, case for case. When the last bats file is gone, bats, its pin and `make test-sh` are removed.
  Commands are tested with `testscript` through `Params.Cmds`, logic with table tests (R3), which makes
  `github.com/rogpeppe/go-internal` (already in the module's `go.sum`) a direct dependency.
- **A bash bootstrap stays** where Go cannot run yet or must not fail open: tool installation and worktree setup
  (before `.bin/harness` exists), `layout.sh` (read by the Makefile at parse time and by CI before Go is set up),
  thin `.claude/hooks/` wrappers with `hook-bootstrap.sh` and `timing.sh` (a missing binary exits 127, which
  Claude Code does not treat as blocking), and the git hooks. This is exactly the set that stays (R2). Fixture
  overlays and payloads ([ADR 0009](0009-harness-selftest.md)) stay bash: they are test data.
- **A missing or stale binary blocks with a way out.** A ported hook wrapper reads its input once and lets
  `make setup` and `make .bin/harness` through *before* it calls the binary, so the escape works whatever an old
  binary answers (one built before the slice answers an unknown subcommand with usage text and exit 2). Every
  usage error (an unknown subcommand, a wrong argument or flag) exits 64 from then on. This changes the
  dispatcher's exit code for usage errors (today 2, `main.go:200-201`, with the comment at `main.go:30-31`
  updated alongside); it is outside the sensor output contract, which stays as it is. Wrappers and git hooks
  treat 64, 126 and 127, and an exit 2 whose stderr starts with the usage text, as a stale binary: they block
  and say to rebuild.
- **Go never runs a ported script.** Non-test harness Go runs no script under `scripts/` except the bootstrap; a
  callee is ported first or in the same slice (rule `harness-go-runs-no-script`).
- **New harness logic is Go.** From this ADR on, new harness logic is a subcommand, not a new `.sh` file outside
  the bootstrap and the fixture data. The plan's slice 2 proposes a sensor for it.
- **Subcommands register themselves.** Each subcommand's file registers its name, usage line and function in one
  registry from `init()`; a duplicate name panics at start-up. The usage text and `harness help` print the list
  from the registry and the package doc keeps none, so `main.go` holds no list. Inside `.bin/harness` this is the
  one place the harness prefers registration over explicit wiring
  ([core belief 1](../design-docs/core-beliefs.md)); the analyzers plugin already registers from `init()` for
  golangci's loader (ADR 0031). A product-only subcommand is then one more file, not an edit to `main.go` at every
  template sync. A product's variant of a shared subcommand takes its own name; the product then deletes the
  template's file and declines it in `.harness-version` (ADR 0026), or keeps a caller for it, because a name
  nothing calls fails `repo-no-orphan-subcommands`.
- **Subcommands take their root, environment and working directory as arguments** and never call `os.Exit`, so
  tests can call them in-process and in parallel.
- **The trigger list decides when the harness tests re-run.** Go's test cache rechecks only files inside the
  module root, by size and modification time, so a repo-root file a test reads or runs (`.claude/hooks/*`,
  `scripts/harness/*`, `protected-paths.txt`, `layout.env`) is not part of its key. In scope `changed`,
  `test-harness` runs with `-count=1` when `unchanged.sh` reports a change to `sh_sensor`'s pathspec (`*.sh`,
  `*.bash`, `*.bats`, `scripts/*`; `unchanged.sh` adds every protected path) or to another repo file a test
  reads, and uses the cache otherwise. A test reads only files inside the module or matched by that trigger.
  Executables are hardlinked into a test's sandbox, never copied (macOS checks every new executable on its first
  run, about 0.35 s), with a guard that fails the run when a test writes through a hardlink.
- **The output contract does not change** (ADR 0012): same rule ids, same `file:line: message [rule-id]` lines,
  same exit codes, fixtures still rejected.
- **The harness module is linted** (amends ADR 0024, whose scope left it out). A `lint-harness` target runs
  golangci in the module with its own protected config, with the size and complexity rules of ADR 0024 at
  starting numbers measured when it lands (a deliberate second set, which only goes down until it meets the
  product thresholds), plus `tparallel` and a ban on `os/exec` in tests outside one sandbox helper. Each of these
  gets a rule id and a fixture (ADR 0024 asks one id per measure).
- **Subcommands need a caller** (amends ADR 0031). Go code is never a referencer, and a dispatched subcommand is
  never dead code, so a new rule `repo-no-orphan-subcommands` asks that every key of the subcommand registry has
  an executable caller (Makefile, workflow, hook, setting or referenced script). It reads the keys from the
  registry in-process, not by parsing Go source.
- **Harness tests stay protected.** The module's tests, `_test.go` and `testdata/`, keep the module's `config`
  class in `protected-paths.txt`, so the `test-writer` stays out of them
  ([ADR 0039](0039-tests-are-written-apart-from-the-implementation.md)). Changing that class needs its own ADR.

The new rules (`repo-no-orphan-subcommands`, `harness-go-runs-no-script`, and the harness module's lint limits
and test bans) are adopted as `team-convention` under
[ADR 0015](0015-anticipated-rules-and-nfr-classes-b-o.md); this ADR is their decision record.

### ADR 0012 clauses this replaces
- "Port only what parses, in order of risk … `check-specs` and `check-commits` move only if they cause
  defects": every script moves, in the plan's order.
- "Orchestration stays in bash/make": orchestration leaves bash. The Makefile stays (ADR 0001).
- "for hooks only the parsing moves and the wrapper stays" and "The wrapper has a bats test for both paths":
  hook logic moves too (`protect-harness`'s Bash heuristic, `fast-check`'s routing); the wrapper stays and is
  tested from Go.
- "Not ported: the Bash branch of `protect-harness` stays a heuristic over command text": it is ported. The port
  keeps every block of the bash version and may add more; replacing it with guard-bash's AST walk is a separate
  change.
- "`integrity-check` computes its merge base in Go, the same way as `merge-base.sh` … kept in step by hand": the
  `merge-base` subcommand reuses the same Go function, so there is one implementation.

Everything else in ADR 0012 stays: one protected module, the output contract, thin hook wrappers, the staleness
rule for `.bin/harness`, the base-branch binary for `pull_request_target`, one port per PR.

## Consequences
- One language for harness logic and one test runner. bats goes, and with it 192 s of serial tests in
  `make check` on a harness change. Until the last bats file is gone, NFR-H4 stays broken on harness changes.
- A harness change re-runs all harness tests uncached; that is fail closed, and the merged suite needs a measured
  `budgets.txt` row (NFR-H5), which a human approves.
- More Go code to build before the harness runs; `.bin/harness` is already a prerequisite of `check`,
  `check-all` and `harness-selftest`.
- Workflows that run a ported script gain `setup-go` and build `.bin/harness` before any step that holds a
  secret. Under `pull_request_target` (`harness-integrity.yml`'s `pr-conventions` job, today without Go) it is
  built from the base checkout, never the PR head ([ADR 0010](0010-base-branch-gates.md)).
- The agent confinement and leak controls in `pr-review.sh` and `agent-gc.sh` are re-implemented; their port
  needs a human security review.
- All harness logic now lives under `<API_DIR>/tools/harness`, a layout-dependent path, where it was at
  `scripts/harness/` for every adopter. `sync-check` must handle another `API_DIR` (TD-129), and a layout check
  fails when no protected glob covers the module.
- Adopters: README adoption step 3, the "Generic parts" list, `drop.txt` and `adoption-probe.sh` name
  `scripts/test/` and bats files ([ADR 0038](0038-readme-brownfield-adoption-is-probed.md), TD-99). Each port
  updates them, and `make adoption-probe` stays green; the probe runs only in the template, which is one reason
  the port happens here first (R1). Once its script is gone, the probe's opt-out needs a key that
  `harness-integrity` reads; that key amends ADR 0038 and is decided in its own ADR when the probe's slice
  lands.
- Products take each slice through `make harness-sync-check` ([ADR 0026](0026-template-sync-is-a-drift-report.md)).
  A product that adapted a shared script decides before that slice whether to upstream the change or keep it as
  a separate subcommand file; from then on a local change to a shared sensor is a Go edit in a shared module, and
  every upstream change to that file a hand merge. Products port their own scripts after the template's slice.
- A product-only subcommand file lives in `package main` and compiles against the template's unexported helpers,
  where a product script today depends only on a command line. A template change to a helper breaks the product
  build only when it syncs. The plan names a small stable surface for product files (the command type, the
  value that carries root, environment and working directory, finding output, the layout reader, the sandbox
  helper); a change to it is a breaking template change and says so in its PR.
- Harness tests get the protection class of sensor code: `apps/api/tools/harness/*` is `config`, so every test
  change needs the `harness-change-approved` label, where `scripts/test/*` needs only CODEOWNERS review.
- Two code doubles remain, each kept in step by a shared vector file: `layout.sh` and `layout.go`, and the
  timing rows written by `timing.sh` (hooks) and the Go `run` (sensors).
- ADRs that name a bash script keep their text as history; the plan's slices update the guides and rules that
  name them.
- About 45 harness PRs in 11 slices, each needing a human. The pilot (`check-commits` and `protect-harness`)
  measures test time, lines and cache behaviour, and is the point to stop if the design does not hold.

## Alternatives considered
- **Keep bats and run it in parallel** (`bats --jobs` with GNU `parallel`). The product measured 32–43 s with
  eight files at a time, under NFR-H4. Rejected: it keeps two languages per script, adds a pinned tool, and the
  user decided bats goes.
- **Keep the bash scripts and test them from Go.** Removes bats but keeps bash's parsing and BSD/GNU drift, and
  the user decided the scripts move to Go. Rejected.
- **Move only `scripts/harness/`.** Leaves `obs-query.sh`, `new-domain.sh` and others in bash, so bats stays. It
  is option R2 (b), not chosen.
- **Point `.claude/settings.json` straight at `.bin/harness`** (R2 c). Removes the hook wrappers, but a missing
  binary exits 127 and the hook fails open. Not chosen, for the reason ADR 0012 gives.
- **Rely on Go's test cache with a read-first helper.** Reading a file in the test does not put it in the cache
  key when the file lies outside the module root. Rejected for the trigger rule.
- **Sub-packages per subcommand.** Each importing file carries the module path, which differs between template
  and product, so `sync-check` would report every such file as changed. Rejected for one package with a
  subcommand registry.
- **Port in the product first, or only there** (R1 b, c). A port only in a product turns every later template
  change to a script into a hand port from bash into Go, and the adoption probe, which proves that README step 3
  still names the files it copies, runs only in the template. Not chosen.
- **Plain Go table tests only, no `testscript`** (R3 b). No new direct dependency, but long fixture trees end up
  as `testdata/` directories and command cases read less like today's bats files. Not chosen.
- **Several binaries** (R4 b), for example one for hooks. Starts a little faster, but each binary is one more
  Make target, stamp, protected path and stale-binary case. Not chosen.
