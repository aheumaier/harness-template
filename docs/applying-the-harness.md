# Applying the harness to your codebase

A guide for principal engineers who want to take this repository's agent harness to their own stack, such as
PHP (Laravel or Symfony) or .NET (ASP.NET Core). It explains each principle, the sensor that enforces it,
where that sensor gates, and the Claude Code hooks and settings that tie it together. It also records what we
learned building it, including what failed.

**Scope.** Claude Code is the only agent considered. Everything here is in this repository unless marked
otherwise. **The PHP and .NET mappings are this guide's analogies, not something the repository ships or has
tested.** Treat them as a starting point for your own ADRs.

**Contents**

1. [The ten things to copy](#1-the-ten-things-to-copy)
2. [Mental model: guides, sensors, and the feedback loop](#2-mental-model-guides-sensors-and-the-feedback-loop)
3. [The sensor contract (the portable core)](#3-the-sensor-contract-the-portable-core)
4. [The layer rule](#4-the-layer-rule)
5. [The seven golden principles](#5-the-seven-golden-principles)
6. [Supporting mechanics](#6-supporting-mechanics)
7. [The Claude Code layer](#7-the-claude-code-layer)
8. [Learnings](#8-learnings)
9. [Adoption order for a brownfield codebase](#9-adoption-order-for-a-brownfield-codebase)
10. [Appendix A: tool mapping](#appendix-a-tool-mapping)
11. [Appendix B: rule catalogue](#appendix-b-rule-catalogue)
12. [Appendix C: a per-edit hook for PHP and .NET](#appendix-c-a-per-edit-hook-for-php-and-net)

---

## 1. The ten things to copy

If you take nothing else, take these. Each is explained below.

1. **Every rule exists twice:** as a sentence an agent reads, and as a deterministic sensor that fails with a
   rule id and a one-sentence remediation. A rule without a sensor is a suggestion.
2. **The remediation text lives in one place** ([harness/rules.md](harness/rules.md)) and the sensor runner
   prints it verbatim. Guides and sensors cannot drift apart.
3. **Every sensor is tested against a known-bad fixture.** A sensor that silently stops firing is worse than no
   sensor ([ADR 0009](adr/0009-harness-selftest.md)).
4. **One entry point (`make`), pinned tools, fail closed.** A sensor that cannot read its input fails.
5. **Two speeds:** a scoped `make check` in seconds after every change, and a full uncached `make check-all`
   before a PR and in CI ([ADR 0014](adr/0014-make-check-scoped-to-the-change.md)).
6. **Feedback at the moment of the mistake:** a Claude Code `PostToolUse` hook lints the file the agent just
   edited and feeds the failure straight back to it.
7. **The harness protects itself:** agents cannot edit linter configs, hooks or sensors; a `PreToolUse` hook
   blocks it locally and a base-branch CI gate blocks it remotely.
8. **LLM reviewers are advisory, never gates.** A finding class accepted twice is promoted to a deterministic
   rule ([ADR 0019](adr/0019-inferential-sensors-are-advisory.md)).
9. **Latency warns, reliability fails.** Sensor timings are budgets that alert; determinism and fail-closed
   behaviour are gates ([ADR 0013](adr/0013-harness-nfr-budgets.md)).
10. **Do not gate on coverage, mutation score or latency.** Agents game thresholds
    ([ADR 0020](adr/0020-test-coverage-is-a-trend.md), [ADR 0023](adr/0023-mutation-testing-is-a-drift-report.md)).

---

## 2. Mental model: guides, sensors, and the feedback loop

The harness has two halves.

- **Guides** (feedforward) tell the agent what good looks like before it acts: [AGENTS.md](../AGENTS.md),
  [ARCHITECTURE.md](../ARCHITECTURE.md), nested `AGENTS.md` files per area,
  [core beliefs](design-docs/core-beliefs.md), ADRs and skills.
- **Sensors** (feedback) tell it what went wrong after it acts. They come in two kinds:
  - **Computational sensors** are deterministic: linters, architecture checks, tests, custom checkers. They
    exit non-zero with a rule id and a remediation. They can gate.
  - **Inferential sensors** are LLM reviewers (subagents, CI review jobs). Their verdicts are not reproducible,
    so they are advisory only and never a required check.

Sensors run at increasing cost and decreasing frequency. The earlier a mistake is caught, the cheaper the fix:

```text
 agent edits a file
   │
   ├─ PreToolUse hook  (≤ 50 ms)   protect-harness: is this a harness file?  guard-bash: is this command safe?
   ├─ PostToolUse hook (≤ 1 s)     fast-check: cheapest sensor for that one file
   │
 agent commits
   ├─ pre-commit       (≤ 2 s)     secrets, format, shell lint, links — on the staged index, not the tree
   ├─ commit-msg                   Conventional Commits
   │
 agent says "done"
   ├─ make check       (≤ 10 s warm, ≤ 60 s worst case)   scoped to what changed
   ├─ code-reviewer subagent       advisory LLM review; runs make check itself
   │
 agent opens a PR
   ├─ make check-all               full, uncached: migrations, contract, vulns, dead code, e2e, integration
   ├─ CI required checks           check-all, selftest, generator probe, contract gate, harness integrity, …
   │
 weekly / nightly
   └─ drift report, NFR report     never gates; opens one issue
```

Every layer is in the repository: Claude Code hooks in [.claude/](../.claude/settings.json), git hooks in
[scripts/git-hooks/](../scripts/git-hooks/pre-commit), make targets in the [Makefile](../Makefile), sensors in
[scripts/harness/](../scripts/harness/run.sh) and the Go checker in
[apps/api/tools/harness/](../apps/api/tools/harness/main.go), and CI in `.github/workflows/`.

---

## 3. The sensor contract (the portable core)

This is the part a PHP or .NET port must reproduce exactly. The tools differ per stack; the contract does not.

### 3.1 One runner, one output shape

Every sensor runs through [scripts/harness/run.sh](../scripts/harness/run.sh):

```sh
scripts/harness/run.sh <sensor-id> <rule-id[,rule-id…]> [--dir <d>] -- <command…>
```

- **Success is silent.** An agent's context is the scarcest resource; green output is noise.
- **Failure prints** the sensor, the command, the first 80 lines of output (ANSI stripped), and then for each
  rule that fired:

  ```text
  rule: go-layers
  remediation: Move the code to the layer that is allowed to own it (see ARCHITECTURE.md#layers); …
  docs: docs/harness/rules.md#go-layers
  ```

- The remediation is read from the `- **Remediation:**` line under `### <rule-id>` in
  [harness/rules.md](harness/rules.md). It exists nowhere else.
- When a tool does not print the rule id (most third-party linters), `run.sh` maps the linter's own tag
  (`(funlen)`, `(gosec)`, `max-params`) to a rule id.

### 3.2 Exit codes

| Exit | Meaning | Hook behaviour |
|------|---------|----------------|
| 0 | pass | silent |
| 1 | finding | sensor fails |
| 2 | cannot run (missing tool, unparseable input) | **fails too** — fail closed |

Claude Code hooks block only on exit 2, so hooks deliberately exit 2 for every block, and the hook wrappers
convert any unexpected exit from the checker into a block.

### 3.3 Fixtures and selftest

Each rule with a local sensor has a violation fixture in `scripts/harness/fixtures/<name>/` (about 90 today):

```sh
# scripts/harness/fixtures/go-function-too-long/fixture.conf
RULE=code-function-length
TARGET=lint-go
EXPECT='Function .FixtureTooLong. has too many statements \([0-9]+ > [0-9]+\) \(funlen\)'
```

A fixture holds `files/` (an overlay), optionally `base/` (committed first, so diff-based sensors have a
history) and `apply.sh` (for edits that are not whole files). `make harness-selftest`
([selftest.sh](../scripts/harness/selftest.sh)) copies the repo to a temp git repository, applies each
fixture, runs `make <TARGET>`, and passes only if make fails, the output matches `EXPECT`, **and** the output
contains `rule: <RULE>` followed by a non-empty remediation. A fixture must fail for the right reason.

The rule doc itself is checked: [rules-fixture-claims](harness/rules.md#rules-fixture-claims) fails if a rule
claims a fixture that does not exist, or has neither a fixture nor `none (<reason>)`.

### 3.4 Rule lifecycle

Rules in [harness/rules.md](harness/rules.md) follow one format (rule · why · sensor · remediation · fixture)
and carry a status:

- **seed:** shipped with the template.
- **team-convention:** adopted by an ADR instead of an incident
  ([ADR 0015](adr/0015-anticipated-rules-and-nfr-classes-b-o.md)).
- **earned:** an inferential finding class accepted twice, promoted to a deterministic rule. This
  ratchet is how LLM review turns into computational sensors over time.

Rules also leave. A weekly report lists rules that never fired in 90 days, and a human decides to keep or
retire each one ([ADR 0032](adr/0032-rules-that-never-fire-are-a-drift-report.md)). A rule that never fires may
be guarding a mistake agents stopped making, or it may be dead.

### 3.5 Scope and budgets

- `make check` runs with `HARNESS_SCOPE=changed`. A sensor is skipped only when none of its inputs **and no
  harness file** changed since the merge base ([unchanged.sh](../scripts/harness/unchanged.sh),
  [affected.sh](../scripts/harness/affected.sh)); any error means run. Go lint and tests, and TS lint and architecture, run only for
  packages affected by the change; TS typecheck and tests always run and rely on the Turborepo cache. This took `make check` from 42–60 s to about 2 s on a one-file change.
- `make check-all` and CI run with `HARNESS_SCOPE=full`, uncached.
- [timing.sh](../scripts/harness/timing.sh) times every sensor and hook against
  [budgets.txt](../scripts/harness/budgets.txt) (default 5 s; PreToolUse 50 ms; fast-check 1 s; pre-commit
  2 s), appends a row to `.cache/harness/timings.jsonl`, and **warns** when over budget. A nightly job reports
  p95 per sensor. Latency never fails a build: it is flaky on loaded machines. See
  [harness/nfr.md](harness/nfr.md) for the full NFR list.
- Determinism **does** fail: a nightly job runs `make check` twice on the same tree and fails on any difference
  ([harness-deterministic](harness/rules.md#harness-deterministic)). Flaky sensors teach agents to rerun.

### 3.6 Porting the contract

| Element | This repo | PHP | .NET |
|---------|-----------|-----|------|
| Runner | `run.sh` (bash) | keep `run.sh` as is | keep `run.sh` (WSL/Git Bash on Windows), or port to a PowerShell or `dotnet` tool with the same output |
| Remediation source | `docs/harness/rules.md` | same | same |
| Finding → rule mapping | `finding_pattern()` in `run.sh` | map PHPStan identifiers, Deptrac rule names | map Roslyn diagnostic ids (`CA1502`, `S3776`) |
| Checker binary | `.bin/harness` (Go) | reuse: most subcommands (hook input, guard-bash, links, integrity, fixture claims, orphans, layout) are stack-agnostic and read [layout.env](../scripts/harness/layout.env) | same |
| Fixtures | overlay + `fixture.conf` | same | same |

---

## 4. The layer rule

Each backend domain is `internal/<domain>/{model,store,service,api}`, depending one way only:
`model ← store ← service ← api`. `platform` is shared plumbing and imports no domain. Another domain may use
your `service` or `model`, never your `store` or `api`. The frontend mirrors this: `packages/domain` is pure
TypeScript, only `packages/api-client` talks HTTP, apps never import each other
([ARCHITECTURE.md](../ARCHITECTURE.md#layers)).

**Why.** Agents put code where it is easiest to write. Without a sensor, SQL ends up in handlers and HTTP status
codes in services within a few features.

**Sensors here.**

| Rule | Tool | Config |
|------|------|--------|
| [go-layers](harness/rules.md#go-layers) | go-arch-lint | [apps/api/.go-arch-lint.yml](../apps/api/.go-arch-lint.yml) |
| [go-domain-isolation](harness/rules.md#go-domain-isolation) | custom analyzer `domainiso` (go-arch-lint cannot express "same domain") | [apps/api/tools/analyzers/](../apps/api/tools/analyzers/domainiso) |
| [go-db-only-in-store](harness/rules.md#go-db-only-in-store), [go-http-only-in-api](harness/rules.md#go-http-only-in-api) | golangci-lint `depguard` | [apps/api/.golangci.yml](../apps/api/.golangci.yml) |
| [go-no-root-context](harness/rules.md#go-no-root-context) | custom analyzer `noctxroot` | same |
| [web-domain-pure](harness/rules.md#web-domain-pure), [web-no-cross-app](harness/rules.md#web-no-cross-app) | dependency-cruiser + ESLint | [.dependency-cruiser.cjs](../.dependency-cruiser.cjs), [eslint.config.mjs](../eslint.config.mjs) |
| [web-no-direct-http](harness/rules.md#web-no-direct-http) | ESLint `no-restricted-globals/-imports` | same |

**Gates:** per-edit hook (Go analyzers, ESLint), `make check`, CI.

**PHP.** [Deptrac](https://github.com/qossmic/deptrac) for layers (`layers:` by namespace regex, `ruleset:`
for allowed directions; one layer set per domain or a collector on the domain segment). For "DB only in store"
and "HTTP only in api", use `spaze/phpstan-disallowed-calls` to ban `PDO`, `DB::`, Eloquent queries or
`Doctrine\DBAL` outside repositories, and `Request`, `request()`, `$_GET`/`$_POST`, `response()` outside
controllers. Cross-domain isolation (the `domainiso` equivalent) is a Deptrac collector or a small custom
PHPStan rule. In Laravel, also ban facades in the domain layer; they are hidden globals.

**.NET.** The strongest sensor is the compiler: one project per layer (`Billing.Model`, `Billing.Store`,
`Billing.Service`, `Billing.Api`) with `ProjectReference`s only in the allowed direction, and service projects
that do not reference `Microsoft.AspNetCore.App`. Where you keep one project, use
[NetArchTest](https://github.com/BenMorris/NetArchTest) or [ArchUnitNET](https://github.com/TNG/ArchUnitNET)
as unit tests, and `Microsoft.CodeAnalysis.BannedApiAnalyzers` with a `BannedSymbols.txt` per project for the
depguard and noctxroot equivalents (for example, ban `CancellationToken.None` outside `Program.cs`, and
`DbContext` outside the store).

---

## 5. The seven golden principles

These are stated in [AGENTS.md](../AGENTS.md#golden-principles). Each block gives the rule, why, the sensors
and where they gate here, the PHP and .NET equivalents, and the learning.

### 5.1 Contract first

**Rule.** Change `api/openapi.yaml`, run `make gen`, then implement the generated interface. Never hand-edit
generated code.

**Why.** Drift between contract and implementation becomes a build error instead of a production bug
([ADR 0005](adr/0005-spec-first-contract.md)). A breaking change can compile as a refactor, so it needs its
own gate ([ADR 0006](adr/0006-contract-gate.md)).

**Sensors and gates.**

| Rule | Sensor | Gate |
|------|--------|------|
| `api-*` style rules (kebab paths, camelCase, operationId, documented errors, problem+json) | Spectral, [.spectral.yaml](../.spectral.yaml) | per-edit hook on `openapi.yaml`, `check-all` |
| [api-no-breaking-change](harness/rules.md#api-no-breaking-change) | oasdiff vs merge base | required CI check, run from the **base branch**; waived only by a code owner's `breaking-change-approved` label |
| [gen-up-to-date](harness/rules.md#gen-up-to-date) | [gen-check.sh](../scripts/harness/gen-check.sh): hash generated files, run `make gen`, fail if anything changed | `make check` (skipped by stamp when no generator input changed) |
| editing a generated file | per-edit hook refuses `*.gen.go`, `store/`, `api-client/src/gen/` and tells the agent to run `make gen` | per edit |
| [api-responses-approved](harness/rules.md#api-responses-approved), [api-approved-match-contract](harness/rules.md#api-approved-match-contract) | approval tests pin handler responses; a checker validates each approved file against the contract | `make check` |

The skill [add-endpoint](../.claude/skills/add-endpoint/SKILL.md) walks the agent through the order.

**PHP.** Spectral and oasdiff work unchanged (they read YAML). Generate server stubs with openapi-generator
(`php-symfony`, `php-laravel`) or Jane, and the client the same way. Validate requests at runtime with
`league/openapi-psr7-validator` if the generator does not. Approval tests: `spatie/phpunit-snapshot-assertions`.
The gen-check pattern (hash, regenerate, compare) is tool-independent.

**.NET.** Spectral and oasdiff unchanged. Generate with NSwag (controllers or minimal-API interfaces) or
openapi-generator `aspnetcore`; clients with NSwag or Kiota. Approval tests: [Verify](https://github.com/VerifyTests/Verify).
If you go code-first instead (Swashbuckle or `Microsoft.AspNetCore.OpenApi`), export the spec in the build and
run gen-check and oasdiff on the exported file; you lose "contract first" but keep the breaking-change gate.

**Learning.** Generated code is committed and checked, not built on the fly: the agent must be able to read what
it implements. Approved files without a completeness check still miss operations that have no approval test
(tech debt TD-30).

### 5.2 Parse at boundaries

**Rule.** Validate input where it enters (generated strict server, zod schemas in the client); inner code
receives typed values. Errors leave the API as RFC 9457 `application/problem+json`.

**Why.** One validation point means inner code never re-checks, and agents never invent ad-hoc error shapes.

**Sensors.** Spectral rules [api-documented-errors](harness/rules.md#api-documented-errors) and
[api-errors-problem-json](harness/rules.md#api-errors-problem-json) force every operation to document 4xx and
5xx as problem+json. orval generates zod schemas with runtime validation.

**PHP.** Symfony Validator on DTOs or Laravel Form Requests at the controller; `crell/api-problem` or API
Platform for problem+json; PHPStan at max level so inner code gets typed values.

**.NET.** `AddProblemDetails()` and `TypedResults.Problem` are built in; FluentValidation or data annotations
at the endpoint; nullable reference types with warnings as errors.

**Learning.** Be honest about what the generator does. The oapi-codegen strict server does less validation than
the principle claims (TD-61). Check your generator before you write the principle.

### 5.3 Schema changes are new migrations

**Rule.** Expand, migrate, contract. Never edit an applied migration. Never drop or rename in one step.

**Why.** In a rolling deploy the previous version still runs against the new schema. Applied migrations are
history.

| Rule | Sensor | Gate |
|------|--------|------|
| [db-append-only-migrations](harness/rules.md#db-append-only-migrations) | script compares migrations to the merge base | per-edit hook, `check-all` |
| [db-no-destructive-migration](harness/rules.md#db-no-destructive-migration), [db-backward-compatible-migration](harness/rules.md#db-backward-compatible-migration) | Atlas `migrate lint` ([atlas.hcl](../apps/api/atlas.hcl)), squawk | per-edit hook (with Docker), `check-all` |
| [db-safe-locking](harness/rules.md#db-safe-locking) | squawk ([.squawk.toml](../.squawk.toml)): `lock_timeout`, concurrent indexes, no `NOT NULL` without default | per-edit hook, `check-all` |
| [db-schema-doc-fresh](harness/rules.md#db-schema-doc-fresh) | generated [schema doc](generated/db-schema.md) matches migrations | `make check` |

**PHP.** Doctrine Migrations or Laravel migrations are PHP, not SQL, so render them first
(`doctrine:migrations:migrate --dry-run --write-sql`, or `php artisan migrate --pretend`) and lint the output
with squawk (Postgres only). Append-only is the same git diff check on `migrations/`.

**.NET.** EF Core: `dotnet ef migrations script --idempotent` to SQL, then squawk. Append-only on
`Migrations/*.cs`. `dotnet ef migrations has-pending-model-changes` (EF Core 8+) is a cheap gen-check
equivalent: the model and the last migration must agree.

**Learning.** Atlas Community Edition cannot tell a rename from a drop; its backward-compatibility analyzers
need a Pro login ([ADR 0004](adr/0004-migrations-atlas-squawk-sqlc.md)). squawk filled the gap. Expect gaps in
whatever tool you pick, and fixture-test the cases you care about.

### 5.4 Fix the code, not the sensor

**Rule.** A red sensor prints the rule and its remediation; follow it. A suppression (`//nolint`,
`eslint-disable`, `@ts-expect-error`, `# shellcheck disable`) needs `reason: … (rule: <id>)` on the same line. `@ts-ignore`,
`@ts-nocheck`, `jscpd:ignore` and `swift-format-ignore` are never allowed.

**Why.** The cheapest way for an agent to go green is to silence the sensor.

**Sensors.** [harness-suppression-reason](harness/rules.md#harness-suppression-reason): the Go checker scans
**added** lines against the merge base and requires the reason and a rule id that exists as a heading in
`rules.md`. Gate: `check-all` and the required `harness-integrity` CI job.

**PHP.** Same check on `@phpstan-ignore`, `@psalm-suppress`, `// phpcs:ignore`, `@codeCoverageIgnore`. Ban
baseline file growth: a PHPStan baseline may only shrink.

**.NET.** Same check on `#pragma warning disable`, `[SuppressMessage]` (require `Justification`), and
`<NoWarn>` additions in `.csproj` or `Directory.Build.props` (those are config, see 5.5).

**Learning.** A suppression syntax that accepts a trailing comment can always be satisfied, so some are banned
outright (proposed [ADR 0043](adr/0043-swift-lint-format-and-xcode-pin.md)). Look for the equivalent in your tools.

### 5.5 Harness files are not yours to change

**Rule.** Linter configs, `scripts/harness/**`, `.claude/**`, named `.github/` workflows, `AGENTS.md`,
`CLAUDE.md`, the `Makefile`, `docs/harness/*` and the rest of
[protected-paths.txt](../scripts/harness/protected-paths.txt) are protected. If a rule is wrong, propose the
change in the PR description.

**Why.** A sensor an agent can edit is a suggestion.

| Rule | Sensor | Gate |
|------|--------|------|
| [harness-files-protected](harness/rules.md#harness-files-protected) | `PreToolUse` hook [protect-harness.sh](../.claude/hooks/protect-harness.sh) | every agent edit, in every worktree |
| [harness-config-not-loosened](harness/rules.md#harness-config-not-loosened) | Go checker diffs protected config lines vs merge base | `check-all`; required CI job, run from the base branch via `pull_request_target`; waived by `harness-change-approved` |
| CODEOWNERS | code-owner review on protected paths | branch protection |

The base-branch gate matters: a PR could otherwise rewrite the workflow that checks it
([ADR 0010](adr/0010-base-branch-gates.md)). The PR's code is only data to the gate.

**PHP.** Protect `phpstan.neon`, `phpstan-baseline.neon`, `deptrac.yaml`, `.php-cs-fixer.dist.php`,
`psalm.xml`, `infection.json5`, `composer.json` `scripts` section if your gates live there.

**.NET.** Protect `.editorconfig` (analyzer severities live there), `Directory.Build.props`,
`Directory.Packages.props`, `BannedSymbols.txt`, `global.json`, `*.ruleset`, `CodeMetricsConfig.txt`, and
architecture test projects.

**Learning.** Protecting all of `.github/` forced a human label on every product workflow edit; name the
harness workflows instead ([ADR 0035](adr/0035-harness-files-under-github-are-named.md)). The local hook is a
convenience, not a control: it misses writes from inside an interpreter such as `python -c` (TD-73; the hook's header names the current
gap). The control is the base-branch CI gate
plus CODEOWNERS, which only works with branch protection (see [caveats](#83-caveats-and-open-problems)).

### 5.6 Done means green

**Rule.** `make check` before saying a task is done; `make check-all` before a PR.

**Why.** It turns "done" from a claim into a verifiable state, and it is the one instruction every agent run
can follow without judgement.

**Composition** (from the [Makefile](../Makefile)):

- `make check` runs in parallel (`-k -j4`): layout check, Go lint, architecture, Go tests, analyzer and harness
  tests, shell lint, format and tests, workflow lint, secret scan, schema doc freshness, doc links, pinned
  references, fixture claims, rule lists, approved-file check, scenario tracing, web lint, architecture,
  typecheck and tests, the Swift lock file and, with the `ios` stack group, Swift lint, format and tests. Then
  serially `gen-check`.
- `make check-all` runs `check`, then in parallel: Go and dependency vulnerabilities, Atlas lint, squawk,
  append-only, Spectral, contract check, knip, deadcode, clone detection, quality-doc shape, harness integrity,
  orphans, commits, specs. Then serially: integration tests, observability end-to-end, browser e2e.

**PHP / .NET.** Keep `make` as the entry point (or `just`; on .NET, a Nuke or Cake build works if your team
lives there, but one command must remain the definition of green). Map each step via
[Appendix A](#appendix-a-tool-mapping).

**Learning.** `make check` grew to 42 s unnoticed before budgets existed. Agents run it dozens of times per
task: its latency is your throughput. Scope it ([ADR 0014](adr/0014-make-check-scoped-to-the-change.md)) and
keep heavy, Docker-dependent and network-dependent sensors in `check-all`.

### 5.7 The repo is the system of record

**Rule.** Decisions go in [adr/](adr/README.md); product capabilities in `specs/NNN-slug/` via spec-kit; other
multi-step work in [exec-plans/](exec-plans/tech-debt-tracker.md); known debt in the tech-debt tracker.

**Why.** An agent has no memory between sessions and no access to your chat history or wiki.

| Rule | Sensor | Gate |
|------|--------|------|
| [docs-links-resolve](harness/rules.md#docs-links-resolve) | link and anchor checker | per edit of any `.md`, pre-commit, `make check` |
| [docs-claude-imports-agents](harness/rules.md#docs-claude-imports-agents) | every `AGENTS.md` has a sibling `CLAUDE.md` containing `@AGENTS.md` | same |
| [specs-required-for-features](harness/rules.md#specs-required-for-features) | a `feat` commit touching `apps/` or `api/` must change `specs/` | required `pr-conventions` CI job |
| [specs-scenarios-tested](harness/rules.md#specs-scenarios-tested) | every acceptance scenario id is named by a test or marked `(untested: <reason>)` | `make check` |
| [docs-references-pinned](harness/rules.md#docs-references-pinned) | `docs/references/<tool>-llms.txt` matches the pinned tool version | `make check` |
| [git-conventional-commits](harness/rules.md#git-conventional-commits) | regex on subject and PR title | `commit-msg` hook, required CI job |

**PHP / .NET.** All of these are stack-independent. The one worth copying first is
`docs/references/<tool>-llms.txt`: agents write against the library version in their training data, not the
one you pinned ([ADR 0029](adr/0029-tool-references-follow-the-pin.md)). Put the docs of your pinned Laravel,
Symfony, EF Core or ASP.NET version in the repo.

**Learning.** Agents tick tasks so acceptance scenarios ship unproven; tracing scenario ids to test names made
that visible ([ADR 0036](adr/0036-acceptance-scenarios-are-traced-to-tests.md)).

---

## 6. Supporting mechanics

These are not golden principles but the harness depends on them.

### 6.1 Single entry point and pinned tools

- Every tool is installed into `.bin/` by [install-tools.sh](../scripts/install-tools.sh) from versions in
  [tool-versions.env](../scripts/tool-versions.env). Binary downloads are verified against a per-OS/arch
  sha256 pin and the script fails closed on a mismatch or a missing pin. A new release must be 14 days old
  before it is pinned ([ADR 0016](adr/0016-tool-supply-chain.md)); a compromised scanner release is the reason.
- The Makefile puts `.bin` first on `PATH`. Never install global versions.
- **PHP:** pin the PHP version by Docker image digest or a version manager file; tools via `composer.lock`
  (with `bamarni/composer-bin-plugin` to isolate tool dependencies) or PHIVE with signature checks.
- **.NET:** `global.json` (SDK, `rollForward: disable`), `.config/dotnet-tools.json` for local tools,
  `Directory.Packages.props` (central package management) and `packages.lock.json` with `RestoreLockedMode`.

### 6.2 One layout file

All product paths (API dir, spec, layer glob, generated globs, stack groups) live in
[layout.env](../scripts/harness/layout.env), parsed by a validating reader that fails closed. Before this,
paths were hard-coded in about 15 protected files and a missed one skipped silently
([ADR 0030](adr/0030-one-layout-file-for-every-sensor.md)). A sensor test fails if the Go checker spells a
template path literally ([harness-reads-layout](harness/rules.md#harness-reads-layout)). Do this from day one in
a port; it is what makes the checker reusable.

### 6.3 Per-worktree isolation

`make setup` gives each git worktree its own Postgres, ports (a block of 10 derived from a hash of the path,
collision-checked against other worktrees) and `.env.local`
([setup-worktree.sh](../scripts/setup-worktree.sh), [ADR 0008](adr/0008-per-worktree-isolation.md)). Parallel
agents in parallel worktrees would otherwise collide on ports and databases. The same idea works with Laravel
Sail, Docker Compose project names, or .NET Aspire per worktree.

### 6.4 Size, complexity and duplication

Function length, cognitive complexity, parameter count and file length are rules with thresholds that **only go
down** ([ADR 0024](adr/0024-size-and-complexity-limits-are-maintainability-sensors.md)). Every finding says how
to split. Token clones are a rule too ([ADR 0027](adr/0027-token-clones-are-a-maintainability-sensor.md)).
Agents produce long functions and copy-paste readily; these are cheap, deterministic proxies for "is this
readable". Current thresholds per stack are in [code-function-length](harness/rules.md#code-function-length)
and its sibling rules.

### 6.5 Stack profiles

`STACK` in `layout.env` (`go`, `db`, `web`, `ios`) selects sensor groups, so a Go-only service runs no web
sensors ([ADR 0033](adr/0033-stack-profiles.md)). A port adds its own group (`php`, `dotnet`) instead of
replacing the existing ones.

---

## 7. The Claude Code layer

### 7.1 Guide files

- **[AGENTS.md](../AGENTS.md)** is the canonical, tool-agnostic guide: commands, layer rule, golden principles,
  where to look, working agreements. Keep it short; it is a map, not a manual.
- **[CLAUDE.md](../CLAUDE.md)** is one line, `@AGENTS.md`, followed by Claude-only instructions (hooks exist,
  which skills to use, which subagents to run before "done", never read `.env.local`).
- **Nested `AGENTS.md`** in `apps/api/internal/`, `apps/api/db/`, `api/`, `apps/web/` and `apps/ios/` add
  area rules. Each has a sibling `CLAUDE.md` that is exactly `@AGENTS.md`, because Claude Code loads
  `CLAUDE.md`, not `AGENTS.md`. A sensor enforces the pairing
  ([docs-claude-imports-agents](harness/rules.md#docs-claude-imports-agents)).

### 7.2 Settings

[.claude/settings.json](../.claude/settings.json) at the time of writing (`$schema` line omitted; the file is
the source):

```json
{
  "permissions": {
    "deny": [
      "Read(./.env*)", "Read(**/.env*)",
      "Bash(git push --force *)", "Bash(git push -f *)", "Bash(git push --force-with-lease *)",
      "Bash(git push * --force*)", "Bash(git push * -f *)",
      "Bash(git push * main)", "Bash(git push * *:main)",
      "Bash(rm -rf /*)", "Bash(rm -rf ~*)", "Bash(rm -rf ..*)", "Bash(rm -rf $HOME*)"
    ]
  },
  "hooks": {
    "PreToolUse": [
      { "matcher": "Edit|Write|MultiEdit|NotebookEdit|Bash",
        "hooks": [{ "type": "command", "command": "\"$CLAUDE_PROJECT_DIR\"/.claude/hooks/protect-harness.sh" }] },
      { "matcher": "Bash",
        "hooks": [{ "type": "command", "command": "\"$CLAUDE_PROJECT_DIR\"/.claude/hooks/guard-bash.sh" }] }
    ],
    "PostToolUse": [
      { "matcher": "Edit|Write|MultiEdit",
        "hooks": [{ "type": "command", "command": "\"$CLAUDE_PROJECT_DIR\"/.claude/hooks/fast-check.sh", "timeout": 120 }] }
    ]
  }
}
```

Notes:

- **No `allow` or `ask` lists are committed.** Allow lists are personal; the repository only commits what must
  never happen.
- **The deny list is text matching** and misses cases (a bare `git push` while on `main`,
  `rm -rf "$HOME/x"`). That is why `guard-bash` exists as a second layer.
- The deny list is portable to any stack as is. Add `Read(**/appsettings.*.json)` or `Read(**/secrets.json)`
  for .NET user secrets, and `Read(**/auth.json)` for Composer credentials.

### 7.3 Hooks

All hooks read Claude Code's JSON on stdin (`cwd`, `tool_name`, `tool_input.command`, `tool_input.file_path`).
`protect-harness`, `fast-check` and the subagent hooks parse it through `.bin/harness hook-input`; `guard-bash`
hands it to `.bin/harness guard-bash`. All fail closed on unparseable input, and exit 2 to block. Stderr on exit 2 goes
back to the agent, so every block message includes what to do instead. If `.bin/harness` is missing, every hook
blocks and says to run `make setup`.

| Hook | Event / matcher | Scope | What it does | Budget |
|------|-----------------|-------|--------------|--------|
| [protect-harness.sh](../.claude/hooks/protect-harness.sh) | PreToolUse `Edit\|Write\|MultiEdit\|NotebookEdit\|Bash` | all agents | Blocks edits to paths matching `protected-paths.txt`, checked in every worktree. For Bash, a heuristic catches redirections, `tee`, `cp/mv/rm/chmod`, `sed -i`, `git checkout/restore`. Escape hatch: `HARNESS_EDIT=1`, set by a human. | 50 ms |
| [guard-bash.sh](../.claude/hooks/guard-bash.sh) | PreToolUse `Bash` | all agents | Parses the command as a shell AST (Go, `mvdan.cc/sh`, without running it). Blocks force pushes, pushes to `main` (including a bare push while on `main`), `rm -r` outside the repo or with unexpanded paths, any command naming a `.env` file, nested `bash -c`/`eval` up to depth 4, and anything that does not parse. | 50 ms |
| [fast-check.sh](../.claude/hooks/fast-check.sh) | PostToolUse `Edit\|Write\|MultiEdit`, timeout 120 s | all agents | Dispatches on the edited file's path to the cheapest sensor through `run.sh`: Go → custom golangci-lint on the package; migrations → append-only, hash, squawk; queries → `sqlc compile`; `openapi.yaml` → Spectral; TS → ESLint on the file; shell → shellcheck and shfmt; Markdown → link check; generated files → refuse and say "run `make gen`". Silent on success. | 1 s |
| [only-make-check.sh](../.claude/hooks/only-make-check.sh) | PreToolUse `Bash` | `code-reviewer`, `test-writer` (in their frontmatter) | Allows only `^make check(-all)?$`. | — |
| [only-test-files.sh](../.claude/hooks/only-test-files.sh) | PreToolUse `Edit\|Write\|MultiEdit\|NotebookEdit` | `test-writer` | Allows writes only to product test files, using the same predicate as the `specs-scenarios-tested` sensor. | — |

**Why scope in hooks, not in frontmatter.** A subagent's `tools:` frontmatter can grant or withhold Bash, but
cannot restrict which commands or which paths. Hooks declared in the subagent's frontmatter can.

**Why `fast-check` does not typecheck.** `tsc` on a package is over the 1 s per-edit budget, so the per-edit
hook runs ESLint only and `make check` is the full gate. The same will be true for a .NET build; see
[Appendix C](#appendix-c-a-per-edit-hook-for-php-and-net).

**Caveat.** For Go, the per-edit hook runs the linters but not every finding names its rule id, so the agent
sees a related remediation. Do not promise per-edit rule ids for every linter.

### 7.4 Subagents

| Subagent | Tools | Confinement | Role |
|----------|-------|-------------|------|
| [code-reviewer](../.claude/agents/code-reviewer.md) | Read, Grep, Glob, Bash | hook: only `make check(-all)` | Runs `make check`, reviews the diff against AGENTS.md, golden principles and rules.md. Output: `APPROVE`/`CHANGES REQUESTED`, then `path:line — problem — fix (rule)`. Advisory, but its `make check` result is the done gate. |
| [architecture-reviewer](../.claude/agents/architecture-reviewer.md) | Read, Grep, Glob | read-only by tools | Drift linters cannot express: misplaced responsibility, leaky abstraction, decisions without an ADR. Ends with "New sensor candidates". |
| [reuse-reviewer](../.claude/agents/reuse-reviewer.md) | Read, Grep, Glob | read-only by tools | New code that re-implements something the repo, stdlib or a pinned dependency already has. |
| [evasion-reviewer](../.claude/agents/evasion-reviewer.md) | Read, Grep, Glob, Bash | **prompt only** | For changes to sensors: tries to make the new sensor pass on bad input (symlinks, comments, build tags, empty listings) in a scratch worktree and reproduces each bypass. |
| [test-writer](../.claude/agents/test-writer.md) | Read, Grep, Glob, Write, Edit, Bash | hooks: only `make check`, only test files | Writes failing unit tests from the spec before implementation ([ADR 0039](adr/0039-tests-are-written-apart-from-the-implementation.md)). |

[CLAUDE.md](../CLAUDE.md) says which reviewers to run before calling a change done. All are advisory
([ADR 0019](adr/0019-inferential-sensors-are-advisory.md)); "done" still means `make check` is green.

**The test-writer pattern** is worth copying on any stack. It has three layers: the prompt limits scope; a hook
refuses non-test paths; and `code-reviewer` later compares the committed test commit (trailer
`Test-Writer: specs/<NNN>`) with the final diff and reports loosened assertions (skips added, expected values
widened). The evidence for its benefit is still thin (two features per arm; re-measure tracked as TD-126).

**Reviewers that find the same class twice** feed the ratchet: the finding becomes a rule with a sensor and a
fixture, via a human-approved harness PR.

### 7.5 Skills

Skills in [.claude/skills/](../.claude/skills/new-domain/SKILL.md) encode workflows the agent must not
improvise:

- **Recipes:** `new-domain` (always `make new-domain NAME=…`, never by hand), `add-endpoint` (contract → gen →
  implement → check), `add-migration` (Atlas, lock timeouts, expand/contract).
- **Runtime evidence:** `query-runtime` (logs, traces, metrics of one request through make targets),
  `drive-app` (Playwright against this worktree's running app).
- **Scheduled reviews:** `review-security`, `review-modularity`, `review-data-handling` (read-only, advisory,
  weekly in CI).
- **Specs:** ten vendored spec-kit skills (`speckit-specify` → `speckit-plan` → `speckit-tasks` → …).

A scaffolding command is the strongest kind of skill: `make new-domain` generates every layer, the migration,
the contract tag, the client hook and passing tests, and a required CI job (`generator-probe`) proves the
scaffold still passes `make check`. Write `make new-module` for your PHP or .NET structure before you let
agents create modules.

### 7.6 Claude Code in CI

- PR review and the weekly scheduled reviews run `claude -p` with `--restricted --tools Read,Grep,Glob
  --disallowedTools 'mcp__*'` (full flag list in [ADR 0019](adr/0019-inferential-sensors-are-advisory.md)),
  from a pinned binary, with a leak screen on the output. They are never required checks.
- PR review runs on `pull_request`, never `pull_request_target`, so it never sees secrets with PR code.
- `agent-gc` (an agent that proposes doc-link fixes as a PR) is confined to its target files, fails if anything
  else changed, caps changed lines, and is off by default.
- Confinement lives in flags and settings, not in the prompt. `--bare` was tried and dropped: it blocks subscription tokens and skips `CLAUDE.md` discovery
  ([ADR 0040](adr/0040-ci-agents-may-use-a-subscription-token.md)).

---

## 8. Learnings

### 8.1 What we tried and what failed

| Tried | Failed because | Now | Source |
|-------|----------------|-----|--------|
| Coverage threshold | Agents wrote assertion-free tests to reach it | Coverage is a trend report | [ADR 0020](adr/0020-test-coverage-is-a-trend.md) |
| Mutation score as a gate | Same gaming as coverage | Weekly drift report | [ADR 0023](adr/0023-mutation-testing-is-a-drift-report.md) |
| Latency gates | Flaky on loaded machines | Budgets warn; nightly p95 report | [ADR 0013](adr/0013-harness-nfr-budgets.md) |
| Unbounded `make check` | Grew to 42 s unnoticed | Scoped check, ~2 s on one file | [ADR 0014](adr/0014-make-check-scoped-to-the-change.md) |
| PR runs its own gate workflow | A PR could rewrite its gate | Base-branch gates, PR code as data | [ADR 0010](adr/0010-base-branch-gates.md) |
| Protecting all of `.github/` | Every product workflow edit needed a human label | Named harness workflows only | [ADR 0035](adr/0035-harness-files-under-github-are-named.md) |
| Prose confinement for agents | A prompt is not enforced; an agent with Bash can do anything its shell can | CLI flags and settings in CI; hooks for local subagents (evasion-reviewer still prose, TD-107) | [ADR 0019](adr/0019-inferential-sensors-are-advisory.md) |
| `--bare` for CI agents | Blocks subscription tokens, skips CLAUDE.md | Confinement restored flag by flag | [ADR 0040](adr/0040-ci-agents-may-use-a-subscription-token.md) |
| Parsing in bash/awk | Edge cases misparsed | Parsing in one Go binary, orchestration in bash | [ADR 0012](adr/0012-parsing-sensors-in-go.md) |
| Serial bats suite for the harness | 192 s, over budget every run | Moving to `go test` | [ADR 0045](adr/0045-harness-scripts-and-tests-in-go.md) |
| Hard-coded product paths | A missed path skipped silently | One layout file, fail-closed reader | [ADR 0030](adr/0030-one-layout-file-for-every-sensor.md) |
| Version pins as integrity | A scanner release was compromised | sha256 pins, 14-day age rule | [ADR 0016](adr/0016-tool-supply-chain.md) |
| Prose brownfield setup in README | Rotted with each harness PR | A required CI job replays the README | [ADR 0038](adr/0038-readme-brownfield-adoption-is-probed.md) |
| Dead-code gate in `make check` | Orphans are normal mid-change | Moved to `check-all` | [ADR 0031](adr/0031-no-orphan-scripts-or-make-targets.md) |
| One architecture linter | Could not express "same domain" | Plus a custom analyzer | [ADR 0003](adr/0003-layer-enforcement.md) |

### 8.2 Principles behind the learnings

- **Gate on invariants, report on trends.** Anything an agent can satisfy without doing the work (coverage,
  mutation score, a date in a table) belongs in a weekly drift report, not a gate
  ([ADR 0028](adr/0028-quality-grades-per-domain-and-layer.md)).
- **Make the cheap path the right path.** The fastest way to green should be fixing the code. Protect configs,
  require reasons on suppressions, refuse edits to generated files.
- **Every message is a prompt.** A sensor's failure output is read by an agent. It must say what to do, in one
  sentence, at the moment of the mistake.
- **Prose rots; probes do not.** Where a procedure matters (brownfield adoption, scaffolding), a CI job runs it.
- **Test the sensors.** Fixtures, determinism, fail-closed parsing. The harness is code and gets the same
  treatment as product code.
- **Earn rules.** New rules come from incidents or from reviewer findings accepted twice. Rules that never fire
  get reviewed.

### 8.3 Caveats and open problems

Be honest with your team about these; most will apply to a port too. Details are in the
[tech-debt tracker](exec-plans/tech-debt-tracker.md).

- **Required checks need branch protection.** This repository is private on GitHub Free and `main` has no
  branch protection (TD-2), so the required checks are advisory in practice. The protection script exists
  ([github-branch-protection.sh](../scripts/github-branch-protection.sh)) but cannot be applied on that plan.
  Without branch protection and CODEOWNERS, the harness-integrity gate is not a control.
- **The local protect hook is bypassable** from Bash in ways the heuristic does not catch, such as writes from `python -c` or `node -e` (TD-73). Treat it as
  a fast warning; the CI gate is the control.
- **`evasion-reviewer` has Bash with no hook**; its limits are in its prompt only.
- **Per-edit Go findings** do not always name their rule id (7.3).
- **Budgets are not always met.** At the last measurement in [harness/nfr.md](harness/nfr.md), `make check`
  after a Go edit was over its 10 s budget (NFR-H3), and a harness change took minutes while the bats suite
  still ran (NFR-H4). That is why latency warns instead of failing.
- **The harness is mid-migration** from bash and bats to Go
  ([plan 0005](exec-plans/active/0005-harness-scripts-in-go.md)); `make test-sh` still runs bats.
- **CI agents with secrets are untested live** (TD-38), and `agent-gc` is off by default (TD-50).
- **The harness costs maintenance.** Expect harness PRs to be a steady share of your changes, and budget review
  time for them; they need a human by design.

---

## 9. Adoption order for a brownfield codebase

This order is a recommendation drawn from the ADRs and the brownfield procedure in [README.md](../README.md);
the repository does not prescribe it for other stacks. Each step is useful on its own.

1. **One entry point and pinned tools.** `make setup`, `make check`, tools in `.bin/`, versions with checksums.
   Nothing else works without reproducible tools.
2. **The sensor contract.** Copy `run.sh`, `timing.sh`, `budgets.txt` and start `docs/harness/rules.md` with
   the sensors you already have (PHPStan, PHP-CS-Fixer, or `dotnet format` and analyzers). Every rule gets an id
   and a remediation sentence.
3. **Fixtures and selftest** before you add any new sensor. Copy `selftest.sh` and the fixture layout.
4. **`AGENTS.md` and `CLAUDE.md`.** Short. Commands, layer rule, golden principles, where to look.
5. **Claude Code hooks.** `fast-check` with your stack's cheapest per-file sensors (Appendix C), the deny list,
   and `guard-bash` (reuse the Go binary).
6. **Protect the harness.** `protected-paths.txt`, the PreToolUse hook, CODEOWNERS, a base-branch CI gate,
   branch protection. Do this before agents start touching linter configs, not after.
7. **Layer enforcement** with a baseline: Deptrac or project references / ArchUnitNET. Freeze existing
   violations in a baseline that may only shrink.
8. **Scope `make check`** once it exceeds ~10 s: skip sensors whose inputs did not change, run tests only for
   affected packages, keep the full run for CI.
9. **Secrets and dependency vulnerabilities** as gates (Trivy works for both stacks; plus `composer audit` or
   NuGet audit).
10. **Contract first and migrations** if you own an HTTP API and a relational database: Spectral, oasdiff from
    the base branch, gen-check, squawk on rendered SQL, append-only migrations.
11. **Size, complexity and clone limits**, set at today's worst values, only going down.
12. **Advisory subagents** (code, architecture, reuse reviewers) and the ratchet.
13. **Drift reports** (coverage trend, mutation score, rules that never fire, dead code) once the gates are
    stable.

Do not add: coverage or mutation gates, latency gates, or LLM reviewers as required checks.

---

## Appendix A: tool mapping

The PHP and .NET columns are suggestions, not tested in this repository.

| Concern | This repo (Go / TS) | PHP | .NET |
|---------|---------------------|-----|------|
| Layers | go-arch-lint, depguard, dependency-cruiser | Deptrac, PHPat | project references, NetArchTest, ArchUnitNET |
| Banned APIs per layer | depguard, forbidigo, ESLint `no-restricted-*` | `spaze/phpstan-disallowed-calls` | `BannedApiAnalyzers` + `BannedSymbols.txt` |
| Custom analyzers | `go/analysis` plugins in golangci-lint | custom PHPStan rules | Roslyn analyzers |
| Static analysis / types | golangci-lint, `tsc --strict`, typescript-eslint strict | PHPStan max / Psalm | nullable reference types, `AnalysisLevel latest-all`, warnings as errors |
| Security lint | gosec | Psalm taint analysis, PHPStan security rules | Security Code Scan, `CA2xxx`/`CA3xxx`/`CA5xxx` |
| Format | gofmt/goimports, shfmt | PHP-CS-Fixer, Laravel Pint | `dotnet format --verify-no-changes`, CSharpier |
| Unit tests | `go test -race`, Vitest | PHPUnit, Pest | xUnit, NUnit, MSTest |
| Approval tests | in-repo `approvaltest` | `spatie/phpunit-snapshot-assertions` | Verify |
| Size / complexity | funlen, gocognit, revive, ESLint | PHPMD, PHP_CodeSniffer metrics, cognitive-complexity PHPStan extension | `CA1502`/`CA1505`/`CA1506`, SonarAnalyzer (`S138`, `S3776`, `S107`, `S104`) |
| Clones | dupl, jscpd | jscpd | jscpd |
| Dead code | deadcode, knip | `shipmonk/dead-code-detector`, `composer-unused` | `IDE0051`/`IDE0052`, `CA1812` |
| Dependency vulns | govulncheck, Trivy | `composer audit`, Trivy | NuGet audit (`NuGetAuditMode all`), `dotnet list package --vulnerable`, Trivy |
| Secrets | Trivy secret scan | same | same |
| OpenAPI lint / breaking | Spectral, oasdiff | same | same |
| Codegen from OpenAPI | oapi-codegen, orval | openapi-generator, Jane | NSwag, Kiota, openapi-generator |
| Migrations | Atlas, sqlc | Doctrine Migrations, Laravel migrations | EF Core migrations |
| Migration lint | Atlas lint, squawk | squawk on rendered SQL (Postgres) | squawk on `ef migrations script` (Postgres) |
| Mutation (report only) | gremlins, Stryker | Infection | Stryker.NET |
| E2E | Playwright | Playwright, Laravel Dusk | Playwright for .NET |
| Tool pinning | `.bin/` + sha256 in `tool-versions.env` | Docker digest, `composer.lock`, PHIVE | `global.json`, tool manifest, central packages, lock files |
| Shell, CI, docs, commits, harness integrity | shellcheck, actionlint, `.bin/harness` (including `check-commits`) | reuse as is | reuse as is |

---

## Appendix B: rule catalogue

Every rule in [harness/rules.md](harness/rules.md) at the time of writing, with where each gates and a suggested equivalent. Follow
the link for rule, why, sensor, remediation and fixture.

**Gates:** `H` PreToolUse hook · `E` per-edit PostToolUse hook · `PC` pre-commit · `CM` commit-msg ·
`C` `make check` · `CA` `make check-all` · `R` required CI job · `D` weekly drift · `N` nightly.
Everything in `C` also runs in `CA` and in the required CI `check-all` job.

### Go and database

| Rule | Gates | PHP | .NET |
|------|-------|-----|------|
| [go-layers](harness/rules.md#go-layers) | C | Deptrac | project references / ArchUnitNET |
| [go-domain-isolation](harness/rules.md#go-domain-isolation) | E, C | Deptrac collector or custom PHPStan rule | ArchUnitNET rule per domain |
| [go-db-only-in-store](harness/rules.md#go-db-only-in-store) | E, C | disallowed-calls on PDO/DBAL/Eloquent | BannedSymbols on `DbContext` |
| [go-http-only-in-api](harness/rules.md#go-http-only-in-api) | E, C | disallowed-calls on Request/response helpers | no ASP.NET reference in service projects |
| [go-test-helpers-only-in-tests](harness/rules.md#go-test-helpers-only-in-tests) | E, C | Deptrac: test helpers only from tests | `InternalsVisibleTo` / test-only project |
| [go-no-root-context](harness/rules.md#go-no-root-context) | E, C | ban request globals outside controllers | ban `CancellationToken.None` outside `Program.cs` |
| [go-structured-logging](harness/rules.md#go-structured-logging) | E, C | ban `echo`, `var_dump`, `dd`, `error_log`; Monolog JSON | ban `Console.Write*`; `ILogger` + JSON |
| [go-sec](harness/rules.md#go-sec) | E, C | Psalm taint | security analyzers |
| [go-format](harness/rules.md#go-format) | E, PC, C | PHP-CS-Fixer / Pint | `dotnet format` |
| [go-tests-pass](harness/rules.md#go-tests-pass) | C | PHPUnit / Pest | `dotnet test` |
| [go-no-known-vulns](harness/rules.md#go-no-known-vulns) | CA, D | `composer audit` | NuGet audit |
| [go-no-dead-code](harness/rules.md#go-no-dead-code) | CA, D | dead-code-detector | `IDE0051`, `CA1812` |
| [db-no-destructive-migration](harness/rules.md#db-no-destructive-migration) | E (with Docker), CA | squawk on rendered SQL | squawk on EF script |
| [db-backward-compatible-migration](harness/rules.md#db-backward-compatible-migration) | E (with Docker), CA | same | same |
| [db-safe-locking](harness/rules.md#db-safe-locking) | E, CA | same | same |
| [db-append-only-migrations](harness/rules.md#db-append-only-migrations) | E, CA | git diff on `migrations/` | git diff on `Migrations/` |
| [db-schema-doc-fresh](harness/rules.md#db-schema-doc-fresh) | C | generated schema doc | generated schema doc |

### Contract

| Rule | Gates | PHP | .NET |
|------|-------|-----|------|
| [api-kebab-case-paths](harness/rules.md#api-kebab-case-paths) | E, CA | Spectral (same) | Spectral (same) |
| [api-camel-case-properties](harness/rules.md#api-camel-case-properties) | E, CA | same | same |
| [api-operation-metadata](harness/rules.md#api-operation-metadata) | E, CA | same | same |
| [api-documented-errors](harness/rules.md#api-documented-errors) | E, CA | same | same |
| [api-errors-problem-json](harness/rules.md#api-errors-problem-json) | E, CA | same | same |
| [api-no-breaking-change](harness/rules.md#api-no-breaking-change) | CA, R | oasdiff (same) | oasdiff (same) |
| [api-upstream-no-breaking-change](harness/rules.md#api-upstream-no-breaking-change) | D | same | same |
| [api-responses-approved](harness/rules.md#api-responses-approved) | C | snapshot assertions | Verify |
| [api-approved-match-contract](harness/rules.md#api-approved-match-contract) | C | validate snapshots against the spec | same |

### Web, size, duplication, generated code

| Rule | Gates | PHP | .NET |
|------|-------|-----|------|
| [web-no-direct-http](harness/rules.md#web-no-direct-http) | E, C | same if you ship a TS frontend | same |
| [web-domain-pure](harness/rules.md#web-domain-pure) | E, C | same | same |
| [web-no-cross-app](harness/rules.md#web-no-cross-app) | C | same | same |
| [web-lint](harness/rules.md#web-lint) | E, C | same | same |
| [web-typecheck](harness/rules.md#web-typecheck) | C | same | same |
| [web-tests-pass](harness/rules.md#web-tests-pass) | C | same | same |
| [web-e2e-journeys](harness/rules.md#web-e2e-journeys) | CA | Playwright | Playwright |
| [web-no-dead-code](harness/rules.md#web-no-dead-code) | CA, D | same | same |
| [code-function-length](harness/rules.md#code-function-length) | E, C | PHPMD `ExcessiveMethodLength` | `S138` |
| [code-complexity](harness/rules.md#code-complexity) | E, C | cognitive-complexity extension | `S3776` / `CA1502` |
| [code-parameters](harness/rules.md#code-parameters) | E, C | PHPMD `ExcessiveParameterList` | `S107` |
| [code-file-length](harness/rules.md#code-file-length) | E, C | PHPMD `ExcessiveClassLength` | `S104` |
| [repo-no-clones](harness/rules.md#repo-no-clones) | C (Go), CA (TS) | jscpd | jscpd |
| [gen-up-to-date](harness/rules.md#gen-up-to-date) | E, C | hash → regenerate → compare | same, plus `has-pending-model-changes` |

The five iOS rules ([ios-tests-pass](harness/rules.md#ios-tests-pass), [ios-lint](harness/rules.md#ios-lint),
[ios-format](harness/rules.md#ios-format), [ios-xcode-pinned](harness/rules.md#ios-xcode-pinned),
[ios-app-builds](harness/rules.md#ios-app-builds)) and [deps-swift-resolved](harness/rules.md#deps-swift-resolved)
apply only with the `ios` stack group. They are a worked example of adding a stack group: tests, lint, format,
toolchain pin and lock file, each with its own rule.

### Shell, secrets, dependencies

| Rule | Gates | PHP / .NET |
|------|-------|------------|
| [sh-shellcheck](harness/rules.md#sh-shellcheck) | E, PC, C | same (the harness is shell) |
| [sh-format](harness/rules.md#sh-format) | E, PC, C | same |
| [sh-tests-pass](harness/rules.md#sh-tests-pass) | C | same, or test the harness in your checker's language |
| [repo-no-orphan-scripts](harness/rules.md#repo-no-orphan-scripts) | CA | same |
| [repo-no-secrets](harness/rules.md#repo-no-secrets) | PC, C | same (Trivy) |
| [deps-no-known-vulns](harness/rules.md#deps-no-known-vulns) | CA, D | Trivy reads `composer.lock` and `packages.lock.json` |

### Harness integrity, commits, specs, runtime, docs

These are stack-independent; reuse them as they are.

| Rule | Gates |
|------|-------|
| [harness-suppression-reason](harness/rules.md#harness-suppression-reason) | CA, R |
| [harness-config-not-loosened](harness/rules.md#harness-config-not-loosened) | CA, R |
| [harness-files-protected](harness/rules.md#harness-files-protected) | H, CODEOWNERS |
| [harness-latency-budget](harness/rules.md#harness-latency-budget) | every run (warns), N |
| [harness-deterministic](harness/rules.md#harness-deterministic) | N |
| [harness-scope](harness/rules.md#harness-scope) | C |
| [harness-upstream-in-sync](harness/rules.md#harness-upstream-in-sync) | D |
| [ci-workflows-valid](harness/rules.md#ci-workflows-valid) | C |
| [harness-layout-valid](harness/rules.md#harness-layout-valid) | C |
| [harness-reads-layout](harness/rules.md#harness-reads-layout) | C |
| [harness-rule-lists-match](harness/rules.md#harness-rule-lists-match) | C |
| [harness-adoption-probe](harness/rules.md#harness-adoption-probe) | R |
| [git-conventional-commits](harness/rules.md#git-conventional-commits) | CM, CA, R |
| [specs-structure](harness/rules.md#specs-structure) | CA, R |
| [specs-required-for-features](harness/rules.md#specs-required-for-features) | CA, R |
| [specs-scenarios-tested](harness/rules.md#specs-scenarios-tested) | C |
| [obs-request-queryable](harness/rules.md#obs-request-queryable) | CA |
| [docs-links-resolve](harness/rules.md#docs-links-resolve) | E, PC, C |
| [docs-claude-imports-agents](harness/rules.md#docs-claude-imports-agents) | E, PC, C |
| [docs-quality-rows-well-formed](harness/rules.md#docs-quality-rows-well-formed) | CA |
| [docs-quality-rows-match-domains](harness/rules.md#docs-quality-rows-match-domains) | CA |
| [docs-quality-row-fresh](harness/rules.md#docs-quality-row-fresh) | D |
| [docs-references-pinned](harness/rules.md#docs-references-pinned) | C |
| [rules-fixture-claims](harness/rules.md#rules-fixture-claims) | C |

**Required CI checks** (eight): `check-all`, `harness-selftest`, `generator-probe`, `adoption-probe`,
`check-ios`, `api-no-breaking-change`, `harness-integrity`, `pr-conventions`. See [harness/ci.md](harness/ci.md)
for jobs, label waivers and scheduled workflows.

---

## Appendix C: a per-edit hook for PHP and .NET

A sketch of the dispatch part of `fast-check.sh` for other stacks. It has not been run in this repository; use
it as the shape, and keep the rules from this repo's hook: read the path from the hook JSON, run the cheapest
sensor for that one file through `run.sh`, stay silent on success, exit 2 with the sensor output on failure,
and keep each branch under the 1 s budget.

```sh
case "$rel" in
  # Specific patterns first: case takes the first match.
  database/migrations/*.php | migrations/*.php | */Migrations/*.cs)
    run.sh migrations-append-only db-append-only-migrations -- scripts/harness/migrations-append-only.sh
    ;;
  *.php)
    run.sh php-cs-fixer php-format -- vendor/bin/php-cs-fixer fix --dry-run --diff -- "$rel"
    run.sh phpstan php-static,php-layers -- vendor/bin/phpstan analyse --no-progress --error-format=raw -- "$rel"
    ;;
  *.cs)
    # A Roslyn build is usually over 1 s: format only here, analyzers and build in make check.
    run.sh dotnet-format cs-format -- dotnet format whitespace --verify-no-changes --include "$rel"
    ;;
  api/openapi.yaml)
    run.sh spectral api-kebab-case-paths,api-documented-errors -- spectral lint --ruleset .spectral.yaml "$rel"
    ;;
  *.md)
    run.sh docs-links docs-links-resolve -- .bin/harness check-links .
    ;;
esac
```

Measure each branch with `timing.sh` from the first day. If PHPStan on one file with a warm result cache
exceeds the budget, move it to `make check` and keep only the formatter per edit, as this repository does with
`tsc`.
