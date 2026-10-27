@AGENTS.md

## Claude Code specifics

- After every Edit/Write, `.claude/hooks/fast-check.sh` runs the cheapest sensor for that file. It is silent
  when the file is fine; if it blocks, fix what it prints before moving on. Still run `make check` at the end.
- Harness files (see golden principle 5) are protected by a PreToolUse hook. Do not work around it; propose
  the change in the PR description.
- Use the skills in `.claude/skills/` (`new-domain`, `add-endpoint`, `add-migration`) instead of improvising
  those workflows.
- Before declaring work done on a non-trivial change, ask the `code-reviewer` subagent to review the diff;
  for changes that add packages, domains or dependencies, also ask `architecture-reviewer`.
- Each worktree has its own `.env.local`; never read or print it, and never copy it between worktrees.
