#!/usr/bin/env bats
# scripts/test/helpers.bash: sandbox() hardlinks executables (no macOS first-exec assessment) and copies data;
# own() breaks the link before a test writes. teardown_file guards the real files in every bats file.
bats_require_minimum_version 1.5.0
load helpers

setup() {
  sandbox scripts/harness/merge-base.sh scripts/harness/protected-paths.txt
}

@test "sandbox hardlinks executables and copies data files" {
  ln "$REPO/scripts/harness/merge-base.sh" "$BATS_TEST_TMPDIR/probe" 2>/dev/null ||
    skip "repo and BATS_TEST_TMPDIR are on different volumes: sandbox copies instead"
  [[ "$SANDBOX/scripts/harness/merge-base.sh" -ef "$REPO/scripts/harness/merge-base.sh" ]]
  [[ ! "$SANDBOX/scripts/harness/protected-paths.txt" -ef "$REPO/scripts/harness/protected-paths.txt" ]]
  cmp "$SANDBOX/scripts/harness/protected-paths.txt" "$REPO/scripts/harness/protected-paths.txt"
}

@test "own breaks the link and keeps content and mode, so a write stays in the sandbox" {
  own scripts/harness/merge-base.sh
  [[ ! "$SANDBOX/scripts/harness/merge-base.sh" -ef "$REPO/scripts/harness/merge-base.sh" ]]
  cmp "$SANDBOX/scripts/harness/merge-base.sh" "$REPO/scripts/harness/merge-base.sh"
  [[ -x "$SANDBOX/scripts/harness/merge-base.sh" ]]
  echo 'exit 0' >>"$SANDBOX/scripts/harness/merge-base.sh"
  run ! cmp -s "$SANDBOX/scripts/harness/merge-base.sh" "$REPO/scripts/harness/merge-base.sh"
}

@test "sandbox_base commits a hardlinked executable as a regular file" {
  sandbox_base
  run -0 git -C "$SANDBOX" ls-files -s scripts/harness/merge-base.sh
  has '100755 '
}

@test "host makes uname -s name that host and leaves the other uname calls alone" {
  host Linux
  run -0 uname -s
  [[ "$output" == Linux ]] || false
  run -0 uname -m
  [[ "$output" == "$(/usr/bin/uname -m)" ]]
}
