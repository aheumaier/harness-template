#!/usr/bin/env bats
# Makefile: STACK (ADR 0033) drops the db and web sensors from make check, check-all and gen. A dry run
# (make -n) of the real Makefile: the sub-make lines it echoes name the sensors that would run.
bats_require_minimum_version 1.5.0
load helpers

# dry <HARNESS_STACK>: `make -n check check-all gen` as a top-level make, output in $output. make -n still runs
# every recipe line that names $(MAKE), so a real sub-make would run e2e and test-obs for real (and fail where
# apps/web is missing); MAKE='echo make' turns those lines into the target lists they would run.
dry() {
  run -0 env -u MAKEFLAGS -u MAKELEVEL -u MFLAGS HARNESS_STACK="$1" make -s -C "$REPO" -n MAKE='echo make' check check-all gen
  SUBMAKES="$(grep -E '^make -k ' <<<"$output" | tr ' ' '\n')"
}

# targets <DB_TARGETS|WEB_TARGETS>: the Makefile's list, without the gen-* steps (asserted by command below).
targets() { sed -n "s/^$1 *:= *//p" "$REPO/Makefile" | tr ' ' '\n' | grep -v '^gen-'; }

@test "STACK=go: no db or web sensor in check, check-all or gen" {
  dry go
  local t bad=''
  for t in $(targets DB_TARGETS) $(targets WEB_TARGETS); do
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
}

# CI's setup action exports layout.env's keys: a STACK or STACK_OFF from the environment, even under make -e,
# loses to layout.env's (override), so it cannot drop the db sensors past layout-check. The sub-makes match a run
# without them; where layout.env's STACK has db (the template's), they still list its sensors.
@test "STACK=go or STACK_OFF in the environment, with or without make -e, does not beat layout.env" {
  local flag want
  run -0 env -u MAKEFLAGS -u MAKELEVEL -u MFLAGS -u HARNESS_STACK -u STACK -u STACK_OFF \
    make -s -C "$REPO" -n MAKE='echo make' check check-all
  want="$(grep -E '^make -k ' <<<"$output")"
  for flag in -s -se; do
    run -0 env -u MAKEFLAGS -u MAKELEVEL -u MFLAGS -u HARNESS_STACK STACK=go STACK_OFF="$(targets DB_TARGETS | tr '\n' ' ')" \
      make "$flag" -C "$REPO" -n MAKE='echo make' check check-all
    [ "$(grep -E '^make -k ' <<<"$output")" == "$want" ]
    ! grep -qE '^STACK=.*db' "$REPO/scripts/harness/layout.env" ||
      grep -E '^make -k ' <<<"$output" | tr ' ' '\n' | grep -qxF squawk
  done
}

# A command-line STACK or STACK_OFF beats layout.env's and would drop the db and web sensors past layout-check.
@test "STACK or STACK_OFF on the make command line stops make before any sensor runs" {
  local arg
  for arg in STACK=go STACK_OFF=lint-go; do
    run -2 env -u MAKEFLAGS -u MAKELEVEL -u MFLAGS make -s -C "$REPO" -n MAKE='echo make' check "$arg"
    has "${arg%%=*} "
    has 'for one run use HARNESS_STACK=… [harness-layout-valid]'
    run -1 grep -q 'make -k ' <<<"$output"
  done
}
