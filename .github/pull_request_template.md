## What and why

<!-- One paragraph. Link the spec (specs/NNN-…/) for a product feature, or the exec plan
     (docs/exec-plans/active/…) for harness/infra work. The PR title is a Conventional Commit of at most 90 characters. -->

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
