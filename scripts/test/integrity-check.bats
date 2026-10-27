#!/usr/bin/env bats
# scripts/harness/integrity-check.sh: added suppressions need a reason; harness configs are not loosened.
# Suppression markers are assembled at runtime so this file never contains one itself.
bats_require_minimum_version 1.5.0
load helpers

setup() {
  sandbox scripts/harness/integrity-check.sh scripts/harness/merge-base.sh scripts/harness/protected-paths.txt \
    docs/harness/rules.md .squawk.toml
  printf '%s\n' 'package x' >"$SANDBOX/x.go"
  sandbox_base
  NOLINT='//''nolint:noctxroot'
  ESLINT='// eslint''-disable-next-line no-console'
  SHELLCHECK='# shellcheck'' disable=SC2086'
  TSIGNORE='// @ts''-ignore'
  TSNOCHECK='// @ts''-nocheck'
}

check() { BASE_REF=main run "$@" "$SANDBOX/scripts/harness/integrity-check.sh"; }
add() { printf '%s\n' "${@:2}" >>"$SANDBOX/$1"; }

@test "passes when nothing changed" {
  check -0
}

@test "flags suppressions without a reason, with path and line" {
  add x.go "f() $NOLINT"
  add a.ts "$ESLINT"
  add s.sh "$SHELLCHECK"
  check -1 # a.ts and s.sh are untracked: still part of the change
  has 'x.go:2: suppression without'
  has 'a.ts:1: suppression without'
  has 's.sh:1: suppression without'
  has '[harness-suppression-reason]'
}

@test "accepts suppressions with reason and rule id" {
  add x.go "f() $NOLINT // reason: closing a read-only file (rule: go-no-root-context)"
  add s.sh "$SHELLCHECK # reason: word splitting intended (rule: sh-shellcheck)"
  git -C "$SANDBOX" add -A
  check -0
}

@test "never accepts ts-ignore or ts-nocheck, even with a reason" {
  add a.ts "$TSIGNORE reason: x (rule: web-typecheck)"
  add b.mts "$TSNOCHECK reason: x (rule: web-typecheck)"
  git -C "$SANDBOX" add -A
  check -1
  has 'a.ts:1: @ts-''ignore is never allowed'
  has 'b.mts:1: @ts-''nocheck is never allowed'
}

@test "flags SQL, staticcheck, inline eslint and workflow suppressions without a reason" {
  add m.sql '-- atlas'':nolint DS103' '-- squawk''-ignore ban-drop-column'
  add y.go '//lint'':ignore SA1019 old api' '//lint'':file-ignore U1000 generated elsewhere'
  add c.jsx '/* eslint'' no-console: off */'
  mkdir -p "$SANDBOX/.github/workflows"
  add .github/workflows/w.yml "      - run: echo \$x $SHELLCHECK"
  check -1
  has 'm.sql:1: suppression without'
  has 'm.sql:2: suppression without'
  has 'y.go:1: suppression without'
  has 'y.go:2: suppression without'
  has 'c.jsx:1: suppression without'
  has '.github/workflows/w.yml:1: suppression without'
}

@test "flags a reason that names no rule in rules.md" {
  add x.go "f() $NOLINT // reason: trust me (rule: go-lint)"
  check -1
  has "x.go:2: suppression names 'go-lint', not a rule id in docs/harness/rules.md"
}

@test "reports paths with spaces" {
  printf '%s\n' 'package y' >"$SANDBOX/a b.go"
  git -C "$SANDBOX" add -A
  git -C "$SANDBOX" -c user.name=t -c user.email=t@localhost -c commit.gpgsign=false commit -qm space
  add 'a b.go' "f() $NOLINT"
  check -1
  has 'a b.go:2: suppression without'
}

@test "fails closed when BASE_REF does not resolve" {
  BASE_REF=no-such-branch run -2 "$SANDBOX/scripts/harness/integrity-check.sh"
  has "BASE_REF 'no-such-branch' is not a commit"
}

@test "ignores fixture overlays in the suppression check (a new fixture is still a harness change)" {
  mkdir -p "$SANDBOX/scripts/harness/fixtures/f/files"
  add scripts/harness/fixtures/f/files/x.go "$NOLINT"
  git -C "$SANDBOX" add -A
  HARNESS_CHANGE_APPROVED=1 check -0
}

@test "ignores vendored .specify/ in the suppression check, not extensionless git hooks" {
  mkdir -p "$SANDBOX/.specify/scripts/bash" "$SANDBOX/scripts/git-hooks"
  add .specify/scripts/bash/x.sh "$SHELLCHECK"
  add scripts/git-hooks/commit-msg "$SHELLCHECK"
  git -C "$SANDBOX" add -A
  HARNESS_CHANGE_APPROVED=1 check -1
  has 'scripts/git-hooks/commit-msg:1: suppression without'
  [[ "$output" != *'.specify/'* ]]
}

@test "the PR's .gitattributes cannot hide a hunk" {
  printf '%s\n' '*.go -diff' '*.toml binary' >"$SANDBOX/.gitattributes"
  add x.go "f() $NOLINT"
  add .squawk.toml 'excluded_rules = ["ban-drop-column"]'
  check -1
  has 'x.go:2: suppression without'
  has '.squawk.toml:5: added harness config line: excluded_rules'
  has '.gitattributes:1: added harness config line: *.go -diff'
}

@test "flags a change to sensor code" {
  add scripts/harness/merge-base.sh 'exit 0'
  check -1
  has 'scripts/harness/merge-base.sh:27: added harness config line: exit 0'
}

@test "flags a removed or loosening line in a harness config" {
  sed -i.bak '/^assume_in_transaction/d' "$SANDBOX/.squawk.toml"
  add .squawk.toml 'excluded_rules = ["require-timeout-settings"]'
  check -1
  has '.squawk.toml:4: removed harness config line: assume_in_transaction = true'
  has '.squawk.toml:4: added harness config line: excluded_rules'
  has '[harness-config-not-loosened]'
}

@test "allows comment-only changes to a harness config" {
  sed -i.bak 's/^# Atlas applies.*/# reworded comment/' "$SANDBOX/.squawk.toml"
  check -0
}

@test "flags a tightening line too: any config change is a human decision" {
  add .squawk.toml 'pg_version_strict = true'
  check -1
  has '.squawk.toml:5: added harness config line: pg_version_strict = true'
}

@test "judges HARNESS_ROOT with its own protected-paths.txt, so a change cannot unprotect itself" {
  pr="$BATS_TEST_TMPDIR/pr"
  git clone -q "$SANDBOX" "$pr"
  sed -i.bak '/squawk.toml/d' "$pr/scripts/harness/protected-paths.txt"
  printf '%s\n' 'excluded_rules = ["ban-drop-column"]' >>"$pr/.squawk.toml"
  HARNESS_ROOT="$pr" BASE_REF=main run -1 "$SANDBOX/scripts/harness/integrity-check.sh"
  has '.squawk.toml:5: added harness config line: excluded_rules'
}

@test "HARNESS_CHANGE_APPROVED waives the config check, not the suppression check" {
  add .squawk.toml 'excluded_rules = ["ban-drop-column"]'
  add x.go "$NOLINT"
  HARNESS_CHANGE_APPROVED=1 check -1
  has 'x.go:2: suppression without'
  [[ "$output" != *harness-config-not-loosened* ]]
}

@test "a // or /* line in a config is not a comment: it can comment a setting out" {
  mkdir -p "$SANDBOX/apps/web"
  printf '%s\n' '{' '  "strict": true' '}' >"$SANDBOX/apps/web/tsconfig.json"
  git -C "$SANDBOX" add -A
  git -C "$SANDBOX" -c user.name=t -c user.email=t@localhost -c commit.gpgsign=false commit -qm ts
  printf '%s\n' '{' '  /*' '  "strict": true' '  // */' '}' >"$SANDBOX/apps/web/tsconfig.json"
  check -1
  has 'apps/web/tsconfig.json:2: added harness config line:   /*'
}

@test "looks rule ids up in its own rules.md, not the change's" {
  pr="$BATS_TEST_TMPDIR/pr"
  git clone -q "$SANDBOX" "$pr"
  printf '%s\n' '### made-up-rule' >>"$pr/docs/harness/rules.md"
  printf '%s\n' "f() $NOLINT // reason: trust me (rule: made-up-rule)" >>"$pr/x.go"
  HARNESS_ROOT="$pr" HARNESS_CHANGE_APPROVED=1 BASE_REF=main run -1 "$SANDBOX/scripts/harness/integrity-check.sh"
  has "x.go:2: suppression names 'made-up-rule', not a rule id"
}

@test "fails closed without config entries" {
  printf '%s\n' '# emptied' >"$SANDBOX/scripts/harness/protected-paths.txt"
  check -2
  has 'refusing to pass'
}
