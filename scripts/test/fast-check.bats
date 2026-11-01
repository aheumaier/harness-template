#!/usr/bin/env bats
# .claude/hooks/fast-check.sh: routes an edited file to its cheapest sensor (PostToolUse).
bats_require_minimum_version 1.5.0
load helpers

@test "rejects edits to generated code, given as argument or hook JSON" {
  f="$(git -C "$REPO" ls-files '*.gen.go' | head -1)"
  [ -n "$f" ]
  run -2 "$REPO/.claude/hooks/fast-check.sh" "$f"
  has "$f is generated"
  run -2 bash -c 'printf "{\"tool_input\":{\"file_path\":\"%s\"}}" "$1" | "$2"' _ "$REPO/$f" "$REPO/.claude/hooks/fast-check.sh"
  has 'make gen'
}

@test "ignores files outside the repo, deleted files and files without a sensor" {
  run -0 "$REPO/.claude/hooks/fast-check.sh" /etc/hosts
  run -0 "$REPO/.claude/hooks/fast-check.sh" apps/api/does-not-exist.go
  run -0 "$REPO/.claude/hooks/fast-check.sh" Makefile
  run -0 bash -c 'echo "{}" | "$1"' _ "$REPO/.claude/hooks/fast-check.sh"
}

@test "passes a clean shell script" {
  run -0 "$REPO/.claude/hooks/fast-check.sh" scripts/harness/squawk.sh
  [ -z "$output" ]
}

@test "skips fixture overlays (deliberate violations)" {
  run -0 "$REPO/.claude/hooks/fast-check.sh" scripts/harness/fixtures/sh-unquoted-variable/files/scripts/clean.sh
}

# fake <tool> <VAR>: logs its arguments to $BATS_TEST_TMPDIR/calls, prints $FAKE_<VAR>_OUT and exits with
# $FAKE_<VAR>_EXIT (default 0).
fake() {
  stub "$1" "echo \"$1 \$*\" >>'$BATS_TEST_TMPDIR/calls'; printf '%s' \"\${FAKE_$2_OUT:-}\"; exit \"\${FAKE_$2_EXIT:-0}\""
}

# Sandboxed: the hook puts the sandbox's (absent) .bin and node_modules/.bin first, so these fakes run.
web_sandbox() {
  sandbox .claude/hooks/fast-check.sh scripts/harness/run.sh scripts/harness/timing.sh scripts/tool-versions.env \
    docs/harness/rules.md
  mkdir -p "$SANDBOX/apps/web/src"
  echo '{}' >"$SANDBOX/apps/web/tsconfig.json"
  echo 'export const n: number = "x";' >"$SANDBOX/apps/web/src/a.ts"
  fake eslint ESLINT
  fake tsc TSC
  PATH="$BATS_TEST_TMPDIR/bin:$PATH"
}

@test "lints an edited web file on its own, without typechecking the package (make check does)" {
  web_sandbox
  FAKE_TSC_EXIT=1 run -0 "$SANDBOX/.claude/hooks/fast-check.sh" apps/web/src/a.ts
  run -0 cat "$BATS_TEST_TMPDIR/calls"
  [[ "$output" == 'eslint --max-warnings=0 apps/web/src/a.ts' ]]
}

@test "blocks a web file that fails eslint, with the remediation" {
  web_sandbox
  FAKE_ESLINT_EXIT=1 run -2 "$SANDBOX/.claude/hooks/fast-check.sh" apps/web/src/a.ts
  has 'remediation:'
}

@test "checks an edited Markdown file's links through .bin/harness, with the remediation" {
  sandbox .claude/hooks/fast-check.sh scripts/harness/run.sh scripts/harness/timing.sh scripts/tool-versions.env \
    docs/harness/rules.md .bin/harness
  echo docs/harness/rules.md >>"$SANDBOX/.git/info/exclude" # run.sh reads it; its links point outside the sandbox
  printf '# A\n\n[ok](#a)\n' >"$SANDBOX/docs/a.md"
  run -0 "$SANDBOX/.claude/hooks/fast-check.sh" docs/a.md
  printf '[gone](gone.md)\n' >"$SANDBOX/docs/a.md"
  run -2 "$SANDBOX/.claude/hooks/fast-check.sh" docs/a.md
  has 'docs/a.md: broken link (gone.md): docs/gone.md does not exist [docs-links-resolve]'
  has 'remediation:'
}

# Sandboxed Swift tools on a Mac: fakes in the sandbox's .bin, which the hook puts first on PATH and tests for.
swift_sandbox() {
  sandbox .claude/hooks/fast-check.sh scripts/harness/run.sh scripts/harness/timing.sh scripts/tool-versions.env \
    scripts/harness/file-sensors.sh scripts/harness/swiftlint.sh scripts/harness/swift-format.sh docs/harness/rules.md
  mkdir -p "$SANDBOX/apps/ios/Sources" "$SANDBOX/tools" "$SANDBOX/.bin"
  echo 'let a = 1' | tee "$SANDBOX/apps/ios/Sources/A.swift" >"$SANDBOX/tools/T.swift"
  fake swiftlint SWIFTLINT
  fake swift-format SWIFT_FORMAT
  mv "$BATS_TEST_TMPDIR/bin/"* "$SANDBOX/.bin/"
  host Darwin
}

@test "lints and format-checks an edited Swift file on its own, with the repo-root configs (ADR 0043)" {
  swift_sandbox
  run -0 "$SANDBOX/.claude/hooks/fast-check.sh" apps/ios/Sources/A.swift
  [ -z "$output" ]
  run -0 cat "$BATS_TEST_TMPDIR/calls"
  [[ "$output" == "swiftlint lint --strict --quiet --no-cache --reporter relative-path --config .swiftlint.yml apps/ios/Sources/A.swift
swift-format lint --strict --follow-symlinks --configuration .swift-format apps/ios/Sources/A.swift" ]]
  FAKE_SWIFT_FORMAT_EXIT=1 FAKE_SWIFT_FORMAT_OUT='apps/ios/Sources/A.swift:1:1: error: [Indentation] indent by 4 spaces' \
    run -2 "$SANDBOX/.claude/hooks/fast-check.sh" apps/ios/Sources/A.swift
  has 'rule:        ios-format'
  has 'remediation:'
  FAKE_SWIFT_FORMAT_EXIT=1 run -2 "$SANDBOX/.claude/hooks/fast-check.sh" apps/ios/Sources/A.swift
  has "sensor 'swift-format' failed (exit 2)"
  FAKE_SWIFTLINT_EXIT=134 run -2 "$SANDBOX/.claude/hooks/fast-check.sh" apps/ios/Sources/A.swift
  has "sensor 'swiftlint' failed (exit 2)"
}

@test "leaves Swift alone outside PRODUCT_GLOBS and WEB_GLOBS, without ios in STACK, or off macOS" {
  swift_sandbox
  FAKE_SWIFTLINT_EXIT=1 run -0 "$SANDBOX/.claude/hooks/fast-check.sh" tools/T.swift
  FAKE_SWIFTLINT_EXIT=1 HARNESS_STACK=go:db:web run -0 "$SANDBOX/.claude/hooks/fast-check.sh" apps/ios/Sources/A.swift
  host Linux
  FAKE_SWIFTLINT_EXIT=1 run -0 "$SANDBOX/.claude/hooks/fast-check.sh" apps/ios/Sources/A.swift
  [ ! -e "$BATS_TEST_TMPDIR/calls" ]
}

@test "blocks a Swift edit on a Mac when a Swift tool is missing, rather than pass it unchecked" {
  swift_sandbox
  rm "$SANDBOX/.bin/swift-format"
  run -2 "$SANDBOX/.claude/hooks/fast-check.sh" apps/ios/Sources/A.swift
  has 'fast-check: SwiftLint or swift-format is missing from .bin: run `make setup` (ADR 0043).'
  [ ! -e "$BATS_TEST_TMPDIR/calls" ]
}

@test "refuses an edit to a generated Swift file (layout.env's GENERATED_GLOBS), with or without ios in STACK" {
  swift_sandbox
  edit scripts/harness/layout.env 's|^GENERATED_GLOBS=|GENERATED_GLOBS=apps/ios/Generated/*:|'
  mkdir -p "$SANDBOX/apps/ios/Generated" && echo 'let g = 1' >"$SANDBOX/apps/ios/Generated/Client.swift"
  run -2 "$SANDBOX/.claude/hooks/fast-check.sh" apps/ios/Generated/Client.swift
  has 'fast-check: apps/ios/Generated/Client.swift is generated'
  HARNESS_STACK=go:db:web run -2 "$SANDBOX/.claude/hooks/fast-check.sh" apps/ios/Generated/Client.swift
  [ ! -e "$BATS_TEST_TMPDIR/calls" ]
}
