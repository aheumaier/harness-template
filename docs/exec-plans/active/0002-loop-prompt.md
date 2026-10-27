# 0002 loop prompt — one slice per iteration

The prompt below implements [exec plan 0002](0002-fast-reliable-harness.md) one slice at a time. Each run
starts with a fresh context; all state lives in the plan's **Progress** checklist and in git, so a run can
crash or be stopped and the next one resumes.

## Running it

A human starts the loop. It edits harness files, so it needs `HARNESS_EDIT=1` and must run in its own
worktree (ADR 0008):

```bash
git worktree add -b perf/harness-nfr ../skeleton-harness-nfr docs/harness-nfr-plan
cd ../skeleton-harness-nfr && make setup
export HARNESS_EDIT=1
for i in $(seq 1 15); do
  before="$(git rev-parse HEAD)"
  out="$(claude -p "$(sed -n '/^## Prompt$/,$p' docs/exec-plans/active/0002-loop-prompt.md)")" || break
  printf '%s\n' "$out" | tail -n 15
  [[ "$(git rev-parse HEAD)" != "$before" ]] || { echo "no commit this iteration: stopping"; break; }
  printf '%s\n' "$out" | grep -q '^LOOP: CONTINUE$' || break
done
```

Give `claude -p` the permissions you trust in an isolated worktree. It needs Edit/Write plus Bash for `make`,
`git`, `go`, `bats`, `node`, `pnpm` and the read-only tools. In an interactive session (prefix the start with
`HARNESS_EDIT=1`), `/loop` with the prompt below works too.

The loop stops when an iteration ends in `LOOP: BLOCKED` or `LOOP: DONE`, when an iteration makes no commit,
or after 15 iterations. It never pushes. A human reviews the branch, opens the PR (Harness change: Y) and adds
the `harness-change-approved` label.

## Prompt

You are one iteration of a loop that implements exec plan 0002 in this repository. You start with no memory
of earlier iterations. The plan's **Progress** checklist and git history are the only state. Do exactly one
slice, leave the tree green and committed, then stop.

**1. Preconditions.** Run `git status --short`, `git branch --show-current` and `echo "$HARNESS_EDIT"`. The
branch must not be `main`, the tree must be clean, and `HARNESS_EDIT` must be `1`. If any of these fails, do
not change anything: print the reason and end with `LOOP: BLOCKED`.

**2. Read.** Read `AGENTS.md`, `docs/exec-plans/active/0002-fast-reliable-harness.md` (NFRs, phases,
Progress), `docs/adr/0012-parsing-sensors-in-go.md`, `docs/adr/0013-harness-nfr-budgets.md`,
`docs/adr/0009-harness-selftest.md`, and the sections of `docs/harness/rules.md` and the scripts the slice
touches. Run `git log --oneline -15` to see the earlier slices.

**3. Pick.** Take the first `- [ ]` item in Progress. If there is none, go to step 8. If an item is `- [!]`,
end with `LOOP: BLOCKED` and do nothing else.

**4. Measure before.** Record the numbers for the NFR this slice targets, for example `make check` wall time,
the sensor's time, or hook time per call. Run each measurement twice and keep the second (warm) result.

**5. Implement only this slice.** Follow these constraints:
- **Keep the output contract.** Rule ids, `file:line: message [rule-id]` lines, exit codes, and `run.sh`
  printing the remediation from `docs/harness/rules.md` stay as they are. Timing and budget warnings never
  change a sensor's exit status, and a failure to write timings never fails a sensor.
- **Never loosen.** Do not weaken a rule, a fixture, an `EXPECT`, or a test assertion to get green. A new rule
  gets a fixture in `scripts/harness/fixtures/` and an entry in `rules.md` (ADR 0009). A test that fails because
  of your change means your change is wrong, unless the slice says the behaviour changes.
- **Portability.** Scripts must run on macOS `/bin/bash` 3.2 and on Linux bash 5:
  - no `EPOCHREALTIME`, no `date +%N` without a fallback;
  - empty arrays under `set -u` need `${a[@]+"${a[@]}"}`;
  - BSD/GNU differences need the existing workarounds (`sed -i.bak`).
  Get millisecond timing portably (for example `perl -MTime::HiRes`, which both macOS and the Ubuntu runners
  have).
- **Fail closed.** Missing tools, a missing base ref, or unparseable input exit non-zero (NFR-R2). A skipped
  or scoped sensor runs everything when it cannot tell what changed.
- **Known traps:**
  - Bats sandbox symlinks write through to the real file: `integrity-check.bats` "flags a change to sensor
    code" appends to `merge-base.sh`. Copy a file before any test writes to it, and assert in `teardown` that
    `git -C "$REPO" diff --quiet -- scripts .claude` passes.
  - `guard-bash` blocks `rm -r` outside the repo; use bats' `BATS_TEST_TMPDIR` or paths inside the repo.
  - `.cache/` must be gitignored.
- **Go module (P8–P10).** Use `apps/api/tools/harness`, a nested module like `tools/analyzers`. Add its glob
  to `scripts/harness/protected-paths.txt` (kind `config`) and to `.github/CODEOWNERS`. `.bin/harness` is a
  Make target depending on its sources. A hook that finds it missing or stale fails closed with "run make
  setup". Delete the ported bash script and its bats file in the same slice, and replace them with Go table
  tests covering every case the bats file had. The script's fixtures must still be rejected.
- **CI (P2, P3).** Pin actions to commit SHAs as the existing workflows do, use minimal `permissions`, keep
  `actionlint` green, and follow the `drift.yml` pattern for the issue (one open issue, comment on repeats,
  60 000-char cap).
- **Comments.** Match the surrounding style: a header comment per script saying what it checks and how to
  run it, with dense, factual comments.

**6. Verify.** Run `make check`, then `make harness-selftest` (set `SELFTEST_SKIP_DOCKER=1` only if Docker is
unavailable, and say so). For slices touching CI, a sensor's wiring, or the Makefile, also run
`make check-all`. If verification fails, fix the cause; do not edit the sensor to pass. If you cannot get
green within this iteration:
- `git stash` or revert your changes;
- mark the item `- [!] Pn — <what failed, the command, the first error line>`;
- commit only that line (`docs(harness): mark Pn blocked`);
- end with `LOOP: BLOCKED`.

**7. Measure after and record.** Measure again as in step 4. Tick the item as
`- [x] Pn — <short sha> — <before> → <after>`. Update the "Today" column of each NFR you measured. Commit
the slice and the plan update together:
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

**Output.** The last line of your reply is exactly one of `LOOP: CONTINUE`, `LOOP: BLOCKED`, `LOOP: DONE`.
