# harness-template

A template for products built by coding agents. It has a Go HTTP API (`apps/api`), a React web app (`apps/web`),
an iOS app (`apps/ios`: a Swift package and an XcodeGen app target), a shared OpenAPI contract
(`api/openapi.yaml`) and Postgres. There are no business domains yet. What the template really offers is the
**harness**:

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
- Node ≥ 24 with npm. pnpm is installed into `.bin/`. Only the `web` group of `STACK` needs them.
- Docker running, for Postgres. Use `SKIP_DB=1` to skip it once, or leave `db` out of `STACK`.
- `git`, `make` (macOS make 3.81 is fine), `curl`, `tar`, `shasum` or `sha256sum`, `lsof`.

`STACK` in [scripts/harness/layout.env](scripts/harness/layout.env) lists the sensor groups the product has,
`go:db:web:ios` here; `HARNESS_STACK=go,db` overrides it for one run
([ADR 0033](docs/adr/0033-stack-profiles.md), [ADR 0041](docs/adr/0041-ios-stack-group.md)). The `ios`
sensors need macOS (Xcode or Apple's command-line tools): on another host `make check` leaves them out and says
so ([ADR 0042](docs/adr/0042-swift-unit-tests-in-make-check.md)). With only the command-line tools, `make check`
runs the Swift tests, SwiftLint and swift-format, and `make check-ios` skips the Xcode version check and the app
build, which need Xcode selected ([ADR 0043](docs/adr/0043-swift-lint-format-and-xcode-pin.md)). CI's `check-ios`
job runs all of them on a macOS runner with Xcode ([ADR 0044](docs/adr/0044-ios-ci-on-macos.md)).

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
   history. Write the template commit you copied from to `.harness-version`, so
   [template sync](#keeping-up-with-the-template) can tell what you have not taken yet (step 3 points
   `HARNESS_UPSTREAM` at the template). The copied file still holds the template's own older marker, not
   the commit you copied, so replace it. Make the first commit before step 2: the web layer sensor
   (dependency-cruiser) fails in a repo without `HEAD`. Until `make setup` has built `.bin/harness`, the
   Claude Code hooks block every shell command except a bare `make setup` (`cd <repo> && make setup` is
   blocked too). Without an iOS app, delete `apps/ios` and drop `ios` from `STACK` in
   `scripts/harness/layout.env` in the same commit ([ADR 0041](docs/adr/0041-ios-stack-group.md)):
   `make layout-check` fails while Swift stays and `ios` is gone. `.swiftlint.yml`, `.swift-format` and
   `scripts/swift-format/` may go too: only the ios sensors read them.
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

   It is a dry run that lists each file with the number of lines that would change; `WRITE=1` applies.
   `make deps` then relinks the renamed workspace packages, or lint-web cannot resolve the API client's types:
   ```sh
   make rename NAME=acme-shop MODULE=github.com/acme/shop OWNER=@acme
   make rename NAME=acme-shop MODULE=github.com/acme/shop OWNER=@acme WRITE=1
   make deps
   make gen check-all harness-selftest
   ```
   - **What it covers:** every tracked text file; untracked files are left alone. That includes go.mod
     files and imports, linter configs, `api/openapi.yaml`, package.json files, `pnpm-lock.yaml`, the
     generated client, the fixture overlays and the vendored `.specify/` (the constitution title).
   - **What it skips:** history (`docs/adr/`, `docs/exec-plans/`) and this README. Rewrite the README for your
     product, and by hand the template's own description in `AGENTS.md` (line 3) and
     [ARCHITECTURE.md](ARCHITECTURE.md); both are protected, so a human does it.
   - **MODULE is the repository path.** It also becomes the OpenAPI `url: https://<MODULE>` and every
     `https://github.com/aheumaier/harness-template/...` link, so use `host.tld/owner/repo`. A bare module
     (no dot in its first element, such as `shop`) is accepted too; the rename then keeps those URLs and lists
     them as leftovers, since `https://shop` is no URL.
   - **Template sync:** it also writes `HARNESS_UPSTREAM=https://github.com/aheumaier/harness-template` to
     `scripts/tool-versions.env`, the one line it neither renames nor reports: from here on the repo is a
     product that [template sync](#keeping-up-with-the-template) compares with the template. The same line
     switches off the template-only adoption probe, which replays this README's section (b) that you rewrite:
     `make adoption-probe` and CI's `adoption-probe` job pass without running, and `make harness-selftest` skips
     its fixture ([ADR 0038](docs/adr/0038-readme-brownfield-adoption-is-probed.md)).
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
     `CONFIRM=1` to require the eight status checks. One of them, `adoption-probe`, passes without running once
     step 3's `make rename WRITE=1` has set `HARNESS_UPSTREAM`; another, `check-ios`, passes with a notice without
     `ios` in `STACK`.
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
   - Fill in [the PR template](.github/pull_request_template.md): contract, migration and harness change
     (Y/N), and under Assumptions each default you picked without asking (a timeout, limit, name or fallback).
   - Use Conventional Commit titles.

## (b) Brownfield project

For a codebase on another stack (PHP, .NET), [docs/applying-the-harness.md](docs/applying-the-harness.md)
explains each principle, sensor and hook and maps them to that stack's tools.

The harness has three kinds of parts:

- **Generic parts** carry over to any repo as they are:
  - the guide layout and `AGENTS.md`/`CLAUDE.md` pairs;
  - the rules.md format (rule, why, sensor, remediation, fixture);
  - in `scripts/harness/`: `run.sh`, `unchanged.sh`, `determinism.sh`, `timing.sh`, `nfr-report.sh`,
    `merge-base.sh`, `trivy.sh`, `hook-bootstrap.sh`, `sync-check.sh`, and `selftest.sh`;
  - in `.claude/hooks/`: `protect-harness.sh`, `guard-bash.sh`, `only-make-check.sh` and `only-test-files.sh`;
  - the Go harness tool in `<API_DIR>/tools/harness/` (integrity-check's excludes come from `GENERATED_GLOBS`);
  - the shape of CI and CODEOWNERS, ADRs and exec plans.
- **Layout-coupled parts** read where the code lives from one file,
  [scripts/harness/layout.env](scripts/harness/layout.env) ([ADR 0030](docs/adr/0030-one-layout-file-for-every-sensor.md)):
  every other script in `scripts/harness/`, the `Makefile`, `.bin/harness`, the domainiso analyzer and the
  CI workflows. Step 5 writes your layout into it. The files that cannot read it (linter configs, CODEOWNERS,
  codegen configs, `fast-check.sh`, ...) are listed in step 5, each to be re-pointed by hand.
- **Stack-specific parts** carry over only if the stack matches (Go, React/TS, OpenAPI, Postgres, Swift):
  - the `go-*`, `web-*`, `api-*`, `db-*` and `ios-*` rules;
  - their linter configs;
  - the generators (oapi-codegen, sqlc, orval, atlas).

There is no adoption script; once adopted, [template sync](#keeping-up-with-the-template) reports what the
template changed since. Adopt by hand, in one reviewed harness PR:

1. **Branch** in the existing repo, for example `harness/adopt`. Note the template commit you copy from, in the
   PR description, in an ADR and in `.harness-version`. `make rename` (step 4) sets `HARNESS_UPSTREAM` in
   `scripts/tool-versions.env` to the template's git URL; until then the value is empty, which means "this
   repository is the template", so template sync stays silent; once it is set, the template-only adoption probe
   (step 6) passes without running.
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

   A `STACK` without `web` (step 5) drops the first two, one without `db` the third.

   So build the parser alone; it needs only Go:
   1. Copy `scripts/tool-versions.env`, [scripts/harness/layout.env](scripts/harness/layout.env) and
      [scripts/harness/layout.sh](scripts/harness/layout.sh).
   2. Set `API_DIR` in `layout.env` to the directory of your Go module (`.` for the repo root). Step 5 fills
      in the other keys; until then they keep the template's values, which parse.
   3. Copy the template's `apps/api/tools/harness/` to `<API_DIR>/tools/harness/`.
   4. If the repo has no Makefile, copy the template's and run `make .bin/harness` (its `$(HARNESS)` target,
      which builds `$(API_DIR)/tools/harness`). If the repo has its own Makefile, run that recipe by hand,
      from the repo root; it reads `API_DIR` through the validating parser:
      ```sh
      out="$PWD/.bin/harness" && api="$(scripts/harness/layout.sh --print | sed -n 's/^API_DIR=//p')" && [ -n "$api" ] && (source scripts/tool-versions.env && cd "$api/tools/harness" && GOWORK=off GOTOOLCHAIN=$GO_TOOLCHAIN go build -o "$out" .)
      ```
   The hooks look for `<root>/.bin/harness`, so the output path must be absolute, as in the recipe. `GOWORK=off`
   keeps a root `go.work` that does not list the tool module from breaking the build; the `make` recipe lacks
   it, so with such a `go.work` add the module to it first. Check with `.bin/harness`, which prints its usage.
   Do this before step 3 merges `.claude/settings.json`, because that file is what turns the hooks on.
3. **Copy and merge the harness.** Copy these:
   - `docs/harness/`, `docs/adr/` (format and the ADRs you keep), `docs/exec-plans/`;
   - `scripts/harness/` (but `layout.env`, which step 2 copied and edited), `scripts/git-hooks/`,
     `scripts/test/`, `scripts/install-tools.sh`, `scripts/swift-format/Package.resolved`, `scripts/setup-worktree.sh`,
     `scripts/github-branch-protection.sh`;
   - the workflows and action listed in [protected-paths.txt](scripts/harness/protected-paths.txt),
     `.github/CODEOWNERS` and `.github/pull_request_template.md` (not `.github/` whole: the repo's own
     workflows there stay its own), `.swiftlint.yml`, `.swift-format`, `trivy.yaml`, `.shellcheckrc` and
     `.editorconfig`;
   - every guide the template's `AGENTS.md` "Where to look" table links to: `ARCHITECTURE.md`,
     `docs/design-docs/`, `docs/QUALITY.md`, `docs/SECURITY.md`, `docs/RELIABILITY.md`, `docs/generated/`,
     `docs/references/` and the area `AGENTS.md`/`CLAUDE.md` pairs (`apps/api/internal/`'s pair goes to
     `<API_DIR>/internal/`). Step 5 rewrites or prunes them; a link to a missing file turns
     [docs-links-resolve](docs/harness/rules.md#docs-links-resolve) red in `make check`;
   - `compose.yaml`, if you keep db (`setup-worktree.sh` starts its `db` service);
   - the linter configs of each stack you keep, for example `.spectral.yaml` for api and `.squawk.toml` for db.
     For Go: the template's `apps/api/.golangci.yml`, `apps/api/.custom-gcl.yml` and `apps/api/.go-arch-lint.yml`
     go to `<API_DIR>/`, and `apps/api/tools/analyzers/` to `<API_DIR>/tools/analyzers/`.

   Merge these into what the repo already has; never overwrite them:
   - **`AGENTS.md` and `CLAUDE.md`.** Move the body of your `CLAUDE.md` into `AGENTS.md`, next to the
     template's sections. `CLAUDE.md` keeps an `@AGENTS.md` line plus Claude-only notes, nothing else. The
     [docs-claude-imports-agents](docs/harness/rules.md#docs-claude-imports-agents) rule requires exactly
     this: every `AGENTS.md` has a sibling `CLAUDE.md` with an `@AGENTS.md` line.
   - **`.claude/`.** Copy the template's hook scripts, agents and skills. In your own `.claude/settings.json`, add the
     template's three hooks (`protect-harness.sh`, `guard-bash.sh`, `fast-check.sh`) and its
     `permissions.deny` list next to what is there. Decide per existing hook whether it stays, and say so in
     the PR. Never drop a Stop or dev-gate hook silently: it may be the repo's definition of done. Keep one
     copy of each `speckit-*` skill. Leave out the stack skills you have no use for: `add-endpoint` without
     the api set, `add-migration` without db, and `new-domain` when you drop `make new-domain` (step 5's table).
   - **`.specify/`.** Keep one copy. A repo that already runs spec-kit keeps its own constitution and
     `feature.json`. Once the harness is in, `.specify/memory/` is protected, so a later constitution change
     needs a human or `HARNESS_EDIT=1`.
   - **`Makefile`, `.github/workflows/` and `.gitignore`.** Keep the `make` target names, because hooks, CI and
     guides call them.
     - With no Makefile of your own, the template's is adopted whole. It reads its paths from `layout.env`
       (step 5); `WITH_ENV` (which needs `.env.local`) and its web and db targets are handled in steps 5–6.
     - When target names collide (`check`, `fmt`, `gen`, `lint`), the template's recipe gets the name. Rename
       the old recipe. If it was a test or CI entry point, add it to `check-all`, and point any CI job that
       called the old name at the new one.
   - **Harness and product overlap.** The harness's own workflows and action are the files whose first line
     is `# Harness file (protected)`, each named in [protected-paths.txt](scripts/harness/protected-paths.txt)
     and [CODEOWNERS](.github/CODEOWNERS)
     ([ADR 0035](docs/adr/0035-harness-files-under-github-are-named.md)). The product's own workflows and
     composite actions in `.github/` stay the product's: agents may edit them, and
     [harness-config-not-loosened](docs/harness/rules.md#harness-config-not-loosened) does not ask for the
     `harness-change-approved` label. They still need a code owner's review, and a product job must not reuse
     a required check's name (`check-all`, `harness-integrity`, …): branch protection matches checks by name.
     A product workflow whose file name collides with a harness one (its own `ci.yml`) gets a new name instead
     of a merge, or it becomes a harness file. The `Makefile` stays protected whole: a product target added to
     it needs the label.
4. **Rename the copied identity (human).** The copied files still carry `harness-template`,
   `github.com/aheumaier/harness-template` and `@aheumaier`. Rename them to your product's:
   ```sh
   make rename NAME=<your-product> MODULE=<your-module-root|skip> OWNER=<@you-or-@org/team>
   make rename NAME=<your-product> MODULE=<your-module-root|skip> OWNER=<@you-or-@org/team> WRITE=1
   ```
   - `MODULE` is a Go module path: `host.tld[/path]`, or a bare module such as `emergencies` or
     `example/hello` (no dot in the first element; `renameModule` in
     [rename.go](apps/api/tools/harness/rename.go)). With a bare module only text starting with
     `https://github.com/aheumaier/harness-template` stays, and is listed as a leftover, since
     `https://emergencies` is no URL. Every other mention of the template's module path becomes the bare
     module and is not reported, a URL with another scheme (`http://`, `pkg.go.dev/`) or a
     `go install github.com/aheumaier/harness-template/...@latest` included: search the dry-run's files for
     those by hand.
   - `MODULE=skip` keeps every Go module path and renames only the name and the owner. Use it when your
     module path should not take the template's place in the copied Go code; it shadows a bare module
     literally named `skip`. Text holding the template's module path, its `https://` URLs included, is then
     kept and not reported as a leftover; the owner's other forms (`git@github.com:aheumaier/...`) are.
   - Unless `MODULE=skip`, `MODULE` replaces the template's module root. Copied code then imports
     `<MODULE>/apps/api/...`. Pick the value that makes this match your Go module. If your layout has no
     `apps/api`, fix the remaining import paths in step 6, when you port the fixtures.
   - The copied harness tool's `go.mod` (`<API_DIR>/tools/harness/go.mod`, `API_DIR` read from `layout.env`,
     which step 2 set) becomes `<MODULE>/<API_DIR>/tools/harness` (`<MODULE>/tools/harness` for `API_DIR=.`), a
     nested module of its own; with `MODULE=skip` it keeps the template's path. The rename prints that path.
     `tools/analyzers` keeps
     `<MODULE>/apps/api/tools/analyzers`, which is fine even when the repo has no `apps/api`.
   - The rename scans your existing tracked files too. Read the dry-run list, and its leftovers, before you
     add `WRITE=1`. `git add` the copied files first, since untracked files are not renamed.
   - It skips `docs/adr/`, `docs/exec-plans/` and `README.md`, so copied ADRs keep their history and your own
     README is left alone.
5. **Describe what exists and re-point the sensors.** Every file this step changes is protected, so the whole
   step runs as a human or with `HARNESS_EDIT=1`.
   - Rewrite [ARCHITECTURE.md](ARCHITECTURE.md) for the real codemap and layers.
   - Rewrite the "Layer rule" and "Commands" sections of `AGENTS.md`.
   - Add an `AGENTS.md` (plus a `CLAUDE.md` containing `@AGENTS.md`) per area that has its own rules.
   - Write the real layout into [scripts/harness/layout.env](scripts/harness/layout.env) first: where the Go
     module (`API_DIR`, `.` for the repo root), the contract, migrations, queries, generated client, JS
     workspaces, generated code and product code live, the domain layers' path (`LAYER_GLOB`), and which sensor groups
     the product has (`STACK`: `go`, plus `db` for Postgres migrations and queries, `web` for the JS
     workspaces, `ios` for a Swift app in `IOS_DIR`, under a `PRODUCT_GLOBS` or `WEB_GLOBS` entry (a product that
     adopted `ios` before ADR 0042 adds `IOS_DIR` when it syncs); [ADR 0033](docs/adr/0033-stack-profiles.md),
     [ADR 0041](docs/adr/0041-ios-stack-group.md), [ADR 0042](docs/adr/0042-swift-unit-tests-in-make-check.md)). If the product vendors another repo's spec, list
     it in `UPSTREAM_SPECS` with its source and revision; if other repos vendor this contract, name them in
     `CONTRACT_CONSUMERS` ([ADR 0037](docs/adr/0037-upstream-spec-drift-is-a-drift-report.md)). The scripts
     in `scripts/harness/`, the Makefile, `.bin/harness`, the domainiso analyzer and the CI workflows read it
     ([ADR 0030](docs/adr/0030-one-layout-file-for-every-sensor.md)), so this one file re-points them.
     Then re-point `protected-paths.txt` (table below) and run `make layout-check` (in `make check`,
     [harness-layout-valid](docs/harness/rules.md#harness-layout-valid)). It fails while a path layout.env names
     does not exist, `LAYER_GLOB` matches no Go package, or a flat `<dir>/<pkg>` package sits beside the layers
     (`<dir>/platform` excepted), or while no `config` entry of `protected-paths.txt` matches the harness's own
     module in `<API_DIR>/tools/harness/` (a symlink on the way to it or inside it, or a path naming it in
     another case, fails too). `API_DIR` is written plainly, `apps/api` or `.`, not `apps/api/` or `./apps/api`. `LAYER_GLOB` must end
     in two levels, `<dir>/*/*` (domain, then layer; `<dir>` may be nested, such as `pkg/domains`): a flat `internal/<pkg>` layout has no layer, so either
     restructure or drop [go-domain-isolation](docs/harness/rules.md#go-domain-isolation) and `go-layers` in
     step 6. `layout.env` is itself protected harness config: your first edit needs the
     `harness-change-approved` label once, and [template sync](#keeping-up-with-the-template) reports it
     whenever the template changes its copy (decline that with a `# declined scripts/harness/layout.env: <why>`
     line in `.harness-version`).
   - Then re-point each file below by hand, or drop it with its rule in step 6. This is the single list of
     files that cannot read `layout.env`; each names template paths itself. Left alone, most of them do not
     turn red. They skip, so the sensor looks green while it checks nothing. After each change, make the
     sensor fire once on a deliberate violation.

     | File | Hard-codes | Left alone |
     |------|------------|------------|
     | [protected-paths.txt](scripts/harness/protected-paths.txt) | `apps/api/tools/analyzers/*`, `apps/api/tools/harness/*`, `apps/api/internal/platform/approvaltest/*`, `apps/web/e2e/global-setup.ts` | with the tools at `<API_DIR>/tools/`, the harness's own Go code is unprotected: protect-harness lets edits through and [harness-config-not-loosened](docs/harness/rules.md#harness-config-not-loosened) never asks for the label. `make layout-check` fails until a `config` entry matches `<API_DIR>/tools/harness/`; it does not check the analyzers (tracker TD-130). Fix this one first |
     | [CODEOWNERS](.github/CODEOWNERS) | `/api/openapi.yaml`, `/apps/api/db/migrations/`, `/apps/api/tools/*` | contract and migration reviews never trigger (not checked by `make layout-check`, tracker TD-130) |
     | [fast-check.sh](.claude/hooks/fast-check.sh) | its per-file branches keyed on `apps/api`, `api/openapi.yaml`, `packages/` | edits to your code get no per-file check (tracker TD-66) |
     | [.custom-gcl.yml](apps/api/.custom-gcl.yml) | `destination: ../../.bin`, `path: ./tools/analyzers` | move it, `.golangci.yml` and `.go-arch-lint.yml` next to your `go.mod` (`API_DIR`) and fix both paths, or the custom golangci-lint build lands outside `.bin/` |
     | [.golangci.yml](apps/api/.golangci.yml) | depguard paths `**/internal/*/store/**`, `**/internal/*/api/**`; forbidigo exclusion `internal/platform/` | go-db-only-in-store and go-http-only-in-api judge the wrong files |
     | [.go-arch-lint.yml](apps/api/.go-arch-lint.yml) | its components, `internal/*/model/**` to `internal/*/api/**` | `ignoreNotFoundComponents: true`, so go-layers passes with zero matches |
     | [atlas.hcl](apps/api/atlas.hcl) | `dir = "file://db/migrations"`, relative to `API_DIR` | `atlas migrate` reads an empty or wrong directory; set it to `MIGRATIONS_DIR` as seen from `API_DIR` |
     | [sqlc.yaml](apps/api/sqlc.yaml) | one entry per domain, schema from `db/migrations`, output `internal/<domain>/store` | `make gen-sqlc` generates nothing or into the template layout |
     | `orval.config.ts` in `API_CLIENT_DIR` | `input: { target: '../../api/openapi.yaml' }`, relative to `API_CLIENT_DIR` | the client generates from a missing contract |
     | [eslint.config.mjs](eslint.config.mjs) | ignores `packages/api-client/src/gen/**` and `apps/api/**`; React rules on `apps/web/**` | your generated client and Go module get linted; your web app misses the hooks rules |
     | [.dependency-cruiser.cjs](.dependency-cruiser.cjs) | the web layers (`packages/domain`, `packages/api-client`, apps) | web-layers judges the wrong files; record your layering decision in an ADR |
     | [jscpd.json](scripts/harness/jscpd.json) | ignores `**/packages/api-client/src/gen/**` | your generated client counts as duplicated code |
     | [pnpm-workspace.yaml](pnpm-workspace.yaml), [knip.json](knip.json) | the workspaces `apps/web`, `packages/*`; knip's ignores | `pnpm install` and knip miss your workspaces; keep them in step with `WEB_GLOBS` |
     | The web app `apps/web` (tracker TD-46) | the `dev`, `e2e` and `e2e-deps` Makefile targets; Playwright in [install-tools.sh](scripts/install-tools.sh) (`browsers`) and the setup action's `Locked Playwright version` step; the hints in `coverage.sh` and `mutation.sh` | `make dev` and `make e2e` fail; with input `browser`, CI's `check-all` job fails in setup |
     | The iOS app's shape, with `ios` in `STACK` ([ADR 0044](docs/adr/0044-ios-ci-on-macos.md)) | the scheme `App` in the Makefile's `build-ios`; the app's sources in `App/` in the `ios-app-build-broken` fixture | `build-ios` fails on a missing scheme; the selftest fails until the app keeps that shape |
     | [action.yml](.github/actions/setup/action.yml) | the `Install JS dependencies` step (`make deps`, a no-op without `web` in `STACK`); node is set up in every job | with `web`, every CI job fails in setup without a root `package.json` and lockfile |
     | [install-tools.sh](scripts/install-tools.sh) | with `web` in `STACK`: installs pnpm and exits 1 unless the root `package.json` pins `packageManager` | `make setup` aborts after the other tools land; leave `web` out of `STACK` if you have no JS workspaces |
     | [setup-worktree.sh](scripts/setup-worktree.sh) | `make tools deps`, with `db` in `STACK` compose `db` and `atlas migrate apply` in `API_DIR` | `make setup` fails in `pnpm install` without a lockfile (with `web`) or without Docker (with `db`) |
     | `make new-domain`: [new-domain.sh](scripts/new-domain.sh) and `scripts/templates/new-domain/*` | `apps/api/internal/<name>`, `apps/api/db/queries`, `apps/api/db/migrations`, `apps/api/cmd/api/routes.go`, `apps/api/sqlc.yaml` | it writes the template layout, and CI's `make new-domain` probe job fails; rewrite it for your layout, or in step 6 drop its `Makefile` target, CI's `generator-probe` job and required check, that job's `ci.md` row and the skill (tracker TD-67) |
     | `scripts/test/*.bats`, for example [affected.bats](scripts/test/affected.bats) | template paths and the template's identity in the test cases; tests of scripts step 3 does not copy (`new-domain.bats`, `obs-query.bats`, `scaffold-tests.bats`) | `make check`'s bats fails; update each with the script it tests, or drop it as in step 6 ([drop.txt](scripts/templates/adoption-probe/drop.txt) lists them for a root module, tracker TD-99) |
     | [rules.md](docs/harness/rules.md) | test paths on `**Fixture:**` lines, `apps/api/tools/...` | [rules-fixture-claims](docs/harness/rules.md#rules-fixture-claims) fails; replace `apps/api/` by `<API_DIR>/` |
     | `<API_DIR>/internal/AGENTS.md` | links `../../../docs/`, relative to `apps/api/internal/` | [docs-links-resolve](docs/harness/rules.md#docs-links-resolve) fails |
     | [QUALITY.md](docs/QUALITY.md) | no rows: the template has no domain | `make check-all`'s [docs-quality-rows-match-domains](docs/harness/rules.md#docs-quality-rows-match-domains) fails; add one row per domain under `LAYER_GLOB`, linking its `AGENTS.md` |
6. **Keep, adapt or drop each rule in [rules.md](docs/harness/rules.md).**
   - A dropped rule takes out more than its id. Remove all of this, plus one ADR line saying why:
     - its whole `make` target from the `check` and `check-all` targets' lists in the [Makefile](Makefile);
     - its generator from the `gen` target and its inputs from `INPUTS` in
       [gen-check.sh](scripts/harness/gen-check.sh) (generated paths come from `GENERATED_GLOBS` in
       `layout.env`). `gen-check`, the last step of `make check`, runs `make gen`
       ([gen-up-to-date](docs/harness/rules.md#gen-up-to-date)); a deleted target that `gen` still names fails
       with `No rule to make target`;
     - its fixture directory under `scripts/harness/fixtures/`, and its tests in `scripts/test/` with their
       mentions on its `**Fixture:**` line;
     - its branch in `fast-check.sh` and its kind in `file-sensors.sh`;
     - its tool from `install-tools.sh` and `tool-versions.env`, or from the root `package.json`;
     - its gate workflow and its CODEOWNERS lines.
   - Rules drop in sets:

     | Set | Rules | `make` targets | Tools |
     |-----|-------|----------------|-------|
     | web | `web-*` (7) | `WEB_TARGETS`: `lint-web`, `arch-web`, `typecheck-web`, `test-web`, `knip`, `clones-web`, `e2e`, `gen-api-client` | eslint, dependency-cruiser, turbo, knip, jscpd, typescript from `package.json` |
     | db | `db-*` (5) | `DB_TARGETS`: `schema-doc-fresh`, `lint-migrations`, `squawk`, `migrations-append-only`, `test-integration`, `test-obs`, `e2e`, `gen-sqlc`, `gen-atlas-sum` | atlas, sqlc, squawk |
     | api | `api-*` (6) | `lint-api`, `contract-check`, `gen-oapi` | oasdiff, oapi-codegen, spectral |
     | ios | `ios-*` (5) | `IOS_TARGETS` (macOS only): `test-ios`, `lint-ios`, `fmt-check-ios` (`IOS_SWIFT_TARGETS`, in `make check`), `xcode-pinned`, `build-ios` (`IOS_XCODE_TARGETS`, in `make check-ios`, need Xcode) | Swift from Xcode or Apple's command-line tools (XCTest, `xcode-pinned` and `build-ios` need Xcode selected), SwiftLint, swift-format and XcodeGen from `install-tools.sh`, `.swiftlint.yml`, `.swift-format`, `scripts/swift-format/Package.resolved` |

     The ios set grows with [exec plan 0004](docs/exec-plans/active/0004-ios-stack-group.md); it also counts
     Swift tests for [specs-scenarios-tested](docs/harness/rules.md#specs-scenarios-tested), and without it
     any Swift under a `PRODUCT_GLOBS` or `WEB_GLOBS` entry fails `layout-check`. A product that runs `make harness-selftest` on
     Linux with `ios` in `STACK` sets `SELFTEST_SKIP_SWIFT=1` there, and on a Mac with only the command-line tools
     `SELFTEST_SKIP_XCODE=1`. With ios, the app keeps `IOS_DIR/project.yml` with its sources in `App/` and its scheme
     named `App`, the shape `build-ios` and its fixture expect. Without ios, `lint-ios` goes, and with it the Swift
     part of the size rules (`code-*`): narrow their entries to Go and TS. Nothing reads `.swiftlint.yml`, `.swift-format` or
     `scripts/swift-format/` then: leave them out of step 3; template sync names one only when the template
     changes it, and a `# declined <path>: <why>` line in `.harness-version` skips it.
     The web and db sets are `STACK` groups ([ADR 0033](docs/adr/0033-stack-profiles.md)), and their targets are
     the [Makefile](Makefile)'s `WEB_TARGETS` and `DB_TARGETS` as listed; `e2e` is in both: it needs both
     groups, and leaving out db or web drops it. A group drops whole targets: without db, `test-integration` and `test-obs` go entirely, so
     [go-tests-pass](docs/harness/rules.md#go-tests-pass) runs no integration test and
     [obs-request-queryable](docs/harness/rules.md#obs-request-queryable) never runs; without web,
     `clones-web` goes (the TS half of [repo-no-clones](docs/harness/rules.md#repo-no-clones)); without
     db or web, `e2e` goes ([web-e2e-journeys](docs/harness/rules.md#web-e2e-journeys)) too. Drop those rules with their
     group, or narrow the rules.md entries you keep to what still runs. Leaving a group out
     of `STACK` in `layout.env` already drops its targets from `check`, `check-all` and `gen`, its tools from
     `make tools` (pnpm and `make deps` with web, Postgres in `make setup` with db, SwiftLint and swift-format with
     ios on macOS, where `make setup` builds swift-format from source once, in about 3 minutes) and its fixtures from
     `make harness-selftest`, so the Makefile, `gen-check.sh`, `install-tools.sh` and the fixtures stay as
     they are. Still remove the dropped rules' rules.md entries, and the links to the directories you deleted:
     `make docs-links` and `make references-pinned` name them (in the template: the `apps/web/` and
     `apps/api/db/` area links in `AGENTS.md`, the `apps/web/AGENTS.md` link in
     `.claude/agents/test-writer.md`, the orval link in `AGENTS.md` and
     `docs/references/orval-llms.txt`, whose pin file is `packages/api-client/package.json`; the links to the
     skills step 3 left out, in `AGENTS.md` and `.specify/memory/constitution.md`). The api set is
     part of `go` and drops by hand as above. bats, squawk and spectral are pinned in `tool-versions.env`, so
     no set needs a root `package.json` but web.
   - A rule you keep must still reject its fixture. Port fixtures to your layout until these are green:
     ```sh
     make harness-selftest
     make fixture-claims
     make rule-lists
     ```
     `make harness-selftest` cannot be green until the dropped fixtures are gone. It asserts each fixture's
     `EXPECT` pattern, and a fixture for a missing `apps/web` fails for the wrong reason.
     When only `API_DIR` moves, the port is mechanical: `git mv` each fixture's `files/apps/api/*` and
     `base/apps/api/*` to `files/<API_DIR>/` and `base/<API_DIR>/`, and replace `apps/api/` by `<API_DIR>/`
     (nothing for `.`) in each `apply.sh` and `fixture.conf` and in the depguard path of
     `<API_DIR>/.golangci.yml`. Five fixtures edit the template's own product code and have no mechanical port:
     `api-approved-violates-contract` and `api-response-not-approved` (`platform/health`), `go-dead-function`
     (`platform/problem`), `go-test-helper-in-prod` (`platform/approvaltest`) and `specs-feat-without-spec` (a
     file under `apps/`, which must stay under `PRODUCT_GLOBS`). Rewrite them against your code, as
     [scripts/templates/adoption-probe/fixtures/](scripts/templates/adoption-probe/fixtures/) does for a root
     module, or drop them with their rules. Your approved-response tests then name
     `[api-responses-approved]` when they fail ([ADR 0022](docs/adr/0022-approved-api-responses.md)). Fixture
     directory names stay, so the `**Fixture:**` lines do not change.
   - Delete with `git rm`: the shell sensors list git-known files, so a deleted script still in the index fails
     `make check`'s shfmt and shellcheck.
   - The adoption probe is the template's own ([ADR 0038](docs/adr/0038-readme-brownfield-adoption-is-probed.md)):
     drop its own files, decline them for [template sync](#keeping-up-with-the-template), and hand-edit none of
     the synced harness files that wire it up.
     - `git rm` the probe `scripts/harness/adoption-probe.sh`, its fixture
       `scripts/harness/fixtures/harness-adoption-stale-layout/` and its tests `scripts/test/adoption-probe.bats`
       (step 3 never copied its brownfield repo, `scripts/templates/adoption-probe/`), and remove its entry in
       `docs/harness/rules.md`, whose `**Fixture:**` line names the files you removed.
     - Add a `# declined <path>: template-only` line to `.harness-version` for each file of the probe, its
       fixture, its tests and its brownfield repo, so template sync stops offering them back.
     - Leave the rest as it is: its job in `.github/workflows/ci.yml`, its target in the `Makefile`, its required
       check in `scripts/github-branch-protection.sh` and `docs/harness/ci.md`,
       its budget in `scripts/harness/budgets.txt`, and the brownfield repo's lines in
       `scripts/harness/protected-paths.txt` and `.github/CODEOWNERS`. These are synced harness files
       ([ADR 0026](docs/adr/0026-template-sync-is-a-drift-report.md)): a hand edit there is a merge by hand at
       every template bump, and an upstream change merged later would bring the entry back. Since step 4's
       `make rename WRITE=1` set `HARNESS_UPSTREAM`, the CI job passes without running, as it also does with the
       script removed and declined, so the required check stays green; entries
       for missing paths are harmless, and `make adoption-probe`, which only that job calls, fails for want of
       its script. So far only the adoption probe switches off this way: dropping `make new-domain` (step
       5's table: its target, CI's `generator-probe` job and required check) still edits them by hand (tracker
       TD-136).
7. **Run `make setup && make check`.** Expect `make setup` to need steps 5–6 first, because of the
   `install-tools.sh`, `make deps` and `setup-worktree.sh` assumptions in step 2. Without Postgres, leave
   `db` out of `STACK` (or run `SKIP_DB=1 make setup` once). Then run `make check` and expect red. Don't loosen a sensor to get green
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

Template sync reports drift and never applies anything ([ADR 0026](docs/adr/0026-template-sync-is-a-drift-report.md),
[rule](docs/harness/rules.md#harness-upstream-in-sync)):

- `.harness-version` holds the template commit this repo was copied from, or last synced to. Use the commit
  you actually copied: a newer one hides the changes in between. `# declined <path>: <why>` lines in it skip
  harness files you chose not to take.
- `HARNESS_UPSTREAM` in `scripts/tool-versions.env` is the template's git URL, written by `make rename`.
  Empty means this repository is the template: the check prints a notice and
  exits 0. `make harness-sync-check HARNESS_UPSTREAM=<url>` overrides it for one run.
- `make harness-sync-check` fetches the upstream's default branch anonymously into a temp repo and lists each
  protected path (except `.harness-version`) that changed there since the marker and differs here, marking
  files you changed too or no longer have. The script exits 0 when in sync, 1 when behind, and 2 when it
  cannot check (a malformed marker, a refused URL such as one with credentials in it, a missing or malformed
  `layout.env` here, an upstream `layout.env` without a single clean `API_DIR` line, a failed fetch, a marker
  not in upstream's history); make reports 1 and 2 alike as its exit 2. It reads the network, so it is not
  in `make check` or `check-all`; the weekly drift run includes it, where a private template exits 2 (run it
  locally with your credentials).

When your `API_DIR` differs from the template's (`apps/api`, read from the template's `layout.env`), the
template's files under its `API_DIR` are compared with their copies under yours, and your `protected-paths.txt`
entries written under your `API_DIR` also select the template's copies. The report names your path and the
template's, as in `tools/harness/go.mod (upstream apps/api/tools/harness/go.mod)`. Decline such a file by your path.

To catch up, port the listed files (or decline them) in a harness PR, then write the upstream commit the
report names to `.harness-version`.

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
