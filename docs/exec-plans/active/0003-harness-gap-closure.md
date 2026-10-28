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

New rows. Budgets land in `scripts/harness/budgets.txt` (protected; raising one is a loosening). The table
moves to `docs/harness/nfr.md` with 0002's (Q5) once this plan is accepted. ADR 0013 defines only latency (H,
warn) and reliability (R, fail) NFRs; the behaviour (B) and observability (O) classes are added by the Q0
ADR.

| Id | Requirement | Target | Today | Measured by | On violation |
|----|-------------|--------|-------|-------------|--------------|
| NFR-H8 | `pre-commit` git hook, typical commit (≤ 20 staged files) | ≤ 2 s | no hook | `timing.sh` row `pre-commit`, local only (the hook never runs in CI) | warn |
| NFR-H9 | `repo-no-secrets` in `make check` | ≤ 5 s | 0.43 s warm, full tree (Q1) | `run.sh`, budgets row `repo-no-secrets 5000 NFR-H9` | warn; alert. If it stays over budget, the full scan moves to `check-all` and `check` scans changed files only |
| NFR-B1 | Every domain created by `make new-domain` has a passing service test, handler test and integration test before its first edit | 100 % | 0 domains, scaffold has no tests | CI generator probe (rules.md:240) runs the generated tests | **fail** |
| NFR-B2 | The app's critical path is covered by an e2e test that runs against this worktree's `make dev` | ≥ 1 journey (`/healthz` + web shell renders) | none | `make e2e` in `check-all` and CI | **fail** |
| NFR-O1 | An agent in a fresh worktree can fetch the log lines of one request by its ID and a p95 latency, through `make` targets only | yes | no | integration test in `check-all`: request → `make logs REQUEST_ID=…` | **fail** |

Inferential sensors get **no** blocking NFR: an LLM verdict is not reproducible, so it cannot meet NFR-R1.
They comment, never fail (Phase 3, ADR).

## Phases

| # | Phase | Exit criterion | State |
|---|-------|----------------|-------|
| 0 | Quick wins: Trivy, pre-commit, structured-logging lints, doc truth | secrets and JS/nested-Go vulns are detected; `rules-fixture-claims` is green; every drift item above is fixed | proposed |
| 1 | Runtime legibility (G1) | NFR-O1 met: logs by request ID and a p95 latency, per worktree, via `make` | proposed |
| 2 | Behaviour harness (G2) | NFR-B1 and NFR-B2 met; coverage reported per package; weekly mutation score in the drift issue | proposed |
| 3 | Inferential sensors and garbage collection (G3, G4, G8) | reviewer comments on every PR (advisory); scheduled reviews and `agent-gc` open issues/PRs; QUALITY.md has a staleness check | proposed |
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
- [ ] Q2 — H — `pre-commit` git hook: secret scan + cheap per-file sensors on staged files (NFR-H8)
- [ ] Q3 — H — doc-truth sensor `rules-fixture-claims`; then fix every claim it flags
- [ ] Q4 — H — `go-structured-logging`: `forbidigo` (`fmt.Print*`, `log.*` outside `platform`) + `sloglint`
- [ ] Q5 — H — doc drift: `minimumReleaseAge`, web test stack, "<60 s", plan 0002 states, loop prompt, NFRs into
  `docs/harness/nfr.md` (ADR 0013)
- [ ] Q6 — H — `go-sec`: `gosec` in golangci-lint (SAST for Go)
- [ ] P1.1 — – — ADR: per-worktree observability stack
- [ ] P1.2 — – — contract + platform middleware: request ID, access log, `otelhttp` server spans, pgx tracer
- [ ] P1.3 — – — OTLP exporter wired to config; metrics (RED per route)
- [ ] P1.4 — H — `compose.yaml` profile `obs` on the worktree's port block; `make obs-up`, `make logs`,
  `make metrics`
- [ ] P1.5 — H — `make dev` writes structured logs to `.tmp/dev/*.log`; skill `query-runtime`
- [ ] P2.1 — H — `new-domain.sh` scaffolds service, handler and integration tests; generator probe runs them
  (NFR-B1)
- [ ] P2.2 — – — web: jsdom + Testing Library, or keep `renderToString` (guide fixed in Q5)
- [ ] P2.3 — H — Playwright: `make e2e` against `make dev`, one journey (NFR-B2); skill `drive-app`
- [ ] P2.4 — H — approved-fixture snapshots of API responses (contract-shaped golden files)
- [ ] P2.5 — H — coverage report per package (Go + Vitest) into the CI summary; trend in the drift issue
- [ ] P2.6 — H — weekly mutation testing on files changed that week (Go: `gremlins`; TS: Stryker)
- [ ] P3.1 — – — ADR: inferential sensors are advisory only
- [ ] P3.2 — H — `code-reviewer` on every PR in CI, posting one comment; never a required check
- [ ] P3.3 — H — scheduled security, data-handling and modularity reviews → drift issue
- [ ] P3.4 — H — enable `agent-gc`: doc-gardening PRs for drift the computational checks found
- [ ] P3.5 — H — QUALITY.md grades per domain and layer, `Updated:` date, staleness check in drift
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
  directory, so the fixture still fires.
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

  The CI generator probe (rules.md:240) generates a domain and runs them (NFR-B1). The scaffold's inline heredocs move to template files
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
  collector.
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
