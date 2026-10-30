# ADR 0030: The product layout is declared once in layout.env (amends 0002)

Status: proposed · 2026-10-29

## Context
Gap G12 of exec plan 0003 (`docs/exec-plans/*/0003-harness-gap-closure.md`, slice P4.6): the template layout
(`apps/api`, `api/openapi.yaml`, `apps/api/db/migrations`, `internal/<domain>/<layer>`, ...) was hard-coded in
about fifteen protected files: scripts, the Makefile (`API := apps/api`), the Go harness tool, the domainiso
analyzer and the CI workflows. A brownfield repo had to edit each by hand, and a missed one did not go red: it
skipped, so its sensor looked green while it checked nothing. [ADR 0002](0002-go-service-layout.md) fixes the
Go module at `apps/api`; brownfield adoption (README "(b) Brownfield project") needs it elsewhere, including
the repo root.

## Decision
- **The product layout is declared once, in the protected `scripts/harness/layout.env`.** Nine keys: the
  strings `API_DIR`, `SPEC`, `MIGRATIONS_DIR`, `QUERIES_DIR`, `API_CLIENT_DIR`, `LAYER_GLOB` and the
  `:`-separated lists `WEB_GLOBS`, `GENERATED_GLOBS`, `PRODUCT_GLOBS`. This amends ADR 0002: the module's
  location is configurable (`API_DIR`, `.` for the repo root); `apps/api` stays the template's value.
- **Every sensor reads it through one parser per runtime, and fails closed.** The file is data, never executed,
  sourced or included:
  - `scripts/harness/layout.sh` for the scripts (sourced), and with `--print` for the Makefile
    (`$(shell …)` plus `$(eval)` of the validated `KEY=value` lines) and CI (`>>"$GITHUB_ENV"`);
  - `apps/api/tools/harness/layout.go` for `.bin/harness` (`layout-check`, integrity-check's excludes from
    `GENERATED_GLOBS`, quality-rows' domains directory from `API_DIR` and `LAYER_GLOB`).

  Both accept only the known keys, each once, each value from `[A-Za-z0-9_./*@+-]` with no empty list entry.
  A missing file, an unknown, repeated or missing key, or a bad value stops the reader with
  `<file>:<line>: <message> [harness-layout-valid]`, never a default. The Makefile errors at parse time,
  sourced scripts exit 2. `make layout-check` (in `make check`) also checks that each path exists and that
  `LAYER_GLOB` matches at least one Go package.
- **Named exceptions: the files that cannot read it.** Linter configs (`.golangci.yml`, `.go-arch-lint.yml`,
  `.custom-gcl.yml`, `eslint.config.mjs`, `.dependency-cruiser.cjs`), CODEOWNERS, `protected-paths.txt`,
  `jscpd.json`, the codegen configs (`atlas.hcl`, `sqlc.yaml`, `orval.config.ts`), `pnpm-workspace.yaml`,
  `knip.json`, the `make new-domain` generator and `.claude/hooks/fast-check.sh` keep their literal paths.
  README "(b) Brownfield project" step 5 is the single list of them, each with what goes wrong when it is left
  alone.
- **Which copy each workflow reads.** Gates that run with secrets on `pull_request_target`
  (`harness-integrity.yml`, `contract-gate.yml`) load the **base** branch's `layout.env`, as
  [ADR 0010](0010-base-branch-gates.md) requires for every input of a base-branch gate: a PR cannot re-point
  the sensor that judges it. `ci.yml` jobs load the PR's copy through the setup action; they run without
  secrets, and a changed `layout.env` is itself a protected change that needs the label.
  `migration-gate.yml` and `migrations-append-only.sh` read **both** copies: a PR that moves
  `MIGRATIONS_DIR` diffs the old and the new directory, and a migration that moved unchanged (same blob) is
  allowed while one edited on the way is not. A `paths:` filter cannot read the layout, so
  `migration-gate.yml` always starts and a diff step decides whether to run.
- **`API_CLIENT_DIR` is a key** because `make gen-api-client` and gen-check run orval there; the other web
  paths derive from `WEB_GLOBS`. There is no `WEB_APP_DIR` key: the web app's remaining uses (`make dev`,
  `make e2e`, Playwright install, the setup action's browser cache) stay literal and are listed in step 5 and
  in [TD-46](../exec-plans/tech-debt-tracker.md).
- **`LAYER_GLOB` is two levels deep, `<dir>/*/*`** (domain, then layer), relative to `API_DIR`. The domainiso
  analyzer derives its layer prefix from it. Every segment of `<dir>` is a literal name: empty segments, `.`,
  `..` and wildcards are rejected, as are a leading or trailing `/`.
- **domainiso keeps its own parser** (an exception to [ADR 0012](0012-parsing-sensors-in-go.md)'s one parser
  per concern). It is a separate Go module, loaded by golangci-lint as a plugin, and `tools/harness` is a main
  package it cannot import. It reads only `LAYER_GLOB` and fails closed without it. To keep the two Go parsers
  from drifting, both test against one table of vectors,
  `apps/api/tools/harness/testdata/layer-globs.txt` (`<glob> <prefix|reject>`).
- **Platform is no domain; flat packages fail.** domainiso skips `<dir>/platform` and every flat package
  `<dir>/<pkg>` (it needs a domain and a layer). `layout-check` reports each flat package other than
  `platform` by name, so a flat `internal/<pkg>` layout goes red instead of silently unchecked. A tree with
  only `platform` packages passes: that is the template before its first domain, and domainiso has nothing to
  check until `make new-domain` adds one. Excluding `platform` from the "matches a package" count was
  considered and rejected: the template has no domain yet, so it would turn the template's own `make check`
  red. The flat-package finding covers the same blind spot.

## Consequences
- Re-pointing a brownfield repo is one file plus the step-5 list, not fifteen protected files.
- **Residual risks.**
  - The step-5 list is maintained by hand. A new file that hard-codes a template path and is missing there
    goes unnoticed until an adopter trips on it; reviewers check new literal paths against it.
  - Base-copy workflows see the old layout on a PR that moves a path. A PR that moves `SPEC` runs
    `contract-gate` against the base's `SPEC`, so the move shows as a removed contract; such a PR needs the
    label and a human review anyway.
  - golangci-lint's analysis cache does not key on `layout.env`: after a `LAYER_GLOB` change, cached packages
    can replay their old domainiso verdict until their Go files change ([TD-65](../exec-plans/tech-debt-tracker.md)).
    `make layout-check` still catches a glob that matches no package.
  - A flat `internal/<pkg>` layout cannot express a layer: such a repo restructures or drops
    go-domain-isolation and go-layers.
  - `layout.env` is protected config: an adopter's first edit needs the `harness-change-approved` label once,
    and template sync reports the file whenever the template changes its copy.
  - Template sync reads the template's literal paths: a harness file under the template's `apps/api/` lives
    under `API_DIR` in a re-pointed repo, so each upstream change there is listed as a file this repo no
    longer has, and is ported by hand to its `API_DIR` copy, not declined ([TD-129](../exec-plans/tech-debt-tracker.md)).
