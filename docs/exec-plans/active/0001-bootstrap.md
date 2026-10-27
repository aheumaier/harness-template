# 0001 — Bootstrap the agent harness

Status: active · Owner: @aheumaier · Started: 2026-10-27

## Goal

Template repo `harness-template` (Go module root `github.com/aheumaier/harness-template`) whose value is
the harness: guides (feedforward) + sensors (feedback). No business domains.

## Phases

| # | Phase | Exit criterion | State |
|---|-------|----------------|-------|
| 0 | git init, exec plan, tool pinning, `make check` skeleton, run.sh | `make check` green | done |
| 1 | Go backend skeleton, platform, /healthz, depguard, go-arch-lint, custom analyzer, govulncheck, race tests | `make check` green | done |
| 2 | Atlas migrations, migrate lint, squawk, sqlc, schema doc | `make check` green | done |
| 3 | OpenAPI 3.1, oapi-codegen strict server, Spectral, oasdiff | `make check` green | done |
| 4 | pnpm + Turbo, web app, api-client (orval+zod), dep-cruiser, ESLint, knip, Vitest | `make check` green | done |
| 5 | setup-worktree.sh, dev/db-up/db-reset, per-worktree ports | two worktrees up simultaneously | done |
| 6 | check / check-all / gen-check, fixtures, harness-selftest | selftest green | done |
| 7 | new-domain generator + skill + CI probe | probe + `make check` green | done (CI probe: phase 10) |
| 8 | AGENTS.md, nested guides, ARCHITECTURE.md, docs | links resolve, AGENTS ≤100 lines | done |
| 9 | .claude settings, hooks, subagents, skills | hooks tested | done (protection hook: last) |
| 10 | GitHub workflows, CODEOWNERS, PR template, branch-protection script | YAML valid | done (actionlint; not run on GitHub) |

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
| ESLint / typescript-eslint | 10.13.0 / **8.72.0** | flat config; 8.72.1 rejected by pnpm minimumReleaseAge |
| eslint-plugin-react-hooks / @eslint/js | 7.1.1 / 10.0.1 | |
| orval / zod | 8.45.0 / 4.6.7 | |
| dependency-cruiser / knip | 18.7.0 / 6.43.0 | |
| shellcheck / shfmt | v0.11.0 / v3.14.1 | shellcheck release tarball; shfmt via `go install` |
| bats-core | 1.13.0 | npm `bats` (root devDependency) |
| actionlint | v1.7.12 | `go install`; runs shellcheck on `run:` blocks |
| Node (CI) | 24.9.0 | `NODE_VERSION` for actions/setup-node; locally any Node matching `engines` |
| GitHub Actions | checkout v7.0.1, setup-go v7.0.0, setup-node v7.1.0, cache v6.1.0 | pinned to commit SHAs |

## Open assumptions

- A1: Go module lives in `apps/api` as `github.com/aheumaier/harness-template/apps/api` (repo-path-correct
  module path for a monorepo subdir). Analyzer is a nested module so plugin deps stay out of the app.
- A2: oapi-codegen `include-tags` lets us generate one strict server per domain. **Confirmed** (v2.8.1, ADR 0005).
- A3: go-arch-lint glob components tolerate zero matches. **Confirmed** with `ignoreNotFoundComponents: true`.
- A4: sqlc rejects zero packages and empty query files. **Resolved:** `make gen` skips sqlc without domains; generator emits one trivial query.
- A5: Docker is available locally and in CI (Atlas dev DB, Postgres for integration tests).
- A6: Agent-file `tools:` cannot restrict Bash to one command; restrict via a per-agent PreToolUse hook. Confirmed: `only-make-check.sh` via agent frontmatter `hooks:`.
- A7: "rm -rf outside the repo" cannot be fully expressed as a deny pattern; covered by deny patterns + PreToolUse hook. Resolved: `guard-bash.sh` (heredoc bodies ignored).

## Deviations from prompt

- TypeScript 6.0.3 instead of latest 7.0.2 (peer-dep constraint, see table).
- Atlas **Community Edition** build: official binary gates `migrate lint` behind Pro login since v0.38.
  BC analyzers are Pro-only; renames are caught as DS103 destructive + squawk `renaming-column` (ADR 0004).
- pnpm installed into `.bin/.pnpm` via npm (corepack/pnpm self-switch to 12.x failed with ENOEXEC).
- Second custom analyzer `domainiso`: go-arch-lint globs cannot express "same domain only" (ADR 0003).
- Auth stub is fail-closed: requires explicit `AUTH_MODE=dev-stub` (security review finding).
- golang.org/x/text bumped to v0.43.0 (govulncheck GO-2026-6629/-5970 via pgx).
- macOS make 3.81: no `--output-sync`, `.SHELLFLAGS` ignored; recipes use explicit `set -e` where needed.
- typescript-eslint 8.72.0 (8.72.1 younger than pnpm 12's default minimumReleaseAge; policy kept).
- No TS formatter (Prettier) added: not requested; `make check` "format" covers Go only (ADR 0007).
- Spectral built-ins `operation-operationId`/`operation-tags` replaced by custom rules (rule id in message).
- harness-selftest links node_modules/.bin into the temp copy instead of copying them (305 MB + 381 MB).
- Generator stub goes beyond "stubs": one `GET /<kebab>/status` operation + `Ping` query wire every layer, so
  oapi-codegen, sqlc and the route are exercised (empty tags/query files are rejected by the tools, A4).
- Added sensor `docs-links` (`check-links.sh`, in `make check`) so "all docs links resolve" is enforced, not
  checked once; anchors use GitHub heading slugs.
- `squawk` runs via `scripts/harness/squawk.sh`: files with `-- atlas:txmode none` are checked as
  non-transactional (otherwise `CREATE INDEX CONCURRENTLY` could never pass). Verified with Atlas apply.
- Added `make db-migrate`, `docs/SECURITY.md`, `docs/RELIABILITY.md` (referenced by guides).
- The "empty" migration holds only the lock/statement timeout header squawk requires.
- Shell is treated as code (owner request during phase 9): rules `sh-shellcheck`, `sh-format`, `sh-tests-pass`;
  `.shellcheckrc` (SC2016 off: single-quoted awk/sed/Go templates), `.editorconfig` (shfmt style), bats tests in
  `scripts/test/` for hooks, run.sh, check-links.sh, squawk.sh and new-domain name validation. Existing scripts
  reformatted with shfmt; `# shellcheck disable` follows the suppression format of golden principle 4.
- Protected harness paths are one list, `scripts/harness/protected-paths.txt` (CODEOWNERS mirrors it). Beyond
  the prompt's list it adds `.custom-gcl.yml`, `.squawk.toml`, `.shellcheckrc`, `.editorconfig`, `knip.json`,
  `tsconfig.base.json`, `apps/api/tools/analyzers/`, `docs/harness/`, `CLAUDE.md` and `Makefile`: each one
  either configures a sensor or is the sensor's entry point.
- Added sensors `ci-workflows-valid` (actionlint, in `make check`), `harness-integrity` and
  `migrations-append-only` (in `make check-all`); the integrity and append-only checks are scripts so CI and
  local runs are identical. Config-loosening detection is a keyword heuristic (TD-2).
- `harness-change-approved` PR label waives only the config check (human decision, documented in ci.md).
- `drift.yml` agent-GC stub is gated by repository variable `AGENT_GC_ENABLED` (actionlint rejects `if: false`).
- `.claude/settings.json` keeps the deny rule `Bash(rm -rf /*)` (reviewer suggested dropping it as redundant
  with guard-bash; the agent's own removal was refused by the session's permission classifier: left to a human).
- guard-bash hardened after review: fails closed on unparsable input, resolves wrappers (`sudo`, `env`,
  `xargs`, …), line continuations, here-strings, `HEAD`/`@` push targets and `.env` reads through any command.
- Generator inserts at `harness:*` marker comments in openapi.yaml, routes.go and QUALITY.md; refuses dirty files.

## Log

- 2026-10-27: phase 10 done: 5 workflows + composite setup action, CODEOWNERS, PR template,
  branch-protection script (dry run by default; not run). New sensors: actionlint, harness-integrity,
  migrations-append-only; 4 fixtures. Workflows validated with actionlint only; never executed on GitHub.
- 2026-10-27: phase 9 done: settings (deny rules), fast-check PostToolUse hook (now also shell), guard-bash
  PreToolUse hook (first version parsed a commit-message heredoc as commands and blocked its own commit; fixed,
  regression test added), reviewer subagents. Added shellcheck/shfmt/bats: 22 tests, 2 fixtures; mutation of
  run.sh made its test fail. `make check` 12.7 s warm.
- 2026-10-27: phase 8 done: AGENTS.md 70 lines, 4 nested AGENTS.md, CLAUDE.md, ARCHITECTURE.md, core beliefs;
  add-endpoint/add-migration skills written early because guides link them.
- 2026-10-27: phase 7 done: `make new-domain NAME=zzz_probe` → `make check` (~6 s) and `check-all` green, endpoint
  served 200; two domains in one tree (FORCE=1) green; invalid/reserved/duplicate names rejected.
- 2026-10-27: phase 6 done: 18 fixtures, `make harness-selftest` green (~20 s). First run passed the Atlas
  fixtures for the wrong reason (atlas.sum mismatch, `set -e` ignored inside an `&&` chain); EXPECT now pins the
  tool's own diagnostic code (DS102/DS103). `deadcode` exits 0 on findings, so `deadcode-go` fails on output.
- 2026-10-27: phase 5 done: two worktrees ran `make dev` simultaneously (distinct ports, separate Postgres).

- 2026-10-27: inputs fixed (name `harness-template`, owner `@aheumaier`). git initialised.
- 2026-10-27: phase 4 done (orval forceSuccessResponse avoids zod-parsing error bodies as success).
- 2026-10-27: phases 0–3 done; each sensor verified to fire on a hand-made violation.
