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
  ShellCheck, jq, spectral, squawk, bats, and Claude Code for CI only via `install-tools.sh claude`) is checked against a sha256
  pinned per OS/arch in `scripts/tool-versions.env` and not installed on a mismatch or a missing pin;
  scanners and CI agents run only as these release binaries, never as GitHub Actions, npm packages or
  container images ([ADR 0016](adr/0016-tool-supply-chain.md)).
- **A self-hosted Mac runs branch code.** While a Mac runner is registered to the repo, anyone who can push a
  branch can run code on it, whether or not the repo variable `IOS_RUNNER` points `check-ios` at it, and what a
  job leaves there stays for the next job. So it runs as a dedicated standard macOS user with no keychain items,
  SSH keys or personal files, under its own label (`ios-mac`), only for a private repo whose writers are trusted;
  before the repo goes public, unset `IOS_RUNNER` and remove the runner (`./config.sh remove`)
  ([ci.md](harness/ci.md#self-hosted-mac-runner)).
- **The CI PR reviewer's key trusts write-access collaborators.** `pr-review.yml` runs on `pull_request`,
  where GitHub runs the PR's own copy of the workflow with the `ANTHROPIC_API_KEY` secret: anyone who can push
  a branch can read or spend it. It must be a dedicated Anthropic workspace key with a hard spend limit. The
  alternative `CLAUDE_CODE_OAUTH_TOKEN` secret is the owner's subscription token: exposed the same way as the
  key, with no spend limit, valid for a year and revocable by the owner alone ([ADR 0040](adr/0040-ci-agents-may-use-a-subscription-token.md)); an
  `environment:` with required reviewers is optional hardening. Forks get no secrets. Against PR content
  (prompt injection) the agent has read-only file tools confined to the base checkout and no shell, web or
  MCP tool; a verbatim screen of its output is a backstop, not the control
  ([ci.md](harness/ci.md#pr-review), [ADR 0019](adr/0019-inferential-sensors-are-advisory.md)).
- **Dependency vulnerabilities:** `govulncheck` (root Go module) and Trivy (`pnpm-lock.yaml` and every
  `go.mod`, rule `deps-no-known-vulns`) in `make check-all`, CI and weekly drift. What the spectral (a
  Node runtime and its npm tree), squawk and bats downloads bundle is in no lockfile and not scanned; their
  bumps need the provenance note ([ADR 0033](adr/0033-stack-profiles.md), tracker TD-86). JS installs honour
  pnpm's `minimumReleaseAge: 1440` (24 h, `pnpm-workspace.yaml`). It is not strict: when no version that old
  matches, pnpm installs a newer one and records it in `minimumReleaseAgeExclude`, which a PR review must
  check.
- **SAST:** gosec in golangci-lint (rule `go-sec`, `make lint-go`, so `make check`, the per-edit hook and
  CI), tests included, generated code excluded; a false positive is suppressed on its line only with a reason.
- **Errors** never expose internals: unexpected errors are a generic 500 problem; details go to logs.
- **Postgres and the obs stack** bind to `127.0.0.1` only; OTLP has no auth and Grafana keeps `admin`/`admin`
  ([ADR 0017](adr/0017-per-worktree-observability-stack.md)).

## Before production

- Implement a real verifier in `platform/auth` (JWT signature, issuer, audience, expiry, or introspection)
  and add authorization per operation; declare `securitySchemes` in `api/openapi.yaml`.
- Secrets from a secret manager, not environment files; TLS termination; rate limiting.
- Add a threat model as a design doc and an ADR for the auth choice.
- Stop trusting client correlation keys at the edge: today any caller's `traceparent` (trace ID and sampled
  flag) and valid `X-Request-Id` are kept. Use `otelhttp.WithPublicEndpointFn` so an outside parent becomes
  a span link, and generate request IDs for untrusted callers.
