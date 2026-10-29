---
name: review-security
description: Read-only security review of the whole repository for what the security scanners cannot see (auth seams, trust in client input, error detail, CI token scope). Advisory findings as file:line — problem — fix. Use for the weekly scheduled review (drift.yml) or when asked for a security review of the codebase rather than of one diff.
---

# Security review

A read-only review of the repository as it is, not of one diff: read files only; change, create or run
nothing. Its findings are advisory (ADR 0019): a human decides; a finding class accepted twice becomes a
computational rule. Every path below is relative to the repository root.

## Skip

- What a sensor already catches on every PR (`docs/harness/rules.md`): `go-sec` (gosec), `repo-no-secrets`,
  `deps-no-known-vulns`, `go-no-known-vulns`, `api-errors-problem-json`, `go-structured-logging`,
  `ci-workflows-valid`.
- Generated code and fixtures: `scripts/harness/fixtures/` (deliberately broken examples), `.specify/`,
  `*.gen.go`, `apps/api/internal/*/store/` and `packages/api-client/src/gen/`.

## Look for

1. **Authentication and authorisation seam.** `apps/api/internal/platform/auth` is a development stub until
   the "Before production" list in `docs/SECURITY.md` is done. Find handlers that skip it, that decide access
   from client-supplied data (a user id in the body or path without a check against the caller), or that
   would stay open when the stub is replaced.
2. **Trust in client input.** Headers the server takes at face value (`traceparent`, `X-Request-Id`,
   `X-Forwarded-*`), values used in SQL outside sqlc, in file paths, in redirects or in log fields that
   operators act on. Input is parsed where it enters (golden principle 2); report inner code that re-parses
   or trusts raw strings.
3. **Error detail.** `problem+json` bodies or log lines that carry stack traces, SQL, internal hostnames or
   wrapped errors from dependencies to the client.
4. **Transport and defaults.** Timeouts, body size limits, CORS, cookie flags and TLS assumptions that
   `docs/SECURITY.md` lists as in place: check that the code still does what the doc says.
5. **CI and agents.** Workflows in `.github/workflows/` with broader `permissions` than their steps need, a
   token in a step that runs third-party code, `${{ }}` expressions from untrusted input inside `run:`,
   actions not pinned by SHA, `pull_request_target` with a checkout of the PR head.

## Never

- Never read `.env`, `.env.local` or any other env file, and never print a secret you come across: name the
  file and line only. The configuration's shape is in `apps/api/internal/platform/config` and
  `compose.yaml`.
- Never run code, start services or reach the network.

## Output

One line per finding, most severe first, at most 15:

`path:line — problem — fix` (severity high, medium or low)

Report only what a reader can verify at that line. No finding is a valid result; say so in one line.
