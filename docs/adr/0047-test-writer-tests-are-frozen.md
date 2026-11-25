# ADR 0047: The test writer's tests are frozen until the spec changes (amends 0039, 0046)

Status: proposed · 2026-11-25 · amends [ADR 0039](0039-tests-are-written-apart-from-the-implementation.md) and
[ADR 0046](0046-agent-verdicts-are-advisory-and-measured.md)

Slice T1 of [exec plan 0006](../exec-plans/active/0006-anthropic-harness-gaps.md), gap A6. On acceptance the notes
on ADR 0039 and ADR 0046 lose their "proposed"; if this ADR is withdrawn, the notes are removed.

## Context
ADR 0039 has `test-writer` write the failing tests from the spec; the caller commits them with the trailer
`Test-Writer: specs/<NNN-slug>` before the implementer starts. A hook confines the test writer to test files.
Nothing confines the implementer: it may edit an expected value, delete a case or add `t.Skip`, and
`make check` is green again. Only `code-reviewer` check 7 looks for that. It is advisory, it depends on the
caller passing it the diff, and it does not run in CI
([TD-125](../exec-plans/tech-debt-tracker.md)). Anthropic's
[Effective harnesses for long-running agents](https://www.anthropic.com/engineering/effective-harnesses-for-long-running-agents)
gives its agents the rule "It is unacceptable to remove or edit tests". Whether a line of a test changed since a
commit is a question git answers, so it needs no model.

## Decision
- **One rule, `specs-tests-frozen`**, team-convention
  ([ADR 0015](0015-anticipated-rules-and-nfr-classes-b-o.md); this ADR is its decision record), checked by
  `harness specs-frozen` in the Go harness binary ([ADR 0012](0012-parsing-sensors-in-go.md),
  [ADR 0045](0045-harness-scripts-and-tests-in-go.md)) as `make specs-frozen`. It runs in `make check-all` and so
  in CI's `check-all` job, not in `make check`, because it reads the branch's commits. It is never waived by a
  label.
- **What the branch did** is the commits `HEAD` holds and `BASE_REF` does not. CI checks out GitHub's merge of the PR
  and the base, so a comparison with `HEAD` would also hold what the base did since the test commit. Each commit
  is compared with its own parent instead, a merge commit with each parent of the branch, and `git blame` says
  which commit a removed line came from. The base renaming a test file, skipping another test in it or editing the
  spec neither fails nor lifts anything.
- **A test commit** holds tests `test-writer` wrote from the spec, without reading the implementation. The sensor
  knows it by its form: such a commit with a line `Test-Writer: <value>` in its message (letter case and the blanks
  after the colon do not matter) that changes only product test files and files under `specs/`. A commit that
  carries the line and changes anything else is no test commit: the sensor prints a notice and goes on. A merge
  commit is none either. Squashing
  the tests into the implementation, or amending one into the other, makes such a commit, and freezing the
  implementer's own tests with it would be wrong.
- **Every test commit is frozen, not only the first.** ADR 0039 allows one test commit per story. If only the
  first froze, the second story's tests could be edited freely. A line belongs to the test commit that wrote it.
- **Findings**, in the product test files (`productTests`, the predicate the test writer's own hook and
  `specs-scenarios-tested` use) that a test commit `T` added or changed:
  - a line of `T` that a later commit of the branch took away (a changed line is a removed and an added one) and
    no later commit brought back, in a file `HEAD` still has: one finding per line, with its number in `T` and both
    commits. The same text in the same file brings a line back, so a moved line or a restored one is none. A
    test commit's line brings nothing back: its lines are its own, and the `}` of a new test would excuse a removed
    one. Blank lines check nothing and are never frozen;
  - a file that is gone at `HEAD`, when the branch's newest deletion of it is not a test commit's: one finding.
    Renames are not followed, so a renamed file is a deleted one;
  - a line the branch added after `T` that skips a test (the markers `specs-scenarios-tested` reads, Swift's
    `#if false` among them) or is a Go build constraint (`//go:build`, `// +build`).

  "After" and "later" mean not before `T`: a commit on a side branch counts too. Each finding is per test commit, so
  an added skip in a file of two of them is printed twice. A file that `HEAD` has exactly as `T` left it has no
  finding, whatever happened in between and whoever restored it. Other added lines and added files pass.
- **A later test commit may change what an earlier one wrote.** What a test commit removes or deletes is no
  finding. So the second story's tests may join the first story's file, and a test that is wrong, or that no
  longer fits a changed plan.md, goes back to `test-writer`. An implementer's edit stays a finding, also when a
  test commit follows: restore the test in a commit first, then let `test-writer` correct it.
- **A spec change lifts the freeze.** The findings of `T` do not count when the branch changed
  `specs/<NNN-slug>/spec.md`, named by `T`'s line, after `T`, and the content at `HEAD` differs from the one at
  `T` or is gone (the spec was dropped or renumbered). The sensor prints a notice with the commit, the spec and the
  number of changes it let through. The plan's first text said "changed in the same range"; read as `<base>..HEAD`
  that would lift the freeze on nearly every branch, because a feature's spec is committed before its tests, in the
  same branch. A value that is not `specs/<NNN-slug>`, or whose spec `T` does not hold, lifts nothing: the finding
  says why, and that rewording the commit, or moving the spec's commit before it, corrects it. A branch that leaves
  its tests alone is not blocked by a typo in the line.
- **Notices are shown.** `run.sh` dropped everything a green sensor printed, so nobody would have read "this
  branch's tests are not frozen". It now prints a green sensor's lines that start with `notice: `.
- **Coarser than check 7, on purpose.** The sensor asks whether a test writer's line was touched without a spec
  change. Reformatting a line, or turning `import "testing"` into a block, touches it too. The remediation
  names the ways out: put added tests into a new file, send a wrong test back to `test-writer`, or change the
  spec first.
- **Check 7 stays**, for what the sensor cannot judge: whether the spec change explains the test change, whether
  a later test commit loosens, and loosening through another file (`TestMain`, testdata, a runner config), by a
  value computed with the code under test, or by added code around a frozen line (an early `return`, an
  `if false`, a line inside a frozen block). It also covers what reading lines cannot tell:
  - a changed line whose old text the branch adds again elsewhere in the same file counts as moved;
  - of lines that look alike in a row, git picks the one that counts as removed, which can be a frozen neighbour
    of the implementer's own;
  - a conflict resolution that drops a line both sides of the merge hold;
  - a copy of a frozen line in a commit of a side branch without the trailer: after a merge, git can call the line
    that commit's;
  - a skip in a form the markers do not know (`it.only`, `it.skipIf(…)`, `b.Skip()`).

  It finds test commits the same way as the sensor: its caller drops a commit that changes anything else.
- **A journey the app evaluator added** ([ADR 0046](0046-agent-verdicts-are-advisory-and-measured.md)) **is frozen
  the same way, under its own trailer `App-Evaluator: specs/<NNN-slug>`.** Without the freeze the builder could
  accept a finding and then loosen the one thing that makes it binding. Its commit holds the journey and the fix,
  often with unit tests, so under this trailer only journeys freeze (the files slice V2 lets the evaluator write)
  and the rule "changes only test files" does not apply. A later commit with the same trailer may change the
  journey, as above, but only if it changes journeys and nothing else: a fix and the loosened journey could
  otherwise share one commit. The trailer is its own because `Test-Writer:` says who wrote the tests and from what, and
  because the builder commits the journey together with the fix. No code yet: slice V3 makes the sensor read the
  second key, and if the evaluator is dropped (ADR 0046's trial) this decision takes no effect.

## What this changes in ADR 0039 and ADR 0046
- **ADR 0039, "A later change to them needs a spec change first, and code-reviewer check 7 reports it".** A sensor
  now fails on it, and a later test commit by `test-writer` is the second way.
- **ADR 0039, check 7 "since the first `Test-Writer:` commit".** It becomes: since each test commit.
- **ADR 0046, "Whether an evaluator journey is frozen … is T1's question".** It is, under `App-Evaluator:`.
- **ADR 0039, the flow.** `test-writer` gets a second task: correct a test of its own that the spec contradicts.
  The implementation may exist by then; it still does not read it.
- If ADR 0039 is rejected after its re-measure ([TD-126](../exec-plans/tech-debt-tracker.md)), the `Test-Writer:`
  part goes with `test-writer`, and the rule and its sensor too unless V3 has built `App-Evaluator:`.

## Consequences
- An implementer that edits a frozen test sees `make check-all` red with the file, the line, the commit and the
  ways out.
- **It makes a loosened test visible; it does not make it impossible.** Any edit to the spec lifts the freeze, a
  blank line or the template's `**Status**:` line too. A commit can carry a forged `Test-Writer:` line. Both show
  in the PR, and check 7 judges them. The order test-then-code is still the caller's duty (ADR 0039).
- Time: the sensor runs one or two git commands per commit of a branch with test commits (a few more where a frozen
  line was removed), one blame per touched test file, and a fixed few on a branch without one. `make check` is
  unchanged. The measured time is in the plan's T1 tick.
- **The freeze lasts for one PR.** Once the test commit is in `BASE_REF`, it is out of the range: a later feature
  changes old tests like any other code. In a stack, CI compares with the PR's own base, so the tests of the PR
  below are frozen only there; a local run compares with `origin/main` and covers both. A branch continued after
  its PR was squash-merged still holds its old test commits: start a new branch.
- **The history is trusted.** A test commit whose trailer is dropped, or that is squashed into the
  implementation, is invisible, as a missing one is.
- **It reads commits.** An uncommitted edit is found once it is committed, and a restored line is green once the
  restore is committed.
- **Inside a merge commit** a removed line and an added skip are found; a file deleted there is not, and a spec edited
  there lifts nothing.
- **A restored skip or build constraint of `T`,** in a file that has other lines than `T` by now, reads as an added
  one: drop its removal with `git rebase -i`. **An added line with a call named `.skip(…)`** reads as a skip
  whatever it calls; put that test into a new file.
- **A spec edit that the branch takes back** lifts the freeze all the same when the base changed that spec too.
- **A shallow clone** that ends inside the branch is refused (exit 2).
- **When the base renames a test file,** the lines of `T` in it are no longer followed. **When the branch deletes a
  test file and restores it,** and the base deletes it later, the sensor reports the branch's deletion: rebase.
- **Deleting an old test and adding one with the same line in the same commit** can report the new line's
  twin as removed (1 to 4 of 40 generated histories). Delete in a commit of its own.
- **A skip a later test commit adds to an earlier one's file is a finding too.** A test writer has no reason to
  skip; a constraint for a whole file belongs in a new file.
- **In CI it runs with the PR's own harness**, as most of `check-all` does. A PR that changes the binary, the
  Makefile or the layout to pass needs the `harness-change-approved` label
  ([ADR 0010](0010-base-branch-gates.md)). It reads the tree at `HARNESS_ROOT` and takes the layout and the
  protected paths from its own root, so the base branch's harness could run it later with a workflow change alone.
- Testdata and golden files stay with check 7: a test writer's testdata is often regenerated, and it is no
  product test file to the predicate.

## Alternatives considered
- **Compare each test commit with `HEAD` (`git diff T HEAD`).** Built first and rejected after review: the diff also
  holds what the base and later test commits did, and it pairs the lines that look alike in every test (`}`,
  `t.Parallel()`) differently each time, so an untouched test turned red and a deleted one passed. Blame follows
  each line through each commit. Blame and diff must use the same algorithm, so both run with `--minimal`; patience
  in the diff and git's default in the blame disagreed.
- **Compare test syntax (Go AST, TypeScript AST) and let harmless edits pass.** Rejected: one parser per
  language, and "harmless" is the reviewer's judgment.
- **Freeze a file at the latest test commit that touched it**, instead of per test commit. Rejected: T1 writes an
  assertion, the implementer loosens it, the second story's test commit appends to the file, and the comparison
  with that commit is empty.
- **Read the trailer with `git log --format='%(trailers)'`.** Rejected: git reads trailers in the last paragraph
  only, so `git commit -m … -m 'Test-Writer: …' -m 'Co-Authored-By: …'` has none and would silently not be
  frozen.
- **Ignore blank-line and `**Status**:` edits of the spec.** Not now: a test that is wrong no longer needs a
  spec edit, so the cheap way out is the honest one.
- **Compare with the working tree, as `integrity-check` does.** Rejected: what the branch did is read from
  commits, and the two would disagree.
- **A PreToolUse hook that blocks the edit.** Rejected: a hook sees the edit and not whether the spec change
  follows, and it does not bind an agent without hooks or a human. The sensor reads the result.
- **A label waiver.** Rejected: the spec is the waiver, and it is the record golden principle 7 wants.
- **`Test-Writer:` for the evaluator's journey too.** It would freeze it today with no code, but makes the
  trailer mean two things and puts an after-the-fact test where check 7 expects one written first.
- **Hashes of test files in a committed list.** Rejected: a second record of what git already stores.
