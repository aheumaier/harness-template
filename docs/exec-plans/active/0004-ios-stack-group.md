# 0004 — iOS stack group

Status: phase 1 merged (PR #12); phase 2a merged (PR #14); phase 2b merged (PR #15, 095e846); phase 2c merged (PR #17, 750c094), its macOS job not yet run · Owner: @aheumaier · Started: 2026-10-31

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
| Q8 | What counts as iOS code | Any Swift under a `PRODUCT_GLOBS` entry, hybrid app shells and server-side Swift included. Built as `PRODUCT_GLOBS` or `WEB_GLOBS`, the places where Swift tests count (architecture review of phase 1). |

This Mac (2026-10-31) has Apple's command-line tools (Swift 6.2.4) but no Xcode. Plain `swift test` fails
there with `no such module 'Testing'` (and `'XCTest'`): the tools ship Swift Testing in
`/Library/Developer/CommandLineTools/Library/Developer/Frameworks`, which `swift test` finds only when passed
`-Xswiftc -F -Xswiftc <dir> -Xlinker -rpath -Xlinker <dir>`; then the skeleton's tests pass. Phase 2's
`swift test` target must add those flags when `xcode-select -p` is the command-line tools. An Xcode project,
`xcodebuild` and the simulator need Xcode (the macOS CI job, or a Mac with Xcode).

## Design

**D1. "Full stack" keeps its meaning.** Superseded on 2026-11-01: full means every group `layout.env` declares
(ruling (b), [ADR 0042](../../adr/0042-swift-unit-tests-in-make-check.md)). With the skeleton (Q1), the template's `STACK` has every known group,
so `selftest.sh:40-41` (a run is full when every group in `HARNESS_STACK_GROUPS` is in `STACK`; under a full
stack a skipped fixture fails, NFR-R3) needs no change. `HARNESS_STACK_DEFAULT` stays `go:db:web`: it is the
default for a `layout.env` without `STACK`, and ADR 0033 keeps that the old behaviour.

**D2. iOS fixtures skip only where there is no Xcode, and only on request.** A `fixture.conf` key
`NEEDS_XCODE=1` skips the fixture when Xcode is missing and the caller set `SELFTEST_SKIP_XCODE=1`, the way
`NEEDS_DOCKER` and `SELFTEST_SKIP_DOCKER` work. The Ubuntu jobs set it; the macOS job never does, so every
iOS fixture runs there. Fixtures that need only Swift (`swift test`) say `NEEDS_SWIFT=1` instead, so they also
run on this Mac. Fixtures stay in `scripts/harness/fixtures/` (`fixture-claims` accepts no other place).
As built (2026-11-01, superseded by [ADR 0043](../../adr/0043-swift-lint-format-and-xcode-pin.md)): off macOS
an ios fixture's target is in `HOST_OFF`, so `SELFTEST_SKIP_SWIFT=1` skips both kinds there (D3);
`SELFTEST_SKIP_XCODE=1` is only for a Mac with the command-line tools and no Xcode.

**D3. iOS targets are left out on a host that is not macOS, as their own class.** `STACK_OFF` depends on the
stack only. A `HOST_OFF` beside it holds the iOS targets when the host is not darwin; `make check` names them
on stderr when `ios` is in `STACK` (so on Ubuntu, never silently). `selftest.sh` treats a fixture whose
target is in `HOST_OFF` like D2's skip.

**D4. A left-out `ios` group is found, not hidden** (ADR 0033, "fail closed, both ways"). When `ios` is not
in `STACK`, `layout-check` reports a git-known `*.swift` or `*.xcodeproj/project.pbxproj` under a
`PRODUCT_GLOBS` or `WEB_GLOBS` entry (Q8), for example
`STACK=go:db:web: leaves out ios, but apps/ios/Package.swift exists`. It lists those files once (`gitKnown`, so a
`fixtures/` folder inside the app still counts) and keeps the ones `matchesAny` puts under an entry, the
predicate `productTests` uses, so an entry with or without a trailing `*` works the same. XcodeGen's
`project.yml` (an app with no Swift) is phase 2's. Fixture `harness-stack-hides-ios`: its `apply.sh` writes a
Swift marker under the first `PRODUCT_GLOBS` entry read from `layout.sh` (so it holds in the adoption probe's
layout, `STACK=go`, `PRODUCT_GLOBS=cmd/*:internal/*:api/*`) and removes only `ios` from `STACK`; its EXPECT
matches the ios line only.

**D5. Swift test files and skips.**
- A Swift test file is `*Tests.swift` inside a directory whose name ends in `Tests` (a bare `Tests/` is in no
  target), not under a SwiftPM source folder, `Sources/`, `Source/`, `src/` or `srcs/` (SwiftPM compiles every
  subfolder there): SwiftPM's `Tests/<X>Tests/`, Xcode's
  `<App>Tests/` and `<App>UITests/`. The name alone is not enough: Swift has no
  build rule like `_test.go`, and Xcode 16's synchronized folders compile any file dropped into the app's
  folder, so `apps/ios/App/LoginTests.swift` would ship in the app and still pass the `test-writer` guard.
  `testdata/test-files.txt` gets rows for both.
- A Swift test line names nothing when it has `XCTSkip` (`throw XCTSkip(…)`, `XCTSkipIf`, `XCTSkipUnless`),
  `.disabled(` or `.enabled(` with any condition (Swift Testing, with optional spaces or backticks), and neither
  does a line inside a bare `#if false` (its `#else` branch counts; other ways to compile code out: TD-150). These apply to `.swift` files only, so a SwiftUI `.disabled(` modifier or a TS
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
runs `swift test` on macOS (Q5). On Linux it writes the Swift test and says it could not confirm it red (ADR 0042).

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
unscoped web run). The suppression forms (`swiftlint:disable`, `swift-format-ignore`) arrive with the linters;
as built in 2b, `swiftlint:disable` needs a reason and `swift-format-ignore` is never allowed (ADR 0043).

**D9. The skeleton is small and dependency-free.** Phase 1: `apps/ios/Package.swift` (platforms iOS and
macOS, so `swift test` runs without a simulator), one source file in `Sources/AppCore/`, two tests in
`Tests/AppCoreTests/`, and an `apps/ios/AGENTS.md` (with its `CLAUDE.md`, `docs-links`). No remote Swift
packages, so it has no `Package.resolved` yet. Module names are neutral (`AppCore`),
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
- Adoption: README (a) (adopters without iOS delete `apps/ios` and drop `ios`). README (b) step 3 never copies
  `apps/ios`, so neither (b) nor the adoption probe's `drop_sets` needs an ios set; the fixtures write their
  Swift under the probe's own `PRODUCT_GLOBS`.
- Docs: rules.md `harness-layout-valid` and `specs-scenarios-tested`; README's `STACK` paragraphs; nfr.md
  NFR-R3 wording if needed; a new ADR amending ADR 0033 (the group, D4, Q8, the skeleton).
- Fixtures: `specs-swift-test-skipped` (`.disabled(` on the same line names nothing),
  `specs-swift-test-outside-tests` (a `*Tests.swift` in an app source folder is no test file),
  `harness-stack-hides-ios`. That a Swift test does count is a case in `scenarios_test.go`: a fixture proves a
  rejection, not a pass.

**Phase 2 (macOS job: Xcode pin, lint, format, `swift test`, the app target, the `test-writer`)**
- From the phase 1 architecture review, to settle at phase 2's start:
  - `ios` in `STACK` must need an iOS app: a non-empty `IOS_DIR` with a git-known `Package.swift` or
    `project.yml`, with a fixture (ADR 0033's "both ways" is half done for ios: a Go-only adopter who deletes
    `apps/ios` but keeps `ios` gets no warning). Done in 2a: `IOS_DIR` and fixture `harness-stack-ios-without-app`.
  - `swift test` in `make check` needs an `unchanged.sh` gate on Swift inputs to hold NFR-H3, and the
    command-line tools' `-F`/`-rpath` flags when `xcode-select -p` is the command-line tools. Done in 2a.
  - The selftest's "full stack" is now `go:db:web:ios`, so every product that drops `ios` loses NFR-R3's
    "a skip fails" once iOS-only targets exist: decide whether full means "every group the product declares". Decided 2026-11-01: yes (ADR 0042).
  - XcodeGen's `project.yml` marks an iOS app for D4 and decides Xcode test folders by target, which a path
    cannot (a `*Tests` subfolder of an Xcode app folder counts today). D4 done in 2c; test folders by target not yet
    ([TD-153](../tech-debt-tracker.md)).
  - Re-point the literal `apps/ios` in `test-writer.md`, `apps/ios/AGENTS.md` and the `layout.env` comment to
    `IOS_DIR`. Done in 2a.
- CI (2c): the macOS job (Q7), the setup action's `":ios:"` steps, `REQUIRED_CHECKS` and `docs/harness/ci.md`,
  the workflow in `protected-paths.txt` and `CODEOWNERS`; the Ubuntu jobs' `SELFTEST_SKIP_XCODE=1` is redundant
  there (`SELFTEST_SKIP_SWIFT=1` covers both off macOS, see D2). From 2b's architecture review: the job selects
  `/Applications/Xcode_${XCODE_VERSION}.app` and fails when it is missing, asserts that `xcode-select -p` is not
  the command-line tools (or `xcode-pinned` would skip), starts on every PR and passes with a notice when nothing
  iOS changed (Q7), and caches `.bin/swift-format` under a key of `SWIFT_FORMAT_COMMIT` and the sha256 of
  `scripts/swift-format/Package.resolved` (TD-156). Done in 2c.
- `Makefile`: `IOS_TARGETS`, `HOST_OFF` (D3), a `##@ iOS sensors` section, `check-ios` (D7), `test-ios`
  (`swift test`) in `make check` on darwin (Q5), `scripts/test/make-stack.bats`, `budgets.txt` rows. Done in 2a
  and 2b.
- `layout.env`, `layout.sh`, `layout.go`: `IOS_DIR` (D8) and its path check; `affected.sh:79`.
- Selftest: `NEEDS_XCODE`, `NEEDS_SWIFT`, `SELFTEST_SKIP_XCODE` (D2). Done in 2a and 2b.
- Tools: `install-tools.sh` (an `ios` block before `harness_stack_has web || exit 0`, darwin only),
  `tool-versions.env` (SwiftLint, swift-format, XcodeGen, `XCODE_VERSION`). Done in 2b, XcodeGen in 2c.
- `run.sh` `finding_pattern` (SwiftLint size findings → the `code-*` rules, ADR 0024); `integritycheck.go`
  suppressions. Done in 2b.
- The app target: `apps/ios/project.yml`, the generated project ignored or checked by `gen-check`. Done in 2c
  (ignored).
- `.claude/agents/test-writer.md` and ADR 0039's amendment (D6); `file-sensors.sh` and
  `.claude/hooks/fast-check.sh` (a Swift kind). The hook and `file-sensors.sh` done in 2b.
- `protected-paths.txt` and `CODEOWNERS`: `*.swiftlint.yml`, `*.swift-format`. Done in 2b (root configs only,
  as the callers always name them).
- rules.md `ios-*` sections, each with its fixture; the review agents' Skip lists if `rule-lists` asks. Done in
  2a and 2b for their rules.
- Supply chain: fail when a Swift package with remote dependencies has no git-known `Package.resolved`. Done in
  2c (`deps-swift-resolved`).

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
| 1 | `ios` group and skeleton, no Xcode: D1, D4, D5, D9, Trivy, `.gitignore`, adoption, rules.md, ADR 0041 | `make check`, `make check-all` and `HARNESS_SCOPE=full make harness-selftest` green on Ubuntu; `adoption-probe` green; the three Phase 1 fixtures rejected; `swift test` on the skeleton green on this Mac; a product with `STACK=go:db:web:ios` and Swift tests under `apps/ios/Tests/…Tests/` passes `layout-check` and `specs-tested` | merged (PR #12) |
| 2 | macOS job, D2, D3, D6, D7, D8, Xcode pin check, SwiftLint, swift-format, `swift test` in `make check` on macOS, the app target | the macOS job green and required; each new rule rejects its fixture there; Ubuntu jobs skip iOS only by name | 2a (`IOS_DIR`, `HOST_OFF`, `test-ios`, `NEEDS_SWIFT`, the `test-writer`; ADR 0042) merged (PR #14); 2b (SwiftLint, swift-format, the Xcode pin, `IOS_SWIFT_TARGETS`/`IOS_XCODE_TARGETS`, `NEEDS_XCODE`, the Swift kind in the per-file sensors; ADR 0043) merged (PR #15, 095e846); 2c (the macOS job `check-ios`, the app target, XcodeGen, `build-ios`, the `Package.resolved` and undeclared-test checks; ADR 0044) merged (PR #17, 750c094); pending: XcodeGen run by its real path (the app did not build, fix/ios-xcodegen-presets), the first green `check-ios` run, its budget rows |
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
  in; the owner decided Q1–Q8. Phase 1 built on `feat/ios-stack-phase1` (ADR 0041).
- 2026-11-01: phase 1 merged (PR #12) after the obs-e2e flake fix (PR #11); ADR 0041 accepted. Phase 2 split
  into three PRs: 2a `swift test` in `make check`, 2b lint,
  format and the Xcode pin, 2c the macOS job and the app target. Built 2a on `feat/ios-swift-test`
  ([ADR 0042](../../adr/0042-swift-unit-tests-in-make-check.md)); ruled the selftest's full stack to be every
  group `layout.env` declares (option (b)), recorded there. For 2b, from 2a's architecture review: the Xcode
  pin check is an ios target that needs Xcode, so `IOS_TARGETS` splits into what `make check` runs (Swift only)
  and what `check-ios` runs (Xcode), the bad-`fixture.conf` rule becomes "`NEEDS_SWIFT` or `NEEDS_XCODE`", and
  a Mac with only the command-line tools is a third host class beside `HOST_OFF`'s "not macOS". In 2c, the
  `layout_test.go` trees that use `apps/ios/project.yml` as a neutral file need another once `project.yml`
  marks an iOS app (D4).
- 2026-11-01: 2a merged (PR #14). Built 2b on `feat/ios-lint`
  ([ADR 0043](../../adr/0043-swift-lint-format-and-xcode-pin.md)), with these rulings recorded there: Swift tests
  are not exempt from the size rules (TD-154); SwiftLint's size thresholds are its 0.65.1 defaults, written into
  `.swiftlint.yml`; SwiftLint's `line_length` is 120 while swift-format wraps at 100; both configs sit at the
  repo root and are always named; `swift-format-ignore` is never allowed; swift-format is built from source at
  its release commit (TD-156); `XCODE_VERSION=26.6` follows the `macos-26` arm64 runner image; a Mac with only
  the command-line tools skips `xcode-pinned` by name. The `ios-xcode-mismatch` fixture waits for 2c's macOS
  job (TD-155).
- 2026-11-01: 2b reviews (shell, evasion, senior architect) folded in, recorded in ADR 0043: SwiftLint runs
  through `scripts/harness/swiftlint.sh` (relative paths; exit 2 for a broken sensor); its style-kind and naming
  rules are off; the Swift file list is read NUL-separated and follows directory symlinks, and swift-format runs
  with `--follow-symlinks`; the hook blocks on a Mac when a Swift tool is missing; a non-breaking space no longer
  hides a `swiftlint:disable`; `NEEDS_SWIFT`/`NEEDS_XCODE` on a non-ios fixture is a bad `fixture.conf`;
  swift-format's resolved file moved to `scripts/swift-format/Package.resolved`, where Trivy scans it; no Swift
  step in the pre-commit hook. The 2c touchpoints gained the Xcode selection, the command-line-tools assertion
  and the cache key. Second review round: any non-letter between `swiftlint:` and `disable` counts;
  `swift-format.sh` turns swift-format's exit 1 for an unreadable config into 2; a linked directory's hidden
  `.build` is not linted; the tools stamp no longer needs the resolved file, so a product without ios may delete
  the three ios harness files, as README now says. Third round: a `.swift-format-ignore` file is protected config;
  SwiftLint runs with `--no-cache`; the integrity check matches config paths in any ASCII case, reports a config
  added, retargeted or deleted as a symlink and no longer reads a `#` line holding a carriage return, or ending in
  `\`, as a comment; `install-tools.sh` cleans up on every swift-format failure and names a missing resolved file.
- 2026-11-02: 2b merged (PR #15, 095e846). Built 2c on `feat/ios-ci`
  ([ADR 0044](../../adr/0044-ios-ci-on-macos.md)), with these rulings recorded there: the generated
  `.xcodeproj` is git-ignored and generated by `make setup` and `build-ios` (option (b)), not checked by
  `gen-check`; a push to main always runs the macOS job; `XCODE_VERSION` stays 26.6 from 2b. The macOS job
  `check-ios` (`ios.yml`, a required check) runs `make check-ios` and the selftest without the Docker fixtures;
  XcodeGen 2.46.0 is installed on demand; `project.yml` marks an iOS app for D4; the new checks
  `deps-swift-resolved` and the undeclared-test half of `ios-tests-pass` (TD-153's fix candidate) are `.bin/harness`
  subcommands in Go; the setup action caches swift-format apart (TD-156). Only the macOS job on the PR proves the
  Xcode parts (`build-ios`, `project.yml`, the `ios-app-build-broken` and `ios-xcode-mismatch` fixtures).
  After review, the gate of `ios.yml` no longer counts a change to `SPEC` (phase 3 adds the contract once Swift is
  generated from it), the swift-resolved check reads manifests in any case, refuses a linked manifest and an empty
  or intent-to-add lockfile, and counts a backticked `.package` or one with a comment before its parenthesis;
  the gate's blind spots, the build's fresh package resolution and the unread timings artifact are TD-164, TD-162
  and TD-163. The second review round added: `package(url:`, `id:` or `name:` without the dot (a bare `package(`
  matched the template's own comment "a Swift package (ADR 0041)"), a lockfile must pin a package, a link under
  `Tests/` is a finding, and Xcode project and `IOS_DIR` names match in any case.
- 2026-11-02: 2c merged (PR #17, 750c094); no hosted macOS runner picked up its `check-ios` job (cancelled after
  15 minutes, capacity), so no CI ran the Xcode parts. Run locally with Xcode 26.1 (`DEVELOPER_DIR`, not the
  pinned 26.6), `make build-ios` failed: through the `.bin/xcodegen` link XcodeGen found no setting presets, so
  the app had no `PRODUCT_NAME`. `.bin/xcodegen` is now a script that runs it by its real path.
