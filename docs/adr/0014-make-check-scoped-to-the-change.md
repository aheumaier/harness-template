# ADR 0014: make check runs what the change can affect; check-all and CI run everything uncached

Status: accepted · 2026-10-28 · amended 2026-11-07 by [ADR 0044](0044-ios-ci-on-macos.md) (to save Actions
minutes, `check-ios` and `generator-probe` pass without running when their inputs did not change; `check-all`
and `harness-selftest` still run everything on every PR)

## Context
Agents run `make check` after every change, so its wall time is agent throughput (NFR-H3, ADR 0013). Most of
it was spent re-running sensors whose inputs had not changed: bats on every edit, golangci-lint and go test on
every package for a one-file change, gen-check regenerating everything. A sensor whose inputs did not change
can only repeat its last verdict. Skipping it is safe only if "its inputs" is computed correctly, and wrong
in the dangerous direction (a missed sensor) if it is not. Measured in exec plan 0002 (P6, P9, P11).

## Decision
- **Two scopes,** set by `HARNESS_SCOPE`: `changed` (the default, `make check`) and `full` (`make check-all`,
  every CI workflow, `make harness-determinism`). Any other value behaves as `full`.
- In scope `changed`, a sensor may be **skipped** when none of its inputs and no harness file
  (`protected-paths.txt`) changed since the merge base with `BASE_REF`, counting staged, unstaged and
  untracked files (`scripts/harness/unchanged.sh`; `gen-check.sh --unchanged` compares a stamp of its
  inputs and outputs from its last green run).
- In scope `changed`, `lint-go`, `test-go`, `lint-web` and `arch-web` are **narrowed** to the affected set:
  the changed Go packages or JS workspaces plus everything that depends on them, tests included
  (`scripts/harness/affected.sh`). This assumes every rule these sensors check is judged at an importer or
  inside a package (exec plan 0002, assumption A2); a whole-module rule (go-arch-lint) is not narrowed.
- In scope `changed`, tools may use **their own caches**: Go test results, the dependency-cruiser graph,
  Turborepo's task cache (which replays unchanged packages of `typecheck-web` and `test-web` instead of
  scoping them). In any other scope every tool runs uncached (`-count=1`, no `--cache`, `turbo --force`).
- **Fail closed:** no merge base, a git, `go list` or `turbo ls` error, a missing or unmatched stamp, or a
  change to a harness file, `go.mod`, `go.sum`, a `package.json`, the lockfile or workspace config means run
  everything.

## Consequences
- `make check` takes ~2 s for a one-file change instead of 42–60 s (exec plan 0002, NFR-H3), and stays
  proportional to the change, not the repo, at monorepo scale.
- "`make check` is green" (golden principle 6) now means "green for what this change can affect".
  `make check-all` before a PR and CI on every PR still run everything, so nothing merges on a scoped run.
- A trigger list that misses an input lets a sensor skip when it should run. The remediation is to add the
  input (rules.md#harness-scope); the full run in CI catches it before merge.
- The rules for skipping and scoping are harness code with bats tests (`unchanged.bats`, `gen-check.bats`,
  `affected.bats`), and the scoped sensors have fixtures that run in scope `changed`.
