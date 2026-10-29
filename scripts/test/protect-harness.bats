#!/usr/bin/env bats
# .claude/hooks/protect-harness.sh: blocks edits to harness files unless HARNESS_EDIT=1.
bats_require_minimum_version 1.5.0
load helpers

# edit <tool> <path>: feeds a PreToolUse file-tool payload to the hook.
edit() {
  node -e 'process.stdout.write(JSON.stringify({ cwd: process.argv[3], tool_name: process.argv[1],
    tool_input: { file_path: process.argv[2] } }))' "$1" "$2" "$REPO" | "$REPO/.claude/hooks/protect-harness.sh"
}

# bash_cmd <command>: feeds a PreToolUse Bash payload (tool_name set) to the hook.
bash_cmd() {
  node -e 'process.stdout.write(JSON.stringify({ cwd: process.argv[2], tool_name: "Bash",
    tool_input: { command: process.argv[1] } }))' "$1" "$REPO" | "$REPO/.claude/hooks/protect-harness.sh"
}

setup() { unset HARNESS_EDIT; }

@test "blocks file tools on harness files, with the reason and what to do instead" {
  for p in .golangci.yml apps/api/.go-arch-lint.yml apps/web/eslint.config.js .claude/settings.json \
    .claude/hooks/guard-bash.sh .github/workflows/ci.yml scripts/harness/run.sh AGENTS.md "$REPO/Makefile"; do
    run -2 edit Edit "$p"
    has 'is a harness file'
  done
  has 'propose the change in the PR description'
  run -2 edit Write atlas.hcl
  run -2 edit MultiEdit .spectral.yaml
  run -2 edit Edit scripts/test/protect-harness.bats
  run -2 edit Edit scripts/tool-versions.env
  run -2 edit Edit apps/web/tsconfig.json
}

@test "allows file tools on product code and docs outside the harness" {
  for p in apps/api/internal/platform/x.go apps/web/src/App.tsx api/openapi.yaml docs/QUALITY.md \
    ARCHITECTURE.md apps/api/AGENTS.md /tmp/elsewhere/AGENTS.md; do
    run -0 edit Edit "$p"
  done
}

@test "blocks shell writes to harness files" {
  for c in 'echo x >> .golangci.yml' 'echo x >AGENTS.md' 'echo x > .squawk.toml' 'sed -i "" s/a/b/ Makefile' \
    'cp /tmp/x .claude/settings.json' 'rm scripts/harness/run.sh' 'make check && tee -a knip.json <x' \
    'git checkout -- .golangci.yml' 'mv a "AGENTS.md"'; do
    run -2 bash_cmd "$c"
    has 'is a harness file'
  done
}

@test "allows shell reads of harness files and writes elsewhere" {
  for c in 'cat AGENTS.md' 'grep -n x .golangci.yml' 'sed -n 1,5p Makefile' 'echo x > apps/api/x.go' \
    'make check' 'git diff -- AGENTS.md' 'cp AGENTS.md /tmp/x'; do
    run -0 bash_cmd "$c"
  done
}

@test "HARNESS_EDIT=1 (set by a human) allows the edit" {
  HARNESS_EDIT=1 run -0 edit Edit AGENTS.md
  HARNESS_EDIT=1 run -0 bash_cmd 'echo x >> Makefile'
}

@test "guards every worktree of the repo with this checkout's list: a sibling lane, a nested agent worktree" {
  sandbox .claude/hooks/protect-harness.sh .bin/harness scripts/harness/protected-paths.txt scripts/harness/timing.sh
  local lane="$BATS_TEST_TMPDIR/lane" other="$BATS_TEST_TMPDIR/other"
  git -C "$SANDBOX" add -A && git -C "$SANDBOX" -c user.name=t -c user.email=t@localhost commit -qm x
  git -C "$SANDBOX" worktree add -q -b lane "$lane"
  git -C "$SANDBOX" worktree add -q -b agent "$SANDBOX/.claude/worktrees/agent"
  git init -q "$other"
  : >"$lane/scripts/harness/protected-paths.txt" # a worktree cannot loosen its own copy
  for p in "$lane/Makefile" "$lane/.claude/settings.json" "$SANDBOX/.claude/worktrees/agent/AGENTS.md"; do
    REPO="$SANDBOX" run -2 edit Edit "$p"
    has 'is a harness file'
  done
  for p in "$lane/apps/api/x.go" "$SANDBOX/.claude/worktrees/agent/apps/api/x.go" "$other/Makefile"; do
    REPO="$SANDBOX" run -0 edit Edit "$p"
  done
  REPO="$SANDBOX" run -2 bash_cmd "echo x >> $lane/AGENTS.md"
  git init -q "$BATS_TEST_TMPDIR/lane2" # shares the lane's prefix, is no worktree
  REPO="$SANDBOX" run -0 edit Edit "$BATS_TEST_TMPDIR/lane2/Makefile"
  # A session rooted in the nested agent worktree guards the main checkout too.
  REPO="$SANDBOX/.claude/worktrees/agent" run -2 edit Edit "$SANDBOX/Makefile"
  has 'is a harness file'
}

@test "a worktree registered inside this checkout does not hide its harness files" {
  sandbox .claude/hooks/protect-harness.sh .bin/harness scripts/harness/protected-paths.txt scripts/harness/timing.sh
  git -C "$SANDBOX" add -A && git -C "$SANDBOX" -c user.name=t -c user.email=t@localhost commit -qm x
  git -C "$SANDBOX" worktree add -q -b fake "$BATS_TEST_TMPDIR/fake"
  # Repoint the fake worktree's admin entry at scripts/, so git lists $SANDBOX/scripts as a worktree.
  echo "$SANDBOX/scripts/.git" >"$SANDBOX/.git/worktrees/fake/gitdir"
  echo "gitdir: $SANDBOX/.git/worktrees/fake" >"$SANDBOX/scripts/.git"
  run git -C "$SANDBOX" worktree list
  has "$SANDBOX/scripts"
  REPO="$SANDBOX" run -2 edit Edit scripts/harness/run.sh
  has 'is a harness file'
  REPO="$SANDBOX" run -2 edit Edit "$SANDBOX/scripts/harness/protected-paths.txt"
  has 'is a harness file'
}

@test "moving this checkout's .git away does not unprotect it" {
  sandbox .claude/hooks/protect-harness.sh .bin/harness scripts/harness/protected-paths.txt scripts/harness/timing.sh
  mv "$SANDBOX/.git" "$SANDBOX/.git.x"
  for p in AGENTS.md .claude/settings.json; do
    REPO="$SANDBOX" run -2 edit Edit "$p"
    has 'is a harness file'
  done
  REPO="$SANDBOX" run -0 edit Edit apps/api/x.go
}

@test "fails closed when git cannot list the worktrees a path needs" {
  sandbox .claude/hooks/protect-harness.sh .bin/harness scripts/harness/protected-paths.txt scripts/harness/timing.sh
  git -C "$SANDBOX" add -A && git -C "$SANDBOX" -c user.name=t -c user.email=t@localhost commit -qm x
  git -C "$SANDBOX" worktree add -q -b lane "$BATS_TEST_TMPDIR/lane"
  mkdir "$BATS_TEST_TMPDIR/nogit" && printf '#!/bin/sh\nexit 1\n' >"$BATS_TEST_TMPDIR/nogit/git"
  chmod +x "$BATS_TEST_TMPDIR/nogit/git"
  PATH="$BATS_TEST_TMPDIR/nogit:$PATH" REPO="$SANDBOX" run -2 edit Edit "$BATS_TEST_TMPDIR/lane/apps/x.go"
  has 'cannot list the worktrees'
  # Paths in this checkout or in no repository need no list.
  PATH="$BATS_TEST_TMPDIR/nogit:$PATH" REPO="$SANDBOX" run -0 edit Edit apps/x.go
  PATH="$BATS_TEST_TMPDIR/nogit:$PATH" REPO="$SANDBOX" run -0 edit Edit "$BATS_TEST_TMPDIR/nogit/x"
}

@test "fails closed on unparseable input" {
  run -2 "$REPO/.claude/hooks/protect-harness.sh" <<<'not json'
  has 'cannot parse'
}
