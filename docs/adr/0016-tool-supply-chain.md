# ADR 0016: Downloaded tools are sha256-pinned; scanners run only as pinned release binaries

Status: accepted · 2026-10-28

## Context
[ADR 0001](0001-tool-pinning-and-single-entry-point.md) pins every tool's version in
`scripts/tool-versions.env` and installs it into `.bin/`. A version pin is not an integrity pin. Go tools come
through `go install <module>@<version>`, which the Go checksum database verifies. The downloaded binaries
(`atlas`, `shellcheck`) were fetched with `curl` and installed unchecked: a re-tagged release would have been
installed silently.

Exec plan 0003 (`docs/exec-plans/*/0003-harness-gap-closure.md`, Q1) adds Trivy for `repo-no-secrets` and
`deps-no-known-vulns`. In March 2026 Trivy's release pipeline was compromised (CVE-2026-33634, in CISA KEV):
v0.69.4 was a malicious release, most `aquasecurity/trivy-action` tags and all `setup-trivy` tags were
force-pushed to credential-stealing code, and on 2026-03-22 the Docker Hub images v0.69.5 and v0.69.6 were
malicious too. The alternative was gitleaks for secrets plus `pnpm audit` for JS vulns, with no scanner for
the nested `go.mod`s `govulncheck` does not reach.

## Decision
- **Trivy over gitleaks + `pnpm audit`**, for breadth: one tool scans secrets, JS dependencies and every
  `go.mod`. We accept its compromise history and contain it with the rules below.
- **Every downloaded binary is sha256-pinned per OS/arch** in `tool-versions.env`
  (`<TOOL>_SHA256_<OS>_<ARCH>`). `scripts/install-tools.sh` verifies the download before installing it and
  fails closed: no pin for this platform, a failed download or a mismatch installs nothing and removes the
  partial file.
- **Scanners run only as pinned release binaries from `.bin/`**, locally and in CI (`make setup`, then
  `.bin/trivy`). Never `trivy-action`, `setup-trivy` or a Trivy container image.
- **CI agent runtimes are sha256-pinned native binaries too**, installed only by an explicit
  `install-tools.sh <tool>` mode (today `install-tools.sh claude`, for `pr-review.yml` and the scheduled
  reviews in `drift.yml`, exec plan 0003 P3.2 and P3.3): never by `make tools`, which every worktree runs,
  never by a marketplace action and never from npm. Every CI agent runs that binary in a `run:` step under
  the one contract in [ADR 0019](0019-inferential-sensors-are-advisory.md) (`scripts/harness/ci-agent.sh`).
  Claude Code's provenance check compares the pins with the sha256 values in the release's `manifest.json`
  (fetched over TLS from `downloads.claude.ai`); its signature is not verified.
- **Every version bump needs a human provenance check**, recorded in the PR: the new pins against upstream's
  checksums (Trivy: the release's `checksums.txt` and its cosign signature; Atlas: ariga's `.sha256` files;
  ShellCheck publishes none, so the GitHub asset digests). No cosign at install time: it would need its own
  verified bootstrap. The main defence stays the age rule: adopt a release only once it is at least 14 days
  old, and never v0.69.4 (v0.76.0 was the newest such release on 2026-10-28).
- **The vuln DB comes from `ghcr.io/aquasecurity/trivy-db:2`**, Aqua's own registry, not the default first
  choice `mirror.gcr.io` (a Docker Hub mirror). It is a mutable tag: an accepted risk, since the DB is data
  (advisories), not code, and a wrong DB changes a `check-all` verdict, not what runs.
- **A new enforcement point, the `pre-commit` git hook**, runs the same secret scan on the index
  (`trivy.sh secret --staged`) before a commit exists. `--no-verify` skips it; CI is the backstop.

## Consequences
- A re-tagged or tampered release fails `make setup` instead of running in every worktree and CI job.
- Bumping Trivy, Atlas or ShellCheck is a harness PR (`tool-versions.env` is protected, kind `config`) with
  four new pins and a provenance note; a bump cannot be a one-line version change.
- CI pulls nothing from the compromised channels; the release binary is the only Trivy code that runs.
- **Known gap:** pnpm is bootstrapped with `npm install pnpm@<version>` into `.bin/.pnpm` without a pinned
  integrity hash (tech-debt tracker). JS packages themselves are covered by `pnpm-lock.yaml` integrity and
  pnpm's `minimumReleaseAge`.
- Playwright's browser for `make e2e` has no sha256 pin: a known gap recorded in
  [ADR 0025](0025-browser-e2e-is-the-nfr-b2-sensor.md).
- If Trivy is compromised again past the age window, the pins still hold the old binary; the next bump's
  provenance check is where it must be caught.
