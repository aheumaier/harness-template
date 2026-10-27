# ADR 0010: Label-waivable gates run the base branch's harness (pull_request_target)

Status: accepted · 2026-10-27

## Context
With `on: pull_request`, GitHub runs the PR's own copy of the workflow file. A PR could rewrite
`harness-integrity.yml` or `contract-gate.yml` (or the Makefile and scripts they call) to always pass, and the
required check would still turn green. The only remaining control would be CODEOWNERS review, which the
admin can skip (`enforce_admins: false`).

## Decision
- `harness-integrity.yml` and `contract-gate.yml` use `pull_request_target`. The workspace is the base branch
  (workflow, Makefile, scripts, protected-path list, tool versions). The PR head is a detached worktree, and
  the sensors read it through `HARNESS_ROOT`.
- Safety invariants, each restated in both workflow headers:
  1. Nothing from the PR tree is executed. It is read with git, awk and grep, and its spec is parsed by oasdiff.
  2. Only `head.sha` reaches a `run:` block, and only through `env`.
  3. `permissions: contents: read`, `persist-credentials: false`, and no secrets.
- `harness-config-not-loosened` covers sensor code (scripts/harness, analyzers, hooks, install script)
  and `.gitattributes`, as well as configs.

## Consequences
- `ci.yml` must run PR code, so it runs the PR's own workflow. A PR that weakens that workflow changes a
  `config` path, so the base-branch `harness-integrity` gate goes red until a human adds the label.
- A change to a gate takes effect only after it merges. Test it with `make harness-selftest` and the bats
  suite before merging.
