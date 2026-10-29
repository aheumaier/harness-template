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

# ADR 0030: the base copy of layout.env names the base's MIGRATIONS_DIR, so moving it cannot hide an edit.
move_dir() { # <mv|copy>
  if [[ "$1" == mv ]]; then
    git -C "$SANDBOX" mv apps/api/db/migrations apps/api/db/schema
  else
    cp -R "$MIG" "$SANDBOX/apps/api/db/schema"
  fi
  edit scripts/harness/layout.env 's|^MIGRATIONS_DIR=.*|MIGRATIONS_DIR=apps/api/db/schema|'
  git -C "$SANDBOX" add -A
}

@test "passes when MIGRATIONS_DIR moves with its migrations unchanged" {
  move_dir mv
  check -0
}

@test "flags a migration edited while MIGRATIONS_DIR moves" {
  move_dir mv
  printf '%s\n' '-- edited' >>"$SANDBOX/apps/api/db/schema/1_a.sql"
  check -1
  has 'apps/api/db/migrations/1_a.sql: migration on main was modified [db-append-only-migrations]'
}

@test "flags an edit left in the base's MIGRATIONS_DIR after the layout points elsewhere" {
  move_dir copy
  printf '%s\n' '-- edited' >>"$MIG/1_a.sql"
  check -1
  has 'apps/api/db/migrations/1_a.sql: migration on main was modified [db-append-only-migrations]'
}

@test "reads a base layout.env written before STACK (9 keys) and still flags an edit (ADR 0033)" {
  local env="$SANDBOX/scripts/harness/layout.env"
  cp "$env" "$BATS_TEST_TMPDIR/head.env"
  edit scripts/harness/layout.env '/^STACK=/d'
  git -C "$SANDBOX" -c user.name=test -c user.email=test@localhost -c commit.gpgsign=false commit -qam "no STACK"
  cp "$BATS_TEST_TMPDIR/head.env" "$env"
  printf '%s\n' '-- edited' >>"$MIG/1_a.sql"
  check -1
  has 'apps/api/db/migrations/1_a.sql: migration on main was modified [db-append-only-migrations]'
}

@test "fails closed when the base's layout.env is invalid" {
  local env="$SANDBOX/scripts/harness/layout.env"
  cp "$env" "$BATS_TEST_TMPDIR/valid.env"
  printf '%s\n' 'MIGRATIONS_DIR=$(id)' >>"$env"
  git -C "$SANDBOX" -c user.name=test -c user.email=test@localhost -c commit.gpgsign=false commit -qam broken
  cp "$BATS_TEST_TMPDIR/valid.env" "$env"
  check -2
  has 'scripts/harness/layout.env on main is invalid'
}
