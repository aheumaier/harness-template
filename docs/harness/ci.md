# CI and branch protection

CI runs the same `make` targets you run locally; a workflow adds no checks of its own. If CI is red, the
same target is red on your machine (with `BASE_REF=origin/main` for diff-based sensors).

## Workflows (`.github/workflows/`)

| Workflow | Trigger | Job (= status check) | Runs | Required |
|----------|---------|----------------------|------|----------|
| `ci.yml` | PR, push to main | `check-all` | `make setup && make check-all` minus `contract-check` and `harness-integrity` (own workflows, label waivers) | yes |
| | | `harness-selftest` | every fixture rejected by its sensor ([rules.md](rules.md)) | yes |
| | | `generator-probe` | `make new-domain NAME=zzz_probe && make check`, then discards the probe and asserts a clean tree | yes |
| `contract-gate.yml` | PR via `pull_request_target` (incl. label changes) | `api-no-breaking-change` | the **base branch's** `make contract-check` against the PR's spec when `api/openapi.yaml` changed (oasdiff, red only on breaking changes) | yes |
| `harness-integrity.yml` | PR via `pull_request_target` (incl. label changes) | `harness-integrity` | the **base branch's** `make harness-integrity` against the PR head (`HARNESS_ROOT`) | yes |
| `migration-gate.yml` | PR touching `apps/api/db/migrations/**` | `migrations` | append-only, Atlas lint, squawk | no (path-filtered; same sensors in `check-all`) |
| `drift.yml` | weekly, manual | `drift` | dead code, knip, govulncheck, schema doc, doc links → one open `drift` issue | no |

`.github/actions/setup` installs the pinned toolchain (`scripts/tool-versions.env`) and caches `.bin`,
Go modules and the pnpm store. Actions are pinned to commit SHAs; `make lint-ci` (actionlint) validates all
of it as part of `make check`.

## Label waivers

Two sensors can be waived per PR by a human, after reading the rationale in the PR description; the job
re-runs on the label event. Agents never add these labels.

| Label | Waives | Never waives |
|-------|--------|--------------|
| `harness-change-approved` | [harness-config-not-loosened](rules.md#harness-config-not-loosened) (any change to a harness config) | the suppression-reason check |
| `breaking-change-approved` | [api-no-breaking-change](rules.md#api-no-breaking-change) (the PR must describe the client migration) | anything else |

GitHub lets anyone with triage or write access add a label, including the PR author; the label is a
signal, not a control. The control is the required code-owner review: every harness path and
`api/openapi.yaml` is in `.github/CODEOWNERS`, so a labelled PR still cannot merge without the owner.
Diff-based sensors fail closed (exit 2) when `BASE_REF` is missing or shallow; CI checks out full history.

## Base-branch gates

`contract-gate` and `harness-integrity` use `pull_request_target`: the workflow, Makefile and scripts come
from the base branch, so a PR cannot rewrite the gate that judges it ([ADR 0010](../adr/0010-base-branch-gates.md)).
That trigger is only safe while the PR tree is **read, never executed**, only `head.sha` reaches a `run:`
block (through `env`), and the job has `contents: read` and no secrets. Keep those invariants in any edit.
`ci.yml` has to run PR code, so it runs the PR's own workflow; a PR that weakens it changes a `config`
path, and the base-branch `harness-integrity` gate stays red until a human adds the label.

## Branch protection

`scripts/github-branch-protection.sh` configures `main` via `gh api`: the five required checks above,
strict (branch up to date), one approving review including a code owner (`.github/CODEOWNERS`), stale
reviews dismissed, linear history, no force pushes or deletions. Admins are not included
(`enforce_admins: false`): with a single code owner, GitHub forbids approving your own PR. It is not run by any
workflow or by `make`. A repository admin runs it once, after the first push (checks must have reported
once before GitHub accepts them as required):

```sh
scripts/github-branch-protection.sh             # dry run: prints the request
CONFIRM=1 scripts/github-branch-protection.sh   # applies it (gh auth with admin rights)
```

When a workflow job is renamed, update `REQUIRED_CHECKS` in the script and re-run it.

## Agent GC (not configured)

`drift.yml` has an `agent-gc` job stub, enabled only when the repository variable `AGENT_GC_ENABLED` is
`true`. Its intended behaviour is documented in the workflow: take one drift finding, fix it on a branch,
open a small PR for human review.
