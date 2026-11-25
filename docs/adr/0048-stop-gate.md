# ADR 0048: A session that stops on an unchecked or red tree is held back once (amends 0013, 0045)

Status: proposed · 2026-11-25 · amends [ADR 0013](0013-harness-nfr-budgets.md),
[ADR 0045](0045-harness-scripts-and-tests-in-go.md)

Slice S2 of [exec plan 0006](../exec-plans/active/0006-anthropic-harness-gaps.md), gap A2. On acceptance the notes on
ADR 0013 and 0045 lose their "proposed"; if this ADR is withdrawn, the notes are removed.

## Context
Golden principle 6 says done means `make check` is green. Nothing holds an agent to it: the PreToolUse and
PostToolUse hooks judge one call or one file, and the next check is the PR's CI. Claude Code runs a `Stop` hook when
the main agent has finished its answer; exit 2 sends the hook's stderr back to the model and the turn goes on. The
[hooks reference](https://code.claude.com/docs/en/hooks) and probes on 2026-11-25 (Claude Code 2.1.311, recorded in
the plan's Open assumptions) say:

- `stop_hook_active` is `true` on every stop after a block in the same turn, also after tool calls, and `false`
  again after the next prompt.
- A hook that runs into its `timeout` is killed and the stop goes through, silently.
- `Stop` does not fire for a subagent (that is `SubagentStop`), and has no `agent_id`. It does fire each time the
  main turn ends while background tasks run, and again when they have finished. `background_tasks` lists them with
  `type` (`subagent`, `shell`) and, for a shell, its `command`.
- Plain output at exit 0 does not reach the model. `{"systemMessage": "…"}` at exit 0 is shown to the person.

A check that costs minutes at every stop would be switched off. `make check` takes 2 s on a web edit, 11 s on a Go
edit and 173 s on a harness edit, where the shell tests run (2026-10-28 to 2026-11-02, [nfr.md](../harness/nfr.md)).

## Decision
- **One rule, `harness-stop-is-green`**, team-convention ([ADR 0015](0015-anticipated-rules-and-nfr-classes-b-o.md);
  this ADR is its decision record): a Claude Code session that changed the tree stops on one that `make check` called
  green, or is told once that it does not. The plan's `agent-stop-is-green` had a prefix no rule has; this one sits
  with the other hook rules.
- **The hook** is `.claude/hooks/stop-gate.sh`, a wrapper like the others (ADR 0045), which runs `.bin/harness
  stop-gate <root>`. It judges the checkout it belongs to, whatever directory the session is in. In order:
  1. `permission_mode` is `plan`: exit 0, silent. A plan session changes nothing and must not run the tree's code.
     Only the binary looks at it; the wrapper blocks a first stop without the binary in plan mode too.
  2. The tree is the one `make check` last called green, or the merge base's: exit 0, silent.
  3. `stop_hook_active` is not `false`: exit 0 with a `systemMessage` that the tree is not known to be green. A
     missing field counts as a repeated stop, so that the gate can never block twice in a row.
  4. A subagent in `background_tasks` is running, or a background shell whose whole command is `make check` or
     `make check-all` (a `cd <dir> &&` or assignments before it; options, assignments, redirections and pipes after
     it): exit 0, silent. The tree is
     half-written or about to be judged, and the gate runs again when the task has finished. This is not a pass;
     nothing is stamped. `make check && make dev` does not count: it would silence the gate until the server ends. A
     check it does not recognize (`time make check`) costs a second check at the same time, cut off at 15 s.
  5. Otherwise it runs `make check`, without the make variables of the session's environment (`MAKE*`, `MFLAGS`,
     `GNUMAKEFLAGS`; an `-i` there would end a red check green). Green: exit 0, silent. Red: exit 2 with every failed sensor,
     its rule and remediation, then the last 40 lines of the output.
  6. `make check` is not done after **15 s**: the gate ends it and exits 2 with "run `make check` yourself". The next
     stop goes through.
  7. Input it cannot parse, or no make: exit 2 with the reason (NFR-R2), on a first stop only (see "Block once"). A
     tree whose key cannot be taken (no git) is not known to be green, so step 2 does not apply and the others do.
- **The 15 s limit is one rule for every slow check.** Example: a Go edit whose check takes 40 s blocks once with "run
  `make check` yourself". The agent runs it, it is green, it leaves its stamp, and the next stop is silent. The plan
  had the gate skip the check and exit 0 with a notice when shell tests are in scope; the model would never see that
  notice. The plan also held the gate to NFR-H3's 10 s, which `make check` itself breaks on a Go edit. The gate gets
  its own line, NFR-H10, and its own row in `budgets.txt`. The limit covers the key's git calls too.
- **Two timeouts.** The 15 s is the gate's own: it ends the check's process group (TERM, 2 s later KILL), so that it
  can still block. Then it takes the key again, for 1 s at most. That is about 15.3 s in practice and 22 s at worst (15,
  2 to end the check, 1 for the key, 2 to end the key's git, 2 for what still holds its output open). `timeout` in `settings.json` is 30 s, for a gate that
  hangs; a hook killed there lets the stop through (probe). The binary also ends its check when it gets TERM, INT or
  HUP itself; the wrapper does not pass a signal on.
- **The message at the limit does not blame the change.** A cold cache, a loaded machine or golangci-lint's
  machine-wide lock, held by another worktree, look the same. Once the check has started, it asks for `make gen` first
  when the tree is not the one the gate started with, or cannot be shown to be: `gen-check` runs `make gen`, and one that was cut off can leave a generated file
  half-written. The next `make check` then goes red in a sensor before `gen-check` and never gets to rewrite it.
- **The stamp.** `make check` ends with `.bin/harness check-stamp . <key>`, which writes `.cache/harness/check.stamp`.
  The key is the tree as `git add -A` would stage it (tracked and untracked files, deletions and modes), the merge
  base with `BASE_REF` (`none` without one), and the effective `STACK`. That is what `make check` judges, but for the
  blind spots under Consequences: its sensors read the working tree, and scope `changed` compares it with the merge base
  ([ADR 0014](0014-make-check-scoped-to-the-change.md)). `tool-versions.env` is in the tree. `HEAD` is not in the
  key, so committing what was green needs no second check.
- **The key writes nothing to the repository.** It takes a copy of the index and a temporary `GIT_OBJECT_DIRECTORY`.
  The copy keeps the index's modification time: without it git misses a same-size edit made in the second of the last
  commit (5 of 5 in a probe; 0 of 5 with it). Split index and git hooks are off in these calls, as both can write to
  the repository.
- **The Makefile and the binary share one contract.** make expands a recipe's `$(shell …)` before it runs the first
  line, so one line takes the key from before the sensors and, when they are green, hands it to `check-stamp`, which
  takes it again and stamps only when both are equal. The key travels in that line and belongs to that one run; two
  runs at once cannot mix theirs, and the file is replaced by rename. `make check-all` stamps too (a superset). `-i`
  does not, since it ends green on red sensors: the line looks for an `i` in the words of `MAKEFLAGS` that are neither
  long options nor variables. `-n` and `-t` do not either, as the line holds no `$(MAKE)`. A `HARNESS_STACK` or a
  `BASE_REF` with another merge base, given for one run, leaves a stamp the gate does not match.
  `TestMakeCheckStamps` runs the real recipe green, red, under `-k`, `-i`, `-n` and `-t`, with another stack and with a
  tree that changes meanwhile; `TestTreeKey` covers another base.
- **The stamp is best effort.** `check-stamp` exits 0 for every tree; only a wrong call is a usage error. Without a
  key, or on a tree that changed while the check ran, it says so in one line on stderr; the price is one more check at
  the next stop.
- **Block once, also without the binary.** The wrapper reads `stop_hook_active` from the raw input, as
  `hook-bootstrap.sh` reads commands. It blocks for a binary that is missing, stale or crashed only when the input
  says `false` and nowhere `true`; otherwise it prints the `systemMessage`. A quote inside a JSON string is escaped,
  so only the key matches.

## What this changes in ADR 0045, ADR 0013 and NFR-R2
ADR 0045: "a missing or stale binary blocks with a way out". ADR 0013 and NFR-R2: a sensor that cannot run exits
non-zero. A stop that passes without the binary breaks the letter of all three, and is a recorded exception, for this
hook only:

- A PreToolUse hook that lets go lets a *call* through that nobody judged. A stop has no call to let through: what it
  would have judged is judged again by CI, which is required.
- The way out of a blocked call is not to make it. A stop has no such way out: an agent that cannot build the binary
  (no network, a broken Go file it may not touch) would never hand back to the person.
- It fails closed exactly once, and the pass is visible to the person. No verdict is claimed: the message says the
  tree is not known to be green.

## Consequences
- An agent that says "done" on a red tree reads the failing rule and its remediation before the person does. One that
  is green pays the key and the wrapper: 0.1–0.2 s (2026-11-25, Apple Silicon).
- **A session that changed nothing can be held back once:** on a branch that has no stamp yet, on a tree another
  session left red, or in a fresh clone without `.bin/harness`. Every block, the wrapper's own too, says the next stop passes.
- **A tree that stays red is held back once per turn,** not once per tree. An expected red tree (a `test-writer`
  commit before its implementation) costs one check and one more answer each turn.
- **A flaky red on a tree that was green once** is not seen: the stamp stays. Removing it at the start of a run would
  make a dry run write. Edits that end where they began (A, B, A) count as green for the same reason, which is right;
  so does a change made and taken back while a check ran (a stash popped before it ends), which is not.
- **It is a reminder, not a wall.** The agent can stop a second time, and can write the stamp file itself; both buy
  the same thing. CI's required checks are the wall.
- **The gate runs code from the tree without a prompt.** Plan mode is excluded. Two cases remain and are accepted: a
  person who asks about a stranger's checked-out PR has its `make check` run at the first stop, and an agent without
  Bash permission gets its edits executed. Every `make check` a person runs there does the same.
- **A second `Stop` hook that blocks** (a person's own) sets `stop_hook_active` for this one too. Both run on the
  first stop, so this gate has judged by then, unless that stop passed as busy; what the agent edits after the other
  hook's block is not judged. Accepted: telling the two apart needs a marker file per session.
- **A `make check` in a foreground call elsewhere** slows both, and the gate may hit its limit: it blocks once. When
  Claude Code kills the gate with KILL, its check runs on to its end. So does a process that left the check's group.
- **Only Claude Code sessions started in this checkout.** One started in a parent folder, in another worktree, or with
  hooks disabled (the CI agents) is not gated. Other agents have CI.
- **What the key does not see,** so that the gate can take a red tree for green. CI judges the commit and sees it:
  - the index alone: `deps-swift-resolved` and `codeowners.bats` read `git ls-files`, and `git rm --cached` leaves the
    key as it is;
  - what git does not stage as it is on disk: files marked `skip-worktree` or `assume-unchanged`, sparse checkouts,
    and what filters, attributes and `core.*` settings rewrite;
  - ignored files, the tools in `.bin/` among them, and uncommitted edits in a nested repository (one without a commit,
    or a file git cannot read, gives no key, and the check runs);
  - the environment of the run that left the stamp (`GOFLAGS`, `make check RUN=true`, `MAKE='make -i' make check`,
    `make -i check MAKEFLAGS=`), and a session's own `HARNESS_STACK`, which the gate follows, or `GIT_DIR` with
    `GIT_WORK_TREE`, which point the key at another repository;
  - an untracked `GNUmakefile`, which make reads instead of `Makefile` (TD-178: CI does not see a committed one
    either);
  - the merge base's own tree counts as green. On the default branch without `origin/main`, or with `BASE_REF=HEAD`,
    that is the last commit, red or not.
  Each needs a deliberate step or a rare setting.
- **Adopters take five things or none:** the hook, its `settings.json` entry, the last line of `check` with `STAMP`,
  the binary, and `scripts/harness/hook-bootstrap.sh` with `harness_stale`. Their `.gitignore` must name `.bin/` and
  `.cache/`, or the stamp changes the key it records. Without the Makefile line every stop on a changed tree runs the
  check. Their own `Stop` hook runs beside it. `make adoption-probe` does not run Claude Code, so it does not exercise
  the entry.
- **A check that is cut off leaves its sensors' temp logs** in `TMPDIR` (`run.sh` has no trap,
  [TD-177](../exec-plans/tech-debt-tracker.md)).
- The `harness-latency-budget` warning of NFR-H10 is a warning as everywhere (ADR 0013); the 15 s limit is not a
  latency verdict. It says "not checked", never "red".

## Alternatives considered
- **Skip the check when shell tests are in scope (the plan's text).** Rejected: exit 0 does not reach the model, and
  the 15 s limit covers that case and every other slow one.
- **No limit of its own, only `timeout` in `settings.json`.** Rejected: the killed hook lets the stop through.
- **`HEAD` plus `git status` plus the content of changed files as the key.** Rejected: committing a green tree would
  make it unchecked, and no sensor in `make check` reads `HEAD` beyond the merge base.
- **The stamp written by the gate only.** Rejected: an agent that just ran `make check` would pay for it twice.
- **A stamp for red too, so that the gate blocks once per tree.** Not built: the plan says once per stop. It is the
  next step if expected red trees cost too much.
- **A marker file per session to count blocks.** Rejected: `stop_hook_active` says it (probe).
- **`make check` in the background, verdict at the next stop.** Rejected: the next stop may never come.
- **`SubagentStop` too.** Rejected: reviewers change nothing and `test-writer` must leave red tests.
- **Block until green.** Rejected: a tree the agent cannot fix would trap the session; Claude Code's own cap of eight
  does not count when the model calls tools in between.
