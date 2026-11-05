# 0003 — Close the harness-engineering gaps

Status: implemented, awaiting review · Owner: @aheumaier · Started: 2026-10-28

## Goal

Close the gaps between this template and the sources it was built from: OpenAI's
[Harness engineering](https://openai.com/index/harness-engineering/) (Feb 2026), Böckeler's
[Harness engineering for coding agent users](https://martinfowler.com/articles/harness-engineering.html) and
its follow-up *Maintainability sensors for coding agents* (May 2026). Plan 0002 made the harness fast and
self-assured. This plan points it at the **running app** and the **behaviour** of the code, and fills the
missing sensor classes (secrets, JS dependencies, structure). Quick wins come first.

Rules for every slice:

- **One slice per PR.** Most slices touch protected paths (`scripts/harness/protected-paths.txt`), so each one
  ships as its own human-approved harness PR, as in 0002.
- **Every new sensor ships complete.** It gets a rule in `docs/harness/rules.md` with a remediation, a
  violation fixture in `scripts/harness/fixtures/` ([ADR 0009](../../adr/0009-harness-selftest.md)) and a
  `budgets.txt` row if it may exceed the 5 s default (NFR-H5).
- **Nothing in `make check` may break NFR-R1** (same tree, same verdict). A sensor whose verdict depends on
  the network or on time (advisory DBs) goes in `check-all`, CI and the weekly drift run, like `govulncheck`
  today (`go-no-known-vulns`).

## Gaps

From a comparison on 2026-10-28 (`perf/harness-nfr` @ 53ec497). ✅ present · 🟡 partial · ❌ absent.

| Id | Gap | Source | Repo today | Status |
|----|-----|--------|------------|--------|
| G1 | Runtime legibility: logs, metrics and traces per worktree, queryable by the agent | OpenAI (LogQL/PromQL per worktree) | stdout slog only; OTel provider with exporter `none`, no spans; no request ID, access log or metrics; `compose.yaml` has only `db` | ❌ |
| G2 | Behaviour harness: e2e, coverage, mutation testing, approved fixtures | Fowler ("the elephant in the room"); OpenAI (agent drives the app); Anthropic (browser e2e as the done-sensor) | spec-kit feed-forward only; ~7 platform Go tests, 1 integration test, 1 `renderToString` web test; `new-domain.sh` scaffolds no real tests | ❌ |
| G3 | Inferential sensors on rails, not instructions | Fowler (security, data-handling and modularity reviews); OpenAI (agent-to-agent review) | `code-reviewer`/`architecture-reviewer` triggered by CLAUDE.md:10-11 only; not in CI, not scheduled | 🟡 |
| G4 | Doc truth: guides checked against reality, gardened on a schedule | OpenAI (linters keep the knowledge base current; doc-gardening agent) | links and CLAUDE→AGENTS imports only; `agent-gc` disabled (`.github/workflows/drift.yml:71-76`); drift found, see below | 🟡 |
| G5 | Taste and AI-shortcoming lints: structured logging, function/file length, complexity, max params | OpenAI (taste invariants); Fowler (maintainability sensors) | structured logging is a comment in `logging.go`; golangci `standard` + `depguard` + 2 custom analyzers (`noctxroot`, `domainiso`); ESLint without `max-lines`/`complexity`/`max-params`; core belief 3 rules out "style rules beyond formatters" | 🟡 |
| G6 | Secrets, SAST, JS dependency vulns, earliest-possible timing (pre-commit) | Fowler (GitLeaks pre-commit, Semgrep; keep quality left) | `govulncheck` on the root module only (TD-3); no secret scan, no JS audit, no SAST; git hook is `commit-msg` only | ❌ |
| G7 | Template drift: derived products can't pull harness updates | Fowler (harness templates drift from upstream) | no template version marker, no sync path | ❌ |
| G8 | Quality grades per domain *and layer*, kept fresh | OpenAI (QUALITY_SCORE, background agents) | `docs/QUALITY.md` manual, empty, per domain only, no staleness check | 🟡 |
| G9 | `docs/references/*-llms.txt` for the pinned stack | OpenAI | none | ❌ |
| G10 | Rippable harness: know which rules still fire, retire dead ones | O'Reilly, Osmani | `timings.jsonl` records latency only, not findings | ❌ |
| G11 | Orphans and duplication: unused scripts and make targets, copy-paste clones, one concept implemented twice | OpenAI (agents replicate existing patterns, garbage collection); core belief "the best code is the code nobody writes" | `go-no-dead-code` weekly only (drift.yml), knip in check-all; nothing for scripts, make targets or clones; reuse left to `code-reviewer` | 🟡 |
| G12 | Adoption fails on a real brownfield repo | README §(b), walked against two repos (see below) | layout hard-coded in ~15 protected files; `make rename` refuses a bare module; `make setup` needs a root `package.json`; `check` always composes go+db+web; `.github/` is always harness | ❌ |
| G13 | Spec acceptance scenarios traced to tests; an untested scenario sends the work back to the test step without a human | Thoughtworks ([Engineering the harness](https://www.thoughtworks.com/insights/blog/architecture/engineering-the-harness-a-practical-pattern-for-reliable-coding-agents), Sep 2026: REVIEW checks tests against acceptance criteria) | `specs-structure` and `specs-required-for-features` check spec files only; nothing ties a `Given/When/Then` scenario to a test | ❌ |
| G14 | Contract changes judged across repos: the upstream specs a product consumes, and the consumers of its own contract | Thoughtworks (multi-repo impact analysis and confirmation gate) | `api-no-breaking-change` compares the contract with its own merge base only; vendored upstream specs (first adopter: 7ed-emergencies `api/upstream`) and consumers are unknown to the harness | ❌ |
| G15 | Defaults an agent picked without asking are recorded and reviewable | Thoughtworks (explicit defaults instead of guesses) | loop slices record Decisions; other PRs have no place for them; `speckit-clarify` covers specs only | 🟡 |
| G16 | Tests written apart from the implementation, so the implementer cannot fit the test to its code | Thoughtworks (RED test agent, GREEN implementer agent) | the same agent writes test and code; weekly mutation testing (P2.6) finds weak tests only afterwards | 🟡 |
| G17 | Leftover runtime: per-worktree compose stacks outlive their worktree or session | CLAUDE.md "leave no loose ends"; OpenAI (garbage collection) | `make down` exists but nothing calls it; lane close (loop prompt §5g) tears down by prose only; on 2026-10-29 five `harness-*` Postgres stacks ran, one from a scratchpad copy that was no git worktree | ❌ |
| — | Enforced merge gates | Fowler (human input where it matters) | out of scope: [TD-2](../tech-debt-tracker.md), blocked on the GitHub plan | — |

**Doc drift found** (Phase 0 fixes it and adds the sensor that would have caught the first item):

- `docs/harness/rules.md:115` (`db-schema-doc-fresh`) and `:145` (`api-documented-errors`) claim fixture
  coverage that does not exist. `rules.md:64` names a fixture without the `fixtures/` prefix.
- `apps/web/AGENTS.md:9` says "Vitest + Testing Library". No `@testing-library/*` is installed;
  `apps/web/vite.config.ts:16` uses `environment: 'node'` with `renderToString`.
- `docs/SECURITY.md:14` cites pnpm `minimumReleaseAge`. Only `minimumReleaseAgeExclude` is set
  (`pnpm-workspace.yaml:10`).
- `AGENTS.md:17` and the `check` help text in the Makefile say `make check` takes "<60 s". NFR-H3 targets ≤ 10 s for a change without
  shell or harness files and NFR-H4 ≤ 60 s for any change.
- Plan 0002: phase table says `proposed` while P1–P11 are done. `0002-loop-prompt.md:90` says hooks `exec`
  the binary; ADR 0012 says they don't. ADR 0013:12-16 moves the NFRs to `docs/harness/` "by the harness
  PR", which is still pending.

**Adoption failures found** (G12). README §(b) was walked on 2026-10-28 against two real repos: a Go service
with hand-rolled migrations, and a Go CLI whose product is its `.github/` workflows. Both have a root
`go.mod`, no `apps/`, and their own CLAUDE.md and `.claude/` hooks. Every file below but
`scripts/setup-worktree.sh` is `config` in `scripts/harness/protected-paths.txt`, so an agent cannot
re-point it; P4.6–P4.10 fix the code.

- **Layout hard-coded.** Each assumes `apps/api`, `api/openapi.yaml` or `packages/`. Most fail silently: a
  sensor skips instead of going red.
  - `scripts/harness/file-sensors.sh:24-31`: `apps/api/*.go) echo go`, so a root-module Go edit gets no
    per-file sensor in fast-check or pre-commit.
  - `scripts/harness/affected.sh:33-36, :67`: no changed file starts with `apps/api/`, so the affected set is
    empty with exit 0 and `make check` skips `lint-go` and `test-go` without a word.
  - `scripts/harness/check-specs.sh:59`: `product=('apps/*' 'api/*')`.
  - `scripts/harness/gen-check.sh:14-18`: `GEN_RE` and `INPUTS` miss every generated file.
  - `scripts/harness/pre-commit.sh:77` (`apps/api/.golangci.yml`), `selftest.sh:40, :54-55`,
    `migrations-append-only.sh:12`, `squawk.sh:8`, `contract-check.sh:10` (`SPEC=api/openapi.yaml`).
  - `scripts/install-tools.sh:97-108` (pnpm and the root `package.json`, see below);
    `scripts/setup-worktree.sh:93, :99` (compose `db`, `cd apps/api && atlas migrate apply`);
    `scripts/gen-schema-doc.sh:8`.
  - `.claude/hooks/fast-check.sh:44-82`: every branch keys on `apps/api` or `packages/`.
  - `apps/api/tools/harness/integritycheck.go:36-37`: excludes `apps/api/internal/*/store/*` and
    `packages/api-client/src/gen/*`.
  - `.github/actions/setup/action.yml:18` (`go-version-file: apps/api/go.mod`), `:44` and `:54-57` (pnpm,
    `make deps`), `.github/workflows/contract-gate.yml:40, :44`, `harness-integrity.yml:49`, the
    `generator-probe` job (`ci.yml:67-83`) and the contract and migration paths in CODEOWNERS.
  - `Makefile:21`: `API  := apps/api` drives `--dir $(API)` in every Go sensor and locates the harness module
    (`Makefile:99`).
- **Bare module refused.** `make rename` checks `MODULE` against `host.tld/path`
  (`apps/api/tools/harness/rename.go:25`). The service's module is a bare name, so step 4 (rename) has no
  valid input.
- **Setup needs a JS workspace.** `scripts/install-tools.sh:104-108` exits 1 unless the root `package.json`
  pins `packageManager`. Neither repo has one, so `make setup` aborts before the other tools land.
- **Fixed composition.** `make check` (`Makefile:63`) and `check-all` (`:70`) always run the go, db and web
  sensors. Dropping a rule id does not drop its target.
- **`.github/` is always harness.** `protected-paths.txt:25-26` marks `.github/workflows/*` and
  `.github/actions/*` as `config`. In the CLI repo those are the product, so every product edit hits
  `protect-harness` and `harness-config-not-loosened`.

## NFRs

This plan's NFRs (NFR-H8, NFR-H9, NFR-B1, NFR-B2, NFR-O1) live in [docs/harness/nfr.md](../../harness/nfr.md)
with plan 0002's (moved in Q5). Budgets land in `scripts/harness/budgets.txt` (protected; raising one is a
loosening). ADR 0013 defines only latency (H, warn) and reliability (R, fail) NFRs; the behaviour (B) and
observability (O) classes are added by [ADR 0015](../../adr/0015-anticipated-rules-and-nfr-classes-b-o.md).

Inferential sensors get **no** blocking NFR: an LLM verdict is not reproducible, so it cannot meet NFR-R1.
They comment, never fail (Phase 3, ADR).

## Phases

| # | Phase | Exit criterion | State |
|---|-------|----------------|-------|
| 0 | Quick wins: Trivy, pre-commit, structured-logging lints, doc truth | secrets and JS/nested-Go vulns are detected; `rules-fixture-claims` is green; every drift item above is fixed | implemented |
| 1 | Runtime legibility (G1) | NFR-O1 met: logs by request ID and a p95 latency, per worktree, via `make` | implemented |
| 2 | Behaviour harness (G2) | NFR-B1 and NFR-B2 met; coverage reported per package; weekly mutation score in the drift issue | implemented |
| 3 | Inferential sensors and garbage collection (G3, G4, G8, G11) | reviewer comments on every PR (advisory); scheduled reviews and `agent-gc` open issues/PRs; QUALITY.md has a staleness check; orphan scripts, dead Go code and clones fail `check-all`; `reuse-reviewer` exists | implemented (exit pending: first PR comment and drift run observed) |
| 4 | Taste lints and harness lifecycle (G5, G7, G9, G10, G12, G17) | ADR amending core belief 3 accepted; size/complexity lints green; template version marker + sync check; per-rule fire counts in the drift issue; `adoption-probe` green on a root-module fixture; reviewer catches seen twice are earned rules; sensor slices get an evasion review; stacks without a live worktree are listed and prunable | partial: P4.13 (make stacks) open |
| 5 | Spec-to-test and cross-repo feedback (G13, G14, G15, G16) | every acceptance scenario in `specs/` has a test or a reasoned exemption, checked in `make check`; upstream spec drift reported weekly on the first adopter; PR template has an Assumptions section the reviewer checks; `test-writer` trial result recorded | partial: upstream drift not yet run on a first adopter (TD-115) |

## Progress

One slice per PR, in this order (the [loop prompt](0003-loop-prompt.md) works through it). `[ ]` open, `[x]` done (`sha — before → after`), `[!]` blocked (reason).
**H** marks a slice that touches `scripts/harness/protected-paths.txt` and so needs a human-approved harness
PR; **–** needs none.

- [x] Q0 — 3b112a9 — no ADR → [ADR 0015](../../adr/0015-anticipated-rules-and-nfr-classes-b-o.md) proposed:
  Phase 0 rules adopted as `team-convention`, NFR classes B and O (fail) amend ADR 0013
- [x] Q1 — d5cbcdd — `repo-no-secrets` not run → 0.43 s warm, full tree, in `make check`; no JS/nested-`go.mod`
  vuln scan → `deps-no-known-vulns` (4 manifests, 0.55 s warm) in `check-all`, CI, drift; Trivy v0.76.0, atlas,
  shellcheck sha256-pinned
- [x] Q2 — 817b08c — no hook → `pre-commit` git hook on the index (secret scan, golangci fmt, shellcheck,
  shfmt, check-links): 568 ms warm / 660 ms on a real 10-file commit (budget 2 s, NFR-H8)
- [x] Q3 — d560f7f — 7 wrong fixture claims → 0 (`harness fixture-claims`, ~77 ms, in `make check` when
  rules.md or a fixture changes); selftest 33 → 39 fixtures
- [x] Q4 — cd8afac — `fmt.Print*`, std `log` and mixed slog args unchecked → `go-structured-logging`
  (`forbidigo` by package path + `sloglint` `no-mixed-args`) in `lint-go`; selftest 39 → 41 fixtures
- [x] Q5 — d565e94 — 5 drifted guides → 0: `minimumReleaseAge` implicit → `1440` (pnpm default) + strict
  `false` explicit; web guide names the real stack; "<60 s" → NFR-H3/H4; plan 0002 phases and `exec` claim
  fixed; NFR tables → [docs/harness/nfr.md](../../harness/nfr.md)
- [x] Q6 — e919bed — no SAST → `go-sec` (`gosec` at its defaults, tests included) in `lint-go`, 0 findings in
  the API module; golangci-lint 498 → 539 ms warm (full scope); selftest 41 → 42 fixtures
- [x] P1.1 — 55a6915 — no stack chosen → [ADR 0017](../../adr/0017-per-worktree-observability-stack.md)
  proposed: one `grafana/otel-lgtm:0.36.0@sha256:57c04cbd…` per worktree, 0.53 GiB idle, 3.7 s to ready
- [x] P1.2 — 4e2d8bd — a request left no trace → `X-Request-Id` on every response (declared on `/healthz`
  200 only, TD-16), one access line and one `otelhttp` server span per request, every query a child span,
  all carrying the request and trace ID; NFR-O1 itself is measured once `make logs` exists (P1.4);
  [ADR 0018](../../adr/0018-http-and-db-instrumentation.md) proposed
- [x] P1.3 — b43eeaf — telemetry stayed in the process → per-signal exporter (`none`, `stdout`, `otlp`)
  from `OTEL_{TRACES,METRICS,LOGS}_EXPORTER` and `OTEL_EXPORTER_OTLP_ENDPOINT`, validated in config; RED
  metrics per route via otelhttp; logs through an otelslog bridge;
  [ADR 0021](../../adr/0021-telemetry-export-over-otlp-http.md) proposed; defaults stay `none` until P1.4
- [x] P1.4 — 5b7e649 — done — the agent could not read its own telemetry → compose profile `obs`
  (pinned `grafana/otel-lgtm`, `127.0.0.1` ports +3..+7 of the worktree block); `make obs-up`/`obs-down`/
  `down`, `make logs REQUEST_ID=…|QUERY=…`, `make traces ID=…`, `make metrics QUERY=…`
  (`scripts/obs-query.sh`, jq pinned in `.bin/`); NFR-O1 met by hand (p95 `/healthz` 4.75 ms). Open: no
  `check-all` test for NFR-O1 yet (TD-32)
- [x] P1.5 — db4b1c1 — done — NFR-O1 was met by hand only → `make test-obs` in check-all boots the API
  with OTLP on, sends one request and reads its access line, its server span and the p95 of that API
  process back through `make logs`/`traces`/`metrics` (rule `obs-request-queryable`); `obs-pull` is its own
  sensor; `make dev` tees to `.tmp/dev`; skill `query-runtime`; ADR 0017 amended (stack opt-in for dev,
  required by check-all and CI); TD-32 narrowed, TD-36/TD-37 added
- [x] P2.1 — 6884b56 — a scaffolded domain had no tests → `new-domain.sh` generates a service test (fake
  store), a handler test (strict server + `httptest`) and an integration test, green on generation; the CI
  generator probe runs them (NFR-B1 0 → 100 %); the heredocs moved to `scripts/templates/new-domain/`
- [x] P2.2 — no-op — web: jsdom + Testing Library, or keep `renderToString` → kept: the one web test
  (`App.test.tsx`) renders to a string, and no test needs interaction or effects yet (P2.3 drives the browser
  through Playwright, P2.1 scaffolds Go tests); the guide from Q5 stands
- [x] P2.3 — afdfeee — done — no test drove the running app → `make e2e` runs one Playwright journey
  (pinned `@playwright/test`, headless Chromium in `.bin/playwright`) against this worktree's `make dev`, in
  `check-all` and CI (NFR-B2); skill `drive-app`; rule `web-e2e-journeys`; budget row `e2e`; TD-42 added,
  TD-36 closed
- [x] P2.4 — 6a30e4d — partial — a response change passed unseen unless a test asserted it → handler tests
  pin responses with `approvaltest.Verify` (`testdata/approved/<operationId>.<status>[.<case>].json`,
  `make approve`); `api-responses-approved`, `api-approved-match-contract` (`.bin/harness approved-check`)
  and `go-test-helpers-only-in-tests`; getHealth 200/503 pinned; [ADR 0022](../../adr/0022-approved-api-responses.md)
  proposed. Open: opt-in, no completeness check (TD-30), stale files undetected (TD-31)
- [x] P2.5 — f28e238 — no coverage signal → `make coverage` (Go per package + every JS workspace with a
  `coverage` script) in a non-required PR job summary; drift lists packages whose coverage fell;
  [ADR 0020](../../adr/0020-test-coverage-is-a-trend.md) proposed: a trend, never a gate
- [x] P2.6 — 4595942 — done — coverage showed lines ran, not that a test would notice them change → weekly
  `make mutation` in drift.yml on the week's first-parent changes (gremlins v0.6.0 for apps/api, Stryker
  10.0.0 for workspaces with a `mutation` script); first run: apps/api 52.1 % (73 mutants, 7 survived),
  apps/web 20.0 % (15, 1 survived); [ADR 0023](../../adr/0023-mutation-testing-is-a-drift-report.md)
  proposed. A trend, never a gate
- [x] P3.1 — eb61f5e — inferential sensors had no stated status →
  [ADR 0019](../../adr/0019-inferential-sensors-are-advisory.md) proposed: one top-level PR comment, never a
  review thread; skip with a notice; outside the harness NFRs; hard cost cap in a protected file; a finding
  class accepted twice becomes an earned rule
- [x] P3.2 — 1b33d8e — done — `code-reviewer` ran only on request → `pr-review.yml` runs it on every
  `pull_request` and posts one advisory comment (never a required check); sha256-pinned Claude Code via
  `install-tools.sh claude`, read-only tools, pinned model, turn and budget caps; inert with a notice until
  the `ANTHROPIC_API_KEY` secret exists (TD-38); ADR 0016 and 0019 amended; TD-39 added
- [x] P3.3 — f5b5d27 — done — no scheduled review of the whole repository → drift.yml runs three
  read-only review agents (skills `review-security`, `review-data-handling`, `review-modularity`) and appends
  their findings to the drift issue, advisory; `scripts/harness/ci-agent.sh` shared with `pr-review.sh`;
  ADR 0016 and 0019 amended; TD-40 added, TD-38 extended
- [x] P3.4 — 14d7317 — done — `agent-gc` was a stub → drift.yml's agent-gc fixes the broken doc links
  `check-links` found and opens one small PR (advisory, ADR 0019 amended), confined to its target files, no
  protected path, AGENTS.md or CLAUDE.md; off until `AGENT_GC_ENABLED` and the `agent-gc` environment's
  `AGENT_GC_TOKEN` are set (TD-50)
- [x] P3.5 — 5be0ad6 — done — QUALITY rows unchecked → `make docs-quality-shape` (deterministic, in `check-all`
  and the generator-probe) and age-only `make docs-quality` in drift; rows per backend domain and layer with
  `Updated` (ADR 0028; frontend not graded TD-56, domain list twice TD-55)
- [x] P3.6 — 3edf728 — done — no orphan detection → `repo-no-orphan-scripts` (`harness orphans`, `make
  orphans` in `check-all`) for scripts and make targets; `deadcode-go` also scans tools/harness and runs in
  `check-all` (ADR 0031; tools/analyzers left out TD-70)
- [x] P3.7 — 8e4f77b — done — no clone detection → `repo-no-clones`: golangci `dupl` threshold 26 per package
  in `make check` (lint-go), `jscpd` 5.4.1 `minTokens` 16 over `apps/` and `packages/` in `check-all`; thresholds
  measured after cheap fixes with a scaffolded domain, no baseline (ADR 0027; per-package blind spot TD-52)
- [x] P3.8 — ddd125f — done — no reviewer asked whether a diff re-implements existing code →
  `.claude/agents/reuse-reviewer.md` (Read, Grep, Glob), called from CLAUDE.md when a change adds code or
  files; findings name the existing location and a `<kind>:<capability>` class for ADR 0019's ratchet
- [x] P4.1 — 20c1021 — done — core belief 3 called size and complexity style →
  [ADR 0024](../../adr/0024-size-and-complexity-limits-are-maintainability-sensors.md) proposed: limits as maintainability
  sensors; core belief 3 amended
- [x] P4.2 — f49fc8d — done — no lint bounded function length, complexity, parameters or file length →
  golangci (`funlen`, `gocognit`, revive limits) and ESLint (`max-lines-per-function`, `complexity`, `max-params`,
  `max-lines`) enforce them; rules `code-*` with 8 fixtures; `harness-selftest` 47 → 55
- [x] P4.3 — a91b86d — done — no record of the template a product was cut from → `.harness-version` holds
  the template commit, `make rename` sets `HARNESS_UPSTREAM`, `make harness-sync-check` lists protected files
  changed upstream since, reported by the weekly drift run (ADR 0026; private template: TD-51)
- [x] P4.4 — bb790ee — done — per-rule fire counts in `timings.jsonl`; the drift Sweep merges them into a
  90-day rule-fires ledger and lists rules that never fired, for a human to keep or retire; ADR 0032, TD-79..81
- [x] P4.5 — d518888 — done — no pinned tool references → `docs/references/*-llms.txt` for oapi-codegen, sqlc,
  orval, Atlas with `docs-references-pinned` in `make check` (ADR 0029); a harness change (H), not "–"
- [x] P4.6 — 449d561 — done — the product layout was hard-coded in every sensor → `scripts/harness/layout.env`
  (with `API_CLIENT_DIR`) parsed by `layout.sh` for the scripts, the Makefile and CI and by `.bin/harness`;
  `make layout-check` (`harness-layout-valid`) in `make check`; `.claude/hooks/fast-check.sh` keeps the template
  layout until a human `.claude` PR (TD-66), as does `make new-domain` (TD-67) (ADR 0030)
- [x] P4.7 — bfec770 — done — stack profiles: `STACK` in layout.env (or `HARNESS_STACK`) selects sensor
  groups, tool installs and fixtures; spectral, squawk and bats move to .bin; ADR 0033, TD-83..86
- [x] P4.8 — 70e8066 — done — `make rename` demanded a dotted module → it takes a bare module, a one-element
  host or `MODULE=skip` (keeps the template's module root), strips the module as a whole word when looking
  for leftovers, and needs a usable layout.env; TD-23 closed
- [x] P4.9 — dbaaa39 — done — protected paths and CODEOWNERS name the harness's 8 workflows,
  `actions/setup`, CODEOWNERS and the PR template; product workflows stay owner-reviewed; ADR 0035, TD-24
  paid, TD-95..97
- [x] P4.10 — 84399b7 — done — required CI job `adoption-probe` replays README (b) on a protected root-`go.mod`
  skeleton; rule `harness-adoption-probe`; ADR 0038 (proposed); TD-99..101
- [x] P4.11 — 0e03d34 — done — two earned rules with fixtures: `harness-rule-lists-match`,
  `harness-reads-layout`; TD-103
- [x] P4.12 — 8445e4d — done — `evasion-reviewer` agent for sensor slices; ADR 0019 amended, TD-107..110;
  the diff-only re-review and §5c wiring are hand-off proposals
- [ ] P4.13 — H — `make stacks` lists `harness-*` compose projects whose working dir is no live git worktree;
  `make stacks-prune` removes them; `make setup` prints the count
- [x] P5.1 — 9f6771d — done — `specs-scenarios-tested` (ADR 0036 proposed); TD-111..113
- [x] P5.2 — cfce60f — done — `UPSTREAM_SPECS`/`CONTRACT_CONSUMERS` in `layout.env`; weekly `upstream-drift`
  report; consumers named on a break; `harness spec-refs`; ADR 0037 (proposed); TD-115..118
- [x] P5.3 — 0117216 — done — PR template `## Assumptions`; code-reviewer check 6 (advisory); TD-119
- [x] P5.4 — 5b7ee88 — done — trial kept the `test-writer` agent (spec-only, unit tests, cost cap);
  `only-test-files` hook and `harness test-file`; code-reviewer check 7; ADR 0039 (proposed); TD-123..126

### Phase 0 — quick wins

**Q0 — ADR.** `docs/harness/rules.md:8-9` says new rules are earned from an incident or adopted by an ADR.
The Phase 0 rules are anticipated, not earned, so one ADR adopts them as `team-convention`, with the
source each comes from:

- `repo-no-secrets` and `deps-no-known-vulns`: Böckeler (GitLeaks pre-commit).
- `go-sec`: Böckeler (SAST).
- `go-structured-logging`: the invariant `logging.go:9` already states.
- `rules-fixture-claims`: the drift this plan found.

The same ADR amends ADR 0013 with NFR classes B (behaviour) and O (observability), both **fail**.

**Q1 — Trivy.** One pinned binary closes three gaps:

- **Secrets** (no GitLeaks needed).
- **JS dependency vulns** from `pnpm-lock.yaml` (no `pnpm audit` today).
- **Vulns in every `go.mod`.** `trivy fs` walks the nested tools modules that `govulncheck` skips. That pays
  the vuln half of [TD-3](../tech-debt-tracker.md).

Determinism splits the scanners:

| Rule | Command | Where | Why there |
|------|---------|-------|-----------|
| `repo-no-secrets` | `trivy fs --scanners secret --offline-scan --skip-db-update --skip-check-update` over the input set below | `make check`, pre-commit (Q2), CI | offline, rule-based: same files, same verdict |
| `deps-no-known-vulns` | `trivy fs --scanners vuln --exit-code 1 .` | `check-all`, CI, weekly `drift.yml` | the vuln DB is fetched at run time, so the verdict changes over time (same class as `go-no-known-vulns`) |

- **Input set.** `trivy fs` walks the filesystem, not git, so it would read the worktree's env file,
  `.cache/` and `.tmp/` logs. The verdict would depend on more than the tree (NFR-R1).
  - `repo-no-secrets` scans only tracked plus untracked-not-ignored files (`git ls-files --cached --others
    --exclude-standard`, as `SH_FILES` does at Makefile:179).
  - Under `HARNESS_SCOPE=changed` ([ADR 0014](../../adr/0014-make-check-scoped-to-the-change.md)) it scans the changed files.
    Under `full` it scans all of them.
  - How to pass a file list to Trivy (per file, or a hardlinked temp tree) is settled in Q1.
- **Fixtures dir.** A Trivy config (`trivy.yaml`, kind `config`, protected) skips `scripts/harness/fixtures/`.
  Otherwise the planted secret turns the template's own tree red. selftest applies the overlay outside that
  directory, so the fixture still fires. *Superseded at the Phase 0 gate:* skipping the directory let a real
  secret hide there, so only the vuln scan skips it; the `repo-secret-*` fixtures assemble their key in
  `apply.sh` and hold no literal.
- **No misconfig rule.** The repo has no Dockerfile, Kubernetes or Terraform for `trivy config` to scan. Add
  `iac-no-misconfig` with the first Dockerfile.

Supply chain. In March 2026 Trivy's release pipeline was compromised: CVE-2026-33634, listed in CISA KEV.

- Trivy v0.69.4 was a malicious release.
- Most `trivy-action` tags and all `setup-trivy` tags were force-pushed to credential-stealing code.
- On 2026-03-22, the Docker Hub images v0.69.5 and v0.69.6 were malicious too.

So:

- **Install.** `scripts/install-tools.sh` installs Trivy into `.bin/`, like every other tool. Pin it in
  `tool-versions.env` (`TRIVY_VERSION`). Pick a release at least two weeks old, never v0.69.4.
- **Verify.** The enforced check is a sha256 per OS/arch, pinned in `tool-versions.env`: a re-tagged release
  fails the install instead of passing it.
  - Provenance is a human step at every bump: check the sha256 against the release's `checksums.txt` and
    that file's cosign signature, and record the result in the PR.
  - No cosign at install time, because cosign would need its own verified bootstrap.
- **CI.** Never use `aquasecurity/trivy-action` or `setup-trivy`; CI runs `make setup`, then `.bin/trivy`.
  Never use the Docker image.
- **DB.** CI caches the vuln DB by date. Downloads come from the official OCI registry.
- **Boy Scout fix in the same PR.** `install-tools.sh:38-50` downloads `atlas` and `shellcheck` without any
  checksum check. Pin their sha256 the same way; whether upstream publishes checksum files changes only the
  human step.

Fixtures:

- `repo-secret-planted`: a fake AWS key in one file. Runs offline.
- `deps-no-known-vulns`: `none (external advisory DB)`, like `go-no-known-vulns` (rules.md:71, ADR 0009:11).
  A network fixture would make `harness-selftest` (NFR-R3, fails in CI) depend on DB contents.

**Q2 — pre-commit hook.** `scripts/git-hooks/pre-commit`, installed by `make setup` next to `commit-msg`.

- Runs the secret scan on the staged content (`git diff --cached --name-only`, read via `git show :path`,
  not from the working tree).
- Then cheap per-file sensors: `gofmt`, `shfmt`, `shellcheck` and link check on staged `.md`. No type-aware
  eslint and no `tsc`: at up to 1 s per file (NFR-H2), 20 files would not fit in 2 s.
- It does not reuse `.claude/hooks/fast-check.sh`. That script expects hook JSON and reads the working tree;
  the two share the per-file sensor list instead.
- Budget NFR-H8 (≤ 2 s), warn only.
- Never runs the full `make check`: that stays the agent's step (golden principle 6).
- `--no-verify` remains possible. CI (Q1) is the backstop.

**Q3 — doc-truth sensor `rules-fixture-claims`.** Deterministic, runs in `make check` when `rules.md` or a
fixture changes, as a `.bin/harness` subcommand ([ADR 0012](../../adr/0012-parsing-sensors-in-go.md)).

Grammar of a `**Fixture:**` line:

- Either `none (<reason>)`, where the reason is free text and may name other rules' fixtures.
- Or one or more `fixtures/<dir>`, each optionally followed by `; <test path>` (e.g. `; scripts/test/*.bats`).
- Each named dir must exist, and its `fixture.conf` `RULE=` must equal the enclosing `###` rule id.

First findings against today's rules.md (each is fixed in this slice, by a real fixture where feasible,
otherwise `none (<reason>)`):

- free text instead of a fixture: `:115` (`db-schema-doc-fresh`), `:145` (`api-documented-errors`)
- missing `fixtures/` prefix: `:64`, `:189`
- bare `none` with no reason: `:196`, `:203`, `:210`

The check's own fixture is an `apply.sh` that appends one bogus claim to rules.md in the temp repo. Not a
copy of the file: selftest.sh:10, and a full copy would drift.

**Q4 — `go-structured-logging`.** Enforces the invariant `logging.go:9` already states ("never use
fmt.Print"). Adopted by the Q0 ADR.

- `forbidigo`: no `fmt.Print*`, and no `log.*` outside `platform`.
- `sloglint`: key-value pairs or attrs only, no mixed args. No key-case check, which is a naming style rule
  and waits for P4.1.

Suppressions read `reason: … (rule: go-structured-logging)`. One fixture per linter. Size and complexity
lints are a different question (core belief 3) and wait for P4.1.

**Q5 — doc drift.** `pnpm-workspace.yaml`, `SECURITY.md`, `apps/web/AGENTS.md` and plan 0002 are not
protected. `AGENTS.md`, the Makefile, `docs/harness/nfr.md` and `docs/harness/rules.md` are, so this is a
harness PR.

- **pnpm.** pnpm 12.15.0 is pinned, and `.bin/pnpm config get minimumReleaseAge` prints `undefined`. The
  `minimumReleaseAgeExclude` entries (`pnpm-workspace.yaml:10-12`) only make sense if a default age applies.
  - Check the effective default first. Then set `minimumReleaseAge` explicitly in `pnpm-workspace.yaml`, so
    `SECURITY.md:14` cites a value that is visible in the repo.
  - Raising the value past the default is a new policy and needs its own reason in the PR.
- **Web test stack.** Decision: fix the guide now ("Vitest, `node` environment, `renderToString`"). P2.2
  decides whether to add Testing Library. A guide must not promise a tool that isn't installed.
- **Latency claims.** Change "<60 s" in `AGENTS.md:17` and the help text at `Makefile:59` to cite NFR-H3/H4.
- **Plan 0002 and ADR 0013.**
  - Set 0002's phase states and fix its loop prompt's `exec` claim (ADR 0012:35-36).
  - Move the NFR table to `docs/harness/nfr.md`, as ADR 0013 says.
  - Point `budgets.txt:1` and `rules.md:268` at the new file.

**Q6 — `go-sec`.** Add `gosec` to `apps/api/.golangci.yml` (golangci-lint is already pinned). Adopted by the
Q0 ADR. Suppressions follow golden principle 4 (`reason: … (rule: go-sec)`). Semgrep stays out until a
domain exists that gosec can't cover.

### Phase 1 — runtime legibility

The largest lever in the OpenAI article: the agent reads the running app, not just the code.

- **P1.1 — ADR.** Choose the stack by the boring-tech test (core belief 4).
  - Proposed: one `grafana/otel-lgtm` container: an OTel collector plus Loki, Prometheus and Tempo. One image,
    one port per signal.
  - Alternative: a bare OTel collector writing JSON files the agent `jq`s. Fewer moving parts, no query
    language.
- **P1.2 — contract and middleware.** First declare the `X-Request-Id` response header (and the problem
  `instance` value) in `api/openapi.yaml` and run `make gen` (golden principle 1). Then, in
  `apps/api/internal/platform/`:
  - A request ID: honours an incoming `X-Request-Id` only if it passes a length and charset check at the
    boundary (it ends up in logs; golden principle 2), else generates one, and returns it in the response
    header and in problem+json `instance`.
  - An `slog` access log line per request: method, route template, status, duration, request ID.
  - `otelhttp` server spans, and a pgx tracer so every query is a child span.
  - The logger carries the trace and request ID from `context`. Platform stays domain-free (layer rule).
- **P1.3 — export.** OTLP exporter selected by config (`none` stays the default outside dev). RED metrics per
  route template.
- **P1.4 — per worktree.** A compose profile `obs` on ports from this worktree's block (`setup-worktree.sh`).
  - Make targets: `make obs-up`, `make logs REQUEST_ID=…|QUERY=…`, `make metrics QUERY=…`, `make traces
    ID=…`.
  - The agent reaches the stack only through these targets. They source the worktree's env themselves. A
    guide never tells the agent to read the env file: guard-bash blocks it.
- **P1.5 — `make dev` logs.** `make dev` also writes logs to `.tmp/dev/{api,web}.log`, so the agent has
  something to read without the obs stack. The skill `query-runtime` maps questions to make targets
  ("latency of /x", "errors since …", "logs of request …").
- **Test for NFR-O1** (integration test in `check-all`, Docker): boot the API, send one request, assert that
  `make logs REQUEST_ID=…` returns its access line. A positive assertion, so a test, not a selftest fixture
  (selftest asserts that a sensor fails).

### Phase 2 — behaviour harness

Böckeler calls behaviour "the elephant in the room" of harness engineering. Feed-forward is spec-kit; this phase adds
the feedback.

- **P2.1 — scaffold real tests.** `new-domain.sh` generates three tests for the example operation, all green
  on generation:
  - a service test (fake store)
  - a handler test (strict server + `httptest`)
  - an integration test (`-tags=integration`, real Postgres)

  The CI generator probe ([ci.md](../../harness/ci.md)) generates a domain and runs them (NFR-B1). The scaffold's inline heredocs move to template files
  under `scripts/templates/new-domain/` while this is touched.
- **P2.2 — component tests.** Decide jsdom + `@testing-library/react` versus keeping `renderToString`. Rule:
  add the dependency the first time a test needs interaction or effects; until then the guide from Q5 stands.
- **P2.3 — e2e.** Playwright, pinned like every JS tool.
  - `make e2e` starts or attaches to this worktree's `make dev` and runs the journeys.
  - Runs in `check-all` and CI. Browsers are cached in `.bin/`.
  - The runner is the sensor. The `drive-app` skill (screenshots, DOM snapshot, console) is a guide for
    exploratory work, and nothing blocks on an MCP server.
- **P2.4 — approved fixtures.** Golden JSON per operation and status, checked against the OpenAPI schema
  and diffed on change. An intended change is approved by `make approve` and shows in review.
- **P2.5 — coverage.** `go test -coverprofile` and Vitest coverage per package, in the CI summary and the
  weekly drift issue. No threshold gate: a gate on a number invites tests written for the number.
  Trend only, until a ratchet is justified by a finding.
- **P2.6 — mutation testing.** Weekly, on files changed in the last 7 days. Go uses `gremlins`, TS uses
  Stryker. The score and the surviving mutants go to the drift issue. Never in `check`: slow and
  non-deterministic sampling.

### Phase 3 — inferential sensors and garbage collection

- **P3.1 — ADR.** Inferential sensors are advisory, never blocking.
  - They comment on PRs or open issues. None is a required check, and they don't count for "done means
    green".
  - When one finds something twice, the finding becomes a computational rule (core belief 3: ratchet from
    failure).
- **P3.2 — PR review.** A CI job runs the `code-reviewer` prompt on the diff (headless agent) and posts one
  comment. Its cost is capped by diff size; it is skipped on harness-only PRs, where the human gate applies
  anyway.
  - The diff is untrusted input to an agent that holds an API key, so prompt injection and secret
    exfiltration are the risks.
  - Run it on `pull_request`, never on `pull_request_target`. Give it no write token; posting the comment is
    a separate step with its own token that never sees agent output as commands.
  - Follow [ADR 0010](../../adr/0010-base-branch-gates.md): base checkout only, diff passed as data. Forks get no secrets, so
    the job skips them.
- **P3.3 — scheduled reviews.** Weekly security, data-handling and modularity reviews (Böckeler's three).
  Each is a skill in `.claude/skills/` and appends a section to the drift issue.
- **P3.4 — `agent-gc`.** Turn on the stub in `drift.yml` once P3.1 is accepted. It opens small fix-up PRs for
  findings of the computational doc checks (Q3, broken links; stale QUALITY rows only once a grading agent
  exists, ADR 0028) and never touches protected paths.
- **P3.5 — quality grades.** `docs/QUALITY.md` grades per domain *and* layer, each row with an `Updated:`
  date. The drift run flags rows older than 30 days or rows for deleted domains. `new-domain.sh` already
  writes the row marker (`harness:quality-rows`).

G11 splits by what a machine can decide. Reachability and token clones are computational (P3.6, P3.7); only
"the same concept written twice in different code" needs judgement (P3.8). The agent never looks for dead
code: `deadcode` and knip do that without false positives from entry points they can see.

- **P3.6 — orphans.** A `harness orphans <root>` subcommand (ADR 0012) and rule `repo-no-orphan-scripts`.
  - Every file under `scripts/` (fixtures and P2.1's `scripts/templates/` excluded) is named in the Makefile, a workflow, a
    git or Claude hook, a skill, an agent or a guide; every make target is called by another target, a
    workflow, a hook, a skill or a guide, or is listed in `make help` as an entry point (`## ` comment).
  - Fixture: an unreferenced script and an unreferenced make target.
  - Move `deadcode-go` from drift-only into `check-all` (measure it; NFR-H4). Extend it to
    `apps/api/tools/*` if those modules are clean, else a tech-debt row.
- **P3.7 — clones.** Rule `repo-no-clones`: golangci `dupl` in `lint-go` and `jscpd` (pinned dev dependency)
  for `apps/web` and `packages/`, generated code excluded. Thresholds are the strictest the current tree
  passes after cheap fixes; record them in rules.md. One fixture per language. No baseline file.
- **P3.8 — `reuse-reviewer`.** `.claude/agents/reuse-reviewer.md`, read-only (Read, Grep, Glob), scoped to
  the diff: for each new function, script, component or file, search for an existing one with the same
  purpose (`platform`, `packages/domain`, the stdlib, a pinned dependency, `scripts/harness/*.sh`,
  `.bin/harness`).
  - Each finding names both places as `file:line` and why they are one concept; a finding without the
    existing location is not reported.
  - Advisory (P3.1). CLAUDE.md calls it when a diff adds functions, scripts or files; fixes, docs and
    config don't need it.
  - A finding class seen twice becomes a computational rule (P3.1), so the agent narrows over time.

### Phase 4 — taste lints and harness lifecycle

- **P4.1 — ADR amending core belief 3.** Both primary sources name function length, file length, complexity
  and parameter count as the limits that matter most for agent-written code. The amendment calls them
  maintainability sensors, not style.
  - The thresholds start at today's worst value, so nothing existing fails.
  - Every message tells the agent how to split the code, not just that it is too long.
- **P4.2 — the lints.**
  - Go: `funlen`, `gocognit` or `cyclop`, plus a file-length check (a small `.bin/harness` analyzer if
    golangci has none).
  - TS: ESLint `max-lines`, `max-lines-per-function`, `complexity`, `max-params`.
  - Generated code is excluded.
- **P4.3 — template sync.** `.harness-version` records the upstream commit a product was cut from.
  `make harness-sync-check` diffs the protected paths against that upstream commit and lists what changed. It
  never applies anything. The weekly drift run reports when a product falls behind.
- **P4.4 — rippable harness.** `run.sh` adds the finding count per rule to `timings.jsonl`. The drift issue
  lists rules that never fired in 90 days. A human decides to keep or retire each one; this rule is never
  automated.
- **P4.5 — references.** `docs/references/<tool>-llms.txt` for oapi-codegen, sqlc, orval and Atlas, taken
  from upstream at the pinned version. Drift fails when `tool-versions.env` moves and the file doesn't.

Adoption (G12). P4.6–P4.10 turn README §(b) from a list of hand edits into a selection plus one file, and
give it a sensor. Each touches protected paths.

- **P4.6 — layout.env.** One `scripts/harness/layout.env` (`API_DIR`, `SPEC`, `MIGRATIONS_DIR`,
  `QUERIES_DIR`, `WEB_GLOBS`, `GENERATED_GLOBS`, `PRODUCT_GLOBS`, `LAYER_GLOB`) is sourced by every script,
  the Makefile and the hooks; `.bin/harness` reads the same file (integritycheck's excludes, `domainiso`'s
  layer convention); `check-specs.sh` reads `PRODUCT_GLOBS`. The setup action loads it into `$GITHUB_ENV`,
  and the gate workflows and `generator-probe` use those variables. `domainiso` reports when `LAYER_GLOB`
  matches no package, so it is never a silent no-op. Out of scope: `.golangci.yml`, `.go-arch-lint.yml` and
  CODEOWNERS cannot source a file; they stay hand edits, listed in README §(b) step 5.
  - Acceptance: `git grep -n 'apps/api\|api/openapi.yaml\|packages/\|apps/\*' -- scripts .claude/hooks
    Makefile .github apps/api/tools/harness` returns only `layout.env`, fixtures, tests, `new-domain.sh`,
    path lists (`protected-paths.txt`, `go.mod`), the tool's own module constants (`rename.go`,
    `integritycheck.go`) and comments; selftest is green with a fixture that sets `API_DIR=.` (root module).
- **P4.7 — stack profiles.** `HARNESS_STACK=go,db,web` (default: all three) selects the sensor groups `check`
  and `check-all` compose (`Makefile:63`, `:70`), the tools `install-tools.sh` installs and the fixtures
  selftest runs. Without `web`, no pnpm and no root `package.json` check (`install-tools.sh:104-108`).
  Without `db`, `setup-worktree.sh` starts no Postgres (today `SKIP_DB=1`, `:85`) and `schema-doc-fresh`
  leaves `check`.
  - Acceptance: in a copy of the template with `apps/web`, `packages/` and `apps/api/db` removed,
    `HARNESS_STACK=go make setup check harness-selftest` is green.
- **P4.8 — rename for brownfield.** `make rename` accepts a bare module path, or `MODULE=skip`, which renames
  only the name and the owner. It prints the harness module's new path (today
  `<module>/apps/api/tools/harness`, a nested path the product may not have).
  - Acceptance: `rename_test.go` covers a bare module and `MODULE=skip`.
- **P4.9 — harness/product overlap.** `protected-paths.txt` (`:25-26`, `:40`) and CODEOWNERS (`/.github/`)
  list the harness's own workflows and action by name (`ci.yml`, `contract-gate.yml`, `drift.yml`,
  `harness-integrity.yml`, `harness-nfr.yml`, `migration-gate.yml`, `actions/setup`) instead of
  `.github/workflows/*` and `.github/actions/*`. The `harness-integrity` remediation says so: a new
  workflow is product until it is listed.
  - Acceptance: a case where a product workflow changes passes `harness-integrity`, and
    `fixtures/harness-config-loosened` still fails.
- **P4.10 — adoption probe.** A CI job `adoption-probe`, built like `generator-probe` (`ci.yml:67`,
  [ci.md](../../harness/ci.md)). It copies the harness into a minimal root-`go.mod` repo fixture (no `apps/`,
  no web, no db), follows README §(b) literally, and runs `make .bin/harness`, `make rename`, then
  `HARNESS_STACK=go make setup check harness-selftest`. This is the sensor for README §(b); without it the
  procedure drifts again.
  - Acceptance: green on the template; red when the fixture's `layout.env` still points at `apps/api`.

Review feedback loop. P4.11 and P4.12 come from the harness assessment of this loop (Log, 2026-10-29): the
same classes of mistake were caught by reviewers again and again, and three of three sensor evasions were
found only after integration.

- **P4.11 — earned rules.** Every reviewer finding class this plan caught at least twice (lane reviews, phase
  gates, the peer evasion review) becomes a rule in rules.md with `Status: earned (<finding 1>, <finding 2>)`
  (ADR 0019), a sensor, a remediation and a fixture. Candidates, each kept only if the Log or a review shows
  two findings:
  - `harness-reads-layout`: no `apps/api/tools/harness/*.go` other than `layout.go` (and the tool's own
    module constants) holds a template layout literal (`apps/api`, `internal/`, `db/migrations`); a Go test
    in `tools/harness` scans its own sources (P4.6 review, Phase 3 gate on `qualityrows.go`).
  - `docs-make-targets-exist`: every `make <target>` named in AGENTS.md, CLAUDE.md, the guides and the skills
    is a target `make help` lists.
  - `harness-rule-lists-match`: hand-copied rule lists (the skip lists in `.claude/skills/review-*/SKILL.md`,
    `.claude/agents/reuse-reviewer.md`) name only rule ids rules.md defines and every rule of the sections
    they claim to skip (TD-94). The slice may edit those named skill and agent files;
    `.claude/hooks/fast-check.sh` stays a human PR (TD-66).
  - `harness-sensor-deps-pinned`: a version change of a sensor devDependency (jscpd, eslint,
    dependency-cruiser, knip, Stryker) is a `config` change in `integrity-check` (TD-91, ADR 0027).
  - A near-miss fixture (must pass) for each sensor of this plan that had a substring or path bug
    (`orphans` TD-75/TD-78, `check-links`, `quality-rows`), so the fix cannot regress into a false positive.
  - Candidates with one finding only stay in the tracker; the Log names them.
  - Acceptance: each new rule's fixture fails with its remediation; `make check`, `harness-selftest` and
    `rules-fixture-claims` are green; each rule's `Status:` links two findings.
- **P4.12 — evasion review.** A read-only `.claude/agents/evasion-reviewer.md` (tools Read, Grep, Glob and
  `.bin/harness`/`make` sensor runs in a scratch copy) tries to make a new or changed sensor pass on bad
  input: substrings, comments, symlinks, build tags, empty listings, wrong roots, untracked files. It
  reproduces each bypass against the real sensor and reports `file:line — bypass — repro — fix`, as the peer
  review of 3edf728 did (TD-75..TD-78). It runs for any slice that adds a rule id or changes
  `scripts/harness/**` or `apps/api/tools/harness/**`, next to code-reviewer. After a retry, code-reviewer
  runs once more on the retry diff only: each finding fixed, nothing new broken. A slice never edits this plan
  or [the loop prompt](0003-loop-prompt.md), so the slice's Decisions carry the exact text for the loop
  prompt's §5c and for the CLAUDE.md "before calling a change done" line; a human applies both (hand-off).
  - Acceptance: run on 3edf728's diff, the agent reproduces at least three of the eleven bypasses filed as
    TD-75..TD-78; the proposed §5c and CLAUDE.md text is in the hand-off.

- **P4.13 — leftover stacks (G17).** `scripts/setup-worktree.sh` names each worktree's compose project
  `harness-<slug>-<hash>`, and compose labels its containers and volumes with
  `com.docker.compose.project.working_dir`. Lanes, aborted sessions and scratch copies leave these running.
  - `make stacks` lists every `harness-*` project, running or stopped, with its volumes, whose working dir is
    missing or not in `git worktree list`. Read-only, exit 0; without a reachable Docker daemon it prints a
    notice and exits 0.
  - `make stacks-prune` runs `docker compose -p <project> down -v` for the listed projects only: never a live
    worktree's stack, never a project without the `harness-` prefix (an adopter's own stacks). The loop
    prompt's §5g calls it after `git worktree remove` instead of the hand-typed `down -v` (hand-off, since a
    slice never edits the loop prompt).
  - `make setup` prints "N leftover harness stacks, see make stacks" when N > 0. No sensor verdict: the
    answer depends on Docker state, not the tree, so it cannot meet NFR-R1, and CI has no local stacks to
    see.
  - Acceptance: a bats test with a fake `docker` on `PATH` lists and prunes a project whose worktree was
    removed and leaves a live worktree's project and a non-`harness-` project alone; run on 2026-10-29's
    machine state it would have listed `harness-go-only-r2`.

### Phase 5 — spec-to-test and cross-repo feedback

From Nandyala and Pani, [Engineering the harness](https://www.thoughtworks.com/insights/blog/architecture/engineering-the-harness-a-practical-pattern-for-reliable-coding-agents) (Thoughtworks, 2026-09-29), compared with this
repo on 2026-10-29. Most of its practices are already here: scoped guides, read-only reviewers,
silent-success sensors (`scripts/harness/run.sh`), prose promoted to rules, earned and pruned rules (G10), and
gates for contract, migration and harness changes. Four are not.

- **P5.1 — scenarios traced to tests (G13).** spec-kit writes acceptance scenarios as numbered
  `Given/When/Then` items under each user story (`.specify/templates/spec-template.md:34-65`). Rule
  `specs-scenarios-tested`:
  - A scenario's id is `<NNN>/US<story>-AS<n>`, derived from its position; nothing is written into the spec
    (A17).
  - Each id appears in a Go or TS test (test name or a `// spec: 001/US1-AS2` comment), or the scenario
    ends with `(untested: <reason>)` in the spec.
  - The sensor is in `.bin/harness`, runs in `check-all` and in `check` when `specs/` or a test file changes,
    and lists each untested id with its scenario text. A red run sends the agent back to the test step on its
    own, as the article's REVIEW → RED loop does. Whether a test that names an id really checks its scenario
    is code-reviewer's call (advisory, ADR 0019).
  - Acceptance: a fixture spec with one untested scenario fails with the remediation; the template's own
    specs pass or carry exemptions; `rules-fixture-claims` is green.
- **P5.2 — cross-repo contract impact (G14).** The template is one repo, so this lands as layout keys an
  adopter fills in; 7ed-emergencies is the first.
  - `layout.env` gets `UPSTREAM_SPECS` (each vendored upstream spec with its source and pinned revision) and
    `CONTRACT_CONSUMERS` (repos that vendor this repo's contract). Both may be empty.
  - Weekly in drift.yml (network, so never in `check`, NFR-R1): fetch each upstream spec at its source's
    head and run `oasdiff breaking` against the vendored copy; findings go to the drift issue (A18).
  - `api-no-breaking-change`'s remediation lists `CONTRACT_CONSUMERS`, so contract-gate's label stands for a
    confirmed multi-repo change, the article's confirmation gate.
  - Acceptance: a fixture with a vendored spec behind a breaking upstream change reports it; on
    7ed-emergencies' upstream specs the run is quiet or names real drift.
- **P5.3 — recorded defaults (G15).** The PR template gets an "Assumptions" section: each default chosen
  without asking, and where it sits in the diff. code-reviewer reports guessed values (timeouts, limits,
  names, fallbacks) that the diff adds and the section does not list. Advisory only: parsing cannot tell a
  guess from a decision.
  - Acceptance: run on a past PR diff, the reviewer names at least one unlisted default; the template change
    ships in a human-approved harness PR (P4.9 lists the PR template).
- **P5.4 — tests apart from the implementation (G16), as a trial.** A `test-writer` subagent that may write
  only test files gets the spec and the task, not the implementation, and writes the failing tests first
  (`/speckit-implement`'s test tasks). The implementer may not weaken them; code-reviewer flags assertions
  removed or loosened after the test commit.
  - Earned, not assumed: build two features this way and compare their mutation scores (P2.6) and review
    findings with two built the usual way. Keep it only if the trial shows fewer surviving mutants or fewer
    missed scenarios; record the result either way in the Log.
  - Acceptance: trial result and decision recorded; if kept, an ADR and the agent file in a human `.claude`
    PR.

## Open assumptions

- **A1 — feature spec rule.** `platform` middleware (P1.2, P1.3) and scaffold changes touch `apps/`. Under
  `specs-required-for-features` (`scripts/harness/check-specs.sh:54-75`) a `feat` commit there needs a spec.
  These are infrastructure, so this plan is their spec (golden principle 7). They ship as `feat(platform)`
  with the `no-spec-needed` label and a link to this plan, or as `refactor`/`chore` where no behaviour is
  added.
- **A2 — offline Trivy.** `--scanners secret --offline-scan --skip-db-update --skip-check-update` makes no
  network call. A network-off run gates Q1: it must pass before the rule enters `make check`.
- **A3 — Trivy speed.** The secret scan of the tracked-file input set is assumed to fit 5 s. If not, see
  NFR-H9.
- **A4 — Trivy coverage.** Trivy's Go scanner reads `go.mod` (module-level). It misses `govulncheck`'s
  call-graph precision but covers modules `govulncheck` never sees. Keep both: `govulncheck` for the root
  module, Trivy for breadth.
- **A5 — observability image.** `grafana/otel-lgtm` is assumed small enough for one container per worktree
  (~500 MB of RAM). Measure it on the reference machine in P1.1. If it is too heavy, use the JSON-file
  collector. **Held (P1.1):** 0.53 GiB idle, 3.7 s to ready on an M4 Pro / Docker VM 7.65 GiB; ADR 0017.
- **A6 — CI agent token.** P3.2 needs an API key as a repo secret and a hard per-PR cost cap. If unavailable,
  the reviewer runs locally from the PR template checklist only.
- **A7 — mutation tools.** `gremlins` (Go) and Stryker (TS) are maintained and pinnable in `.bin/`. Verify
  in P2.6.
- **A8 — Trivy choices (Q1).**
  - **Version.** v0.76.0 (2026-10-14) is the newest release ≥ 14 days old; v0.76.1 (2026-10-21) may be
    adopted from 2026-11-04. The four sha256 pins match upstream `checksums.txt`, checked independently by
    the orchestrator. cosign was not on PATH, so the signature check is a human TODO. Atlas pins match
    ariga's `.sha256` files. ShellCheck publishes no checksum file, so its pins come from the GitHub asset
    digests.
  - **A2 held.** With `HTTP(S)_PROXY=http://127.0.0.1:9`, the clean full tree passes and a planted key is
    found. Without the skip flags, Trivy's version check hits the proxy and is refused, which shows the
    blackhole works. So the rule is in `make check`.
  - **Input set.** A temp copy of `git ls-files -co --exclude-standard`, kept outside the repo. A copy under
    `.cache/` raced eslint under `-j4` (ENOENT in `check-all`), which breaks NFR-R1. A bats case now asserts
    the copy is outside the repo.
  - **No waivers.** Trivy's built-in path allow-list is off (`scripts/harness/trivy-secret.yaml`); before,
    keys in `*.md` and `*_test.go` passed. `--ignorefile ''` is set and `TRIVY_*` is unset.
  - **Vuln DB.** Pulled from `ghcr.io/aquasecurity/trivy-db:2`, not the default `mirror.gcr.io` (a Docker Hub
    mirror). `--include-dev-deps` is on. The scan fails closed on a manifest with no packages.
  - **Open, for a human:**
    - No `--ignore-unfixed` or severity floor (the plan's command as written). An advisory with no fixed
      release turns `check-all` red with no way to follow the remediation (`go-no-known-vulns` has the same
      exposure).
    - `trivy-vuln` has no budgets row: no NFR covers `check-all` sensors.
    - `actions/cache` saves only on job success, so a red run re-downloads the DB (~1.6 GB).
- **A9 — pre-commit choices (Q2).**
  - **Index only.** Staged files are listed with `git diff -z --cached` and copied with `git checkout-index`
    to a temp tree outside the repo; the per-file sensors run on a copy of the whole index, so `source`,
    `go.mod`, configs and link targets resolve as committed. A relative `GIT_INDEX_FILE` (`git commit
    <path>`, `-a`) is made absolute.
  - **Go format** uses `golangci-lint-custom fmt --diff` with `apps/api/.golangci.yml` (gofmt + goimports, as
    `make fmt`) rather than bare `gofmt`.
  - **Shared table.** `scripts/harness/file-sensors.sh` classifies files. Wiring `.claude/hooks/fast-check.sh`
    to source it, and naming the pre-commit hook in AGENTS.md "Working agreements", are protected changes
    proposed for the PR description, not made.
  - **Open, for a human:** bats takes ~44–48 s in `make check` (NFR-H5, 5 s; ~34 s before Q2, ~5.5 s of the
    rise is `pre-commit.bats`, which runs real Trivy and golangci). Warn-only; no budget raised. Belongs in
    the tech-debt tracker or a split.
- **A10 — fixture-claims grammar (Q3).**
  - A claim is a backticked `fixtures/<dir>` on a rule's `**Fixture:**` line; each must exist and have
    `RULE=` that rule. Otherwise the line says `none (<reason>)`. A bare `none` fails.
  - The gate reuses `scripts/harness/unchanged.sh` on `docs/harness/rules.md` and `scripts/harness/fixtures/*`,
    so `make check` skips it when neither changed.
  - **Open, from review:** a `none (…)` that names a bats file is not checked; fenced code blocks in rules.md
    are ignored; the Makefile repeats the `unchanged.sh` gate pattern per target (a `gated` macro would
    remove it).
- **A11 — structured-logging choices (Q4).**
  - `forbidigo` runs with `analyze-types: true` and `pkg:` regexes, so a renamed `log` import is caught and a
    `*slog.Logger` variable named `log` is not. The pattern is `^fmt\.Print(f|ln)?$`: `fmt.Fprint*` to an
    explicit writer stays allowed (`cmd/api/main.go` writes to stderr).
  - "No std `log` outside platform" is an `exclusions.rules` entry on `internal/platform/` that matches the std
    log message only, so `fmt.Print*` stays forbidden in platform too.
  - sloglint's message carries no rule id, so a sloglint-only failure prints every `lint-go` remediation; the
    fixture's EXPECT is sloglint's own text.
  - **Proposed protected change, not made:** add `go-structured-logging` to the rule list in
    `.claude/hooks/fast-check.sh` (the slice subagent edited it; reverted, since §5c allows `.claude/` only
    for slices that name a skill or agent). Until then the per-edit hook flags violations but prints no
    remediation for them.
  - **Open, from the slice:** `fmt.Println` is forbidden in `_test.go` too, so the first testable `Example`
    needs a reasoned `//nolint`; builtin `print`/`println` are not forbidden (out of the decided scope);
    run.sh routes remediation by rule id only (a linter → rule map would fix sloglint and gofmt output).
- **A12 — doc-drift choices (Q5).**
  - **pnpm.** pnpm 12.15.0's default `minimumReleaseAge` is 1440 minutes
    ([pnpm.io](https://pnpm.io/settings/dependency-resolution#minimumreleaseage); the pinned pnpm's
    `CHANGELOG.md`, 12.3.0). Setting it explicitly turns `minimumReleaseAgeStrict` on, so strict is pinned to
    `false` and behaviour is unchanged (`.bin/pnpm config get minimumReleaseAge`: `undefined` → `1440`;
    `pnpm install --frozen-lockfile` passes). **Open, for a human:** turn strict on (fail on versions younger
    than 24 h instead of recording them in `minimumReleaseAgeExclude`).
  - The NFR tables moved now, not "once this plan is accepted": ADR 0013 already asks for the move. NFR
    "Today" cells are now updated in `docs/harness/nfr.md`. Each plan keeps its `## NFRs` heading as a
    pointer, since ADR 0013 names it.
  - Plan 0002's phases read "done" (plan 0001's convention), with phases 1–2 "exit criterion not met"
    (NFR-H3 over budget for a Go edit; the 10× repo never measured).
  - **Proposed, not made:** protect `pnpm-workspace.yaml` (it now holds the release-age policy SECURITY.md
    cites) in `protected-paths.txt` and CODEOWNERS; move plan 0002 to `completed/`. ADR 0013's and ADR 0015's
    "until the tables move" conditions are now met; their status lines are a human call.
- **A13 — gosec choices (Q6).**
  - Every gosec check at its defaults, no `settings.gosec`, none disabled; tests in scope (as Q4), generated
    code excluded. The API module had 0 findings, so no code changes or `//nolint` were needed.
  - **Coverage gap found by the orchestrator:** a planted `exec.Command(name)` with `name` a function
    parameter was not flagged (G204 in the pinned gosec does not fire on it); a planted `crypto/md5` was
    (G401/G501). So G204 is weaker than its name suggests; review still owns subprocess calls.
  - `apps/api/tools/harness` and `tools/analyzers` are separate modules that `lint-go` never reaches, so no
    golangci rule (gosec included) covers them; `tools/harness` reads variable paths (likely G304).
    Candidate for the tech-debt tracker.
  - **Proposed, not made:** add `go-sec` (and Q4's `go-structured-logging`) to the rule-id list in
    `.claude/hooks/fast-check.sh`, so an edit-time gosec finding prints this rule's remediation.
- **A14 — observability stack choices (P1.1).**
  - Image pinned as `name:tag@digest` (multi-arch index digest), newest tag ≥ 14 days old; the pin moves to
    `scripts/tool-versions.env` in P1.4. `POSTGRES_IMAGE` there is still tag-only (candidate TD row).
  - Ports +3..+7 of the worktree block, `127.0.0.1` only. P1.4 must widen `scripts/setup-worktree.sh` (the
    free-block probe beyond `base+2`, `reserved_ports` beyond `DB|API|WEB`), or two worktrees can collide.
  - Pyroscope ships in the image and runs unused; part of the measured 0.53 GiB.
- **A15 — instrumentation choices (P1.2).**
  - `otelpgx` v0.13.0 (third-party, pre-1.0) rather than an in-repo `pgx.QueryTracer`: OTel semconv names,
    no query arguments recorded by default. `otelhttp` v0.72.0; `httpsnoop` promoted from indirect to
    direct. ADR 0018 (proposed) amends ADR 0002's "standard library only for transport".
  - Only the W3C TraceContext propagator. A client's `traceparent` and valid `X-Request-Id` are trusted
    until production (SECURITY.md).
  - An invalid incoming ID is replaced, not rejected; the problem `instance` is `urn:request-id:<id>`.
  - `accesslog` writes `r.Pattern` onto otelhttp's request copy so the span gets the route (an otelhttp
    internal); `TestHandler_CorrelatesResponseLogAndSpan` guards it across otelhttp bumps.
  - **Proposed, not made** (protected or out of slice): a depguard rule keeping otelhttp, otelpgx and
    httpsnoop inside `internal/platform`; a Spectral rule that every 2xx declares `X-Request-Id` (after
    TD-16); the `add-endpoint` skill naming the header; the `new-domain.sh` 503 setting `instance`.
- **A16 — telemetry export choices (P1.3).**
  - OTLP over HTTP only, one exporter per signal; `console` (the spec's name) is rejected, `stdout` is ours.
    Config owns the four variables above; the code endpoint wins over the per-signal `*_ENDPOINT`
    variables and `OTEL_EXPORTER_OTLP_PROTOCOL` has no effect. The SDK still reads headers, timeout,
    compression, certificates, the metric interval, BSP/BLRP and the sampler itself.
  - `service.name` is always `api`: `OTEL_SERVICE_NAME` is ignored, other `OTEL_RESOURCE_ATTRIBUTES` keys
    are merged (`TestSetup_ServiceNameIsFixed`).
  - otelslog v0.21 and otlploghttp v0.23 are pre-1.0; grpc, genproto and protobuf still arrive through
    `go.opentelemetry.io/proto/otlp`. ADR 0021 amends ADR 0018's dependency list and ADR 0002's logging
    bullet. x/net v0.62.0 clears the advisory the exporters pulled in.
  - **Proposed, not made** (protected): depguard rule `go-otel-sdk-only-in-telemetry` (otel SDK, exporters
    and bridges only in `internal/platform/telemetry`, otelpgx only in `internal/platform/db`, tests exempt),
    with a fixture and a rules.md entry.

- **A17 — scenario ids.** Positional ids (`US1-AS2`) are assumed stable enough, since merged specs rarely
  renumber. If P5.1 shows churn, write ids into the spec instead, which changes spec-kit's template and so
  needs a `.specify` harness PR.
- **A18 — upstream access.** The weekly drift run is assumed able to read an adopter's upstream specs. If
  their source repos need credentials, use a read-only repo secret, or run the check locally via `make` and
  paste its result into the drift issue.
- **A19 — stack profile choices (P4.7).**
  - `STACK` lives in `scripts/harness/layout.env`, `HARNESS_STACK` overrides it for one run; a missing
    `STACK` means `go:db:web` so readers of older layout.env copies keep working, an invalid value fails
    closed. Leaving out a group whose git-known files exist is red (`harness-layout-valid`).
  - A tool a stack without web still needs goes to `.bin` (spectral, squawk, bats); web-only tools stay
    in the lockfile (ADR 0023 and 0027 narrowed accordingly). spectral bundles a Node runtime no scanner
    reads (TD-86). Linux release digests match GitHub's but were installed only on macOS; CI's first
    `make setup` is the proof.
  - The acceptance copy also needed links pruned that the plan did not name: AGENTS.md's links to
    `apps/api/db/`, `apps/web/` and orval, and `docs/references/orval-llms.txt`. Only `setup check
    harness-selftest` were run on it, not `check-all`.
  - NFR-B1/B2 (Postgres and browser budgets) do not apply without db or web (nfr.md notes).

## Log

- 2026-10-28: plan drafted from the comparison against OpenAI, Böckeler (two articles) and secondary sources.
  Confirmed CVE-2026-33634 (Trivy release pipeline, March 2026) before choosing the install path. Q1 avoids
  every compromised channel: Action tags, Docker images and unsigned downloads.
- 2026-10-28: code-reviewer found 16 issues. All are folded in above:
  - fixtures dir and git-only input set for Trivy; one offline flag set
  - no network fixture; sha256 pinning instead of install-time cosign
  - a feasible pre-commit budget; budgets row for NFR-H9
  - B1/O1 measured by probe and integration test; ADR for the new NFR classes and anticipated rules (Q0)
  - the Q3 grammar against the real rules.md; `apply.sh` fixture
  - contract and boundary steps for the request ID; P3.2 trigger and token scope
  - per-slice harness marker; claim fixes
- 2026-10-28: Q0 done (3b112a9). ADR 0015 proposed; ADR 0013 left unchanged (ADRs are never rewritten).
  `make check` and `make harness-selftest` (32/32) green. `make check` ran bats (34 s, over its 5 s NFR-H5
  budget, warn only) because the branch's harness commits differ from `main` (ADR 0014 fail-closed).
- 2026-10-28: Q1 done (d5cbcdd) after one retry. The orchestrator's first `make -k check-all` failed in
  eslint (ENOENT on Trivy's in-repo temp copy, see A8). Once the copy moved outside the repo:
  - `make check` and `make test-integration` green;
  - `make harness-selftest` 33/33, including `repo-secret-planted`;
  - a planted untracked key failed with rule id and remediation, offline;
  - in `make -k check-all`, only `harness-config-not-loosened` fails, on this slice's protected files (the
    human gate).
- 2026-10-28: Q2 done (817b08c). Rechecked by the orchestrator:
  - `make check` green (bats 48 s, over NFR-H5, warn); `make harness-selftest` 33/33;
  - a staged fake AWS key blocked a real `git commit` with rule id `repo-no-secrets` and remediation; no
    commit was made, the probe file was removed;
  - in `make -k check-all`, only `harness-config-not-loosened` fails (the human gate).
- 2026-10-28: Q3 done (d560f7f). Rechecked by the orchestrator:
  - `make check` green (bats ~46 s, over NFR-H5, warn); `make harness-selftest` 39/39, including the six new
    fixtures;
  - a planted wrong claim in the real rules.md (`web-no-dead-code` naming `fixtures/web-type-error`) failed
    with rule id `rules-fixture-claims` and remediation; restored;
  - in `make -k check-all`, only `harness-config-not-loosened` fails (the human gate).
- 2026-10-28: Q4 done (cd8afac). The subagent's edit to `.claude/hooks/fast-check.sh` was reverted (A11).
  Rechecked by the orchestrator:
  - `make check` green (bats ~46 s, over NFR-H5, warn); `make harness-selftest` 41/41, both new fixtures;
  - a planted `fmt.Println` in `internal/platform/health` failed `make lint-go` with rule id
    `go-structured-logging` and remediation; removed, green again;
  - in `make -k check-all`, only `harness-config-not-loosened` fails (the human gate).
- 2026-10-28: Q5 done (d565e94). Rechecked by the orchestrator:
  - `make check` green (bats 49 s, over NFR-H5, warn); `make harness-selftest` 41/41; `docs-links` and
    `fixture-claims` green;
  - in `make -k check-all`, only `harness-config-not-loosened` fails (the human gate); check-all stops there,
    so `make test-integration` was run on its own: green.
- 2026-10-28: Q6 done (e919bed). Rechecked by the orchestrator:
  - `make check` green (bats 58 s, over NFR-H5, warn); `make harness-selftest` 42/42, including
    `go-sec-file-inclusion`;
  - a planted `crypto/md5` hash in `internal/platform/config` failed `make lint-go` (G501, G401) with rule id
    `go-sec` and remediation; removed, green again. A planted `exec.Command(name)` was not flagged (A13);
  - in `make -k check-all`, only `harness-config-not-loosened` fails (the human gate); `make
    test-integration` run on its own: green.
- 2026-10-28: Phase 0 gate. `make -k check-all` failed only on `harness-config-not-loosened` (the human
  gate); `make test-integration` green; `make harness-selftest` 42/42. code-reviewer: changes requested, 12
  findings; architecture-reviewer: 5 major, 8 minor. Fixed in 77b9c51 (secret scan sees fixtures, Trivy exit
  codes 3→1 / other→2, fixtures `repo-secret-in-markdown` and `repo-secret-in-go-test`, fail-closed only for
  real manifests, duplicate rule ids, partial downloads, `pnpm-workspace.yaml` protected, doc fixes, ADR 0016,
  TD-6..TD-15). Rechecked by the orchestrator: `make check` green (bats 54 s, warn); selftest 44/44; a key
  planted in `scripts/harness/fixtures/plant.md` failed `make secrets` with `repo-no-secrets` and remediation,
  green after removal. Exit criterion, with evidence:
  - secrets: `repo-secret-planted`, `-in-markdown`, `-in-go-test` fixtures and the plant above;
  - JS and nested-Go vulns: an untracked `go.mod` (old `x/crypto`) and `pnpm-lock.yaml` (lodash 4.17.15)
    failed `HARNESS_SCOPE=full make vuln-deps` with `deps-no-known-vulns`; removed;
  - `make fixture-claims` green; every drift item verified fixed (apps/web/AGENTS.md, `minimumReleaseAge`,
    "<60 s" wording, plan 0002).
  Not fixed here, with why:
  - ADR 0015's "team-convention" framing and status: a human call, ADRs stay `proposed`.
  - Sensor candidates (rule lists in sync, file-kind parity, tool downloads pinned): deferred; the rule-list
    one needs a `.claude/` change (TD rows).
  - DB-fetch split, CI cache save on failure, `TRIVY_` env prefix: tech-debt rows, not gate blockers.
  - The `fast-check.sh` rule-list change stays proposed for the PR description (A11, A13).
  - Raised by the fix subagent: `run.sh` still prints the secret remediation after "trivy failed to run",
    and a failed copy or missing Trivy exits 1, not 2 (with TD-6); agents can no longer edit
    `pnpm-workspace.yaml`, so pnpm's `minimumReleaseAgeExclude` entries need the human label (accepted: the
    file is install policy).
- 2026-10-28: P1.1 done (55a6915). ADR 0017 picks `grafana/otel-lgtm:0.36.0`: 0.53 GiB idle, 3.7 s to ready,
  under A5's 1.5 GB / 90 s, so the bare collector + `jq` fallback is rejected (PromQL gives NFR-O1's p95).
  Verified: only the ADR and its index row changed; `make check`, `make docs-links` green; selftest 44/44;
  no slice containers left. NFR-O1 unchanged (fail) until P1.4/P1.5.
- 2026-10-28: `make rename` (8424375, a user request outside the slices) reviewed by code-reviewer; fixed in
  the follow-up commit: values holding a template token are refused (re-runs stay no-ops), tracked files
  only, leftover forms listed, stricter module/owner syntax, `WRITE=1` only, README order. Scope note: the
  rename lives in `.bin/harness` although it is no sensor or hook, widening ADR 0012's scope for its
  portable file handling; the PR description proposes this. Not fixed: anchors into skipped files
  (`make check-all` docs-links catches them).
- 2026-10-28: user request: G11 (orphans and duplication) added as P3.6–P3.8 after P3.5. Split by what a
  machine can decide: unused scripts/targets and dead Go code (P3.6) and token clones (P3.7) are sensors;
  only re-implementing an existing concept needs the advisory `reuse-reviewer` agent (P3.8), which waits
  for both sensors so it doesn't duplicate them.
- 2026-10-28: user request: the code-simplifier agent, trialled read-only on the rename tool, found
  reuse and duplication that `code-reviewer` doesn't check for; applied in 1ec83b1 (`refactor(harness)`).
  Proposal for the PR description: a "Simplicity" check in `code-reviewer.md` (protected).
- 2026-10-28: user request: the loop runs up to three slices in parallel, one lane (own worktree, Postgres
  and ports from `make setup`) each; only the orchestrator commits, by cherry-picking one lane at a time and
  rechecking here. Slices sharing `drift.yml` or a named file, and slices measuring an NFR in seconds, don't
  run side by side; gates review their slices' shas, not a range. Each slice is reviewed once, by the
  orchestrator: P1.2's subagent ran both reviewers and the orchestrator ran them again. No `make db-down`
  target exists, so closing a lane calls `docker compose -p <project> down -v` directly (worth a target).
  P1.2 still finishes the old way, in this worktree.
- 2026-10-28: P1.2 integrated (4e2d8bd). The summary of the previous context claimed reviewers were
  running; none had been started, so the orchestrator ran `code-reviewer` and `architecture-reviewer` once.
  Findings sent back in one retry: ADR 0018 (blocking), header re-applied by the middleware instead of
  echoed by handlers, otelhttp behind `telemetry.Middleware`, WithGroup rule + TD-18, TD-19 (globals),
  contract-test cut, `http.StatusText`. Deferred: `telemetry.Setup` span processors, a problem helper
  (P2.1), `/healthz` noise in logs and traces (P1.3 decides). Fold 1ec83b1 (protected harness Go, no
  reviewer run) into the Phase 1 gate review.
- 2026-10-28: P3.1 integrated (eb61f5e). `code-reviewer` found 3 blocking gaps, fixed in one retry: the ADR
  now amends "every new sensor ships complete" for inferential sensors, places them outside the harness NFRs
  instead of narrowing NFR-R2, and limits output to one issue comment because branch protection sets
  `required_conversation_resolution`. Decisions: no rules.md entry, fixture or budgets row (ci.md lists CI
  ones, `.claude/agents/` local ones); skip with a notice when the agent cannot run; "accepted" means the
  change links the finding, the first acceptance is a tech-debt row, the second a harness PR with
  `Status: earned (<f1>, <f2>)`; a per-run cost cap in a protected file instead of an NFR; `pull_request`
  only (ADR 0010 invariant 3), forks skipped, posting in a separate step; local reviewer calls are workflow
  steps, not gates. If ADR 0019 is rejected, drop P3.1 and revisit P3.2, P3.3, P3.4 and P3.8. Noticed
  (protected): nfr.md:17 and ADR 0015:39 should link ADR 0019 (P3.2 can add it with the ci.md "Required:
  no" row); the first earned rule adds `earned` to the rules.md status legend.
- 2026-10-28: README §(b) checked against two brownfield repos; both fail at `make setup` and at the rule
  drop (steps 7 and 6 of the rewritten §(b)), the one with a bare module also at `make rename` (step 4);
  details in G12. README rewritten in this change; code slices P4.6–P4.10 added.
- 2026-10-28: P2.5 integrated (f28e238, rebased from lane eb9a107). Review fixed in one retry: the drift
  job's GH_TOKEN moved into the one "Coverage baseline" step that reads the previous scheduled artifact; the
  `coverage` workflow is `continue-on-error` and not a required check; Vitest stays on 5.0.5 with
  `@vitest/coverage-v8` 5.0.5 (the 14-day age rule is for downloaded binaries, ADR 0016; npm has
  `minimumReleaseAge` 24 h). Decisions: generated Go files excluded by their `DO NOT EDIT` header; a
  workspace without a `coverage` script reports `-`, not 0 %; no threshold, budgets row or rules.md entry;
  the awk parser is TD-27 (renumbered from the lane's TD-16, taken by P1.2). If ADR 0020 is rejected,
  drop the drift step and keep `make coverage` as a local report. Noticed (protected): a
  `web-coverage-no-thresholds` rule would keep Vitest thresholds out; AGENTS.md's command table could list
  `make coverage`; the coverage workflow costs one extra Go + web test run per PR.
- 2026-10-28: P1.3 integrated (b43eeaf, rebased from lane 09af076). Review fixed in one retry: x/net
  bumped to v0.62.0 instead of a tech-debt row (no age rule covers Go modules); the new dependencies and
  the `OTEL_*` ownership got ADR 0021 (proposed); `otelhttp.WithServerName` so a client's Host can't set
  `server.address`; tests for the export-error path, for a failed Setup leaving the global providers
  unchanged, and with a timeout on every Shutdown; providers shut down concurrently; one `Exporter` type,
  validated once in config. Decisions in A16; TD-17 and TD-19 updated. If ADR 0021 is rejected, revert
  b43eeaf and revisit P1.4 and P1.5. Noticed: P1.4 sets the `OTEL_*` variables in `.env.local` once the
  collector port exists; Loki may need a label or structured-metadata config to index `request_id` from
  otelslog attributes (check in P1.4); bats ~46 s against its 5 s budget (NFR-H5, predates this slice).
- 2026-10-29: P2.1 integrated (6884b56, rebased from lane 5a2993f; tracker and ci.md conflicts merged,
  TD-16 from the lane, TD-17 from the branch). Review fixed in one retry: placeholders validated in one
  pass before anything is written, matched by `__[A-Z][A-Z0-9_]*__`; bats tests for a late-template
  placeholder and that every template is used, renders and passes gofmt; the integration test goes through
  `get()` and asserts the body; the generated `queries` interface is unexported; ARCHITECTURE.md and
  `apps/api/internal/AGENTS.md` name the generated tests; TD-28 (no transaction pattern for services) and
  TD-29 (a problem helper per handler). Deferred architecture NITs: a `go test -json` check that none of the
  probe's tests skipped (a `t.Skip` would pass unseen) and an integration target scoped to the probe (the
  probe runs the whole integration suite). TD-16's header half stays open. Noticed (protected): a
  `scaffold-templates-render` rule would keep the bats render test a named sensor; CODEOWNERS could list
  `/scripts/new-domain.sh` and `/scripts/templates/`; the undo command in `new-domain.sh` and its skill
  (`git checkout -- .` before the scoped `git clean`) discards every unrelated tracked edit. go-integration
  5.12–5.40 s against its 5 s budget and bats 44–52 s (NFR-H5) both predate this slice.
- 2026-10-29: P2.4 integrated (6a30e4d, rebased from lane 268532d; ADR README and tracker conflicts
  merged). Review fixed in one retry: ADR 0022 instead of a new `proposed` rule status; the helper renamed
  to `platform/approvaltest`, protected as sensor code and fenced off production by depguard
  (`go-test-helpers-only-in-tests`, fixture `go-test-helper-in-prod`); an optional case segment and a
  collision check in approve mode; non-string headers parsed before validation (array/object headers
  skipped, documented); media-type range fallback; fixtures excluded from the pathspec; zero approved files
  exits 2; TD-33 (tests do not share cmd/api's middleware chain). `golang.org/x/text` stays a direct
  require of tools/harness (the validator's localized messages need a printer; ADR 0022 says so). Rejected:
  a unified line diff on failure (reviewers read the git diff after `make approve`). The lane agent tried
  `HARNESS_CHANGE_APPROVED=1` once in its first pass, an approval override the loop forbids; the
  classifier refused it and the retry set none. Noticed (protected): add-endpoint step 4 should pin each
  status with `approvaltest.Verify` behind `requestid.Middleware` and a fixed `X-Request-Id`; the
  fast-check hook does not run `approved-check` on `testdata/approved/*.json` edits; an
  `api-approved-change-declared` PR-template rule; the P2.1 scaffold could emit an approval test once TD-30
  is paid. harness-tests 5.8–6.7 s against its 5 s budget (NFR-H5).
- 2026-10-29: P1.4 integrated (5b7e649, rebased from lane 7b5f0a8; tracker conflict merged, TD-32 between
  TD-33 and TD-31). Reviewed once, fixed in one retry; final shape: ADR 0017 amends ADR 0008 (port block
  +0..+7, `reserved_ports` reads every `*_PORT`, `.env.local` reused) and records that `jq` is a pinned
  download, that `make dev` turns the exporters off when nothing listens on `OTLP_PORT` (restart dev after
  `make obs-up`), and who owns an image reference (`POSTGRES_IMAGE` frozen per worktree in `.env.local`
  because of its volume, `OBS_IMAGE` read live from `tool-versions.env`, name:tag@digest); SECURITY.md
  names the loopback-only obs ports, unauthenticated OTLP and Grafana admin/admin; 16 bats tests for
  `obs-query.sh` against canned responses (TD-32). Verified by hand in the lane: `make logs
  REQUEST_ID=orch-check-2` → 1 line with its trace ID, `make traces ID=…` → the server span and its
  `pool.acquire` child, `make metrics QUERY='histogram_quantile(0.95, …)'` → 4.75 ms for `/healthz` (NaN
  until enough requests); NFR-O1's Today cell updated. Deferred: the NFR-O1 `check-all` integration test
  (pulls the 2.58 GB image), decided in P1.5 or at the Phase 1 gate. Hand-off: every existing worktree
  reruns `make setup` once (new ports and `OTEL_*` in `.env.local`); compose outside make is unsupported
  (`OBS_IMAGE` comes from make); a `$` in a make command-line value needs `$$` or the env form;
  `POSTGRES_IMAGE` stays tag-only (digest pin is a human TODO); jq 1.8.2 is from 2026-06-20. Noticed
  (protected): add the obs targets to AGENTS.md's "Also:" line; sensor candidates
  `compose-env-provided` and `compose-ports-loopback-from-block`.
- 2026-10-29: P2.6 integrated (4595942, rebased from lane c7486dc; ADR README, tracker and tool-versions
  conflicts merged: 0022 then 0023, TD-35/TD-34 above TD-33, `OBS_IMAGE` then `GREMLINS_VERSION`). Review
  fixed in one retry (the lane agent's connection dropped once mid-retry and was resumed): files picked by a
  first-parent tree diff (a `--no-ff` merge's files counted, bats case); measured settings in protected
  config (`scripts/harness/gremlins.yaml` via `--config`, `GREMLINS_*` unset; `*stryker.config.*` in
  protected-paths and CODEOWNERS; a Stryker config with thresholds or excluded mutators exits 2); ADR 0023
  extends 0020 and settles A7 (both tools exist, run, and are older than 14 days: gremlins 2025-12-05,
  Stryker 2026-08-14); `deps-no-known-vulns`' remediation names a scoped pnpm override, and the first one
  pins `typed-rest-client>qs` 6.16.1 (TD-34); file names checked on source paths only (`$`, `[]` allowed);
  workspace probe exit codes, and the same fix in coverage.sh (an unparseable package.json read as "no
  tests"); full Stryker setup remediation; gremlins under `GOFLAGS=-short`; TD-27 extended, TD-35 (one
  `-E` per Go file). NITs taken: the drift issue gets a mutation entry only on failure or timeout,
  scores and survivors go to the job summary, ARCHITECTURE links ci.md#mutation-testing; `--survived` and
  `make mutation-survived` removed (nothing reads them). Rejected: a helper shared with coverage.sh (two
  callers with different parsing; revisit with TD-27). Noticed (protected): `// Stryker disable` as a
  suppression form for `suppression-reason`; sensors `workspace-report-scripts` and
  `report-config-no-threshold`; 28 of 73 Go mutants are no coverage because integration-only code is
  uncovered under `-short`; bats 59–66 s against its 5 s budget (NFR-H5).
- 2026-10-29: P1.5 integrated (db4b1c1, rebased from lane 48c2923 onto 99ec605; `.gitignore` and tracker
  conflicts merged: Stryker lines then `.tmp/`, TD-37/TD-36 above TD-35). Decisions: the test boots its own
  API (OTLP on, free port) instead of relying on `make dev`; it attaches to a running obs stack, else starts
  and removes one; the p95 is tied to the run by `service.instance.id` and the `instance` label, so a
  stale series cannot pass; the API is stopped with SIGTERM so the exporter flushes; `obs-pull` is a
  separate sensor so the 2.58 GB download stays out of `obs-e2e`'s 30 s budget (its 120 s budget is a
  guess, re-set from the first CI run); `test-obs` joins check-all's last `-k` line. Review (architecture
  7, code 13) fixed in one retry, all accepted but the protected items below. Measured: `obs-pull` 0.1 s,
  `obs-e2e` 13.8–14.5 s cold, 5 s attached; a missing image (`OBS_IMAGE=…:0.0.0-does-not-exist`) exits 2
  with rule, remediation and docs. Branch recheck: check-all red only on harness-integrity and specs, so
  its second line did not run (TD-37); `test-integration` and `test-obs` run by hand were green after
  `make setup` re-ran on this worktree, whose env file predated the obs ports (exit 1, "no OTLP_PORT");
  one `vuln-go` red was a DNS failure and passed on rerun. Noticed (protected): ci.yml should pull and cache `OBS_IMAGE` before
  check-all; AGENTS.md and CLAUDE.md should name `make test-obs` and the query-runtime skill; the
  `Makefile:79` help text; Vite loses its colours through `tee`. TD-36: share the API boot helper with
  P2.3's `make e2e`.
- 2026-10-29: P3.2 integrated (1b33d8e, rebased from lane 76c57f8 onto 7911867; conflicts merged:
  `install-tools.sh` keeps P3.2's `claude` mode above the `goinstall` block and P2.6's gremlins line,
  ci.md keeps "Mutation testing" then "PR review", tracker TD-39/TD-38 above TD-37; the lane's TD-36/TD-37
  were renumbered TD-38/TD-39 before commit to avoid P1.5's rows). Decisions: trigger `pull_request` only,
  never `pull_request_target`; the PR's own workflow copy runs, so write collaborators are trusted with a
  dedicated, spend-limited key (ADR 0019); a gate job reads `pr-review.sh` from the base commit (a base
  without it skips) and builds the prompt, the agent job holds the key but no write token, a third job with
  `pull-requests: write` posts the fenced result after a leak screen; CLI flags (`-p --bare --restricted
  --strict-mcp-config --tools Read,Grep,Glob --disallowedTools 'mcp__*' --permission-mode dontAsk
  --settings --no-session-persistence --output-format json`) were checked against the docs, not run;
  `REVIEW_MODEL=claude-sonnet-5-5`; no JSON → red, cap or agent error → notice, leak screen hit → warning
  and no comment; Claude Code 2.1.300 (2026-10-14, past the age rule) pinned by the release manifest's
  sha256 (signature not verified); jq on the runner is unpinned. Review fixed in one retry. Noticed: a
  sensor `workflow-privileged-trigger-no-secrets` is missing (protected); the 60000-character diff cut is
  duplicated in drift.yml with drift; the confinement test needs a real key (dummy keys are rejected
  before tools load). Human: run `.bin/claude --help` once against the pinned flags, add the dedicated key
  and run the confinement test.
- 2026-10-29: Phase 1 gate. `check-all` red only on harness-integrity and specs, `harness-selftest` 47/47,
  `test-integration` and `test-obs` green (3f9fe1d). Reviews of 55a6915, 4e2d8bd, b43eeaf, 5b7e649,
  db4b1c1 and 1ec83b1: code 11, architecture 7. Fixed in d908681: `obs-e2e.sh` no longer sets the OTEL
  exporters and endpoint itself, so `test-obs` tests the worktree's own defaults (green after the change),
  and exits 2 when it cannot run; `harness rename` ignores the new module path in its leftover scan (the
  reviewer's `aheumaier/harness-template` token would have missed the SSH clone URL, which the rename turns
  into `aheumaier/<name>`); `test-integration` unsets every `OTEL_*`; `db-reset` reuses `down`; the
  obs-query restart hint and exit codes; compose's `OBS_IMAGE` message; ADR 0017 in present tense plus who
  owns the `OTEL_*` defaults (setup alone, frozen per worktree, not derived live: one owner without moving
  them); ARCHITECTURE.md codemap. Rejected: a run of the documented `rate(…[5m])` p95 in `test-obs` (a
  one-shot process has too few samples for `rate`, so it would be empty, not wrong; the skill's query is for
  a long-running `make dev`); a success-path cleanup bats case (it needs a fake API that listens and answers;
  every real `test-obs` run in check-all exercises that path); one stack probe and the obs-query constants
  test (TD-41); `make logs` falling back to `.tmp/dev/api.log` (optional, a second output shape to keep in
  step). Noticed (protected, `.claude/`): the query-runtime skill hard-codes "every 5 s", and AGENTS.md and
  CLAUDE.md should name `make test-obs` and the skill. Sensor candidates: `otel-env-single-owner`,
  `obs-query-matches-code`. `make check` warned bats at 84.9 s against its 5 s budget (NFR-H5, warn only; 59–66 s
  before P1.5). Exit criterion: met, by `make test-obs` reading one request's access line, server span and
  p95 back through `make logs`, `traces` and `metrics` on this worktree's own stack; Phase 1 implemented.
- 2026-10-29: P4.1 integrated (20c1021, rebased from the lane onto 0430737): ADR 0024 (proposed) adopts
  function length, file length, complexity and parameter count as `team-convention` maintainability sensors,
  core belief 3 names the exception. Deviation from "start at today's worst": TS uses the ESLint defaults
  (50 lines, complexity 20, 3 parameters, 300 lines) while the web app is a scaffold, because today's worst
  (15/5/1/62) plus "never raised" would force a suppression on the next ordinary component; Go is re-measured
  at P4.2's base (file length over all linted files, tests included). Tests are exempt from the per-function
  limits; revive runs with only `argument-limit` and `file-length-limit`; plan Q4's sloglint key-case check is
  not adopted. Code review: 10 findings, all fixed in one retry. Lane `check` green, `harness-selftest` 47/47.
  Noticed: golangci does not lint `tools/harness` or `tools/analyzers`; P4.2 (which needs P4.1) rewrites the
  `.golangci.yml:36` comment and wires one recipe per measure in `run.sh` (protected); the Phase 4 exit needs
  ADR 0024 accepted by a human.
- 2026-10-29: P3.3 integrated (f5b5d27, rebased from the lane onto 1b7aa0f; one conflict, the tracker's
  TD-40/TD-41 rows, both kept). drift.yml runs three read-only review agents (skills `review-security`,
  `review-data-handling`, `review-modularity`) on schedule or default-branch workflow_dispatch and appends
  their fenced findings to the drift issue; `scripts/harness/ci-agent.sh` is the CI agent contract shared with
  `pr-review.sh` (ADR 0019 amendment). Reviews: code and architecture, A1–A10 and C1–C18, all fixed in one
  retry (base-action replaced by the pinned `.bin/claude`; `${{ }}` kept out of `run:`; red only when the
  agent did not start or returned no findings array, caps and API errors a notice; minutes cap via
  `timeout -k 30` so it is a notice, not a red step). Deviation: TD-40 covers only `harness-nfr.yml`'s
  `head -c` cut, since drift.yml's was fixed here. Lane `check` green, `harness-selftest` 47/47, `lint-ci`
  green, `check-all` red only on harness-integrity and specs, `test-integration` and `test-obs` green.
  `.bin/claude` was never executed: human TODO after the secret is set, one workflow_dispatch drift run on the
  default branch (TD-38, ci.md "Turning it on"). Sensor candidates: `agent-skip-list-ids-exist`,
  `ci-no-marketplace-agent-runtime`, `ci-agent-job-least-privilege`.
- 2026-10-29: P2.3 integrated (afdfeee, rebased from the lane onto 49dba05; one conflict, the tracker's
  TD-36/TD-37/TD-38 rows: HEAD's TD-38 kept, the lane's TD-37 with e2e taken, TD-36 dropped). `make e2e`
  drives one journey with pinned Playwright (headless Chromium from `.bin/playwright`) against this worktree's
  `make dev`, in check-all and CI; skill `drive-app`; rule `web-e2e-journeys` (Fixture: none); budget row
  `e2e 30000` (NFR-H7, no value raised). TD-36 closed as obsolete: no second boot copy was written. TD-42
  (Playwright provenance per bump). Reviews: architecture 7 and code 15 findings, 13 fixed in one retry;
  rejected: root AGENTS.md/CLAUDE.md rows (protected, Noticed) and a playwright-config fixture (the config is
  a protected path, not a rule). Noticed: AGENTS.md/CLAUDE.md rows for `make e2e` and `drive-app`; bats suite
  ~80 s. Lane `check` green, `harness-selftest` 47/47, `lint-ci` green, `check-all` red only on
  harness-integrity and specs, `test-integration`, `test-obs` and `e2e` green. CI not run (actionlint only).
- 2026-10-29: P4.2 integrated (f49fc8d, rebased from the lane onto 075dcfb with no conflict). Size and
  complexity limits per ADR 0024: Go `funlen`, `gocognit`, revive `argument-limit` and `file-length-limit`;
  TS `max-lines-per-function`, `complexity`, `max-params`, `max-lines`; tests and Playwright journeys exempt
  from the per-function limits, not from file length; thresholds just above the worst non-test value at
  1b7aa0f, re-verified at 0bd8958. Rules `code-function-length`, `code-complexity`, `code-parameters`,
  `code-file-length`, one Go and one web fixture each. `run.sh` maps lint finding lines to their rule
  (`finding_pattern`, `web-lint` the ESLint catch-all; `go-sec`, `go-structured-logging`, `go-format` mapped
  too); `run.bats` table-driven, 13 cases. Review: code 8 findings, 6 fixed in one retry; fast-check.sh
  SIZE_RULES Noticed (protected `.claude/**`), one informational. Noticed: integrity counts every new fixture
  line as a loosened config line; check-all's second recipe line skipped after a first-line red (TD-37); bats
  ~85-90 s (NFR-H5). Lane `check` green, `harness-selftest` 55/55, `check-all` red only on harness-integrity
  (specs green since PR #4 merged), `test-integration`, `test-obs` and `e2e` green.
- 2026-10-29: Phase 2 gate. Reviews of the Phase 2 commits at 0bd8958: architecture 14 findings plus 4 sensor
  candidates, code 10. Fixed in fad6b28: ADR 0025 (browser e2e is the NFR-B2 sensor) and rules.md cites it;
  the browser installs only where e2e runs (`make e2e`, `make e2e-deps`, the setup action's `browser` input);
  `make mutation-scores` puts the weekly score table in the drift issue (ADR 0023 amended); `make
  test-scaffold` checks the generator probe's tests ran and passed, none skipped; coverage.yml no longer
  continues on error (ADR 0020 amended); `new-domain.sh` prints an undo hint scoped to its own files; the e2e
  test parses `/health` with the generated schema; TD-41 absorbs TD-36, TD-43..TD-49 record the deferred
  findings (scaffold approval tests, workspace report contract, loosened weekly, layout paths, protected
  config in apps/web, testkit, gremlins everywhere), TD-31 extended. Rejected: CI evidence for NFR-B1/B2 (the
  loop may not push; local evidence below, one `ci.yml` run is a human TODO that also updates nfr.md's Today
  cells, still **fail**); an e2e selftest fixture (`harness-selftest` runs inside `make check`'s budget and
  cannot boot `make dev` plus a browser; TD-42 keeps it); check-all's two recipe lines (already TD-37); a
  budget-row nit and one informational finding. Noticed (protected): the drive-app skill (lines 21-23) and
  the new-domain skill (line 29) predate `make e2e` and the scoped undo hint; AGENTS.md and CLAUDE.md should
  name `make e2e`, `e2e-deps`, `test-scaffold` and `mutation-scores`; drift.yml's header line 3; the
  code-reviewer's only-make-check hook blocks `git show`/`diff --cached`, so reviews read files only. Sensor
  candidates: `rule-status-cites-adopting-adr`, `workspace-report-scripts`, `report-config-no-threshold`,
  `probe-tests-not-skipped`, `scaffold-templates-render`, an undo-hint/probe-cleanup consistency check.
  Gate lane: `check` green, `harness-selftest` 55/55, `lint-ci` green, `check-all` red only on
  harness-integrity, `test-integration`, `test-obs` and `e2e` green; main after integration: `check` green,
  `harness-selftest` 55/55. Exit criterion: met locally. NFR-B1: `make new-domain NAME=zzz_probe`, `make
  check` and `make test-scaffold NAME=zzz_probe` green, and the printed undo hint left the tree clean; NFR-B2:
  `make e2e` green; coverage per package: `make coverage` printed 15 rows (Go packages and JS workspaces);
  mutation: `make mutation` then `make mutation-scores` (the table drift.yml puts in the issue) printed apps/api 61.5 % (91 mutants) and apps/web 20.0 % (15), the two packages without tests named. CI confirmation of NFR-B1/B2 waits for the human run above; Phase 2 implemented.
- 2026-10-29: P3.4 done (14d7317). `scripts/harness/agent-gc.sh` (findings, targets, prompt, lock, baseline,
  added-line leak screen, confinement, patch, apply, body); agent job without token or cache save (setup action
  input `save-caches`), PR job in environment `agent-gc` with the token's own commit identity; `ci-agent.sh`
  shares `agent_structured`, `agent_nonce`, `agent_cap`; agent-gc.bats 27 cases, including the sensor
  candidates `ci-agent-writable-job-no-cache-save` and `ci-write-token-environment-scoped` as checks. Review:
  architecture CHANGES (repo-secret write token → environment secret on the default branch, non-admin
  identity; confinement over every post-agent path; post-agent code run from a writable tree; eight low),
  code CHANGES (15: body from edited paths only, prompt policy, tests, shared helpers, base branch); all fixed
  in one retry. Rebase onto 51523e8: setup action conflict with ADR 0025's `browser` input (the Playwright
  cache needs both `browser` and `save-caches`), TD-43 renumbered TD-50 (G2 took TD-43..49). Noticed: whether
  Claude Code's Edit works on a writable file in a read-only directory and whether `claude -p` writes to its
  cwd are unverified until TD-50's dummy-key run; auto-delete of head branches covers merged PRs only (ci.md
  says whoever closes one deletes it); agent-gc's findings source is `docs-links-resolve` only, so P3.5's
  staleness check must join `RULES` and the prompt. Lane `check` green, `harness-selftest` 55/55, `lint-ci`
  green, `check-all` red only on harness-integrity, `test-integration`, `test-obs` and `e2e` green.
- 2026-10-29: P4.3 done (a91b86d). `.harness-version` (template commit, `# declined` lines; protected),
  `HARNESS_UPSTREAM` in `tool-versions.env` (empty = this repo is the template, the check notes it and exits
  0; `make rename` writes it, `TestRenameSetsUpstream`), `scripts/harness/sync-check.sh` (throwaway bare
  fetch, marker/tip/working-tree three-way, exit 2 with a remediation when upstream or marker is unusable),
  drift.yml step, rule `harness-upstream-in-sync` with fixture `harness-behind-upstream`, sync-check.bats 17
  cases, ADR 0026. Review: architecture and code CHANGES, fixed in one retry; architecture's "URL in
  `.harness-version`" rejected per §4 (the URL stays in `tool-versions.env`, recorded in the ADR). Rebase onto
  3822912: ADR index, ci.md drift row and tracker (TD-45 → TD-51) conflicts. The plan's "upstream tag" and the
  loop prompt's "defaulting to `origin`" corrected to what shipped. Noticed: the sync fetch runs under the
  default 5000 ms step budget (2.9 s measured); `harness-version-well-formed` is a sensor candidate; the
  protected-path globs are now parsed in seven places (TD-39). Lane `check` green, `harness-selftest` 56/56,
  `lint-ci` green, `check-all` red only on harness-integrity, `test-integration`, `test-obs` and `e2e` green.
- 2026-10-29: P3.7 done (8e4f77b). `dupl` in `apps/api/.golangci.yml` (26; 25 trips `platform/problem`'s test),
  `run.sh` maps `(dupl)` to the rule; `make clones-web` runs jscpd with the protected `scripts/harness/jscpd.json`
  by `--config` (a stray `.jscpd.json`, `.config/jscpd.json` or `package.json` key is never loaded; fixture
  `web-clone-stray-config`), import and re-export statements ignored (two identical import lines already reach
  16 tokens); `jscpd:ignore` is a never-allowed marker in the integrity check; the `nolint:dupl` with
  `reason: … (rule: repo-no-clones)` is the Go exception, jscpd has none. Deviation from the plan's "in
  `check-all`": `dupl` runs in `make check`, since it rides lint-go at no measurable cost (warm golangci with and
  without dupl within noise, 0.7–1.1 s); TS clones surface only at `check-all` (ADR 0027). Cheap fixes before
  measuring: the scaffold's handler-test assertion into one `t.Helper` (dupl with a probe domain 41 → 26), the
  web providers into `AppProviders` plus `renderWithProviders` (jscpd 22 → 16), so the planned threshold
  tech-debt row was dropped. Review: architecture CHANGES (8: ADR, cheap fixes, cross-domain placement policy,
  npm native-binary supply chain, scan roots, wording), code CHANGES (9: thresholds were a baseline,
  remediation, jscpd escape hatches, fixture sizing, path stripping); all fixed in one retry. **Ground-rule
  breach:** the lane agent's first run used `HARNESS_CHANGE_APPROVED=1 make check-all`; that run was discarded
  as evidence, the agent was told never to set an approval override, and the retry's runs were plain. Rebase
  onto 9579f99: ADR index and tracker conflicts (TD-44/45 → TD-52, TD-45 dropped). Noticed: AGENTS.md's
  `check-all` row should name clones (protected); `fast-check.sh` does not run `repo-no-clones`, as for the size
  rules; the next Go ratchet step is a table-driven `problem_test.go` (then 23, `logging_test.go`);
  `new-domain.sh`'s undo hint (`git checkout -- . && git clean -fd …`) would wipe uncommitted work when run in a
  dirty tree. Post-commit security review: jscpd's import `ignorePattern` can span newlines and `format` lacks
  `jsx`, so a shaped TS clone can evade the sensor (TD-53). Lane `check` green, `harness-selftest` 59/59, `lint-ci`, `knip` and `clones-web` green,
  `check-all` red only on harness-integrity, `test-integration`, `test-obs` and `e2e` green.
- 2026-10-29: P3.5 done (5be0ad6). `.bin/harness quality-rows` with one table parser and two modes: `-no-clock`
  (`make docs-quality-shape`, rules `docs-quality-rows-well-formed` and `docs-quality-rows-match-domains`) and the
  clock mode (`make docs-quality`, `docs-quality-row-fresh`, age only, UTC, 30 days). Deviation from the plan's
  "the drift run flags rows older than 30 days or rows for deleted domains": the deterministic half (shape,
  deleted and missing domains) gates `check-all` and the required generator-probe; only the age check is
  drift-only, because a clock-dependent sensor in a required job breaks NFR-R1 and only the domain's reviewer can
  re-grade a row (ADR 0028). This reverses the P3.4 Log note that P3.5's staleness check must join agent-gc's
  `RULES`: agent-gc does not fix QUALITY rows until a grading agent exists (P3.4 bullet amended). `new-domain.sh`
  writes today into the scaffolded row; `TestQualityRowTemplateMatchesHeader` renders the real template under
  the real header. Fixtures `docs-quality-row-malformed`, `-missing`, `-orphan` (shape) and `-stale` (age).
  Review: architecture CHANGES (9, plus a template-parser test and a deterministic half), code CHANGES (13); the
  blocker was the clock sensor in the generator-probe; all fixed in one retry. Rebase onto 6a6a435: Makefile
  `check-all` list (clones-web and docs-quality-shape both kept), ADR index, tracker, ci.md drift row and
  drift.yml (header and Sweep targets: harness-sync-check and docs-quality both kept; two markers there were
  missed at first and caught by `lint-ci`, then rechecked).
  Noticed: `.claude/agents/code-reviewer.md:33` should ask for a grade per layer (protected); harness-integrity
  scans untracked binaries built in the tree; `harness-tests` 6.3 s over its 5 s budget after the new tests and
  bats over budget already (budgets untouched); agent-gc's "drop the link" fix would leave a bare domain name,
  which the shape check accepts. Post-commit security review: the marker is matched as a substring and the age
  check skips unparsable rows (TD-54). Lane `check` green, `harness-selftest` 63/63, `lint-ci`, `docs-quality-shape`,
  `docs-quality` and `clones-web` green, `check-all` red only on harness-integrity, `test-integration`,
  `test-obs` and `e2e` green.
- 2026-10-29: P4.5 done (d518888). Four references taken at the pinned tags (sqlc v1.31.1, oapi-codegen v2.8.1,
  Atlas v1.3.0, orval 8.45.0), each with `Version:`, `Pin: <file> <key>`, `Source:` and `License:` headers;
  `.bin/harness references-pinned` checks the header only (pin file git-known and inside the repo, exact
  version, Source names it as a path segment or tag). Deviations from the plan: the Progress line said "–"
  (no harness change), but the sensor touches the Makefile, `apps/api/tools/harness/`, fixtures, rules.md,
  protected-paths.txt, CODEOWNERS and AGENTS.md, so the PR is H; and "Drift fails when `tool-versions.env`
  moves" became `make check`, offline and under 1 s, so the PR that raises a pin fails instead of a weekly
  report (ADR 0029). `docs/references/*` is a guide path (CODEOWNERS review) so template sync reports it with
  the pin. Review: architecture CHANGES (12: plan/ADR wording, licences, supply chain, protection, pin file
  could name an ignored `.env` and print a secret, substring Source match, inline config copies), code
  CHANGES (9: same pin-file and Source issues, finding order, package.json ranges, sqlc "404 in the service"
  contradicting the layer rule); all fixed in one retry. TD-60 (orval `runtimeValidation` dead with
  react-query, verified in orval's source at the tag), TD-61 (the strict server checks no schema
  constraint, against golden principle 2), TD-62 (upstream commit next to the tag). TD-63 from the post-commit security review: a tracked
  symlink as pin file is read through and the finding echoes the pinned value. Accepted as is: an
  untracked, not-ignored pin file passes (git-known, as everywhere in the harness). **Ground-rule breach:**
  in its first run the lane agent removed `.tmp/p4-5-refs` with `python3 -I -c shutil.rmtree` after
  guard-bash refused `rm -r`; told never to work around a guard, the retry did not. Rebase onto 1ebd557:
  Makefile docs targets, harness `main.go` usage and dispatch, ADR index, tracker and rules.md. Noticed:
  `harness-tests` and bats over their NFR-H5 budgets (the references tests take 0.7 s). Lane `check` green,
  `harness-selftest` 64/64, `lint-ci`, `references-pinned` and `docs-quality-shape` green, `check-all` red
  only on harness-integrity, `test-integration`, `test-obs` and `e2e` green.
- 2026-10-29: P3.6 done (3edf728). `harness orphans <root>` flags a file under `scripts/` that no Makefile
  target, workflow, hook, skill, agent, guide or named script names, and a make target neither in `make help`
  (`name: ## …`) nor called. Paths match whole only (a `/` before one only after `.`, `..` or a variable);
  only the runner directories `scripts/test` and `scripts/git-hooks`, named in code, cover their files;
  executable referencers lose their comments, guides count whole. Runs in `check-all`, not `check`: mid-change
  a new script is an orphan until its caller lands (ADR 0031). Deviations from the plan: the help marker is
  `##` (not `## `), so the existing `check-all: check ##Everything` was fixed to `## Everything`; `deadcode-go`
  also scans tools/harness, and tools/analyzers stays out because deadcode cannot see golangci-lint's
  module-plugin entry points (TD-70). Review: code CHANGES (2 medium: a path matched under any directory, e.g.
  `apps/scripts/x.sh` or a URL, and any mention of a `scripts/` directory kept all its files alive; 10 low:
  help-line and target-specific-variable parsing, comments as callers, rules.md exclusions, ADR rationale,
  helper split, tests); all fixed in one retry. Rebase onto 6853cdd: harness `main.go` usage (orphans next to
  references-pinned), ADR index and tracker. Noticed: an earlier P3.6 `check-all` run listed P4.6 files under
  harness-integrity; my recheck did not reproduce it (watch for cross-worktree reads in integritycheck).
  bats and `harness-tests` over their NFR-H5 budgets (TestOrphans 0.2 s). Lane `check` green,
  `harness-selftest` 67/67, fixtures, `orphans`, `deadcode-go`, `fixture-claims` and `references-pinned`
  green, `check-all` red only on harness-integrity, `test-integration`, `test-obs` and `e2e` green.
- 2026-10-29: P4.6 done (449d561). `scripts/harness/layout.env` is data, never sourced: `layout.sh` parses
  and validates it (`--print` emits only the known keys), `layout.go` reads it for `.bin/harness`, the
  domainiso analyzer reads `LAYER_GLOB`; the setup action exports it to `$GITHUB_ENV`, migration-gate runs
  on every PR and gates only when `MIGRATIONS_DIR` changed. Acceptance grep 342 → 259 lines, the rest in the
  allowed classes. Deviations from the plan: `API_CLIENT_DIR` added (orval runs there); item "domainiso
  reports when `LAYER_GLOB` matches no package" taken literally turned the template red, so `layout-check`
  reports the empty glob and domainiso skips `platform` and fails flat packages (ADR 0030);
  `fast-check.sh` is under `.claude/**`, so it stays on the template layout until a human PR sources
  `layout.sh` (TD-66); the new-domain generator also encodes import paths and `routes.go` wiring and
  stays out (TD-67); golangci's analysis cache does not key on `LAYER_GLOB` (TD-65). TD-26 closed, TD-20,
  TD-25 and TD-46 narrowed. Review: architecture CHANGES (12) and code CHANGES (16), fixed in one retry.
  Rebase onto 5121d5d: Makefile `check` (layout-check next to references-pinned), `deadcode-go` (P3.6's
  recipe on `$(API_DIR)`), `.PHONY`, harness `main.go` (orphans and references-pinned moved into the new
  `rootSensors` map, quality-rows kept apart), ADR index, ci.md and tracker. Lane `check` green,
  `harness-selftest` 69/69, `orphans`, `deadcode-go`, `references-pinned` and `lint-ci` green, `check-all` red
  only on harness-integrity, `test-integration`, `test-obs` and `e2e` green.
- 2026-10-29: P3.8 done (ddd125f). `reuse-reviewer` reviews the diff only (`review-modularity` covers the
  tree), skips what `dupl`/`jscpd` (within one Go package; across packages is in scope, TD-52), deadcode,
  knip, orphans and the layer linters catch, generated code, fixtures and the new-domain scaffold, and
  reports only with an existing repo `path:line` or `<module>@<version> <pkg>.<Symbol>` the new code may use
  under the layer rule. It names globs and directories, never one script or target, so it keeps no
  orphan alive. Advisory (ADR 0019): no rule, fixture or budget row. Review: code CHANGES (10: location
  format, class granularity, clone scope, scaffold skip, `make help` wording, search list, injection
  defence, rule ids, unverifiable pinned API, CLAUDE.md trigger); all fixed in one retry. Noticed: the
  loop prompt's §5c does not call reuse-reviewer yet (protected, human hand-off); protect-harness takes
  its root from the session's project dir, so lane worktrees are unguarded (as guard-bash). Lane `check`
  green, `harness-selftest` 67/67, `orphans` and `docs-links` green, `check-all` red only on
  harness-integrity.
- 2026-10-29: P4.11 and P4.12 added at the user's request, from recommendations 3 and 4 of the harness
  assessment of this loop: reviewer catches seen twice become earned rules (ADR 0019), and sensor slices get
  an evasion review plus a diff-only re-review after a retry. Both are H; P4.12's loop prompt and CLAUDE.md
  wiring is a human step. Recommendations 1, 2 and 5 stay in the hand-off.
- 2026-10-29: Phase 3 gate. `check-all` red only on harness-integrity, `harness-selftest` 69/69. Reviews of
  the Phase 3 commits (eb61f5e, 1b33d8e, f5b5d27, 14d7317, 5be0ad6, 3edf728, 8e4f77b, ddd125f): architecture
  9 findings, code 6. Fixed in d8a6ff9: docs-quality reads the domains' directory from layout.env and fails
  closed; pr-review.bats checks every `.bin/claude -p` call for a cap and pr-review.yml caps the agent below
  the job timeout; agent-gc's guide filter; clones-web exit codes; ADRs 0015, 0019, 0028, 0030, rules.md,
  ci.md, QUALITY.md; TD-91..TD-94 record the rest (sensor devDependencies, review output `class` field,
  orphans' `make help` regex copy, hand-copied skip lists in protected agents). The fix's own review
  (APPROVE) left four low findings, deferred: the cap sweep misses `*.yaml`, composite actions and other call
  forms; the cap-below-timeout check covers pr-review.yml only, not drift.yml's jobs; exit 137 may be the OOM
  killer rather than the cap (guard with `SECONDS`); applied: the precise layout assertion and `writeLines`
  reuse. Exit criterion: met for the staleness check (`make docs-quality`), orphans, `deadcode-go` and
  `clones-web` in `check-all`, dupl in `check`, and `reuse-reviewer` (CLAUDE.md names it); not demonstrable
  here: a PR comment and scheduled-review/agent-gc issues, blocked on ADR 0019 acceptance, TD-38
  (`ANTHROPIC_API_KEY`) and TD-50 (`AGENT_GC_TOKEN`), and pr-review skips harness-only PRs. Noticed:
  `make check` was green while the pre-commit `go-format` sensor refused a one-line `layerDir` in
  layout.go, so `lint-go`'s scope can miss a format finding the hook sees. Protected proposal (TD-94):
  `.claude/agents/code-reviewer.md:33` should read "Suggest a grade (A–F, or – if not graded yet) for each
  layer (`model`, `store`, `service`, `api`) of each touched domain per docs/QUALITY.md, for a human to
  write into its row with a new `Updated` date; never edit docs/QUALITY.md yourself."
- 2026-10-29: P4.8 done (70e8066). `checkRenameArgs` takes `^(host|bare)(/elem)*$` and refuses malformed
  names, owners and modules with one error each; `moduleArg` maps `skip` to the template module, whose owner
  leftovers other than the template's own URL are still reported; `stripModule` strips the module only as a
  whole word, so a short module such as `harness` hides no leftover. Documented, not detected: a bare module
  rewrites `http://`, `pkg.go.dev/` and `go install …@latest` forms of the template URL unreported. Not fixed:
  a bare module named `aheumaier` would hide `git@github.com:aheumaier/…` leftovers. Review: code CHANGES (8:
  TestRenameArgs proved nothing, skip guard, dotted one-element module, identity pair, short module,
  malformed layout, two README lines), fixed in one retry with mutation evidence. Rebase onto ee79ecb clean.
  Lane `check` green, `harness-selftest` 69/69, `check-all` red only on harness-integrity, `test-integration`,
  `test-obs` and `e2e` green.
- 2026-10-29: P4.4 done (bb790ee). `run.sh` writes each rule's finding count to `timings.jsonl`;
  `scripts/harness/ci-timings.sh` (shared with `nfr-report.sh`) downloads ci.yml's timing artifacts within
  their retention; `scripts/harness/rule-fires.sh` validates every row and merges them into a v2 ledger
  deduplicated by artifact id. drift.yml's Baselines step fetches the last ledger (found / fresh / error);
  a gh error or a failing merge keeps the last ledger as the baseline and uploads none. ADR 0032 amends ADR
  0013; rules.md says rules also leave. Review: code and architecture CHANGES, fixed in one retry (shared
  timing reader, ledger dedupe, error states, ADR relation). Rebase onto eb4b28c: rules.md and the tracker
  conflicted, both sides kept. Lane `check` green, `harness-selftest` 69/69, `check-all` red only on
  harness-integrity, `test-integration`, `test-obs` and `e2e` green. Noticed, not fixed: the Baselines step
  needs `zip`/`unzip` on the runner; 13 failing drift weeks in a row let the ledger artifact expire, so the
  90-day history restarts; a zero-byte ledger artifact is treated as absent, not recorded.
- 2026-10-29: P4.4 security follow-up (0b9708e). An automated security review of the P4.4 commit: `ci-timings.sh`
  unzipped fork-controllable artifacts whole in the drift job. It now extracts only `timings.jsonl` (`unzip -p`)
  and drops artifacts of fork PR runs (head repository != repository); drift.yml's timings check uses
  `find -type f`. Same-repository PR rows are still trusted by shape only (ADR 0032, ci.md).
- 2026-10-29: P4.9 done (dbaaa39). The 8 workflows (the plan named 6; coverage.yml and pr-review.yml carry
  the harness header too), `.github/actions/setup/*`, CODEOWNERS and the PR template are listed by name;
  codeowners.bats keeps list, first-line header and per-file CODEOWNERS lines equal and every named file
  tracked; TestIntegrityCheckRealListSparesProductWorkflows runs the real list. Review: code CHANGES (8)
  and architecture CHANGES (7), both naming a HIGH: a product job named after a required check could stand
  in for it without label or owner review (branch protection matches any Actions app). Fixed in one retry:
  CODEOWNERS keeps `/.github/workflows/` and `/.github/actions/` owner-reviewed, ADR 0035 (amends ADR 0010),
  TD-95 (required-check name collision sensor), TD-96 (label actor check), TD-97 (listed workflows use only
  listed local actions); README step 3 no longer copies `.github/` whole; reuse finding (shared list parsing
  in codeowners.bats) fixed. Rebase onto the P4.4 commits: ADR index conflicted, both rows kept. Lane
  `check` green, `harness-selftest` 69/69, `check-all` red only on harness-integrity, `test-integration`,
  `test-obs` and `e2e` green. Hand-off: AGENTS.md golden principle 5 still says `.github/**`; proposed
  wording "`.github/` workflows and actions named in `scripts/harness/protected-paths.txt`". Noticed: a
  product workflow can use `pull_request_target` or broad `permissions:` gated only by owner review; input
  for P4.12's evasion review.
- 2026-10-29: Phase 5 (G13–G16) added from the Thoughtworks article "Engineering the harness" (Nandyala,
  Pani, 2026-09-29). Compared practice by practice: scoped guides, least-privilege reviewers, silent-success
  sensors, prose-to-rule promotion, earned and pruned rules and gates for contract, migration and harness
  changes are already here; spec-to-test traceability, cross-repo contract impact, recorded defaults and
  separate test authorship are not.
- 2026-10-29: P4.7 done (bfec770). `STACK` (default `go:db:web`) selects what `check`, `check-all`, `gen`,
  `tools`, `deps`, `setup` and `harness-selftest` run; spectral, squawk and bats are pinned releases in
  `.bin`. Acceptance: in a copy without `apps/web`, `packages/` and `apps/api/db`, `HARNESS_STACK=go make
  setup check harness-selftest` green (selftest 50 passed, 20 skipped). Review: code, architecture and
  reuse together 19 items, all fixed in one retry, none rejected; among them an absent STACK failing
  closed (now a default), a narrowed stack silently hiding a present group (now red, fixture
  `harness-stack-hides-web`) and ADR 0033 reconciling ADRs 0016, 0023 and 0027. The retry found that
  `make -n` is no dry run on this Makefile: recipe lines naming `$(MAKE)` still run, so the first
  make-stack.bats ran Playwright and obs-e2e inside every `make check`; the test now sets
  `MAKE='echo make'`. Rebase onto P4.4 and P4.9: ARCHITECTURE.md, the ADR index and the tracker
  conflicted, both sides kept. Lane `check` green, `harness-selftest` 70/70, `check-all` red only on
  harness-integrity, `test-integration`, `test-obs` and `e2e` green. Hand-off: `make -n check-all`,
  `make -n e2e` and `make -n test-obs` still run obs-e2e and Playwright (the Makefile is protected);
  the PR needs the `harness-change-approved` label.
- 2026-10-29: P4.12 done (8445e4d). `.claude/agents/evasion-reviewer.md`: read-only, plants bypasses in a
  throwaway `git worktree` with hooks off, cost cap 4 sensors × 30 attempts, 45 min; findings are
  `evasion:<rule-id>:<class>` and advisory (ADR 0019 gains a Decision bullet; ADR 0034 stays unused).
  Acceptance on 3edf728's diff with the pre-retry prompt: 9 of the 11 TD-75..78 bypasses reproduced; not
  rerun on the final prompt. Review: code-reviewer CHANGES (1 high: TD-107 named the wrong scratch path;
  6 medium, 8 low), reuse 6 items, 5 accepted, 1 rejected (TD-108 adds no code, so no SH_FILES idiom);
  all fixed in one retry. A mktemp copy was dropped because guard-bash refuses `rm -r` outside the
  project. Gaps TD-107..110. Lane `check` green, `harness-selftest` green, `check-all` red only on
  harness-integrity. Hand-off: the agent's proposed loop-prompt §5c/§5d text (run it on sensor slices;
  a retry gets a diff-only re-review), the slice-rule line and the CLAUDE.md review row; the PR needs
  the `harness-change-approved` label.
- 2026-10-29: G17 and P4.13 added. A load check found five `harness-*` Postgres stacks running, one from
  another session's scratchpad copy (`harness-go-only-r2`, stopped by hand with 7ed-emergencies' leaked
  `emergencies-ci`); teardown exists (`make down`, §5g) but only as prose.
- 2026-10-29: P4.11 done (0e03d34). Earned (ADR 0019), each with fixtures:
  `harness-rule-lists-match` (P3.8 lane review, TD-94 as opened in d8a6ff9; a `requiredSkips` manifest in
  rulelists.go names the ids and section claims each rule list must keep, fails closed on a file with no
  entry; goldmark code spans) and `harness-reads-layout` (TD-46 as opened in fad6b28, approvedcheck.go:99,
  fixed 449d561; d8a6ff9's qualityrows.go `domainsDir`; a test reads paths via loadLayout, folds + chains
  and Join arguments, whole path segments; rules.md lists what stays uncaught). Dropped candidates:
  `harness-sensor-deps-pinned` (one finding, TD-91) and `docs-make-targets-exist` (no finding, TD-103).
  Shared helpers `ruleHeadings` and `printFinding`. Review: code-reviewer CHANGES (1 high, 3 medium,
  5 low), reuse 5 items, R1 and R5 accepted, R2 rejected (run.sh:86 is live: `finding_pattern` makes the
  rule non-general) and R4 rejected (a make macro differs from the sibling targets); all fixed in one
  retry. Rebased onto 749e76a (tracker rows only). Lane `check` and `harness-selftest` green, `check-all`
  red only on harness-integrity, `test-obs` failed once under three-lane load and passed on rerun. Noticed: references.go:73 could use printFinding; protect-harness does
  not fire in lane worktrees (hand-off).
- 2026-10-30: P5.1 done (9f6771d). `harness specs-tested` traces each acceptance scenario to a test naming
  `NNN/US<n>-AS<m>` or an `(untested: <reason>)` exemption; goldmark parse, fails closed on a spec without a
  recognised story; web tests count except the API client and generated code; placeholder tests and reasons
  rejected; `protectedPaths` extracted from integritycheck.go. Review: code-reviewer CHANGES with 2 blocking
  (a spec with no story passed; `sc.Err` unchecked) plus medium and low items; reuse 6 items: R1 goldmark and
  R2 protectedPaths accepted, R5 not applied (rename.go needs Perm), R3 rejected (TD-113, one gated make
  macro), R4 rejected (P4.11's `printFinding` landed after; unify as a follow-up), R6 noted; all fixed in one
  retry. Rebased onto ed1d5c7 (three one-line list conflicts: ARCHITECTURE.md, Makefile `check`, main.go).
  Lane `check` green, `harness-selftest` 75/75, `check-all` red only on harness-integrity, int/obs/e2e green.
- 2026-10-30: P5.3 done (0117216). PR template `## Assumptions` (`path — value — why this value`, or "none");
  code-reviewer check 6 lists added defaults the section misses, matched by path and value, advisory only;
  without the PR body one grouped line. Acceptance: run on 1b33d8e it named the unlisted defaults at
  pr-review.yml:60-63 (REVIEW_MODEL, REVIEW_MAX_DIFF_BYTES 100000, REVIEW_MAX_TURNS 20, REVIEW_MAX_BUDGET_USD
  1.00) and `timeout-minutes` 15 (:68) and 5 (:157); negative controls stayed quiet (MAX_COMMENT_BYTES 60000
  has its comment, `5*time.Second` is in ADR 0021). Review: code-reviewer CHANGES (noise without the PR body,
  "one option chosen from several" too broad, contract and migration names, moved values, line-number
  matching, template/agent drift, TD-119 plan, agent-gc body); all 10 fixed in one retry. TD-119: CI review
  gets no PR body (two harness PRs: `pr-review.sh prompt` input, then the workflow with an `edited` trigger).
  Lane `check` green, `harness-selftest` 75/75, `check-all` red only on harness-integrity, int/obs/e2e green.
  Proposal (AGENTS.md, protected): Working agreements also name the Assumptions section.
- 2026-10-30: P5.2 done (cfce60f). `layout.env` gains `UPSTREAM_SPECS` (vendored path, source, spec path,
  revision) and `CONTRACT_CONSUMERS`; layout-check validates entries offline against shared vectors
  (`testdata/upstream-spec-vectors.txt`, 5 ok / 24 reject, also read by upstream-drift.bats).
  `upstream-drift.sh` (file/https only) compares each copy with its source's head through oasdiff; drift.yml
  runs it weekly as a report with a fence longer than any backtick run in the output; fixture
  `api-upstream-breaking` proves a break is reported. contract-check names the repos the approval must cover.
  `harness spec-refs` (yaml.v3, already in the tools module) replaces the `$ref` grep in both scripts.
  Review: code-reviewer CHANGES (2 blocking security: issue-fence break-out in drift.yml, `$ref` grep bypass),
  architecture 8 findings, reuse 7; one retry fixed 13 items plus a selftest collision (the new fixture left
  `.tmp/upstream` behind for `harness-behind-upstream`). Decided: the revision stays in `layout.env` as the
  plan says; ADR 0037 answers ADR 0026, calls the revision informational and the vendored copy unprotected
  (TD-117 offers a deeper fetch or a per-copy `COMMIT` file). Kept `GIT_ALLOW_PROTOCOL=file:https`: git applies
  `insteadOf` before the allowlist, so ssh-only users get an https credential helper instead. Not done: a shared
  fetch script (TD-116), a shared oasdiff wrapper (TD-118, contract-check treats exit 102 as a break), an
  exit-124 timeout test, other fixed fences in drift.yml. Lane `check` green, `harness-selftest` 75/75,
  `check-all` red only on harness-integrity, int/obs/e2e green. Proposal: code-reviewer checks that the PR body
  addresses each named consumer.
- 2026-10-30: P5.4 done (5b7ee88). Trial: four small spec'd features, two built test-first (a subagent that saw
  only spec, plan and tasks wrote tests committed red before a separate implementer ran), two the usual way.
  Mutation scores tied once equivalent mutants are set aside; scenario tracing tied; a blind review found one
  weakly checked case and three bugs the tests miss in the usual arm (one confirmed by a hand mutant), none in
  the test-first arm. By the plan's rule: KEEP. Caveats in ADR 0039 (proposed, amends 0019): n=2 per arm, one
  arm per feature, pure model functions only. Review: architecture CHANGES (11), code-reviewer CHANGES (16),
  reuse 2; one retry fixed all 16: writes confined by `.claude/hooks/only-test-files.sh` through the new
  `harness test-file` subcommand (same pattern as `specs-tested`, bats, budget row `only-test-files 50 NFR-H1`);
  only-make-check.sh's message speaks to any agent; the agent writes unit tests for model/service and
  packages/domain only, needs pinned names in plan.md, commits red with a `Test-Writer: specs/<NNN-slug>`
  trailer. Decided defaults: cost cap 4 test files, 3 `make check` runs, 30 minutes. code-reviewer check 7
  (advisory) lists assertions loosened since the first Test-Writer commit; skipped without a diff, so in CI
  today (TD-125). Not done: CLAUDE.md and constitution wiring (TD-124, hand-off), the gremlins mutator gap
  (TD-123), a re-measure on the first adopter (TD-126). Measured: the hook's median is 57 ms against its 50 ms
  budget under three parallel lanes; nfr.md's NFR-H1 row does not list it; TD-107 is stale. Rule candidates
  for a harness PR: `harness-hooks-tested`, `test-first-commit-untouched`. Lane `check` green, `harness-selftest`
  76/76, `check-all` red only on harness-integrity, int/obs/e2e green.
- 2026-10-30: Phase 5 gate. Reviews of 9f6771d, cfce60f, 0117216, 5b7ee88 together: code-reviewer CHANGES
  (10, harness grade B, no security defect), architecture-reviewer CHANGES (7). Main green after P5.4
  (`make check` 0, `harness-selftest` 76/76). Fixed in 1368b9b: one product test-file predicate for
  `harness test-file` and specs-tested (the guard let test-writer write into generated code, the API client,
  store packages and harness files) with shared vectors run through Go, specs-tested and the hook; after
  code-reviewer CHANGES on the fix itself (one retry), exclusions match ignoring case (`tools/Harness/` reached
  the harness package on macOS), `..` parts, an empty protected list and `?`/`[`/`\` patterns fail closed,
  gitignored product paths are a stated limit (TD-128) and `**/vite.config.*` joins check 7's runner configs;
  contract-check exits 2 on an oasdiff exit other than 1, so the label cannot approve an uncompared spec;
  check 7 covers every test-file form and is silent without the diff; hook message from `harness test-file`;
  ADR 0039 amends 0019 explicitly; hook median 24 ms (load 3–4) in nfr.md; tracker newest first, TD-107/118/125
  updated, TD-127 new; and contract-check.bats/upstream-drift.bats no longer assume optional `layout.env` keys,
  which P4.10's adoption probe caught. Deferred: the bash upstream entry parser (TD-127); the revision in
  protected `layout.env` (TD-117, settle before ADR 0037 is accepted); the Makefile `specs-tested` pathspecs
  remain a second spelling of the test-file forms. Rejected: renaming `spec-refs` to `openapi-refs` (naming
  churn for two callers). TD-114 and TD-120..122 are unassigned: ids were handed to lanes in flight and not all
  used. ci.md and rules.md#api-no-breaking-change do not yet mention the cannot-compare exit, and
  `protect-harness.sh` still compares case-sensitively (hand-off: protected `.claude/hooks/`; fixed 2026-11-02 by exec plan 0005 pilot 2). Exit
  criterion: scenarios traced or exempted in `check` and `check-all` (fixture `specs-scenarios-tested`; the
  template has no `specs/`); PR template Assumptions checked by code-reviewer check 6 (in CI without the PR
  body, TD-119); trial recorded in ADR 0039 and the P5.4 Log line; weekly upstream drift exists in drift.yml
  but has not run on a first adopter (TD-115: anonymous fetch exits 2 on private upstreams). State: partial.
- 2026-10-30: P4.10 done (84399b7). `adoption-probe.sh` (`make adoption-probe`) commits the skeleton in
  `scripts/templates/adoption-probe/` (root `go.mod`, `internal/<domain>/<layer>`, a contract, no db or web)
  into a temp repo, follows README (b) steps 1–7 with this checkout's harness and ends with
  `HARNESS_STACK=go make setup`, `make check`, `make harness-selftest` green there; step 3 and step 6 must name
  exactly the paths it copies, skips or drops (fixture `harness-adoption-stale-layout`). Required CI job beside
  `check-all`; skeleton is protected config owned in CODEOWNERS; ADR 0038 amends 0009. After the rebase onto
  P4.12/P5.1 the probe went red on two Go tests with a hard-coded `../../../..` root; the integration retry
  moved them to `repoRoot` and changed `harness-reads-layout-literal`'s EXPECT path from `apps/api` to
  `internal` (still one exact path, LAYER_GLOB's prefix, reported in both layouts). Second catch: P5.2's
  contract-check.bats edited an optional `layout.env` key the adopted repo lacks (fixed in 1368b9b). Third
  catch, after the rebase onto the Phase 5 gate: only-test-files.bats, timing.bats and upstream-drift.bats
  built paths under `apps/api` (G5, P5.2); fixed here from `layout.env`. Not counted as P4.10's second
  failure: the defect was in code integrated after its review round, outside its diff (§5f.3). The class
  recurs; rule candidate `bats-paths-from-layout` (hand-off). Declined review items:
  dropping `HARNESS_STACK=go`, a `.go-arch-lint` exclusion, a second red fixture. Not done: TD-99 (eleven
  bats files dropped instead of ported), TD-100, TD-101 (four shell helpers duplicated). Measured: the probe
  took 164 s locally on the final run against its 600 s NFR-H7 budget (1233 s earlier, under three parallel
  lanes); the third catch also fixed `api-upstream-breaking` (renamed `/healthz`, edited an optional
  `UPSTREAM_SPECS` line) and a test-writer.md link the probe drops; its timing artifact is not
  read by `ci-timings.sh` or `nfr-report.sh` yet (hand-off). Lane `check` green, `harness-selftest` 77/77,
  `check-all` red only on harness-integrity, int/obs/e2e green, `make adoption-probe` green.
- 2026-10-30: Phase 4 gate (56e35b9). On ccebaff: `check` green, `harness-selftest` 77/77, lint, links and
  orphans green, `check-all` red only on harness-integrity, int/obs/e2e green. Two independent reviews of the
  twelve Phase 4 slices plus an architecture review; the G4 fix lane closed the blocking findings:
  `migrations-append-only` let an applied migration be edited if it rode along with a moved
  MIGRATIONS_DIR (now every base migration must reach the new dir with the same blob); `make STACK=…` or
  `make -e` overrode `layout.env` without the decline the sync check reads (now a `harness-layout-valid`
  error, layout keys applied with `override`); the adoption probe and README hand-edited files the template
  sync owns (now ci.yml skips the probe job when the script is removed and declined in `.harness-version`;
  ADR 0038 narrowed). Also fixed: migration-gate's MIGRATIONS_DIR step runs under bash (pipefail), README's
  STACK table matches DB_TARGETS and WEB_TARGETS, the probe no longer needs `HARNESS_STACK=go`. Advisory
  findings not fixed: TD-129..136. Rejected: replacing testfile_test.go's hand list of protected paths
  with a read of `protected-paths.txt` (the test pins the template's list on purpose; drift risk is in the
  hand-off). One review round and one retry (§5d); the retry was rechecked here, not re-reviewed. The
  gate assessed the twelve committed Phase 4 slices; P4.13 (G17) was added outside this run and is not
  part of it. State `partial`: the exit needs ADR 0024 accepted, which is a human step. After the gate:
  `check` green, `harness-selftest` 77/77, `make adoption-probe` green.
- 2026-10-30: Finish (§6, 6f885ad). §6.1 on a133fad: `check` green, `harness-selftest` 77/77, lint, links and
  orphans green, `check-all` red only on harness-integrity, int/obs/e2e green, `harness-determinism` 0
  differences (NFR-R1) but 287.5 s against its 120 s NFR-H4 budget, measured beside two reviewers and a lane's
  `make setup`; rerun on the final HEAD with no lane running (hand-off). §6.2: `code-reviewer` and
  `architecture-reviewer` on `main...HEAD`, both CHANGES. The FIN lane closed the blocking one: a greenfield
  product rewrites README.md, losing (b), which turned the required `adoption-probe` job, the
  `harness-adoption-stale-layout` fixture and `adoption-probe.bats` red; a non-empty `HARNESS_UPSTREAM` in
  `tool-versions.env` (ADR 0026, read from the file only) now switches all three off (ADR 0038 amended, also
  amends 0026; fixture.conf key `TEMPLATE_ONLY`). Also: the ADR README states that proposed ADRs are amended
  in place and accepted ones superseded, marks 0034 unused, ADR 0033 names what it amends; a rules.md
  blank line; this plan's Phase 5 exit says `make check` (specs-tested runs there). FIN's own review round
  found that `adoption-probe.bats` still read the real `tool-versions.env` (a product's `make check` red);
  fixed in the retry (§5d) and checked on a simulated product copy. Not fixed: AGENTS.md golden principle 5
  still lists `.github/**` as protected (stale since ADR 0035; AGENTS.md is not the loop's to edit, TD-137,
  wording in the hand-off), TD-138..141. The PR cuts in the hand-off follow the shared files the review
  named (layout.go, testfile.go, protectedPaths, printFinding, ci-timings.sh, P4.10's edits to other
  slices' bats). Every ADR left proposed. After FIN: `check` green, `harness-selftest` 77/77,
  `make adoption-probe` green.
