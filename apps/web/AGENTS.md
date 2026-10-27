# apps/web

- Vite + React 19 + TanStack Query. Data comes only from hooks in `@harness-template/api-client`
  (generated from `api/openapi.yaml`); no `fetch`, `axios` or `XMLHttpRequest` here (ESLint).
- Show API errors with `problemOf(error)`; it returns the parsed RFC 9457 problem.
- Domain logic without React (calculations, validation, formatting rules) goes in `packages/domain`, which
  may not import React, browser APIs or apps (dependency-cruiser).
- Never import from another app (`apps/*`); share code through `packages/*`.
- Tests: Vitest + Testing Library next to the component (`*.test.tsx`), run by `make check`.
- `make dev` serves this app on the worktree's `WEB_PORT`.
