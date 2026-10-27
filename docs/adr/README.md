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
