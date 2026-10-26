# ADR 0004: Migrations with Atlas (Community Edition), squawk and sqlc

Status: accepted · 2026-10-27

## Context
Schema changes are the riskiest thing an agent can write. We want versioned, reviewable SQL, a linter that
understands data loss and compatibility, a linter that understands Postgres locks, and type-safe queries.
Since Atlas v0.38, `atlas migrate lint` in the official binary requires an Atlas Pro login.

## Decision
- **Atlas v1.3.0 Community Edition** (`atlas-community-*` build, open source): versioned migrations in
  `apps/api/db/migrations`, `atlas.sum` integrity file, `docker://postgres/17/dev` dev database, `migrate lint`
  with `destructive` and `incompatible` analyzers at error level. CE limitation: the backward-incompatible
  (BC) analyzers are Pro-only; renames surface as destructive drops (DS103) and squawk's `renaming-column`.
- **squawk 2.70.0** (`.squawk.toml`, `assume_in_transaction = true` because Atlas wraps each file in a
  transaction) for lock safety. All default rules on.
- **sqlc 1.31.1**, pgx/v5 driver, one `sql:` entry per domain (appended by the generator), schema read from
  the migrations directory. sqlc refuses an empty package list and empty query files, so `make gen` skips
  sqlc when no domain exists and the generator emits one trivial query per domain.
- `docs/generated/db-schema.md` is produced by `atlas schema inspect` (mermaid ER + SQL) and stamped with the
  `atlas.sum` hash, giving an **offline** staleness check in `make check`; regeneration needs Docker and runs
  in `make gen-schema-doc` / CI.

## Consequences
- No Atlas Cloud account needed. If the team adopts Atlas Pro, switch the download URL and enable BC analyzers.
- Docker is required for migration lint and schema-doc regeneration (check-all, CI), not for `make check`.
