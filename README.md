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
session with `HARNESS_EDIT=1`. Without it, `.claude/hooks/protect-harness.sh` blocks the edit. In a
brownfield adoption, steps 3–6 of (b) change protected files throughout, so they run this way too.

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
   - **What it covers:** every tracked text file; untracked files are left alone. That includes go.mod
     files and imports, linter configs, `api/openapi.yaml`, package.json files, `pnpm-lock.yaml`, the
     generated client, the fixture overlays and the vendored `.specify/` (the constitution title).
   - **What it skips:** history (`docs/adr/`, `docs/exec-plans/`) and this README. Rewrite the README for your
     product, and by hand the template's own description in `AGENTS.md` (line 3) and
     [ARCHITECTURE.md](ARCHITECTURE.md); both are protected, so a human does it.
   - **MODULE is the repository path.** It also becomes the OpenAPI `url: https://<MODULE>` and every
     `https://github.com/aheumaier/harness-template/...` link, so use `host.tld/owner/repo`.
   - **Leftovers:** the tool lists every path and line that still holds `harness-template` or `aheumaier`
     in a form it doesn't rename (an SSH remote, a file name). Fix those by hand.
   - **Running it again:** a second run changes nothing. A value that contains a template token
     (`OWNER=@aheumaier/platform`) is refused, since a second run would rename it again.
   - **Protected files:** many of the changed files are protected, so the rename PR needs the
     `harness-change-approved` label (created in step 4; merge the rename after it).
4. **Ownership (human).**
   - Review the owners in [.github/CODEOWNERS](.github/CODEOWNERS); the rename set them to `OWNER`.
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

The harness has three kinds of parts:

- **Generic parts** carry over to any repo as they are:
  - the guide layout and `AGENTS.md`/`CLAUDE.md` pairs;
  - the rules.md format (rule, why, sensor, remediation, fixture);
  - in `scripts/harness/`: `run.sh`, `unchanged.sh`, `determinism.sh`, `check-commits.sh`, `timing.sh`,
    `nfr-report.sh`, `merge-base.sh`, `trivy.sh`, `hook-bootstrap.sh`, and `selftest.sh` except its
    `node_modules` copy and its `atlas migrate hash` call;
  - in `.claude/hooks/`: `protect-harness.sh`, `guard-bash.sh` and `only-make-check.sh`;
  - the Go harness tool in `apps/api/tools/harness/`, except the two path excludes in `integritycheck.go`;
  - the shape of CI and CODEOWNERS, ADRs and exec plans.
- **Layout-coupled parts** hard-code the template layout: `apps/api`, `api/openapi.yaml`, `packages/` and
  pnpm. That is every other script in `scripts/harness/`, `.claude/hooks/fast-check.sh`,
  `scripts/install-tools.sh`, `scripts/setup-worktree.sh`, the `Makefile`, `.github/` and CODEOWNERS. Copy
  them anyway; step 5 re-points them.
- **Stack-specific parts** carry over only if the stack matches (Go, React/TS, OpenAPI, Postgres):
  - the `go-*`, `web-*`, `api-*` and `db-*` rules;
  - their linter configs;
  - the generators (oapi-codegen, sqlc, orval, atlas).

There is no adoption script and no template sync yet (see "Keeping up with the template" below). Adopt by
hand, in one reviewed harness PR:

1. **Branch** in the existing repo, for example `harness/adopt`. Note the template commit you copy from, in the
   PR description and in an ADR.
2. **Build the hooks' parser first.** The template's Claude Code hooks fail closed while `.bin/harness` is
   missing. Every Bash call is then blocked, and without `HARNESS_EDIT=1` every edit too, except `make setup`
   and `make .bin/harness` ([hook-bootstrap.sh](scripts/harness/hook-bootstrap.sh)). The block message says to
   run `make setup`, but
   on an existing repo `make setup` cannot be the first command:
   - [install-tools.sh](scripts/install-tools.sh) exits 1 unless the root `package.json` has
     `"packageManager": "pnpm@<PNPM_VERSION>"`, with the version from
     [tool-versions.env](scripts/tool-versions.env);
   - `make deps` runs `pnpm install --frozen-lockfile`, which needs a `pnpm-lock.yaml`;
   - [setup-worktree.sh](scripts/setup-worktree.sh) starts a compose `db` service and runs
     `cd apps/api && atlas migrate apply`.

   So build the parser alone; it needs only Go. Copy `apps/api/tools/harness/` and
   `scripts/tool-versions.env`. If the repo has no Makefile, copy the template's and run `make .bin/harness`
   (the `$(HARNESS)` recipe, [Makefile](Makefile) lines 98–99). If the repo has its own Makefile, run that
   recipe by hand, from the repo root:
   ```sh
   out="$PWD/.bin/harness" && (source scripts/tool-versions.env && cd apps/api/tools/harness && GOWORK=off GOTOOLCHAIN=$GO_TOOLCHAIN go build -o "$out" .)
   ```
   The hooks look for `<root>/.bin/harness`, so the output path must be absolute, as in the recipe. `GOWORK=off`
   keeps a root `go.work` that does not list the tool module from breaking the build; the `make` recipe lacks
   it, so with such a `go.work` add the module to it first. Check with `.bin/harness`, which prints its usage.
   Do this before step 3 merges `.claude/settings.json`, because that file is what turns the hooks on.
3. **Copy and merge the harness.** Copy these:
   - `docs/harness/`, `docs/adr/` (format and the ADRs you keep), `docs/exec-plans/`;
   - `scripts/harness/`, `scripts/git-hooks/`, `scripts/test/`, `scripts/install-tools.sh`,
     `scripts/setup-worktree.sh`, `scripts/github-branch-protection.sh`;
   - `.github/`, `trivy.yaml`, `.shellcheckrc` and `.editorconfig`;
   - every guide the template's `AGENTS.md` "Where to look" table links to: `ARCHITECTURE.md`,
     `docs/design-docs/`, `docs/QUALITY.md`, `docs/SECURITY.md`, `docs/RELIABILITY.md`, `docs/generated/` and
     the area `AGENTS.md`/`CLAUDE.md` pairs. Step 5 rewrites or prunes them; a link to a missing file turns
     [docs-links-resolve](docs/harness/rules.md#docs-links-resolve) red in `make check`;
   - `compose.yaml`, if you keep db (`setup-worktree.sh` starts its `db` service);
   - the linter configs of each stack you keep, for example `.spectral.yaml` for api and `.squawk.toml` for db.
     For Go: `apps/api/.golangci.yml`, `apps/api/.custom-gcl.yml`, `apps/api/.go-arch-lint.yml` and
     `apps/api/tools/analyzers/`.

   Merge these into what the repo already has; never overwrite them:
   - **`AGENTS.md` and `CLAUDE.md`.** Move the body of your `CLAUDE.md` into `AGENTS.md`, next to the
     template's sections. `CLAUDE.md` keeps an `@AGENTS.md` line plus Claude-only notes, nothing else. The
     [docs-claude-imports-agents](docs/harness/rules.md#docs-claude-imports-agents) rule requires exactly
     this: every `AGENTS.md` has a sibling `CLAUDE.md` with an `@AGENTS.md` line.
   - **`.claude/`.** Copy the template's hook scripts and agents. In your own `.claude/settings.json`, add the
     template's three hooks (`protect-harness.sh`, `guard-bash.sh`, `fast-check.sh`) and its
     `permissions.deny` list next to what is there. Decide per existing hook whether it stays, and say so in
     the PR. Never drop a Stop or dev-gate hook silently: it may be the repo's definition of done. Keep one
     copy of each `speckit-*` skill. Copy `new-domain`, `add-endpoint` and `add-migration` only if the stack
     matches.
   - **`.specify/`.** Keep one copy. A repo that already runs spec-kit keeps its own constitution and
     `feature.json`. Once the harness is in, `.specify/memory/` is protected, so a later constitution change
     needs a human or `HARNESS_EDIT=1`.
   - **`Makefile`, `.github/workflows/` and `.gitignore`.** Keep the `make` target names, because hooks, CI and
     guides call them.
     - With no Makefile of your own, the template's is adopted whole. Its `API :=` line, `WITH_ENV` (which
       needs `.env.local`) and its web and db targets are then handled in steps 5–6.
     - When target names collide (`check`, `fmt`, `gen`, `lint`), the template's recipe gets the name. Rename
       the old recipe. If it was a test or CI entry point, add it to `check-all`, and point any CI job that
       called the old name at the new one.
   - **Harness and product overlap.** The product may itself live in `.github/` (reusable workflows,
     composite actions), the `Makefile` or `scripts/`. Then narrow
     [protected-paths.txt](scripts/harness/protected-paths.txt) and [CODEOWNERS](.github/CODEOWNERS) to the
     harness's own files before the first `make harness-integrity`: `ci.yml`, `harness-*.yml`,
     `contract-gate.yml`, `migration-gate.yml`, `drift.yml` and `.github/actions/setup`. Otherwise
     protect-harness blocks every product edit, and
     [harness-config-not-loosened](docs/harness/rules.md#harness-config-not-loosened) demands the
     `harness-change-approved` label on every product PR.
4. **Rename the copied identity (human).** The copied files still carry `harness-template`,
   `github.com/aheumaier/harness-template` and `@aheumaier`. Rename them to your product's:
   ```sh
   make rename NAME=<your-product> MODULE=<your-module-root> OWNER=<@you-or-@org/team>
   make rename NAME=<your-product> MODULE=<your-module-root> OWNER=<@you-or-@org/team> WRITE=1
   ```
   - `MODULE` must be a Go module path of the form `host.tld/path`
     ([rename.go](apps/api/tools/harness/rename.go), line 25). A bare module such as `emergencies` is refused.
     Either change your Go module path first, or skip `make rename` and replace the three tokens by hand, in
     every tracked file except `docs/adr/`, `docs/exec-plans/`, `README.md`, `rename.go` and `rename_test.go`:
     `github.com/aheumaier/harness-template`, then `harness-template`, then `@aheumaier`. Replace the module
     path before the name, since the name is part of it. A blanket replace that reaches `rename.go` rewrites
     the template constants and breaks its test. Then run `git grep -n -e harness-template -e aheumaier`; only
     the five excluded paths may still hold them.
   - `MODULE` replaces the template's module root. Copied code then imports `<MODULE>/apps/api/...`. With a
     `host.tld/path` module, pick the value that makes this match your Go module. If your layout has no
     `apps/api`, fix the remaining import paths in step 6, when you port the fixtures.
   - The copied harness tool's `go.mod` becomes `<MODULE>/apps/api/tools/harness`, a nested module of its own.
     That path is fine to keep even when the repo has no `apps/api`. The same holds for `tools/analyzers`.
   - The rename scans your existing tracked files too. Read the dry-run list, and its leftovers, before you
     add `WRITE=1`. `git add` the copied files first, since untracked files are not renamed.
   - It skips `docs/adr/`, `docs/exec-plans/` and `README.md`, so copied ADRs keep their history and your own
     README is left alone.
5. **Describe what exists and re-point the sensors.** Every file this step changes is protected, so the whole
   step runs as a human or with `HARNESS_EDIT=1`.
   - Rewrite [ARCHITECTURE.md](ARCHITECTURE.md) for the real codemap and layers.
   - Rewrite the "Layer rule" and "Commands" sections of `AGENTS.md`.
   - Add an `AGENTS.md` (plus a `CLAUDE.md` containing `@AGENTS.md`) per area that has its own rules.
   - Re-point each file below at the real layout, or drop it with its rule in step 6. Left alone, most of them
     do not turn red. They skip, so the sensor looks green while it checks nothing. After each change, make
     the sensor fire once on a deliberate violation.

     | File | Hard-codes | Left alone |
     |------|------------|------------|
     | [file-sensors.sh](scripts/harness/file-sensors.sh) | file kinds `apps/api/*.go`, `api/openapi.yaml`, `apps/*.ts` (lines 24–31) | your files get no kind, so the pre-commit hook checks nothing |
     | [affected.sh](scripts/harness/affected.sh) | `apps/api/` prefix, `cd apps/api` (lines 33–36, 67) | no changed file starts with `apps/api/`, so the affected set is empty and `make check` skips `lint-go` and `test-go`; [harness-scope](docs/harness/rules.md#harness-scope) turns into no check at all |
     | [check-specs.sh](scripts/harness/check-specs.sh) | `product=('apps/*' 'api/*')` (line 59) | [specs-required-for-features](docs/harness/rules.md#specs-required-for-features) never fires on your code, and fires on a `feat` commit that touches an unrelated `api/` directory |
     | [gen-check.sh](scripts/harness/gen-check.sh) | generated files and generator inputs (lines 14–18) | drift in your generated code is never seen |
     | [pre-commit.sh](scripts/harness/pre-commit.sh) | `--config apps/api/.golangci.yml` (line 77) | once the Go configs move next to your `go.mod`, go-format at commit reads a config that is not there |
     | [selftest.sh](scripts/harness/selftest.sh) | `apps/*/node_modules` copy, `cd $work/apps/api` (lines 40, 54–55) | migration fixtures fail for the wrong reason |
     | [migrations-append-only.sh](scripts/harness/migrations-append-only.sh), [squawk.sh](scripts/harness/squawk.sh) | `apps/api/db/migrations/*.sql` (lines 12 and 8) | append-only sees no files and passes; squawk checks a path that does not exist |
     | [contract-check.sh](scripts/harness/contract-check.sh) | `SPEC=api/openapi.yaml` (line 10) | with no contract at that path on the base branch it exits 0 |
     | [fast-check.sh](.claude/hooks/fast-check.sh) | every branch keyed on `apps/api`, `api/openapi.yaml`, `packages/` (lines 44–82) | edits to your code get no per-file check |
     | [integritycheck.go](apps/api/tools/harness/integritycheck.go) | excludes `apps/api/internal/*/store/*`, `packages/api-client/src/gen/*` (lines 36–37) | your generated code is judged as hand-written |
     | [action.yml](.github/actions/setup/action.yml) | `go-version-file: apps/api/go.mod` (line 21), `.bin/pnpm store path` (line 50), `make deps` (lines 84–87) and, for the browser cache, `./apps/web/package.json` (line 69) | every CI job fails in setup; with input `browser`, the `check-all` job fails there |
     | [contract-gate.yml](.github/workflows/contract-gate.yml), [harness-integrity.yml](.github/workflows/harness-integrity.yml) | `go-version-file: apps/api/go.mod` (lines 44, 49), `api/openapi.yaml` | the gates fail in setup, or never see a contract change |
     | [ci.yml](.github/workflows/ci.yml) | the `make new-domain` probe job (lines 83–108) | it fails, since the generator writes the template layout |
     | [Makefile](Makefile) | `API := apps/api` (line 21), `../../$(GCL)` (lines 104, 108), `$(API)/tools/harness` and `$(API)/tools/analyzers` (lines 28, 30, 99, 117, 120) | every Go sensor runs in `apps/api`, which holds only the copied tools, not your code. Set `API` to your `go.mod` directory and move the three Go configs there with it (the `$(GCL)` recipe, lines 91–94, follows them). Fix the two `../../` paths, and `destination: ../../.bin` and `path: ./tools/analyzers` in `.custom-gcl.yml`. In the five `$(API)/tools/` lines, write the literal path where you copied the tools, or `test-analyzers` and `test-harness` look under `<API>/tools/` and go red |
     | [install-tools.sh](scripts/install-tools.sh) | installs pnpm and exits 1 unless the root `package.json` pins `packageManager` (lines 97–108) | `make setup` aborts before the other tools land; see step 6 on why a root `package.json` stays |
     | [setup-worktree.sh](scripts/setup-worktree.sh) | `make tools deps` (line 71), compose `db` and `cd apps/api && atlas migrate apply` (lines 93, 99) | `make setup` fails in `pnpm install` without a lockfile; `SKIP_DB=1` skips only the db part |
     | [domainiso.go](apps/api/tools/analyzers/domainiso/domainiso.go) | the layer is the second path element under `internal/` (lines 28–38) | a flat `internal/<pkg>` has no layer, so go-domain-isolation reports nothing |
     | [.golangci.yml](apps/api/.golangci.yml) | depguard paths `**/internal/*/store/**`, `**/internal/*/api/**`; forbidigo exclusion `internal/platform/` | go-db-only-in-store and go-http-only-in-api judge the wrong files |
     | [.go-arch-lint.yml](apps/api/.go-arch-lint.yml) | components `internal/*/model/**` to `internal/*/api/**` (lines 16–22) | `ignoreNotFoundComponents: true`, so go-layers passes with zero matches |
     | [CODEOWNERS](.github/CODEOWNERS) | `/api/openapi.yaml`, `/apps/api/db/migrations/`, `/apps/api/tools/*` | contract and migration reviews never trigger |
     | `scripts/test/*.bats`, for example [affected.bats](scripts/test/affected.bats) | template paths in the test cases of the scripts above | they keep testing the old paths; update each with its script |

   - Point the web layer sensor too if you keep web: [.dependency-cruiser.cjs](.dependency-cruiser.cjs) or your
     stack's equivalent. Record the layering decision in an ADR.
6. **Keep, adapt or drop each rule in [rules.md](docs/harness/rules.md).**
   - A dropped rule takes out more than its id. Remove all of this, plus one ADR line saying why:
     - its whole `make` target from `check` ([Makefile](Makefile) line 63) and `check-all` (line 70);
     - its generator from `gen` (line 236) and its paths from `GEN_RE` and `INPUTS` in
       [gen-check.sh](scripts/harness/gen-check.sh) (lines 14–18). `gen-check`, the last step of `make check`,
       runs `make gen` ([gen-up-to-date](docs/harness/rules.md#gen-up-to-date)); a deleted target that `gen`
       still names fails with `No rule to make target`;
     - its fixture directory under `scripts/harness/fixtures/`;
     - its branch in `fast-check.sh` and its kind in `file-sensors.sh`;
     - its tool from `install-tools.sh` and `tool-versions.env`, or from the root `package.json`;
     - its gate workflow and its CODEOWNERS lines.
   - Rules drop in sets:

     | Set | Rules | `make` targets | Tools |
     |-----|-------|----------------|-------|
     | web | `web-*` (7) | `lint-web`, `arch-web`, `typecheck-web`, `test-web`, `knip`, `gen-api-client` | eslint, dependency-cruiser, turbo, knip, typescript from `package.json` |
     | db | `db-*` (5) | `lint-migrations`, `squawk`, `migrations-append-only`, `schema-doc-fresh`, `gen-schema-doc`, `gen-sqlc`, `gen-atlas-sum`, Postgres in `test-integration` | atlas, sqlc; squawk from `package.json` |
     | api | `api-*` (6) | `lint-api`, `contract-check`, `gen-oapi` | oasdiff, oapi-codegen; spectral from `package.json` |

     `schema-doc-fresh` sits in the fast `check` list, not only in `check-all`. Dropping the web set does not
     drop pnpm: the root `package.json` also provides bats, squawk and spectral, and `test-sh`
     ([sh-tests-pass](docs/harness/rules.md#sh-tests-pass), in `make check`) runs the harness's own bats tests.
     Keep a root `package.json` with `packageManager` and the dev dependencies the remaining sets need (at
     least `bats`), or install those tools another way and take `deps` out of `setup-worktree.sh` line 71 and
     `action.yml` lines 50 and 84–87 as well.
   - A rule you keep must still reject its fixture. Port fixtures to your layout until these are green:
     ```sh
     make harness-selftest
     make fixture-claims
     ```
     `make harness-selftest` cannot be green until the dropped fixtures are gone. It asserts each fixture's
     `EXPECT` pattern, and a fixture for a missing `apps/web` fails for the wrong reason.
7. **Run `make setup && make check`.** Expect `make setup` to need steps 5–6 first, because of the
   `install-tools.sh`, `make deps` and `setup-worktree.sh` assumptions in step 2. Without Postgres, run
   `SKIP_DB=1 make setup`. Then run `make check` and expect red. Don't loosen a sensor to get green
   ([golden principle 4](AGENTS.md#golden-principles)):
   - Fix what is cheap.
   - Give an unavoidable exception an inline suppression with `reason: … (rule: <id>)`.
   - Log the rest as rows in the [tech-debt tracker](docs/exec-plans/tech-debt-tracker.md).

   On the adoption branch every harness file differs from the base, so `make check` runs at full scope
   ([harness-scope](docs/harness/rules.md#harness-scope)); the changed-file scope only helps once the PR has
   merged. There is no baseline file, and CI requires `check-all` green, so a tracker row alone does not get
   the PR in. Each remaining violation is fixed, suppressed with a reason, or its rule is dropped in step 6
   before the PR merges; the tracker row records what was dropped or suppressed and why.
8. **Apply the human steps from (a).**
   - Steps 4–5: labels, branch protection, constitution. The rename already set the CODEOWNERS owner.
   - Set the [NFR](docs/harness/nfr.md) "Today" cells from your first measurements.
9. **New work follows (a)** steps 6–7. If the stack matches, start with `make new-domain`; otherwise use your
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
