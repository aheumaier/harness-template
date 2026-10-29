---
name: review-modularity
description: Read-only review of module boundaries across the whole repository (responsibilities in the wrong layer, one concept implemented twice, coupling the layer linters cannot express). Advisory findings as file:line — problem — fix. Use for the weekly scheduled review (drift.yml) or when asked whether the codebase's structure still holds.
---

# Modularity review

A read-only review of the repository as it is, not of one diff: read files only; change, create or run
nothing. Its findings are advisory (ADR 0019): a human decides; a finding class accepted twice becomes a
computational rule. Every path below is relative to the repository root.

## Skip

- What a sensor already catches on every PR (`docs/harness/rules.md`): `go-layers`, `go-domain-isolation`,
  `go-db-only-in-store`, `go-http-only-in-api`, `go-test-helpers-only-in-tests`, `web-no-direct-http`,
  `web-domain-pure`, `web-no-cross-app`, `go-no-dead-code`, `web-no-dead-code`.
- Generated code and fixtures: `scripts/harness/fixtures/` (deliberately broken examples), `.specify/`,
  `*.gen.go`, `apps/api/internal/*/store/` and `packages/api-client/src/gen/`.

## Look for

1. **The architecture checklist, repository-wide.** Read `.claude/agents/architecture-reviewer.md` and apply
   its list (what the import linters cannot see) to the whole repository instead of a diff, reading the
   documents it names first. Its output format does not apply; use the one below.
2. **One concept twice.** The same validation, mapping, constant or type written in two places (the Go and
   TS sides of one contract excepted when both are generated from `api/openapi.yaml`); a hand-written type
   that duplicates a generated one.
3. **Growth without structure.** A package or file doing several unrelated things, a `utils` or `common`
   catch-all, a domain that others reach into through more than its `service` and `model`.

## Never

- Never read `.env`, `.env.local` or any other env file.
- Never run code or reach the network.

## Output

One line per finding, most severe first, at most 15:

`path:line — problem — fix` (severity high, medium or low)

Report only what a reader can verify at that line. No finding is a valid result; say so in one line.
