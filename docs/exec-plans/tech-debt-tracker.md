# Tech debt tracker

Known debt, newest first. Add a row in the PR that knowingly takes on debt; remove it in the PR that pays
it back. Larger paydowns get their own plan in [active/](active/).

| Id | Area | Debt | Impact | Added | Plan |
|----|------|------|--------|-------|------|
| TD-5 | harness | Each scoped `make check` target (`lint-go`, `test-go`, `lint-web`, `arch-web`) runs `scripts/harness/affected.sh` itself, so one `make check` computes the same affected set up to four times | About 40 ms per target on this repo; grows with `go list`/`turbo ls` time on a large codebase | 2026-10-28 | Compute it once in `check` and pass it down (exec plan 0002 P11 log) |
| TD-3 | harness | `vuln-go` and `lint-go` run in `apps/api` only; the nested modules `apps/api/tools/analyzers` and `apps/api/tools/harness` (hooks' parser, `mvdan.cc/sh`) get no govulncheck or golangci-lint, only `go vet` in fast-check and their tests | A vulnerable dependency or lint finding in sensor code goes unseen | 2026-10-28 | Exec plan 0002 step 8, or a harness PR adding both modules to `vuln-go`/`lint-go` |
| TD-2 | github | `main` has no branch protection: the API returns 403 for private repos on GitHub Free, so `scripts/github-branch-protection.sh` is not applied | Required checks and code-owner review are advisory; a red PR can be merged. Locally only guard-bash blocks pushing `main` | 2026-10-27 | Apply the script after a GitHub Pro upgrade or making the repo public |
| TD-1 | scripts | `install-tools.sh`, `setup-worktree.sh` and `gen-schema-doc.sh` have no bats tests (rule sh-tests-pass); covered only end to end by `make setup` / `make check` | A regression shows up as a failed setup, not a pointed test failure | 2026-10-27 | — |
