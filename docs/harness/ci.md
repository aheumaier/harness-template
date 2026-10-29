# CI and branch protection

CI runs the same `make` targets you run locally; a workflow adds no checks of its own. If CI is red, the
same target is red on your machine (with `BASE_REF=origin/main` for diff-based sensors). CI sets
`HARNESS_SCOPE=full`, so every sensor runs; a local `make check` skips those whose inputs did not change
([harness-scope](rules.md#harness-scope)).

## Workflows (`.github/workflows/`)

| Workflow | Trigger | Job (= status check) | Runs | Required |
|----------|---------|----------------------|------|----------|
| `ci.yml` | PR, push to main | `check-all` | `make setup && make check-all` minus `contract-check`, `harness-integrity`, `commits` and `specs` (own workflows, PR title, label waivers); needs Docker and pulls `OBS_IMAGE` (2.58 GB) for `make test-obs` on each fresh runner, no image cache ([ADR 0017](../adr/0017-per-worktree-observability-stack.md)); uploads `.cache/harness/timings.jsonl` as `harness-timings-check-all` (8 days) and shows latency-budget warnings as annotations | yes |
| | | `harness-selftest` | every fixture rejected by its sensor ([rules.md](rules.md)) | yes |
| | | `generator-probe` | `make new-domain NAME=zzz_probe && make check`, then the probe's generated service, handler and integration tests (`make test-integration`, NFR-B1); discards the probe and asserts a clean tree | yes |
| `coverage.yml` | PR, push to main | `coverage` | `make setup && make coverage` into the job summary ([coverage](#coverage)); `continue-on-error`, a report, not a sensor | no |
| `contract-gate.yml` | PR via `pull_request_target` (incl. label changes) | `api-no-breaking-change` | the **base branch's** `make contract-check` against the PR's spec when `api/openapi.yaml` changed (oasdiff, red only on breaking changes) | yes |
| `harness-integrity.yml` | PR via `pull_request_target` (incl. label and title changes) | `harness-integrity` | the **base branch's** `make harness-integrity` against the PR head (`HARNESS_ROOT`) | yes |
| | | `pr-conventions` | the **base branch's** `make commits specs` against the PR head and title (Conventional Commits, PR title ≤ 90 characters, spec-kit specs) | yes |
| `migration-gate.yml` | PR touching `apps/api/db/migrations/**` | `migrations` | append-only, Atlas lint, squawk | no (path-filtered; same sensors in `check-all`) |
| `drift.yml` | weekly, manual | `drift` | dead code, knip, govulncheck, Trivy (`vuln-deps`), schema doc, doc links, coverage that fell since the last run ([coverage](#coverage)), a mutation run that could not measure ([mutation testing](#mutation-testing); its scores go to the job summary) → one open `drift` issue | no |
| `harness-nfr.yml` | nightly, manual | `harness-nfr` | p95 per sensor over 7 days of `harness-timings-check-all` artifacts vs `budgets.txt` (`scripts/harness/nfr-report.sh`, alerts only) and `make harness-determinism` (`make check` twice; fails on any difference) → one open `harness-nfr` issue | no |

`.github/actions/setup` installs the pinned toolchain (`scripts/tool-versions.env`) and caches `.bin`,
Go modules and the pnpm store. Actions are pinned to commit SHAs; `make lint-ci` (actionlint) validates all
of it as part of `make check`.

## Coverage

`make coverage` (`scripts/harness/coverage.sh`) prints unit-test statement coverage per Go package of
`apps/api` (`go test -short`, generated files excluded, plus the module total `apps/api/...`) and per JS
workspace with a `coverage` script (Vitest; `coverage.sh` sets the v8 provider and the json-summary
reporter; a workspace with neither `test` nor `coverage` reads `no tests`, one with `test` only fails), and
keeps the numbers in `.cache/coverage/coverage.tsv`. It is a trend, never a gate
([ADR 0020](../adr/0020-test-coverage-is-a-trend.md)): a gate on a number invites tests written for the
number. It fails only when it cannot measure (missing tool, failing tests, unparseable input), never on a
percentage. It is a report, not a sensor: no rule id, in neither `make check` nor `make check-all`, and run
by no required check.

- `coverage.yml` writes the table to the `coverage` job summary of every PR and push to `main`.
- `drift.yml` writes it to its job summary with the change in percentage points since the newest earlier
  scheduled drift run on the same branch (a manual run never becomes the baseline), uploads
  `coverage.tsv` as the `coverage` artifact (35 days) for the next run, and adds to the `drift` issue only
  the packages whose coverage fell (`make coverage-fell`). Only its baseline step gets a token.
- A ratchet or any other threshold needs its own ADR, justified by a finding.

## Mutation testing

`make mutation` (`scripts/harness/mutation.sh`) mutates the source files changed in the last 7 days
(`MUTATION_SINCE`, any date `git rev-list --before` takes) and reports, per package, how many mutants the
tests killed: [gremlins](https://github.com/go-gremlins/gremlins) for `apps/api` (`*_test.go`, `testdata/`,
generated files and the nested `tools/` modules excluded) and [Stryker](https://stryker-mutator.io) for each
JS workspace's `src/` (tests, `*.d.ts` and `src/gen/` excluded) through its `mutation` script; a workspace
with neither `test` nor `mutation` reads `no tests`, one with `test` only fails with what to add. The score
is killed and timed-out mutants over all valid ones; a surviving mutant is a covered line whose change no
test notices. It is a weekly drift report, never a gate
([ADR 0023](../adr/0023-mutation-testing-is-a-drift-report.md), extending
[ADR 0020](../adr/0020-test-coverage-is-a-trend.md)): it fails only when it cannot measure (missing tool,
failing tests, shallow history, unparseable input, a loosened Stryker config), never on a score. A report,
not a sensor: no rule id, in neither `make check` (minutes, and its window reads the clock) nor
`make check-all`.

- Changed means the tree diff between `HEAD` and the newest first-parent commit at or before the window
  start, so a branch committed earlier and merged inside the window counts. Whole files, not changed lines:
  a test for a line usually guards its neighbours too.
- What is measured is protected config: gremlins reads only `scripts/harness/gremlins.yaml` (`--config`,
  every `GREMLINS_*` variable unset; two workers and a timeout coefficient of 10, because with one worker
  per core mutants time out from load, not because they hang), and each workspace's `stryker.config.json`
  (protected) may set neither `thresholds.break` nor `excludedMutations`.
- Same test set as [coverage](#coverage) and `make test-go`: gremlins runs `go test` with `GOFLAGS=-short`,
  so integration tests are skipped and code only they reach reads as `No coverage`.
- The summary keeps `No coverage` and `no tests` rows: they are where a test is missing, which a score
  alone hides.
- `drift.yml` checks out the full history (`fetch-depth: 0`; a shallow clone fails closed) and runs it in
  its own step capped at 20 minutes. The scores and survivors go to its job summary only; the `drift` issue
  gets a mutation entry only when it cannot measure: a failing `make mutation` adds its output, a step that
  hit the cap one line pointing at the step log.
- A threshold or ratchet needs its own ADR, justified by a finding.

## Label waivers

Three sensors can be waived per PR by a human, after reading the rationale in the PR description; the job
re-runs on the label event. Agents never add these labels.

| Label | Waives | Never waives |
|-------|--------|--------------|
| `harness-change-approved` | [harness-config-not-loosened](rules.md#harness-config-not-loosened) (any change to a harness config) | the suppression-reason check |
| `breaking-change-approved` | [api-no-breaking-change](rules.md#api-no-breaking-change) (the PR must describe the client migration) | anything else |
| `no-spec-needed` | [specs-required-for-features](rules.md#specs-required-for-features) (a `feat` without a spec in `specs/`) | `specs-structure`, `git-conventional-commits` |

GitHub lets anyone with triage or write access add a label, including the PR author; the label is a
signal, not a control. The control is the required code-owner review: every harness path and
`api/openapi.yaml` is in `.github/CODEOWNERS`, so a labelled PR still cannot merge without the owner.
Diff-based sensors fail closed (exit 2) when `BASE_REF` is missing or shallow; CI checks out full history.

## Base-branch gates

`contract-gate` and `harness-integrity` (incl. `pr-conventions`) use `pull_request_target`: the workflow, Makefile and scripts come
from the base branch, so a PR cannot rewrite the gate that judges it ([ADR 0010](../adr/0010-base-branch-gates.md)).
That trigger is only safe while the PR tree is **read, never executed**, only `head.sha` is used (as the
checkout ref; the PR title reaches `pr-conventions` only as the `PR_TITLE` env var), and the job has `contents: read` and no secrets. Keep those invariants in any edit.
`ci.yml` has to run PR code, so it runs the PR's own workflow; a PR that weakens it changes a `config`
path, and the base-branch `harness-integrity` gate stays red until a human adds the label.

## Branch protection

`scripts/github-branch-protection.sh` configures `main` via `gh api`: the six required checks above,
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
