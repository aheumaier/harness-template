# The iOS app (`IOS_DIR`)

The iOS app, the `ios` group of `STACK` ([ADR 0041](../../docs/adr/0041-ios-stack-group.md),
[exec plan 0004](../../docs/exec-plans/active/0004-ios-stack-group.md)), in the directory `IOS_DIR` of
`scripts/harness/layout.env` names (`apps/ios` in the template; a product that moves it changes `IOS_DIR`). A
product without an iOS app deletes this directory and drops `ios` from `STACK` there; `make layout-check` fails
while Swift code stays and `ios` is gone, and while `ios` stays without a `Package.swift` or `project.yml` in
`IOS_DIR`.

- The app's logic is the local Swift package here (`Package.swift`, `Sources/AppCore/`). It builds for iOS and
  macOS, so its tests run with `swift test`, without a simulator. The SwiftUI app target comes later, from an
  XcodeGen `project.yml`.
- Tests: Swift Testing, in `Tests/<Target>Tests/*Tests.swift`. A Swift test file must sit in a directory
  whose name ends in `Tests` (Xcode compiles any other file into the app). A test names the acceptance
  scenario it checks in its title, `@Test("001/US1-AS2 signs out")`; an XCTest UI test, whose method names
  cannot hold `/`, in a `// spec: 001/US1-AS2` comment
  ([specs-scenarios-tested](../../docs/harness/rules.md#specs-scenarios-tested)). Skip on the same line,
  `@Test("001/US1-AS2 signs out", .disabled("why"))`: a skip on another line is not seen.
- `make check` runs the package's tests with `swift test` (`make test-ios`) on macOS, with Xcode or with only
  Apple's command-line tools (XCTest needs Xcode selected; only Swift Testing runs with the command-line tools
  alone); on another host it says it leaves `test-ios` out
  ([ADR 0042](../../docs/adr/0042-swift-unit-tests-in-make-check.md)). Lint, format and the Xcode build
  arrive later in phase 2 of the exec plan.
- No remote Swift package dependencies yet: once there are, `Package.resolved` is committed and scanned for
  known vulnerabilities ([deps-no-known-vulns](../../docs/harness/rules.md#deps-no-known-vulns)).
