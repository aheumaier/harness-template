#!/usr/bin/env bats
# .claude/hooks/only-test-files.sh: the test-writer subagent may write the product test files specs-tested reads
# and nothing else (ADR 0039). The predicate itself is table-tested in apps/api/tools/harness/testfile_test.go.
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

@test "allows exactly the product test files of testdata/test-files.txt, as specs-tested counts them" {
  local p want n=0
  while read -r p want; do
    [[ -z "$p" || "$p" == \#* ]] && continue
    n=$((n + 1))
    if [[ "$want" == allow ]]; then
      run -0 write "$p"
    else
      run -2 write "$p"
      has "only-test-files: blocked: $p is "
    fi
  done <"$REPO/apps/api/tools/harness/testdata/test-files.txt"
  ((n > 0))
}

@test "names the test file forms and why a test file is refused" {
  run -2 write packages/domain/src/a.ts
  has 'is not a test file (*_test.go, or *.test.*, *.spec.* or *.e2e.* ending in .js, .ts, .jsx or .tsx'
  run -2 write apps/api/internal/x/store/q_test.go
  has 'is generated code (GENERATED_GLOBS:'
  run -2 write apps/api/tools/harness/a_test.go
  has 'is a harness file (apps/api/tools/harness/* in scripts/harness/protected-paths.txt)'
}

@test "blocks a non-test file and names it" {
  for f in apps/api/internal/x/model/x.go packages/domain/src/a.ts apps/api/internal/x/model/x_test.go.orig; do
    run -2 write "$f"
    has "only-test-files: blocked: $f is not a test file"
  done
}

@test "blocks a path outside the repo, and any path with a .. part" {
  run -2 write "$BATS_TEST_TMPDIR/x_test.go"
  has 'is outside the repository'
  for f in ../x_test.go apps/../../x_test.go apps/api/internal/x/model/../model/x_test.go; do
    run -2 write "$f"
    has "$f has a \`..\` part, which can lead outside the repository"
  done
}

@test "blocks a case variant of an excluded path: on a case-insensitive disk it is that path" {
  for f in apps/api/tools/Harness/zz_test.go apps/api/internal/x/Store/q_test.go packages/API-client/src/a.test.ts; do
    run -2 write "$f"
    has "only-test-files: blocked: $f is "
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
