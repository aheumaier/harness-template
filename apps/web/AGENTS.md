# apps/web

- Vite + React 19 + TanStack Query. Data comes only from hooks in `@harness-template/api-client`
  (generated from `api/openapi.yaml`); no `fetch`, `axios` or `XMLHttpRequest` here (ESLint).
- Show API errors with `problemOf(error)`; it returns the parsed RFC 9457 problem.
- Domain logic without React (calculations, validation, formatting rules) goes in `packages/domain`, which
  may not import React, browser APIs or apps (dependency-cruiser).
- Never import from another app (`apps/*`); share code through `packages/*`.
- Tests: Vitest, `node` environment, `renderToString` from `react-dom/server`, next to the component
  (`*.test.tsx`), run by `make check`. No DOM and no Testing Library yet (exec plan 0003, P2.2).
- Browser journeys: Playwright, one file per user path in `e2e/*.e2e.ts` (never `*.test.ts` or `*.spec.ts`,
  which Vitest collects), run by `make e2e` against this worktree's `make dev` and by `make check-all`
  ([web-e2e-journeys](../../docs/harness/rules.md#web-e2e-journeys)). How to run, debug and write them:
  the [drive-app](../../.claude/skills/drive-app/SKILL.md) skill.
- Coverage: `make coverage` runs each workspace's `coverage` script (here `vitest run --coverage`). Any
  workspace (`apps/*`, `packages/*`) with a `test` script needs one, and it must accept the
  `--coverage.provider`, `--coverage.reporter` and `--coverage.reportsDirectory` flags and write an
  istanbul json-summary there (Vitest does). No `thresholds`: coverage is a trend, never a gate
  ([ADR 0020](../../docs/adr/0020-test-coverage-is-a-trend.md), [ci.md](../../docs/harness/ci.md#coverage)).
- Mutation testing: `make mutation` runs each workspace's `mutation` script as
  `pnpm run mutation stryker.config.json --mutate <files> --reporters json --cleanTempDir always` on the
  `src/` files changed in the last 7 days. Any workspace with a `test` script needs three pieces, as here:
  the `"mutation": "stryker run"` script, the devDependencies `@stryker-mutator/core` and
  `@stryker-mutator/vitest-runner` at this workspace's exact versions, and a `stryker.config.json` copied
  from this one (under pnpm the Vitest runner must be listed in `plugins`; `vitest.related: false` keeps a
  changed file without tests from failing the run). The config is protected: no `thresholds.break`, no
  `excludedMutations` ([ADR 0023](../../docs/adr/0023-mutation-testing-is-a-drift-report.md),
  [ci.md](../../docs/harness/ci.md#mutation-testing)).
- `make dev` serves this app on the worktree's `WEB_PORT`.
