#!/usr/bin/env bats
# .claude/hooks/guard-bash.sh: blocks force pushes, pushes to main, rm -r outside the repo and .env access.
# The cases themselves are Go table tests (apps/api/tools/harness/guardbash_test.go, ADR 0012); these check
# the wrapper end to end.
bats_require_minimum_version 1.5.0
load helpers

@test "blocks through the binary, with its reason" {
  run -2 hook guard-bash.sh 'git push --force origin feat'
  has 'guard-bash: blocked: force/destructive push (--force)'
  run -2 hook guard-bash.sh 'rm -rf /tmp/x'
  has 'guard-bash: blocked: rm -r outside the repository'
  run -2 hook guard-bash.sh 'cat .env.local'
  has '.env files hold'
}

@test "allows an ordinary command" {
  run -0 hook guard-bash.sh 'ls; git status && rm -rf dist'
  [ -z "$output" ]
}

@test "fails closed on unparseable input" {
  run -2 "$REPO/.claude/hooks/guard-bash.sh" <<<'not json'
  has 'cannot parse'
}
