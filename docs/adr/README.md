# Architecture Decision Records

Short records: context, decision, consequences. One file per decision, numbered, never rewritten —
supersede with a new ADR. Architectural PRs link an ADR in the PR template.

| ADR | Title | Status |
|-----|-------|--------|
| [0001](0001-tool-pinning-and-single-entry-point.md) | Tool pinning in `.bin` and Makefile as single entry point | accepted |
| [0002](0002-go-service-layout.md) | Go service layout: module in `apps/api`, net/http, slog, pgx | accepted |
| [0003](0003-layer-enforcement.md) | Layer enforcement: go-arch-lint + depguard + custom analyzers | accepted |
| [0004](0004-migrations-atlas-squawk-sqlc.md) | Migrations: Atlas CE, squawk, sqlc | accepted |
| [0005](0005-spec-first-contract.md) | Spec-first contract, one strict server per tag, RFC 9457 errors | accepted |
| [0006](0006-contract-gate.md) | Breaking contract changes need a human (contract-gate) | accepted |
| [0007](0007-frontend-stack-and-boundaries.md) | Frontend stack and boundaries (pnpm, orval+zod, dep-cruiser) | accepted |
| [0008](0008-per-worktree-isolation.md) | One isolated environment (DB, ports) per git worktree | accepted |
| [0009](0009-harness-selftest.md) | Sensors are tested with violation fixtures (harness-selftest) | accepted |
| [0010](0010-base-branch-gates.md) | Label-waivable gates run the base branch's harness (pull_request_target) | accepted |
| [0011](0011-conventional-commits-and-spec-kit.md) | Conventional Commits and spec-kit specs for product features | accepted |
| [0012](0012-parsing-sensors-in-go.md) | Sensors that parse move to one Go binary; orchestration stays in bash | proposed |
| [0013](0013-harness-nfr-budgets.md) | Harness NFRs are budgets; latency alerts, reliability fails | proposed |
| [0014](0014-make-check-scoped-to-the-change.md) | make check runs what the change can affect; check-all and CI run everything uncached | proposed |
| [0015](0015-anticipated-rules-and-nfr-classes-b-o.md) | Phase 0 rules are adopted as team-convention; NFR classes B and O fail | proposed |
| [0016](0016-tool-supply-chain.md) | Downloaded tools are sha256-pinned; scanners run only as pinned release binaries | proposed |
| [0017](0017-per-worktree-observability-stack.md) | One pinned grafana/otel-lgtm container per worktree is the observability stack (amends 0008) | proposed |
| [0018](0018-http-and-db-instrumentation.md) | HTTP and database instrumentation through otelhttp and otelpgx (amends 0002) | proposed |
| [0019](0019-inferential-sensors-are-advisory.md) | Inferential sensors are advisory; a finding class seen twice becomes a computational rule | proposed |
| [0020](0020-test-coverage-is-a-trend.md) | Test coverage is a trend, never a gate | proposed |
| [0021](0021-telemetry-export-over-otlp-http.md) | Telemetry leaves the process over OTLP/HTTP, one exporter per signal (amends 0002, 0018) | proposed |
| [0022](0022-approved-api-responses.md) | Handler tests pin API responses to approved files, checked against the contract | proposed |
| [0023](0023-mutation-testing-is-a-drift-report.md) | Mutation testing is a weekly drift report, never a gate (extends 0020) | proposed |
| [0024](0024-size-and-complexity-limits-are-maintainability-sensors.md) | Size and complexity limits are maintainability sensors, not style (amends core belief 3) | proposed |
