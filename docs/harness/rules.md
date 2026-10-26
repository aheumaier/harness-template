# Harness rules

Every architectural rule exists twice: as a pointer an agent can read (here, linked from AGENTS.md) and as a
deterministic sensor that fails with the remediation below. `scripts/harness/run.sh` prints the
**Remediation** line of a rule verbatim when its sensor fails, so keep it one actionable sentence.

Status: **seed** = created with the template. New rules must be earned (link the incident/PR that motivated
them) and need a fixture in `scripts/harness/fixtures/` before they merge.

Format per rule: rule · why · sensor · remediation · fixture.

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
- **Sensor:** `atlas.sum` integrity via `make gen-check` (hash changes) and the modified-file check in `migration-gate.yml`.
- **Remediation:** Revert your edit to the existing migration and create a new one with `atlas migrate new <name> --env local` (see .claude/skills/add-migration).
- **Fixture:** none (git-history based; exercised in migration-gate) · Status: seed

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
- **Rule:** No breaking change to the contract compared to the merge base with `main` unless a CODEOWNER approves.
- **Why:** Deployed clients cannot be changed atomically with the server.
- **Sensor:** oasdiff `breaking --fail-on ERR` (`scripts/harness/contract-check.sh`), `make contract-check`, required check `contract-gate` (`.github/workflows/contract-gate.yml`).
- **Remediation:** Make the change additive (new optional field, new path, deprecate before removing); if the break is intended, explain it in the PR and get CODEOWNERS approval on contract-gate.
- **Fixture:** `fixtures/api-breaking-change` · Status: seed

## Generated code

### gen-up-to-date
- **Rule:** Generated code (sqlc stores, oapi-codegen servers, the orval API client, `atlas.sum`) is committed, current, and never hand-edited.
- **Why:** Generated code is a projection of the contract/schema; hand edits are silently lost and stale output hides contract drift.
- **Sensor:** `scripts/harness/gen-check.sh` (`make gen-check`, part of `make check`).
- **Remediation:** Change the source (api/openapi.yaml, db/queries/*.sql, db/migrations) instead, run `make gen`, and commit the regenerated files.
- **Fixture:** `fixtures/gen-hand-edit-api-client` · Status: seed
