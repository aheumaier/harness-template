#!/usr/bin/env bash
# Trivy sensors (exec plan 0003, Q1), config in trivy.yaml and scripts/harness/trivy-secret.yaml:
#
#   scripts/harness/trivy.sh secret   repo-no-secrets: offline, rule-based (make check)
#   scripts/harness/trivy.sh secret --staged
#                                     the same scan of the staged files as they are in the index, not in the
#                                     working tree (the pre-commit git hook, exec plan 0003 Q2)
#   scripts/harness/trivy.sh vuln    deps-no-known-vulns: every go.mod and pnpm-lock.yaml against the vuln DB,
#                                     fetched into .cache/trivy (make check-all, CI, drift.yml)
#
# Trivy walks the filesystem, not git, so both scan a temp copy of the git-known files (tracked plus untracked,
# not ignored): never .env.local, .cache/ or node_modules. `secret` in HARNESS_SCOPE=changed copies only the
# files changed since the merge base, and everything when a harness file changed (it can change the verdict on
# unchanged files) or the change set cannot be computed; in any other scope everything. `--staged` ignores the
# scope and copies the index version of each staged file; trivy.yaml and trivy-secret.yaml are still read from
# the working tree, as in `make check`. Prints
# `file:line: … [rule-id]` per finding, sorted, and exits 1; silent and exit 0 when clean; exit 2 when Trivy
# itself failed to run (its fatal errors share exit 1 with findings unless --exit-code says otherwise).
set -uo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT" || exit 1
mode="${1:-}"
staged=0
case "$mode ${2:-}" in
  "secret ") rule=repo-no-secrets ;;
  "secret --staged") rule=repo-no-secrets staged=1 ;;
  "vuln ") rule=deps-no-known-vulns ;;
  *)
    echo "usage: trivy.sh secret [--staged] | vuln" >&2
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
  if ((staged)); then # added, copied, modified, renamed or type-changed; GIT_INDEX_FILE is git commit's index
    git diff -z --cached --name-only --no-renames --diff-filter=ACMRT --
  elif [[ "$mode" == secret ]] && scripts/harness/unchanged.sh trivy.yaml &&
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
copy() {
  if ((staged)); then # the index's blobs, never the working tree's files
    if [[ -s "$list" ]]; then git checkout-index -z --stdin --prefix="$tree/" <"$list"; fi
    return
  fi
  # Deleted files are in the diff; copy only paths that exist.
  while IFS= read -r -d '' f; do
    if [[ -e "$f" ]]; then printf '%s\0' "$f"; fi # `[[ ]] && …` would fail the loop on a deleted last path
  done <"$list" |
    tar -cf - --null -T - | tar -xf - -C "$tree"
}
copy || {
  echo "could not copy the files to scan [$rule]" >&2
  exit 1
}

# TRIVY_* variables override trivy.yaml and the flags' defaults (skip dirs, severity, ignore file): the verdict
# must depend on the tree only (NFR-R1). CI also exports TRIVY_VERSION and the TRIVY_SHA256_* pins.
for v in $(compgen -v TRIVY_); do unset "$v"; done
# --ignorefile '': no .trivyignore can waive a finding; a waiver is a reviewed change to this script.
# --exit-code 3: Trivy also exits 1 on a fatal error, so findings get a code of their own and a crash cannot
# pass for a finding (whose remediation is to edit the files).
common=(fs --quiet --config "$ROOT/trivy.yaml" --cache-dir "$ROOT/.cache/trivy" --ignorefile ''
  --skip-version-check --disable-telemetry --exit-code 3 --format template)
verdict() { # trivy's exit status → status: 0 clean, 3 findings (→ 1); anything else exits 2 (fail closed)
  case "$1" in
    0) status=0 ;;
    3) status=1 ;;
    *)
      [[ -z "$out" ]] || printf '%s\n' "$out" >&2
      echo "trivy failed to run (exit $1) [$rule]" >&2
      exit 2
      ;;
  esac
}
has_packages() { # a go.mod with a require directive, or a pnpm-lock.yaml with a non-empty packages: section
  case "$1" in
    *go.mod) grep -Eq '^require[[:space:](]' "$1" ;;
    *) awk '/^packages:/ { p = 1; next } p && /^[^[:space:]]/ { exit } p && /^[[:space:]]+[^[:space:]]/ { f = 1; exit }
      END { exit !f }' "$1" ;;
  esac
}
if [[ "$mode" == secret ]]; then
  # No network: no DB, checks bundle, VEX repo, version check or telemetry (exec plan 0003, A2).
  # Built-in rules only, with Trivy's path allow-list (tests, examples, *.md, …) off: trivy-secret.yaml.
  # --cache-backend memory: no scan cache on disk, so it never contends for .cache/trivy with vuln mode or a
  # pre-commit hook running at the same time.
  out="$("$trivy" "${common[@]}" --scanners secret --cache-backend memory --offline-scan --skip-db-update \
    --skip-check-update --skip-vex-repo-update --secret-config "$ROOT/scripts/harness/trivy-secret.yaml" \
    --template '{{range .}}{{$t := .Target}}{{range .Secrets}}{{$t}}:{{.StartLine}}: {{.Severity}} secret: {{.Title}} (trivy {{.RuleID}}) [repo-no-secrets]{{"\n"}}{{end}}{{end}}' \
    "$tree")"
  verdict $?
else
  # --include-dev-deps: the JS toolchain (eslint, vite, orval, …) runs on every dev machine and in CI.
  # --db-repository: Aqua's own GHCR, not the default first choice mirror.gcr.io (a Docker Hub mirror).
  # --skip-dirs: the fixtures' overlays are deliberate violations, not dependencies anyone installs.
  # Each `#target` line names a file Trivy parsed packages from.
  out="$("$trivy" "${common[@]}" --scanners vuln --include-dev-deps --db-repository ghcr.io/aquasecurity/trivy-db:2 \
    --skip-dirs scripts/harness/fixtures \
    --template '{{range .}}{{$t := .Target}}#target {{$t}}{{"\n"}}{{range .Vulnerabilities}}{{$t}}: {{.PkgName}} {{.InstalledVersion}}: {{.VulnerabilityID}} ({{.Severity}}), fixed in {{if .FixedVersion}}{{.FixedVersion}}{{else}}no release yet{{end}} [deps-no-known-vulns]{{"\n"}}{{end}}{{end}}' \
    "$tree")"
  verdict $? # a failed run exits here: no "found no packages" lines for a scan that never ran
  # --quiet hides parse warnings: a lockfile or go.mod Trivy cannot read yields no packages and no finding.
  # Fail closed on every git-known one (fixtures aside) that declares packages but is not a target (Trivy
  # reports no target for a go.mod without requires or a lockfile without packages).
  targets="$(printf '%s\n' "$out" | sed -n 's/^#target //p')"
  while IFS= read -r -d '' f; do
    case "$f" in
      scripts/harness/fixtures/*) ;;
      pnpm-lock.yaml | go.mod | */go.mod)
        if [[ -e "$f" ]] && has_packages "$f" && ! grep -qxF "$f" <<<"$targets"; then
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
