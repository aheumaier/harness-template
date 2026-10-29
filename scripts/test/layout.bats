#!/usr/bin/env bats
# scripts/harness/layout.sh: parses scripts/harness/layout.env (never executes it) into the variables every
# script reads the repo layout from, and fails closed on a malformed file (rules.md#harness-layout-valid).
bats_require_minimum_version 1.5.0
load helpers

# A fixed layout.env, not the template's: the tests do not move when the template's layout does.
setup() {
  sandbox
  L="$SANDBOX/scripts/harness/layout.env"
  printf '%s\n' '# layout' API_DIR=apps/api SPEC=api/openapi.yaml MIGRATIONS_DIR=apps/api/db/migrations \
    QUERIES_DIR=apps/api/db/queries API_CLIENT_DIR=packages/api-client 'WEB_GLOBS=apps/*:packages/*' \
    'GENERATED_GLOBS=*.gen.go:apps/api/internal/*/store/*:packages/api-client/src/gen/*:docs/generated/*' \
    'PRODUCT_GLOBS=apps/*:api/*' 'LAYER_GLOB=internal/*/*' STACK=go:db:web >"$L"
}

# load_layout: source the sandbox's layout.sh in a child shell and print what it set.
load_layout() {
  run bash -c 'source "$1" || exit 2; printf "%s|" "$API_DIR" "$API_PREFIX" "$SPEC" "$LAYER_GLOB" \
    "${#WEB_GLOBS[@]}" "${WEB_DIRS[*]}" "${GENERATED_GLOBS[1]}" "${PRODUCT_GLOBS[*]}"' _ \
    "$SANDBOX/scripts/harness/layout.sh"
}

@test "reads a layout: strings, lists split at ':', workspace dirs, the API prefix" {
  load_layout
  [ "$status" -eq 0 ]
  [ "$output" = "apps/api|apps/api/|api/openapi.yaml|internal/*/*|2|apps packages|apps/api/internal/*/store/*|apps/* api/*|" ]
}

@test "API_DIR=. (a module at the repo root) has an empty prefix" {
  edit scripts/harness/layout.env 's|^API_DIR=.*|API_DIR=.|'
  load_layout
  [ "$status" -eq 0 ]
  has ".||api/openapi.yaml|"
}

@test "an unknown, repeated or missing key and a quoted value fail closed with the line" {
  edit scripts/harness/layout.env 's|^LAYER_GLOB=.*|LAYOUT_GLOB=internal/*/*|; s|^SPEC=.*|SPEC="api/openapi.yaml"|'
  echo 'API_DIR=apps/api' >>"$L"
  load_layout
  [ "$status" -eq 2 ]
  has "layout.env:$(grep -n '^SPEC=' "$L" | cut -d: -f1): not KEY=value"
  has ": unknown key LAYOUT_GLOB"
  has ": API_DIR set twice [harness-layout-valid]"
  has "layout.env: missing LAYER_GLOB [harness-layout-valid]"
}

@test "a missing layout.env fails closed" {
  rm "$L"
  load_layout
  [ "$status" -eq 2 ]
  has "missing or unreadable"
}

@test "a list with an empty entry fails closed" {
  edit scripts/harness/layout.env 's|^PRODUCT_GLOBS=.*|PRODUCT_GLOBS=apps/*:|'
  load_layout
  [ "$status" -eq 2 ]
  has "layout.env:$(grep -n '^PRODUCT_GLOBS=' "$L" | cut -d: -f1): not KEY=value (no spaces, quotes, \$ or empty list entries)"
}

@test "DOMAINS_DIR is the API prefix plus LAYER_GLOB's literal directories" {
  edit scripts/harness/layout.env 's|^LAYER_GLOB=.*|LAYER_GLOB=pkg/domains/*/*|'
  run bash -c 'source "$1" || exit 2; echo "$DOMAINS_DIR"' _ "$SANDBOX/scripts/harness/layout.sh"
  [ "$status" -eq 0 ]
  [ "$output" = "apps/api/pkg/domains" ]
}

@test "--print emits exactly the known keys, validated (for the Makefile and CI's GITHUB_ENV)" {
  run -0 --separate-stderr "$SANDBOX/scripts/harness/layout.sh" --print
  [ "${#lines[@]}" -eq 10 ]
  [ "${lines[0]}" = "API_DIR=apps/api" ]
  [ "${lines[8]}" = "LAYER_GLOB=internal/*/*" ]
  [ "${lines[9]}" = "STACK=go:db:web" ]
  run -1 grep -v '^[A-Z_]*=' <<<"$output" # every line is KEY=value
}

@test "HARNESS_STACK overrides STACK for one run, with , or : (ADR 0033)" {
  HARNESS_STACK=go,web run -0 "$SANDBOX/scripts/harness/layout.sh" --print
  [ "${lines[9]}" = "STACK=go:web" ]
  HARNESS_STACK=go run -0 bash -c 'source "$1" || exit 2; harness_stack_has go && ! harness_stack_has db &&
    ! harness_stack_has web && echo ok' _ "$SANDBOX/scripts/harness/layout.sh"
  [ "$output" = ok ]
  edit scripts/harness/layout.env 's|^STACK=.*|STACK=go:db|'
  run -0 bash -c 'source "$1" || exit 2; harness_stack_has db && ! harness_stack_has web && echo ok' _ \
    "$SANDBOX/scripts/harness/layout.sh"
  [ "$output" = ok ]
}

@test "a stack without go, with an unknown or repeated group, or empty fails closed" {
  HARNESS_STACK=db,db,ui run -2 --separate-stderr "$SANDBOX/scripts/harness/layout.sh" --print
  [ -z "$output" ]
  grep -qF "HARNESS_STACK=db,db,ui: db named twice [harness-layout-valid]" <<<"$stderr"
  grep -qF "HARNESS_STACK=db,db,ui: unknown group ui (known: go db web)" <<<"$stderr"
  grep -qF "HARNESS_STACK=db,db,ui: no go (every stack" <<<"$stderr"
  HARNESS_STACK='' run -2 "$SANDBOX/scripts/harness/layout.sh" --print
  has "HARNESS_STACK=: not a list of groups"
  HARNESS_STACK='go;id' run -2 "$SANDBOX/scripts/harness/layout.sh" --print
  has "not a list of groups"
  edit scripts/harness/layout.env 's|^STACK=.*|STACK=web|'
  load_layout
  [ "$status" -eq 2 ]
  has "layout.env:$(grep -n '^STACK=' "$L" | cut -d: -f1): STACK=web: no go"
}

@test "a layout.env without STACK (written before the key) reads as go:db:web (ADR 0033)" {
  edit scripts/harness/layout.env '/^STACK=/d'
  run -0 --separate-stderr "$SANDBOX/scripts/harness/layout.sh" --print
  [ "${#lines[@]}" -eq 10 ]
  [ "${lines[9]}" = "STACK=go:db:web" ]
  [ -z "$stderr" ]
  HARNESS_STACK=go run -0 "$SANDBOX/scripts/harness/layout.sh" --print
  [ "${lines[9]}" = "STACK=go" ]
}

@test "STACK values: the vectors layout.go's test shares (testdata/stack-vectors.txt)" {
  local value want got n=0 bad=''
  while read -r value want; do
    [[ -n "$value" && "$value" != "#"* ]] || continue
    n=$((n + 1))
    edit scripts/harness/layout.env "s|^STACK=.*|STACK=$value|"
    "$SANDBOX/scripts/harness/layout.sh" --print >/dev/null 2>&1 && got=accept || got=reject
    [[ "$got" == "$want" ]] || bad+="STACK=$value: $got, want $want"$'\n'
  done <"$REPO/apps/api/tools/harness/testdata/stack-vectors.txt"
  [ "$n" -gt 0 ]
  [ -z "$bad" ] || {
    printf '%s' "$bad" >&2
    return 1
  }
}

@test "--print of another file fails closed with nothing on stdout" {
  printf '%s\n' 'API_DIR=apps/api' 'GITHUB_TOKEN=x' >"$BATS_TEST_TMPDIR/other.env"
  run -2 --separate-stderr "$SANDBOX/scripts/harness/layout.sh" --print "$BATS_TEST_TMPDIR/other.env"
  [ -z "$output" ]
  grep -qF "unknown key GITHUB_TOKEN" <<<"$stderr"
}

@test "executed without --print is a usage error" {
  run -2 "$SANDBOX/scripts/harness/layout.sh"
  has "usage:"
}
