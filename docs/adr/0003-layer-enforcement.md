# ADR 0003: Layer enforcement — go-arch-lint, depguard, custom analyzers

Status: accepted · 2026-10-27

## Context
Layer rules must be deterministic sensors, not prose. Tools differ in what they can express:
go-arch-lint maps packages to components via globs and checks edges between components, but a glob such as
`internal/*/store/**` cannot express "the same domain", so `foo/service → bar/store` looks like a legal
`service → store` edge.

## Decision
- **go-arch-lint** (v3 config): the layer graph (`model → store → service → api`, platform imports no
  domain). `ignoreNotFoundComponents: true` because a fresh template has zero domains.
- **depguard** (golangci-lint v2): third-party/stdlib restrictions — pgx/database/sql only in store and
  platform; net/http only in api, platform, cmd.
- **Custom analyzers** in `apps/api/tools/analyzers`, built into `.bin/golangci-lint-custom` as a module
  plugin: `noctxroot` (seed rule from the brief) and `domainiso` (cross-domain store/api imports — the edge
  go-arch-lint cannot express). The brief asked for one analyzer; the second is the cheapest deterministic
  sensor for a rule the brief also asked for.
- Every diagnostic embeds the rule id and remediation, and links `docs/harness/rules.md#<id>`.

## Consequences
- The custom golangci binary is rebuilt when the analyzers or `.custom-gcl.yml` change (Make dependency).
- Adding a domain needs no config edits: globs pick it up.
