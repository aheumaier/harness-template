# Quality

One row per backend domain, with a grade A–F for each layer (`model`, `store`, `service`, `api`), given by the
human reviewer of the last change that touched it (the `code-reviewer` subagent suggests grades in its review;
a human writes the row). Grade what an agent
would experience: can it change this layer safely using only the repo? `make new-domain` adds a row with `–`
grades and today's date; update the row in the PR that changes the domain.

`Updated` is the day the row was last graded (`YYYY-MM-DD`). Change it only when you grade the row again,
never just to make it fresh: the weekly drift run lists rows older than 30 days (`make docs-quality`,
[docs-quality-row-fresh](harness/rules.md#docs-quality-row-fresh)). `make check-all` checks that each row
has one cell per column (cells hold no `|`), grades `A`–`F` or `–` (en dash, not a hyphen) and a real date,
and that the rows match the domains under `apps/api/internal/` (the domains' directory in `scripts/harness/layout.env`) one to one (`make docs-quality-shape`,
[docs-quality-rows-well-formed](harness/rules.md#docs-quality-rows-well-formed) and
[docs-quality-rows-match-domains](harness/rules.md#docs-quality-rows-match-domains)).

| Domain | model | store | service | api | Tests | Docs | Updated | Notes |
|--------|-------|-------|---------|-----|-------|------|---------|-------|
<!-- harness:quality-rows (generator marker, keep) -->

Grades: **A** clear model, tested behaviour, AGENTS.md current · **B** minor gaps · **C** works, hard to change
safely · **D** known defects or missing tests · **F** do not build on it · **–** not graded yet. Debt goes to
[exec-plans/tech-debt-tracker.md](exec-plans/tech-debt-tracker.md).
