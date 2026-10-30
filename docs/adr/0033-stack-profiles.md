# ADR 0033: Stack profiles select the sensor groups a product has (amends 0030, 0009, 0023)

Status: accepted · 2026-10-29

## Context
Gap G12 of exec plan 0003 (`docs/exec-plans/*/0003-harness-gap-closure.md`, slice P4.7): the harness assumed
every product has the Go API, Postgres and the web app. A Go-only service had to keep `apps/web`, `packages/`
and `apps/api/db` alive or hand-edit `check`, `check-all`, `install-tools.sh` and `setup-worktree.sh`: pnpm
was installed and the root `package.json` checked, Postgres started (or `SKIP_DB=1` on every setup), and the
web and migration sensors failed on paths that do not exist. [ADR 0030](0030-one-layout-file-for-every-sensor.md)
made `scripts/harness/layout.env` the one place the product's shape is declared, and fails closed on a missing
path, so a removed `apps/web` turned `make layout-check` red.

## Decision
- **The stack is a tenth `layout.env` key, `STACK`**, a `:`-separated list of sensor groups, `go:db:web` in
  the template. A group left out drops its sensors from `make check` and `make check-all`, its generators from
  `make gen`, its tools from `make tools`, its fixtures from `make harness-selftest` and, for db, Postgres from
  `make setup`. One mechanism, not a second config file: the same two parsers (`layout.sh`, `layout.go`) read
  and validate it, against shared vectors (`apps/api/tools/harness/testdata/stack-vectors.txt`).
- **`STACK` is optional and defaults to `go:db:web`.** The other nine keys stay mandatory. A reader also parses
  older copies of `layout.env`: `migrations-append-only.sh` and the migration gate read the base branch's
  copy, which predates the key until the adopting PR merges. **A new layout key must keep readers of older
  copies working**: optional with the old behaviour as its default, or the change ships a migration path for
  the base copy. The default is the old behaviour, so a missing key never drops a sensor.
- **`HARNESS_STACK` in the environment overrides it for one run** (`HARNESS_STACK=go make setup check`), with
  `,` or `:` between groups. Both parsers apply it, and `layout.sh --print` emits the effective `STACK`, so
  the Makefile and CI see one value. On the make command line (`make check HARNESS_STACK=go`) the Makefile
  passes it on to `layout.sh` too. `make check` names the targets the stack leaves out on stderr, once.
- **The groups and what they own**:
  - `go` (mandatory): the Go API, the contract, the shell and harness sensors, every target not named below.
  - `db`: `MIGRATIONS_DIR`, `QUERIES_DIR`; `schema-doc-fresh`, `lint-migrations`, `squawk`,
    `migrations-append-only`, `test-integration`, `test-obs`, `gen-sqlc`, `gen-atlas-sum`; sqlc, Atlas and
    squawk; Postgres in `make setup`.
  - `web`: `API_CLIENT_DIR`, `WEB_GLOBS`; `lint-web`, `arch-web`, `typecheck-web`, `test-web`, `knip`,
    `clones-web`, `gen-api-client`; pnpm, the root `package.json` check and `make deps`.
  - `e2e` needs both db and web.

  The Makefile names them once (`DB_TARGETS`, `WEB_TARGETS`, tested by `scripts/test/make-stack.bats`) and
  drops them with `$(call stacked,…)`; `$(call stack_has,<group>)` gates the rest.
- **Fail closed, both ways.**
  - A set `STACK` (or `HARNESS_STACK`) must be a non-empty list of known groups (`go`, `db`, `web`), each
    named once, and include `go`. Anything else stops the reader with `[harness-layout-valid]`, like every
    other bad layout value. A typo never silently drops a group.
  - `layout-check` skips a left-out group's paths, but reports the group when the tree still has its code: a
    git-known migration or query (db), or a `package.json` under a `WEB_GLOBS` entry or the `API_CLIENT_DIR`
    directory (web). So `HARNESS_STACK=go make check` on a full tree is red, not a quiet way around the db and
    web sensors (fixture `harness-stack-hides-web`).
- **The criterion for `.bin` versus npm: who needs the tool.** A tool that a stack without web still needs
  is a `.bin` tool, installed by `install-tools.sh` and sha256-pinned per OS/arch in the protected
  `scripts/tool-versions.env` under [ADR 0016](0016-tool-supply-chain.md) (version, digests, 14-day age rule,
  a provenance note at each bump). A tool only the web group runs stays an npm devDependency in
  `pnpm-lock.yaml`, under the JS controls (sha512 integrity, `minimumReleaseAge`, Trivy over the lockfile).
  So:
  - spectral (`lint-api`, go), squawk (db) and bats (`test-sh`, go) move from `package.json` to `.bin`, at
    the versions `package.json` had. Provenance at adoption: the spectral and squawk pins match GitHub's
    release asset digests (neither project publishes a checksum file); bats is the npm registry's tarball,
    one arch-independent pin, whose sha512 matched the integrity `pnpm-lock.yaml` recorded for bats@1.13.0.
  - jscpd ([ADR 0027](0027-token-clones-are-a-maintainability-sensor.md)) and Stryker
    ([ADR 0023](0023-mutation-testing-is-a-drift-report.md)) stay in the lockfile: they judge only JS code.
- **This narrows ADR 0023's argument for the lockfile.** ADR 0023 (Consequences, coupling) keeps Stryker's
  tree in `pnpm-lock.yaml` so `deps-no-known-vulns` sees attack surface that runs in CI. That still holds for
  web-only tools. For the three tools above, a Go-only stack cannot have the lockfile at all, so the trade
  goes the other way: their bundled dependencies leave the scan (below, TD-86) in exchange for no pnpm. ADR
  0027's "not a `.bin` tool" for jscpd follows from the same criterion.
- **Fixtures follow their target (amends [ADR 0009](0009-harness-selftest.md)).** `selftest.sh` asks the
  Makefile for the targets the stack leaves out (`STACK_OFF`) and skips their fixtures
  (`- <name>: skipped (STACK=go leaves out lint-web)`). A fixture whose target is in the go group but whose
  overlay needs another group (an `apply.sh` that edits `apps/web`) names it in `fixture.conf` (`STACK=web`);
  an unknown group there is a bad `fixture.conf`. This is a second skip class beside ADR 0009's
  `SELFTEST_SKIP_DOCKER`, and it cannot hide a fixture: under the full stack a skip is a failure, so
  NFR-R3 (every rule with a fixture rejects it) is measured on the full template, as before.
- **CI reads the same key.** The setup action loads `layout.sh --print` (now with `STACK`), keys its caches on
  `layout.env`, and caches the pnpm store only when web is in the stack. `make deps` is a no-op without web.
  `gen-check.sh`'s stamp includes the effective `STACK`, so a run under another stack never reuses a verdict.

## Consequences
- In a copy of the template without `apps/web`, `packages/` and `apps/api/db`,
  `HARNESS_STACK=go make setup check harness-selftest` is green: no pnpm, no `node_modules`, no Postgres;
  50 fixtures pass and 20 are skipped. The adopter first drops what links to the removed directories
  (README "(b) Brownfield project" step 6): in the template, the `apps/api/db/AGENTS.md`, `apps/web/AGENTS.md`
  and orval links in `AGENTS.md`, and `docs/references/orval-llms.txt`, whose pin file is
  `packages/api-client/package.json`. `check-all` was not run on that copy.
- A full stack is unchanged: same targets, same fixtures (70 with `harness-stack-hides-web`).
- **NFR-B1 and NFR-B2 do not apply without db and web.** NFR-B1's scaffold integration test needs Postgres
  and NFR-B2's journey needs the web app; a stack without them has no such target to measure
  ([nfr.md](../harness/nfr.md)).
- **Residual risks.**
  - Targets outside `check`, `check-all`, `gen`, `tools`, `deps`, `setup` and `harness-selftest` are not
    stack-aware: `make dev`, `db-up`, `e2e-deps`, `new-domain` and `test-scaffold` assume the template stack,
    as do the CI jobs that call them, `migration-gate.yml` and the drift run's `gen-schema-doc`
    ([TD-83](../exec-plans/tech-debt-tracker.md), [TD-84](../exec-plans/tech-debt-tracker.md)).
  - `.claude/hooks/fast-check.sh` is protected and keeps its db and web branches. On a Go-only stack their
    paths are mostly gone, but an edit to a kept `sqlc.yaml` runs `sqlc`, which `make tools` no longer
    installs: the per-file check fails with "command not found" ([TD-85](../exec-plans/tech-debt-tracker.md)).
  - spectral, squawk and bats left `pnpm-lock.yaml`, so `deps-no-known-vulns` no longer sees what
    they bundle, and pnpm's `minimumReleaseAge` no longer gates their bumps (ADR 0016's age rule
    does). spectral is a `pkg`-bundled Node runtime plus its npm tree in one binary, and nothing scans it
    ([TD-86](../exec-plans/tech-debt-tracker.md)). Their versions are in a protected file, so a bump is a
    harness PR.
  - The Linux amd64 and arm64 pins of spectral and squawk match GitHub's digests but were not installed on
    Linux in this slice; CI's first `make setup` is that proof.
  - A db or web target added later must be added to `DB_TARGETS` or `WEB_TARGETS`, or a stack without the
    group runs it and fails on a missing path. The failure is loud, not silent.
