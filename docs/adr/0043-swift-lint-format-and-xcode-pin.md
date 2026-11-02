# ADR 0043: SwiftLint, swift-format and the Xcode pin (amends 0016, 0024, 0042)

Status: proposed · 2026-11-01 · amends [ADR 0016](0016-tool-supply-chain.md),
[ADR 0024](0024-size-and-complexity-limits-are-maintainability-sensors.md), [ADR 0042](0042-swift-unit-tests-in-make-check.md)

## Context
[ADR 0042](0042-swift-unit-tests-in-make-check.md) (the first PR of phase 2 of
[exec plan 0004](../exec-plans/active/0004-ios-stack-group.md)) runs Swift unit tests in `make check` on macOS.
Nothing lints or formats Swift yet, the size rules do not reach it, and nothing says which Xcode builds it. The
owner decided on 2026-10-31 that SwiftLint and swift-format are pinned `.bin` tools, macOS only, installed only
with `ios` in `STACK`, and that their suppressions need `reason: … (rule: <id>)` (plan question Q4); that
`XCODE_VERSION` lives in `scripts/tool-versions.env`, the macOS CI job selects it and a sensor fails on a mismatch
(Q3); and that any Swift under a `PRODUCT_GLOBS` or `WEB_GLOBS` entry is iOS code (Q8). This ADR records the
second phase 2 PR. The macOS CI job comes with the third.

## Decision
- **Tools.** SwiftLint 0.65.1 (released 2026-08-21) from its portable zip, one universal binary, checked against
  a sha256 pin that matches GitHub's asset digest. swift-format publishes no binaries, so `install-tools.sh`
  clones 604.0.0 (released 2026-09-16), checks out the tag's commit (`SWIFT_FORMAT_COMMIT`, which git verifies)
  and builds it with `--only-use-versions-from-resolved-file` against `scripts/swift-format/Package.resolved`, the
  dependency revisions of each dependency's release tag (swift-argument-parser 1.8.2, swift-cmark 0.9.0,
  swift-markdown 0.9.0, swift-syntax 604.0.0), all released before 2026-10-18, so every pin passes the 14-day
  age rule of [ADR 0016](0016-tool-supply-chain.md). The build takes about 170 s once per worktree
  ([TD-156](../exec-plans/tech-debt-tracker.md)). Both are installed only on macOS with `ios` in `STACK`. The
  Makefile's `.bin/.tools-stamp` depends on `scripts/swift-format/Package.resolved` too, so a bump installs the
  tools before `lint-ios` or `fmt-check-ios` runs them; swift-format's own stamp names its version, its commit and
  the resolved file's sha256, so a change to any of them rebuilds it.
- **A tool built from source is pinned by commit (amends ADR 0016).** ADR 0016 pins a downloaded archive by its
  sha256. A tool with no release binary is pinned by `<TOOL>_COMMIT` (the release tag's commit, which git checks)
  and, for a SwiftPM tool, by a `Package.resolved` the build is held to; a bump updates the version, the commit and
  the resolved file in one harness PR, and checks every revision in it against the 14-day age rule. The resolved
  file sits in `scripts/swift-format/`, where `deps-no-known-vulns` (Trivy) scans it, so swift-format's
  dependencies are scanned like the product's. `SWIFTLINT_SHA256_DARWIN` carries no arch: SwiftLint's portable zip
  is one universal binary.
- **Configs at the repo root, always named.** `.swiftlint.yml` and `.swift-format` sit at the root and every
  caller passes them explicitly (`--config`, `--configuration`), so a nested config never applies. Both are
  protected `config` files, and so is `scripts/swift-format/Package.resolved`.
- **SwiftLint (rule `ios-lint`)** runs its default rules with `--strict`, so a warning fails. `trailing_comma` is
  off because swift-format's `multiElementCollectionTrailingCommas` adds those commas; the formatter wins.
  `scripts/harness/swiftlint.sh` calls it for `make lint-ios` and the hook: it keeps the reported paths relative
  (a rule id in the checkout's own path would otherwise match every finding line in `run.sh`), exits 1 only when
  SwiftLint printed a finding and 2 for any other failure (a bad config, SourceKit not loading), so a broken sensor
  never reads as a finding, and sets `TOOLCHAIN_DIR` while only the command-line tools are selected, the one way
  SwiftLint finds SourceKit there.
- **Ruling: no style rules from SwiftLint.** The default rules of SwiftLint's `style` kind and the naming rules
  (`generic_type_name`, `type_name`) are off in `.swiftlint.yml` (core belief 3, ADR 0024): layout is
  swift-format's (`ios-format`), and two tools that judge the same line disagree. A SwiftLint bump turns off each
  new default rule of kind `style` in the same PR.
- **Size rules reach Swift (amends ADR 0024).** `function_body_length` 50, `cyclomatic_complexity` 10,
  `function_parameter_count` 5 and `file_length` 400 are SwiftLint 0.65.1's documented warning defaults, written
  into `.swiftlint.yml` so a SwiftLint bump cannot move them. `run.sh` maps each of these findings to its
  `code-*` rule; every other SwiftLint finding goes to `ios-lint`.
- **Ruling: Swift tests are not exempt from the size rules.** SwiftLint applies a config's `excluded:` paths to
  every rule at once, with no per-rule exclusion, and a second config for `Tests/` would be one more protected
  file whose loosening review has to catch. The skeleton's tests pass the limits
  ([TD-154](../exec-plans/tech-debt-tracker.md) revisits this when a real test suite hits them).
- **Ruling: `line_length` 120, swift-format's `lineLength` 100.** swift-format wraps code at its default of 100;
  SwiftLint's `line_length` catches what the formatter cannot wrap (comments, long literals) at its default of
  120, the width the repo's comments use. A limit of 100 failed the skeleton's own comments.
- **swift-format (rule `ios-format`)** is swift-format 604.0.0's default configuration with 4-space indentation,
  checked by `swift-format lint --strict`; `make fmt` rewrites the files. swift-format exits 1 for a finding and for
  a config it cannot read alike, so `scripts/harness/swift-format.sh` calls it for `make fmt-check-ios` and the hook
  and, as `swiftlint.sh` does, exits 1 only when it printed a finding and 2 for any other failure.
- **Suppressions.** `swiftlint:disable` (all its forms, with anything but letters and digits between
  `swiftlint:` and `disable`, so no invisible character SwiftLint skips can hide one) needs
  `reason: … (rule: <id>)` on the same line, as every other suppression does (`harness-suppression-reason`); a
  Swift file is now a suppression source.
  **Ruling: `swift-format-ignore` is never allowed**: swift-format reads it only as a comment on its own, so a
  trailing reason turns it off, and the formatter can always be satisfied. A `.swift-format-ignore` file (paths
  swift-format skips) is a protected `config` file wherever it sits, so adding one needs the harness-change label;
  for it, the integrity check now matches config paths in any ASCII case
  ([TD-161](../exec-plans/tech-debt-tracker.md)), reports a config added, retargeted or deleted as a symlink, and no longer reads a `#`
  line holding a carriage return as a comment (swift-format and SwiftLint's YAML start a line there), nor one
  ending in `\` (make continues it onto the next line).
  SwiftLint runs with `--no-cache`: its cache keys on a linked file's own modification time, so an edit behind the
  link would pass on the machine that linted it before.
- **Which files (Q8).** `lint-ios` and `fmt-check-ios` read the tracked and untracked (not ignored) `*.swift`
  under a `PRODUCT_GLOBS` or `WEB_GLOBS` entry, without `GENERATED_GLOBS`, and are skipped by name while none
  exists or none changed against the base (`make check-all` reads them all). The list is read NUL-separated, so a
  name git would quote (a non-ASCII byte, `"`, `\`) stays in; a symlink to a directory, which git lists as one
  file and SwiftPM compiles through, adds the Swift files behind it (none under a hidden directory such as
  `.build`, where a built package keeps its dependencies' sources), and swift-format runs with
  `--follow-symlinks`, without which it skips a linked file and passes. `file-sensors.sh` gets a `swift` kind with
  the same globs, and the fast-check hook lints and format-checks an edited Swift file with it; on a Mac with `ios`
  in `STACK` and a tool missing from `.bin`, the hook blocks with "run `make setup`" rather than pass the edit
  unchecked.
- **Ruling: no Swift step in the pre-commit hook.** The tools are macOS only, so the step would check a commit on
  a Mac and wave the same commit through elsewhere, and SwiftLint's start-up eats much of the hook's 2 s budget
  (NFR-H8). `make check` and the per-edit hook already run both tools on every Mac, and the macOS CI job (phase
  2c) runs them on every PR.
- **Budgets.** `swiftlint` and `swift-format` get a 2000 ms row in `scripts/harness/budgets.txt` (NFR-H5): they
  took 0.33 s and 0.15 s on the skeleton's 4 files, both grow with the app, and the rows warn well before NFR-H5's
  5 s default.
- **The Xcode pin (rule `ios-xcode-pinned`).** `XCODE_VERSION=26.6` is the default Xcode of GitHub's `macos-26`
  arm64 runner image: [macos-26-arm64-Readme.md](https://github.com/actions/runner-images/blob/6d2bc0ac6b93f732b1a5baebf587d812853892c4/images/macos/macos-26-arm64-Readme.md)
  at commit `6d2bc0a` (2026-10-30, image `20261026.0455.2`, Xcode 26.6 build 17F113). `make xcode-pinned` fails
  when `xcodebuild -version` names another version, exits 2 when no developer directory is selected or
  `xcodebuild` prints no version, and is skipped by name only while the command-line tools are selected and
  `xcodebuild` fails there, so a developer directory named like the command-line tools that holds a working
  `xcodebuild` is still compared.
- **Two lists of ios targets (amends ADR 0042).** `IOS_SWIFT_TARGETS` (`test-ios`, `lint-ios`,
  `fmt-check-ios`) need only Swift and run in `make check`; `IOS_XCODE_TARGETS` (`xcode-pinned`) need Xcode and
  run in `make check-ios`, which runs both lists and is the macOS CI job's target. `IOS_TARGETS` is both, for
  `STACK_OFF` and `HOST_OFF`.
- **Ruling: a third host class.** A Mac with only the command-line tools runs the Swift sensors and skips the
  Xcode ones. The selftest's `NEEDS_XCODE=1` marks a fixture whose `TARGET` is in `IOS_XCODE_TARGETS` (a bad
  `fixture.conf` otherwise, and any other ios target still needs `NEEDS_SWIFT=1`); on such a Mac it fails unless
  `SELFTEST_SKIP_XCODE=1` skips it, as `SELFTEST_SKIP_SWIFT` does for Swift.

## Consequences
- A product with `ios` in `STACK` gets its Swift linted, formatted and held to the size rules by `make check` and
  the per-edit hook on any Mac, and `make fmt` formats it.
- `make setup` in a fresh worktree on macOS with `ios` builds swift-format, about 3 minutes once.
- A Mac with only the command-line tools runs `make harness-selftest` with `SELFTEST_SKIP_XCODE=1`. Until the
  macOS CI job of the third phase 2 PR, nothing proves that the `ios-xcode-mismatch` fixture is rejected
  ([TD-155](../exec-plans/tech-debt-tracker.md)); `make-stack.bats` tests `xcode-pinned` with a stub instead.
- The macOS CI job (phase 2c) selects `/Applications/Xcode_<XCODE_VERSION>.app` by path, so a runner image
  whose default Xcode moves on does not turn it red; it bites only when the image drops that Xcode, and then the
  job fails on the missing path and the fix is a harness PR that bumps `XCODE_VERSION`. A developer with another
  Xcode sees `xcode-pinned` red from `make check-ios`, not from `make check`.
- A swift-format bump changes three things that must agree (version, commit, resolved file); install-tools.sh
  rebuilds on any of them, and Trivy scans the new resolved file.
- A long Swift test fails `code-function-length`; the fix is a helper or a suppression with a reason.
