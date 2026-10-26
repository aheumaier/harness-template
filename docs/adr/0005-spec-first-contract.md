# ADR 0005: Spec-first contract, one strict server per tag, RFC 9457 errors

Status: accepted · 2026-10-27

## Context
Backend and frontend are written by agents in parallel. They need one source of truth for the HTTP
contract that generates both sides, so drift is a build error rather than a runtime surprise. The domain
generator must be able to add paths without editing a shared, ever-growing server interface.

## Decision
- `api/openapi.yaml` (OpenAPI 3.1) is the contract. Change it first, run `make gen`, then implement.
- **oapi-codegen v2.8.1**, `std-http-server` + `strict-server`, configured per package by an
  `oapi-codegen.yaml` with `output-options.include-tags: [<tag>]`. Each tag is one domain and yields one
  generated strict server in `internal/<domain>/api` (`platform` → `internal/platform/health`). `make gen-oapi`
  runs every such config. Unimplemented operations of a tag are a compile error in that package only.
- Errors are RFC 9457 `application/problem+json` (`components/schemas/Problem`, shared responses
  `ClientError`, `ServiceUnavailable`, `ServerError` using `4XX`/`5XX` ranges). `internal/platform/problem`
  writes them; strict-server request/response error hooks use it, so internals never leak.
- **Spectral 6.17.0** (`.spectral.yaml`) extends `spectral:oas` recommended and adds seed rules: kebab-case
  paths, camelCase properties, operationId + exactly one tag, documented 4xx and 5xx, errors only as
  problem+json. Built-in `operation-operationId`/`operation-tags` are replaced by custom rules so every
  message carries a rule id and a fix.

## Consequences
- Generated code is committed and guarded by `gen-check`; nobody edits `*.gen.go` or the api-client.
- A tag with no operations would generate an empty package; the generator creates the tag and a path stub together.
