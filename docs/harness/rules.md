# Harness rules

Every architectural rule exists twice: as a pointer an agent can read (here, linked from AGENTS.md) and as a
deterministic sensor that fails with the remediation below. `scripts/harness/run.sh` prints the
**Remediation** line of a rule verbatim when its sensor fails, so keep it one actionable sentence.

Status: **seed** = created with the template; **team-convention** = a convention the team already follows,
adopted as a rule by a recorded decision (ADR) instead of an incident. New rules must be earned (link the
incident/PR that motivated them) and need a fixture in `scripts/harness/fixtures/` before they merge.

Format per rule: rule · why · sensor · remediation · fixture. `make harness-selftest` applies every fixture to
a temp copy of the repo and asserts its sensor fails with the expected message and this remediation
(`scripts/harness/selftest.sh`; fixture layout documented in its header). The fixture line names only fixtures
whose `fixture.conf` tests that rule, or says `none (<reason>)` ([rules-fixture-claims](#rules-fixture-claims)).

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

### go-test-helpers-only-in-tests
- **Rule:** `internal/platform/approvaltest` may only be imported by `_test.go` files.
- **Why:** The helper exists for handler tests: it reads an environment variable and writes files under `testdata/`, so production code that imported it would ship test behaviour; the package lives under `platform` only because no other location passes `go-layers` and `go-domain-isolation` ([ADR 0022](../adr/0022-approved-api-responses.md)).
- **Sensor:** golangci-lint `depguard` rule `go-test-helpers-only-in-tests` (`$all`, `!$test`), `make lint-go`.
- **Remediation:** Import `approvaltest` only from a `_test.go` file; production code never needs it.
- **Fixture:** `fixtures/go-test-helper-in-prod` · Status: team-convention ([ADR 0022](../adr/0022-approved-api-responses.md))

### go-no-root-context
- **Rule:** `context.Background()` / `context.TODO()` only under `cmd/` or in `_test.go`.
- **Why:** Root contexts drop cancellation, deadlines and trace spans; requests then leak goroutines and lose traces.
- **Sensor:** custom analyzer `noctxroot` (`apps/api/tools/analyzers/noctxroot`) via golangci-lint, `make lint-go`.
- **Remediation:** Accept `ctx context.Context` as the first parameter and pass it down; in HTTP handlers use `r.Context()`; for work that must outlive the request use `context.WithoutCancel(ctx)`.
- **Fixture:** `fixtures/go-no-root-context` · Status: seed

### go-structured-logging
- **Rule:** No `fmt.Print`/`Printf`/`Println` anywhere in the API module, tests included (the invariant `internal/platform/logging/logging.go` states; `fmt.Fprint*` to an explicit writer stays allowed), no std `log` package outside `internal/platform`, and no `slog` call that mixes key-value pairs and `slog.Attr` values (plan 0003 Q4). Not covered, so review owns them: `fmt.Fprint*` to `os.Stdout` or `os.Stderr`, `os.Stdout.Write`/`os.Stderr.Write`, and the builtin `print`/`println`. Suppress with `//nolint:forbidigo // reason: … (rule: go-structured-logging)` (or `nolint:sloglint`).
- **Why:** Agents read the running app through its JSON logs; unstructured lines on stdout cannot be filtered by request ID or level, and mixed slog args are easy to misalign.
- **Sensor:** golangci-lint `forbidigo` (by package path, `analyze-types`) and `sloglint` (`no-mixed-args`, no key-case check yet: plan 0003 P4.1), `make lint-go` (the per-edit hook runs the same linters but does not list this rule id yet, so it prints the other `lint-go` rules' remediations instead of this one). sloglint's message cannot carry the rule id, so a sloglint-only failure prints every `lint-go` remediation, and one next to another rule's finding prints only that rule's.
- **Remediation:** Log through the injected `*slog.Logger` (built by `logging.New` in platform) with either key-value pairs or `slog.Attr` values in one call, not both, or return the error to the caller instead of printing it.
- **Fixture:** `fixtures/go-structured-logging-fmt-print`, `fixtures/go-structured-logging-slog-mixed-args` · Status: team-convention ([ADR 0015](../adr/0015-anticipated-rules-and-nfr-classes-b-o.md))

### go-sec
- **Rule:** No gosec finding in the API module (SAST: file paths built from variables, some subprocess calls built from variables, weak crypto or randomness, unchecked integer conversions, hard-coded credentials, servers without timeouts, …), tests included, since a test can still leak a credential or shell out unsafely (plan 0003 Q6). Generated code is excluded. Suppress a false positive on its line only, with `//nolint:gosec // reason: … (rule: go-sec)`; never disable a gosec check globally. gosec does not catch every subprocess call built from variables: G204 in the pinned gosec does not fire on `exec.Command(name)` with `name` a function parameter (exec plan 0003, A13), so review owns every `exec.Command`. Typical fixes: `os.Root` or a fixed root plus `filepath.Clean` for paths, a fixed argv for `exec.Command`, `crypto/rand`, bounds-checked conversions, credentials from config, `ReadHeaderTimeout` on servers.
- **Why:** Agents write code faster than humans review it; a computational SAST sensor catches the common Go security mistakes before review does (Böckeler, SAST as a computational sensor).
- **Sensor:** golangci-lint `gosec` with all checks at its defaults, `make lint-go` (the per-edit hook runs the same linters but does not list this rule id yet, so it prints the other `lint-go` rules' remediations instead of this one). Semgrep stays out until a domain needs what gosec cannot cover. gosec's message (`G304: … (gosec)`) cannot carry the rule id, so a gosec-only failure prints every `lint-go` remediation, and one next to another rule's finding prints only that rule's. The separate modules `tools/analyzers` and `tools/harness` are not linted by `lint-go`.
- **Remediation:** Fix what the finding's `G<nnn>` id names (see the gosec rule list); for a false positive add `//nolint:gosec // reason: … (rule: go-sec)` on that line only.
- **Fixture:** `fixtures/go-sec-file-inclusion` · Status: team-convention ([ADR 0015](../adr/0015-anticipated-rules-and-nfr-classes-b-o.md))

### go-format
- **Rule:** Go code is `gofmt`/`goimports` formatted (local prefix `github.com/aheumaier/harness-template`).
- **Why:** Formatting noise hides real diffs from reviewers and agents.
- **Sensor:** golangci-lint formatters, `make lint-go`; the `pre-commit` git hook (`scripts/harness/pre-commit.sh`) on the staged Go files (`golangci-lint fmt --diff`).
- **Remediation:** Run `make fmt`.
- **Fixture:** none (formatter, auto-fixable) · Status: seed

### go-tests-pass
- **Rule:** `go test -race` passes for the app; `go test` for the analyzers and the hook parser (`apps/api/tools/harness`).
- **Why:** Tests are the cheapest behavioural sensor; the race detector catches concurrency bugs agents rarely reason about.
- **Sensor:** `make test-go`, `make test-analyzers`, `make test-harness` (Go test cache in `make check`, `-count=1` in `make check-all` and CI: [harness-scope](#harness-scope)); `make test-integration` (`-tags=integration`, in `make check-all`). The CI generator probe runs both on a fresh `make new-domain`, whose scaffold ships a service, handler and integration test (NFR-B1).
- **Remediation:** Read the first failing test above, fix the code (not the test) unless the behaviour change is intended and documented in the PR.
- **Fixture:** `fixtures/go-tests-dependent-package` · Status: seed

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
- **Fixture:** `fixtures/db-schema-doc-stale` · Status: seed

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
- **Fixture:** `fixtures/api-missing-4xx` · Status: seed

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

### api-responses-approved
- **Rule:** A handler test that pins a response with `approvaltest.Verify(t, "<operationId>", rec)` (`apps/api/internal/platform/approvaltest`; `approvaltest.Case("<name>")` tells apart several responses with one status) gets exactly the response in `<package>/testdata/approved/<operationId>.<status>[.<case>].json`: its headers and its JSON body. An intended change is approved with `make approve`, which rewrites the files, and reviewed as their diff. Pinning is opt-in per test (no completeness check yet, [TD-30](../exec-plans/tech-debt-tracker.md)); a file no test verifies any more is not reported ([TD-31](../exec-plans/tech-debt-tracker.md)).
- **Why:** The approved file pins the handler's response, as the test's handler chain produces it (only the middleware the test wraps, [TD-33](../exec-plans/tech-debt-tracker.md)); a refactor that changes a field, a header or an error text without anyone deciding so is the change this catches, and the approved file shows the decision in review (exec plan 0003, P2.4).
- **Sensor:** `go test` (`make test-go`, part of `make check`): `approvaltest.Verify` fails the test with the file name and an approved/received diff, or with the file missing; under `make approve` it fails when one file name gets two different responses in one run.
- **Remediation:** If the change is not intended, fix the handler until the received response equals the approved one; if it is, run `make approve`, review the rewritten `testdata/approved/*.json` and explain the change in the PR; when the shape changed, it is a contract change: update `api/openapi.yaml` first.
- **Fixture:** `fixtures/api-response-not-approved` · Status: team-convention ([ADR 0022](../adr/0022-approved-api-responses.md))

### api-approved-match-contract
- **Rule:** Every approved response (`*/testdata/approved/<operationId>.<status>[.<case>].json`, outside `scripts/harness/fixtures/`) is one `api/openapi.yaml` allows: the operationId exists, it declares the status (exactly, else `<n>XX`, else `default`), the `Content-Type` is one of that response's media types (exactly, else `<type>/*`, else `*/*`) and the body is valid against its schema (JSON Schema 2020-12, formats asserted), and every required header is present and every declared header valid against its schema (an `integer`, `number` or `boolean` header parsed first). Not checked yet: headers the response does not declare, so `getHealth.503.json` carries an `X-Request-Id` the spec declares only on the 200 ([TD-16](../exec-plans/tech-debt-tracker.md)); header values whose schema is an `array` or `object`; operations under a path item that is a `$ref`. At least one approved file must exist (the template ships `getHealth`'s), else the sensor fails to run.
- **Why:** `make approve` records what the code returns; this sensor proves that what was approved is what the contract promises, so an approved file can never pin a response the spec forbids.
- **Sensor:** `.bin/harness approved-check` (`apps/api/tools/harness/approvedcheck.go`; `make approved-check`, part of `make check` and run by `make approve`; skipped while the spec, the approved files and the other harness files are unchanged: [harness-scope](#harness-scope)).
- **Remediation:** Fix the handler so it returns what the spec declares and run `make approve` again, or, if the spec is what is wrong, change `api/openapi.yaml` first (contract first, `make gen`); never hand-edit an approved file to make it pass.
- **Fixture:** `fixtures/api-approved-violates-contract` · Status: team-convention ([ADR 0022](../adr/0022-approved-api-responses.md))

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
- **Sensor:** ESLint, `make lint-web`; fast-check hook per edited file.
- **Remediation:** Fix the reported line; do not add `eslint-disable` (harness-integrity rejects disables without a reason and rule reference).
- **Fixture:** `fixtures/web-lint-dependent-workspace` · Status: seed

### web-typecheck
- **Rule:** All workspace packages type-check under `tsconfig.base.json` (`strict`, `noUncheckedIndexedAccess`).
- **Why:** The type system carries the contract from the OpenAPI spec into components.
- **Sensor:** `tsc --noEmit` per package via Turborepo, `make typecheck-web` (part of `make check`). Not in the fast-check hook: tsc checks a whole package, too slow per edit (NFR-H2).
- **Remediation:** Fix the type error at its source; never widen to `any` or add `@ts-ignore`/`@ts-expect-error`.
- **Fixture:** `fixtures/web-type-error` · Status: seed

### web-tests-pass
- **Rule:** Vitest suites pass.
- **Why:** Cheapest behavioural sensor for the frontend.
- **Sensor:** `vitest run` per package via Turborepo, `make test-web`.
- **Remediation:** Read the first failing test above and fix the code (not the test) unless the behaviour change is intended and documented in the PR.
- **Fixture:** `fixtures/web-failing-test` · Status: seed

### web-no-dead-code
- **Rule:** No unused files, exports or dependencies in the JS workspace.
- **Why:** Agents leave orphans behind; dead code misleads the next agent reading the repo.
- **Sensor:** knip (`knip.json`), `make knip` (check-all, drift.yml).
- **Remediation:** Delete the unused file/export/dependency; if it is used in a way knip cannot see, configure it in `knip.json` and say why in the PR.
- **Fixture:** `fixtures/web-unused-file` · Status: seed

## Generated code

### gen-up-to-date
- **Rule:** Generated code (sqlc stores, oapi-codegen servers, the orval API client, `atlas.sum`) is committed, current, and never hand-edited.
- **Why:** Generated code is a projection of the contract/schema; hand edits are silently lost and stale output hides contract drift.
- **Sensor:** `scripts/harness/gen-check.sh` (`make gen-check`, part of `make check`; skipped while unchanged since its last green run, [harness-scope](#harness-scope)).
- **Remediation:** Change the source (api/openapi.yaml, db/queries/*.sql, db/migrations) instead, run `make gen`, and commit the regenerated files.
- **Fixture:** `fixtures/gen-hand-edit-api-client` · Status: seed

## Shell (scripts/, .claude/hooks/)

### sh-shellcheck
- **Rule:** Every shell script, bats file and git hook (`scripts/git-hooks/`) passes shellcheck (`.shellcheckrc`); a `# shellcheck disable=` needs a reason ([harness-suppression-reason](#harness-suppression-reason)). Vendored `.specify/` (spec-kit, [ADR 0011](../adr/0011-conventional-commits-and-spec-kit.md)) is excluded here and in `sh-format`: it is replaced on upgrade, not edited.
- **Why:** The harness itself is shell: an unquoted variable or a failed `cd` in a sensor or hook silently turns it green or deletes the wrong path.
- **Sensor:** shellcheck (`make lint-sh`, part of `make check`; fast-check hook per file; `pre-commit` git hook on the staged files).
- **Remediation:** Fix the line shellcheck names (the SCxxxx code links to https://www.shellcheck.net/wiki/SCxxxx); quote expansions, add `|| exit` after `cd`, and prefer an explicit `if` to `a && b || c`.
- **Fixture:** `fixtures/sh-unquoted-variable` · Status: seed

### sh-format
- **Rule:** Shell scripts and bats files are `shfmt` formatted (style in `.editorconfig`: 2 spaces, indented `case`).
- **Why:** Formatting noise hides real diffs from reviewers and agents.
- **Sensor:** shfmt (`make fmt-check-sh`, part of `make check`; fast-check hook per file; `pre-commit` git hook on the staged files).
- **Remediation:** Run `make fmt`.
- **Fixture:** `fixtures/sh-unformatted` · Status: seed

### sh-tests-pass
- **Rule:** The bats tests in `scripts/test/` pass; a new script under `scripts/` or `.claude/hooks/` with branching logic comes with a `scripts/test/<script>.bats` (existing gaps: docs/exec-plans/tech-debt-tracker.md).
- **Why:** Sensors and hooks are code that guards all other code: a hook that blocks too much stalls agents, one that blocks too little is a false sense of safety.
- **Sensor:** bats-core (`make test-sh`, part of `make check`). Scripts that need the full toolchain (`selftest.sh`, `gen-check.sh`, `contract-check.sh`, `new-domain.sh` end to end) are exercised by `make harness-selftest` and the CI generator probe instead.
- **Remediation:** Read the first `not ok` test and its `expected output to contain` lines, fix the script (not the test) unless the behaviour change is intended and stated in the PR.
- **Fixture:** none (a failing bats test is its own fixture) · Status: seed

## Secrets and dependencies (Trivy)

Both run the pinned, sha256-verified Trivy release binary from `.bin/` (never its Docker image or GitHub
Action: CVE-2026-33634, exec plan 0003 Q1) through `scripts/harness/trivy.sh`, on a temp copy of the git-known
files (tracked plus untracked, not ignored: never `.env.local` or `.cache/`). The secret scan skips nothing, so
it sees `scripts/harness/fixtures/` too: the secret fixtures assemble their fake key in `apply.sh`, so no
key-shaped string is in the tree. Only the vuln scan skips that directory (its overlays are not installed
dependencies). No finding can be waived outside a reviewed harness change: no `.trivyignore` is loaded and
`TRIVY_*` environment variables are unset before the scan. Trivy's findings exit 3 and `trivy.sh` maps them to 1;
any other Trivy exit is a failure to run (exit 2, `trivy failed to run (exit N) [<rule>]`), never a finding.

### repo-no-secrets
- **Rule:** No secret (cloud keys, tokens, private keys: Trivy's built-in secret rules) in any file git knows, committed or not.
- **Why:** Agents copy credentials from logs, env files and docs into code and fixtures; once pushed, a secret must be rotated, not just deleted.
- **Sensor:** `trivy fs --scanners secret` offline (no DB, check, VEX or version download, no telemetry; `make secrets`, part of `make check`, and CI through `make check-all`). In `make check` it scans only the files changed since the merge base, and everything when a harness file changed ([harness-scope](#harness-scope)); budget 5 s on the full tree (`budgets.txt`, NFR-H9). Trivy's built-in rules only, with its built-in path allow-list (test files, examples, vendor dirs, every `*.md`) switched off by the protected `scripts/harness/trivy-secret.yaml` (the `repo-secret-in-markdown` and `repo-secret-in-go-test` fixtures prove it against the real binary); `scripts/test/trivy.bats` pins the flags. The `pre-commit` git hook (`scripts/harness/pre-commit.sh`, installed by `make setup`) runs the same scan on the staged files as they are in the index (`trivy.sh secret --staged`), before the commit exists; `git commit --no-verify` skips it, CI does not.
- **Remediation:** Remove the secret from the file named (read it from the environment or a gitignored `.env.local` instead); if it was ever committed or pushed, rotate it, since history keeps it.
- **Fixture:** `fixtures/repo-secret-planted`, `fixtures/repo-secret-in-markdown`, `fixtures/repo-secret-in-go-test`; `scripts/test/pre-commit.bats` · Status: team-convention ([ADR 0015](../adr/0015-anticipated-rules-and-nfr-classes-b-o.md))

### deps-no-known-vulns
- **Rule:** No known vulnerability in the JS dependencies of `pnpm-lock.yaml` (dev dependencies included) or in the modules of any `go.mod` (`apps/api` and the nested `apps/api/tools/*` modules [go-no-known-vulns](#go-no-known-vulns) does not reach).
- **Why:** Agents add dependencies freely; the JS toolchain and the harness's own Go modules run on every dev machine and in CI. Trivy reads `go.mod` without call graphs, so it is broader and less precise than `govulncheck`: both run.
- **Sensor:** `trivy fs --scanners vuln --include-dev-deps` against the Trivy DB, fetched into `.cache/trivy` from `ghcr.io/aquasecurity/trivy-db:2` (not the default first choice, the Docker Hub mirror `mirror.gcr.io`) (`make vuln-deps`, in `make check-all`, CI with the DB cached by date, and drift.yml). Fails closed on a `pnpm-lock.yaml` with a non-empty `packages:` section or a `go.mod` with a `require` directive that it found no packages in (an unsupported lockfile format would otherwise pass as clean); a `go.mod` without requires has nothing to scan and passes.
- **Remediation:** Upgrade the package named in the finding to its fixed version (`pnpm update <pkg>`; for a transitive one an `overrides` entry in pnpm-workspace.yaml (protected: a harness PR) scoped to the chain, e.g. `"a>b": x.y.z`, with the dependency chain and when to drop it; `go get <module>@<fixed-version>` then `go mod tidy` in the module of the named `go.mod`).
- **Fixture:** none (external advisory DB) · Status: team-convention ([ADR 0015](../adr/0015-anticipated-rules-and-nfr-classes-b-o.md))

## Harness integrity and CI (.github/)

### harness-suppression-reason
- **Rule:** Every added suppression (`//nolint`, `//lint:ignore`, `//lint:file-ignore`, `eslint-disable`, `/* eslint … */`, `@ts-expect-error`, `# shellcheck disable`, `-- atlas:nolint`, `-- squawk-ignore`) has `reason: … (rule: <id>)` on the same line, where `<id>` is a `### <id>` heading in this file; `@ts-ignore` and `@ts-nocheck` are never allowed.
- **Why:** A bare suppression silently turns a sensor off; the reason and rule id make each exception reviewable and greppable.
- **Sensor:** `.bin/harness integrity-check` (`apps/api/tools/harness`) on lines added since the merge base with `BASE_REF` in Go, TS/JS, shell, SQL, git hook and workflow files, not vendored `.specify/` (`make harness-integrity`, in `make check-all` and `harness-integrity.yml`, which builds the binary from the base branch; the PR's `.gitattributes` cannot hide a hunk).
- **Remediation:** Fix the code instead; if the suppression is truly needed, append `reason: <why it is safe> (rule: <rule-id>)` on the same line with an id from this file as it is on the base branch, and replace `@ts-ignore`/`@ts-nocheck` with a fixed type or a reasoned `@ts-expect-error`.
- **Fixture:** `fixtures/harness-suppression-no-reason` · Status: seed

### harness-config-not-loosened
- **Rule:** No non-comment line of a harness config (`config` entries in `scripts/harness/protected-paths.txt`: linter configs, Makefile, workflows, CODEOWNERS, `.gitattributes`, tool versions, the protected-path list itself, and sensor code: `scripts/harness/`, custom analyzers, Claude hooks, the tool installer) is added or removed, and no such file changes its mode, unless a human approves (a shebang is not a comment). Tightening counts too: telling it from loosening takes judgment. Only `#` and blank lines count as comments (`//` and `/* */` can comment code out).
- **Why:** The cheapest way to make a red sensor green is to weaken its config; that must be a visible human decision, not an agent's shortcut.
- **Sensor:** `.bin/harness integrity-check` on the diff since the merge base with `BASE_REF` (`make harness-integrity`, `harness-integrity.yml`; CI runs the base branch's workflow, binary (built from the base checkout) and path list via `pull_request_target` ([ADR 0010](../adr/0010-base-branch-gates.md)), so a PR cannot unprotect itself). Waived only by `HARNESS_CHANGE_APPROVED=1`, set in CI by the `harness-change-approved` PR label.
- **Remediation:** Revert the config change and fix the code; if the rule itself is wrong, explain why in the PR description (Harness change: Y + rationale) and ask a code owner to add the `harness-change-approved` label.
- **Fixture:** `fixtures/harness-config-loosened` · Status: seed

### harness-files-protected
- **Rule:** Agents do not edit harness files (`scripts/harness/protected-paths.txt`); they propose the change in the PR description. A human allows edits by starting the agent session with `HARNESS_EDIT=1`.
- **Why:** A sensor the agent can edit is a suggestion: the cheapest fix for a red check is to change the check.
- **Sensor:** PreToolUse hook `.claude/hooks/protect-harness.sh` (file tools exactly; shell writes heuristically) for Claude Code (like every hook, it parses its JSON input with `.bin/harness hook-input`, [ADR 0012](../adr/0012-parsing-sensors-in-go.md), and without the binary blocks everything but `make setup`); `.github/CODEOWNERS` review and `harness-config-not-loosened` for every agent.
- **Remediation:** Do not edit the file: fix the code the sensor points at, or describe the harness change (file, diff, why) in the PR description under "Harness change".
- **Fixture:** none (a hook, not a make target); `scripts/test/protect-harness.bats` · Status: seed

### harness-latency-budget
- **Rule:** Every sensor and hook finishes within its budget in `scripts/harness/budgets.txt` (`default` row: 5 s, NFR-H5; PreToolUse hooks 50 ms, NFR-H1; `fast-check` 1 s, NFR-H2; the `pre-commit` git hook 2 s, NFR-H8); the budgets are the latency NFRs in [nfr.md](nfr.md) ([ADR 0013](../adr/0013-harness-nfr-budgets.md)).
- **Why:** Agents run `make check` after every change, so harness latency is agent throughput; it degrades a little per PR, and no single review notices.
- **Sensor:** `scripts/harness/timing.sh`, used by `scripts/harness/run.sh`, every hook in `.claude/hooks/` and the `pre-commit` git hook, times each run, appends it to `.cache/harness/timings.jsonl` and prints `harness: sensor '<id>' took <t> s (budget <b> s, <nfr>)` (or `hook '<name>'`) when over budget. In GitHub Actions the line is also a `::warning` annotation and a job-summary entry, and `ci.yml` uploads `timings.jsonl` as the `harness-timings-check-all` artifact. Nightly, `harness-nfr.yml` computes each name's p95 over the last 7 days of those artifacts (`scripts/harness/nfr-report.sh`) and opens or comments on one `harness-nfr` issue for every p95 over budget. Hooks run in CI only inside bats tests, so their rows there do not measure real tool calls: NFR-H1 and NFR-H2 have the local warning and no trend yet. A warning only: the sensor or hook keeps its own exit status.
- **Remediation:** Make the sensor or hook do less (scope it to the change, use the tool's cache, skip it when its inputs did not change: exec plan 0002 phases 1–2); if the budget itself is wrong, propose a new row in `budgets.txt` in the PR description (Harness change: Y).
- **Fixture:** none (warns, never fails; the selftest asserts a failure); `scripts/test/run.bats`, `scripts/test/timing.bats`, `scripts/test/nfr-report.bats` · Status: seed

### harness-deterministic
- **Rule:** The same tree gives the same verdict, the same findings and the same tree after `make check`, run after run (NFR-R1, [ADR 0013](../adr/0013-harness-nfr-budgets.md)).
- **Why:** A sensor that flips between runs teaches agents to rerun until green, and then every rule is only as strong as its flakiest sensor.
- **Sensor:** `scripts/harness/determinism.sh` (`make harness-determinism`) runs `make check` twice in scope `full` (in `changed` scope the [gen-check stamp](#harness-scope) makes the second run skip by design) and compares exit status, findings (sorted output lines, durations and temp paths masked) and `git status`. Nightly on `main` in `harness-nfr.yml`, which fails and opens or comments on the `harness-nfr` issue.
- **Remediation:** Remove the state the sensor depends on besides the tree: caches keyed on time or not on inputs, network calls, unseeded randomness, test order, shared temp files, tests that write into the tree. Make output order-stable (sort it). Never retry a sensor until it passes.
- **Fixture:** `fixtures/harness-nondeterministic`; `scripts/test/determinism.bats` · Status: seed

### harness-scope
- **Rule:** `make check` (scope `changed`) may skip a sensor only when none of its inputs and no harness file (`protected-paths.txt`) changed since the merge base, counting staged, unstaged and untracked files; `make check-all` and CI (scope `full`, `HARNESS_SCOPE=full`) run every sensor. Skipped today: `lint-sh`, `fmt-check-sh`, `test-sh` (inputs: `*.sh`, `*.bash`, `*.bats`, `scripts/**`), the main cost of `make check` (NFR-H3, [ADR 0013](../adr/0013-harness-nfr-budgets.md), [ADR 0014](../adr/0014-make-check-scoped-to-the-change.md)); `fixture-claims` (inputs: `docs/harness/rules.md`, `scripts/harness/fixtures/**`); `approved-check` (inputs: `api/openapi.yaml`, `*/testdata/approved/*`); `gen-check` while its inputs and the generated files hash to the stamp of its last green run. Scoped to the affected set in scope `changed`: `lint-go` and `test-go` to the changed Go packages of `apps/api` plus every package that imports them (tests included), `lint-web` and `arch-web` to the changed JS workspaces plus their dependents; skipped when that set is empty; `secrets` ([repo-no-secrets](#repo-no-secrets)) to the changed and untracked files, all git-known files when a harness file changed. `typecheck-web` and `test-web` are not scoped: Turborepo's task cache already replays unchanged packages. `arch-go` is not scoped (go-arch-lint checks the whole module). Cached only in scope `changed`: Go test results (`test-go`, `test-analyzers`, `test-harness` without `-count=1`; integration tests always `-count=1`) the dependency-cruiser graph (`--cache node_modules/.cache/depcruise`) and Turborepo's task cache (`typecheck-web`, `test-web`; `--force` in any other scope, so `make harness-determinism` really runs vitest and tsc twice).
- **Why:** Agents run `make check` after every change; a sensor whose inputs did not change can only repeat its last verdict, so running it costs time and finds nothing.
- **Sensor:** `scripts/harness/unchanged.sh` decides (`gen-check.sh --unchanged` for `gen-check`), `scripts/harness/affected.sh go|web` the affected set (`$(call scoped,…)` in the Makefile); the target prints `<target>: skipped, …` instead of running. It fails closed: no merge base (`BASE_REF`), a git error, a missing or unmatched stamp, or any scope but `changed` means run and uncached; the affected set is everything on a harness file, `go.mod`, `go.sum`, a `package.json`, the lockfile or workspace config, a non-Go file under `apps/api`, a `.go` file in no package, a JS/TS/JSON file outside a workspace, or a failing `go list` or `turbo ls`. Timing rows (`harness-latency-budget`) carry the scope.
- **Remediation:** A skipped sensor that should have run means its trigger pathspecs miss an input: add the input to the `$(call sh_sensor,…)` pathspecs in the Makefile, to the `unchanged.sh` pathspecs of its `fixture-claims` or `approved-check` recipe, to `INPUTS` in `gen-check.sh`, or to what makes `affected.sh` return everything (Harness change: Y); to run everything now, `make check-all` or `HARNESS_SCOPE=full make check`.
- **Fixture:** none (decides whether a sensor runs, never fails); `scripts/test/unchanged.bats`, `scripts/test/gen-check.bats`, `scripts/test/affected.bats`, `scripts/test/trivy.bats`; the scoped sensors' fixtures `go-tests-dependent-package`, `web-lint-dependent-workspace` run in scope `changed` (`SCOPE=changed` in `fixture.conf`, also in CI) · Status: seed

### ci-workflows-valid
- **Rule:** GitHub workflows and composite actions pass actionlint, including shellcheck on every `run:` block.
- **Why:** A broken workflow fails only after push, or worse, never runs the gate it was meant to run.
- **Sensor:** actionlint (`make lint-ci`, part of `make check`).
- **Remediation:** Fix the line actionlint names (unknown context/expression, bad `needs`/`if`, or the shellcheck code in a `run:` block); pin new actions to a full commit SHA with a `# vX.Y.Z` comment.
- **Fixture:** `fixtures/ci-invalid-workflow` · Status: seed

## Commits and specs (git history, specs/)

Team conventions, not earned from an incident: the team already writes Conventional Commits and specifies
product features with [spec-kit](https://github.com/github/spec-kit); PR #1 adopted them as rules, a
deliberate exception recorded in [ADR 0011](../adr/0011-conventional-commits-and-spec-kit.md).
In CI these run in the `pr-conventions` job of `harness-integrity.yml` (base branch's scripts, PR title as data).

### git-conventional-commits
- **Rule:** Every non-merge commit subject on a branch, and the PR title, is `<type>(<scope>)!: <description>` (scope and `!` optional; scope lowercase `[a-z0-9._/-]`) with type one of build, chore, ci, docs, feat, fix, perf, refactor, revert, style, test. A subject is at most 100 characters, the PR title at most 90.
- **Why:** The subject is the changelog and the release signal (`feat` = minor, `!` = major); a squash merge of several commits makes the PR title plus ` (#N)` the commit on `main`, so the title is checked too, with room for the suffix.
- **Sensor:** `scripts/harness/check-commits.sh` on `BASE_REF..HEAD` and `PR_TITLE` (`make commits`, in `make check-all` locally); the `commit-msg` git hook (`scripts/git-hooks/`, installed by `make setup`) rejects a bad message before the commit exists.
- **Remediation:** Reword the subject as `<type>(<scope>): <imperative description>`, e.g. `feat(billing): add invoice export`: the last commit with `git commit --amend`, older ones with `git rebase -i` and `reword`, the PR title in the PR.
- **Fixture:** `fixtures/git-commit-not-conventional` · Status: team-convention (PR #1)

### specs-structure
- **Rule:** Every `specs/<dir>/` is a directory (not a symlink) named `NNN-kebab-slug` with a unique `NNN`, contains `spec.md`, contains `plan.md` once it has `tasks.md`, no Markdown file in it has an open clarification (a `[NEEDS CLARIFICATION: …]` marker, or a plan field left as `: NEEDS CLARIFICATION`; `checklists/` is skipped because it names the marker), and no file in it is an OpenAPI document (`contracts/` describes the change to `api/openapi.yaml` in prose).
- **Why:** spec-kit's commands find a feature by its numbered directory and build plan → tasks on spec; an open clarification merged into `main` is a decision nobody made; a second OpenAPI document escapes Spectral, oasdiff and `make gen` and drifts from the one contract.
- **Sensor:** `scripts/harness/check-specs.sh` (`make specs`, in `make check-all`).
- **Remediation:** Create spec directories with `/speckit-specify` (never by hand), run `/speckit-plan` before `/speckit-tasks`, resolve every `NEEDS CLARIFICATION` with `/speckit-clarify` or by writing the decision into the spec, and put contract changes in `api/openapi.yaml` (the spec's `contracts/` only lists the operations and schemas).
- **Fixture:** `fixtures/specs-open-clarification` · Status: team-convention (PR #1)

### specs-required-for-features
- **Rule:** A `feat` commit (or a `feat` PR title) that changes `apps/` or `api/`, other than harness files in `scripts/harness/protected-paths.txt` (analyzers, linter configs), comes with an added or changed (not only deleted) `specs/<NNN-slug>/` committed in the same change. Untracked files do not count: CI sees only commits. Harness and infrastructure work is planned in `docs/exec-plans/` instead.
- **Why:** A product feature without a spec has requirements only in a chat log; the spec is what reviewers and the next agent check the code against.
- **Sensor:** `scripts/harness/check-specs.sh` against the merge base with `BASE_REF` (`make specs`, `make check-all`). Waived only by `NO_SPEC_NEEDED=1`, set in CI by the `no-spec-needed` PR label (a human decision).
- **Remediation:** Write the spec with `/speckit-specify` (then `/speckit-plan`, `/speckit-tasks`) and commit `specs/<NNN-slug>/` with the feature; if the change is not a product feature, use another commit type, or ask a reviewer for the `no-spec-needed` label.
- **Fixture:** `fixtures/specs-feat-without-spec` · Status: team-convention (PR #1)

## Runtime (make logs, traces, metrics)

### obs-request-queryable
- **Rule:** One request to the running API can be read back through `make` alone (NFR-O1): its access line by `make logs REQUEST_ID=<id>`, its server span by `make traces ID=<trace_id>` from that line, and the p95 latency of its route from that API process by `make metrics`.
- **Why:** An agent that cannot read its own app's runtime guesses; `scripts/obs-query.sh` is otherwise tested only against canned Loki, Tempo and Prometheus responses (`scripts/test/obs-query.bats`), so a changed `OBS_IMAGE` would show up as empty or wrong output instead of a failed check (exec plan 0003, P1.4).
- **Sensor:** `scripts/harness/obs-e2e.sh`, `make test-obs` (in `make check-all` and CI; needs Docker). It first pulls `OBS_IMAGE` if missing (sensor `obs-pull`), then (sensor `obs-e2e`) starts Postgres, builds the API, runs it on a free port with its exporters on this worktree's obs stack and `service.instance.id` set to the request ID, sends one `GET /healthz` with that `X-Request-Id`, stops it (flushing its telemetry) and polls the three targets for up to 60 s each; the p95 query selects the Prometheus label `instance` (from `service.instance.id`), so an attached stack's other data cannot satisfy it. It uses an obs stack that is already up and leaves it; one it started, it removes. Without Docker, the image or this worktree's obs ports it fails, never skips.
- **Remediation:** Fix what the output names: start Docker, or run `make setup` for missing ports; for a missing line, span or series run `make obs-up` and `make dev`, send a request and read it back with `make logs REQUEST_ID=<id>` to find which part changed (the API's telemetry or access log, or `scripts/obs-query.sh` after an `OBS_IMAGE` bump).
- **Fixture:** none (a positive end-to-end test against Docker and the 2.58 GB obs image, while selftest asserts that a sensor rejects a violation; `scripts/test/obs-e2e.bats` covers its fail-closed and stack-cleanup paths) · Status: team-convention ([ADR 0017](../adr/0017-per-worktree-observability-stack.md), NFR-O1)

## Docs

### docs-links-resolve
- **Rule:** Every relative link and `#anchor` in tracked Markdown files resolves.
- **Why:** Guides are the agent's map; a dead link silently sends it searching (or guessing) instead of reading.
- **Sensor:** `.bin/harness check-links` (`apps/api/tools/harness/checklinks.go`; `make docs-links`, part of `make check`; fast-check hook per Markdown file; `pre-commit` git hook on the staged tree when a Markdown file is staged).
- **Remediation:** Point the link at the file's current path (or heading slug), or remove it if the target is gone; when you move a doc, update every link to it in the same change.
- **Fixture:** `fixtures/docs-broken-link` · Status: seed

### docs-claude-imports-agents
- **Rule:** Every `AGENTS.md` has a `CLAUDE.md` in the same directory with the line `@AGENTS.md` (plus any Claude-Code-only notes).
- **Why:** Claude Code loads `CLAUDE.md`, not `AGENTS.md`; without the import an area guide is invisible to it, while other agents read `AGENTS.md` directly. One source, two entry points.
- **Sensor:** `.bin/harness check-links` (`apps/api/tools/harness/checklinks.go`; `make docs-links`, part of `make check`; fast-check hook per Markdown file; `pre-commit` git hook on the staged tree when a Markdown file is staged).
- **Remediation:** Create `CLAUDE.md` next to the `AGENTS.md` the message names, containing the line `@AGENTS.md`; put shared guidance in `AGENTS.md`, never only in `CLAUDE.md`.
- **Fixture:** `fixtures/docs-agents-without-claude` · Status: seed

### rules-fixture-claims
- **Rule:** Every `### <rule-id>` in this file is unique (one heading per id) and has one `**Fixture:**` line, either `none (<reason>)` (free text, may name other rules' fixtures and tests) or one or more `` `fixtures/<dir>` `` joined by `, `, optionally followed by `; ` and backticked test paths. Each named dir is a git-known `scripts/harness/fixtures/<dir>/fixture.conf` whose `RULE=` is that rule's id; each test path matches a git-known file.
- **Why:** The selftest ([ADR 0009](../adr/0009-harness-selftest.md)) proves only the fixtures that exist; a line claiming coverage that is not there makes this guide promise a tested sensor that nothing tests (exec plan 0003 found seven such lines).
- **Sensor:** `.bin/harness fixture-claims` (`apps/api/tools/harness/fixtureclaims.go`; `make fixture-claims`, part of `make check`, skipped while rules.md, the fixtures and the other harness files are unchanged: [harness-scope](#harness-scope)).
- **Remediation:** Make the line say what is true: name each fixture as `` `fixtures/<dir>` `` (the directory under `scripts/harness/fixtures/` whose `fixture.conf` has `RULE=` this rule), add a fixture if the sensor is deterministic and local, or write `none (<why this rule has no fixture>)`; never claim another rule's fixture as coverage; merge a duplicated `### <id>` into one section or rename one of them.
- **Fixture:** `fixtures/rules-bogus-fixture-claim` · Status: team-convention ([ADR 0015](../adr/0015-anticipated-rules-and-nfr-classes-b-o.md))
