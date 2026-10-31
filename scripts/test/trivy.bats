#!/usr/bin/env bats
# scripts/harness/trivy.sh: what it hands Trivy. A stub .bin/trivy lists the files of the tree it is given, so
# these tests pin the input set (git-known files only; the changed ones in scope `changed`) and fail-closed paths.
# The real scanners are exercised by `make secrets` (fixture repo-secret-planted) and `make vuln-deps`.
bats_require_minimum_version 1.5.0
load helpers

setup() {
  sandbox scripts/harness/trivy.sh scripts/harness/unchanged.sh scripts/harness/merge-base.sh \
    scripts/harness/protected-paths.txt scripts/tool-versions.env
  T="$SANDBOX/scripts/harness/trivy.sh"
  mkdir -p "$SANDBOX/.bin" "$SANDBOX/docs"
  # Records its arguments and TRIVY_* environment; the last argument is the tree to scan.
  cat >"$SANDBOX/.bin/trivy" <<'STUB'
#!/usr/bin/env bash
d="$(dirname "$0")"
printf '%s\n' "$@" >"$d/trivy.args"
env | grep '^TRIVY_' >"$d/trivy.env"
cd "${!#}" && find . -type f | sed "s|^\./|scanned: |"
STUB
  chmod +x "$SANDBOX/.bin/trivy"
  sed -n 's/^TRIVY_VERSION=//p' "$REPO/scripts/tool-versions.env" >"$SANDBOX/.bin/.trivy.version"
  printf '.bin/\n.cache/\n.env.local\n' >"$SANDBOX/.gitignore"
  printf 'scan: {}\n' >"$SANDBOX/trivy.yaml"
  echo old >"$SANDBOX/docs/old.md"
  echo gone >"$SANDBOX/docs/gone.md"
  sandbox_base
  git -C "$SANDBOX" checkout -qb feature
  echo 'SECRET=x' >"$SANDBOX/.env.local"
  export BASE_REF=main
}

@test "usage error without a mode, or with --staged on vuln" {
  run -2 "$T"
  has "usage: trivy.sh secret [--staged] | vuln"
  run -2 "$T" vuln --staged
}

@test "secret --staged: scans the index version of the staged files only, whatever the scope" {
  echo staged >"$SANDBOX/docs/old.md"
  echo new >"$SANDBOX/docs/new.md"
  git -C "$SANDBOX" add docs/old.md docs/new.md
  echo 'working tree only' >"$SANDBOX/docs/old.md" # unstaged edit: not what gets committed
  echo untracked >"$SANDBOX/docs/untracked.md"
  git -C "$SANDBOX" rm -q docs/gone.md
  echo 'cat docs/old.md' >>"$SANDBOX/.bin/trivy" # the stub (written by setup, not a link) prints the content
  HARNESS_SCOPE=full run -0 "$T" secret --staged
  [[ "$output" == $'scanned: docs/new.md\nscanned: docs/old.md\nstaged' ]] || {
    echo "$output" >&2
    return 1
  }
}

@test "secret --staged: nothing staged scans nothing" {
  echo edit >>"$SANDBOX/docs/old.md"
  run -0 "$T" secret --staged
  [[ -z "$output" ]]
}

@test "fails closed without the pinned trivy in .bin" {
  rm "$SANDBOX/.bin/.trivy.version"
  run -1 "$T" secret
  has "run make setup [repo-no-secrets]"
}

@test "scope changed: secret scans only changed and untracked files, never ignored ones" {
  echo new >"$SANDBOX/docs/new.md"
  echo edit >>"$SANDBOX/docs/old.md"
  rm "$SANDBOX/docs/gone.md"
  run -0 "$T" secret
  [[ "$output" == $'scanned: docs/new.md\nscanned: docs/old.md' ]] || {
    echo "$output" >&2
    return 1
  }
}

@test "scope changed: a deleted file last in the diff is skipped, not an error" {
  rm "$SANDBOX/docs/old.md"
  run -0 "$T" secret
  [[ -z "$output" ]]
}

@test "scope changed: a harness file change scans everything" {
  echo '# x' >>"$SANDBOX/trivy.yaml"
  run -0 "$T" secret
  has "scanned: docs/gone.md"
  has "scanned: .gitignore"
  [[ "$output" != *env.local* ]] || return 1
}

@test "scope full, vuln mode and no merge base each scan everything git knows" {
  HARNESS_SCOPE=full run -0 "$T" secret
  has "scanned: docs/old.md"
  [[ "$output" != *env.local* ]] || return 1
  run -0 "$T" vuln
  has "scanned: docs/old.md"
  BASE_REF=nope run -0 "$T" secret
  has "scanned: docs/old.md"
}

@test "secret scan: offline, built-in rules without allow-list, no ignore file, no TRIVY_* overrides" {
  TRIVY_SKIP_FILES=docs/old.md HARNESS_SCOPE=full run -0 "$T" secret
  for flag in --offline-scan --skip-db-update --skip-check-update --skip-vex-repo-update --disable-telemetry; do
    grep -qxF -- "$flag" "$SANDBOX/.bin/trivy.args" || return 1
  done
  grep -A1 -xF -- --ignorefile "$SANDBOX/.bin/trivy.args" | sed -n 2p | grep -qx '' || return 1
  grep -A1 -xF -- --secret-config "$SANDBOX/.bin/trivy.args" | grep -q 'scripts/harness/trivy-secret.yaml$' || return 1
  # No scan cache on disk (no contention with vuln mode or the pre-commit hook for .cache/trivy).
  grep -A1 -xF -- --cache-backend "$SANDBOX/.bin/trivy.args" | sed -n 2p | grep -qx memory || return 1
  # Findings exit 3, so Trivy's own fatal exit 1 is not read as a finding.
  grep -A1 -xF -- --exit-code "$SANDBOX/.bin/trivy.args" | sed -n 2p | grep -qx 3 || return 1
  # The secret scan sees the fixtures too: nothing skipped.
  ! grep -qxF -- --skip-dirs "$SANDBOX/.bin/trivy.args" || return 1
  [[ ! -s "$SANDBOX/.bin/trivy.env" ]] || return 1
}

@test "vuln scan: skips only the fixtures dir, findings exit 3" {
  HARNESS_SCOPE=full run -0 "$T" vuln
  grep -A1 -xF -- --skip-dirs "$SANDBOX/.bin/trivy.args" | sed -n 2p | grep -qx scripts/harness/fixtures || return 1
  grep -A1 -xF -- --exit-code "$SANDBOX/.bin/trivy.args" | sed -n 2p | grep -qx 3 || return 1
}

@test "vuln: a go.mod with requires or a lockfile or Package.resolved with packages that Trivy reports nothing for fails closed" {
  mkdir -p "$SANDBOX/m"
  printf 'module m\n\nrequire example.com/x v1.0.0\n' >"$SANDBOX/m/go.mod"
  printf "lockfileVersion: '9.0'\n\npackages:\n\n  x@1.0.0:\n    resolution: {integrity: sha512-x}\n" >"$SANDBOX/pnpm-lock.yaml"
  mkdir -p "$SANDBOX/apps/ios"
  printf '{ "pins": [ { "identity": "x", "location": "https://example.com/x.git" } ], "version": 3 }\n' >"$SANDBOX/apps/ios/Package.resolved"
  mkdir -p "$SANDBOX/apps/old"
  printf '{ "object": { "pins": [ { "package": "x", "repositoryURL": "https://example.com/x.git" } ] }, "version": 1 }\n' >"$SANDBOX/apps/old/Package.resolved"
  run -1 "$T" vuln
  has "m/go.mod:1: Trivy found no packages in it (unsupported format?) [deps-no-known-vulns]"
  has "pnpm-lock.yaml:1: Trivy found no packages in it (unsupported format?) [deps-no-known-vulns]"
  has "apps/ios/Package.resolved:1: Trivy found no packages in it (unsupported format?) [deps-no-known-vulns]"
  has "apps/old/Package.resolved:1: Trivy found no packages in it (unsupported format?) [deps-no-known-vulns]"
}

@test "vuln: a go.mod without requires and a lockfile or Package.resolved without packages pass (nothing to scan)" {
  mkdir -p "$SANDBOX/m"
  printf 'module m\n\ngo 1.27\n' >"$SANDBOX/m/go.mod"
  printf "lockfileVersion: '9.0'\n\nimporters:\n\n  .: {}\n\npackages: {}\n" >"$SANDBOX/pnpm-lock.yaml"
  printf '{ "pins": [], "version": 3 }\n' >"$SANDBOX/Package.resolved"
  run -0 "$T" vuln
  [[ "$output" != *"found no packages"* ]] || return 1
}

@test "trivy failing to run (exit 1, no output) exits 2 with its exit code, in both modes" {
  printf '#!/usr/bin/env bash\nexit 1\n' >"$SANDBOX/.bin/trivy"
  HARNESS_SCOPE=full run -2 "$T" secret
  has "trivy failed to run (exit 1) [repo-no-secrets]"
  # A go.mod that would trip the fail-closed check: a scan that never ran must not report it.
  mkdir -p "$SANDBOX/m"
  printf 'module m\n\nrequire example.com/x v1.0.0\n' >"$SANDBOX/m/go.mod"
  run -2 "$T" vuln
  has "trivy failed to run (exit 1) [deps-no-known-vulns]"
  [[ "$output" != *"found no packages"* ]] || return 1
}

@test "trivy's findings exit (3) is exit 1, with the finding printed" {
  cat >"$SANDBOX/.bin/trivy" <<'STUB'
#!/usr/bin/env bash
echo 'docs/old.md:1: CRITICAL secret: AWS Access Key ID (trivy aws-access-key-id) [repo-no-secrets]'
exit 3
STUB
  HARNESS_SCOPE=full run -1 "$T" secret
  [[ "$output" == 'docs/old.md:1: CRITICAL secret: AWS Access Key ID (trivy aws-access-key-id) [repo-no-secrets]' ]] || {
    echo "$output" >&2
    return 1
  }
}

@test "the scan copy lives outside the repo and is removed afterwards" {
  export TMPDIR="$BATS_TEST_TMPDIR/tmp"
  mkdir -p "$TMPDIR"
  git -C "$SANDBOX" status --porcelain --ignored >"$BATS_TEST_TMPDIR/before"
  HARNESS_SCOPE=full run -0 "$T" secret
  has "scanned: docs/old.md"
  # The stub saw a tree under TMPDIR, not under the repo.
  [[ "$(tail -1 "$SANDBOX/.bin/trivy.args")" == "$TMPDIR"/trivy-secret.* ]] || return 1
  [[ -z "$(ls -A "$TMPDIR")" ]] || return 1
  git -C "$SANDBOX" status --porcelain --ignored >"$BATS_TEST_TMPDIR/after"
  # Only the stub's own records in .bin/ (ignored) may differ.
  diff <(grep -v '^!! .bin/' "$BATS_TEST_TMPDIR/before") <(grep -v '^!! .bin/' "$BATS_TEST_TMPDIR/after") || return 1
}

@test "nothing changed: scans nothing, silent" {
  run -0 "$T" secret
  [[ -z "$output" ]]
}
