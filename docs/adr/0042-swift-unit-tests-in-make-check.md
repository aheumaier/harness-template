# ADR 0042: Swift unit tests run in `make check` on macOS (amends 0033, 0039, 0041)

Status: proposed · 2026-11-01 · amends [ADR 0033](0033-stack-profiles.md),
[ADR 0039](0039-tests-are-written-apart-from-the-implementation.md), [ADR 0041](0041-ios-stack-group.md) · amended
2026-11-02 by [ADR 0044](0044-ios-ci-on-macos.md) (`swift-tests`: every Swift file in `Tests/` is compiled)

## Context
[ADR 0041](0041-ios-stack-group.md) (phase 1 of [exec plan 0004](../exec-plans/active/0004-ios-stack-group.md))
made Swift tests count for `specs-scenarios-tested`, but no sensor runs them, so the `test-writer` cannot confirm
that a new Swift test fails (ADR 0039) and its brief keeps it away from Swift. The owner decided on 2026-10-31 that
`swift test` joins `make check` on macOS and is skipped by name elsewhere (plan question Q5), that the app's logic
is a Swift package (Q2), and that the macOS CI job comes later in phase 2 (Q7). Phase 2 ships in three PRs; this
ADR records the first. Lint, format, the Xcode pin and the macOS CI job get their own ADRs.

## Decision
- **`IOS_DIR` in `layout.env`**: the iOS app's directory (`apps/ios` in the template). Optional and may be empty,
  as `UPSTREAM_SPECS` is, so an older `layout.env` still parses; appended last in `layout.sh --print`. With `ios`
  in `STACK`, `layout-check` fails when `IOS_DIR` is unset, empty, not a clean path inside the repo, not under a
  `PRODUCT_GLOBS` or `WEB_GLOBS` entry (its Swift tests would not count for `specs-tested`, and the `test-writer`
  could not write them), or holds no git-known `Package.swift` or `project.yml` (fail closed, ADR 0033). It is a literal path; a glob is not expanded. Fixture
  `harness-stack-ios-without-app`.
- **`make test-ios`** runs `swift test` in `IOS_DIR` (rule `ios-tests-pass`, fixture `ios-failing-test`) and is
  part of `make check`, through `IOS_TARGETS`, the ios group's targets. Like the other scoped sensors it is
  skipped while no `*.swift`, `Package.resolved` or file in `IOS_DIR` changed against the base; `make check-all`
  runs it. Budget: 3 s warm (`budgets.txt`, NFR-H5), measured at 0.4 to 0.6 s on Apple Silicon.
- **Only Apple's command-line tools are enough.** While they are the selected developer directory
  (`xcode-select -p` ends in `/CommandLineTools`), `swift test` finds Swift Testing only when the tools' frameworks directory is passed as
  `-F` and `-rpath`; `test-ios` adds these flags then, and only then. XCTest needs Xcode selected.
- **`HOST_OFF`, beside `STACK_OFF`**: the targets this host leaves out, the ios targets on any OS but macOS (from
  `uname -s`). `make check` drops them and names them on stderr ("this host (not macOS) leaves out: test-ios"),
  so a Linux run is never mistaken for one that tested Swift. Like `STACK_OFF` it cannot be set from the command
  line or the environment, and a command-line group target list that feeds both (`IOS_TARGETS`, and
  `DB_TARGETS` and `WEB_TARGETS` with it) is refused too; `HARNESS_STACK` remains the one-run override.
- **Selftest fixtures declare `NEEDS_SWIFT=1`** when they need Swift on macOS; a fixture whose `TARGET` is an ios
  target without it is a bad `fixture.conf`. Without Swift, or on a host whose `HOST_OFF` holds the target, such
  a fixture fails unless `SELFTEST_SKIP_SWIFT=1` skips it, as `NEEDS_DOCKER` and `SELFTEST_SKIP_DOCKER` do.
  CI's Ubuntu `harness-selftest` job sets it.
- **Ruling: the selftest's "full stack" is every group the repo's own `layout.env` declares** (option (b) of the
  open question in the phase 2 prompt, 2026-11-01; amends ADR 0033 and ADR 0041, which defined it as every known
  group). Under the full stack a skip still fails, except one for an undeclared group or its targets. The
  template declares every group, so nothing changes for it. A product with `STACK=go:db:web` keeps "a skip
  fails" for its own groups, which option (a) would have lost as soon as the ios group existed. A product with
  `ios` that runs its selftest on Linux sets `SELFTEST_SKIP_SWIFT=1` (and, with the next PR, the Xcode
  equivalent), so every iOS fixture carries a `NEEDS_*` tag: the condition the prompt set for (b).
- **The `test-writer` writes Swift unit tests (amends ADR 0039).** Its brief adds Swift Testing tests in the
  `Tests/` of the package at `IOS_DIR`; XCTest UI tests stay with the implementer. Swift's red for a missing
  name is `cannot find '…' in scope` or `no such module '…'`. On a host that is not macOS it says it could not
  confirm its Swift tests red.
- **`affected.sh` treats an asset catalog in `IOS_DIR` as no web code** (only while `STACK` has ios): its
  `*.xcassets/**/Contents.json` no longer makes the web sensors run on the whole workspace set; other JSON, JS
  or TS there still does, since web code may import it.

## Consequences
- A product with `ios` in `STACK` and a Mac gets its Swift unit tests run by `make check`, and its `test-writer`
  writes them. On Linux nothing tests Swift until the macOS CI job of the third phase 2 PR.
- A product whose iOS app is not at `apps/ios` sets `IOS_DIR`, under one of its `PRODUCT_GLOBS` or `WEB_GLOBS`; one that drops `ios` from `STACK` may leave
  `IOS_DIR` set or empty.
- A product that runs `make harness-selftest` on Linux with `ios` in `STACK` needs `SELFTEST_SKIP_SWIFT=1`
  there, or the `ios-failing-test` fixture fails.
- An app with only an XcodeGen `project.yml` in `IOS_DIR` passes `layout-check`, and `make test-ios` prints
  that it skipped (no `Package.swift` and no Swift file in `Tests/`) instead of letting `swift test` search the
  parent folders. Swift files in `Tests/` without a `Package.swift` fail it (fixture `ios-tests-without-package`),
  so deleting the package does not silence its tests. The Xcode build of the third phase 2 PR is what tests such
  an app.
- A product that adopted phase 1 with `ios` in `STACK` adds `IOS_DIR=<its app's directory>` to its `layout.env`
  when it syncs this change; until then `layout-check` fails with "IOS_DIR unset".
- Until the macOS CI job of the third phase 2 PR, no CI job runs `test-ios` or checks that the `ios-failing-test`
  fixture is rejected (Ubuntu skips it): a harness PR that touches the ios sensors runs `make harness-selftest` on
  a Mac before it merges.
- `make check` on macOS and on Linux differ by `HOST_OFF` on purpose: NFR-R5 excludes those targets.
- The Makefile's ios targets and the selftest's `NEEDS_SWIFT` list are two copies of one fact; the bad
  `fixture.conf` check keeps them equal for `TARGET`, but an `apply.sh` that needs Swift is not found.
