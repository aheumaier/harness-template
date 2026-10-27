# Tech debt tracker

Known debt, newest first. Add a row in the PR that knowingly takes on debt; remove it in the PR that pays
it back. Larger paydowns get their own plan in [active/](active/).

| Id | Area | Debt | Impact | Added | Plan |
|----|------|------|--------|-------|------|
| TD-1 | scripts | `install-tools.sh`, `setup-worktree.sh` and `gen-schema-doc.sh` have no bats tests (rule sh-tests-pass); covered only end to end by `make setup` / `make check` | A regression shows up as a failed setup, not a pointed test failure | 2026-10-27 | — |
