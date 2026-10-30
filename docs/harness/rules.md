# Harness rules

Every architectural rule exists twice: as a pointer an agent can read (here, linked from AGENTS.md) and as a
deterministic sensor that fails with the remediation below. `scripts/harness/run.sh` prints the
**Remediation** line of a rule verbatim when its sensor fails, so keep it one actionable sentence.

Status: **seed** = created with the template; **team-convention** = a convention the team already follows,
adopted as a rule by a recorded decision (ADR) instead of an incident; **earned** = promoted from an inferential
sensor's finding class accepted twice ([ADR 0019](../adr/0019-inferential-sensors-are-advisory.md)), written
`Status: earned (<finding 1>, <finding 2>)` with both findings linked. New rules must be earned (link the
incident/PR that motivated them) and need a fixture in `scripts/harness/fixtures/` before they merge. They
also leave: the weekly drift issue lists rules that never fired in 90 days ([ci.md](ci.md#rule-fires)), and a
human decides to keep each or retire it in a harness PR (entry, sensor, fixture;
[ADR 0032](../adr/0032-rules-that-never-fire-are-a-drift-report.md)).

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
- **Rule:** `<layer dir>/<a>/**` must not import `<layer dir>/<b>/store` or `<layer dir>/<b>/api`, where `<layer dir>` is `LAYER_GLOB` in `scripts/harness/layout.env` without its `/*/*` (template: `internal`).
- **Why:** A domain's storage and HTTP surface are private; coupling to them makes domains impossible to split or change independently.
- **Sensor:** custom analyzer `domainiso` (`apps/api/tools/analyzers/domainiso`) via golangci-lint, `make lint-go`. It reads only `LAYER_GLOB` and fails closed: without `layout.env`, without `LAYER_GLOB` or with a glob that is not `<dir>[/<dir>…]/*/*` it reports an error instead of passing ([harness-layout-valid](#harness-layout-valid)).
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
- **Sensor:** golangci-lint `forbidigo` (by package path, `analyze-types`) and `sloglint` (`no-mixed-args`; no key-case check, naming style stays out: [ADR 0024](../adr/0024-size-and-complexity-limits-are-maintainability-sensors.md)), `make lint-go` (the per-edit hook runs the same linters but does not list this rule id yet, so it prints the other `lint-go` rules' remediations instead of this one). sloglint's message cannot carry the rule id, so `run.sh` maps a line ending in `(sloglint)` to this rule (`finding_pattern`) and prints this remediation, also next to another rule's finding.
- **Remediation:** Log through the injected `*slog.Logger` (built by `logging.New` in platform) with either key-value pairs or `slog.Attr` values in one call, not both, or return the error to the caller instead of printing it.
- **Fixture:** `fixtures/go-structured-logging-fmt-print`, `fixtures/go-structured-logging-slog-mixed-args` · Status: team-convention ([ADR 0015](../adr/0015-anticipated-rules-and-nfr-classes-b-o.md))

### go-sec
- **Rule:** No gosec finding in the API module (SAST: file paths built from variables, some subprocess calls built from variables, weak crypto or randomness, unchecked integer conversions, hard-coded credentials, servers without timeouts, …), tests included, since a test can still leak a credential or shell out unsafely (plan 0003 Q6). Generated code is excluded. Suppress a false positive on its line only, with `//nolint:gosec // reason: … (rule: go-sec)`; never disable a gosec check globally. gosec does not catch every subprocess call built from variables: G204 in the pinned gosec does not fire on `exec.Command(name)` with `name` a function parameter (exec plan 0003, A13), so review owns every `exec.Command`. Typical fixes: `os.Root` or a fixed root plus `filepath.Clean` for paths, a fixed argv for `exec.Command`, `crypto/rand`, bounds-checked conversions, credentials from config, `ReadHeaderTimeout` on servers.
- **Why:** Agents write code faster than humans review it; a computational SAST sensor catches the common Go security mistakes before review does (Böckeler, SAST as a computational sensor).
- **Sensor:** golangci-lint `gosec` with all checks at its defaults, `make lint-go` (the per-edit hook runs the same linters but does not list this rule id yet, so it prints the other `lint-go` rules' remediations instead of this one). Semgrep stays out until a domain needs what gosec cannot cover. gosec's message (`G304: … (gosec)`) cannot carry the rule id, so `run.sh` maps a line ending in `(gosec)` to this rule (`finding_pattern`) and prints this remediation, also next to another rule's finding. The separate modules `tools/analyzers` and `tools/harness` are not linted by `lint-go`.
- **Remediation:** Fix what the finding's `G<nnn>` id names (see the gosec rule list); for a false positive add `//nolint:gosec // reason: … (rule: go-sec)` on that line only.
- **Fixture:** `fixtures/go-sec-file-inclusion` · Status: team-convention ([ADR 0015](../adr/0015-anticipated-rules-and-nfr-classes-b-o.md))

### go-format
- **Rule:** Go code is `gofmt`/`goimports` formatted (local prefix `github.com/aheumaier/harness-template`).
- **Why:** Formatting noise hides real diffs from reviewers and agents.
- **Sensor:** golangci-lint formatters, `make lint-go` (`run.sh` maps a line ending in `(gofmt)` or `(goimports)` to this rule); the `pre-commit` git hook (`scripts/harness/pre-commit.sh`) on the staged Go files (`golangci-lint fmt --diff`).
- **Remediation:** Run `make fmt`.
- **Fixture:** none (formatter, auto-fixable) · Status: seed

### go-tests-pass
- **Rule:** `go test -race` passes for the app; `go test` for the analyzers and the hook parser (`apps/api/tools/harness`).
- **Why:** Tests are the cheapest behavioural sensor; the race detector catches concurrency bugs agents rarely reason about.
- **Sensor:** `make test-go`, `make test-analyzers`, `make test-harness` (Go test cache in `make check`, `-count=1` in `make check-all` and CI: [harness-scope](#harness-scope)); `make test-integration` (`-tags=integration`, in `make check-all`). The CI generator probe runs `make check` and `make test-scaffold` on a fresh `make new-domain`, whose scaffold ships a service, handler and integration test (NFR-B1): `go test -json` over the new domain, red unless every `Test*` of the scaffold templates ran and passed, none skipped (`scripts/harness/scaffold-tests.sh`).
- **Remediation:** Read the first failing test above, fix the code (not the test) unless the behaviour change is intended and documented in the PR.
- **Fixture:** `fixtures/go-tests-dependent-package`, `fixtures/go-tests-root-module` · Status: seed

### go-no-known-vulns
- **Rule:** No reachable known vulnerabilities in Go dependencies or the standard library.
- **Why:** Agents add dependencies freely; reachability analysis keeps the signal high.
- **Sensor:** `govulncheck`, `make vuln-go` (check-all, drift.yml).
- **Remediation:** Upgrade the module named in the finding (`go get <module>@<fixed-version>` in apps/api, then `go mod tidy`), or bump `GO_TOOLCHAIN` for stdlib findings.
- **Fixture:** none (depends on external advisory DB) · Status: seed

### go-no-dead-code
- **Rule:** No Go function is unreachable from `main` or the tests.
- **Why:** Agents leave orphans behind; dead code misleads the next agent reading the repo.
- **Sensor:** `deadcode -test` (golang.org/x/tools) on `apps/api` and `apps/api/tools/harness`, `make deadcode-go` (in `make check-all` since [ADR 0031](../adr/0031-no-orphan-scripts-or-make-targets.md), and drift.yml weekly). `apps/api/tools/analyzers` is left out: deadcode cannot see golangci-lint's module-plugin entry points ([TD-70](../exec-plans/tech-debt-tracker.md)).
- **Remediation:** Delete the unreachable function, or call it from the code path (or test) that should use it.
- **Fixture:** `fixtures/go-dead-function` · Status: seed

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
- **Sensor:** oasdiff `breaking --fail-on ERR` (`scripts/harness/contract-check.sh`, after `.bin/harness spec-refs` finds every `$ref` internal), `make contract-check`, required check `api-no-breaking-change` (`.github/workflows/contract-gate.yml`). Waived only by `BREAKING_CHANGE_APPROVED=1`, set in CI by the `breaking-change-approved` PR label. A break also prints `CONTRACT_CONSUMERS` from `scripts/harness/layout.env` (in CI the base branch's), the repos that vendor this contract: it names the repos the approval must cover ([ADR 0037](../adr/0037-upstream-spec-drift-is-a-drift-report.md)).
- **Remediation:** Make the change additive (new optional field, new path, deprecate before removing); if the break is intended, explain it and the client migration in the PR, for this repo's clients and each repo the sensor names from `CONTRACT_CONSUMERS` in `scripts/harness/layout.env`, and ask a code owner to add the `breaking-change-approved` label. Agents never add the label. For an external `$ref`, inline the referenced schema under `components` and point at it with `#/components/...`.
- **Fixture:** `fixtures/api-breaking-change`; `scripts/test/contract-check.bats` · Status: seed

### api-upstream-no-breaking-change
- **Rule:** Each upstream spec this repo vendors is listed in `UPSTREAM_SPECS` in `scripts/harness/layout.env` as `<vendored spec>@<source repo>@<spec path in the source>@<revision>` (the source a host path such as `github.com/acme/rooms`, fetched over https, or an absolute local repo path, for tests and fixtures only; the revision the commit the copy was taken at, for people: no check reads it), and the spec at its source's head has no breaking change against the vendored copy. Drift is resolved in one PR: the client code follows the upstream change, and the copy and its revision are refreshed together.
- **Why:** A vendored spec says what the upstream promised at the revision it was taken; the upstream ships breaking changes without a PR here, and code generated from the copy keeps compiling until calls fail in production (exec plan 0003, G14; [ADR 0037](../adr/0037-upstream-spec-drift-is-a-drift-report.md)).
- **Sensor:** `scripts/harness/upstream-drift.sh`, `make upstream-drift` (drift.yml, weekly, findings in the drift issue; reads the network, so in neither `make check` nor `check-all`). An empty `UPSTREAM_SPECS` exits 0 without fetching. Each distinct source is fetched once, shallow, with no prompt (anonymous in CI, your credential helpers locally) and sync-check.sh's time limits, over https or from a local path only (`GIT_ALLOW_PROTOCOL=file:https`), into a temp bare repo; `.bin/harness spec-refs` parses both specs and refuses any `$ref` that is not internal (`#/...`), however the YAML or JSON spells it; oasdiff `breaking --fail-on ERR` compares the vendored copy (base) with the head's spec (revision) and prints `<vendored spec>: <n> breaking change(s) in <source>/<path> at <head> since vendored <revision>: … [api-upstream-no-breaking-change]` with oasdiff's findings, or `<vendored spec>: <path> is gone from <source> at <head> …`. Script exit 0: no drift; 1: drift; 2: cannot check an entry, failing closed after checking the others (a malformed entry, a vendored path that is absolute or has `..`, a missing vendored copy, a revision that is not hex, a source that starts with `-`, has `..` or is neither a host path nor an absolute path, a spec path that is absolute or has a `.`, `..` or empty segment, a failed or timed-out fetch, an external `$ref` in either spec, a spec that does not parse or oasdiff cannot load, any unexpected error). make reports 1 and 2 alike as its exit 2. Without the network, `make layout-check` ([harness-layout-valid](#harness-layout-valid)) refuses the same entries before any fetch; `apps/api/tools/harness/testdata/upstream-spec-vectors.txt` holds both to one rule. Blind spots: an absolute-path source is for tests and fixtures only (CI has no such repo, so a committed one fails the weekly run with exit 2); the revision is not compared with the vendored copy, which is not protected (a copy edited to hide drift changes the generated client, so the code that calls it stops building).
- **Remediation:** For each drifted spec, adapt this repo's code to oasdiff's findings and re-vendor the source's spec at the printed head, writing that head as the entry's revision in `scripts/harness/layout.env` in the same PR; on exit 2, fix the named entry, or, for a private source the anonymous drift fetch cannot read, run `make upstream-drift` locally, where your git credential helpers apply. If you reach the host only over ssh: a `url.<ssh-url>.insteadOf https://<host>/` mapping (e.g. `url.git@github.com:.insteadOf https://github.com/`) does not help here, because the fetch allows only `file:https` and git refuses the rewritten URL ("transport 'ssh' not allowed"); set up an https credential helper for the host instead (for GitHub, `gh auth setup-git`).
- **Fixture:** `fixtures/api-upstream-breaking`; `scripts/test/upstream-drift.bats` · Status: team-convention ([ADR 0037](../adr/0037-upstream-spec-drift-is-a-drift-report.md))

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
- **Sensor:** ESLint, `make lint-web`; fast-check hook per edited file. `run.sh` maps every ESLint finding that no other `lint-web` rule names or maps (the four size rules) to this rule, so its remediation also prints next to a size finding.
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

### web-e2e-journeys
- **Rule:** The app's critical path passes in a real browser against this worktree's `make dev` (NFR-B2): every journey in `apps/web/e2e/*.e2e.ts`, at least `GET /healthz` answering with a body the contract's generated `Health` schema accepts and its `X-Request-Id`, and the web shell rendering the API's status without page or console errors.
- **Why:** Unit tests render components to a string and test handlers in-process; neither proves that Vite, its proxy, the generated client, the API and Postgres work together, which is what a user (and an agent checking its change) sees (exec plan 0003, P2.3).
- **Sensor:** Playwright (`apps/web/playwright.config.ts`, pinned `@playwright/test`, Chromium headless shell in `.bin/playwright`, installed by `make e2e` itself when missing or of another Playwright version, never by `make setup` or `make deps`: [ADR 0025](../adr/0025-browser-e2e-is-the-nfr-b2-sensor.md)), `make e2e [E2E_GREP=<title regex>]` (in `make check-all` and CI; needs Docker). It attaches to this worktree's running `make dev` (when `/healthz` answers through Vite's proxy), else starts one, waits up to 120 s for it and stops it afterwards; in `make check-all` (`HARNESS_SCOPE=full`) it never attaches ([ADR 0014](../adr/0014-make-check-scoped-to-the-change.md)): it starts its own `make dev` and fails with "… is already used" while another one runs. A missing worktree env (`apps/web/e2e/global-setup.ts`) or browser (Playwright's launch) fails, never skips. The config and the global setup are protected (`forbidOnly`, `retries: 0`, the attach rule); journeys (`e2e/*.e2e.ts`) are not.
- **Remediation:** Read the failing step above, then `.tmp/dev/api.log` and `.tmp/dev/web.log` (make dev's output) and the screenshot, page snapshot (`error-context.md`) and trace in `.tmp/e2e/`; fix the code, not the journey, unless the behaviour change is intended. "already used" in `make check-all`: stop your running `make dev` first. A missing env is fixed by `make setup`, a failed browser download by re-running `make e2e`; on Linux, Chromium's system libraries by `make e2e-deps` (needs sudo).
- **Fixture:** none (selftest proves a sensor rejects a planted violation in a copy of the repo; for this sensor that copy would need Docker, its own worktree env with another port block, a Go build and a browser, tens of seconds per selftest run, so it is deferred: [TD-42](../exec-plans/tech-debt-tracker.md)) · Status: team-convention ([ADR 0025](../adr/0025-browser-e2e-is-the-nfr-b2-sensor.md), NFR-B2)

### web-no-dead-code
- **Rule:** No unused files, exports or dependencies in the JS workspace.
- **Why:** Agents leave orphans behind; dead code misleads the next agent reading the repo.
- **Sensor:** knip (`knip.json`), `make knip` (check-all, drift.yml).
- **Remediation:** Delete the unused file/export/dependency; if it is used in a way knip cannot see, configure it in `knip.json` and say why in the PR.
- **Fixture:** `fixtures/web-unused-file` · Status: seed

## Size and complexity (apps/api, apps/web, packages/*)

Maintainability sensors, not style ([ADR 0024](../adr/0024-size-and-complexity-limits-are-maintainability-sensors.md)): each bounds how much code an agent must hold in one read to change it safely. One rule per measure, checked in Go by golangci-lint (`apps/api/.golangci.yml`, `make lint-go`, the `apps/api` module only, not `tools/analyzers` or `tools/harness`) and in TypeScript by ESLint core rules (`eslint.config.mjs`, `make lint-web`). Generated code is excluded. The Go thresholds are the worst value at P4.2's base `1b7aa0f`, re-verified at `0bd8958` (per-function limits: worst non-test function; file length: worst file, tests included); the TS thresholds are ESLint 10.13's documented defaults while the web code is a scaffold. Thresholds only go down, by a harness PR once the code that set them is split; raising one needs an ADR that supersedes 0024. Tests (`*_test.go`, `*.test.ts`, `*.test.tsx`, and the Playwright journeys `*.e2e.ts`, which are tests too) are exempt from the three per-function limits and stay under the file-length limit; test setup that is not a test (`playwright.config.ts`, `e2e/global-setup.ts`) stays under all four. The linters' messages cannot carry the rule id, so `run.sh` maps each finding to its rule (`finding_pattern`: golangci's linter name, revive's rule name, ESLint's rule name) and prints only that measure's split recipe; a failure without such a finding never prints a split recipe. The per-edit hook (`fast-check.sh`) runs the same linters but does not list these rule ids yet, so it prints the other lint rules' remediations instead. golangci reports one finding per line, so a function over two limits shows the second after the first is fixed. A unit that truly cannot be split takes a suppression on its line, e.g. `//nolint:funlen // reason: … (rule: code-function-length)` or `// eslint-disable-next-line max-params -- reason: … (rule: code-parameters)`.

### code-function-length
- **Rule:** No function longer than its threshold: Go at most 35 statements (`funlen`, statements only, line check off); TS at most 50 lines including blanks and comments (`max-lines-per-function`). Tests exempt.
- **Why:** A long function makes the agent hold every step at once to change one; a statement count does not change with formatting or long literals ([ADR 0024](../adr/0024-size-and-complexity-limits-are-maintainability-sensors.md)).
- **Sensor:** golangci-lint `funlen` (`make lint-go`, threshold set by `internal/platform/telemetry/telemetry.go` `(*Telemetry).setup`); ESLint `max-lines-per-function` (`make lint-web`).
- **Remediation:** Extract each step the function performs into a named helper in the same file, kept next to its caller, so the function reads as a list of calls.
- **Fixture:** `fixtures/go-function-too-long`, `fixtures/web-function-too-long` · Status: team-convention ([ADR 0024](../adr/0024-size-and-complexity-limits-are-maintainability-sensors.md))

### code-complexity
- **Rule:** No function more complex than its threshold: Go cognitive complexity at most 13 (`gocognit`); TS cyclomatic complexity at most 20 (`complexity`). Tests exempt.
- **Why:** Nested branches are what make a function hard to read and to change without breaking a case; cognitive complexity penalises nesting ([ADR 0024](../adr/0024-size-and-complexity-limits-are-maintainability-sensors.md)).
- **Sensor:** golangci-lint `gocognit` (`make lint-go`, threshold set by `internal/platform/approvaltest/approvaltest.go` `Verify` and `telemetry.go` `(*Telemetry).setup`); ESLint `complexity` (`make lint-web`).
- **Remediation:** Return early for error and edge cases instead of nesting `else`, move each branch body of a long `if`/`switch` into a function named after the case, and replace a flag parameter with two functions.
- **Fixture:** `fixtures/go-function-too-complex`, `fixtures/web-function-too-complex` · Status: team-convention ([ADR 0024](../adr/0024-size-and-complexity-limits-are-maintainability-sensors.md))

### code-parameters
- **Rule:** No function with more parameters than its threshold: Go at most 4, a variadic counting as one (`revive` `argument-limit`); TS at most 3 (`max-params`). Tests exempt.
- **Why:** Long parameter lists are easy to pass in the wrong order and hide the values that belong together ([ADR 0024](../adr/0024-size-and-complexity-limits-are-maintainability-sensors.md)).
- **Sensor:** golangci-lint `revive` `argument-limit` (`make lint-go`, threshold set by `approvaltest.go` `Verify` and `internal/platform/problem/problem.go` `Write`; tests excluded by a text-matched `exclusions.rules` entry); ESLint `max-params` (`make lint-web`).
- **Remediation:** Group parameters that travel together into a struct (Go) or an object parameter (TS), or make them fields of the receiver they belong to; use an options struct or functional options for optional values.
- **Fixture:** `fixtures/go-too-many-params`, `fixtures/web-too-many-params` · Status: team-convention ([ADR 0024](../adr/0024-size-and-complexity-limits-are-maintainability-sensors.md))

### code-file-length
- **Rule:** No file longer than its threshold, every line counted as `wc -l` does: Go at most 205 lines (`revive` `file-length-limit`); TS/JS at most 300 lines (`max-lines`). Tests included.
- **Why:** A file is the unit an agent reads to learn a responsibility; one that holds several makes every change read them all ([ADR 0024](../adr/0024-size-and-complexity-limits-are-maintainability-sensors.md)).
- **Sensor:** golangci-lint `revive` `file-length-limit` (`make lint-go`, threshold set by `internal/platform/approvaltest/approvaltest.go`, which is protected, so lowering it is a harness PR); ESLint `max-lines` (`make lint-web`).
- **Remediation:** Split the file by responsibility into sibling files in the same package or directory (`handlers.go`, `handlers_export.go`) with no new package and no change to the import graph; in TS, move a component's helpers into their own module next to it.
- **Fixture:** `fixtures/go-file-too-long`, `fixtures/web-file-too-long` · Status: team-convention ([ADR 0024](../adr/0024-size-and-complexity-limits-are-maintainability-sensors.md))

## Duplication (apps/api, apps/web, packages/*)

Agents replicate the code they see (exec plan 0003, G11). Token clones are something a machine decides; the
same concept written twice in different code is left to review. A maintainability sensor like the size rules
([ADR 0027](../adr/0027-token-clones-are-a-maintainability-sensor.md)): Go clones go red in `make check`, TS/JS
clones only in `make check-all`.

### repo-no-clones
- **Rule:** No copy-pasted code: Go at most 25 syntax-tree tokens repeated within one package (`dupl`); TS/JS at most 15 tokens repeated anywhere in `apps/*` and `packages/*` (`jscpd`), import and re-export statements not counted. Tests included; generated code excluded. No baseline or ignore file for existing clones.
- **Why:** A pasted block is fixed in one copy and left wrong in the other, and every copy is more code for the next agent to read and copy again ([ADR 0027](../adr/0027-token-clones-are-a-maintainability-sensor.md)).
- **Sensor:** golangci-lint `dupl` (`make lint-go`, in `make check`, the `apps/api` module only; `apps/api/.golangci.yml` `threshold: 26`, the strictest the tree passes with a `make new-domain` domain in it; 25 trips `internal/platform/problem/problem_test.go`). `dupl` compares the files of one package, so a clone across packages or domains is not seen ([TD-52](../exec-plans/tech-debt-tracker.md)). `jscpd` 5.4.1, a pinned dev dependency (`make clones-web`, in `make check-all`), with the protected `scripts/harness/jscpd.json`: `minTokens` 16 (15 trips `apps/web/e2e/global-setup.test.ts`), `minLines` 1, an `ignorePattern` that skips import and re-export statements (two identical import lines already reach 16 tokens), and `packages/api-client/src/gen/` ignored. It reads every TS, TSX and JS file in `apps/` and `packages/` that `.gitignore` does not exclude. `clones-web` names that config with `--config`, so a `.jscpd.json`, `.config/jscpd.json` or `package.json` `"jscpd"` key elsewhere is never loaded. Thresholds are measured, only go down, by a harness PR once the next clone is deduplicated. golangci's message cannot carry the rule id, so `run.sh` maps a `(dupl)` finding to this rule (`finding_pattern`); the per-edit hook (`fast-check.sh`) does not list it, as for the size rules.
- **Remediation:** Keep one copy and delete the others. Extract the repeated code into a helper next to its call sites, in the same package or app; in tests, a `t.Helper()` function or a table-driven test (TS: one helper module or `test.each`). Across domains, call the owning domain's `service` or `model`; never copy its code and never move domain logic into `internal/platform`, which takes only shared plumbing and test helpers. Pure TS that two apps or packages need goes to `packages/domain`; shared React stays in one module of the app that uses it. A Go clone that must stay takes `//nolint:dupl // reason: … (rule: repo-no-clones)` on the first line of each copy; jscpd has no sanctioned exception: `jscpd:ignore` comments are never allowed ([harness-suppression-reason](#harness-suppression-reason)).
- **Fixture:** `fixtures/go-clone`, `fixtures/web-clone`, `fixtures/web-clone-stray-config` · Status: team-convention ([ADR 0027](../adr/0027-token-clones-are-a-maintainability-sensor.md))

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

### repo-no-orphan-scripts
- **Rule:** Every git-known file under `scripts/` (`scripts/harness/fixtures/` and `scripts/templates/` excluded) is named by its repo path in a referencer: the Makefile, a workflow or action under `.github/`, a Claude hook or `.claude/settings.json`, or a named shell script (executable: only their code counts, not `#` comments or make help text), or a skill, an agent or a guide (Markdown outside `docs/adr/`, `docs/exec-plans/`, `docs/generated/`, `specs/`, `.specify/` and fixtures; all of its text counts). Go code, `package.json` and `compose.yaml` are not referencers. A path counts as a whole token; a `/` before it only after `.`, `..` or a variable expansion (`$ROOT/scripts/x.sh`, `"$root"/scripts/x.sh`), so `apps/scripts/x.sh` does not name `scripts/x.sh`. A file directly in a runner directory, `scripts/test` (`bats scripts/test`) or `scripts/git-hooks` (`core.hooksPath scripts/git-hooks`), is also named when the code of an executable referencer names that directory; no other directory counts. Every target of the Makefile itself whose name matches `[a-z0-9][a-z0-9-]*` either is listed by `make help` (a header line matching `^[a-z0-9-]+:.*##`: one name, no blank before the colon, a `##` comment) or is called by name from another target's prerequisites or recipe, a workflow, an action, a Claude hook, setting, skill or agent, a guide or a named script. Variable-named, file and double-colon targets, names with `_`, `.` or uppercase letters, and targets in included makefiles are not checked; a target-specific variable line (`check-all: export X := y`) is not a call. A fixture is never a referencer, so a script named only by a fixture is an orphan.
- **Why:** Agents add helper scripts and targets and leave them behind when the caller goes; an orphan looks like a live sensor, is kept in sync for nothing and misleads the next agent about how the harness runs.
- **Sensor:** `.bin/harness orphans` (`apps/api/tools/harness`; `make orphans`, in `make check-all`). The runner directories are an allowlist, `runnerDirs` in `orphans.go`: `scripts/test`, `scripts/git-hooks`. Blind spots: a target name that is a common word (`check`, `gen`) is found in any prose, and a script named in a guide counts even if nothing runs it.
- **Remediation:** Wire the script or target up where it should run (a make target, workflow step or hook), give an entry-point target a `##` help comment on its header line (`name: ## <what it does>`), or delete it. Data a script reads counts once that script names it.
- **Fixture:** `fixtures/repo-orphan-script`, `fixtures/repo-orphan-make-target`; `apps/api/tools/harness/orphans_test.go` · Status: team-convention ([ADR 0031](../adr/0031-no-orphan-scripts-or-make-targets.md))

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
- **Rule:** Every added suppression (`//nolint`, `//lint:ignore`, `//lint:file-ignore`, `eslint-disable`, `/* eslint … */`, `@ts-expect-error`, `# shellcheck disable`, `-- atlas:nolint`, `-- squawk-ignore`) has `reason: … (rule: <id>)` on the same line, where `<id>` is a `### <id>` heading in this file; `@ts-ignore`, `@ts-nocheck` and `jscpd:ignore` (jscpd's `jscpd:ignore-start`/`-end`, [repo-no-clones](#repo-no-clones)) are never allowed.
- **Why:** A bare suppression silently turns a sensor off; the reason and rule id make each exception reviewable and greppable.
- **Sensor:** `.bin/harness integrity-check` (`apps/api/tools/harness`) on lines added since the merge base with `BASE_REF` in Go, TS/JS, shell, SQL, git hook and workflow files, not vendored `.specify/` (`make harness-integrity`, in `make check-all` and `harness-integrity.yml`, which builds the binary from the base branch; the PR's `.gitattributes` cannot hide a hunk).
- **Remediation:** Fix the code instead; if the suppression is truly needed, append `reason: <why it is safe> (rule: <rule-id>)` on the same line with an id from this file as it is on the base branch, replace `@ts-ignore`/`@ts-nocheck` with a fixed type or a reasoned `@ts-expect-error`, and remove a `jscpd:ignore` by deduplicating the code.
- **Fixture:** `fixtures/harness-suppression-no-reason` · Status: seed

### harness-config-not-loosened
- **Rule:** No non-comment line of a harness config (`config` entries in `scripts/harness/protected-paths.txt`: linter configs, Makefile, the harness's own workflows and composite action, each named on its own line (a product's workflow or action beside them is not a harness config, [ADR 0035](../adr/0035-harness-files-under-github-are-named.md)), CODEOWNERS, `.gitattributes`, tool versions, the protected-path list itself, and sensor code: `scripts/harness/`, custom analyzers, Claude hooks, the tool installer) is added or removed, and no such file changes its mode, unless a human approves (a shebang is not a comment). Tightening counts too: telling it from loosening takes judgment. Only `#` and blank lines count as comments (`//` and `/* */` can comment code out).
- **Why:** The cheapest way to make a red sensor green is to weaken its config; that must be a visible human decision, not an agent's shortcut.
- **Sensor:** `.bin/harness integrity-check` on the diff since the merge base with `BASE_REF` (`make harness-integrity`, `harness-integrity.yml`; CI runs the base branch's workflow, binary (built from the base checkout) and path list via `pull_request_target` ([ADR 0010](../adr/0010-base-branch-gates.md)), so a PR cannot unprotect itself). Waived only by `HARNESS_CHANGE_APPROVED=1`, set in CI by the `harness-change-approved` PR label.
- **Remediation:** Revert the config change and fix the code; if the rule itself is wrong, explain why in the PR description (Harness change: Y + rationale) and ask a code owner to add the `harness-change-approved` label. A workflow or action under `.github/` that `protected-paths.txt` does not name is the product's: change it freely (it still gets code-owner review, and none of its jobs may reuse a required check's name). A new harness workflow is product until a harness PR lists it there and in `.github/CODEOWNERS`, with `# Harness file (protected)` as its first line (`scripts/test/codeowners.bats` checks that the list and the header name the same workflows and actions, that the list and CODEOWNERS name the same `.github/` files, and that each is tracked; `TestIntegrityCheckRealListSparesProductWorkflows` checks a product workflow change passes against the real list).
- **Fixture:** `fixtures/harness-config-loosened`; `apps/api/tools/harness/integritycheck_test.go`, `scripts/test/codeowners.bats` · Status: seed

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
- **Rule:** `make check` (scope `changed`) may skip a sensor only when none of its inputs and no harness file (`protected-paths.txt`) changed since the merge base, counting staged, unstaged and untracked files; `make check-all` and CI (scope `full`, `HARNESS_SCOPE=full`) run every sensor. Skipped today: `lint-sh`, `fmt-check-sh`, `test-sh` (inputs: `*.sh`, `*.bash`, `*.bats`, `scripts/**`), the main cost of `make check` (NFR-H3, [ADR 0013](../adr/0013-harness-nfr-budgets.md), [ADR 0014](../adr/0014-make-check-scoped-to-the-change.md)); `fixture-claims` (inputs: `docs/harness/rules.md`, `scripts/harness/fixtures/**`); `approved-check` (inputs: `api/openapi.yaml`, `*/testdata/approved/*`); `specs-tested` (inputs: `specs/**`, `*_test.go`, `*.test.*`, `*.spec.*`, `*.e2e.*`); `gen-check` while its inputs and the generated files hash to the stamp of its last green run. Scoped to the affected set in scope `changed`: `lint-go` and `test-go` to the changed Go packages of `apps/api` plus every package that imports them (tests included), `lint-web` and `arch-web` to the changed JS workspaces plus their dependents; skipped when that set is empty; `secrets` ([repo-no-secrets](#repo-no-secrets)) to the changed and untracked files, all git-known files when a harness file changed. `typecheck-web` and `test-web` are not scoped: Turborepo's task cache already replays unchanged packages. `arch-go` is not scoped (go-arch-lint checks the whole module). Cached only in scope `changed`: Go test results (`test-go`, `test-analyzers`, `test-harness` without `-count=1`; integration tests always `-count=1`) the dependency-cruiser graph (`--cache node_modules/.cache/depcruise`) and Turborepo's task cache (`typecheck-web`, `test-web`; `--force` in any other scope, so `make harness-determinism` really runs vitest and tsc twice).
- **Why:** Agents run `make check` after every change; a sensor whose inputs did not change can only repeat its last verdict, so running it costs time and finds nothing.
- **Sensor:** `scripts/harness/unchanged.sh` decides (`gen-check.sh --unchanged` for `gen-check`), `scripts/harness/affected.sh go|web` the affected set (`$(call scoped,…)` in the Makefile); the target prints `<target>: skipped, …` instead of running. It fails closed: no merge base (`BASE_REF`), a git error, a missing or unmatched stamp, or any scope but `changed` means run and uncached; the affected set is everything on a harness file, `go.mod`, `go.sum`, a `package.json`, the lockfile or workspace config, a non-Go file under `apps/api`, a `.go` file in no package, a JS/TS/JSON file outside a workspace, or a failing `go list` or `turbo ls`. Timing rows (`harness-latency-budget`) carry the scope.
- **Remediation:** A skipped sensor that should have run means its trigger pathspecs miss an input: add the input to the `$(call sh_sensor,…)` pathspecs in the Makefile, to the `unchanged.sh` pathspecs of its `fixture-claims`, `approved-check` or `specs-tested` recipe, to `INPUTS` in `gen-check.sh`, or to what makes `affected.sh` return everything (Harness change: Y); to run everything now, `make check-all` or `HARNESS_SCOPE=full make check`.
- **Fixture:** none (decides whether a sensor runs, never fails); `scripts/test/unchanged.bats`, `scripts/test/gen-check.bats`, `scripts/test/affected.bats`, `scripts/test/trivy.bats`; the scoped sensors' fixtures `go-tests-dependent-package`, `web-lint-dependent-workspace` run in scope `changed` (`SCOPE=changed` in `fixture.conf`, also in CI) · Status: seed

### harness-upstream-in-sync
- **Rule:** Every harness file (`protected-paths.txt`, this repo's and upstream's, except `.harness-version` itself) that changed in the upstream template since the commit in `.harness-version` matches upstream here (mode and content), or is declined with a `# declined <path>: <why>` line in `.harness-version`; `.harness-version` moves only in the harness PR that ports those changes. `HARNESS_UPSTREAM` is empty only in the template itself; `make rename` writes the template's URL.
- **Why:** A product keeps the harness of the day it was cut while the template's sensors keep improving; nobody notices a harness falling behind until a fixed bug bites again (exec plan 0003, G7; [ADR 0026](../adr/0026-template-sync-is-a-drift-report.md)).
- **Sensor:** `scripts/harness/sync-check.sh`, `make harness-sync-check [HARNESS_UPSTREAM=<url>]` (drift.yml, weekly; reads the network, so in neither `make check` nor `check-all`). The URL comes from `HARNESS_UPSTREAM` in `scripts/tool-versions.env` or the make command line; empty prints "this repository is the template; nothing to sync" and exits 0. It fetches upstream's default branch anonymously (`GIT_ALLOW_PROTOCOL=file:https:ssh`, no prompts, bounded in time) into a temp bare repo and prints `<path>: changed|added|deleted upstream[, changed here too (merge by hand) | present here too (merge by hand) | missing here] since <commit>` for each harness file whose upstream mode and content differ from the working tree's, then the upstream commit; declined paths print as `<path>: declined (<why>), changed upstream since <commit>` and do not count. It never applies anything. Script exit 0: in sync; 1: behind; 2: cannot check, failing closed (a missing or malformed `.harness-version` or declined line, a URL starting with `-` or holding credentials, a failed or timed-out fetch, a marker commit not in upstream's history, no harness paths, any unexpected error). make reports 1 and 2 alike as its exit 2.
- **Remediation:** For each listed file, read `git diff <marker> FETCH_HEAD -- <path>` after `git fetch <upstream> HEAD` and port the change in a harness PR, or add `# declined <path>: <why>` to `.harness-version`; the same PR writes the printed upstream commit to `.harness-version`. On exit 2: set `HARNESS_UPSTREAM` in `scripts/tool-versions.env` to the template's git URL without credentials (use a credential helper or deploy key; a private template only checks locally, the drift fetch is anonymous) and `.harness-version` to the template commit this repo was copied from.
- **Fixture:** `fixtures/harness-behind-upstream`; `scripts/test/sync-check.bats` · Status: team-convention ([ADR 0026](../adr/0026-template-sync-is-a-drift-report.md))

### ci-workflows-valid
- **Rule:** GitHub workflows and composite actions pass actionlint, including shellcheck on every `run:` block.
- **Why:** A broken workflow fails only after push, or worse, never runs the gate it was meant to run.
- **Sensor:** actionlint (`make lint-ci`, part of `make check`).
- **Remediation:** Fix the line actionlint names (unknown context/expression, bad `needs`/`if`, or the shellcheck code in a `run:` block); pin new actions to a full commit SHA with a `# vX.Y.Z` comment.
- **Fixture:** `fixtures/ci-invalid-workflow` · Status: seed

### harness-layout-valid
- **Rule:** `scripts/harness/layout.env` is the one place that says where the product's code lives (`API_DIR`, `SPEC`, `MIGRATIONS_DIR`, `QUERIES_DIR`, `API_CLIENT_DIR`, `WEB_GLOBS`, `GENERATED_GLOBS`, `PRODUCT_GLOBS`, `LAYER_GLOB`), and its cross-repo contracts (`UPSTREAM_SPECS`, `CONTRACT_CONSUMERS`). The scripts in `scripts/harness/`, the Makefile, `.bin/harness`, the domainiso analyzer and the CI workflows read it and name none of those paths themselves; the files that cannot read it (linter configs, CODEOWNERS, `protected-paths.txt`, codegen configs, `fast-check.sh`, ...) are the named exceptions listed in README.md, "(b) Brownfield project" step 5. It parses (`KEY=value`, no quotes or `$`, every key set once, no unknown key, no empty list entry; only `UPSTREAM_SPECS` and `CONTRACT_CONSUMERS` may be empty or absent), every path it names exists (`QUERIES_DIR` or its parent, until the first domain), each `WEB_GLOBS` entry is `<existing directory>/*`, and each `UPSTREAM_SPECS` entry is `<vendored spec>@<source repo>@<spec path in the source>@<revision>` with an existing vendored file inside the repo (not absolute, no `..`), a source that is a host path or an absolute path (not starting with `-`, no `..`), a plain spec path (not absolute, no `.`, `..` or empty segment) and a 7 to 40 digit hex revision, the checks upstream-drift.sh repeats at run time (shared vectors: `apps/api/tools/harness/testdata/upstream-spec-vectors.txt`; [api-upstream-no-breaking-change](#api-upstream-no-breaking-change)), `LAYER_GLOB` is `<dir>[/<dir>…]/*/*` with literal segments (no empty, `.`, `..` or wildcard segment) and matches at least one Go package under `API_DIR`, with no flat package `<dir>/<pkg>` beside them, since the analyzer skips those (`<dir>/platform` is shared plumbing, skipped, never flat; a tree with only platform packages is the template before its first domain). `STACK` lists the sensor groups the product has (`go`, mandatory, then `db` and `web`, each once; optional, `go:db:web` when unset, so a `layout.env` written before it still reads; `HARNESS_STACK` in the environment overrides it for one run, with `,` or `:`); the paths of a group it leaves out are not checked, but a git-known file of that group (a migration, a query, a `package.json` under a `WEB_GLOBS` entry, or the `API_CLIENT_DIR` directory) is a finding, so narrowing the stack never hides a group the tree still has; its sensors, tools, fixtures and Postgres drop out of `make check`, `check-all`, `tools`, `harness-selftest` and `setup` ([ADR 0033](../adr/0033-stack-profiles.md)) (exec plan 0003, P4.6, P4.7).
- **Why:** A product that moves or renames a directory changed it in a dozen sensors by hand, and a sensor that still pointed at the old path went silently green (a pathspec that matches nothing finds nothing); a `LAYER_GLOB` matching no package turns [go-domain-isolation](#go-domain-isolation) off without a word.
- **Sensor:** `.bin/harness layout-check` (`make layout-check`, part of `make check`) prints `scripts/harness/layout.env:<line>: <KEY>=<value>: <problem> [harness-layout-valid]`. Every reader fails closed on an unreadable layout with `<file>:<line>: <message> [harness-layout-valid]`: a script sourcing `scripts/harness/layout.sh` exits 2, the Makefile stops at parse time (it and CI read `layout.sh --print`; it also stops there, with an `$(error)` citing this rule, when `STACK` or `STACK_OFF` comes from the make command line, and layout.env's keys win over the environment even under `make -e`; `scripts/test/make-stack.bats` covers both), `.bin/harness` and the domainiso analyzer refuse to run, and the migration gate and `migrations-append-only.sh` read both the base and the head copy and exit 2 when either is invalid. Both Go parsers of `LAYER_GLOB` (layout-check and domainiso) test against the shared vectors in `apps/api/tools/harness/testdata/layer-globs.txt`; both parsers of `STACK` (`layout.sh` and `layout.go`) against `apps/api/tools/harness/testdata/stack-vectors.txt`. A group left out while its files exist prints `scripts/harness/layout.env:<line>: STACK=<value>: leaves out <group>, but <path> exists: … [harness-layout-valid]`, and `make check` names the targets the stack leaves out on stderr.
- **Remediation:** Fix the named line in `scripts/harness/layout.env` (or move the code to where it points) and re-point the files that cannot read it, listed in README.md, "(b) Brownfield project" step 5. Harness change: Y.
- **Fixture:** `fixtures/harness-layout-no-layer`, `fixtures/harness-stack-hides-web`; `scripts/test/layout.bats`, `scripts/test/selftest.bats`, `scripts/test/make-stack.bats`, `scripts/test/migrations-append-only.bats`, `apps/api/tools/harness/layout_test.go`, `apps/api/tools/analyzers/domainiso/layout_test.go` · Status: team-convention ([ADR 0030](../adr/0030-one-layout-file-for-every-sensor.md), [ADR 0033](../adr/0033-stack-profiles.md))

### harness-reads-layout
- **Rule:** No string a non-test source of `.bin/harness` (`apps/api/tools/harness/*.go`) other than `layout.go` builds from literals names a path `scripts/harness/layout.env` gives: `API_DIR`, `SPEC`, `MIGRATIONS_DIR`, `QUERIES_DIR`, `API_CLIENT_DIR` (the last three also relative to `API_DIR`) and `LAYER_GLOB`'s literal prefix, in the template `apps/api`, `api/openapi.yaml`, `db/migrations`, `internal` and so on. Checked are a string literal, a `+` of adjacent literals and adjacent literal arguments of `filepath.Join` or `path.Join`, matched whole path segments at a time (`apps/api` is in `apps/api/internal`, not in `apps/apix`). Not caught: a path assembled through a named constant or variable, `fmt.Sprintf`, `strings.Join` or another call, or split by a non-literal argument (`filepath.Join("apps", dir, "api")`); comments and import paths are not checked; what `layout.go` itself spells, and paths layout.env has no key for (`apps/web`, TD-46), are for review. The sensors ask `layout.go` for every product path.
- **Why:** A sensor that spells the template's path checks the template's tree after a product moves its code, and goes silently green there ([harness-layout-valid](#harness-layout-valid)); reviewers caught such literals in `.bin/harness` twice, once before layout.env existed (`approvedcheck.go`) and once after (`qualityrows.go`).
- **Sensor:** `TestNoLayoutLiterals` in `apps/api/tools/harness/layoutliterals_test.go` parses the package's sources (`make test-harness`, part of `make check`) and prints `<file>:<line>: string literal names the template layout path "<path>": … [harness-reads-layout]`.
- **Remediation:** Read the path from `scripts/harness/layout.env` through `layout.go` (`loadLayout`, then `get`, `list` or `apiPath`) instead of spelling it; a kind of path layout.env has no key for is a new key, in a harness PR.
- **Fixture:** `fixtures/harness-reads-layout-literal` · Status: earned ([Phase 2 gate, TD-46 as opened in fad6b28: `approvedcheck.go:99` spelled `apps/api/internal/platform/health/…`, fixed in 449d561](../exec-plans/tech-debt-tracker.md), [Phase 3 gate, Log 2026-10-29: `qualityrows.go` held `domainsDir = "apps/api/internal/"`, fixed in d8a6ff9](../exec-plans/active/0003-harness-gap-closure.md#log))

### harness-rule-lists-match
- **Rule:** The agent prompts that copy rules.md by hand (`.claude/skills/review-*/SKILL.md`, `.claude/agents/reuse-reviewer.md`) keep skipping what `requiredSkips` in `apps/api/tools/harness/rulelists.go` lists for them, skip only rules that exist, and skip every rule of a section they skip whole. A file's `## Skip` section runs to the next heading of level 1 or 2. In it, a code span (it may wrap a line or use double backticks) holding a lowercase token with at least one hyphen (`` `go-layers` ``; not `` `dupl` ``, `` `*.gen.go` `` or a path) is a rule id and must have a `### <id>` heading here, so write tool names such as golangci-lint or pre-commit without backticks there. A code span `` `## <section>` `` (the heading's text before any ` (`) claims that whole section, and then each `### <id>` under it must be named in the same Skip section. Every id and claim `requiredSkips` lists for the file must be there; `requiredSkips` has an entry for each file and names only ids and sections rules.md has. An entry joins it when a new sensor takes over a reviewer's finding class, and leaves only in a harness PR that says why the reviewer reports that class again.
- **Why:** The lists drifted by omission and by misspelling: review-modularity never skipped `repo-no-clones` or `repo-no-orphan-scripts`, so agents reported findings `make check` catches, and a renamed or misspelled id silently stops skipping anything.
- **Sensor:** `.bin/harness rule-lists` (`apps/api/tools/harness/rulelists.go`; `make rule-lists`, part of `make check` when rules.md or a harness file changed) prints `<file>:<line>: <message> [harness-rule-lists-match]`; it fails closed when a glob matches no file, a file has no `## Skip` section, rules.md repeats an id, or `requiredSkips` and the files or rules.md disagree.
- **Remediation:** Put back each id or `## <title>` claim the Skip list must keep (or drop it from `requiredSkips` in a harness PR that says why the reviewer reports it again), name each skipped rule by the `### ` id rules.md gives it, and when the Skip section claims a whole section, add every rule of that section to the list or drop the claim; the agent files are protected, so propose the change in a harness PR.
- **Fixture:** `fixtures/harness-rule-list-unknown-id`, `fixtures/harness-rule-list-section-incomplete`, `fixtures/harness-rule-list-required-missing`; `apps/api/tools/harness/rulelists_test.go` · Status: earned ([P3.8 lane review, Log 2026-10-29: reuse-reviewer's rule ids, fixed before ddd125f](../exec-plans/active/0003-harness-gap-closure.md#log), [Phase 3 gate, TD-94 as opened in d8a6ff9: review-modularity missed `repo-no-clones` and `repo-no-orphan-scripts`](../exec-plans/tech-debt-tracker.md))
### harness-adoption-probe
- **Rule:** README.md, "(b) Brownfield project", followed step by step on a minimal brownfield repo (`scripts/templates/adoption-probe/`: a root `go.mod`, `internal/<domain>/<layer>`, an OpenAPI contract, no db, no web) ends with `make setup`, `make check` and `make harness-selftest` green in that repo. The probe does only the hand edits the README asks for, and each must change its file; the paths README (b) step 3 names are exactly those the probe copies or skips, and the paths of step 6's adoption-probe bullet exactly those it drops or declines plus the synced harness files it leaves as they are ([ADR 0038](../adr/0038-readme-brownfield-adoption-is-probed.md) amendment: CI's job passes on the script's `# declined` line). The skeleton's `layout.env` is what step 5 writes, its `drop.txt` what step 6 drops by path and its `fixtures/` the five fixtures step 6 rewrites against the brownfield code (exec plan 0003, P4.10).
- **Why:** The brownfield procedure is prose that names the harness's files, paths and targets; every harness change can make it wrong, and nothing noticed until an adopter followed it and got stuck. A required check, not a weekly drift report, because the harness PR that breaks README (b) is the one that should fix it.
- **Sensor:** `scripts/harness/adoption-probe.sh` (`make adoption-probe`; CI's required `adoption-probe` job, minutes, so in neither `make check` nor `check-all`). Before any heavy step it compares the path-like code spans of README (b) step 3 and of step 6's adoption-probe bullet with its own lists, both ways. Then it commits the skeleton into a temp git repo, adopts this checkout's harness into it (steps 1–7; `.bin` links to this checkout's pinned tools, the harness tool and custom golangci-lint are built there) and prints, for the first step that fails, its output and `README.md:<line>: step <n> … [harness-adoption-probe]`, `<line>` being that step's line in README.md: ``step <n> names `<path>`, which the adoption probe neither handles nor skips``, ``step <n> no longer names `<path>`…``, `step <n>: hand edit <expr> matched nothing in <file>`, ``step <n>: `<path>` matches no file in the template``, or `step <n> (<what>) no longer adopts the harness into a root-go.mod repo`. Exit 1 on a finding, 2 when it cannot run (no skeleton, an invalid skeleton `layout.env`, a `drop.txt` line naming a path step 3 does not copy). `ADOPTION_PROBE_KEEP=1` keeps the temp repo. Blind spots: one layout (root module, go stack); a repo with its own Makefile, workflows or Claude Code setup takes the README's merge paths, which the probe never walks; README text outside code spans is not compared; `drop.txt` drops eleven bats files whose tests assume the template layout instead of porting them (tracker TD-99), and the overlay fixtures replace five template fixtures (TD-100).
- **Remediation:** Run `ADOPTION_PROBE_KEEP=1 make adoption-probe` and read the failing step in the kept repo. Fix README.md (b) where it no longer matches the harness, or the harness where it hard-codes the template layout; change the probe's lists and hand edits, `layout.env`, `drop.txt` or `fixtures/` only as far as the README's text changes. Harness change: Y.
- **Fixture:** `fixtures/harness-adoption-stale-layout`; `scripts/test/adoption-probe.bats` · Status: team-convention ([ADR 0038](../adr/0038-readme-brownfield-adoption-is-probed.md))

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

### specs-scenarios-tested
- **Rule:** Every acceptance scenario of a spec is named by a Go or JS/TS test, or its text ends with `(untested: <reason>)`. A story is a heading of level 1 to 3 (spec-kit writes `### User Story <n> - <title>`) whose text starts with `User Story <n>`, up to the next heading of level 1 to 3; its scenarios are the items of the ordered list right after its `**Acceptance Scenarios**:` paragraph (a nested list is steps of one scenario). A scenario's id is `<NNN>/US<n>-AS<m>`: `NNN` from the directory, `n` from the story heading, `m` the item's position in the list (1-based, whatever number it is written with); nothing is written into the spec. A test names the id when the id appears anywhere in a test file (title, string or comment, such as `t.Run("001/US1-AS2 pays", …)`, `it('001/US1-AS2: …')` or `// spec: 001/US1-AS2`) as a whole token, with no letter, digit, `_`, `/`, `.` or `-` next to it; the sensor does not parse test syntax. Test files are `*_test.go`, `*.test.*`, `*.spec.*` and `*.e2e.*` under `PRODUCT_GLOBS` and `WEB_GLOBS` (so `packages/domain` counts), not under `API_CLIENT_DIR`, `GENERATED_GLOBS` or the harness files of `protected-paths.txt`. A line that also skips or defers its test (`.todo(`, `.skip(`, `.fixme(`, `xit(`, `xtest(`, `xdescribe(`, `t.Skip`) names nothing. Findings too, since scenarios would have no id, a shared one, or the spec none at all: a spec without a story, a story heading without a number, of level 4 or below, or numbered twice in a spec, a story without an ordered scenario list, and an `**Acceptance Scenarios**:` paragraph outside a story. A reason may hold one level of parentheses; an empty one or a placeholder (`n/a`, `na`, `tbd`, `todo`, `none`, `-`, `x`, `?`) is no exemption.
- **Why:** A scenario nobody tests is a requirement the code was never checked against; agents tick off tasks, not scenarios, so a story can ship with half its scenarios unproven. A red run names the untested ids with their text and sends the agent back to the test step on its own (exec plan 0003, G13). Whether the test that names an id really checks its scenario, and whether an exemption's reason holds, is the reviewer's call ([ADR 0019](../adr/0019-inferential-sensors-are-advisory.md)).
- **Sensor:** `.bin/harness specs-tested` (`apps/api/tools/harness/scenarios.go`, which parses specs with goldmark, so fenced code, HTML blocks and comments hold no stories or scenarios; `make specs-tested`, part of `make check`, skipped while `specs/`, the test files and the other harness files are unchanged: [harness-scope](#harness-scope)) prints `specs/<NNN-slug>/spec.md:<line>: <id> has no test naming it: <scenario> [specs-scenarios-tested]`, or the structural finding. Directories not named `NNN-kebab-slug` are left to [specs-structure](#specs-structure); a symlinked `spec.md` is a finding and a symlinked test file names nothing. Fails closed (exit 2) on an unreadable layout, `protected-paths.txt` or spec, or a git error.
- **Remediation:** Write the test that checks the scenario and name its id in the test's title or a `// spec: <id>` comment (the id is printed above), or, when no automated test can check it, end the scenario in the spec with `(untested: <why>)`, a real reason; or fix the story heading or scenario list as the message says. Never rename or delete a scenario, or skip the test that names it, to silence this.
- **Fixture:** `fixtures/specs-untested-scenario`; `apps/api/tools/harness/scenarios_test.go` · Status: team-convention ([ADR 0036](../adr/0036-acceptance-scenarios-are-traced-to-tests.md))

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

### docs-quality-rows-well-formed
- **Rule:** Every row of `docs/QUALITY.md` between the table header and the `harness:quality-rows` marker has one cell per header column (no `|` inside a cell); its `Domain` is a domain name or `[<domain>](../<domains>/<domain>/AGENTS.md)` (link text and target name the same domain; `<domains>` is the domains' directory from `scripts/harness/layout.env`, `API_DIR` plus `LAYER_GLOB` without its `/*/*`: `apps/api/internal` in the template); each layer (`model`, `store`, `service`, `api`) is graded `A`–`F` or `–` (en dash U+2013: not graded yet; a hyphen is not); and `Updated` is a real date written `YYYY-MM-DD`.
- **Why:** The grades tell an agent which layers it can change safely, and the age check and the domain match can only read a row that parses; a hyphen for "not graded" or a link to another domain's guide reads right to a human and wrong to every tool (exec plan 0003 P3.5, after OpenAI's per-domain and per-layer quality grades).
- **Sensor:** `.bin/harness quality-rows -no-clock` (`apps/api/tools/harness/qualityrows.go`; `make docs-quality-shape`, part of `make check-all`; the CI generator probe runs it on the row a fresh `make new-domain` writes). Deterministic: it reads no clock; it exits 2 when `layout.env` is missing or malformed. `TestQualityRowTemplateMatchesHeader` (`make check`, via `test-harness`) renders the generator's row template under the real header and checks that both modes accept it.
- **Remediation:** Make the row match the header named in the message: one cell per column, the domain's link as written above, grades `A`–`F` or `–` by the legend under the table, `Updated` as `YYYY-MM-DD`.
- **Fixture:** `fixtures/docs-quality-row-malformed` · Status: team-convention ([ADR 0028](../adr/0028-quality-grades-per-domain-and-layer.md))

### docs-quality-rows-match-domains
- **Rule:** `docs/QUALITY.md` has exactly one row per backend domain: none for a name without a git-known file under `<domains>/<domain>/` (the domains' directory from `layout.env`, as in [docs-quality-rows-well-formed](#docs-quality-rows-well-formed); `platform` is no domain), none missing for a domain, none twice.
- **Why:** A row for a deleted domain grades code that no longer exists, and a missing row hides a domain from the grades (and from [docs-quality-row-fresh](#docs-quality-row-fresh)), so deleting a stale row cannot silence the age check. Deleting a domain also breaks its row's link to the domain's `AGENTS.md`, so [docs-links-resolve](#docs-links-resolve) usually turns `make check` red first; this rule names the real fix and also covers a row without that link.
- **Sensor:** `.bin/harness quality-rows -no-clock` (`apps/api/tools/harness/qualityrows.go`; `make docs-quality-shape`, part of `make check-all` and the CI generator probe), with [docs-quality-rows-well-formed](#docs-quality-rows-well-formed) in one pass over the file.
- **Remediation:** Delete the row of a domain that is gone, merge a second row into the first, or add the missing domain's row before the `harness:quality-rows` marker (as `make new-domain` does) and grade it.
- **Fixture:** `fixtures/docs-quality-row-orphan`, `fixtures/docs-quality-row-missing`; `apps/api/tools/harness/qualityrows_test.go` · Status: team-convention ([ADR 0028](../adr/0028-quality-grades-per-domain-and-layer.md))

### docs-quality-row-fresh
- **Rule:** The `Updated` date of every well-formed `docs/QUALITY.md` row is at most 30 days before today's UTC date and not after it.
- **Why:** A grade nobody has looked at for a month describes code that has since changed, and it reads as current.
- **Sensor:** `.bin/harness quality-rows` (`apps/api/tools/harness/qualityrows.go`; `make docs-quality`), run by the weekly `drift.yml` sweep only; its findings go to the `drift` issue. Never in `make check`, `make check-all` or a required CI job: the same tree turns red with time, and only the domain's reviewer can grade a row again, so the author of an unrelated PR could not fix it ([ADR 0028](../adr/0028-quality-grades-per-domain-and-layer.md)). Rows that are not well-formed are left to [docs-quality-rows-well-formed](#docs-quality-rows-well-formed).
- **Remediation:** Grade the row again (each layer `A`–`F` by the legend under the table) and set `Updated` to today; never change only the date.
- **Fixture:** `fixtures/docs-quality-row-stale` · Status: team-convention ([ADR 0028](../adr/0028-quality-grades-per-domain-and-layer.md))

### docs-references-pinned
- **Rule:** Every `docs/references/<tool>-llms.txt` carries, in its first 20 lines, one `Version:` line, one `Pin: <file> <key>` line and at least one `Source:` line. `<file>` is a relative path without `..` that git knows (tracked, or untracked and not ignored): either a `*.env` file, read as plain `KEY=value` lines only (the subset `scripts/tool-versions.env` mandates: no `export`, quotes or spaces; blank and `#` lines skipped), or a `package.json` (`dependencies`/`devDependencies`), whose value must be one exact version, not a range. `Version` equals the value `<key>` has in `<file>`, exact string, and every `Source` URL names it as a whole path segment or tag (`/v1.3.0/`, `@8.45.0`), not inside another version.
- **Why:** The references are upstream docs taken at the pinned version so agents work from the API they actually run, not from training data or the latest docs. Raising a pin without taking the file again leaves an agent reading the old tool's options as current. The sensor checks the header claim, not the body: whoever raises the pin takes the body again. It reads only a pin file git knows, so an ignored local file (`.env.local`) is never read or printed.
- **Sensor:** `.bin/harness references-pinned` (`apps/api/tools/harness/references.go`; `make references-pinned`, part of `make check`).
- **Remediation:** Take the file again from upstream at the version the pin now names (tagged docs or README at that tag, trimmed to what an agent needs) and update `Version:` and every `Source:` in the same change as the pin; a new reference file names its pin in a `Pin: <file> <key>` line that points at a committed pin file; for a range finding, pin the `package.json` dependency exactly (no `^`, `~`, `x`, `||` or hyphen range).
- **Fixture:** `fixtures/docs-reference-stale` · Status: team-convention ([ADR 0029](../adr/0029-tool-references-follow-the-pin.md))

### rules-fixture-claims
- **Rule:** Every `### <rule-id>` in this file is unique (one heading per id) and has one `**Fixture:**` line, either `none (<reason>)` (free text, may name other rules' fixtures and tests) or one or more `` `fixtures/<dir>` `` joined by `, `, optionally followed by `; ` and backticked test paths. Each named dir is a git-known `scripts/harness/fixtures/<dir>/fixture.conf` whose `RULE=` is that rule's id; each test path matches a git-known file.
- **Why:** The selftest ([ADR 0009](../adr/0009-harness-selftest.md)) proves only the fixtures that exist; a line claiming coverage that is not there makes this guide promise a tested sensor that nothing tests (exec plan 0003 found seven such lines).
- **Sensor:** `.bin/harness fixture-claims` (`apps/api/tools/harness/fixtureclaims.go`; `make fixture-claims`, part of `make check`, skipped while rules.md, the fixtures and the other harness files are unchanged: [harness-scope](#harness-scope)).
- **Remediation:** Make the line say what is true: name each fixture as `` `fixtures/<dir>` `` (the directory under `scripts/harness/fixtures/` whose `fixture.conf` has `RULE=` this rule), add a fixture if the sensor is deterministic and local, or write `none (<why this rule has no fixture>)`; never claim another rule's fixture as coverage; merge a duplicated `### <id>` into one section or rename one of them.
- **Fixture:** `fixtures/rules-bogus-fixture-claim` · Status: team-convention ([ADR 0015](../adr/0015-anticipated-rules-and-nfr-classes-b-o.md))
