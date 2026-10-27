# apps/api/db

- `migrations/` is append only. Create files with `cd apps/api && atlas migrate new <name> --env local`
  (or the `add-migration` skill), never by hand, and never edit or delete a migration that is on `main`.
- Every migration starts with `SET lock_timeout` / `SET statement_timeout` and creates indexes
  `CONCURRENTLY` in a file headed `-- atlas:txmode none` (squawk). Drops and renames are destructive: expand → backfill → contract over several PRs.
- After changing migrations run `make gen` (rehashes `atlas.sum`, regenerates sqlc) and `make gen-schema-doc`.
- `queries/<domain>.sql` holds that domain's sqlc queries (`-- name: X :one|:many|:exec`). Output goes to
  `internal/<domain>/store`; never edit it.
- Current schema: [docs/generated/db-schema.md](../../../docs/generated/db-schema.md) (generated, do not edit).
- Sensors: `make lint-migrations squawk` (in `check-all`) and the migration-gate workflow.
