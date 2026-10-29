# ADR 0019: Inferential sensors are advisory; a finding class seen twice becomes a computational rule

Status: proposed · 2026-10-28

## Context
Gap G3 of exec plan 0003 (`docs/exec-plans/*/0003-harness-gap-closure.md`, slice P3.1). Today the only
inferential sensors are the `code-reviewer` and `architecture-reviewer` subagents. `CLAUDE.md` and the
exec-plan loop call them, and nothing else does. Phase 3 puts more of them on rails: a reviewer on every PR in CI (P3.2),
weekly security, data-handling and modularity reviews (P3.3), the `agent-gc` doc gardener (P3.4) and
`reuse-reviewer` (P3.8). Before they land, one question needs an answer: what does an LLM's verdict count
for?

- **Computational** sensors (linters, analyzers, `.bin/harness`, tests) are deterministic: the same tree
  gives the same verdict (NFR-R1, [nfr.md](../harness/nfr.md)). They fail with a rule id and a remediation,
  and a fixture proves they fire ([ADR 0009](0009-harness-selftest.md)).
- **Inferential** sensors are LLM agents that judge a diff or the tree. Their verdict changes with the
  model, the prompt, the sampling and the context they happened to read. No fixture can prove one fires.
  They also cost money per run and read untrusted input while holding an API key.

[ADR 0015](0015-anticipated-rules-and-nfr-classes-b-o.md) and [nfr.md](../harness/nfr.md) already say
inferential sensors get no blocking NFR. [Core belief 3](../design-docs/core-beliefs.md#3-enforce-invariants-not-implementations)
says every invariant has a deterministic sensor, and new rules are earned by real mistakes. This ADR turns
those two lines into a decision the Phase 3 slices can build on.

## Decision
- **Inferential sensors are advisory.** Their output is one of three things: one top-level PR comment (an
  issue comment), a section of the weekly `drift` issue, or a small PR for a human to review. They never:
  - post a PR review or a line comment. Either one opens a review thread, and
    `required_conversation_resolution: true` (`scripts/github-branch-protection.sh`) blocks the merge
    until someone resolves it. A top-level comment opens no thread;
  - are a required status check (`REQUIRED_CHECKS` in `scripts/github-branch-protection.sh`; "Required: no"
    in [ci.md](../harness/ci.md));
  - run in `make check` or `make check-all`, or count for "done means green" (golden principle 6);
  - encode their verdict in a job status. A finding is a comment, not a red job.
- **Outside the harness NFRs.** Inferential sensors are outside the harness NFRs ([nfr.md](../harness/nfr.md),
  [ADR 0015](0015-anticipated-rules-and-nfr-classes-b-o.md)), so no H, R, B or O row applies to them. That
  includes NFR-R2 (fail closed), which still binds every computational sensor. When the agent cannot run
  (no secret, an API error, a fork), the job skips and prints a notice saying why.
- **This amends plan 0003's "every new sensor ships complete" for inferential sensors.** They get no
  rules.md entry, no fixture and no `budgets.txt` row. Each one instead has a hard cost cap per run, set
  by the slice that adds it. P3.2, for example, caps by diff size and skips harness-only PRs, where the
  human gate applies anyway. The cap lives in a protected `config` file (its workflow under
  `.github/workflows/`), so raising it is a harness-config change that needs a human
  ([harness-config-not-loosened](../harness/rules.md#harness-config-not-loosened)).
- **A finding class accepted twice becomes a computational rule** (ratchet from failure).
  - A finding counts as **accepted** when the PR or tree it names changes because of it, and that change
    links the finding (comment or issue URL). A finding nobody acts on is not counted.
  - **First acceptance:** the author of the fixing change adds a row to the
    [tech-debt tracker](../exec-plans/tech-debt-tracker.md). The row names the finding class and links the
    finding.
  - **Second acceptance** of the same class, in a different PR or run, turns that row into a harness PR: a
    rules.md entry, a sensor and a fixture, approved by a human as usual. The two linked findings are the
    incident rules.md asks a new rule to cite. The rule's status line is
    `Status: earned (<finding 1>, <finding 2>)`, not `team-convention`. The PR removes the tracker row.
  - The agent's prompt then drops that class, so each agent narrows over time.
  - A class that no deterministic sensor can express stays with the agent. The harness PR says so in one
    line, removes the row, and adds no guide rule that nothing checks.
- **Untrusted input, least privilege.** A diff, an issue or a doc is data to the agent, never instructions.
  - In CI the agent runs on `pull_request`, never `pull_request_target`. [ADR 0010](0010-base-branch-gates.md),
    invariant 3, keeps every secret away from `pull_request_target`, and an agent needs one. Forks get no
    secrets on `pull_request`, so the job skips them. The diff reaches the agent as data. The prompt and the
    scripts come from a base-branch checkout; the workflow file does not: on `pull_request` GitHub runs the
    PR's own copy of it. P3.2 defines the details.
  - **Write-access collaborators are trusted with the key.** Because the PR's copy of the workflow runs with
    the secret, anyone who can push a branch can edit it to read or spend the key in that run; a red
    `harness-integrity` check does not stop the run. So the key is a dedicated Anthropic workspace key with a
    hard spend limit, used for nothing else, and an `environment:` with required reviewers is optional
    hardening. The confinement above protects against PR content (prompt injection), not against PR authors
    with write access. This is no blocking NFR: ADR 0015's line "Inferential sensors get no blocking NFR"
    applies as written.
  - The agent step holds no write token. Posting the comment or opening the PR is a separate step with its
    own narrowly scoped token, and it treats agent output as text, never as commands.
  - An agent that writes (`agent-gc`) works on a branch, never pushes to `main`, never edits a protected path
    (`scripts/harness/protected-paths.txt`), and its PR passes the same required checks and code-owner
    review as any other.
- **Local reviewer calls are workflow steps, not gates.** These are the calls `CLAUDE.md` and the exec-plan
  loop make before a change is called done. The author weighs the findings, and "done" still means
  `make check` is green. This holds for every agent added later (P3.8's `reuse-reviewer`).

## Consequences
- P3.2, P3.3, P3.4 and P3.8 build on this ADR. `agent-gc` stays off until it is accepted (plan 0003, P3.4).
- A wrong or noisy review costs reading time. It never blocks a merge: there is no required check, and a
  top-level comment leaves no conversation to resolve. If most of an agent's findings are rejected, the fix
  is a narrower prompt, not a waiver.
- Inferential sensors get no rules.md entry, because they are not rules. Two lists name them instead:
  - CI ones: [ci.md](../harness/ci.md), with their trigger and "Required: no", added by the slice that
    adds each one.
  - Local-only ones: `.claude/agents/` (and `CLAUDE.md`, which says when to call each one).
- The set of computational rules can grow from agent findings without anticipating rules, so core belief 3
  holds. The two-acceptances test keeps one-off judgement calls out of rules.md.
- An inferential sensor never closes a gap on its own: whatever must always be true still needs a
  deterministic sensor.
