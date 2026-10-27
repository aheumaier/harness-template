# ADR 0011: Conventional Commits and spec-kit specs for product features

Status: accepted · 2026-10-27

## Context
The team writes [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) and specifies
features with [spec-kit](https://github.com/github/spec-kit) (`/speckit-specify` → `/speckit-plan` →
`/speckit-tasks` → `/speckit-implement`). Neither was enforced: an agent could commit `update stuff` or ship
a feature whose requirements live only in a chat log. The repo already plans multi-step work in
`docs/exec-plans/`, so two planning homes needed a split.

## Decision
- **Split by kind.** Product features (`apps/`, `api/`) are specified in `specs/NNN-slug/` with spec-kit;
  harness and infrastructure work is planned in `docs/exec-plans/`.
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
  then restore our constitution and `.claude/settings.json` from git and review the diff.

## Consequences
- Squash merges make the PR title the commit on `main`, so the title is checked too.
- The gate is per commit: a `feat` that touches `apps/` or `api/` needs a spec in the same PR; `fix`,
  `refactor` etc. do not. Mislabelling a feature as `fix` evades it: reviewers catch that, not a sensor.
- Changes to the `pr-conventions` job take effect only after they merge (ADR 0010).
