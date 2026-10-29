---
name: review-data-handling
description: Read-only review of how the repository handles data (personal data and secrets in logs, traces and metrics, validation at boundaries, retention, error detail). Advisory findings as file:line — problem — fix. Use for the weekly scheduled review (drift.yml) or when asked how the codebase treats user data.
---

# Data-handling review

A read-only review of the repository as it is, not of one diff: read files only; change, create or run
nothing. Its findings are advisory (ADR 0019): a human decides; a finding class accepted twice becomes a
computational rule. Every path below is relative to the repository root.

## Skip

- What a sensor already catches on every PR (`docs/harness/rules.md`): `go-structured-logging`,
  `go-db-only-in-store`, `repo-no-secrets`, `api-errors-problem-json`, `api-camel-case-properties`,
  `db-no-destructive-migration`.
- Generated code and fixtures: `scripts/harness/fixtures/` (deliberately broken examples), `.specify/`,
  `*.gen.go`, `apps/api/internal/*/store/` (read its source, `apps/api/db/queries/`) and
  `packages/api-client/src/gen/`.

## Look for

1. **Telemetry that carries data it should not.** Log fields, span attributes, metric labels and baggage in
   `apps/api` and `apps/web` that hold personal data (names, emails, addresses, client IPs where not
   needed), tokens, request or response bodies, or unbounded values (ids as metric labels blow up
   cardinality). Check `platform/logging`, `platform/accesslog`, `platform/telemetry` and every call site
   that adds attributes.
2. **Validation at boundaries.** Data entering through the API, the database or the browser is parsed where
   it enters (golden principle 2: generated strict server, zod schemas). Report fields the contract leaves
   unconstrained (no `maxLength`, `format`, `pattern` or enum where one is obvious) and frontend code that
   uses an API response without its zod schema.
3. **Storage.** Columns that hold personal data without a note on retention or deletion, data kept in more
   places than needed (a copy in a cache, a log or an event), queries that select more columns than the
   caller uses.
4. **Data leaving the system.** Error responses, exports and client-side storage (`localStorage`, cookies)
   that expose more than the caller needs.
5. **Docs against code.** What `docs/SECURITY.md` and `docs/RELIABILITY.md` say about data, compared with
   what the code does.

## Never

- Never read `.env`, `.env.local` or any other env file, and never print a secret or personal value you come
  across: name the file and line only. The configuration's shape is in `apps/api/internal/platform/config`
  and `compose.yaml`; the schema is in `docs/generated/db-schema.md`.
- Never run code, query a database or reach the network.

## Output

One line per finding, most severe first, at most 15:

`path:line — problem — fix` (severity high, medium or low)

Report only what a reader can verify at that line. No finding is a valid result; say so in one line.
