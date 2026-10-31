## What and why

<!-- One paragraph. Link the spec (specs/NNN-…/) for a product feature, or the exec plan
     (docs/exec-plans/active/…) for harness/infra work. The PR title is a Conventional Commit of at most 90 characters. -->

## Assumptions

<!-- Each default this change picks without anyone deciding it: a timeout, cap, limit, size, interval,
     retry count, threshold, a name others will depend on (service, metric, label, env var), a fallback for
     missing input, or a mode set in config (log level, isolation level, retry policy). One line each:
     `path — value — why this value` (the reviewer matches by path and value; line numbers move on push).
     No line needed for a value a code comment next to it explains, a spec, ADR or plan states, or a name
     declared in api/openapi.yaml or a migration; nor for tests, generated code, pinned versions or values
     the diff only moves. Write "none" if there are none. The code-reviewer reports added defaults this
     section does not list (advisory). -->

## Checklist

- [ ] **Contract change** (`api/openapi.yaml`): Y / N
      If Y: `make gen` output is committed; breaking changes are intentional and versioned (contract-gate).
- [ ] **Migration** (`apps/api/db/migrations/`): Y / N
      If Y: new file only, expand/migrate/contract, `make gen` output committed (migration-gate).
- [ ] **Harness change** (see `scripts/harness/protected-paths.txt`): Y / N
      If Y, rationale: <!-- which rule is wrong or missing, and why the change does not weaken it -->
- [ ] **Spec** (`specs/NNN-…/`, spec-kit): <!-- link, or "not a feature" / no-spec-needed rationale -->
- [ ] **ADR**: <!-- link to docs/adr/NNNN-….md, or "none needed" -->
- [ ] `make check-all` is green locally.
