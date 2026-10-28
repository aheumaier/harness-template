---
name: new-domain
description: Create a new backend business domain (Go layers, SQL queries, migration, OpenAPI tag, generated client hook). Use when the user asks for a new domain, bounded context, module or resource family that has no apps/api/internal/<name> directory yet.
---

# New domain

Never hand-create `apps/api/internal/<name>/`. The generator wires every layer and sensor consistently.

1. Pick `NAME`: lowercase, `[a-z][a-z0-9_]*`, singular noun for the business capability (`billing`, `order_items`).
   It becomes the Go directory, the OpenAPI tag, the sqlc entry and the URL prefix (`_` → `-`).
2. Start from a clean tree for `api/openapi.yaml`, `apps/api/cmd/api/routes.go`, `apps/api/sqlc.yaml` and
   `docs/QUALITY.md` (commit first; `FORCE=1` only when creating several domains in one change).
3. Run `make new-domain NAME=<name>` (needs Docker for the schema doc; otherwise run `make gen-schema-doc` after).
4. Run `make check` and `make test-integration`. Both must be green before you change anything else: the
   domain starts with a service test (fake store), a handler test (strict server + `httptest`) and an
   integration test (`-tags=integration`, real Postgres) for the stub.
5. Replace the stubs, in this order, running `make check` after each:
   - contract: real operations/schemas for the tag in `api/openapi.yaml`, delete `GET /<kebab>/status`
     (see `.claude/skills/add-endpoint`);
   - schema: tables in the generated migration (see `.claude/skills/add-migration`);
   - queries in `apps/api/db/queries/<name>.sql` (delete `Ping`), `make gen`; add each query the service
     calls to its unexported `queries` interface (tests fake it; `*store.Queries` must still satisfy it);
   - `service` use cases, then `api` handlers; replace the generated tests with tests of the real
     operations, keeping one of each kind (`make test-integration` for the integration one).
6. Update the domain's `AGENTS.md` (5–10 lines, what is non-obvious) and its row in `docs/QUALITY.md`.

Undo an unwanted run before committing:
`git checkout -- . && git clean -fd -- apps/api api docs packages/api-client`
