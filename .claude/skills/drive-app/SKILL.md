---
name: drive-app
description: Drive this worktree's running web app in a real browser (Playwright) to check a UI change end to end, reproduce a bug a user sees, or add an e2e journey. Use whenever a change touches what the web app shows or how it talks to the API, and unit tests alone cannot prove it works.
---

# Drive the app

Reach the app **only through `make` targets** and the addresses they print. They load this worktree's ports
themselves; never read, print or source the env file (`.env.local`), and never guess a port.

## 1. Run the journeys

- `make e2e` runs every journey in `apps/web/e2e/*.e2e.ts` in headless Chromium against this worktree's
  `make dev`. It attaches to a `make dev` that already runs, else starts one and stops it afterwards
  (rules.md#web-e2e-journeys). `make e2e E2E_GREP='<title regex>'` runs only the matching journeys; an
  `E2E_GREP` that matches nothing fails. Make expands `$` in a command-line value: write `$$`, or pass it from
  the environment (`E2E_GREP='…' make e2e`).
- `make check-all` runs `make e2e` too, so a broken journey blocks the PR. There it never attaches: it starts
  its own `make dev` from the current code and fails with "… is already used" while yours runs, so **stop your
  `make dev` first**.
- "Executable doesn't exist" or a missing env var: run `make setup` (it installs the browser into
  `.bin/playwright` from the Playwright version in the lockfile). On Linux, a browser that fails to launch for
  missing shared libraries needs `apps/web/node_modules/.bin/playwright install-deps chromium-headless-shell`
  (apt, needs sudo; ask a human if you have none).

## 2. Read a failure

1. The failing step and its expected/actual value, printed by `make e2e` above the rule's remediation.
2. `.tmp/e2e/<test>/`: `test-failed-1.png` (screenshot), `error-context.md` (the page's accessibility tree at
   the failure: read this first, it is text) and `trace.zip` (a human can drop it on https://trace.playwright.dev,
   which reads the file in the browser without uploading it). `make e2e` empties `.tmp/e2e` at each run.
3. `.tmp/dev/api.log` (the API's JSON lines) and `.tmp/dev/web.log` (Vite), written by the `make dev` that ran.
   For a request's logs, traces or latency, use the `query-runtime` skill.
4. "make dev: port … is already in use": a `make dev` of this worktree already runs (or half runs, e.g. the
   API without Vite), and the new one refused to start so as not to empty those two logs. Read them, then stop
   that `make dev`.

Fix the code, not the journey, unless the behaviour change is intended and the spec says so.

## 3. Explore or reproduce by hand

- Start `make dev` **in the background** (it runs until stopped). Its first line prints the API and web
  addresses and the log files; use those addresses, nothing else.
- To look at a page, write a journey rather than clicking around: a new `apps/web/e2e/<slug>.e2e.ts` (not
  `*.test.ts` or `*.spec.ts`: Vitest collects those), run it with `make e2e E2E_GREP='<its title>'`. Inside it,
  `await page.screenshot({ path: '../../.tmp/e2e/<name>.png' })` saves a picture you can read, and
  `page.on('console', …)` / `page.on('pageerror', …)` collect browser errors (see `critical-path.e2e.ts`).
- Delete a throwaway journey when done; keep one that guards a real user path, with a title that says what
  the user does.
- Playwright MCP browser tools, if your client has them, may look at the addresses `make dev` printed. They
  are optional: nothing here depends on them, and a finding from them is proved by a journey in `make e2e`.

## 4. Write a journey that lasts

- One journey per user-visible path, in steps (`test.step`) named for what the user sees.
- Find elements by role and text (`getByRole`, `getByText`), never by CSS class or test-only markup.
- Use relative URLs (`page.goto('/')`): the base URL is this worktree's web port. The API's address is
  `process.env.HTTP_ADDR` (set by `make e2e`).
- No `waitForTimeout`, no retries, no `test.only` (the config forbids it): rely on Playwright's auto-waiting
  `expect`.

## 5. When done

Stop the background `make dev` you started (leave one that was already running) and delete throwaway
journeys and screenshots. `.tmp/` is gitignored; nothing in it is committed.
