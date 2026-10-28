# Security

Defaults this template ships with, and what a product built on it must do before production.

## In place

- **Auth seam, fail closed.** `platform/auth` puts a `Principal` in every request context. The only mode is
  `AUTH_MODE=dev-stub` (trusts any bearer token, local development only); any other value refuses to start.
- **Input validation** against the OpenAPI contract in the generated strict server; responses parsed with zod
  in the web client.
- **No secrets in the repo.** `.env*` is gitignored; `.env.local` is generated per worktree with local-only
  credentials. Agents are denied reading `.env*` (`.claude/settings.json`).
- **Dependency vulnerabilities:** `govulncheck` (root Go module) and Trivy (`pnpm-lock.yaml` and every
  `go.mod`, rule `deps-no-known-vulns`) in `make check-all`, CI and weekly drift; JS installs honour
  pnpm's `minimumReleaseAge: 1440` (24 h, `pnpm-workspace.yaml`). It is not strict: when no version that old
  matches, pnpm installs a newer one and records it in `minimumReleaseAgeExclude`, which a PR review must
  check.
- **Errors** never expose internals: unexpected errors are a generic 500 problem; details go to logs.
- **Postgres** binds to `127.0.0.1` only.

## Before production

- Implement a real verifier in `platform/auth` (JWT signature, issuer, audience, expiry, or introspection)
  and add authorization per operation; declare `securitySchemes` in `api/openapi.yaml`.
- Secrets from a secret manager, not environment files; TLS termination; rate limiting.
- Add a threat model as a design doc and an ADR for the auth choice.
