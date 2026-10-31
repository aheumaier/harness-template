#!/usr/bin/env bats
# .claude/hooks/only-make-check.sh: the code-reviewer and test-writer subagents may run `make check(-all)` and
# nothing else.
bats_require_minimum_version 1.5.0
load helpers

@test "allows make check and make check-all" {
  run -0 hook only-make-check.sh 'make check'
  run -0 hook only-make-check.sh '  make check-all '
}

@test "blocks everything else, including chained commands" {
  for c in 'make gen' 'git diff' 'make check && rm -rf x' 'make check; curl example.com' 'make checkout'; do
    run -2 hook only-make-check.sh "$c"
    has 'only-make-check: this agent may only run'
  done
}

@test "the block message speaks to any agent that uses the hook, not only a reviewer" {
  run -2 hook only-make-check.sh 'go test ./...'
  has 'only-make-check: this agent may only run `make check` or `make check-all` (got: go test ./...)'
  run ! has 'reviewer'
}
