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

# fake <tool> <VAR>: logs its arguments to $BATS_TEST_TMPDIR/calls and exits with $FAKE_<VAR>_EXIT (default 0).
fake() {
  printf '#!/usr/bin/env bash\necho "%s $*" >>"%s"\nexit "${FAKE_%s_EXIT:-0}"\n' \
    "$1" "$BATS_TEST_TMPDIR/calls" "$2" >"$BATS_TEST_TMPDIR/bin/$1"
  chmod +x "$BATS_TEST_TMPDIR/bin/$1"
}

# Sandboxed: the hook puts the sandbox's (absent) .bin and node_modules/.bin first, so these fakes run.
web_sandbox() {
  sandbox .claude/hooks/fast-check.sh scripts/harness/run.sh scripts/harness/timing.sh scripts/tool-versions.env \
    docs/harness/rules.md
  mkdir -p "$SANDBOX/apps/web/src" "$BATS_TEST_TMPDIR/bin"
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
