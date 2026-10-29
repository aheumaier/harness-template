# ADR 0025: Browser e2e is the NFR-B2 sensor

Status: proposed · 2026-10-29

## Context
NFR-B2 ([nfr.md](../harness/nfr.md)) asks for the app's critical path to be covered by an e2e test that runs
against this worktree's `make dev`. Exec plan 0003 slice P2.3 (`docs/exec-plans/*/0003-harness-gap-closure.md`)
built it: `make e2e` drives one journey in a browser and runs in `make check-all` and CI's `check-all` job.
Unit tests render components to a string and test handlers in-process; neither proves that Vite, its proxy,
the generated client, the API and Postgres work together.

Three things about it were decided in the slice but never recorded:

- **A browser is a new kind of tool.** Every other tool in `.bin` is a Go module or a sha256-pinned release
  binary ([ADR 0016](0016-tool-supply-chain.md)). Playwright's Chromium headless shell comes from
  Playwright's CDN, and on Linux it needs system libraries from apt.
- **The rule had no adopting decision.** `web-e2e-journeys` cited [ADR 0015](0015-anticipated-rules-and-nfr-classes-b-o.md),
  which adopts the Phase 0 rules and sets NFR classes B and O; it does not choose a browser runner or adopt
  this rule.
- **Where the browser is installed.** P2.3 installed it from `make deps`, so `make setup` downloaded it in
  every worktree and every CI job, also where nothing runs e2e (the plan's Phase 2 gate).

## Decision
- **One pinned `@playwright/test` drives the journeys** (`apps/web/e2e/*.e2e.ts`) in headless Chromium (the
  headless shell, not the full browser) against this worktree's `make dev`, in `make check-all` and CI's
  `check-all` job. The version is exact in `apps/web/package.json` and locked in `pnpm-lock.yaml`; it names
  the browser build. Journeys assert through the contract's generated schemas (`@harness-template/api-client`),
  not copies of them.
- **Rule `web-e2e-journeys` is adopted as `team-convention` by this ADR** (with this ADR in place of an
  incident, like the rules in ADR 0015). Its config (`apps/web/playwright.config.ts`, `e2e/global-setup.ts`)
  is protected: no `.only`, no retries, never attaching to a running `make dev` in `make check-all`.
- **The browser is installed only where e2e runs.** `make e2e` installs it on demand into `.bin/playwright`
  (`scripts/install-tools.sh browsers`, stamped with the Playwright version, so a second run downloads
  nothing), before the sensor starts, so the download is not charged to its latency budget. `make setup` and
  `make deps` never download it. In CI only `ci.yml`'s `check-all` job installs it (and caches it through
  `.github/actions/setup`'s `browser` input) and runs `make e2e-deps` for its system libraries.
- **Provenance is a human step at each bump.** Until the browser has sha256 pins
  ([TD-42](../exec-plans/tech-debt-tracker.md)), a Playwright bump is a harness PR whose description notes
  where the version was checked (release notes, the npm package's provenance) and its age.

## Consequences
- A worktree that never runs `make e2e`, and every CI job but `check-all` (`harness-selftest`,
  `generator-probe`, `coverage`, `drift`, `pr-review`), downloads no browser and runs no apt step. The
  first `make e2e` in a worktree waits for the download (207 MB unpacked in `.bin/playwright` on macOS arm64,
  Playwright 1.64.1); later ones do not.
- `make e2e-deps` is Linux only and needs sudo; on macOS the headless shell needs no extra libraries.
- **Known gap** (moved from ADR 0016): the headless shell comes from Playwright's CDN with no sha256 pin; it is
  pinned only through the locked `@playwright/test` version, which names the browser build. `make e2e-deps`
  (`playwright install-deps`) installs the system libraries with an unpinned `sudo apt-get`
  ([TD-42](../exec-plans/tech-debt-tracker.md)). Mitigation: the browser drives only this worktree's localhost
  servers; the installer unsets Playwright's download-host and platform overrides, so the environment cannot
  redirect the download; the `check-all` job runs with `contents: read` and no secrets; each Playwright bump
  is a harness PR with a human provenance note. Containment is per job and per worktree, not per process:
  where `make e2e` runs, the browser runs with the developer's or the job's rights.
- `web-e2e-journeys` has no selftest fixture yet (TD-42): a planted broken journey would need Docker, a worktree
  env, a Go build and the browser in the selftest copy.
