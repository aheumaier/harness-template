# ADR 0008: One isolated environment per git worktree

Status: accepted · 2026-10-27

## Context
Agents work in parallel git worktrees. Shared databases or fixed ports make runs interfere: one agent's
migration or test data breaks another's tests, and `make dev` fails with "address already in use".

## Decision
- `make setup` (`scripts/setup-worktree.sh`) derives a compose project `harness-<dir>-<hash8>` and a port block
  (`DB`, `API`, `WEB` = base, +1, +2; base = 20000 + (hash mod 2000) × 10) from the worktree path, skips blocks
  that are listening or claimed by another worktree's `.env.local`, and writes `.env.local` (gitignored).
- Each worktree gets its own Postgres container and volume (`compose.yaml`, image pinned in
  `scripts/tool-versions.env`); `make dev`, `db-up`, `db-reset` and `test-integration` source `.env.local`.
- Re-running `make setup` reuses the existing `.env.local`; `SKIP_DB=1` sets up tools only.

## Consequences
- Verified at bootstrap: two worktrees ran `make dev` simultaneously on distinct ports and databases.
- A removed worktree leaves its container/volume behind; clean up with `make db-reset`'s `down -v` before
  `git worktree remove`, or `docker compose -p <project> down -v`.
