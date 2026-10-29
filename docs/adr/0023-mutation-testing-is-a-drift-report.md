# ADR 0023: Mutation testing is a weekly drift report, never a gate (extends 0020)

Status: proposed · 2026-10-29

## Context
Exec plan 0003 (P2.6) asks for mutation testing. [ADR 0020](0020-test-coverage-is-a-trend.md) reports
coverage and names its blind spot: a test that executes code without asserting anything raises coverage. A
mutation score measures the assertions instead: it changes the code (a `<` becomes `<=`) and counts the
changes no test notices. It is slow (one test run per mutant: minutes for a week's changes, hours for the
repo) and would be gamed like coverage once it blocks a merge.

## Decision
- **What**: the Go and TS source files changed in the last 7 days, mutated by `make mutation`
  (`scripts/harness/mutation.sh`, [ci.md](../harness/ci.md#mutation-testing)). Changed means the tree diff
  between `HEAD` and the newest first-parent commit at or before the window start, so a branch committed
  earlier and merged inside the window counts. Score per package (killed and timed-out mutants over all
  valid ones) plus every surviving mutant.
- **A weekly drift report, never a gate.** `drift.yml` runs it weekly; the scores and survivors go to that
  run's job summary. The `drift` issue gets a mutation entry only when `make mutation` fails or times out,
  so it cannot measure. Not in `make check` (slow; the window reads the clock) or `check-all`, not a
  required check, no rule id. `mutation.sh` fails only when it cannot measure, never on a score. A threshold
  or ratchet needs its own ADR, justified by a finding as in ADR 0020.
- **The tools**, each meeting the 14-day age rule of [ADR 0016](0016-tool-supply-chain.md) on 2026-10-29:
  - Go: **gremlins v0.6.0** (released 2025-12-05), pinned in `scripts/tool-versions.env` and built by
    `go install`, so the module checksum comes from the Go checksum database (`sum.golang.org`), like the
    other `go install` tools. It is pre-1.0: flags and report format can change in a minor bump; the
    report parser fails closed on anything it does not know.
  - TS: **Stryker 10.0.0** (`@stryker-mutator/core`, `@stryker-mutator/vitest-runner`, released
    2026-08-14) as exact-pinned devDependencies of each workspace, covered by `pnpm-lock.yaml` integrity
    and `minimumReleaseAge`. Its dependency tree is large: it added about 75 packages to `pnpm-lock.yaml`
    (157 new `packages`/`snapshots` entries).
- **Workspace contract**: a JS workspace with a `test` script provides a `mutation` script (`stryker run`),
  the two Stryker devDependencies at the versions `apps/web` pins and a `stryker.config.json` copied from
  `apps/web`. A workspace with `test` but no `mutation` script fails the run with that remediation; one with
  neither reads "no tests".
- **What is measured lives in protected config**: `scripts/harness/gremlins.yaml` (passed with `--config`;
  every `GREMLINS_*` variable is unset, `GOFLAGS=-short` gives the test set of `make test-go`) and
  `*stryker.config.*` (protected, kind `config`). `mutation.sh` refuses a Stryker config that sets
  `thresholds.break` or `excludedMutations`.
- **First pnpm `overrides` convention**: a known-vulnerable transitive version is pinned in
  `pnpm-workspace.yaml` (protected; pnpm 12 ignores the `pnpm` field of `package.json`), scoped to its
  chain (`"parent>child": x.y.z`, never a bare package name), with the chain, the advisories and the drop
  condition in a comment and a tech-debt row. First entry: `"typed-rest-client>qs": 6.16.1` (Stryker 10 →
  `typed-rest-client` ~2.3.0 → `qs` 6.15.1 exactly), dropped when Stryker depends on `typed-rest-client`
  ≥ 3.

## Consequences
- **Coupling**: Stryker's tree is in `pnpm-lock.yaml`, so `deps-no-known-vulns` scans it on every PR, and
  an advisory against a package only mutation testing uses can turn an unrelated PR red. Accepted: the
  devDependencies run on developer machines and in CI, so they are attack surface the scan should see; the
  remedy is a Stryker bump or a scoped override, both small harness PRs. A separate lockfile or `pnpm dlx`
  would hide the tree from the scan and bypass `minimumReleaseAge`.
- A week without changed source files reports nothing; the score covers recent changes, not the repo.
- Unit tests only: code exercised only by integration tests reads as `nocoverage`.
- A score can fall because the week's changes were harder to test; the survivors list, not the number, is
  what a human or the garbage-collection agent acts on.
