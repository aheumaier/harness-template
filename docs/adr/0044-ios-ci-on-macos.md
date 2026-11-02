# ADR 0044: iOS CI on macOS, the app target and Swift lockfiles (amends 0042, 0043)

Status: proposed · 2026-11-02 · amends [ADR 0042](0042-swift-unit-tests-in-make-check.md),
[ADR 0043](0043-swift-lint-format-and-xcode-pin.md)

## Context
[ADR 0042](0042-swift-unit-tests-in-make-check.md) and [ADR 0043](0043-swift-lint-format-and-xcode-pin.md) (the
first two PRs of phase 2 of [exec plan 0004](../exec-plans/active/0004-ios-stack-group.md)) run `swift test`,
SwiftLint and swift-format in `make check` on a Mac and pin Xcode. No CI job runs any of them: every job is on
Ubuntu, which skips the ios sensors by name ([TD-151](../exec-plans/tech-debt-tracker.md)). The template's
`apps/ios` is a Swift package with no app target, so no iOS build exists to check, and nothing pins the
dependencies a Swift package pulls from the network. The owner decided on 2026-10-31 that the macOS job always
starts and passes with a notice when nothing iOS changed, so it can be a required check (plan question Q7), and
that the app is described by an XcodeGen spec (Q2). This ADR records the third phase 2 PR.

## Decision
- **The macOS job.** `.github/workflows/ios.yml`, job `check-ios` on `macos-26`, a required status check and a
  protected harness file. It selects `/Applications/Xcode_<XCODE_VERSION>.app` by path (red when the image lacks
  it), asserts that `xcode-select -p` is not the command-line tools (otherwise `xcode-pinned` and `build-ios` skip
  by name and the job proves nothing), runs `make setup` with `SKIP_DB=1`, then `make check-ios` and
  `make harness-selftest` with `SELFTEST_SKIP_DOCKER=1`, and uploads its timings as `harness-timings-check-ios`.
  Hosted macOS runners have no Docker, so no Postgres: the job never runs `check-all`, and the selftest skips by
  name only the fixtures that need Docker, which the Ubuntu `harness-selftest` job runs.
- **When it runs (Q7).** It always starts. It passes with a notice when `STACK` has no `ios`, or on a PR that
  changes no `*.swift`, `Package.swift`, `Package.resolved` or `project.yml` anywhere (Q8), nothing in `IOS_DIR`
  and no harness file (`scripts/harness/unchanged.sh`, which adds every protected path); phase 3 adds the
  contract once Swift is generated from it. **Ruling: a push to main always runs it**, so the newest main is proven
  whatever the PR's gate said (the concurrency group drops a pending run that a newer push overtakes).
- **The Ubuntu jobs** keep `SELFTEST_SKIP_SWIFT=1` and add `SELFTEST_SKIP_XCODE=1`. It is redundant (off macOS a
  `NEEDS_XCODE` fixture is skipped by its Swift check first) and set so the skip does not depend on that order.
- **The app target (Q2).** `apps/ios/project.yml` describes an iOS application `App` (sources in `App/`, iOS 17)
  that depends on the package's `AppCore` library as a local package at `.`, with a scheme `App`. **Ruling (b):
  the generated `.xcodeproj` is git-ignored** (`apps/ios/.gitignore`), not committed and checked by `gen-check`
  (option (a)). This departs from core belief 4 (generated code is committed and checked by `gen-check`): a
  generated `project.pbxproj` is a large file that conflicts on every parallel change, and a `gen-check` for it
  would need XcodeGen and so run on macOS only. Xcode opens the project after `make setup`,
  which generates it where Xcode is selected (`make ios-project`), and `build-ios` generates it on every run.
  `project.yml` also marks an iOS app for [harness-layout-valid](../harness/rules.md#harness-layout-valid), so a
  `STACK` without `ios` fails while one is left (plan D4, fixture `harness-stack-hides-ios`).
- **XcodeGen 2.46.0** (released 2026-07-16, past the 14-day rule of
  [ADR 0016](0016-tool-supply-chain.md)) from its release zip, one universal binary, checked against a sha256 pin
  (`XCODEGEN_SHA256_DARWIN`) that matches GitHub's asset digest. `scripts/install-tools.sh xcodegen` installs it on
  demand, macOS only, into `.bin/.xcodegen/` with `.bin/xcodegen` linked to it (XcodeGen finds its setting presets
  beside the binary through that link); `make tools` does not, since only `ios-project` and `build-ios` use it.
- **The app build (rule `ios-app-builds`).** `make build-ios`, in `IOS_XCODE_TARGETS` beside `xcode-pinned`
  (amends ADR 0043), generates the project and runs `xcodebuild` for `generic/platform=iOS Simulator` with
  `CODE_SIGNING_ALLOWED=NO` and derived data in `.cache/xcode`. **Ruling: the scheme `App` and the sources in
  `App/` are a convention, not a `layout.env` key** (an exception to ADR 0030, listed in README step 5), until a
  product needs another shape. It is skipped by name without a `project.yml` and
  while only the command-line tools are selected. Fixture `ios-app-build-broken` (`NEEDS_XCODE=1`).
- **Every Swift test file is compiled (rule `ios-tests-pass`, amends ADR 0042).** `swift test` compiles only the
  test targets the manifest declares, so a test in an undeclared folder, an `exclude:`d one or one only a
  version-specific manifest declares passed unrun ([TD-153](../exec-plans/tech-debt-tracker.md)). `make test-ios`
  now pipes `swift package describe --type json` into `.bin/harness swift-tests`, which fails on a git-known Swift
  file under `IOS_DIR/Tests/` that no test target compiles (or on a link there), and still runs `swift test`, so
  both kinds of finding show at once. Fixture `ios-test-undeclared`.
- **Swift lockfiles (rule `deps-swift-resolved`).** `.bin/harness swift-resolved`, in `make check` in every stack,
  fails on a git-known manifest that declares a remote package without a `Package.resolved` beside it in git's
  index that pins at least one package (a symlink does not count). Without one, SwiftPM resolves the newest
  allowed versions on each fresh checkout, and Trivy (`deps-no-known-vulns`) has nothing to scan. The manifest is
  read as text, never run: with white space and backticks removed, a remote package is any `.package` (or
  `package(url:` without the dot) that is not `.package(path:`, commented out or not (failing closed). Fixture
  `deps-swift-unresolved`. In scope `full` (CI, `check-all`) `make test-ios` passes
  `--only-use-versions-from-resolved-file`, so a lockfile a manifest change left stale fails instead of resolving
  fresh.
- **The two new checks are Go** (`apps/api/tools/harness`, unit-tested with mutations each caught), as every new
  root sensor is; they call no shell script.
- **swift-format is cached apart (TD-156, CI half).** `.github/actions/setup` caches `.bin/swift-format` and its
  stamp on a macOS runner with `ios` in `STACK`, keyed on `SWIFT_FORMAT_COMMIT` and the sha256 of
  `scripts/swift-format/Package.resolved`, and leaves both out of the `.bin` cache, so a change to
  `tool-versions.env` or `install-tools.sh` does not rebuild it (about 3 minutes). The stamp still makes
  `make tools` rebuild a restored binary whose pins differ.

## Consequences
- Every PR that touches Swift, `IOS_DIR` or the harness has its Swift built, tested, linted and
  formatted, the app built for the simulator, and every ios fixture (`ios-xcode-mismatch` included) proven on a Mac
  with Xcode. NFR-R5 ("selftest on macOS") is met per PR rather than weekly. TD-151 and TD-155 close once the
  first run on the PR is green.
- **Cost.** The repo is private, so GitHub bills macOS minutes at ten times Linux minutes; in the template almost
  every PR touches the harness, so the job does real work on most PRs and on every push to main. A public repo pays
  nothing.
- **Adopters.** `check-ios` is a new required check: a repo admin re-runs `scripts/github-branch-protection.sh`.
  A product without `ios` gets a job that passes with a notice. A product with `ios` keeps `project.yml` with the
  app's sources in `App/` and its scheme named `App`: `build-ios` skips without a `project.yml`, but the
  `ios-app-build-broken` fixture writes into `App/` and expects the build to fail, so the selftest fails until the
  app follows that shape. It commits `Package.resolved` once a remote package appears.
- A required check needs a macOS runner even to print its notice: when the repo's minutes run out under a $0
  spending limit, an org policy blocks macOS runners or hosted macOS is down, every PR is blocked, also in a
  product without `ios`. Remedies: raise the limit, merge as an admin, or drop `check-ios` from `REQUIRED_CHECKS`
  until it is back ([ci.md](../harness/ci.md)).
- When the `macos-26` image drops Xcode `XCODE_VERSION`, the job fails on the missing path; the fix is a harness
  PR that bumps it (ADR 0043).
- **Blind spots.** The build is a simulator build only: no device build, signing, archive, app tests or UI tests
  (plan phases 3 and 4). Remote packages listed in `project.yml` resolve into the git-ignored project, so
  `deps-swift-resolved` does not see them, and their lockfile is not in git; for the same reason `build-ios`
  resolves the app's packages afresh, the package's `Package.resolved` notwithstanding (TD-162). The gate's inputs
  are paths, not the build graph: a `.package(path:)` or a symlink target outside `IOS_DIR` is no input, so a PR
  that changes only it skips the job until the push to main, and a guide-only harness change runs it (TD-164).
  Nothing reads the `harness-timings-check-ios` artifact yet, so the ios rules' timings and fires stay out of the
  weekly reports (TD-163). The Postgres fixtures never run on macOS. A Mac with only the command-line tools still
  cannot run `build-ios` or the `NEEDS_XCODE` fixtures.
