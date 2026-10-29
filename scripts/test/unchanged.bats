#!/usr/bin/env bats
# scripts/harness/unchanged.sh: make check skips a sensor (exit 0) only when none of its inputs and no harness
# file changed since the merge base; anything else, including an error, means run (exit 1).
bats_require_minimum_version 1.5.0
load helpers

setup() {
  sandbox scripts/harness/unchanged.sh scripts/harness/merge-base.sh scripts/harness/protected-paths.txt
  U="$SANDBOX/scripts/harness/unchanged.sh"
  mkdir -p "$SANDBOX/scripts" "$SANDBOX/apps/api"
  printf 'echo hi\n' >"$SANDBOX/scripts/tool.sh"
  printf 'package api\n' >"$SANDBOX/apps/api/a.go"
  printf 'check:\n' >"$SANDBOX/Makefile"
  sandbox_base
  git -C "$SANDBOX" checkout -qb feature
  export BASE_REF=main # not the caller's (make check BASE_REF=HEAD would hide committed changes)
}

commit() {
  git -C "$SANDBOX" add -A
  git -C "$SANDBOX" -c user.name=test -c user.email=test@localhost -c commit.gpgsign=false commit -qm change
}

@test "skips when only files outside its pathspecs changed (committed, unstaged, untracked)" {
  echo '// x' >>"$SANDBOX/apps/api/a.go"
  commit
  echo '// y' >>"$SANDBOX/apps/api/a.go"
  printf 'package api\n' >"$SANDBOX/apps/api/b.go"
  run -0 "$U" '*.sh'
}

@test "runs when a matching file changed: committed, staged, unstaged or untracked" {
  echo 'echo 1' >>"$SANDBOX/scripts/tool.sh"
  commit
  run -1 "$U" '*.sh'

  git -C "$SANDBOX" reset -q --hard main
  echo 'echo 2' >>"$SANDBOX/scripts/tool.sh"
  git -C "$SANDBOX" add scripts/tool.sh
  run -1 "$U" '*.sh'

  git -C "$SANDBOX" reset -q --hard main
  echo 'echo 3' >>"$SANDBOX/scripts/tool.sh"
  run -1 "$U" '*.sh'

  git -C "$SANDBOX" reset -q --hard main
  printf 'echo new\n' >"$SANDBOX/apps/new.sh"
  run -1 "$U" '*.sh'
}

@test "runs when any harness file changed, whatever the pathspecs" {
  printf 'check:\n\t@true\n' >"$SANDBOX/Makefile"
  run -1 "$U" '*.sh'
}

@test "runs in scope full (make check-all, CI)" {
  HARNESS_SCOPE=full run -1 "$U" '*.sh'
}

@test "runs when there is no merge base (fails closed)" {
  BASE_REF=no-such-ref run -1 "$U" '*.sh'
}

@test "needs a pathspec" {
  run -2 "$U"
  has 'usage: unchanged.sh <pathspec>...'
}

@test "make check-all exports HARNESS_SCOPE=full to make check, which then runs uncached; make check alone does not" {
  # Real Makefile, recipes of check and check-all replaced (make 3.81 has no --eval).
  printf '%s\n' "include $REPO/Makefile" \
    'check: ; @echo "scope=$${HARNESS_SCOPE-}. count=$(GO_TEST_COUNT). dc=$(DEPCRUISE_CACHE). turbo=$(TURBO_FORCE)."' 'check-all: ; @true' >"$BATS_TEST_TMPDIR/scope.mk"
  run -0 make -C "$REPO" -f "$BATS_TEST_TMPDIR/scope.mk" check-all
  has 'scope=full. count=-count=1. dc=. turbo=--force.'
  run -0 make -C "$REPO" -f "$BATS_TEST_TMPDIR/scope.mk" check
  has 'scope=. count=. dc=--cache node_modules/.cache/depcruise. turbo=.'
  HARNESS_SCOPE=changed run -0 make -C "$REPO" -f "$BATS_TEST_TMPDIR/scope.mk" check
  has 'count=. dc=--cache'
  HARNESS_SCOPE=ful run -0 make -C "$REPO" -f "$BATS_TEST_TMPDIR/scope.mk" check # any other scope: uncached
  has 'count=-count=1. dc=. turbo=--force.'
}

@test "make harness-determinism runs in scope full, as the nightly run does" {
  printf '%s\n' "include $REPO/Makefile" 'harness-determinism: ; @echo "scope=$${HARNESS_SCOPE-}."' >"$BATS_TEST_TMPDIR/det.mk"
  run -0 make -C "$REPO" -f "$BATS_TEST_TMPDIR/det.mk" harness-determinism
  has 'scope=full.'
}
