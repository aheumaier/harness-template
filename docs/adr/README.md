# Architecture Decision Records

Short records: context, decision, consequences. One file per decision, numbered, never rewritten —
supersede with a new ADR. A proposed ADR may be amended in place with a dated note; an accepted ADR changes
only through a new ADR. Architectural PRs link an ADR in the PR template.

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
| [0012](0012-parsing-sensors-in-go.md) | Sensors that parse move to one Go binary; orchestration stays in bash | accepted |
| [0013](0013-harness-nfr-budgets.md) | Harness NFRs are budgets; latency alerts, reliability fails | accepted |
| [0014](0014-make-check-scoped-to-the-change.md) | make check runs what the change can affect; check-all and CI run everything uncached | accepted |
| [0015](0015-anticipated-rules-and-nfr-classes-b-o.md) | Phase 0 rules are adopted as team-convention; NFR classes B and O fail | accepted |
| [0016](0016-tool-supply-chain.md) | Downloaded tools are sha256-pinned; scanners run only as pinned release binaries | accepted |
| [0017](0017-per-worktree-observability-stack.md) | One pinned grafana/otel-lgtm container per worktree is the observability stack (amends 0008) | accepted |
| [0018](0018-http-and-db-instrumentation.md) | HTTP and database instrumentation through otelhttp and otelpgx (amends 0002) | accepted |
| [0019](0019-inferential-sensors-are-advisory.md) | Inferential sensors are advisory; a finding class seen twice becomes a computational rule | accepted |
| [0020](0020-test-coverage-is-a-trend.md) | Test coverage is a trend, never a gate | accepted |
| [0021](0021-telemetry-export-over-otlp-http.md) | Telemetry leaves the process over OTLP/HTTP, one exporter per signal (amends 0002, 0018) | accepted |
| [0022](0022-approved-api-responses.md) | Handler tests pin API responses to approved files, checked against the contract | accepted |
| [0023](0023-mutation-testing-is-a-drift-report.md) | Mutation testing is a weekly drift report, never a gate (extends 0020) | accepted |
| [0024](0024-size-and-complexity-limits-are-maintainability-sensors.md) | Size and complexity limits are maintainability sensors, not style (amends core belief 3) | accepted |
| [0025](0025-browser-e2e-is-the-nfr-b2-sensor.md) | Browser e2e is the NFR-B2 sensor; its browser is installed only where e2e runs | accepted |
| [0026](0026-template-sync-is-a-drift-report.md) | Template sync is a weekly drift report that never applies anything | accepted |
| [0027](0027-token-clones-are-a-maintainability-sensor.md) | Token clones are a maintainability sensor (extends 0024) | accepted |
| [0028](0028-quality-grades-per-domain-and-layer.md) | Quality grades are kept per domain and layer; check-all gates their shape, the weekly drift run their age | accepted |
| [0029](0029-tool-references-follow-the-pin.md) | Tool references are taken from upstream at the pinned version and follow the pin | accepted |
| [0030](0030-one-layout-file-for-every-sensor.md) | The product layout is declared once in layout.env (amends 0002) | accepted |
| [0031](0031-no-orphan-scripts-or-make-targets.md) | No orphan scripts or make targets; dead Go code is gated in check-all | accepted |
| [0032](0032-rules-that-never-fire-are-a-drift-report.md) | Rules that never fire are a drift report; retirement is a human PR (amends 0013) | accepted |
| [0033](0033-stack-profiles.md) | Stack profiles select the sensor groups a product has (amends 0030, 0009, 0023) | accepted |
| 0034 | Unused: the number was never assigned (exec plan 0003, P4.12 kept evasion review in ADR 0019); not renumbered | — |
| [0035](0035-harness-files-under-github-are-named.md) | Harness files under `.github/` are named, not globbed (amends 0010) | accepted |
| [0036](0036-acceptance-scenarios-are-traced-to-tests.md) | Acceptance scenarios are traced to tests by a positional id | accepted |
| [0037](0037-upstream-spec-drift-is-a-drift-report.md) | Cross-repo contracts: vendored upstream specs are a weekly drift report, consumers are named on a break (amends 0030) | accepted |
| [0038](0038-readme-brownfield-adoption-is-probed.md) | README (b) brownfield adoption is probed by a required check (amends 0009, 0026) | accepted |
| [0039](0039-tests-are-written-apart-from-the-implementation.md) | Tests are written apart from the implementation, from the spec (amends 0019) | accepted |
| [0040](0040-ci-agents-may-use-a-subscription-token.md) | CI agents may authenticate with a Claude subscription token (amends 0019) | accepted |
