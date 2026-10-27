# ADR 0011: Conventional Commits and spec-kit specs for product features

Status: accepted · 2026-10-27

## Context
The team writes [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) and specifies
features with [spec-kit](https://github.com/github/spec-kit) (`/speckit-specify` → `/speckit-plan` →
`/speckit-tasks` → `/speckit-implement`). Neither was enforced: an agent could commit `update stuff` or ship
a feature whose requirements live only in a chat log. The repo already plans multi-step work in
`docs/exec-plans/`, so two planning homes needed a split.

## Decision
- **Split by kind.** New product capabilities (`feat` in `apps/`, `api/`) are specified in `specs/NNN-slug/`
  with spec-kit; all other multi-step work, product refactors included, is planned in `docs/exec-plans/`.
  A spec is a living document: a later `feat` that changes a capability updates its spec in the same PR.
- **The contract stays in one place.** A spec's `contracts/` lists the operations and schemas in prose;
  the contract itself is only `api/openapi.yaml` (ADR 0005). `specs-structure` rejects an OpenAPI document
  under `specs/`.
- **Sensors** (docs/harness/rules.md): `git-conventional-commits` (`make commits`, plus a `commit-msg`
  hook installed by `make setup`), `specs-structure` and `specs-required-for-features` (`make specs`). In CI
  they run in the `pr-conventions` job of `harness-integrity.yml`, which, like the other base-branch gates
  ([ADR 0010](0010-base-branch-gates.md)), runs the base branch's scripts and passes the PR title only as an
  env var. The `no-spec-needed` label waives `specs-required-for-features` only.
- **spec-kit is vendored at 1.1.3** (`.specify/`, `.claude/skills/speckit-*`). Its scripts are third-party:
  excluded from shellcheck/shfmt and the suppression scan, protected like other harness files, never edited
  by hand. `.specify/memory/constitution.md` is ours and points at AGENTS.md and rules.md instead of
  restating them. Upgrade (a human, then a harness PR):
  `uvx --from specify-cli==<version> specify init --here --integration claude --script sh --force --non-interactive`,
  then restore our constitution and `.claude/settings.json` from git and review the diff. The version lives
  here and in `.specify/init-options.json`, not in `scripts/tool-versions.env`: spec-kit is vendored source,
  not a tool installed by `make setup` (an exception to [ADR 0001](0001-tool-pinning-and-single-entry-point.md)).
- **Not used:** `/speckit-taskstoissues` (issues are not the system of record, principle 7) and
  `specify extension add` / presets (they change what every skill does; their paths are protected).
- **Amends ADR 0010, invariant 2:** besides `head.sha`, the PR title and label booleans may reach a step,
  only as env vars, and only as data (never in `run:` or a make recipe).

## Alternatives considered
- **A plain Markdown spec template in `docs/`**: no tooling for clarify → plan → tasks, and agents already
  know spec-kit's commands.
- **Exec plans only**: one planning home, but exec plans carry no requirements to check code against.

spec-kit passes core belief 4 (boring, in-repo-reasonable dependencies): it is Markdown templates and shell
scripts, vendored and readable in the repo, with no runtime in the product. The three rules are team
conventions adopted up front, not earned from an incident; rules.md marks them `team-convention`.

## Consequences
- GitHub's squash commit for several commits is the PR title plus ` (#N)`, so the title is checked too and
  capped at 90 characters; prefer squash or rebase merges so no unchecked merge commit lands on `main`.
- A later PR that implements a merged spec must touch its `specs/` directory (tasks, status) or carry the
  `no-spec-needed` label.
- The gate is per commit: a `feat` that touches `apps/` or `api/` needs a spec in the same PR; `fix`,
  `refactor` etc. do not. Mislabelling a feature as `fix` evades it: reviewers catch that, not a sensor.
- Changes to the `pr-conventions` job take effect only after they merge (ADR 0010).
