#!/usr/bin/env bats
# scripts/github-branch-protection.sh: dry run by default; with CONFIRM=1 it PUTs the protection via gh.
bats_require_minimum_version 1.5.0
load helpers

setup() {
  # Stub gh: records its arguments and stdin, never talks to GitHub.
  mkdir -p "$BATS_TEST_TMPDIR/bin"
  cat >"$BATS_TEST_TMPDIR/bin/gh" <<'SH'
#!/usr/bin/env bash
echo "$*" >>"$BATS_TEST_TMPDIR/gh.args"
[[ "$1" == api ]] && cat >"$BATS_TEST_TMPDIR/gh.body"
[[ "$1" == repo ]] && echo "acme/widget"
exit 0
SH
  chmod +x "$BATS_TEST_TMPDIR/bin/gh"
  export PATH="$BATS_TEST_TMPDIR/bin:$PATH"
}

script() { run "$@" "$REPO/scripts/github-branch-protection.sh"; }

@test "dry run prints the request and never calls gh" {
  script -0
  has 'Dry run'
  has '"contexts": ["check-all","harness-selftest","generator-probe","api-no-breaking-change","harness-integrity"]'
  [[ ! -e "$BATS_TEST_TMPDIR/gh.args" ]]
}

@test "CONFIRM=1 sends valid JSON to the protection endpoint" {
  CONFIRM=1 script -0
  has 'applied to acme/widget:main'
  output="$(cat "$BATS_TEST_TMPDIR/gh.args")"
  has 'api -X PUT repos/acme/widget/branches/main/protection'
  node -e 'const b = JSON.parse(require("fs").readFileSync(process.argv[1], "utf8"));
    if (!b.required_pull_request_reviews.require_code_owner_reviews || b.allow_force_pushes) process.exit(1)' \
    "$BATS_TEST_TMPDIR/gh.body"
}

@test "GH_REPO and BRANCH override the target" {
  CONFIRM=1 GH_REPO=o/r BRANCH=release script -0
  output="$(cat "$BATS_TEST_TMPDIR/gh.args")"
  has 'api -X PUT repos/o/r/branches/release/protection'
}
