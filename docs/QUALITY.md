# Quality

One row per domain, graded A–F by the reviewer of the last change that touched it (human or the
`code-reviewer` subagent). Grade what an agent would experience: can it change this domain safely using only
the repo? `make new-domain` adds a row; update it in the PR that changes the domain.

| Domain | Grade | Tests | Docs | Notes |
|--------|-------|-------|------|-------|
<!-- harness:quality-rows (generator marker, keep) -->

Grades: **A** clear model, tested behaviour, AGENTS.md current · **B** minor gaps · **C** works, hard to change
safely · **D** known defects or missing tests · **F** do not build on it. Debt goes to
[exec-plans/tech-debt-tracker.md](exec-plans/tech-debt-tracker.md).
