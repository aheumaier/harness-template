# ADR 0028: Quality grades are kept per domain and layer; check-all gates their shape, the weekly drift run their age

Status: accepted · 2026-10-29

## Context
Exec plan 0003 (P3.5) closes a gap to OpenAI's *Harness engineering*: a quality document that grades each
product domain and architectural layer and tracks the gaps over time. [docs/QUALITY.md](../QUALITY.md) had
one grade per domain and nothing that noticed a grade going stale or outliving its domain. Grades are
written by a human reviewer (the `code-reviewer` subagent only suggests grades in its review output and
never writes a row), so no sensor can decide whether a grade is
right; it can decide whether a row exists, parses and was graded recently. [ADR 0015](0015-anticipated-rules-and-nfr-classes-b-o.md)
says every rule beyond its five needs an incident or its own ADR; no incident has earned these yet.

## Decision
- **Shape.** One row per backend domain (`apps/api/internal/<domain>/`, the domains' directory in
  `scripts/harness/layout.env`; `platform` excluded), a grade per
  layer (`model`, `store`, `service`, `api`: `A`–`F`, `–` until graded) and an `Updated` date (`YYYY-MM-DD`,
  the day of the last grading). Cells hold no `|`. One row per domain keeps `make new-domain`'s single
  insertion before the `harness:quality-rows` marker; it writes `–` grades and today's date.
- **The frontend is not graded yet.** `packages/domain`, `packages/api-client` and `apps/web` are layers too,
  but no frontend feature exists to grade; the gap is [TD-56](../exec-plans/tech-debt-tracker.md).
- **Three rules, adopted as `team-convention`** by this ADR, in one sensor, `.bin/harness quality-rows`
  ([ADR 0012](0012-parsing-sensors-in-go.md)), with two modes:
  - deterministic (`-no-clock`, `make docs-quality-shape`): `docs-quality-rows-well-formed` (one cell per
    column, the domain's link, grades `A`–`F` or `–`, `Updated` as `YYYY-MM-DD`) and
    `docs-quality-rows-match-domains` (exactly one row per domain: none missing, none twice, none for a gone
    one);
  - clock (`make docs-quality`): `docs-quality-row-fresh`, `Updated` at most 30 days before today and not
    after it.
- **The deterministic half gates.** It reads only the tree, so it runs in `make check-all` (the CI
  `check-all` job) and in the CI generator probe on the row a fresh `make new-domain` writes. A Go test in
  `make check` renders the generator's row template under the real header and checks both modes accept it.
- **The age check is a weekly drift report, never a gate.** It runs only in `drift.yml`, and its findings go
  to the `drift` issue. Unlike a vulnerable dependency, which any author can bump, a stale grade can only be
  fixed by the domain's reviewer grading it again: in `check-all` or a required job, a 30-day clock would
  turn an unrelated PR red with nothing its author could do. (`make check` reads no clock either; NFR-R1.)
- **agent-gc does not fix these findings until a grading agent exists.** A stale row needs a new grade, a
  judgement only a reviewer of the domain makes today; an agent that only moves the date games the rule. The
  deterministic findings never reach main. `scripts/harness/agent-gc.sh` keeps `RULES="docs-links-resolve"`;
  revisit when an agent can grade a domain's layers. The `code-reviewer` subagent does not change this: it
  suggests grades in its review output and never writes a row, so a stale row stays stale until a human
  grades it.

## Consequences
- Every domain is re-graded at least monthly, or the `drift` issue names its row each week.
- A malformed row, a missing row, a duplicate and a row for a deleted domain turn `check-all` red in the PR
  that causes them.
- `Updated` means "graded", not "edited": the guide and the remediation say never to bump it alone; no sensor
  can tell, so the grade's honesty rests on review.
- "What is a domain" is now defined in the harness binary and in the domain-isolation analyzer
  ([TD-55](../exec-plans/tech-debt-tracker.md)).
- A template with no domains has an empty table and no findings.
- A time-dependent rule whose fixture holds a fixed old date (`2020-01-01`) stays a stable selftest case.
