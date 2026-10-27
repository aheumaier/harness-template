#!/usr/bin/env bats
# scripts/harness/migrations-append-only.sh: migrations on the base branch are never edited or deleted.
bats_require_minimum_version 1.5.0
load helpers

setup() {
  sandbox scripts/harness/migrations-append-only.sh scripts/harness/merge-base.sh
  MIG="$SANDBOX/apps/api/db/migrations"
  mkdir -p "$MIG"
  printf '%s\n' 'CREATE TABLE t (x int);' >"$MIG/1_a.sql"
  printf '%s\n' 'CREATE TABLE u (x int);' >"$MIG/2_b.sql"
  sandbox_base
}

check() { BASE_REF=main run "$@" "$SANDBOX/scripts/harness/migrations-append-only.sh"; }

@test "passes when a migration is added" {
  printf '%s\n' 'CREATE TABLE v (x int);' >"$MIG/3_c.sql"
  git -C "$SANDBOX" add -A
  check -0
}

@test "flags edited and deleted migrations" {
  printf '%s\n' '-- edited' >>"$MIG/1_a.sql"
  rm "$MIG/2_b.sql"
  check -1
  has 'apps/api/db/migrations/1_a.sql: migration on main was modified [db-append-only-migrations]'
  has 'apps/api/db/migrations/2_b.sql: migration on main was deleted'
}

@test "flags a renamed migration as deleted" {
  git -C "$SANDBOX" mv apps/api/db/migrations/1_a.sql apps/api/db/migrations/9_a.sql
  check -1
  has '1_a.sql: migration on main was deleted'
}

@test "fails closed when BASE_REF does not resolve" {
  BASE_REF=nope run -2 "$SANDBOX/scripts/harness/migrations-append-only.sh"
  has "is not a commit"
}
