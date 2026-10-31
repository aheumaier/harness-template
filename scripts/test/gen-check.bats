#!/usr/bin/env bats
# scripts/harness/gen-check.sh: fails when `make gen` changes a generated file; after a green run it stamps the
# generator inputs and outputs, and `--unchanged` (make check skips) holds only while both match that stamp.
bats_require_minimum_version 1.5.0
load helpers

setup() {
  sandbox scripts/harness/gen-check.sh
  G="$SANDBOX/scripts/harness/gen-check.sh"
  GEN=packages/api-client/src/gen/client.ts
  mkdir -p "$SANDBOX/api" "$SANDBOX/$(dirname "$GEN")" "$BATS_TEST_TMPDIR/bin"
  echo 'openapi: v1' >"$SANDBOX/api/openapi.yaml"
  # Fake `make gen`: a deterministic generator from api/openapi.yaml; logs each call.
  cat >"$BATS_TEST_TMPDIR/bin/make" <<EOF
#!/usr/bin/env bash
echo "make \$*" >>"$BATS_TEST_TMPDIR/make.log"
sed 's/^/gen /' api/openapi.yaml >"$GEN"
EOF
  chmod +x "$BATS_TEST_TMPDIR/bin/make"
  PATH="$BATS_TEST_TMPDIR/bin:$PATH"
  (cd "$SANDBOX" && make gen)
  sandbox_base
  rm -f "$BATS_TEST_TMPDIR/make.log"
}

stamp() { cat "$SANDBOX/.cache/harness/gen-check.stamp"; }

@test "a green run stamps; --unchanged then holds without regenerating" {
  run -1 "$G" --unchanged # no stamp yet
  run -0 "$G"
  test -s "$SANDBOX/.cache/harness/gen-check.stamp" # not [[ ]]: bash 3.2 ignores a false one mid-test
  rm "$BATS_TEST_TMPDIR/make.log"
  run -0 "$G" --unchanged
  [[ ! -e "$BATS_TEST_TMPDIR/make.log" ]]
}

@test "a changed input (edited or new) breaks the stamp" {
  run -0 "$G"
  echo 'openapi: v2' >"$SANDBOX/api/openapi.yaml"
  run -1 "$G" --unchanged
  run -1 "$G" # stale output: red, regenerated
  has "$GEN"
  run -0 "$G"
  run -0 "$G" --unchanged

  mkdir -p "$SANDBOX/apps/api/db/queries"
  echo 'SELECT 1;' >"$SANDBOX/apps/api/db/queries/new.sql"
  run -1 "$G" --unchanged
}

@test "another effective STACK breaks the stamp: a narrower make gen ran fewer generators (ADR 0033)" {
  run -0 "$G"
  HARNESS_STACK=go run -1 "$G" --unchanged
  run -0 "$G" --unchanged
}

@test "a hand edit of a generated file breaks the stamp, committed or not" {
  run -0 "$G"
  echo '// hand edit' >>"$SANDBOX/$GEN"
  run -1 "$G" --unchanged
  git -C "$SANDBOX" -c user.name=test -c user.email=test@localhost -c commit.gpgsign=false commit -qam edit
  run -1 "$G" --unchanged
  run -1 "$G"
  has '[gen-up-to-date]'
}

@test "a red run writes no stamp" {
  echo '// hand edit' >>"$SANDBOX/$GEN"
  run -1 "$G"
  [[ ! -e "$SANDBOX/.cache/harness/gen-check.stamp" ]]
}

@test "a red run keeps the last green stamp, which no longer matches" {
  run -0 "$G"
  green="$(stamp)"
  echo '// hand edit' >>"$SANDBOX/$GEN"
  run -1 "$G"
  [[ "$(stamp)" == "$green" ]]
}

@test "scope full never skips (make check-all, CI)" {
  run -0 "$G"
  HARNESS_SCOPE=full run -1 "$G" --unchanged
  HARNESS_SCOPE=other run -1 "$G" --unchanged
}

@test "a corrupt stamp means run" {
  run -0 "$G"
  echo garbage >"$SANDBOX/.cache/harness/gen-check.stamp"
  run -1 "$G" --unchanged
}
