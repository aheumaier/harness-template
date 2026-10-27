@AGENTS.md

## Claude Code specifics

- After every Edit/Write, `.claude/hooks/fast-check.sh` runs the cheapest sensor for that file. Silent when the
  file is fine; if it blocks, fix what it prints before moving on.
- Harness files (golden principle 5) are guarded by a PreToolUse hook. Do not work around it; propose the change
  in the PR description.
- For new domains, endpoints and migrations, use the skills in `.claude/skills/`; do not improvise the workflow.
- Before calling a non-trivial change done, have the `code-reviewer` subagent review the diff; if it adds
  packages, domains or dependencies, also `architecture-reviewer`.
- Each worktree has its own `.env.local`; never read or print it, and never copy it between worktrees.

## Developer convictions

- **Boy Scout Rule**, inside the files your change already touches: leave them a little cleaner (a clearer
  name, a dead branch removed, a missing test). Bigger cleanups get their own change or a line in the
  [tech-debt tracker](docs/exec-plans/tech-debt-tracker.md). When done, leave no loose ends: temp files and
  scripts, debug output, background processes, dev servers, containers, scratch branches or worktrees.
- **The best code is the code nobody writes.** Before adding code, check whether existing code, generated code,
  the platform or a deletion solves it; build only what the current task needs.
