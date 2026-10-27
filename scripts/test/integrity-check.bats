#!/usr/bin/env bats
# scripts/harness/integrity-check.sh: added suppressions need a reason; harness configs are not loosened.
# Suppression markers are assembled at runtime so this file never contains one itself.
bats_require_minimum_version 1.5.0
load helpers

setup() {
  sandbox scripts/harness/integrity-check.sh scripts/harness/protected-paths.txt .squawk.toml
  printf '%s\n' 'package x' >"$SANDBOX/x.go"
  sandbox_base
  NOLINT='//''nolint:errcheck'
  ESLINT='// eslint''-disable-next-line no-console'
  SHELLCHECK='# shellcheck'' disable=SC2086'
  TSIGNORE='// @ts''-ignore'
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
  add x.go "f() $NOLINT // reason: closing a read-only file (rule: go-lint)"
  add s.sh "$SHELLCHECK # reason: word splitting intended (rule: sh-shellcheck)"
  git -C "$SANDBOX" add -A
  check -0
}

@test "never accepts ts-ignore, even with a reason" {
  add a.ts "$TSIGNORE reason: x (rule: web-typecheck)"
  git -C "$SANDBOX" add -A
  check -1
  has 'a.ts:1: @ts-''ignore is never allowed'
}

@test "ignores fixture overlays" {
  mkdir -p "$SANDBOX/scripts/harness/fixtures/f/files"
  add scripts/harness/fixtures/f/files/x.go "$NOLINT"
  git -C "$SANDBOX" add -A
  check -0
}

@test "flags a removed or loosening line in a harness config" {
  sed -i.bak '/^assume_in_transaction/d' "$SANDBOX/.squawk.toml"
  add .squawk.toml 'excluded_rules = ["require-timeout-settings"]'
  check -1
  has '.squawk.toml:4: removed harness config line: assume_in_transaction = true'
  has '.squawk.toml:4: harness config line looks like a loosening: excluded_rules'
  has '[harness-config-not-loosened]'
}

@test "allows comments and tightening lines in a harness config" {
  sed -i.bak 's/^# Atlas applies.*/# reworded comment/' "$SANDBOX/.squawk.toml"
  add .squawk.toml 'pg_version_strict = true'
  check -0
}

@test "HARNESS_CHANGE_APPROVED waives the config check, not the suppression check" {
  add .squawk.toml 'excluded_rules = ["ban-drop-column"]'
  add x.go "$NOLINT"
  HARNESS_CHANGE_APPROVED=1 check -1
  has 'x.go:2: suppression without'
  [[ "$output" != *harness-config-not-loosened* ]]
}
