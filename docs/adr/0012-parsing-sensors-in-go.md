# ADR 0012: Sensors that parse move to one Go binary; orchestration stays in bash

Status: proposed · 2026-10-27

## Context
Sensors and hooks are bash (`scripts/harness/*.sh`, `.claude/hooks/*.sh`). Measured warm on macOS
(exec plan [0002](../exec-plans/active/0002-fast-reliable-harness.md)):

- bash itself is not the cost: `run.sh` adds ~16 ms per sensor; the diff-scoped sensors (`check-specs`,
  `check-commits`, `integrity-check`) take 25–51 ms; the PreToolUse hooks take ~40 ms per Bash call. These scale
  with the size of a change, not of the repo.
- Wall time is spent in tools (golangci-lint, go test, tsc, eslint, dependency-cruiser) and in **work that need
  not run** (exec plan 0002, phase 1). The architecture sensors (go-arch-lint, depguard and the custom analyzers,
  dependency-cruiser, eslint boundaries) are Go/Node tools already; rewriting bash does not speed them up.
- The weak spot is reliability where bash *parses* structured input with awk/sed/regex:
  - `guard-bash.sh` tokenizes shell commands with awk (a self-declared heuristic in a security hook);
  - `check-links.sh` approximates GitHub heading slugs (duplicates, Unicode and inline code differ) and forks
    ~6 processes per link: 74 links take ~310 ms, so a 2–4 MLOC monorepo with tens of thousands of links takes
    minutes;
  - `integrity-check.sh` parses unified diffs with awk;
  - both hooks start `node` only to read their JSON input; `run.sh` works around BSD/GNU `sed -i` differences.

## Decision
- One Go module `apps/api/tools/harness` builds one binary, `.bin/harness`, with a subcommand per ported sensor
  or hook (`harness guard-bash`, `harness check-links`, `harness integrity`). It uses real parsers:
  `mvdan.cc/sh/v3/syntax` for shell, goldmark for Markdown, a unified-diff parser for diffs, `encoding/json` for
  hook input.
- Port only scripts that parse, in order of risk: `guard-bash`, `check-links`, `integrity-check`. Others
  (`protect-harness`, `check-specs`, `check-commits`) move only if they cause defects.
- Orchestration stays in bash/make: `run.sh`, `merge-base.sh`, `gen-check.sh`, `setup-worktree.sh`,
  `install-tools.sh`, the Makefile. They call tools; Go adds a build step and nothing else.
- The output contract does not change: same rule ids, same `file:line: message [rule-id]` lines, remediation
  still printed by `run.sh` from `docs/harness/rules.md`. Existing fixtures must still be rejected
  (`make harness-selftest`); the bats tests of a ported script become its Go table tests in the same PR.
- `.bin/harness` is a Make target that depends on its sources (like `$(GCL)` on `ANALYZERS_SRC`), built by
  `make setup`. A hook that finds the binary missing or older than its sources fails closed with
  "run make setup", so nobody runs a stale sensor.

## Consequences
- Parsing is correct by construction and unit-testable; no `node` per hook call; no BSD/GNU drift.
- `check-links` becomes linear in file size instead of forking per link (needed at monorepo scale).
- Two languages in the harness instead of one; Go is already used for the custom analyzers.
- A fresh clone needs `make setup` before hooks work (already true for `.bin` tools).
- Each port is a harness change: human-approved PR, one script per PR, old script deleted in the same PR.
