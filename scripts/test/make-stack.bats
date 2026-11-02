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

# targets <DB_TARGETS|WEB_TARGETS|IOS_SWIFT_TARGETS|IOS_XCODE_TARGETS>: the Makefile's list, without the gen-* steps
# (asserted by command below). ios_targets: IOS_TARGETS, both of its lists.
targets() { sed -n "s/^$1 *:= *//p" "$REPO/Makefile" | tr ' ' '\n' | grep -v '^gen-'; }
ios_targets() { targets IOS_SWIFT_TARGETS && targets IOS_XCODE_TARGETS; }

@test "STACK=go: no db, web or ios sensor in check, check-all or gen" {
  dry go
  local t bad=''
  for t in $(targets DB_TARGETS) $(targets WEB_TARGETS) $(ios_targets); do
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
  local off
  off="$(sed -n 's/.*STACK=go:db:web leaves out: \(.*\) (ADR 0033).*/\1/p' <<<"$output" | tr ' ' '\n')" # the ios sensors STACK leaves out
  for t in $(ios_targets); do grep -qxF "$t" <<<"$off"; done
}

@test "STACK with ios on macOS: its Swift sensors run in check, those needing Xcode only in check-ios (ADR 0043)" {
  host Darwin
  dry go,ios
  local t swift xcode
  swift="$(targets IOS_SWIFT_TARGETS)" xcode="$(targets IOS_XCODE_TARGETS)"
  for t in test-ios lint-ios fmt-check-ios; do grep -qxF "$t" <<<"$swift"; done
  grep -qxF xcode-pinned <<<"$xcode"
  for t in $swift; do grep -qxF "$t" <<<"$SUBMAKES"; done
  for t in $xcode; do run -1 grep -qxF "$t" <<<"$SUBMAKES"; done
  run -1 grep -F 'this host (not macOS)' <<<"$output"
  run -0 env -u MAKEFLAGS -u MAKELEVEL -u MFLAGS HARNESS_STACK=go,ios make -s -C "$REPO" -n MAKE='echo make' check-ios
  has "make -k $(ios_targets | tr '\n' ' ' | sed 's/ $//')"
}

@test "check-ios: skipped by name when STACK has no ios or the host is not macOS" {
  host Darwin
  HARNESS_STACK=go run -0 env -u MAKEFLAGS -u MAKELEVEL -u MFLAGS make -s -C "$REPO" check-ios
  [ "$output" == 'check-ios: skipped, STACK=go has no ios (ADR 0033)' ]
  host Linux
  HARNESS_STACK=go,ios run -0 env -u MAKEFLAGS -u MAKELEVEL -u MFLAGS make -s -C "$REPO" check-ios
  [ "$output" == 'check-ios: skipped, this host is not macOS (ADR 0042)' ]
}

@test "STACK with ios on a host that is not macOS: check names its sensors as left out by the host" {
  host Linux
  dry go,ios
  local t ios
  ios="$(ios_targets)"
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
  for arg in STACK=go STACK_OFF=lint-go HOST_OFF=lint-go DB_TARGETS=lint-go WEB_TARGETS=lint-go IOS_TARGETS=lint-go \
    IOS_SWIFT_TARGETS=lint-go IOS_XCODE_TARGETS=lint-go; do
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
  stub xcode-select 'exit 2'
  PATH="$BATS_TEST_TMPDIR/bin:$PATH" HARNESS_SCOPE=full run -2 make -s -C "$SANDBOX" test-ios
  has 'test-ios: no Xcode or command-line tools: run xcode-select --install [ios-tests-pass]'
}

@test "xcode-pinned: skipped by name with only the command-line tools, fails on another Xcode, passes on XCODE_VERSION" {
  sandbox Makefile scripts/tool-versions.env scripts/harness/run.sh scripts/harness/timing.sh docs/harness/rules.md
  local pin
  pin="$(sed -n 's/^XCODE_VERSION=//p' "$SANDBOX/scripts/tool-versions.env")"
  [ -n "$pin" ]
  # As with only the command-line tools: xcodebuild needs Xcode.
  stub xcodebuild 'echo "xcode-select: error: tool '"'xcodebuild'"' requires Xcode" >&2; exit 1'
  stub xcode-select 'echo /Library/Developer/CommandLineTools'
  # stdout only: the sandbox's Makefile warns on stderr that it has no tools/ to find.
  PATH="$BATS_TEST_TMPDIR/bin:$PATH" run -0 --separate-stderr make -s -C "$SANDBOX" xcode-pinned
  [ "$output" == 'xcode-pinned: skipped, only the command-line tools are selected, no Xcode (ADR 0043)' ]
  # A directory named CommandLineTools (DEVELOPER_DIR) does not skip a working xcodebuild.
  stub xcodebuild 'printf "Xcode 9.0\nBuild version 9A1\n"'
  PATH="$BATS_TEST_TMPDIR/bin:$PATH" run -2 make -s -C "$SANDBOX" xcode-pinned
  has "the selected Xcode is 9.0, scripts/tool-versions.env pins XCODE_VERSION=$pin"
  stub xcode-select 'echo /Applications/Xcode_9.app/Contents/Developer'
  stub xcodebuild 'printf "Xcode 9.0\nBuild version 9A1\n"'
  PATH="$BATS_TEST_TMPDIR/bin:$PATH" run -2 make -s -C "$SANDBOX" xcode-pinned
  has "the selected Xcode is 9.0, scripts/tool-versions.env pins XCODE_VERSION=$pin"
  has 'rule:        ios-xcode-pinned'
  stub xcodebuild "printf 'Xcode $pin\\nBuild version 1A1\\n'"
  PATH="$BATS_TEST_TMPDIR/bin:$PATH" run -0 --separate-stderr make -s -C "$SANDBOX" xcode-pinned
  [ -z "$output" ]
  # A broken xcodebuild is not a pass.
  stub xcodebuild 'exit 1'
  PATH="$BATS_TEST_TMPDIR/bin:$PATH" run -2 make -s -C "$SANDBOX" xcode-pinned
  has "sensor 'xcode-pinned' failed (exit 2)"
}

@test "lint-ios and fmt-check-ios read every Swift file under PRODUCT_GLOBS and WEB_GLOBS but generated code, or skip by name" {
  sandbox Makefile scripts/tool-versions.env scripts/harness/unchanged.sh scripts/harness/run.sh scripts/harness/timing.sh \
    scripts/harness/file-sensors.sh scripts/harness/swiftlint.sh scripts/harness/swift-format.sh docs/harness/rules.md scripts/install-tools.sh \
    scripts/swift-format/Package.resolved
  mkdir -p "$SANDBOX/.bin" && touch "$SANDBOX/.bin/.tools-stamp" # installed: newer than its prerequisites
  # This test's globs, not the repo's (the adoption probe's differ); `kit` has no `*`, so it names what is under it.
  edit scripts/harness/layout.env 's|^PRODUCT_GLOBS=.*|PRODUCT_GLOBS=apps/*:api/*:kit|; s|^WEB_GLOBS=.*|WEB_GLOBS=apps/*:packages/*|'
  edit scripts/harness/layout.env 's|^GENERATED_GLOBS=.*|GENERATED_GLOBS=packages/api-client/src/gen/*|'
  git -C "$SANDBOX" init -q
  # Each argument in brackets: a name split in two would show.
  stub swiftlint 'printf "[%s]" swiftlint "$@"; echo; exit 2'
  stub swift-format 'printf "[%s]" swift-format "$@"; echo; exit 1'
  stub xcode-select 'echo /Library/Developer/CommandLineTools'
  HARNESS_SCOPE=full PATH="$BATS_TEST_TMPDIR/bin:$PATH" run -0 make -s -C "$SANDBOX" lint-ios fmt-check-ios
  has 'lint-ios: skipped, no Swift file under PRODUCT_GLOBS or WEB_GLOBS'
  has 'fmt-check-ios: skipped, no Swift file under PRODUCT_GLOBS or WEB_GLOBS'
  # Names git would quote (non-ASCII, `"`); a link to a directory outside the globs, whose Swift SwiftPM compiles
  # (not what sits under its hidden .build).
  local f all=('apps/ios/Q"t.swift' "apps/ios/Sources/A b.swift" "apps/ios/Über.swift" kit/Sources/L.swift kit2/M.swift
    packages/kit/K.swift packages/api-client/src/gen/G.swift tools/T.swift shared/X.swift
    shared/.build/checkouts/dep/D.swift) picked=()
  for f in "${all[@]}"; do
    mkdir -p "$SANDBOX/$(dirname "$f")" && echo 'let x = 1' >"$SANDBOX/$f"
  done
  ln -s ../../shared "$SANDBOX/apps/ios/Ext"
  local want='[apps/ios/Ext/X.swift][apps/ios/Q"t.swift][apps/ios/Sources/A b.swift][apps/ios/Über.swift][kit/Sources/L.swift][packages/kit/K.swift]'
  HARNESS_SCOPE=full PATH="$BATS_TEST_TMPDIR/bin:$PATH" run -2 make -s -C "$SANDBOX" lint-ios
  has "[swiftlint][lint][--strict][--quiet][--no-cache][--reporter][relative-path][--config][.swiftlint.yml]$want"
  run -1 grep -E 'M\.swift|G\.swift|T\.swift|D\.swift|\[shared' <<<"$output"
  HARNESS_SCOPE=full PATH="$BATS_TEST_TMPDIR/bin:$PATH" run -2 make -s -C "$SANDBOX" fmt-check-ios
  has "[swift-format][lint][--strict][--follow-symlinks][--configuration][.swift-format]$want"
  # The fast-check hook's table (file-sensors.sh) picks the same files, but for the one behind the link.
  # shellcheck source=../harness/file-sensors.sh
  source "$SANDBOX/scripts/harness/file-sensors.sh"
  for f in "${all[@]}"; do
    [[ "$(harness_file_kind "$f")" != swift ]] || picked+=("$f")
  done
  [ "${picked[*]}" == 'apps/ios/Q"t.swift apps/ios/Sources/A b.swift apps/ios/Über.swift kit/Sources/L.swift packages/kit/K.swift' ]
}
