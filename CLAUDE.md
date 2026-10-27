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

## Developer convictions

- **Boy Scout Rule** ([Robert C. Martin](https://lawsofsoftwareengineering.com/laws/boy-scout-rule/)): leave
  the code you touch a little cleaner than you found it (a clearer name, a dead branch removed, a missing
  test). Keep it small and inside the files your change already touches; bigger cleanups get their own
  change or a line in the [tech-debt tracker](docs/exec-plans/tech-debt-tracker.md). The same goes for the
  workspace: when you are done, leave no loose ends (temp files and scripts, debug output, background
  processes, dev servers, containers, scratch branches or worktrees you created).
- **The best code is the code nobody writes**
  ([fullstackruby.dev](https://www.fullstackruby.dev/the-art-of-code/2015/01/30/the-best-code-is-the-code-nobody-writes/)):
  every line is a liability to read, test and maintain. Before adding code, check whether existing code,
  generated code, the platform or a deletion solves it; build only what the current task needs.
