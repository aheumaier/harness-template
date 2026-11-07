# The iOS app (`IOS_DIR`)

The iOS app, the `ios` group of `STACK` ([ADR 0041](../../docs/adr/0041-ios-stack-group.md),
[exec plan 0004](../../docs/exec-plans/active/0004-ios-stack-group.md)), in the directory `IOS_DIR` of
`scripts/harness/layout.env` names (`apps/ios` in the template; a product that moves it changes `IOS_DIR`). A
product without an iOS app deletes this directory and drops `ios` from `STACK` there; `make layout-check` fails
while Swift code stays and `ios` is gone, and while `ios` stays without a `Package.swift` or `project.yml` in
`IOS_DIR`.

- The app's logic is the local Swift package here (`Package.swift`, `Sources/AppCore/`). It builds for iOS and
  macOS, so its tests run with `swift test`, without a simulator. Keep logic there, not in the app target.
- The SwiftUI app target is described by `project.yml` (XcodeGen): target and scheme `App`, sources in `App/`,
  depending on `AppCore`. The generated `App.xcodeproj` is git-ignored: edit `project.yml`, never the project.
  `make setup` generates it where Xcode is selected, and `make ios-project` regenerates it
  ([ADR 0044](../../docs/adr/0044-ios-ci-on-macos.md)).
- Tests: Swift Testing, in `Tests/<Target>Tests/*Tests.swift`. A Swift test file must sit in a directory
  whose name ends in `Tests` (Xcode compiles any other file into the app). A test names the acceptance
  scenario it checks in its title, `@Test("001/US1-AS2 signs out")`; an XCTest UI test, whose method names
  cannot hold `/`, in a `// spec: 001/US1-AS2` comment
  ([specs-scenarios-tested](../../docs/harness/rules.md#specs-scenarios-tested)). Skip on the same line,
  `@Test("001/US1-AS2 signs out", .disabled("why"))`: a skip on another line is not seen.
- `make check` runs the package's tests with `swift test` (`make test-ios`) on macOS, with Xcode or with only
  Apple's command-line tools (XCTest needs Xcode selected; only Swift Testing runs with the command-line tools
  alone); on another host it says it leaves `test-ios` out
  ([ADR 0042](../../docs/adr/0042-swift-unit-tests-in-make-check.md)).
- `make check` also runs SwiftLint (`make lint-ios`: [ios-lint](../../docs/harness/rules.md#ios-lint) and the
  size rules, tests included) and swift-format (`make fmt-check-ios`:
  [ios-format](../../docs/harness/rules.md#ios-format)) on every Swift file under `PRODUCT_GLOBS` or
  `WEB_GLOBS`, with the repo-root `.swiftlint.yml` and `.swift-format`; `make fmt` formats Swift. A suppression
  is `// swiftlint:disable:next <rule> - reason: … (rule: <id>)`; `// swift-format-ignore` is never allowed.
  `make check-ios` adds the check that the selected Xcode is `XCODE_VERSION`
  ([ios-xcode-pinned](../../docs/harness/rules.md#ios-xcode-pinned), [ADR 0043](../../docs/adr/0043-swift-lint-format-and-xcode-pin.md))
  and the app's simulator build (`make build-ios`: [ios-app-builds](../../docs/harness/rules.md#ios-app-builds));
  CI's `check-ios` job runs it on macOS with Xcode on every PR and push to main that touches Swift, this
  directory or a harness file the iOS sensors read ([ADR 0044](../../docs/adr/0044-ios-ci-on-macos.md)). A Swift file in `Tests/` must belong to a test target in `Package.swift`, or `make test-ios`
  fails ([ios-tests-pass](../../docs/harness/rules.md#ios-tests-pass)).
- No remote Swift package dependencies yet: once there are, commit `Package.resolved` with them
  ([deps-swift-resolved](../../docs/harness/rules.md#deps-swift-resolved)); it is scanned for known
  vulnerabilities ([deps-no-known-vulns](../../docs/harness/rules.md#deps-no-known-vulns)). Add remote packages to
  `Package.swift`, not `project.yml`: the generated project's lockfile is not in git.
- `project.yml` carries the template's bundle identifier (`bundleIdPrefix: com.example`,
  `PRODUCT_BUNDLE_IDENTIFIER: com.example.app`); `make rename` does not change it, so set the product's own.
