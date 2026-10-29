#!/usr/bin/env bats
# scripts/harness/layout.sh: parses scripts/harness/layout.env (never executes it) into the variables every
# script reads the repo layout from, and fails closed on a malformed file (rules.md#harness-layout-valid).
bats_require_minimum_version 1.5.0
load helpers

setup() {
  sandbox
  L="$SANDBOX/scripts/harness/layout.env"
}

# load_layout: source the sandbox's layout.sh in a child shell and print what it set.
load_layout() {
  run bash -c 'source "$1" || exit 2; printf "%s|" "$API_DIR" "$API_PREFIX" "$SPEC" "$LAYER_GLOB" \
    "${#WEB_GLOBS[@]}" "${WEB_DIRS[*]}" "${GENERATED_GLOBS[1]}" "${PRODUCT_GLOBS[*]}"' _ \
    "$SANDBOX/scripts/harness/layout.sh"
}

@test "reads the template's layout: strings, lists split at ':', workspace dirs, the API prefix" {
  load_layout
  [ "$status" -eq 0 ]
  [ "$output" = "apps/api|apps/api/|api/openapi.yaml|internal/*/*|2|apps packages|apps/api/internal/*/store/*|apps/* api/*|" ]
}

@test "API_DIR=. (a module at the repo root) has an empty prefix" {
  sed -i.bak 's|^API_DIR=.*|API_DIR=.|' "$L" && rm "$L.bak"
  load_layout
  [ "$status" -eq 0 ]
  has ".||api/openapi.yaml|"
}

@test "an unknown, repeated or missing key and a quoted value fail closed with the line" {
  sed -i.bak -e 's|^LAYER_GLOB=.*|LAYOUT_GLOB=internal/*/*|' -e 's|^SPEC=.*|SPEC="api/openapi.yaml"|' "$L" && rm "$L.bak"
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
  sed -i.bak 's|^PRODUCT_GLOBS=.*|PRODUCT_GLOBS=apps/*:|' "$L" && rm "$L.bak"
  load_layout
  [ "$status" -eq 2 ]
  has "layout.env:$(grep -n '^PRODUCT_GLOBS=' "$L" | cut -d: -f1): not KEY=value (no spaces, quotes, \$ or empty list entries)"
}

@test "DOMAINS_DIR is the API prefix plus LAYER_GLOB's literal directories" {
  sed -i.bak 's|^LAYER_GLOB=.*|LAYER_GLOB=pkg/domains/*/*|' "$L" && rm "$L.bak"
  run bash -c 'source "$1" || exit 2; echo "$DOMAINS_DIR"' _ "$SANDBOX/scripts/harness/layout.sh"
  [ "$status" -eq 0 ]
  [ "$output" = "apps/api/pkg/domains" ]
}

@test "--print emits exactly the known keys, validated (for the Makefile and CI's GITHUB_ENV)" {
  run -0 --separate-stderr "$SANDBOX/scripts/harness/layout.sh" --print
  [ "${#lines[@]}" -eq 9 ]
  [ "${lines[0]}" = "API_DIR=apps/api" ]
  [ "${lines[8]}" = "LAYER_GLOB=internal/*/*" ]
  run -1 grep -v '^[A-Z_]*=' <<<"$output" # every line is KEY=value
}

@test "--print of another file fails closed with nothing on stdout" {
  printf '%s\n' 'API_DIR=apps/api' 'GITHUB_TOKEN=x' >"$BATS_TEST_TMPDIR/other.env"
  run -2 --separate-stderr "$SANDBOX/scripts/harness/layout.sh" --print "$BATS_TEST_TMPDIR/other.env"
  [ -z "$output" ]
  [[ "$stderr" == *"unknown key GITHUB_TOKEN"* ]]
}

@test "executed without --print is a usage error" {
  run -2 "$SANDBOX/scripts/harness/layout.sh"
  has "usage:"
}
