# AGENTS.md

`harness-template` is a template repository for products built by coding agents: a Go HTTP API
(`apps/api`), a React web app (`apps/web`), a shared OpenAPI contract (`api/openapi.yaml`) and Postgres.
It has no business domains yet. Its value is the **harness**: guides that tell you what good looks like
(this file, [ARCHITECTURE.md](ARCHITECTURE.md), [docs/](docs/)) and sensors that fail with a remediation
when code drifts ([docs/harness/rules.md](docs/harness/rules.md)).

## Commands

Everything runs through `make` (`make help` lists all targets). Tools are pinned in `.bin/`; never install
global versions.

| Command | When |
|---------|------|
| `make setup` | Once per clone/worktree: tools, JS deps, this worktree's own Postgres + ports (`.env.local`). |
| `make check` | After every change. Fast (<60 s warm): format, lint, layers, unit tests, generated code up to date. |
| `make check-all` | Before you open a PR: `check` + migrations, contract, harness integrity, commit messages, specs, vulnerabilities, dead code, integration tests. |
| `make gen` | After editing `api/openapi.yaml`, `apps/api/db/queries/*.sql` or migrations. Commit the output. |
| `make new-domain NAME=billing` | To start a new business domain. Never create `apps/api/internal/<domain>/` by hand. |

Also: `make dev` (API + web for this worktree), `make db-reset`, `make fmt`, `make harness-selftest`.

## Layer rule

Each backend domain is `internal/<domain>/{model,store,service,api}` and depends one way only:
`model ← store ← service ← api`; `platform` is shared plumbing and imports no domain.
Another domain may use your `service` or `model`, never your `store` or `api`.

The frontend mirrors this: `packages/domain` is pure TS (no React, no browser APIs); only
`packages/api-client` talks HTTP; apps never import each other. Details: [ARCHITECTURE.md](ARCHITECTURE.md#layers).

## Golden principles

1. **Contract first.** Change `api/openapi.yaml`, run `make gen`, then implement the generated interface.
   Never hand-edit generated code (`*.gen.go`, `internal/*/store/`, `packages/api-client/src/gen/`).
2. **Parse at boundaries.** Input is validated where it enters (generated strict server, zod schemas);
   inner code receives typed values. Errors leave the API as RFC 9457 `application/problem+json`.
3. **Schema changes are new migrations.** Expand, migrate, contract; never edit an applied migration;
   never drop or rename in one step ([.claude/skills/add-migration](.claude/skills/add-migration/SKILL.md)).
4. **Fix the code, not the sensor.** A red sensor prints the rule and its remediation; follow it. A
   suppression (`//nolint`, `eslint-disable`, `@ts-expect-error`, `# shellcheck disable`) needs `reason: … (rule: <id>)` on the same
   line, and `@ts-ignore` is never allowed.
5. **Harness files are not yours to change.** Linter configs, `scripts/harness/**`, `.claude/**`,
   `.github/**` and this file are protected. If a rule is wrong, propose the change in the PR description.
6. **Done means green.** `make check` before you say a task is done; `make check-all` before a PR.
7. **The repo is the system of record.** Decisions go in [docs/adr/](docs/adr/README.md); product features
   are specified in `specs/NNN-slug/` with spec-kit (`/speckit-specify` → `/speckit-plan` → `/speckit-tasks`);
   harness and infra work is planned in [docs/exec-plans/](docs/exec-plans/); known debt in
   [docs/exec-plans/tech-debt-tracker.md](docs/exec-plans/tech-debt-tracker.md).

## Where to look

| Need | Read |
|------|------|
| Codemap, layers, request flow | [ARCHITECTURE.md](ARCHITECTURE.md) |
| Every rule, why, its sensor, how to fix | [docs/harness/rules.md](docs/harness/rules.md) |
| Why the repo is built this way | [docs/design-docs/core-beliefs.md](docs/design-docs/core-beliefs.md), [docs/adr/](docs/adr/README.md) |
| Current database schema (generated) | [docs/generated/db-schema.md](docs/generated/db-schema.md) |
| Domain health | [docs/QUALITY.md](docs/QUALITY.md) |
| CI checks, required status checks, label bypass | [docs/harness/ci.md](docs/harness/ci.md) |
| Security and reliability defaults | [docs/SECURITY.md](docs/SECURITY.md), [docs/RELIABILITY.md](docs/RELIABILITY.md) |
| Area-specific rules | `AGENTS.md` in [apps/api/internal/](apps/api/internal/AGENTS.md), [apps/api/db/](apps/api/db/AGENTS.md), [api/](api/AGENTS.md), [apps/web/](apps/web/AGENTS.md); each has a sibling `CLAUDE.md` that is just `@AGENTS.md` |
| Step-by-step recipes | [new-domain](.claude/skills/new-domain/SKILL.md), [add-endpoint](.claude/skills/add-endpoint/SKILL.md), [add-migration](.claude/skills/add-migration/SKILL.md); spec-kit's `speckit-*` skills for specs ([ADR 0011](docs/adr/0011-conventional-commits-and-spec-kit.md)) |

## Working agreements

- Each git worktree is a fully isolated environment (own DB, own ports): run `make setup` in it first.
- Keep changes small and in one domain where possible; a PR states contract change, migration and harness
  change (Y/N) in its template.
- Commit subjects and PR titles are [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/)
  (`feat(billing): add invoice export`); the `commit-msg` hook from `make setup` checks each commit. A `feat`
  touching `apps/` or `api/` ships with its spec in `specs/`.
- When a sensor is missing for a mistake you or a reviewer caught, add the rule to rules.md with a fixture in
  `scripts/harness/fixtures/` (via a human-approved harness PR) rather than a note in a guide.
