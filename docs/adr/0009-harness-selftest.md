# ADR 0009: Sensors are tested with violation fixtures

Status: accepted · 2026-10-27

## Context
A sensor that silently stops firing (config typo, tool upgrade, renamed rule) is worse than none: the guide
still claims the rule holds. Sensors also fail for the wrong reason (e.g. an `atlas.sum` mismatch instead of
the destructive change), which a plain "did it fail" check accepts.

## Decision
- Every seed rule with a deterministic, local sensor has a fixture in `scripts/harness/fixtures/<name>/`
  (`files/` overlay, optional `base/` overlay and `apply.sh`, `fixture.conf` with `RULE`, `TARGET`, `EXPECT`).
- `make harness-selftest` copies the repo into a temp git repo (node_modules/.bin linked), commits each fixture
  on a branch off `main`, runs `make TARGET`, and asserts: non-zero exit, output matches `EXPECT` (the tool's
  own diagnostic), and run.sh printed `RULE`'s remediation from `docs/harness/rules.md`. Runs in CI.
- New rules need a fixture before they merge (docs/harness/rules.md).

## Consequences
- ~20 s for all fixtures; Atlas fixtures need Docker (`SELFTEST_SKIP_DOCKER=1` skips them locally only).
- Rules without a fixture (formatters, tests, vuln DB, git history) are marked so in rules.md.
