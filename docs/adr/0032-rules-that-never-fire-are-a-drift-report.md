# ADR 0032: Rules that never fire are a drift report; retirement is a human PR (amends ADR 0013)

Status: proposed · 2026-10-29

Amends [ADR 0013](0013-harness-nfr-budgets.md): `.cache/harness/timings.jsonl` and ci.yml's
`harness-timings-check-all` artifact, kept for the latency report, now also carry each rule's finding count
for this report. A change to their row shape must keep both readers working.

## Context
Slice P4.4 of exec plan 0003 (`docs/exec-plans/*/0003-harness-gap-closure.md`, "rippable harness"): rules are
added when they are earned ([rules.md](../harness/rules.md)), but nothing shows when one stops earning its
place. A rule whose sensor has not found anything in months still costs time on every `make check`, a
fixture and a rules.md entry for agents to read. It may also guard a mistake that agents stopped making
because the rule exists. Only a person can tell these cases apart. Until now the harness kept no record of
which rule found what: `.cache/harness/timings.jsonl` had one row per sensor run with its exit status, and
one sensor checks several rules.

ci.yml keeps the timing rows of each `check-all` run for 8 days (the `harness-timings-check-all` artifact),
far shorter than a 90-day window, and the rows come from PR runs, so their content is not trusted.

## Decision
- **`run.sh` counts findings per rule.** Each timing row gets a `"findings"` object with one count per rule
  id the sensor was given (`timing.sh` appends it after the fields `nfr-report.sh` reads). The count follows
  the remediation that `run.sh` prints: 0 on success; on failure, for each rule whose remediation prints, the
  output lines naming it (or holding a finding mapped to it by `finding_pattern`,
  [ADR 0024](0024-size-and-complexity-limits-are-maintainability-sensors.md)), at least 1, so a rule whose tool
  names no rule (a failing test, a gofmt diff) still counts as fired. Every other rule counts 0. Only "more
  than 0" is used; the number itself is a hint, not a measure.
- **The drift report lists rules that never fired in 90 days.** `scripts/harness/rule-fires.sh` runs weekly in
  `drift.yml`. It merges ci.yml's timing artifacts (`scripts/harness/ci-timings.sh` lists them once and
  downloads each by id; the retention is read from ci.yml, its one source) into a ledger, one line per rule of
  rules.md: since when it is tracked, the last day it ran, the last day it fired, and the runs without a
  finding since then. The ledger is carried from one scheduled drift run to the next as the `rule-fires`
  artifact (90 days), like the coverage baseline ([ADR 0020](0020-test-coverage-is-a-trend.md)). The `drift`
  issue lists every rule tracked for the whole window that ran in it and fired never or only before it.
  Rules with no run in the window are named separately as not measured, never as candidates.
- **Each artifact is merged once.** The ledger keeps the id of every artifact it merged until that artifact
  has certainly expired (retention + 1 day). So artifacts downloaded again the next week never count twice,
  and one uploaded after a merge (its run was still going) is merged the next week. An artifact id, not a
  run id: a re-run attempt uploads a new artifact for the same run.
- **A bad week never resets the history.** Tracking starts fresh only when the list of earlier scheduled
  drift runs worked and none has a `rule-fires` artifact, asked apart from downloading it. A gh error while
  listing or downloading the ledger or the timings, or a failing `rule-fires.sh`, merges nothing and uploads
  no ledger, so the last one stays the newest; the issue says so.
- **A human decides; nothing retires a rule automatically.** For each listed rule a person decides to keep
  it or retire it in a harness PR that removes the rules.md entry, its sensor and its fixture. A rule that
  never fires is not proof that it is useless.
- **A report, not a sensor**, like coverage and mutation testing ([ADR 0023](0023-mutation-testing-is-a-drift-report.md)):
  no rule id, no fixture, no budget row, in neither `make check` (it reads the clock and the network) nor
  `make check-all`.
- **Rows are untrusted.** `rule-fires.sh` merges only rows of `timing.sh`'s exact shape, with known rule ids
  and integer counts, and drops a row that starts after the merge, and every symlink. A ledger that does not
  parse fails the step's section in the issue. "merged N rows from M artifacts" goes to the log, and the
  issue says when new artifacts gave no row at all (writer and reader disagree); a bats test feeds a row
  written by the real `run.sh` through both `nfr-report.sh` and `rule-fires.sh`.
- **The reports stay in bash**, despite [ADR 0012](0012-parsing-sensors-in-go.md) (parsing moves to
  `.bin/harness`): `rule-fires.sh` and `ci-timings.sh` parse only `timing.sh`'s fixed-format rows and their
  own ledger, as `nfr-report.sh` does, and run weekly outside `make check`. If they cause defects, they move
  to a `.bin/harness` subcommand.
- **rules.md headings are read a fourth time.** `fixtureclaims.go`, `integritycheck.go` and `run.sh` read the
  `### <rule-id>` headings too. `rule-fires.sh` takes every `### ` heading, as `fixtureclaims.go` does, and
  fails closed on one that is not a rule id, so a heading it cannot parse never drops a rule silently.

## Consequences
- Nothing is listed during the first 90 days of tracking, or for 90 days after the ledger is lost (no
  scheduled drift run for longer than its retention). A rule added later is tracked from its first day.
- Only `check-all` in ci.yml counts. Fires in `make check`, the Claude hooks, `pre-commit` or an agent's local
  run never reach the ledger, nor do the sensors with their own workflow (`contract-check`, `harness-integrity`,
  `commits`, `specs`): those rules read as not measured
  ([TD-79, TD-80](../exec-plans/tech-debt-tracker.md)). A rule that agents trip and fix locally before
  pushing can be listed. The person deciding needs to know that.
- A run that crashes before any finding (tool missing, config error) counts as a fire of every rule its
  remediation fallback prints ([TD-81](../exec-plans/tech-debt-tracker.md)).
- `ci-timings.sh` drops artifacts of fork PR runs and extracts only `timings.jsonl`. Shape validation does
  not make the remaining rows trustworthy: a same-repository PR can still write a well-formed forged row with
  a count above 0 and hide a rule from the report for 90 days. The report errs toward listing too few rules, never toward
  retiring one; the person deciding can check the rule's sensor runs before keeping it.
- When no scheduled drift run happens for longer than the timings retention, rows in between are lost; the
  report says so. A week whose ledger or timings could not be read is skipped, and its rows are merged the
  next week while their artifacts still exist.
