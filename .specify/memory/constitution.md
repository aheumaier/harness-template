# harness-template Constitution

The rules are not restated here: they live in the repo's guides and are enforced by its sensors.
`/speckit-plan` checks every plan against these principles; a plan that violates one says why in its
Complexity Tracking table, or changes.

## Core Principles

### I. The harness is the constitution
[AGENTS.md](../../AGENTS.md#golden-principles) (golden principles, layer rule) and
[docs/harness/rules.md](../../docs/harness/rules.md) (every rule with its sensor and remediation) are
binding. A plan that needs a rule changed proposes a harness change in its PR; it never works around a sensor.

### II. Contract first
Every API change starts in `api/openapi.yaml`, then `make gen`, then the generated interface is implemented
([ADR 0005](../../docs/adr/0005-spec-first-contract.md)). A plan names the operations and schemas it adds;
its `contracts/` lists them in prose and never holds an OpenAPI document (rules.md#specs-structure).

### III. Layers and domains
Backend code lives in `apps/api/internal/<domain>/{model,store,service,api}`, created with
`make new-domain`; frontend logic in `packages/domain` stays pure ([ARCHITECTURE.md](../../ARCHITECTURE.md#layers)).
A plan maps each requirement to a domain and layer.

### IV. Schema changes are new migrations
Expand, migrate, contract; never edit an applied migration ([add-migration](../../.claude/skills/add-migration/SKILL.md)).

### V. Done means green
Tasks end with `make check`; the feature PR with `make check-all`. Commit subjects are Conventional Commits.

## Governance

This file is a protected harness file: a human amends it (`HARNESS_EDIT=1`, `/speckit-constitution`) in a
harness PR, and only to point at the harness: a new rule goes
into rules.md with a sensor and fixture first. Specs live in `specs/NNN-slug/` (product features only;
harness and infra work goes in `docs/exec-plans/`, [ADR 0011](../../docs/adr/0011-conventional-commits-and-spec-kit.md)).

**Version**: 1.0.0 | **Ratified**: 2026-10-27 | **Last Amended**: 2026-10-27
