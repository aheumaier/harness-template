#!/usr/bin/env bats
# scripts/harness/affected.sh: the packages (go) or workspaces (web) make check scopes its sensors to: what
# changed since the merge base plus everything that depends on it. Exit 1 (run on everything) whenever the
# set cannot be narrowed safely, including any error.
bats_require_minimum_version 1.5.0
load helpers

setup() {
  sandbox scripts/harness/affected.sh scripts/harness/unchanged.sh scripts/harness/merge-base.sh \
    scripts/harness/protected-paths.txt scripts/tool-versions.env
  A="$SANDBOX/scripts/harness/affected.sh"
  # model ← service ← cmd; other imports model only in its external test; lone imports nothing.
  local api="$SANDBOX/apps/api"
  mkdir -p "$api/model" "$api/service" "$api/cmd" "$api/other" "$api/lone" "$SANDBOX/packages/domain/src" "$SANDBOX/docs"
  printf 'module example.com/api\n\ngo 1.21\n' >"$api/go.mod"
  printf 'package model\n\nfunc Answer() int { return 42 }\n' >"$api/model/model.go"
  printf 'package service\n\nimport "example.com/api/model"\n\nvar V = model.Answer()\n' >"$api/service/service.go"
  printf 'package main\n\nimport "example.com/api/service"\n\nfunc main() { _ = service.V }\n' >"$api/cmd/main.go"
  printf 'package other\n' >"$api/other/other.go"
  printf 'package other_test\n\nimport (\n\t"testing"\n\n\t"example.com/api/model"\n)\n\nfunc TestX(t *testing.T) { _ = model.Answer() }\n' >"$api/other/other_test.go"
  printf 'package lone\n' >"$api/lone/lone.go"
  printf 'export const x = 1;\n' >"$SANDBOX/packages/domain/src/index.ts"
  printf '{}\n' >"$SANDBOX/packages/domain/package.json"
  printf '# docs\n' >"$SANDBOX/docs/x.md"
  sandbox_base
  git -C "$SANDBOX" checkout -qb feature
  export BASE_REF=main   # not the caller's (make check BASE_REF=HEAD would hide committed changes)
  unset MAKEFLAGS MFLAGS # make check-all BASE_REF=origin/main (ci.yml:44) would reach the inner make below via MAKEFLAGS
}

# is <text>: $output is exactly text (a function: see has in helpers.bash).
is() {
  [[ "$output" == "$1" ]] && return 0
  printf 'expected output:\n%s\nactual output:\n%s\n' "$1" "$output" >&2
  return 1
}

# turbo stub: workspaces apps/web, packages/api-client (apps/web depends on it), packages/domain.
stub_turbo() {
  mkdir -p "$BATS_TEST_TMPDIR/bin"
  cat >"$BATS_TEST_TMPDIR/bin/turbo" <<'EOF'
#!/usr/bin/env bash
[[ -n "${TURBO_FAIL:-}" ]] && exit 1
paths=()
for a in "$@"; do
  case "$a" in
  '--filter=...{./packages/api-client}') paths+=(packages/api-client apps/web) ;;
  --filter=*) w="${a#--filter=...\{./}" && paths+=("${w%\}}") ;;
  esac
done
((${#paths[@]})) || paths=(apps/web packages/api-client packages/domain)
for p in "${paths[@]}"; do printf '    "path": "%s",\n' "$p"; done
EOF
  chmod +x "$BATS_TEST_TMPDIR/bin/turbo"
  export PATH="$BATS_TEST_TMPDIR/bin:$PATH"
}

@test "lists nothing when nothing it reads changed (Markdown, files of the other kind)" {
  stub_turbo
  echo more >>"$SANDBOX/docs/x.md"
  run -0 "$A" go
  is ""
  run -0 "$A" web
  is ""
  echo '// x' >>"$SANDBOX/apps/api/lone/lone.go"
  run -0 "$A" web
  is ""
}

@test "go: a changed package and every package that depends on it, through tests too, sorted" {
  echo '// x' >>"$SANDBOX/apps/api/model/model.go"
  run -0 "$A" go
  is "$(printf './cmd\n./model\n./other\n./service')"
  git -C "$SANDBOX" checkout -q -- .
  printf 'package lone\n' >"$SANDBOX/apps/api/lone/new.go" # untracked
  run -0 "$A" go
  is ./lone
}

@test "go: everything on a non-Go file under apps/api or a .go file in no package (deleted)" {
  printf 'select 1;\n' >"$SANDBOX/apps/api/q.sql"
  run -1 "$A" go
  rm "$SANDBOX/apps/api/q.sql"
  rm -r "$SANDBOX/apps/api/lone"
  run -1 "$A" go
}

@test "go: the module is where layout.env's API_DIR says (here the repo root)" {
  stub_turbo
  git -C "$SANDBOX" checkout -q main
  mv "$SANDBOX/apps/api/"* "$SANDBOX"
  sed -i.bak 's|^API_DIR=.*|API_DIR=.|' "$SANDBOX/scripts/harness/layout.env" && rm "$SANDBOX/scripts/harness/layout.env.bak"
  sandbox_base
  git -C "$SANDBOX" checkout -qB feature
  echo '// x' >>"$SANDBOX/service/service.go"
  run -0 "$A" go
  is "$(printf './cmd\n./service')"
  run -0 "$A" web # Go files of a root module are no workspace's
  is ""
}

@test "go: everything when go list fails" {
  printf 'package service\n\nimport "example.com/api/nope"\n' >"$SANDBOX/apps/api/service/broken.go"
  run -1 "$A" go
}

@test "web: a changed workspace and its dependents" {
  stub_turbo
  mkdir -p "$SANDBOX/packages/api-client/src"
  printf 'export {};\n' >"$SANDBOX/packages/api-client/src/index.ts"
  echo '// x' >>"$SANDBOX/packages/domain/src/index.ts"
  run -0 "$A" web
  is "$(printf 'apps/web\npackages/api-client\npackages/domain')"
}

@test "web: everything on a JS, TS or JSON file outside a workspace, or when turbo fails" {
  stub_turbo
  printf 'export default [];\n' >"$SANDBOX/eslint.config.mjs"
  run -1 "$A" web
  rm "$SANDBOX/eslint.config.mjs"
  echo '// x' >>"$SANDBOX/packages/domain/src/index.ts"
  TURBO_FAIL=1 run -1 "$A" web
}

@test "web: an asset catalog of the iOS app (layout.env's IOS_DIR) is no web code, its other JSON or JS still is" {
  stub_turbo
  mkdir -p "$SANDBOX/apps/ios/Assets.xcassets"
  printf '{}\n' >"$SANDBOX/apps/ios/Assets.xcassets/Contents.json"
  run -0 "$A" web
  is ""
  HARNESS_STACK=go,web run -1 "$A" web            # without ios in STACK, IOS_DIR is unchecked and means nothing
  printf '{}\n' >"$SANDBOX/apps/ios/strings.json" # web code may import it
  run -1 "$A" web
  rm "$SANDBOX/apps/ios/strings.json"
  echo 'var x = 1' >"$SANDBOX/apps/ios/a.js" # eslint . lints it in scope full: changed must run everything too
  run -1 "$A" web
  rm "$SANDBOX/apps/ios/a.js"
  mkdir -p "$SANDBOX/apps/mobile" # outside IOS_DIR: a JSON file in no workspace
  printf '{}\n' >"$SANDBOX/apps/mobile/Contents.json"
  run -1 "$A" web
}

@test "everything on go.mod, a package.json, a harness file, scope full or no merge base" {
  echo >>"$SANDBOX/apps/api/go.mod"
  run -1 "$A" go
  run -1 "$A" web
  git -C "$SANDBOX" checkout -q -- .
  echo ' ' >>"$SANDBOX/packages/domain/package.json"
  run -1 "$A" go
  git -C "$SANDBOX" checkout -q -- .
  printf 'check:\n' >"$SANDBOX/Makefile"
  run -1 "$A" go
  rm "$SANDBOX/Makefile"
  HARNESS_SCOPE=full run -1 "$A" go
  BASE_REF=no-such-ref run -1 "$A" go
}

@test "needs go or web" {
  run -2 "$A"
  has 'usage: affected.sh go|web'
  run -2 "$A" js
}

@test "make: scoped runs the command on the affected set, skips it on none, runs it on everything otherwise" {
  # Real Makefile (its scoped helper), recipe from this test, run in the sandbox.
  printf '%s\n' "include $REPO/Makefile" 't: ; @$(call scoped,go,./...) echo ran $$scope # unquoted, as the sensors take it' >"$BATS_TEST_TMPDIR/s.mk"
  run -0 make -s -C "$SANDBOX" -f "$BATS_TEST_TMPDIR/s.mk" t
  has 't: skipped, nothing it checks is affected (make check-all runs it)'
  echo '// x' >>"$SANDBOX/apps/api/service/service.go"
  run -0 make -s -C "$SANDBOX" -f "$BATS_TEST_TMPDIR/s.mk" t
  has 'ran ./cmd ./service'
  HARNESS_SCOPE=full run -0 make -s -C "$SANDBOX" -f "$BATS_TEST_TMPDIR/s.mk" t
  has 'ran ./...'
  printf '%s\n' "include $REPO/Makefile" 't: ; @$(call scoped,go,./...) false' >"$BATS_TEST_TMPDIR/f.mk"
  run -2 make -s -C "$SANDBOX" -f "$BATS_TEST_TMPDIR/f.mk" t
}
