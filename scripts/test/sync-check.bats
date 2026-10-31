#!/usr/bin/env bats
# scripts/harness/sync-check.sh (make harness-sync-check): harness files changed in the upstream template since
# .harness-version and not taken here. The upstream is a local repo: no network.
bats_require_minimum_version 1.5.0
load helpers

setup() {
  unset HARNESS_UPSTREAM # CI exports tool-versions.env's empty value (.github/actions/setup)
  sandbox scripts/harness/sync-check.sh
  printf '%s\n' '# harness' 'Makefile config' 'scripts/harness/* config' '.harness-version config' \
    >"$SANDBOX/scripts/harness/protected-paths.txt"
  printf '%s\n' 'GO_TOOLCHAIN=go1' 'HARNESS_UPSTREAM=' >"$SANDBOX/scripts/tool-versions.env"
  printf 'all:\n' >"$SANDBOX/Makefile"
  printf 'old\n' >"$SANDBOX/scripts/harness/old.sh"
  printf 'readme\n' >"$SANDBOX/README.md"
  base_and_upstream
}

# base_and_upstream: commits the sandbox, records that commit in .harness-version and clones it as upstream.
base_and_upstream() {
  sandbox_base
  BASE="$(git -C "$SANDBOX" rev-parse HEAD)"
  printf '# marker\n%s\n' "$BASE" >"$SANDBOX/.harness-version"
  UP="$BATS_TEST_TMPDIR/upstream"
  rm -rf "$UP"
  git clone -q "$SANDBOX" "$UP"
}

# upstream_commit: commits everything in the upstream repo.
upstream_commit() {
  git -C "$UP" add -A
  git -C "$UP" -c user.name=test -c user.email=test@localhost -c commit.gpgsign=false commit -qm upstream
}

check() { HARNESS_UPSTREAM="${HARNESS_UPSTREAM-$UP}" run "$@" "$SANDBOX/scripts/harness/sync-check.sh"; }

@test "silent when upstream changed no harness file" {
  printf 'new readme\n' >"$UP/README.md"
  upstream_commit
  check -0
  [[ -z "$output" ]]
}

@test "lists a harness file that changed upstream, with the upstream commit to record" {
  printf 'all: lint\n' >"$UP/Makefile"
  upstream_commit
  check -1
  has "Makefile: changed upstream since ${BASE:0:12} [harness-upstream-in-sync]"
  has "1 harness file(s) behind $UP"
  has "write $(git -C "$UP" rev-parse HEAD) to .harness-version"
}

@test "skips a file already taken here" {
  printf 'all: lint\n' >"$UP/Makefile"
  upstream_commit
  printf 'all: lint\n' >"$SANDBOX/Makefile"
  check -0
}

@test "reports an upstream change of the executable bit alone" {
  chmod +x "$UP/Makefile"
  upstream_commit
  check -1
  has 'Makefile: changed upstream since'
}

@test "flags a file changed on both sides as a manual merge, and one missing here" {
  printf 'all: lint\n' >"$UP/Makefile"
  printf 'newer\n' >"$UP/scripts/harness/old.sh"
  upstream_commit
  printf 'all: test\n' >"$SANDBOX/Makefile"
  rm "$SANDBOX/scripts/harness/old.sh"
  check -1
  has 'Makefile: changed upstream, changed here too (merge by hand) since'
  has 'scripts/harness/old.sh: changed upstream, missing here since'
}

@test "reports added and deleted files, odd names, and paths only upstream's list protects" {
  printf 'new\n' >"$UP/scripts/harness/new name.sh"
  rm "$UP/scripts/harness/old.sh"
  printf 'x\n' >"$UP/lint.yml"
  printf 'lint.yml config\n' >>"$UP/scripts/harness/protected-paths.txt"
  upstream_commit
  check -1
  has 'scripts/harness/new name.sh: added upstream since'
  has 'scripts/harness/old.sh: deleted upstream since'
  has 'lint.yml: added upstream since'
}

@test "a file deleted upstream and changed here is a manual merge" {
  rm "$UP/scripts/harness/old.sh"
  upstream_commit
  printf 'mine\n' >"$SANDBOX/scripts/harness/old.sh"
  check -1
  has 'scripts/harness/old.sh: deleted upstream, changed here too (merge by hand) since'
}

@test "never compares .harness-version itself" {
  printf '%s\n' 0123456789abcdef0123456789abcdef01234567 >"$UP/.harness-version"
  upstream_commit
  check -0
  [[ -z "$output" ]]
}

@test "skips and lists declined paths; a declined line needs a reason" {
  printf 'all: lint\n' >"$UP/Makefile"
  upstream_commit
  printf '# declined Makefile: we build with bazel\n' >>"$SANDBOX/.harness-version"
  check -0
  has 'Makefile: declined (we build with bazel), changed upstream since'
  printf '# declined Makefile\n' >>"$SANDBOX/.harness-version"
  check -2
  has "write '# declined <path>: <why>'"
}

@test "reads HARNESS_UPSTREAM from tool-versions.env; empty means the template, origin is not consulted" {
  printf 'all: lint\n' >"$UP/Makefile"
  upstream_commit
  git -C "$SANDBOX" remote add origin "$UP"
  HARNESS_UPSTREAM='' check -0
  [[ "$output" == 'sync-check: HARNESS_UPSTREAM is empty: this repository is the template; nothing to sync' ]] || false
  printf 'HARNESS_UPSTREAM=%s\n' "$UP" >>"$SANDBOX/scripts/tool-versions.env"
  HARNESS_UPSTREAM='' check -1
  has "behind $UP"
}

@test "make harness-sync-check takes HARNESS_UPSTREAM from the command line" {
  for f in Makefile scripts/tool-versions.env docs/harness/rules.md; do
    mkdir -p "$SANDBOX/$(dirname "$f")"
    cp "$REPO/$f" "$SANDBOX/$f"
  done
  sandbox scripts/harness/run.sh scripts/harness/timing.sh
  base_and_upstream
  printf '# upstream change\n' >>"$UP/Makefile"
  upstream_commit
  run -0 make -C "$SANDBOX" harness-sync-check
  run -2 make -C "$SANDBOX" harness-sync-check HARNESS_UPSTREAM="$UP"
  has 'Makefile: changed upstream since'
  has 'remediation:'
}

@test "fails closed on a missing or malformed .harness-version" {
  rm "$SANDBOX/.harness-version"
  check -2
  has '.harness-version is missing'
  printf 'v1.2.3\n' >"$SANDBOX/.harness-version"
  check -2
  has "expected one 40-hex template commit, got 'v1.2.3'"
}

@test "fails closed when the marker commit is not in upstream's history" {
  printf '%s\n' 0123456789abcdef0123456789abcdef01234567 >"$SANDBOX/.harness-version"
  check -2
  has 'is not in the history of'
  git -C "$UP" checkout -q -b side
  printf 'side\n' >"$UP/README.md"
  upstream_commit
  git -C "$UP" rev-parse HEAD >"$SANDBOX/.harness-version"
  git -C "$UP" checkout -q main
  check -2
  has 'is not in the history of'
}

@test "fails closed without harness paths or an upstream HEAD" {
  rm "$SANDBOX/scripts/harness/protected-paths.txt"
  git -C "$UP" rm -q scripts/harness/protected-paths.txt
  upstream_commit
  check -2
  has 'no harness paths'
  git init -q --bare "$BATS_TEST_TMPDIR/empty.git"
  HARNESS_UPSTREAM="$BATS_TEST_TMPDIR/empty.git" check -2
  has "cannot fetch HEAD of $BATS_TEST_TMPDIR/empty.git"
}

@test "fails closed when upstream cannot be fetched or uses a refused transport" {
  HARNESS_UPSTREAM="$BATS_TEST_TMPDIR/nope" check -2
  has "cannot fetch HEAD of $BATS_TEST_TMPDIR/nope"
  # Allowed by git config, so only GIT_ALLOW_PROTOCOL stands between ext:: and running a command.
  GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=protocol.ext.allow GIT_CONFIG_VALUE_0=always \
    HARNESS_UPSTREAM="ext::sh -c touch% $BATS_TEST_TMPDIR/pwned" check -2
  has 'cannot fetch HEAD of ext::'
  [[ ! -e "$BATS_TEST_TMPDIR/pwned" ]] || false
  HARNESS_UPSTREAM='--upload-pack=touch' check -2
  has "starts with '-'"
}

@test "refuses a URL with credentials and never prints them" {
  for url in 'https://user:s3cret@example.invalid/t.git' 'https://ghp_s3cret@example.invalid/t.git'; do
    HARNESS_UPSTREAM="$url" check -2
    has 'embeds credentials'
    [[ "$output" != *s3cret* ]] || false
  done
}

@test "removes its temp repo after success and after a failure" {
  export TMPDIR="$BATS_TEST_TMPDIR/tmp"
  mkdir "$TMPDIR"
  check -0
  [[ -z "$(ls -A "$TMPDIR")" ]] || false
  HARNESS_UPSTREAM="$BATS_TEST_TMPDIR/nope" check -2
  [[ -z "$(ls -A "$TMPDIR")" ]]
}
