#!/usr/bin/env bats
# .claude/hooks/only-test-files.sh: the test-writer subagent may write test files inside the repo and nothing
# else (ADR 0039). The predicate itself is table-tested in apps/api/tools/harness/testfile_test.go.
bats_require_minimum_version 1.5.0
load helpers

# write <file_path> [tool]: feeds a PreToolUse file-tool payload, cwd = the repo, to the hook.
write() {
  node -e 'process.stdout.write(JSON.stringify({ cwd: process.argv[3], tool_name: process.argv[2], tool_input: { file_path: process.argv[1] } }))' \
    "$1" "${2:-Write}" "$REPO" | "$REPO/.claude/hooks/only-test-files.sh"
}

@test "allows each test-file suffix, absolute or relative to cwd" {
  for f in apps/api/internal/x/model/x_test.go packages/domain/src/a.test.ts apps/web/src/a.test.tsx \
    apps/web/src/a.test.mjs apps/web/e2e/a.e2e.ts "$REPO/apps/api/internal/y/service/new_test.go"; do
    run -0 write "$f"
  done
  run -0 write apps/api/internal/x/model/x_test.go MultiEdit
}

@test "blocks a non-test file and names it" {
  for f in apps/api/internal/x/model/x.go packages/domain/src/a.ts apps/api/internal/x/model/x_test.go.orig; do
    run -2 write "$f"
    has "only-test-files: blocked: $f is not a test file"
  done
}

@test "blocks a path outside the repo" {
  for f in ../x_test.go "$BATS_TEST_TMPDIR/x_test.go" apps/../../x_test.go; do
    run -2 write "$f"
    has 'is outside the repository'
  done
}

@test "blocks malformed input and a payload without a path" {
  run -2 "$REPO/.claude/hooks/only-test-files.sh" <<<'{bad json'
  has 'only-test-files: blocked: cannot parse the hook input'
  run -2 "$REPO/.claude/hooks/only-test-files.sh" <<<'{"tool_name":"Write","tool_input":{}}'
  has 'no file path in the hook input'
}

@test "blocks with run make setup when .bin/harness is missing" {
  sandbox scripts/harness/hook-bootstrap.sh .claude/hooks/only-test-files.sh
  run -2 "$SANDBOX/.claude/hooks/only-test-files.sh" <<<'{"tool_name":"Write","tool_input":{"file_path":"a_test.go"}}'
  has 'only-test-files: blocked: .bin/harness is missing; run `make setup`'
}
