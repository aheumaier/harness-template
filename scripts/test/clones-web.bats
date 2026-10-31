#!/usr/bin/env bats
# make clones-web (rules.md#repo-no-clones): the real Makefile recipe around a stub jscpd. Exit 1 only when
# jscpd failed with a clone line in its output (the root cut off each path); any other failure is exit 2, so a
# broken sensor never reads as a clone. make itself exits 2 for any failed recipe: the recipe's own code is the
# one run.sh reports.
bats_require_minimum_version 1.5.0
load helpers

setup() {
  sandbox scripts/harness/run.sh scripts/harness/timing.sh scripts/harness/jscpd.json scripts/tool-versions.env \
    docs/harness/rules.md
  T="$BATS_TEST_TMPDIR"
  mkdir -p "$SANDBOX/apps" "$SANDBOX/packages" "$SANDBOX/node_modules/.bin"
  # The stub prints $T/jscpd.out with @ROOT@ as the root it runs in and exits with $T/jscpd.rc. The Makefile puts
  # $(CURDIR)/node_modules/.bin first on PATH, so it shadows the real jscpd.
  printf '#!/bin/sh\nsed "s|@ROOT@|$(pwd -P)|g" "%s/jscpd.out"\nexit "$(cat "%s/jscpd.rc")"\n' "$T" "$T" \
    >"$SANDBOX/node_modules/.bin/jscpd"
  chmod +x "$SANDBOX/node_modules/.bin/jscpd"
  unset MAKEFLAGS MFLAGS
}

# jscpd <rc> [line...]: what the stub prints and how it exits.
jscpd() {
  echo "$1" >"$T/jscpd.rc"
  shift
  printf '%s\n' "$@" >"$T/jscpd.out"
}

clones() { make -s -C "$SANDBOX" -f "$REPO/Makefile" clones-web 2>&1; }

@test "clones-web: no clones passes, a clone line is a finding with the root cut off" {
  jscpd 0 ''
  run -0 clones
  jscpd 1 '@ROOT@/apps/web/src/a.ts:1:1: warning: Found 5 lines (1-5) duplicated on file @ROOT@/packages/domain/src/a.ts'
  run -2 clones
  has "✗ sensor 'jscpd' failed (exit 1)"
  has 'apps/web/src/a.ts:1:1: warning: Found 5 lines (1-5) duplicated on file packages/domain/src/a.ts'
  [[ $output != *"$SANDBOX"* ]] || false
}

@test "clones-web: jscpd failing without a clone line, or with another code, cannot run (exit 2)" {
  jscpd 1 'Error: no files found (--fail-on-empty)'
  run -2 clones
  has "✗ sensor 'jscpd' failed (exit 2)"
  jscpd 3 '@ROOT@/apps/web/src/a.ts:1:1: warning: Found 5 lines (1-5) duplicated on file @ROOT@/packages/domain/src/a.ts'
  run -2 clones
  has "✗ sensor 'jscpd' failed (exit 2)"
  jscpd 127 'jscpd: command not found'
  run -2 clones
  has "✗ sensor 'jscpd' failed (exit 2)"
}
