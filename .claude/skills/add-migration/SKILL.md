---
name: add-migration
description: Change the Postgres schema with a new Atlas migration that passes Atlas lint and squawk, then regenerate sqlc code and the schema doc. Use for any table, column, index or constraint change.
---

# Add a migration

Migrations are append only. Never edit or delete one that is on `main`; write a new one.

1. `cd apps/api && atlas migrate new <short_snake_name> --env local` (creates `db/migrations/<ts>_<name>.sql`).
2. Write the SQL. Start every file with:
   ```sql
   SET lock_timeout = '2s';
   SET statement_timeout = '30s';
   ```
   - New index on an existing table: its own file, with Atlas's no-transaction directive and a blank line
     after it (squawk then checks the file as non-transactional):
     ```sql
     -- atlas:txmode none

     SET lock_timeout = '2s';
     SET statement_timeout = '30s';
     CREATE INDEX CONCURRENTLY IF NOT EXISTS orders_customer_id_idx ON orders (customer_id);
     ```
   - New `NOT NULL` column on an existing table: add nullable (or with a constant default), backfill, then
     add the constraint `NOT VALID` and `VALIDATE` it in a later migration.
   - Rename or drop (column, table): never in one step. Expand (add new), migrate code and data, contract
     (drop old) in a later PR once nothing reads it. Atlas flags drops/renames as destructive (DS1xx).
3. `make gen` (rehashes `atlas.sum`, regenerates sqlc store code), then `make gen-schema-doc` (needs Docker).
4. `make lint-migrations squawk`. Fix findings by changing the SQL, not the config.
5. `make db-migrate` to apply to this worktree's database; update queries in `db/queries/<domain>.sql` and
   `make gen` again if needed.
6. `make check`, then `make check-all`. Tick "migration: Y" in the PR template.
