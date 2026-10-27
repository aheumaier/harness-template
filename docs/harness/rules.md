# Harness rules

Every architectural rule exists twice: as a pointer an agent can read (here, linked from AGENTS.md) and as a
deterministic sensor that fails with the remediation below. `scripts/harness/run.sh` prints the
**Remediation** line of a rule verbatim when its sensor fails, so keep it one actionable sentence.

Status: **seed** = created with the template. New rules must be earned (link the incident/PR that motivated
them) and need a fixture in `scripts/harness/fixtures/` before they merge.

Format per rule: rule · why · sensor · remediation · fixture. `make harness-selftest` applies every fixture to
a temp copy of the repo and asserts its sensor fails with the expected message and this remediation
(`scripts/harness/selftest.sh`; fixture layout documented in its header).

## Go (apps/api)

### go-layers
- **Rule:** Per domain `model → store → service → api`. store may import model; service may import model, store, platform; api may import model, service, platform; platform imports no domain.
- **Why:** Keeps transport, business logic and persistence independently replaceable and testable; gives agents one place to put each kind of code.
- **Sensor:** `go-arch-lint` (`apps/api/.go-arch-lint.yml`), `make arch-go`.
- **Remediation:** Move the code to the layer that is allowed to own it (see ARCHITECTURE.md#layers); a service must not import api, a store must not import service, platform must not import any domain.
- **Fixture:** `fixtures/go-layers-service-imports-api`, `fixtures/go-layers-platform-imports-domain` · Status: seed

### go-domain-isolation
- **Rule:** `internal/<a>/**` must not import `internal/<b>/store` or `internal/<b>/api`.
- **Why:** A domain's storage and HTTP surface are private; coupling to them makes domains impossible to split or change independently.
- **Sensor:** custom analyzer `domainiso` (`apps/api/tools/analyzers/domainiso`) via golangci-lint, `make lint-go`.
- **Remediation:** Call the other domain's service (or use its model types) instead of its store or api package.
- **Fixture:** `fixtures/go-domain-isolation` · Status: seed

### go-db-only-in-store
- **Rule:** `github.com/jackc/pgx/**` and `database/sql` may only be imported in `internal/*/store` and `internal/platform`.
- **Why:** SQL lives in sqlc queries, reviewed and linted in one place; services stay testable without a database.
- **Sensor:** golangci-lint `depguard` rule `db-only-in-store`, `make lint-go`.
- **Remediation:** Add a query to `apps/api/db/queries/<domain>.sql`, run `make gen`, and call the generated store method from the service; refer to the pool as `db.Pool` from platform.
- **Fixture:** `fixtures/go-db-only-in-store` · Status: seed

### go-http-only-in-api
- **Rule:** `net/http` may only be imported in `internal/*/api`, `internal/platform` and `cmd`.
- **Why:** Transport concerns (status codes, headers) leaking into services couple business logic to HTTP.
- **Sensor:** golangci-lint `depguard` rule `http-only-in-api`, `make lint-go`.
- **Remediation:** Keep requests, responses and status codes in the api layer; pass plain Go values and typed errors to and from the service.
- **Fixture:** `fixtures/go-http-only-in-api` · Status: seed

### go-no-root-context
- **Rule:** `context.Background()` / `context.TODO()` only under `cmd/` or in `_test.go`.
- **Why:** Root contexts drop cancellation, deadlines and trace spans; requests then leak goroutines and lose traces.
- **Sensor:** custom analyzer `noctxroot` (`apps/api/tools/analyzers/noctxroot`) via golangci-lint, `make lint-go`.
- **Remediation:** Accept `ctx context.Context` as the first parameter and pass it down; in HTTP handlers use `r.Context()`; for work that must outlive the request use `context.WithoutCancel(ctx)`.
- **Fixture:** `fixtures/go-no-root-context` · Status: seed

### go-format
- **Rule:** Go code is `gofmt`/`goimports` formatted (local prefix `github.com/aheumaier/harness-template`).
- **Why:** Formatting noise hides real diffs from reviewers and agents.
- **Sensor:** golangci-lint formatters, `make lint-go`.
- **Remediation:** Run `make fmt`.
- **Fixture:** none (formatter, auto-fixable) · Status: seed

### go-tests-pass
- **Rule:** `go test -race` passes for the app and the analyzers.
- **Why:** Tests are the cheapest behavioural sensor; the race detector catches concurrency bugs agents rarely reason about.
- **Sensor:** `make test-go`, `make test-analyzers`.
- **Remediation:** Read the first failing test above, fix the code (not the test) unless the behaviour change is intended and documented in the PR.
- **Fixture:** none · Status: seed

### go-no-known-vulns
- **Rule:** No reachable known vulnerabilities in Go dependencies or the standard library.
- **Why:** Agents add dependencies freely; reachability analysis keeps the signal high.
- **Sensor:** `govulncheck`, `make vuln-go` (check-all, drift.yml).
- **Remediation:** Upgrade the module named in the finding (`go get <module>@<fixed-version>` in apps/api, then `go mod tidy`), or bump `GO_TOOLCHAIN` for stdlib findings.
- **Fixture:** none (depends on external advisory DB) · Status: seed

### go-no-dead-code
- **Rule:** No Go function is unreachable from `main` or the tests.
- **Why:** Agents leave orphans behind; dead code misleads the next agent reading the repo.
- **Sensor:** `deadcode -test` (golang.org/x/tools), `make deadcode-go` (drift.yml, weekly).
- **Remediation:** Delete the unreachable function, or call it from the code path (or test) that should use it.
- **Fixture:** none (weekly drift sensor) · Status: seed

## Database (apps/api/db)

### db-no-destructive-migration
- **Rule:** Migrations must not drop tables, columns or schemas (Atlas `destructive` analyzer, error level).
- **Why:** Drops lose data irreversibly and break running old binaries during deploys.
- **Sensor:** `atlas migrate lint` (Community Edition), `make lint-migrations`, `migration-gate.yml`.
- **Remediation:** Use expand/contract: stop reading the column in code first, ship, then drop it in a later migration with an `-- atlas:nolint DS1xx` directive and a reason, approved by CODEOWNERS.
- **Fixture:** `fixtures/db-destructive-migration` · Status: seed

### db-backward-compatible-migration
- **Rule:** Migrations must not rename or retype columns/tables in place (Atlas `incompatible` analyzer; in Community Edition renames surface as destructive drops, and squawk `renaming-column` also fires).
- **Why:** The previous app version is still running during a rolling deploy and will break.
- **Sensor:** `atlas migrate lint`, `squawk`, `make lint-migrations squawk`.
- **Remediation:** Add the new column, backfill, switch reads/writes in code, then remove the old column in a later migration.
- **Fixture:** `fixtures/db-rename-column` · Status: seed

### db-safe-locking
- **Rule:** Migrations pass squawk: set `lock_timeout`/`statement_timeout`, create indexes concurrently, no NOT NULL columns without defaults on existing tables, etc.
- **Why:** Long ACCESS EXCLUSIVE locks take production down; squawk knows Postgres lock semantics, Atlas does not.
- **Sensor:** `squawk` (`.squawk.toml`), `make squawk`, `migration-gate.yml`.
- **Remediation:** Apply the `help:` fix squawk prints above (start each migration with `SET lock_timeout = '2s'; SET statement_timeout = '30s';`; use `CREATE INDEX CONCURRENTLY` in a file marked `-- atlas:txmode none`).
- **Fixture:** `fixtures/db-unsafe-lock` · Status: seed

### db-append-only-migrations
- **Rule:** Never edit or delete a migration that exists on the base branch; add a new one.
- **Why:** Applied migrations are history; editing them makes environments diverge silently.
- **Sensor:** `scripts/harness/migrations-append-only.sh` (`make migrations-append-only`, in `make check-all` and `migration-gate.yml`; fast-check hook per migration, before `atlas.sum` is rehashed). Like every diff-based sensor it fails closed (exit 2) when `BASE_REF` does not resolve or has no merge base with `HEAD` (`scripts/harness/merge-base.sh`; fetch full history).
- **Remediation:** Revert your edit to the existing migration (`git checkout $BASE_REF -- <file>`) and put the change in a new one with `atlas migrate new <name> --env local` (see .claude/skills/add-migration).
- **Fixture:** `fixtures/db-edited-migration` · Status: seed

### db-schema-doc-fresh
- **Rule:** `docs/generated/db-schema.md` matches the migrations (stamped with the `atlas.sum` hash).
- **Why:** Agents read the schema doc instead of replaying migrations in their head.
- **Sensor:** `scripts/gen-schema-doc.sh --check` (offline) in `make check`; full regeneration in CI.
- **Remediation:** Run `make gen-schema-doc` (needs Docker) and commit the result.
- **Fixture:** covered by `fixtures/db-destructive-migration` side effects · Status: seed

## Contract (api/openapi.yaml)

### api-kebab-case-paths
- **Rule:** Path segments are lowercase kebab-case; path parameters are `{camelCase}`.
- **Why:** One URL style across domains; agents copy whatever they see first.
- **Sensor:** Spectral `api-kebab-case-paths` (`.spectral.yaml`), `make lint-api`.
- **Remediation:** Rename the path to lowercase words joined by `-` (e.g. `/order-items/{orderItemId}`), then `make gen`.
- **Fixture:** `fixtures/api-snake-case-path` · Status: seed

### api-camel-case-properties
- **Rule:** Schema property names are camelCase.
- **Why:** Generated Go and TS types map 1:1 to JSON without per-field tags or renames.
- **Sensor:** Spectral `api-camel-case-properties`, `make lint-api`.
- **Remediation:** Rename the property to camelCase (e.g. `created_at` → `createdAt`) and run `make gen`.
- **Fixture:** `fixtures/api-snake-case-property` · Status: seed

### api-operation-metadata
- **Rule:** Every operation has an `operationId` and exactly one tag (its owning domain).
- **Why:** operationId names the generated Go method and TS hook; the tag selects the generated server package (one strict server per domain).
- **Sensor:** Spectral `api-operation-id`, `api-operation-tags`, `api-operation-single-tag`, `make lint-api`.
- **Remediation:** Add a unique camelCase `operationId` (verbNoun, e.g. `listOrders`) and exactly one tag naming the owning domain.
- **Fixture:** `fixtures/api-missing-operation-id` · Status: seed

### api-documented-errors
- **Rule:** Every operation documents at least one 4xx and one 5xx response.
- **Why:** Clients (and the generated zod parsers) must know the error shape for every call.
- **Sensor:** Spectral `api-documented-4xx`, `api-documented-5xx`, `make lint-api`.
- **Remediation:** Add `'4XX': {$ref: '#/components/responses/ClientError'}` and `'5XX': {$ref: '#/components/responses/ServerError'}` to the operation's responses.
- **Fixture:** covered by `fixtures/api-error-not-problem-json` · Status: seed

### api-errors-problem-json
- **Rule:** Every 4xx/5xx response has only `application/problem+json` content (RFC 9457 `Problem` schema).
- **Why:** One error format end to end; `internal/platform/problem` writes it, the api-client parses it.
- **Sensor:** Spectral `api-errors-problem-json`, `make lint-api`.
- **Remediation:** `$ref` one of `#/components/responses/{ClientError,ServerError,ServiceUnavailable}`, or use content `application/problem+json` with schema `$ref: '#/components/schemas/Problem'`.
- **Fixture:** `fixtures/api-error-not-problem-json` · Status: seed

### api-no-breaking-change
- **Rule:** No breaking change to the contract compared to the merge base with `BASE_REF` unless a human approves it with the `breaking-change-approved` PR label. The spec is one file: every `$ref` is internal (`#/...`).
- **Why:** Deployed clients cannot be changed atomically with the server.
- **Sensor:** oasdiff `breaking --fail-on ERR` (`scripts/harness/contract-check.sh`), `make contract-check`, required check `api-no-breaking-change` (`.github/workflows/contract-gate.yml`). Waived only by `BREAKING_CHANGE_APPROVED=1`, set in CI by the `breaking-change-approved` PR label.
- **Remediation:** Make the change additive (new optional field, new path, deprecate before removing); if the break is intended, explain it and the client migration in the PR and ask a code owner to add the `breaking-change-approved` label. Agents never add the label. For an external `$ref`, inline the referenced schema under `components` and point at it with `#/components/...`.
- **Fixture:** `fixtures/api-breaking-change` · Status: seed

## Web (apps/web, packages/*)

### web-no-direct-http
- **Rule:** Only `packages/api-client` (generated) talks HTTP: no `fetch`/`XMLHttpRequest`/`window.fetch`, no HTTP libraries (axios, ky, …), no deep imports into the api-client.
- **Why:** Every call is typed from the contract and its response is zod-parsed at the boundary; hand-written calls bypass both.
- **Sensor:** ESLint `no-restricted-globals`/`-properties`/`-imports` (`eslint.config.mjs`), `make lint-web`.
- **Remediation:** Add the operation to `api/openapi.yaml`, run `make gen`, and use the generated `use<OperationId>` hook from `@harness-template/api-client` (e.g. `useGetHealth`).
- **Fixture:** `fixtures/web-direct-fetch` · Status: seed

### web-domain-pure
- **Rule:** `packages/domain` imports no React, TanStack, api-client or app code and uses no browser globals.
- **Why:** Domain logic stays testable in plain Node and reusable outside the browser.
- **Sensor:** dependency-cruiser `web-domain-pure` (`.dependency-cruiser.cjs`, `make arch-web`) for imports; ESLint `no-restricted-globals` (`make lint-web`) for globals.
- **Remediation:** Keep the function pure: take what it needs as parameters and do the React/browser/API wiring in `apps/web`.
- **Fixture:** `fixtures/web-domain-imports-react` · Status: seed

### web-no-cross-app
- **Rule:** An app under `apps/` never imports another app.
- **Why:** Apps deploy independently; cross-imports create hidden coupling and duplicate bundles.
- **Sensor:** dependency-cruiser `web-no-cross-app`, `make arch-web`.
- **Remediation:** Move the shared code into `packages/` (pure logic into `packages/domain`) and import it from there.
- **Fixture:** `fixtures/web-cross-app-import` · Status: seed

### web-lint
- **Rule:** TypeScript passes `typescript-eslint` `strictTypeChecked` and `react-hooks` recommended, with zero warnings and no unused disable directives.
- **Why:** Type-aware lint catches unsafe `any` flows and hook-order bugs that tsc accepts.
- **Sensor:** ESLint, `make lint-web`.
- **Remediation:** Fix the reported line; do not add `eslint-disable` (harness-integrity rejects disables without a reason and rule reference).
- **Fixture:** none · Status: seed

### web-typecheck
- **Rule:** All workspace packages type-check under `tsconfig.base.json` (`strict`, `noUncheckedIndexedAccess`).
- **Why:** The type system carries the contract from the OpenAPI spec into components.
- **Sensor:** `tsc --noEmit` per package via Turborepo, `make typecheck-web`.
- **Remediation:** Fix the type error at its source; never widen to `any` or add `@ts-ignore`/`@ts-expect-error`.
- **Fixture:** none · Status: seed

### web-tests-pass
- **Rule:** Vitest suites pass.
- **Why:** Cheapest behavioural sensor for the frontend.
- **Sensor:** `vitest run` per package via Turborepo, `make test-web`.
- **Remediation:** Read the first failing test above and fix the code (not the test) unless the behaviour change is intended and documented in the PR.
- **Fixture:** none · Status: seed

### web-no-dead-code
- **Rule:** No unused files, exports or dependencies in the JS workspace.
- **Why:** Agents leave orphans behind; dead code misleads the next agent reading the repo.
- **Sensor:** knip (`knip.json`), `make knip` (check-all, drift.yml).
- **Remediation:** Delete the unused file/export/dependency; if it is used in a way knip cannot see, configure it in `knip.json` and say why in the PR.
- **Fixture:** none · Status: seed

## Generated code

### gen-up-to-date
- **Rule:** Generated code (sqlc stores, oapi-codegen servers, the orval API client, `atlas.sum`) is committed, current, and never hand-edited.
- **Why:** Generated code is a projection of the contract/schema; hand edits are silently lost and stale output hides contract drift.
- **Sensor:** `scripts/harness/gen-check.sh` (`make gen-check`, part of `make check`).
- **Remediation:** Change the source (api/openapi.yaml, db/queries/*.sql, db/migrations) instead, run `make gen`, and commit the regenerated files.
- **Fixture:** `fixtures/gen-hand-edit-api-client` · Status: seed

## Shell (scripts/, .claude/hooks/)

### sh-shellcheck
- **Rule:** Every shell script and bats file passes shellcheck (`.shellcheckrc`); a `# shellcheck disable=` needs a reason ([harness-suppression-reason](#harness-suppression-reason)).
- **Why:** The harness itself is shell: an unquoted variable or a failed `cd` in a sensor or hook silently turns it green or deletes the wrong path.
- **Sensor:** shellcheck (`make lint-sh`, part of `make check`; fast-check hook per file).
- **Remediation:** Fix the line shellcheck names (the SCxxxx code links to https://www.shellcheck.net/wiki/SCxxxx); quote expansions, add `|| exit` after `cd`, and prefer an explicit `if` to `a && b || c`.
- **Fixture:** `fixtures/sh-unquoted-variable` · Status: seed

### sh-format
- **Rule:** Shell scripts and bats files are `shfmt` formatted (style in `.editorconfig`: 2 spaces, indented `case`).
- **Why:** Formatting noise hides real diffs from reviewers and agents.
- **Sensor:** shfmt (`make fmt-check-sh`, part of `make check`; fast-check hook per file).
- **Remediation:** Run `make fmt`.
- **Fixture:** `fixtures/sh-unformatted` · Status: seed

### sh-tests-pass
- **Rule:** The bats tests in `scripts/test/` pass; a new script under `scripts/` or `.claude/hooks/` with branching logic comes with a `scripts/test/<script>.bats` (existing gaps: docs/exec-plans/tech-debt-tracker.md).
- **Why:** Sensors and hooks are code that guards all other code: a hook that blocks too much stalls agents, one that blocks too little is a false sense of safety.
- **Sensor:** bats-core (`make test-sh`, part of `make check`). Scripts that need the full toolchain (`selftest.sh`, `gen-check.sh`, `contract-check.sh`, `new-domain.sh` end to end) are exercised by `make harness-selftest` and the CI generator probe instead.
- **Remediation:** Read the first `not ok` test and its `expected output to contain` lines, fix the script (not the test) unless the behaviour change is intended and stated in the PR.
- **Fixture:** none (like go-tests-pass) · Status: seed

## Harness integrity and CI (.github/)

### harness-suppression-reason
- **Rule:** Every added suppression (`//nolint`, `//lint:ignore`, `//lint:file-ignore`, `eslint-disable`, `/* eslint … */`, `@ts-expect-error`, `# shellcheck disable`, `-- atlas:nolint`, `-- squawk-ignore`) has `reason: … (rule: <id>)` on the same line, where `<id>` is a `### <id>` heading in this file; `@ts-ignore` and `@ts-nocheck` are never allowed.
- **Why:** A bare suppression silently turns a sensor off; the reason and rule id make each exception reviewable and greppable.
- **Sensor:** `scripts/harness/integrity-check.sh` on lines added since the merge base with `BASE_REF` in Go, TS/JS, shell, SQL and workflow files (`make harness-integrity`, in `make check-all` and `harness-integrity.yml`, which runs the base branch's copy of the script; the PR's `.gitattributes` cannot hide a hunk).
- **Remediation:** Fix the code instead; if the suppression is truly needed, append `reason: <why it is safe> (rule: <rule-id>)` on the same line with an id from this file as it is on the base branch, and replace `@ts-ignore`/`@ts-nocheck` with a fixed type or a reasoned `@ts-expect-error`.
- **Fixture:** `fixtures/harness-suppression-no-reason` · Status: seed

### harness-config-not-loosened
- **Rule:** No non-comment line of a harness config (`config` entries in `scripts/harness/protected-paths.txt`: linter configs, Makefile, workflows, CODEOWNERS, `.gitattributes`, tool versions, the protected-path list itself, and sensor code: `scripts/harness/`, custom analyzers, Claude hooks, the tool installer) is added or removed unless a human approves. Tightening counts too: telling it from loosening takes judgment. Only `#` and blank lines count as comments (`//` and `/* */` can comment code out).
- **Why:** The cheapest way to make a red sensor green is to weaken its config; that must be a visible human decision, not an agent's shortcut.
- **Sensor:** `scripts/harness/integrity-check.sh` on the diff since the merge base with `BASE_REF` (`make harness-integrity`, `harness-integrity.yml`; CI runs the base branch's workflow, script and path list via `pull_request_target` ([ADR 0010](../adr/0010-base-branch-gates.md)), so a PR cannot unprotect itself). Waived only by `HARNESS_CHANGE_APPROVED=1`, set in CI by the `harness-change-approved` PR label.
- **Remediation:** Revert the config change and fix the code; if the rule itself is wrong, explain why in the PR description (Harness change: Y + rationale) and ask a code owner to add the `harness-change-approved` label.
- **Fixture:** `fixtures/harness-config-loosened` · Status: seed

### harness-files-protected
- **Rule:** Agents do not edit harness files (`scripts/harness/protected-paths.txt`); they propose the change in the PR description. A human allows edits by starting the agent session with `HARNESS_EDIT=1`.
- **Why:** A sensor the agent can edit is a suggestion: the cheapest fix for a red check is to change the check.
- **Sensor:** PreToolUse hook `.claude/hooks/protect-harness.sh` (file tools exactly; shell writes heuristically) for Claude Code; `.github/CODEOWNERS` review and `harness-config-not-loosened` for every agent.
- **Remediation:** Do not edit the file: fix the code the sensor points at, or describe the harness change (file, diff, why) in the PR description under "Harness change".
- **Fixture:** none (a hook, not a make target); `scripts/test/protect-harness.bats` · Status: seed

### ci-workflows-valid
- **Rule:** GitHub workflows and composite actions pass actionlint, including shellcheck on every `run:` block.
- **Why:** A broken workflow fails only after push, or worse, never runs the gate it was meant to run.
- **Sensor:** actionlint (`make lint-ci`, part of `make check`).
- **Remediation:** Fix the line actionlint names (unknown context/expression, bad `needs`/`if`, or the shellcheck code in a `run:` block); pin new actions to a full commit SHA with a `# vX.Y.Z` comment.
- **Fixture:** `fixtures/ci-invalid-workflow` · Status: seed

## Docs

### docs-links-resolve
- **Rule:** Every relative link and `#anchor` in tracked Markdown files resolves.
- **Why:** Guides are the agent's map; a dead link silently sends it searching (or guessing) instead of reading.
- **Sensor:** `scripts/harness/check-links.sh` (`make docs-links`, part of `make check`).
- **Remediation:** Point the link at the file's current path (or heading slug), or remove it if the target is gone; when you move a doc, update every link to it in the same change.
- **Fixture:** `fixtures/docs-broken-link` · Status: seed

### docs-claude-imports-agents
- **Rule:** Every `AGENTS.md` has a `CLAUDE.md` in the same directory with the line `@AGENTS.md` (plus any Claude-Code-only notes).
- **Why:** Claude Code loads `CLAUDE.md`, not `AGENTS.md`; without the import an area guide is invisible to it, while other agents read `AGENTS.md` directly. One source, two entry points.
- **Sensor:** `scripts/harness/check-links.sh` (`make docs-links`, part of `make check`).
- **Remediation:** Create `CLAUDE.md` next to the `AGENTS.md` the message names, containing the line `@AGENTS.md`; put shared guidance in `AGENTS.md`, never only in `CLAUDE.md`.
- **Fixture:** `fixtures/docs-agents-without-claude` · Status: seed
