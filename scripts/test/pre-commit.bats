#!/usr/bin/env bats
# scripts/git-hooks/pre-commit (scripts/harness/pre-commit.sh), run by a real `git commit` in a sandbox with the
# real pinned tools from .bin: it checks the staged content (the index), never the working tree.
bats_require_minimum_version 1.5.0
load helpers

setup() {
  sandbox scripts/git-hooks/pre-commit scripts/harness/pre-commit.sh scripts/harness/file-sensors.sh \
    scripts/harness/run.sh scripts/harness/timing.sh scripts/harness/trivy.sh scripts/harness/unchanged.sh \
    scripts/harness/merge-base.sh scripts/harness/trivy-secret.yaml scripts/harness/protected-paths.txt \
    scripts/tool-versions.env trivy.yaml .editorconfig .shellcheckrc apps/api/.golangci.yml apps/api/go.mod
  mkdir -p "$SANDBOX/.bin" "$SANDBOX/docs/harness" "$SANDBOX/scripts/lib"
  local t
  for t in trivy .trivy.version golangci-lint-custom shellcheck shfmt harness; do
    ln -s "$REPO/.bin/$t" "$SANDBOX/.bin/$t"
  done
  # run.sh reads the remediations from rules.md; only those lines (its links would not resolve in the sandbox).
  grep -E '^### |\*\*Remediation:\*\*' "$REPO/docs/harness/rules.md" >"$SANDBOX/docs/harness/rules.md"
  printf '.bin/\n.cache/\n' >"$SANDBOX/.gitignore"
  printf '#!/usr/bin/env bash\necho "ok"\n' >"$SANDBOX/scripts/lib/ok.sh"
  sandbox_base
  export TMPDIR="$BATS_TEST_TMPDIR/tmp"
  mkdir -p "$TMPDIR"
}

commit() { # git commit with the sandbox's hooks; git exits 1 whenever the hook fails, whatever its status
  git -C "$SANDBOX" -c core.hooksPath=scripts/git-hooks -c user.name=test -c user.email=test@localhost \
    -c commit.gpgsign=false commit -q -m 'test: commit' "$@"
}
stage() { git -C "$SANDBOX" add "$@"; }
commits() { git -C "$SANDBOX" rev-list --count HEAD; }
# A fake AWS access key, assembled here so no key-shaped string is in the repo (fixtures/repo-secret-planted).
plant_secret() { printf '[default]\naws_access_key_id = %s%s\n' AKIA QYLPMN5HHHFPZAM2 >"$SANDBOX/$1"; }

@test "a staged secret blocks the commit with the rule id and its remediation; the scan copy is removed" {
  plant_secret creds.txt
  stage creds.txt
  run -1 commit
  has "creds.txt:2: CRITICAL secret: AWS Access Key ID"
  has "[repo-no-secrets]"
  has "rule:        repo-no-secrets"
  has "remediation: Remove the secret from the file named"
  has "pre-commit: commit blocked"
  [[ "$(commits)" == 1 && -z "$(ls -A "$TMPDIR")" ]] || return 1
}

@test "clean staged Go, shell and Markdown files commit silently, and the hook's time is recorded" {
  mkdir -p "$SANDBOX/apps/api/x" "$SANDBOX/docs"
  printf 'package x\n\n// X is fine.\nfunc X() int { return 1 }\n' >"$SANDBOX/apps/api/x/x.go"
  printf '#!/usr/bin/env bash\nset -euo pipefail\nif [[ -n "${1:-}" ]]; then\n  echo "$1"\nfi\n' >"$SANDBOX/scripts/lib/new.sh"
  printf '# Doc\n\nSee [ok](../scripts/lib/ok.sh).\n' >"$SANDBOX/docs/doc.md"
  stage apps/api/x/x.go scripts/lib/new.sh docs/doc.md
  run -0 commit
  [[ -z "$output" && "$(commits)" == 2 ]] || return 1
  grep -q '"sensor":"pre-commit".*"exit":0' "$SANDBOX/.cache/harness/timings.jsonl"
}

@test "a secret only in the working tree is not scanned; one staged and then removed from the working tree is" {
  echo 'harmless' >"$SANDBOX/notes.txt"
  stage notes.txt
  plant_secret notes.txt
  run -0 commit
  [[ "$(commits)" == 2 ]] || return 1

  plant_secret later.txt
  stage later.txt
  echo 'harmless' >"$SANDBOX/later.txt"
  run -1 commit
  has "later.txt:2: CRITICAL secret: AWS Access Key ID"
}

@test "git commit <path> checks the path through git's temporary index" {
  echo 'harmless' >"$SANDBOX/notes.txt"
  stage notes.txt
  commit notes.txt
  plant_secret notes.txt
  run -1 commit notes.txt
  has "notes.txt:2: CRITICAL secret"
}

@test "unformatted staged Go is rejected even when the working tree copy is formatted" {
  mkdir -p "$SANDBOX/apps/api/x"
  printf 'package x\nfunc  X( ) int { return 1 }\n' >"$SANDBOX/apps/api/x/x.go"
  stage apps/api/x/x.go
  printf 'package x\n\nfunc X() int { return 1 }\n' >"$SANDBOX/apps/api/x/x.go"
  run -1 commit
  has "apps/api/x/x.go"
  has "rule:        go-format"
  has "remediation: Run \`make fmt\`."
  [[ "$(commits)" == 1 ]] || return 1
}

@test "unformatted or shellcheck-failing staged shell is rejected" {
  printf '#!/usr/bin/env bash\nif true; then\n\techo hi\nfi\n' >"$SANDBOX/scripts/lib/tabs.sh"
  stage scripts/lib/tabs.sh
  run -1 commit
  has "rule:        sh-format"
  [[ "$output" != *"rule:        sh-shellcheck"* ]] || return 1

  git -C "$SANDBOX" reset -q
  printf '#!/usr/bin/env bash\necho $1\n' >"$SANDBOX/scripts/lib/unquoted.sh"
  stage scripts/lib/unquoted.sh
  run -1 commit
  has "scripts/lib/unquoted.sh:2:6: note: Double quote to prevent globbing and word splitting. [SC2086]"
  has "rule:        sh-shellcheck"
}

@test "a staged Markdown link must resolve in the staged tree, not in the working tree" {
  mkdir -p "$SANDBOX/docs"
  printf '# Doc\n\nSee [new](new.md).\n' >"$SANDBOX/docs/doc.md"
  echo '# New' >"$SANDBOX/docs/new.md" # untracked: not part of the commit
  stage docs/doc.md
  run -1 commit
  has "docs/doc.md"
  has "rule:        docs-links-resolve"
  stage docs/new.md
  run -0 commit
}

@test "fails closed without a tool it needs" {
  rm "$SANDBOX/.bin/shfmt"
  echo 'echo more' >>"$SANDBOX/scripts/lib/ok.sh"
  stage scripts/lib/ok.sh
  run -1 commit
  has "pre-commit: .bin/shfmt is missing: run make setup"
  [[ "$(commits)" == 1 && "$output" != *"commit blocked"* ]] || return 1

  rm "$SANDBOX/.bin/trivy"
  echo 'harmless' >"$SANDBOX/notes.txt"
  git -C "$SANDBOX" reset -q
  stage notes.txt
  run -1 commit
  has "pre-commit: .bin/trivy $(sed -n 's/^TRIVY_VERSION=//p' "$REPO/scripts/tool-versions.env") is missing: run make setup"
  [[ "$output" != *"rule:        repo-no-secrets"* && "$output" != *"commit blocked"* ]] || return 1
}

@test "a sensor that cannot run (exit above 1) blocks without asking to fix the files" {
  rm "$SANDBOX/.bin/shfmt"
  printf '#!/usr/bin/env bash\necho "shfmt: broken" >&2\nexit 3\n' >"$SANDBOX/.bin/shfmt"
  chmod +x "$SANDBOX/.bin/shfmt"
  echo 'echo more' >>"$SANDBOX/scripts/lib/ok.sh"
  stage scripts/lib/ok.sh
  run -1 commit
  has "shfmt: broken"
  [[ "$output" != *"commit blocked"* && "$(commits)" == 1 ]] || return 1
}

@test "nothing staged: passes without running a sensor" {
  rm "$SANDBOX/.bin/trivy"
  run -0 commit --allow-empty
  [[ -z "$output" ]]
}
