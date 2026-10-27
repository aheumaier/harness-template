---
name: add-endpoint
description: Add or change an HTTP operation spec-first (OpenAPI → make gen → implement → make check). Use whenever an API endpoint, request/response field or error response is added, changed or removed.
---

# Add or change an endpoint (spec first)

1. Find the owning domain's tag in `api/openapi.yaml`. No domain yet → use the `new-domain` skill first.
2. Edit the contract only:
   - path kebab-case (`/order-items/{orderItemId}`), properties camelCase, unique camelCase `operationId`;
   - exactly one tag (the domain);
   - errors: `'4XX': {$ref: '#/components/responses/ClientError'}`, `'5XX': {$ref: '#/components/responses/ServerError'}`,
     plus specific codes the client must handle (404, 409, …) as `application/problem+json`;
   - reuse schemas under `components/schemas`; keep the `harness:domain-*` markers.
   Changing or removing existing fields/paths is a breaking change: add the new shape, deprecate the old
   one (`deprecated: true`), and remove it in a later PR (contract-gate, `make contract-check`).
3. `make lint-api` until clean, then `make gen`. This regenerates `internal/<domain>/api/api.gen.go` and
   `packages/api-client/src/gen/`. Never edit those files.
4. The build now fails until the new `StrictServerInterface` method exists: implement it in
   `internal/<domain>/api/api.go`, delegating to a `service` method (add one if needed, with its unit test).
   Map service errors to the problem responses you declared; do not invent undeclared status codes.
5. Data access: add a query to `apps/api/db/queries/<domain>.sql` and `make gen` (schema changes: `add-migration` skill).
6. Frontend: use the generated hook (`use<OperationId>`) from `@harness-template/api-client`; show errors via `problemOf`.
7. `make check`, then `make check-all` before the PR. Tick "contract change: Y" in the PR template.
