# 0003 loop prompt — one slice per iteration

The prompt below implements [exec plan 0003](0003-harness-gap-closure.md) with Claude Code's `/loop`, one
slice per iteration, starting from a fresh session.

- **What an iteration does.** It works as an **orchestrator**: it hands the slice to a fresh subagent, then
  checks that work itself before committing it.
- **Why subagents.** The session context holds only verdicts, not 28 slices of file dumps.
- **Why the orchestrator checks.** Builder and checker stay separate, so a slice's own report is never its
  proof.

All state lives in the plan's **Progress** checklist and in git. A crash, an Esc or a compaction costs
nothing: rerun the `/loop` line and the next iteration resumes at the first open slice.

## Running it

A human starts the loop. It edits harness files, so it needs `HARNESS_EDIT=1` and its own worktree
(ADR 0008). Docker must be running (integration tests, selftest, observability stack). The network must be
reachable (tool downloads, Trivy DB).

```bash
git worktree add -b harness/gap-closure ../skeleton-gap-closure main
cd ../skeleton-gap-closure && make setup
HARNESS_EDIT=1 claude
```

Then, in that session:

```text
/loop Read the "## Prompt" section of docs/exec-plans/active/0003-loop-prompt.md and do one iteration exactly as it says.
```

Without an interval, `/loop` paces itself: after each iteration it schedules the next. Grant what you trust
in an isolated worktree:

- Edit and Write.
- Bash for `make`, `git`, `go`, `pnpm`, `node`, `bats`, `docker`, `curl`, `shasum`/`sha256sum` and the
  read-only tools.
- The Agent tool.

The loop ends on `LOOP: DONE` or `LOOP: BLOCKED`, or when an iteration makes no commit. Stop it any time
with Esc, and rerun the `/loop` line to resume. It never pushes, opens PRs or sets approval labels. A human
does that from the hand-off the final iteration prints.

## Prompt

You are one iteration of a `/loop` that implements exec plan 0003 in this repository. Do exactly one unit
of work, then stop. A unit is one of:

- one slice;
- one phase gate;
- the finish.

Leave the tree clean and committed.

### 0. Ground rules

- **Only state.** The Progress checklist in `docs/exec-plans/active/0003-harness-gap-closure.md` and
  `git log` are the only state. Don't rely on what you remember from earlier iterations in this session.
- **Never:**
  - push, open a PR, or set any approval override;
  - read, print or copy the worktree's env file;
  - raise a value in `scripts/harness/budgets.txt`;
  - weaken a rule, fixture, `EXPECT` or test to get green;
  - edit an applied migration;
  - use `aquasecurity/trivy-action`, `setup-trivy` or any Trivy Docker image;
  - install a tool globally (everything goes through `scripts/install-tools.sh` into `.bin/`).
- **Don't ask the user.** Settle open questions with the pre-made decisions (§4).
  - Record each choice in the plan's **Open assumptions** with the evidence behind it.
  - Mark a slice `[!]` only for a real external blocker: a secret, an upstream URL, a human decision with no
    default, or a tool that does not exist.

### 1. Preconditions

Run:

```bash
git status --short
git branch --show-current
echo "$HARNESS_EDIT"
docker info >/dev/null && echo docker-ok
```

End with `LOOP: BLOCKED <reason>` and change nothing if any of these is true:

- the branch is `main`;
- the tree is dirty;
- `HARNESS_EDIT` is not `1`;
- Docker is down.

### 2. Read

Read these:

- `AGENTS.md`
- plan 0003's NFRs, Phases and Progress
- the `## Slice rules` section of this file
- `git log --oneline -20`

On the first iteration of a session (nothing about this plan in your context yet), also read
`ARCHITECTURE.md`, `docs/design-docs/core-beliefs.md`, ADRs 0009, 0010, 0012, 0013 and 0014, the top of
`docs/harness/rules.md` (format and adoption rules) and `scripts/harness/protected-paths.txt`.

### 3. Pick the unit

Check these in order:

1. **Phase gate.** If every slice of a phase is `[x]` or `[!]` and that phase's State in the Phases table is
   still `proposed`, the unit is that phase's gate. Go to §5g.
2. **Slice.** Otherwise, take the first `- [ ]` slice whose dependencies are all `[x]`:

   | Slice | Depends on |
   |-------|------------|
   | Q1, Q4, Q6 | Q0 |
   | Q2 | Q1 |
   | P1.2 | P1.1 |
   | P1.3 | P1.2 |
   | P1.4 | P1.1, P1.3 |
   | P1.5 | P1.4 |
   | P2.3 | P1.5 (for `make dev` log paths) |
   | P2.4 | P1.2 (the response header is in the contract) |
   | P3.2, P3.3, P3.4 | P3.1 |
   | P3.4 | Q3 |
   | P3.8 | P3.1, P3.6, P3.7 (the agent skips what the sensors already catch) |
   | P4.2 | P4.1 |

   A slice whose dependency is `[!]` is skipped, not blocked: take the next pickable one.
3. **Finish.** If no `[ ]` slice is left, go to §6.
4. **Blocked.** If `[ ]` slices remain but none is pickable (every remaining one waits on a `[!]`), print
   them with their blocker and end with `LOOP: BLOCKED`.

ADR slices (Q0, P1.1, P3.1, P4.1) leave the ADR `proposed`. Later slices build on the proposed decision. The
finish lists which commits to drop if a human rejects that ADR.

### 4. Pre-made decisions (do not re-litigate)

**Trivy version (Q1).**
- Choose the newest release that is at least 14 days old and is not v0.69.4.
- Download `checksums.txt` and each OS/arch archive. Verify the archives against `checksums.txt`, then pin
  each sha256 in `scripts/tool-versions.env`.
- If `cosign` happens to be on PATH, verify the signature too and say so. Otherwise the finish lists
  "cosign provenance check" as a human TODO.
- Do the same sha256 pinning for `atlas` and `shellcheck`.

**Offline proof (A2).** Run the secret scan with `HTTPS_PROXY=http://127.0.0.1:9 HTTP_PROXY=http://127.0.0.1:9`
(any network attempt then fails).
- If it still passes with correct findings, `repo-no-secrets` goes in `make check`.
- If not, it goes in `check-all` only, and the plan says why.

**Observability stack (P1.1, A5).**
- Write the ADR for `grafana/otel-lgtm` (version pinned by digest).
- Measure its idle RSS and its time to ready on this machine. If it exceeds 1.5 GB RSS or 90 s to ready, the
  ADR instead picks a single OTel collector container writing JSON lines to `.tmp/obs/`, and
  `make logs`/`make metrics` query those files with `jq`.
- Record the numbers in the ADR.

**Web tests (P2.2).** Keep `renderToString` unless P2.3 or a scaffolded test needs interaction. If the
answer is "no change", tick P2.2 as a no-op with the reason.

**pnpm (Q5).**
- Find pnpm 12.15.0's effective `minimumReleaseAge` default from its docs or changelog at that version.
- Set exactly that value explicitly, and cite the source in the commit body. Never a higher one.

**Mutation tools (P2.6, A7).**
- Use `gremlins` (Go) and Stryker (TS), pinned. Check that each exists and still runs on the pinned
  toolchain.
- If one doesn't, ship the other alone and record the gap in the tech-debt tracker. Don't block the slice.

**CI reviewer (P3.2, A6).** Ship the workflow inert without the secret:

- it skips with a notice when `ANTHROPIC_API_KEY` is absent;
- triggered by `pull_request` only, never `pull_request_target`;
- the agent step runs with no write token, and a separate comment step has `pull-requests: write`;
- actions pinned by SHA.

It is verified by `actionlint` and by a dry run of the prompt assembly. Don't mark it blocked; the secret
is a human TODO.

**Orphans, clones, reuse (P3.6–P3.8, G11).**
- P3.6: a make target counts as an entry point when it has a `## ` help comment; an entry point needs no
  caller. A script used only by a fixture is an orphan.
- P3.7: if `jscpd` cannot be pinned within the pnpm release-age policy, ship `dupl` alone plus a tech-debt
  row. Thresholds: the strictest that passes after cheap fixes; never a baseline or ignore file for
  existing clones.
- P3.8: name `reuse-reviewer`; tools Read, Grep, Glob only; the CLAUDE.md line is part of the slice (a
  harness change). It reviews the diff, never the whole repo; a whole-repo run waits for P3.3's schedule.

**Template sync (P4.3).** The upstream URL comes from `HARNESS_UPSTREAM` in `scripts/tool-versions.env`,
defaulting to this repo's `origin`. A missing upstream makes `make harness-sync-check` exit 2 with a
remediation; it never passes silently.

**Feature spec rule (A1).**
- Commits for P1.2 and P1.3 use `feat(platform): …` and say in the body: "Spec: exec plan 0003 (P1.x);
  needs label no-spec-needed."
- The local `specs-required-for-features` finding on them is expected. Treat it like the harness-integrity
  gate: check that it names only those commits.
- Every other slice uses `ci`, `build`, `test`, `refactor`, `docs` or `chore` with scope `harness` (or
  `platform`/`web` where the code lives), never `feat`.

### 5. Slice: delegate, check, commit

**a. Measure before.** If the slice targets an NFR, measure it twice and keep the second (warm) result.

**b. Delegate.** Spawn one `general-purpose` subagent with a fresh context. Use this brief, with the
placeholders filled in:

> You are implementing slice `<ID>` of exec plan 0003 in `<repo path>`, branch `<branch>`.
>
> - Read the section `## Slice rules` of `docs/exec-plans/active/0003-loop-prompt.md` first and follow it
>   exactly.
> - Then read the plan's entry for `<ID>` (Progress line and its phase section) and the files it names.
> - Earlier slices are committed. `git log --oneline -20` shows them; build on them and don't redo them.
> - Decisions already made for this slice: `<paste the matching §4 bullet, or "none">`.
> - Implement only `<ID>`. Leave every change uncommitted.
> - Finish with the report the slice rules define.

**c. Check the report yourself.** Don't trust the report; rerun the checks:

- `git status --short` and `git diff --stat`: every path belongs to this slice. Nothing under `.claude/`
  changed unless the slice names a skill or agent. No budget raised. No suppression without `reason: …
  (rule: <id>)`.
- `make check`, then `make harness-selftest`.
- For slices touching CI, the Makefile, sensor wiring or scoping: also `make check-all`. Expect
  `harness-config-not-loosened` findings for this slice's protected files only (the human gate, not a
  defect). Run check-all's remaining targets with `make -k` to see they are green.
- Every new rule:
  - its `rules.md` entry passes `rules-fixture-claims` (once Q3 exists);
  - its fixture is rejected: `make harness-selftest FIXTURE=<name>`;
  - a planted violation in the real tree fails with the rule id and remediation, and passes again once
    reverted.
- The slice's exit evidence from the plan, such as the NFR measured again or the make target producing the
  claimed output.

**d. Retry once.** On a failed check, send the subagent the failing command and its first error lines via
SendMessage, then check again.

**e. Block.** If it fails a second time:

1. Revert (`git checkout -- . && git clean -fd`, no stash).
2. Mark `- [!] <ID> — <what failed, command, first error line>`.
3. Commit only that line (`docs(harness): mark <ID> blocked`).
4. End with `LOOP: CONTINUE`. The next iteration picks a slice that doesn't depend on this one.

**f. Commit.** Commit the slice: Conventional Commit, the why in the body, ending with
`Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`. Then commit the plan update separately
(`docs(harness): record <ID>`); a commit can't contain its own sha:

- tick `- [x] <ID> — <short sha> — <before> → <after>`;
- update each NFR "Today" cell measured;
- add any new Open assumption;
- add a dated Log line.

End with `LOOP: CONTINUE`.

**g. Phase gate** (a unit of its own):

1. Run `make check-all` and `make harness-selftest`.
2. Have the `code-reviewer` subagent review `git diff <phase's first slice commit>^..HEAD`. When the phase
   added packages, dependencies, containers or CI jobs, also run the `architecture-reviewer` subagent.
3. Fix findings in one `fix(<scope>): …` commit with tests. A finding you reject gets one Log line saying
   why.
4. Check the phase's exit criterion from the Phases table, with evidence. Set that phase's State to
   `implemented` (or `partial: <slices blocked>`), and commit.

End with `LOOP: CONTINUE`.

### 6. Finish

1. Run `make check-all`, `make harness-selftest` and `make harness-determinism`. NFR-R1 must still be 0
   differences, because every new sensor in `make check` has to be deterministic.
2. Have `code-reviewer` and `architecture-reviewer` review `git diff main...HEAD`. Fix the findings as in
   §5g.
3. Set the plan status to `Status: implemented, awaiting review`, or `partially implemented` if anything is
   `[!]`. Leave every ADR `proposed`.
4. Commit and print the hand-off:
   - a table of slice, sha(s), "Harness change", rule ids added, and NFR before → after;
   - suggested PR cuts, with one PR per slice or per tightly coupled slice group, each listing its commits
     in order and the labels it needs (`harness-change-approved`, `no-spec-needed`);
   - human TODOs: ADRs to accept; the cosign provenance check; the `ANTHROPIC_API_KEY` secret;
     `HARNESS_UPSTREAM`; and every `[!]` with its reason;
   - commits to drop if a given ADR is rejected.

End with `LOOP: DONE`.

### Output and pacing

The last line of your reply is exactly one of `LOOP: CONTINUE`, `LOOP: BLOCKED <reason>`, `LOOP: DONE`.

- On `CONTINUE`, schedule the next iteration with the shortest delay the loop allows; nothing external is
  pending.
- On `BLOCKED` or `DONE`, or if this iteration made no commit, end the loop instead of scheduling another.

## Slice rules

These rules bind every slice subagent. Plan 0002's [loop prompt](0002-loop-prompt.md) proved them, and plan
0003 added to them.

**Scope**
- Implement only your slice. When you notice something else, put it in your report under "Noticed".
  Fix it only if it is in a file you already touch and is small (Boy Scout rule).
- Leave changes uncommitted. The orchestrator commits.

**Every new sensor ships complete** (plan 0003 Goal):
- A `docs/harness/rules.md` entry: rule, why, sensor, remediation, fixture, status. Use `team-convention`
  for rules the Q0 ADR adopts.
- A violation fixture in `scripts/harness/fixtures/<name>/` (ADR 0009). Prefer an `apply.sh` that changes
  one line over copying whole files.
- Sensors that depend on an external DB get `**Fixture:** none (<reason>)` instead.
- A `budgets.txt` row if it may exceed 5 s. Never raise an existing row.

**Determinism (NFR-R1)**
- Nothing that reads the network, the clock or an advisory DB goes in `make check`.
- Every skip or scope applies only when `HARNESS_SCOPE=changed`. `check-all` and CI run everything
  (`check-all: export HARNESS_SCOPE := full`).
- When the change set can't be computed, run everything.
- Sensors read git-known files (`git ls-files --cached --others --exclude-standard`), not the raw
  filesystem.

**Output contract**
- `file:line: message [rule-id]`, non-zero exit on violation.
- `run.sh` prints the remediation from `rules.md`.
- Silent on success.
- Timing never changes an exit status.

**Fail closed (NFR-R2).** A missing tool, base ref or binary, or unparseable input, exits non-zero with what
to run (`make setup`).

**Contract first and parse at boundaries** (golden principles 1 and 2)
- An API change starts in `api/openapi.yaml`, then `make gen`; never hand-edit generated code.
- Untrusted input (headers, PR diffs) is validated where it enters.
- Errors leave as `application/problem+json`.

**Layers.** `platform` imports no domain. Web code talks HTTP only through `packages/api-client`.

**Portability.** Scripts run on macOS `/bin/bash` 3.2 and Linux bash 5:
- milliseconds from `EPOCHREALTIME`, else `perl -MTime::HiRes`;
- `${a[@]+"${a[@]}"}` for empty arrays;
- `sed -i.bak`;
- no GNU-only flags.

**Tools.** Every new binary is pinned in `scripts/tool-versions.env` with a per-OS/arch sha256 and installed
by `scripts/install-tools.sh` into `.bin/`. JS tools are pinned in `package.json` at a release at least 14
days old.

**CI**
- Actions pinned by commit SHA; minimal `permissions`; `actionlint` green.
- Never `pull_request_target` for anything that runs PR code or feeds PR content to an agent (ADR 0010).
- Scheduled findings follow the `drift.yml` issue pattern.

**Runtime access.** Agents and skills reach worktree services only through `make` targets that source the
env file themselves. No guide or skill tells an agent to read it.

**Known traps** (from plan 0002):
- bats sandboxes hardlink; break the link before a test writes to a file.
- `guard-bash` blocks `rm -r` outside the repo; use `BATS_TEST_TMPDIR`.
- `.cache/` and `.tmp/` must be gitignored before anything writes there.
- No `eslint --cache` (type-aware rules).
- A fixture under `scripts/harness/fixtures/` must not hold a literal violation that a repo-wide content
  scanner flags, or it turns the real tree red: assemble it in `apply.sh` (as the `repo-secret-*` fixtures
  do), or exclude the directory from that scanner only if the scanner has no blind spot there (the vuln scan
  skips it; the secret scan must not, since Phase 0 gate).

**Verify before reporting.** Run `make check` and `make harness-selftest` (`FIXTURE=<name>` for yours). Run
`make check-all` if you touched CI, the Makefile or wiring. Fix the code, not the sensor.

**Report** (your final message, nothing else):

```text
SLICE <ID>: READY | FAILED
Changed: <paths, one per line, H if protected>
Rules added: <ids or none>
Evidence: <commands run, exit codes, before → after numbers, fixture verdicts>
Decisions: <choices made + evidence, for Open assumptions>
Noticed: <out-of-scope issues or none>
```
