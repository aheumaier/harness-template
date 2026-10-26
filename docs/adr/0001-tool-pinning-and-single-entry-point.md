# ADR 0001: Tool pinning in `.bin` and Makefile as single entry point

Status: accepted · 2026-10-27

## Context
Agents, humans and CI must run identical sensors. Globally installed tools drift between machines and
make sensor output non-reproducible. macOS ships GNU make 3.81.

## Decision
- Non-npm tool versions live in `scripts/tool-versions.env`; `scripts/install-tools.sh` installs them into
  `./.bin` with a version stamp (idempotent). npm tools are exact-pinned in `package.json`.
- `GOTOOLCHAIN` is exported from the Makefile, so the local Go version is irrelevant.
- The Makefile is the only documented interface (`make setup|check|check-all|gen|new-domain`).
  It stays compatible with make 3.81 (no `--output-sync`, no reliance on `.SHELLFLAGS`).
- TypeScript is pinned to 6.0.3, not 7.x: typescript-eslint 8.72.1 declares `typescript <6.1.0`.

## Consequences
- First `make setup` is slow (~2 min, compiles Go tools); later runs are no-ops.
- Bumping a tool is a one-line diff in a reviewed file. Harness-integrity checks catch drift.
