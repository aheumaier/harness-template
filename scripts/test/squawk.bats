#!/usr/bin/env bats
# scripts/harness/squawk.sh: `-- atlas:txmode none` migrations run without the transaction assumption.
bats_require_minimum_version 1.5.0
load helpers

setup() {
  sandbox scripts/harness/squawk.sh
  MIG="$SANDBOX/apps/api/db/migrations"
  mkdir -p "$MIG" "$BATS_TEST_TMPDIR/bin"
  printf '%s\n' '#!/usr/bin/env bash' 'echo "squawk $*"' 'exit "${SQUAWK_EXIT:-0}"' >"$BATS_TEST_TMPDIR/bin/squawk"
  chmod +x "$BATS_TEST_TMPDIR/bin/squawk"
  PATH="$BATS_TEST_TMPDIR/bin:$PATH"
  printf '%s\n' 'CREATE TABLE t (x int);' >"$MIG/1_a.sql"
  printf '%s\n' '-- atlas:txmode none' '' 'CREATE INDEX CONCURRENTLY i ON t (x);' >"$MIG/2_b.sql"
}

@test "splits migrations by txmode" {
  run -0 "$SANDBOX/scripts/harness/squawk.sh"
  has 'squawk --config .squawk.toml apps/api/db/migrations/1_a.sql'
  has 'squawk --config .squawk.toml --no-assume-in-transaction apps/api/db/migrations/2_b.sql'
}

@test "fails when squawk fails" {
  SQUAWK_EXIT=1 run -1 "$SANDBOX/scripts/harness/squawk.sh"
}
