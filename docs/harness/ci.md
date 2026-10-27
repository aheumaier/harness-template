# CI and branch protection

CI runs the same `make` targets you run locally; a workflow adds no checks of its own. If CI is red, the
same target is red on your machine (with `BASE_REF=origin/main` for diff-based sensors).

## Workflows (`.github/workflows/`)

| Workflow | Trigger | Job (= status check) | Runs | Required |
|----------|---------|----------------------|------|----------|
| `ci.yml` | PR, push to main | `check-all` | `make setup && make check-all` | yes |
| | | `harness-selftest` | every fixture rejected by its sensor ([rules.md](rules.md)) | yes |
| | | `generator-probe` | `make new-domain NAME=zzz_probe && make check`, then discards the probe | yes |
| `contract-gate.yml` | every PR | `api-no-breaking-change` | `make contract-check` (oasdiff, red only on breaking changes) | yes |
| `harness-integrity.yml` | PR (incl. label changes) | `harness-integrity` | `make harness-integrity` | yes |
| `migration-gate.yml` | PR touching `apps/api/db/migrations/**` | `migrations` | append-only, Atlas lint, squawk | no (path-filtered; same sensors in `check-all`) |
| `drift.yml` | weekly, manual | `drift` | dead code, knip, govulncheck, schema doc, doc links → one open `drift` issue | no |

`.github/actions/setup` installs the pinned toolchain (`scripts/tool-versions.env`) and caches `.bin`,
Go modules and the pnpm store. Actions are pinned to commit SHAs; `make lint-ci` (actionlint) validates all
of it as part of `make check`.

## Waiving the harness config check

`harness-integrity` rejects loosened harness configs ([rules.md](rules.md#harness-config-not-loosened)).
When a rule is genuinely wrong, the PR says so (template: *Harness change: Y + rationale*) and a code owner
adds the label `harness-change-approved`; the job re-runs on the label event. The label never waives the
suppression-reason check. Agents never add the label.

## Branch protection

`scripts/github-branch-protection.sh` configures `main` via `gh api`: the five required checks above,
strict (branch up to date), one approving review including a code owner (`.github/CODEOWNERS`), stale
reviews dismissed, linear history, no force pushes or deletions, admins included. It is not run by any
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
