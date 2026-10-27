# api

- `openapi.yaml` (OpenAPI 3.1) is the contract for server and client. Change it first, then `make gen`.
- One tag per domain (the generator adds it); every operation has `operationId` (camelCase), exactly one tag
  and documented `4XX`/`5XX` responses using the shared `Problem` responses.
- Paths are kebab-case, properties camelCase, errors `application/problem+json` (Spectral, `make lint-api`).
- Breaking changes (removing or renaming paths, parameters, properties; tightening types) fail the
  contract-gate (`make contract-check`). Prefer additive changes; deprecate before removing.
- Keep the `# harness:domain-*` marker comments: `make new-domain` inserts there.
- Recipe: [.claude/skills/add-endpoint](../.claude/skills/add-endpoint/SKILL.md).
