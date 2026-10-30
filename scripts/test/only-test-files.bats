#!/usr/bin/env bats
# .claude/hooks/only-test-files.sh: the test-writer subagent may write the product test files specs-tested reads
# and nothing else (ADR 0039). The predicate itself is table-tested in apps/api/tools/harness/testfile_test.go.
bats_require_minimum_version 1.5.0
load helpers
# The repo's layout (API_PREFIX, DOMAINS_DIR, API_CLIENT_DIR, WEB_DIRS, the glob lists): the paths below are
# built from it, so the tests hold in an adopted repo too (README "(b) Brownfield project").
# shellcheck source=scripts/harness/layout.sh
source "$REPO/scripts/harness/layout.sh"

# write <file_path> [tool]: feeds a PreToolUse file-tool payload, cwd = the repo, to the hook.
write() {
  node -e 'process.stdout.write(JSON.stringify({ cwd: process.argv[3], tool_name: process.argv[2], tool_input: { file_path: process.argv[1] } }))' \
    "$1" "${2:-Write}" "$REPO" | "$REPO/.claude/hooks/only-test-files.sh"
}

# at <template path>: the same path in this repo's layout. A leading apps/api/internal/ becomes DOMAINS_DIR,
# apps/api/ API_PREFIX, packages/api-client/ API_CLIENT_DIR (a case variant of it upper-cases the client's
# last directory), apps/web/ and packages/domain/ a web/ and domain/ directory in the first and last WEB_GLOBS
# entry; any other path is the same in every layout. In the template, every path stays as it is.
at() {
  local p="$1" lc client="$API_CLIENT_DIR"
  lc=$(tr '[:upper:]' '[:lower:]' <<<"$p")
  case "$lc" in
    apps/api/internal/*) printf '%s\n' "$DOMAINS_DIR/${p#apps/api/internal/}" ;;
    apps/api/*) printf '%s\n' "$API_PREFIX${p#apps/api/}" ;;
    packages/api-client/*)
      [[ "${p:0:19}" == packages/api-client ]] ||
        client="$(dirname "$client")/$(basename "$client" | tr '[:lower:]' '[:upper:]')"
      printf '%s\n' "${client#./}/${p:20}"
      ;;
    apps/web/*) printf '%s\n' "${WEB_DIRS[0]}/web/${p#apps/web/}" ;;
    packages/domain/*) printf '%s\n' "${WEB_DIRS[${#WEB_DIRS[@]} - 1]}/domain/${p#packages/domain/}" ;;
    *) printf '%s\n' "$p" ;;
  esac
}

# keeps <generated:<glob>|protected:<pattern>>: this repo still has the template entry, mapped by at, in
# GENERATED_GLOBS or scripts/harness/protected-paths.txt.
keeps() {
  local e g
  e=$(at "${1#*:}")
  case "$1" in
    generated:*)
      for g in "${GENERATED_GLOBS[@]}"; do [[ "$g" == "$e" ]] && return 0; done
      return 1
      ;;
    protected:*) awk -v e="$e" '$1 == e { found = 1 } END { exit !found }' "$REPO/scripts/harness/protected-paths.txt" ;;
    *)
      echo "unknown entry kind: $1" >&2
      return 2
      ;;
  esac
}

# product <path>: path is in PRODUCT_GLOBS or WEB_GLOBS (`*` matches `/`, as in layout.env).
product() {
  local g
  for g in "${PRODUCT_GLOBS[@]}" "${WEB_GLOBS[@]}"; do
    # shellcheck disable=SC2053 # reason: $g is a glob, matched as one (rule: sh-shellcheck)
    [[ "$1" == $g ]] && return 0
  done
  return 1
}

@test "allows each test-file suffix, absolute or relative to cwd" {
  local f
  for f in apps/api/internal/x/model/x_test.go packages/domain/src/a.test.ts apps/web/src/a.test.tsx \
    apps/web/src/a.test.mjs apps/web/e2e/a.e2e.ts; do
    run -0 write "$(at "$f")"
  done
  run -0 write "$REPO/$(at apps/api/internal/y/service/new_test.go)"
  run -0 write "$(at apps/api/internal/x/model/x_test.go)" MultiEdit
}

@test "allows exactly the product test files of testdata/test-files.txt, as specs-tested counts them" {
  local p want hinge q n=0
  while read -r p want hinge; do
    [[ -z "$p" || "$p" == \#* ]] && continue
    n=$((n + 1))
    q=$(at "$p")
    # The reject hinges on a template entry this layout does not have: the file is product code here.
    if [[ -n "$hinge" ]] && ! keeps "$hinge"; then want=allow; fi
    if [[ "$want" == allow ]]; then
      run -0 write "$q"
    else
      run -2 write "$q"
      has "only-test-files: blocked: $q is "
    fi
  done <"$REPO/${API_PREFIX}tools/harness/testdata/test-files.txt"
  ((n > 0))
}

@test "names the test file forms and why a test file is refused" {
  local f
  run -2 write "$(at packages/domain/src/a.ts)"
  has 'is not a test file (*_test.go, or *.test.*, *.spec.* or *.e2e.* ending in .js, .ts, .jsx or .tsx'
  f=$(at apps/api/internal/x/store/q_test.go)
  if keeps 'generated:apps/api/internal/*/store/*'; then
    run -2 write "$f"
    has 'is generated code (GENERATED_GLOBS:'
  else # no generated store packages in this layout (a root module without sqlc): a store test is product code
    run -0 write "$f"
  fi
  run -2 write "$(at apps/web/playwright.config.test.ts)"
  has 'is a harness file (*playwright.config.* in scripts/harness/protected-paths.txt)'
  f=$(at apps/api/tools/harness/a_test.go)
  run -2 write "$f"
  if product "$f"; then
    has "is a harness file (${API_PREFIX}tools/harness/* in scripts/harness/protected-paths.txt)"
  else # API_DIR=.: tools/ is outside PRODUCT_GLOBS, which is checked first
    has "only-test-files: blocked: $f is not product code (PRODUCT_GLOBS or WEB_GLOBS:"
  fi
}

@test "blocks a non-test file and names it" {
  local f
  for f in apps/api/internal/x/model/x.go packages/domain/src/a.ts apps/api/internal/x/model/x_test.go.orig; do
    f=$(at "$f")
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
