# CI and branch protection

CI runs the same `make` targets you run locally; a workflow adds no checks of its own. If CI is red, the
same target is red on your machine (with `BASE_REF=origin/main` for diff-based sensors). CI sets
`HARNESS_SCOPE=full`, so every sensor runs; a local `make check` skips those whose inputs did not change
([harness-scope](rules.md#harness-scope)). Inferential sensors are the exception
([ADR 0019](../adr/0019-inferential-sensors-are-advisory.md)): `pr-review.yml` and `drift.yml`'s scheduled
`reviews` and `agent-gc` jobs run agents that no `make` target runs, and they never block.

## Workflows (`.github/workflows/`)

These workflows and `.github/actions/setup` are harness files, named one by one in
[protected-paths.txt](../../scripts/harness/protected-paths.txt) and `.github/CODEOWNERS`. A product's own
workflows and actions beside them are not: agents may edit them, and a change to them needs no
`harness-change-approved` label ([harness-config-not-loosened](rules.md#harness-config-not-loosened),
[ADR 0035](../adr/0035-harness-files-under-github-are-named.md)). They still need a code owner's review
(CODEOWNERS owns `/.github/workflows/` and `/.github/actions/`), and a product job must not reuse a required
check's name: branch protection matches checks by name, so such a job could stand in for the harness's.

| Workflow | Trigger | Job (= status check) | Runs | Required |
|----------|---------|----------------------|------|----------|
| `ci.yml` | PR, push to main | `check-all` | `make setup && make check-all` minus `contract-check`, `harness-integrity`, `commits` and `specs` (own workflows, PR title, label waivers); needs Docker and pulls `OBS_IMAGE` (2.58 GB) for `make test-obs` on each fresh runner, no image cache ([ADR 0017](../adr/0017-per-worktree-observability-stack.md)); the only job that installs a browser: `make e2e-deps` installs the system libraries of Playwright's Chromium headless shell (unpinned apt, [ADR 0025](../adr/0025-browser-e2e-is-the-nfr-b2-sensor.md) known gap) and `make e2e` the browser itself, which `.github/actions/setup` (input `browser: "true"`) caches as `.bin/playwright` apart from the rest of `.bin`, keyed on the locked Playwright version; uploads `.cache/harness/timings.jsonl` as `harness-timings-check-all` (8 days) and shows latency-budget warnings as annotations; on failure uploads `.tmp/e2e` and `.tmp/dev` as `e2e-failure` (3 days) | yes |
| | | `harness-selftest` | every fixture rejected by its sensor ([rules.md](rules.md)) | yes |
| | | `generator-probe` | `make new-domain NAME=zzz_probe && make check`, then `make docs-quality-shape` (the generated `QUALITY.md` row is well-formed and matches its domain; deterministic, [ADR 0028](../adr/0028-quality-grades-per-domain-and-layer.md)), then `make test-scaffold NAME=zzz_probe`: the probe's generated service, handler and integration tests through `go test -json`, red unless each ran and passed, none skipped (NFR-B1); restores only the paths the generator touched and asserts a clean tree | yes |
| `coverage.yml` | PR, push to main | `coverage` | `make setup && make coverage` into the job summary ([coverage](#coverage)); a report, not a sensor: a failure to measure turns it red, never blocking | no |
| `pr-review.yml` | PR (`pull_request`, never `pull_request_target`) | `review`, `comment` | the `code-reviewer` prompt, headless, on the PR's diff → one top-level PR comment, replaced on each push ([PR review](#pr-review)); an inferential sensor, advisory ([ADR 0019](../adr/0019-inferential-sensors-are-advisory.md)) | no |
| `contract-gate.yml` | PR via `pull_request_target` (incl. label changes) | `api-no-breaking-change` | the **base branch's** `make contract-check` against the PR's spec when `api/openapi.yaml` changed (oasdiff, red only on breaking changes) | yes |
| `harness-integrity.yml` | PR via `pull_request_target` (incl. label and title changes) | `harness-integrity` | the **base branch's** `make harness-integrity` against the PR head (`HARNESS_ROOT`) | yes |
| | | `pr-conventions` | the **base branch's** `make commits specs` against the PR head and title (Conventional Commits, PR title ≤ 90 characters, spec-kit specs) | yes |
| `migration-gate.yml` | every PR; its gates run only when `MIGRATIONS_DIR` (`scripts/harness/layout.env`) changed | `migrations` | append-only, Atlas lint, squawk | no (same sensors in `check-all`) |
| `drift.yml` | weekly, manual | `drift` | dead code, knip, govulncheck, Trivy (`vuln-deps`), schema doc, doc links, harness files behind the template (`make harness-sync-check`; anonymous fetch, so a private template exits 2 here and is checked locally), `QUALITY.md` rows not graded in the last 30 days (`make docs-quality`, the only clock-dependent half; the deterministic `make docs-quality-shape` is in `check-all`, [ADR 0028](../adr/0028-quality-grades-per-domain-and-layer.md)), coverage that fell since the last run ([coverage](#coverage)), the weekly mutation score table, or a mutation run that could not measure ([mutation testing](#mutation-testing); survivors go to the job summary only), rules that never fired in 90 days ([rule fires](#rule-fires); a human decides to keep or retire each), the findings of the scheduled reviews → one open `drift` issue | no |
| | | `review-gate`, `reviews (<review>)` | the [scheduled reviews](#scheduled-reviews): security, data handling, modularity, one read-only agent each, on the default branch only; skipped with a notice without the `ANTHROPIC_API_KEY` secret | no |
| | | `gc-gate`, `agent-gc`, `agent-gc-pr` | [agent GC](#agent-gc): one agent fixes broken doc links in unprotected docs → one small PR labelled `agent-gc` for human review; off until the `AGENT_GC_ENABLED` variable is `true` | no |
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
  its own step capped at 20 minutes. The scores and survivors go to its job summary; the `drift` issue gets
  a mutation entry every week: the score table per package (`make mutation-scores`, `mutation.sh report
  --scores`, no survivors) when the run succeeded, the output of a failing `make mutation`, or one line
  pointing at the step log for a step that hit the cap.
- A threshold or ratchet needs its own ADR, justified by a finding.

## Rule fires

A rule that has not found anything for months may guard a rare mistake, guard one that agents stopped making
because the rule exists, or no longer earn its place. The weekly drift run lists those rules; a human decides
which case it is ([ADR 0032](../adr/0032-rules-that-never-fire-are-a-drift-report.md)). Nothing retires a rule
automatically.

- `scripts/harness/run.sh` adds each rule's finding count to its row in `.cache/harness/timings.jsonl`
  (`"findings":{"<rule-id>":<n>,...}`): 0 on success; on failure, for each rule whose remediation prints, the
  output lines naming it, at least 1; 0 for the others. Only "more than 0" is used.
- `drift.yml`'s baseline step (the only one with a token) fetches the `rule-fires` ledger of the newest
  earlier scheduled drift run and ci.yml's `harness-timings-check-all` artifacts of their retention window
  (`scripts/harness/ci-timings.sh`, which reads the retention from ci.yml and also serves `harness-nfr.yml`).
  The Sweep runs `scripts/harness/rule-fires.sh`, which merges each artifact once (the ledger keeps the ids
  it merged until they expire, so one uploaded after a merge is merged the next week) and uploads the ledger
  as the `rule-fires` artifact (90 days) for the next run.
- The `drift` issue lists every rule of rules.md that was tracked for the whole 90 days, ran in them and
  never fired in them, with when it last fired and its runs without a finding since. Rules with no run in the
  window are named as not measured, never as candidates: hook-only rules, the sensors with their own
  workflow ([TD-79, TD-80](../exec-plans/tech-debt-tracker.md)) and conventions without a sensor.
- Nothing is listed in the first 90 days of tracking, nor for 90 days after the ledger is lost (no scheduled
  run for longer than its retention); the issue says when tracking starts or when a gap longer than the
  timings retention lost rows. A rule added to rules.md is tracked from that day; one removed drops out of the
  ledger.
- A bad week never resets the history: tracking starts fresh only when the run list worked and no earlier
  scheduled run has a `rule-fires` artifact. A gh error listing or downloading the ledger or the timings, or
  a failing `rule-fires.sh`, merges nothing and uploads no ledger, so the last one stays the newest; the
  issue says so.
- Rows come from PR runs, so `rule-fires.sh` trusts none: only `timing.sh`'s exact shape, known rule ids
  and integer counts, no row starting after the merge, no symlink. `ci-timings.sh` drops fork PR runs'
  artifacts and extracts only `timings.jsonl`; a well-formed forged row from a same-repository PR can still hide a rule ([ADR 0032](../adr/0032-rules-that-never-fire-are-a-drift-report.md)).
  The step log says how many rows were merged from how many artifacts; the issue says when new artifacts
  gave none.
- A report, not a sensor: no rule id, in neither `make check` nor `make check-all` (it reads the clock and
  the network). Retiring a rule is a harness PR removing its rules.md entry, sensor and fixture.

## PR review

`pr-review.yml` runs the [`code-reviewer`](../../.claude/agents/code-reviewer.md) prompt on every PR with
Claude Code (pinned with sha256 in `scripts/tool-versions.env`, installed by `scripts/install-tools.sh
claude`) and posts its verdict and findings as one top-level PR comment. It is an inferential sensor
([ADR 0019](../adr/0019-inferential-sensors-are-advisory.md)): a finding is a suggestion, never a red job,
and neither job is a required check. A top-level comment opens no review thread, so
`required_conversation_resolution` never waits on it.

- **Trust boundary: write-access collaborators are trusted with the key.** On `pull_request` GitHub runs the
  PR's own copy of the workflow, with the secret, so anyone who can push a branch can edit it to print or
  spend the key in that run; a red `harness-integrity` does not stop the run. The key must therefore be a
  dedicated Anthropic workspace key with a hard spend limit, used for nothing else. Optional hardening: an
  `environment:` with required reviewers on the `review` job, so a human releases the key for each run.
- **Untrusted diff.** The workspace is the PR's base commit (prompt, scripts, protected-path list); the head
  is read with `git diff` only, and the diff reaches the agent between nonce-tagged delimiters as data. The
  gate step builds the prompt and the agent's settings without the key; the agent step runs only `claude` and
  the leak screen, with a `contents: read` token. The agent runs with `--bare --restricted --tools
  Read,Grep,Glob --disallowedTools 'mcp__*' --strict-mcp-config --permission-mode dontAsk`, every flag in the
  [CLI reference](https://code.claude.com/docs/en/cli-reference): no shell, web or MCP tool, and file tools
  confined to the working directories. Its `--settings` file (`pr-review.sh settings`) also blocks reads outside
  the working directory and denies Read under `/proc`, `/etc`, the runner's temp, tool cache and config dirs.
  A separate `comment` job with `pull-requests: write` and no secret posts the text, fenced so no mention,
  link or image in it renders. The workflow header lists every invariant and `scripts/test/pr-review.bats`
  checks them; keep them in any edit.
- **Leak screen** (`pr-review.sh screen`): output holding the key verbatim, or `sk-ant-` in any case, is
  withheld with a warning. It is a backstop for a verbatim leak, not the main control: an encoded key passes
  it. The tool confinement above is the control.
- **Skips with a notice** when the `ANTHROPIC_API_KEY` repository secret is missing, on a fork (no secrets),
  a draft, a base commit without `pr-review.sh`, an empty or harness-only diff (every path, both sides of a
  rename, in `scripts/harness/protected-paths.txt`, where the code-owner review applies), a diff over the size
  cap, or when the agent stops (API error, turn or budget cap; the notice gives the exit code, the error and
  the last stderr lines). Only a successful JSON result is posted, so a cap message never is.
- **Red only on harness breakage**: the agent did not start (no JSON result: a bad flag, a broken install),
  or a commit is missing (fetch depth, or a head lost to a force-push race). Never on a finding.
- **Cost cap per run**: the `REVIEW_*` values at the top of the workflow (diff size, turns, US dollars,
  model, minutes: `REVIEW_MAX_MINUTES` stops the agent with a notice before the job's own timeout); a new push cancels the running review. The workflow is protected, so raising a cap needs the
  `harness-change-approved` label.
- **Dry run** of what the agent gets: `scripts/harness/pr-review.sh prompt <(git diff main...)`.
- **Turning it on** is a human step, after ADR 0019 is accepted: run `.bin/claude --help` and a
  confinement test with a dummy key, then add the `ANTHROPIC_API_KEY` repository secret
  ([TD-38](../exec-plans/tech-debt-tracker.md)). Until then every run skips.
- A finding class accepted twice becomes a computational rule ([ADR 0019](../adr/0019-inferential-sensors-are-advisory.md)).

## Scheduled reviews

Three weekly reviews of the whole repository, the part no linter expresses: security
([review-security](../../.claude/skills/review-security/SKILL.md)), data handling
([review-data-handling](../../.claude/skills/review-data-handling/SKILL.md)) and modularity
([review-modularity](../../.claude/skills/review-modularity/SKILL.md)). Each is a skill, usable
interactively too, that skips what a sensor already reports. Inferential sensors, so advisory
([ADR 0019](../adr/0019-inferential-sensors-are-advisory.md)): no rule id, no fixture, no budget row, never
a required check, in neither `make check` nor `make check-all`; a human decides what to act on, and a
finding class accepted twice becomes a computational rule.

- `drift.yml` runs them in the `reviews` job, one matrix entry per review, before the `drift` job, on the
  default branch only (a schedule, or a manual run of the default branch; a run of any other branch skips
  them with a notice) and one sweep at a time (`concurrency: drift`). Each agent gets the prompt of
  `make review-prompt REVIEW=<review>` (`scripts/harness/scheduled-review.sh`: a fixed CI preamble, then
  the skill without its front matter; run it locally to see exactly what is sent) and answers in a JSON
  schema (`--json-schema`: `file`, `line`, `severity`, `problem`, `fix`; at most 15).
- **Same CI agent contract as [PR review](#pr-review)** (`scripts/harness/ci-agent.sh`, shared with
  `pr-review.sh`): Claude Code installed by `scripts/install-tools.sh claude`, run as `.bin/claude -p --bare
  --restricted --strict-mcp-config --tools Read,Grep,Glob --disallowedTools 'mcp__*' --permission-mode
  dontAsk --no-session-persistence --output-format json` with `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1`
  and a `--settings` file that blocks reads outside the working directory and denies Read under system and
  runner dirs and of every env file. The skills are read-only too. The agent step holds the API key and no
  write token (`contents: read`, credentials not persisted), and runs only `claude`, the schema and the
  leak screen (`scheduled-review.sh screen`, a verbatim backstop as in PR review).
- **Trust boundary: write-access collaborators are trusted with the key.** A manual run executes the
  dispatched branch's copy of the workflow with the secret, so anyone who can push a branch can edit it to
  print or spend the key; the default-branch gate keeps reviews on reviewed code but an edited copy can drop
  it. The key must be a dedicated Anthropic workspace key with a hard spend limit, used for nothing else.
  Optional hardening: an `environment:` on the `reviews` job limited to the default branch, holding the key
  as an environment secret.
- **Cost cap per review**: the `REVIEW_*` values at the top of the workflow: `REVIEW_MODEL`
  (`claude-sonnet-5-5`), `REVIEW_MAX_TURNS` (40), `REVIEW_MAX_BUDGET_USD` (3.00) and `REVIEW_MAX_MINUTES`
  (15). The workflow is protected, so raising a cap needs the `harness-change-approved` label.
- **Skips with a notice** when the secret is missing, off the default branch, or when the agent stops on a
  cap or an API error. **Red only on harness breakage**: the agent did not start, or it succeeded without
  structured findings (empty or missing `structured_output`). The `drift` issue names a failed review job
  and findings that cannot be rendered. Never on a finding.
- **Untrusted output.** The `reviews` job uploads the screened findings JSON; the `drift` job renders it
  (`scheduled-review.sh render`, with the pinned `.bin/jq`): findings with a file and line, high severity
  first, control characters flattened, `sk-ant-` tokens redacted, each field cut at the schema's length and
  each section at 16000 bytes, all inside one code fence the text cannot close, so no mention, link, image
  or HTML in it renders. Reviews come last in the report, security first, so the issue's 60000-byte cut
  (on a line boundary) drops modularity first. The log shows the report between `::stop-commands::`
  markers, and only the issue step writes to GitHub.
- **Turning it on** is a human step, with [PR review](#pr-review)'s: set the `ANTHROPIC_API_KEY` repository
  secret, then start one manual `drift` run on the default branch and check each review's notice or
  findings ([TD-38](../exec-plans/tech-debt-tracker.md)). Until then the `review-gate` job leaves a notice,
  `reviews` is skipped and the `drift` job runs as before.

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

## Agent GC

`agent-gc`, the doc gardener (exec plan 0003 P3.4): each week one agent fixes the broken links and anchors
the computational doc check found and proposes the edit as a small PR for human review. An inferential
sensor, so advisory ([ADR 0019](../adr/0019-inferential-sensors-are-advisory.md)): no rule id, no fixture,
no budget row, never a required check; its PR is a proposal like any other, merged or closed by a human.

- **What it fixes.** The `docs-links-resolve` findings of `.bin/harness check-links` in tracked `*.md` files
  outside `scripts/harness/protected-paths.txt` that are no `AGENTS.md` or `CLAUDE.md` (guides every agent
  reads, edited by a human) and no history or generated Markdown (`docs/adr/`, `docs/exec-plans/`,
  `docs/generated/`, `specs/`, `.specify/`, any `fixtures/`: `NOT_GUIDE_RE` in `agent-gc.sh`, the twin of
  `notGuideRE` in `orphans.go`), from at most `REVIEW_GC_MAX_FILES` files (`scripts/harness/agent-gc.sh findings`
  and `targets`). The other `docs-links-resolve` findings (a protected or untracked file, an agent guide,
  history or generated Markdown, past the cap) are not agent-gc's: the job leaves a notice with their count, and the drift issue's
  `make docs-links` section lists every finding. Other rules are not read: `rules-fixture-claims` findings
  are all in the protected `rules.md`; the `make docs-quality` findings are in the unprotected `QUALITY.md`,
  but until a grading agent exists a stale row needs a reviewer's new grade, not a gardener's new date, and
  the deterministic `QUALITY.md` rules gate `check-all`, so a row for a deleted domain never reaches main
  ([ADR 0028](../adr/0028-quality-grades-per-domain-and-layer.md)).
- **Same CI agent contract as the [scheduled reviews](#scheduled-reviews)**, with Edit added:
  `--tools Read,Grep,Glob,Edit`, and a `--settings` file (`agent-gc.sh settings`) that allows Edit on each
  target file alone and still denies Write, Bash and the web. The agent gets the findings as data between
  nonce lines (`agent-gc.sh prompt`; in each file it fixes every finding or leaves the file untouched) and
  answers in a JSON schema (`{"edited": [...]}`); the diff, not that answer, is what it proposes.
- **A job that trusts nothing the agent touched.** The `agent-gc` job holds `contents: read` and no write
  token, saves no cache (the setup action's `save-caches: "false"`: `.bin` restored only, no Go, pnpm or
  Playwright cache) and keeps every file it writes (findings, targets, prompt, settings, the agent's output,
  the patch) under `$RUNNER_TEMP`. Before the agent runs the tree must be clean, untracked files included
  (`agent-gc.sh baseline` records the ignored ones), and the checkout is made read-only, `.git` included,
  except the target files (`agent-gc.sh lock`). The agent step holds the API key and runs only `claude`, the
  schema, `git diff-files` and the leak screen (on the agent's output and on the lines its edit adds).
- **Confined, whatever the agent's outcome** (success, cap, timeout or withheld output): `agent-gc.sh
  confined` turns the job red when the tree holds anything but edits of target files: a new, deleted or
  renamed file, a mode change, an edit of any other file, a new or deleted ignored file, or any file
  (ignored or under `.git`) written after the baseline.
- **Checked before anything is proposed** (`agent-gc.sh patch`): a notice and no PR when the agent changed
  nothing, changed more than `REVIEW_GC_MAX_CHANGED_LINES` lines, or the doc check, run again, reports a new
  finding or one left in an edited file.
- **The PR.** The `agent-gc-pr` job runs no agent. It checks out the same commit, checks the patch again and
  writes the PR body, both without the token (`agent-gc.sh apply`: target files only, none protected, an
  agent guide or history or generated Markdown, within the cap, no file created, deleted or renamed and no mode change; `agent-gc.sh body`:
  fixed text plus the findings of the files the patch changes, in a code fence). It then commits as the
  token's identity (`docs: fix broken doc links (agent-gc)`), pushes only the branch `agent-gc/<run id>` and
  opens a PR labelled `agent-gc` against the branch it ran on. It never pushes to the default branch. The PR
  goes through the same required checks and code-owner review as any other; a PR opened with `GITHUB_TOKEN`
  would start no workflow, hence `AGENT_GC_TOKEN`. One open `agent-gc` PR at a time: the `gc-gate` job skips
  while one is open.
- **The write token.** The agent-gc write token is an environment secret limited to the default branch, held
  by a non-admin identity outside every branch-protection bypass and without Workflows or Administration
  permission; write collaborators are trusted with the key, not with this token. So `AGENT_GC_TOKEN` is a
  secret of the `agent-gc` environment, whose deployment-branch policy is the default branch only, and only
  `agent-gc-pr` names that environment. It belongs to a machine user that is not an admin, not the code
  owner (a PR its owner authored cannot be approved by them, so a code-owner token would leave its PRs
  unreviewable) and in no bypass list of the branch protection or rulesets. It is a fine-grained token for
  this repository only with Contents and Pull requests read and write, and no Workflows, Administration or
  Issues permission. A GitHub App token minted per run is the planned replacement
  ([TD-50](../exec-plans/tech-debt-tracker.md)).
- **Cost cap per run**: the scheduled reviews' `REVIEW_MODEL`, `REVIEW_MAX_TURNS`, `REVIEW_MAX_BUDGET_USD`
  and `REVIEW_MAX_MINUTES`, plus `REVIEW_GC_MAX_FILES` (5) and `REVIEW_GC_MAX_CHANGED_LINES` (40), at the top
  of the protected workflow.
- **Skips with a notice** when it is off, the API key is missing, off the default branch (`review-gate`), an
  `agent-gc` PR is open, there is nothing to fix, or the agent stops on a cap or an API error. **Red only on
  harness breakage**: the doc check broke, the tree was not clean, the agent did not start or succeeded
  without structured output, the confinement failed, the patch fails its second check, or the `agent-gc`
  environment has no `AGENT_GC_TOKEN`.
- **Turning it on** is a human step, after [ADR 0019](../adr/0019-inferential-sensors-are-advisory.md) is
  accepted and the scheduled reviews run ([TD-38](../exec-plans/tech-debt-tracker.md)): create the `agent-gc`
  environment (deployment branches: the default branch only) and its `AGENT_GC_TOKEN` secret as above; create
  the `agent-gc` label once by hand (`gh label create agent-gc`; the token cannot); turn on "Automatically
  delete head branches", so merged `agent-gc/<run id>` branches go (whoever closes an agent-gc PR unmerged
  deletes its branch); then set the
  repository variable `AGENT_GC_ENABLED` to `true` and start one manual `drift` run on the default branch
  ([TD-50](../exec-plans/tech-debt-tracker.md)). Until then `gc-gate` and the jobs after it are skipped.
