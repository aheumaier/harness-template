#!/usr/bin/env bash
# Trivy sensors (exec plan 0003, Q1), config in trivy.yaml and scripts/harness/trivy-secret.yaml:
#
#   scripts/harness/trivy.sh secret   repo-no-secrets: offline, rule-based (make check)
#   scripts/harness/trivy.sh vuln     deps-no-known-vulns: every go.mod and pnpm-lock.yaml against the vuln DB,
#                                     fetched into .cache/trivy (make check-all, CI, drift.yml)
#
# Trivy walks the filesystem, not git, so both scan a temp copy of the git-known files (tracked plus untracked,
# not ignored): never .env.local, .cache/ or node_modules. `secret` in HARNESS_SCOPE=changed copies only the
# files changed since the merge base, and everything when a harness file changed (it can change the verdict on
# unchanged files) or the change set cannot be computed; in any other scope everything. Prints
# `file:line: … [rule-id]` per finding, sorted; silent and exit 0 when clean.
set -uo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT" || exit 1
mode="${1:-}"
case "$mode" in
  secret) rule=repo-no-secrets ;;
  vuln) rule=deps-no-known-vulns ;;
  *)
    echo "usage: trivy.sh secret|vuln" >&2
    exit 2
    ;;
esac
# shellcheck source=../tool-versions.env
source scripts/tool-versions.env
trivy="$ROOT/.bin/trivy"
[[ -x "$trivy" && "$(cat "$ROOT/.bin/.trivy.version" 2>/dev/null)" == "$TRIVY_VERSION" ]] || {
  echo "trivy $TRIVY_VERSION is not installed in .bin: run make setup [$rule]" >&2
  exit 1
}

files() { # NUL-separated paths to scan
  local base
  if [[ "$mode" == secret ]] && scripts/harness/unchanged.sh trivy.yaml &&
    base="$(scripts/harness/merge-base.sh 2>/dev/null)"; then
    git diff -z --name-only --no-renames "$base" -- && git ls-files -z -o --exclude-standard
  else
    git ls-files -z -co --exclude-standard
  fi
}

# Outside the repo: a copy inside it would be walked by eslint & co. running in parallel (make -j4).
tmp="${TMPDIR:-/tmp}"
tree="$(mktemp -d "${tmp%/}/trivy-$mode.XXXXXX")" || exit 1
list="$tree.list"
trap 'rm -rf "$tree" "$list"' EXIT
files >"$list" || {
  echo "git could not list the files to scan [$rule]" >&2
  exit 1
}
# Deleted files are in the diff; copy only paths that exist.
while IFS= read -r -d '' f; do
  if [[ -e "$f" ]]; then printf '%s\0' "$f"; fi # `[[ ]] && …` would fail the loop on a deleted last path
done <"$list" |
  tar -cf - --null -T - | tar -xf - -C "$tree" || {
  echo "could not copy the files to scan [$rule]" >&2
  exit 1
}

# TRIVY_* variables override trivy.yaml and the flags' defaults (skip dirs, severity, ignore file): the verdict
# must depend on the tree only (NFR-R1). CI also exports TRIVY_VERSION and the TRIVY_SHA256_* pins.
for v in $(compgen -v TRIVY_); do unset "$v"; done
# --ignorefile '': no .trivyignore can waive a finding; a waiver is a reviewed change to this script.
common=(fs --quiet --config "$ROOT/trivy.yaml" --cache-dir "$ROOT/.cache/trivy" --ignorefile ''
  --skip-version-check --disable-telemetry --exit-code 1 --format template)
if [[ "$mode" == secret ]]; then
  # No network: no DB, checks bundle, VEX repo, version check or telemetry (exec plan 0003, A2).
  # Built-in rules only, with Trivy's path allow-list (tests, examples, *.md, …) off: trivy-secret.yaml.
  out="$("$trivy" "${common[@]}" --scanners secret --offline-scan --skip-db-update --skip-check-update \
    --skip-vex-repo-update --secret-config "$ROOT/scripts/harness/trivy-secret.yaml" \
    --template '{{range .}}{{$t := .Target}}{{range .Secrets}}{{$t}}:{{.StartLine}}: {{.Severity}} secret: {{.Title}} (trivy {{.RuleID}}) [repo-no-secrets]{{"\n"}}{{end}}{{end}}' \
    "$tree")"
  status=$?
else
  # --include-dev-deps: the JS toolchain (eslint, vite, orval, …) runs on every dev machine and in CI.
  # --db-repository: Aqua's own GHCR, not the default first choice mirror.gcr.io (a Docker Hub mirror).
  # Each `#target` line names a file Trivy parsed packages from.
  out="$("$trivy" "${common[@]}" --scanners vuln --include-dev-deps --db-repository ghcr.io/aquasecurity/trivy-db:2 \
    --template '{{range .}}{{$t := .Target}}#target {{$t}}{{"\n"}}{{range .Vulnerabilities}}{{$t}}: {{.PkgName}} {{.InstalledVersion}}: {{.VulnerabilityID}} ({{.Severity}}), fixed in {{if .FixedVersion}}{{.FixedVersion}}{{else}}no release yet{{end}} [deps-no-known-vulns]{{"\n"}}{{end}}{{end}}' \
    "$tree")"
  status=$?
  # --quiet hides parse warnings: a lockfile or go.mod Trivy cannot read yields no packages and no finding.
  # Fail closed on every git-known one (fixtures aside) that is not a target.
  targets="$(printf '%s\n' "$out" | sed -n 's/^#target //p')"
  while IFS= read -r -d '' f; do
    case "$f" in
      scripts/harness/fixtures/*) ;;
      pnpm-lock.yaml | go.mod | */go.mod)
        if [[ -e "$f" ]] && ! grep -qxF "$f" <<<"$targets"; then
          echo "$f:1: Trivy found no packages in it (unsupported format?) [deps-no-known-vulns]"
          status=1
        fi
        ;;
    esac
  done <"$list"
  out="$(printf '%s\n' "$out" | grep -v '^#target ')"
fi
[[ -z "$out" ]] || printf '%s\n' "$out" | LC_ALL=C sort -u
exit "$status"
