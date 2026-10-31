# ADR 0035: Harness files under .github/ are named, not globbed (amends 0010)

Status: accepted · 2026-10-29

## Context
`scripts/harness/protected-paths.txt` marked `.github/workflows/*` and `.github/actions/*` as `config` and
protected `.github/*` whole, and `.github/CODEOWNERS` routed `/.github/` to the harness owner (TD-24, paid by
this ADR). A product that ships its own workflows (deploy, release, preview) was then blocked by
`protect-harness` on every edit, and `harness-config-not-loosened` asked for the `harness-change-approved` label
on every change to them. The label stopped meaning "the harness changed", which is the only reason to look at
it. Exec plan 0003, slice P4.9.

## Decision
- **The harness's own workflows and composite action are named one by one**: in `protected-paths.txt` as
  `config` entries, in `.github/CODEOWNERS` as per-file lines, and in the file itself with
  `# Harness file (protected)` as its first line. `.github/CODEOWNERS` and `.github/pull_request_template.md`
  are listed in both files too. The three must agree: `scripts/test/codeowners.bats` checks that the list and the
  header name the same workflows and actions, that the list and CODEOWNERS name the same `.github/` files, and
  that each named file is tracked.
- **Everything else under `.github/` is the product's**: no hook blocks it and no label is needed. It is still
  reviewed: CODEOWNERS owns `/.github/workflows/` and `/.github/actions/` for the owner's eye (like
  `ARCHITECTURE.md`, `api/openapi.yaml` and the migrations), on lines before the harness's per-file lines so
  that the last match wins for those.
- **A new harness workflow is product until a harness PR lists it** in both files and gives it the header.

## Consequences
- **Required-check spoofing.** Branch protection matches a required check by job name and the GitHub Actions
  app (`scripts/github-branch-protection.sh`), and every workflow runs as that app. A product workflow with a
  job named `check-all` or `harness-integrity` could report green under that name without the harness label.
  The backstop today is code-owner review of `/.github/workflows/`, and the guides say a product job must not
  reuse a required check's name. TD-95 asks for a sensor; TD-96 asks `harness-integrity.yml` to check who
  applied the label.
- **A listed workflow can still reach unlisted code** through a local `uses: ./.github/actions/<x>` or a
  script outside the list. TD-97 asks for a sensor that a listed workflow's local `uses:` resolve to listed
  paths.
- **Amends [ADR 0010](0010-base-branch-gates.md)**: its consequence "a PR that weakens `ci.yml` changes a
  `config` path" now holds only for the listed workflows. A product workflow change passes `harness-integrity`
  without the label.
- The `Makefile` stays protected whole: a product target added to it still needs the label.
- `make harness-sync` compares a smaller `.github/` surface: the product's own workflows no longer show as
  "changed here".
