#!/usr/bin/env bats
# Makefile: STACK (ADR 0033) drops the db, web and ios sensors from make check, check-all and gen, and a host
# other than macOS the ios ones (HOST_OFF, ADR 0042). A dry run (make -n) of the real Makefile: the sub-make lines
# it echoes name the sensors that would run. A uname stub sets the host, so both hosts are tested on either.
bats_require_minimum_version 1.5.0
load helpers

# dry <HARNESS_STACK>: `make -n check check-all gen` as a top-level make, output in $output. make -n still runs
# every recipe line that names $(MAKE), so a real sub-make would run e2e and test-obs for real (and fail where
# apps/web is missing); MAKE='echo make' turns those lines into the target lists they would run.
dry() {
  run -0 env -u MAKEFLAGS -u MAKELEVEL -u MFLAGS HARNESS_STACK="$1" make -s -C "$REPO" -n MAKE='echo make' check check-all gen
  SUBMAKES="$(grep -E '^make -k ' <<<"$output" | tr ' ' '\n')"
}

# targets <DB_TARGETS|WEB_TARGETS|IOS_TARGETS>: the Makefile's list, without the gen-* steps (asserted by command below).
targets() { sed -n "s/^$1 *:= *//p" "$REPO/Makefile" | tr ' ' '\n' | grep -v '^gen-'; }

@test "STACK=go: no db, web or ios sensor in check, check-all or gen" {
  dry go
  local t bad=''
  for t in $(targets DB_TARGETS) $(targets WEB_TARGETS) $(targets IOS_TARGETS); do
    ! grep -qxF "$t" <<<"$SUBMAKES" || bad+=" $t"
  done
  [ -z "$bad" ] || {
    echo "sub-makes run:$bad" >&2
    return 1
  }
  grep -qxF lint-go <<<"$SUBMAKES"
  run -1 grep -E 'orval|sqlc generate|atlas migrate hash' <<<"$output"
}

@test "STACK=go:db:web: every db and web sensor in check or check-all, and gen runs orval, sqlc and atlas" {
  dry go,db,web
  local t missing=''
  for t in $(targets DB_TARGETS) $(targets WEB_TARGETS); do
    grep -qxF "$t" <<<"$SUBMAKES" || missing+=" $t"
  done
  [ -z "$missing" ] || {
    echo "sub-makes miss:$missing" >&2
    return 1
  }
  has orval
  has 'sqlc generate'
  has 'atlas migrate hash'
  has 'leaves out: test-ios' # check names the ios sensors STACK leaves out
}

@test "STACK with ios on macOS: its sensors run in check" {
  host Darwin
  dry go,ios
  local t ios
  ios="$(targets IOS_TARGETS)"
  grep -qxF test-ios <<<"$ios"
  for t in $ios; do grep -qxF "$t" <<<"$SUBMAKES"; done
  run -1 grep -F 'this host (not macOS)' <<<"$output"
}

@test "STACK with ios on a host that is not macOS: check names its sensors as left out by the host" {
  host Linux
  dry go,ios
  local t ios
  ios="$(targets IOS_TARGETS)"
  grep -qxF test-ios <<<"$ios"
  has "this host (not macOS) leaves out: $(tr '\n' ' ' <<<"$ios" | sed 's/ $//')"
  for t in $ios; do run -1 grep -qxF "$t" <<<"$SUBMAKES"; done
}

@test "the stack functions stacked and stack_has on the make command line do not drop a sensor" {
  host Darwin
  dry go,ios
  grep -qxF test-ios <<<"$SUBMAKES"
  run -0 env -u MAKEFLAGS -u MAKELEVEL -u MFLAGS HARNESS_STACK=go,ios make -s -C "$REPO" -n MAKE='echo make' check \
    'stacked=$(filter-out test-ios,$(1))' stack_has=
  grep -E '^make -k ' <<<"$output" | tr ' ' '\n' | grep -qxF test-ios
}

# CI's setup action exports layout.env's keys: a STACK, STACK_OFF or HOST_OFF from the environment, even under make -e,
# loses to layout.env's (override), so it cannot drop the db sensors past layout-check. The sub-makes match a run
# without them; where layout.env's STACK has db (the template's), they still list its sensors.
@test "STACK=go, STACK_OFF or HOST_OFF in the environment, with or without make -e, does not beat layout.env" {
  local flag want
  run -0 env -u MAKEFLAGS -u MAKELEVEL -u MFLAGS -u HARNESS_STACK -u STACK -u STACK_OFF -u HOST_OFF \
    make -s -C "$REPO" -n MAKE='echo make' check check-all
  want="$(grep -E '^make -k ' <<<"$output")"
  for flag in -s -se; do
    run -0 env -u MAKEFLAGS -u MAKELEVEL -u MFLAGS -u HARNESS_STACK STACK=go STACK_OFF="$(targets DB_TARGETS | tr '\n' ' ')" HOST_OFF=lint-go \
      make "$flag" -C "$REPO" -n MAKE='echo make' check check-all
    [ "$(grep -E '^make -k ' <<<"$output")" == "$want" ]
    ! grep -qE '^STACK=.*db' "$REPO/scripts/harness/layout.env" ||
      grep -E '^make -k ' <<<"$output" | tr ' ' '\n' | grep -qxF squawk
  done
}

# A command-line STACK, STACK_OFF, HOST_OFF or group target list beats layout.env's and would drop sensors past
# layout-check: a group's targets feed STACK_OFF and HOST_OFF.
@test "STACK, STACK_OFF, HOST_OFF or a group's targets on the make command line stops make before any sensor runs" {
  local arg
  for arg in STACK=go STACK_OFF=lint-go HOST_OFF=lint-go DB_TARGETS=lint-go WEB_TARGETS=lint-go IOS_TARGETS=lint-go; do
    run -2 env -u MAKEFLAGS -u MAKELEVEL -u MFLAGS make -s -C "$REPO" -n MAKE='echo make' check "$arg"
    has "${arg%%=*} "
    has 'for one run use HARNESS_STACK=… [harness-layout-valid]'
    run -1 grep -q 'make -k ' <<<"$output"
  done
}

@test "test-ios: skipped by name with an empty IOS_DIR or with neither a Package.swift nor Swift tests, stops without a developer directory" {
  sandbox Makefile scripts/tool-versions.env scripts/harness/unchanged.sh scripts/harness/run.sh scripts/harness/timing.sh \
    docs/harness/rules.md
  # An adopter's layout.env may have no IOS_DIR line yet (README: it adds one when it syncs).
  edit scripts/harness/layout.env '/^IOS_DIR=/d'
  echo 'IOS_DIR=' >>"$SANDBOX/scripts/harness/layout.env"
  HARNESS_SCOPE=full run -0 make -s -C "$SANDBOX" test-ios
  has 'test-ios: skipped, IOS_DIR is empty'
  edit scripts/harness/layout.env 's|^IOS_DIR=.*|IOS_DIR=apps/ios|'
  mkdir -p "$SANDBOX/apps/ios/Tests" && echo 'name: App' >"$SANDBOX/apps/ios/project.yml"
  echo x >"$SANDBOX/apps/ios/Tests/README"
  HARNESS_SCOPE=full run -0 make -s -C "$SANDBOX" test-ios
  has 'test-ios: skipped, no Package.swift and no Swift file in Tests/ in IOS_DIR=apps/ios'
  echo '// swift-tools-version: 6.0' >"$SANDBOX/apps/ios/Package.swift"
  mkdir -p "$BATS_TEST_TMPDIR/bin" && printf '#!/bin/sh\nexit 2\n' >"$BATS_TEST_TMPDIR/bin/xcode-select"
  chmod +x "$BATS_TEST_TMPDIR/bin/xcode-select"
  PATH="$BATS_TEST_TMPDIR/bin:$PATH" HARNESS_SCOPE=full run -2 make -s -C "$SANDBOX" test-ios
  has 'test-ios: no Xcode or command-line tools: run xcode-select --install [ios-tests-pass]'
}
