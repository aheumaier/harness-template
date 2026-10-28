# harness-template

A template for products built by coding agents. It has a Go HTTP API (`apps/api`), a React web app (`apps/web`),
a shared OpenAPI contract (`api/openapi.yaml`) and Postgres. There are no business domains yet. What the
template really offers is the **harness**:

- **Guides** say what good looks like: [AGENTS.md](AGENTS.md), [ARCHITECTURE.md](ARCHITECTURE.md) and
  [docs/](docs/).
- **Sensors** fail with a remediation. They are listed in [docs/harness/rules.md](docs/harness/rules.md).

This file explains how to set the template up in two cases:

- (a) you start a **new project** from it;
- (b) you bring the harness into a **brownfield project** that already has code and structure.

After setup, [AGENTS.md](AGENTS.md) is the entry point for every session.

## Who does what

Some steps change harness files ([golden principle 5](AGENTS.md#golden-principles), listed in
[protected-paths.txt](scripts/harness/protected-paths.txt)). A human does those steps, or starts the agent
session with `HARNESS_EDIT=1`. Without it, `.claude/hooks/protect-harness.sh` blocks the edit.

A human also does these; an agent never does them:

- adds the waiver labels;
- applies branch protection;
- accepts ADRs.

Steps that need a human are marked **(human)**.

## Prerequisites

- Any Go install. `GOTOOLCHAIN` pins the real toolchain from
  [scripts/tool-versions.env](scripts/tool-versions.env).
- Node ≥ 24 with npm. pnpm is installed into `.bin/`.
- Docker running. Use `SKIP_DB=1` to skip Postgres.
- `git`, `make` (macOS make 3.81 is fine), `curl`, `tar`, `shasum` or `sha256sum`, `lsof`.

Never install a tool globally. `make setup` puts every pinned, checksum-verified tool into `.bin/`.

`make setup` is run once per clone or worktree. Each worktree gets:

- its own Postgres;
- its own ports;
- its own `.env.local`.

Never read, print or copy `.env.local` between worktrees. `make setup` also points `core.hooksPath` at
`scripts/git-hooks`, which installs the pre-commit and commit-msg hooks. Until `.bin/harness` is built, the
Claude Code hooks fail closed.

## (a) New project

1. **Create the repo** from this template, using GitHub's "Use this template" or a fresh clone with new
   history.
2. **Set up and prove green:**
   ```sh
   make setup
   make check
   make harness-selftest
   ```
3. **Rename the template identity (human).** One command renames:
   - the package name (`harness-template`, `@harness-template/*`);
   - the Go module path (`github.com/aheumaier/harness-template`);
   - the CODEOWNERS owner (`@aheumaier`).

   It is a dry run that lists each file and its changed lines; `WRITE=1` applies:
   ```sh
   make rename NAME=acme-shop MODULE=github.com/acme/shop OWNER=@acme
   make rename NAME=acme-shop MODULE=github.com/acme/shop OWNER=@acme WRITE=1
   make gen check-all harness-selftest
   ```
   - **What it covers:** every git-known text file. That includes go.mod files and imports, linter configs,
     `api/openapi.yaml`, package.json files, `pnpm-lock.yaml`, the generated client and the fixture overlays.
   - **What it skips:** history (`docs/adr/`, `docs/exec-plans/`) and this README. Rewrite the README for your
     product.
   - **Running it again:** a second run changes nothing.
   - **Protected files:** many of the changed files are protected, so the rename PR needs the
     `harness-change-approved` label.
4. **Ownership (human).**
   - Replace `@aheumaier` in [.github/CODEOWNERS](.github/CODEOWNERS).
   - Create the labels `harness-change-approved`, `breaking-change-approved` and `no-spec-needed`
     ([ci.md](docs/harness/ci.md)).
   - Dry-run [scripts/github-branch-protection.sh](scripts/github-branch-protection.sh), then apply it with
     `CONFIRM=1` to require the six status checks.
5. **Constitution.** Run `/speckit-constitution` to adapt `.specify/memory/constitution.md` to the product.
   This file is protected, so it needs a human or `HARNESS_EDIT=1`.
6. **First domain.** Follow the [new-domain skill](.claude/skills/new-domain/SKILL.md):
   ```sh
   make new-domain NAME=billing
   make check
   ```
   Then replace the stubs in this order, running `make check` after each:
   1. the contract;
   2. the migration ([add-migration](.claude/skills/add-migration/SKILL.md));
   3. the queries;
   4. the service and API ([add-endpoint](.claude/skills/add-endpoint/SKILL.md)).

   Never create `apps/api/internal/<domain>/` by hand.
7. **First feature.**
   - Write the spec with `/speckit-specify` → `/speckit-plan` → `/speckit-tasks` → `/speckit-implement`. It
     lands in `specs/NNN-slug/`. A `feat` commit touching `apps/` or `api/` without a spec fails
     `make specs`.
   - Run `make check-all` before the PR.
   - Fill in [the PR template](.github/pull_request_template.md): contract, migration and harness change (Y/N).
   - Use Conventional Commit titles.

## (b) Brownfield project

The harness has two parts:

- **Generic parts** carry over to any stack:
  - the guide layout and `AGENTS.md`/`CLAUDE.md` pairs;
  - the rules.md format (rule, why, sensor, remediation, fixture);
  - `scripts/harness/` (`run.sh`, `selftest.sh`, budgets, timing, integrity), the git hooks, `.claude/`
    hooks, agents and skills;
  - the CI and CODEOWNERS shape;
  - ADRs and exec plans.
- **Stack-specific parts** carry over only if the stack matches (Go, React/TS, Postgres):
  - the `go-*`, `web-*` and `db-*` rules;
  - their linter configs;
  - the generators (oapi-codegen, sqlc, orval, atlas).

There is no adoption script and no template sync yet (see "Keeping up with the template" below). Adopt by
hand, in one reviewed harness PR:

1. **Branch** in the existing repo, for example `harness/adopt`. Note the template commit you copy from, in the
   PR description and in an ADR.
2. **Copy the generic harness.** Bring over:
   - `AGENTS.md`, `CLAUDE.md`;
   - `docs/harness/`, `docs/adr/` (format and the ADRs you keep), `docs/exec-plans/`;
   - `scripts/harness/`, `scripts/git-hooks/`, `scripts/install-tools.sh`, `scripts/tool-versions.env`,
     `scripts/setup-worktree.sh`;
   - `apps/api/tools/harness/`. It builds `.bin/harness`, and the hooks fail closed without that binary;
   - `.claude/`, `.github/`, `trivy.yaml`.

   Merge `Makefile`, `.github/workflows/` and `.gitignore` into the existing ones; don't overwrite them. Keep
   the `make` target names, because hooks, CI and guides call them.
3. **Rename the copied identity (human).** The copied files still carry `harness-template`,
   `github.com/aheumaier/harness-template` and `@aheumaier`. Rename them to your product's:
   ```sh
   make rename NAME=<your-product> MODULE=<your-module-root> OWNER=<@you-or-@org/team>
   make rename NAME=<your-product> MODULE=<your-module-root> OWNER=<@you-or-@org/team> WRITE=1
   ```
   - `MODULE` replaces the template's module root. Copied code then imports `<MODULE>/apps/api/...`. Pick the
     value that makes this match your Go module. If your layout has no `apps/api`, fix the remaining import
     paths in step 5, when you port the fixtures.
   - The rename scans your existing files too. Read the dry-run list before you add `WRITE=1`.
   - It skips `docs/adr/`, `docs/exec-plans/` and `README.md`, so copied ADRs keep their history and your own
     README is left alone.
4. **Describe what exists.**
   - Rewrite [ARCHITECTURE.md](ARCHITECTURE.md) for the real codemap and layers.
   - Rewrite the "Layer rule" and "Commands" sections of `AGENTS.md`.
   - Add an `AGENTS.md` (plus a `CLAUDE.md` containing `@AGENTS.md`) per area that has its own rules.
   - Point the layer sensors at the real layout: `.go-arch-lint.yml`, `.dependency-cruiser.cjs`, or your
     stack's equivalent. Record the layering decision in an ADR.
5. **Keep, adapt or drop each rule in [rules.md](docs/harness/rules.md).**
   - A rule that doesn't fit the stack is removed together with its fixture and its id in the Makefile rule
     lists, plus one ADR line saying why.
   - A rule you keep must still reject its fixture. Port fixtures to your layout until these are green:
     ```sh
     make harness-selftest
     make fixture-claims
     ```
6. **Run `make setup && make check`, and expect red.** Don't loosen a sensor to get green
   ([golden principle 4](AGENTS.md#golden-principles)):
   - Fix what is cheap.
   - Give an unavoidable exception an inline suppression with `reason: … (rule: <id>)`.
   - Log the rest as rows in the [tech-debt tracker](docs/exec-plans/tech-debt-tracker.md).

   `make check` scopes most sensors to changed files (`HARNESS_SCOPE=changed`). That keeps day-to-day work
   green while the full-scope debt in `make check-all` is paid down. There is no baseline file. Each
   remaining violation is either fixed, suppressed with a reason, or tracked.
7. **Apply the human steps from (a).**
   - Steps 4–5: labels, branch protection, constitution. The rename already set the CODEOWNERS owner.
   - Set the [NFR](docs/harness/nfr.md) "Today" cells from your first measurements.
8. **New work follows (a)** steps 6–7. If the stack matches, start with `make new-domain`; otherwise use your
   own scaffold that passes the same sensors.

## Keeping up with the template

Template drift is not automated yet. [Exec plan 0003](docs/exec-plans/active/0003-harness-gap-closure.md) (P4.3)
plans two pieces:

- a `.harness-version` marker;
- `make harness-sync-check`, which diffs the protected paths against an upstream tag and never applies
  anything.

Until then, compare the protected paths against the template commit you recorded, and port changes in a harness
PR.

## Where next

| Need | Read |
|------|------|
| Session entry point, commands, golden principles | [AGENTS.md](AGENTS.md) |
| Codemap, layers, request flow | [ARCHITECTURE.md](ARCHITECTURE.md) |
| Every rule, its sensor and remediation | [docs/harness/rules.md](docs/harness/rules.md) |
| CI jobs, required checks, waiver labels | [docs/harness/ci.md](docs/harness/ci.md) |
| Harness speed and reliability promises | [docs/harness/nfr.md](docs/harness/nfr.md) |
| Decisions | [docs/adr/](docs/adr/README.md) |
| Why it is built this way | [docs/design-docs/core-beliefs.md](docs/design-docs/core-beliefs.md) |
