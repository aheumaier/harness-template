# ADR 0041: An `ios` stack group, with a Swift skeleton in the template (amends 0033, 0036, 0039)

Status: accepted · 2026-10-31 · amends [ADR 0033](0033-stack-profiles.md),
[ADR 0036](0036-acceptance-scenarios-are-traced-to-tests.md),
[ADR 0039](0039-tests-are-written-apart-from-the-implementation.md)

## Context
The template's clients are the web app only. The first product built from it decided on a native iOS app, and
its first spec has 33 acceptance scenarios, about 20 testable only in that app. `specs-scenarios-tested`
(ADR 0036) counts only Go and JS/TS test files, so that spec could not go green, and ADR 0033 knows the groups
`go`, `db` and `web` only. [Exec plan 0004](../exec-plans/active/0004-ios-stack-group.md) plans the iOS
harness in four phases; the owner decided its open questions on 2026-10-31. This ADR records phase 1, which
needs no Xcode and runs on the Ubuntu CI as it is.

## Decision
- **A fourth group, `ios`**, in both `STACK` parsers (`layout.sh`, `layout.go`) and their shared vectors. The
  template's `STACK` is `go:db:web:ios`. A `layout.env` without `STACK` still reads as `go:db:web` (ADR 0033:
  a missing key keeps the old behaviour).
- **The template ships `apps/ios`**, a local Swift package with no dependencies (`AppCore`, one test), like
  `apps/web`. A product without an iOS app deletes it and drops `ios` from `STACK`. The selftest's "full
  stack" (every known group in `STACK`; a skipped fixture fails then, NFR-R3) keeps its meaning, because the
  template has every group.
- **A left-out `ios` is found** (ADR 0033, "fail closed, both ways"): without `ios` in `STACK`, `layout-check`
  reports a git-known `*.swift` or `*.xcodeproj/project.pbxproj` under a `PRODUCT_GLOBS` entry. Any Swift
  counts, a hybrid app's shell and server-side Swift included; code that should not be judged as iOS stays
  outside `PRODUCT_GLOBS`. Fixture `harness-stack-hides-ios`.
- **Swift test files (amends ADR 0036).** A product test file is also `*Tests.swift` inside a directory whose
  name ends in `Tests` (SwiftPM's `Tests/<X>Tests/`, Xcode's `<App>Tests/` and `<App>UITests/`). The name
  alone is not enough: Swift has no build rule like `_test.go`, and Xcode compiles any file in an app target's
  folder, so `Sources/AppCore/PayTests.swift` would ship in the app. In a `.swift` file, a line with `XCTSkip`
  or Swift Testing's `.disabled(` names nothing. The check stays per line: Swift Testing names a scenario in
  its title, `@Test("001/US1-AS2 signs out")`, and skips on that line; an XCTest skip below a
  `// spec: <id>` comment is not seen, the same gap as Go's `t.Skip` on a following line.
- **The `test-writer`'s guard widens with it (amends ADR 0039).** One predicate decides what
  `specs-tested` reads and what the `test-writer` may write, so it may now write a Swift test file of that
  shape. Its brief does not send it Swift work until phase 2, when `swift test` joins `make check` on macOS and
  it can confirm a new test fails, as ADR 0039 requires.
- **Swift packages are scanned**: Trivy already reads `Package.resolved`; `trivy.sh`'s fail-closed check now
  also reports a `Package.resolved` with pins that Trivy found no packages in.

## Consequences
- A product can trace scenarios to Swift tests before the harness runs any Swift: until phase 2 the product
  runs `swift test` and its Xcode tests itself (`apps/ios/AGENTS.md` says how, also with only Apple's
  command-line tools, which need the Testing framework passed to `swift test`).
- Every adopter without iOS deletes `apps/ios`, as it deletes `apps/web` without a web app; README (a) and (b)
  say so, and the adoption probe drops it with the other optional sets.
- `WEB_GLOBS=apps/*` also covers `apps/ios`. eslint, dependency-cruiser and jscpd read only JS, so Swift is not
  judged as web; JS tooling put under `apps/ios` would be.
- A third copy of the test-file name patterns: `testFilePathspecs` and the Makefile's `specs-tested` trigger
  both gain `*Tests.swift`.
- Phases 2–4 (macOS CI, Xcode pin, SwiftLint, swift-format, `swift test` in `make check` on macOS, the
  generated client, UI tests) each get their own ADR or amendment.
