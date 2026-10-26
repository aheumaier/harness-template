# 0001 — Bootstrap the agent harness

Status: active · Owner: @aheumaier · Started: 2026-10-27

## Goal

Template repo `harness-template` (Go module root `github.com/aheumaier/harness-template`) whose value is
the harness: guides (feedforward) + sensors (feedback). No business domains.

## Phases

| # | Phase | Exit criterion | State |
|---|-------|----------------|-------|
| 0 | git init, exec plan, tool pinning, `make check` skeleton, run.sh | `make check` green | todo |
| 1 | Go backend skeleton, platform, /healthz, depguard, go-arch-lint, custom analyzer, govulncheck, race tests | `make check` green | todo |
| 2 | Atlas migrations, migrate lint, squawk, sqlc, schema doc | `make check` green | todo |
| 3 | OpenAPI 3.1, oapi-codegen strict server, Spectral, oasdiff | `make check` green | todo |
| 4 | pnpm + Turbo, web app, api-client (orval+zod), dep-cruiser, ESLint, knip, Vitest | `make check` green | todo |
| 5 | setup-worktree.sh, dev/db-up/db-reset, per-worktree ports | two worktrees up simultaneously | todo |
| 6 | check / check-all / gen-check, fixtures, harness-selftest | selftest green | todo |
| 7 | new-domain generator + skill + CI probe | probe + `make check` green | todo |
| 8 | AGENTS.md, nested guides, ARCHITECTURE.md, docs | links resolve, AGENTS ≤100 lines | todo |
| 9 | .claude settings, hooks, subagents, skills | hooks tested | todo |
| 10 | GitHub workflows, CODEOWNERS, PR template, branch-protection script | YAML valid | todo |

## Tool and version choices (pinned, verified by running)

Single source: `scripts/tool-versions.env` (Go/binary tools) and `package.json` files (JS, exact versions).

| Tool | Version | Notes |
|------|---------|-------|
| Go toolchain | go1.27.3 | `GOTOOLCHAIN` exported by Makefile; local go1.25.6 auto-downloads |
| golangci-lint | v2.15.0 | v2 config; module plugin via `golangci-lint custom` |
| go-arch-lint | v1.19.0 | |
| govulncheck | v1.9.0 | |
| deadcode (x/tools) | v0.52.0 | |
| sqlc | v1.31.1 | pgx/v5 driver |
| oapi-codegen | v2.8.1 | strict-server, std-http |
| oasdiff | v1.34.1 | |
| Atlas | v1.3.0 | binary from release.ariga.io |
| squawk | 2.70.0 | npm `squawk-cli` |
| Spectral | 6.17.0 | npm `@stoplight/spectral-cli` |
| pgx | v5.11.0 | |
| OpenTelemetry Go | v1.47.0 | |
| Node | ≥24 (local 24.9.0) | |
| pnpm | 12.15.0 | `packageManager` field |
| Turborepo | 2.11.10 | |
| TypeScript | **6.0.3** | deviation: 7.0.2 is latest but typescript-eslint 8.72.1 peer is `<6.1.0` |
| Vite / plugin-react / Vitest | 8.3.5 / 6.1.2 / 5.0.5 | |
| React / TanStack Query | 19.3.1 / 5.105.0 | |
| ESLint / typescript-eslint | 10.13.0 / 8.72.1 | flat config |
| orval / zod | 8.45.0 / 4.6.7 | |
| dependency-cruiser / knip | 18.7.0 / 6.43.0 | |

## Open assumptions

- A1: Go module lives in `apps/api` as `github.com/aheumaier/harness-template/apps/api` (repo-path-correct
  module path for a monorepo subdir). Analyzer is a nested module so plugin deps stay out of the app.
- A2: oapi-codegen `include-tags` lets us generate one strict server per domain (needed so the generator
  can add a path without breaking the build). Verify in Phase 3.
- A3: go-arch-lint glob components tolerate zero matches (no domains exist). Verify in Phase 1.
- A4: sqlc tolerates an empty query set per domain; else generator emits a trivial query. Verify Phase 2/7.
- A5: Docker is available locally and in CI (Atlas dev DB, Postgres for integration tests).
- A6: Agent-file `tools:` cannot restrict Bash to one command; restrict via a per-agent PreToolUse hook. Verify Phase 9.
- A7: "rm -rf outside the repo" cannot be fully expressed as a deny pattern; covered by deny patterns + PreToolUse hook.

## Deviations from prompt

- TypeScript 6.0.3 instead of latest 7.0.2 (peer-dep constraint, see table).

## Log

- 2026-10-27: inputs fixed (name `harness-template`, owner `@aheumaier`). git initialised.
