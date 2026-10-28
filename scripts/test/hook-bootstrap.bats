#!/usr/bin/env bats
# scripts/harness/hook-bootstrap.sh: without .bin/harness every hook fails closed (exit 2, "run make setup"),
# except for the commands that build the binary, so the repo cannot lock itself (ADR 0012).
bats_require_minimum_version 1.5.0
load helpers

setup() {
  unset HARNESS_EDIT
  sandbox scripts/harness/hook-bootstrap.sh scripts/harness/protected-paths.txt scripts/tool-versions.env \
    .claude/hooks/protect-harness.sh .claude/hooks/guard-bash.sh .claude/hooks/only-make-check.sh \
    .claude/hooks/fast-check.sh
  [ ! -e "$SANDBOX/.bin/harness" ]
}

# bash_payload <command>: a PreToolUse Bash payload, written by hand: node is not needed for these hooks.
bash_payload() { printf '{"cwd":"%s","tool_name":"Bash","tool_input":{"command":"%s","description":"x"}}' "$SANDBOX" "$1"; }

@test "the commands that build the binary pass the PreToolUse hooks" {
  for c in 'make setup' 'make .bin/harness'; do
    for h in guard-bash protect-harness only-make-check; do
      run -0 "$SANDBOX/.claude/hooks/$h.sh" < <(bash_payload "$c")
    done
  done
  run -0 "$SANDBOX/.claude/hooks/only-make-check.sh" < <(bash_payload 'make check')
}

@test "anything else is blocked with run make setup" {
  for c in 'ls' 'make setup; rm -rf /x' 'make setup && ls' 'make setup\" ; rm -rf /x' 'make setupx' 'make check'; do
    for h in guard-bash protect-harness; do
      run -2 "$SANDBOX/.claude/hooks/$h.sh" < <(bash_payload "$c")
      has "$h: blocked: .bin/harness is missing; run \`make setup\`"
    done
  done
  run -2 "$SANDBOX/.claude/hooks/protect-harness.sh" <<<'{"tool_name":"Edit","tool_input":{"file_path":"x"}}'
  has 'run `make setup`'
  run -2 "$SANDBOX/.claude/hooks/only-make-check.sh" < <(bash_payload 'make check; ls')
  run -2 "$SANDBOX/.claude/hooks/fast-check.sh" <<<'{"tool_input":{"file_path":"x"}}'
  has 'fast-check: blocked: .bin/harness is missing'
}

@test "a file argument to fast-check needs no binary" {
  run -0 "$SANDBOX/.claude/hooks/fast-check.sh" scripts/tool-versions.env
}

@test "HARNESS_EDIT=1 still short-circuits protect-harness" {
  HARNESS_EDIT=1 run -0 "$SANDBOX/.claude/hooks/protect-harness.sh" <<<'{"tool_name":"Edit","tool_input":{"file_path":"x"}}'
}
