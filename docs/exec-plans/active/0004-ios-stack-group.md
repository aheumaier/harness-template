# 0004 — iOS stack group

Status: draft, awaiting the owner's decisions (below) · Owner: @aheumaier · Started: 2026-10-31

## Goal

A product built from this template can have a native iOS app (SwiftUI, under `apps/ios/`) and get the same
harness the Go API and the web app get: its acceptance scenarios traced to Swift tests, its code linted and
formatted, its tests run, its generated API client kept up to date, its Swift packages scanned, all through
an `ios` group of `STACK` ([ADR 0033](../../adr/0033-stack-profiles.md)). Without it, a template-built repo
cannot have an iOS spec go green: `specs-scenarios-tested` counts only Go and JS/TS test files.

Every item touches protected files, so each phase ships as its own human-approved harness PR. Phase 1 needs
no Xcode and runs on the Ubuntu CI as it is; it is what unblocks the first adopter.

## Why now

The first adopter (prostrength, created from 84fe092) accepted an ADR on 2026-10-31 for a native iOS app
for athletes. Its first spec, `001-athlete-account`, has 33 acceptance scenarios, about 20 of them testable
only in the iOS app. That ADR requires this template to:

- count `*Tests.swift` as test files, or the scenario check fails every iOS scenario;
- say whether the `test-writer` agent ([ADR 0039](../../adr/0039-tests-are-written-apart-from-the-implementation.md))
  writes Swift tests;
- keep `apps/ios` out of the web sensors (`WEB_GLOBS=apps/*:packages/*` matches it);
- add `ios` to both `STACK` parsers and to `stack-vectors.txt`;
- run Xcode builds in `make check-all` only (NFR-H3: `make check` ≤ 10 s warm);
- pin and scan the Swift packages (`Package.resolved`) under [ADR 0016](../../adr/0016-tool-supply-chain.md);
- say how the Xcode version is pinned and checked;
- make the iOS client accept unknown response enum values, or count a new one as breaking.

## Verified on 2026-10-31

| Claim | Result |
|-------|--------|
| A Swift test under `apps/ios/` is a "product test file" once its name matches | Yes: `PRODUCT_GLOBS=apps/*:api/*` already includes `apps/ios`, and `productTests` reads `PRODUCT_GLOBS` and `WEB_GLOBS` (`testfile.go:52-53`). Only the name pattern is missing. |
| Trivy (pinned 0.76.0) reads `Package.resolved` | Yes: `trivy fs` on a version 3 sample reports type `swift`, 1 package, its known vulnerabilities. `trivy.sh` lists only `go.mod` and `pnpm-lock.yaml` today (`has_packages`, and the "found no packages" check). |
| The contract gate flags a new response enum value | Yes: oasdiff 1.34.1 rates `response-property-enum-value-added` as an error, and `contract-check.sh` runs `oasdiff breaking --fail-on ERR`; a sample spec adding `degraded` to a response enum exits 1. oasdiff's own remediation: declare a growing set with `x-extensible-enum`. So the gate already counts it as breaking; what is missing is a rule for when to use `x-extensible-enum` and a generated client that tolerates it. |
| CI can run Xcode | No: every job is `runs-on: ubuntu-24.04`; there is no macOS runner. NFR-R5's weekly macOS selftest is a target, not a job. |
| Hosted macOS runners have Docker | No. So the template's `make setup` (Postgres), `test-integration`, `test-obs` and `e2e` cannot run there. |

## Decisions proposed

**D1. "Full stack" for the selftest means "the template's default groups", not "every known group".**
Today `selftest.sh:40-41` treats a run as full when every group in `HARNESS_STACK_GROUPS` is in `STACK`, and
under a full stack a skipped fixture is a failure (NFR-R3). Adding `ios` to the known groups while the
template's `STACK` stays `go:db:web` would make every template run "not full", so skips would quietly stop
failing. Instead, a run is full when `STACK` has every group of `HARNESS_STACK_DEFAULT` (`go:db:web`). That
is exactly today's behaviour for `go`, `db` and `web`, and keeps a Go-only product's runs non-full as now.
nfr.md's NFR-R3 row ("measured on the full stack") is reworded to say so.

**D2. iOS fixtures bring their own iOS app, and skip only without Xcode.** Under D1, a fixture whose target
is an iOS target would be "left out under the full stack", which is a failure. And `selftest.sh` works out
what is left out from the repo's own `STACK`, before the fixture is applied. So from Phase 2 on:
- iOS fixtures live in `scripts/harness/fixtures/` like every other (`fixture-claims` accepts no other place);
- each one's `apply.sh` copies the iOS skeleton (Q1) into a `PRODUCT_GLOBS` directory read from `layout.sh`,
  and adds `ios` to the copy's `STACK` itself;
- a new `fixture.conf` key `NEEDS_XCODE=1` takes the fixture out of the left-out check and skips it only when
  Xcode is missing and the caller opted in (`SELFTEST_SKIP_XCODE=1`), the way `NEEDS_DOCKER` and
  `SELFTEST_SKIP_DOCKER` work. The Ubuntu jobs opt in; the macOS job never does, so every iOS fixture runs
  somewhere on every run of it.

This is needed whatever Q1 decides. Phase 2 cannot start before D2 and Q1 are accepted.

**D3. iOS targets are left out on a host that is not macOS, as their own class.** `STACK_OFF` depends on the
stack only. From Phase 2, a `HOST_OFF` beside it holds the iOS targets when the host is not darwin. `make
check` names them on stderr only when `ios` is in `STACK` (the template and non-iOS products never see
them). A product with `ios` runs them in its macOS job (D7); its Ubuntu jobs skip them by name, never
silently.

**D4. A left-out `ios` group is found, not hidden** (ADR 0033, "fail closed, both ways"). When `ios` is not
in `STACK`, `layout-check` reports a git-known `Package.swift`, `*.xcodeproj/project.pbxproj` or `*.swift`
under a `PRODUCT_GLOBS` entry: `STACK=go:db:web: leaves out ios, but apps/ios/Package.swift exists`. The
marker pathspecs need `**`: `hiddenGroups` uses `:(glob)`, where `*` does not cross `/`. A hybrid app's shell
(`apps/mobile/ios/App/AppDelegate.swift`) and server-side Swift count as `ios` too; the amending ADR says so
(Q8).
Fixture `harness-stack-hides-ios`: its `apply.sh` writes the marker under the first `PRODUCT_GLOBS` entry
read from `layout.sh`, removes only `ios` from `STACK`, and its EXPECT matches the ios line only. Written as
`harness-stack-hides-web` is (a file in `files/apps/…` and a `STACK=web` tag), it would break the adoption
probe, whose layout has `STACK=go` and `PRODUCT_GLOBS=cmd/*:internal/*:api/*`.

**D5. Swift test files and skips.**
- A Swift test file is `*Tests.swift` inside a directory whose name ends in `Tests`: SwiftPM's
  `Tests/<X>Tests/`, Xcode's `<App>Tests/` and `<App>UITests/`. The name alone is not enough: Swift has no
  build rule like `_test.go`, and Xcode 16's synchronized folders compile any file dropped into the app's
  folder, so `apps/ios/App/LoginTests.swift` would ship in the app and still pass the `test-writer` guard.
  `testdata/test-files.txt` gets rows for both (`…/AppTests/SignOutTests.swift allow`,
  `apps/ios/App/LoginTests.swift reject`).
- A Swift test line names nothing when it has `XCTSkip` (`throw XCTSkip(…)`, `XCTSkipIf`, `XCTSkipUnless`) or
  `.disabled(` (Swift Testing `@Test(…, .disabled(…))`). These apply to `.swift` files only, so a SwiftUI
  `.disabled(` modifier or a TS line is never mistaken for a skip. `withKnownIssue` stays counted, like JS
  `.fails(`: the test runs.
- The skip check is line by line (`scenarios.go:162`). An XCTest scenario is named in a `// spec: 001/US1-AS2`
  comment (method names cannot hold `/`), and its `throw XCTSkip()` sits on another line, or in
  `setUpWithError`; that skip is not seen, the same gap Go's `t.Skip` on a following line has. rules.md says
  so and recommends Swift Testing, `@Test("001/US1-AS2 signs out")`, where a skip is
  `@Test("001/US1-AS2 signs out", .disabled("…"))` on the same line. The fixture uses that shape.
- `skippedPath` drops every path with a `fixtures/` part, so `Tests/Fixtures/XTests.swift` never counts;
  rules.md says so.

**D6. The `test-writer` guard widens with the pattern, its brief does not.** One predicate (`productTests`)
decides both what `specs-tested` reads and what the `test-writer` may write (ADR 0039), so Phase 1 lets it
write Swift test files (D5's shape) under `PRODUCT_GLOBS`. Its brief (`.claude/agents/test-writer.md`,
lines 33-38) still sends it only Go `model`/`service` and `packages/domain`; only line 30, which lists the
hook's file forms, is updated. See Q5.

**D7. Xcode runs on macOS only, never in `make check`, and not as `make check-all`.** The macOS job cannot run
the template's `check-all` (no Docker), and narrowing to `HARNESS_STACK=go:ios` on the full tree makes
`layout-check` report db and web, by design. So the job runs a `check-ios` aggregate of the iOS targets, plus
`harness-selftest`, which also closes NFR-R5 (weekly selftest on macOS).

**D8. `IOS_DIR` and the suppression change move to Phase 2.** In Phase 1 nothing would read `IOS_DIR` except
a scope shortcut in `affected.sh` (which only makes a product with both web and ios run its web sensors
unscoped when an asset catalog's `Contents.json` changes: slower, never wrong). It is added in Phase 2 with
its first real reader, as an optional key that may be empty (`optionalKeys` and `HARNESS_LAYOUT_EMPTY_OK`),
appended last in `layout.sh --print` (ADR 0033: a new key keeps older copies of `layout.env` working). The
suppression forms (`swiftlint:disable`, `swift-format-ignore`) wait for Q4.

## Open questions for the owner

Each has a recommendation. The adopter's own suggestions are marked as such; none of them is decided.

- **Q1. Where do the iOS sensors get an app to run on?**
  - (a) A minimal `apps/ios` in the template, `STACK=go:db:web:ios`, like `apps/web` today. Every adopter
    without iOS deletes it; README "(b)" and the adoption probe gain an iOS set.
  - (b) A skeleton under `scripts/templates/ios-probe/`, like the adoption probe's, copied in by the iOS
    fixtures (D2) and by the macOS job. The template's `STACK` stays `go:db:web`.
  - **Recommendation: (b).** Most products built from this template will have no iOS app, and nothing is
    left for them to delete. It needs `protected-paths.txt` and `CODEOWNERS` entries like
    `scripts/templates/adoption-probe/`, and the skeleton stays free of Swift package dependencies, or its
    `Package.resolved` would turn the template's `vuln-deps` red on someone else's vulnerability.
- **Q2. Xcode project or local Swift package?** Adopter's suggestion: an Xcode project for the app target,
  with the logic in a local Swift package (`apps/ios/Packages/…`). **Recommendation: require the package for
  logic.** `swift test` runs it without a simulator, but only if it builds for macOS (no UIKit, no iOS-only
  `platforms:`). A Foundation-only package could even run on Ubuntu with a pinned Swift toolchain, which
  would save macOS minutes and could let `make check` and the `test-writer` cover it, at the cost of Linux
  Foundation differences. Worth deciding with Q5.
- **Q3. How is the Xcode version pinned?** It cannot be digest-pinned like a `.bin` tool (ADR 0016). The
  precedent is `GO_TOOLCHAIN` in `tool-versions.env`: a version pin plus a check of what runs.
  **Recommendation:** `XCODE_VERSION` there, `xcode-select` to it in the macOS job, and a sensor that fails
  when `xcodebuild -version` differs. CI images change Xcode on their own schedule.
- **Q4. Linters.** Adopter's suggestion: SwiftLint and swift-format, both pinned. **Recommendation: agree**,
  as `.bin` tools with darwin pins only, installed only with `ios` in `STACK` and only on macOS (the Ubuntu
  jobs run `make tools` unconditionally; a tool without a Linux pin must be skipped there, not fail). Their
  suppressions need `reason: … (rule: <id>)` like every other (`integritycheck.go`).
- **Q5. Does the `test-writer` write Swift tests?** ADR 0039 scopes it to unit tests that `make check` runs,
  and its flow confirms each test is red with `make check`; its only shell is `make check`/`check-all`. D7
  keeps Swift out of `make check`. **Recommendation: not yet.** Revisit when a target it may run covers Swift
  (Q2's Ubuntu option, or `swift test` in `make check` on darwin with a `budgets.txt` row), and amend ADR
  0039 then.
- **Q6. The generated client.** Adopter's candidate: Apple's swift-openapi-generator, pinned like the other
  generators. **Recommendation: agree**, and decide in Phase 3's ADR how it handles `x-extensible-enum`.
- **Q7. macOS CI and merge rules.** GitHub cannot make a check "required only when it ran": a required job
  that a `paths:` filter never starts stays pending and blocks the merge. **Recommendation:** the
  `adoption-probe` pattern ([ci.md](../../harness/ci.md)): the macOS job always starts, and passes with a
  notice when the PR touches neither `apps/ios` nor the contract nor the iOS harness files (and in every
  product without `ios`). In this template almost every PR touches the harness, so it would run on most PRs;
  GitHub bills macOS minutes at ten times Linux for private repos (public repos are free).
- **Q8. What counts as an iOS app for D4?** **Recommendation:** any Swift under a `PRODUCT_GLOBS` entry,
  hybrid shells and server Swift included; a product that does not want the iOS sensors on it keeps that
  code outside `PRODUCT_GLOBS`.

## Touchpoints

An inventory on 2026-10-31 found about 40 places that know the groups `go`, `db` and `web` or would have to
learn a new one. By phase:

**Phase 1 (no Xcode)**
- Parsers: `layout.sh` (`HARNESS_STACK_GROUPS`, header), `layout.go` (`stackGroups`, `hiddenGroups`),
  `layout.env` (the `STACK` comment), `testdata/stack-vectors.txt`, `layout_test.go`,
  `scripts/test/layout.bats` (`known: go db web` at `layout.bats:96`, `layout_test.go:115`).
- Selftest: `selftest.sh:40-41` (D1), `selftest.bats:38` (literal group list), nfr.md NFR-R3.
- Test files: `testfile.go` (`testFileRE`, `testFileForms`, `testFilePathspecs`), `testdata/test-files.txt`
  and `only-test-files.bats` (`at()`), `scenarios.go` (`skippedTestRE`, Swift-only forms), the `specs-tested`
  file patterns and help text in the `Makefile` (about line 413), `.claude/agents/test-writer.md` line 30.
- Supply chain: `trivy.sh` (`Package.resolved` in `has_packages` and in the fail-closed check).
- Repo: `.gitignore` (`DerivedData/`, `.build/`, `xcuserdata/`, `*.xcuserstate`).
- Docs: rules.md `harness-layout-valid` and `specs-scenarios-tested`; README's `STACK` paragraphs; a new ADR
  amending ADR 0033 (groups, D1, D4, Q8).
- Fixtures: `specs-swift-test-counts` (a Swift Testing test naming a scenario counts),
  `specs-swift-test-skipped` (`.disabled(` on the same line does not), `harness-stack-hides-ios` (D4).

**Phase 2 (macOS: Xcode pin, lint, format, unit tests)**
- `Makefile`: `IOS_TARGETS`, `HOST_OFF` (D3), a `##@ iOS sensors` section, `check-ios` (D7),
  `scripts/test/make-stack.bats`.
- `layout.env`, `layout.sh`, `layout.go`: `IOS_DIR` (D8), its path check, and a non-empty `IOS_DIR` with `ios`
  left out reported like web's `API_CLIENT_DIR`.
- Selftest: `NEEDS_XCODE`, `SELFTEST_SKIP_XCODE` (D2).
- Tools: `install-tools.sh` (an `ios` block before `harness_stack_has web || exit 0`, darwin only),
  `tool-versions.env` (SwiftLint, swift-format, `XCODE_VERSION`), `budgets.txt` rows.
- `run.sh` `finding_pattern` (SwiftLint size findings → the `code-*` rules, ADR 0024); `integritycheck.go`
  suppressions; `affected.sh:79` skips `IOS_DIR`.
- `file-sensors.sh` and `.claude/hooks/fast-check.sh`: a Swift kind, if per-edit lint is wanted.
- CI: the macOS job (Q7) and the setup action's `":ios:"` steps; `REQUIRED_CHECKS` and `docs/harness/ci.md`;
  a new workflow listed in `protected-paths.txt` and `CODEOWNERS`.
- `protected-paths.txt` and `CODEOWNERS`: `*.swiftlint.yml`, `*.swift-format`, `scripts/templates/ios-probe/`.
- rules.md `ios-*` sections, each with its fixture; the review agents' Skip lists if `rule-lists` asks.
- Supply chain: fail when a Swift package with remote dependencies has no git-known `Package.resolved`.

**Phase 3 (generated client)**
- `gen-check.sh` `INPUTS`, `GENERATED_GLOBS`, a `gen-*` target in `IOS_TARGETS`, `make gen`.
- The contract rule for `x-extensible-enum` (rules.md, `api/AGENTS.md`), the generator's pin.

**Phase 4 (UI tests, layer rule, agents)**
- An `ios-e2e` target (simulator) and its budget.
- A layer rule for the app (the Swift counterpart of `go-domain-isolation` and `web-domain-pure`).
- `ARCHITECTURE.md` and `AGENTS.md` rows (human-owned: proposed in the PR description).

## Phases

| # | Phase | Exit criterion | State |
|---|-------|----------------|-------|
| 1 | `ios` group without Xcode: parsers, D1, D4, D5, D6, Trivy, `.gitignore`, rules.md, nfr.md, the amending ADR | `make check`, `make check-all` and `HARNESS_SCOPE=full make harness-selftest` green on Ubuntu; `adoption-probe` green; the three Phase 1 fixtures rejected; a product with `STACK=go:db:web:ios` and Swift tests under `apps/ios/…Tests/` passes `layout-check` and `specs-tested` | proposed |
| 2 | macOS: D2, D3, D7, D8, Xcode pin check, SwiftLint, swift-format, `swift test` | the macOS job green; each new rule rejects its fixture there; Ubuntu jobs unchanged and skipping only by name | blocked on Q1–Q4, Q7 |
| 3 | Generated Swift client | `make gen` regenerates it; `gen-check` fails on a hand edit (fixture); the `x-extensible-enum` rule documented | blocked on Q6 |
| 4 | Simulator UI tests, layer rule, `test-writer` per Q5 | `ios-e2e` in `check-ios` within its budget; the layer rule rejects its fixture | blocked on Q5 |

## Risks

- **Phase 1 makes Swift scenario tests count before any Swift sensor runs them.** A product can trace
  scenarios to Swift tests that nothing in the template runs yet. That is the adopter's ask (their tests run
  in their own Xcode until Phase 2), and rules.md says so.
- **`WEB_GLOBS=apps/*` still includes `apps/ios`.** Harmless for Swift (eslint, dependency-cruiser and jscpd
  read only JS), but JS tooling put under `apps/ios` would be linted as web. Entries must stay `<dir>/*`, so
  the fix is a documented rule, not a narrower glob.
- **A third copy of the test-file patterns.** `testFilePathspecs` and the Makefile's `specs-tested` file list
  both gain the Swift form; the skeleton (Q1) drifts with Xcode's project format, and only the macOS job
  notices.
- **TD-147 matters more once fixtures copy a skeleton in:** state that carries over between fixtures outside
  git (`.cache/`, the linked `.bin`) must not include Xcode build output.

## Process

Plan reviewed by `senior-architect-reviewer` on 2026-10-31; its findings are folded in above (D2, D3, D7, D8,
the D4 fixture, D5's directory rule and line-local skips, Q5, Q7, Q8). After the owner answers Q1–Q8, Phase 1
starts as its own branch and harness PR, with `code-reviewer`, `evasion-reviewer` and
`go-reviewer`/`shell-reviewer` on the diff.

## Log

- 2026-10-31: drafted from the adopter's request and an inventory of the harness; architecture review folded
  in.
