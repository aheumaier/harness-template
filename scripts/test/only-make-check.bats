#!/usr/bin/env bats
# .claude/hooks/only-make-check.sh: the code-reviewer subagent may run `make check(-all)` and nothing else.
bats_require_minimum_version 1.5.0
load helpers

@test "allows make check and make check-all" {
  run -0 hook only-make-check.sh 'make check'
  run -0 hook only-make-check.sh '  make check-all '
}

@test "blocks everything else, including chained commands" {
  for c in 'make gen' 'git diff' 'make check && rm -rf x' 'make check; curl example.com' 'make checkout'; do
    run -2 hook only-make-check.sh "$c"
    has 'only-make-check: this reviewer may only run'
  done
}
