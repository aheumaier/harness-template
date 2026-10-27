# ADR 0012: Sensors that parse move to one Go binary; orchestration stays in bash

Status: proposed · 2026-10-27

## Context
Sensors and hooks are bash (`scripts/harness/*.sh`, `.claude/hooks/*.sh`). Measured warm on macOS
(exec plan 0002, `docs/exec-plans/*/0002-fast-reliable-harness.md`):

- bash itself is not the cost: `run.sh` adds ~16 ms per sensor; the diff-scoped sensors (`check-specs`,
  `check-commits`, `integrity-check`) take 25–51 ms. These scale with the size of a change, not of the repo.
- The PreToolUse hooks take 37 ms (`protect-harness`) and 32 ms (`guard-bash`) per Bash call; ~20 ms of each
  is starting `node` only to read the hook's JSON input.
- Wall time is spent in tools (golangci-lint, go test, tsc, eslint, dependency-cruiser) and in **work that need
  not run** (exec plan 0002, phase 1). The architecture sensors (go-arch-lint, depguard and the custom analyzers,
  dependency-cruiser, eslint boundaries) are Go/Node tools already; rewriting bash does not speed them up.
- The weak spot is reliability where bash *parses* structured input with awk/sed/regex:
  - `guard-bash.sh` tokenizes shell commands with awk (a self-declared heuristic in a security hook);
  - `check-links.sh` approximates GitHub heading slugs (duplicates, Unicode and inline code differ) and forks
    ~6 processes per anchor link: 74 links take ~310 ms, so a 2–4 MLOC monorepo with tens of thousands of
    links takes minutes;
  - `integrity-check.sh` parses unified diffs with awk;
  - `run.sh` works around BSD/GNU `sed -i` differences.

## Decision
- One Go module `apps/api/tools/harness` (nested, like `tools/analyzers`) builds one binary, `.bin/harness`,
  with a subcommand per ported sensor or hook. It uses real parsers: `mvdan.cc/sh/v3/syntax` for shell,
  goldmark for Markdown, a unified-diff parser for diffs, `encoding/json` for hook input.
- Port only what parses, in order of risk: `guard-bash`'s shell parsing together with the JSON input of
  **every** hook (`protect-harness`, `guard-bash`, `fast-check`, `only-make-check` all start `node` for it
  today), then `check-links`, then `integrity-check`. `check-specs` and `check-commits` move only if they cause defects.
- Orchestration stays in bash/make: `run.sh`, `merge-base.sh`, `gen-check.sh`, `setup-worktree.sh`,
  `install-tools.sh`, the Makefile. They call tools; Go adds a build step and nothing else.
- **The module is sensor code:** `apps/api/tools/harness/*` is added to `scripts/harness/protected-paths.txt`
  as `config` and to `.github/CODEOWNERS` in the PR that creates it.
- **Hooks keep a thin bash wrapper** in `.claude/hooks/` (so `.claude/settings.json` does not change) that
  `exec`s `.bin/harness`. A missing binary would exit 127, which Claude Code treats as a non-blocking error,
  so the wrapper checks for it and exits 2 with "run make setup". To avoid a deadlock, the guard-bash wrapper
  lets exactly `make setup` and `make .bin/harness` through when the binary is missing. The wrapper has a
  bats test for both paths.
- **Staleness:** `.bin/harness` is a Make target that depends on its sources (like `$(GCL)` on
  `ANALYZERS_SRC`) and is a prerequisite of `check`, `check-all` and `harness-selftest`, so every run rebuilds
  a stale binary. Hooks only check that it exists; comparing source times on every call would cost NFR-H1
  budget.
- **The output contract does not change:** same rule ids, same `file:line: message [rule-id]` lines, same
  exit codes. Sensors run through `run.sh` still get their remediation from `docs/harness/rules.md`; hooks
  print their own reason as today. Existing fixtures must still be rejected (`make harness-selftest`). A
  ported sensor's bats tests become Go table tests in the same PR, and its old script is deleted; for hooks
  only the parsing moves and the wrapper stays.

## Consequences
- Parsing is correct by construction and unit-testable; no `node` to parse hook input (fast-check still runs
  eslint on `.ts` edits); no BSD/GNU drift.
- `check-links` becomes linear in file size instead of forking per anchor link (needed at monorepo scale).
- Two languages in the harness instead of one; Go is already used for the custom analyzers.
- A fresh clone needs `make setup` before hooks work (already true for `.bin` tools).
- `integrity-check` runs from `pull_request_target` on the base branch's harness (ADR 0010). After the port
  the workflow must build `.bin/harness` from the base-branch checkout and never from the PR head; otherwise a
  PR could ship the binary that judges it.
- Each port is a harness change: human-approved PR, one port per PR.
