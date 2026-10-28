# 0003 — Close the harness-engineering gaps

Status: proposed · Owner: @aheumaier · Started: 2026-10-28

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
| 1 | Runtime legibility (G1) | NFR-O1 met: logs by request ID and a p95 latency, per worktree, via `make` | proposed |
| 2 | Behaviour harness (G2) | NFR-B1 and NFR-B2 met; coverage reported per package; weekly mutation score in the drift issue | proposed |
| 3 | Inferential sensors and garbage collection (G3, G4, G8, G11) | reviewer comments on every PR (advisory); scheduled reviews and `agent-gc` open issues/PRs; QUALITY.md has a staleness check; orphan scripts, dead Go code and clones fail `check-all`; `reuse-reviewer` exists | proposed |
| 4 | Taste lints and harness lifecycle (G5, G7, G9, G10) | ADR amending core belief 3 accepted; size/complexity lints green; template version marker + sync check; per-rule fire counts in the drift issue | proposed |

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
- [ ] P1.2 — – — contract + platform middleware: request ID, access log, `otelhttp` server spans, pgx tracer
- [ ] P1.3 — – — OTLP exporter wired to config; metrics (RED per route)
- [ ] P1.4 — H — `compose.yaml` profile `obs` on the worktree's port block; `make obs-up`, `make logs`,
  `make metrics`
- [ ] P1.5 — H — `make dev` writes structured logs to `.tmp/dev/*.log`; skill `query-runtime`
- [ ] P2.1 — H — `new-domain.sh` scaffolds service, handler and integration tests; generator probe runs them
  (NFR-B1)
- [x] P2.2 — no-op — web: jsdom + Testing Library, or keep `renderToString` → kept: the one web test
  (`App.test.tsx`) renders to a string, and no test needs interaction or effects yet (P2.3 drives the browser
  through Playwright, P2.1 scaffolds Go tests); the guide from Q5 stands
- [ ] P2.3 — H — Playwright: `make e2e` against `make dev`, one journey (NFR-B2); skill `drive-app`
- [ ] P2.4 — H — approved-fixture snapshots of API responses (contract-shaped golden files)
- [ ] P2.5 — H — coverage report per package (Go + Vitest) into the CI summary; trend in the drift issue
- [ ] P2.6 — H — weekly mutation testing on files changed that week (Go: `gremlins`; TS: Stryker)
- [ ] P3.1 — – — ADR: inferential sensors are advisory only
- [ ] P3.2 — H — `code-reviewer` on every PR in CI, posting one comment; never a required check
- [ ] P3.3 — H — scheduled security, data-handling and modularity reviews → drift issue
- [ ] P3.4 — H — enable `agent-gc`: doc-gardening PRs for drift the computational checks found
- [ ] P3.5 — H — QUALITY.md grades per domain and layer, `Updated:` date, staleness check in drift
- [ ] P3.6 — H — `repo-no-orphan-scripts` (unreferenced scripts and make targets); `deadcode-go` into
  `check-all`
- [ ] P3.7 — H — clone detection: golangci `dupl` (Go) and `jscpd` (TS) with thresholds, in `check-all`
- [ ] P3.8 — H — `reuse-reviewer` agent: does the diff re-implement something that exists?
- [ ] P4.1 — – — ADR amending core belief 3: size and complexity limits as sensors, not style
- [ ] P4.2 — H — Go: `funlen`, `cyclop`/`gocognit`, file length; TS: `max-lines`, `complexity`, `max-params`
- [ ] P4.3 — H — template version marker (`.harness-version`) + `make harness-sync-check` against upstream
- [ ] P4.4 — H — per-rule fire counts in `timings.jsonl`; drift issue lists rules that never fired in 90 days
- [ ] P4.5 — – — `docs/references/*-llms.txt` for oapi-codegen, sqlc, orval, Atlas (pinned versions)

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
  findings of the computational doc checks (Q3, broken links, stale QUALITY rows) and never touches protected
  paths.
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
  `make harness-sync-check` diffs the protected paths against that upstream tag and lists what changed. It
  never applies anything. The weekly drift run reports when a product falls behind.
- **P4.4 — rippable harness.** `run.sh` adds the finding count per rule to `timings.jsonl`. The drift issue
  lists rules that never fired in 90 days. A human decides to keep or retire each one; this rule is never
  automated.
- **P4.5 — references.** `docs/references/<tool>-llms.txt` for oapi-codegen, sqlc, orval and Atlas, taken
  from upstream at the pinned version. Drift fails when `tool-versions.env` moves and the file doesn't.

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
