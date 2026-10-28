# Security

Defaults this template ships with, and what a product built on it must do before production.

## In place

- **Auth seam, fail closed.** `platform/auth` puts a `Principal` in every request context. The only mode is
  `AUTH_MODE=dev-stub` (trusts any bearer token, local development only); any other value refuses to start.
- **Input validation** against the OpenAPI contract in the generated strict server; responses parsed with zod
  in the web client.
- **No secrets in the repo.** `.env*` is gitignored; `.env.local` is generated per worktree with local-only
  credentials. Agents are denied reading `.env*` (`.claude/settings.json`). Rule `repo-no-secrets` (Trivy's
  built-in secret rules, offline, with its path allow-list off so docs and tests count too) scans the
  git-known files (in `make check` the changed ones, in CI all of them), and the `pre-commit` git hook (installed by `make setup`) scans the
  staged files before a commit exists.
- **Downloaded tools are verified:** each binary `scripts/install-tools.sh` downloads (Trivy, Atlas,
  ShellCheck) is checked against a sha256 pinned per OS/arch in `scripts/tool-versions.env` and not installed
  on a mismatch or a missing pin; scanners run only as these release binaries, never as GitHub Actions or
  container images ([ADR 0016](adr/0016-tool-supply-chain.md)).
- **Dependency vulnerabilities:** `govulncheck` (root Go module) and Trivy (`pnpm-lock.yaml` and every
  `go.mod`, rule `deps-no-known-vulns`) in `make check-all`, CI and weekly drift; JS installs honour
  pnpm's `minimumReleaseAge: 1440` (24 h, `pnpm-workspace.yaml`). It is not strict: when no version that old
  matches, pnpm installs a newer one and records it in `minimumReleaseAgeExclude`, which a PR review must
  check.
- **SAST:** gosec in golangci-lint (rule `go-sec`, `make lint-go`, so `make check`, the per-edit hook and
  CI), tests included, generated code excluded; a false positive is suppressed on its line only with a reason.
- **Errors** never expose internals: unexpected errors are a generic 500 problem; details go to logs.
- **Postgres** binds to `127.0.0.1` only.

## Before production

- Implement a real verifier in `platform/auth` (JWT signature, issuer, audience, expiry, or introspection)
  and add authorization per operation; declare `securitySchemes` in `api/openapi.yaml`.
- Secrets from a secret manager, not environment files; TLS termination; rate limiting.
- Add a threat model as a design doc and an ADR for the auth choice.
- Stop trusting client correlation keys at the edge: today any caller's `traceparent` (trace ID and sampled
  flag) and valid `X-Request-Id` are kept. Use `otelhttp.WithPublicEndpointFn` so an outside parent becomes
  a span link, and generate request IDs for untrusted callers.
