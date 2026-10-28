# 0002 loop prompt — one slice per iteration

The prompt below implements [exec plan 0002](0002-fast-reliable-harness.md) one slice at a time with Claude
Code's `/loop`. All state lives in the plan's **Progress** checklist and in git, so a session can crash or be
stopped and a new one resumes where the last commit left off.

## Running it

A human starts the loop. It edits harness files, so the session needs `HARNESS_EDIT=1` and must run in its
own worktree (ADR 0008):

```bash
git worktree add -b perf/harness-nfr ../skeleton-harness-nfr docs/harness-nfr-plan
cd ../skeleton-harness-nfr && make setup
HARNESS_EDIT=1 claude
```

Then, in that session:

```text
/loop Read the "## Prompt" section of docs/exec-plans/active/0002-loop-prompt.md and do one iteration exactly as it says.
```

Without an interval, `/loop` paces itself: after each iteration it schedules the next. Give the session the
permissions you trust in an isolated worktree: Edit/Write plus Bash for `make`, `git`, `go`, `bats`, `node`,
`pnpm` and the read-only tools.

The loop ends when an iteration ends in `LOOP: BLOCKED` or `LOOP: DONE`, or when an iteration makes no commit.
Stop it any time with Esc; rerun the `/loop` line to resume. It never pushes. A human reviews the branch,
opens the PR (Harness change: Y) and adds the `harness-change-approved` label.

## Prompt

You are one iteration of a `/loop` that implements exec plan 0002 in this repository. Treat the plan's
**Progress** checklist and git history as the only state; do not rely on what you remember from earlier
iterations in this session. Do exactly one slice, leave the tree green and committed, then stop.

**1. Preconditions.** Run `git status --short`, `git branch --show-current` and `echo "$HARNESS_EDIT"`. The
branch must not be `main`, the tree must be clean, and `HARNESS_EDIT` must be `1`. If any of these fails, do
not change anything: print the reason and end with `LOOP: BLOCKED`.

**2. Read.** Read `AGENTS.md`, `docs/exec-plans/active/0002-fast-reliable-harness.md` (NFRs, phases,
Progress), `docs/adr/0012-parsing-sensors-in-go.md`, `docs/adr/0013-harness-nfr-budgets.md`,
`docs/adr/0009-harness-selftest.md`, `docs/adr/0010-base-branch-gates.md`, and the sections of
`docs/harness/rules.md` and the scripts the slice touches. Run `git log --oneline -15` to see the earlier
slices.

**3. Pick.** Take the first `- [ ]` item in Progress. If there is none, go to step 8. If an item is `- [!]`,
end with `LOOP: BLOCKED` and do nothing else.

**4. Measure before.** Record the numbers for the NFR this slice targets, for example `make check` wall time,
the sensor's time, or hook time per call. Run each measurement twice and keep the second (warm) result.
Measure hooks with `HARNESS_EDIT` unset (`env -u HARNESS_EDIT`): it short-circuits `protect-harness`.

**5. Implement only this slice.** Follow these constraints:
- **Keep the output contract.** Rule ids, `file:line: message [rule-id]` lines, exit codes, and `run.sh`
  printing the remediation from `docs/harness/rules.md` stay as they are. Timing and budget warnings never
  change a sensor's exit status, and a failure to time or write timings never fails a sensor.
- **Never loosen.** Do not weaken a rule, a fixture, an `EXPECT`, or a test assertion to get green. A new rule
  gets an entry in `rules.md` and a fixture in `scripts/harness/fixtures/` (ADR 0009), unless it only warns:
  then it is marked "no fixture" and covered by bats tests. A test that fails because of your change means your
  change is wrong, unless the slice says the behaviour changes.
- **Full scope stays full.** Every skip, cache or scope applies only when `HARNESS_SCOPE` is `changed` (the
  `check` default). `check-all` and CI run with `HARNESS_SCOPE=full` and must run exactly what they run today,
  `-count=1` included. In the Makefile write `check-all: export HARNESS_SCOPE := full`; without `export` the
  scripts never see it. When the change set cannot be computed, run everything.
- **Portability.** Scripts must run on macOS `/bin/bash` 3.2 and on Linux bash 5:
  - milliseconds from `EPOCHREALTIME` when set, else `perl -MTime::HiRes` (both macOS and the Ubuntu runners
    have it); no `date +%N`;
  - empty arrays under `set -u` need `${a[@]+"${a[@]}"}`;
  - BSD/GNU differences need the existing workarounds (`sed -i.bak`); no `cp --remove-destination`, use
    `rm -f "$f" && cp "$src" "$f"`.
- **Fail closed.** Missing tools, a missing base ref, or unparseable input exit non-zero (NFR-R2). A skipped
  or scoped sensor runs everything when it cannot tell what changed.
- **Known traps:**
  - Linked files write through: `integrity-check.bats` "flags a change to sensor code" appends to the
    sandboxed `merge-base.sh`. Break the link before any test writes to a file, and assert in `teardown_file`
    that a checksum of the real `scripts/` and `.claude/hooks/` taken in `setup_file` is unchanged. Do not use
    `git diff --quiet` for this; it fails when the developer has uncommitted harness edits.
  - Use hardlinks, not symlinks, in the sandbox: `sandbox_base` commits the files, and a symlink later
    replaced by a copy shows up as a type change.
  - `guard-bash` blocks `rm -r` outside the repo; use bats' `BATS_TEST_TMPDIR` or paths inside the repo.
  - `.cache/` must be in `.gitignore` before anything writes to it.
  - `eslint.config.mjs` uses type-aware rules: do not add `eslint --cache`.
- **Go module (P8–P10).** Use `apps/api/tools/harness`, a nested module like `tools/analyzers`. In P8, add
  `apps/api/tools/harness/* config` to `scripts/harness/protected-paths.txt` and the same path to
  `.github/CODEOWNERS`. `.bin/harness` is a Make target depending on its sources and a prerequisite of
  `check`, `check-all` and `harness-selftest`.
  - Hooks: all four (`protect-harness`, `guard-bash`, `fast-check`, `only-make-check`) read their JSON
    input through the binary, not `node`. Keep `.claude/hooks/*.sh` as thin wrappers that `exec` the binary, so `.claude/settings.json` does
    not change. Missing binary → exit 2 with "run make setup"; the guard-bash wrapper lets `make setup` and
    `make .bin/harness` through in that case. Bats tests cover both paths. No source-time check per call.
  - Ported sensors (P9, P10): delete the old script and its bats file in the same slice, and replace them with
    Go table tests covering every case the bats file had. The script's fixtures must still be rejected.
  - P10: the `pull_request_target` workflow builds `.bin/harness` from the base-branch checkout only, never
    from the PR head (ADR 0010).
- **CI (P2, P3, P10).** Pin actions to commit SHAs as the existing workflows do, use minimal `permissions`,
  keep `actionlint` green, and follow the `drift.yml` pattern for the issue (one open issue, comment on
  repeats, 60 000-char cap).
- **Comments.** Match the surrounding style: a header comment per script saying what it checks and how to
  run it, with dense, factual comments.

**6. Verify.** Run `make check`, then `make harness-selftest` (set `SELFTEST_SKIP_DOCKER=1` only if Docker is
unavailable, and say so). For slices touching CI, a sensor's wiring, scoping, or the Makefile, also run
`make check-all`. Its `harness-integrity` step fails on every slice with `harness-config-not-loosened`
findings for the protected files you changed: that is the human gate (the PR label), not a defect. Check
that it names only files this slice changed, do not set `HARNESS_CHANGE_APPROVED`, and run the rest of
`check-all` directly (`make -k -j4 <its other targets>` and `make test-integration`). If verification fails, fix the cause; do not edit the sensor to pass. If you cannot get
green within this iteration:
- revert your changes (`git checkout -- . && git clean -fd`; no stash, it outlives the iteration);
- mark the item `- [!] Pn — <what failed, the command, the first error line>`;
- commit only that line (`docs(harness): mark Pn blocked`);
- end with `LOOP: BLOCKED`.

**7. Measure after and record.** Measure again as in step 4. Tick the item as
`- [x] Pn — <short sha> — <before> → <after>`. Update the "Today" column of each NFR you measured and add a
dated line to the plan's **Log**. Commit the slice first, then the plan update with the slice's sha in a
`docs(harness): record Pn` commit (a commit cannot contain its own sha):
- use a Conventional Commit with scope `harness` and type `perf`, `ci`, `test`, `refactor` or `build`, never
  `feat`;
- put why in the body;
- end the message with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

Then end with `LOOP: CONTINUE`.

**8. All slices done.**
1. Run `make check-all` and `make harness-selftest`.
2. Have the `code-reviewer` and `architecture-reviewer` subagents review `git diff docs/harness-nfr-plan...HEAD`,
   then fix their findings with tests in one `fix(harness): …` commit.
3. Re-measure every NFR into the table and set the plan status line to `Status: implemented, awaiting review`.
   Leave the ADRs `proposed`, because a human accepts them.
4. Commit, print a summary table (slice, sha, NFR before → after), and end with `LOOP: DONE`.

**Output and pacing.** The last line of your reply is exactly one of `LOOP: CONTINUE`, `LOOP: BLOCKED`,
`LOOP: DONE`. On `CONTINUE`, schedule the next iteration with the shortest delay the loop allows; nothing
external is pending. On `BLOCKED` or `DONE`, or if this iteration made no commit, end the loop instead of
scheduling another iteration.
