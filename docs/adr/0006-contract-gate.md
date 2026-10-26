# ADR 0006: Breaking contract changes need a human (contract-gate)

Status: accepted · 2026-10-27

## Context
An agent can make a breaking API change look like a routine refactor: both generated sides compile and all
tests pass, yet deployed clients break. The decision to break a contract is a product decision, not a lint fix.

## Decision
- **oasdiff v1.34.1** `breaking --fail-on ERR` compares `api/openapi.yaml` with its version at the merge
  base of `BASE_REF` (default `main`) via `scripts/harness/contract-check.sh` (`make contract-check`, in
  `check-all`). A spec that does not exist at the base passes.
- In CI it runs as its own workflow, `contract-gate.yml`, a **separate required status check**. It is red
  only for a breaking change and always runs (green when the spec is untouched), so it never blocks
  unrelated PRs. `api/openapi.yaml` is owned by CODEOWNERS; an intended break is merged by a human who
  approves the PR and records the reason in the PR description.

## Consequences
- Warnings (e.g. deprecations) do not fail the gate; only ERR-level changes do.
- The gate cannot know about clients outside this repo; it assumes every published operation has clients.
