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
