# 0004 — iOS stack group

Status: decided, Phase 1 not started · Owner: @aheumaier · Started: 2026-10-31

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

## Owner's decisions (2026-10-31)

| # | Question | Decision |
|---|----------|----------|
| Q1 | Where do the iOS sensors get an app to run on? | **A skeleton `apps/ios` in the template**, like `apps/web`; the template's `STACK` becomes `go:db:web:ios`. Adopters without iOS delete it and drop `ios` from `STACK`. |
| Q2 | Xcode project or Swift package? | **A local Swift package for the logic now; the app target later from a checked-in project spec** (XcodeGen, pinned), generated, so nobody hand-edits `project.pbxproj`. |
| Q3 | Xcode pin | `XCODE_VERSION` in `tool-versions.env` (like `GO_TOOLCHAIN`), `xcode-select` to it in the macOS job, a sensor that fails when `xcodebuild -version` differs. |
| Q4 | Linters | SwiftLint and swift-format, pinned `.bin` tools with macOS pins only, installed only with `ios` in `STACK` and only on macOS; their suppressions need a reason. |
| Q5 | Does the `test-writer` write Swift tests? | **Yes**, unit tests in the logic package. `swift test` on that package joins `make check` on macOS, with its own time budget, and is skipped by name elsewhere. ADR 0039 is amended. |
| Q6 | Generated client | Apple's swift-openapi-generator, pinned; Phase 3's ADR decides how it handles `x-extensible-enum`. |
| Q7 | macOS CI and merge rules | The macOS job always starts and passes with a notice when nothing iOS changed (the `adoption-probe` pattern), so it can be a required check. |
| Q8 | What counts as iOS code | Any Swift under a `PRODUCT_GLOBS` entry, hybrid app shells and server-side Swift included. |

This Mac (2026-10-31) has the Swift command-line tools (Swift 6.2.4) but no Xcode: `swift test` runs on a
package here; an Xcode project, `xcodebuild` and the simulator need the macOS CI job (or a Mac with Xcode).

## Design

**D1. "Full stack" keeps its meaning.** With the skeleton (Q1), the template's `STACK` has every known group,
so `selftest.sh:40-41` (a run is full when every group in `HARNESS_STACK_GROUPS` is in `STACK`; under a full
stack a skipped fixture fails, NFR-R3) needs no change. `HARNESS_STACK_DEFAULT` stays `go:db:web`: it is the
default for a `layout.env` without `STACK`, and ADR 0033 keeps that the old behaviour.

**D2. iOS fixtures skip only where there is no Xcode, and only on request.** A `fixture.conf` key
`NEEDS_XCODE=1` skips the fixture when Xcode is missing and the caller set `SELFTEST_SKIP_XCODE=1`, the way
`NEEDS_DOCKER` and `SELFTEST_SKIP_DOCKER` work. The Ubuntu jobs set it; the macOS job never does, so every
iOS fixture runs there. Fixtures that need only Swift (`swift test`) say `NEEDS_SWIFT=1` instead, so they also
run on this Mac. Fixtures stay in `scripts/harness/fixtures/` (`fixture-claims` accepts no other place).

**D3. iOS targets are left out on a host that is not macOS, as their own class.** `STACK_OFF` depends on the
stack only. A `HOST_OFF` beside it holds the iOS targets when the host is not darwin; `make check` names them
on stderr when `ios` is in `STACK` (so on Ubuntu, never silently). `selftest.sh` treats a fixture whose
target is in `HOST_OFF` like D2's skip.

**D4. A left-out `ios` group is found, not hidden** (ADR 0033, "fail closed, both ways"). When `ios` is not
in `STACK`, `layout-check` reports a git-known `Package.swift`, XcodeGen `project.yml`,
`*.xcodeproj/project.pbxproj` or `*.swift` under a `PRODUCT_GLOBS` entry (Q8), for example
`STACK=go:db:web: leaves out ios, but apps/ios/Package.swift exists`. The pathspecs need `**`: `hiddenGroups`
uses `:(glob)`, where `*` does not cross `/`. Fixture `harness-stack-hides-ios`: its `apply.sh` removes only
`ios` from `STACK` and, when the tree has no Swift (the adoption probe's repo, whose layout is `STACK=go`
and `PRODUCT_GLOBS=cmd/*:internal/*:api/*`), first writes a marker under the first `PRODUCT_GLOBS` entry read
from `layout.sh`; its EXPECT matches the ios line only.

**D5. Swift test files and skips.**
- A Swift test file is `*Tests.swift` inside a directory whose name ends in `Tests`: SwiftPM's
  `Tests/<X>Tests/`, Xcode's `<App>Tests/` and `<App>UITests/`. The name alone is not enough: Swift has no
  build rule like `_test.go`, and Xcode 16's synchronized folders compile any file dropped into the app's
  folder, so `apps/ios/App/LoginTests.swift` would ship in the app and still pass the `test-writer` guard.
  `testdata/test-files.txt` gets rows for both.
- A Swift test line names nothing when it has `XCTSkip` (`throw XCTSkip(…)`, `XCTSkipIf`, `XCTSkipUnless`) or
  `.disabled(` (Swift Testing). These apply to `.swift` files only, so a SwiftUI `.disabled(` modifier or a TS
  line is never mistaken for a skip. `withKnownIssue` stays counted, like JS `.fails(`.
- The skip check is line by line (`scenarios.go:162`). rules.md says so and recommends Swift Testing,
  `@Test("001/US1-AS2 signs out")`, skipped as `@Test("001/US1-AS2 signs out", .disabled("…"))` on the same
  line. XCTest names a scenario in a `// spec: 001/US1-AS2` comment (method names cannot hold `/`), and an
  `XCTSkip` on another line is not seen, the same gap Go's `t.Skip` on a following line has.
- `skippedPath` drops every path with a `fixtures/` part, so `Tests/Fixtures/XTests.swift` never counts.

**D6. The `test-writer` writes Swift unit tests (Q5).** One predicate (`productTests`) decides what
`specs-tested` reads and what the `test-writer` may write (ADR 0039), so D5 widens both at once. Its brief
(`.claude/agents/test-writer.md`) adds unit tests in the logic package's `Tests/` beside Go `model`/`service`
and `packages/domain`; UI tests stay out, as `apps/web` does today. It confirms red with `make check`, which
runs `swift test` on macOS (Q5). On Linux it cannot confirm a Swift test and says so instead of writing one.

**D7. Xcode runs on macOS only and never in `make check`; the macOS job does not run `check-all`.** Hosted
macOS runners have no Docker, so the template's `make setup` (Postgres), `test-integration`, `test-obs` and
`e2e` cannot run there, and narrowing `STACK` makes `layout-check` report db and web, by design. The job runs
a `check-ios` aggregate (Xcode pin, lint, format, `swift test`, the app build, later UI tests) plus
`harness-selftest`, which also closes NFR-R5 (selftest on macOS). `swift test` alone joins `make check` on
macOS (Q5); `xcodebuild` stays in `check-ios`.

**D8. `IOS_DIR=apps/ios` is an optional layout key that may be empty** (`optionalKeys` and
`HARNESS_LAYOUT_EMPTY_OK`), appended last in `layout.sh --print` (ADR 0033: a new key keeps older copies of
`layout.env` working; base-branch readers run a parser at least as new). It arrives in Phase 2 with its first
readers (the iOS targets, `affected.sh:79`, which today sends an asset catalog's `Contents.json` to an
unscoped web run). The suppression forms (`swiftlint:disable`, `swift-format-ignore`) arrive with the linters.

**D9. The skeleton is small and dependency-free.** Phase 1: `apps/ios/Package.swift` (platforms iOS and
macOS, so `swift test` runs without a simulator), one source file in `Sources/AppCore/`, one test in
`Tests/AppCoreTests/`, and an `apps/ios/AGENTS.md` (with its `CLAUDE.md`, `docs-links`). No remote Swift
packages, so its `Package.resolved` holds nothing for Trivy to report. Module names are neutral (`AppCore`),
so `make rename` has nothing to rewrite there. Phase 2 adds the app target as an XcodeGen `project.yml`.

## Touchpoints

An inventory on 2026-10-31 found about 40 places that know the groups `go`, `db` and `web` or would have to
learn a new one. By phase:

**Phase 1 (Ubuntu CI as it is; `swift test` only on this Mac)**
- Skeleton (D9); `layout.env` `STACK=go:db:web:ios` and its comment.
- Parsers: `layout.sh` (`HARNESS_STACK_GROUPS`, header), `layout.go` (`stackGroups`, `hiddenGroups`),
  `testdata/stack-vectors.txt`, `layout_test.go`, `scripts/test/layout.bats` (`known: go db web` at
  `layout.bats:96`, `layout_test.go:115`), `selftest.bats:38`.
- Test files: `testfile.go` (`testFileRE`, `testFileForms`, `testFilePathspecs`), `testdata/test-files.txt`
  and `only-test-files.bats` (`at()`), `scenarios.go` (`skippedTestRE`, Swift-only forms), the `specs-tested`
  file patterns and help text in the `Makefile` (about line 413).
- Supply chain: `trivy.sh` (`Package.resolved` in `has_packages` and in the fail-closed check).
- Repo: `.gitignore` (`DerivedData/`, `.build/`, `.swiftpm/`, `xcuserdata/`, `*.xcuserstate`).
- Adoption: README (a) (adopters without iOS delete `apps/ios` and drop `ios`), README (b) and the adoption
  probe's `drop_sets` (an ios set), `scripts/templates/adoption-probe/` if a fixture needs a rewrite.
- Docs: rules.md `harness-layout-valid` and `specs-scenarios-tested`; README's `STACK` paragraphs; nfr.md
  NFR-R3 wording if needed; a new ADR amending ADR 0033 (the group, D4, Q8, the skeleton).
- Fixtures: `specs-swift-test-counts` (a Swift Testing test naming a scenario counts),
  `specs-swift-test-skipped` (`.disabled(` on the same line does not), `specs-swift-test-outside-tests`
  (`apps/ios/Sources/AppCore/LoginTests.swift` naming a scenario does not count), `harness-stack-hides-ios`.

**Phase 2 (macOS job: Xcode pin, lint, format, `swift test`, the app target, the `test-writer`)**
- CI: the macOS job (Q7), the setup action's `":ios:"` steps, `REQUIRED_CHECKS` and `docs/harness/ci.md`, the
  workflow in `protected-paths.txt` and `CODEOWNERS`; the Ubuntu jobs set `SELFTEST_SKIP_XCODE=1`.
- `Makefile`: `IOS_TARGETS`, `HOST_OFF` (D3), a `##@ iOS sensors` section, `check-ios` (D7), `test-ios`
  (`swift test`) in `make check` on darwin (Q5), `scripts/test/make-stack.bats`, `budgets.txt` rows.
- `layout.env`, `layout.sh`, `layout.go`: `IOS_DIR` (D8) and its path check; `affected.sh:79`.
- Selftest: `NEEDS_XCODE`, `NEEDS_SWIFT`, `SELFTEST_SKIP_XCODE` (D2).
- Tools: `install-tools.sh` (an `ios` block before `harness_stack_has web || exit 0`, darwin only),
  `tool-versions.env` (SwiftLint, swift-format, XcodeGen, `XCODE_VERSION`).
- `run.sh` `finding_pattern` (SwiftLint size findings → the `code-*` rules, ADR 0024); `integritycheck.go`
  suppressions.
- The app target: `apps/ios/project.yml`, the generated project ignored or checked by `gen-check`.
- `.claude/agents/test-writer.md` and ADR 0039's amendment (D6); `file-sensors.sh` and
  `.claude/hooks/fast-check.sh` (a Swift kind).
- `protected-paths.txt` and `CODEOWNERS`: `*.swiftlint.yml`, `*.swift-format`.
- rules.md `ios-*` sections, each with its fixture; the review agents' Skip lists if `rule-lists` asks.
- Supply chain: fail when a Swift package with remote dependencies has no git-known `Package.resolved`.

**Phase 3 (generated client)**
- `gen-check.sh` `INPUTS`, `GENERATED_GLOBS`, a `gen-*` target in `IOS_TARGETS`, `make gen`.
- The contract rule for `x-extensible-enum` (rules.md, `api/AGENTS.md`), the generator's pin.

**Phase 4 (UI tests, layer rule)**
- An `ios-e2e` target (simulator) in `check-ios` and its budget.
- A layer rule for the app (the Swift counterpart of `go-domain-isolation` and `web-domain-pure`).
- `ARCHITECTURE.md` and `AGENTS.md` rows (human-owned: proposed in the PR description).

## Phases

| # | Phase | Exit criterion | State |
|---|-------|----------------|-------|
| 1 | `ios` group and skeleton, no Xcode: D1, D4, D5, D9, Trivy, `.gitignore`, adoption, rules.md, the amending ADR | `make check`, `make check-all` and `HARNESS_SCOPE=full make harness-selftest` green on Ubuntu; `adoption-probe` green; the four Phase 1 fixtures rejected; `swift test` on the skeleton green on this Mac; a product with `STACK=go:db:web:ios` and Swift tests under `apps/ios/Tests/…Tests/` passes `layout-check` and `specs-tested` | next |
| 2 | macOS job, D2, D3, D6, D7, D8, Xcode pin check, SwiftLint, swift-format, `swift test` in `make check` on macOS, the app target | the macOS job green and required; each new rule rejects its fixture there; Ubuntu jobs skip iOS only by name | after 1 |
| 3 | Generated Swift client | `make gen` regenerates it; `gen-check` fails on a hand edit (fixture); the `x-extensible-enum` rule documented | after 2 |
| 4 | Simulator UI tests, layer rule | `ios-e2e` in `check-ios` within its budget; the layer rule rejects its fixture | after 3 |

## Risks

- **Until Phase 2, nothing in CI runs Swift.** Phase 1 makes Swift scenario tests count before the macOS job
  exists; the adopter runs them in their own Xcode until then, and rules.md says so.
- **Every adopter without iOS deletes `apps/ios`** (Q1). README (a) and (b) say how; `layout-check` (D4)
  fails if Swift stays while `ios` is dropped.
- **`WEB_GLOBS=apps/*` includes `apps/ios`.** Harmless for Swift (eslint, dependency-cruiser and jscpd read
  only JS), but JS tooling put under `apps/ios` would be linted as web. Entries must stay `<dir>/*`, so the fix
  is a documented rule, not a narrower glob.
- **macOS minutes.** In this template almost every PR touches the harness, so the always-started macOS job
  runs real work on most of them; GitHub bills macOS at ten times Linux for private repos (public: free).
- **A third copy of the test-file patterns** (`testFilePathspecs` and the Makefile's `specs-tested` list).
- **TD-147 matters more once fixtures build Swift:** `.build/` and Xcode output must not carry over between
  fixtures (`.gitignore` covers git, not the selftest's shared state).

## Process

Plan reviewed by `senior-architect-reviewer` on 2026-10-31 before the owner's decisions; its findings are in
D2–D8 and the D4 fixture. The decisions (Q1 = skeleton, Q5 = yes) removed the original D1 change and moved
the `test-writer` and `swift test` into Phase 2. Each phase is its own branch and harness PR, with
`code-reviewer`, `evasion-reviewer` and `go-reviewer`/`shell-reviewer` on the diff, and
`senior-architect-reviewer` again at each phase's end.

## Log

- 2026-10-31: drafted from the adopter's request and an inventory of the harness; architecture review folded
  in; the owner decided Q1–Q8.
