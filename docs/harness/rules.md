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
